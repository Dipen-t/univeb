package handlers

import (
	
	"log"
	"univibe/conductor/models"
	"univibe/conductor/repository"
)

type EventHandler struct {
	Repo *repository.QueueRepo
}

func NewEventHandler(repo *repository.QueueRepo) *EventHandler {
	return &EventHandler{Repo: repo}
}

// --- HANDLE PAUSE ---
func (h *EventHandler) HandlePause(roomID string) *models.WebSocketMessage {
	log.Println("⏸️ Pausing Room")
	h.Repo.SetRoomState(roomID, false)
	return &models.WebSocketMessage{Type: "PAUSE", Data: nil}
}

// --- HANDLE PLAY ---
func (h *EventHandler) HandlePlay(roomID string) *models.WebSocketMessage {
	log.Println("▶️ Resuming Room")
	h.Repo.SetRoomState(roomID, true)
	// We might want to send the current song time here later, for now just RESUME
	return &models.WebSocketMessage{Type: "PLAY", Data: nil}
}

// --- HANDLE NEXT ---
func (h *EventHandler) HandleNext(roomID string) *models.WebSocketMessage {
	log.Println("⏭️ Skipping Forward")

	// 1. Mark current as played
	current, err := h.Repo.GetCurrent(roomID)
	if err == nil {
		h.Repo.MarkAsPlayed(current.ID)
	}

	// 2. Get Next
	next, err := h.Repo.GetNext(roomID)
	if err != nil {
		log.Println("📭 Queue Empty")
		return &models.WebSocketMessage{Type: "QUEUE_EMPTY", Data: nil}
	}

	// 3. Play Next
	h.Repo.MarkAsPlaying(next.ID)
	h.Repo.SetRoomState(roomID, true) // Ensure room is playing

	return &models.WebSocketMessage{Type: "PLAY_NOW", Data: next.Song}
}

// --- HANDLE PREVIOUS (THE FIX) ---
func (h *EventHandler) HandlePrevious(roomID string) *models.WebSocketMessage {
	log.Println("⏮️ Rewinding")

	// 1. Get Current
	current, _ := h.Repo.GetCurrent(roomID)
	
	// 2. Get Previous (History)
	prev, err := h.Repo.GetPrevious(roomID)
	if err != nil {
		log.Println("🚫 No History Found")
		return nil // Do nothing if no history
	}

	// 3. Swap Logic
	// If something is playing, push it back to the TOP of the queue (Position 1)
	if current != nil {
		h.Repo.MarkAsWaiting(current.ID, 1) 
		// You might want to shift other songs down, but for now this puts it at top
	}

	// 4. Play Previous
	h.Repo.MarkAsPlaying(prev.ID)
	h.Repo.SetRoomState(roomID, true)

	return &models.WebSocketMessage{Type: "PLAY_NOW", Data: prev.Song}
	
}

func (h *EventHandler) HandleRemove(roomID string, itemID uint) *models.WebSocketMessage {
	log.Printf("🗑️ Removing Queue Item ID: %d", itemID)

	// 1. Remove it from DB
	if err := h.Repo.RemoveItem(itemID); err != nil {
		log.Printf("❌ Failed to remove item: %v", err)
		return nil
	}

	// 2. Get the Updated Queue to show everyone the new list
	items, err := h.Repo.GetQueue(roomID)
	if err != nil {
		log.Printf("❌ Failed to fetch queue: %v", err)
		return nil
	}

	// 3. Format for Frontend
	queueList := []models.SongJSON{}
	for _, item := range items {
		queueList = append(queueList, models.SongJSON{
			QueueID:   item.ID,
			Title:     item.Song.Title,
			Artist:    item.Song.Artist,
			YoutubeID: item.Song.YoutubeID,
			CoverURL:  item.Song.CoverURL,
		})
	}

	// 4. Broadcast the NEW Queue list
	return &models.WebSocketMessage{Type: "QUEUE_UPDATED", Data: queueList}
}
// --- HANDLE JUMP TO SONG ---
func (h *EventHandler) HandleJump(roomID string, itemID uint) *models.WebSocketMessage {
	log.Printf("🦘 Jumping to Item ID: %d", itemID)

	// 1. Perform Jump in DB
	err := h.Repo.JumpToSong(roomID, itemID)
	if err != nil {
		log.Printf("❌ Jump failed: %v", err)
		return nil
	}

	// 2. Get the new "Playing" song details
	// We can cheat and just query the specific item
	var item models.QueueItem
	h.Repo.DB.Preload("Song").First(&item, itemID)

	// 3. Return PLAY_NOW so the player updates
	return &models.WebSocketMessage{Type: "PLAY_NOW", Data: item.Song}
}