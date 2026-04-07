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
	MediaType    string    `json:"mediaType,omitempty"`
	IsIntegrated bool      `json:"isIntegrated"`
	CreatedAt    time.Time `json:"createdAt"`
}

type RemarkContainer struct {
	ID        string       `json:"id"`
	UserID    string       `json:"userId"`
	Name      string       `json:"name"`
	Content   string       `json:"content"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Items     []RemarkItem `json:"items,omitempty"`
}

type RemarkItem struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	ContainerID *string   `json:"containerId,omitempty"`
	LogID       int       `json:"logId"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	Log         *ChatLog  `json:"log,omitempty"` // Details needed for frontend
}
