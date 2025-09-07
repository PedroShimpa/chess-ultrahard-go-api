package models

type Move struct {
	ID         uint   `gorm:"primaryKey"`
	GameID     uint   `gorm:"not null;index"` 
	Player string `gorm:"not null"`       
	PlayerMove string `gorm:"not null"`   
	EngineMove string `gorm:"not null"`   
	Fen        string `gorm:"not null"`   
}
