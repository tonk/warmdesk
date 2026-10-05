package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tonk/warmdesk/database"
	"github.com/tonk/warmdesk/middleware"
	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// ticketReportBucket is one row of a breakdown table (by priority, type, …).
type ticketReportBucket struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	Resolved    int    `json:"resolved"`
	Breached    int    `json:"breached"`
	MTTRMinutes *int   `json:"mttr_minutes"`
	mttrSum     int
}

type ticketAgeBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// ticketReport holds the helpdesk KPIs. Rates are percentages (0–100) and
// nil when there is nothing to measure them on.
type ticketReport struct {
	From        string `json:"from"`
	To          string `json:"to"`
	GeneratedAt string `json:"generated_at"`
	CompanyName string `json:"company_name"`
	CompanyLogo string `json:"company_logo"`

	Created        int `json:"created"`
	Resolved       int `json:"resolved"`
	Backlog        int `json:"backlog"`
	MajorIncidents int `json:"major_incidents"`

	MTTRMinutes              *int     `json:"mttr_minutes"`
	MeanFirstResponseMinutes *int     `json:"mean_first_response_minutes"`
	ResponseCompliance       *float64 `json:"response_compliance"`
	ResolutionCompliance     *float64 `json:"resolution_compliance"`
	FCRRate                  *float64 `json:"fcr_rate"`
	ReopenRate               *float64 `json:"reopen_rate"`

	Ageing     []ticketAgeBucket    `json:"ageing"`
	ByPriority []ticketReportBucket `json:"by_priority"`
	ByType     []ticketReportBucket `json:"by_type"`
	ByCategory []ticketReportBucket `json:"by_category"`
	ByCustomer []ticketReportBucket `json:"by_customer"`
}

func pct(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	v := float64(num) * 100 / float64(den)
	return &v
}

func meanMinutes(sum, n int) *int {
	if n == 0 {
		return nil
	}
	v := sum / n
	return &v
}

// ticketReportScope applies the access and query-string filters shared by
// the period population and the backlog. It returns an error status when a
// requested filter is not accessible.
func ticketReportScope(c *gin.Context, q *gorm.DB) (*gorm.DB, int, string) {
	userID := middleware.GetUserID(c)
	role := middleware.GetGlobalRole(c)
	q = q.Where("is_spam = ? OR is_spam IS NULL", false)
	if v := c.Query("customer_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, http.StatusBadRequest, "invalid customer_id"
		}
		if requireCustomerAccess(uint(id), userID, role) != nil {
			return nil, http.StatusForbidden, "forbidden"
		}
		q = q.Where("customer_id = ?", id)
	} else if ids, all := accessibleCustomerIDs(userID, role); !all {
		if len(ids) == 0 {
			q = q.Where("customer_id IS NULL")
		} else {
			q = q.Where("customer_id IS NULL OR customer_id IN ?", ids)
		}
	}
	for _, f := range []struct{ param, col string }{
		{"group_id", "group_id"}, {"assigned_to_id", "assigned_to_id"},
	} {
		if v := c.Query(f.param); v != "" {
			id, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return nil, http.StatusBadRequest, "invalid " + f.param
			}
			q = q.Where(f.col+" = ?", id)
		}
	}
	if v := c.Query("category_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, http.StatusBadRequest, "invalid category_id"
		}
		ids := []uint{uint(id)}
		var children []uint
		database.DB.Model(&models.TicketCategory{}).Where("parent_id = ?", id).Pluck("id", &children)
		q = q.Where("category_id IN ?", append(ids, children...))
	}
	if v := c.Query("type"); v != "" {
		if !services.ValidTicketTypes[v] {
			return nil, http.StatusBadRequest, "invalid type"
		}
		q = q.Where("type = ?", v)
	}
	if v := c.Query("priority"); v != "" {
		if !services.ValidTicketPriorities[v] {
			return nil, http.StatusBadRequest, "invalid priority"
		}
		q = q.Where("priority = ?", v)
	}
	return q, 0, ""
}

