package models

import (
	"time"
	
)

// --- DATABASE MODELS ---

type Room struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Code      string `gorm:"uniqueIndex"`
	IsPlaying bool   `gorm:"default:false"` // <--- NEW: Tracks Play/Pause state
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Song struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	YoutubeID string `gorm:"uniqueIndex"`
	Title     string
	Artist    string
	CoverURL  string
	Duration  int
}

type QueueItem struct {
	ID        uint   `gorm:"primaryKey"`
	RoomID    string `gorm:"type:uuid;index"` // Foreign Key
	SongID    string `gorm:"type:uuid"`       // Foreign Key
	Song      Song   `gorm:"foreignKey:SongID"`
	Status    string `gorm:"default:'waiting'"` // 'playing', 'waiting', 'played'
	Position  int    `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time // Used to sort history
}

// --- JSON MESSAGES ---

type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type SongJSON struct {
	QueueID   uint   `json:"queue_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	YoutubeID string `json:"youtube_id"`
	CoverURL  string `json:"cover_url"`
	Status    string `json:"status"` // <--- NEW FIELD
}