package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"github.com/tonk/warmdesk/testutil"
	"gorm.io/gorm"
)

type transferFixture struct {
	db                  *gorm.DB
	member, stranger    *models.User
	src, dst            *models.Project
	srcTodo, srcDone    *models.Column
	dstBacklog, dstDone *models.Column
}

func newTransferFixture(t *testing.T) *transferFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cleanup := testutil.SetupTestDB()
	t.Cleanup(cleanup)
	prevDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = prevDB })

	f := &transferFixture{db: db}
	f.member = &models.User{Username: "member", Email: "member@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	f.stranger = &models.User{Username: "stranger", Email: "stranger@example.com", PasswordHash: "x", GlobalRole: "user", IsActive: true}
	require.NoError(t, db.Create(f.member).Error)
	require.NoError(t, db.Create(f.stranger).Error)

	f.src = &models.Project{Name: "Source", Slug: "src", KeyPrefix: "SRC", CreatedByID: f.member.ID, CardCounter: 12}
	f.dst = &models.Project{Name: "Dest", Slug: "dst", KeyPrefix: "DST", CreatedByID: f.member.ID, CardCounter: 46}
	require.NoError(t, db.Create(f.src).Error)
	require.NoError(t, db.Create(f.dst).Error)
	for _, p := range []*models.Project{f.src, f.dst} {
		require.NoError(t, db.Create(&models.ProjectMember{ProjectID: p.ID, UserID: f.member.ID, Role: "member"}).Error)
	}
	// The stranger may see the source project only.
	require.NoError(t, db.Create(&models.ProjectMember{ProjectID: f.src.ID, UserID: f.stranger.ID, Role: "member"}).Error)

	mkCol := func(p *models.Project, name string, pos float64) *models.Column {
		col := &models.Column{ProjectID: p.ID, Name: name, Position: pos}
		require.NoError(t, db.Create(col).Error)
		return col
	}
	f.srcTodo = mkCol(f.src, "To Do", 1000)
	f.srcDone = mkCol(f.src, "Done", 2000)
	f.dstBacklog = mkCol(f.dst, "Backlog", 1000)
	f.dstDone = mkCol(f.dst, "done", 2000)
	return f
}

func (f *transferFixture) card(t *testing.T, col *models.Column, num int, title string, parent *uint) *models.Card {
	t.Helper()
	card := &models.Card{ProjectID: col.ProjectID, ColumnID: col.ID, Title: title, CardNumber: num, CreatedByID: f.member.ID, ParentCardID: parent}
	require.NoError(t, f.db.Create(card).Error)
	return card
}

func (f *transferFixture) transfer(t *testing.T, card *models.Card, body string, scope *uint) (int, transferCardResponse) {
	t.Helper()
	c, w := ginTestContext(t, http.MethodPost, "/projects/src/cards/x/transfer", body)
	c.Set(middleware.ContextUserID, f.member.ID)
	c.Set(middleware.ContextGlobalRole, "user")
	if scope != nil {
		c.Set(middleware.ContextAPIKeyProjectID, *scope)
	}
	c.Params = gin.Params{{Key: "projectSlug", Value: "src"}, {Key: "cardId", Value: fmt.Sprint(card.ID)}}
	TransferCard(c)
	var out transferCardResponse
	if w.Code == http.StatusCreated {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w.Code, out
}

func TestTransferCardMoveKeepsCardAndRenumbers(t *testing.T) {
	f := newTransferFixture(t)
	db := f.db

	card := f.card(t, f.srcTodo, 12, "Move me", nil)
	require.NoError(t, db.Create(&models.CardComment{CardID: card.ID, UserID: f.member.ID, Body: "hello"}).Error)
	require.NoError(t, db.Create(&models.CardChecklistItem{CardID: card.ID, Body: "step"}).Error)

	epic := &models.Epic{ProjectID: f.src.ID, Name: "Epic"}
	require.NoError(t, db.Create(epic).Error)
	sprint := &models.Sprint{ProjectID: f.src.ID, Name: "S1"}
	require.NoError(t, db.Create(sprint).Error)
	require.NoError(t, db.Model(card).Updates(map[string]interface{}{"epic_id": epic.ID, "assignee_id": f.stranger.ID}).Error)
	require.NoError(t, db.Create(&models.SprintCard{SprintID: sprint.ID, CardID: card.ID}).Error)

	// "Bug" exists in the target under another case; "Infra" does not.
	srcBug := &models.Label{ProjectID: f.src.ID, Name: "Bug", Color: "#ff0000"}
	srcInfra := &models.Label{ProjectID: f.src.ID, Name: "Infra", Color: "#00ff00"}
	dstBug := &models.Label{ProjectID: f.dst.ID, Name: "bug", Color: "#aa0000"}
	for _, l := range []*models.Label{srcBug, srcInfra, dstBug} {
		require.NoError(t, db.Create(l).Error)
	}
	require.NoError(t, db.Create(&models.CardLabel{CardID: card.ID, LabelID: srcBug.ID}).Error)
	require.NoError(t, db.Create(&models.CardLabel{CardID: card.ID, LabelID: srcInfra.ID}).Error)

	require.NoError(t, db.Create(&models.CardAssignee{CardID: card.ID, UserID: f.member.ID}).Error)
	require.NoError(t, db.Create(&models.CardAssignee{CardID: card.ID, UserID: f.stranger.ID}).Error)
	require.NoError(t, db.Exec("INSERT INTO card_watchers (card_id, user_id) VALUES (?, ?), (?, ?)", card.ID, f.member.ID, card.ID, f.stranger.ID).Error)

	code, out := f.transfer(t, card, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"move"}`, f.dstBacklog.ID), nil)
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, card.ID, out.ID, "a move keeps the same card")
	assert.Equal(t, "DST-47", out.Key)
	assert.Equal(t, "SRC-12", out.PreviousKey)

	var moved models.Card
	require.NoError(t, db.First(&moved, card.ID).Error)
	assert.Equal(t, f.dst.ID, moved.ProjectID)
	assert.Equal(t, f.dstBacklog.ID, moved.ColumnID)
	assert.Equal(t, 47, moved.CardNumber)
	assert.Nil(t, moved.EpicID)
	assert.Nil(t, moved.AssigneeID, "legacy assignee without target access is cleared")

	var n int64
	db.Model(&models.CardComment{}).Where("card_id = ?", card.ID).Count(&n)
	assert.EqualValues(t, 1, n, "comments stay with the card")
	db.Model(&models.CardChecklistItem{}).Where("card_id = ?", card.ID).Count(&n)
	assert.EqualValues(t, 1, n, "checklist stays with the card")
	db.Model(&models.SprintCard{}).Where("card_id = ?", card.ID).Count(&n)
	assert.Zero(t, n, "source sprint membership is dropped")

	var labels []models.Label
	db.Joins("JOIN card_labels ON card_labels.label_id = labels.id").Where("card_labels.card_id = ?", card.ID).Order("labels.name").Find(&labels)
	require.Len(t, labels, 2)
	for _, l := range labels {
		assert.Equal(t, f.dst.ID, l.ProjectID)
	}
	assert.Equal(t, dstBug.ID, labels[1].ID, "existing target label is reused by name")
	assert.Equal(t, "Infra", labels[0].Name)
	assert.Equal(t, "#00ff00", labels[0].Color, "missing label is created with the same colour")

	var assignees, watchers []uint
	db.Model(&models.CardAssignee{}).Where("card_id = ?", card.ID).Pluck("user_id", &assignees)
	db.Table("card_watchers").Where("card_id = ?", card.ID).Pluck("user_id", &watchers)
	assert.Equal(t, []uint{f.member.ID}, assignees)
	assert.Equal(t, []uint{f.member.ID}, watchers)

	var hist models.CardHistory
	require.NoError(t, db.Where("card_id = ? AND event_type = ?", card.ID, "project_moved").First(&hist).Error)
	assert.Equal(t, "SRC-12 → DST-47", hist.Detail)

	// The old key resolves to the moved card; the new one does too.
	for _, key := range []struct {
		prefix string
		num    int
	}{{"SRC", 12}, {"src", 12}, {"DST", 47}} {
		found, err := services.FindCardByKey(key.prefix, key.num)
		require.NoError(t, err, key)
		assert.Equal(t, card.ID, found.ID)
	}

	// ResolveCardRef reports the card's current location for an old key.
	resolve := func(userID uint) *httptest.ResponseRecorder {
		c, w := ginTestContext(t, http.MethodGet, "/cards/resolve/SRC-12", "")
		c.Set(middleware.ContextUserID, userID)
		c.Set(middleware.ContextGlobalRole, "user")
		c.Params = gin.Params{{Key: "ref", Value: "SRC-12"}}
		ResolveCardRef(c)
		return w
	}
	// The stranger may see the source project, not the card's new one.
	assert.Equal(t, http.StatusNotFound, resolve(f.stranger.ID).Code, "no access to the card's project")
	w := resolve(f.member.ID)
	require.Equal(t, http.StatusOK, w.Code)
	var ref struct {
		ProjectSlug string `json:"project_slug"`
		CardNumber  int    `json:"card_number"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ref))
	assert.Equal(t, "dst", ref.ProjectSlug)
	assert.Equal(t, 47, ref.CardNumber)

	// Moving it back keeps both former keys resolvable.
	code, out = transferBack(t, f, card)
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "SRC-13", out.Key)
	for _, num := range []struct {
		prefix string
		n      int
	}{{"SRC", 12}, {"DST", 47}, {"SRC", 13}} {
		found, err := services.FindCardByKey(num.prefix, num.n)
		require.NoError(t, err, num)
		assert.Equal(t, card.ID, found.ID)
	}
}

