package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// jsonScan is the shared Scan implementation for the JSON-in-text columns below.
func jsonScan(src any, dst any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case nil:
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into JSON column", src)
	}
	if s == "" || s == "null" {
		return nil
	}
	return json.Unmarshal([]byte(s), dst)
}

// ── Knowledge management ───────────────────────────────────────────────────

// KnowledgeArticle is a knowledge base article. Internal articles are for
// agents only; public ones are also shown to customer-portal users — to
// every customer, or only to CustomerID's users when that is set.
type KnowledgeArticle struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `gorm:"not null;size:300" json:"title"`
	Summary     string     `gorm:"size:500" json:"summary"`
	Body        string     `gorm:"type:text" json:"body"`
	Status      string     `gorm:"not null;size:20;default:'draft'" json:"status"`        // draft | published | archived
	Visibility  string     `gorm:"not null;size:20;default:'internal'" json:"visibility"` // internal | public
	CustomerID  *uint      `gorm:"index" json:"customer_id,omitempty"`
	CategoryID  *uint      `gorm:"index" json:"category_id,omitempty"`
	Keywords    string     `gorm:"size:500" json:"keywords"`
	AuthorID    uint       `gorm:"not null" json:"author_id"`
	UpdatedByID uint       `json:"updated_by_id"`
	Views       int        `gorm:"not null;default:0" json:"views"`
	HelpfulYes  int        `gorm:"not null;default:0" json:"helpful_yes"`
	HelpfulNo   int        `gorm:"not null;default:0" json:"helpful_no"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Author    *User           `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	UpdatedBy *User           `gorm:"foreignKey:UpdatedByID" json:"updated_by,omitempty"`
	Customer  *Customer       `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Category  *TicketCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
}

// TicketArticleLink records that a knowledge article was used on a ticket.
type TicketArticleLink struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TicketID    uint      `gorm:"not null;index;uniqueIndex:idx_ticket_article" json:"ticket_id"`
	ArticleID   uint      `gorm:"not null;index;uniqueIndex:idx_ticket_article" json:"article_id"`
	CreatedByID uint      `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// ── Service request management ─────────────────────────────────────────────

// CatalogField is one input of a service catalogue request form.
type CatalogField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // text | textarea | select | checkbox | date | number
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Help     string   `json:"help,omitempty"`
}

type CatalogFields []CatalogField

func (f CatalogFields) Value() (driver.Value, error) {
	if f == nil {
		f = CatalogFields{}
	}
	b, err := json.Marshal(f)
	return string(b), err
}

func (f *CatalogFields) Scan(src any) error { return jsonScan(src, f) }

