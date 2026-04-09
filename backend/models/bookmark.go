package models

import "time"

type Bookmark struct {
	ID         string    `json:"id"`
	UserID     string    `json:"-"` // Internal only
	ParentID   *string   `json:"parentId"`
	Title      *string   `json:"title"`
	URL        *string   `json:"url"`
	Category   *string   `json:"category"`
	IconURL    *string   `json:"iconUrl"`
	PasswordID *string   `json:"passwordId"`
	IsFolder   *bool     `json:"isFolder"`
	SortOrder  *int      `json:"sortOrder"`
	CreatedAt  time.Time `json:"createdAt"`
}
