package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"univibe/conductor/handlers"   // <--- Added
	"univibe/conductor/models"     // <--- Added
	"univibe/conductor/repository" // <--- Added

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var rdb *redis.Client
var db *gorm.DB // <--- Global Database connection

// UPDATED: Connect to Postgres with Retry Logic
func connectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback for local dev if env is missing
		dsn = "host=univibe-db user=postgres password=postgres dbname=univibe port=5432 sslmode=disable"
	}
	var err error

	// Retry loop (wait for DB to wake up)
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			log.Println("✅ Connected to Postgres!")

			// 1. Migrate Tables (Create Schema)
			// Note: We use models.Room, models.Song, etc.
			err = db.AutoMigrate(&models.Room{}, &models.Song{}, &models.QueueItem{})
			if err != nil {
				log.Printf("⚠️ Migration warning: %v", err)
			}
			
			// Note: "demo_room" seeding is removed. 
			// We now rely on the /create-room endpoint.

			return
		}

		log.Printf("⏳ Database not ready yet... retrying in 2s (%d/10)", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("❌ Failed to connect to Database after 10 attempts: %v", err)
}

func main() {
	// 1. DYNAMIC PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 2. CONNECT TO REDIS
	redisURL := os.Getenv("REDIS_URL")
	var opts *redis.Options
	var err error

	if redisURL != "" {
		// Production (Upstash/Cloud)
		opts, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("❌ Invalid Redis URL: %v", err)
		}
	} else {
		// Local Development
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			addr = "univibe-redis:6379" // Default to docker service name
		}
		opts = &redis.Options{
			Addr: addr,
		}
	}

	rdb = redis.NewClient(opts)
	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("❌ Could not connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis!")

	// 3. CONNECT TO DB (Postgres)
	connectDB()

	// 4. INITIALIZE COMPONENTS
	// Create Repos & Handlers
	roomRepo := repository.NewRoomRepo(db)
	roomHandler := handlers.NewRoomHandler(roomRepo)

	// Create Hub
	hub := newHub(rdb, db)
	go hub.run()
	go hub.listenToRedis()

	// 5. DEFINE ROUTES
	// WebSocket Endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Create Room Endpoint (NEW)
	http.HandleFunc("/create-room", roomHandler.CreateRoom)

	log.Printf("🚀 Conductor is conducting on port %s...", port)

	// Listen on 0.0.0.0 (Required for Docker/Render)
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}