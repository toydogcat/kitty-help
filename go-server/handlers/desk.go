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
	userClaims := c.Locals("user").(*Claims)
	var dbUserID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	rows, err := database.LocalDB.Query(context.Background(), "SELECT id, user_id, name, color, sort_order, created_at FROM desk_shelves WHERE user_id = $1 ORDER BY sort_order ASC", dbUserID)
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
	userClaims := c.Locals("user").(*Claims)
	var s models.DeskShelf
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var dbUserID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := "INSERT INTO desk_shelves (user_id, name, color, sort_order) VALUES ($1, $2, $3, $4) RETURNING id, created_at"
	err = database.LocalDB.QueryRow(context.Background(), query, dbUserID, s.Name, s.Color, s.SortOrder).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create shelf"})
	}
	s.UserID = dbUserID
	return c.JSON(s)
}

func UpdateShelf(c *fiber.Ctx) error {
	id := c.Params("id")
	var s models.DeskShelf
	if err := c.BodyParser(&s); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	query := "UPDATE desk_shelves SET name = $1, color = $2, sort_order = $3 WHERE id = $4"
	_, err := database.LocalDB.Exec(context.Background(), query, s.Name, s.Color, s.SortOrder, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Update failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

func DeleteShelf(c *fiber.Ctx) error {
	id := c.Params("id")
	// Note: We don't delete desk_items automatically here if we want them to stay as orphans on desktop,
// but according to user "Shelf is 1 layer", let's move deleted shelf items to desktop (null shelf_id).
	_, _ = database.LocalDB.Exec(context.Background(), "UPDATE desk_items SET shelf_id = NULL WHERE shelf_id = $1", id)
	
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM desk_shelves WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

// --- Desk Items Handlers ---

type DeskItemResponse struct {
	models.DeskItem
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

func GetDeskItems(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	shelfId := c.Query("shelfId") // Can be empty for desktop items

	var dbUserID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := `
		SELECT di.id, di.user_id, di.shelf_id, di.type, di.ref_id, di.sort_order, di.created_at,
		CASE 
			WHEN di.type = 'bookmark' THEN (SELECT title FROM bookmarks WHERE id = di.ref_id)
			WHEN di.type = 'snippet' THEN (SELECT name FROM snippets WHERE id = di.ref_id)
			WHEN di.type = 'media' THEN (SELECT title FROM media_archives WHERE id = di.ref_id)
			ELSE 'Unknown Item'
		END as title,
		CASE 
			WHEN di.type = 'bookmark' THEN (SELECT url FROM bookmarks WHERE id = di.ref_id)
			ELSE ''
		END as url
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

	rows, err := database.LocalDB.Query(context.Background(), query, args...)
	if err != nil {
		log.Printf("GetDeskItems error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch desk items"})
	}
	defer rows.Close()

	items := []DeskItemResponse{}
	for rows.Next() {
		var it DeskItemResponse
		err := rows.Scan(&it.ID, &it.UserID, &it.ShelfID, &it.Type, &it.RefID, &it.SortOrder, &it.CreatedAt, &it.Title, &it.URL)
		if err != nil {
			log.Printf("Scan DeskItem error: %v", err)
			continue
		}
		items = append(items, it)
	}
	return c.JSON(items)
}

func AddDeskItem(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var it models.DeskItem
	if err := c.BodyParser(&it); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var dbUserID string
	err := database.LocalDB.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	query := "INSERT INTO desk_items (user_id, shelf_id, type, ref_id, sort_order) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at"
	err = database.LocalDB.QueryRow(context.Background(), query, dbUserID, it.ShelfID, it.Type, it.RefID, it.SortOrder).Scan(&it.ID, &it.CreatedAt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add item to desk"})
	}
	it.UserID = dbUserID
	return c.JSON(it)
}

func UpdateDeskItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var it models.DeskItem
	if err := c.BodyParser(&it); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	query := "UPDATE desk_items SET shelf_id = $1, sort_order = $2 WHERE id = $3"
	_, err := database.LocalDB.Exec(context.Background(), query, it.ShelfID, it.SortOrder, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Update failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}

func DeleteDeskItem(c *fiber.Ctx) error {
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM desk_items WHERE id = $1", id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Delete failed"})
	}
	return c.JSON(fiber.Map{"success": true})
}
