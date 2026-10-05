package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

func withTestDB(t *testing.T) {
	t.Helper()
	db, cleanup := testutil.SetupTestDB()
	prev := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = prev
		cleanup()
	})
}

// officeHours is Mon–Fri 09:00–17:00 Europe/Amsterdam with 2026-12-25 off.
func officeHours(t *testing.T) *WorkCalendar {
	t.Helper()
	w := NewWorkCalendar(&models.BusinessCalendar{
		Timezone: "Europe/Amsterdam", WorkDays: "1,2,3,4,5", StartTime: "09:00", EndTime: "17:00",
		Holidays: []models.BusinessCalendarHoliday{{Date: "2026-12-25"}},
	})
	require.NotNil(t, w)
	return w
}

func ams(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Amsterdam")
	require.NoError(t, err)
	v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	require.NoError(t, err)
	return v
}

func TestAddBusinessMinutes(t *testing.T) {
	w := officeHours(t)
	cases := []struct {
		name, from string
		minutes    int
		want       string
	}{
		{"within the same day", "2026-10-05 10:00", 120, "2026-10-05 12:00"},
		{"exactly to closing time", "2026-10-05 15:00", 120, "2026-10-05 17:00"},
		{"rolls over to next morning", "2026-10-05 16:00", 120, "2026-10-06 10:00"},
		{"before opening starts at 09:00", "2026-10-05 06:30", 30, "2026-10-05 09:30"},
		{"friday evening skips the weekend", "2026-10-09 16:30", 60, "2026-10-12 09:30"},
		{"saturday start counts from monday", "2026-10-10 11:00", 60, "2026-10-12 10:00"},
		{"holiday is skipped", "2026-12-24 16:00", 120, "2026-12-28 10:00"},
		{"multiple days", "2026-10-05 09:00", 3 * 8 * 60, "2026-10-07 17:00"},
		{"across the DST change", "2026-10-23 16:00", 120, "2026-10-26 10:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AddBusinessMinutes(w, ams(t, tc.from), tc.minutes)
			assert.True(t, got.Equal(ams(t, tc.want)), "got %s", got.In(ams(t, tc.want).Location()))
		})
	}

	t.Run("nil calendar is wall-clock time", func(t *testing.T) {
		from := ams(t, "2026-10-10 11:00")
		assert.True(t, AddBusinessMinutes(nil, from, 90).Equal(from.Add(90*time.Minute)))
	})
	t.Run("zero minutes returns the start", func(t *testing.T) {
		from := ams(t, "2026-10-10 11:00")
		assert.True(t, AddBusinessMinutes(w, from, 0).Equal(from))
	})
}

func TestBusinessMinutesBetween(t *testing.T) {
	w := officeHours(t)
	assert.Equal(t, 120, BusinessMinutesBetween(w, ams(t, "2026-10-05 10:00"), ams(t, "2026-10-05 12:00")))
	assert.Equal(t, 120, BusinessMinutesBetween(w, ams(t, "2026-10-05 16:00"), ams(t, "2026-10-06 10:00")))
	assert.Equal(t, 0, BusinessMinutesBetween(w, ams(t, "2026-10-10 08:00"), ams(t, "2026-10-11 20:00")), "weekend")
	assert.Equal(t, 8*60, BusinessMinutesBetween(w, ams(t, "2026-10-09 00:00"), ams(t, "2026-10-12 00:00")))
	assert.Equal(t, 0, BusinessMinutesBetween(w, ams(t, "2026-10-06 10:00"), ams(t, "2026-10-05 10:00")), "reversed range")
	assert.Equal(t, 90, BusinessMinutesBetween(nil, ams(t, "2026-10-10 10:00"), ams(t, "2026-10-10 11:30")))

	// Adding N minutes and measuring back must round-trip.
	from := ams(t, "2026-10-08 14:17")
	to := AddBusinessMinutes(w, from, 1000)
	assert.Equal(t, 1000, BusinessMinutesBetween(w, from, to))
}

func TestNewWorkCalendarRejectsUnusableCalendars(t *testing.T) {
	assert.Nil(t, NewWorkCalendar(&models.BusinessCalendar{WorkDays: "", StartTime: "09:00", EndTime: "17:00"}))
	assert.Nil(t, NewWorkCalendar(&models.BusinessCalendar{WorkDays: "1", StartTime: "17:00", EndTime: "09:00"}))
	assert.Nil(t, NewWorkCalendar(&models.BusinessCalendar{WorkDays: "1", StartTime: "9:00", EndTime: "17:00"}))
	assert.NotNil(t, NewWorkCalendar(&models.BusinessCalendar{WorkDays: "1", StartTime: "00:00", EndTime: "00:00"}), "00:00 end = midnight")
}

