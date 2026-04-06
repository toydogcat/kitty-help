package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

var (
	LocalDB *pgxpool.Pool
	CloudDB *pgxpool.Pool
)

// CheckDB returns true if the local database is connected
func CheckLocalDB() bool {
	return LocalDB != nil
}

func CheckCloudDB() bool {
	return CloudDB != nil
}

func InitDB() {
	// Load .env from root
	err := godotenv.Load("../.env")
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	cloudURL := os.Getenv("DATABASE_URL")
	localURL := os.Getenv("DATABASE_URL_LOCAL")

	if cloudURL == "" {
		log.Fatal("DATABASE_URL (Cloud) is not set")
	}
	if localURL == "" {
		log.Println("Warning: DATABASE_URL_LOCAL is not set, falling back to DATABASE_URL for local operations (Not recommended for Phase 2)")
		localURL = cloudURL
	}

	// Connect to Cloud DB
	configCloud, err := pgxpool.ParseConfig(cloudURL)
	if err != nil {
		log.Fatalf("Unable to parse Cloud DB URL: %v", err)
	}
	CloudDB, err = pgxpool.ConnectConfig(context.Background(), configCloud)
	if err != nil {
		log.Printf("⚠️ Warning: Unable to connect to Cloud DB: %v", err)
	} else {
		fmt.Println("✅ Connected to Cloud PostgreSQL (Supabase)")
	}

	// Connect to Local DB
	configLocal, err := pgxpool.ParseConfig(localURL)
	if err != nil {
		log.Printf("⚠️ Warning: Unable to parse Local DB URL: %v", err)
		return
	}
	LocalDB, err = pgxpool.ConnectConfig(context.Background(), configLocal)
	if err != nil {
		log.Printf("⚠️ Warning: Unable to connect to Local DB: %v", err)
	} else {
		// Database connected successfully
		fmt.Printf("✅ Connected to Local PostgreSQL at %s\n", configLocal.ConnConfig.Host)
	}
}

func CloseDB() {
	if CloudDB != nil {
		CloudDB.Close()
	}
	if LocalDB != nil {
		LocalDB.Close()
	}
}

