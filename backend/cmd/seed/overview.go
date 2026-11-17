package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// overviewSeed names what the year of demo time entries for the time overviews
// (norm, checks, travel, vacation) is booked on.
type overviewSeed struct {
	UserID uint

	Acme, Globex, SmartOwl uint // customer ids
	Web, Devops            uint // board project ids
	Phase1, Managed        uint // contract ids
	Travel, Holidays       uint // time-tracking-only project ids
	Sick, Internal         uint
}

type overviewSeedResult struct {
	Entries       int
	Holidays      int // holiday entries (this and last year)
	VacationDays  int // this year, booked up to and after today
	PrevVacation  int // last year, to make the carry-over suggestion non-empty
	SickDays      int
	TravelEntries int
	PlannedDays   int // vacation days after today
}

func clockMinutes(s string) int {
	h, _ := strconv.Atoi(s[:2])
	m, _ := strconv.Atoi(s[3:])
	return h*60 + m
}

// seedOverviewYear books a believable year for one user: the Dutch public
// holidays as holiday entries, ordinary work days on a board project, on-site
// days with a Heen/Terug travel pair and distance, a few sick days, two
// summer vacation weeks (plus some planned ahead), and last year's vacation.
// The last three weeks are left alone — the regular seed already books those —
// except for planned vacation after today.
//
// On purpose a few things are off, so the checks overview has something to
// show: one overlap, one weekend entry, one travel entry without distance,
// a short and a long day, and some days without a contract.
func seedOverviewYear(db *gorm.DB, s overviewSeed, now time.Time) (res overviewSeedResult, err error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	year := today.Year()
	cutoff := today.AddDate(0, 0, -21)

	create := func(e models.TimeEntry) {
		if err != nil {
			return
		}
		e.UserID = s.UserID
		if e.Description != "" && !e.IsHoliday && e.StartTime != nil && e.EndTime != nil {
			e.Minutes = clockMinutes(*e.EndTime) - clockMinutes(*e.StartTime)
		}
		err = db.Create(&e).Error
		res.Entries++
	}
	uptr := ptr[uint]
	book := func(d time.Time, cust, proj, contract uint, desc, start, end string, dist float64) {
		e := models.TimeEntry{
			CustomerID: uptr(cust), ProjectID: uptr(proj), Date: d,
			Description: desc, StartTime: ptr(start), EndTime: ptr(end),
		}
		if contract != 0 {
			e.ContractID = uptr(contract)
		}
		if dist > 0 {
			e.Distance = ptr(dist)
		}
		create(e)
	}

	// Norm settings: a 40-hour week, 200 vacation hours a year, and last year's
	// remainder (200 − 22 days of 8 h = 24 h) carried over into this year.
	for _, ns := range []models.TimeNormYear{
		{UserID: s.UserID, Year: year - 1, HoursPerWeek: ptr(40.0), VacationHours: ptr(200.0), CarryOverHours: ptr(0.0)},
		{UserID: s.UserID, Year: year, HoursPerWeek: ptr(40.0), VacationHours: ptr(200.0), CarryOverHours: ptr(24.0)},
	} {
		ns := ns
		if err = db.Create(&ns).Error; err != nil {
			return
		}
	}
	// An explicit project → category mapping (the overviews would otherwise
	// guess from the project names).
	for cat, proj := range map[string]uint{"sick": s.Sick, "vacation": s.Holidays, "travel": s.Travel} {
		if err = db.Create(&models.TimeCategoryProject{UserID: s.UserID, ProjectID: proj, Category: cat}).Error; err != nil {
			return
		}
	}

	// Holidays: what makes a weekday free in the norm.
	isHoliday := map[string]bool{}
	key := func(d time.Time) string { return d.Format("2006-01-02") }
	for _, y := range []int{year - 1, year} {
		hs, ok := services.PublicHolidays("nl", y)
		if !ok {
			return res, fmt.Errorf("no public holidays for locale nl")
		}
		for _, h := range hs {
			d := time.Date(h.Date.Year(), h.Date.Month(), h.Date.Day(), 0, 0, 0, 0, time.UTC)
			isHoliday[key(d)] = true
			create(models.TimeEntry{Date: d, IsHoliday: true, Description: h.Name})
			res.Holidays++
		}
	}
	weekend := func(d time.Time) bool { return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday }

	vacation := func(d time.Time, desc string) {
		book(d, s.SmartOwl, s.Holidays, 0, desc, "08:00", "16:00", 0)
	}

	// Last year: 22 vacation days from mid July.
	for d, left := time.Date(year-1, 7, 13, 0, 0, 0, 0, time.UTC), 22; left > 0 && d.Year() == year-1; d = d.AddDate(0, 0, 1) {
		if weekend(d) || isHoliday[key(d)] {
			continue
		}
		vacation(d, "Annual leave")
		res.PrevVacation++
		left--
	}

	// This year's vacation: two summer weeks, the last days of the year, and
	// four days planned ahead of today.
	vacDates := map[string]bool{}
	for _, r := range [][2]time.Time{
		{time.Date(year, 7, 20, 0, 0, 0, 0, time.UTC), time.Date(year, 7, 24, 0, 0, 0, 0, time.UTC)},
		{time.Date(year, 8, 3, 0, 0, 0, 0, time.UTC), time.Date(year, 8, 7, 0, 0, 0, 0, time.UTC)},
		{time.Date(year, 12, 28, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)},
	} {
		for d := r[0]; !d.After(r[1]); d = d.AddDate(0, 0, 1) {
			vacDates[key(d)] = true
		}
	}
	for d, n := today.AddDate(0, 0, 10), 0; n < 4; d = d.AddDate(0, 0, 1) {
		if d.Year() != year {
			break
		}
		if weekend(d) || isHoliday[key(d)] {
			continue
		}
		vacDates[key(d)] = true
		n++
	}

	webDesc := []string{"Sprint work — website redesign", "Design review", "Frontend implementation", "Code review and QA"}
	opsDesc := []string{"Infrastructure automation", "Monitoring and alerting", "Kubernetes operations", "CI/CD maintenance"}

	n, globexTrips, overlapDone, weekendDone := 0, 0, false, false
	for d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		k := key(d)
		past := !d.After(cutoff)

		if weekend(d) {
			// One weekend call-out, the first Saturday of May.
			if !weekendDone && past && d.Weekday() == time.Saturday && d.Month() == time.May {
				book(d, s.Globex, s.Devops, s.Managed, "Incident follow-up (weekend)", "10:00", "12:00", 0)
				weekendDone = true
			}
			continue
		}
		if isHoliday[k] {
			continue
		}
		if vacDates[k] && (past || d.After(today)) {
			vacation(d, "Annual leave")
			res.VacationDays++
			if d.After(today) {
				res.PlannedDays++
			}
			continue
		}
		if !past {
			continue
		}
		if d.Month() == time.February && d.Day() >= 9 && d.Day() <= 11 {
			book(d, s.SmartOwl, s.Sick, 0, "Sick — flu", "08:00", "16:00", 0)
			res.SickDays++
			continue
		}

		n++
		_, isoWeek := d.ISOWeek()
		cust, proj, contract, pool := s.Acme, s.Web, s.Phase1, webDesc
		if isoWeek%2 == 1 {
			cust, proj, contract, pool = s.Globex, s.Devops, s.Managed, opsDesc
		}
		desc := pool[n%len(pool)]

		switch {
		case n%10 == 3: // on site at Acme: 42 km each way
			book(d, s.SmartOwl, s.Travel, 0, "Acme Corporation - Slough - Heen", "06:30", "08:00", 42)
			book(d, s.Acme, s.Web, s.Phase1, "On-site workshop", "08:00", "12:00", 0)
			book(d, s.Acme, s.Web, s.Phase1, "On-site workshop", "12:30", "16:30", 0)
			book(d, s.SmartOwl, s.Travel, 0, "Acme Corporation - Slough - Terug", "16:30", "18:00", 42)
			res.TravelEntries += 2
		case n%10 == 8: // on site at Globex: 67 km each way; the second trip back has no distance
			globexTrips++
			back := 67.0
			if globexTrips == 2 {
				back = 0
			}
			book(d, s.SmartOwl, s.Travel, 0, "Globex Systems - Cork - Heen", "06:00", "08:00", 67)
			book(d, s.Globex, s.Devops, s.Managed, "On-site infrastructure audit", "08:00", "12:00", 0)
			book(d, s.Globex, s.Devops, s.Managed, "On-site infrastructure audit", "12:30", "16:30", 0)
			book(d, s.SmartOwl, s.Travel, 0, "Globex Systems - Cork - Terug", "16:30", "18:30", back)
			res.TravelEntries += 2
		default:
			c := contract
			if n%17 == 2 { // a few days that forgot the contract
				c = 0
			}
			book(d, cust, proj, c, desc, "08:30", "12:30", 0)
			switch {
			case n%23 == 5: // short day: only the morning
			case n >= 40 && !overlapDone: // a meeting booked on top of the afternoon
				overlapDone = true
				book(d, cust, proj, c, desc, "13:00", "17:00", 0)
				book(d, s.SmartOwl, s.Internal, 0, "Planning meeting", "16:00", "17:00", 0)
			case n%31 == 7: // long day: an evening on call
				book(d, cust, proj, c, desc, "13:00", "17:00", 0)
				book(d, cust, proj, c, "Evening deployment", "19:00", "21:00", 0)
			default:
				book(d, cust, proj, c, desc, "13:00", "17:00", 0)
			}
		}
	}
	return res, err
}
