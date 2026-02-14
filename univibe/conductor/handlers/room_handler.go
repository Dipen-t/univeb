package handlers

import (
	"encoding/json"
	"net/http"
	"univibe/conductor/repository"
)

type RoomHandler struct {
	Repo *repository.RoomRepo
}

func NewRoomHandler(repo *repository.RoomRepo) *RoomHandler {
	return &RoomHandler{Repo: repo}
}

// POST /create-room
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
    // CORS Setup (Allow frontend to call this)
    w.Header().Set("Access-Control-Allow-Origin", "*")
    if r.Method == "OPTIONS" {
        return
    }

	room, err := h.Repo.CreateRoom()
	if err != nil {
		http.Error(w, "Failed to create room", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"code": room.Code,
	})
}