package bots

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"strconv"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/toydogcat/kitty-help/go-server/database"
	"github.com/toydogcat/kitty-help/go-server/sockets"
)

type TelegramBot struct {
	*BaseChannel
	bot              *telego.Bot
	storehouseChatID int64
}

func NewTelegramBot(token string, admins []string, storehouseID int64) (*TelegramBot, error) {
	bot, err := telego.NewBot(token)
	if err != nil {
		return nil, err
	}

	return &TelegramBot{
		BaseChannel:      NewBaseChannel("telegram", admins),
		bot:              bot,
		storehouseChatID: storehouseID,
	}, nil
}

func (t *TelegramBot) Start(ctx context.Context) error {
	updates, err := t.bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		return err
	}

	t.SetRunning(true)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				if update.Message != nil {
					t.handleMessage(ctx, update.Message)
				}
			}
		}
	}()

	log.Printf("✅ Telegram Bot (@%s) started in polling mode", t.bot.Username())
	return nil
}

func (t *TelegramBot) Stop(ctx context.Context) error {
	t.SetRunning(false)
	return nil
}

func (t *TelegramBot) handleMessage(ctx context.Context, msg *telego.Message) {
	userID := fmt.Sprintf("%d", msg.From.ID)
	senderName := fmt.Sprintf("%s %s", msg.From.FirstName, msg.From.LastName)
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	log.Printf("📩 Telegram message from %s (ID: %s): %s", senderName, userID, text)

	isGeneral, isAdmin, content := t.ParseTriggers(text)
	isAuthorized := t.IsAuthorized(userID)
	isSuper := t.IsAdmin(userID)

	// 0. Handle Join/Link Request (Allow even for authorized users to facilitate linking)
	if text == "我請求加入" {
		userName := fmt.Sprintf("%s %s", msg.From.FirstName, msg.From.LastName)
		token, err := t.GenerateJoinToken(userID, userName)
		if err != nil {
			t.Reply(msg.Chat.ID, msg.MessageID, "❌ 系統繁忙，請稍後再試。")
			return
		}
		t.Reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf("🐱 您的驗證碼是: **%s**\n\n請前往登入頁面輸入驗證碼：\nhttp://localhost:5173/\n\n填寫後請等待 AdminToby 審核以完成帳號綁定。", token))
		return
	}

	if !isSuper && !isAuthorized {
		t.Reply(msg.Chat.ID, msg.MessageID, "🐾 你好！我是 Kitty-Help 小貓助理。目前您尚未獲得授權。\n\n如果要驗證請打：**我請求加入**")
		return
	}

	if isAdmin {
		if !isSuper {
			t.Reply(msg.Chat.ID, msg.MessageID, "⚠️ Unauthorized: SuperAdmin only.")
			return
		}
		t.handleAdminCommand(msg.Chat.ID, msg.MessageID, content)
		return
	}

	if isGeneral {
		t.handleGeneralCommand(msg.Chat.ID, msg.MessageID, content)
		t.LogChat(ctx, userID, senderName, text, "text", nil)
		return
	}

	msgType := "text"
	var mediaID *string
	if msg.Photo != nil || msg.Document != nil || msg.Video != nil || msg.Audio != nil {
		id := t.forwardToStorehouse(msg)
		if id != "" {
			mediaID = &id
			msgType = "media"
		}
	}

	t.LogChat(ctx, userID, senderName, text, msgType, mediaID)
}

func (t *TelegramBot) handleAdminCommand(chatID int64, msgID int, cmd string) {
	switch cmd {
	case "ping":
		t.Reply(chatID, msgID, "pong! Admin is recognized.")
	case "status":
		t.Reply(chatID, msgID, "🚀 Super Kitty Backend (Go) is operational.")
	default:
		t.Reply(chatID, msgID, fmt.Sprintf("❓ Unknown admin command: %s", cmd))
	}
}

func (t *TelegramBot) handleGeneralCommand(chatID int64, msgID int, cmd string) {
	if cmd == "help" {
		t.Reply(chatID, msgID, "🐱 **Kitty-Help General Services**\n\nUsage:\n- `/cat <text>` : Sync text to shared clipboard")
		return
	}
	
	targetDB := database.LocalDB
	if targetDB == nil {
		targetDB = database.CloudDB
	}

	_, _ = targetDB.Exec(context.Background(), "UPDATE common_state SET content = $1, updated_at = CURRENT_TIMESTAMP WHERE key = 'text'", cmd)
	targetDB.Exec(context.Background(), "INSERT INTO common_text_history (content, user_id) VALUES ($1, 'telegram-bot')", cmd)
	
	sockets.Broadcast("commonUpdate", map[string]interface{}{
		"key":     "text",
		"content": cmd,
	})

	t.Reply(chatID, msgID, fmt.Sprintf("✅ Clipboard synced: %s", cmd))
}