func TestMatchSlaPolicyPrefersMostSpecific(t *testing.T) {
	withTestDB(t)
	cust := models.Customer{Name: "Acme"}
	require.NoError(t, database.DB.Create(&cust).Error)
	parent := models.TicketCategory{Name: "Network", IsActive: true}
	require.NoError(t, database.DB.Create(&parent).Error)
	child := models.TicketCategory{Name: "VPN", ParentID: &parent.ID, IsActive: true}
	require.NoError(t, database.DB.Create(&child).Error)

	catchAll := models.SlaPolicy{Name: "Default", IsActive: true}
	high := models.SlaPolicy{Name: "High", PriorityFilter: "high,critical", IsActive: true}
	acmeHigh := models.SlaPolicy{Name: "Acme high", PriorityFilter: "high", CustomerID: &cust.ID, IsActive: true}
	network := models.SlaPolicy{Name: "Network incidents", CategoryID: &parent.ID, TypeFilter: "incident", IsActive: true}
	for _, p := range []*models.SlaPolicy{&catchAll, &high, &acmeHigh, &network} {
		require.NoError(t, database.DB.Create(p).Error)
	}
	inactive := models.SlaPolicy{Name: "Inactive", PriorityFilter: "low", CustomerID: &cust.ID}
	require.NoError(t, database.DB.Create(&inactive).Error)
	require.NoError(t, database.DB.Model(&inactive).Update("is_active", false).Error)

	other := uint(999)
	match := func(tk models.Ticket) string {
		p := MatchSlaPolicy(&tk)
		if p == nil {
			return ""
		}
		return p.Name
	}
	assert.Equal(t, "Default", match(models.Ticket{Priority: "low", Type: "incident"}))
	assert.Equal(t, "High", match(models.Ticket{Priority: "critical", Type: "incident"}))
	assert.Equal(t, "High", match(models.Ticket{Priority: "high", Type: "incident", CustomerID: &other}))
	assert.Equal(t, "Acme high", match(models.Ticket{Priority: "high", Type: "incident", CustomerID: &cust.ID}))
	assert.Equal(t, "Network incidents", match(models.Ticket{Priority: "low", Type: "incident", CategoryID: &child.ID}), "subcategory matches parent policy")
	assert.Equal(t, "Default", match(models.Ticket{Priority: "low", Type: "service_request", CategoryID: &child.ID}))
	assert.Equal(t, "Default", match(models.Ticket{Priority: "low", CustomerID: &cust.ID}), "inactive policy is ignored")
}

func TestEvaluateSlaBreaches(t *testing.T) {
	created := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	deadline := created.Add(4 * time.Hour)
	at := func(h int) *time.Time { v := created.Add(time.Duration(h) * time.Hour); return &v }

	base := func() models.Ticket {
		return models.Ticket{Status: "open", CreatedAt: created, SlaResponseDeadline: &deadline, SlaResolutionDeadline: &deadline}
	}

	t.Run("open and overdue", func(t *testing.T) {
		tk := base()
		resp, res := EvaluateSlaBreaches(&tk, *at(5))
		assert.True(t, resp)
		assert.True(t, res)
	})
	t.Run("open within deadline", func(t *testing.T) {
		tk := base()
		resp, res := EvaluateSlaBreaches(&tk, *at(2))
		assert.False(t, resp)
		assert.False(t, res)
	})
	t.Run("closed late without anyone viewing it is still breached", func(t *testing.T) {
		tk := base()
		tk.Status, tk.ResolvedAt, tk.FirstResponseAt = "closed", at(6), at(1)
		resp, res := EvaluateSlaBreaches(&tk, *at(48))
		assert.False(t, resp)
		assert.True(t, res)
	})
	t.Run("resolved in time stays met", func(t *testing.T) {
		tk := base()
		tk.Status, tk.ResolvedAt, tk.FirstResponseAt = "resolved", at(3), at(1)
		resp, res := EvaluateSlaBreaches(&tk, *at(48))
		assert.False(t, resp)
		assert.False(t, res)
	})
	t.Run("paused before the deadline is not breached", func(t *testing.T) {
		tk := base()
		tk.Status, tk.SlaPausedAt, tk.FirstResponseAt = "pending", at(2), at(1)
		_, res := EvaluateSlaBreaches(&tk, *at(10))
		assert.False(t, res)
	})
	t.Run("legacy resolved ticket keeps its stored flags", func(t *testing.T) {
		tk := base()
		tk.Status, tk.SlaResolutionBreached = "closed", true
		_, res := EvaluateSlaBreaches(&tk, *at(1))
		assert.True(t, res)
	})
}

