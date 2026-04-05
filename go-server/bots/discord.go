package bots

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/security"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

type DiscordBot struct {
	*BaseChannel
	session *discordgo.Session
}

func NewDiscordBot(token string, admins []string) (*DiscordBot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	return &DiscordBot{
		BaseChannel: NewBaseChannel("discord", admins),
		session:     dg,
	}, nil
}

func (d *DiscordBot) Start(ctx context.Context) error {
	d.session.AddHandler(d.handleMessage)
	d.session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	
	if err := d.session.Open(); err != nil {
		return err
	}

	d.SetRunning(true)
	username := "KittyHelp"
	if d.session.State != nil && d.session.State.User != nil {
		username = d.session.State.User.Username
	}
	log.Printf("✅ Discord Bot (@%s) started", username)
	return nil
}

func (d *DiscordBot) Stop(ctx context.Context) error {
	d.SetRunning(false)
	return d.session.Close()
}

func (d *DiscordBot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("📩 Discord message from %s (ID: %s): %s", m.Author.Username, m.Author.ID, m.Content)

	isGeneral, isAdmin, content := d.ParseTriggers(m.Content)

	// Determine Authorization
	isAuthorized := d.IsAuthorized(m.Author.ID)
	isSuper := d.IsAdmin(m.Author.ID)

	// 0. Handle Join/Link Request (Allow even for authorized users to facilitate linking)
	if m.Content == "我請求加入" {
		token, err := d.GenerateJoinToken(m.Author.ID, m.Author.Username)
		if err != nil {
			s.ChannelMessageSendReply(m.ChannelID, "❌ 系統繁忙，請稍後再試。", m.Reference())
			return
		}
		s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("🐱 您的驗證碼是: **%s**\n\n請前往登入頁面輸入驗證碼：\nhttp://localhost:5173/\n\n填寫後請等待 AdminToby 審核以完成帳號綁定。", token), m.Reference())
		return
	}

	// 1. Handle Unrecognized Users
	if !isSuper && !isAuthorized {
		s.ChannelMessageSendReply(m.ChannelID, "🐾 你好！我是 Kitty-Help 小貓助理。目前您尚未獲得授權。\n\n如果要驗證請打：**我請求加入**", m.Reference())
		return
	}

	// 2. Handle Admin Commands
	if isAdmin {
		if !isSuper {
			s.ChannelMessageSendReply(m.ChannelID, "⚠️ Unauthorized: SuperAdmin only.", m.Reference())
			return
		}
		d.handleAdminCommand(s, m, content)
		return
	}

	// Security: /verify <token> (Legacy)
	if len(m.Content) >= 8 && m.Content[:7] == "/verify" {
		token := m.Content[8:]
		msg, err := security.HandleBotVerify("discord", m.Author.ID, token)
		if err != nil {
			s.ChannelMessageSendReply(m.ChannelID, "❌ 驗證失敗：系統錯誤。", m.Reference())
			return
		}
		s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
		return
	}

	if isGeneral {
		d.handleGeneralCommand(s, m, content)
		d.LogChat(context.Background(), m.Author.ID, m.Author.Username, m.Content, "text", nil)
		return
	}

	msgType := "text"
	var mediaID *string
	if len(m.Attachments) > 0 {
		id := d.forwardToStorehouse(s, m)
		if id != "" {
			mediaID = &id
			msgType = "media"
		}
	}

	d.LogChat(context.Background(), m.Author.ID, m.Author.Username, m.Content, msgType, mediaID)
}

func (d *DiscordBot) handleAdminCommand(s *discordgo.Session, m *discordgo.MessageCreate, cmd string) {
	switch cmd {
	case "ping":
		s.ChannelMessageSendReply(m.ChannelID, "pong! Admin is recognized.", m.Reference())
	case "help":
		helpText := "🐱 **Kitty-Help Admin Commands (Discord)**\n\n- `!help`: Show this list\n- `!ping`: Test responsiveness\n- `!webhook`: Get current tunnel URL"
		s.ChannelMessageSendReply(m.ChannelID, helpText, m.Reference())
	default:
		s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("❓ Unknown admin command: %s", cmd), m.Reference())
	}
}

