package handlers

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

// GetRemarks returns all containers and staged items for the user
func GetRemarks(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" {
		return c.JSON(fiber.Map{
			"containers": []models.RemarkContainer{},
			"staged":     []models.RemarkItem{},
		})
	}
	db := database.LocalDB

	// 1. Fetch Containers
	containers := []models.RemarkContainer{}
	rows, err := db.Query(context.Background(), 
		"SELECT id, user_id, name, content, is_pinned, created_at, updated_at FROM remark_containers WHERE user_id = $1 ORDER BY is_pinned DESC, created_at DESC", 
		user.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rc models.RemarkContainer
			if err := rows.Scan(&rc.ID, &rc.UserID, &rc.Name, &rc.Content, &rc.IsPinned, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
				fmt.Printf("[DB SCAN ERROR] Containers: %v\n", err)
				continue
			}
			containers = append(containers, rc)
		}
	}

	// 2. Fetch All Items with Log Details
	// (Including those in staging area AND those in containers)
	sql := `
		SELECT 
			ri.id, ri.container_id, ri.log_id, ri.sort_order,
			cl.id, cl.platform, cl.sender_id, cl.sender_name, cl.content, cl.msg_type, cl.media_id, cl.created_at,
			COALESCE(m.media_type, '') as media_type
		FROM remark_items ri
		JOIN chat_logs cl ON ri.log_id::text = cl.id::text
		LEFT JOIN media_archives m ON cl.media_id::text = m.id::text
		WHERE ri.user_id = $1
		ORDER BY ri.sort_order ASC, ri.created_at DESC
	`
	rows, err = db.Query(context.Background(), sql, user.ID)
	if err != nil {
		fmt.Printf("[DB ERROR] Remarks: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "sql": "remarks_query"})
	}
	defer rows.Close()

	staged := []models.RemarkItem{}
	containerItems := make(map[string][]models.RemarkItem)

	for rows.Next() {
		var ri models.RemarkItem
		var cl models.ChatLog
		err := rows.Scan(
			&ri.ID, &ri.ContainerID, &ri.LogID, &ri.SortOrder,
			&cl.ID, &cl.Platform, &cl.SenderID, &cl.SenderName, &cl.Content, &cl.MsgType, &cl.MediaID, &cl.CreatedAt, &cl.MediaType,
		)
		if err != nil { continue }
		ri.Log = &cl

		if ri.ContainerID == nil {
			staged = append(staged, ri)
		} else {
			containerItems[*ri.ContainerID] = append(containerItems[*ri.ContainerID], ri)
		}
	}

	// Map items to containers
	for i := range containers {
		if items, ok := containerItems[containers[i].ID]; ok {
			containers[i].Items = items
		} else {
			containers[i].Items = []models.RemarkItem{}
		}
	}

	return c.JSON(fiber.Map{
		"containers": containers,
		"staged":     staged,
	})
}

func CreateRemark(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	var body struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	var id string
	query := `
		INSERT INTO remark_containers (id, user_id, name, content) 
		VALUES (COALESCE(NULLIF($1, ''), gen_random_uuid()::text), $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET 
			name = EXCLUDED.name, 
			content = EXCLUDED.content,
			updated_at = NOW()
		RETURNING id
	`
	err := database.LocalDB.QueryRow(context.Background(), query, 
		body.ID, user.ID, body.Name, body.Content).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"id": id, "status": "synced"})
}

func UpdateRemark(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	id := c.Params("id")
	var body struct {
		Name     string `json:"name"`
		Content  string `json:"content"`
		IsPinned bool   `json:"isPinned"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	_, err := database.LocalDB.Exec(context.Background(), 
		"UPDATE remark_containers SET name = $1, content = $2, is_pinned = $3, updated_at = NOW() WHERE id = $4 AND user_id = $5", 
		body.Name, body.Content, body.IsPinned, id, user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

func DeleteRemark(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM remark_containers WHERE id = $1 AND user_id = $2", id, user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

// ToggleIntegration adds or removes a log from the items list
func ToggleIntegration(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	var body struct {
		LogID string `json:"logId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Check if exists
	var existingID string
	err := database.LocalDB.QueryRow(context.Background(), 
		"SELECT id FROM remark_items WHERE user_id = $1 AND log_id = $2", user.ID, body.LogID).Scan(&existingID)
	
	if err == nil {
		// Already exists, remove it (Toggle Off)
		database.LocalDB.Exec(context.Background(), "DELETE FROM remark_items WHERE id = $1", existingID)
		return c.JSON(fiber.Map{"status": "removed"})
	}

	// Add to staging area (Toggle On)
	_, err = database.LocalDB.Exec(context.Background(), 
		"INSERT INTO remark_items (user_id, log_id) VALUES ($1, $2)", user.ID, body.LogID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "added"})
}

// MoveRemarkItem moves an item between container and staging
func MoveRemarkItem(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	var body struct {
		ItemID      string  `json:"itemId"`
		ContainerID *string `json:"containerId"` // nil or ID
		SortOrder   int     `json:"sortOrder"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Handle empty string as NULL for UUID column - support virtual IDs
	var cid interface{} = body.ContainerID
	if body.ContainerID != nil && (*body.ContainerID == "" || *body.ContainerID == "root" || *body.ContainerID == "staged" || *body.ContainerID == "null") {
		cid = nil
	}

	res, err := database.LocalDB.Exec(context.Background(), 
		"UPDATE remark_items SET container_id = $1, sort_order = $2 WHERE id::text = $3 AND user_id = $4", 
		cid, body.SortOrder, body.ItemID, user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if res.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found or unauthorized"})
	}
	return c.SendStatus(200)
}

func RemoveRemarkItem(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	id := c.Params("id")
	_, err := database.LocalDB.Exec(context.Background(), "DELETE FROM remark_items WHERE id = $1 AND user_id = $2", id, user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

// AddRemarkItem adds a log directly to a container
func AddRemarkItem(c *fiber.Ctx) error {
	user := c.Locals("user").(*Claims)
	if user.ID == "" { return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"}) }
	var body struct {
		ID          string `json:"id"`
		ContainerId string `json:"containerId"`
		LogId       string `json:"logId"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Handle virtual IDs for the container_id UUID column
	var id string
	var cid interface{} = body.ContainerId
	if body.ContainerId == "" || body.ContainerId == "root" || body.ContainerId == "staged" || body.ContainerId == "null" {
		cid = nil
	}

	query := `
		INSERT INTO remark_items (id, user_id, container_id, log_id) 
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET 
			container_id = EXCLUDED.container_id,
			log_id = EXCLUDED.log_id
		RETURNING id
	`
	err := database.LocalDB.QueryRow(context.Background(), query, 
		body.ID, user.ID, cid, body.LogId).Scan(&id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"id": id, "status": "synced"})
}
