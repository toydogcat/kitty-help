package handlers

import (
	"context"
	"fmt"
	"time"
	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
)

// --- Admin APIs (Protected) ---

func GetPendingBotRequests(c *fiber.Ctx) error {
	rows, err := database.LocalDB.Query(context.Background(), 
		`SELECT b.id, b.token, b.platform, b.account_id, b.account_name, b.user_id, b.created_at, b.expires_at, u.email, u.name
		 FROM bot_auth_requests b
		 LEFT JOIN users u ON b.user_id = u.id
		 WHERE b.status = 'pending' AND (b.expires_at IS NULL OR b.expires_at > CURRENT_TIMESTAMP)
		 ORDER BY b.created_at DESC`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch requests: " + err.Error()})
	}
	defer rows.Close()

	requests := []fiber.Map{}
	for rows.Next() {
		var id, token, platform, accountID, accountName string
		var createdAt time.Time
		var uid, uemail, uname *string
		var expiresAt *time.Time
		
		err := rows.Scan(&id, &token, &platform, &accountID, &accountName, &uid, &createdAt, &expiresAt, &uemail, &uname)
		if err != nil {
			fmt.Printf("[GET REQUESTS ERROR] Scan failed: %v\n", err)
			continue
		}
		
		userID := ""; if uid != nil { userID = *uid }
		userEmail := ""; if uemail != nil { userEmail = *uemail }
		userName := ""; if uname != nil { userName = *uname }

		requests = append(requests, fiber.Map{
			"id": id,
			"token": token,
			"platform": platform,
			"account_id": accountID,
			"account_name": accountName,
			"user_id": userID,
			"user_email": userEmail,
			"user_name": userName,
			"created_at": createdAt,
			"expires_at": expiresAt,
		})
	}
	return c.JSON(requests)
}

func ApproveBotRequest(c *fiber.Ctx) error {
	var body struct {
		ID   string `json:"id"`
		Role string `json:"role"` // client, vip, admin
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if body.Role == "" {
		body.Role = "client" // Default
	}

	// 1. Get info from request
	var platform, accountID, accountName string
	var userID *string // Can be nil if not linked before submit
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT platform, account_id, account_name, user_id FROM bot_auth_requests WHERE id = $1", body.ID).Scan(&platform, &accountID, &accountName, &userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Request not found"})
	}

	// 2. Add to authorized users
	_, err = database.LocalDB.Exec(context.Background(), 
		"INSERT INTO bot_authorized_users (platform, account_id, account_name, user_id, role) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (platform, account_id) DO UPDATE SET user_id = EXCLUDED.user_id, role = EXCLUDED.role", 
		platform, accountID, accountName, userID, body.Role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to authorize user: " + err.Error()})
	}

	// 3. Level up the user's primary role if they have a linked user_id
	if userID != nil && *userID != "" {
		// Prevent accidental demotion of superadmin/toby
		database.LocalDB.Exec(context.Background(), 
			"UPDATE users SET role = $1 WHERE id = $2 AND role NOT IN ('superadmin', 'toby')", body.Role, *userID)
		
		// Also update platform IDs in users table
		if platform == "discord" {
			database.LocalDB.Exec(context.Background(), "UPDATE users SET discord_id = $1 WHERE id = $2", accountID, *userID)
		} else if platform == "line" {
			database.LocalDB.Exec(context.Background(), "UPDATE users SET line_id = $1 WHERE id = $2", accountID, *userID)
		} else if platform == "telegram" {
			database.LocalDB.Exec(context.Background(), "UPDATE users SET telegram_id = $1 WHERE id = $2", accountID, *userID)
		}
	}

	// 4. Mark request as approved
	database.LocalDB.Exec(context.Background(), "UPDATE bot_auth_requests SET status = 'approved' WHERE id = $1", body.ID)

	return c.JSON(fiber.Map{"success": true})
}

