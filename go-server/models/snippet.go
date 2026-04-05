package models

import "time"

type Snippet struct {
	ID           string    `json:"id"`
	UserID       string    `json:"-"` // Internal only
	ParentID     *string   `json:"parentId"`
	LinkedNodeID *string   `json:"linkedNodeId"`
	Name         string    `json:"name"`
	Content      *string   `json:"content"`
	IsFolder     bool      `json:"isFolder"`
	SortOrder    *int      `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
}
