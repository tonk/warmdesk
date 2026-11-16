package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

func TestIsoWeekMonday(t *testing.T) {
	cases := map[string]string{
		"2026-11-16": "2026-11-16", // Monday
		"2026-11-19": "2026-11-16", // Thursday
		"2026-11-22": "2026-11-16", // Sunday belongs to the week before it
		"2027-01-01": "2026-12-28", // across a year boundary
	}
	for in, want := range cases {
		d, _ := time.Parse("2006-01-02", in)
		require.Equal(t, want, isoWeekMonday(d).Format("2006-01-02"), in)
	}
}

// Weekly contract hours count every user's entries on the contract within
// one ISO week; holidays, other weeks and other contracts don't count. The
// save responses carry the usage so the client can warn on every booking
// once the limit is passed.
func TestContractUsageAcrossUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	cust := &models.Customer{Name: "Acme"}
	require.NoError(t, db.Create(cust).Error)
	hours := 10.0
	limited := &models.Contract{CustomerID: cust.ID, Name: "Retainer", HoursPerWeek: &hours}
	require.NoError(t, db.Create(limited).Error)
	open := &models.Contract{CustomerID: cust.ID, Name: "Ad hoc"}
	require.NoError(t, db.Create(open).Error)

	alice := &models.User{Username: "alice", Email: "alice@example.com", PasswordHash: "x", GlobalRole: "admin"}
	bob := &models.User{Username: "bob", Email: "bob@example.com", PasswordHash: "x", GlobalRole: "admin"}
	require.NoError(t, db.Create(alice).Error)
	require.NoError(t, db.Create(bob).Error)

	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	seed := []models.TimeEntry{
		{UserID: bob.ID, CustomerID: &cust.ID, ContractID: &limited.ID, Date: day("2026-11-17"), Minutes: 360},
		{UserID: bob.ID, CustomerID: &cust.ID, ContractID: &limited.ID, Date: day("2026-11-18"), Minutes: 480, IsHoliday: true},
		{UserID: bob.ID, CustomerID: &cust.ID, ContractID: &limited.ID, Date: day("2026-11-15"), Minutes: 480}, // previous week
		{UserID: bob.ID, CustomerID: &cust.ID, ContractID: &open.ID, Date: day("2026-11-17"), Minutes: 480},
	}
	require.NoError(t, db.Create(&seed).Error)

	save := func(method, path, body string, params gin.Params) timeEntryWithUsage {
		c, w := ginTestContext(t, method, path, body)
		c.Set(middleware.ContextUserID, alice.ID)
		c.Set(middleware.ContextGlobalRole, "admin")
		c.Params = params
		if method == http.MethodPost {
			CreateTimeEntry(c)
		} else {
			UpdateTimeEntry(c)
		}
		require.Less(t, w.Code, 300, w.Body.String())
		var out timeEntryWithUsage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}

	// Bob's 6h + Alice's 3h = 9h of 10h: not exceeded yet.
	out := save(http.MethodPost, "/time-entries",
		`{"customer_id":1,"contract_id":1,"date":"2026-11-22","minutes":180}`, nil)
	require.NotNil(t, out.ContractUsage)
	require.Equal(t, "2026-11-16", out.ContractUsage.WeekStart)
	require.Equal(t, 540, out.ContractUsage.BookedMinutes)
	require.Equal(t, 600, out.ContractUsage.LimitMinutes)
	require.False(t, out.ContractUsage.Exceeded)

	// Growing Alice's entry to 5h passes the limit (11h of 10h).
	id := out.ID
	out = save(http.MethodPut, "/time-entries/x",
		`{"customer_id":1,"contract_id":1,"date":"2026-11-22","minutes":300}`,
		gin.Params{{Key: "id", Value: itoa(id)}})
	require.Equal(t, 660, out.ContractUsage.BookedMinutes)
	require.True(t, out.ContractUsage.Exceeded)

	// A contract without hours per week carries no usage.
	out = save(http.MethodPost, "/time-entries",
		`{"customer_id":1,"contract_id":2,"date":"2026-11-17","minutes":60}`, nil)
	require.Nil(t, out.ContractUsage)

	// The grid endpoint reports only contracts with a weekly limit.
	c, w := ginTestContext(t, http.MethodGet, "/time-entries/contract-usage?date=2026-11-19&contract_ids=1,2", "")
	c.Set(middleware.ContextUserID, alice.ID)
	c.Set(middleware.ContextGlobalRole, "admin")
	GetContractUsage(c)
	require.Equal(t, http.StatusOK, w.Code)
	var usage []ContractWeekUsage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &usage))
	require.Len(t, usage, 1)
	require.Equal(t, limited.ID, usage[0].ContractID)
	require.Equal(t, 660, usage[0].BookedMinutes)
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }
