package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"gorm.io/gorm"
)

var validArticleStatuses = map[string]bool{"draft": true, "published": true, "archived": true}
var validArticleVisibility = map[string]bool{"internal": true, "public": true}

// canReadArticle: agents read everything (drafts included); customer-portal
// users only published public articles, global or for a customer they belong to.
func canReadArticle(a *models.KnowledgeArticle, userID uint, role string) bool {
	if role != "customer" {
		return true
	}
	if a.Status != "published" || a.Visibility != "public" {
		return false
	}
	return canSeeCustomerScoped(a.CustomerID, userID, role, true)
}

// articleScope restricts an article query to what the caller may read.
func articleScope(q *gorm.DB, userID uint, role string) *gorm.DB {
	if role != "customer" {
		return q
	}
	q = q.Where("status = ? AND visibility = ?", "published", "public")
	ids, _ := accessibleCustomerIDs(userID, role)
	if len(ids) == 0 {
		return q.Where("customer_id IS NULL")
	}
	return q.Where("customer_id IS NULL OR customer_id IN ?", ids)
}

// searchTerms splits a query into lower-case words of at least 2 characters.
func searchTerms(q string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len([]rune(w)) >= 2 {
			out = append(out, w)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// scoreArticle ranks an article for the search terms: title matches weigh
// most, then keywords, summary and body.
func scoreArticle(a *models.KnowledgeArticle, terms []string) int {
	title, kw := strings.ToLower(a.Title), strings.ToLower(a.Keywords)
	summary, body := strings.ToLower(a.Summary), strings.ToLower(a.Body)
	score := 0
	for _, t := range terms {
		switch {
		case strings.Contains(title, t):
			score += 6
		case strings.Contains(kw, t):
			score += 4
		case strings.Contains(summary, t):
			score += 2
		case strings.Contains(body, t):
			score++
		}
	}
	return score
}

// searchArticles returns readable articles matching the terms (any term
// matching), best first, at most limit.
func searchArticles(q *gorm.DB, terms []string, limit int) []models.KnowledgeArticle {
	if len(terms) > 0 {
		var conds []string
		var args []any
		for _, t := range terms {
			like := "%" + t + "%"
			conds = append(conds, "(LOWER(title) LIKE ? OR LOWER(keywords) LIKE ? OR LOWER(summary) LIKE ? OR LOWER(body) LIKE ?)")
			args = append(args, like, like, like, like)
		}
		q = q.Where(strings.Join(conds, " OR "), args...)
	}
	var arts []models.KnowledgeArticle
	q.Preload("Category").Preload("Customer").Order("updated_at desc").Limit(500).Find(&arts)
	if len(terms) > 0 {
		scores := make(map[uint]int, len(arts))
		for i := range arts {
			scores[arts[i].ID] = scoreArticle(&arts[i], terms)
		}
		sort.SliceStable(arts, func(i, j int) bool { return scores[arts[i].ID] > scores[arts[j].ID] })
	}
	if limit > 0 && len(arts) > limit {
		arts = arts[:limit]
	}
	for i := range arts {
		arts[i].Body = "" // list responses carry the summary only
	}
	return arts
}

// ListKnowledgeArticles GET /api/v1/kb/articles?q=&status=&visibility=&category_id=&customer_id=
func ListKnowledgeArticles(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	q := articleScope(database.DB.Model(&models.KnowledgeArticle{}), userID, role)
	if v := c.Query("status"); v != "" && validArticleStatuses[v] {
		q = q.Where("status = ?", v)
	} else if role != "customer" && c.Query("include_archived") != "true" {
		q = q.Where("status <> ?", "archived")
	}
	if v := c.Query("visibility"); v != "" && validArticleVisibility[v] {
		q = q.Where("visibility = ?", v)
	}
	if v := c.Query("category_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			var children []uint
			database.DB.Model(&models.TicketCategory{}).Where("parent_id = ?", id).Pluck("id", &children)
			q = q.Where("category_id IN ?", append(children, uint(id)))
		}
	}
	if v := c.Query("customer_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("customer_id IS NULL OR customer_id = ?", id)
		}
	}
	c.JSON(http.StatusOK, searchArticles(q, searchTerms(c.Query("q")), 0))
}

