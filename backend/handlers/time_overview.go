package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
)

// Time overviews: year norm, consistency checks, travel and kilometres, and
// the vacation balance. All of them describe one user's entries for one year
// (?year=, default the current year; admins and time-tracking viewers may pass
// ?user_id=). The rules live here so the overviews cannot disagree:
//
//   - A norm day is a Monday–Friday that is not one of the user's holiday
//     entries (is_holiday). A holiday entry is what makes a day free.
//   - The norm per day is hours_per_week/5 (TimeNormYear, default 40 h).
//   - Entries are classified by project: "sick", "vacation" or "travel"
//     (TimeCategoryProject); every other entry counts as worked time. With no
//     mapping stored the category is guessed from the project name.
//   - A day counts as booked when it has at least one non-holiday entry, so
//     "days" never double-counts a day with several entries.

const (
	overviewCatSick     = "sick"
	overviewCatVacation = "vacation"
	overviewCatTravel   = "travel"

	defaultHoursPerWeek = 40.0
)

// overviewNow is the clock the overviews use; tests replace it.
var overviewNow = time.Now

var (
	sickNames     = map[string]bool{"ziek": true, "ziekte": true, "sick": true, "sick leave": true, "krank": true, "malade": true}
	vacationNames = map[string]bool{"vrij": true, "verlof": true, "vakantie": true, "vacation": true, "urlaub": true, "holiday": true, "holidays": true, "leave": true, "congé": true}
	travelNames   = map[string]bool{"reistijd": true, "reizen": true, "travel": true, "travel time": true, "reisezeit": true}

	directionRE = regexp.MustCompile(`(?i)^(.*?)\s*-\s*(heen|terug|outbound|return)\s*$`)
)

// guessOverviewCategory guesses a category from a project name.
func guessOverviewCategory(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case sickNames[n]:
		return overviewCatSick
	case vacationNames[n]:
		return overviewCatVacation
	case travelNames[n]:
		return overviewCatTravel
	}
	return ""
}

// travelDirection splits "Course - Place - Heen" into the route
// ("Course - Place") and the direction ("out", "back" or "" when the
// description has no direction suffix).
func travelDirection(desc string) (string, string) {
	m := directionRE.FindStringSubmatch(strings.TrimSpace(desc))
	if m == nil {
		return strings.TrimSpace(desc), ""
	}
	switch strings.ToLower(m[2]) {
	case "heen", "outbound":
		return strings.TrimSpace(m[1]), "out"
	}
	return strings.TrimSpace(m[1]), "back"
}

type overviewScope struct {
	UserID  uint
	Year    int
	Today   time.Time // date only, UTC midnight
	From    time.Time
	To      time.Time
	Entries []models.TimeEntry
	Cfg     normSettings

	explicit map[uint]string
	auto     bool
}

type normSettings struct {
	Year           int
	HoursPerWeek   float64
	VacationHours  float64
	CarryOverHours float64
	HasRow         bool
}

func (s normSettings) normMinutesPerDay() int {
	return int(math.Round(s.HoursPerWeek * 60 / 5))
}

func dateKey(t time.Time) string { return t.UTC().Format("2006-01-02") }

