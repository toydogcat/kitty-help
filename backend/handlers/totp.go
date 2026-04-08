package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"github.com/toydogcat/kitty-help/go-server/database"
)

// TOTPStatusResponse for frontend guidance
type TOTPStatusResponse struct {
	Enabled        bool  `json:"enabled"`
	Verified       bool  `json:"verified"`
	VerifiedUntil  int64 `json:"verifiedUntil"`
}

func GetTOTPStatus(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "DB offline"}) }

	var enabled bool
	err := db.QueryRow(context.Background(), "SELECT totp_enabled FROM users WHERE id = $1", userClaims.ID).Scan(&enabled)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	verified := false
	if userClaims.TOTPVerifiedAt > 0 {
		// Window: 30 minutes
		if time.Since(time.Unix(userClaims.TOTPVerifiedAt, 0)) < 30*time.Minute {
			verified = true
		}
	}

	return c.JSON(TOTPStatusResponse{
		Enabled:       enabled,
		Verified:      verified,
		VerifiedUntil: userClaims.TOTPVerifiedAt + (30 * 60),
	})
}

func SetupTOTP(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "DB offline"}) }

	// Generate key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Kitty-Help",
		AccountName: userClaims.Email,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate TOTP secret"})
	}

	// Save secret (but don't enable yet)
	_, err = db.Exec(context.Background(), "UPDATE users SET totp_secret = $1 WHERE id = $2", key.Secret(), userClaims.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save secret"})
	}

	return c.JSON(fiber.Map{
		"secret": key.Secret(),
		"url":    key.URL(),
	})
}

func VerifyAndEnableTOTP(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "DB offline"}) }

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var secret string
	err := db.QueryRow(context.Background(), "SELECT totp_secret FROM users WHERE id = $1", userClaims.ID).Scan(&secret)
	if err != nil || secret == "" {
		return c.Status(400).JSON(fiber.Map{"error": "TOTP not set up"})
	}

	valid := totp.Validate(req.Code, secret)
	if !valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid TOTP code"})
	}

	// Enable
	_, err = db.Exec(context.Background(), "UPDATE users SET totp_enabled = true WHERE id = $1", userClaims.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to enable TOTP"})
	}

	// Upgrade JWT immediately
	userClaims.TOTPVerifiedAt = time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaims)
	tokenString, _ := token.SignedString(getJWTSecret())
	c.Set("X-Refresh-Token", tokenString)

	return c.JSON(fiber.Map{"status": "success", "token": tokenString})
}

func AuthenticateTOTP(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "DB offline"}) }

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var secret string
	var enabled bool
	err := db.QueryRow(context.Background(), "SELECT totp_secret, totp_enabled FROM users WHERE id = $1", userClaims.ID).Scan(&secret, &enabled)
	if err != nil || !enabled {
		return c.Status(403).JSON(fiber.Map{"error": "2FA is required but not enabled"})
	}

	valid := totp.Validate(req.Code, secret)
	if !valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid TOTP code"})
	}

	// Upgrade JWT
	userClaims.TOTPVerifiedAt = time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaims)
	tokenString, _ := token.SignedString(getJWTSecret())
	c.Set("X-Refresh-Token", tokenString)

	return c.JSON(fiber.Map{"status": "success", "token": tokenString})
}

// TOTPCheckMiddleware enforces 2FA for specific routes
func TOTPCheckMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }

	// 1. Check if 2FA is enabled for this user
	var enabled bool
	err := database.LocalDB.QueryRow(context.Background(), "SELECT totp_enabled FROM users WHERE id = $1", user.ID).Scan(&enabled)
	
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	if !enabled {
		return c.Status(403).JSON(fiber.Map{"error": "2FA_REQUIRED_SETUP", "message": "Please set up Google Authenticator first"})
	}

	// 2. Check if verified within 30 minutes
	if user.TOTPVerifiedAt == 0 || time.Since(time.Unix(user.TOTPVerifiedAt, 0)) > 30*time.Minute {
		return c.Status(403).JSON(fiber.Map{"error": "2FA_REQUIRED_VERIFY", "message": "2FA verification required (expired or not verified)"})
	}

	return c.Next()
}
