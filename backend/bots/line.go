package bots

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/security"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

type LineBot struct {
	*BaseChannel
	Bot           *linebot.Client
	channelSecret string
}

func NewLineBot(secret, token string, admins []string) (*LineBot, error) {
	bot, err := linebot.New(secret, token)
	if err != nil {
		return nil, err
	}

	return &LineBot{
		BaseChannel:   NewBaseChannel("line", admins),
		Bot:           bot,
		channelSecret: secret,
	}, nil
}

func (l *LineBot) Start(ctx context.Context) error {
	l.SetRunning(true)
	log.Printf("✅ LINE Bot channel ready (Waiting for webhooks)")
	return nil
}

func (l *LineBot) Stop(ctx context.Context) error {
	l.SetRunning(false)
	return nil
}

// validateSignature manually verifies the LINE message signature
func (l *LineBot) validateSignature(signature string, body []byte) bool {
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	hash := hmac.New(sha256.New, []byte(l.channelSecret))
	hash.Write(body)
	return hmac.Equal(hash.Sum(nil), decoded)
}

// HandleFiberWebhook handles LINE webhooks using Fiber context directly
func (l *LineBot) HandleFiberWebhook(c *fiber.Ctx) error {
	signature := c.Get("X-Line-Signature")
	body := c.Body()

	log.Printf("📩 Received LINE Webhook: %d bytes (Signature: %s)", len(body), signature)

	// 1. Validate Signature
	if !l.validateSignature(signature, body) {
		log.Printf("⚠️ LINE Webhook Error: Invalid Signature!")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid Signature")
	}

	// 2. Unmarshal JSON
	var request struct {
		Events []*linebot.Event `json:"events"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		log.Printf("❌ Failed to unmarshal LINE request: %v", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid JSON")
	}

	log.Printf("✅ Parsed %d LINE events", len(request.Events))

	for _, event := range request.Events {
		log.Printf("🔹 LINE Event Type: %s from User: %s", event.Type, event.Source.UserID)
		if event.Type == linebot.EventTypeMessage {
			senderName := "LINE User"
			profile, err := l.Bot.GetProfile(event.Source.UserID).Do()
			if err == nil {
				senderName = profile.DisplayName
			}

			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				log.Printf("📩 Received LINE Message: %s", message.Text)
				l.handleTextMessage(event.ReplyToken, event.Source.UserID, senderName, message.Text)
			case *linebot.ImageMessage:
				id := l.forwardToStorehouse(event, "photo")
				l.LogChat(context.Background(), event.Source.UserID, senderName, "[Image]", "media", &id)
			case *linebot.VideoMessage:
				id := l.forwardToStorehouse(event, "video")
				l.LogChat(context.Background(), event.Source.UserID, senderName, "[Video]", "media", &id)
			case *linebot.AudioMessage:
				id := l.forwardToStorehouse(event, "audio")
				l.LogChat(context.Background(), event.Source.UserID, senderName, "[Audio]", "media", &id)
			}
		} else {
			log.Printf("ℹ️ Non-message LINE event: %s", event.Type)
		}
	}
	return c.SendString("OK")
}

func (l *LineBot) handleTextMessage(replyToken, userID, senderName string, text string) {
	isGeneral, isAdmin, content := l.ParseTriggers(text)

	// Determine Authorization
	isAuthorized := l.IsAuthorized(userID)
	isSuper := l.IsAdmin(userID)

	// 0. Handle Join/Link Request (Allow even for authorized users to facilitate linking)
	if text == "我請求加入" {
		token, err := l.GenerateJoinToken(userID, senderName)
		if err != nil {
			l.Reply(replyToken, "❌ 系統繁忙，請稍後再試。")
			return
		}
		l.Reply(replyToken, fmt.Sprintf("🐱 您的驗證碼是: **%s**\n\n請前往登入頁面輸入驗證碼：\nhttp://localhost:5173/\n\n填寫後請等待 AdminToby 審核以完成帳號綁定。", token))
		return
	}

	// 1. Handle Unrecognized Users
	if !isSuper && !isAuthorized {
		l.Reply(replyToken, "🐾 你好！我是 Kitty-Help 小貓助理。目前您尚未獲得授權。\n\n如果要驗證請打：**我請求加入**")
		return
	}

	if isAdmin {
		if !isSuper {
			l.Reply(replyToken, "⚠️ Unauthorized: SuperAdmin only.")
			return
		}
		l.handleAdminCommand(replyToken, content)
		l.LogChat(context.Background(), userID, senderName, text, "text", nil)
		return
	}

	// Security: /verify <token>
	if len(text) >= 8 && text[:7] == "/verify" {
		token := text[8:]
		msg, err := security.HandleBotVerify("line", userID, token)
		if err != nil {
			l.Reply(replyToken, "❌ 驗證失敗：系統錯誤。")
			return
		}
		l.Reply(replyToken, msg)
		l.LogChat(context.Background(), userID, senderName, text, "text", nil)
		return
	}

	if isGeneral {
		l.handleGeneralCommand(replyToken, content, userID)
		l.LogChat(context.Background(), userID, senderName, text, "text", nil)
		return
	}

	l.LogChat(context.Background(), userID, senderName, text, "text", nil)
}

func (l *LineBot) handleAdminCommand(replyToken, cmd string) {
	switch cmd {
	case "ping":
		l.Reply(replyToken, "pong! Admin is recognized.")
	case "help":
		l.Reply(replyToken, "🐱 **Kitty-Help Admin Commands**\n\n- `!help`: Show this list\n- `!ping`: Test responsiveness\n- `!status`: Check backend status\n- `!webhook`: Get LINE webhook URL")
	case "webhook":
		url := l.GetWebhookURL()
		l.Reply(replyToken, fmt.Sprintf("🔗 **Webhook URL (LINE)**\n\n`%s/webhook/line`", url))
	default:
		l.Reply(replyToken, fmt.Sprintf("❓ Unknown admin command: %s", cmd))
	}
}

func (l *LineBot) handleGeneralCommand(replyToken, cmd string, userID string) {
	// Case 1: News Command
	if strings.HasPrefix(cmd, "news") {
		args := strings.TrimSpace(strings.TrimPrefix(cmd, "news"))
		openCLIArgs := "news"
		switch args {
		case "1": openCLIArgs = "news top"
		case "2": openCLIArgs = "news ai"
		case "3": openCLIArgs = "news bbc"
		}
		
		output, err := l.GetNewsFromWorker(openCLIArgs)
		if err != nil {
			l.Reply(replyToken, "❌ 無法聯絡文書機，請確認 Tailscale 連線。")
			return
		}
		l.Reply(replyToken, fmt.Sprintf("📰 **Kitty News (%s)**\n\n%s", openCLIArgs, output))
		return
	}

	// Case 2: Cross-platform messaging (L2D)
	if strings.HasPrefix(cmd, "d ") {
		msg := strings.TrimSpace(strings.TrimPrefix(cmd, "d "))
		dsBotIf, ok := BotManager.Get("discord")
		if ok {
			// Find the shared channel from settings/env or use a specific one
			// For now, we'll try to find an admin chat ID or a fixed one
			targetChannel := os.Getenv("DISCORD_ADMIN_CHANNEL_ID") 
			if targetChannel != "" {
				err := dsBotIf.SendMessage(targetChannel, fmt.Sprintf("🐱 **LINE Message** from [%s]:\n%s", userID, msg))
				if err == nil {
					l.Reply(replyToken, "✅ 訊息已轉發至 Discord。")
					return
				}
			}
		}
		l.Reply(replyToken, "❌ 轉發失敗，請檢查 Discord 設定。")
		return
	}

	if cmd == "help" {
		l.Reply(replyToken, "🐱 **Kitty-Help General Services**\n\n- `/cat news [1|2|3]` : Get latest news\n- `/cat d <text>` : Send to Discord\n- `/cat <text>` : Sync to shared clipboard")
		return
	}

	// Default: Sync to shared clipboard
	query := "UPDATE common_state SET content = $1, updated_at = CURRENT_TIMESTAMP WHERE key = 'text'"
	_, err := database.CloudDB.Exec(context.Background(), query, cmd)
	if err != nil {
		l.Reply(replyToken, "❌ Failed to sync text to clipboard.")
		return
	}
	
	// History
	database.CloudDB.Exec(context.Background(), "INSERT INTO common_text_history (content, user_id) VALUES ($1, 'line-bot')", cmd)

	// Broadcast
	sockets.Broadcast("commonUpdate", map[string]interface{}{
		"key":     "text",
		"content": cmd,
	})

	l.Reply(replyToken, fmt.Sprintf("✅ Clipboard synced (LINE): %s", cmd))
}

func (l *LineBot) forwardToStorehouse(event *linebot.Event, mediaType string) string {
	tgBotIf, ok := BotManager.Get("telegram")
	var tgBot *TelegramBot
	if ok {
		tgBot = tgBotIf.(*TelegramBot)
	}

	if tgBot == nil || tgBot.storehouseChatID == 0 {
		log.Printf("⚠️ Telegram Storehouse not available for LINE backup")
		return ""
	}

	// 1. Fetch content from LINE
	var messageID string
	var caption string
	switch msg := event.Message.(type) {
	case *linebot.ImageMessage:
		messageID = msg.ID
	case *linebot.VideoMessage:
		messageID = msg.ID
	case *linebot.AudioMessage:
		messageID = msg.ID
	}

	contentResp, err := l.Bot.GetMessageContent(messageID).Do()
	if err != nil {
		log.Printf("❌ Failed to fetch LINE content: %v", err)
		return ""
	}
	defer contentResp.Content.Close()

	// 2. Fetch User Profile for metadata
	senderName := "LINE User"
	profile, err := l.Bot.GetProfile(event.Source.UserID).Do()
	if err == nil {
		senderName = profile.DisplayName
	}

	// 3. Cloud Backup to Telegram Storehouse
	telegramFileID := ""
	syncBotIf, isSyncBotFound := BotManager.Get("telegram")
	if isSyncBotFound {
		tgBot := syncBotIf.(*TelegramBot)
		backupMessageBody := fmt.Sprintf("📦 **Media Backup**\n\n**Source Platform**: `line`\n**Sender**: `%s`\n**Timestamp**: `%s`\n**Content**: %s",
			senderName, time.Now().Format("2006-01-02 15:04:05"), caption)

		cloudID, err := tgBot.UploadMedia(tgBot.storehouseChatID, contentResp.Content, "line_"+messageID, mediaType, backupMessageBody)
		if err == nil {
			telegramFileID = cloudID
			log.Printf("☁️ Cloud Backup SUCCESS: %s", cloudID)
		} else {
			log.Printf("❌ Cloud Backup FAIL: %v", err)
		}
	}

	// 4. Insert into DB (Save Telegram ID if exists, but keep source_platform as LINE)
	finalFileID := telegramFileID
	if finalFileID == "" {
		finalFileID = messageID
	}

	targetDB := database.LocalDB
	if targetDB == nil { targetDB = database.CloudDB }
	if targetDB == nil { return "" }

	var mediaID string
	err = targetDB.QueryRow(context.Background(),
		"INSERT INTO media_archives (file_id, message_id, media_type, caption, source_platform, sender_name, sender_id, is_indexable) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
		finalFileID, 0, mediaType, caption, "line", senderName, event.Source.UserID, true).Scan(&mediaID)

	if err != nil {
		log.Printf("❌ Failed to record LINE media: %v", err)
		return ""
	}

	log.Printf("📦 Recorded LINE-to-Telegram media: %s (%s)", telegramFileID, senderName)
	
	// NEW: Auto-push to Impression Discovery Queue for authorized users
	unifiedID, err := l.GetUnifiedUserID(context.Background(), event.Source.UserID)
	if err == nil && unifiedID != "" {
		_, err = targetDB.Exec(context.Background(), 
			"INSERT INTO impression_temp (media_id, user_id, title) VALUES ($1, $2, $3) ON CONFLICT (media_id) DO NOTHING", 
			mediaID, unifiedID, fmt.Sprintf("Sync from LINE: %s", time.Now().Format("15:04:05")))
		if err == nil {
			log.Printf("🌌 Auto-indexed to Impression Discovery for %s", senderName)
			sockets.Broadcast("discoveryUpdate", map[string]interface{}{ "status": "new_discovery" })
		}
	}

	sockets.Broadcast("storehouseUpdate", map[string]interface{}{ "status": "new_item" })
	return mediaID
}

func (l *LineBot) Reply(replyToken, text string) {
	_, err := l.Bot.ReplyMessage(replyToken, linebot.NewTextMessage(text)).Do()
	if err != nil {
		log.Printf("❌ Failed to reply to LINE: %v", err)
	}
}

func (l *LineBot) SendMessage(targetID string, text string) error {
	_, err := l.Bot.PushMessage(targetID, linebot.NewTextMessage(text)).Do()
	return err
}

func (l *LineBot) SendMedia(targetID string, mediaType string, filePath string, caption string) error {
	baseURL := l.GetWebhookURL()
	fileURL := fmt.Sprintf("%s/uploads/%s", baseURL, filepath.Base(filePath))

	var msg linebot.SendingMessage
	if mediaType == "photo" || mediaType == "image" {
		msg = linebot.NewImageMessage(fileURL, fileURL)
	} else if mediaType == "video" {
		msg = linebot.NewVideoMessage(fileURL, fileURL)
	} else {
		// Fallback for generic files if NewFileMessage has issues
		msg = linebot.NewTextMessage(fmt.Sprintf("📄 傳送檔案: %s\n🔗 連結: %s", filepath.Base(filePath), fileURL))
	}

	_, err := l.Bot.PushMessage(targetID, msg).Do()
	if err != nil { return err }

	if caption != "" && mediaType != "text" {
		_, _ = l.Bot.PushMessage(targetID, linebot.NewTextMessage(caption)).Do()
	}
	return nil
}