func hhmmToMinutes(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// loadNormSettings returns the effective settings for a year: the year's own
// row, else hours_per_week and vacation_hours of the nearest earlier year,
// else the defaults (40 h, no vacation). Carry-over is never inherited.
func loadNormSettings(userID uint, year int) normSettings {
	cfg := normSettings{Year: year, HoursPerWeek: defaultHoursPerWeek}
	var rows []models.TimeNormYear
	database.DB.Where("user_id = ? AND year <= ?", userID, year).Order("year desc").Find(&rows)
	gotHours, gotVac := false, false
	for _, r := range rows {
		if r.Year == year {
			cfg.HasRow = true
			if r.CarryOverHours != nil {
				cfg.CarryOverHours = *r.CarryOverHours
			}
		}
		if !gotHours && r.HoursPerWeek != nil {
			cfg.HoursPerWeek, gotHours = *r.HoursPerWeek, true
		}
		if !gotVac && r.VacationHours != nil {
			cfg.VacationHours, gotVac = *r.VacationHours, true
		}
	}
	return cfg
}

func loadOverviewScope(c *gin.Context) (*overviewScope, int, string) {
	userID := middleware.GetUserID(c)
	globalRole := middleware.GetGlobalRole(c)

	var viewer models.User
	database.DB.Select("time_tracking_viewer", "timezone").First(&viewer, userID)

	target := userID
	if globalRole == "admin" || viewer.TimeTrackingViewer {
		if s := c.Query("user_id"); s != "" {
			id, err := strconv.ParseUint(s, 10, 64)
			if err != nil || id == 0 {
				return nil, http.StatusBadRequest, "invalid user_id"
			}
			target = uint(id)
		}
	}
	year := intOrDefault(c.Query("year"), overviewNow().Year())
	return buildOverviewScope(target, year)
}

func buildOverviewScope(target uint, year int) (*overviewScope, int, string) {
	now := overviewNow()
	if year < 1970 || year > 2200 {
		return nil, http.StatusBadRequest, "invalid year"
	}

	var tu models.User
	database.DB.Select("timezone").First(&tu, target)
	loc := time.UTC
	if tu.Timezone != "" {
		if l, err := time.LoadLocation(tu.Timezone); err == nil {
			loc = l
		}
	}
	ln := now.In(loc)
	today := time.Date(ln.Year(), ln.Month(), ln.Day(), 0, 0, 0, 0, time.UTC)

	sc := &overviewScope{UserID: target, Year: year, Today: today}
	sc.From = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	sc.To = sc.From.AddDate(1, 0, 0)
	if err := database.DB.Preload("Project").Preload("Customer").
		Where("user_id = ? AND date >= ? AND date < ?", target, sc.From, sc.To).
		Order("date asc, id asc").Find(&sc.Entries).Error; err != nil {
		return nil, http.StatusInternalServerError, "internal error"
	}
	sc.Cfg = loadNormSettings(target, year)

	var rows []models.TimeCategoryProject
	database.DB.Where("user_id = ?", target).Find(&rows)
	sc.auto = len(rows) == 0
	sc.explicit = map[uint]string{}
	for _, r := range rows {
		sc.explicit[r.ProjectID] = r.Category
	}
	return sc, 0, ""
}

// category returns "sick", "vacation", "travel" or "" (worked time).
func (sc *overviewScope) category(e models.TimeEntry) string {
	if e.IsHoliday || e.Project == nil {
		return ""
	}
	if sc.auto {
		return guessOverviewCategory(e.Project.Name)
	}
	return sc.explicit[e.Project.ID]
}

// holidays returns the user's holiday entry dates of the year.
func (sc *overviewScope) holidays() map[string]bool {
	h := map[string]bool{}
	for _, e := range sc.Entries {
		if e.IsHoliday {
			h[dateKey(e.Date)] = true
		}
	}
	return h
}

// normDays lists the norm days of the year (Mon–Fri, not a holiday entry).
func (sc *overviewScope) normDays() []time.Time {
	hol := sc.holidays()
	var out []time.Time
	for d := sc.From; d.Before(sc.To); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday || hol[dateKey(d)] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// bookedDays returns the dates with at least one non-holiday entry.
func (sc *overviewScope) bookedDays() map[string]bool {
	b := map[string]bool{}
	for _, e := range sc.Entries {
		if !e.IsHoliday {
			b[dateKey(e.Date)] = true
		}
	}
	return b
}

// ── Year norm ─────────────────────────────────────────────────────────────

type normMonthRow struct {
	Month         int `json:"month"`
	NormDays      int `json:"norm_days"`
	NormMinutes   int `json:"norm_minutes"`
	WorkedMinutes int `json:"worked_minutes"` // everything except sick and vacation, incl. travel
	TravelMinutes int `json:"travel_minutes"` // part of worked_minutes
	// TravelDeclarableMinutes is the part of travel_minutes that remains
	// after the project's undeclarable minutes (see projUndecl).
	TravelDeclarableMinutes int     `json:"travel_declarable_minutes"`
	Distance                float64 `json:"distance"` // km on all entries of the month, any project
	SickMinutes             int     `json:"sick_minutes"`
	VacationMinutes         int     `json:"vacation_minutes"`
	BookedMinutes           int     `json:"booked_minutes"`
	DiffMinutes             int     `json:"diff_minutes"`
	BookedDays              int     `json:"booked_days"`
	UnbookedNormDays        int     `json:"unbooked_norm_days"`
	PastUnbookedNormDays    int     `json:"past_unbooked_norm_days"`
}

// NormOverview is the shape returned by GetOverviewNorm.
type NormOverview struct {
	Year               int            `json:"year"`
	HoursPerWeek       float64        `json:"hours_per_week"`
	NormMinutesPerDay  int            `json:"norm_minutes_per_day"`
	Weekdays           int            `json:"weekdays"`
	HolidayWeekdays    []string       `json:"holiday_weekdays"`
	Months             []normMonthRow `json:"months"`
	Total              normMonthRow   `json:"total"`
	NormToDateMinutes  int            `json:"norm_to_date_minutes"`
	BookedToDateMinute int            `json:"booked_to_date_minutes"`
	DiffToDateMinutes  int            `json:"diff_to_date_minutes"`
	PlannedMinutes     int            `json:"planned_minutes"` // booked after today
	TotalDays          int            `json:"total_days"`
}

func buildNormOverview(sc *overviewScope) NormOverview {
	perDay := sc.Cfg.normMinutesPerDay()
	rows := make([]normMonthRow, 12)
	for i := range rows {
		rows[i].Month = i + 1
	}
	hol := sc.holidays()
	booked := sc.bookedDays()
	out := NormOverview{Year: sc.Year, HoursPerWeek: sc.Cfg.HoursPerWeek, NormMinutesPerDay: perDay}

	weekdays := 0
	for d := sc.From; d.Before(sc.To); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		weekdays++
		if hol[dateKey(d)] {
			out.HolidayWeekdays = append(out.HolidayWeekdays, dateKey(d))
		}
	}
	out.Weekdays = weekdays
	if out.HolidayWeekdays == nil {
		out.HolidayWeekdays = []string{}
	}

	normToDate := 0
	for _, d := range sc.normDays() {
		r := &rows[int(d.Month())-1]
		r.NormDays++
		r.NormMinutes += perDay
		if !booked[dateKey(d)] {
			r.UnbookedNormDays++
			if d.Before(sc.Today) {
				r.PastUnbookedNormDays++
			}
		}
		if !d.After(sc.Today) {
			normToDate += perDay
		}
	}
	for d := range booked {
		t, _ := time.Parse("2006-01-02", d)
		rows[int(t.Month())-1].BookedDays++
	}
	for _, e := range sc.Entries {
		if e.IsHoliday {
			continue
		}
		r := &rows[int(e.Date.UTC().Month())-1]
		switch sc.category(e) {
		case overviewCatSick:
			r.SickMinutes += e.Minutes
		case overviewCatVacation:
			r.VacationMinutes += e.Minutes
		case overviewCatTravel:
			r.TravelMinutes += e.Minutes
			r.TravelDeclarableMinutes += e.Minutes - projUndecl(e)
			r.WorkedMinutes += e.Minutes
		default:
			r.WorkedMinutes += e.Minutes
		}
		r.BookedMinutes += e.Minutes
		if e.Distance != nil {
			r.Distance += *e.Distance
		}
		if !e.Date.UTC().After(sc.Today) {
			out.BookedToDateMinute += e.Minutes
		} else {
			out.PlannedMinutes += e.Minutes
		}
	}
	for i := range rows {
		rows[i].DiffMinutes = rows[i].BookedMinutes - rows[i].NormMinutes
		t := &out.Total
		t.NormDays += rows[i].NormDays
		t.NormMinutes += rows[i].NormMinutes
		t.WorkedMinutes += rows[i].WorkedMinutes
		t.TravelMinutes += rows[i].TravelMinutes
		t.TravelDeclarableMinutes += rows[i].TravelDeclarableMinutes
		t.Distance += rows[i].Distance
		t.SickMinutes += rows[i].SickMinutes
		t.VacationMinutes += rows[i].VacationMinutes
		t.BookedMinutes += rows[i].BookedMinutes
		t.BookedDays += rows[i].BookedDays
		t.UnbookedNormDays += rows[i].UnbookedNormDays
		t.PastUnbookedNormDays += rows[i].PastUnbookedNormDays
	}
	out.Total.DiffMinutes = out.Total.BookedMinutes - out.Total.NormMinutes
	out.Months = rows
	out.NormToDateMinutes = normToDate
	out.DiffToDateMinutes = out.BookedToDateMinute - normToDate
	out.TotalDays = out.Total.BookedDays
	return out
}

// GetOverviewNorm returns the booked hours of a year against the norm, per
// month, with worked / sick / vacation (and the travel part of worked) apart.
func GetOverviewNorm(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildNormOverview(sc))
}

