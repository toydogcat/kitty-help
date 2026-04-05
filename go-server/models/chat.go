package models

import "time"

type ChatLog struct {
	ID         string    `json:"id"`
	Platform   string    `json:"platform"`
	SenderID   string    `json:"senderId"`
	SenderName string    `json:"senderName"`
	Content    string    `json:"content"`
	MsgType    string    `json:"msgType"`
	MediaID    *string   `json:"mediaId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}
