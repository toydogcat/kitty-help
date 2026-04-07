package handlers

import (
"context"
"log"

"github.com/gofiber/fiber/v2"
"github.com/toydogcat/kitty-help/go-server/database"
"github.com/toydogcat/kitty-help/go-server/models"
)

// --- Shelves Handlers ---

func GetShelves(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	rows, err := db.Query(context.Background(), "SELECT id, user_id, name, color, sort_order, created_at FROM desk_shelves WHERE user_id = $1 ORDER BY sort_order ASC", dbUserID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch shelves"})
	}
	defer rows.Close()

	shelves := []models.DeskShelf{}
	for rows.Next() {
		var s models.DeskShelf
		rows.Scan(&s.ID, &s.UserID, &s.Name, &s.Color, &s.SortOrder, &s.CreatedAt)
		shelves = append(shelves, s)
	}
	return c.JSON(shelves)
}

func CreateShelf(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var s models.DeskShelf
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := "INSERT INTO desk_shelves (user_id, name, color, sort_order) VALUES ($1, $2, $3, $4) RETURNING id, created_at"
	err = db.QueryRow(context.Background(), query, dbUserID, s.Name, s.Color, s.SortOrder).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create shelf"})
	}
	s.UserID = dbUserID
	return c.JSON(s)
}

func UpdateShelf(c *fiber.Ctx) error {
	id := c.Params("id")
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var s models.DeskShelf
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := "UPDATE desk_shelves SET name = $1, color = $2, sort_order = $3 WHERE id = $4 AND user_id = $5"
	_, err = db.Exec(context.Background(), query, s.Name, s.Color, s.SortOrder, id, dbUserID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Update failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

func DuplicateShelf(c *fiber.Ctx) error {
	id := c.Params("id")
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	// 1. Get original shelf info
	var name, color string
	err = db.QueryRow(context.Background(), "SELECT name, color FROM desk_shelves WHERE id = $1 AND user_id = $2", id, dbUserID).Scan(&name, &color)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Original shelf not found"})
	}

	// 2. Create new shelf
	var newID string
	err = database.LocalDB.QueryRow(context.Background(),
		"INSERT INTO desk_shelves (user_id, name, color, sort_order) VALUES ($1, $2, $3, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM desk_shelves WHERE user_id = $1)) RETURNING id",
		dbUserID, name+" (Copy)", color).Scan(&newID)
	if err != nil {
		log.Printf("Duplicate shelf insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create duplicate shelf"})
	}

	// 3. Duplicate items from original to new
	_, err = database.LocalDB.Exec(context.Background(),
		"INSERT INTO desk_items (user_id, shelf_id, type, ref_id, sort_order) SELECT user_id, $1, type, ref_id, sort_order FROM desk_items WHERE shelf_id = $2",
		newID, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to duplicate shelf items"})
	}

	return c.JSON(fiber.Map{"success": true, "newId": newID})
}

func DeleteShelf(c *fiber.Ctx) error {
	id := c.Params("id")
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	// Move deleted shelf items to desktop (null shelf_id).
	_, _ = db.Exec(context.Background(), "UPDATE desk_items SET shelf_id = NULL WHERE shelf_id = $1", id)
	
	_, err = db.Exec(context.Background(), "DELETE FROM desk_shelves WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

// --- Desk Items Handlers ---

type DeskItemResponse struct {
	models.DeskItem
	Title   string `json:"title"`
	Content string `json:"content,omitempty"` // For snippets or media notes
	URL     string `json:"url,omitempty"`
	FileID  string `json:"fileId,omitempty"` // For media
	Source  string `json:"source,omitempty"` // For media
}

func GetDeskItems(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	shelfId := c.Query("shelfId") // Can be empty for desktop items

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := `
		SELECT di.id, di.user_id, di.shelf_id, di.type, di.ref_id, di.sort_order, di.created_at,
		COALESCE(
			(SELECT title FROM bookmarks WHERE id::text = di.ref_id::text),
			(SELECT name FROM snippets WHERE id::text = di.ref_id::text),
			(SELECT title FROM media_archives WHERE id::text = di.ref_id::text),
			(SELECT name FROM remark_containers WHERE id::text = di.ref_id::text),
			'Untitled Item'
		) as title,
		COALESCE(
			(SELECT content FROM snippets WHERE id::text = di.ref_id::text),
			(SELECT notes FROM media_archives WHERE id::text = di.ref_id::text),
			(SELECT content FROM remark_containers WHERE id::text = di.ref_id::text),
			''
		) as content,
		COALESCE(
			(SELECT url FROM bookmarks WHERE id::text = di.ref_id::text),
			''
		) as url,
		COALESCE((SELECT file_id FROM media_archives WHERE id::text = di.ref_id::text), '') as file_id,
		COALESCE((SELECT source_platform FROM media_archives WHERE id::text = di.ref_id::text), '') as source
		FROM desk_items di
		WHERE di.user_id = $1
	`
	
	var args []interface{}
	args = append(args, dbUserID)
	
	if shelfId == "null" || shelfId == "" {
		query += " AND di.shelf_id IS NULL"
	} else {
		query += " AND di.shelf_id = $2"
		args = append(args, shelfId)
	}
	query += " ORDER BY di.sort_order ASC"

	rows, err := db.Query(context.Background(), query, args...)
	if err != nil {
		log.Printf("GetDeskItems error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch desk items"})
	}
	defer rows.Close()

	items := []DeskItemResponse{}
	for rows.Next() {
		var it DeskItemResponse
		err := rows.Scan(&it.ID, &it.UserID, &it.ShelfID, &it.Type, &it.RefID, &it.SortOrder, &it.CreatedAt, &it.Title, &it.Content, &it.URL, &it.FileID, &it.Source)
		if err != nil {
			log.Printf("Scan DeskItem error: %v", err)
			continue
		}
		items = append(items, it)
	}
	return c.JSON(items)
}

func AddDeskItem(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var it models.DeskItem
	if err := c.BodyParser(&it); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE LOWER(email) = LOWER($1)", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	// Fix for empty shelfId
	var sId interface{} = it.ShelfID
	if it.ShelfID == nil || *it.ShelfID == "" || *it.ShelfID == "null" {
		sId = nil
	}

	query := "INSERT INTO desk_items (user_id, shelf_id, type, ref_id, sort_order) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at"
	err = db.QueryRow(context.Background(), query, dbUserID, sId, it.Type, it.RefID, it.SortOrder).Scan(&it.ID, &it.CreatedAt)
	if err != nil {
		log.Printf("AddDeskItem SQL error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add item to desk"})
	}
	it.UserID = dbUserID
	return c.JSON(it)
}

func UpdateDeskItem(c *fiber.Ctx) error {
	id := c.Params("id")
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var it models.DeskItem
	if err := c.BodyParser(&it); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Fix for empty shelfId
	var sId interface{} = it.ShelfID
	if it.ShelfID == nil || *it.ShelfID == "" || *it.ShelfID == "null" {
		sId = nil
	}

	query := "UPDATE desk_items SET shelf_id = $1, sort_order = $2 WHERE id = $3"
	_, err = db.Exec(context.Background(), query, sId, it.SortOrder, id)
	if err != nil {
		log.Printf("UpdateDeskItem error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Update failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

func DeleteDeskItem(c *fiber.Ctx) error {
	id := c.Params("id")
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	_, err = db.Exec(context.Background(), "DELETE FROM desk_items WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}
