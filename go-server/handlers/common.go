package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
)

type CommonState struct {
	Key       string  `json:"key"`
	Content   *string `json:"content"`
	FileURL   *string `json:"file_url"`
	FileName  *string `json:"file_name"`
	UpdatedBy *string `json:"updated_by"`
	UpdatedAt string  `json:"updated_at"`
}

type TextHistory struct {
	ID        int     `json:"id"`
	Content   string  `json:"content"`
	UserName  *string `json:"user_name"`
	CreatedAt string  `json:"created_at"`
}

func GetCommonState(c *fiber.Ctx) error {
	rows, err := database.CloudDB.Query(context.Background(), "SELECT key, content, file_url, file_name, updated_by, updated_at FROM common_state")
	if err != nil {
		log.Printf("Query common_state failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch common state"})
	}
	defer rows.Close()

	stateMap := make(map[string]CommonState)
	for rows.Next() {
		var s CommonState
		if err := rows.Scan(&s.Key, &s.Content, &s.FileURL, &s.FileName, &s.UpdatedBy, &s.UpdatedAt); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Scan failed"})
		}
		stateMap[s.Key] = s
	}
	return c.JSON(stateMap)
}

func GetCommonHistory(c *fiber.Ctx) error {
	// Note: users table is in LocalDB, so we skip the join for now.
	rows, err := database.CloudDB.Query(context.Background(), `
		SELECT id, content, created_at 
		FROM common_text_history 
		ORDER BY created_at DESC 
		LIMIT 10
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch history"})
	}
	defer rows.Close()

	history := []fiber.Map{}
	for rows.Next() {
		var id int
		var content, createdAt string
		rows.Scan(&id, &content, &createdAt)
		history = append(history, fiber.Map{
			"id": id,
			"content": content,
			"user_name": "Unknown (Dual DB Sync pending)", 
			"created_at": createdAt,
		})
	}
	return c.JSON(history)
}

func UpdateCommonState(c *fiber.Ctx) error {
	var body struct {
		Key      string  `json:"key"`
		Content  *string `json:"content"`
		FileURL  *string `json:"fileUrl"`
		FileName *string `json:"fileName"`
		UserID   *string `json:"userId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	query := `
		UPDATE common_state 
		SET content = $1, file_url = $2, file_name = $3, updated_by = $4, updated_at = CURRENT_TIMESTAMP 
		WHERE key = $5
	`
	_, err := database.CloudDB.Exec(context.Background(), query, body.Content, body.FileURL, body.FileName, body.UserID, body.Key)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Update common state failed"})
	}

	// Handle history for text
	if body.Key == "text" && body.Content != nil && *body.Content != "" {
		database.CloudDB.Exec(context.Background(), 
			"INSERT INTO common_text_history (content, user_id) VALUES ($1, $2)", 
			body.Content, body.UserID)
		
		// Clean up over 10 (Simplified for now)
		database.CloudDB.Exec(context.Background(), `
			DELETE FROM common_text_history WHERE id NOT IN (
				SELECT id FROM common_text_history ORDER BY created_at DESC LIMIT 10
			)
		`)
	}

	return c.JSON(fiber.Map{"success": true})
}

// --- Global Settings (LocalDB) ---

func GetSettings(c *fiber.Ctx) error {
	if database.LocalDB == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Local DB not connected"})
	}
	rows, err := database.LocalDB.Query(context.Background(), "SELECT key, value FROM settings")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		settings[k] = v
	}
	return c.JSON(settings)
}

func UpdateSetting(c *fiber.Ctx) error {
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	query := `
		INSERT INTO settings (key, value, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`
	_, err := database.LocalDB.Exec(context.Background(), query, body.Key, body.Value)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update setting"})
	}

	return c.JSON(fiber.Map{"success": true})
}
