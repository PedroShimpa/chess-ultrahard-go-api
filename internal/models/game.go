package models

type Game struct {
	ID    uint   `gorm:"primaryKey"`
	UserID string `gorm:"not null;index"`
	Fen    string `gorm:"not null"`
	Moves  []Move `gorm:"foreignKey:GameID"`
}
