package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/testutil"
)

// A regression guard for a bug where UpdateCustomer unconditionally
// overwrote color/billing/VAT/PO fields from the request body, so any
// partial update (e.g. the web UI's inline rename, which only sends
// name/description/logo_url) silently wiped them back to empty.
func TestUpdateCustomerPreservesOmittedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	cust := &models.Customer{
		Name:          "Acme",
		Color:         "#3b82f6",
		BillingStreet: "1 Main St",
		VATNumber:     "NL123456789B01",
		POReference:   "PO-42",
	}
	require.NoError(t, db.Create(cust).Error)

	admin := &models.User{Username: "admin", Email: "admin@example.com", PasswordHash: "x", GlobalRole: "admin"}
	require.NoError(t, db.Create(admin).Error)

	c, w := ginTestContext(t, http.MethodPut, "/customers/1", `{"name":"Acme Renamed"}`)
	c.Set(middleware.ContextUserID, admin.ID)
	c.Set(middleware.ContextGlobalRole, "admin")
	c.Params = gin.Params{{Key: "customerId", Value: "1"}}

	UpdateCustomer(c)
	require.Equal(t, http.StatusOK, w.Code)

	var reloaded models.Customer
	require.NoError(t, db.First(&reloaded, cust.ID).Error)
	require.Equal(t, "Acme Renamed", reloaded.Name)
	require.Equal(t, "#3b82f6", reloaded.Color, "color must survive a partial update that doesn't mention it")
	require.Equal(t, "1 Main St", reloaded.BillingStreet)
	require.Equal(t, "NL123456789B01", reloaded.VATNumber)
	require.Equal(t, "PO-42", reloaded.POReference)
}

func ginTestContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// Contract hours per week are optional: they round-trip through create and
// update, an update without them clears them, and out-of-range values
// (negative, or more than the 168 hours a week has) are refused.
func TestContractHoursPerWeek(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	cust := &models.Customer{Name: "Acme"}
	require.NoError(t, db.Create(cust).Error)

	asAdmin := func(method, path, body string, params gin.Params) *httptest.ResponseRecorder {
		c, w := ginTestContext(t, method, path, body)
		c.Set(middleware.ContextGlobalRole, "admin")
		c.Params = params
		switch method {
		case http.MethodPost:
			CreateContract(c)
		case http.MethodPut:
			UpdateContract(c)
		}
		return w
	}
	custParams := gin.Params{{Key: "customerId", Value: "1"}}

	w := asAdmin(http.MethodPost, "/customers/1/contracts", `{"name":"Retainer","hours_per_week":24}`, custParams)
	require.Equal(t, http.StatusCreated, w.Code)
	var con models.Contract
	require.NoError(t, db.First(&con).Error)
	require.NotNil(t, con.HoursPerWeek)
	require.Equal(t, 24.0, *con.HoursPerWeek)

	conParams := gin.Params{{Key: "contractId", Value: "1"}}
	w = asAdmin(http.MethodPut, "/contracts/1", `{"name":"Retainer","hours_per_week":32.5}`, conParams)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, db.First(&con, con.ID).Error)
	require.Equal(t, 32.5, *con.HoursPerWeek)

	w = asAdmin(http.MethodPut, "/contracts/1", `{"name":"Retainer","hours_per_week":169}`, conParams)
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = asAdmin(http.MethodPost, "/customers/1/contracts", `{"name":"Bad","hours_per_week":-1}`, custParams)
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = asAdmin(http.MethodPut, "/contracts/1", `{"name":"Retainer"}`, conParams)
	require.Equal(t, http.StatusOK, w.Code)
	var reloaded models.Contract
	require.NoError(t, db.First(&reloaded, con.ID).Error)
	require.Nil(t, reloaded.HoursPerWeek)
}
