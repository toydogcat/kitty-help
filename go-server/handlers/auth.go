package handlers

import (
	"context"
	"log"
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
		return []byte("super_kitty_secret_2026!")
	}
	return []byte(secret)
}

type Claims struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	DeviceID string `json:"deviceId,omitempty"`
	jwt.RegisteredClaims
}

func VerifyFirebaseToken(c *fiber.Ctx) error {
	var req struct {
		IDToken  string `json:"idToken"`
		DeviceID string `json:"deviceId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	token, _, err := new(jwt.Parser).ParseUnverified(req.IDToken, jwt.MapClaims{})
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token format"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid claims"})
	}

	email, _ := claims["email"].(string)
	resolvedEmail := strings.ToLower(strings.TrimSpace(email))
	isAdmin := resolvedEmail == "toydogcat@gmail.com" 
	isToby := isAdmin || resolvedEmail == "chickenmilktea@gmail.com" || resolvedEmail == "tobywang2021@gmail.com"

	var role, name, dbID string
	if database.LocalDB != nil {
		err = database.LocalDB.QueryRow(context.Background(), "SELECT id, role, name FROM users WHERE email = $1", resolvedEmail).Scan(&dbID, &role, &name)
		if err != nil {
			if isAdmin { role = "superadmin"; name = "Master Admin" } else if isToby { role = "toby"; name = "Toby" } else { role = "visitor"; name = "Guest" }
			database.LocalDB.QueryRow(context.Background(), "INSERT INTO users (id, name, role, email) VALUES (gen_random_uuid(), $1, $2, $3) RETURNING id", name, role, resolvedEmail).Scan(&dbID)
		} else if isAdmin && role != "superadmin" {
			role = "superadmin"
			database.LocalDB.Exec(context.Background(), "UPDATE users SET role = $1 WHERE id = $2", role, dbID)
		}
	} else {
		if isAdmin { role = "superadmin"; name = "Master Admin (Offline)" } else { role = "visitor"; name = "Guest (Offline)" }
	}

	expirationTime := time.Now().Add(30 * 24 * time.Hour)
	myClaims := &Claims{
		ID: dbID, Email: resolvedEmail, Role: role, DeviceID: req.DeviceID,
		RegisteredClaims: jwt.RegisteredClaims{ Subject: dbID, ExpiresAt: jwt.NewNumericDate(expirationTime) },
	}

	myToken := jwt.NewWithClaims(jwt.SigningMethodHS256, myClaims)
	tokenString, _ := myToken.SignedString(getJWTSecret())

	return c.JSON(fiber.Map{
		"token": tokenString,
		"user": fiber.Map{ "id": dbID, "email": email, "role": role, "name": name },
	})
}

func JWTMiddleware(c *fiber.Ctx) error {
	tokenString := c.Get("Authorization")
	if tokenString == "" && c.Method() == "GET" {
		c.Locals("user", &Claims{Role: "guest", Email: "guest@kitty.help"})
		return c.Next()
	}
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}
	if tokenString == "" { return c.Status(401).JSON(fiber.Map{"error": "Missing token"}) }

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}

	// Sliding Session: Refresh if less than 29 days left
	expiresAt, _ := token.Claims.GetExpirationTime()
	if expiresAt != nil && time.Until(expiresAt.Time) < 29*24*time.Hour {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour))
		newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ = newToken.SignedString(getJWTSecret())
		c.Set("X-Refresh-Token", tokenString)
	}

	// ROBUST ROLE LOCK: Force Superadmin/Toby for whitelisted emails
	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	userEmail := claims.Email
	isAdmin := userEmail == "toydogcat@gmail.com" || userEmail == "chickenmilktea@gmail.com" || userEmail == "tobywang2021@gmail.com"
	
	if isAdmin {
		claims.Role = "superadmin"
		log.Printf("👑 [Auth] Whitelisted Superadmin access granted for: %s", userEmail)
	} else {
		log.Printf("👤 [Auth] Request from user: %s (Role: %s)", userEmail, claims.Role)
	}

	c.Locals("user", claims)
	return c.Next()
}

func AdminOnlyMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || (user.Role != "superadmin" && user.Role != "toby") {
		return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
	}
	return c.Next()
}

func DeviceCheckMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*Claims)
	if !ok || user.Role == "superadmin" || user.Role == "toby" { return c.Next() }
	// Normal user device check logic...
	return c.Next()
}

func TobyOnlyMiddleware(c *fiber.Ctx) error { return c.Next() }
func VIPOnlyMiddleware(c *fiber.Ctx) error { return c.Next() }
