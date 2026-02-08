package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	hub *Hub
	conn *websocket.Conn
	send chan []byte
	roomID string
}

// Standard Message Format
type IncomingMessage struct {
	Type  string `json:"type"`  // "SEARCH" or "CHAT"
	Data  string `json:"data"`  // "Starboy" or "Hello"
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// Parse the JSON message
		var msg IncomingMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		// DECISION LOGIC
		if msg.Type == "SEARCH" {
			log.Printf("🔍 Search Request: %s", msg.Data)
			// Send to Redis/Python
			c.hub.pushSearchJob(c.roomID, msg.Data)
		} else {
			// Normal Chat/Echo
			c.hub.broadcast <- message
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil { return }
			w.Write(message)
			if err := w.Close(); err != nil { return }
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil { return }
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	keys := r.URL.Query()
	roomID := keys.Get("room")
	if roomID == "" { roomID = "default" }
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), roomID: roomID}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
}