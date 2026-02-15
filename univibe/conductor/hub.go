package main

import (
	"context"
	"encoding/json"
	"log"

	"univibe/conductor/handlers"
	"univibe/conductor/models"
	"univibe/conductor/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	rdb        *redis.Client
	repo       *repository.QueueRepo
	handler    *handlers.EventHandler
}

func newHub(rdb *redis.Client, db *gorm.DB) *Hub {
	repo := repository.NewQueueRepo(db)
	handler := handlers.NewEventHandler(repo)

	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rdb:        rdb,
		repo:       repo,
		handler:    handler,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			go h.SendInitialState(client)

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			var msg struct {
				Type string      `json:"type"`
				Data interface{} `json:"data"`
				Room string      `json:"room"`
			}
			if err := json.Unmarshal(message, &msg); err != nil {
				continue
			}

			// Look up the actual room from the payload
			room, err := h.repo.GetRoom(msg.Room)
			if err != nil {
				continue
			}
			roomID := room.ID
			roomCode := room.Code

			var response *models.WebSocketMessage

			switch msg.Type {
			case "NEXT_SONG":
				response = h.handler.HandleNext(roomID)
				if response != nil {
					h.broadcastToRoom(roomCode, response)
					h.broadcastQueue(roomID, roomCode)
					continue
				}

			case "PREV_SONG":
				response = h.handler.HandlePrevious(roomID)
				if response != nil {
					h.broadcastToRoom(roomCode, response)
					h.broadcastQueue(roomID, roomCode)
					continue
				}

			case "JUMP_TO_SONG":
				if idFloat, ok := msg.Data.(float64); ok {
					response = h.handler.HandleJump(roomID, uint(idFloat))
					if response != nil {
						h.broadcastToRoom(roomCode, response)
						h.broadcastQueue(roomID, roomCode)
						continue
					}
				}

			case "PAUSE":
				response = h.handler.HandlePause(roomID)

			case "PLAY":
				response = h.handler.HandlePlay(roomID)

			case "SEARCH":
				query, ok := msg.Data.(string)
				if ok {
					log.Printf("🔍 Search Request in %s: %s", roomCode, query)
					h.pushSearchJob(roomCode, query)
				}
				continue

			case "REMOVE_SONG":
				var id uint
				if idFloat, ok := msg.Data.(float64); ok {
					id = uint(idFloat)
				} else if idInt, ok := msg.Data.(int); ok {
					id = uint(idInt)
				}
				if id > 0 {
					response = h.handler.HandleRemove(roomID, id)
				}

			default:
				response = &models.WebSocketMessage{Type: msg.Type, Data: msg.Data}
			}

			if response != nil {
				h.broadcastToRoom(roomCode, response)
			}
		}
	}
}

