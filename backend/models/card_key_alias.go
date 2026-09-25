package models

import "time"

// CardKeyAlias keeps a card's former key (e.g. "PRJ-12") resolvable after the
// card has moved to another project and been renumbered there. The prefix is
// stored as a string rather than a project id because external references
// (commit messages, ticket texts) carry the text, not the project.
type CardKeyAlias struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	KeyPrefix  string    `gorm:"not null;size:10;uniqueIndex:idx_card_key_alias" json:"key_prefix"`
	CardNumber int       `gorm:"not null;uniqueIndex:idx_card_key_alias" json:"card_number"`
	CardID     uint      `gorm:"not null;index" json:"card_id"`
}
