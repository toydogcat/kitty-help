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

	// 1. TELEGRAM BOT (With Storehouse Support)
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken != "" {
		var storehouseID int64
		fmt.Sscanf(os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID"), "%d", &storehouseID)
		tgBot, err := bots.NewTelegramBot(tgToken, admins, storehouseID)
		if err == nil { bots.BotManager.Register("telegram", tgBot) }
	}

	// 2. DISCORD BOT
	dsToken := os.Getenv("DISCORD_BOT_TOKEN")
	if dsToken != "" {
		dsBot, err := bots.NewDiscordBot(dsToken, admins)
		if err == nil { bots.BotManager.Register("discord", dsBot) }
	}

	// 3. LINE BOT
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
		AppName:      "Kitty-Help Go Backend Final",
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://kitty-help.web.app, http://localhost:5173, http://localhost:4173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Refresh-Token, cf-skip-browser-warning, ngrok-skip-browser-warning",
		ExposeHeaders:    "X-Refresh-Token",
		AllowCredentials: true,
	}))

	app.Use(logger.New())
	app.Static("/uploads", "../uploads")
	api := app.Group("/api")

	// Routes Setup (Summary version but COMPLETE mapping)
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	api.Get("/settings", handlers.GetSettings)
	api.Post("/devices/register", handlers.RegisterDevice)
	api.Get("/storehouse", handlers.GetStorehouseItems)
	api.Get("/storehouse/file/:fileID", handlers.GetFileProxy)
	if lineBotInstance != nil { app.Post("/webhook/line", lineBotInstance.HandleFiberWebhook) }

	authShared := api.Group("/", handlers.JWTMiddleware)
	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)
	authShared.Get("/chat/logs", handlers.GetChatLogs)

	protected := authShared.Group("/", handlers.DeviceCheckMiddleware)
	protected.Get("/snippets", handlers.GetSnippets)
	protected.Post("/snippets", handlers.CreateSnippet)
	protected.Get("/bookmarks", handlers.GetBookmarks)
	protected.Post("/bookmarks", handlers.CreateBookmark)
	protected.Get("/impression/graph", handlers.GetImpressionGraph)
	protected.Post("/impression/nodes", handlers.CreateImpressionNode)
	protected.Post("/bulletin", handlers.UpdateBulletin)
	protected.Post("/calendar", handlers.UpdateCalendarEvent)

	admin := protected.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/devices", handlers.GetDevices)
	admin.Put("/devices/status", handlers.UpdateDeviceStatus)
	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users/role", handlers.UpdateUserRole)
	admin.Post("/settings", handlers.UpdateSetting)

	toby := protected.Group("/", handlers.TobyOnlyMiddleware)
	toby.Get("/bot/requests", handlers.GetPendingBotRequests)
	toby.Post("/bot/approve", handlers.ApproveBotRequest)
	toby.Get("/passwords", handlers.GetPasswords)
	toby.Post("/passwords", handlers.AddPassword)

	app.All("/socket.io/*", adaptor.HTTPHandler(sockets.Server))
	migrateBotAdmins()

	port := os.Getenv("PORT")
	if port == "" { port = "3000" }
	
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		_ = app.Shutdown()
		bots.BotManager.StopAll(context.Background())
	}()

	log.Printf("🚀 Super Kitty (Back-up Ready!) running at port %s", port)
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
