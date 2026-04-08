package bots

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"os"
	"bufio"
	"github.com/toydogcat/kitty-help/go-server/database"
)

type BaseChannel struct {
	Platform string
	mu       sync.RWMutex
	running  bool
	Admins   []string
}

func NewBaseChannel(platform string, admins []string) *BaseChannel {
	return &BaseChannel{
		Platform: platform,
		Admins:   admins,
	}
}

func (c *BaseChannel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *BaseChannel) SetRunning(s bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = s
}

func (c *BaseChannel) IsAdmin(userID string) bool {
	if database.LocalDB == nil {
		return false
	}
	// 1. Check in global users table (for web/email matched IDs)
	var role string
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT role FROM users WHERE id = $1 OR email = $1", userID).Scan(&role)
	if err == nil && (role == "superadmin" || role == "toby") {
		return true
	}

	// 2. Check in bot authorized users table (for platform-specific bot IDs)
	err = database.LocalDB.QueryRow(context.Background(), 
		"SELECT role FROM bot_authorized_users WHERE platform = $1 AND account_id = $2", 
		c.Platform, userID).Scan(&role)
	return err == nil && role == "superadmin"
}

func (c *BaseChannel) IsAuthorized(userID string) bool {
	if database.LocalDB == nil {
		return false
	}
	// Check if this user is in the authorized list for this platform
	var id string
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT id FROM bot_authorized_users WHERE platform = $1 AND account_id = $2", 
		c.Platform, userID).Scan(&id)
	return err == nil
}

func (c *BaseChannel) GenerateJoinToken(userID string, userName string) (string, error) {
	// Generate 8-digit random number
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Simple number generation
	num := (uint32(b[0]) << 24) | (uint32(b[1]) << 16) | (uint32(b[2]) << 8) | uint32(b[3])
	token := fmt.Sprintf("%08d", num%100000000)

	expiresAt := time.Now().Add(30 * time.Minute)
	_, err := database.LocalDB.Exec(context.Background(), 
		"INSERT INTO bot_auth_requests (token, platform, account_id, account_name, expires_at) VALUES ($1, $2, $3, $4, $5)", 
		token, c.Platform, userID, userName, expiresAt)
	return token, err
}

func (c *BaseChannel) NormalizeInput(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "！", "!") // Handle Chinese !
	text = strings.ReplaceAll(text, "／", "/") // Handle Chinese /
	return text
}

func (c *BaseChannel) ParseTriggers(text string) (isGeneral bool, isAdmin bool, content string) {
	normalized := c.NormalizeInput(text)
	
	if strings.HasPrefix(normalized, "!") || strings.HasPrefix(normalized, "！") {
		isAdmin = true
		content = strings.TrimSpace(strings.TrimPrefix(normalized, "!"))
		content = strings.TrimSpace(strings.TrimPrefix(content, "！"))
		return
	}
	
	if strings.HasPrefix(normalized, "/cat") || strings.HasPrefix(normalized, "／cat") {
		isGeneral = true
		content = strings.TrimSpace(strings.TrimPrefix(normalized, "/cat"))
		content = strings.TrimSpace(strings.TrimPrefix(content, "／cat"))
		return
	}
	
	return false, false, normalized
}

func (c *BaseChannel) GetUnifiedUserID(ctx context.Context, accountID string) (string, error) {
	if database.LocalDB == nil {
		return "", fmt.Errorf("local db disconnected")
	}
	var userID string
	err := database.LocalDB.QueryRow(ctx, "SELECT user_id FROM bot_authorized_users WHERE platform = $1 AND account_id = $2", c.Platform, accountID).Scan(&userID)
	return userID, err
}

func (c *BaseChannel) GetWebhookURL() string {
	// Try to read from .env first to get the freshest URL from catch_url.py
	file, err := os.Open("../.env")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VITE_API_URL=") {
				return strings.TrimPrefix(line, "VITE_API_URL=")
			}
		}
	}

	// Fallback to environment
	url := os.Getenv("VITE_API_URL")
	if url == "" {
		url = "https://your-tunnel.trycloudflare.com" 
	}
	return url
}

func (c *BaseChannel) GetNewsFromWorker(args string) (string, error) {
	workerURL := "http://100.103.50.4:7080/api/opencli"
	
	reqBody, _ := json.Marshal(map[string]string{"args": args})
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(workerURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("connect worker failed: %v", err)
	}
	defer resp.Body.Close()

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %v", err)
	}

	// Try JSON first
	var result struct {
		Output string `json:"output"`
		Error  string `json:"error"`
		Reply  string `json:"reply"` // Some versions use 'reply'
	}
	if err := json.Unmarshal(resBody, &result); err == nil {
		if result.Error != "" {
			return "", fmt.Errorf("%s", result.Error)
		}
		if result.Output != "" { return result.Output, nil }
		if result.Reply != "" { return result.Reply, nil }
	}

	// Fallback to raw string
	return string(resBody), nil
}

func (c *BaseChannel) LogChat(ctx context.Context, senderID string, senderName string, content string, msgType string, mediaID *string) {
	if database.LocalDB == nil {
		return
	}
	_, _ = database.LocalDB.Exec(ctx, 
		"INSERT INTO chat_logs (platform, sender_id, sender_name, content, msg_type, media_id) VALUES ($1, $2, $3, $4, $5, $6)", 
		c.Platform, senderID, senderName, content, msgType, mediaID)
}