// parseReportRange reads ?from=&to= (YYYY-MM-DD, inclusive), defaulting to
// the last 30 days.
func parseReportRange(c *gin.Context, now time.Time) (from, to time.Time, ok bool) {
	to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from = to.AddDate(0, 0, -29)
	if v := c.Query("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return from, to, false
		}
		from = t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return from, to, false
		}
		to = t
	}
	return from, to, !to.Before(from)
}

// agentReplyCounts returns, per ticket, the number of public replies written
// by agents (not inbound email, not customer-portal users).
func agentReplyCounts(ticketIDs []uint) map[uint]int {
	out := map[uint]int{}
	for start := 0; start < len(ticketIDs); start += 500 {
		end := start + 500
		if end > len(ticketIDs) {
			end = len(ticketIDs)
		}
		var rows []struct {
			TicketID uint
			N        int
		}
		database.DB.Table("ticket_messages").
			Select("ticket_messages.ticket_id AS ticket_id, COUNT(*) AS n").
			Joins("JOIN users ON users.id = ticket_messages.user_id").
			Where("ticket_messages.ticket_id IN ?", ticketIDs[start:end]).
			Where("(ticket_messages.is_private = ? OR ticket_messages.is_private IS NULL)", false).
			Where("(ticket_messages.inbound = ? OR ticket_messages.inbound IS NULL)", false).
			Where("(ticket_messages.from_name = '' OR ticket_messages.from_name IS NULL)").
			Where("users.global_role <> ?", "customer").
			Group("ticket_messages.ticket_id").
			Scan(&rows)
		for _, r := range rows {
			out[r.TicketID] = r.N
		}
	}
	return out
}

