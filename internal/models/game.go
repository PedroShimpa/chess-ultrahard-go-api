package models

type Game struct {
	ID    uint   `gorm:"primaryKey"`
	Fen    string `gorm:"not null"`
	Moves  []Move `gorm:"foreignKey:GameID"`
}
