package main

import (
	"fmt"
	"time"

	"github.com/tonk/warmdesk/models"
	"github.com/tonk/warmdesk/services"
	"gorm.io/gorm"
)

// Demo data for the ITIL features: ticket categories, knowledge base, CMDB,
// problems, the service catalogue with approvals, and changes. Everything is
// matched by name in removeITSMDemoData so --reset re-seeds cleanly.

var (
	seedCategoryNames = []string{"Access & accounts", "Network", "VPN", "Applications", "Infrastructure"}
	seedArticleTitles = []string{
		"How to reset your WarmDesk password",
		"Connecting to the company VPN",
		"Requesting access to an Acme project",
		"Known error: Safari 17 login returns 500",
		"Runbook: draining a Kubernetes node",
		"Rotating API keys without downtime",
	}
	seedCINames = []string{
		"Acme Web Portal", "Acme API Gateway", "acme-web-01", "acme-web-02", "acme-db-01",
		"Globex Kubernetes cluster", "k8s-node-01", "k8s-node-02", "k8s-node-03", "Globex Monitoring",
		"WarmDesk SaaS platform", "Core network", "Backup storage",
	}
	seedCatalogNames  = []string{"New laptop", "Software licence", "Access to a project"}
	seedITSMGroups    = []string{"Change Advisory Board", "Service Approvers"}
	seedFreezeNames   = []string{"Year-end change freeze"}
	seedCITypeDefault = []struct{ name, icon string }{
		{"Service", "🧩"}, {"Application", "📦"}, {"Server", "🖥"}, {"Database", "🗄"},
		{"Network device", "🌐"}, {"Workstation", "💻"}, {"Storage", "💾"},
	}
)

