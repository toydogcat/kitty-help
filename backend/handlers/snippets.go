package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func GetSnippets(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	dbUserID := userClaims.ID
	if dbUserID == "" {
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

	query = "SELECT id, user_id, parent_id, name, content, is_folder, sort_order, created_at FROM snippets WHERE user_id = $1"
	if all != "true" {
		if parentId == "root" || parentId == "" {
			query += " AND parent_id IS NULL"
		} else {
			query += " AND parent_id = $2"
			args = append(args, parentId)
		}
	}
	query += " ORDER BY is_folder DESC, sort_order ASC, name ASC"

	rows, err := database.LocalDB.Query(context.Background(), query, args...)
	if err != nil {
		log.Printf("[SNIPPETS ERROR] Query execution failed: %v | UserID: %s", err, dbUserID)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch snippets: " + err.Error()})
	}
	defer rows.Close()

	snippets := []models.Snippet{}
	for rows.Next() {
		var s models.Snippet
		err := rows.Scan(&s.ID, &s.UserID, &s.ParentID, &s.Name, &s.Content, &s.IsFolder, &s.SortOrder, &s.CreatedAt)
		if err != nil {
			log.Printf("[SNIPPETS ERROR] Scan failed: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to scan snippets row: " + err.Error()})
		}
		snippets = append(snippets, s)
	}

	return c.JSON(snippets)
}

func CreateSnippet(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var s models.Snippet
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	dbUserID := userClaims.ID
	if dbUserID == "" {
		err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
		}
	}
	s.UserID = dbUserID

	if s.ParentID != nil && (*s.ParentID == "root" || *s.ParentID == "") {
		s.ParentID = nil
	}

	if s.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	// Insert into Local PG with UPSERT support for EverSync
	query := `
		INSERT INTO snippets (id, user_id, parent_id, name, content, is_folder, sort_order) 
		VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()::text), $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			content = EXCLUDED.content,
			parent_id = EXCLUDED.parent_id,
			sort_order = EXCLUDED.sort_order
		RETURNING id, created_at
	`
	err := database.LocalDB.QueryRow(context.Background(), query, 
		s.ID, s.UserID, s.ParentID, s.Name, s.Content, s.IsFolder, s.SortOrder).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		log.Printf("Insert snippet failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create snippet"})
	}

	return c.JSON(s)
}

func UpdateSnippet(c *fiber.Ctx) error {
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	id := c.Params("id")
	var s models.Snippet
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if s.ParentID != nil && (*s.ParentID == "root" || *s.ParentID == "") {
		s.ParentID = nil
	}

	// Support updating parentId (moving between folders) and sortOrder (manual positioning)
	query := `UPDATE snippets SET name = $1, content = $2, parent_id = $3, sort_order = $4, updated_at = NOW() 
	          WHERE id = $5 RETURNING id, user_id, parent_id, name, content, is_folder, sort_order, created_at`
	err = db.QueryRow(context.Background(), query, s.Name, s.Content, s.ParentID, s.SortOrder, id).
		Scan(&s.ID, &s.UserID, &s.ParentID, &s.Name, &s.Content, &s.IsFolder, &s.SortOrder, &s.CreatedAt)
	if err != nil {
		log.Printf("Update snippet failed: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Update failed"})
	}

	return c.JSON(s)
}

func DeleteSnippet(c *fiber.Ctx) error {
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM snippets WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}
