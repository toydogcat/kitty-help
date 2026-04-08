package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func GetCalendarEvents(c *fiber.Ctx) error {
	query := `
		SELECT c.id, c.user_id, c.event_date, c.content, c.created_at, u.name
		FROM calendar_events c
		JOIN users u ON c.user_id = u.id
		ORDER BY c.event_date ASC
	`
	if database.CloudDB == nil {
		return c.JSON([]models.CalendarEvent{})
	}
	rows, err := database.CloudDB.Query(context.Background(), query)
	if err != nil {
		log.Printf("Query calendar failed: %v", err)
		return c.JSON([]models.CalendarEvent{})
	}
	defer rows.Close()

	events := []models.CalendarEvent{}
	for rows.Next() {
		var e models.CalendarEvent
		err := rows.Scan(&e.ID, &e.UserID, &e.Date, &e.Content, &e.CreatedAt, &e.UserName)
		if err != nil {
			log.Printf("Scan calendar event failed: %v", err)
			continue
		}
		events = append(events, e)
	}
	return c.JSON(events)
}

func UpdateCalendarEvent(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var body struct {
		Date    string `json:"date"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Resolve internal DB User ID from email
	var dbUserID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := `
		INSERT INTO calendar_events (user_id, event_date, content, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, event_date) 
		DO UPDATE SET content = EXCLUDED.content, updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	if database.CloudDB == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Cloud database not connected"})
	}

	_, err = database.CloudDB.Exec(context.Background(), query, dbUserID, body.Date, body.Content)
	if err != nil {
		log.Printf("Update calendar failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update calendar"})
	}

	return c.JSON(fiber.Map{"success": true})
}

func GetBulletin(c *fiber.Ctx) error {
	db := database.LocalDB
	if db == nil {
		db = database.CloudDB
	}

	if db == nil {
		return c.JSON(fiber.Map{"message": "Welcome to Kitty-Help! (No DB connected)"})
	}

	var message string
	err := db.QueryRow(context.Background(), "SELECT message FROM bulletin ORDER BY updated_at DESC LIMIT 1").Scan(&message)
	if err != nil {
		return c.JSON(fiber.Map{"message": "Welcome back, Kitty-Admin!"})
	}
	return c.JSON(fiber.Map{"message": message})
}

func UpdateBulletin(c *fiber.Ctx) error {
	var body struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	val := c.Locals("user")
	if val == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	claims, ok := val.(*Claims)
	if !ok || claims.Role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden: Admin access required"})
	}

	db := database.LocalDB
	if db == nil {
		db = database.CloudDB
	}

	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database not connected"})
	}

	_, err := db.Exec(context.Background(), "INSERT INTO bulletin (message, updated_at) VALUES ($1, CURRENT_TIMESTAMP)", body.Message)
	if err != nil {
		log.Printf("Failed to update bulletin: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Update bulletin failed"})
	}

	return c.JSON(fiber.Map{"success": true})
}