// ── Travel and kilometres ─────────────────────────────────────────────────

type travelAcc struct {
	Entries           int     `json:"entries"`
	Days              int     `json:"days"`
	TotalMinutes      int     `json:"total_minutes"`
	OwnMinutes        int     `json:"own_minutes"`
	DeclarableMinutes int     `json:"declarable_minutes"`
	Distance          float64 `json:"distance"`

	dates map[string]bool
}

func (a *travelAcc) add(e models.TimeEntry) {
	if a.dates == nil {
		a.dates = map[string]bool{}
	}
	own := projUndecl(e)
	a.Entries++
	a.TotalMinutes += e.Minutes
	a.OwnMinutes += own
	a.DeclarableMinutes += e.Minutes - own
	if e.Distance != nil {
		a.Distance += *e.Distance
	}
	a.dates[dateKey(e.Date)] = true
	a.Days = len(a.dates)
}

type travelMonthRow struct {
	Month int `json:"month"`
	travelAcc
}

type travelCustomerRow struct {
	CustomerID *uint  `json:"customer_id"`
	Customer   string `json:"customer"`
	travelAcc
}

type travelRouteRow struct {
	Route    string `json:"route"`
	Customer string `json:"customer"`
	travelAcc
}

// TravelOverview is the shape returned by GetOverviewTravel.
type TravelOverview struct {
	Year               int                 `json:"year"`
	Months             []travelMonthRow    `json:"months"`
	Total              travelAcc           `json:"total"`
	Out                travelAcc           `json:"out"`
	Back               travelAcc           `json:"back"`
	NoDirection        travelAcc           `json:"no_direction"`
	ByCustomer         []travelCustomerRow `json:"by_customer"`
	ByRoute            []travelRouteRow    `json:"by_route"`
	WithoutDistance    int                 `json:"entries_without_distance"`
	TotalDays          int                 `json:"total_days"`
	TravelProjectCount int                 `json:"travel_project_count"`
}

