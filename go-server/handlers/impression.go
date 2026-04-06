package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func GetRandomImpressionNodeID(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User profile not found"}) }

	var nodeID string
	err = db.QueryRow(context.Background(), 
		"SELECT id::TEXT FROM impression_nodes WHERE user_id = $1 ORDER BY RANDOM() LIMIT 1", 
		dbUserID).Scan(&nodeID)
	
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "No nodes found"}) }
	return c.JSON(fiber.Map{"id": nodeID})
}

func GetImpressionTemp(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	
	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Database not connected"})
	}

	// 0. FORCE INITIALIZE & MIGRATE for Admin Toby on NUC DB
	if userClaims.Email == "toby@family.local" {
		// Step A: Wipe this discord_id from ANYONE ELSE who might be sitting on it
		db.Exec(context.Background(), "UPDATE users SET discord_id = NULL WHERE discord_id = '840468194456371211' AND email != $1", userClaims.Email)
		
		// Step B: Upsert Toby with the correct info
		_, err := db.Exec(context.Background(), 
			"INSERT INTO users (id, name, email, role, discord_id) VALUES ('82507694-4205-49d4-8099-9e18ba997581', 'Master Admin', $1, 'superadmin', '840468194456371211') ON CONFLICT (email) DO UPDATE SET discord_id = '840468194456371211', role = 'superadmin'", 
			userClaims.Email)
		
		if err != nil {
			log.Printf("[DB DEBUG] Forced Toby Migration failed: %v", err)
		} else {
			log.Printf("[DB DEBUG] Forced Toby Migration SUCCESS for identity: %s", userClaims.Email)
		}
	}

	// 1. Resolve System User ID from email
	var dbUserID string
	query := `SELECT id FROM users WHERE email = $1 LIMIT 1`
	err := db.QueryRow(context.Background(), query, userClaims.Email).Scan(&dbUserID)
	if err != nil {
		log.Printf("⚠️ No UserID found for identity %s (Error: %v)", userClaims.Email, err)
		return c.JSON([]fiber.Map{}) 
	}

	// 2. Fetch recent photos/videos from ALL platforms linked to this system user
	sql := `SELECT m.id, m.file_id, m.media_type, m.title, m.caption, m.notes, m.created_at, m.source_platform
	        FROM media_archives m
	        JOIN bot_authorized_users b ON m.sender_id = b.account_id AND m.source_platform = b.platform
	        WHERE b.user_id = $1 
	        AND (m.media_type = 'photo' OR m.media_type = 'image' OR m.media_type = 'video')
	        AND m.created_at > (CURRENT_TIMESTAMP - INTERVAL '7 days')
	        ORDER BY m.created_at DESC 
	        LIMIT 50`
	
	rows, err := db.Query(context.Background(), sql, dbUserID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, fileID, mediaType, sourcePlatform string
		var createdAt time.Time
		var title, caption, notes *string
		err := rows.Scan(&id, &fileID, &mediaType, &title, &caption, &notes, &createdAt, &sourcePlatform)
		if err != nil {
			continue
		}
		
		// Get Base URL for absolute images
		baseURL := os.Getenv("VITE_API_URL")
		if baseURL == "" {
			baseURL = c.BaseURL()
		}

		items = append(items, fiber.Map{
			"id":         id,
			"file_id":    fileID,
			"title":      title,
			"caption":    caption,
			"notes":      notes,
			"created_at": createdAt,
			"source":     sourcePlatform,
			"imageUrl":   baseURL + "/api/storehouse/file/" + id,
		})
	}

	return c.JSON(items)
}

