package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
					FROM (SELECT DISTINCT ON (file_id) * FROM media_archives ORDER BY file_id, created_at DESC) m
					JOIN bot_authorized_users b ON m.sender_id = b.account_id AND m.source_platform = b.platform
					WHERE b.user_id = $1 AND m.embedding IS NOT NULL`
				args = append(args, dbUserID)
				argIdx = 2
			} else {
				// Admin or unauthenticated sees global (semantic)
				sql = `SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status 
				       FROM (SELECT DISTINCT ON (file_id) * FROM media_archives ORDER BY file_id, created_at DESC) m WHERE embedding IS NOT NULL`
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
				FROM (SELECT DISTINCT ON (file_id) * FROM media_archives ORDER BY file_id, created_at DESC) m
				JOIN bot_authorized_users b ON m.sender_id = b.account_id AND m.source_platform = b.platform
				WHERE b.user_id = $1`
			args = append(args, dbUserID)
			argIdx = 2
		} else {
			// Admin sees all
			sql = `SELECT id, file_id, media_type, title, caption, notes, source_platform, sender_name, created_at, is_indexable, index_status 
			       FROM (SELECT DISTINCT ON (file_id) * FROM media_archives ORDER BY file_id, created_at DESC) m WHERE 1=1`
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

	if platform == "local" || (platform == "" && query != "") {
		// Only show local files if specifically requested or if there's a search query
		// to avoid cluttering the main view with thousands of vault files
		root := "/root/obsidian"
		if _, err := os.Stat(root); err == nil {
			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
					return nil
				}
				
				rel, _ := filepath.Rel(root, path)
				if query != "" && !strings.Contains(strings.ToLower(rel), strings.ToLower(query)) {
					return nil
				}

				items = append(items, fiber.Map{
					"id": rel, // Use relative path as ID
					"file_id": rel,
					"category": "document",
					"title": info.Name(),
					"caption": nil,
					"notes": "Local Obsidian Note",
					"source": "local",
					"sender": "Obsidian",
					"created_at": info.ModTime(),
					"is_indexable": false,
					"index_status": "not_indexed",
				})

				if len(items) >= limit + 20 { // Buffer limit
					return filepath.SkipDir
				}
				return nil
			})
		}
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
	platform := c.Query("platform", "telegram")
	fullPath := c.Params("*")
	
	fileID := ""
	fakeFileName := ""

	// 🚀 Path Intelligence Analysis:
	// If platform is 'local' (Obsidian), the path IS the ID (it naturally contains slashes).
	// For other platforms (Telegram/Media), we apply the 'Chromebook Fix' by stripping the fake trailing filename.
	if platform == "local" {
		fileID = fullPath
		fakeFileName = filepath.Base(fullPath)
	} else {
		parts := strings.Split(fullPath, "/")
		if len(parts) > 1 {
			fileID = strings.Join(parts[:len(parts)-1], "/")
			fakeFileName = parts[len(parts)-1]
		} else {
			fileID = parts[0]
			fakeFileName = "document"
		}
	}
	width := c.QueryInt("w", 0)

	c.Set("Cache-Control", "public, max-age=604800, immutable")
	c.Set("ETag", fileID)

	if c.Get("If-None-Match") == fileID {
		return c.SendStatus(304)
	}

	// 🚀 NEW: Check for local uploads in standardized workspace first
	workspaceUploadPath := filepath.Join("/root/.kitty-help/workspace/uploads", fileID)
	if _, err := os.Stat(workspaceUploadPath); err == nil {
		return c.SendFile(workspaceUploadPath)
	}

	if platform == "local" {
		root := "/root/obsidian"
		decodedID, err := url.PathUnescape(fileID)
		if err != nil {
			decodedID = fileID
		}
		fullPath := filepath.Join(root, decodedID)
		// Security check: ensure path is within root
		if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(root)) {
			return c.Status(403).JSON(fiber.Map{"error": "Forbidden path"})
		}
		
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Local file not found"})
		}
		
		ext := strings.ToLower(filepath.Ext(fullPath))
		contentType := "application/octet-stream"
		switch ext {
		case ".md", ".markdown", ".txt":
			contentType = "text/markdown; charset=utf-8"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".gif":
			contentType = "image/gif"
		case ".svg":
			contentType = "image/svg+xml"
		case ".webp":
			contentType = "image/webp"
		case ".pdf":
			contentType = "application/pdf"
		}
		
		c.Set("Content-Type", contentType)
		c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(fullPath)))
		return c.Send(data)
	}

	bodyBytes, contentType, err := services.MediaManager.FetchAndCache(c.UserContext(), fileID, platform, width)
	if err != nil {
		log.Printf("❌ [Proxy] Resource error: %v", err)
		return c.Status(502).JSON(fiber.Map{"error": err.Error()})
	}

	// Smart Content-Type fix for Documents
	if contentType == "application/octet-stream" || contentType == "" {
		// Try to detect by file extension if possible, or look up in DB
		var title string
		_ = database.LocalDB.QueryRow(context.Background(), "SELECT title FROM media_archives WHERE file_id = $1 OR id::text = $1", fileID).Scan(&title)
		title = strings.ToLower(title)
		if strings.HasSuffix(title, ".pdf") {
			contentType = "application/pdf"
		} else if strings.HasSuffix(title, ".epub") {
			contentType = "application/epub+zip"
		} else if strings.HasSuffix(title, ".djvu") {
			contentType = "image/vnd.djvu"
		}
	}

	// 🚀 FINAL DEFENSE: Active Content Sniffing if type is still generic
	if (contentType == "application/octet-stream" || contentType == "") && len(bodyBytes) > 0 {
		limit := len(bodyBytes)
		if limit > 512 {
			limit = 512
		}
		contentType = http.DetectContentType(bodyBytes[:limit])
		log.Printf("🔍 [Proxy] Sniffed type: %s for %s", contentType, fileID)
	}

	if c.Query("download") == "1" {
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fakeFileName))
	} else {
		// 🛠️ Enhanced iPad/ChromeOS Preview Support: Force inline with filename
		c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fakeFileName))
	}

	c.Set("Content-Type", contentType)
	c.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	return c.Send(bodyBytes)
}

