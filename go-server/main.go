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
	"github.com/toydogcat/kitty-help/go-server/handlers"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

func main() {
	_ = godotenv.Load(".env")
	sockets.InitSocketIO()
	bots.InitManager()
	
	// Define admins from env
	admins := []string{"toydogcat@gmail.com"}
	if envAdmins := os.Getenv("ADMIN_EMAILS"); envAdmins != "" {
		admins = append(admins, strings.Split(envAdmins, ",")...)
	}

	// Bot Setup (Minimal snippets for clarity, same logic as before)
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
		AppName:      "Kitty-Help Go Backend Pro",
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	// 🛠️ ROBUST CORS: Standardized whitelist with Credentials support
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://kitty-help.web.app, http://localhost:5173, http://localhost:4173",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Refresh-Token, cf-skip-browser-warning, ngrok-skip-browser-warning",
		ExposeHeaders:    "X-Refresh-Token, Content-Disposition",
		AllowCredentials: true,
	}))

	app.Use(logger.New())

	api := app.Group("/api")
	api.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong 🐱") })

	// Auth & User
	api.Post("/auth/verify", handlers.VerifyFirebaseToken)
	api.Get("/bulletin", handlers.GetBulletin)
	api.Get("/calendar", handlers.GetCalendarEvents)
	api.Post("/devices/register", handlers.RegisterDevice)

	// Protected Group with Sliding Session
	authShared := api.Group("/", handlers.JWTMiddleware)
	authShared.Get("/bot/my-status", handlers.GetMyBotStatus)
	authShared.Post("/bot/link", handlers.LinkBotAccount)

	// Admin Only
	admin := authShared.Group("/", handlers.AdminOnlyMiddleware)
	admin.Get("/users", handlers.GetUsers)
	admin.Post("/users/role", handlers.UpdateUserRole)

	// Sockets
	app.All("/socket.io/*", adaptor.HTTPHandler(sockets.Server))

	// Listen on all interfaces for Cloudflare Tunneling
	port := os.Getenv("PORT")
	if port == "" { port = "3000" }
	
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\n🛑 Gracefully shutting down...")
		_ = app.Shutdown()
		bots.BotManager.StopAll(context.Background())
	}()

	log.Printf("🚀 Super Kitty (Go) running on all interfaces at port %s", port)
	log.Fatal(app.Listen("0.0.0.0:" + port))
}
