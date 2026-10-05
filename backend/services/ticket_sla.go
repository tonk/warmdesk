package services

import (
	"strconv"
	"strings"
	"time"

	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
)

// WorkCalendar is the evaluated form of a models.BusinessCalendar: the working
// window per weekday plus a holiday set, all in one time zone. A nil
// *WorkCalendar means "24×7" — every SLA helper treats it as plain wall-clock
// time, which is how SLA policies without a calendar have always behaved.
type WorkCalendar struct {
	loc      *time.Location
	days     [7]bool
	startMin int
	endMin   int
	holidays map[string]bool
}

// maxCalendarDays bounds the day-by-day walks below so a calendar with no
// usable working time can never loop forever.
const maxCalendarDays = 3660

// NewWorkCalendar converts a stored calendar into a WorkCalendar. It returns
// nil (24×7) when the calendar has no usable working window, so a
// misconfigured calendar degrades to wall-clock deadlines instead of
// deadlines that never arrive.
func NewWorkCalendar(cal *models.BusinessCalendar) *WorkCalendar {
	if cal == nil {
		return nil
	}
	loc, err := time.LoadLocation(cal.Timezone)
	if err != nil || cal.Timezone == "" {
		loc = time.UTC
	}
	w := &WorkCalendar{loc: loc, holidays: map[string]bool{}}
	for _, d := range strings.Split(cal.WorkDays, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(d))
		if err == nil && n >= 0 && n <= 6 {
			w.days[n] = true
		}
	}
	var ok bool
	if w.startMin, ok = parseHHMM(cal.StartTime); !ok {
		return nil
	}
	if w.endMin, ok = parseHHMM(cal.EndTime); !ok {
		return nil
	}
	// "24:00" isn't valid HH:MM; "00:00" as an end time means end of day.
	if w.endMin == 0 {
		w.endMin = 24 * 60
	}
	if w.endMin <= w.startMin {
		return nil
	}
	anyDay := false
	for _, d := range w.days {
		anyDay = anyDay || d
	}
	if !anyDay {
		return nil
	}
	for _, h := range cal.Holidays {
		w.holidays[h.Date] = true
	}
	return w
}

// ValidCalendar reports whether a calendar definition yields usable working
// time — the admin handler rejects anything that would silently fall back to 24×7.
func ValidCalendar(cal *models.BusinessCalendar) bool {
	return NewWorkCalendar(cal) != nil
}

func parseHHMM(s string) (int, bool) {
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

// window returns the working interval of the calendar day containing day
// (which must already be in w.loc), or ok=false on non-working days.
func (w *WorkCalendar) window(day time.Time) (start, end time.Time, ok bool) {
	if !w.days[day.Weekday()] || w.holidays[day.Format("2006-01-02")] {
		return time.Time{}, time.Time{}, false
	}
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, w.loc)
	start = midnight.Add(time.Duration(w.startMin) * time.Minute)
	end = midnight.Add(time.Duration(w.endMin) * time.Minute)
	return start, end, true
}

// AddBusinessMinutes returns the moment that lies the given number of working
// minutes after from.
func AddBusinessMinutes(w *WorkCalendar, from time.Time, minutes int) time.Time {
	if minutes <= 0 {
		return from
	}
	if w == nil {
		return from.Add(time.Duration(minutes) * time.Minute)
	}
	remaining := time.Duration(minutes) * time.Minute
	t := from.In(w.loc)
	for i := 0; i < maxCalendarDays; i++ {
		midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, w.loc)
		if ws, we, ok := w.window(midnight); ok {
			if t.Before(ws) {
				t = ws
			}
			if t.Before(we) {
				avail := we.Sub(t)
				if remaining <= avail {
					return t.Add(remaining)
				}
				remaining -= avail
			}
		}
		t = midnight.AddDate(0, 0, 1)
	}
	return t
}

// BusinessMinutesBetween counts the working minutes in [a, b).
func BusinessMinutesBetween(w *WorkCalendar, a, b time.Time) int {
	if !b.After(a) {
		return 0
	}
	if w == nil {
		return int(b.Sub(a) / time.Minute)
	}
	var total time.Duration
	t := a.In(w.loc)
	for i := 0; i < maxCalendarDays && t.Before(b); i++ {
		midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, w.loc)
		if ws, we, ok := w.window(midnight); ok {
			lo, hi := ws, we
			if t.After(lo) {
				lo = t
			}
			if b.Before(hi) {
				hi = b
			}
			if hi.After(lo) {
				total += hi.Sub(lo)
			}
		}
		t = midnight.AddDate(0, 0, 1)
	}
	return int(total / time.Minute)
}

// LoadWorkCalendar loads a business calendar by ID; nil ID or a missing row
// yields nil (24×7).
func LoadWorkCalendar(id *uint) *WorkCalendar {
	if id == nil {
		return nil
	}
	var cal models.BusinessCalendar
	if err := database.DB.Preload("Holidays").First(&cal, *id).Error; err != nil {
		return nil
	}
	return NewWorkCalendar(&cal)
}

func policyCalendar(policy *models.SlaPolicy) *WorkCalendar {
	if policy == nil {
		return nil
	}
	return LoadWorkCalendar(policy.CalendarID)
}

