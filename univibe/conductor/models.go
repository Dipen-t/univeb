package main

import (
	"time"
)

type Room struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Code      string `gorm:"uniqueIndex"`
	CreatedAt time.Time
}

type Song struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	YoutubeID string `gorm:"uniqueIndex"`
	Title     string
	Artist    string
	CoverURL  string
	Duration  int
	CreatedAt time.Time
}

type QueueItem struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	RoomID    string 
	SongID    string 
	Song      Song   `gorm:"foreignKey:SongID"` // Join logic
	Status    string // 'waiting', 'playing', 'played'
	Position  int
	AddedBy   string
	CreatedAt time.Time
}