// SuggestKnowledgeArticles GET /api/v1/kb/suggest?q=…&customer_id=…&limit=5 —
// published articles matching free text (e.g. a ticket title), for "related
// articles" panels. Agents also get internal articles.
func SuggestKnowledgeArticles(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	terms := searchTerms(c.Query("q"))
	if len(terms) == 0 {
		c.JSON(http.StatusOK, []models.KnowledgeArticle{})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 || limit > 20 {
		limit = 5
	}
	q := articleScope(database.DB.Model(&models.KnowledgeArticle{}), userID, role).Where("status = ?", "published")
	if v := c.Query("customer_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			q = q.Where("customer_id IS NULL OR customer_id = ?", id)
		}
	}
	c.JSON(http.StatusOK, searchArticles(q, terms, limit))
}

// GetKnowledgeArticle GET /api/v1/kb/articles/:id — counts a view unless count_view=false.
func GetKnowledgeArticle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var a models.KnowledgeArticle
	if database.DB.Preload("Author").Preload("UpdatedBy").Preload("Category").Preload("Customer").First(&a, id).Error != nil ||
		!canReadArticle(&a, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	// ?count_view=false (the edit form) reads without counting a view.
	if c.Query("count_view") != "false" {
		database.DB.Model(&models.KnowledgeArticle{}).Where("id = ?", a.ID).UpdateColumn("views", gorm.Expr("views + 1"))
		a.Views++
	}
	c.JSON(http.StatusOK, a)
}

type articleRequest struct {
	Title      *string `json:"title"`
	Summary    *string `json:"summary"`
	Body       *string `json:"body"`
	Status     *string `json:"status"`
	Visibility *string `json:"visibility"`
	CustomerID *uint   `json:"customer_id"`
	CategoryID *uint   `json:"category_id"`
	Keywords   *string `json:"keywords"`
	// TicketID links the new article to the ticket it was written from.
	TicketID *uint `json:"ticket_id"`
}

func (r *articleRequest) updates(a *models.KnowledgeArticle) (map[string]any, string) {
	u := map[string]any{}
	if r.Title != nil {
		t := strings.TrimSpace(*r.Title)
		if t == "" {
			return nil, "title is required"
		}
		u["title"] = t
	}
	if r.Summary != nil {
		u["summary"] = strings.TrimSpace(*r.Summary)
	}
	if r.Body != nil {
		u["body"] = *r.Body
	}
	if r.Keywords != nil {
		u["keywords"] = strings.TrimSpace(*r.Keywords)
	}
	if r.Status != nil {
		if !validArticleStatuses[*r.Status] {
			return nil, "invalid status"
		}
		u["status"] = *r.Status
		if *r.Status == "published" && a.PublishedAt == nil {
			u["published_at"] = time.Now()
		}
	}
	if r.Visibility != nil {
		if !validArticleVisibility[*r.Visibility] {
			return nil, "invalid visibility"
		}
		u["visibility"] = *r.Visibility
	}
	for _, ch := range []struct {
		id    *uint
		col   string
		model any
	}{
		{r.CustomerID, "customer_id", &models.Customer{}},
		{r.CategoryID, "category_id", &models.TicketCategory{}},
	} {
		if ch.id == nil {
			continue
		}
		if *ch.id != 0 && database.DB.First(ch.model, *ch.id).Error != nil {
			return nil, ch.col + " not found"
		}
		u[ch.col] = nullableID(ch.id)
	}
	return u, ""
}

// CreateKnowledgeArticle POST /api/v1/kb/articles (agents)
func CreateKnowledgeArticle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if requireNotCustomerRole(role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req articleRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	a := models.KnowledgeArticle{Status: "draft", Visibility: "internal", AuthorID: userID, UpdatedByID: userID}
	u, msg := req.updates(&a)
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if req.CustomerID != nil && *req.CustomerID != 0 && requireCustomerAccess(*req.CustomerID, userID, role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to that customer"})
		return
	}
	a.Title = u["title"].(string)
	if err := database.DB.Create(&a).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create article"})
		return
	}
	database.DB.Model(&a).Updates(u)
	if req.TicketID != nil && *req.TicketID != 0 {
		var t models.Ticket
		if database.DB.Select("id", "customer_id", "type").First(&t, *req.TicketID).Error == nil && canSeeTicket(&t, userID, role) {
			database.DB.Create(&models.TicketArticleLink{TicketID: t.ID, ArticleID: a.ID, CreatedByID: userID})
		}
	}
	database.DB.Preload("Author").Preload("Category").Preload("Customer").First(&a, a.ID)
	c.JSON(http.StatusCreated, a)
}

