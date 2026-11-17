package models

import "time"

// TimeNormYear holds a user's yearly time norm settings. One row per user and
// year; a year without a row inherits hours_per_week and vacation_hours from
// the nearest earlier year (see handlers/time_overview.go), carry_over_hours
// is never inherited because it is filled in per year.
type TimeNormYear struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;uniqueIndex:idx_time_norm_user_year" json:"user_id"`
	Year   int  `gorm:"not null;uniqueIndex:idx_time_norm_user_year" json:"year"`
	// HoursPerWeek is the contractual working week; the norm per working day
	// is HoursPerWeek/5.
	HoursPerWeek *float64 `json:"hours_per_week"`
	// VacationHours is the vacation entitlement for this year, in hours.
	VacationHours *float64 `json:"vacation_hours"`
	// CarryOverHours is the unused vacation from previous years, in hours,
	// as entered by the user for this year.
	CarryOverHours *float64  `json:"carry_over_hours"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TimeCategoryProject maps a project to the overview category it counts as for
// a user: "sick", "vacation" or "travel". Without any rows for a user the
// overviews guess the categories from the project names.
type TimeCategoryProject struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_time_category_user_project" json:"user_id"`
	ProjectID uint      `gorm:"not null;uniqueIndex:idx_time_category_user_project" json:"project_id"`
	Category  string    `gorm:"size:10;not null" json:"category"`
	CreatedAt time.Time `json:"created_at"`
}
