// Seed populates the database with demo projects, users, cards, comments, and more.
//
// Usage (run from the backend/ directory):
//
//	go run ./cmd/seed
//	go run ./cmd/seed --config /path/to/coworker.yaml
//	go run ./cmd/seed --reset   # drop all demo data first, then re-seed
//
// The script is idempotent: it exits early when it detects that the demo data
// is already present (looks for username "demo.admin").
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/tonk/coworker/config"
	"github.com/tonk/coworker/database"
	"github.com/tonk/coworker/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func must(err error) {
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
}

func hashPassword(plain string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	must(err)
	return string(h)
}

func ptr[T any](v T) *T { return &v }

func days(n int) *time.Time {
	t := time.Now().UTC().AddDate(0, 0, n).Truncate(24 * time.Hour)
	return &t
}

// ─── demo data definitions ──────────────────────────────────────────────────

type seedProject struct {
	name    string
	slug    string
	prefix  string
	color   string
	desc    string
	columns []string
	labels  []struct{ name, color string }
}

var demoProjects = []seedProject{
	{
		name:   "Website Redesign",
		slug:   "website-redesign",
		prefix: "WEB",
		color:  "#6366f1",
		desc:   "Full redesign of the marketing website — new brand, new stack, new speed.",
		columns: []string{"Backlog", "In Progress", "Review", "Done"},
		labels: []struct{ name, color string }{
			{"Bug", "#ef4444"}, {"Feature", "#3b82f6"},
			{"Design", "#8b5cf6"}, {"Content", "#10b981"},
		},
	},
	{
		name:   "Mobile App v2",
		slug:   "mobile-app-v2",
		prefix: "MOB",
		color:  "#f59e0b",
		desc:   "Ground-up rewrite of the iOS and Android apps with a shared React Native core.",
		columns: []string{"Ideas", "Development", "Testing", "Released"},
		labels: []struct{ name, color string }{
			{"Bug", "#ef4444"}, {"Enhancement", "#3b82f6"},
			{"iOS", "#0ea5e9"}, {"Android", "#22c55e"},
		},
	},
	{
		name:   "DevOps & Infrastructure",
		slug:   "devops-infra",
		prefix: "INF",
		color:  "#10b981",
		desc:   "Kubernetes migration, monitoring, backups, and everything that keeps the lights on.",
		columns: []string{"Todo", "In Progress", "Done"},
		labels: []struct{ name, color string }{
			{"Critical", "#ef4444"}, {"Enhancement", "#3b82f6"},
			{"Monitoring", "#f59e0b"}, {"Security", "#8b5cf6"},
		},
	},
}

// ─── main ────────────────────────────────────────────────────────────────────

