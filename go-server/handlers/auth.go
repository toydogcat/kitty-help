package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/toydogcat/kitty-help/go-server/database"
)

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("super_kitty_secret_2026!") // Fallback matching common env
	}
	return []byte(secret)
}

type AuthRequest struct {
	IDToken  string `json:"idToken"`
	DeviceID string `json:"deviceId"` // UUID from frontend
}

type Claims struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	DeviceID string `json:"deviceId,omitempty"`
	jwt.RegisteredClaims
}

func VerifyFirebaseToken(c *fiber.Ctx) error {
	var req AuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// In a real production app, we should use firebase.google.com/go/v4/auth
	// to properly verify the token. For Phase 1, we will decode it and check the email.
	// NOTE: We MUST verify the signature in Phase 2.
	
	// Temporary simple decode for initial migration:
	token, _, err := new(jwt.Parser).ParseUnverified(req.IDToken, jwt.MapClaims{})
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token format"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid claims"})
	}

	email, _ := claims["email"].(string)
	if email == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Email not found in token"})
	}

	// Resolve actual identity Email for grouping
	resolvedEmail := strings.ToLower(strings.TrimSpace(email))

	// 1. Check user classification
	isAdmin := resolvedEmail == "toydogcat@gmail.com" 
	isToby := isAdmin || resolvedEmail == "chickenmilktea@gmail.com" || resolvedEmail == "tobywang2021@gmail.com"

	fmt.Printf("\n[AUTH DEBUG] Verifying Email: [%s] (isAdmin: %v, isToby: %v)\n", resolvedEmail, isAdmin, isToby)

	var role, name, dbID string
	
	if database.LocalDB == nil {
		if isAdmin {
			role = "superadmin"
			name = "Master Admin (DB Down Offline Mode)"
		} else if isToby {
			role = "toby"
			name = "Toby (DB Down Offline Mode)"
		} else {
			return c.Status(503).JSON(fiber.Map{"error": "Database unavailable"})
		}
	} else {
		// Only check email, since resolvedEmail is always an email string here
		err = database.LocalDB.QueryRow(context.Background(), "SELECT id, role, name FROM users WHERE email = $1", resolvedEmail).Scan(&dbID, &role, &name)
		
		if err != nil {
			dbID = "" 
			if isAdmin {
				role = "superadmin"
				name = "Master Admin"
				err = database.LocalDB.QueryRow(context.Background(), "INSERT INTO users (id, name, role, email) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id", name, role, resolvedEmail).Scan(&dbID)
			} else if isToby {
				role = "toby"
				name = "Toby (Family)"
				err = database.LocalDB.QueryRow(context.Background(), "INSERT INTO users (id, name, role, email) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id", name, role, resolvedEmail).Scan(&dbID)
			} else {
				// 🤖 New Enrollment Case: Create a 'visitor' record for anyone with a Google Account
				role = "visitor"
				// Hard fail-safe for the specific test account during this debugging session
				if email == "mousekingfat@gmail.com" {
					role = "visitor"
					fmt.Printf("[CRITICAL DEBUG] Force-locking %s to VISITOR role.\n", email)
				}
				name = claims["name"].(string)
				if name == "" { name = "New Guest" }
				err = database.LocalDB.QueryRow(context.Background(), "INSERT INTO users (id, name, role, email) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id", name, role, resolvedEmail).Scan(&dbID)
			}
		} else {
			// Update existing user roles if needed (e.g. they were just 'user' before)
			// But PRESERVE roles like "vip" or "manager" unless it's a promotion to superadmin/toby
			if isAdmin && role != "superadmin" {
				role = "superadmin"
				database.LocalDB.Exec(context.Background(), "UPDATE users SET role = $1 WHERE email = $2", role, resolvedEmail)
			} else if isToby && (role != "toby" && role != "superadmin" && role != "vip") {
				// If they are toby group but not superadmin/toby/vip yet, set to toby
				role = "toby"
				database.LocalDB.Exec(context.Background(), "UPDATE users SET role = $1 WHERE email = $2", role, resolvedEmail)
			}
		}

		// 2. Device Registration / Check (Implementation of "設備指紋")
		if req.DeviceID != "" {
			var deviceStatus string
			err = database.LocalDB.QueryRow(context.Background(), "SELECT status FROM devices WHERE id = $1", req.DeviceID).Scan(&deviceStatus)
			if err != nil {
				// Register new device as pending
				deviceStatus = "pending"
				userAgent := c.Get("User-Agent")
				database.LocalDB.Exec(context.Background(), 
					"INSERT INTO devices (id, user_agent, status, user_id) VALUES ($1, $2, $3, $4)",
					req.DeviceID, userAgent, deviceStatus, dbID)
			} else {
				// Update device association if it was empty
				database.LocalDB.Exec(context.Background(), 
					"UPDATE devices SET user_id = $1, last_active = CURRENT_TIMESTAMP WHERE id = $2 AND user_id IS NULL",
					dbID, req.DeviceID)
			}
		}
	}

	fmt.Printf("[AUTH DEBUG] Final assigned identity: ID=%s, Email=%s, Role=%s, Name=%s\n", dbID, resolvedEmail, role, name)

	// 2. Issue our own JWT
	expirationTime := time.Now().Add(24 * 7 * time.Hour) // 1 week
	myClaims := &Claims{
		ID:       dbID,
		Email:    resolvedEmail,
		Role:     role,
		DeviceID: req.DeviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   dbID,
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	myToken := jwt.NewWithClaims(jwt.SigningMethodHS256, myClaims)
	tokenString, err := myToken.SignedString(getJWTSecret())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to issue token"})
	}

	return c.JSON(fiber.Map{
		"token": tokenString,
		"user": fiber.Map{
			"id":    dbID,
			"email": email, // Keep original email for UI display if needed
			"resolved": resolvedEmail,
			"role":  role,
			"name":  name,
		},
	})
}

