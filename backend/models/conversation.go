package models

import "time"

// Conversation is a 1-on-1 or group message thread.
type Conversation struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Name        string                `gorm:"size:200" json:"name"`
	Avatar      string                `gorm:"size:500" json:"avatar"`
	IsGroup     bool                  `gorm:"default:false" json:"is_group"`
	CreatedByID uint                  `gorm:"not null;index" json:"created_by_id"`
	Members     []ConversationMember  `gorm:"foreignKey:ConversationID" json:"members"`
}

// ConversationMember records which users belong to a conversation.
type ConversationMember struct {
	ConversationID uint      `gorm:"primaryKey" json:"conversation_id"`
	UserID         uint      `gorm:"primaryKey" json:"user_id"`
	User           User      `gorm:"foreignKey:UserID" json:"user"`
	JoinedAt       time.Time `json:"joined_at"`
}

// ConversationMessage is a single message inside a Conversation.
type ConversationMessage struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ConversationID uint      `gorm:"not null;index" json:"conversation_id"`
	SenderID       uint      `gorm:"not null;index" json:"sender_id"`
	Sender         User      `gorm:"foreignKey:SenderID" json:"sender"`
	Body           string    `gorm:"type:text;not null" json:"body"`
	IsEdited       bool      `gorm:"default:false" json:"is_edited"`
	IsDeleted      bool      `gorm:"default:false" json:"is_deleted"`
}
