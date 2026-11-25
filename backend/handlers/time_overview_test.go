package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

func TestGuessOverviewCategory(t *testing.T) {
	cases := map[string]string{
		"Ziek": "sick", " ziek ": "sick", "Sick": "sick",
		"Vrij": "vacation", "Vakantie": "vacation", "Vacation": "vacation", "Holidays": "vacation",
		"Reistijd": "travel", "Travel time": "travel",
		"Lesgeven": "", "Ziekenhuis bezoek": "", "": "",
	}
	for in, want := range cases {
		require.Equal(t, want, guessOverviewCategory(in), in)
	}
}

func TestTravelDirection(t *testing.T) {
	cases := []struct{ in, route, dir string }{
		{"Python - Nieuwegein - Heen", "Python - Nieuwegein", "out"},
		{"Python - Nieuwegein - Terug", "Python - Nieuwegein", "back"},
		{"Velp - heen", "Velp", "out"},
		{"Python - MinDef - Soesterberg - Terug  ", "Python - MinDef - Soesterberg", "back"},
		{"Los reisje", "Los reisje", ""},
		{"Terugkomdag - Schiphol", "Terugkomdag - Schiphol", ""}, // "terug" only counts as a " - Terug" suffix
	}
	for _, c := range cases {
		route, dir := travelDirection(c.in)
		require.Equal(t, c.route, route, c.in)
		require.Equal(t, c.dir, dir, c.in)
	}
}

type overviewFixture struct {
	user                          *models.User
	acme, other                   *models.Customer
	teach, sick, vacation, travel *models.Project
	contract                      *models.Contract
	cleanup                       func()
}

func setupOverview(t *testing.T) *overviewFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	prevDB := database.DB
	database.DB = db
	prevNow := overviewNow
	overviewNow = func() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC) }

	f := &overviewFixture{}
	f.cleanup = func() {
		overviewNow = prevNow
		database.DB = prevDB
		cleanup()
	}
	f.user = &models.User{Username: "ton", Email: "ton@example.com", PasswordHash: "x", GlobalRole: "member"}
	require.NoError(t, db.Create(f.user).Error)
	f.acme = &models.Customer{Name: "Acme"}
	f.other = &models.Customer{Name: "Other"}
	require.NoError(t, db.Create(f.acme).Error)
	require.NoError(t, db.Create(f.other).Error)
	mk := func(name, slug string, undecl int) *models.Project {
		p := &models.Project{Name: name, Slug: slug, KeyPrefix: slug, BoardType: "kanban", TimeTrackingOnly: true, UndeclarableMinutes: undecl, CreatedByID: f.user.ID}
		require.NoError(t, db.Create(p).Error)
		return p
	}
	f.teach = mk("Lesgeven", "teach", 0)
	f.sick = mk("Ziek", "sick", 0)
	f.vacation = mk("Vrij", "vac", 0)
	f.travel = mk("Reistijd", "trv", 75)
	f.contract = &models.Contract{CustomerID: f.acme.ID, Name: "Employment"}
	require.NoError(t, db.Create(f.contract).Error)
	return f
}

func day(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
func sp(s string) *string    { return &s }
func fp(f float64) *float64  { return &f }

func (f *overviewFixture) add(t *testing.T, e models.TimeEntry) models.TimeEntry {
	t.Helper()
	e.UserID = f.user.ID
	require.NoError(t, database.DB.Create(&e).Error)
	return e
}

func (f *overviewFixture) get(t *testing.T, h gin.HandlerFunc, query string, out interface{}) int {
	t.Helper()
	c, w := ginTestContext(t, http.MethodGet, "/x"+query, "")
	c.Set(middleware.ContextUserID, f.user.ID)
	c.Set(middleware.ContextGlobalRole, "member")
	h(c)
	if out != nil && w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), out))
	}
	return w.Code
}

func (f *overviewFixture) put(t *testing.T, body string, out interface{}) int {
	t.Helper()
	c, w := ginTestContext(t, http.MethodPut, "/x", body)
	c.Set(middleware.ContextUserID, f.user.ID)
	c.Set(middleware.ContextGlobalRole, "member")
	UpdateOverviewSettings(c)
	if out != nil && w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), out))
	}
	return w.Code
}

