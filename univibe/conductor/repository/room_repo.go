package repository

import (
	"crypto/rand"
	"math/big"
	"univibe/conductor/models"
	"gorm.io/gorm"
)

type RoomRepo struct {
	DB *gorm.DB
}

func NewRoomRepo(db *gorm.DB) *RoomRepo {
	return &RoomRepo{DB: db}
}

// 1. Generate a random 4-letter code
func generateRoomCode() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	ret := make([]byte, 4)
	for i := 0; i < 4; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		ret[i] = letters[num.Int64()]
	}
	return string(ret)
}

// 2. Create a Room
func (r *RoomRepo) CreateRoom() (*models.Room, error) {
	// Try up to 5 times to generate a unique code
	for i := 0; i < 5; i++ {
		code := generateRoomCode()
		room := models.Room{Code: code}
		
		// Try to create. If code exists, it will fail (due to unique index).
		if err := r.DB.Create(&room).Error; err == nil {
			return &room, nil
		}
	}
	return nil, gorm.ErrInvalidData // Should rarely happen
}

// 3. Find a Room (For Joining)
func (r *RoomRepo) GetRoom(code string) (*models.Room, error) {
	var room models.Room
	err := r.DB.Where("code = ?", code).First(&room).Error
	return &room, err
}