package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

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