func main() {
	configPath := flag.String("config", "", "path to coworker.yaml (optional)")
	reset := flag.Bool("reset", false, "remove existing demo data before seeding")
	flag.Parse()

	cfg := config.Load(*configPath)
	cfg.DBLog = "silent" // keep seed output readable
	must(database.Init(cfg))
	db := database.DB

	// Guard: already seeded?
	var existing models.User
	if err := db.Where("username = ?", "demo.admin").First(&existing).Error; err == nil {
		if !*reset {
			fmt.Println("✓ Demo data already present (username 'demo.admin' found).")
			fmt.Println("  Run with --reset to wipe and re-seed.")
			return
		}
		fmt.Println("⚠  --reset: removing existing demo data…")
		removeDemoData(db)
	}

	fmt.Println("🌱 Seeding demo data…")
	fmt.Println()

	// ── 1. Users ──────────────────────────────────────────────────────────────
	fmt.Println("→ Creating users…")

	users := map[string]*models.User{
		"admin": {
			Email: "admin@demo.example", Username: "demo.admin",
			PasswordHash: hashPassword("demo1234"), GlobalRole: "admin",
			FirstName: "Alex", LastName: "Admin", DisplayName: "Alex Admin",
			IsActive: true, EmailNotifications: true,
		},
		"sarah": {
			Email: "sarah@demo.example", Username: "demo.sarah",
			PasswordHash: hashPassword("demo1234"), GlobalRole: "user",
			FirstName: "Sarah", LastName: "Chen", DisplayName: "Sarah Chen",
			IsActive: true, EmailNotifications: true,
		},
		"marc": {
			Email: "marc@demo.example", Username: "demo.marc",
			PasswordHash: hashPassword("demo1234"), GlobalRole: "user",
			FirstName: "Marc", LastName: "Dubois", DisplayName: "Marc Dubois",
			IsActive: true, EmailNotifications: true,
		},
		"lisa": {
			Email: "lisa@demo.example", Username: "demo.lisa",
			PasswordHash: hashPassword("demo1234"), GlobalRole: "user",
			FirstName: "Lisa", LastName: "Park", DisplayName: "Lisa Park",
			IsActive: true, EmailNotifications: true,
		},
		"viewer": {
			Email: "viewer@demo.example", Username: "demo.viewer",
			PasswordHash: hashPassword("demo1234"), GlobalRole: "viewer",
			FirstName: "Victor", LastName: "Viewer", DisplayName: "Victor Viewer",
			IsActive: true, EmailNotifications: false,
		},
	}

	for _, u := range users {
		must(db.Create(u).Error)
	}
	fmt.Printf("   Created %d users (password for all: demo1234)\n", len(users))

	// ── 2. Projects + columns + labels + members ──────────────────────────────
	fmt.Println("→ Creating projects…")

	type projectData struct {
		project *models.Project
		cols    map[string]*models.Column
		labels  map[string]*models.Label
	}
	projects := map[string]*projectData{}

	projectMembers := map[string][]struct {
		user string
		role string
	}{
		"website-redesign": {
			{"admin", "owner"}, {"sarah", "member"}, {"marc", "member"}, {"viewer", "viewer"},
		},
		"mobile-app-v2": {
			{"sarah", "owner"}, {"marc", "member"}, {"lisa", "member"}, {"viewer", "viewer"},
		},
		"devops-infra": {
			{"marc", "owner"}, {"admin", "member"}, {"lisa", "member"}, {"viewer", "viewer"},
		},
	}

	for _, sp := range demoProjects {
		p := &models.Project{
			Name: sp.name, Slug: sp.slug, KeyPrefix: sp.prefix,
			Color: sp.color, Description: sp.desc,
			CreatedByID: users["admin"].ID,
		}
		must(db.Create(p).Error)

		pd := &projectData{
			project: p,
			cols:    map[string]*models.Column{},
			labels:  map[string]*models.Label{},
		}

		// Columns
		for i, colName := range sp.columns {
			col := &models.Column{
				ProjectID: p.ID, Name: colName,
				Position: float64((i + 1) * 1000),
			}
			must(db.Create(col).Error)
			pd.cols[colName] = col
		}

		// Labels
		for _, l := range sp.labels {
			lbl := &models.Label{ProjectID: p.ID, Name: l.name, Color: l.color}
			must(db.Create(lbl).Error)
			pd.labels[l.name] = lbl
		}

		// Members
		for _, m := range projectMembers[sp.slug] {
			must(db.Create(&models.ProjectMember{
				ProjectID: p.ID, UserID: users[m.user].ID, Role: m.role,
			}).Error)
		}

		projects[sp.slug] = pd
	}
	fmt.Printf("   Created %d projects\n", len(projects))

	// ── 3. Cards ──────────────────────────────────────────────────────────────
	fmt.Println("→ Creating cards…")

	type cardSpec struct {
		title    string
		col      string
		priority string
		labels   []string
		tags     []string
		assignee string // user key or ""
		dueInDays *int
		timeMin  int    // time_spent_minutes
		checklist []struct{ body string; done bool }
		comments  []struct{ author string; body string }
	}

	webCards := []cardSpec{
		// Backlog
		{
			title: "Redesign homepage hero section", col: "Backlog",
			priority: "high", labels: []string{"Feature", "Design"},
			tags: []string{"homepage", "design"},
		},
		{
			title: "Add cookie consent banner", col: "Backlog",
			priority: "medium", labels: []string{"Feature"},
			assignee: "marc", dueInDays: ptr(14),
		},
		{
			title: "Write new About page copy", col: "Backlog",
			priority: "none", labels: []string{"Content"},
			assignee: "sarah",
		},
		// In Progress
		{
			title: "Implement dark mode toggle", col: "In Progress",
			priority: "medium", labels: []string{"Feature", "Design"},
			assignee: "sarah", timeMin: 180, dueInDays: ptr(5),
			checklist: []struct{ body string; done bool }{
				{"Research CSS variables approach", true},
				{"Implement toggle button in header", true},
				{"Persist preference to localStorage", false},
				{"Test on Safari and Firefox", false},
			},
			comments: []struct{ author string; body string }{
				{"marc", "I suggest using `prefers-color-scheme` as the initial default — saves a flash of wrong theme on first load."},
				{"sarah", "Good call! Adding that to the checklist."},
			},
		},
		{
			title: "Fix mobile navigation overflow on small screens", col: "In Progress",
			priority: "high", labels: []string{"Bug"},
			assignee: "marc", timeMin: 90, dueInDays: ptr(2),
			comments: []struct{ author string; body string }{
				{"sarah", "Reproduced on iPhone SE (375 px). The hamburger menu clips behind the logo."},
				{"marc", "On it — looks like `overflow: hidden` is missing on the nav container."},
			},
		},
		{
			title: "Update brand colour palette across all components", col: "In Progress",
			priority: "low", labels: []string{"Design"},
			assignee: "sarah", timeMin: 120,
		},
		// Review
		{
			title: "Optimise image loading with lazy + WebP", col: "Review",
			priority: "medium", labels: []string{"Feature"},
			assignee: "sarah", timeMin: 240, dueInDays: ptr(1),
			comments: []struct{ author string; body string }{
				{"marc", "LCP went from 3.8 s to 1.2 s in local tests — great improvement!"},
				{"sarah", "Waiting for sign-off on the WebP fallback strategy for older browsers."},
			},
		},
		{
			title: "Accessibility audit and ARIA fixes", col: "Review",
			priority: "high", labels: []string{"Feature"},
			assignee: "marc", timeMin: 300,
		},
		// Done
		{
			title: "Set up GitHub Actions CI/CD pipeline", col: "Done",
			priority: "high", labels: []string{"Feature"},
			assignee: "admin", timeMin: 360, dueInDays: ptr(-10),
		},
		{
			title: "Migrate DNS and SSL to new hosting provider", col: "Done",
			priority: "critical", labels: []string{"Feature"},
			assignee: "admin", timeMin: 480, dueInDays: ptr(-5),
		},
		{
			title: "Create component library documentation", col: "Done",
			priority: "low", labels: []string{"Design", "Content"},
			assignee: "sarah", timeMin: 210, dueInDays: ptr(-3),
		},
		{
			title: "Audit and fix all broken links", col: "Done",
			priority: "medium", labels: []string{"Bug"},
			assignee: "marc", timeMin: 60, dueInDays: ptr(-7),
		},
	}

	mobCards := []cardSpec{
		// Ideas
		{
			title: "Offline mode with sync queue", col: "Ideas",
			priority: "high", labels: []string{"Enhancement"},
			tags: []string{"offline", "ux"},
		},
		{
			title: "Push notification preferences screen", col: "Ideas",
			priority: "medium", labels: []string{"Enhancement"},
		},
		{
			title: "Biometric login (Face ID / fingerprint)", col: "Ideas",
			priority: "medium", labels: []string{"Enhancement"},
			tags: []string{"security", "auth"},
		},
		// Development
		{
			title: "User profile screen", col: "Development",
			priority: "high", labels: []string{"Enhancement", "iOS", "Android"},
			assignee: "sarah", timeMin: 300, dueInDays: ptr(7),
			checklist: []struct{ body string; done bool }{
				{"Approve design mockup", true},
				{"Build form components", true},
				{"Connect to profile API", true},
				{"Add avatar upload", false},
				{"Write E2E tests", false},
			},
			comments: []struct{ author string; body string }{
				{"marc", "The avatar cropper might need a third-party library — I found `react-easy-crop`, looks solid."},
				{"sarah", "Added to the checklist. Let's decide before Thursday."},
				{"lisa", "I can test on Android once the form components are done."},
			},
		},
		{
			title: "Settings / preferences screen", col: "Development",
			priority: "medium", labels: []string{"Enhancement", "iOS", "Android"},
			assignee: "marc", timeMin: 180, dueInDays: ptr(10),
		},
		{
			title: "Dark mode support", col: "Development",
			priority: "low", labels: []string{"Enhancement"},
			assignee: "lisa", timeMin: 240,
		},
		{
			title: "App crashes on empty conversation list", col: "Development",
			priority: "critical", labels: []string{"Bug"},
			assignee: "marc", timeMin: 45, dueInDays: ptr(1),
			comments: []struct{ author string; body string }{
				{"lisa", "Stack trace attached. Null check missing in `ConversationListScreen.tsx` line 42."},
				{"marc", "Fix is one line — pushing a hotfix build now."},
			},
		},
		// Testing
		{
			title: "Integration tests for authentication flow", col: "Testing",
			priority: "high", labels: []string{"Enhancement", "iOS", "Android"},
			assignee: "lisa", timeMin: 360, dueInDays: ptr(3),
		},
		{
			title: "Performance profiling on low-end Android devices", col: "Testing",
			priority: "medium", labels: []string{"Enhancement", "Android"},
			assignee: "sarah", timeMin: 150,
			comments: []struct{ author string; body string }{
				{"marc", "Focus on the feed screen — renders ~200 items without virtualisation right now."},
			},
		},
		// Released
		{
			title: "Initial 1.0 app release", col: "Released",
			priority: "critical", labels: []string{"Enhancement", "iOS", "Android"},
			assignee: "sarah", timeMin: 1200, dueInDays: ptr(-30),
		},
		{
			title: "Bug fix release 1.0.1", col: "Released",
			priority: "high", labels: []string{"Bug"},
			assignee: "marc", timeMin: 120, dueInDays: ptr(-14),
		},
	}

	infCards := []cardSpec{
		// Todo
		{
			title: "Set up Kubernetes cluster on cloud provider", col: "Todo",
			priority: "high", labels: []string{"Enhancement"},
			dueInDays: ptr(21),
			checklist: []struct{ body string; done bool }{
				{"Choose cloud provider (GKE / EKS / AKS)", false},
				{"Design namespace and RBAC structure", false},
				{"Configure networking and ingress", false},
				{"Set up auto-scaling policies", false},
				{"Disaster recovery runbook", false},
			},
		},
		{
			title: "Add Prometheus + Grafana monitoring stack", col: "Todo",
			priority: "medium", labels: []string{"Monitoring"},
			assignee: "lisa", dueInDays: ptr(28),
		},
		{
			title: "Quarterly security audit", col: "Todo",
			priority: "critical", labels: []string{"Security"},
			dueInDays: ptr(7),
		},
		// In Progress
		{
			title: "Migrate primary database to PostgreSQL", col: "In Progress",
			priority: "high", labels: []string{"Enhancement"},
			assignee: "marc", timeMin: 480, dueInDays: ptr(4),
			comments: []struct{ author string; body string }{
				{"admin", "Remember to keep the SQLite DB as a read-only fallback for at least two weeks after cutover."},
				{"marc", "Agreed. I've scripted the data export — schema diff is smaller than expected."},
				{"lisa", "I'll keep an eye on slow-query logs during the transition."},
			},
		},
		{
			title: "Automate database backups with off-site retention", col: "In Progress",
			priority: "high", labels: []string{"Monitoring"},
			assignee: "lisa", timeMin: 180, dueInDays: ptr(6),
		},
		{
			title: "Renew and automate SSL certificate rotation", col: "In Progress",
			priority: "critical", labels: []string{"Security"},
			assignee: "marc", timeMin: 60, dueInDays: ptr(2),
		},
		// Done
		{
			title: "Set up private Docker registry", col: "Done",
			priority: "medium", labels: []string{"Enhancement"},
			assignee: "marc", timeMin: 240, dueInDays: ptr(-10),
		},
		{
			title: "Configure Nginx load balancer with health checks", col: "Done",
			priority: "high", labels: []string{"Enhancement"},
			assignee: "lisa", timeMin: 300, dueInDays: ptr(-8),
		},
		{
			title: "Deploy staging environment", col: "Done",
			priority: "high", labels: []string{"Enhancement"},
			assignee: "marc", timeMin: 360, dueInDays: ptr(-15),
		},
	}

	projectCards := map[string][]cardSpec{
		"website-redesign": webCards,
		"mobile-app-v2":    mobCards,
		"devops-infra":     infCards,
	}

	totalCards := 0
	for slug, specs := range projectCards {
		pd := projects[slug]
		for i, spec := range specs {
			col := pd.cols[spec.col]
			if col == nil {
				log.Fatalf("seed: unknown column %q in project %s", spec.col, slug)
			}

			// Increment project card counter
			must(db.Model(&models.Project{}).Where("id = ?", pd.project.ID).
				UpdateColumn("card_counter", gorm.Expr("card_counter + 1")).Error)
			var proj models.Project
			must(db.Select("card_counter").First(&proj, pd.project.ID).Error)

			var assigneeID *uint
			if spec.assignee != "" {
				assigneeID = &users[spec.assignee].ID
			}

			priority := spec.priority
			if priority == "" {
				priority = "none"
			}

			var dueDate *time.Time
			if spec.dueInDays != nil {
				dueDate = days(*spec.dueInDays)
			}

			card := &models.Card{
				ColumnID:         col.ID,
				ProjectID:        pd.project.ID,
				Title:            spec.title,
				Priority:         priority,
				AssigneeID:       assigneeID,
				CreatedByID:      users["admin"].ID,
				Position:         float64((i + 1) * 1000),
				CardNumber:       proj.CardCounter,
				DueDate:          dueDate,
				TimeSpentMinutes: spec.timeMin,
			}
			must(db.Create(card).Error)

			// Labels
			for _, lname := range spec.labels {
				if lbl, ok := pd.labels[lname]; ok {
					must(db.Exec("INSERT INTO card_labels (card_id, label_id) VALUES (?, ?)", card.ID, lbl.ID).Error)
				}
			}

			// Tags
			for _, tag := range spec.tags {
				must(db.Create(&models.CardTag{CardID: card.ID, Name: tag}).Error)
			}

			// Multi-assignees (mirror the single assignee for display consistency)
			if assigneeID != nil {
				must(db.Exec("INSERT OR IGNORE INTO card_assignees (card_id, user_id) VALUES (?, ?)", card.ID, *assigneeID).Error)
			}

			// Checklist
			for j, item := range spec.checklist {
				must(db.Create(&models.CardChecklistItem{
					CardID:      card.ID,
					Body:        item.body,
					IsCompleted: item.done,
					Position:    float64((j + 1) * 1000),
				}).Error)
			}

			// Comments
			for _, c := range spec.comments {
				author := users[c.author]
				if author == nil {
					author = users["admin"]
				}
				must(db.Create(&models.CardComment{
					CardID: card.ID, UserID: author.ID, Body: c.body,
				}).Error)
			}

			totalCards++
		}
	}
	fmt.Printf("   Created %d cards\n", totalCards)

	// ── 4. Topics ─────────────────────────────────────────────────────────────
	fmt.Println("→ Creating topics…")

	type topicSpec struct {
		project string
		author  string
		title   string
		body    string
		pinned  bool
		replies []struct{ author, body string }
	}

	topicSpecs := []topicSpec{
		{
			project: "website-redesign",
			author:  "admin",
			title:   "Q4 Design Direction",
			pinned:  true,
			body: `Hi everyone 👋

After last week's brand workshop I want to summarise the three pillars we agreed on:

1. **Clarity over cleverness** — every page should answer the visitor's question in under 5 seconds.
2. **Performance as a feature** — target a Lighthouse score ≥ 90 on all pages.
3. **Accessible by default** — WCAG AA minimum, aiming for AAA on primary flows.

Designs go into Figma first; no dev work starts without a reviewed mockup.
Questions or push-back? Reply here!`,
			replies: []struct{ author, body string }{
				{"sarah", "Fully on board with the performance target. I'll set up Lighthouse CI so every PR gets a score automatically."},
				{"marc", "On the accessibility point — should we bring in an external auditor mid-project or rely on our own review?"},
				{"admin", "@demo.marc Good question. Let's do an internal pass first, then budget for one external review before launch."},
			},
		},
		{
			project: "mobile-app-v2",
			author:  "sarah",
			title:   "API integration strategy — REST vs GraphQL",
			body: `We need to decide how the mobile app talks to the backend before we go deeper into development.

**Option A — REST (current approach)**
- Proven, simple, easy to cache
- We already have the endpoints; just needs mobile-friendly pagination

**Option B — GraphQL**
- Fetch exactly what you need (big win on mobile bandwidth)
- Requires a new gateway layer; non-trivial migration

My preference is to stick with REST for v2 and add a thin BFF (Backend for Frontend) layer to shape responses for mobile. Thoughts?`,
			replies: []struct{ author, body string }{
				{"marc", "I'd vote REST + BFF too. GraphQL would be ideal but the migration cost isn't justified for v2."},
				{"lisa", "Agree. We can always add a GraphQL layer for v3 once we know our real query patterns from production data."},
				{"sarah", "Settled then — REST + BFF it is. I'll open a card for the BFF scaffolding."},
			},
		},
		{
			project: "devops-infra",
			author:  "marc",
			title:   "PostgreSQL migration — go/no-go checklist",
			pinned:  true,
			body: `Before we cut over to Postgres in production, everyone needs to sign off on this list:

- [ ] Schema migration tested on a production-size data copy
- [ ] All queries verified for Postgres compatibility (no SQLite-isms)
- [ ] Read-replica in place for reporting queries
- [ ] Rollback procedure documented and rehearsed
- [ ] Monitoring dashboards updated for Postgres metrics
- [ ] On-call runbook updated

I'll move us to "go" once all boxes are checked. Please comment here with your sign-off.`,
			replies: []struct{ author, body string }{
				{"lisa", "Monitoring dashboards are live in Grafana. Signing off on that item ✓"},
				{"admin", "Rollback procedure is documented in the wiki and tested in staging. ✓"},
				{"marc", "Schema migration tested — 4.2 M rows, completed in 8 minutes. Well within our maintenance window. ✓"},
			},
		},
	}

	totalTopics := 0
	for _, ts := range topicSpecs {
		pd := projects[ts.project]
		topic := &models.Topic{
			ProjectID: pd.project.ID,
			UserID:    users[ts.author].ID,
			Title:     ts.title,
			Body:      ts.body,
			IsPinned:  ts.pinned,
		}
		must(db.Create(topic).Error)

		for _, r := range ts.replies {
			author := users[r.author]
			if author == nil {
				author = users["admin"]
			}
			must(db.Create(&models.TopicReply{
				TopicID: topic.ID, UserID: author.ID, Body: r.body,
			}).Error)
		}
		totalTopics++
	}
	fmt.Printf("   Created %d topics\n", totalTopics)

	// ── 5. Summary ────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("✅ Demo data seeded successfully!")
	fmt.Println()
	fmt.Println("  Accounts (password: demo1234)")
	fmt.Println("  ┌─────────────────────┬─────────────────┬────────┐")
	fmt.Println("  │ Username            │ Display name    │ Role   │")
	fmt.Println("  ├─────────────────────┼─────────────────┼────────┤")
	fmt.Println("  │ demo.admin          │ Alex Admin      │ admin  │")
	fmt.Println("  │ demo.sarah          │ Sarah Chen      │ user   │")
	fmt.Println("  │ demo.marc           │ Marc Dubois     │ user   │")
	fmt.Println("  │ demo.lisa           │ Lisa Park       │ user   │")
	fmt.Println("  │ demo.viewer         │ Victor Viewer   │ viewer │")
	fmt.Println("  └─────────────────────┴─────────────────┴────────┘")
	fmt.Println()
	fmt.Printf("  Projects  : %d\n", len(demoProjects))
	fmt.Printf("  Cards     : %d\n", totalCards)
	fmt.Printf("  Topics    : %d\n", totalTopics)
	fmt.Println()
	fmt.Println("  Start the server and log in at http://localhost:8080")
}

