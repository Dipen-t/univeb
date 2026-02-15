package repository

import (
	"crypto/rand"
	"math/big"
	"univibe/conductor/models"
	"gorm.io/gorm"
	"log"
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
func (r *RoomRepo) FindOrCreateSong(inputSong *models.Song) (*models.Song, error) {
	var song models.Song

	// 1. Check if song already exists by YouTube ID
    // We use .First() to allow GORM to fill the 'song' struct with the DB data (including UUID)
	err := r.DB.Where("youtube_id = ?", inputSong.YoutubeID).First(&song).Error

	if err == nil {
		// ✅ Song found! Return it (it has the correct UUID)
		return &song, nil
	}

	// 2. If not found, Create it!
	log.Printf("🎵 Song not found in DB. Creating: %s", inputSong.Title)
	
    // Create new song record
	newSong := models.Song{
		Title:     inputSong.Title,
		Artist:    inputSong.Artist,
		YoutubeID: inputSong.YoutubeID,
		CoverURL:  inputSong.CoverURL,
	}

	err = r.DB.Create(&newSong).Error
	if err != nil {
		log.Printf("❌ Failed to create song: %v", err)
		return nil, err
	}

	// ✅ Return the newly created song (GORM fills in the new ID automatically)
	return &newSong, nil
}