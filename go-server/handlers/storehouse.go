package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/services"
)

type UpdateStorehouseItemRequest struct {
	Title string `json:"title"`
	Notes string `json:"notes"`
}

func GetStorehouseItems(c *fiber.Ctx) error {
	db := database.LocalDB
	if db == nil {
		db = database.CloudDB
	}

	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database not connected"})
	}

	platform := c.Query("platform")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	query := c.Query("q")
	mode := c.Query("mode", "keyword") // keyword or semantic
	limit := c.QueryInt("limit", 20)

	var sql string
	args := []interface{}{}
	argIdx := 1

	if mode == "semantic" && query != "" {
		// 1. Get embedding for query
		apiKey := os.Getenv("GOOGLE_API_KEY")
		embedding, err := services.GenerateMultimodalEmbedding(c.Context(), apiKey, "", "text", query)
		if err != nil {
			log.Printf("⚠️ Semantic search error: %v", err)
			// Fallback to keyword if semantic fails
			mode = "keyword"
		} else {
			// 2. Perform vector similarity search
			sql = "SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status, index_status FROM media_archives WHERE embedding IS NOT NULL"
			if platform != "" {
				sql += fmt.Sprintf(" AND source_platform = $%d", argIdx)
				args = append(args, platform)
				argIdx++
			}
			// Order by Cosine Distance (<=>) or Inner Product (<#>) or L2 (<->)
			// pgvector: 1 - (v1 <=> v2) is Cosine Similarity
			sql += fmt.Sprintf(" ORDER BY embedding <=> $%d", argIdx)
			args = append(args, services.Float32SliceToVector(embedding))
			argIdx++
			sql += fmt.Sprintf(" LIMIT %d", limit)
		}
	}

	if mode == "keyword" || sql == "" {
		sql = "SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status FROM media_archives WHERE 1=1"
		if platform != "" {
			sql += fmt.Sprintf(" AND source_platform = $%d", argIdx)
			args = append(args, platform)
			argIdx++
		}
		if startDate != "" {
			sql += fmt.Sprintf(" AND created_at >= $%d", argIdx)
			args = append(args, startDate)
			argIdx++
		}
		if endDate != "" {
			sql += fmt.Sprintf(" AND created_at <= $%d", argIdx)
			args = append(args, endDate)
			argIdx++
		}
		if query != "" {
			sql += fmt.Sprintf(" AND (title ILIKE $%d OR notes ILIKE $%d OR caption ILIKE $%d)", argIdx, argIdx, argIdx)
			args = append(args, "%"+query+"%")
			argIdx++
		}
		sql += " ORDER BY created_at DESC"
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(context.Background(), sql, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, fileID, mediaType, sourcePlatform, indexStatus string
		var title, caption, notes, senderName *string
		var createdAt time.Time
		var isIndexable bool
		
		// Handle different column counts between modes if necessary, but here we kept them mostly same
		err := rows.Scan(&id, &fileID, &mediaType, &title, &caption, &notes, &sourcePlatform, &senderName, &createdAt, &isIndexable, &indexStatus)
		if err != nil {
			log.Printf("DEBUG: Row scan failed: %v", err)
			continue
		}
		
		displayTitle := ""
		if title != nil && *title != "" {
			displayTitle = *title
		} else if caption != nil && *caption != "" {
			displayTitle = *caption
		} else {
			displayTitle = "Untitled Resource"
		}

		items = append(items, fiber.Map{
			"id": id,
			"file_id": fileID,
			"category": mediaType,
			"title": displayTitle,
			"caption": caption,
			"notes": notes,
			"source": sourcePlatform,
			"sender": senderName,
			"created_at": createdAt,
			"is_indexable": isIndexable,
			"index_status": indexStatus,
		})
	}

	return c.JSON(items)
}

func UpdateStorehouseItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var req UpdateStorehouseItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	db := database.LocalDB
	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database not connected"})
	}

	_, err := db.Exec(context.Background(), 
		"UPDATE media_archives SET title = $1, notes = $2 WHERE id = $3", 
		req.Title, req.Notes, id)
	
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func IndexStorehouseItem(c *fiber.Ctx) error {
	id := c.Params("id")
	db := database.LocalDB
	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database not connected"})
	}

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Google API key missing"})
	}

	// 1. Fetch item details
	var fileID, category, source string
	var title, notes, caption *string
	err := db.QueryRow(context.Background(), 
		"SELECT file_id, media_type, source_platform, title, notes, caption FROM media_archives WHERE id = $1", 
		id).Scan(&fileID, &category, &source, &title, &notes, &caption)
	
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
	}

	// 2. Download file to temp
	var localPath string
	if source == "telegram" {
		tgBotIf, _ := bots.BotManager.Get("telegram")
		tgBot := tgBotIf.(*bots.TelegramBot)
		file, err := tgBot.GetFile(c.Context(), fileID)
		if err == nil {
			token := os.Getenv("TELEGRAM_BOT_TOKEN")
			url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, file.FilePath)
			
			resp, err := http.Get(url)
			if err == nil {
				defer resp.Body.Close()
				localPath = filepath.Join(os.TempDir(), fileID)
				out, _ := os.Create(localPath)
				defer out.Close()
				io.Copy(out, resp.Body)
			}
		}
	}

	// 3. Generate Embedding
	fullText := ""
	if title != nil { fullText += *title + " " }
	if notes != nil { fullText += *notes + " " }
	if caption != nil { fullText += *caption }

	db.Exec(context.Background(), "UPDATE media_archives SET index_status = 'indexing' WHERE id = $1", id)

	embedding, err := services.GenerateMultimodalEmbedding(c.Context(), apiKey, localPath, category, fullText)
	if localPath != "" { os.Remove(localPath) }

	if err != nil {
		db.Exec(context.Background(), "UPDATE media_archives SET index_status = 'failed' WHERE id = $1", id)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// 4. Update Database
	_, err = db.Exec(context.Background(), 
		"UPDATE media_archives SET embedding = $1, index_status = 'indexed', embedding_model = $2 WHERE id = $3", 
		services.Float32SliceToVector(embedding), services.GeminiEmbeddingModel, id)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "indexed"})
}