func CreateImpressionNode(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var n models.ImpressionNode
	if err := c.BodyParser(&n); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	// Resolve internal DB User ID from email
	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		log.Printf("❌ Identity Fail: email %s not found in users table", userClaims.Email)
		return c.Status(404).JSON(fiber.Map{"error": "User profile not found"})
	}

	// 1. Check if a node with this mediaId already exists for this user (Upsert/Overwrite)
	if n.MediaID != nil && *n.MediaID != "" {
		var existingID string
		err = db.QueryRow(context.Background(), 
			"SELECT id::TEXT FROM impression_nodes WHERE user_id = $1 AND media_id = $2", 
			dbUserID, *n.MediaID).Scan(&existingID)
		
		if err == nil {
			updateQuery := `UPDATE impression_nodes 
			                SET title = $1, content = $2, node_type = $3 
			                WHERE id = $4 
			                RETURNING id, linked_snippet_id, created_at`
			
			err = db.QueryRow(context.Background(), updateQuery, 
				n.Title, n.Content, n.NodeType, existingID).Scan(&n.ID, &n.LinkedSnippetID, &n.CreatedAt)
			
			if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
			n.UserID = dbUserID
			return c.JSON(n)
		}
	}

	// 2. Normal INSERT
	query := `INSERT INTO impression_nodes (user_id, media_id, title, content, node_type) 
	          VALUES ($1, $2, $3, $4, $5) 
	          RETURNING id, linked_snippet_id, created_at`
	
	err = db.QueryRow(context.Background(), query, 
		dbUserID, n.MediaID, n.Title, n.Content, n.NodeType).Scan(&n.ID, &n.LinkedSnippetID, &n.CreatedAt)
	
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	n.UserID = dbUserID
	return c.JSON(n)
}

func GetImpressionGraph(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	centerID := c.Query("centerId")

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	// Resolve internal DB User ID from email
	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// 1. If centerID is empty, find the latest node
	isFocusMode := centerID != ""
	if centerID == "" {
		err := db.QueryRow(context.Background(), 
			"SELECT id FROM impression_nodes WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1", 
			dbUserID).Scan(&centerID)
		if err != nil {
			return c.JSON(models.GraphResponse{Nodes: []models.ImpressionNode{}, Edges: []models.ImpressionEdge{}})
		}
	}

	// 2. Fetch Nodes using Recursive CTE (2 Degrees)
	nodesQuery := `
	WITH RECURSIVE graph_nodes AS (
		SELECT id, user_id, media_id, linked_snippet_id, title, content, node_type, created_at, 0 as depth
		FROM impression_nodes
		WHERE id = $1 AND user_id = $2
		UNION
		SELECT n.id, n.user_id, n.media_id, n.linked_snippet_id, n.title, n.content, n.node_type, n.created_at, gn.depth + 1
		FROM graph_nodes gn
		JOIN impression_edges e ON (e.source_id = gn.id OR e.target_id = gn.id)
		JOIN impression_nodes n ON (n.id = CASE WHEN e.source_id = gn.id THEN e.target_id ELSE e.source_id END)
		WHERE gn.depth < 2 AND n.user_id = $2
	),
	recent_nodes AS (
		SELECT id, user_id, media_id, linked_snippet_id, title, content, node_type, created_at, 99 as depth
		FROM impression_nodes
		WHERE user_id = $2 AND NOT $3
		ORDER BY created_at DESC
		LIMIT 50
	),
	all_nodes AS (
		SELECT * FROM graph_nodes
		UNION
		SELECT * FROM recent_nodes
	)
	SELECT DISTINCT 
		an.id::TEXT, 
		an.user_id::TEXT, 
		an.media_id::TEXT, 
		an.linked_snippet_id::TEXT,
		an.title, 
		an.content, 
		an.node_type, 
		an.created_at, 
		m.file_id, 
		m.source_platform
	FROM all_nodes an
	LEFT JOIN media_archives m ON an.media_id = m.id
	`

	rows, err := db.Query(context.Background(), nodesQuery, centerID, dbUserID, isFocusMode)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	nodeMap := make(map[string]models.ImpressionNode)
	nodesList := []models.ImpressionNode{}
	for rows.Next() {
		var n models.ImpressionNode
		err := rows.Scan(&n.ID, &n.UserID, &n.MediaID, &n.LinkedSnippetID, &n.Title, &n.Content, &n.NodeType, &n.CreatedAt, &n.FileID, &n.SourcePlatform)
		if err == nil {
			if n.FileID != nil && *n.FileID != "" {
				baseURL := os.Getenv("VITE_API_URL")
				if baseURL == "" { baseURL = c.BaseURL() }
				n.ImageURL = baseURL + "/api/storehouse/file/" + *n.FileID
				if n.SourcePlatform != nil {
					n.ImageURL += "?platform=" + *n.SourcePlatform
				}
			}
			nodesList = append(nodesList, n)
			nodeMap[n.ID] = n
		}
	}

	// 3. Fetch all edges between these nodes
	edgesQuery := "SELECT id::TEXT, user_id::TEXT, source_id::TEXT, target_id::TEXT, label, created_at FROM impression_edges WHERE user_id = $1"
	rowsE, err := db.Query(context.Background(), edgesQuery, dbUserID)
	edges := []models.ImpressionEdge{}
	if err != nil {
		log.Printf("⚠️ GetGraph SQL Warning: %v", err)
	} else {
		defer rowsE.Close()
		for rowsE.Next() {
			var e models.ImpressionEdge
			err := rowsE.Scan(&e.ID, &e.UserID, &e.SourceID, &e.TargetID, &e.Label, &e.CreatedAt)
			if err == nil {
				if _, sExists := nodeMap[e.SourceID]; sExists {
					if _, tExists := nodeMap[e.TargetID]; tExists {
						edges = append(edges, e)
					}
				}
			}
		}
	}

	return c.JSON(models.GraphResponse{Nodes: nodesList, Edges: edges})
}

