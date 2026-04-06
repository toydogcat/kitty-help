package handlers

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nfnt/resize"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/services"
)

type UpdateStorehouseItemRequest struct {
	Title string `json:"title"`
	Notes string `json:"notes"`
}

func GetStorehouseItems(c *fiber.Ctx) error {
	userClaims, _ := c.Locals("user").(*Claims)
	db := database.LocalDB
	
	// Admin always uses LocalDB
	if userClaims != nil && (userClaims.Role != "superadmin" && userClaims.Role != "toby") && db == nil {
		db = database.CloudDB
	}

	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Local Database not connected. Please check NUC DB status."})
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

	dbUserID := ""
	isAdmin := false
	if userClaims != nil {
		dbUserID = userClaims.ID
		if userClaims.Role == "superadmin" || userClaims.Role == "toby" {
			isAdmin = true
		}
	}

	if mode == "semantic" && query != "" {
		// 1. Get embedding for query
		apiKey := os.Getenv("GOOGLE_API_KEY")
		embedding, err := services.GenerateMultimodalEmbedding(c.Context(), apiKey, "", "text", query)
		if err != nil {
			log.Printf("⚠️ Semantic search error: %v", err)
			// Fallback to keyword if semantic fails
			mode = "keyword"
		} else {
			// 2. Perform user-filtered vector similarity search
			if dbUserID != "" && !isAdmin {
				sql = `
					SELECT m.id, m.file_id, m.media_type, m.title, m.caption, m.notes, m.source_platform, m.sender_name, m.created_at, m.is_indexable, m.index_status 
					FROM media_archives m
					JOIN bot_authorized_users b ON m.sender_id = b.account_id AND m.source_platform = b.platform
					WHERE b.user_id = $1 AND m.embedding IS NOT NULL`
				args = append(args, dbUserID)
				argIdx = 2
			} else {
				// Admin or unauthenticated sees global (semantic)
				sql = `SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status 
				       FROM media_archives WHERE embedding IS NOT NULL`
				argIdx = 1
			}
			
			if platform != "" {
				sql += fmt.Sprintf(" AND m.source_platform = $%d", argIdx)
				args = append(args, platform)
				argIdx++
			}
			sql += fmt.Sprintf(" ORDER BY m.embedding <=> $%d", argIdx)
			args = append(args, services.Float32SliceToVector(embedding))
			argIdx++
			sql += fmt.Sprintf(" LIMIT %d", limit)
		}
	}

	if mode == "keyword" || sql == "" {
		// 1. Base Query with User Filtering via JOIN
		if dbUserID != "" && !isAdmin {
			sql = `
				SELECT m.id, m.file_id, m.media_type, m.title, m.caption, m.notes, m.source_platform, m.sender_name, m.created_at, m.is_indexable, m.index_status 
				FROM media_archives m
				JOIN bot_authorized_users b ON m.sender_id = b.account_id AND m.source_platform = b.platform
				WHERE b.user_id = $1`
			args = append(args, dbUserID)
			argIdx = 2
		} else {
			// Admin sees all
			sql = `SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status 
			       FROM media_archives m WHERE 1=1`
			argIdx = 1
		}

		if platform != "" {
			sql += fmt.Sprintf(" AND m.source_platform = $%d", argIdx)
			args = append(args, platform)
			argIdx++
		}
		if startDate != "" {
			sql += fmt.Sprintf(" AND m.created_at >= $%d", argIdx)
			args = append(args, startDate)
			argIdx++
		}
		if endDate != "" {
			sql += fmt.Sprintf(" AND m.created_at <= $%d", argIdx)
			args = append(args, endDate)
			argIdx++
		}
		if query != "" {
			sql += fmt.Sprintf(" AND (m.title ILIKE $%d OR m.notes ILIKE $%d OR m.caption ILIKE $%d)", argIdx, argIdx, argIdx)
			args = append(args, "%"+query+"%")
			argIdx++
		}
		sql += " ORDER BY m.created_at DESC"
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

	// 2. Download file to memory/temp via Bot Manager
	var localPath string
	var data []byte
	if source == "telegram" {
		tgBotIf, _ := bots.BotManager.Get("telegram")
		tgBot := tgBotIf.(*bots.TelegramBot)
		var err error
		data, _, err = tgBot.GetFile(c.UserContext(), fileID)
		if err == nil {
			localPath = filepath.Join(os.TempDir(), fileID)
			os.WriteFile(localPath, data, 0644)
		} else {
			log.Printf("❌ Failed to download file for indexing: %v", err)
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
	log.Printf("🖼️ [ProxyRequest] Entering GetFileProxy for ID: %s", fileID)
	
	platform := c.Query("platform", "telegram")
	c.Set("Access-Control-Allow-Origin", "*")
	
	// ⚡ [Performance Booster] Robust Caching Headers
	// Allow browser to cache for 7 days. Even if tunnel URL changes, the ETag (fileID) will stay same.
	c.Set("Cache-Control", "public, max-age=604800, immutable")
	c.Set("ETag", fileID)
	
	// Check if client has a matching ETag (304 Not Modified)
	if c.Get("If-None-Match") == fileID {
		return c.SendStatus(304)
	}

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
	// or it contains 'AgAC' (common Telegram image prefix),
	// we should treat this as a cloud backup and fetch from Telegram instead.
	if (len(fileID) > 40 || strings.Contains(fileID, "AgAC")) && platform != "telegram" {
		log.Printf("☁️ [Proxy] Redirecting %s request to Telegram Cloud Backup for ID: %s", platform, fileID)
		platform = "telegram"
	}

	// 🛠️ Thumbnail Logic: Check Cache first
	width := c.QueryInt("w", 0)
	if width > 0 {
		cachePath := filepath.Join(os.TempDir(), fmt.Sprintf("t_%s_%d.jpg", fileID, width))
		if cachedData, err := os.ReadFile(cachePath); err == nil {
			c.Set("Content-Type", "image/jpeg")
			c.Set("X-Cache", "HIT")
			return c.Send(cachedData)
		}
	}

	var bodyBytes []byte
	var contentType string

	if platform == "line" {
		lnBotIf, ok := bots.BotManager.Get("line")
			if !ok { return c.Status(503).JSON(fiber.Map{"error": "LINE bot not initialized"}) }
			lnBot := lnBotIf.(*bots.LineBot)
			content, err := lnBot.Bot.GetMessageContent(fileID).Do()
			if err != nil { return c.Status(404).JSON(fiber.Map{"error": "Content expired"}) }
			defer content.Content.Close()
			bodyBytes, _ = io.ReadAll(content.Content)
			contentType = content.ContentType
	} else if platform == "discord" {
		if !strings.HasPrefix(fileID, "http") { return c.Status(400).JSON(fiber.Map{"error": "Invalid Discord URL"}) }
		resp, err := http.Get(fileID)
		if err != nil { return c.Status(502).JSON(fiber.Map{"error": "Discord error"}) }
		defer resp.Body.Close()
		bodyBytes, _ = io.ReadAll(resp.Body)
		contentType = resp.Header.Get("Content-Type")
	} else if platform == "telegram" {
		tgBotIf, ok := bots.BotManager.Get("telegram")
		if !ok { return c.Status(503).JSON(fiber.Map{"error": "TG bot not initialized"}) }
		tgBot := tgBotIf.(*bots.TelegramBot)
		data, cType, err := tgBot.GetFile(c.UserContext(), fileID)
		if err != nil { return c.Status(404).JSON(fiber.Map{"error": "File not found on TG"}) }
		bodyBytes = data
		contentType = cType
	} else {
		return c.Status(400).JSON(fiber.Map{"error": "Unknown platform"})
	}

	// 🛠️ Apply Resizing if width is requested
	if width > 0 && strings.HasPrefix(contentType, "image/") {
		img, _, err := image.Decode(bytes.NewReader(bodyBytes))
		if err == nil {
			// Resize using Lanczos3 (Best quality)
			newImg := resize.Resize(uint(width), 0, img, resize.Lanczos3)
			buf := new(bytes.Buffer)
			if err := jpeg.Encode(buf, newImg, &jpeg.Options{Quality: 80}); err == nil {
				resizedData := buf.Bytes()
				// Save to Cache
				cachePath := filepath.Join(os.TempDir(), fmt.Sprintf("t_%s_%d.jpg", fileID, width))
				os.WriteFile(cachePath, resizedData, 0644)
				
				c.Set("Content-Type", "image/jpeg")
				c.Set("X-Cache", "MISS")
				return c.Send(resizedData)
			}
		}
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	return c.Send(bodyBytes)
}
