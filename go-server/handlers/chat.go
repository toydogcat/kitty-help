package handlers

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
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

	if platform == "" {
		return c.Status(400).JSON(fiber.Map{"error": "platform query required"})
	}

	sql := "SELECT id, platform, sender_id, sender_name, content, media_type, file_id, created_at FROM media_archives WHERE platform = $1"
	args := []interface{}{platform}
	argIdx := 2

	if searchQuery != "" {
		sql += fmt.Sprintf(" AND content ILIKE $%d", argIdx)
		args = append(args, "%"+searchQuery+"%")
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

	sql += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)

	rows, err := db.Query(context.Background(), sql, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	logs := []models.ChatLog{}
	for rows.Next() {
		var l models.ChatLog
		err := rows.Scan(&l.ID, &l.Platform, &l.SenderID, &l.SenderName, &l.Content, &l.MsgType, &l.MediaID, &l.CreatedAt)
		if err != nil {
			continue
		}
		logs = append(logs, l)
	}

	return c.JSON(logs)
}

func GetMyBotStatus(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || user == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	fmt.Printf("[STATUS DEBUG] Checking status for UserID: [%s]\n", user.ID)
	if user.ID == "82507694-4205-49d4-8099-9e18ba997581" {
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
