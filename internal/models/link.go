package models

import "time"

type Link struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	TargetURL string     `json:"target_url"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Clicks    int64      `json:"clicks"`
	Active    bool       `json:"active"`
}
