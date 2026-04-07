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
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/toydogcat/kitty-help/go-server/bots"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/handlers"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

func main() {
	_ = godotenv.Load(".env")
	database.InitDB()
	database.EnsureTables()
	sockets.InitSocketIO()
	bots.InitManager()
	
	admins := []string{"toydogcat@gmail.com"}
	if env := os.Getenv("ADMIN_EMAILS"); env != "" {
		admins = append(admins, strings.Split(env, ",")...)
	}

	// 🤖 Bot Initialization
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken != "" {
		var storehouseID int64
		fmt.Sscanf(os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID"), "%d", &storehouseID)
		tgBot, _ := bots.NewTelegramBot(tgToken, admins, storehouseID)
		bots.BotManager.Register("telegram", tgBot)
	}
	dsToken := os.Getenv("DISCORD_BOT_TOKEN")
	if dsToken != "" {
		dsBot, _ := bots.NewDiscordBot(dsToken, admins)
		bots.BotManager.Register("discord", dsBot)
	}
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

	bots.BotManager.StartAll(context.Background())

	app := fiber.New(fiber.Config{
		AppName:      "Kitty-Help Master Suite 6.0",
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	// Dynamic & Intelligent CORS Control
	allowedOrigins := "https://kitty-help.web.app, http://localhost:5173, http://localhost:4173"
	if extra := os.Getenv("ALLOWED_ORIGINS"); extra != "" {
		allowedOrigins += ", " + extra
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		// 🛡️ [Security Booster] Automatically trust all Cloudflare Tunnels (trycloudflare.com)
		AllowOriginsFunc: func(origin string) bool {
			return strings.HasSuffix(origin, ".trycloudflare.com")
		},
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Refresh-Token, cf-skip-browser-warning, ngrok-skip-browser-warning",
		ExposeHeaders:    "X-Refresh-Token, Content-Disposition",
		AllowCredentials: true,
	}))

	app.Use(logger.New())
	app.Static("/uploads", "../uploads")
	api := app.Group("/api")

	// --- 1. Public API ---
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	api.Get("/settings", handlers.GetSettings)
	api.Post("/devices/register", handlers.RegisterDevice)
	api.Get("/storehouse", handlers.GetStorehouseItems)
	api.Put("/storehouse/:id", handlers.UpdateStorehouseItem) // ← MISSING BEFORE
	api.Post("/storehouse/:id/index", handlers.IndexStorehouseItem) // ← MISSING BEFORE
	api.Get("/storehouse/file/:fileID", handlers.GetFileProxy)
	api.Post("/opencli", handlers.ProxyOpenCLI)
	if lineBotInstance != nil { app.Post("/webhook/line", lineBotInstance.HandleFiberWebhook) }

	// --- 2. JWT Protected (Sliding Session) ---
	authShared := api.Group("/", handlers.JWTMiddleware)
	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)
	authShared.Get("/chat/logs", handlers.GetChatLogs)
	authShared.Get("/chat/photos", handlers.GetRecentPhotos)
	authShared.Get("/chat/remarks", handlers.GetRemarks)
	authShared.Post("/chat/remarks", handlers.CreateRemark)
	authShared.Put("/chat/remarks/:id", handlers.UpdateRemark)
	authShared.Delete("/chat/remarks/:id", handlers.DeleteRemark)
	authShared.Post("/chat/remarks/toggle", handlers.ToggleIntegration)
	authShared.Post("/chat/remarks/items", handlers.AddRemarkItem)
	authShared.Post("/chat/remarks/move", handlers.MoveRemarkItem)
	authShared.Delete("/chat/remarks/items/:id", handlers.RemoveRemarkItem)
	authShared.Post("/security/challenge", handlers.RequestChallenge)
	authShared.Get("/security/status", handlers.GetSecurityStatus)

	// --- 3. Device Protected ---
	protected := authShared.Group("/", handlers.DeviceCheckMiddleware)
	// 🧠 IMPRESSION GRAPH
	protected.Get("/impression/graph", handlers.GetImpressionGraph)
	protected.Get("/impression/kgs", handlers.GetKnowledgeGraphs)
	protected.Get("/impression/export", handlers.ExportImpressionGraph)
	protected.Post("/impression/import", handlers.ImportImpressionGraph)
	protected.Get("/impression/temp", handlers.GetImpressionTemp)
	protected.Get("/impression/search", handlers.SearchImpression)
	protected.Get("/impression/random", handlers.GetRandomImpressionNodeID)
	
	protected.Post("/impression/nodes", handlers.CreateImpressionNode)
	protected.Put("/impression/nodes/:id", handlers.UpdateImpressionNode)
	protected.Delete("/impression/nodes/:id", handlers.DeleteImpressionNode)
	protected.Post("/impression/copy", handlers.DuplicateKnowledgeGraph)
	protected.Post("/impression/nodes/:id/clone", handlers.CloneImpressionNode)
	protected.Post("/impression/nodes/:id/sync", handlers.SyncNodeToSnippet)
	
	protected.Post("/impression/links", handlers.CreateImpressionLink)
	protected.Put("/impression/links/:id", handlers.UpdateImpressionEdge)
	protected.Delete("/impression/links/:id", handlers.DeleteImpressionEdge)
	
	protected.Get("/impression/snippets/:id", handlers.GetLinkedSnippet)
	
	// 📑 SNIPPETS (Clipboard)
	protected.Get("/snippets", handlers.GetSnippets)
	protected.Post("/snippets", handlers.CreateSnippet)
	protected.Put("/snippets/:id", handlers.UpdateSnippet)
	protected.Delete("/snippets/:id", handlers.DeleteSnippet)
	
	// 🔖 BOOKMARKS
	protected.Get("/bookmarks", handlers.GetBookmarks)
	protected.Post("/bookmarks", handlers.CreateBookmark)
	protected.Put("/bookmarks/:id", handlers.UpdateBookmark)
	protected.Delete("/bookmarks/:id", handlers.DeleteBookmark)

	// 📚 BOOKCASE (Digital Library)
	protected.Get("/bookcase", handlers.GetBookcase)
	protected.Post("/bookcase", handlers.AddBookToBookcase)
	protected.Put("/bookcase/:id/legacy-notes", handlers.UpdateBookNotes) // Legacy single-note endpoint
	protected.Delete("/bookcase/:id", handlers.RemoveBook)
	protected.Get("/bookcase/available", handlers.GetAvailableBooks)
	protected.Put("/bookcase/:id/folder", handlers.UpdateBookFolder)
	
	// Bookcase Notes (One book, multiple notes)
	protected.Get("/bookcase/:id/notes", handlers.GetBookNotes)
	protected.Post("/bookcase/:id/notes", handlers.AddBookNote)
	protected.Put("/bookcase/notes/:note_id", handlers.UpdateBookNote)
	protected.Delete("/bookcase/notes/:note_id", handlers.RemoveBookNote)

	// 🖥️ DESK (Work Desk & Shelves)
	protected.Get("/desk/shelves", handlers.GetShelves)
	protected.Post("/desk/shelves", handlers.CreateShelf)
	protected.Put("/desk/shelves/:id", handlers.UpdateShelf)
	protected.Delete("/desk/shelves/:id", handlers.DeleteShelf)
	protected.Post("/desk/shelves/:id/duplicate", handlers.DuplicateShelf)
	
	protected.Get("/desk/items", handlers.GetDeskItems)
	protected.Post("/desk/items", handlers.AddDeskItem)
	protected.Put("/desk/items/:id", handlers.UpdateDeskItem)
	protected.Delete("/desk/items/:id", handlers.DeleteDeskItem)

	protected.Post("/bulletin", handlers.UpdateBulletin)
	protected.Post("/calendar", handlers.UpdateCalendarEvent)

	// --- 4. Admin Only ---
	admin := protected.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/devices", handlers.GetDevices)
	admin.Put("/devices/status", handlers.UpdateDeviceStatus)
	admin.Delete("/devices/:id", handlers.DeleteDevice)
	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users/role", handlers.UpdateUserRole)
	admin.Post("/settings", handlers.UpdateSetting)

	// --- 5. Toby Only ---
	toby := protected.Group("/", handlers.TobyOnlyMiddleware)
	toby.Get("/bot/requests", handlers.GetPendingBotRequests)
	toby.Post("/bot/approve", handlers.ApproveBotRequest)
	toby.Get("/bot/users", handlers.GetAuthorizedBotUsers)
	toby.Post("/bot/users/delete", handlers.DeleteAuthorizedBotUser)
	toby.Delete("/passwords/:id", handlers.DeletePassword)

	// --- 6. 🔐 2FA / TOTP Management ---
	authShared.Get("/auth/2fa/status", handlers.GetTOTPStatus)
	authShared.Post("/auth/2fa/setup", handlers.SetupTOTP)
	authShared.Post("/auth/2fa/enable", handlers.VerifyAndEnableTOTP)
	authShared.Post("/auth/2fa/verify", handlers.AuthenticateTOTP)

	// 📑 PASSWORD VAULT (Protected by 2FA)
	passVault := protected.Group("/passwords", handlers.TOTPCheckMiddleware)
	passVault.Get("", handlers.GetPasswords)
	passVault.Post("", handlers.AddPassword)
	passVault.Delete("/:id", handlers.DeletePassword)

	app.All("/socket.io/*", adaptor.HTTPHandler(sockets.Server))
	migrateBotAdmins()

	port := os.Getenv("PORT")
	if port == "" { port = "3000" }
	
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\n🛑 Kitty Master Suite shutting down...")
		_ = app.Shutdown()
		bots.BotManager.StopAll(context.Background())
	}()

	log.Printf("🚀 Super Kitty (Save & Search Restored!) running at port %s", port)
	log.Fatal(app.Listen("0.0.0.0:" + port))
}

func migrateBotAdmins() {
	if database.LocalDB == nil { return }
	admins := []struct{ p, k string }{{"telegram", "ADMIN_TELEGRAM_ID"}, {"discord", "ADMIN_DISCORD_ID"}, {"line", "ADMIN_LINE_ID"}}
	for _, a := range admins {
		id := os.Getenv(a.k)
		if id != "" {
			database.LocalDB.Exec(context.Background(), `INSERT INTO bot_authorized_users (platform, account_id, account_name, role) VALUES ($1, $2, 'Admin From Env', 'superadmin') ON CONFLICT (platform, account_id) DO UPDATE SET role = 'superadmin'`, a.p, id)
		}
	}
}