func buildTravelOverview(sc *overviewScope) TravelOverview {
	out := TravelOverview{Year: sc.Year}
	months := make([]travelMonthRow, 12)
	for i := range months {
		months[i].Month = i + 1
	}
	byCust := map[uint]*travelCustomerRow{}
	byRoute := map[string]*travelRouteRow{}
	projects := map[uint]bool{}
	for _, e := range sc.Entries {
		if sc.category(e) != overviewCatTravel {
			continue
		}
		projects[e.Project.ID] = true
		months[int(e.Date.UTC().Month())-1].add(e)
		out.Total.add(e)
		route, dir := travelDirection(e.Description)
		switch dir {
		case "out":
			out.Out.add(e)
		case "back":
			out.Back.add(e)
		default:
			out.NoDirection.add(e)
		}
		if e.Distance == nil {
			out.WithoutDistance++
		}
		var cid uint
		cname := ""
		if e.CustomerID != nil {
			cid = *e.CustomerID
		}
		if e.Customer != nil {
			cname = e.Customer.Name
		}
		cr := byCust[cid]
		if cr == nil {
			cr = &travelCustomerRow{Customer: cname}
			if e.CustomerID != nil {
				v := *e.CustomerID
				cr.CustomerID = &v
			}
			byCust[cid] = cr
		}
		cr.add(e)
		key := cname + "\x00" + route
		rr := byRoute[key]
		if rr == nil {
			rr = &travelRouteRow{Route: route, Customer: cname}
			byRoute[key] = rr
		}
		rr.add(e)
	}
	out.Months = months
	out.TotalDays = out.Total.Days
	out.TravelProjectCount = len(projects)
	out.ByCustomer = []travelCustomerRow{}
	for _, r := range byCust {
		out.ByCustomer = append(out.ByCustomer, *r)
	}
	sort.Slice(out.ByCustomer, func(i, j int) bool {
		if out.ByCustomer[i].TotalMinutes != out.ByCustomer[j].TotalMinutes {
			return out.ByCustomer[i].TotalMinutes > out.ByCustomer[j].TotalMinutes
		}
		return out.ByCustomer[i].Customer < out.ByCustomer[j].Customer
	})
	out.ByRoute = []travelRouteRow{}
	for _, r := range byRoute {
		out.ByRoute = append(out.ByRoute, *r)
	}
	sort.Slice(out.ByRoute, func(i, j int) bool {
		if out.ByRoute[i].Distance != out.ByRoute[j].Distance {
			return out.ByRoute[i].Distance > out.ByRoute[j].Distance
		}
		return out.ByRoute[i].Route < out.ByRoute[j].Route
	})
	return out
}

