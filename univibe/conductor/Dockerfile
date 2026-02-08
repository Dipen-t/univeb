# Use the official Golang image
FROM golang:1.22-alpine

# Set working directory
WORKDIR /app

# --- FIX START ---
# Install git AND security certificates (Crucial for Upstash/SSL)
RUN apk add --no-cache git ca-certificates
# --- FIX END ---

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
RUN go build -o main .

# Run the executable
CMD ["./main"]