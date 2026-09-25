package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

func TestTimerSegments(t *testing.T) {
	ams, err := time.LoadLocation("Europe/Amsterdam")
	require.NoError(t, err)
	at := func(day, hh, mm, ss int) time.Time { return time.Date(2026, 9, day, hh, mm, ss, 0, ams) }
	seg := func(date, start, end string, min int) timerSegment {
		return timerSegment{Date: date, StartTime: start, EndTime: end, Minutes: min}
	}

	cases := []struct {
		name       string
		start, end time.Time
		want       []timerSegment
	}{
		{"short run rounds up to 15, seconds dropped from the start",
			at(25, 9, 3, 40), at(25, 9, 10, 0), []timerSegment{seg("2026-09-25", "09:03", "09:18", 15)}},
		{"exact multiple stays",
			at(25, 9, 0, 0), at(25, 9, 30, 0), []timerSegment{seg("2026-09-25", "09:00", "09:30", 30)}},
		{"one second over rounds up",
			at(25, 9, 0, 0), at(25, 9, 30, 1), []timerSegment{seg("2026-09-25", "09:00", "09:45", 45)}},
		{"zero duration books one unit",
			at(25, 9, 0, 0), at(25, 9, 0, 0), []timerSegment{seg("2026-09-25", "09:00", "09:15", 15)}},
		{"split at midnight",
			at(25, 23, 50, 0), at(26, 0, 20, 0), []timerSegment{
				seg("2026-09-25", "23:50", "00:00", 10),
				seg("2026-09-26", "00:00", "00:20", 20)}},
		{"rounding itself crosses midnight",
			at(25, 23, 40, 0), at(25, 23, 58, 0), []timerSegment{
				seg("2026-09-25", "23:40", "00:00", 20),
				seg("2026-09-26", "00:00", "00:10", 10)}},
		{"multi-day run gets a whole-day entry in between",
			at(25, 22, 0, 0), at(27, 2, 0, 0), []timerSegment{
				seg("2026-09-25", "22:00", "00:00", 120),
				seg("2026-09-26", "00:00", "00:00", 1440),
				seg("2026-09-27", "00:00", "02:00", 120)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, timerSegments(tc.start, tc.end, ams))
		})
	}

	t.Run("daylight saving: minutes follow real time, not the wall clock", func(t *testing.T) {
		// 2026-10-25 03:00 CEST becomes 02:00 CET: 01:30–03:30 on the wall is 3 real hours.
		start := time.Date(2026, 10, 25, 1, 30, 0, 0, ams)
		segs := timerSegments(start, start.Add(3*time.Hour), ams)
		require.Len(t, segs, 1)
		assert.Equal(t, 180, segs[0].Minutes)
		assert.Equal(t, "03:30", segs[0].EndTime)
	})

	t.Run("dates follow the timer's zone, not UTC", func(t *testing.T) {
		// 00:30 in Amsterdam is still the previous day in UTC.
		segs := timerSegments(time.Date(2026, 9, 26, 0, 30, 0, 0, ams), time.Date(2026, 9, 26, 1, 0, 0, 0, ams), ams)
		assert.Equal(t, "2026-09-26", segs[0].Date)
	})
}

func TestTimerEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	user := &models.User{Username: "u", Email: "u@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true, Timezone: "Europe/Amsterdam"}
	other := &models.User{Username: "o", Email: "o@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(other).Error)

	acme := &models.Customer{Name: "Acme"}
	globex := &models.Customer{Name: "Globex"}
	require.NoError(t, db.Create(acme).Error)
	require.NoError(t, db.Create(globex).Error)
	require.NoError(t, db.Create(&models.CustomerAccess{CustomerID: globex.ID, UserID: user.ID, Role: "member"}).Error)

	board := &models.Project{Name: "Website", Slug: "website", KeyPrefix: "WEB", CreatedByID: other.ID, CustomerID: &acme.ID}
	hidden := &models.Project{Name: "Secret", Slug: "secret", KeyPrefix: "SEC", CreatedByID: other.ID}
	travel := &models.Project{Name: "Travel", Slug: "travel", KeyPrefix: "TRV", CreatedByID: user.ID, TimeTrackingOnly: true}
	for _, p := range []*models.Project{board, hidden, travel} {
		require.NoError(t, db.Create(p).Error)
	}
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: board.ID, UserID: user.ID, Role: "member"}).Error)

	call := func(h gin.HandlerFunc, method, body string) (int, []byte) {
		c, w := ginTestContext(t, method, "/timer", body)
		c.Set(middleware.ContextUserID, user.ID)
		c.Set(middleware.ContextGlobalRole, "user")
		h(c)
		c.Writer.WriteHeaderNow() // a bare c.Status (the 204) is only flushed by the server
		return w.Code, w.Body.Bytes()
	}
	var entries []models.TimeEntry

	t.Run("targets list what the user may book on", func(t *testing.T) {
		code, body := call(GetTimerTargets, http.MethodGet, "")
		require.Equal(t, http.StatusOK, code)
		var out timerTargets
		require.NoError(t, json.Unmarshal(body, &out))
		names := []string{}
		for _, p := range out.Projects {
			names = append(names, p.Name)
		}
		assert.ElementsMatch(t, []string{"Website", "Travel"}, names, "no project without access")
		require.Len(t, out.Customers, 1)
		assert.Equal(t, "Globex", out.Customers[0].Name, "only customers with access")
	})

	t.Run("nothing running", func(t *testing.T) {
		code, body := call(GetTimer, http.MethodGet, "")
		require.Equal(t, http.StatusOK, code)
		assert.JSONEq(t, `{"running":false}`, string(body))
		code, _ = call(StopTimer, http.MethodPost, "")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = call(CancelTimer, http.MethodDelete, "")
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("rejections", func(t *testing.T) {
		for _, body := range []string{
			`{}`,
			fmt.Sprintf(`{"project_id":%d}`, hidden.ID),
			fmt.Sprintf(`{"customer_id":%d}`, acme.ID),
			fmt.Sprintf(`{"project_id":%d,"customer_id":%d}`, board.ID, globex.ID),
		} {
			code, _ := call(StartTimer, http.MethodPost, body)
			assert.Equal(t, http.StatusBadRequest, code, body)
		}
	})

	t.Run("a board project brings its own customer", func(t *testing.T) {
		code, body := call(StartTimer, http.MethodPost, fmt.Sprintf(`{"project_id":%d,"description":"homepage"}`, board.ID))
		require.Equal(t, http.StatusCreated, code, string(body))
		var out timerResponse
		require.NoError(t, json.Unmarshal(body, &out))
		require.NotNil(t, out.Timer)
		assert.Equal(t, acme.ID, *out.Timer.CustomerID)
		assert.Equal(t, "Europe/Amsterdam", out.Timer.TimeZone, "falls back to the user's zone")
		assert.Empty(t, out.StoppedEntries)
	})

	t.Run("starting again books the running timer first", func(t *testing.T) {
		require.NoError(t, db.Model(&models.TimeTimer{}).Where("user_id = ?", user.ID).
			Update("started_at", time.Now().Add(-50*time.Minute)).Error)
		code, body := call(StartTimer, http.MethodPost,
			fmt.Sprintf(`{"project_id":%d,"customer_id":%d,"time_zone":"UTC"}`, travel.ID, globex.ID))
		require.Equal(t, http.StatusCreated, code, string(body))
		var out timerResponse
		require.NoError(t, json.Unmarshal(body, &out))
		require.NotEmpty(t, out.StoppedEntries)
		total := 0
		for _, e := range out.StoppedEntries {
			total += e.Minutes
			assert.Equal(t, board.ID, *e.ProjectID)
			assert.Equal(t, "homepage", e.Description)
		}
		assert.Equal(t, 60, total, "50 minutes round up to 60")
		assert.Equal(t, "UTC", out.Timer.TimeZone)
		assert.Equal(t, globex.ID, *out.Timer.CustomerID)
	})

	t.Run("stop with an end time books a forgotten timer", func(t *testing.T) {
		start := time.Now().Add(-3 * time.Hour).Truncate(time.Minute)
		require.NoError(t, db.Model(&models.TimeTimer{}).Where("user_id = ?", user.ID).Update("started_at", start).Error)

		code, _ := call(StopTimer, http.MethodPost, fmt.Sprintf(`{"end":%q}`, time.Now().Add(time.Hour).Format(time.RFC3339)))
		assert.Equal(t, http.StatusBadRequest, code, "end in the future")
		code, _ = call(StopTimer, http.MethodPost, fmt.Sprintf(`{"end":%q}`, start.Add(-time.Minute).Format(time.RFC3339)))
		assert.Equal(t, http.StatusBadRequest, code, "end before start")

		code, body := call(StopTimer, http.MethodPost, fmt.Sprintf(`{"end":%q}`, start.Add(80*time.Minute).Format(time.RFC3339)))
		require.Equal(t, http.StatusOK, code, string(body))
		require.NoError(t, json.Unmarshal(body, &entries))
		total := 0
		for _, e := range entries {
			total += e.Minutes
			assert.Equal(t, travel.ID, *e.ProjectID)
			assert.Equal(t, globex.ID, *e.CustomerID)
			require.NotNil(t, e.StartTime)
		}
		assert.Equal(t, 90, total, "80 minutes round up to 90")

		code, body = call(GetTimer, http.MethodGet, "")
		require.Equal(t, http.StatusOK, code)
		assert.JSONEq(t, `{"running":false}`, string(body))
	})

	t.Run("cancel discards without booking", func(t *testing.T) {
		code, _ := call(StartTimer, http.MethodPost, fmt.Sprintf(`{"project_id":%d}`, board.ID))
		require.Equal(t, http.StatusCreated, code)
		var before int64
		db.Model(&models.TimeEntry{}).Count(&before)
		code, _ = call(CancelTimer, http.MethodDelete, "")
		assert.Equal(t, http.StatusNoContent, code)
		var after int64
		db.Model(&models.TimeEntry{}).Count(&after)
		assert.Equal(t, before, after)
	})
}