func transferBack(t *testing.T, f *transferFixture, card *models.Card) (int, transferCardResponse) {
	c, w := ginTestContext(t, http.MethodPost, "/projects/dst/cards/x/transfer",
		fmt.Sprintf(`{"target_project_slug":"src","column_id":%d,"action":"move"}`, f.srcTodo.ID))
	c.Set(middleware.ContextUserID, f.member.ID)
	c.Set(middleware.ContextGlobalRole, "user")
	c.Params = gin.Params{{Key: "projectSlug", Value: "dst"}, {Key: "cardId", Value: fmt.Sprint(card.ID)}}
	TransferCard(c)
	var out transferCardResponse
	if w.Code == http.StatusCreated {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w.Code, out
}

func TestTransferCardSubCards(t *testing.T) {
	t.Run("move along into the column of the same name", func(t *testing.T) {
		f := newTransferFixture(t)
		outside := f.card(t, f.srcTodo, 1, "Outside parent", nil)
		parent := f.card(t, f.srcTodo, 2, "Parent", &outside.ID)
		child := f.card(t, f.srcDone, 3, "Child", &parent.ID)
		grandchild := f.card(t, f.srcTodo, 4, "Grandchild", &child.ID)

		code, out := f.transfer(t, parent, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"move"}`, f.dstBacklog.ID), nil)
		require.Equal(t, http.StatusCreated, code)
		assert.ElementsMatch(t, []uint{child.ID, grandchild.ID}, out.MovedSubCardIDs)

		var p, ch, gc models.Card
		f.db.First(&p, parent.ID)
		f.db.First(&ch, child.ID)
		f.db.First(&gc, grandchild.ID)
		assert.Nil(t, p.ParentCardID, "a parent left in the source project is detached")
		assert.Equal(t, f.dst.ID, ch.ProjectID)
		assert.Equal(t, f.dstDone.ID, ch.ColumnID, "\"Done\" maps onto target's \"done\"")
		assert.Equal(t, f.dstBacklog.ID, gc.ColumnID, "no \"To Do\" in target: falls back to the chosen column")
		require.NotNil(t, gc.ParentCardID)
		assert.Equal(t, child.ID, *gc.ParentCardID, "the tree stays intact")
		assert.ElementsMatch(t, []int{47, 48, 49}, []int{p.CardNumber, ch.CardNumber, gc.CardNumber})

		found, err := services.FindCardByKey("SRC", 3)
		require.NoError(t, err)
		assert.Equal(t, child.ID, found.ID)
	})

	t.Run("detach", func(t *testing.T) {
		f := newTransferFixture(t)
		parent := f.card(t, f.srcTodo, 1, "Parent", nil)
		child := f.card(t, f.srcTodo, 2, "Child", &parent.ID)

		code, out := f.transfer(t, parent, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"move","sub_cards":"detach"}`, f.dstBacklog.ID), nil)
		require.Equal(t, http.StatusCreated, code)
		assert.Empty(t, out.MovedSubCardIDs)

		var ch models.Card
		f.db.First(&ch, child.ID)
		assert.Equal(t, f.src.ID, ch.ProjectID)
		assert.Nil(t, ch.ParentCardID)
	})

	t.Run("invalid option", func(t *testing.T) {
		f := newTransferFixture(t)
		parent := f.card(t, f.srcTodo, 1, "Parent", nil)
		code, _ := f.transfer(t, parent, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"move","sub_cards":"keep"}`, f.dstBacklog.ID), nil)
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

func TestTransferCardRejections(t *testing.T) {
	f := newTransferFixture(t)
	card := f.card(t, f.srcTodo, 12, "Card", nil)

	t.Run("key scoped to the source project cannot write into the target", func(t *testing.T) {
		for _, action := range []string{"move", "copy"} {
			code, _ := f.transfer(t, card, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"%s"}`, f.dstBacklog.ID, action), &f.src.ID)
			assert.Equal(t, http.StatusForbidden, code, action)
		}
		var still models.Card
		require.NoError(t, f.db.First(&still, card.ID).Error)
		assert.Equal(t, f.src.ID, still.ProjectID)
	})

	t.Run("move within the same project", func(t *testing.T) {
		code, _ := f.transfer(t, card, fmt.Sprintf(`{"target_project_slug":"src","column_id":%d,"action":"move"}`, f.srcDone.ID), nil)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("column of another project", func(t *testing.T) {
		code, _ := f.transfer(t, card, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"move"}`, f.srcDone.ID), nil)
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("copy still creates a new card", func(t *testing.T) {
		code, out := f.transfer(t, card, fmt.Sprintf(`{"target_project_slug":"dst","column_id":%d,"action":"copy"}`, f.dstBacklog.ID), nil)
		require.Equal(t, http.StatusCreated, code)
		assert.NotEqual(t, card.ID, out.ID)
		assert.Equal(t, "DST-47", out.Key)
		assert.Empty(t, out.PreviousKey)
	})
}

func TestTicketTransfer(t *testing.T) {
	f := newTransferFixture(t)
	card := f.card(t, f.srcTodo, 12, "Via API", nil)

	call := func(handler gin.HandlerFunc, method, slug, ref, body string, scope *uint) (int, map[string]interface{}) {
		c, w := ginTestContext(t, method, "/ticket/"+slug+"/cards/"+ref, body)
		c.Set(middleware.ContextUserID, f.member.ID)
		c.Set(middleware.ContextGlobalRole, "user")
		if scope != nil {
			c.Set(middleware.ContextAPIKeyProjectID, *scope)
		}
		c.Params = gin.Params{{Key: "projectSlug", Value: slug}, {Key: "cardId", Value: ref}}
		handler(c)
		var out map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	t.Run("validation", func(t *testing.T) {
		code, _ := call(TicketTransfer, http.MethodPost, "src", "SRC-12", `{"column":"Backlog"}`, nil)
		assert.Equal(t, http.StatusBadRequest, code, "target_project is required")
		code, _ = call(TicketTransfer, http.MethodPost, "src", "SRC-12", `{"target_project":"dst"}`, nil)
		assert.Equal(t, http.StatusBadRequest, code, "column is required")
		code, _ = call(TicketTransfer, http.MethodPost, "src", "SRC-12", `{"target_project":"dst","column":"Nope"}`, nil)
		assert.Equal(t, http.StatusBadRequest, code)
		code, _ = call(TicketTransfer, http.MethodPost, "src", "SRC-12", `{"target_project":"dst","column":"Backlog","colour":"x"}`, nil)
		assert.Equal(t, http.StatusBadRequest, code, "unknown field")
		code, _ = call(TicketTransfer, http.MethodPost, "src", "SRC-12", `{"target_project":"dst","column":"Backlog"}`, &f.src.ID)
		assert.Equal(t, http.StatusForbidden, code, "source-scoped key")
	})

	t.Run("move by key and lane name, then find it by its old key", func(t *testing.T) {
		code, out := call(TicketTransfer, http.MethodPost, "src", "src-12", `{"target_project":"dst","column":"backlog"}`, nil)
		require.Equal(t, http.StatusCreated, code, out)
		assert.Equal(t, "DST-47", out["key"])
		assert.Equal(t, "Backlog", out["column_name"])
		assert.EqualValues(t, card.ID, out["id"])

		code, out = call(TicketGetCard, http.MethodGet, "dst", "SRC-12", "", nil)
		require.Equal(t, http.StatusOK, code, "old key resolves in the new project")
		assert.Equal(t, "DST-47", out["key"])

		code, out = call(TicketGetCard, http.MethodGet, "src", "SRC-12", "", nil)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "DST-47", out["moved_to"])
		assert.Equal(t, "dst", out["project"])

		code, out = call(TicketGetCard, http.MethodGet, "src", "SRC-12", "", &f.src.ID)
		assert.Equal(t, http.StatusNotFound, code)
		assert.Nil(t, out["moved_to"], "a key scoped to the source project learns nothing about the target")
	})
}