func CreateImpressionLink(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var e models.ImpressionEdge
	if err := c.BodyParser(&e); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	var exists bool
	db.QueryRow(context.Background(), 
		"SELECT EXISTS(SELECT 1 FROM impression_edges WHERE source_id = $1 AND target_id = $2 AND label = $3 AND user_id = $4)", 
		e.SourceID, e.TargetID, e.Label, dbUserID).Scan(&exists)

	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Relationship already exists"})
	}

	query := `INSERT INTO impression_edges (user_id, source_id, target_id, label) 
	          VALUES ($1, $2, $3, $4) 
	          RETURNING id, created_at`
	
	err = db.QueryRow(context.Background(), query, 
		dbUserID, e.SourceID, e.TargetID, e.Label).Scan(&e.ID, &e.CreatedAt)
	
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	e.UserID = dbUserID
	return c.JSON(e)
}

func DeleteImpressionNode(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	_, err = db.Exec(context.Background(), "DELETE FROM impression_nodes WHERE id = $1 AND user_id = $2", id, dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	return c.JSON(fiber.Map{"status": "deleted"})
}

func SearchImpressionNodes(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	q := c.Query("q")

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	nodesQuery := `
	SELECT 
		n.id::TEXT, 
		n.title, 
		n.node_type, 
		m.file_id, 
		m.source_platform
	FROM impression_nodes n
	LEFT JOIN media_archives m ON n.media_id = m.id
	WHERE n.user_id = $1 AND n.title ILIKE $2
	LIMIT 10
	`

	rows, err := db.Query(context.Background(), nodesQuery, dbUserID, "%"+q+"%")
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	nodes := []fiber.Map{}
	for rows.Next() {
		var id, title, nodeType string
		var fileID, sourcePlatform *string
		if err := rows.Scan(&id, &title, &nodeType, &fileID, &sourcePlatform); err == nil {
			nodeMap := fiber.Map{
				"id":       id,
				"title":    title,
				"nodeType": nodeType,
			}
			if fileID != nil && sourcePlatform != nil {
				// Use consistent URL format with the rest of the app
				nodeMap["imageUrl"] = "/api/storehouse/file/" + *fileID + "?platform=" + *sourcePlatform + "&t=" + fmt.Sprint(time.Now().UnixMilli())
			}
			nodes = append(nodes, nodeMap)
		}
	}

	log.Printf("🔍 DB Search: q=[%s] user=[%s] count=%d", q, dbUserID, len(nodes))
	return c.JSON(nodes)
}

func UpdateImpressionNode(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)
	var n models.ImpressionNode
	if err := c.BodyParser(&n); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	query := `UPDATE impression_nodes 
	          SET title = $1, content = $2, node_type = $3 
	          WHERE id = $4 AND user_id = $5 
	          RETURNING id, linked_snippet_id, created_at`
	
	err = db.QueryRow(context.Background(), query, 
		n.Title, n.Content, n.NodeType, id, dbUserID).Scan(&n.ID, &n.LinkedSnippetID, &n.CreatedAt)
	
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	n.UserID = dbUserID
	return c.JSON(n)
}

func UpdateImpressionEdge(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)
	var e models.ImpressionEdge
	if err := c.BodyParser(&e); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	query := `UPDATE impression_edges SET label = $1 WHERE id = $2 AND user_id = $3 RETURNING id, created_at`
	err = db.QueryRow(context.Background(), query, e.Label, id, dbUserID).Scan(&e.ID, &e.CreatedAt)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	e.UserID = dbUserID
	return c.JSON(e)
}

