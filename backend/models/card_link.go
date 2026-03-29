package models

import "time"

// CardLink records a reference from a git event (commit, PR, issue) to a card.
// Created automatically when a webhook payload contains a card reference like "PRJ-42".
type CardLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CardID    uint      `gorm:"not null;index" json:"card_id"`
	Platform  string    `gorm:"size:20;not null" json:"platform"`  // github, gitlab, gitea, forgejo
	LinkType  string    `gorm:"size:20;not null" json:"link_type"` // commit, pr, issue
	Title     string    `gorm:"size:500" json:"title"`
	URL       string    `gorm:"size:2000" json:"url"`
	Reference string    `gorm:"size:200" json:"reference"` // commit SHA, PR number, issue number
	Author    string    `gorm:"size:200" json:"author"`
	Status    string    `gorm:"size:20" json:"status"` // open, closed, merged
	RepoName  string    `gorm:"size:300" json:"repo_name"`
}
