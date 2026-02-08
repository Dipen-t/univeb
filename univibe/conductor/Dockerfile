# Use the official Golang image
FROM golang:1.22-alpine

# Set working directory
WORKDIR /app

# Install git (needed for go mod)
RUN apk add --no-cache git

# Copy go.mod
COPY go.mod ./

# Copy the source code
COPY . .

# FORCE generate the go.sum file
RUN go mod tidy

# Build the Go app
RUN go build -o main .

# Run the executable
CMD ["./main"]