// UpdateKnowledgeArticle PUT /api/v1/kb/articles/:id (agents)
func UpdateKnowledgeArticle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	if requireNotCustomerRole(role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var a models.KnowledgeArticle
	if database.DB.First(&a, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	var req articleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	u, msg := req.updates(&a)
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if req.CustomerID != nil && *req.CustomerID != 0 && requireCustomerAccess(*req.CustomerID, userID, role) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to that customer"})
		return
	}
	u["updated_by_id"] = userID
	database.DB.Model(&a).Updates(u)
	database.DB.Preload("Author").Preload("UpdatedBy").Preload("Category").Preload("Customer").First(&a, a.ID)
	c.JSON(http.StatusOK, a)
}

// DeleteKnowledgeArticle DELETE /api/v1/kb/articles/:id — the author or an admin.
func DeleteKnowledgeArticle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var a models.KnowledgeArticle
	if database.DB.First(&a, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	if role != "admin" && a.AuthorID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the author or an admin can delete an article; archive it instead"})
		return
	}
	database.DB.Where("article_id = ?", a.ID).Delete(&models.TicketArticleLink{})
	database.DB.Delete(&a)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// KnowledgeArticleFeedback POST /api/v1/kb/articles/:id/feedback {"helpful": true}
func KnowledgeArticleFeedback(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Helpful *bool `json:"helpful"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Helpful == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "helpful is required"})
		return
	}
	var a models.KnowledgeArticle
	if database.DB.First(&a, id).Error != nil || !canReadArticle(&a, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	col := "helpful_no"
	if *req.Helpful {
		col = "helpful_yes"
	}
	database.DB.Model(&models.KnowledgeArticle{}).Where("id = ?", a.ID).UpdateColumn(col, gorm.Expr(col+" + 1"))
	database.DB.First(&a, a.ID)
	c.JSON(http.StatusOK, gin.H{"helpful_yes": a.HelpfulYes, "helpful_no": a.HelpfulNo})
}

// ListTicketArticles GET /customers/:customerId/tickets/:ticketId/articles
func ListTicketArticles(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, false)
	if !ok {
		return
	}
	var ids []uint
	database.DB.Model(&models.TicketArticleLink{}).Where("ticket_id = ?", t.ID).Pluck("article_id", &ids)
	out := []models.KnowledgeArticle{}
	if len(ids) > 0 {
		articleScope(database.DB.Where("id IN ?", ids), userID, role).Order("title asc").Find(&out)
		for i := range out {
			out[i].Body = ""
		}
	}
	c.JSON(http.StatusOK, out)
}

// LinkTicketArticle POST /customers/:customerId/tickets/:ticketId/articles {"article_id": 4}
func LinkTicketArticle(c *gin.Context) {
	t, userID, role, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	var req struct {
		ArticleID uint `json:"article_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "article_id is required"})
		return
	}
	var a models.KnowledgeArticle
	if database.DB.First(&a, req.ArticleID).Error != nil || !canReadArticle(&a, userID, role) {
		c.JSON(http.StatusNotFound, gin.H{"error": "article not found"})
		return
	}
	var n int64
	database.DB.Model(&models.TicketArticleLink{}).Where("ticket_id = ? AND article_id = ?", t.ID, a.ID).Count(&n)
	if n == 0 {
		database.DB.Create(&models.TicketArticleLink{TicketID: t.ID, ArticleID: a.ID, CreatedByID: userID})
		database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "article_linked", Detail: a.Title})
	}
	a.Body = ""
	c.JSON(http.StatusCreated, a)
}

// UnlinkTicketArticle DELETE /customers/:customerId/tickets/:ticketId/articles/:articleId
func UnlinkTicketArticle(c *gin.Context) {
	t, userID, _, ok := loadCustomerTicket(c, true)
	if !ok {
		return
	}
	aid, ok := paramID(c, "articleId")
	if !ok {
		return
	}
	database.DB.Where("ticket_id = ? AND article_id = ?", t.ID, aid).Delete(&models.TicketArticleLink{})
	database.DB.Create(&models.TicketHistory{TicketID: t.ID, UserID: userID, EventType: "article_unlinked"})
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}
