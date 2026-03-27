package models

import "time"

type DirectMessage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	SenderID   uint      `gorm:"not null;index" json:"sender_id"`
	Sender     User      `json:"sender"`
	ReceiverID uint      `gorm:"not null;index" json:"receiver_id"`
	Receiver   User      `json:"receiver"`
	Body       string    `gorm:"type:text;not null" json:"body"`
	IsEdited   bool      `gorm:"default:false" json:"is_edited"`
	IsDeleted  bool      `gorm:"default:false" json:"is_deleted"`
}