// ServiceCatalogItem is an orderable service. Submitting it creates a
// service_request ticket routed to GroupID, with the item's checklist as
// fulfilment tasks and — when ApprovalsRequired > 0 — an approval round by
// ApprovalGroupID.
type ServiceCatalogItem struct {
	ID                  uint          `gorm:"primaryKey" json:"id"`
	Name                string        `gorm:"not null;size:200" json:"name"`
	Description         string        `gorm:"type:text" json:"description"`
	Icon                string        `gorm:"size:16" json:"icon"`
	CategoryID          *uint         `gorm:"index" json:"category_id,omitempty"`
	CustomerID          *uint         `gorm:"index" json:"customer_id,omitempty"`
	Fields              CatalogFields `gorm:"type:text" json:"fields"`
	DefaultPriority     string        `gorm:"size:20;default:'medium'" json:"default_priority"`
	GroupID             *uint         `gorm:"index" json:"group_id,omitempty"`
	ApprovalGroupID     *uint         `gorm:"index" json:"approval_group_id,omitempty"`
	ApprovalsRequired   int           `gorm:"not null;default:0" json:"approvals_required"`
	ChecklistTemplateID *uint         `gorm:"index" json:"checklist_template_id,omitempty"`
	IsActive            bool          `gorm:"not null;default:true" json:"is_active"`
	SortOrder           int           `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

// RequestData holds the answers of a service request form, keyed by field key.
type RequestData map[string]any

func (d RequestData) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	b, err := json.Marshal(d)
	return string(b), err
}

func (d *RequestData) Scan(src any) error { return jsonScan(src, d) }

// ── Approvals (service requests and changes) ───────────────────────────────

// TicketApproval is one approver's decision on a ticket's current approval round.
type TicketApproval struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"not null;index;uniqueIndex:idx_ticket_approver_round" json:"ticket_id"`
	Round     int       `gorm:"not null;default:1;uniqueIndex:idx_ticket_approver_round" json:"round"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_ticket_approver_round" json:"user_id"`
	Decision  string    `gorm:"not null;size:20" json:"decision"` // approved | rejected
	Comment   string    `gorm:"type:text" json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      *User     `json:"user,omitempty"`
}

// ── Change enablement ──────────────────────────────────────────────────────

// ChangeFreezeWindow is a period in which normal changes may not be
// scheduled (emergency changes may, with a warning). CustomerID limits it to
// one customer's changes.
type ChangeFreezeWindow struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"not null;size:200" json:"name"`
	StartAt    time.Time `gorm:"not null" json:"start_at"`
	EndAt      time.Time `gorm:"not null" json:"end_at"`
	CustomerID *uint     `gorm:"index" json:"customer_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ── Configuration management (CMDB) ────────────────────────────────────────

// CIType is an admin-managed configuration item type (Server, Application, …).
type CIType struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;size:100;uniqueIndex" json:"name"`
	Icon      string    `gorm:"size:16" json:"icon"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CIAttributes are free-form key/value attributes of a configuration item.
type CIAttributes map[string]string

func (a CIAttributes) Value() (driver.Value, error) {
	if a == nil {
		a = CIAttributes{}
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *CIAttributes) Scan(src any) error { return jsonScan(src, a) }

// ConfigurationItem is a CMDB record. CustomerID nil = internal CI, visible
// to every agent; otherwise access follows the customer access rules.
type ConfigurationItem struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	Name        string       `gorm:"not null;size:200" json:"name"`
	TypeID      uint         `gorm:"not null;index" json:"type_id"`
	CustomerID  *uint        `gorm:"index" json:"customer_id,omitempty"`
	Status      string       `gorm:"not null;size:20;default:'active'" json:"status"` // planned | active | maintenance | retired
	Environment string       `gorm:"size:20" json:"environment"`                      // production | acceptance | test | development
	Criticality string       `gorm:"size:10" json:"criticality"`                      // low | medium | high
	OwnerID     *uint        `gorm:"index" json:"owner_id,omitempty"`
	GroupID     *uint        `gorm:"index" json:"group_id,omitempty"`
	Description string       `gorm:"type:text" json:"description"`
	Attributes  CIAttributes `gorm:"type:text" json:"attributes"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	Type     *CIType    `gorm:"foreignKey:TypeID" json:"type,omitempty"`
	Customer *Customer  `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Owner    *User      `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Group    *UserGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
}

// CIRelation is a directional relationship, read "source <type> target":
// depends_on | runs_on | part_of | connected_to.
type CIRelation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SourceID  uint      `gorm:"not null;index;uniqueIndex:idx_ci_relation" json:"source_id"`
	TargetID  uint      `gorm:"not null;index;uniqueIndex:idx_ci_relation" json:"target_id"`
	Type      string    `gorm:"not null;size:20;uniqueIndex:idx_ci_relation" json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// TicketCI links a ticket (incident, problem, change, …) to an affected CI.
type TicketCI struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TicketID    uint      `gorm:"not null;index;uniqueIndex:idx_ticket_ci" json:"ticket_id"`
	CIID        uint      `gorm:"column:ci_id;not null;index;uniqueIndex:idx_ticket_ci" json:"ci_id"`
	CreatedByID uint      `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (TicketCI) TableName() string { return "ticket_cis" }