func RejectBotRequest(c *fiber.Ctx) error {
	var body struct {
		ID string `json:"id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	database.LocalDB.Exec(context.Background(), "UPDATE bot_auth_requests SET status = 'rejected' WHERE id = $1", body.ID)
	return c.JSON(fiber.Map{"success": true})
}

func GetAuthorizedBotUsers(c *fiber.Ctx) error {
	rows, err := database.LocalDB.Query(context.Background(), 
		"SELECT id, platform, account_id, account_name, user_id, created_at FROM bot_authorized_users ORDER BY created_at DESC")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch authorized users"})
	}
	defer rows.Close()

	users := []fiber.Map{}
	for rows.Next() {
		var id, platform, accountID, accountName, createdAt string
		var uid *string
		rows.Scan(&id, &platform, &accountID, &accountName, &uid, &createdAt)
		
		userID := ""
		if uid != nil { userID = *uid }

		users = append(users, fiber.Map{
			"id": id,
			"platform": platform,
			"account_id": accountID,
			"account_name": accountName,
			"user_id": userID,
			"created_at": createdAt,
		})
	}
	return c.JSON(users)
}

func DeleteAuthorizedBotUser(c *fiber.Ctx) error {
	executor, _ := c.Locals("user").(*Claims)
	var body struct {
		ID string `json:"id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// 1. Get info before deletion
	var platform, accountID, role string
	var userID *string
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT platform, account_id, user_id, role FROM bot_authorized_users WHERE id = $1", body.ID).Scan(&platform, &accountID, &userID, &role)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Link record not found"})
	}

	// Admin Protection: Only superadmin/toby can delete another admin's bot link
	if role == "admin" && (executor.Role != "superadmin" && executor.Role != "toby") {
		return c.Status(403).JSON(fiber.Map{"error": "Only AdminToby can revoke Admin platform links"})
	}

	// 2. Perform cleanup in users table
	if userID != nil && *userID != "" {
		column := ""
		switch platform {
		case "discord": column = "discord_id"
		case "line": column = "line_id"
		case "telegram": column = "telegram_id"
		}
		if column != "" {
			database.LocalDB.Exec(context.Background(), 
				fmt.Sprintf("UPDATE users SET %s = NULL WHERE id = $1 AND %s = $2", column, column), *userID, accountID)
		}
	}

	// 3. Delete link record
	database.LocalDB.Exec(context.Background(), "DELETE FROM bot_authorized_users WHERE id = $1", body.ID)
	
	return c.JSON(fiber.Map{"success": true})
}

// --- Public APIs ---

func VerifyJoinToken(c *fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var platform, accountName string
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT platform, account_name FROM bot_auth_requests WHERE token = $1 AND status = 'pending'", body.Token).Scan(&platform, &accountName)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Invalid or expired token"})
	}

	return c.JSON(fiber.Map{
		"platform": platform,
		"name": accountName,
	})
}

func LinkBotAccount(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || user == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// 1. Find request (Must be pending and NOT expired)
	var platform, accountID, accountName string
	err := database.LocalDB.QueryRow(context.Background(),
		`SELECT platform, account_id, account_name 
		 FROM bot_auth_requests 
		 WHERE token = $1 AND status = 'pending' 
		 AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`, 
		body.Token).Scan(&platform, &accountID, &accountName)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Invalid or expired verification code (Codes expire in 30 mins)"})
	}

	// 2. Already Bound Check
	column := ""
	switch platform {
	case "discord": column = "discord_id"
	case "telegram": column = "telegram_id"
	case "line": column = "line_id"
	}

	if column != "" {
		var existingEmail, existingRole string
		err = database.LocalDB.QueryRow(context.Background(), 
			fmt.Sprintf("SELECT email, role FROM users WHERE %s = $1 AND id != $2", column), accountID, user.ID).Scan(&existingEmail, &existingRole)
		if err == nil {
			// Found conflict. Check if it's an "Admin Trust" case
			requesterIsAdmin := user.Role == "superadmin" || user.Role == "toby" || user.Role == "admin"
			ownerIsAdmin := existingRole == "superadmin" || existingRole == "toby" || existingRole == "admin"
			
			if !(requesterIsAdmin && ownerIsAdmin) {
				return c.Status(400).JSON(fiber.Map{
					"error": fmt.Sprintf("Platform account already linked to another user (%s). Identity sharing is restricted to Admin accounts.", existingEmail),
				})
			}
		}
	}

	// 3. Mark request as submitted-and-linked
	_, err = database.LocalDB.Exec(context.Background(),
		"UPDATE bot_auth_requests SET user_id = $1 WHERE token = $2", user.ID, body.Token)
	
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to link request"})
	}
	
	return c.JSON(fiber.Map{"success": true, "platform": platform, "name": accountName})
}
