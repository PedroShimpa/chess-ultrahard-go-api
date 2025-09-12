package controllers

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/notnil/chess"
	"gorm.io/gorm"
)

type Game struct {
	ID    uint   `gorm:"primaryKey"`
	Fen   string `gorm:"type:text"`
	Moves []Move
}

type Move struct {
	ID         uint `gorm:"primaryKey"`
	GameID     uint `gorm:"index"`
	PlayerMove string
	EngineMove string
	Fen        string `gorm:"type:text"`
}

type GameController struct {
	mu     sync.Mutex
	game   *chess.Game
	gameID uint
}

var games = struct {
	m  map[uint]*GameController
	mu sync.Mutex
}{m: make(map[uint]*GameController)}

var db *gorm.DB

func InitDatabase(database *gorm.DB) {
	db = database
	db.AutoMigrate(&Game{}, &Move{})
}

func NewGame() *GameController {
	g := chess.NewGame()
	controller := &GameController{
		game: g,
	}
	return controller
}

func StartGame(c *gin.Context) {
	controller := NewGame()

	game := Game{
		Fen: controller.game.FEN(),
	}

	if err := db.Create(&game).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar partida"})
		return
	}

	controller.gameID = game.ID

	games.mu.Lock()
	games.m[game.ID] = controller
	games.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "nova partida iniciada",
		"fen":     controller.game.FEN(),
		"game_id": game.ID,
	})
}

func runStockfish(fen string, movetime int) (string, string, error) {
	cmd := exec.Command(os.Getenv("STOCKFISH_PATH"))
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	writer := bufio.NewWriter(stdin)

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	fmt.Fprintf(writer, "uci\n")
	writer.Flush()
	fmt.Fprintf(writer, "position fen %s\n", fen)
	writer.Flush()
	fmt.Fprintf(writer, "go movetime %d\n", movetime)
	writer.Flush()

	var bestMove string
	var eval string

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) >= 2 && fields[0] == "bestmove" {
			bestMove = fields[1]
			break
		}

		for i := 0; i < len(fields); i++ {
			if fields[i] == "score" && i+2 < len(fields) {
				if fields[i+1] == "cp" || fields[i+1] == "mate" {
					eval = fields[i+2]
				}
			}
		}
	}

	cmd.Process.Kill()
	return bestMove, eval, nil
}

func MakeMove(c *gin.Context) {
	var req struct {
		GameID uint   `json:"game_id"`
		Move   string `json:"move"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Move == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("movimento inválido: %v", err),
		})
		return
	}

	games.mu.Lock()
	gc := games.m[req.GameID]
	games.mu.Unlock()

	if gc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nenhuma partida ativa"})
		return
	}

	gc.mu.Lock()
	defer gc.mu.Unlock()

	move, err := UCItoMove(gc.game, req.Move)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("jogada inválida: %v", err),
		})
		return
	}

	if err := gc.game.Move(move); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("jogada inválida: %v", err),
		})
		return
	}

	fen := gc.game.Position().String()
	bestMove, _, err := runStockfish(fen, 500)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("falha ao rodar engine: %v", err),
		})
		return
	}

	if bestMove == "(none)" || bestMove == "" {
		c.JSON(http.StatusOK, gin.H{"message": "jogo terminado"})
		return
	}

	engineMove, err := UCItoMove(gc.game, bestMove)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("engine move inválido: %v", err),
		})
		return
	}
	if err := gc.game.Move(engineMove); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("engine move inválido: %v", err),
		})
		return
	}

	moveRecord := Move{
		GameID:     gc.gameID,
		PlayerMove: req.Move,
		EngineMove: bestMove,
		Fen:        gc.game.FEN(),
	}
	if err := db.Create(&moveRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("erro ao salvar movimento: %v", err),
		})
		return
	}
	if err := db.Model(&Game{}).
		Where("id = ?", gc.gameID).
		Update("fen", gc.game.FEN()).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("erro ao atualizar jogo: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"player_move": req.Move,
		"engine_move": bestMove,
		"fen":         gc.game.FEN(),
		"game_id":     gc.gameID,
	})
}

func UCItoMove(g *chess.Game, uci string) (*chess.Move, error) {
	from := uci[:2]
	to := uci[2:4]
	for _, m := range g.ValidMoves() {
		if m.S1().String() == from && m.S2().String() == to {
			return m, nil
		}
	}
	return nil, errors.New("movimento inválido")
}

func SoloGame(c *gin.Context) {
	var req struct {
		GameID uint   `json:"game_id"`
		Move   string `json:"move"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Move == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "movimento inválido"})
		return
	}

	games.mu.Lock()
	gc := games.m[req.GameID]
	games.mu.Unlock()

	if gc == nil {
		gc = NewGame()
		game := Game{
			Fen: gc.game.FEN(),
		}
		if err := db.Create(&game).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar partida"})
			return
		}
		gc.gameID = game.ID

		games.mu.Lock()
		games.m[game.ID] = gc
		games.mu.Unlock()
	}

	gc.mu.Lock()
	defer gc.mu.Unlock()

	move, err := UCItoMove(gc.game, req.Move)
	if err != nil || gc.game.Move(move) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jogada inválida"})
		return
	}

	bestMove, eval, err := runStockfish(gc.game.FEN(), 2000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao rodar engine"})
		return
	}

	moveRecord := Move{
		GameID:     gc.gameID,
		PlayerMove: req.Move,
		EngineMove: bestMove,
		Fen:        gc.game.FEN(),
	}
	if err := db.Create(&moveRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar movimento"})
		return
	}
	db.Model(&Game{}).Where("id = ?", gc.gameID).Update("fen", gc.game.FEN())

	c.JSON(http.StatusOK, gin.H{
		"player_move":        req.Move,
		"fen":                gc.game.FEN(),
		"evaluation":         eval,
		"best_move_for_side": bestMove,
		"game_id":            gc.gameID,
	})
}
