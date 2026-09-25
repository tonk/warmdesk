package models

import "time"

// TimeEntryRowOrder persists the user's custom row ordering for the time-tracking grid.
// One row per user; OrderedKeys is a JSON array of row-key strings.
type TimeEntryRowOrder struct {
	UserID      uint   `gorm:"primaryKey" json:"user_id"`
	OrderedKeys string `gorm:"type:text" json:"ordered_keys"`
}

// TimeEntryWeekRowOrder persists row-key order for a specific ISO week, including empty rows.
// Comments is a JSON object mapping rowKey → comment text.
type TimeEntryWeekRowOrder struct {
	UserID      uint   `gorm:"primaryKey" json:"user_id"`
	Year        int    `gorm:"primaryKey" json:"year"`
	Week        int    `gorm:"primaryKey" json:"week"`
	OrderedKeys string `gorm:"type:text" json:"ordered_keys"`
	Comments    string `gorm:"type:text" json:"comments"`
}

// TimeMacroLibrary stores a user's time-tracking macro templates as JSON
// (same shape as the frontend timeTracking.macroTemplates.v2 value).
type TimeMacroLibrary struct {
	UserID  uint   `gorm:"primaryKey" json:"user_id"`
	Payload string `gorm:"type:text;not null" json:"payload"`
}

type TimeEntry struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	User        *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CustomerID  *uint     `gorm:"index" json:"customer_id"`
	Customer    *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	ProjectID   *uint     `gorm:"index" json:"project_id"`
	Project     *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	ContractID  *uint     `gorm:"index" json:"contract_id"`
	Contract    *Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
	TicketID    *uint     `gorm:"index" json:"ticket_id,omitempty"`
	Ticket      *Ticket   `gorm:"foreignKey:TicketID" json:"ticket,omitempty"`
	LocationID  *uint             `gorm:"index" json:"location_id,omitempty"`
	Location    *CustomerLocation `gorm:"foreignKey:LocationID" json:"location,omitempty"`
	Date        time.Time `gorm:"not null;index" json:"date"`
	Minutes     int     `gorm:"not null" json:"minutes"`
	Description string  `json:"description"`
	IsHoliday   bool     `gorm:"default:false" json:"is_holiday"`
	StartTime   *string  `gorm:"size:5" json:"start_time,omitempty"`
	EndTime     *string  `gorm:"size:5" json:"end_time,omitempty"`
	Distance    *float64 `json:"distance,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TimeTimer is a user's running time-tracking timer — at most one per user.
// Stopping it turns the elapsed time into regular TimeEntry rows (see
// handlers/timer.go); nothing else reads it.
type TimeTimer struct {
	UserID      uint      `gorm:"primaryKey;autoIncrement:false" json:"user_id"`
	CustomerID  *uint     `json:"customer_id"`
	Customer    *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	ProjectID   *uint     `json:"project_id"`
	Project     *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Description string    `json:"description"`
	// TimeZone is the IANA zone the timer was started in; entry dates and
	// wall-clock times are computed in it when the timer stops.
	TimeZone  string    `gorm:"size:100" json:"time_zone"`
	StartedAt time.Time `gorm:"not null" json:"started_at"`
}