func ListObsidianFiles(c *fiber.Ctx) error {
	relPath := c.Query("path", "")
	root := "/root/obsidian"
	targetDir := filepath.Join(root, relPath)

	// Security: prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(targetDir), filepath.Clean(root)) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden path"})
	}

	showHidden := c.Query("showHidden", "false") == "true"
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	files := []fiber.Map{}
	for _, entry := range entries {
		name := entry.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		info, _ := entry.Info()
		files = append(files, fiber.Map{
			"name":    name,
			"isDir":   entry.IsDir(),
			"size":    info.Size(),
			"modTime": info.ModTime(),
			"path":    filepath.Join(relPath, name),
		})
	}

	return c.JSON(fiber.Map{
		"currentPath": relPath,
		"files":       files,
	})
}

func SearchObsidianFiles(c *fiber.Ctx) error {
	query := strings.ToLower(c.Query("q", ""))
	if query == "" {
		return c.JSON(fiber.Map{"results": []fiber.Map{}})
	}

	root := "/root/obsidian"
	var results []fiber.Map

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Check filename
		relPath, _ := filepath.Rel(root, path)
		if strings.Contains(strings.ToLower(info.Name()), query) {
			results = append(results, fiber.Map{
				"name":    info.Name(),
				"path":    relPath,
				"snippet": "匹配檔案名稱",
				"isDir":   false,
				"modTime": info.ModTime(),
			})
			return nil
		}

		// Check content (limit to some size to prevent memory issues)
		if info.Size() > 1024*1024 { // Skip files > 1MB for now
			return nil
		}

		content, err := os.ReadFile(path)
		if err == nil {
			contentStr := string(content)
			lowerContent := strings.ToLower(contentStr)
			if idx := strings.Index(lowerContent, query); idx != -1 {
				// Get a small snippet
				start := idx - 30
				if start < 0 {
					start = 0
				}
				end := idx + 70
				if end > len(contentStr) {
					end = len(contentStr)
				}
				snippet := contentStr[start:end]
				
				// Highlight the query in the snippet
				highlighted := strings.ReplaceAll(snippet, query, "<strong>"+query+"</strong>")
				if query == strings.ToLower(query) {
					// Also handle potential case differences if query was normalized
					// Simplified: we'll just use ReplaceAll for the direct match for now
				}

				results = append(results, fiber.Map{
					"name":    info.Name(),
					"path":    relPath,
					"snippet": "..." + highlighted + "...",
					"isDir":   false,
					"modTime": info.ModTime(),
				})
			}
		}

		return nil
	})

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"results": results})
}
