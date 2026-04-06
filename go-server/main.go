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
		AppName:      "Kitty-Help Master Suite 4.0",
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://kitty-help.web.app, http://localhost:5173, http://localhost:4173",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Refresh-Token, cf-skip-browser-warning, ngrok-skip-browser-warning",
		ExposeHeaders:    "X-Refresh-Token, Content-Disposition",
		AllowCredentials: true,
	}))

	app.Use(logger.New())
	app.Static("/uploads", "../uploads")
	api := app.Group("/api")

	// --- PUBLIC ---
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	api.Get("/settings", handlers.GetSettings)
	api.Post("/devices/register", handlers.RegisterDevice)
	api.Get("/storehouse", handlers.GetStorehouseItems)
	api.Get("/storehouse/file/:fileID", handlers.GetFileProxy)
	api.Post("/opencli", handlers.ProxyOpenCLI)
	if lineBotInstance != nil { app.Post("/webhook/line", lineBotInstance.HandleFiberWebhook) }

	// --- PROTECTED (JWT + Sliding Session) ---
	authShared := api.Group("/", handlers.JWTMiddleware)
	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)
	authShared.Get("/chat/logs", handlers.GetChatLogs)

	// --- DEVICE PROTECTED ---
	protected := authShared.Group("/", handlers.DeviceCheckMiddleware)
	protected.Get("/snippets", handlers.GetSnippets)
	protected.Post("/snippets", handlers.CreateSnippet)
	protected.Put("/snippets/:id", handlers.UpdateSnippet)
	protected.Delete("/snippets/:id", handlers.DeleteSnippet)
	protected.Get("/bookmarks", handlers.GetBookmarks)
	protected.Post("/bookmarks", handlers.CreateBookmark)
	protected.Put("/bookmarks/:id", handlers.UpdateBookmark)
	protected.Delete("/bookmarks/:id", handlers.DeleteBookmark)
	
	// 🧠 IMPRESSION COMPLETELY RESTORED
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

	protected.Post("/bulletin", handlers.UpdateBulletin)
	protected.Post("/calendar", handlers.UpdateCalendarEvent)

	// --- ADMIN ONLY ---
	admin := protected.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/devices", handlers.GetDevices)
	admin.Put("/devices/status", handlers.UpdateDeviceStatus)
	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users/role", handlers.UpdateUserRole)
	admin.Post("/settings", handlers.UpdateSetting)

	// --- TOBY ONLY ---
	toby := protected.Group("/", handlers.TobyOnlyMiddleware)
	toby.Get("/bot/requests", handlers.GetPendingBotRequests)
	toby.Post("/bot/approve", handlers.ApproveBotRequest)
	toby.Get("/passwords", handlers.GetPasswords)
	toby.Post("/passwords", handlers.AddPassword)
	toby.Delete("/passwords/:id", handlers.DeletePassword)

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

	log.Printf("🚀 Super Kitty (Impress Fully Restored!) running at port %s", port)
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