// GetOverviewTravel returns travel time (total, own and declarable) and
// kilometres per month, direction, customer and route. Direction and route
// come from the description: "<route> - Heen" / "<route> - Terug".
func GetOverviewTravel(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildTravelOverview(sc))
}

// ── Vacation balance ──────────────────────────────────────────────────────

type vacationMonthRow struct {
	Month   int `json:"month"`
	Minutes int `json:"minutes"`
	Days    int `json:"days"`
}

// VacationOverview is the shape returned by GetOverviewVacation.
type VacationOverview struct {
	Year                int                `json:"year"`
	EntitlementMinutes  int                `json:"entitlement_minutes"`
	CarryOverMinutes    int                `json:"carry_over_minutes"`
	AvailableMinutes    int                `json:"available_minutes"`
	UsedMinutes         int                `json:"used_minutes"`    // vacation booked up to and including today
	PlannedMinutes      int                `json:"planned_minutes"` // vacation booked after today
	RemainingMinutes    int                `json:"remaining_minutes"`
	RemainingAfterPlan  int                `json:"remaining_after_planned_minutes"`
	Months              []vacationMonthRow `json:"months"`
	TotalDays           int                `json:"total_days"`
	SuggestedCarryOver  *int               `json:"suggested_carry_over_minutes"` // previous year's remaining, when known
	HasSettingsRow      bool               `json:"has_settings_row"`
	VacationProjectsSet bool               `json:"vacation_projects_set"`
}

func vacationTotals(sc *overviewScope) (used, planned int, months []vacationMonthRow, days int) {
	months = make([]vacationMonthRow, 12)
	monthDays := make([]map[string]bool, 12)
	for i := range months {
		months[i].Month = i + 1
		monthDays[i] = map[string]bool{}
	}
	for _, e := range sc.Entries {
		if sc.category(e) != overviewCatVacation {
			continue
		}
		i := int(e.Date.UTC().Month()) - 1
		months[i].Minutes += e.Minutes
		monthDays[i][dateKey(e.Date)] = true
		if e.Date.UTC().After(sc.Today) {
			planned += e.Minutes
		} else {
			used += e.Minutes
		}
	}
	for i := range months {
		months[i].Days = len(monthDays[i])
		days += months[i].Days
	}
	return
}

func buildVacationOverview(sc *overviewScope) VacationOverview {
	used, planned, months, days := vacationTotals(sc)
	ent := int(math.Round(sc.Cfg.VacationHours * 60))
	carry := int(math.Round(sc.Cfg.CarryOverHours * 60))
	out := VacationOverview{
		Year:                sc.Year,
		EntitlementMinutes:  ent,
		CarryOverMinutes:    carry,
		AvailableMinutes:    ent + carry,
		UsedMinutes:         used,
		PlannedMinutes:      planned,
		Months:              months,
		TotalDays:           days,
		HasSettingsRow:      sc.Cfg.HasRow,
		VacationProjectsSet: !sc.auto,
	}
	out.RemainingMinutes = out.AvailableMinutes - used
	out.RemainingAfterPlan = out.AvailableMinutes - used - planned

	// Suggest the previous year's remaining vacation as this year's carry-over
	// when that year has data (a settings row or vacation entries).
	if prev, st, _ := buildOverviewScope(sc.UserID, sc.Year-1); st == 0 {
		pu, pp, _, _ := vacationTotals(prev)
		if prev.Cfg.HasRow || pu+pp > 0 {
			rem := int(math.Round((prev.Cfg.VacationHours+prev.Cfg.CarryOverHours)*60)) - pu - pp
			out.SuggestedCarryOver = &rem
		}
	}
	return out
}

// GetOverviewVacation returns the vacation balance of a year: entitlement plus
// the hours carried over from previous years (entered per year) minus the
// vacation booked, split into used (up to today) and planned (after today).
func GetOverviewVacation(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildVacationOverview(sc))
}

// ── Checks ────────────────────────────────────────────────────────────────