func DeleteImpressionEdge(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	_, err = db.Exec(context.Background(), "DELETE FROM impression_edges WHERE id = $1 AND user_id = $2", id, dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	return c.SendStatus(204)
}

func ExportImpressionGraph(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	rows, err := db.Query(context.Background(), "SELECT id, media_id, title, content, node_type, created_at FROM impression_nodes WHERE user_id = $1", dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	nodes := []models.ImpressionNode{}
	for rows.Next() {
		var n models.ImpressionNode
		if err := rows.Scan(&n.ID, &n.MediaID, &n.Title, &n.Content, &n.NodeType, &n.CreatedAt); err == nil {
			nodes = append(nodes, n)
		}
	}

	eRows, err := db.Query(context.Background(), "SELECT id, source_id, target_id, label, created_at FROM impression_edges WHERE user_id = $1", dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer eRows.Close()

	edges := []models.ImpressionEdge{}
	for eRows.Next() {
		var e models.ImpressionEdge
		if err := eRows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Label, &e.CreatedAt); err == nil {
			edges = append(edges, e)
		}
	}

	return c.JSON(models.GraphResponse{Nodes: nodes, Edges: edges})
}

func ImportImpressionGraph(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	var graph models.GraphResponse
	if err := c.BodyParser(&graph); err != nil { return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"}) }

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer tx.Rollback(ctx)

	for _, n := range graph.Nodes {
		_, err = tx.Exec(ctx, `
			INSERT INTO impression_nodes (id, user_id, media_id, title, content, node_type, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				title = EXCLUDED.title,
				content = EXCLUDED.content,
				node_type = EXCLUDED.node_type,
				media_id = EXCLUDED.media_id
		`, n.ID, dbUserID, n.MediaID, n.Title, n.Content, n.NodeType, n.CreatedAt)
		if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Node upsert failed: " + err.Error()}) }
	}

	for _, e := range graph.Edges {
		_, err = tx.Exec(ctx, `
			INSERT INTO impression_edges (id, user_id, source_id, target_id, label, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				label = EXCLUDED.label,
				source_id = EXCLUDED.source_id,
				target_id = EXCLUDED.target_id
		`, e.ID, dbUserID, e.SourceID, e.TargetID, e.Label, e.CreatedAt)
		if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Edge upsert failed: " + err.Error()}) }
	}

	if err := tx.Commit(ctx); err != nil { return c.Status(500).JSON(fiber.Map{"error": "Commit failed"}) }

	return c.JSON(fiber.Map{"status": "imported", "nodes": len(graph.Nodes), "edges": len(graph.Edges)})
}

func SyncNodeToSnippet(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User not found"}) }

	// 1. Fetch node AND check if already linked
	var n models.ImpressionNode
	err = db.QueryRow(context.Background(), 
		"SELECT title, content, linked_snippet_id FROM impression_nodes WHERE id = $1 AND user_id = $2", id, dbUserID).Scan(&n.Title, &n.Content, &n.LinkedSnippetID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "Node not found"}) }

	if n.LinkedSnippetID != nil && *n.LinkedSnippetID != "" {
		return c.JSON(fiber.Map{"status": "existing", "snippetId": *n.LinkedSnippetID})
	}

	// 2. Create Snippet
	var snippetID string
	snippetQuery := `INSERT INTO snippets (user_id, name, content) VALUES ($1, $2, $3) RETURNING id`
	err = db.QueryRow(context.Background(), snippetQuery, dbUserID, n.Title, n.Content).Scan(&snippetID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Failed to create: " + err.Error()}) }

	// 3. Update Link IDs
	db.Exec(context.Background(), "UPDATE impression_nodes SET linked_snippet_id = $1 WHERE id = $2", snippetID, id)
	db.Exec(context.Background(), "UPDATE snippets SET linked_node_id = $1 WHERE id = $2", id, snippetID)

	return c.JSON(fiber.Map{"status": "linked", "snippetId": snippetID})
}

func GetLinkedSnippet(c *fiber.Ctx) error {
	id := c.Params("id") // snippetId
	userClaims := c.Locals("user").(*Claims)

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User profile not found"}) }

	var s models.Snippet
	err = db.QueryRow(context.Background(), 
		"SELECT id, name, content FROM snippets WHERE id = $1 AND user_id = $2", id, dbUserID).Scan(&s.ID, &s.Name, &s.Content)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "Snippet not found or unauthorized"}) }

	return c.JSON(s)
}