func TestTransitionTicketStatus(t *testing.T) {
	withTestDB(t)
	policy := models.SlaPolicy{Name: "P", ResponseTimeMinutes: 60, ResolutionTimeMinutes: 240, IsActive: true}
	require.NoError(t, database.DB.Create(&policy).Error)
	created := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	tk := models.Ticket{Status: "open", Priority: "medium", Type: "incident", CreatedAt: created}
	ApplySla(&tk)
	require.NotNil(t, tk.SlaResolutionDeadline)
	assert.True(t, tk.SlaResolutionDeadline.Equal(created.Add(4*time.Hour)))

	// Pending for 90 minutes pushes the resolution deadline back by 90 minutes.
	TransitionTicketStatus(&tk, "pending", created.Add(time.Hour), TransitionOptions{})
	require.NotNil(t, tk.SlaPausedAt)
	u := TransitionTicketStatus(&tk, "in_progress", created.Add(150*time.Minute), TransitionOptions{})
	assert.Nil(t, tk.SlaPausedAt)
	assert.Equal(t, 90, tk.SlaPausedMinutes)
	assert.True(t, tk.SlaResolutionDeadline.Equal(created.Add(4*time.Hour+90*time.Minute)))
	assert.Contains(t, u, "sla_resolution_deadline")
	assert.True(t, tk.SlaResponseDeadline.Equal(created.Add(time.Hour)), "the response clock never pauses")

	// Resolving stamps resolved_at; closing stamps closed_at and keeps resolved_at.
	resolvedAt := created.Add(5 * time.Hour)
	TransitionTicketStatus(&tk, "resolved", resolvedAt, TransitionOptions{})
	require.NotNil(t, tk.ResolvedAt)
	assert.True(t, tk.ResolvedAt.Equal(resolvedAt))
	assert.Nil(t, tk.ClosedAt)
	TransitionTicketStatus(&tk, "closed", created.Add(50*time.Hour), TransitionOptions{})
	assert.True(t, tk.ResolvedAt.Equal(resolvedAt))
	require.NotNil(t, tk.ClosedAt)

	// Reopening clears both and counts the reopen.
	TransitionTicketStatus(&tk, "open", created.Add(60*time.Hour), TransitionOptions{CountReopen: true})
	assert.Nil(t, tk.ResolvedAt)
	assert.Nil(t, tk.ClosedAt)
	assert.Equal(t, 1, tk.ReopenCount)

	// An administrative flip (spam/unspam) doesn't count as a reopen.
	TransitionTicketStatus(&tk, "closed", created.Add(61*time.Hour), TransitionOptions{})
	TransitionTicketStatus(&tk, "open", created.Add(62*time.Hour), TransitionOptions{})
	assert.Equal(t, 1, tk.ReopenCount)
}

func TestApplySlaCountsFromCreation(t *testing.T) {
	withTestDB(t)
	low := models.SlaPolicy{Name: "Low", PriorityFilter: "low", ResolutionTimeMinutes: 24 * 60, IsActive: true}
	high := models.SlaPolicy{Name: "High", PriorityFilter: "high", ResolutionTimeMinutes: 4 * 60, IsActive: true}
	require.NoError(t, database.DB.Create(&low).Error)
	require.NoError(t, database.DB.Create(&high).Error)

	created := time.Now().Add(-10 * time.Hour)
	tk := models.Ticket{Priority: "low", Type: "incident", Status: "open", CreatedAt: created}
	ApplySla(&tk)
	tk.Priority = "high"
	ApplySla(&tk)
	require.NotNil(t, tk.SlaResolutionDeadline)
	assert.True(t, tk.SlaResolutionDeadline.Equal(created.Add(4*time.Hour)), "raising priority must not hand out a fresh deadline")
	_, res := EvaluateSlaBreaches(&tk, time.Now())
	assert.True(t, res)
}

func TestDerivePriority(t *testing.T) {
	m := DefaultPriorityMatrix
	p, ok := DerivePriority(m, "high", "high")
	assert.True(t, ok)
	assert.Equal(t, "critical", p)
	p, _ = DerivePriority(m, "low", "medium")
	assert.Equal(t, "low", p)
	_, ok = DerivePriority(m, "", "high")
	assert.False(t, ok)
	_, ok = DerivePriority(m, "huge", "high")
	assert.False(t, ok)
}

func TestHelpdeskSettingsDefaultsAndOverrides(t *testing.T) {
	withTestDB(t)
	s := LoadHelpdeskSettings()
	assert.Equal(t, 5, s.ResolvedAutoCloseDays)
	assert.False(t, s.RequireResolutionCode)
	assert.Equal(t, "critical", s.PriorityMatrix["high:high"])

	database.DB.Create(&models.SystemSetting{Key: SettingHelpdeskPriorityMatrix, Value: `{"high:high":"high","bogus":"low","low:low":"nope"}`})
	database.DB.Create(&models.SystemSetting{Key: SettingHelpdeskResolvedAutoClose, Value: "0"})
	s = LoadHelpdeskSettings()
	assert.Equal(t, "high", s.PriorityMatrix["high:high"])
	assert.Equal(t, "low", s.PriorityMatrix["low:low"], "invalid values fall back to the default")
	assert.NotContains(t, s.PriorityMatrix, "bogus")
	assert.Equal(t, 0, s.ResolvedAutoCloseDays)
}

