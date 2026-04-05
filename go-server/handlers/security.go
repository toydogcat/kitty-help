package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/security"
)

// RequestChallenge starts a new 2FA process
func RequestChallenge(c *fiber.Ctx) error {
	var body struct {
		UserID   string `json:"userId"`
		DeviceID string `json:"deviceId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	token := security.GenerateRandomToken(6)
	expiresAt := time.Now().Add(10 * time.Minute)

	if database.LocalDB == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database unavailable"})
	}

	_, err := database.LocalDB.Exec(context.Background(),
		"INSERT INTO security_sessions (user_id, device_id, token, expires_at) VALUES ($1, $2, $3, $4)",
		body.UserID, body.DeviceID, token, expiresAt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create challenge"})
	}

	return c.JSON(fiber.Map{"token": token, "expiresAt": expiresAt})
}

// GetSecurityStatus checks the current verification state
func GetSecurityStatus(c *fiber.Ctx) error {
	userId := c.Query("userId")
	deviceId := c.Query("deviceId")
	token := c.Query("token")

	if userId == "" {
		// Fallback to JWT claims if available (e.g. from JWTMiddleware)
		if user, ok := c.Locals("user").(*Claims); ok && user != nil {
			userId = user.Email // In this project, userId used in security_sessions matches email for identification
		}
	}

	if userId == "" || deviceId == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing userId or deviceId"})
	}

	if database.LocalDB == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database unavailable"})
	}

	// 1. Check for active granted session (30m trust window)
	var grantedAt *time.Time
	err := database.LocalDB.QueryRow(context.Background(),
		"SELECT granted_at FROM security_sessions WHERE user_id = $1 AND device_id = $2 AND status = 'granted' AND granted_at > $3 ORDER BY granted_at DESC LIMIT 1",
		userId, deviceId, time.Now().Add(-30*time.Minute)).Scan(&grantedAt)
	
	if err == nil && grantedAt != nil {
		remaining := time.Until(grantedAt.Add(30 * time.Minute))
		return c.JSON(fiber.Map{
			"status": "granted",
			"remainingSeconds": int(remaining.Seconds()),
		})
	}

	// 2. If token is provided, check the progress of the challenge
	if token != "" {
		var lineAt, discordAt *time.Time
		var status string
		err := database.LocalDB.QueryRow(context.Background(),
			"SELECT line_verified_at, discord_verified_at, status FROM security_sessions WHERE token = $1",
			token).Scan(&lineAt, &discordAt, &status)
		if err == nil {
			return c.JSON(fiber.Map{
				"status": status,
				"lineVerified": lineAt != nil,
				"discordVerified": discordAt != nil,
			})
		}
	}

	return c.JSON(fiber.Map{"status": "no_session"})
}
