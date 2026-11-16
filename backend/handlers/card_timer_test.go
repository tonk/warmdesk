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

// Card timers and timed columns: moving a card into a timed column starts
// the mover's timer on that card, timed → timed keeps it running, and moving
// it out to an untimed column books it with the card id.
func TestTimedColumns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()
	prevMode := serverAppMode
	defer func() { serverAppMode = prevMode }()

	user := &models.User{Username: "u", Email: "u@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true, TimeTrackingEnabled: true}
	noTT := &models.User{Username: "n", Email: "n@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(noTT).Error)

	acme := &models.Customer{Name: "Acme"}
	require.NoError(t, db.Create(acme).Error)
	web := &models.Project{Name: "Website", Slug: "website", KeyPrefix: "WEB", CreatedByID: user.ID, CustomerID: &acme.ID}
	other := &models.Project{Name: "Other", Slug: "other", KeyPrefix: "OTH", CreatedByID: user.ID}
	require.NoError(t, db.Create(web).Error)
	require.NoError(t, db.Create(other).Error)
	for _, p := range []*models.Project{web, other} {
		require.NoError(t, db.Create(&models.ProjectMember{ProjectID: p.ID, UserID: user.ID, Role: "owner"}).Error)
		require.NoError(t, db.Create(&models.ProjectMember{ProjectID: p.ID, UserID: noTT.ID, Role: "member"}).Error)
	}

	todo := &models.Column{ProjectID: web.ID, Name: "Todo", Position: 1000}
	doing := &models.Column{ProjectID: web.ID, Name: "In Progress", Position: 2000, Timed: true}
	review := &models.Column{ProjectID: web.ID, Name: "Review", Position: 3000, Timed: true}
	done := &models.Column{ProjectID: web.ID, Name: "Done", Position: 4000}
	for _, col := range []*models.Column{todo, doing, review, done} {
		require.NoError(t, db.Create(col).Error)
	}
	card := &models.Card{ProjectID: web.ID, ColumnID: todo.ID, Title: "Build login", CardNumber: 1, CreatedByID: user.ID}
	require.NoError(t, db.Create(card).Error)
	otherCol := &models.Column{ProjectID: other.ID, Name: "Todo"}
	require.NoError(t, db.Create(otherCol).Error)
	otherCard := &models.Card{ProjectID: other.ID, ColumnID: otherCol.ID, Title: "Elsewhere", CardNumber: 1, CreatedByID: user.ID}
	require.NoError(t, db.Create(otherCard).Error)

	as := func(u *models.User) func(h gin.HandlerFunc, method, body string, params ...gin.Param) (int, []byte) {
		return func(h gin.HandlerFunc, method, body string, params ...gin.Param) (int, []byte) {
			c, w := ginTestContext(t, method, "/", body)
			c.Params = params
			c.Set(middleware.ContextUserID, u.ID)
			c.Set(middleware.ContextGlobalRole, "user")
			h(c)
			c.Writer.WriteHeaderNow()
			return w.Code, w.Body.Bytes()
		}
	}
	call := as(user)
	move := func(u *models.User, colID uint) {
		code, body := as(u)(MoveCard, http.MethodPatch, fmt.Sprintf(`{"column_id":%d,"position":1}`, colID),
			gin.Param{Key: "projectSlug", Value: "website"},
			gin.Param{Key: "cardId", Value: fmt.Sprint(card.ID)})
		require.Equal(t, http.StatusOK, code, string(body))
	}
	timerOf := func(u *models.User) *models.TimeTimer {
		var tm models.TimeTimer
		if db.Where("user_id = ?", u.ID).First(&tm).Error != nil {
			return nil
		}
		return &tm
	}
	backdate := func(u *models.User, d time.Duration) {
		require.NoError(t, db.Model(&models.TimeTimer{}).Where("user_id = ?", u.ID).
			Update("started_at", time.Now().Add(-d)).Error)
	}

	t.Run("into a timed column starts a timer on the card", func(t *testing.T) {
		move(user, doing.ID)
		tm := timerOf(user)
		require.NotNil(t, tm)
		require.NotNil(t, tm.CardID)
		assert.Equal(t, card.ID, *tm.CardID)
		assert.Equal(t, web.ID, *tm.ProjectID)
		assert.Equal(t, acme.ID, *tm.CustomerID, "the project brings its customer")
		assert.Equal(t, "WEB-1 Build login", tm.Description, "no earlier description: card key and title")
	})

	t.Run("timed to timed keeps the same timer", func(t *testing.T) {
		before := timerOf(user).StartedAt
		move(user, review.ID)
		tm := timerOf(user)
		require.NotNil(t, tm)
		assert.True(t, tm.StartedAt.Equal(before))
	})

	t.Run("out to an untimed column books it on the card", func(t *testing.T) {
		backdate(user, 40*time.Minute)
		move(user, done.ID)
		assert.Nil(t, timerOf(user))
		var entries []models.TimeEntry
		require.NoError(t, db.Where("card_id = ?", card.ID).Find(&entries).Error)
		require.Len(t, entries, 1)
		assert.Equal(t, 45, entries[0].Minutes)
		assert.Equal(t, web.ID, *entries[0].ProjectID)
	})

	t.Run("back in, the last description on the card is resumed", func(t *testing.T) {
		require.NoError(t, db.Model(&models.TimeEntry{}).Where("card_id = ?", card.ID).
			Update("description", "Investigating approaches").Error)
		move(user, doing.ID)
		tm := timerOf(user)
		require.NotNil(t, tm)
		assert.Equal(t, "Investigating approaches", tm.Description)
	})

	t.Run("another card's timer is booked when switching", func(t *testing.T) {
		code, body := call(StartTimer, http.MethodPost, fmt.Sprintf(`{"card_id":%d,"description":"other work"}`, otherCard.ID))
		require.Equal(t, http.StatusCreated, code, string(body))
		move(user, todo.ID)
		assert.NotNil(t, timerOf(user), "leaving the timed column only stops a timer on this card")
		backdate(user, 10*time.Minute)
		move(user, doing.ID)
		tm := timerOf(user)
		require.NotNil(t, tm)
		assert.Equal(t, card.ID, *tm.CardID)
		var n int64
		db.Model(&models.TimeEntry{}).Where("card_id = ? AND description = ?", otherCard.ID, "other work").Count(&n)
		assert.EqualValues(t, 1, n)
		require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&models.TimeTimer{}).Error)
		move(user, todo.ID)
	})

	t.Run("users without time tracking are not timed", func(t *testing.T) {
		move(noTT, doing.ID)
		assert.Nil(t, timerOf(noTT))
		move(noTT, todo.ID)
	})

	t.Run("no timers on a board-only server", func(t *testing.T) {
		serverAppMode = "board"
		defer func() { serverAppMode = prevMode }()
		move(user, doing.ID)
		assert.Nil(t, timerOf(user))
		move(user, todo.ID)
	})

	t.Run("start with card_id", func(t *testing.T) {
		code, body := call(StartTimer, http.MethodPost, fmt.Sprintf(`{"card_id":%d,"project_id":%d}`, card.ID, other.ID))
		assert.Equal(t, http.StatusBadRequest, code, "card of another project")
		code, body = call(StartTimer, http.MethodPost, `{"card_id":999999}`)
		assert.Equal(t, http.StatusBadRequest, code, string(body))

		code, body = call(StartTimer, http.MethodPost, fmt.Sprintf(`{"card_id":%d}`, card.ID))
		require.Equal(t, http.StatusCreated, code, string(body))
		var out timerResponse
		require.NoError(t, json.Unmarshal(body, &out))
		require.NotNil(t, out.Timer.Card)
		assert.Equal(t, "Build login", out.Timer.Card.Title)
		assert.Equal(t, "Investigating approaches", out.Timer.Description)
	})

	t.Run("card time sums per user and description and lists running timers", func(t *testing.T) {
		code, body := call(GetCardTime, http.MethodGet, "",
			gin.Param{Key: "projectSlug", Value: "website"},
			gin.Param{Key: "cardId", Value: fmt.Sprint(card.ID)})
		require.Equal(t, http.StatusOK, code, string(body))
		var out cardTimeResponse
		require.NoError(t, json.Unmarshal(body, &out))
		require.Len(t, out.Rows, 1)
		assert.Equal(t, "Investigating approaches", out.Rows[0].Description)
		assert.Equal(t, "u", out.Rows[0].UserName)
		assert.Equal(t, 60, out.TotalMinutes, "45 from leaving the column, 15 booked by the switch")
		require.Len(t, out.Running, 1)
		assert.Equal(t, user.ID, out.Running[0].UserID)

		code, _ = call(GetCardTime, http.MethodGet, "",
			gin.Param{Key: "projectSlug", Value: "other"},
			gin.Param{Key: "cardId", Value: fmt.Sprint(card.ID)})
		assert.Equal(t, http.StatusNotFound, code, "card is looked up within the project")
	})

	t.Run("columns store the timed flag", func(t *testing.T) {
		code, body := call(UpdateColumn, http.MethodPut, `{"timed":false}`,
			gin.Param{Key: "projectSlug", Value: "website"},
			gin.Param{Key: "columnId", Value: fmt.Sprint(doing.ID)})
		require.Equal(t, http.StatusOK, code, string(body))
		var col models.Column
		require.NoError(t, db.First(&col, doing.ID).Error)
		assert.False(t, col.Timed)

		code, body = call(UpdateColumn, http.MethodPut, `{"name":"Doing"}`,
			gin.Param{Key: "projectSlug", Value: "website"},
			gin.Param{Key: "columnId", Value: fmt.Sprint(review.ID)})
		require.Equal(t, http.StatusOK, code, string(body))
		var reviewCol models.Column
		require.NoError(t, db.First(&reviewCol, review.ID).Error)
		assert.True(t, reviewCol.Timed, "omitting timed leaves it alone")
	})
}