// The norm counts Monday–Friday minus the user's own holiday entries, 8 h a
// day at 40 h/week; booked time is split into worked (travel is a part of it),
// sick and vacation, days are counted once, and time after today is planned.
func TestOverviewNorm(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	f.add(t, models.TimeEntry{Date: day("2026-01-01"), IsHoliday: true, Description: "Nieuwjaarsdag"})
	f.add(t, models.TimeEntry{Date: day("2026-04-05"), IsHoliday: true, Description: "Eerste Paasdag"}) // Sunday: no norm day lost
	f.add(t, models.TimeEntry{Date: day("2026-04-27"), IsHoliday: true, Description: "Koningsdag"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.sick.ID, Date: day("2026-01-02"), Minutes: 480})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-03-30"), Minutes: 480, Distance: fp(10)})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.travel.ID, Date: day("2026-03-30"), Minutes: 105, Distance: fp(42.5)})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.vacation.ID, Date: day("2026-12-24"), Minutes: 480}) // planned

	var o NormOverview
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewNorm, "?year=2026", &o))

	require.Equal(t, 261, o.Weekdays)
	require.Equal(t, []string{"2026-01-01", "2026-04-27"}, o.HolidayWeekdays)
	require.Equal(t, 259, o.Total.NormDays)
	require.Equal(t, 259*480, o.Total.NormMinutes)
	require.Equal(t, 480, o.NormMinutesPerDay)
	require.Equal(t, 40.0, o.HoursPerWeek)

	jan := o.Months[0]
	require.Equal(t, 21, jan.NormDays) // 22 weekdays minus Nieuwjaarsdag
	require.Equal(t, 480, jan.SickMinutes)
	require.Equal(t, 0, jan.WorkedMinutes)

	mar := o.Months[2]
	require.Equal(t, 585, mar.WorkedMinutes)
	require.Equal(t, 105, mar.TravelMinutes)
	require.Equal(t, 30, mar.TravelDeclarableMinutes) // 105 minus 75 undeclarable
	require.Equal(t, 52.5, mar.Distance)              // travel and teaching km together
	require.Equal(t, 30, o.Total.TravelDeclarableMinutes)
	require.Equal(t, 52.5, o.Total.Distance)
	require.Equal(t, 1, mar.BookedDays) // two entries, one day

	require.Equal(t, 480, o.Total.SickMinutes)
	require.Equal(t, 480, o.Total.VacationMinutes)
	require.Equal(t, 585, o.Total.WorkedMinutes)
	require.Equal(t, 480+585+480, o.Total.BookedMinutes)
	require.Equal(t, o.Total.BookedMinutes-o.Total.NormMinutes, o.Total.DiffMinutes)
	require.Equal(t, 3, o.TotalDays)
	require.Equal(t, 480, o.PlannedMinutes)
	require.Equal(t, 480+585, o.BookedToDateMinute)

	// Past norm days without any booking.
	require.Greater(t, o.Total.PastUnbookedNormDays, 150)
	require.Equal(t, o.Total.NormDays-3, o.Total.UnbookedNormDays) // booked norm days: 2 Jan, 30 Mar, 24 Dec
	// ... and norm to date stops at today (2026-11-17).
	require.Less(t, o.NormToDateMinutes, o.Total.NormMinutes)

	// A 36-hour week lowers the norm per day to 432 minutes.
	require.NoError(t, database.DB.Create(&models.TimeNormYear{UserID: f.user.ID, Year: 2026, HoursPerWeek: fp(36)}).Error)
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewNorm, "?year=2026", &o))
	require.Equal(t, 432, o.NormMinutesPerDay)
	require.Equal(t, 259*432, o.Total.NormMinutes)
}

