package models

import (
"time"
)

type DeskShelf struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index"`
	Name      string    `json:"name"`
	Color     string    `json:"color"` // 可選顏色方便記憶
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DeskItem struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index"`
	ShelfID   *string   `json:"shelfId" gorm:"index"` // 為空則在桌面
	Type      string    `json:"type"`    // "bookmark", "snippet", "attachment"
	RefID     string    `json:"refId"`   // 指向原始資料 ID
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
