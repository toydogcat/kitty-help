package models

import "time"

type ImpressionNode struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	MediaID         *string   `json:"mediaId"`
	LinkedSnippetID *string   `json:"linkedSnippetId"`
	DeskShelfID     *string   `json:"deskShelfId"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	NodeType        string    `json:"nodeType"`
	CreatedAt       time.Time `json:"createdAt"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	FileID          *string   `json:"fileId,omitempty"`
	SourcePlatform  *string   `json:"sourcePlatform,omitempty"`
	KGName          string    `json:"kgName"`
}

type ImpressionEdge struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	SourceID  string    `json:"sourceId"`
	TargetID  string    `json:"targetId"`
	Label     string    `json:"label"`
	KGName    string    `json:"kgName"`
	CreatedAt time.Time `json:"createdAt"`
}

type GraphResponse struct {
	Nodes []ImpressionNode `json:"nodes"`
	Edges []ImpressionEdge `json:"edges"`
}
