FROM golang:1.24-alpine

WORKDIR /app

# 1. Install dependencies
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download

# 2. COPY EVERYTHING (Crucial: This copies handlers, models, repo folders)
COPY . . 

# 3. Build
RUN go build -o main .

CMD ["./main"]