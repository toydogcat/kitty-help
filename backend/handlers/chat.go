package handlers

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"os"
	"path/filepath"
	"strings"
)

func GetChatLogs(c *fiber.Ctx) error {
	db := database.LocalDB
	if db == nil {
		db = database.CloudDB
	}

	platform := c.Query("platform")
	searchQuery := c.Query("q")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	limit := c.QueryInt("limit", 100)

	user := c.Locals("user").(*Claims)
	userID := user.ID
	if userID == "" {
		userID = "00000000-0000-0000-0000-000000000000"
	}
	
	sql := `
		SELECT 
			c.id, c.platform, c.sender_id, c.sender_name, c.content, c.msg_type, c.media_id, c.created_at,
			COALESCE(m.media_type, '') as media_type,
			CASE WHEN ri.id IS NOT NULL THEN true ELSE false END as is_integrated
		FROM chat_logs c
		LEFT JOIN media_archives m ON c.media_id::text = m.id::text
		LEFT JOIN remark_items ri ON c.id::text = ri.log_id::text AND ri.user_id = $1
		WHERE 1=1
	`
	args := []interface{}{userID}
	argIdx := 2

	if platform != "" {
		sql += fmt.Sprintf(" AND c.platform = $%d", argIdx)
		args = append(args, platform)
		argIdx++
	}

	if searchQuery != "" {
		sql += fmt.Sprintf(" AND c.content ILIKE $%d", argIdx)
		args = append(args, "%"+searchQuery+"%")
		argIdx++
	}

	if startDate != "" {
		sql += fmt.Sprintf(" AND c.created_at >= $%d", argIdx)
		args = append(args, startDate)
		argIdx++
	}

	if endDate != "" {
		sql += fmt.Sprintf(" AND c.created_at <= $%d", argIdx)
		args = append(args, endDate)
		argIdx++
	}

	sql += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT %d", limit)

	rows, err := db.Query(context.Background(), sql, args...)
	if err != nil {
		fmt.Printf("[DB ERROR] Chat Logs: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "sql": "chat_logs_query"})
	}
	defer rows.Close()

	logs := []models.ChatLog{}
	for rows.Next() {
		var l models.ChatLog
		err := rows.Scan(&l.ID, &l.Platform, &l.SenderID, &l.SenderName, &l.Content, &l.MsgType, &l.MediaID, &l.CreatedAt, &l.MediaType, &l.IsIntegrated)
		if err != nil {
			fmt.Printf("[SCAN ERROR] %v\n", err)
			continue
		}
		logs = append(logs, l)
	}

	return c.JSON(logs)
}

func GetRecentPhotos(c *fiber.Ctx) error {
	db := database.LocalDB
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	offset := (page - 1) * limit

	sql := "SELECT id, platform, sender_id, sender_name, content, msg_type, media_id, created_at FROM chat_logs WHERE msg_type = 'image' OR content ILIKE '%[Image]%' ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := db.Query(context.Background(), sql, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	photos := []models.ChatLog{}
	for rows.Next() {
		var l models.ChatLog
		rows.Scan(&l.ID, &l.Platform, &l.SenderID, &l.SenderName, &l.Content, &l.MsgType, &l.MediaID, &l.CreatedAt)
		photos = append(photos, l)
	}

	var total int
	db.QueryRow(context.Background(), "SELECT COUNT(*) FROM chat_logs WHERE msg_type = 'image' OR content ILIKE '%[Image]%'").Scan(&total)

	return c.JSON(fiber.Map{"photos": photos, "total": total})
}


func GetMyBotStatus(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || user == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Recognition for both legacy and new Admin IDs during transition
	if user.ID == "41023b15-a1db-4aac-aab8-ba75d8d90905" || user.ID == "82507694-4205-49d4-8099-9e18ba997581" {
		// FORCE INITIALIZE (Overwrite) for Admin Toby on THIS specific database
		database.LocalDB.Exec(context.Background(),
			"INSERT INTO bot_authorized_users (platform, account_id, account_name, user_id, role) VALUES ('telegram', '1089079202', 'Master Admin-Toby', $1, 'superadmin') ON CONFLICT (platform, account_id) DO UPDATE SET user_id = EXCLUDED.user_id, role = EXCLUDED.role", user.ID)
		database.LocalDB.Exec(context.Background(),
			"INSERT INTO bot_authorized_users (platform, account_id, account_name, user_id, role) VALUES ('discord', '840468194456371211', 'Master Admin-Toby', $1, 'superadmin') ON CONFLICT (platform, account_id) DO UPDATE SET user_id = EXCLUDED.user_id, role = EXCLUDED.role", user.ID)
		database.LocalDB.Exec(context.Background(),
			"INSERT INTO bot_authorized_users (platform, account_id, account_name, user_id, role) VALUES ('line', 'Uaecf740fc05ef668b671fa90da9c832e', 'Master Admin-Toby', $1, 'superadmin') ON CONFLICT (platform, account_id) DO UPDATE SET user_id = EXCLUDED.user_id, role = EXCLUDED.role", user.ID)
	}

	rows, err := database.LocalDB.Query(context.Background(),
		"SELECT platform FROM bot_authorized_users WHERE user_id = $1", user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	status := fiber.Map{
		"telegram": false,
		"discord":  false,
		"line":     false,
	}
	for rows.Next() {
		var platform string
		rows.Scan(&platform)
		status[platform] = true
	}

	return c.JSON(status)
}

// ListUploads returns a list of files in the standardized uploads directory
func ListUploads(c *fiber.Ctx) error {
	workspacePath := "/root/.kitty-help/workspace"
	uploadDir := filepath.Join(workspacePath, "uploads")
	if _, err := os.Stat(workspacePath); err != nil {
		uploadDir = "../uploads"
	}

	files, err := os.ReadDir(uploadDir)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	result := []fiber.Map{}
	for _, f := range files {
		if !f.IsDir() {
			info, _ := f.Info()
			result = append(result, fiber.Map{
				"name": f.Name(),
				"size": info.Size(),
				"time": info.ModTime(),
				"type": strings.ToLower(filepath.Ext(f.Name())),
			})
		}
	}
	return c.JSON(result)
}

func SendBotMessage(c *fiber.Ctx) error {
	userClaims, _ := c.Locals("user").(*Claims)
	platform := c.FormValue("platform")
	content := c.FormValue("content")
	targetID := c.FormValue("targetId")
	selectedFiles := c.FormValue("selectedFiles")

	if platform == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Platform is required"})
	}

	botIf, ok := bots.BotManager.Get(platform)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"error": "Bot platform missing"})
	}

	// 1. Resolve Target ID (PRIORITY: 1. Manual Input -> 2. Database Link -> 3. Environment Fallback)
	resolvedMethod := "Manual"
	if targetID == "" && userClaims != nil {
		err := database.LocalDB.QueryRow(context.Background(),
			"SELECT account_id FROM bot_authorized_users WHERE user_id = $1 AND platform = $2",
			userClaims.ID, platform).Scan(&targetID)
		if err == nil && targetID != "" {
			resolvedMethod = "Database"
		}
	}

	if targetID == "" {
		switch platform {
		case "telegram": targetID = os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID")
		case "discord": targetID = os.Getenv("DISCORD_ADMIN_CHANNEL_ID")
		case "line": targetID = os.Getenv("ADMIN_LINE_ID")
		}
		if targetID != "" {
			resolvedMethod = "Environment"
		}
	}

	if targetID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "No target ID found. Please link your account via /link command in bot."})
	}
	fmt.Printf("🤖 [BOT DISPATCH] Platform: %s | Target: %s | Method: %s\n", platform, targetID, resolvedMethod)

	// 2. Handle Files (Multi-mode)
	workspacePath := "/root/.kitty-help/workspace"
	uploadDir := filepath.Join(workspacePath, "uploads")
	if _, err := os.Stat(workspacePath); err != nil {
		uploadDir = "../uploads"
	}
	os.MkdirAll(uploadDir, 0755)

	var finalFileNames []string

	// A. Direct Uploads (Multiple)
	form, err := c.MultipartForm()
	if err == nil {
		for _, file := range form.File["files"] {
			tempPath := filepath.Join(uploadDir, file.Filename)
			if err := c.SaveFile(file, tempPath); err == nil {
				finalFileNames = append(finalFileNames, file.Filename)
			}
		}
	}

	// B. Selected from Existing
	if selectedFiles != "" {
		for _, f := range strings.Split(selectedFiles, ",") {
			trimmedFile := strings.TrimSpace(f)
			if trimmedFile != "" {
				finalFileNames = append(finalFileNames, trimmedFile)
			}
		}
	}

	// 3. Dispatch
	if content != "" {
		_ = botIf.SendMessage(targetID, content)
	}

	for _, filename := range finalFileNames {
		tempPath := filepath.Join(uploadDir, filename)
		ext := strings.ToLower(filepath.Ext(filename))
		botMediaType := "document"
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			botMediaType = "photo"
		} else if ext == ".mp4" || ext == ".mov" {
			botMediaType = "video"
		}

		err := botIf.SendMedia(targetID, botMediaType, tempPath, "")
		if err != nil {
			fmt.Printf("[BOT SEND FAIL] %s error: %v (Target: %s)\n", platform, err, targetID)
			return c.Status(500).JSON(fiber.Map{"error": "Send failed to "+targetID+": " + err.Error()})
		}

		// Log to DB
		botName := "Bot Dispatcher (" + platform + ")"
		mediaID := filename
		_, _ = database.LocalDB.Exec(context.Background(),
			"INSERT INTO chat_logs (platform, sender_id, sender_name, content, msg_type, media_id) VALUES ($1, $2, $3, $4, $5, $6)",
			platform, "bot-api", botName, "(File: "+filename+")", "media", &mediaID)
	}

	return c.JSON(fiber.Map{"status": "success", "count": len(finalFileNames)})
}
