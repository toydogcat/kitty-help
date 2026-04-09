package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

// Bookmark struct moved to models package

func GetBookmarks(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	dbUserID := userClaims.ID
	if dbUserID == "" {
		// Fallback for system tokens or incomplete claims
		err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
		}
	}

	parentId := c.Query("parentId")
	all := c.Query("all")

	var query string
	var args []interface{}
	args = append(args, dbUserID)

	if all == "true" {
		query = "SELECT id, user_id, parent_id, title, url, category, icon_url, password_id, is_folder, sort_order, created_at FROM bookmarks WHERE user_id = $1 ORDER BY sort_order ASC, created_at DESC"
	} else if parentId == "root" || parentId == "" {
		query = "SELECT id, user_id, parent_id, title, url, category, icon_url, password_id, is_folder, sort_order, created_at FROM bookmarks WHERE user_id = $1 AND parent_id IS NULL ORDER BY sort_order ASC, created_at DESC"
	} else {
		query = "SELECT id, user_id, parent_id, title, url, category, icon_url, password_id, is_folder, sort_order, created_at FROM bookmarks WHERE user_id = $1 AND parent_id = $2 ORDER BY sort_order ASC, created_at DESC"
		args = append(args, parentId)
	}

	rows, err := database.LocalDB.Query(context.Background(), query, args...)
	if err != nil {
		log.Printf("Query bookmarks failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch bookmarks"})
	}
	defer rows.Close()

	bookmarks := []models.Bookmark{}
	for rows.Next() {
		var b models.Bookmark
		err := rows.Scan(&b.ID, &b.UserID, &b.ParentID, &b.Title, &b.URL, &b.Category, &b.IconURL, &b.PasswordID, &b.IsFolder, &b.SortOrder, &b.CreatedAt)
		if err != nil {
			log.Printf("Scan bookmark failed: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to scan bookmarks"})
		}
		bookmarks = append(bookmarks, b)
	}

	return c.JSON(bookmarks)
}

func CreateBookmark(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var b models.Bookmark
	if err := c.BodyParser(&b); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	dbUserID := userClaims.ID
	if dbUserID == "" {
		err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
		}
	}
	b.UserID = dbUserID

	if b.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Title is required"})
	}
	// Folders don't need URL, but regular bookmarks do
	if !b.IsFolder && (b.URL == nil || *b.URL == "") {
		return c.Status(400).JSON(fiber.Map{"error": "URL is required for standard bookmarks"})
	}

	if b.Category == "" {
		b.Category = "uncategorized"
	}

	// Internal validation for virtual IDs to NULL (Postgres UUID requirement)
	if b.ParentID != nil && (*b.ParentID == "" || *b.ParentID == "root") {
		b.ParentID = nil
	}
	if b.PasswordID != nil && (*b.PasswordID == "" || *b.PasswordID == "root") {
		b.PasswordID = nil
	}

	query := `
		INSERT INTO bookmarks (id, user_id, parent_id, title, url, category, icon_url, password_id, is_folder, sort_order) 
		VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()::text), $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			url = EXCLUDED.url,
			category = EXCLUDED.category,
			parent_id = EXCLUDED.parent_id,
			sort_order = EXCLUDED.sort_order,
			updated_at = NOW()
		RETURNING id, created_at
	`
	err = db.QueryRow(context.Background(), query, 
		b.ID, b.UserID, b.ParentID, b.Title, b.URL, b.Category, b.IconURL, b.PasswordID, b.IsFolder, b.SortOrder).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		log.Printf("Insert bookmark failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create bookmark"})
	}

	return c.JSON(b)
}

func UpdateBookmark(c *fiber.Ctx) error {
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	id := c.Params("id")
	var b models.Bookmark
	if err := c.BodyParser(&b); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Prepare dynamic update - handle virtual IDs
	if b.ParentID != nil && (*b.ParentID == "" || *b.ParentID == "root") {
		b.ParentID = nil
	}

	query := "UPDATE bookmarks SET parent_id = $1, title = $2, url = $3, category = $4, sort_order = $5 WHERE id = $6"
	_, err = db.Exec(context.Background(), query, b.ParentID, b.Title, b.URL, b.Category, b.SortOrder, id)
	if err != nil {
		log.Printf("Update bookmark failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update bookmark"})
	}

	return c.JSON(fiber.Map{"success": true})
}

func DeleteBookmark(c *fiber.Ctx) error {
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM bookmarks WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}
