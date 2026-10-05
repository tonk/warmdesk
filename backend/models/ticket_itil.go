package models

import "time"

// TicketCategory is an admin-managed, two-level ticket categorisation
// (a category with optional subcategories via ParentID).
type TicketCategory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ParentID  *uint     `gorm:"index" json:"parent_id,omitempty"`
	Name      string    `gorm:"not null;size:200" json:"name"`
	IsActive  bool      `gorm:"not null;default:true" json:"is_active"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TicketResolutionCode is an admin-managed closure code recorded when a
// ticket is resolved (e.g. "Fixed", "Workaround provided", "Duplicate").
type TicketResolutionCode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;size:200" json:"name"`
	IsActive  bool      `gorm:"not null;default:true" json:"is_active"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BusinessCalendar defines the working hours SLA deadlines are counted in.
// WorkDays is a comma-separated list of Go weekday numbers (0 = Sunday);
// StartTime/EndTime are "HH:MM" wall-clock times in Timezone.
type BusinessCalendar struct {
	ID        uint                      `gorm:"primaryKey" json:"id"`
	Name      string                    `gorm:"not null;size:200" json:"name"`
	Timezone  string                    `gorm:"not null;size:64;default:'UTC'" json:"timezone"`
	WorkDays  string                    `gorm:"not null;size:20;default:'1,2,3,4,5'" json:"work_days"`
	StartTime string                    `gorm:"not null;size:5;default:'09:00'" json:"start_time"`
	EndTime   string                    `gorm:"not null;size:5;default:'17:00'" json:"end_time"`
	Holidays  []BusinessCalendarHoliday `gorm:"foreignKey:CalendarID" json:"holidays,omitempty"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

// BusinessCalendarHoliday is a non-working day ("YYYY-MM-DD") in a calendar.
type BusinessCalendarHoliday struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	CalendarID uint   `gorm:"not null;index;uniqueIndex:idx_cal_holiday" json:"calendar_id"`
	Date       string `gorm:"not null;size:10;uniqueIndex:idx_cal_holiday" json:"date"`
	Name       string `gorm:"size:200" json:"name"`
}