// Settings are stored per year; a year without a row inherits the weekly
// hours and vacation entitlement from the nearest earlier year, but never the
// carry-over. Out-of-range values and unknown projects are refused.
func TestOverviewSettings(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	var s OverviewSettings
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2025,"hours_per_week":36,"vacation_hours":294,"carry_over_hours":10}`, &s))
	require.Equal(t, 36.0, s.HoursPerWeek)
	require.True(t, s.HasRow)

	require.Equal(t, http.StatusOK, f.get(t, GetOverviewSettings, "?year=2026", &s))
	require.Equal(t, 36.0, s.HoursPerWeek)
	require.Equal(t, 294.0, s.VacationHours)
	require.Equal(t, 0.0, s.CarryOverHours)
	require.False(t, s.HasRow)
	require.True(t, s.AutoCategories)

	require.Equal(t, http.StatusOK, f.get(t, GetOverviewSettings, "?year=2024", &s)) // before any row: defaults
	require.Equal(t, 40.0, s.HoursPerWeek)
	require.Equal(t, 0.0, s.VacationHours)

	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":2026,"hours_per_week":0}`, nil))
	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":2026,"hours_per_week":169}`, nil))
	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":2026,"vacation_hours":-1}`, nil))
	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":1800}`, nil))
	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":2026,"categories":{"sick":[99999]}}`, nil))
	require.Equal(t, http.StatusBadRequest, f.put(t, `{"year":2026,"categories":{"bogus":[]}}`, nil))

	// Auto categories guess Ziek/Vrij/Reistijd once the user has booked on them.
	f.add(t, models.TimeEntry{ProjectID: &f.sick.ID, Date: day("2026-02-02"), Minutes: 60})
	f.add(t, models.TimeEntry{ProjectID: &f.travel.ID, Date: day("2026-02-02"), Minutes: 60})
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewSettings, "?year=2026", &s))
	require.Equal(t, []uint{f.sick.ID}, s.Categories["sick"])
	require.Equal(t, []uint{f.travel.ID}, s.Categories["travel"])

	// An explicit mapping replaces the guess; an empty one returns to guessing.
	body := `{"year":2026,"categories":{"vacation":[` + itoa(f.teach.ID) + `]}}`
	require.Equal(t, http.StatusOK, f.put(t, body, &s))
	require.False(t, s.AutoCategories)
	require.Equal(t, []uint{f.teach.ID}, s.Categories["vacation"])
	require.Empty(t, s.Categories["sick"])
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2026,"categories":{}}`, &s))
	require.True(t, s.AutoCategories)

	// A key that is left out is not touched: setting only the carry-over keeps
	// the year's own hours and vacation.
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2026,"hours_per_week":32,"vacation_hours":180}`, &s))
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2026,"carry_over_hours":12}`, &s))
	require.Equal(t, 32.0, s.HoursPerWeek)
	require.Equal(t, 180.0, s.VacationHours)
	require.Equal(t, 12.0, s.CarryOverHours)

	// Clearing a number (null) removes the year's own value again.
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2026,"hours_per_week":null,"vacation_hours":null}`, &s))
	require.Equal(t, 36.0, s.HoursPerWeek) // inherited from 2025 again
}

// Vacation balance: entitlement + the carry-over entered for the year, minus
// vacation used (up to today) and planned (after today); the previous year's
// remaining is suggested as carry-over when that year has data.
func TestOverviewVacation(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	require.Equal(t, http.StatusOK, f.put(t, `{"year":2025,"vacation_hours":294,"carry_over_hours":10}`, nil))
	require.Equal(t, http.StatusOK, f.put(t, `{"year":2026,"carry_over_hours":16}`, nil))
	f.add(t, models.TimeEntry{ProjectID: &f.vacation.ID, Date: day("2025-08-01"), Minutes: 480*30 + 240}) // 30.5 days in 2025
	f.add(t, models.TimeEntry{ProjectID: &f.vacation.ID, Date: day("2026-07-01"), Minutes: 480})
	f.add(t, models.TimeEntry{ProjectID: &f.vacation.ID, Date: day("2026-07-02"), Minutes: 240})
	f.add(t, models.TimeEntry{ProjectID: &f.vacation.ID, Date: day("2026-12-24"), Minutes: 480}) // planned

	var v VacationOverview
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewVacation, "?year=2026", &v))
	require.Equal(t, 294*60, v.EntitlementMinutes) // inherited from 2025
	require.Equal(t, 16*60, v.CarryOverMinutes)    // entered for 2026, not inherited
	require.Equal(t, (294+16)*60, v.AvailableMinutes)
	require.Equal(t, 720, v.UsedMinutes)
	require.Equal(t, 480, v.PlannedMinutes)
	require.Equal(t, (294+16)*60-720, v.RemainingMinutes)
	require.Equal(t, (294+16)*60-720-480, v.RemainingAfterPlan)
	require.Equal(t, 3, v.TotalDays)
	require.Equal(t, 480, v.Months[11].Minutes)
	require.NotNil(t, v.SuggestedCarryOver)
	require.Equal(t, (294+10)*60-(480*30+240), *v.SuggestedCarryOver)

	// A year with no data and no row suggests nothing.
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewVacation, "?year=2030", &v))
	require.Nil(t, v.SuggestedCarryOver)
}

