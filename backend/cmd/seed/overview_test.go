package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

// The overview seed books a whole year for demo.admin, and the data has the
// shape the overviews rely on: holiday entries, vacation (also planned and
// last year), sick days, travel in Heen/Terug pairs with distances, and a
// few deliberate problems for the checks.
func TestSeedOverviewYear(t *testing.T) {
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()

	user := &models.User{Username: "demo.admin", Email: "a@example.com", PasswordHash: "x", GlobalRole: "admin"}
	require.NoError(t, db.Create(user).Error)
	cust := func(name string) uint {
		c := &models.Customer{Name: name}
		require.NoError(t, db.Create(c).Error)
		return c.ID
	}
	proj := func(name, slug string, undecl int) uint {
		p := &models.Project{Name: name, Slug: slug, KeyPrefix: slug, BoardType: "kanban", TimeTrackingOnly: true, UndeclarableMinutes: undecl, CreatedByID: user.ID}
		require.NoError(t, db.Create(p).Error)
		return p.ID
	}
	acme, globex := cust("Acme"), cust("Globex")
	contract := func(c uint, name string) uint {
		ct := &models.Contract{CustomerID: c, Name: name}
		require.NoError(t, db.Create(ct).Error)
		return ct.ID
	}
	s := overviewSeed{
		UserID: user.ID, Acme: acme, Globex: globex, SmartOwl: cust("Smart Owl"),
		Web: proj("Web", "web", 0), Devops: proj("Ops", "ops", 0),
		Phase1: contract(acme, "Phase 1"), Managed: contract(globex, "Managed"),
		Travel: proj("Travel", "trv", 45), Holidays: proj("Holidays", "hol", 480),
		Sick: proj("Sick leave", "sick", 0), Internal: proj("Internal", "int", 60),
	}

	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	res, err := seedOverviewYear(db, s, now)
	require.NoError(t, err)

	var entries []models.TimeEntry
	require.NoError(t, db.Order("date, id").Find(&entries).Error)
	require.Equal(t, res.Entries, len(entries))

	holidayDates := map[string]bool{}
	for _, e := range entries {
		if e.IsHoliday {
			holidayDates[e.Date.Format("2006-01-02")] = true
		}
	}
	require.Equal(t, 22, res.Holidays) // 11 Dutch holidays, this and last year
	require.True(t, holidayDates["2026-04-27"], "Koningsdag is a holiday entry")

	cutoff := now.AddDate(0, 0, -21)
	var planned, sick, noDistTravel, travel, weekend, mismatch, onHoliday int
	byDayVac := map[string]bool{}
	for _, e := range entries {
		d := e.Date.Format("2006-01-02")
		if e.IsHoliday {
			require.Equal(t, 0, e.Minutes)
			continue
		}
		if holidayDates[d] {
			onHoliday++
		}
		if e.StartTime != nil && e.EndTime != nil && clockMinutes(*e.EndTime)-clockMinutes(*e.StartTime) != e.Minutes {
			mismatch++
		}
		if wd := e.Date.Weekday(); wd == time.Saturday || wd == time.Sunday {
			weekend++
		}
		switch *e.ProjectID {
		case s.Holidays:
			if e.Date.After(now) {
				planned++
			} else if e.Date.Year() == 2026 {
				byDayVac[d] = true
			}
			// Never inside the window the regular seed already books.
			require.False(t, e.Date.After(cutoff) && !e.Date.After(now), "vacation in the booked window: %s", d)
		case s.Sick:
			sick++
		case s.Travel:
			travel++
			if e.Distance == nil {
				noDistTravel++
			}
		default:
			require.False(t, e.Date.After(cutoff) && !e.Date.After(now), "work in the booked window: %s", d)
		}
	}
	require.Equal(t, 0, onHoliday, "no work on a holiday")
	require.Equal(t, 0, mismatch, "minutes match start/end")
	require.Equal(t, res.PlannedDays, planned)
	require.Greater(t, planned, 0)
	require.Equal(t, 3, sick)
	require.Equal(t, 3, res.SickDays)
	require.Equal(t, 22, res.PrevVacation)
	require.Equal(t, 1, weekend, "one weekend call-out")
	require.Equal(t, 1, noDistTravel, "one travel entry without distance")
	require.Equal(t, res.TravelEntries, travel)
	require.Greater(t, travel, 10)

	// Travel comes as Heen/Terug pairs per day.
	type pair struct{ heen, terug int }
	pairs := map[string]*pair{}
	for _, e := range entries {
		if e.ProjectID != nil && *e.ProjectID == s.Travel {
			p := pairs[e.Date.Format("2006-01-02")]
			if p == nil {
				p = &pair{}
				pairs[e.Date.Format("2006-01-02")] = p
			}
			if len(e.Description) > 5 && e.Description[len(e.Description)-5:] == " Heen" {
				p.heen++
			} else if len(e.Description) > 6 && e.Description[len(e.Description)-6:] == " Terug" {
				p.terug++
			}
		}
	}
	for d, p := range pairs {
		require.Equal(t, 1, p.heen, d)
		require.Equal(t, 1, p.terug, d)
	}

	// Settings and the explicit category mapping exist for the user.
	var years []models.TimeNormYear
	require.NoError(t, db.Order("year").Find(&years).Error)
	require.Len(t, years, 2)
	require.Equal(t, 2025, years[0].Year)
	require.Equal(t, 24.0, *years[1].CarryOverHours) // 200 − 22 days × 8 h, last year's remainder
	var cats int64
	db.Model(&models.TimeCategoryProject{}).Where("user_id = ?", user.ID).Count(&cats)
	require.Equal(t, int64(3), cats)
}
