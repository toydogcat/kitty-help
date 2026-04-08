package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func GetUsers(c *fiber.Ctx) error {
	rows, err := database.LocalDB.Query(context.Background(), "SELECT id, name, role, google_id, email FROM users ORDER BY name")
	if err != nil {
		log.Printf("Query users failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users"})
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Role, &u.GoogleID, &u.Email); err != nil {
			log.Printf("Scan user failed: %v", err)
			continue
		}
		users = append(users, u)
	}
	return c.JSON(users)
}

func CreateUser(c *fiber.Ctx) error {
	var u models.User
	if err := c.BodyParser(&u); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	err := database.LocalDB.QueryRow(context.Background(),
		"INSERT INTO users (name, role) VALUES ($1, $2) RETURNING id",
		u.Name, u.Role).Scan(&u.ID)
	if err != nil {
		log.Printf("Insert user failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
	}

	return c.JSON(u)
}

func UpdateUserRole(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var body struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Permission check from JWT
	if userClaims.Role != "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "Only superadmin can manage roles"})
	}

	_, err := database.LocalDB.Exec(context.Background(), "UPDATE users SET role = $1 WHERE id = $2", body.Role, body.UserID)
	if err != nil {
		log.Printf("Update user role failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update role"})
	}

	return c.JSON(fiber.Map{"success": true})
}

func DeleteUser(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	id := c.Params("id")

	if userClaims.Role != "superadmin" && userClaims.Role != "toby" {
		return c.Status(403).JSON(fiber.Map{"error": "Only superadmin can delete users"})
	}

	// Prevent self-deletion
	if userClaims.ID == id {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot delete yourself"})
	}

	// Check if target is superadmin
	var targetRole string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT role FROM users WHERE id = $1", id).Scan(&targetRole)
	if err == nil && targetRole == "superadmin" {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot delete another superadmin"})
	}

	_, err = database.LocalDB.Exec(context.Background(), "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		log.Printf("Delete user failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete user"})
	}
	
	// Also cleanup associated bot authorized entries
	database.LocalDB.Exec(context.Background(), "DELETE FROM bot_authorized_users WHERE user_id = $1", id)

	return c.JSON(fiber.Map{"success": true})
}
