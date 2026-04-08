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

	fmt.Printf("[STATUS DEBUG] Checking status for UserID: [%s]\n", user.ID)
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
	count := 0
	for rows.Next() {
		var platform string
		rows.Scan(&platform)
		fmt.Printf("[STATUS DEBUG] Found linked platform: [%s] for UserID: [%s]\n", platform, user.ID)
		status[platform] = true
		count++
	}
	fmt.Printf("[STATUS DEBUG] Total linked platforms found: %d for UserID: %s\n", count, user.ID)

	return c.JSON(status)
}
func SendBotMessage(c *fiber.Ctx) error {
	platform := c.FormValue("platform")
	content := c.FormValue("content")
	targetID := c.FormValue("targetId")

	if platform == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Platform is required"})
	}

	botIf, ok := bots.BotManager.Get(platform)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"error": "Bot for this platform not found"})
	}

	// 1. Resolve Default Target ID
	if targetID == "" {
		switch platform {
		case "telegram":
			targetID = os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID")
		case "discord":
			targetID = os.Getenv("DISCORD_ADMIN_CHANNEL_ID")
		case "line":
			targetID = os.Getenv("ADMIN_LINE_ID")
		}
	}

	if targetID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Target ID is missing and no default available in .env"})
	}

	// 2. Handle File Upload (Optional)
	file, err := c.FormFile("file")
	var mediaID *string = nil
	msgType := "text"

	if err == nil {
		// Save file to uploads/
		tempPath := filepath.Join("..", "uploads", file.Filename)
		if err := c.SaveFile(file, tempPath); err == nil {
			// For now, if it's Telegram, we can try to use UploadMedia if implemented
			// But for simplicity in this MVP, we just send the text link or record it
			msgType = "media"
			// (Future: Actually upload to TG/Discord storage)
		}
	}

	// 3. Send via Bot
	err = botIf.SendMessage(targetID, content)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Send failed: %v", err)})
	}

	// 4. Log to DB
	// We use a dummy ID for the bot sender (or get bot's own ID)
	botName := "KittyBot (" + platform + ")"
	_, _ = database.LocalDB.Exec(context.Background(),
		"INSERT INTO chat_logs (platform, sender_id, sender_name, content, msg_type, media_id) VALUES ($1, $2, $3, $4, $5, $6)",
		platform, "bot-api", botName, content, msgType, mediaID)

	return c.JSON(fiber.Map{"status": "success", "message": "Sent to " + platform})
}
