package models

type CalendarEvent struct {
	ID        string `json:"id"`
	UserID    string `json:"userId"`
	Date      string `json:"eventDate"`
	Content   string `json:"content"`
	UserName  string `json:"userName"`
	CreatedAt string `json:"createdAt"`
}
