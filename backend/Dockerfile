# Step 1: Build the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git for dependencies if needed
RUN apk add --no-cache git

# Copy files
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o kitty-server main.go

# Step 2: Final Image
FROM alpine:latest

WORKDIR /root/

# Install runtime dependencies
RUN apk add --no-cache ffmpeg

# Copy the binary
COPY --from=builder /app/kitty-server .

# Copy environment files (if they exist)
COPY --from=builder /app/.env* ./

EXPOSE 3000

CMD ["./kitty-server"]
