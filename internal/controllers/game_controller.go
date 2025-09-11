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
	"github.com/pedroShimpa/go-api/internal/models"
	"gorm.io/gorm"
)

type Game struct {
	ID     uint   `gorm:"primaryKey"`
	UserID string `gorm:"index"`
	Fen    string `gorm:"type:text"`
	Moves  []Move
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
	m  map[string]*GameController
	mu sync.Mutex
}{m: make(map[string]*GameController)}

var db *gorm.DB

func InitDatabase(database *gorm.DB) {
	db = database
	db.AutoMigrate(&Game{}, &Move{})
}

func NewGameForUser(username string) *GameController {
	g := chess.NewGame()
	controller := &GameController{
		game: g,
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

	game := models.Game{
		UserID: username,
		Fen:    controller.game.FEN(),
	}

	if err := db.Create(&game).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar partida"})
		return
	}

	controller.gameID = game.ID

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
	move, err := UCItoMove(gc.game, req.Move)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jogada inválida"})
		return
	}
	if err := gc.game.Move(move); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jogada inválida"})
		return
	}
	fen := gc.game.Position().String()
	bestMove, _, err := runStockfish(fen, 500)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao rodar engine"})
		return
	}
	if bestMove == "(none)" || bestMove == "" {
		c.JSON(http.StatusOK, gin.H{"message": "jogo terminado"})
		return
	}
	engineMove, err := UCItoMove(gc.game, bestMove)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine move inválido"})
		return
	}
	if err := gc.game.Move(engineMove); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "engine move inválido"})
		return
	}
	moveRecord := models.Move{
		GameID:     gc.gameID,
		Player:     username,
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
	userIDVal, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
		return
	}
	username := userIDVal.(string)

	gc := GetGameForUser(username)
	if gc == nil {
		gc = NewGameForUser(username)
		game := models.Game{
			UserID: username,
			Fen:    gc.game.FEN(),
		}
		if err := db.Create(&game).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar partida"})
			return
		}
		gc.gameID = game.ID
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

	moveRecord := models.Move{
		GameID:     gc.gameID,
		Player:     username,
		PlayerMove: req.Move,
		EngineMove: bestMove,
		Fen:        gc.game.FEN(),
	}
	if err := db.Create(&moveRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao salvar movimento"})
		return
	}
	db.Model(&models.Game{}).Where("id = ?", gc.gameID).Update("fen", gc.game.FEN())

	c.JSON(http.StatusOK, gin.H{
		"player_move":        req.Move,
		"fen":                gc.game.FEN(),
		"evaluation":         eval,
		"best_move_for_side": bestMove,
	})
}
