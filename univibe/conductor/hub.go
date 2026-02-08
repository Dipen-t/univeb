package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	rdb        *redis.Client // Redis connection
}

func newHub(rdb *redis.Client) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rdb:        rdb,
	}
}

// Listen for messages FROM Python (Redis Pub/Sub)
func (h *Hub) listenToRedis() {
	ctx := context.Background()
	// Subscribe to the channel Python publishes to
	pubsub := h.rdb.Subscribe(ctx, "room_updates")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for msg := range ch {
		// msg.Payload is the JSON string {"room_id": "...", "payload": {...}}
		var update struct {
			RoomID  string          `json:"room_id"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
			log.Printf("Error unmarshalling Redis msg: %v", err)
			continue
		}

		// Broadcast ONLY to the specific room (For now, we broadcast to everyone)
		// Optimization: Later we will filter h.clients by RoomID
		h.broadcast <- []byte(update.Payload)
	}
}

// Push a job TO Python (Redis List)
func (h *Hub) pushSearchJob(roomID string, query string) {
	ctx := context.Background()
	
	job := map[string]string{
		"room_id": roomID,
		"query":   query,
	}
	
	jsonJob, _ := json.Marshal(job)
	
	// Push to the "search_queue" list
	err := h.rdb.LPush(ctx, "search_queue", jsonJob).Err()
	if err != nil {
		log.Printf("Error pushing to Redis: %v", err)
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}