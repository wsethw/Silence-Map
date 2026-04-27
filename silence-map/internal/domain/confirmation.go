package domain

import "time"

type Confirmation struct {
	ID        string    `json:"id"`
	ReportID  string    `json:"report_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
