package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func main() {
	// 1. DYNAMIC PORT (Required for Render)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default for local development
	}

	// 2. CONNECT TO REDIS (Handles both Local and Upstash/Cloud)
	redisURL := os.Getenv("REDIS_URL")
	
	var opts *redis.Options
	var err error

	if redisURL != "" {
		// CASE A: Production (Upstash sends a full URL with password)
		// e.g., "rediss://default:password@fly-redis.upstash.io:6379"
		opts, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("❌ Invalid Redis URL: %v", err)
		}
	} else {
		// CASE B: Local Development (Simple address)
		// Fallback to Docker service name or localhost
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "localhost:6379"
		}
		opts = &redis.Options{
			Addr: addr,
		}
	}

	rdb = redis.NewClient(opts)

	// Test connection
	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("❌ Could not connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis!")

	// 3. Initialize the Hub
	hub := newHub(rdb)
	go hub.run()

	// 4. Start the Redis Listener (To hear back from Python)
	go hub.listenToRedis()

	// 5. Start Web Server
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Printf("Conductor is conducting on port %s...", port)
	
	// Listen on 0.0.0.0 (Required for Docker/Render)
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}