// Travel splits into own (undeclarable) and declarable time per entry, and
// kilometres add up per direction, customer and route.
func TestOverviewTravel(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.travel.ID, Date: day("2026-03-30"), Minutes: 90, Description: "Python - Plaats - Heen", Distance: fp(95), StartTime: sp("06:15"), EndTime: sp("07:45")})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.travel.ID, Date: day("2026-03-30"), Minutes: 105, Description: "Python - Plaats - Terug", Distance: fp(95), StartTime: sp("16:30"), EndTime: sp("18:15")})
	f.add(t, models.TimeEntry{CustomerID: &f.other.ID, ProjectID: &f.travel.ID, Date: day("2026-10-01"), Minutes: 45, Description: "Velp - Heen"}) // shorter than the 75 own minutes, no distance
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-03-30"), Minutes: 480})                              // not travel
	// Kilometres on a work entry count; its time, entry and day do not.
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-11-04"), Minutes: 480, Description: "Lesgeven", Distance: fp(12.5)})
	f.add(t, models.TimeEntry{Date: day("2026-12-25"), IsHoliday: true, Description: "Kerst", Distance: fp(99)}) // holiday: ignored

	var o TravelOverview
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewTravel, "?year=2026", &o))
	require.Equal(t, 3, o.Total.Entries)
	require.Equal(t, 240, o.Total.TotalMinutes)
	require.Equal(t, 75+75+45, o.Total.OwnMinutes) // own time never exceeds the entry
	require.Equal(t, 15+30, o.Total.DeclarableMinutes)
	require.Equal(t, 202.5, o.Total.Distance)
	require.Equal(t, 2, o.Total.Days)
	require.Equal(t, 2, o.TotalDays)
	require.Equal(t, 1, o.WithoutDistance)
	require.Equal(t, 2, o.Out.Entries)
	require.Equal(t, 1, o.Back.Entries)
	require.Equal(t, 0, o.NoDirection.Entries)
	require.Equal(t, 12.5, o.NoDirection.Distance)

	require.Equal(t, 195, o.Months[2].TotalMinutes)
	require.Equal(t, 1, o.Months[2].Days)
	require.Equal(t, 45, o.Months[9].TotalMinutes)
	require.Equal(t, 12.5, o.Months[10].Distance)
	require.Equal(t, 0, o.Months[10].Entries)
	require.Equal(t, 0, o.Months[10].Days)
	require.Equal(t, 0.0, o.Months[11].Distance)

	require.Len(t, o.ByCustomer, 2)
	require.Equal(t, "Acme", o.ByCustomer[0].Customer)
	require.Equal(t, 202.5, o.ByCustomer[0].Distance)
	require.Equal(t, 3, o.ByCustomer[0].Entries+o.ByCustomer[1].Entries)
	require.Len(t, o.ByRoute, 3) // plus "Lesgeven" with only its kilometres
	require.Equal(t, "Python - Plaats", o.ByRoute[0].Route)
	require.Equal(t, 2, o.ByRoute[0].Entries)
	require.Equal(t, 190.0, o.ByRoute[0].Distance)
}