// removeDemoData deletes all records created by the seed (identified by the
// demo users and projects), then the users themselves.
func removeDemoData(db *gorm.DB) {
	demoUsernames := []string{"demo.admin", "demo.sarah", "demo.marc", "demo.lisa", "demo.viewer"}
	demoSlugs := []string{"website-redesign", "mobile-app-v2", "devops-infra"}

	// Collect user IDs
	var userIDs []uint
	db.Model(&models.User{}).Where("username IN ?", demoUsernames).Pluck("id", &userIDs)

	// Collect project IDs
	var projectIDs []uint
	db.Model(&models.Project{}).Where("slug IN ?", demoSlugs).Pluck("id", &projectIDs)

	if len(projectIDs) > 0 {
		// Collect card IDs
		var cardIDs []uint
		db.Model(&models.Card{}).Where("project_id IN ?", projectIDs).Pluck("id", &cardIDs)

		if len(cardIDs) > 0 {
			db.Where("card_id IN ?", cardIDs).Delete(&models.CardComment{})
			db.Where("card_id IN ?", cardIDs).Delete(&models.CardChecklistItem{})
			db.Where("card_id IN ?", cardIDs).Delete(&models.CardTag{})
			db.Exec("DELETE FROM card_labels WHERE card_id IN ?", cardIDs)
			db.Exec("DELETE FROM card_assignees WHERE card_id IN ?", cardIDs)
		}

		db.Unscoped().Where("project_id IN ?", projectIDs).Delete(&models.Card{})
		db.Unscoped().Where("project_id IN ?", projectIDs).Delete(&models.Column{})
		db.Unscoped().Where("project_id IN ?", projectIDs).Delete(&models.Label{})
		db.Where("project_id IN ?", projectIDs).Delete(&models.ProjectMember{})

		// Topics
		var topicIDs []uint
		db.Model(&models.Topic{}).Where("project_id IN ?", projectIDs).Pluck("id", &topicIDs)
		if len(topicIDs) > 0 {
			db.Unscoped().Where("topic_id IN ?", topicIDs).Delete(&models.TopicReply{})
		}
		db.Unscoped().Where("project_id IN ?", projectIDs).Delete(&models.Topic{})
		db.Unscoped().Where("id IN ?", projectIDs).Delete(&models.Project{})
	}

	if len(userIDs) > 0 {
		db.Unscoped().Where("id IN ?", userIDs).Delete(&models.User{})
	}

	fmt.Println("   Done.")
}
