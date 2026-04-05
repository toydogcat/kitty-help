package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func GetDevices(c *fiber.Ctx) error {
	query := `
		SELECT d.id, d.status, d.device_name, d.user_agent, d.user_id, u.name as user_name 
		FROM devices d 
		LEFT JOIN users u ON d.user_id = u.id 
		ORDER BY d.created_at DESC
	`
	rows, err := database.LocalDB.Query(context.Background(), query)
	if err != nil {
		log.Printf("Query devices failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch devices"})
	}
	defer rows.Close()

	devices := []models.Device{}
	for rows.Next() {
		var d models.Device
		err := rows.Scan(&d.ID, &d.Status, &d.DeviceName, &d.UserAgent, &d.UserID, &d.UserName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to scan devices"})
		}
		devices = append(devices, d)
	}
	return c.JSON(devices)
}

func RegisterDevice(c *fiber.Ctx) error {
	var body struct {
		ID        string `json:"id"`
		UserAgent string `json:"userAgent"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Check if exists
	var d models.Device
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id, status, device_name FROM devices WHERE id = $1", body.ID).Scan(&d.ID, &d.Status, &d.DeviceName)
	if err != nil {
		// Insert
		status := "pending"
		_, err = database.LocalDB.Exec(context.Background(),
			"INSERT INTO devices (id, user_agent, status) VALUES ($1, $2, $3)",
			body.ID, body.UserAgent, status)
		if err != nil {
			log.Printf("Insert device failed: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to register"})
		}
		d.ID = body.ID
		d.Status = status
		d.UserAgent = &body.UserAgent
	}

	// Update last_active and get status + user role
	query := `
		SELECT d.id, d.status, d.device_name, u.role as user_role, u.name as user_name
		FROM devices d
		JOIN users u ON d.user_id = u.id
		WHERE d.id = $1
	`
	// Try to update last_active separately to keep RETURNING simple for the JOIN case
	database.LocalDB.Exec(context.Background(), "UPDATE devices SET last_active = CURRENT_TIMESTAMP WHERE id = $1", body.ID)

	err = database.LocalDB.QueryRow(context.Background(), query, body.ID).Scan(&d.ID, &d.Status, &d.DeviceName, &d.UserRole, &d.UserName)
	if err != nil {
		// If no user assigned yet, already got basic info from first query or insert
	}

	return c.JSON(d)
}

func UpdateDeviceStatus(c *fiber.Ctx) error {
	var body struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		DeviceName string `json:"deviceName"`
		UserID     string `json:"userId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	_, err := database.LocalDB.Exec(context.Background(),
		"UPDATE devices SET status = $1, device_name = $2, user_id = $3 WHERE id = $4",
		body.Status, body.DeviceName, body.UserID, body.ID)
	if err != nil {
		log.Printf("Update device status failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Update status failed"})
	}

	return c.JSON(fiber.Map{"success": true})
}

func DeleteDevice(c *fiber.Ctx) error {
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM devices WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}
