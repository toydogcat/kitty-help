package models

type User struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	GoogleID *string `json:"googleId"`
	Email    *string `json:"email"`
}