func (t *TelegramBot) forwardToStorehouse(msg *telego.Message) string {
	if t.storehouseChatID == 0 {
		return ""
	}

	params := &telego.ForwardMessageParams{
		ChatID:     tu.ID(t.storehouseChatID),
		FromChatID: tu.ID(msg.Chat.ID),
		MessageID:  msg.MessageID,
	}
	_, err := t.bot.ForwardMessage(context.Background(), params)
	if err != nil {
		log.Printf("❌ Failed to forward to storehouse: %v", err)
		return ""
	}

	var fileID string
	var mediaType string
	caption := msg.Caption
	if caption == "" {
		caption = msg.Text
	}

	if msg.Photo != nil {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
		mediaType = "photo"
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		mediaType = "document"
	} else if msg.Video != nil {
		fileID = msg.Video.FileID
		mediaType = "video"
	} else if msg.Audio != nil {
		fileID = msg.Audio.FileID
		mediaType = "audio"
	}

	if fileID == "" {
		return ""
	}

	sourcePlatform := "telegram"
	senderName := fmt.Sprintf("%s %s", msg.From.FirstName, msg.From.LastName)
	senderID := fmt.Sprintf("%d", msg.From.ID)

	title := ""
	if msg.Document != nil {
		title = msg.Document.FileName
	}

	targetDB := database.LocalDB
	if targetDB == nil {
		targetDB = database.CloudDB
	}

	if targetDB == nil {
		log.Printf("❌ No database connection available")
		return ""
	}

	var mediaID string
	err = targetDB.QueryRow(context.Background(),
		"INSERT INTO media_archives (file_id, message_id, media_type, title, caption, source_platform, sender_name, sender_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
		fileID, msg.MessageID, mediaType, title, caption, sourcePlatform, senderName, senderID).Scan(&mediaID)

	if err == nil {
		log.Printf("📦 Recorded media to storehouse: %s (%s)", fileID, sourcePlatform)
		
		// Format and send the "Media Backup" message to the group itself if this isn't already the storehouse
		if msg.Chat.ID != t.storehouseChatID {
			backupMsg := fmt.Sprintf("📦 **Media Backup**\n\n**Source Platform**: `telegram`\n**Sender ID**: `%s`\n**Chat ID**: `%d`\n**Timestamp**: `%s`\n**Username**: %s\n**Content**: %s",
				senderID, msg.Chat.ID, time.Now().Format("2006-01-02 15:04:05"), senderName, caption)
			t.Reply(t.storehouseChatID, 0, backupMsg)
		}

		// Run Async SOP Pre-check if needed or indexable
		go func(fid, cat string) {
			ctxSOP, cancelSOP := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancelSOP()
			
			// For now, let's just mark it as not_indexed
			targetDB.Exec(ctxSOP, "UPDATE media_archives SET index_status = $1 WHERE file_id = $2", 
				"not_indexed", fid)
		}(fileID, mediaType)

		sockets.Broadcast("storehouseUpdate", map[string]interface{}{ "status": "new_item" })
		return mediaID
	} else {
		log.Printf("❌ Failed to record media: %v", err)
	}
	return ""
}

func (t *TelegramBot) Reply(chatID int64, msgID int, text string) {
	params := tu.Message(tu.ID(chatID), text)
	params.ReplyParameters = &telego.ReplyParameters{
		MessageID: msgID,
	}
	_, _ = t.bot.SendMessage(context.Background(), params)
}

func (t *TelegramBot) GetFile(ctx context.Context, fileID string) ([]byte, string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()

	f, err := t.bot.GetFile(dlCtx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, "", err
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, f.FilePath)
	log.Printf("📥 [Telegram] Downloading file from: %s", downloadURL)

	req, _ := http.NewRequestWithContext(dlCtx, "GET", downloadURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("failed to download: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	log.Printf("✅ [Telegram] Downloaded %d bytes, Content-Type: %s", len(data), contentType)

	return data, contentType, nil
}

// Custom NamedReader for telego upload
type namedReader struct {
	io.Reader
	name string
}

func (r namedReader) Name() string {
	return r.name
}

func (t *TelegramBot) UploadMedia(chatID int64, reader io.Reader, filename string, mediaType string, caption string) (string, error) {
	var m *telego.Message
	var err error
	
	// Wrap reader in NamedReader so telego can determine the filename
	file := telego.InputFile{
		File: namedReader{Reader: reader, name: filename},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	switch mediaType {
	case "photo":
		params := tu.Photo(tu.ID(chatID), file).WithCaption(caption)
		m, err = t.bot.SendPhoto(ctx, params)
	case "video":
		params := tu.Video(tu.ID(chatID), file).WithCaption(caption)
		m, err = t.bot.SendVideo(ctx, params)
	default:
		params := tu.Document(tu.ID(chatID), file).WithCaption(caption)
		m, err = t.bot.SendDocument(ctx, params)
	}

	if err != nil {
		return "", err
	}

	// Get the file ID from the sent message
	if m.Photo != nil {
		return m.Photo[len(m.Photo)-1].FileID, nil
	} else if m.Document != nil {
		return m.Document.FileID, nil
	} else if m.Video != nil {
		return m.Video.FileID, nil
	} else if m.Audio != nil {
		return m.Audio.FileID, nil
	}
	return "", fmt.Errorf("unknown media type in response")
}

func (t *TelegramBot) SendMessage(targetID string, text string) error {
	chatID, err := strconv.ParseInt(targetID, 10, 64)
	if err != nil {
		return err
	}
	_, err = t.bot.SendMessage(context.Background(), tu.Message(tu.ID(chatID), text))
	return err
}
