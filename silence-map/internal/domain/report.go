package domain

import "time"

type Report struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	Location          Point     `json:"location"`
	QuietnessLevel    int       `json:"quietness_level"`
	PlaceName         string    `json:"place_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	ConfirmationCount int       `json:"confirmations"`
	Weight            float64   `json:"weight,omitempty"`
}

type QuietPlace struct {
	PlaceName         string    `json:"place_name"`
	Location          Point     `json:"location"`
	AverageQuietness  float64   `json:"average_quietness"`
	WeightedScore     float64   `json:"weighted_score"`
	ReportCount       int       `json:"report_count"`
	ConfirmationCount int       `json:"confirmation_count"`
	LastReportAt      time.Time `json:"last_report_at"`
	Confidence        float64   `json:"confidence"`
}