func GetFileProxy(c *fiber.Ctx) error {
	fileID := c.Params("fileID")
	platform := c.Query("platform", "telegram")
	if c.Query("download") == "1" {
		c.Set("Content-Disposition", "attachment")
	}

	db := database.LocalDB
	if db == nil {
		db = database.CloudDB
	}

	// If the fileID looks like a UUID, resolve it to the actual platform file_id
	// If the fileID looks like a UUID (36 chars), resolve it from the database first
	if len(fileID) == 36 {
		var resolvedFileID, resolvedPlatform string
		var err error
		
		// Attempt A: Local DB
		if database.LocalDB != nil {
			err = database.LocalDB.QueryRow(context.Background(),
				"SELECT file_id, source_platform FROM media_archives WHERE id = $1",
				fileID).Scan(&resolvedFileID, &resolvedPlatform)
		}
		
		// Attempt B: Cloud DB fallback
		if (err != nil || database.LocalDB == nil) && database.CloudDB != nil {
			err = database.CloudDB.QueryRow(context.Background(),
				"SELECT file_id, source_platform FROM media_archives WHERE id = $1",
				fileID).Scan(&resolvedFileID, &resolvedPlatform)
		}
		
		if err == nil {
			log.Printf("🔍 RESOLVED UUID %s -> %s (%s)", fileID, resolvedFileID, resolvedPlatform)
			fileID = resolvedFileID
			platform = resolvedPlatform
		} else {
			log.Printf("⚠️ FAILED to resolve UUID %s in any DB: %v", fileID, err)
			return c.Status(404).JSON(fiber.Map{"error": "Media record not found in local or cloud DB"})
		}
	}

	// SMART DETECT: If the fileID looks like a Telegram ID (long base64 string),
	// we should treat this as a cloud backup and fetch from Telegram instead, regardless of original platform.
	if len(fileID) > 40 && platform != "telegram" {
		log.Printf("☁️ Redirecting %s request to Telegram Cloud Backup for ID: %s", platform, fileID)
		platform = "telegram"
	}

	if platform == "line" {
		lnBotIf, ok := bots.BotManager.Get("line")
			if !ok { return c.Status(503).JSON(fiber.Map{"error": "LINE bot not initialized"}) }
			
			lnBot := lnBotIf.(*bots.LineBot)
			content, err := lnBot.Bot.GetMessageContent(fileID).Do()
			if err != nil {
				log.Printf("❌ LINE content fetch failed for %s (Likely Expired): %v", fileID, err)
				return c.Status(404).JSON(fiber.Map{"error": "Content expired or not found on LINE"})
			}
			defer content.Content.Close()
			
			bodyBytes, err := io.ReadAll(content.Content)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to read content stream"})
			}
			c.Set("Content-Type", content.ContentType)
			c.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
			return c.Send(bodyBytes)
	}

	if platform == "discord" {
		// In Discord, fileID IS the URL
		if !strings.HasPrefix(fileID, "http") {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid Discord file URL"})
		}
		
		resp, err := http.Get(fileID)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "Discord upstream error"})
		}
		defer resp.Body.Close()
		
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", resp.Header.Get("Content-Type"))
		c.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
		return c.Send(bodyBytes)
	}

	if platform == "telegram" {
		tgBotIf, ok := bots.BotManager.Get("telegram")
		if !ok {
			return c.Status(503).JSON(fiber.Map{"error": "Telegram bot not initialized"})
		}
		
		tgBot := tgBotIf.(*bots.TelegramBot)
		file, err := tgBot.GetFile(c.Context(), fileID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "File not found"})
		}

		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, file.FilePath)
		
		resp, err := http.Get(downloadURL)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "Telegram upstream error"})
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", resp.Header.Get("Content-Type"))
		c.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
		return c.Send(bodyBytes)
	}

	return c.Status(400).JSON(fiber.Map{"error": "Unsupported platform"})
}