func csvContains(list, value string) bool {
	for _, v := range strings.Split(list, ",") {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	return false
}

// policySpecificity returns -1 when the policy doesn't apply to the ticket,
// otherwise the number of filters it set — more filters means more specific.
func policySpecificity(p *models.SlaPolicy, t *models.Ticket) int {
	score := 0
	if p.PriorityFilter != "" {
		if !csvContains(p.PriorityFilter, t.Priority) {
			return -1
		}
		score++
	}
	if p.TypeFilter != "" {
		if !csvContains(p.TypeFilter, t.Type) {
			return -1
		}
		score++
	}
	if p.CustomerID != nil {
		if t.CustomerID == nil || *t.CustomerID != *p.CustomerID {
			return -1
		}
		score++
	}
	if p.CategoryID != nil {
		if t.CategoryID == nil || !categoryWithin(*t.CategoryID, *p.CategoryID) {
			return -1
		}
		score++
	}
	return score
}

// categoryWithin reports whether category id is want itself or one of its
// subcategories, so a policy on a parent category covers its children.
func categoryWithin(id, want uint) bool {
	if id == want {
		return true
	}
	var cat models.TicketCategory
	if err := database.DB.Select("id", "parent_id").First(&cat, id).Error; err != nil {
		return false
	}
	return cat.ParentID != nil && *cat.ParentID == want
}

// MatchSlaPolicy picks the active SLA policy for a ticket: every non-empty
// filter (priority, type, customer, category) must match, and the policy with
// the most filters wins; ties go to the lowest ID, so a single catch-all
// policy keeps behaving the way the priority-only matcher always did.
func MatchSlaPolicy(t *models.Ticket) *models.SlaPolicy {
	var policies []models.SlaPolicy
	database.DB.Where("is_active = ?", true).Order("id asc").Find(&policies)
	var best *models.SlaPolicy
	bestScore := -1
	for i := range policies {
		if s := policySpecificity(&policies[i], t); s > bestScore {
			best, bestScore = &policies[i], s
		}
	}
	return best
}

// ComputeSlaDeadlines computes a ticket's deadlines from its creation time.
// pausedMinutes (business minutes spent in "pending") push only the
// resolution deadline back; the response clock never pauses.
func ComputeSlaDeadlines(policy *models.SlaPolicy, cal *WorkCalendar, createdAt time.Time, pausedMinutes int) (response, resolution *time.Time) {
	if policy.ResponseTimeMinutes > 0 {
		d := AddBusinessMinutes(cal, createdAt, policy.ResponseTimeMinutes)
		response = &d
	}
	if policy.ResolutionTimeMinutes > 0 {
		d := AddBusinessMinutes(cal, createdAt, policy.ResolutionTimeMinutes+pausedMinutes)
		resolution = &d
	}
	return
}

// ApplySla (re)matches the ticket's SLA policy and recomputes its deadlines,
// always counted from created_at — changing priority must not hand out a
// fresh deadline. It mutates t and returns the column updates to persist.
// Breach flags are re-evaluated by the caller via EvaluateSlaBreaches.
func ApplySla(t *models.Ticket) map[string]any {
	created := t.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	updates := map[string]any{}
	policy := MatchSlaPolicy(t)
	oldResp, oldRes := t.SlaResponseDeadline, t.SlaResolutionDeadline
	if policy == nil {
		t.SlaPolicyID, t.SlaPolicy = nil, nil
		t.SlaResponseDeadline, t.SlaResolutionDeadline = nil, nil
	} else {
		resp, res := ComputeSlaDeadlines(policy, policyCalendar(policy), created, t.SlaPausedMinutes)
		t.SlaPolicyID, t.SlaPolicy = &policy.ID, policy
		t.SlaResponseDeadline, t.SlaResolutionDeadline = resp, res
	}
	updates["sla_policy_id"] = t.SlaPolicyID
	updates["sla_response_deadline"] = t.SlaResponseDeadline
	updates["sla_resolution_deadline"] = t.SlaResolutionDeadline
	// A moved deadline earns a fresh warning.
	if !sameTime(oldResp, t.SlaResponseDeadline) || !sameTime(oldRes, t.SlaResolutionDeadline) {
		t.SlaWarnedAt = nil
		updates["sla_warned_at"] = nil
	}
	return updates
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// EvaluateSlaBreaches derives the breach flags from timestamps alone, so the
// result doesn't depend on whether anyone happened to view the ticket between
// its deadline and its resolution.
func EvaluateSlaBreaches(t *models.Ticket, now time.Time) (response, resolution bool) {
	if d := t.SlaResponseDeadline; d != nil {
		switch {
		case t.FirstResponseAt != nil:
			response = t.FirstResponseAt.After(*d)
		case t.ResolvedAt != nil:
			// Resolved without a public reply: the resolution was the response.
			response = t.ResolvedAt.After(*d)
		case IsResolvedStatus(t.Status):
			// Legacy ticket resolved before resolved_at existed: keep the stored flag.
			response = t.SlaResponseBreached
		default:
			response = now.After(*d)
		}
	}
	if d := t.SlaResolutionDeadline; d != nil {
		switch {
		case t.ResolvedAt != nil:
			resolution = t.ResolvedAt.After(*d)
		case IsResolvedStatus(t.Status):
			resolution = t.SlaResolutionBreached
		case t.SlaPausedAt != nil:
			// Paused clock: only breached if it was already overdue when the pause began.
			resolution = t.SlaPausedAt.After(*d)
		default:
			resolution = now.After(*d)
		}
	}
	return
}

// RefreshSlaBreaches re-evaluates and persists the breach flags when they changed.
func RefreshSlaBreaches(t *models.Ticket, now time.Time) {
	resp, res := EvaluateSlaBreaches(t, now)
	if resp == t.SlaResponseBreached && res == t.SlaResolutionBreached {
		return
	}
	t.SlaResponseBreached, t.SlaResolutionBreached = resp, res
	database.DB.Model(&models.Ticket{}).Where("id = ?", t.ID).Updates(map[string]any{
		"sla_response_breached":   resp,
		"sla_resolution_breached": res,
	})
}
