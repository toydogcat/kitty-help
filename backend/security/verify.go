package security

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/toydogcat/kitty-help/go-server/database"
)

// GenerateRandomToken creates a 6-character alphanumeric code, excluding confusing chars
func GenerateRandomToken(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// HandleBotVerify internal logic to process /verify commands from bots
func HandleBotVerify(platform, accountId, token string) (string, error) {
	// 1. Find user linked to this bot ID
	var userId string
	column := "line_id"
	if platform == "discord" {
		column = "discord_id"
	}
	
	err := database.LocalDB.QueryRow(context.Background(), 
		fmt.Sprintf("SELECT id FROM users WHERE %s = $1", column), accountId).Scan(&userId)
	if err != nil {
		return "❌ 你的帳號尚未與 Kitty-Help 網頁版連結。請先至網頁版設定頁面進行驗證綁定。", nil
	}

	// 2. Find pending session
	var id string
	var lineAt *time.Time
	err = database.LocalDB.QueryRow(context.Background(),
		"SELECT id, line_verified_at FROM security_sessions WHERE token = $1 AND user_id = $2 AND status = 'pending' AND expires_at > NOW()",
		token, userId).Scan(&id, &lineAt)
	
	if err != nil {
		return "❌ 無效或已過期的驗證碼。請在網頁版重新產生。", nil
	}

	if platform == "line" {
		_, err = database.LocalDB.Exec(context.Background(), 
			"UPDATE security_sessions SET line_verified_at = NOW() WHERE id = $1", id)
		return "✅ [Line] 驗證成功！下一步請到 Discord 輸入同樣的驗證碼完成啟動。", nil
	}

	if platform == "discord" {
		if lineAt == nil {
			return "⚠️ 嚴格驗證中：請先在 Line 通過第一階段驗證！", nil
		}
		// Final step: Grant 30m window
		_, err = database.LocalDB.Exec(context.Background(), 
			"UPDATE security_sessions SET discord_verified_at = NOW(), status = 'granted', granted_at = NOW() WHERE id = $1", id)
		if err != nil {
			return "❌ 啟動失敗，系統錯誤。", err
		}
		return "🎉 雙重驗證達成！書籤密碼已開啟 30 分鐘授權時窗。請點擊網頁書籤查看。", nil
	}

	return "Invalid platform", nil
}