// --- HELPER: BROADCAST TO SPECIFIC ROOM ---
func (h *Hub) broadcastToRoom(roomCode string, v interface{}) {
	msg, _ := json.Marshal(v)
	for client := range h.clients {
		if client.roomID == roomCode {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

// --- HELPER: BROADCAST QUEUE ---
func (h *Hub) broadcastQueue(roomID string, roomCode string) {
	items, err := h.repo.GetQueue(roomID)
	if err != nil {
		return
	}

	queueList := []models.SongJSON{}
	for _, item := range items {
		queueList = append(queueList, models.SongJSON{
			QueueID:   item.ID,
			Title:     item.Song.Title,
			Artist:    item.Song.Artist,
			YoutubeID: item.Song.YoutubeID,
			CoverURL:  item.Song.CoverURL,
			Status:    item.Status,
		})
	}

	h.broadcastToRoom(roomCode, models.WebSocketMessage{
		Type: "QUEUE_UPDATED",
		Data: queueList,
	})
}

// --- REDIS LISTENER (FIXED FOR UUIDs) ---
func (h *Hub) listenToRedis() {
	pubsub := h.rdb.Subscribe(context.Background(), "room_updates")
	defer pubsub.Close()
	ch := pubsub.Channel()

	for msg := range ch {
		var incoming struct {
			RoomCode string `json:"room_id"`
			Payload  struct {
				Type string `json:"type"`
				Data struct {
					Title     string `json:"title"`
					Artist    string `json:"artist"`
					YoutubeID string `json:"youtube_id"`
					CoverURL  string `json:"cover_url"`
				} `json:"data"`
			} `json:"payload"`
		}

		if err := json.Unmarshal([]byte(msg.Payload), &incoming); err != nil {
			continue
		}

		if incoming.Payload.Type == "SEARCH_RESULT" {
			data := incoming.Payload.Data
			room, err := h.repo.GetRoom(incoming.RoomCode)
			if err != nil {
				continue
			}
			roomID := room.ID

			// ----------------------------------------------------
			// FIX: Robust logic to handle UUID creation
			// ----------------------------------------------------
			var song models.Song
			// 1. Check if song exists
			err = h.repo.DB.Where("youtube_id = ?", data.YoutubeID).First(&song).Error

			if err != nil {
				// 2. Not found? Create it!
				song = models.Song{
					YoutubeID: data.YoutubeID,
					Title:     data.Title,
					Artist:    data.Artist,
					CoverURL:  "https://img.youtube.com/vi/" + data.YoutubeID + "/hqdefault.jpg",
				}
				// GORM will create it and fill song.ID with the new UUID
				if createErr := h.repo.DB.Create(&song).Error; createErr != nil {
					log.Printf("❌ Failed to create song: %v", createErr)
					continue
				}
			}

			// ----------------------------------------------------
			// End Fix: song.ID is now guaranteed to be valid
			// ----------------------------------------------------

			current, _ := h.repo.GetCurrent(roomID)

			if current == nil {
				// Create Playing Item
				newItem := models.QueueItem{RoomID: roomID, SongID: song.ID, Status: "playing", Position: 0}
				h.repo.DB.Create(&newItem)

				newItem.Song = song
				h.broadcastToRoom(incoming.RoomCode, models.WebSocketMessage{Type: "PLAY_NOW", Data: newItem.Song})
				h.broadcastQueue(roomID, incoming.RoomCode)
			} else {
				// Create Waiting Item
				var lastPos int
				h.repo.DB.Model(&models.QueueItem{}).Where("room_id = ?", roomID).Select("COALESCE(MAX(position), 0)").Scan(&lastPos)
				
				newItem := models.QueueItem{RoomID: roomID, SongID: song.ID, Status: "waiting", Position: lastPos + 1}
				h.repo.DB.Create(&newItem)

				h.broadcastQueue(roomID, incoming.RoomCode)
			}
		}
	}
}

// --- SYNC STATE ---
func (h *Hub) SendInitialState(client *Client) {
	room, err := h.repo.GetRoom(client.roomID)
	if err != nil {
		return
	}
	roomID := room.ID

	playingItem, _ := h.repo.GetCurrent(roomID)
	queueItems, _ := h.repo.GetQueue(roomID)

	response := models.WebSocketMessage{
		Type: "SYNC_STATE",
		Data: map[string]interface{}{
			"current_song": nil,
			"queue":        []models.SongJSON{},
		},
	}

	dataMap := response.Data.(map[string]interface{})

	if playingItem != nil {
		dataMap["current_song"] = models.SongJSON{
			Title:     playingItem.Song.Title,
			Artist:    playingItem.Song.Artist,
			YoutubeID: playingItem.Song.YoutubeID,
			CoverURL:  playingItem.Song.CoverURL,
		}
	}

	queueList := []models.SongJSON{}
	for _, item := range queueItems {
		queueList = append(queueList, models.SongJSON{
			QueueID:   item.ID,
			Title:     item.Song.Title,
			Artist:    item.Song.Artist,
			YoutubeID: item.Song.YoutubeID,
			CoverURL:  item.Song.CoverURL,
			Status:    item.Status,
		})
	}
	dataMap["queue"] = queueList

	msg, _ := json.Marshal(response)
	client.send <- msg
}

func (h *Hub) pushSearchJob(roomCode string, query string) {
	ctx := context.Background()
	job := map[string]string{"room_id": roomCode, "query": query}
	jsonJob, _ := json.Marshal(job)
	h.rdb.LPush(ctx, "search_queue", jsonJob)
}