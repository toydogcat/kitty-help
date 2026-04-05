package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

// Password struct moved to models package

func GetPasswords(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	// Identify user DB ID from email (resolvedEmail)
	var userID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	rows, err := database.LocalDB.Query(context.Background(), 
		"SELECT id, user_id, site_name, account, password_raw, category, notes, created_at FROM passwords WHERE user_id = $1 ORDER BY created_at DESC", 
		userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var passwords []models.Password
	for rows.Next() {
		var p models.Password
		var id, uid uuid.UUID
		var createdAt interface{}
		err := rows.Scan(&id, &uid, &p.SiteName, &p.Account, &p.PasswordRaw, &p.Category, &p.Notes, &createdAt)
		if err == nil {
			p.ID = id.String()
			p.UserID = uid.String()
			passwords = append(passwords, p)
		}
	}

	return c.JSON(passwords)
}

func AddPassword(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var p models.Password
	if err := c.BodyParser(&p); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Resolve User ID
	var userID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	_, err = database.LocalDB.Exec(context.Background(), 
		"INSERT INTO passwords (user_id, site_name, account, password_raw, category, notes) VALUES ($1, $2, $3, $4, $5, $6)",
		userID, p.SiteName, p.Account, p.PasswordRaw, p.Category, p.Notes)
	
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save password"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

func DeletePassword(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing ID"})
	}

	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM passwords WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete password"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}
