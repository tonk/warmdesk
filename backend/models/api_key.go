package models

import "time"

type APIKey struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	User      User       `json:"-"`
	Name      string     `gorm:"not null;size:100" json:"name"`
	KeyHash   string     `gorm:"not null;uniqueIndex;size:64" json:"-"`
	KeyPrefix string     `gorm:"not null;size:12" json:"key_prefix"` // e.g. "cwk_abc123" first 12 chars shown in UI
}
