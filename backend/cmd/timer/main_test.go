package main

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/handlers"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

func TestMatchTarget(t *testing.T) {
	acme := uint(1)
	list := []target{
		{ID: 1, Name: "Website", CustomerID: &acme, CustomerName: "Acme"},
		{ID: 2, Name: "Website", CustomerName: "Globex"},
		{ID: 3, Name: "Travel", TimeTrackingOnly: true},
		{ID: 4, Name: "Support desk"},
	}
	got, err := matchTarget(list, "travel", "project")
	require.NoError(t, err)
	assert.EqualValues(t, 3, got.ID, "case-insensitive exact match")

	got, err = matchTarget(list, "supp", "project")
	require.NoError(t, err)
	assert.EqualValues(t, 4, got.ID, "unique partial match")

	_, err = matchTarget(list, "website", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Website (Acme), Website (Globex)")
	assert.Contains(t, err.Error(), "-c CUSTOMER")

	_, err = matchTarget(list, "nope", "project")
	assert.ErrorContains(t, err, "warmdesk-timer projects")

	// An exact name beats longer names containing it.
	got, err = matchTarget([]target{{ID: 1, Name: "Ops"}, {ID: 2, Name: "DevOps"}}, "ops", "project")
	require.NoError(t, err)
	assert.EqualValues(t, 1, got.ID)
}

func TestParseAt(t *testing.T) {
	now := time.Date(2026, 9, 25, 14, 0, 0, 0, time.Local)
	end, err := parseAt("12:30", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 25, 12, 30, 0, 0, time.Local), end)

	end, err = parseAt("17:30", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 24, 17, 30, 0, 0, time.Local), end, "a later time means yesterday")

	_, err = parseAt("5pm", now)
	assert.Error(t, err)
}

func TestLoadConfigPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "timer.yaml")
	require.NoError(t, os.WriteFile(path, []byte("url: https://file.example/\napi_key: filekey\n"), 0o600))

	t.Setenv("WARMDESK_URL", "")
	t.Setenv("WARMDESK_API_KEY", "")
	cfg, err := loadConfig(path, "", "", false)
	require.NoError(t, err)
	assert.Equal(t, config{URL: "https://file.example", APIKey: "filekey"}, cfg)

	t.Setenv("WARMDESK_API_KEY", "envkey")
	cfg, _ = loadConfig(path, "", "", false)
	assert.Equal(t, "envkey", cfg.APIKey, "environment beats the file")

	cfg, _ = loadConfig(path, "https://flag.example", "flagkey", false)
	assert.Equal(t, config{URL: "https://flag.example", APIKey: "flagkey"}, cfg, "flags beat everything")

	_, err = loadConfig(filepath.Join(dir, "missing.yaml"), "", "", true)
	assert.Error(t, err, "an explicit --config must exist")

	t.Setenv("WARMDESK_API_KEY", "")
	_, err = loadConfig(filepath.Join(dir, "missing.yaml"), "https://x", "", false)
	assert.ErrorContains(t, err, "API key are required")
}

// TestEndToEnd drives the CLI against the real timer handlers.
func TestEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	user := &models.User{Username: "u", Email: "u@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(user).Error)
	acme := &models.Customer{Name: "Acme"}
	require.NoError(t, db.Create(acme).Error)
	board := &models.Project{Name: "Website", Slug: "website", KeyPrefix: "WEB", CreatedByID: user.ID, CustomerID: &acme.ID}
	travel := &models.Project{Name: "Travel", Slug: "travel", KeyPrefix: "TRV", CreatedByID: user.ID, TimeTrackingOnly: true}
	require.NoError(t, db.Create(board).Error)
	require.NoError(t, db.Create(travel).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: board.ID, UserID: user.ID, Role: "member"}).Error)
	// Combining Acme with a time-tracking project needs access to Acme
	// itself, as in the web interface's customer list.
	require.NoError(t, db.Create(&models.CustomerAccess{CustomerID: acme.ID, UserID: user.ID, Role: "member"}).Error)

	r := gin.New()
	api := r.Group("/api/v1", func(c *gin.Context) {
		if c.GetHeader("X-API-Key") != "secret" {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid API key"})
			return
		}
		c.Set(middleware.ContextUserID, user.ID)
		c.Set(middleware.ContextGlobalRole, "user")
	})
	api.GET("/timer", handlers.GetTimer)
	api.GET("/timer/targets", handlers.GetTimerTargets)
	api.POST("/timer/start", handlers.StartTimer)
	api.POST("/timer/stop", handlers.StopTimer)
	api.DELETE("/timer", handlers.CancelTimer)
	srv := httptest.NewServer(r)
	defer srv.Close()

	t.Setenv("WARMDESK_URL", "")
	t.Setenv("WARMDESK_API_KEY", "")
	cli := func(args ...string) (string, error) {
		var out bytes.Buffer
		all := append([]string{"--url", srv.URL, "--key", "secret"}, args...)
		err := run(all, &out, time.Now())
		return out.String(), err
	}

	out, err := cli("status")
	require.NoError(t, err)
	assert.Equal(t, "No timer running.\n", out)

	out, err = cli("start", "web", "-m", "homepage")
	require.NoError(t, err)
	assert.Contains(t, out, "Started Website (Acme) — homepage at ")

	out, err = cli("status")
	require.NoError(t, err)
	assert.Contains(t, out, "Running: Website (Acme) — homepage — since ")

	// Pretend it ran for 20 minutes, then switch: the first timer is booked.
	require.NoError(t, db.Model(&models.TimeTimer{}).Where("user_id = ?", user.ID).
		Update("started_at", time.Now().Add(-20*time.Minute)).Error)
	out, err = cli("start", "-c", "acme", "Travel")
	require.NoError(t, err)
	assert.Contains(t, out, "Booked 30m on Website (Acme) — homepage:")
	assert.Contains(t, out, "Started Travel (Acme) at ")

	start := time.Now().Add(-2 * time.Hour)
	require.NoError(t, db.Model(&models.TimeTimer{}).Where("user_id = ?", user.ID).Update("started_at", start).Error)
	out, err = cli("stop", "--at", start.Add(61*time.Minute).Local().Format("15:04"))
	require.NoError(t, err)
	assert.Contains(t, out, "Booked 1h 15m on Travel (Acme):")

	var entries []models.TimeEntry
	db.Order("id").Find(&entries)
	require.GreaterOrEqual(t, len(entries), 2)
	sum := 0
	for _, e := range entries {
		sum += e.Minutes
	}
	assert.Equal(t, 30+75, sum)

	_, err = cli("stop")
	assert.ErrorContains(t, err, "no timer running")

	_, err = cli("start")
	var ue usageError
	assert.ErrorAs(t, err, &ue)

	_, err = cli("start", "Website", "-c", "globex")
	assert.ErrorContains(t, err, "no customer matches")

	out, err = cli("projects")
	require.NoError(t, err)
	assert.True(t, strings.Contains(out, "Website  (Acme)") && strings.Contains(out, "Travel  (time tracking"), out)

	var bad bytes.Buffer
	err = run([]string{"--url", srv.URL, "--key", "wrong", "status"}, &bad, time.Now())
	assert.ErrorContains(t, err, "check the API key")
}
