package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/google/uuid"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/models"
)

func getBestDB(c *fiber.Ctx) (*pgxpool.Pool, *Claims, error) {
	userClaims, ok := c.Locals("user").(*Claims)
	if !ok { return nil, nil, fmt.Errorf("user not authenticated") }
	
	db := database.LocalDB
	isAdmin := userClaims.Role == "superadmin" || userClaims.Role == "toby" || userClaims.Email == "toydogcat@gmail.com"
	
	if !isAdmin && db == nil {
		db = database.CloudDB
	}
	
	if db == nil {
		return nil, nil, fmt.Errorf("Local Database NOT connected (NUC Offline). Please check your Docker volume.")
	}
	return db, userClaims, nil
}

func GetRandomImpressionNodeID(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var dbUserID string
	err = db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User profile not found"}) }

	var nodeID string
	err = db.QueryRow(context.Background(), 
		"SELECT id::TEXT FROM impression_nodes WHERE user_id = $1 ORDER BY RANDOM() LIMIT 1", 
		dbUserID).Scan(&nodeID)
	
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "No nodes found"}) }
	return c.JSON(fiber.Map{"id": nodeID})
}

func GetImpressionTemp(c *fiber.Ctx) error {
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

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
	err = db.QueryRow(context.Background(), query, userClaims.Email).Scan(&dbUserID)
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

	// Get Base URL once outside loop
	baseURL := os.Getenv("VITE_API_URL")
	if baseURL == "" { baseURL = c.BaseURL() }

	items := []fiber.Map{}
	for rows.Next() {
		var id, fileID, mediaType, sourcePlatform string
		var createdAt time.Time
		var title, caption, notes *string
		err := rows.Scan(&id, &fileID, &mediaType, &title, &caption, &notes, &createdAt, &sourcePlatform)
		if err != nil { continue }
		
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
	db, userClaims, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	var n models.ImpressionNode
	if err := c.BodyParser(&n); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	dbUserID := userClaims.ID

	// Default KG if empty
	if n.KGName == "" { n.KGName = "default" }

	// 1. Check for UPSERT (same media_id and kg_name)
	if n.MediaID != nil && *n.MediaID != "" {
		var existingID string
		err := db.QueryRow(context.Background(), 
			"SELECT id FROM impression_nodes WHERE user_id = $1 AND media_id = $2 AND kg_name = $3", 
			dbUserID, n.MediaID, n.KGName).Scan(&existingID)
		
		if err == nil {
			updateQuery := `UPDATE impression_nodes 
			                SET title = $1, content = $2, node_type = $3, desk_shelf_id = $4
			                WHERE id = $5 
			                RETURNING id, linked_snippet_id, desk_shelf_id, created_at`
			
			err = db.QueryRow(context.Background(), updateQuery, 
				n.Title, n.Content, n.NodeType, n.DeskShelfID, existingID).Scan(&n.ID, &n.LinkedSnippetID, &n.DeskShelfID, &n.CreatedAt)
			
			if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
			n.UserID = dbUserID
			return c.JSON(n)
		}
	}

	// 2. Normal INSERT
	query := `INSERT INTO impression_nodes (user_id, media_id, title, content, node_type, desk_shelf_id, kg_name) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) 
	          RETURNING id, linked_snippet_id, desk_shelf_id, created_at`
	
	err = db.QueryRow(context.Background(), query, 
		dbUserID, n.MediaID, n.Title, n.Content, n.NodeType, n.DeskShelfID, n.KGName).Scan(&n.ID, &n.LinkedSnippetID, &n.DeskShelfID, &n.CreatedAt)
	
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	n.UserID = dbUserID
	return c.JSON(n)
}

func GetImpressionGraph(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	kgName := c.Query("kgName")
	if kgName == "" { kgName = "default" }

	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	dbUserID := userClaims.ID

	// 1. Fetch ALL Nodes in this KG
	nodesQuery := `
		SELECT 
			n.id::TEXT, n.user_id::TEXT, n.media_id::TEXT, n.linked_snippet_id::TEXT, n.desk_shelf_id::TEXT,
			n.title, COALESCE(n.content, '') as content, n.node_type, n.created_at, 
			COALESCE(m.file_id, '') as file_id, COALESCE(m.source_platform, 'telegram') as source_platform,
			n.kg_name
		FROM impression_nodes n
		LEFT JOIN media_archives m ON n.media_id = m.id
		WHERE n.user_id = $1 AND n.kg_name = $2
	`

	rows, err := db.Query(context.Background(), nodesQuery, dbUserID, kgName)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	nodes := []models.ImpressionNode{}
	for rows.Next() {
		var n models.ImpressionNode
		var sourcePlatform string
		if err := rows.Scan(&n.ID, &n.UserID, &n.MediaID, &n.LinkedSnippetID, &n.DeskShelfID, &n.Title, &n.Content, &n.NodeType, &n.CreatedAt, &n.FileID, &sourcePlatform, &n.KGName); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		n.SourcePlatform = &sourcePlatform
		nodes = append(nodes, n)
	}

	// 2. Fetch ALL Edges in this KG
	edgesQuery := `
		SELECT id::TEXT, user_id::TEXT, source_id::TEXT, target_id::TEXT, label, created_at, kg_name
		FROM impression_edges
		WHERE user_id = $1 AND kg_name = $2
	`
	rowsE, err := db.Query(context.Background(), edgesQuery, dbUserID, kgName)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rowsE.Close()

	edges := []models.ImpressionEdge{}
	for rowsE.Next() {
		var e models.ImpressionEdge
		if err := rowsE.Scan(&e.ID, &e.UserID, &e.SourceID, &e.TargetID, &e.Label, &e.CreatedAt, &e.KGName); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		edges = append(edges, e)
	}

	return c.JSON(models.GraphResponse{Nodes: nodes, Edges: edges})
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

	dbUserID := userClaims.ID
	if e.KGName == "" { e.KGName = "default" }

	var exists bool
	db.QueryRow(context.Background(), 
		"SELECT EXISTS(SELECT 1 FROM impression_edges WHERE source_id = $1 AND target_id = $2 AND label = $3 AND user_id = $4 AND kg_name = $5)", 
		e.SourceID, e.TargetID, e.Label, dbUserID, e.KGName).Scan(&exists)

	if exists {
		return c.Status(409).JSON(fiber.Map{"error": "Relationship already exists"})
	}

	query := `INSERT INTO impression_edges (user_id, source_id, target_id, label, kg_name) 
	          VALUES ($1, $2, $3, $4, $5) 
	          RETURNING id, created_at`
	
	err := db.QueryRow(context.Background(), query, 
		dbUserID, e.SourceID, e.TargetID, e.Label, e.KGName).Scan(&e.ID, &e.CreatedAt)
	
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

func SearchImpression(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	q := c.Query("q")
	kgSearch := c.Query("kgName") // Optional: search all or specific

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	dbUserID := userClaims.ID

	// 1. Search Nodes
	nodesSQL := `
	SELECT n.id::TEXT, n.title, n.node_type, m.file_id, m.source_platform, n.kg_name, 'node' as result_type
	FROM impression_nodes n
	LEFT JOIN media_archives m ON n.media_id = m.id
	WHERE n.user_id = $1 AND n.title ILIKE $2
	`
	if kgSearch != "" {
		nodesSQL += " AND n.kg_name = '" + kgSearch + "'"
	}
	nodesSQL += " LIMIT 20"

	rowsN, err := db.Query(context.Background(), nodesSQL, dbUserID, "%"+q+"%")
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rowsN.Close()

	results := []fiber.Map{}
	for rowsN.Next() {
		var id, title, nodeType, kgName, resType string
		var fileID, sourcePlatform *string
		if err := rowsN.Scan(&id, &title, &nodeType, &fileID, &sourcePlatform, &kgName, &resType); err == nil {
			nodeMap := fiber.Map{
				"id":         id,
				"title":      title,
				"nodeType":   nodeType,
				"resultType": resType,
				"kgName":     kgName,
			}
			if fileID != nil && sourcePlatform != nil {
				nodeMap["imageUrl"] = "/api/storehouse/file/" + *fileID + "?platform=" + *sourcePlatform
			}
			results = append(results, nodeMap)
		}
	}

	// 2. Search Edges (Links)
	edgesSQL := `
	SELECT e.id::TEXT, e.label, 'edge' as result_type, e.kg_name, s.title as source_title, t.title as target_title
	FROM impression_edges e
	JOIN impression_nodes s ON e.source_id = s.id
	JOIN impression_nodes t ON e.target_id = t.id
	WHERE e.user_id = $1 AND e.label ILIKE $2
	`
	if kgSearch != "" {
		edgesSQL += " AND e.kg_name = '" + kgSearch + "'"
	}
	edgesSQL += " LIMIT 20"

	rowsE, err := db.Query(context.Background(), edgesSQL, dbUserID, "%"+q+"%")
	if err == nil {
		defer rowsE.Close()
		for rowsE.Next() {
			var id, label, resType, kgName, stTitle, tgTitle string
			if err := rowsE.Scan(&id, &label, &resType, &kgName, &stTitle, &tgTitle); err == nil {
				results = append(results, fiber.Map{
					"id":          id,
					"title":       label,
					"resultType":  resType,
					"kgName":      kgName,
					"sourceTitle": stTitle,
					"targetTitle": tgTitle,
				})
			}
		}
	}

	return c.JSON(results)
}

func GetKnowledgeGraphs(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db, _, err := getBestDB(c)
	if err != nil { return c.Status(503).JSON(fiber.Map{"error": err.Error()}) }

	sql := `SELECT DISTINCT kg_name FROM impression_nodes WHERE user_id = $1 
	        UNION 
	        SELECT DISTINCT kg_name FROM impression_edges WHERE user_id = $1`
	
	rows, err := db.Query(context.Background(), sql, userClaims.ID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	kgs := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil { kgs = append(kgs, name) }
	}
	if len(kgs) == 0 { kgs = []string{"default"} }
	return c.JSON(kgs)
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
	          SET title = $1, content = $2, node_type = $3, desk_shelf_id = $4, media_id = $5 
	          WHERE id = $6 AND user_id = $7 
	          RETURNING id, linked_snippet_id, desk_shelf_id, created_at, media_id`
	
	err = db.QueryRow(context.Background(), query, 
		n.Title, n.Content, n.NodeType, n.DeskShelfID, n.MediaID, id, dbUserID).Scan(&n.ID, &n.LinkedSnippetID, &n.DeskShelfID, &n.CreatedAt, &n.MediaID)
	
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

	rows, err := db.Query(context.Background(), "SELECT id, media_id, title, content, node_type, desk_shelf_id, created_at, kg_name FROM impression_nodes WHERE user_id = $1", dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	nodes := []models.ImpressionNode{}
	for rows.Next() {
		var n models.ImpressionNode
		if err := rows.Scan(&n.ID, &n.MediaID, &n.Title, &n.Content, &n.NodeType, &n.DeskShelfID, &n.CreatedAt, &n.KGName); err == nil {
			nodes = append(nodes, n)
		}
	}

	eRows, err := db.Query(context.Background(), "SELECT id, source_id, target_id, label, created_at, kg_name FROM impression_edges WHERE user_id = $1", dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer eRows.Close()

	edges := []models.ImpressionEdge{}
	for eRows.Next() {
		var e models.ImpressionEdge
		if err := eRows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Label, &e.CreatedAt, &e.KGName); err == nil {
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
		// Clean kg_name: if empty in JSON, set to 'default'
		kg := n.KGName
		if kg == "" { kg = "default" }

		_, err = tx.Exec(ctx, `
			INSERT INTO impression_nodes (id, user_id, media_id, title, content, node_type, created_at, kg_name, desk_shelf_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				title = EXCLUDED.title,
				content = EXCLUDED.content,
				node_type = EXCLUDED.node_type,
				media_id = EXCLUDED.media_id,
				kg_name = EXCLUDED.kg_name,
				desk_shelf_id = EXCLUDED.desk_shelf_id
		`, n.ID, dbUserID, n.MediaID, n.Title, n.Content, n.NodeType, n.CreatedAt, kg, n.DeskShelfID)
		if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Node upsert failed: " + err.Error()}) }
	}

	for _, e := range graph.Edges {
		kg := e.KGName
		if kg == "" { kg = "default" }

		_, err = tx.Exec(ctx, `
			INSERT INTO impression_edges (id, user_id, source_id, target_id, label, created_at, kg_name)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				label = EXCLUDED.label,
				source_id = EXCLUDED.source_id,
				target_id = EXCLUDED.target_id,
				kg_name = EXCLUDED.kg_name
		`, e.ID, dbUserID, e.SourceID, e.TargetID, e.Label, e.CreatedAt, kg)
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

func DuplicateKnowledgeGraph(c *fiber.Ctx) error {
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	type DupeReq struct { Source string `json:"source"`; Target string `json:"target"` }
	var req DupeReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).JSON(fiber.Map{"error": "Invalid body"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User profile not found"}) }

	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer tx.Rollback(ctx)

	// Map oldID -> newID
	idMap := make(map[string]string)

	rows, err := tx.Query(ctx, "SELECT id, media_id, title, content, node_type, desk_shelf_id FROM impression_nodes WHERE user_id = $1 AND kg_name = $2", dbUserID, req.Source)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Fetch nodes failed"}) }
	
	type nodeRec struct { ID, MediaID, Title, Content, NodeType, DeskShelfID *string }
	nodes := []nodeRec{}
	for rows.Next() {
		var r nodeRec
		rows.Scan(&r.ID, &r.MediaID, &r.Title, &r.Content, &r.NodeType, &r.DeskShelfID)
		nodes = append(nodes, r)
	}
	rows.Close()

	for _, n := range nodes {
		newID := uuid.New().String()
		idMap[*n.ID] = newID
		_, err = tx.Exec(ctx, `
			INSERT INTO impression_nodes (id, user_id, media_id, title, content, node_type, desk_shelf_id, kg_name)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, newID, dbUserID, n.MediaID, n.Title, n.Content, n.NodeType, n.DeskShelfID, req.Target)
		if err != nil { return c.Status(500).JSON(fiber.Map{"error": "Clone nodes failed: " + err.Error()}) }
	}

	eRows, err := tx.Query(ctx, "SELECT source_id, target_id, label FROM impression_edges WHERE user_id = $1 AND kg_name = $2", dbUserID, req.Source)
	if err == nil {
		type edgeRec struct { Src, Tgt, Lbl string }
		edges := []edgeRec{}
		for eRows.Next() {
			var er edgeRec
			eRows.Scan(&er.Src, &er.Tgt, &er.Lbl)
			edges = append(edges, er)
		}
		eRows.Close()

		for _, e := range edges {
			newSrc, ok1 := idMap[e.Src]
			newTgt, ok2 := idMap[e.Tgt]
			if ok1 && ok2 {
				_, err = tx.Exec(ctx, "INSERT INTO impression_edges (user_id, source_id, target_id, label, kg_name) VALUES ($1, $2, $3, $4, $5)", dbUserID, newSrc, newTgt, e.Lbl, req.Target)
			}
		}
	}

	tx.Commit(ctx)
	return c.JSON(fiber.Map{"status": "cloned", "nodes": len(nodes)})
}

func CloneImpressionNode(c *fiber.Ctx) error {
	id := c.Params("id")
	userClaims := c.Locals("user").(*Claims)
	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "User profile not found"}) }

	var n models.ImpressionNode
	err = db.QueryRow(context.Background(), "SELECT media_id, title, content, node_type, desk_shelf_id, kg_name FROM impression_nodes WHERE id = $1 AND user_id = $2", id, dbUserID).Scan(&n.MediaID, &n.Title, &n.Content, &n.NodeType, &n.DeskShelfID, &n.KGName)
	if err != nil { return c.Status(404).JSON(fiber.Map{"error": "Source node not found"}) }

	newID := uuid.New().String()
	newTitle := n.Title + " - Copy"

	query := `INSERT INTO impression_nodes (id, user_id, media_id, title, content, node_type, desk_shelf_id, kg_name) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	err = db.QueryRow(context.Background(), query, newID, dbUserID, n.MediaID, newTitle, n.Content, n.NodeType, n.DeskShelfID, n.KGName).Scan(&n.ID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }

	// Duplicate edges where this node is source
	db.Exec(context.Background(), `INSERT INTO impression_edges (user_id, source_id, target_id, label, kg_name) 
		SELECT user_id, $1, target_id, label, kg_name FROM impression_edges WHERE source_id = $2 AND user_id = $3`, n.ID, id, dbUserID)
	// Duplicate edges where this node is target
	db.Exec(context.Background(), `INSERT INTO impression_edges (user_id, source_id, target_id, label, kg_name) 
		SELECT user_id, source_id, $1, label, kg_name FROM impression_edges WHERE target_id = $2 AND user_id = $3`, n.ID, id, dbUserID)

	return c.JSON(n)
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