// assembleTicketReport computes the KPIs. Status is non-zero on error.
func assembleTicketReport(c *gin.Context) (*ticketReport, int, string) {
	now := time.Now()
	from, to, ok := parseReportRange(c, now)
	if !ok {
		return nil, http.StatusBadRequest, "invalid date range"
	}

	q, status, msg := ticketReportScope(c, database.DB.Model(&models.Ticket{}))
	if status != 0 {
		return nil, status, msg
	}
	var tickets []models.Ticket
	q.Where("created_at >= ? AND created_at < ?", from, to.AddDate(0, 0, 1)).
		Preload("Customer").Preload("Category").Find(&tickets)

	settings := loadAllSettings()
	r := &ticketReport{
		From:        from.Format("2006-01-02"),
		To:          to.Format("2006-01-02"),
		GeneratedAt: now.Format("2006-01-02 15:04"),
		CompanyName: settings[settingCompanyName],
		CompanyLogo: settings[settingCompanyLogo],
		Created:     len(tickets),
	}

	ids := make([]uint, len(tickets))
	for i, t := range tickets {
		ids[i] = t.ID
	}
	replies := agentReplyCounts(ids)

	byPriority := map[string]*ticketReportBucket{}
	byType := map[string]*ticketReportBucket{}
	byCategory := map[string]*ticketReportBucket{}
	byCustomer := map[string]*ticketReportBucket{}
	bump := func(m map[string]*ticketReportBucket, key, label string, t *models.Ticket, breached bool) {
		b, ok := m[key]
		if !ok {
			b = &ticketReportBucket{Key: key, Label: label}
			m[key] = b
		}
		b.Count++
		if t.ResolvedAt != nil {
			b.Resolved++
			b.mttrSum += int(t.ResolvedAt.Sub(t.CreatedAt) / time.Minute)
		}
		if breached {
			b.Breached++
		}
	}

	var mttrSum, frSum, frN int
	var respMet, respN, resMet, resN int
	var fcr, everResolved, reopened int
	for i := range tickets {
		t := &tickets[i]
		resp, res := services.EvaluateSlaBreaches(t, now)
		if t.IsMajorIncident {
			r.MajorIncidents++
		}
		if t.ResolvedAt != nil {
			r.Resolved++
			mttrSum += int(t.ResolvedAt.Sub(t.CreatedAt) / time.Minute)
			if t.ReopenCount == 0 && replies[t.ID] <= 1 {
				fcr++
			}
		}
		if t.FirstResponseAt != nil {
			frSum += int(t.FirstResponseAt.Sub(t.CreatedAt) / time.Minute)
			frN++
		}
		if d := t.SlaResponseDeadline; d != nil && (t.FirstResponseAt != nil || t.ResolvedAt != nil || now.After(*d)) {
			respN++
			if !resp {
				respMet++
			}
		}
		if t.SlaResolutionDeadline != nil && (t.ResolvedAt != nil || res) {
			resN++
			if !res {
				resMet++
			}
		}
		if t.ResolvedAt != nil || t.ReopenCount > 0 {
			everResolved++
			if t.ReopenCount > 0 {
				reopened++
			}
		}

		breached := resp || res
		bump(byPriority, t.Priority, t.Priority, t, breached)
		bump(byType, t.Type, t.Type, t, breached)
		catKey, catLabel := "", ""
		if t.Category != nil {
			catKey, catLabel = strconv.FormatUint(uint64(t.Category.ID), 10), t.Category.Name
		}
		bump(byCategory, catKey, catLabel, t, breached)
		custKey, custLabel := "", ""
		if t.Customer != nil {
			custKey, custLabel = strconv.FormatUint(uint64(t.Customer.ID), 10), t.Customer.Name
		}
		bump(byCustomer, custKey, custLabel, t, breached)
	}
	r.MTTRMinutes = meanMinutes(mttrSum, r.Resolved)
	r.MeanFirstResponseMinutes = meanMinutes(frSum, frN)
	r.ResponseCompliance = pct(respMet, respN)
	r.ResolutionCompliance = pct(resMet, resN)
	r.FCRRate = pct(fcr, r.Resolved)
	r.ReopenRate = pct(reopened, everResolved)

	r.ByPriority = orderedBuckets(byPriority, []string{"critical", "high", "medium", "low"})
	r.ByType = orderedBuckets(byType, []string{"incident", "problem", "service_request", "change_request"})
	r.ByCategory = orderedBuckets(byCategory, nil)
	r.ByCustomer = orderedBuckets(byCustomer, nil)

	// Backlog: everything still open now, regardless of the period.
	bq, _, _ := ticketReportScope(c, database.DB.Model(&models.Ticket{}))
	var open []models.Ticket
	bq.Select("id", "created_at").Where("status NOT IN ?", []string{"resolved", "pending_close", "closed"}).Find(&open)
	r.Backlog = len(open)
	ages := []ticketAgeBucket{{Key: "0-1d"}, {Key: "1-3d"}, {Key: "3-7d"}, {Key: "7d+"}}
	for _, t := range open {
		switch age := now.Sub(t.CreatedAt); {
		case age < 24*time.Hour:
			ages[0].Count++
		case age < 72*time.Hour:
			ages[1].Count++
		case age < 7*24*time.Hour:
			ages[2].Count++
		default:
			ages[3].Count++
		}
	}
	r.Ageing = ages
	return r, 0, ""
}

// orderedBuckets turns a bucket map into a slice: keys in order first, then
// the rest by descending count and label.
func orderedBuckets(m map[string]*ticketReportBucket, order []string) []ticketReportBucket {
	out := make([]ticketReportBucket, 0, len(m))
	seen := map[string]bool{}
	for _, k := range order {
		if b, ok := m[k]; ok {
			out = append(out, *b)
			seen[k] = true
		}
	}
	var rest []ticketReportBucket
	for k, b := range m {
		if !seen[k] {
			rest = append(rest, *b)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].Count != rest[j].Count {
			return rest[i].Count > rest[j].Count
		}
		return rest[i].Label < rest[j].Label
	})
	out = append(out, rest...)
	for i := range out {
		out[i].MTTRMinutes = meanMinutes(out[i].mttrSum, out[i].Resolved)
	}
	return out
}

// GetTicketReport GET /api/v1/helpdesk/reports/tickets
func GetTicketReport(c *gin.Context) {
	if requireNotCustomerRole(middleware.GetGlobalRole(c)) != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	r, status, msg := assembleTicketReport(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, r)
}
