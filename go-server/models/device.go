package models

type Device struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	DeviceName *string `json:"deviceName"`
	UserAgent  *string `json:"userAgent"`
	UserID     *string `json:"userId"`
	UserName   *string `json:"userName"`
	UserRole   *string `json:"userRole"`
}