func (d *DiscordBot) handleGeneralCommand(s *discordgo.Session, m *discordgo.MessageCreate, cmd string) {
	if cmd == "help" {
		s.ChannelMessageSendReply(m.ChannelID, "🐱 **Kitty-Help General Services**\n\nUsage:\n- `/cat <text>` : Sync text to shared clipboard\n- `/cat status` : Check system status", m.Reference())
		return
	}

	// Default: Sync to shared clipboard
	query := "UPDATE common_state SET content = $1, updated_at = CURRENT_TIMESTAMP WHERE key = 'text'"
	_, err := database.CloudDB.Exec(context.Background(), query, cmd)
	if err != nil {
		s.ChannelMessageSendReply(m.ChannelID, "❌ Failed to sync text to clipboard.", m.Reference())
		return
	}
	
	// Also add to history (CloudDB)
	database.CloudDB.Exec(context.Background(), "INSERT INTO common_text_history (content, user_id) VALUES ($1, 'discord-bot')", cmd)

	// Broadcast
	sockets.Broadcast("commonUpdate", map[string]interface{}{
		"key":     "text",
		"content": cmd,
	})

	s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("✅ Clipboard synced: %s", cmd), m.Reference())
}

func (d *DiscordBot) forwardToStorehouse(s *discordgo.Session, m *discordgo.MessageCreate) string {
	tgBotIf, ok := BotManager.Get("telegram")
	var tgBot *TelegramBot
	if ok {
		tgBot = tgBotIf.(*TelegramBot)
	}

	var lastMediaID string
	for _, att := range m.Attachments {
		mediaType := "photo"
		ext := filepath.Ext(att.Filename)
		if ext == ".mp4" || ext == ".mov" || ext == ".webm" {
			mediaType = "video"
		} else if ext == ".mp3" || ext == ".wav" || ext == ".ogg" {
			mediaType = "audio"
		}

		// 1. Upload to Telegram Storehouse
		resp, err := http.Get(att.URL)
		if err != nil {
			log.Printf("❌ Failed to download Discord attachment: %v", err)
			continue
		}
		defer resp.Body.Close()

		// 2. Format backup message
		backupMsg := fmt.Sprintf("📦 **Media Backup**\n\n**Source Platform**: `discord`\n**Sender ID**: `%s`\n**Chat ID**: `%s`\n**Timestamp**: `%s`\n**Username**: @%s\n**Content**: %s",
			m.Author.ID, m.ChannelID, time.Now().Format("2006-01-02 15:04:05"), m.Author.Username, m.Content)
		
		var telegramFileID string
		if tgBot != nil && tgBot.storehouseChatID != 0 {
			telegramFileID, err = tgBot.UploadMedia(tgBot.storehouseChatID, resp.Body, att.Filename, mediaType, backupMsg)
			if err != nil {
				log.Printf("❌ Failed to upload Discord media to Telegram: %v", err)
				continue
			}
		} else {
			log.Printf("⚠️ Telegram Storehouse not available for Discord backup")
			continue
		}

		// 3. Insert into DB (using Telegram FileID)
		targetDB := database.LocalDB
		if targetDB == nil { targetDB = database.CloudDB }
		if targetDB == nil { continue }

		msgIDInt, _ := strconv.ParseInt(m.ID, 10, 64)

		var mediaID string
		err = targetDB.QueryRow(context.Background(),
			"INSERT INTO media_archives (file_id, message_id, media_type, caption, source_platform, sender_name, sender_id, is_indexable) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
			telegramFileID, msgIDInt, mediaType, m.Content, "discord", m.Author.Username, m.Author.ID, true).Scan(&mediaID)

		if err != nil {
			log.Printf("❌ Failed to record Discord media: %v", err)
			continue
		}

		lastMediaID = mediaID
		log.Printf("📦 Recorded Discord-to-Telegram media: %s", telegramFileID)
		sockets.Broadcast("storehouseUpdate", map[string]interface{}{ "status": "new_item" })
	}
	return lastMediaID
}
