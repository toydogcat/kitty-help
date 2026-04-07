package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4/pgxpool"
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
	centerID := c.Query("centerId")
	kgName := c.Query("kgName")
	if kgName == "" { kgName = "default" }

	db := database.LocalDB
	if db == nil { db = database.CloudDB }
	if db == nil { return c.Status(503).JSON(fiber.Map{"error": "Database not connected"}) }

	// Resolve internal DB User ID from email
	var dbUserID string
	err := db.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", userClaims.Email).Scan(&dbUserID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// 1. If centerID is empty, find the latest node in this KG
	isFocusMode := centerID != ""
	if centerID == "" {
		err := db.QueryRow(context.Background(), 
			"SELECT id FROM impression_nodes WHERE user_id = $1 AND kg_name = $2 ORDER BY created_at DESC LIMIT 1", 
			dbUserID, kgName).Scan(&centerID)
		if err != nil {
			return c.JSON(models.GraphResponse{Nodes: []models.ImpressionNode{}, Edges: []models.ImpressionEdge{}})
		}
	}

	// 2. Fetch Nodes using Recursive CTE (2 Degrees Bi-directional) within KG
	nodesQuery := `
	WITH RECURSIVE graph_nodes AS (
		SELECT id, user_id, media_id, linked_snippet_id, desk_shelf_id, title, content, node_type, created_at, kg_name, 0 as depth
		FROM impression_nodes
		WHERE id = $1 AND user_id = $2
		UNION
		SELECT n.id, n.user_id, n.media_id, n.linked_snippet_id, n.desk_shelf_id, n.title, n.content, n.node_type, n.created_at, n.kg_name, gn.depth + 1
		FROM graph_nodes gn
		JOIN impression_edges e ON (e.source_id = gn.id OR e.target_id = gn.id)
		JOIN impression_nodes n ON (n.id = CASE WHEN e.source_id = gn.id THEN e.target_id ELSE e.source_id END)
		WHERE gn.depth < 2 AND n.user_id = $2 AND n.kg_name = $4
	),
	recent_nodes AS (
		SELECT id, user_id, media_id, linked_snippet_id, desk_shelf_id, title, content, node_type, created_at, kg_name, 99 as depth
		FROM impression_nodes
		WHERE user_id = $2 AND kg_name = $4 AND NOT $3
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
		an.desk_shelf_id::TEXT,
		an.title, 
		COALESCE(an.content, '') as content, 
		an.node_type, 
		an.created_at, 
		COALESCE(m.file_id, '') as file_id, 
		COALESCE(m.source_platform, 'telegram') as source_platform,
		an.kg_name
	FROM all_nodes an
	LEFT JOIN media_archives m ON an.media_id = m.id
	`

	rows, err := db.Query(context.Background(), nodesQuery, centerID, dbUserID, isFocusMode, kgName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	nodeMap := make(map[string]models.ImpressionNode)
	nodesList := []models.ImpressionNode{}
	for rows.Next() {
		var it models.ImpressionNode
		err := rows.Scan(&it.ID, &it.UserID, &it.MediaID, &it.LinkedSnippetID, &it.DeskShelfID, &it.Title, &it.Content, &it.NodeType, &it.CreatedAt, &it.FileID, &it.SourcePlatform)
		if err == nil {
			if it.FileID != nil && *it.FileID != "" {
				it.ImageURL = "/api/storehouse/file/" + *it.FileID
				if it.SourcePlatform != nil {
					it.ImageURL += "?platform=" + *it.SourcePlatform
				}
			}
			nodesList = append(nodesList, it)
			nodeMap[it.ID] = it
		}
	}

	// 3. Fetch all edges between these nodes ONLY
	nodeIDs := make([]string, 0, len(nodeMap))
	for id := range nodeMap { nodeIDs = append(nodeIDs, id) }

	edges := []models.ImpressionEdge{}
	if len(nodeIDs) > 0 {
		edgesQuery := `
			SELECT id::TEXT, user_id::TEXT, source_id::TEXT, target_id::TEXT, label, created_at 
			FROM impression_edges 
			WHERE user_id = $1 AND source_id = ANY($2::uuid[]) AND target_id = ANY($2::uuid[])
		`
		rowsE, err := db.Query(context.Background(), edgesQuery, dbUserID, nodeIDs)
		if err != nil {
			log.Printf("⚠️ GetGraph SQL Warning: %v", err)
		} else {
			defer rowsE.Close()
			for rowsE.Next() {
				var e models.ImpressionEdge
				err := rowsE.Scan(&e.ID, &e.UserID, &e.SourceID, &e.TargetID, &e.Label, &e.CreatedAt)
				if err == nil {
					edges = append(edges, e)
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
	          SET title = $1, content = $2, node_type = $3, desk_shelf_id = $4 
	          WHERE id = $5 AND user_id = $6 
	          RETURNING id, linked_snippet_id, desk_shelf_id, created_at`
	
	err = db.QueryRow(context.Background(), query, 
		n.Title, n.Content, n.NodeType, n.DeskShelfID, id, dbUserID).Scan(&n.ID, &n.LinkedSnippetID, &n.DeskShelfID, &n.CreatedAt)
	
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

	rows, err := db.Query(context.Background(), "SELECT id, media_id, title, content, node_type, desk_shelf_id, created_at FROM impression_nodes WHERE user_id = $1", dbUserID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	defer rows.Close()

	nodes := []models.ImpressionNode{}
	for rows.Next() {
		var n models.ImpressionNode
		if err := rows.Scan(&n.ID, &n.MediaID, &n.Title, &n.Content, &n.NodeType, &n.DeskShelfID, &n.CreatedAt); err == nil {
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
