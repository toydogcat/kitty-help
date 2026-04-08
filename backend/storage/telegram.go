package storage

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var Bot *tgbotapi.BotAPI
var ChatID int64

func InitTelegram() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Println("Warning: TELEGRAM_BOT_TOKEN not set, storage will fail")
		return
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Telegram Bot initialization failed: %v", err)
		return
	}

	Bot = bot
	// ChatID should be set in .env (e.g., -100123456789)
	// We'll parse it here or use a placeholder.
    // In actual use, fetch this from ENV
    // fmt.Sscanf(os.Getenv("TELEGRAM_STOREHOUSE_CHAT_ID"), "%d", &ChatID)
}

func UploadToTelegram(file multipart.File, header *multipart.FileHeader) (string, string, error) {
	if Bot == nil {
		return "", "", fmt.Errorf("Telegram Bot not initialized")
	}

    // Default to a group ID from ENV if possible
    // chatID := ChatID
	
	// Create File Bytes
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, file); err != nil {
		return "", "", err
	}

	fileBytes := tgbotapi.FileBytes{
		Name:  header.Filename,
		Bytes: buf.Bytes(),
	}

	// Send to Telegram
	// For kitty-help, we might use sendPhoto if it's an image, or sendDocument for everything.
	// Using Document is safer for all file types.
	doc := tgbotapi.NewDocument(ChatID, fileBytes)
    doc.Caption = fmt.Sprintf("📦 Kitty-Help Upload: %s", header.Filename)
	
	msg, err := Bot.Send(doc)
	if err != nil {
		return "", "", err
	}

	// Return FileID and Message Link
	fileID := ""
	if msg.Document != nil {
		fileID = msg.Document.FileID
	} else if len(msg.Photo) > 0 {
		fileID = msg.Photo[len(msg.Photo)-1].FileID
	}

	return fileID, fmt.Sprintf("https://t.me/c/%d/%d", -ChatID-1000000000000, msg.MessageID), nil
}
