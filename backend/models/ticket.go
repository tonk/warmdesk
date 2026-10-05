package models

import "time"

type Ticket struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	CustomerID            *uint      `gorm:"index" json:"customer_id"`
	Customer              *Customer  `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Title                 string     `gorm:"not null;size:500" json:"title"`
	Description           string     `gorm:"type:text" json:"description"`
	Type                  string     `gorm:"not null;size:30;default:'incident'" json:"type"`
	Status                string     `gorm:"not null;size:20;default:'new'" json:"status"`
	Priority              string     `gorm:"not null;size:20;default:'medium'" json:"priority"`
	CreatedByID           uint       `gorm:"not null" json:"created_by_id"`
	AssignedToID          *uint      `gorm:"index" json:"assigned_to_id,omitempty"`
	OwnerID               *uint      `gorm:"index" json:"owner_id,omitempty"`
	GroupID               *uint      `gorm:"index" json:"group_id,omitempty"`
	SlaPolicyID           *uint      `gorm:"index" json:"sla_policy_id,omitempty"`
	FirstResponseAt       *time.Time `json:"first_response_at,omitempty"`
	SlaResponseDeadline   *time.Time `json:"sla_response_deadline,omitempty"`
	SlaResolutionDeadline *time.Time `json:"sla_resolution_deadline,omitempty"`
	SlaResponseBreached   bool       `json:"sla_response_breached"`
	SlaResolutionBreached bool       `json:"sla_resolution_breached"`
	ReminderAt            *time.Time `json:"reminder_at,omitempty"`
	CloseAt               *time.Time `json:"close_at,omitempty"`
	IsSpam                bool       `gorm:"default:false" json:"is_spam"`
	ChecklistTemplateID   *uint      `gorm:"index" json:"checklist_template_id,omitempty"`
	// ITIL incident fields. Priority is derived from Impact × Urgency via the
	// admin priority matrix when both are set (services.DerivePriority), but
	// can still be overridden manually.
	Impact           string     `gorm:"size:10" json:"impact"`
	Urgency          string     `gorm:"size:10" json:"urgency"`
	CategoryID       *uint      `gorm:"index" json:"category_id,omitempty"`
	ResolutionCodeID *uint      `gorm:"index" json:"resolution_code_id,omitempty"`
	ResolutionNotes  string     `gorm:"type:text" json:"resolution_notes"`
	IsMajorIncident  bool       `gorm:"default:false" json:"is_major_incident"`
	ReopenCount      int        `gorm:"not null;default:0" json:"reopen_count"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	// SLA clock state. The resolution clock pauses while the ticket is
	// "pending" (waiting on the customer): SlaPausedAt marks when the current
	// pause began, SlaPausedMinutes accumulates the business minutes of all
	// completed pauses, which push the resolution deadline back.
	SlaPausedAt      *time.Time `json:"sla_paused_at,omitempty"`
	SlaPausedMinutes int        `gorm:"not null;default:0" json:"sla_paused_minutes"`
	SlaWarnedAt      *time.Time `json:"sla_warned_at,omitempty"`
	SlaEscalatedAt   *time.Time `json:"sla_escalated_at,omitempty"`
	// Problem-management fields — only meaningful when Type == "problem".
	ProblemStatus  string                `gorm:"size:30" json:"problem_status"`
	RootCause      string                `gorm:"type:text" json:"root_cause"`
	Workaround     string                `gorm:"type:text" json:"workaround"`
	IsKnownError   bool                  `gorm:"default:false" json:"is_known_error"`
	EmailMessageID *string               `gorm:"uniqueIndex;size:998" json:"email_message_id,omitempty"`
	FromEmail      *string               `gorm:"size:254" json:"from_email,omitempty"`
	FromName       *string               `gorm:"size:150" json:"from_name,omitempty"`
	RawEmail       *string               `gorm:"type:text" json:"raw_email,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	Messages       []TicketMessage       `gorm:"foreignKey:TicketID" json:"messages,omitempty"`
	Tags           []TicketTag           `gorm:"foreignKey:TicketID" json:"tags,omitempty"`
	CreatedBy      User                  `json:"created_by,omitempty"`
	AssignedTo     *User                 `json:"assigned_to,omitempty"`
	Owner          *User                 `json:"owner,omitempty"`
	Group          *UserGroup            `json:"group,omitempty"`
	SlaPolicy      *SlaPolicy            `json:"sla_policy,omitempty"`
	Category       *TicketCategory       `json:"category,omitempty"`
	ResolutionCode *TicketResolutionCode `json:"resolution_code,omitempty"`
	Attachments    []Attachment          `gorm:"-" json:"attachments,omitempty"`
	ChecklistItems []TicketChecklistItem `gorm:"foreignKey:TicketID" json:"checklist_items,omitempty"`
}

type TicketMessage struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	TicketID    uint            `gorm:"not null;index" json:"ticket_id"`
	UserID      uint            `gorm:"not null" json:"user_id"`
	Body        string          `gorm:"type:text;not null" json:"body"`
	FromName    string          `gorm:"size:150" json:"from_name"`
	EmailSent   bool            `gorm:"default:false" json:"email_sent"`
	IsPrivate   bool            `gorm:"default:false" json:"is_private"`
	ParentID    *uint           `gorm:"index" json:"parent_id,omitempty"`
	Parent      *TicketMessage  `gorm:"foreignKey:ParentID" json:"-"`
	Replies     []TicketMessage `gorm:"foreignKey:ParentID" json:"replies,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	User        User            `json:"user,omitempty"`
	Attachments []Attachment    `gorm:"-" json:"attachments,omitempty"`
	// Inbound marks a message that arrived from the requester (an IMAP reply)
	// rather than one written by an agent — used by the first-contact
	// resolution metric.
	Inbound bool `gorm:"default:false" json:"inbound"`
}

// TicketView records the last time each user viewed a ticket.
type TicketView struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	TicketID uint      `gorm:"not null;uniqueIndex:idx_ticket_view" json:"ticket_id"`
	UserID   uint      `gorm:"not null;uniqueIndex:idx_ticket_view" json:"user_id"`
	ViewedAt time.Time `json:"viewed_at"`
	User     User      `json:"user"`
}

// TicketHistory records activity events on a ticket.
type TicketHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	TicketID  uint      `gorm:"not null;index" json:"ticket_id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	User      User      `json:"user"`
	EventType string    `gorm:"size:50;not null" json:"event_type"`
	Detail    string    `gorm:"size:500" json:"detail"`
}