func TestAutoCloseResolvedTickets(t *testing.T) {
	withTestDB(t)
	old := time.Now().AddDate(0, 0, -6)
	recent := time.Now().AddDate(0, 0, -1)
	a := models.Ticket{Title: "old", Status: "resolved", ResolvedAt: &old}
	b := models.Ticket{Title: "recent", Status: "resolved", ResolvedAt: &recent}
	require.NoError(t, database.DB.Create(&a).Error)
	require.NoError(t, database.DB.Create(&b).Error)

	AutoCloseTickets()
	database.DB.First(&a, a.ID)
	database.DB.First(&b, b.ID)
	assert.Equal(t, "closed", a.Status)
	require.NotNil(t, a.ClosedAt)
	assert.True(t, a.ResolvedAt.Equal(old), "closing keeps the original resolution time")
	assert.Equal(t, "resolved", b.Status)
}

func TestBackfillTicketTimestamps(t *testing.T) {
	withTestDB(t)
	tk := models.Ticket{Title: "legacy", Status: "closed"}
	require.NoError(t, database.DB.Create(&tk).Error)
	resolvedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	database.DB.Create(&models.TicketHistory{TicketID: tk.ID, EventType: "status_changed", Detail: "open", CreatedAt: resolvedAt.Add(-time.Hour)})
	database.DB.Create(&models.TicketHistory{TicketID: tk.ID, EventType: "status_changed", Detail: "pending_close", CreatedAt: resolvedAt})
	database.DB.Create(&models.TicketHistory{TicketID: tk.ID, EventType: "status_changed", Detail: "closed", CreatedAt: resolvedAt.Add(48 * time.Hour)})

	BackfillTicketTimestamps()
	database.DB.First(&tk, tk.ID)
	require.NotNil(t, tk.ResolvedAt)
	require.NotNil(t, tk.ClosedAt)
	assert.True(t, tk.ResolvedAt.Equal(resolvedAt))
	assert.True(t, tk.ClosedAt.Equal(resolvedAt.Add(48*time.Hour)))
}

func TestProcessSlaTicketsWarnsAndEscalatesOnce(t *testing.T) {
	withTestDB(t)
	agent := models.User{Username: "agent", Email: "a@example.com", PasswordHash: "x", GlobalRole: "user"}
	lead := models.User{Username: "lead", Email: "l@example.com", PasswordHash: "x", GlobalRole: "user"}
	require.NoError(t, database.DB.Create(&agent).Error)
	require.NoError(t, database.DB.Create(&lead).Error)
	group := models.UserGroup{Name: "2nd line"}
	require.NoError(t, database.DB.Create(&group).Error)

	policy := models.SlaPolicy{Name: "P", ResolutionTimeMinutes: 100, WarningPercent: 75,
		EscalationGroupID: &group.ID, EscalationUserID: &lead.ID, EscalationReassign: true, IsActive: true}
	require.NoError(t, database.DB.Create(&policy).Error)

	created := time.Now().Add(-80 * time.Minute)
	tk := models.Ticket{Title: "t", Status: "open", Priority: "medium", Type: "incident", AssignedToID: &agent.ID, CreatedAt: created}
	ApplySla(&tk)
	require.NoError(t, database.DB.Create(&tk).Error)

	// 80% used → warning, no escalation.
	ProcessSlaTickets(time.Now())
	database.DB.First(&tk, tk.ID)
	assert.NotNil(t, tk.SlaWarnedAt)
	assert.Nil(t, tk.SlaEscalatedAt)

	// Past the deadline → escalated and reassigned exactly once.
	later := created.Add(101 * time.Minute)
	ProcessSlaTickets(later)
	ProcessSlaTickets(later.Add(time.Minute))
	database.DB.First(&tk, tk.ID)
	require.NotNil(t, tk.SlaEscalatedAt)
	assert.True(t, tk.SlaResolutionBreached)
	require.NotNil(t, tk.GroupID)
	assert.Equal(t, group.ID, *tk.GroupID)
	require.NotNil(t, tk.AssignedToID)
	assert.Equal(t, lead.ID, *tk.AssignedToID)

	var escalations int64
	database.DB.Model(&models.TicketHistory{}).Where("ticket_id = ? AND event_type = ?", tk.ID, "sla_escalated").Count(&escalations)
	assert.Equal(t, int64(1), escalations)
}
