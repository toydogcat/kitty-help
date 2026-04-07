package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
)

type BookcaseItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	StoreID   string    `json:"storeId"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func GetBookcase(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var userId string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userId)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	rows, err := database.LocalDB.Query(context.Background(), 
		"SELECT id, user_id, store_id, title, category, notes, created_at, updated_at FROM bookcase WHERE user_id = $1 ORDER BY updated_at DESC", 
		userId)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	items := []BookcaseItem{}
	for rows.Next() {
		var it BookcaseItem
		err := rows.Scan(&it.ID, &it.UserID, &it.StoreID, &it.Title, &it.Category, &it.Notes, &it.CreatedAt, &it.UpdatedAt)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		items = append(items, it)
	}

	return c.JSON(items)
}

func AddBookToBookcase(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var userId string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userId)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	var req struct {
		StoreID  string `json:"storeId"`
		Title    string `json:"title"`
		Category string `json:"category"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	_, err = database.LocalDB.Exec(context.Background(),
		"INSERT INTO bookcase (user_id, store_id, title, category) VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, store_id) DO UPDATE SET updated_at = now()",
		userId, req.StoreID, req.Title, req.Category)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{"message": "Book added to bookcase"})
}

func UpdateBookNotes(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var userId string
	_ = database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userId)
	id := c.Params("id")

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	_, err := database.LocalDB.Exec(context.Background(),
		"UPDATE bookcase SET notes = $1, updated_at = now() WHERE id = $2 AND user_id = $3",
		req.Notes, id, userId)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Notes updated"})
}

func RemoveBook(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var userId string
	_ = database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userId)
	id := c.Params("id")

	_, err := database.LocalDB.Exec(context.Background(),
		"DELETE FROM bookcase WHERE id = $1 AND user_id = $2",
		id, userId)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Book removed from bookcase"})
}

func GetAvailableBooks(c *fiber.Ctx) error {
	query := c.Query("q", "")
	
	// Filter by document formats: PDF, EPUB, DJVU
	// Case-insensitive search on title/caption
	sqlQuery := `
		SELECT id, file_id, media_type, title, caption, created_at 
		FROM media_archives 
		WHERE (LOWER(media_type) LIKE '%pdf%' OR LOWER(media_type) LIKE '%epub%' OR LOWER(media_type) LIKE '%djvu%')
	`
	params := []interface{}{}
	if query != "" {
		sqlQuery += " AND (LOWER(title) LIKE $1 OR LOWER(caption) LIKE $1)"
		params = append(params, "%"+strings.ToLower(query)+"%")
	}
	sqlQuery += " ORDER BY created_at DESC LIMIT 50"

	rows, err := database.LocalDB.Query(context.Background(), sqlQuery, params...)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	type AvailableBook struct {
		ID        string    `json:"id"`
		FileID    string    `json:"fileId"`
		MediaType string    `json:"mediaType"`
		Title     string    `json:"title"`
		Caption   string    `json:"caption"`
		CreatedAt time.Time `json:"createdAt"`
	}

	results := []AvailableBook{}
	for rows.Next() {
		var b AvailableBook
		var title, caption *string
		err := rows.Scan(&b.ID, &b.FileID, &b.MediaType, &title, &caption, &b.CreatedAt)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if title != nil { b.Title = *title }
		if caption != nil { b.Caption = *caption }
		results = append(results, b)
	}

	return c.JSON(results)
}