// The checks flag overlaps, mismatching times, travel without distance,
// entries without a contract (when the customer has one), over/under-norm
// days, weekend entries and past norm days without any booking.
func TestOverviewChecks(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	// 2026-03-02 (Monday): overlap + mismatch + no contract + travel without distance.
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ContractID: &f.contract.ID, ProjectID: &f.teach.ID, Date: day("2026-03-02"), Minutes: 480, StartTime: sp("08:30"), EndTime: sp("16:30"), Description: "Les"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ContractID: &f.contract.ID, ProjectID: &f.travel.ID, Date: day("2026-03-02"), Minutes: 90, StartTime: sp("16:00"), EndTime: sp("17:30"), Description: "Les - Terug"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ContractID: &f.contract.ID, ProjectID: &f.teach.ID, Date: day("2026-03-03"), Minutes: 60, StartTime: sp("08:00"), EndTime: sp("10:00"), Description: "Mismatch"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-03-04"), Minutes: 600, Description: "Lang, geen contract, geen tijden"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ContractID: &f.contract.ID, ProjectID: &f.travel.ID, Date: day("2026-03-07"), Minutes: 60, Distance: fp(10), StartTime: sp("10:00"), EndTime: sp("11:00"), Description: "Zaterdag - Heen"})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ContractID: &f.contract.ID, ProjectID: &f.teach.ID, Date: day("2026-03-05"), Minutes: 480, StartTime: sp("08:00"), EndTime: sp("16:00"), Description: "Les"})
	f.add(t, models.TimeEntry{ProjectID: &f.sick.ID, Date: day("2026-03-06"), Minutes: 480, Description: "Ziek"}) // sick without times is fine

	var o ChecksOverview
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewChecks, "?year=2026", &o))
	require.Equal(t, 1, o.Counts["overlap"])
	require.Equal(t, 1, o.Counts["duration_mismatch"])
	require.Equal(t, 1, o.Counts["missing_distance"]) // 2 March travel; the 7 March one has a distance
	require.Equal(t, 1, o.Counts["no_contract"])
	require.Equal(t, 1, o.Counts["no_times"]) // 4 March; Ziek is exempt
	require.Equal(t, 1, o.Counts["weekend_entries"])
	require.Equal(t, 1, o.Counts["day_over_norm"])  // 4 March: 600 > 480
	require.Equal(t, 1, o.Counts["day_under_norm"]) // 3 March: 60 < 480

	dates := map[string][]string{}
	for _, fi := range o.Findings {
		dates[fi.Type] = append(dates[fi.Type], fi.Date)
	}
	require.Equal(t, []string{"2026-03-02"}, dates["overlap"])
	require.Contains(t, dates["day_without_entries"], "2026-03-09") // a past Monday with nothing booked
	require.NotContains(t, dates["day_without_entries"], "2026-03-02")
	require.NotContains(t, dates["day_without_entries"], "2026-11-18") // after today

	// Description variants per project, most variants first.
	require.NotEmpty(t, o.DescriptionVariants)
	require.Equal(t, "Lesgeven", o.DescriptionVariants[0].Project)
	require.Len(t, o.DescriptionVariants[0].Variants, 3)
	require.Equal(t, "Les", o.DescriptionVariants[0].Variants[0].Description)
	require.Equal(t, 2, o.DescriptionVariants[0].Variants[0].Count)
}

// Another user's overview is only readable by admins and time-tracking
// viewers; a plain user's ?user_id= is ignored.
func TestOverviewOtherUser(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	other := &models.User{Username: "oth", Email: "oth@example.com", PasswordHash: "x", GlobalRole: "member"}
	require.NoError(t, database.DB.Create(other).Error)
	require.NoError(t, database.DB.Create(&models.TimeEntry{UserID: other.ID, ProjectID: &f.vacation.ID, Date: day("2026-07-01"), Minutes: 480}).Error)

	var v VacationOverview
	require.Equal(t, http.StatusOK, f.get(t, GetOverviewVacation, "?year=2026&user_id="+itoa(other.ID), &v))
	require.Equal(t, 0, v.UsedMinutes) // ignored: still the caller's own data

	c, w := ginTestContext(t, http.MethodGet, "/x?year=2026&user_id="+itoa(other.ID), "")
	c.Set(middleware.ContextUserID, f.user.ID)
	c.Set(middleware.ContextGlobalRole, "admin")
	GetOverviewVacation(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v))
	require.Equal(t, 480, v.UsedMinutes)
}

// The report counts distinct days: a day with several entries counts once, in
// its group and in the total, and the total is not the sum of the groups.
func TestReportDistinctDays(t *testing.T) {
	f := setupOverview(t)
	defer f.cleanup()

	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-03-02"), Minutes: 240})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.travel.ID, Date: day("2026-03-02"), Minutes: 60})
	f.add(t, models.TimeEntry{CustomerID: &f.other.ID, ProjectID: &f.teach.ID, Date: day("2026-03-02"), Minutes: 240})
	f.add(t, models.TimeEntry{CustomerID: &f.acme.ID, ProjectID: &f.teach.ID, Date: day("2026-03-03"), Minutes: 480})
	f.add(t, models.TimeEntry{Date: day("2026-04-27"), IsHoliday: true, Description: "Koningsdag"}) // holidays are not days worked

	var r TimeEntryReportResponse
	require.Equal(t, http.StatusOK, f.get(t, GetTimeEntryReport, "?period=year&year=2026&group_by=customer", &r))
	days := map[string]int{}
	sum := 0
	for _, g := range r.Groups {
		days[g.Label] = g.Days
		sum += g.Days
	}
	require.Equal(t, 2, days["Acme"])
	require.Equal(t, 1, days["Other"])
	require.Equal(t, 3, sum)
	require.Equal(t, 2, r.TotalDays) // 2 and 3 March, not 3
}