// seedITSM creates the ITIL demo data. customerIDs is ordered Acme, Globex,
// Initech; tickets maps the seeded customer tickets by title.
func seedITSM(db *gorm.DB, users map[string]*models.User, customerIDs []uint, tickets map[string]*models.Ticket) {
	now := time.Now()
	acme, globex, initech := customerIDs[0], customerIDs[1], customerIDs[2]
	admin := users["admin"]
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	day := 24 * time.Hour
	history := func(t *models.Ticket, userKey, event, detail string, at time.Time) {
		must(db.Create(&models.TicketHistory{TicketID: t.ID, UserID: users[userKey].ID, EventType: event, Detail: detail, CreatedAt: at}).Error)
	}

	// ── Categories and resolution codes ───────────────────────────────────────
	fmt.Println("→ Creating ticket categories…")
	services.SeedResolutionCodes()
	cats := map[string]*models.TicketCategory{}
	mkCat := func(name string, parent *models.TicketCategory, order int) {
		c := &models.TicketCategory{Name: name, IsActive: true, SortOrder: order}
		if parent != nil {
			c.ParentID = &parent.ID
		}
		must(db.Create(c).Error)
		cats[name] = c
	}
	mkCat("Access & accounts", nil, 0)
	mkCat("Network", nil, 1)
	mkCat("VPN", cats["Network"], 0)
	mkCat("Applications", nil, 2)
	mkCat("Infrastructure", nil, 3)
	for title, cat := range map[string]string{
		"Login page returns 500 on Safari":               "Applications",
		"Rate limiting causing 429 errors on API":        "Applications",
		"Kubernetes cluster node drain failing":          "Infrastructure",
		"SSO integration — SAML metadata endpoint":       "Access & accounts",
		"Request: Read-only dashboard access for intern": "Access & accounts",
	} {
		if t, ok := tickets[title]; ok {
			must(db.Model(t).Update("category_id", cats[cat].ID).Error)
		}
	}

	// ── Knowledge base ────────────────────────────────────────────────────────
	fmt.Println("→ Creating knowledge base articles…")
	type articleSeed struct {
		title, summary, body, keywords, status, visibility, category, author string
		customer                                                             *uint
		views, yes, no                                                       int
		ageDays                                                              int
	}
	articleSeeds := []articleSeed{
		{
			title: "How to reset your WarmDesk password", summary: "Reset a forgotten password from the login page in two minutes.",
			body:     "## Steps\n\n1. Open the login page and click **Forgot password?**\n2. Enter the email address of your account.\n3. Open the email you receive and click the reset link (valid for one hour).\n4. Choose a new password of at least 12 characters.\n\n> Didn't get the email? Check your spam directory, or ask your administrator to resend it.",
			keywords: "password reset forgot login", status: "published", visibility: "public", category: "Access & accounts", author: "admin",
			views: 214, yes: 37, no: 2, ageDays: 120,
		},
		{
			title: "Connecting to the company VPN", summary: "Install the VPN client and connect with your company account.",
			body:     "## Install\n\n- **Windows / macOS:** download the client from the self-service portal.\n- **Linux:** install `openconnect` from your distribution.\n\n## Connect\n\nServer: `vpn.example.com` — sign in with your company account and approve the MFA prompt.\n\n## Troubleshooting\n\n- *Authentication failed*: your password may have expired; reset it first.\n- *Connected but no access*: disconnect, wait 30 seconds and reconnect.",
			keywords: "vpn remote openconnect", status: "published", visibility: "public", category: "VPN", author: "lisa",
			views: 98, yes: 15, no: 3, ageDays: 60,
		},
		{
			title: "Requesting access to an Acme project", summary: "Ask for access to an Acme board through the service catalogue.",
			body:     "Access to Acme project boards is requested through the **Service catalogue → Access to a project** form.\n\nYour Acme contact approves the request; the service desk then adds you to the board. Expect access within one working day.",
			keywords: "access project board permission", status: "published", visibility: "public", category: "Access & accounts", author: "sarah",
			customer: &acme, views: 31, yes: 6, ageDays: 20,
		},
		{
			title: "Known error: Safari 17 login returns 500", summary: "Internal: root cause and workaround for the Safari login failure.",
			body:     "## Symptom\n\nUsers on Safari 17 get *500 Internal Server Error* right after submitting the login form.\n\n## Cause\n\nSafari 17 rejects the session cookie because its `SameSite` attribute is sent without `Secure` behind the old load balancer.\n\n## Workaround\n\nAsk the user to sign in with another browser, or to disable *Prevent cross-site tracking* temporarily.\n\n## Fix\n\nTracked in problem record *Safari 17 rejects the session cookie on login*.",
			keywords: "safari login 500 cookie samesite", status: "published", visibility: "internal", category: "Applications", author: "marc",
			views: 44, yes: 9, ageDays: 6,
		},
		{
			title: "Runbook: draining a Kubernetes node", summary: "Internal: safely drain a node for maintenance.",
			body:     "1. `kubectl cordon <node>`\n2. Check pod disruption budgets: `kubectl get pdb -A` — a budget with *allowed disruptions = 0* blocks the drain.\n3. `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data --timeout=10m`\n4. Do the maintenance, then `kubectl uncordon <node>`.\n\nIf the drain hangs, see problem *Pod disruption budgets block node drains*.",
			keywords: "kubernetes k8s drain node maintenance pdb", status: "published", visibility: "internal", category: "Infrastructure", author: "raj",
			views: 57, yes: 11, no: 1, ageDays: 45,
		},
		{
			title: "Rotating API keys without downtime", summary: "Draft: overlap old and new keys during rotation.",
			body:     "_Draft — to be reviewed by the platform team._\n\nCreate the new key, deploy it to all consumers, verify traffic, then revoke the old key.",
			keywords: "api key rotation", status: "draft", visibility: "internal", category: "Applications", author: "james",
			ageDays: 2,
		},
	}
	articles := map[string]*models.KnowledgeArticle{}
	for _, as := range articleSeeds {
		created := now.Add(-time.Duration(as.ageDays) * day)
		a := &models.KnowledgeArticle{
			Title: as.title, Summary: as.summary, Body: as.body, Keywords: as.keywords,
			Status: as.status, Visibility: as.visibility, CustomerID: as.customer,
			CategoryID: &cats[as.category].ID, AuthorID: users[as.author].ID, UpdatedByID: users[as.author].ID,
			Views: as.views, HelpfulYes: as.yes, HelpfulNo: as.no, CreatedAt: created, UpdatedAt: created,
		}
		if as.status == "published" {
			a.PublishedAt = &created
		}
		must(db.Create(a).Error)
		articles[as.title] = a
	}
	fmt.Printf("   Created %d knowledge base articles\n", len(articleSeeds))

	// ── CMDB ──────────────────────────────────────────────────────────────────
	fmt.Println("→ Creating CMDB configuration items…")
	ciTypes := map[string]uint{}
	for i, ts := range seedCITypeDefault {
		var t models.CIType
		if db.Where("name = ?", ts.name).First(&t).Error != nil {
			t = models.CIType{Name: ts.name, Icon: ts.icon, SortOrder: i}
			must(db.Create(&t).Error)
		}
		ciTypes[ts.name] = t.ID
	}
	groupID := func(name string) *uint {
		var g models.UserGroup
		if db.Where("name = ?", name).First(&g).Error != nil {
			return nil
		}
		return &g.ID
	}
	devops := groupID("DevOps Team")
	frontend := groupID("Frontend Team")
	type ciSeed struct {
		name, typ, status, env, crit, desc string
		customer                           *uint
		owner                              string
		group                              *uint
		attrs                              models.CIAttributes
	}
	ciSeeds := []ciSeed{
		{"Acme Web Portal", "Service", "active", "production", "high", "Customer-facing web portal of Acme Corporation.", &acme, "sarah", frontend, models.CIAttributes{"url": "https://portal.acme.example", "sla": "Critical"}},
		{"Acme API Gateway", "Application", "active", "production", "high", "Public REST API with rate limiting in front of the portal back end.", &acme, "marc", devops, models.CIAttributes{"version": "3.8.2", "port": "443"}},
		{"acme-web-01", "Server", "active", "production", "medium", "Application server, availability zone A.", &acme, "lisa", devops, models.CIAttributes{"os": "Debian 12", "ip": "10.20.1.11", "cpu": "4 vCPU", "memory": "16 GB"}},
		{"acme-web-02", "Server", "active", "production", "medium", "Application server, availability zone B.", &acme, "lisa", devops, models.CIAttributes{"os": "Debian 12", "ip": "10.20.1.12", "cpu": "4 vCPU", "memory": "16 GB"}},
		{"acme-db-01", "Database", "active", "production", "high", "Primary PostgreSQL database of the portal.", &acme, "raj", devops, models.CIAttributes{"engine": "PostgreSQL 16", "ip": "10.20.2.21", "backup": "nightly"}},
		{"Globex Kubernetes cluster", "Service", "active", "production", "high", "Managed Kubernetes cluster running all Globex workloads.", &globex, "raj", devops, models.CIAttributes{"version": "1.30", "nodes": "3"}},
		{"k8s-node-01", "Server", "active", "production", "medium", "Worker node.", &globex, "raj", devops, models.CIAttributes{"os": "Ubuntu 24.04", "ip": "10.30.0.11"}},
		{"k8s-node-02", "Server", "maintenance", "production", "medium", "Worker node — cordoned for kernel update.", &globex, "raj", devops, models.CIAttributes{"os": "Ubuntu 24.04", "ip": "10.30.0.12"}},
		{"k8s-node-03", "Server", "active", "production", "medium", "Worker node.", &globex, "raj", devops, models.CIAttributes{"os": "Ubuntu 24.04", "ip": "10.30.0.13"}},
		{"Globex Monitoring", "Application", "active", "production", "medium", "Prometheus and Grafana stack for Globex.", &globex, "lisa", devops, models.CIAttributes{"url": "https://grafana.globex.example"}},
		{"WarmDesk SaaS platform", "Service", "active", "production", "high", "The hosted WarmDesk service itself.", nil, "admin", devops, models.CIAttributes{"region": "eu-west"}},
		{"Core network", "Network device", "active", "production", "high", "Data-centre core switches and firewalls.", nil, "lisa", devops, models.CIAttributes{"vendor": "Juniper"}},
		{"Backup storage", "Storage", "active", "production", "medium", "Off-site object storage for backups.", nil, "raj", devops, models.CIAttributes{"capacity": "40 TB"}},
	}
	cis := map[string]*models.ConfigurationItem{}
	for _, cs := range ciSeeds {
		ci := &models.ConfigurationItem{
			Name: cs.name, TypeID: ciTypes[cs.typ], CustomerID: cs.customer, Status: cs.status,
			Environment: cs.env, Criticality: cs.crit, Description: cs.desc, Attributes: cs.attrs,
			OwnerID: &users[cs.owner].ID, GroupID: cs.group,
		}
		must(db.Create(ci).Error)
		cis[cs.name] = ci
	}
	relations := []struct{ src, typ, dst string }{
		{"Acme Web Portal", "depends_on", "Acme API Gateway"},
		{"Acme API Gateway", "runs_on", "acme-web-01"},
		{"Acme API Gateway", "runs_on", "acme-web-02"},
		{"Acme API Gateway", "depends_on", "acme-db-01"},
		{"acme-web-01", "connected_to", "Core network"},
		{"acme-web-02", "connected_to", "Core network"},
		{"acme-db-01", "depends_on", "Backup storage"},
		{"k8s-node-01", "part_of", "Globex Kubernetes cluster"},
		{"k8s-node-02", "part_of", "Globex Kubernetes cluster"},
		{"k8s-node-03", "part_of", "Globex Kubernetes cluster"},
		{"Globex Monitoring", "runs_on", "Globex Kubernetes cluster"},
		{"Globex Kubernetes cluster", "depends_on", "Core network"},
		{"WarmDesk SaaS platform", "depends_on", "Core network"},
		{"WarmDesk SaaS platform", "depends_on", "Backup storage"},
	}
	for _, r := range relations {
		must(db.Create(&models.CIRelation{SourceID: cis[r.src].ID, TargetID: cis[r.dst].ID, Type: r.typ}).Error)
	}
	linkCI := func(t *models.Ticket, ci string) {
		must(db.Create(&models.TicketCI{TicketID: t.ID, CIID: cis[ci].ID, CreatedByID: admin.ID}).Error)
	}
	for title, ci := range map[string]string{
		"Login page returns 500 on Safari":        "Acme Web Portal",
		"Rate limiting causing 429 errors on API": "Acme API Gateway",
		"Kubernetes cluster node drain failing":   "k8s-node-02",
	} {
		if t, ok := tickets[title]; ok {
			linkCI(t, ci)
		}
	}
	fmt.Printf("   Created %d configuration items with %d relations\n", len(ciSeeds), len(relations))

	// ── Problems ──────────────────────────────────────────────────────────────
	fmt.Println("→ Creating problem records…")
	// Older demo tickets of type "problem" predate problem management; give
	// them a problem status so they don't show up as "not set".
	must(db.Model(&models.Ticket{}).Where("type = ? AND (problem_status = '' OR problem_status IS NULL)", "problem").
		Update("problem_status", "investigating").Error)
	type problemSeed struct {
		customer                                   uint
		title, desc, status, rootCause, workaround string
		problemStatus, priority, owner             string
		knownError                                 bool
		incident, ci, article                      string
		ageDays                                    int
	}
	problemSeeds := []problemSeed{
		{
			customer: acme, title: "Safari 17 rejects the session cookie on login", priority: "high", owner: "marc", ageDays: 5,
			desc:   "Several incidents report a 500 error on login from Safari 17. Investigating the common cause.",
			status: "in_progress", problemStatus: "known_error", knownError: true,
			rootCause:  "The old load balancer strips the `Secure` flag from the session cookie while `SameSite=None` is set; Safari 17 rejects such cookies, so the login handler finds no session and fails.",
			workaround: "Sign in with another browser, or disable *Prevent cross-site tracking* in Safari settings until the load balancer is replaced.",
			incident:   "Login page returns 500 on Safari", ci: "Acme Web Portal", article: "Known error: Safari 17 login returns 500",
		},
		{
			customer: acme, title: "API rate limiter counts retries as new requests", priority: "medium", owner: "marc", ageDays: 3,
			desc:   "Clients that retry on timeouts hit the rate limit much faster than expected.",
			status: "open", problemStatus: "root_cause_identified",
			rootCause:  "The gateway's limiter keys on connection instead of API key, so each client retry opens a new bucket entry and is counted twice.",
			workaround: "Raise the client retry back-off to at least 2 seconds.",
			incident:   "Rate limiting causing 429 errors on API", ci: "Acme API Gateway",
		},
		{
			customer: globex, title: "Pod disruption budgets block node drains", priority: "medium", owner: "raj", ageDays: 2,
			desc:   "Node drains hang during maintenance windows.",
			status: "open", problemStatus: "investigating",
			workaround: "Temporarily scale the affected deployment to two replicas before draining.",
			incident:   "Kubernetes cluster node drain failing", ci: "Globex Kubernetes cluster", article: "Runbook: draining a Kubernetes node",
		},
	}
	for _, ps := range problemSeeds {
		cid := ps.customer
		created := now.Add(-time.Duration(ps.ageDays) * day)
		p := &models.Ticket{
			CustomerID: &cid, Title: ps.title, Description: ps.desc, Type: "problem", Status: ps.status,
			Priority: ps.priority, CreatedByID: users[ps.owner].ID, OwnerID: &users[ps.owner].ID,
			AssignedToID: &users[ps.owner].ID, GroupID: devops,
			ProblemStatus: ps.problemStatus, RootCause: ps.rootCause, Workaround: ps.workaround,
			IsKnownError: ps.knownError, CreatedAt: created, UpdatedAt: created,
		}
		category := cats["Infrastructure"]
		if ps.customer == acme {
			category = cats["Applications"]
		}
		p.CategoryID = &category.ID
		must(db.Create(p).Error)
		history(p, ps.owner, "created", "", created)
		if inc, ok := tickets[ps.incident]; ok {
			must(db.Create(&models.TicketLink{SourceTicketID: inc.ID, TargetTicketID: p.ID, LinkType: "caused_by"}).Error)
			history(inc, ps.owner, "ticket_linked", fmt.Sprintf("caused_by #%d %s", p.ID, p.Title), created.Add(time.Hour))
		}
		linkCI(p, ps.ci)
		if ps.article != "" {
			must(db.Create(&models.TicketArticleLink{TicketID: p.ID, ArticleID: articles[ps.article].ID, CreatedByID: users[ps.owner].ID}).Error)
		}
	}
	if t, ok := tickets["Login page returns 500 on Safari"]; ok {
		must(db.Create(&models.TicketArticleLink{TicketID: t.ID, ArticleID: articles["Known error: Safari 17 login returns 500"].ID, CreatedByID: users["marc"].ID}).Error)
	}
	fmt.Printf("   Created %d problems, linked to their incidents\n", len(problemSeeds))

	// ── Approver groups and change settings ───────────────────────────────────
	fmt.Println("→ Creating approver groups, service catalogue and requests…")
	mkGroup := func(name, desc string, members ...string) uint {
		g := &models.UserGroup{Name: name, Description: desc}
		must(db.Create(g).Error)
		for _, m := range members {
			must(db.Create(&models.GroupMember{GroupID: g.ID, UserID: users[m].ID}).Error)
		}
		return g.ID
	}
	cab := mkGroup("Change Advisory Board", "Approves normal and emergency changes.", "admin", "marc", "lisa")
	approvers := mkGroup("Service Approvers", "Approves service requests that cost money.", "admin", "sarah")
	seedSetting(db, services.SettingChangeCABGroup, fmt.Sprint(cab))
	seedSetting(db, services.SettingChangeApprovals, "2")
	seedSetting(db, services.SettingChangeEmergencyApprovals, "1")

	// ── Service catalogue ─────────────────────────────────────────────────────
	laptop := &models.ServiceCatalogItem{
		Name: "New laptop", Icon: "💻", DefaultPriority: "low", IsActive: true, SortOrder: 0,
		Description: "Order a laptop for a new or existing employee. Needs approval from a service approver.",
		GroupID:     devops, ApprovalGroupID: &approvers, ApprovalsRequired: 1, CategoryID: &cats["Access & accounts"].ID,
		Fields: models.CatalogFields{
			{Key: "employee", Label: "Employee", Type: "text", Required: true},
			{Key: "model", Label: "Model", Type: "select", Required: true, Options: []string{"13-inch ultrabook", "15-inch workstation"}},
			{Key: "needed_by", Label: "Needed by", Type: "date"},
			{Key: "docking_station", Label: "Docking station", Type: "checkbox"},
		},
	}
	licence := &models.ServiceCatalogItem{
		Name: "Software licence", Icon: "🔑", DefaultPriority: "low", IsActive: true, SortOrder: 1,
		Description: "Request a paid software licence.",
		GroupID:     devops, ApprovalGroupID: &approvers, ApprovalsRequired: 1,
		Fields: models.CatalogFields{
			{Key: "product", Label: "Product", Type: "text", Required: true},
			{Key: "seats", Label: "Number of seats", Type: "number", Required: true},
			{Key: "justification", Label: "Business justification", Type: "textarea", Required: true},
		},
	}
	access := &models.ServiceCatalogItem{
		Name: "Access to a project", Icon: "🔓", DefaultPriority: "medium", IsActive: true, SortOrder: 2,
		Description: "Get access to a project board. No approval needed.",
		GroupID:     frontend, CategoryID: &cats["Access & accounts"].ID,
		Fields: models.CatalogFields{
			{Key: "project", Label: "Project", Type: "text", Required: true},
			{Key: "role", Label: "Role", Type: "select", Required: true, Options: []string{"Viewer", "Member"}},
		},
	}
	for _, it := range []*models.ServiceCatalogItem{laptop, licence, access} {
		must(db.Create(it).Error)
	}

	// Service requests in each approval state.
	type requestSeed struct {
		customer            uint
		item                *models.ServiceCatalogItem
		title, requester    string
		answers             models.RequestData
		approval, decidedBy string
		comment             string
		status              string
		ageHours            int
	}
	requestSeeds := []requestSeed{
		{acme, laptop, "New laptop for Jane Doe", "cust1", models.RequestData{"employee": "Jane Doe", "model": "13-inch ultrabook", "needed_by": now.Add(10 * day).Format("2006-01-02"), "docking_station": true}, services.ApprovalPending, "", "", "new", 5},
		{globex, licence, "Software licence: Figma (5 seats)", "cust2", models.RequestData{"product": "Figma Professional", "seats": 5.0, "justification": "Design team grows from 3 to 8 people next month."}, services.ApprovalApproved, "sarah", "Within budget.", "in_progress", 30},
		{initech, laptop, "New laptop for the CEO", "sarah", models.RequestData{"employee": "Bill Lumbergh", "model": "15-inch workstation", "docking_station": false}, services.ApprovalRejected, "admin", "Replacement cycle is in Q1 — please request again then.", "open", 52},
		{acme, access, "Access to Website Redesign board", "cust1", models.RequestData{"project": "Website Redesign", "role": "Viewer"}, "", "", "", "resolved", 75},
	}
	for _, rs := range requestSeeds {
		cid := rs.customer
		created := now.Add(-time.Duration(rs.ageHours) * time.Hour)
		desc := ""
		for _, f := range rs.item.Fields {
			if v, ok := rs.answers[f.Key]; ok {
				val := fmt.Sprint(v)
				if b, isBool := v.(bool); isBool {
					val = map[bool]string{true: "yes", false: "no"}[b]
				}
				desc += fmt.Sprintf("- **%s:** %s\n", f.Label, val)
			}
		}
		t := &models.Ticket{
			CustomerID: &cid, Title: rs.title, Description: desc, Type: "service_request", Status: rs.status,
			Priority: rs.item.DefaultPriority, CreatedByID: users[rs.requester].ID, GroupID: rs.item.GroupID,
			CatalogItemID: &rs.item.ID, RequestData: rs.answers, CategoryID: rs.item.CategoryID,
			CreatedAt: created, UpdatedAt: created,
		}
		if rs.approval != "" {
			t.ApprovalStatus, t.ApprovalGroupID, t.ApprovalsRequired, t.ApprovalRound = rs.approval, rs.item.ApprovalGroupID, rs.item.ApprovalsRequired, 1
		}
		if rs.status == "resolved" {
			t.ResolvedAt = ago(time.Duration(rs.ageHours-6) * time.Hour)
			t.FirstResponseAt = ago(time.Duration(rs.ageHours-1) * time.Hour)
		}
		must(db.Create(t).Error)
		history(t, rs.requester, "created", "service catalogue: "+rs.item.Name, created)
		if rs.approval != "" {
			history(t, rs.requester, "approval_requested", "", created)
		}
		if rs.decidedBy != "" {
			decision := services.ApprovalApproved
			if rs.approval == services.ApprovalRejected {
				decision = services.ApprovalRejected
			}
			at := created.Add(3 * time.Hour)
			must(db.Create(&models.TicketApproval{TicketID: t.ID, Round: 1, UserID: users[rs.decidedBy].ID, Decision: decision, Comment: rs.comment, CreatedAt: at, UpdatedAt: at}).Error)
			history(t, rs.decidedBy, "approval_"+decision, rs.comment, at)
		}
	}
	fmt.Printf("   Created %d catalogue items and %d service requests\n", 3, len(requestSeeds))

	// ── Changes ───────────────────────────────────────────────────────────────
	fmt.Println("→ Creating changes and a freeze window…")
	type changeSeed struct {
		customer                                    uint
		title, desc, changeType, changeStatus, risk string
		status, owner                               string
		start, end                                  *time.Time
		impl, backout, test, review, outcome        string
		approval                                    string
		required                                    int
		decisions                                   []struct{ user, decision, comment string }
		ci                                          string
		ageDays                                     int
	}
	nextWeek := now.Add(7 * day).Truncate(time.Hour)
	changeSeeds := []changeSeed{
		{
			customer: globex, title: "Upgrade Kubernetes cluster to 1.31", changeType: "normal", changeStatus: "awaiting_approval", risk: "high",
			desc: "Upgrade control plane and worker nodes from 1.30 to 1.31 before 1.30 goes out of support.", status: "open", owner: "raj",
			start: &nextWeek, end: ptr(nextWeek.Add(4 * time.Hour)),
			impl:     "1. Upgrade control plane\n2. Drain and upgrade nodes one at a time\n3. Run smoke tests",
			backout:  "Restore the etcd snapshot taken before the upgrade and roll the nodes back to the 1.30 image.",
			test:     "Smoke test all Globex services and check the monitoring dashboards for 30 minutes.",
			approval: services.ApprovalPending, required: 2,
			decisions: []struct{ user, decision, comment string }{{"marc", "approved", "Plan looks good; make sure the etcd snapshot is verified."}},
			ci:        "Globex Kubernetes cluster", ageDays: 3,
		},
		{
			customer: acme, title: "Monthly OS patching acme-web-01", changeType: "standard", changeStatus: "scheduled", risk: "low",
			desc: "Pre-approved monthly security patching.", status: "open", owner: "lisa",
			start: ptr(now.Add(3 * day).Truncate(time.Hour)), end: ptr(now.Add(3 * day).Truncate(time.Hour).Add(time.Hour)),
			impl: "apt upgrade, reboot, verify health check", backout: "Boot the previous kernel from the GRUB menu.",
			approval: services.ApprovalApproved, ci: "acme-web-01", ageDays: 1,
		},
		{
			customer: acme, title: "Emergency: rotate expired TLS certificate", changeType: "emergency", changeStatus: "completed", risk: "medium",
			desc: "The portal's TLS certificate expired at 02:00; rotate it immediately.", status: "resolved", owner: "marc",
			start: ptr(now.Add(-6 * day).Truncate(time.Hour)), end: ptr(now.Add(-6 * day).Truncate(time.Hour).Add(30 * time.Minute)),
			impl: "Issue a new certificate and reload the gateway.", backout: "Re-enable the maintenance page.",
			review: "Rotated within 25 minutes. Added certificate expiry alerting to monitoring.", outcome: "successful",
			approval: services.ApprovalApproved, required: 1,
			decisions: []struct{ user, decision, comment string }{{"admin", "approved", "Go."}},
			ci:        "Acme API Gateway", ageDays: 6,
		},
	}
	for _, cs := range changeSeeds {
		cid := cs.customer
		created := now.Add(-time.Duration(cs.ageDays) * day)
		t := &models.Ticket{
			CustomerID: &cid, Title: cs.title, Description: cs.desc, Type: "change_request", Status: cs.status,
			Priority: "medium", CreatedByID: users[cs.owner].ID, OwnerID: &users[cs.owner].ID, AssignedToID: &users[cs.owner].ID,
			GroupID: devops, CategoryID: &cats["Infrastructure"].ID,
			ChangeType: cs.changeType, ChangeStatus: cs.changeStatus, Risk: cs.risk, PlannedStart: cs.start, PlannedEnd: cs.end,
			ImplementationPlan: cs.impl, BackoutPlan: cs.backout, TestPlan: cs.test, ReviewNotes: cs.review, ChangeOutcome: cs.outcome,
			ApprovalStatus: cs.approval, CreatedAt: created, UpdatedAt: created,
		}
		if cs.required > 0 {
			t.ApprovalGroupID, t.ApprovalsRequired, t.ApprovalRound = &cab, cs.required, 1
		}
		if cs.status == "resolved" {
			t.ResolvedAt = cs.end
		}
		must(db.Create(t).Error)
		history(t, cs.owner, "created", "", created)
		for i, d := range cs.decisions {
			at := created.Add(time.Duration(i+1) * time.Hour)
			must(db.Create(&models.TicketApproval{TicketID: t.ID, Round: 1, UserID: users[d.user].ID, Decision: d.decision, Comment: d.comment, CreatedAt: at, UpdatedAt: at}).Error)
			history(t, d.user, "approval_"+d.decision, d.comment, at)
		}
		linkCI(t, cs.ci)
	}
	yearEnd := time.Date(now.Year(), time.December, 20, 0, 0, 0, 0, time.Local)
	if now.After(yearEnd.AddDate(0, 0, 14)) {
		yearEnd = yearEnd.AddDate(1, 0, 0)
	}
	must(db.Create(&models.ChangeFreezeWindow{Name: "Year-end change freeze", StartAt: yearEnd, EndAt: yearEnd.AddDate(0, 0, 14)}).Error)
	fmt.Printf("   Created %d changes (one awaiting CAB approval) and a freeze window\n", len(changeSeeds))
}

