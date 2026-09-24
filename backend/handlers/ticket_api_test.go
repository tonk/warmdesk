package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

func TestTicketListCards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	owner := &models.User{Username: "owner", Email: "owner@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	viewer := &models.User{Username: "viewer", Email: "viewer@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	outsider := &models.User{Username: "outsider", Email: "outsider@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(owner).Error)
	require.NoError(t, db.Create(viewer).Error)
	require.NoError(t, db.Create(outsider).Error)

	project := &models.Project{Name: "Ansible", Slug: "ansible", KeyPrefix: "ANSI", CreatedByID: owner.ID}
	require.NoError(t, db.Create(project).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: project.ID, UserID: viewer.ID, Role: "viewer"}).Error)

	backlog := &models.Column{ProjectID: project.ID, Name: "Backlog", Position: 1000}
	done := &models.Column{ProjectID: project.ID, Name: "Done", Position: 2000}
	require.NoError(t, db.Create(backlog).Error)
	require.NoError(t, db.Create(done).Error)

	mk := func(col *models.Column, title string, pos float64, num int, closed bool) {
		card := &models.Card{ColumnID: col.ID, ProjectID: project.ID, Title: title, Position: pos, CreatedByID: owner.ID, CardNumber: num}
		require.NoError(t, db.Create(card).Error)
		if closed {
			require.NoError(t, db.Model(card).Update("closed", true).Error)
		}
	}
	mk(done, "Shipped", 1000, 1, false)
	mk(backlog, "Second", 2000, 2, false)
	mk(backlog, "First", 1000, 3, false)
	mk(backlog, "Old", 3000, 4, true)

	list := func(userID uint, query string) (*http.Response, []ticketAPICard) {
		c, w := ginTestContext(t, http.MethodGet, "/ticket/ansible/cards"+query, "")
		c.Set(middleware.ContextUserID, userID)
		c.Set(middleware.ContextGlobalRole, "user")
		c.Params = gin.Params{{Key: "projectSlug", Value: "ansible"}}
		TicketListCards(c)
		var out []ticketAPICard
		if w.Code == http.StatusOK {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		}
		return w.Result(), out
	}

	t.Run("viewer sees open cards ordered by column then position", func(t *testing.T) {
		res, cards := list(viewer.ID, "")
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Len(t, cards, 3)
		assert.Equal(t, []string{"First", "Second", "Shipped"}, []string{cards[0].Title, cards[1].Title, cards[2].Title})
		assert.Equal(t, "Backlog", cards[0].ColumnName)
		assert.Equal(t, "ANSI-3", cards[0].Key)
	})

	t.Run("filter by column name is case-insensitive", func(t *testing.T) {
		res, cards := list(viewer.ID, "?column=backlog")
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Len(t, cards, 2)
		for _, c := range cards {
			assert.Equal(t, backlog.ID, c.ColumnID)
		}
	})

	t.Run("include_closed returns closed cards", func(t *testing.T) {
		_, cards := list(viewer.ID, "?column=Backlog&include_closed=true")
		assert.Len(t, cards, 3)
	})

	t.Run("unknown column is a 400", func(t *testing.T) {
		res, _ := list(viewer.ID, "?column=Nope")
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("non-member is forbidden", func(t *testing.T) {
		res, _ := list(outsider.ID, "")
		assert.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

func TestTicketUpdateCard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	member := &models.User{Username: "member", Email: "member@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	viewer := &models.User{Username: "viewer", Email: "viewer@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(member).Error)
	require.NoError(t, db.Create(viewer).Error)

	project := &models.Project{Name: "Ansible", Slug: "ansible", KeyPrefix: "ANSI", CreatedByID: member.ID}
	other := &models.Project{Name: "Other", Slug: "other", KeyPrefix: "OTH", CreatedByID: member.ID}
	require.NoError(t, db.Create(project).Error)
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: project.ID, UserID: member.ID, Role: "member"}).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: project.ID, UserID: viewer.ID, Role: "viewer"}).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: other.ID, UserID: member.ID, Role: "member"}).Error)

	col := &models.Column{ProjectID: project.ID, Name: "Backlog", Position: 1000}
	otherCol := &models.Column{ProjectID: other.ID, Name: "Backlog", Position: 1000}
	require.NoError(t, db.Create(col).Error)
	require.NoError(t, db.Create(otherCol).Error)

	sp := 3
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	card := &models.Card{ColumnID: col.ID, ProjectID: project.ID, Title: "h03-11 Loops", Description: "old", Priority: "none",
		CreatedByID: member.ID, CardNumber: 1, StoryPoints: &sp, StartDate: &start}
	require.NoError(t, db.Create(card).Error)
	foreign := &models.Card{ColumnID: otherCol.ID, ProjectID: other.ID, Title: "Foreign", CreatedByID: member.ID, CardNumber: 1}
	require.NoError(t, db.Create(foreign).Error)

	patch := func(userID uint, cardID uint, body string) (int, ticketAPICard) {
		c, w := ginTestContext(t, http.MethodPatch, "/ticket/ansible/cards/x", body)
		c.Set(middleware.ContextUserID, userID)
		c.Set(middleware.ContextGlobalRole, "user")
		c.Params = gin.Params{{Key: "projectSlug", Value: "ansible"}, {Key: "cardId", Value: strconv.FormatUint(uint64(cardID), 10)}}
		TicketUpdateCard(c)
		var out ticketAPICard
		if w.Code == http.StatusOK {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		}
		return w.Code, out
	}
	reload := func() models.Card {
		var got models.Card
		require.NoError(t, db.First(&got, card.ID).Error)
		return got
	}

	t.Run("partial update leaves other fields alone", func(t *testing.T) {
		before := reload()
		code, out := patch(member.ID, card.ID, `{"priority":"medium","start_date":"2026-09-24"}`)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "medium", out.Priority)
		assert.Equal(t, "ANSI-1", out.Key)
		assert.Equal(t, "Backlog", out.ColumnName)
		got := reload()
		assert.Equal(t, "medium", got.Priority)
		require.NotNil(t, got.StartDate)
		assert.Equal(t, "2026-09-24", got.StartDate.UTC().Format("2006-01-02"))
		assert.Equal(t, "h03-11 Loops", got.Title)
		assert.Equal(t, "old", got.Description)
		require.NotNil(t, got.StoryPoints)
		assert.Equal(t, 3, *got.StoryPoints)
		assert.False(t, got.UpdatedAt.Before(before.UpdatedAt))

		var events []string
		db.Model(&models.CardHistory{}).Where("card_id = ?", card.ID).Order("id").Pluck("event_type", &events)
		assert.ElementsMatch(t, []string{"priority_changed", "start_date_changed"}, events)
	})

	t.Run("title and description", func(t *testing.T) {
		code, _ := patch(member.ID, card.ID, `{"title":"h06-02 Loops","description":"new"}`)
		require.Equal(t, http.StatusOK, code)
		got := reload()
		assert.Equal(t, "h06-02 Loops", got.Title)
		assert.Equal(t, "new", got.Description)
	})

	t.Run("RFC 3339 dates are accepted", func(t *testing.T) {
		code, _ := patch(member.ID, card.ID, `{"due_date":"2026-10-01T12:00:00Z"}`)
		require.Equal(t, http.StatusOK, code)
		require.NotNil(t, reload().DueDate)
	})

	t.Run("explicit null clears nullable fields", func(t *testing.T) {
		code, _ := patch(member.ID, card.ID, `{"start_date":null,"due_date":null,"story_points":null}`)
		require.Equal(t, http.StatusOK, code)
		got := reload()
		assert.Nil(t, got.StartDate)
		assert.Nil(t, got.DueDate)
		assert.Nil(t, got.StoryPoints)
		assert.Equal(t, "h06-02 Loops", got.Title)
	})

	t.Run("validation errors change nothing", func(t *testing.T) {
		for _, body := range []string{
			`{"title":""}`,
			`{"title":null}`,
			`{"priority":"urgent"}`,
			`{"start_date":"24-09-2026"}`,
			`{"story_points":-1}`,
			`{"story_points":"five"}`,
			`{"column_id":2}`,
			`{"start_date":"2026-09-24","due_date":"2026-09-01"}`,
			`{"priority":"high","title":""}`,
			`not json`,
		} {
			code, _ := patch(member.ID, card.ID, body)
			assert.Equal(t, http.StatusBadRequest, code, body)
		}
		assert.Equal(t, "medium", reload().Priority, "a rejected request must not apply any field")
	})

	t.Run("due before existing start is rejected", func(t *testing.T) {
		code, _ := patch(member.ID, card.ID, `{"start_date":"2026-09-24"}`)
		require.Equal(t, http.StatusOK, code)
		code, _ = patch(member.ID, card.ID, `{"due_date":"2026-09-23"}`)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("card from another project is 404", func(t *testing.T) {
		code, _ := patch(member.ID, foreign.ID, `{"priority":"high"}`)
		assert.Equal(t, http.StatusNotFound, code)
		var got models.Card
		require.NoError(t, db.First(&got, foreign.ID).Error)
		assert.NotEqual(t, "high", got.Priority)
	})

	t.Run("viewer is forbidden", func(t *testing.T) {
		code, _ := patch(viewer.ID, card.ID, `{"priority":"high"}`)
		assert.Equal(t, http.StatusForbidden, code)
	})
}

func TestTicketAPIKeysLanesAndCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	defer cleanup()
	prevDB := database.DB
	database.DB = db
	defer func() { database.DB = prevDB }()

	member := &models.User{Username: "member", Email: "member@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(member).Error)
	project := &models.Project{Name: "Ansible", Slug: "ansible", KeyPrefix: "ANSI", CreatedByID: member.ID}
	other := &models.Project{Name: "Other", Slug: "other", KeyPrefix: "OTH", CreatedByID: member.ID}
	require.NoError(t, db.Create(project).Error)
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: project.ID, UserID: member.ID, Role: "member"}).Error)
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: other.ID, UserID: member.ID, Role: "member"}).Error)

	backlog := &models.Column{ProjectID: project.ID, Name: "Backlog", Position: 1000}
	done := &models.Column{ProjectID: project.ID, Name: "Done", Position: 2000}
	otherCol := &models.Column{ProjectID: other.ID, Name: "Done", Position: 1000}
	for _, col := range []*models.Column{backlog, done, otherCol} {
		require.NoError(t, db.Create(col).Error)
	}
	require.NoError(t, db.Model(&models.Project{}).Where("id = ?", project.ID).Update("card_counter", 4).Error)
	card := &models.Card{ColumnID: backlog.ID, ProjectID: project.ID, Title: "Loops", Priority: "none", CreatedByID: member.ID, CardNumber: 4}
	require.NoError(t, db.Create(card).Error)
	foreign := &models.Card{ColumnID: otherCol.ID, ProjectID: other.ID, Title: "Foreign", CreatedByID: member.ID, CardNumber: 4}
	require.NoError(t, db.Create(foreign).Error)

	call := func(h gin.HandlerFunc, method, cardRef, body string) (int, ticketAPICard) {
		c, w := ginTestContext(t, method, "/ticket/ansible/x", body)
		c.Set(middleware.ContextUserID, member.ID)
		c.Set(middleware.ContextGlobalRole, "user")
		c.Params = gin.Params{{Key: "projectSlug", Value: "ansible"}}
		if cardRef != "" {
			c.Params = append(c.Params, gin.Param{Key: "cardId", Value: cardRef})
		}
		h(c)
		var out ticketAPICard
		if w.Code < 300 {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		}
		return w.Code, out
	}

	t.Run("card can be addressed by key, case-insensitively", func(t *testing.T) {
		for _, ref := range []string{"ANSI-4", "ansi-4", strconv.FormatUint(uint64(card.ID), 10)} {
			code, out := call(TicketGetCard, http.MethodGet, ref, "")
			require.Equal(t, http.StatusOK, code, ref)
			assert.Equal(t, card.ID, out.ID, ref)
		}
	})

	t.Run("keys never reach another project's cards", func(t *testing.T) {
		code, _ := call(TicketGetCard, http.MethodGet, "OTH-4", "")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = call(TicketGetCard, http.MethodGet, "ANSI-99", "")
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = call(TicketGetCard, http.MethodGet, "not-a-key", "")
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("close and reopen via PATCH", func(t *testing.T) {
		code, out := call(TicketUpdateCard, http.MethodPatch, "ANSI-4", `{"closed":true}`)
		require.Equal(t, http.StatusOK, code)
		assert.True(t, out.Closed)
		assert.NotNil(t, out.ClosedAt)
		code, out = call(TicketUpdateCard, http.MethodPatch, "ANSI-4", `{"closed":false}`)
		require.Equal(t, http.StatusOK, code)
		assert.False(t, out.Closed)
		assert.Nil(t, out.ClosedAt)
		var events []string
		db.Model(&models.CardHistory{}).Where("card_id = ?", card.ID).Order("id").Pluck("event_type", &events)
		assert.Equal(t, []string{"closed", "reopened"}, events)
		code, _ = call(TicketUpdateCard, http.MethodPatch, "ANSI-4", `{"closed":null}`)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("move by lane name", func(t *testing.T) {
		code, out := call(TicketMove, http.MethodPatch, "ANSI-4", `{"column":"done"}`)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, done.ID, out.ColumnID)
		assert.Equal(t, "Done", out.ColumnName)
		for _, body := range []string{`{"column":"Nope"}`, `{}`, fmt.Sprintf(`{"column_id":%d}`, otherCol.ID), `{"column":"Done","column_id":1}`} {
			code, _ := call(TicketMove, http.MethodPatch, "ANSI-4", body)
			assert.Equal(t, http.StatusBadRequest, code, body)
		}
	})

	t.Run("create with lane name and extra fields", func(t *testing.T) {
		code, out := call(TicketAdd, http.MethodPost, "",
			`{"title":"New","column":"Backlog","priority":"high","start_date":"2026-09-24","due_date":"2026-09-30","story_points":5,"unrelated":"ignored"}`)
		require.Equal(t, http.StatusCreated, code)
		assert.Equal(t, backlog.ID, out.ColumnID)
		assert.Equal(t, "high", out.Priority)
		assert.Equal(t, "ANSI-5", out.Key)
		require.NotNil(t, out.StartDate)
		require.NotNil(t, out.StoryPoints)
		assert.Equal(t, 5, *out.StoryPoints)
	})

	t.Run("create still works with column_id only", func(t *testing.T) {
		code, out := call(TicketAdd, http.MethodPost, "", fmt.Sprintf(`{"title":"Plain","column_id":%d}`, done.ID))
		require.Equal(t, http.StatusCreated, code)
		assert.Equal(t, "none", out.Priority)
	})

	t.Run("create validation", func(t *testing.T) {
		for _, body := range []string{
			`{"column":"Backlog"}`,
			`{"title":"x"}`,
			`{"title":"x","column":"Nope"}`,
			`{"title":"x","column":"Backlog","priority":"urgent"}`,
			`{"title":"x","column":"Backlog","start_date":"2026-09-24","due_date":"2026-09-01"}`,
		} {
			code, _ := call(TicketAdd, http.MethodPost, "", body)
			assert.Equal(t, http.StatusBadRequest, code, body)
		}
	})
}
