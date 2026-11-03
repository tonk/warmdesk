package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
)

func listMyCards(t *testing.T, user *models.User) []MyCardItem {
	t.Helper()
	c, w := ginTestContext(t, http.MethodGet, "/my-cards", "")
	c.Set(middleware.ContextUserID, user.ID)
	c.Set(middleware.ContextGlobalRole, user.GlobalRole)
	ListMyCards(c)
	require.Equal(t, http.StatusOK, w.Code)
	var out []MyCardItem
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestListMyCards(t *testing.T) {
	f := newTransferFixture(t)
	db := f.db

	due := func(d int) *time.Time { v := time.Now().AddDate(0, 0, d); return &v }

	// Legacy single assignee, no due date.
	legacy := f.card(t, f.srcTodo, 1, "legacy", nil)
	require.NoError(t, db.Model(legacy).Update("assignee_id", f.member.ID).Error)
	// Multi-assignee, due later.
	multi := f.card(t, f.dstBacklog, 1, "multi", nil)
	require.NoError(t, db.Model(multi).Update("due_date", due(5)).Error)
	require.NoError(t, db.Create(&models.CardAssignee{CardID: multi.ID, UserID: f.member.ID}).Error)
	// Both legacy and multi on the same card, due soon — must appear once.
	both := f.card(t, f.srcDone, 2, "both", nil)
	require.NoError(t, db.Model(both).Updates(map[string]any{"assignee_id": f.member.ID, "due_date": due(1)}).Error)
	require.NoError(t, db.Create(&models.CardAssignee{CardID: both.ID, UserID: f.member.ID}).Error)
	// Closed card — excluded.
	closed := f.card(t, f.srcTodo, 3, "closed", nil)
	require.NoError(t, db.Model(closed).Updates(map[string]any{"assignee_id": f.member.ID, "closed": true}).Error)
	// Not assigned — excluded.
	f.card(t, f.srcTodo, 4, "unassigned", nil)
	// Assigned to the stranger in a project they are no member of — excluded for them.
	foreign := f.card(t, f.dstBacklog, 2, "foreign", nil)
	require.NoError(t, db.Create(&models.CardAssignee{CardID: foreign.ID, UserID: f.stranger.ID}).Error)

	got := listMyCards(t, f.member)
	titles := make([]string, len(got))
	for i, c := range got {
		titles[i] = c.Title
	}
	assert.Equal(t, []string{"both", "multi", "legacy"}, titles, "sorted by due date, undated last")
	assert.Equal(t, "Done", got[0].ColumnName)
	assert.Equal(t, "src", got[0].Project.Slug)
	assert.Equal(t, "DST", got[1].Project.KeyPrefix)

	assert.Empty(t, listMyCards(t, f.stranger))

	// Closing a project hides its cards.
	require.NoError(t, db.Model(f.src).Update("is_closed", true).Error)
	got = listMyCards(t, f.member)
	require.Len(t, got, 1)
	assert.Equal(t, "multi", got[0].Title)
}