// removeITSMDemoData removes what seedITSM created. Ticket-bound rows
// (approvals, CI and article links) of demo tickets are removed with the
// tickets themselves via removeITSMTicketRows.
func removeITSMDemoData(db *gorm.DB) {
	var ciIDs []uint
	db.Model(&models.ConfigurationItem{}).Where("name IN ?", seedCINames).Pluck("id", &ciIDs)
	if len(ciIDs) > 0 {
		db.Where("source_id IN ? OR target_id IN ?", ciIDs, ciIDs).Delete(&models.CIRelation{})
		db.Where("ci_id IN ?", ciIDs).Delete(&models.TicketCI{})
		db.Where("id IN ?", ciIDs).Delete(&models.ConfigurationItem{})
	}
	var artIDs []uint
	db.Model(&models.KnowledgeArticle{}).Where("title IN ?", seedArticleTitles).Pluck("id", &artIDs)
	if len(artIDs) > 0 {
		db.Where("article_id IN ?", artIDs).Delete(&models.TicketArticleLink{})
		db.Where("id IN ?", artIDs).Delete(&models.KnowledgeArticle{})
	}
	db.Where("name IN ?", seedCatalogNames).Delete(&models.ServiceCatalogItem{})
	db.Where("name IN ?", seedFreezeNames).Delete(&models.ChangeFreezeWindow{})
	var catIDs []uint
	db.Model(&models.TicketCategory{}).Where("name IN ?", seedCategoryNames).Pluck("id", &catIDs)
	if len(catIDs) > 0 {
		db.Model(&models.Ticket{}).Where("category_id IN ?", catIDs).Update("category_id", nil)
		db.Where("id IN ?", catIDs).Delete(&models.TicketCategory{})
	}
	var groupIDs []uint
	db.Model(&models.UserGroup{}).Where("name IN ?", seedITSMGroups).Pluck("id", &groupIDs)
	if len(groupIDs) > 0 {
		db.Where("group_id IN ?", groupIDs).Delete(&models.GroupMember{})
		db.Where("id IN ?", groupIDs).Delete(&models.UserGroup{})
	}
}

// removeITSMTicketRows deletes the rows that reference the given tickets
// (approvals reference users too, so they must go before the demo users).
func removeITSMTicketRows(db *gorm.DB, ticketIDs []uint) {
	if len(ticketIDs) == 0 {
		return
	}
	db.Where("ticket_id IN ?", ticketIDs).Delete(&models.TicketApproval{})
	db.Where("ticket_id IN ?", ticketIDs).Delete(&models.TicketCI{})
	db.Where("ticket_id IN ?", ticketIDs).Delete(&models.TicketArticleLink{})
}
