package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/handlers"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

func main() {
	// 1. Load .env (try local first, then parent for dev)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	// 1. Initialize Databases
	database.InitDB()
	defer database.CloseDB()
	database.EnsureTables()
	if database.LocalDB != nil {
		database.LocalDB.Exec(context.Background(), "ALTER TABLE bot_authorized_users ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)")
		database.LocalDB.Exec(context.Background(), "ALTER TABLE bot_authorized_users ADD COLUMN IF NOT EXISTS role TEXT DEFAULT 'user'")
	}

	// 2. Initialize Sockets
	sockets.InitSocketIO()

	// 3. Initialize Bots
	bots.InitManager()
	
	// Define admins
	var admins []string
	if envAdmins := os.Getenv("ADMIN_EMAILS"); envAdmins != "" {
		admins = strings.Split(envAdmins, ",")
	} else {
		admins = []string{"toydogcat@gmail.com"}
	}
	if id := os.Getenv("ADMIN_TELEGRAM_ID"); id != "" {
		admins = append(admins, id)
	}
	if id := os.Getenv("ADMIN_DISCORD_ID"); id != "" {
		admins = append(admins, id)
	}
	if id := os.Getenv("ADMIN_LINE_ID"); id != "" {
		admins = append(admins, id)
	}
	
	// Telegram Bot Setup
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken != "" {
		var storehouseID int64
		fmt.Sscanf(os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID"), "%d", &storehouseID)
		
		tgBot, err := bots.NewTelegramBot(tgToken, admins, storehouseID)
		if err == nil {
			bots.BotManager.Register("telegram", tgBot)
		} else {
			log.Printf("Failed to create Telegram bot: %v", err)
		}
	}

	// Discord Bot Setup
	dsToken := os.Getenv("DISCORD_BOT_TOKEN")
	if dsToken != "" {
		dsBot, err := bots.NewDiscordBot(dsToken, admins)
		if err == nil {
			bots.BotManager.Register("discord", dsBot)
		} else {
			log.Printf("Failed to create Discord bot: %v", err)
		}
	}

	// LINE Bot Setup
	lineSecret := os.Getenv("LINE_CHANNEL_SECRET")
	lineToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	var lineBotInstance *bots.LineBot
	if lineSecret != "" && lineToken != "" {
		lb, err := bots.NewLineBot(lineSecret, lineToken, admins)
		if err == nil {
			lineBotInstance = lb
			bots.BotManager.Register("line", lb)
		}
	}

	// Start all bot channels
	bots.BotManager.StartAll(context.Background())

	// 3. Setup Fiber App with Robust Timeouts
	app := fiber.New(fiber.Config{
		AppName:      "Kitty-Help Go Backend",
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	app.Use(logger.New())
	// 4. Aggressive CORS Strategy
	// Manually inject headers for ALL requests to ensure reliability over tunnels
	app.Use(func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		
		// [REQUEST DEBUG] Tracking every incoming request for diagnostics
		fmt.Printf("[REQUEST DEBUG] %s %s (Origin: %s)\n", c.Method(), c.Path(), origin)
		
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, cf-skip-browser-warning, ngrok-skip-browser-warning")
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Expose-Headers", "Content-Length")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(204)
		}
		return c.Next()
	})

	// Dedicated Socket.io CORS
	app.Use("/socket.io", func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		
		// Set headers for ALL requests (Preflight AND Handshake)
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, cf-skip-browser-warning")
		c.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if c.Method() == "OPTIONS" {
			return c.SendStatus(204)
		}
		return c.Next()
	})

	// Static & API Routes
	app.Static("/uploads", "../uploads")
	api := app.Group("/api")

	api.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong - Super Kitty is awake! 🐱")
	})

	// Auth
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	
	// Read-only public API for initial load
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	
	// Device Registration (Must be public for new devices to request approval)
	api.Post("/devices/register", handlers.RegisterDevice)
	api.Post("/devices/status", handlers.UpdateDeviceStatus)

	// Sockets (wrapped for Fiber)
	app.All("/socket.io/*", adaptor.HTTPHandler(sockets.Server))

	// Webhooks
	if lineBotInstance != nil {
		app.Post("/webhook/line", lineBotInstance.HandleFiberWebhook)
	}

	// Settings (Public GET, Protected POST)
	api.Get("/settings", handlers.GetSettings)

	// Bot Auth (Public)
	api.Post("/bot/verify", handlers.VerifyJoinToken)
	api.Get("/storehouse", handlers.GetStorehouseItems)
	api.Put("/storehouse/:id", handlers.UpdateStorehouseItem)
	api.Post("/storehouse/:id/index", handlers.IndexStorehouseItem)
	api.Get("/storehouse/file/:fileID", handlers.GetFileProxy)

	// OpenCLI Proxy (Document Chicken 7080)
	api.Post("/opencli", handlers.ProxyOpenCLI)

	// API Handlers (Protected)
	// Phase 1: JWT Verification
	authShared := api.Group("/", handlers.JWTMiddleware)

	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)
	authShared.Get("/chat/logs", handlers.GetChatLogs)

	// Phase 2: Device Approval Check
	protected := authShared.Group("/", handlers.DeviceCheckMiddleware)

	// Device Management (Admin Only)
	admin := protected.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/devices", handlers.GetDevices)
	admin.Put("/devices/status", handlers.UpdateDeviceStatus)
	admin.Delete("/devices/:id", handlers.DeleteDevice)

	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users", handlers.CreateUser)
	admin.Post("/users/role", handlers.UpdateUserRole)
	admin.Delete("/users/:id", handlers.DeleteUser)
	
	protected.Get("/snippets", handlers.GetSnippets)
	protected.Post("/snippets", handlers.CreateSnippet)
	protected.Put("/snippets/:id", handlers.UpdateSnippet)
	protected.Delete("/snippets/:id", handlers.DeleteSnippet)

	protected.Get("/common", handlers.GetCommonState)
	protected.Post("/common/update", handlers.UpdateCommonState)
	protected.Get("/common/history", handlers.GetCommonHistory)
	protected.Get("/chat/logs", handlers.GetChatLogs)

	protected.Get("/calendar", handlers.GetCalendarEvents)
	protected.Post("/calendar", handlers.UpdateCalendarEvent)

	protected.Get("/bulletin", handlers.GetBulletin)
	protected.Post("/bulletin", handlers.UpdateBulletin)

	// Admin Settings Update
	protected.Post("/settings", handlers.UpdateSetting)

	// Bot Management (Toby & Admin)
	toby := protected.Group("/", handlers.TobyOnlyMiddleware)
	toby.Get("/bot/requests", handlers.GetPendingBotRequests)
	toby.Post("/bot/approve", handlers.ApproveBotRequest)
	toby.Post("/bot/reject", handlers.RejectBotRequest)
	toby.Get("/bot/users", handlers.GetAuthorizedBotUsers)
	toby.Post("/bot/users/delete", handlers.DeleteAuthorizedBotUser)
	protected.Get("/bot/my-status", handlers.GetMyBotStatus)
	protected.Post("/bot/link", handlers.LinkBotAccount)
	// Password Vault (Private for Toby/Admin)
	toby.Get("/passwords", handlers.GetPasswords)
	toby.Post("/passwords", handlers.AddPassword)
	toby.Delete("/passwords/:id", handlers.DeletePassword)

	// Bookmarks (Private per user)
	protected.Get("/bookmarks", handlers.GetBookmarks)
	protected.Post("/bookmarks", handlers.CreateBookmark)
	protected.Put("/bookmarks/:id", handlers.UpdateBookmark)
	protected.Delete("/bookmarks/:id", handlers.DeleteBookmark)

	// Security 2FA Sessions (Allow even on pending devices to facilitate approval/2FA)
	authShared.Post("/security/challenge", handlers.RequestChallenge)
	authShared.Get("/security/status", handlers.GetSecurityStatus)

	// Impression (Graph Knowledge Canvas)
	protected.Get("/impression/temp", handlers.GetImpressionTemp)
	protected.Get("/impression/graph", handlers.GetImpressionGraph)
	protected.Get("/impression/search", handlers.SearchImpressionNodes)
	protected.Get("/impression/export", handlers.ExportImpressionGraph)
	protected.Post("/impression/nodes", handlers.CreateImpressionNode)
	protected.Post("/impression/links", handlers.CreateImpressionLink)
	protected.Post("/impression/import", handlers.ImportImpressionGraph)
	protected.Put("/impression/links/:id", handlers.UpdateImpressionEdge)
	protected.Delete("/impression/links/:id", handlers.DeleteImpressionEdge)
	protected.Get("/impression/random", handlers.GetRandomImpressionNodeID)
	protected.Post("/impression/nodes/:id/sync", handlers.SyncNodeToSnippet)
	protected.Get("/impression/snippets/:id", handlers.GetLinkedSnippet)
	protected.Delete("/impression/nodes/:id", handlers.DeleteImpressionNode)
	protected.Put("/impression/nodes/:id", handlers.UpdateImpressionNode)

	// 4. Migrate Admin IDs from .env to LocalDB (if not already done)
	migrateBotAdmins()

	// 5. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\n🛑 Gracefully shutting down...")
		_ = app.Shutdown()
		bots.BotManager.StopAll(context.Background())
	}()

	// 6. API Only: No local frontend hosting needed (Using Firebase)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "online",
			"message": "Kitty-Help API Gateway",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("🚀 Super Kitty (Go) running on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

func migrateBotAdmins() {
	if database.LocalDB == nil {
		return
	}
	admins := []struct {
		platform string
		envKey   string
	}{
		{"telegram", "ADMIN_TELEGRAM_ID"},
		{"discord", "ADMIN_DISCORD_ID"},
		{"line", "ADMIN_LINE_ID"},
	}

	for _, a := range admins {
		id := os.Getenv(a.envKey)
		if id != "" {
			_, err := database.LocalDB.Exec(context.Background(), 
				`INSERT INTO bot_authorized_users (platform, account_id, account_name, role) 
				 VALUES ($1, $2, 'Admin From Env', 'superadmin') 
				 ON CONFLICT (platform, account_id) DO UPDATE SET role = 'superadmin'`, 
				a.platform, id)
			if err != nil {
				log.Printf("⚠️ Failed to migrate %s admin: %v", a.platform, err)
			} else {
				log.Printf("✅ Migrated %s admin ID to database.", a.platform)
			}
		}
	}
}