// EnsureTables initializes the table structure for both databases
func EnsureTables() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- Initialize Cloud DB Tables (Shared Data) ---
	cloudTables := []string{
		`CREATE TABLE IF NOT EXISTS bulletin (
			id SERIAL PRIMARY KEY,
			message TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID, -- References users table (will be synced/placeholder)
			event_date TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, event_date)
		)`,
		`CREATE TABLE IF NOT EXISTS common_state (
			key TEXT PRIMARY KEY,
			content TEXT,
			file_url TEXT,
			file_name TEXT,
			updated_by UUID,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	if CloudDB != nil {
		for _, q := range cloudTables {
			if _, err := CloudDB.Exec(ctx, q); err != nil {
				log.Printf("Error creating cloud table: %v", err)
			}
		}

		// Initialize bulletin if empty
		var count int
		CloudDB.QueryRow(ctx, "SELECT COUNT(*) FROM bulletin").Scan(&count)
		if count == 0 {
			CloudDB.Exec(ctx, "INSERT INTO bulletin (message) VALUES ('Welcome to kitty-help! Go Backend successfully initialized.')")
		}
	}

	// --- Initialize Local DB Tables (Private Data) ---
	localTables := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT UNIQUE NOT NULL,
			role TEXT DEFAULT 'user',
			google_id TEXT UNIQUE,
			email TEXT UNIQUE,
			discord_id TEXT,
			line_id TEXT,
			telegram_id TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			status TEXT DEFAULT 'pending',
			device_name TEXT,
			user_agent TEXT,
			user_id UUID REFERENCES users(id),
			last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS snippets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			parent_id UUID REFERENCES snippets(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			content TEXT,
			is_folder BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS media_archives (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			file_id TEXT NOT NULL,
			message_id BIGINT,
			media_type TEXT NOT NULL,
			title TEXT,
			caption TEXT,
			notes TEXT,
			source_platform TEXT DEFAULT 'telegram',
			sender_name TEXT,
			sender_id TEXT,
			chat_id BIGINT,
			embedding vector(1536), -- Gemini Embedding dimension
			is_indexable BOOLEAN DEFAULT FALSE,
			index_status TEXT DEFAULT 'not_indexed', -- not_indexed, indexing, indexed, unsupported
			embedding_model TEXT,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bot_auth_requests (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			token TEXT UNIQUE,
			platform TEXT NOT NULL,
			account_id TEXT NOT NULL,
			account_name TEXT,
			status TEXT DEFAULT 'pending', -- pending, approved, rejected
			user_id UUID REFERENCES users(id),
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bot_authorized_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			platform TEXT NOT NULL,
			account_id TEXT NOT NULL,
			account_name TEXT,
			user_id UUID REFERENCES users(id),
			role TEXT DEFAULT 'user', -- user, toby, superadmin
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(platform, account_id)
		)`,
		`CREATE TABLE IF NOT EXISTS passwords (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			site_name TEXT NOT NULL,
			account TEXT NOT NULL,
			password_raw TEXT NOT NULL, -- Currently using raw/basic for local trial; will add crypto if needed
			category TEXT, -- Work, Social, Admin, etc.
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bulletin (
			id SERIAL PRIMARY KEY,
			message TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID,
			event_date TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, event_date)
		)`,
		`CREATE TABLE IF NOT EXISTS common_state (
			key TEXT PRIMARY KEY,
			content TEXT,
			file_url TEXT,
			file_name TEXT,
			updated_by UUID,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bookmarks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			category TEXT DEFAULT 'uncategorized',
			icon_url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS impression_nodes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			media_id UUID REFERENCES media_archives(id) ON DELETE SET NULL,
			title TEXT NOT NULL,
			content TEXT,
			node_type TEXT DEFAULT 'general', -- person, place, event, etc.
			desk_shelf_id UUID REFERENCES desk_shelves(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS impression_edges (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			source_id UUID REFERENCES impression_nodes(id) ON DELETE CASCADE,
			target_id UUID REFERENCES impression_nodes(id) ON DELETE CASCADE,
			label TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS impression_temp (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			media_id UUID REFERENCES media_archives(id) ON DELETE CASCADE,
			user_id UUID,
			title TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(media_id)
		)`,
		`CREATE TABLE IF NOT EXISTS desk_shelves (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			color TEXT,
			sort_order INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS desk_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			shelf_id UUID REFERENCES desk_shelves(id) ON DELETE CASCADE, -- NULL means on desktop
			type TEXT NOT NULL,    -- 'bookmark', 'snippet', 'media'
			ref_id UUID NOT NULL,   -- Points to actual record ID
			sort_order INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	if LocalDB != nil {
		for _, q := range localTables {
			if _, err := LocalDB.Exec(ctx, q); err != nil {
				log.Printf("Error creating local table: %v", err)
			}
		}

		// Migrations for existing tables
		migrations := []string{
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS title TEXT`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS notes TEXT`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS embedding vector(1536)`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS metadata JSONB`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS is_indexable BOOLEAN DEFAULT FALSE`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS index_status TEXT DEFAULT 'not_indexed'`,
			`ALTER TABLE media_archives ADD COLUMN IF NOT EXISTS embedding_model TEXT`,
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS line_id TEXT`,
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS discord_id TEXT`,
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_id TEXT`,
			`ALTER TABLE bot_auth_requests ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
			`ALTER TABLE bot_auth_requests ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP`,
			`ALTER TABLE bot_authorized_users ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
			`ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS password_id UUID REFERENCES passwords(id) ON DELETE SET NULL`,
			`ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES bookmarks(id) ON DELETE CASCADE`,
			`ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS is_folder BOOLEAN DEFAULT FALSE`,
			`ALTER TABLE bookmarks ADD COLUMN IF NOT EXISTS sort_order INT DEFAULT 0`,
			`ALTER TABLE bookmarks ALTER COLUMN url DROP NOT NULL`,
			`ALTER TABLE impression_nodes ADD COLUMN IF NOT EXISTS desk_shelf_id UUID REFERENCES desk_shelves(id) ON DELETE SET NULL`,
		}
		for _, m := range migrations {
			LocalDB.Exec(ctx, m)
		}

		// Security Sessions table for the 30-min dual-platform trust window
		securityTable := `CREATE TABLE IF NOT EXISTS security_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			device_id TEXT NOT NULL,
			token TEXT NOT NULL,
			line_verified_at TIMESTAMP,
			discord_verified_at TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			granted_at TIMESTAMP,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
		LocalDB.Exec(ctx, securityTable)
	}

	fmt.Println("✅ All database tables verified/initialized")
}