type checkFinding struct {
	Type     string   `json:"type"`
	Date     string   `json:"date"`
	EntryIDs []uint   `json:"entry_ids,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Minutes  int      `json:"minutes,omitempty"`
	Norm     int      `json:"norm_minutes,omitempty"`
}

type descriptionVariant struct {
	Description string `json:"description"`
	Count       int    `json:"count"`
	Minutes     int    `json:"minutes"`
}

type projectVariants struct {
	ProjectID uint                 `json:"project_id"`
	Project   string               `json:"project"`
	Customer  string               `json:"customer"`
	Variants  []descriptionVariant `json:"variants"`
}

// ChecksOverview is the shape returned by GetOverviewChecks.
type ChecksOverview struct {
	Year                int               `json:"year"`
	Findings            []checkFinding    `json:"findings"`
	Counts              map[string]int    `json:"counts"`
	DescriptionVariants []projectVariants `json:"description_variants"`
	TotalDays           int               `json:"total_days"`
}

func entryLabel(e models.TimeEntry) string {
	p := ""
	if e.Project != nil {
		p = e.Project.Name
	}
	t := ""
	if e.StartTime != nil && e.EndTime != nil {
		t = " " + *e.StartTime + "–" + *e.EndTime
	}
	return strings.TrimSpace(p + t + " " + e.Description)
}

func buildChecks(sc *overviewScope) ChecksOverview {
	out := ChecksOverview{Year: sc.Year, Findings: []checkFinding{}, Counts: map[string]int{}}
	add := func(f checkFinding) {
		out.Findings = append(out.Findings, f)
		out.Counts[f.Type]++
	}
	perDay := sc.Cfg.normMinutesPerDay()
	byDay := map[string][]models.TimeEntry{}
	for _, e := range sc.Entries {
		if !e.IsHoliday {
			byDay[dateKey(e.Date)] = append(byDay[dateKey(e.Date)], e)
		}
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	out.TotalDays = len(days)

	// Customers that have at least one contract: entries for them should
	// reference one.
	custContract := map[uint]bool{}
	var cids []uint
	for _, e := range sc.Entries {
		if e.CustomerID != nil {
			cids = append(cids, *e.CustomerID)
		}
	}
	if len(cids) > 0 {
		var cs []models.Contract
		database.DB.Select("customer_id").Where("customer_id IN ?", cids).Find(&cs)
		for _, ct := range cs {
			custContract[ct.CustomerID] = true
		}
	}

	for _, d := range days {
		es := byDay[d]
		type iv struct {
			s, e int
			ent  models.TimeEntry
		}
		var ivs []iv
		work := 0
		for _, e := range es {
			cat := sc.category(e)
			if cat != overviewCatTravel {
				work += e.Minutes
			}
			if e.StartTime != nil && e.EndTime != nil {
				s, ok1 := hhmmToMinutes(*e.StartTime)
				en, ok2 := hhmmToMinutes(*e.EndTime)
				if !ok1 || !ok2 {
					add(checkFinding{Type: "time_format", Date: d, EntryIDs: []uint{e.ID}, Labels: []string{entryLabel(e)}})
					continue
				}
				if en == 0 {
					en = 1440
				}
				if en-s != e.Minutes {
					add(checkFinding{Type: "duration_mismatch", Date: d, EntryIDs: []uint{e.ID}, Labels: []string{entryLabel(e)}, Minutes: e.Minutes})
				}
				ivs = append(ivs, iv{s, en, e})
			} else if cat != overviewCatSick && cat != overviewCatVacation && e.Minutes > 0 {
				add(checkFinding{Type: "no_times", Date: d, EntryIDs: []uint{e.ID}, Labels: []string{entryLabel(e)}, Minutes: e.Minutes})
			}
			if cat == overviewCatTravel && e.Distance == nil {
				add(checkFinding{Type: "missing_distance", Date: d, EntryIDs: []uint{e.ID}, Labels: []string{entryLabel(e)}})
			}
			if e.CustomerID != nil && e.ContractID == nil && custContract[*e.CustomerID] {
				add(checkFinding{Type: "no_contract", Date: d, EntryIDs: []uint{e.ID}, Labels: []string{entryLabel(e)}})
			}
		}
		sort.Slice(ivs, func(i, j int) bool { return ivs[i].s < ivs[j].s })
		for i := 1; i < len(ivs); i++ {
			if ivs[i].s < ivs[i-1].e {
				add(checkFinding{Type: "overlap", Date: d,
					EntryIDs: []uint{ivs[i-1].ent.ID, ivs[i].ent.ID},
					Labels:   []string{entryLabel(ivs[i-1].ent), entryLabel(ivs[i].ent)}})
			}
		}
		t, _ := time.Parse("2006-01-02", d)
		if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			add(checkFinding{Type: "weekend_entries", Date: d, Minutes: work})
		} else if work > perDay {
			add(checkFinding{Type: "day_over_norm", Date: d, Minutes: work, Norm: perDay})
		} else if work < perDay {
			add(checkFinding{Type: "day_under_norm", Date: d, Minutes: work, Norm: perDay})
		}
	}
	// Past norm days without any booking.
	booked := sc.bookedDays()
	for _, d := range sc.normDays() {
		if d.Before(sc.Today) && !booked[dateKey(d)] {
			add(checkFinding{Type: "day_without_entries", Date: dateKey(d)})
		}
	}
	sort.SliceStable(out.Findings, func(i, j int) bool {
		if out.Findings[i].Date != out.Findings[j].Date {
			return out.Findings[i].Date < out.Findings[j].Date
		}
		return out.Findings[i].Type < out.Findings[j].Type
	})

	// Description variants per project.
	vm := map[uint]*projectVariants{}
	vc := map[uint]map[string]*descriptionVariant{}
	for _, e := range sc.Entries {
		if e.IsHoliday || e.Project == nil {
			continue
		}
		pv := vm[e.Project.ID]
		if pv == nil {
			cust := ""
			if e.Customer != nil {
				cust = e.Customer.Name
			}
			pv = &projectVariants{ProjectID: e.Project.ID, Project: e.Project.Name, Customer: cust}
			vm[e.Project.ID] = pv
			vc[e.Project.ID] = map[string]*descriptionVariant{}
		}
		dv := vc[e.Project.ID][e.Description]
		if dv == nil {
			dv = &descriptionVariant{Description: e.Description}
			vc[e.Project.ID][e.Description] = dv
		}
		dv.Count++
		dv.Minutes += e.Minutes
	}
	out.DescriptionVariants = []projectVariants{}
	for id, pv := range vm {
		for _, dv := range vc[id] {
			pv.Variants = append(pv.Variants, *dv)
		}
		sort.Slice(pv.Variants, func(i, j int) bool {
			if pv.Variants[i].Count != pv.Variants[j].Count {
				return pv.Variants[i].Count > pv.Variants[j].Count
			}
			return pv.Variants[i].Description < pv.Variants[j].Description
		})
		out.DescriptionVariants = append(out.DescriptionVariants, *pv)
	}
	sort.Slice(out.DescriptionVariants, func(i, j int) bool {
		a, b := out.DescriptionVariants[i], out.DescriptionVariants[j]
		if len(a.Variants) != len(b.Variants) {
			return len(a.Variants) > len(b.Variants)
		}
		return a.Project < b.Project
	})
	return out
}

// GetOverviewChecks lists what looks wrong or inconsistent in a year's
// entries: overlapping or mismatching times, travel without distance, entries
// without a contract, days over or under the norm, norm days without any
// booking, plus the distinct descriptions per project.
func GetOverviewChecks(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildChecks(sc))
}

// ── Settings ──────────────────────────────────────────────────────────────

// OverviewSettings is the shape of GET/PUT .../overview/settings.
type OverviewSettings struct {
	Year           int               `json:"year"`
	HoursPerWeek   float64           `json:"hours_per_week"`
	VacationHours  float64           `json:"vacation_hours"`
	CarryOverHours float64           `json:"carry_over_hours"`
	HasRow         bool              `json:"has_row"`
	AutoCategories bool              `json:"auto_categories"`
	Categories     map[string][]uint `json:"categories"`
}

func buildOverviewSettings(sc *overviewScope) OverviewSettings {
	s := OverviewSettings{
		Year: sc.Year, HoursPerWeek: sc.Cfg.HoursPerWeek, VacationHours: sc.Cfg.VacationHours,
		CarryOverHours: sc.Cfg.CarryOverHours, HasRow: sc.Cfg.HasRow, AutoCategories: sc.auto,
		Categories: map[string][]uint{overviewCatSick: {}, overviewCatVacation: {}, overviewCatTravel: {}},
	}
	if sc.auto {
		// Effective mapping guessed from every project the user ever booked.
		var ps []models.Project
		database.DB.Raw(`SELECT DISTINCT p.* FROM projects p JOIN time_entries t ON t.project_id = p.id
			WHERE t.user_id = ? AND p.deleted_at IS NULL`, sc.UserID).Scan(&ps)
		for _, p := range ps {
			if cat := guessOverviewCategory(p.Name); cat != "" {
				s.Categories[cat] = append(s.Categories[cat], p.ID)
			}
		}
	} else {
		for id, cat := range sc.explicit {
			s.Categories[cat] = append(s.Categories[cat], id)
		}
	}
	for k := range s.Categories {
		sort.Slice(s.Categories[k], func(i, j int) bool { return s.Categories[k][i] < s.Categories[k][j] })
	}
	return s
}

// GetOverviewSettings returns the effective norm settings for a year and the
// project categories.
func GetOverviewSettings(c *gin.Context) {
	sc, status, msg := loadOverviewScope(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildOverviewSettings(sc))
}

// UpdateOverviewSettings stores a year's norm settings and, when "categories"
// is present, replaces the project categories (an empty mapping switches back
// to guessing from project names). A number that is left out is not touched; a
// null clears it for the year, so it is inherited again (carry-over falls back
// to 0).
func UpdateOverviewSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	globalRole := middleware.GetGlobalRole(c)

	var req struct {
		Year           int               `json:"year"`
		HoursPerWeek   *float64          `json:"hours_per_week"`
		VacationHours  *float64          `json:"vacation_hours"`
		CarryOverHours *float64          `json:"carry_over_hours"`
		Categories     map[string][]uint `json:"categories"`
	}
	body, err := c.GetRawData()
	if err != nil || json.Unmarshal(body, &req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	// A key that is absent leaves the stored value alone; null clears it.
	var present map[string]json.RawMessage
	_ = json.Unmarshal(body, &present)
	has := func(k string) bool { _, ok := present[k]; return ok }
	if req.Year < 1970 || req.Year > 2200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
		return
	}
	if req.HoursPerWeek != nil && (*req.HoursPerWeek <= 0 || *req.HoursPerWeek > 168) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid hours_per_week"})
		return
	}
	if req.VacationHours != nil && (*req.VacationHours < 0 || *req.VacationHours > 2000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vacation_hours"})
		return
	}
	if req.CarryOverHours != nil && (*req.CarryOverHours < -2000 || *req.CarryOverHours > 2000) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid carry_over_hours"})
		return
	}
	target := userID
	if s := c.Query("user_id"); s != "" && globalRole == "admin" {
		id, err := strconv.ParseUint(s, 10, 64)
		if err != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		target = uint(id)
	}
	for cat, ids := range req.Categories {
		if cat != overviewCatSick && cat != overviewCatVacation && cat != overviewCatTravel {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
			return
		}
		if len(ids) > 0 {
			var n int64
			database.DB.Model(&models.Project{}).Where("id IN ? AND deleted_at IS NULL", ids).Count(&n)
			if int(n) != len(uniqueIDs(ids)) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unknown project"})
				return
			}
		}
	}

	var row models.TimeNormYear
	err = database.DB.Where("user_id = ? AND year = ?", target, req.Year).First(&row).Error
	row.UserID, row.Year = target, req.Year
	if has("hours_per_week") {
		row.HoursPerWeek = req.HoursPerWeek
	}
	if has("vacation_hours") {
		row.VacationHours = req.VacationHours
	}
	if has("carry_over_hours") {
		row.CarryOverHours = req.CarryOverHours
	}
	if err != nil {
		err = database.DB.Create(&row).Error
	} else {
		err = database.DB.Save(&row).Error
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if req.Categories != nil {
		database.DB.Where("user_id = ?", target).Delete(&models.TimeCategoryProject{})
		seen := map[uint]bool{}
		for cat, ids := range req.Categories {
			for _, id := range ids {
				if seen[id] {
					continue
				}
				seen[id] = true
				database.DB.Create(&models.TimeCategoryProject{UserID: target, ProjectID: id, Category: cat})
			}
		}
	}

	sc, status, msg := buildOverviewScope(target, req.Year)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, buildOverviewSettings(sc))
}

func uniqueIDs(ids []uint) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
