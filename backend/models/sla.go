package models

import "time"

type SlaPolicy struct {
	ID                    uint   `gorm:"primaryKey" json:"id"`
	Name                  string `gorm:"not null;size:200" json:"name"`
	ResponseTimeMinutes   int    `gorm:"not null;default:0" json:"response_time_minutes"`
	ResolutionTimeMinutes int    `gorm:"not null;default:0" json:"resolution_time_minutes"`
	PriorityFilter        string `gorm:"size:200" json:"priority_filter"`
	// Optional extra match filters. A policy matches a ticket only when every
	// non-empty filter matches; the most specific matching policy wins
	// (services.MatchSlaPolicy).
	CustomerID *uint  `gorm:"index" json:"customer_id,omitempty"`
	TypeFilter string `gorm:"size:200" json:"type_filter"`
	CategoryID *uint  `gorm:"index" json:"category_id,omitempty"`
	// CalendarID selects the business hours deadlines are counted in; nil = 24×7.
	CalendarID *uint `gorm:"index" json:"calendar_id,omitempty"`
	// WarningPercent: share of the SLA time after which assignee/group are
	// warned (0 disables the warning).
	WarningPercent int `gorm:"not null;default:75" json:"warning_percent"`
	// Escalation on breach: notify the group/user and, when EscalationReassign
	// is set, reassign the ticket to them.
	EscalationGroupID  *uint     `gorm:"index" json:"escalation_group_id,omitempty"`
	EscalationUserID   *uint     `gorm:"index" json:"escalation_user_id,omitempty"`
	EscalationReassign bool      `gorm:"not null;default:true" json:"escalation_reassign"`
	IsActive           bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