func JWTMiddleware(c *fiber.Ctx) error {
	tokenString := c.Get("Authorization")
	
	// --- Visitor Mode Check ---
	// If no token, check if this is a GET request and if Visitor Mode is ON
	if tokenString == "" && c.Method() == "GET" {
		if database.LocalDB == nil {
			// Fail-safe: if DB is down, visitor mode is implicitly OFF for safety
			return c.Status(503).JSON(fiber.Map{"error": "Database unavailable"})
		}
		var visitorMode string
		database.LocalDB.QueryRow(context.Background(), "SELECT value FROM settings WHERE key = 'visitor_mode'").Scan(&visitorMode)
		if visitorMode == "true" {
			// Allow access to public GET resources even without token
			// We set a mock user in Locals to avoid nil pointer issues downstream
			c.Locals("user", &Claims{Role: "guest", Email: "guest@kitty.help"})
			return c.Next()
		}
	}

	if tokenString == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Missing token"})
	}

	// Remove "Bearer " if present
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired token"})
	}

	c.Locals("user", claims)
	return c.Next()
}

func DeviceCheckMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || user == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	// White-list: Elevated roles bypass device check for convenience
	role := user.Role
	if role == "superadmin" || role == "admin" || role == "toby" || role == "vip" {
		return c.Next()
	}

	if user.DeviceID == "" {
		return c.Status(403).JSON(fiber.Map{"error": "Device identification required"})
	}

	var status string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT status FROM devices WHERE id = $1", user.DeviceID).Scan(&status)
	if err != nil {
		return c.Status(403).JSON(fiber.Map{"error": "Device not registered"})
	}

	if status != "approved" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Device pending approval",
			"status": status,
			"deviceId": user.DeviceID,
		})
	}

	return c.Next()
}

func AdminOnlyMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || (user.Role != "superadmin" && user.Role != "toby") {
		return c.Status(403).JSON(fiber.Map{"error": "AdminToby access required"})
	}

	// AdminToby / SuperAdmin always bypass device check for administrative purposes
	return c.Next()
}

func TobyOnlyMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || (user.Role != "toby" && user.Role != "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Toby or Admin access required"})
	}
	return c.Next()
}

func VIPOnlyMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || (user.Role != "vip" && user.Role != "toby" && user.Role != "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "VIP access required"})
	}
	return c.Next()
}
