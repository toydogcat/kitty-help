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
	Folder    string    `json:"folder"`
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
		"SELECT id, user_id, store_id, title, category, notes, folder, created_at, updated_at FROM bookcase WHERE user_id = $1 ORDER BY updated_at DESC", 
		userId)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	items := []BookcaseItem{}
	for rows.Next() {
		var it BookcaseItem
		err := rows.Scan(&it.ID, &it.UserID, &it.StoreID, &it.Title, &it.Category, &it.Notes, &it.Folder, &it.CreatedAt, &it.UpdatedAt)
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
	// Expanded detection to check both media_type and filenames/captions
	// Using DISTINCT ON to avoid double-showing the same unique file
	sqlQuery := `
		SELECT id, file_id, media_type, title, caption, created_at 
		FROM (
			SELECT DISTINCT ON (file_id) id, file_id, media_type, title, caption, created_at
			FROM media_archives 
			WHERE (
				LOWER(media_type) LIKE '%pdf%' OR LOWER(media_type) LIKE '%epub%' OR LOWER(media_type) LIKE '%djvu%' OR
				LOWER(media_type) = 'document' OR
				LOWER(title) LIKE '%.pdf%' OR LOWER(title) LIKE '%.epub%' OR LOWER(title) LIKE '%.djvu%' OR
				LOWER(caption) LIKE '%.pdf%' OR LOWER(caption) LIKE '%.epub%' OR LOWER(caption) LIKE '%.djvu%'
			)
			ORDER BY file_id, created_at DESC
		) sub
		WHERE 1=1
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

func GetBookNotes(c *fiber.Ctx) error {
	bookID := c.Params("id")
	rows, err := database.LocalDB.Query(context.Background(),
		"SELECT id, book_id, title, content, note_type, created_at, updated_at FROM bookcase_notes WHERE book_id = $1 ORDER BY created_at ASC",
		bookID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	notes := []fiber.Map{}
	for rows.Next() {
		var id, bookID, title, content, noteType string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &bookID, &title, &content, &noteType, &createdAt, &updatedAt); err != nil {
			continue
		}
		notes = append(notes, fiber.Map{
			"id":        id,
			"bookId":    bookID,
			"title":     title,
			"content":   content,
			"noteType":  noteType,
			"createdAt": createdAt,
			"updatedAt": updatedAt,
		})
	}
	return c.JSON(notes)
}

func AddBookNote(c *fiber.Ctx) error {
	bookID := c.Params("id")
	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		NoteType string `json:"noteType"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.NoteType == "" { req.NoteType = "markdown" }

	var id string
	err := database.LocalDB.QueryRow(context.Background(),
		"INSERT INTO bookcase_notes (book_id, title, content, note_type) VALUES ($1, $2, $3, $4) RETURNING id",
		bookID, req.Title, req.Content, req.NoteType).Scan(&id)
	
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success", "id": id})
}

func UpdateBookNote(c *fiber.Ctx) error {
	noteID := c.Params("note_id")
	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		NoteType string `json:"noteType"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	_, err := database.LocalDB.Exec(context.Background(),
		"UPDATE bookcase_notes SET title = $1, content = $2, note_type = $3, updated_at = now() WHERE id = $4",
		req.Title, req.Content, req.NoteType, noteID)
	
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func RemoveBookNote(c *fiber.Ctx) error {
	noteID := c.Params("note_id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM bookcase_notes WHERE id = $1", noteID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func UpdateBookFolder(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	var userId string
	_ = database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&userId)
	
	bookID := c.Params("id")
	var req struct {
		Folder string `json:"folder"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	_, err := database.LocalDB.Exec(context.Background(),
		"UPDATE bookcase SET folder = $1, updated_at = now() WHERE id = $2 AND user_id = $3",
		req.Folder, bookID, userId)
	
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success"})
}
