package models

import "time"

type StarredProject struct {
	UserID    uint      `gorm:"primaryKey;index" json:"user_id"`
	ProjectID uint      `gorm:"primaryKey;index" json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
}
