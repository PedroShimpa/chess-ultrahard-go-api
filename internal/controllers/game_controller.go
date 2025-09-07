package controllers

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/notnil/chess"
)

type GameController struct {
	mu     sync.Mutex
	game   *chess.Game
	engine *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
}

var games = struct {
	m  map[string]*GameController
	mu sync.Mutex
}{m: make(map[string]*GameController)}

func NewGameForUser(username string) *GameController {
	cmd := exec.Command(os.Getenv("STOCKFISH_PATH"))

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	writer := bufio.NewWriter(stdin)
	scanner := bufio.NewScanner(stdout)

	g := chess.NewGame()

	controller := &GameController{
		game:   g,
		engine: cmd,
		stdin:  writer,
		stdout: scanner,
	}

	games.mu.Lock()
	games.m[username] = controller
	games.mu.Unlock()

	return controller
}

func GetGameForUser(username string) *GameController {
	games.mu.Lock()
	defer games.mu.Unlock()
	return games.m[username]
}

func StartGame(c *gin.Context) {
	userIDVal, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
		return
	}
	username := userIDVal.(string)

	controller := NewGameForUser(username)
	c.JSON(http.StatusOK, gin.H{"message": "nova partida iniciada", "fen": controller.game.FEN()})
}

func MakeMove(c *gin.Context) {
	userIDVal, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
		return
	}
	username := userIDVal.(string)

	gc := GetGameForUser(username)
	if gc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nenhuma partida ativa"})
		return
	}

	var req struct {
		Move string `json:"move"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Move == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "movimento inválido"})
		return
	}

	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Converte UCI para *chess.Move
	move, err := UCItoMove(gc.game, req.Move)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jogada inválida"})
		return
	}

	// Aplica movimento do jogador
	if err := gc.game.Move(move); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jogada inválida"})
		return
	}

	// Atualiza posição para o motor
	fen := gc.game.Position().String()
	fmt.Fprintf(gc.stdin, "position fen %s\n", fen)
	gc.stdin.Flush()
	fmt.Fprintf(gc.stdin, "go movetime 500\n")
	gc.stdin.Flush()

	// Lê melhor movimento do engine
	var bestMove string
	for gc.stdout.Scan() {
		line := gc.stdout.Text()
		if len(line) >= 8 && line[:8] == "bestmove" {
			fmt.Sscanf(line, "bestmove %s", &bestMove)
			break
		}
	}

	// Se não houver movimento, jogo terminou
	if bestMove == "(none)" || bestMove == "" {
		c.JSON(http.StatusOK, gin.H{"message": "jogo terminado"})
		return
	}

	// Aplica movimento do engine
	engineMove, err := UCItoMove(gc.game, bestMove)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine move inválido"})
		return
	}
	if err := gc.game.Move(engineMove); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine move inválido"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"player_move": req.Move,
		"engine_move": bestMove,
		"fen":         gc.game.FEN(),
	})
}

func UCItoMove(g *chess.Game, uci string) (*chess.Move, error) {
	from := uci[:2]
	to := uci[2:4]
	for _, m := range g.ValidMoves() {
		if m.S1().String() == from && m.S2().String() == to {
			// Ignora promoção por enquanto
			return m, nil
		}
	}
	return nil, errors.New("movimento inválido")
}
