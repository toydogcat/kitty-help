package services

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"github.com/toydogcat/kitty-help/go-server/database"
)

type ResourceService struct{}

var MediaManager = &ResourceService{}

// FetchAndCache handles the complex logic of fetching data from multiple platforms with fallbacks and caching.
func (s *ResourceService) FetchAndCache(ctx context.Context, fileID string, initialPlatform string, width int) ([]byte, string, error) {
	platform := initialPlatform
	db := database.LocalDB
	if db == nil { db = database.CloudDB }

	// 1. Resolve UUID to actual file path/id if needed
	if len(fileID) == 36 {
		var resolvedID, resolvedPlatform string
		var err error
		if database.LocalDB != nil {
			err = database.LocalDB.QueryRow(ctx, "SELECT file_id, source_platform FROM media_archives WHERE id = $1", fileID).Scan(&resolvedID, &resolvedPlatform)
		}
		if (err != nil || database.LocalDB == nil) && database.CloudDB != nil {
			err = database.CloudDB.QueryRow(ctx, "SELECT file_id, source_platform FROM media_archives WHERE id = $1", fileID).Scan(&resolvedID, &resolvedPlatform)
		}
		
		if err == nil {
			fileID = resolvedID
			platform = resolvedPlatform
		}
	}

	// 2. Smart Redirection (e.g., Telegram Cloud Backup for expired LINE/Discord content)
	// If it's a non-URL ID from LINE/Discord, it's likely archived in Telegram Cloud
	if !strings.HasPrefix(fileID, "http") && platform != "telegram" && (len(fileID) > 20 || strings.Contains(fileID, "AgAC")) {
		log.Printf("☁️ [Service] Redirecting %s request to Telegram Cloud Backup for ID: %s", platform, fileID)
		platform = "telegram"
	}

	// 3. Check Cache for thumbnails
	if width > 0 {
		cacheKey := fmt.Sprintf("t_%s_%d.jpg", fileID, width)
		cacheKey = strings.ReplaceAll(cacheKey, "/", "_") // Sanitize path
		cachePath := filepath.Join(os.TempDir(), cacheKey)
		if data, err := os.ReadFile(cachePath); err == nil {
			return data, "image/jpeg", nil
		}
	}

	// 4. Fetch based on platform
	var bodyBytes []byte
	var contentType string
	var err error

	switch platform {
	case "line":
		bodyBytes, contentType, err = s.fetchFromLine(fileID)
	case "discord":
		bodyBytes, contentType, err = s.fetchFromDiscord(fileID)
	case "telegram":
		bodyBytes, contentType, err = s.fetchFromTelegram(ctx, fileID)
	default:
		return nil, "", fmt.Errorf("unknown platform: %s", platform)
	}

	if err != nil { return nil, "", err }

	// 5. Apply Resizing & Caching
	if width > 0 && strings.HasPrefix(contentType, "image/") {
		bodyBytes, err = s.resizeImage(bodyBytes, width, fileID)
		if err == nil {
			contentType = "image/jpeg"
		}
	}

	return bodyBytes, contentType, nil
}

func (s *ResourceService) fetchFromLine(fileID string) ([]byte, string, error) {
	actualInterface, _ := bots.BotManager.Get("line")
	switch b := actualInterface.(type) {
	case *bots.LineBot:
		if b.Bot == nil { return nil, "", fmt.Errorf("LINE SDK not initialized") }
		content, err := b.Bot.GetMessageContent(fileID).Do()
		if err != nil { return nil, "", fmt.Errorf("content expired") }
		defer content.Content.Close()
		data, _ := io.ReadAll(content.Content)
		return data, content.ContentType, nil
	}
	return nil, "", fmt.Errorf("failed to cast or initialize line bot")
}

func (s *ResourceService) fetchFromDiscord(url string) ([]byte, string, error) {
	if !strings.HasPrefix(url, "http") { return nil, "", fmt.Errorf("invalid discord url") }
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, "", err }
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.Header.Get("Content-Type"), nil
}

func (s *ResourceService) fetchFromTelegram(ctx context.Context, fileID string) ([]byte, string, error) {
	tgBotIf, ok := bots.BotManager.Get("telegram")
	if !ok { return nil, "", fmt.Errorf("TG bot not initialized") }
	tgBot := tgBotIf.(*bots.TelegramBot)
	data, cType, err := tgBot.GetFile(ctx, fileID)
	return data, cType, err
}

func (s *ResourceService) resizeImage(data []byte, width int, fileID string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return nil, err }
	
	newImg := resize.Resize(uint(width), 0, img, resize.Lanczos3)
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, newImg, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	
	resizedData := buf.Bytes()
	// Cache it
	cacheKey := fmt.Sprintf("t_%s_%d.jpg", fileID, width)
	cacheKey = strings.ReplaceAll(cacheKey, "/", "_")
	cachePath := filepath.Join(os.TempDir(), cacheKey)
	_ = os.WriteFile(cachePath, resizedData, 0644)
	
	return resizedData, nil
}
