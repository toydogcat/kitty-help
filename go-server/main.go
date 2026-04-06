package main

import (
	"context"
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

	// Bot Setup
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken != "" {
		tgBot, _ := bots.NewTelegramBot(tgToken, admins, 0)
		bots.BotManager.Register("telegram", tgBot)
	}
	dsToken := os.Getenv("DISCORD_BOT_TOKEN")
	if dsToken != "" {
		dsBot, _ := bots.NewDiscordBot(dsToken, admins)
		bots.BotManager.Register("discord", dsBot)
	}

	bots.BotManager.StartAll(context.Background())

	app := fiber.New(fiber.Config{
		AppName:      "Kitty-Help Go Backend Full",
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

	// Public API
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	api.Post("/devices/register", handlers.RegisterDevice)
	api.Get("/storehouse", handlers.GetStorehouseItems)
	api.Get("/storehouse/file/:fileID", handlers.GetFileProxy)
	api.Post("/opencli", handlers.ProxyOpenCLI)

	// Protected with Sliding Session
	authShared := api.Group("/", handlers.JWTMiddleware)
	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)
	authShared.Get("/chat/logs", handlers.GetChatLogs)

	// Device Protected
	protected := authShared.Group("/", handlers.DeviceCheckMiddleware)
	protected.Get("/snippets", handlers.GetSnippets)
	protected.Post("/snippets", handlers.CreateSnippet)
	protected.Put("/snippets/:id", handlers.UpdateSnippet)
	protected.Delete("/snippets/:id", handlers.DeleteSnippet)
	protected.Get("/bookmarks", handlers.GetBookmarks)
	protected.Post("/bookmarks", handlers.CreateBookmark)
	protected.Get("/impression/graph", handlers.GetImpressionGraph)
	protected.Post("/impression/nodes", handlers.CreateImpressionNode)

	// Admin Only
	admin := protected.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users/role", handlers.UpdateUserRole)
	admin.Get("/devices", handlers.GetDevices)
	admin.Put("/devices/status", handlers.UpdateDeviceStatus)

	app.All("/socket.io/*", adaptor.HTTPHandler(sockets.Server))

	port := os.Getenv("PORT")
	if port == "" { port = "3000" }
	
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		_ = app.Shutdown()
		bots.BotManager.StopAll(context.Background())
	}()

	log.Printf("🚀 Super Kitty (Go) running on all interfaces at port %s", port)
	log.Fatal(app.Listen("0.0.0.0:" + port))
}
