import redis
import json
import os
import sys
import yt_dlp
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading

# Flush output immediately so logs appear in Render dashboard
sys.stdout.reconfigure(line_buffering=True)

# --- 1. CONNECT TO REDIS (Production Ready) ---
# This handles both Local ('redis://localhost') and Cloud ('rediss://user:pass@host...')
redis_url = os.getenv('REDIS_URL', 'redis://localhost:6379')

try:
    # from_url automatically handles passwords, ports, and SSL
    r = redis.from_url(redis_url)
    # Test connection immediately
    r.ping()
    print(f"👷 Librarian connected to Redis at {redis_url.split('@')[-1]}...") # Hides password in logs
except Exception as e:
    print(f"❌ CRITICAL: Could not connect to Redis. Error: {e}")
    sys.exit(1)

# Safer Options for yt-dlp
ydl_opts = {
    'format': 'bestaudio/best',
    'quiet': True,
    'noplaylist': True,
    'extract_flat': True, # FAST mode (doesn't download video)
}

def search_youtube(query):
    print(f"🔍 Searching YouTube for: {query}")
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            # STRATEGY: Search for "lyrics audio" to avoid official music video blocks
            search_query = f"ytsearch1:{query} lyrics audio"
            
            info = ydl.extract_info(search_query, download=False)
            
            if 'entries' in info and len(info['entries']) > 0:
                video = info['entries'][0]
                return {
                    "title": video.get('title', 'Unknown Title'),
                    "artist": video.get('uploader', 'Unknown Artist'),
                    "youtube_id": video['id'],
                    "spotify_id": "none"
                }
                
    except Exception as e:
        print(f"❌ Search Error: {e}")
    
    return None


class HealthCheckHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"I am alive")

def start_dummy_server():
    port = int(os.getenv("PORT", 8000)) # Render gives us a PORT
    server = HTTPServer(('0.0.0.0', port), HealthCheckHandler)
    print(f"👻 Dummy Server listening on port {port}...")
    server.serve_forever()

# Start the dummy server in the background
threading.Thread(target=start_dummy_server, daemon=True).start()

# --- END HACK ---

while True:
    try:
        # Wait for job (Blocking Pop)
        # using keys/timeout is safer for some redis versions, but blpop is standard
        result = r.blpop("search_queue", timeout=0)
        
        if result:
            queue, data = result
            job = json.loads(data)
            query = job.get('query')
            room_id = job.get('room_id')
            
            search_result = search_youtube(query)
            
            if search_result:
                print(f"✅ Found: {search_result['title']} ({search_result['youtube_id']})")
                
                message_to_go = {
                    "room_id": room_id,
                    "payload": {
                        "type": "SEARCH_RESULT",
                        "data": search_result
                    }
                }
                # Publish back to Go
                r.publish("room_updates", json.dumps(message_to_go))
            else:
                print(f"⚠️ No valid results found for {query}")
            
    except Exception as e:
        print(f"❌ Worker Error: {e}")