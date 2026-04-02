package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ─── HTTP helper ─────────────────────────────────────────────────────────────

// do performs an HTTP request and returns the response body, status code, and
// any transport-level error. A non-2xx status is NOT treated as an error here;
// callers inspect the status code themselves.
func cwDo(method, url string, headers map[string]string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}
	return data, resp.StatusCode, nil
}

// ─── Auth ────────────────────────────────────────────────────────────────────

// Login authenticates with Coworker and returns the JWT access token.
func Login(baseURL, username, password string) (string, error) {
	type loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	type loginResp struct {
		AccessToken string `json:"access_token"`
	}

	data, status, err := cwDo("POST", baseURL+"/api/v1/auth/login", nil, loginReq{
		Username: username,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("login failed (HTTP %d): %s", status, string(data))
	}

	var resp loginResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse login response: %w", err)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in login response")
	}
	return resp.AccessToken, nil
}

// ─── Read project ─────────────────────────────────────────────────────────────

// apiProject matches the Coworker JSON project response.
type apiProject struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
}

type apiColumn struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type apiCard struct {
	ID               uint        `json:"id"`
	CardNumber       int         `json:"card_number"`
	Title            string      `json:"title"`
	Description      string      `json:"description"`
	Priority         string      `json:"priority"`
	DueDate          *time.Time  `json:"due_date"`
	Closed           bool        `json:"closed"`
	TimeSpentMinutes int         `json:"time_spent_minutes"`
	Labels           []apiLabel  `json:"labels"`
	Tags             []apiTag    `json:"tags"`
	Assignees        []apiUser   `json:"assignees"`
	Assignee         *apiUser    `json:"assignee"`
	Attachments      []apiAttach `json:"attachments"`
}

type apiLabel struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type apiTag struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type apiUser struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type apiAttach struct {
	ID       uint   `json:"id"`
	Filename string `json:"original_filename"`
	MimeType string `json:"mime_type"`
}

type apiComment struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      apiUser   `json:"user"`
}

type apiCheckItem struct {
	Body        string `json:"body"`
	IsCompleted bool   `json:"is_completed"`
}

type apiTopic struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	User       apiUser   `json:"user"`
	ReplyCount int       `json:"reply_count"`
}

type apiTopicDetail struct {
	ID      uint            `json:"id"`
	Title   string          `json:"title"`
	Body    string          `json:"body"`
	User    apiUser         `json:"user"`
	Replies []apiTopicReply `json:"replies"`
}

type apiTopicReply struct {
	Body string  `json:"body"`
	User apiUser `json:"user"`
}

// ReadProject fetches the full project from Coworker into the canonical types.
func ReadProject(baseURL, token, slug string) (*Project, error) {
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	// ── project meta ─────────────────────────────────────────────────────────
	data, status, err := cwDo("GET", baseURL+"/api/v1/projects/"+slug, hdrs, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get project (HTTP %d): %s", status, string(data))
	}
	var proj apiProject
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}

	result := &Project{
		Name:        proj.Name,
		Description: proj.Description,
	}

	// ── columns ───────────────────────────────────────────────────────────────
	data, status, err = cwDo("GET", baseURL+"/api/v1/projects/"+slug+"/columns", hdrs, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get columns (HTTP %d): %s", status, string(data))
	}
	var apiColumns []apiColumn
	if err := json.Unmarshal(data, &apiColumns); err != nil {
		return nil, fmt.Errorf("parse columns: %w", err)
	}

	for _, col := range apiColumns {
		cwCol := Column{Name: col.Name}

		// ── cards in column ───────────────────────────────────────────────────
		cardURL := fmt.Sprintf("%s/api/v1/projects/%s/columns/%d/cards", baseURL, slug, col.ID)
		data, status, err = cwDo("GET", cardURL, hdrs, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("get cards for column %d (HTTP %d)", col.ID, status)
		}
		var cards []apiCard
		if err := json.Unmarshal(data, &cards); err != nil {
			return nil, fmt.Errorf("parse cards: %w", err)
		}

		for _, c := range cards {
			card, err := fetchFullCard(baseURL, slug, token, c)
			if err != nil {
				return nil, err
			}
			cwCol.Cards = append(cwCol.Cards, card)
		}

		result.Columns = append(result.Columns, cwCol)
	}

	// ── topics ────────────────────────────────────────────────────────────────
	data, status, err = cwDo("GET", baseURL+"/api/v1/projects/"+slug+"/topics", hdrs, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get topics (HTTP %d): %s", status, string(data))
	}
	var topics []apiTopic
	if err := json.Unmarshal(data, &topics); err != nil {
		return nil, fmt.Errorf("parse topics: %w", err)
	}

	for _, t := range topics {
		topic, err := fetchTopicDetail(baseURL, slug, token, t.ID)
		if err != nil {
			return nil, err
		}
		result.Topics = append(result.Topics, *topic)
	}

	return result, nil
}

// fetchFullCard loads comments, checklist and formats card from the API.
func fetchFullCard(baseURL, slug, token string, c apiCard) (Card, error) {
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	card := Card{
		Title:       c.Title,
		Description: c.Description,
		Priority:    c.Priority,
		Closed:      c.Closed,
		TimeMinutes: c.TimeSpentMinutes,
	}
	if c.CardNumber > 0 {
		card.Ref = fmt.Sprintf("%d", c.CardNumber)
	}
	if c.DueDate != nil {
		card.DueDate = c.DueDate.Format("2006-01-02")
	}

	// Labels
	for _, l := range c.Labels {
		card.Labels = append(card.Labels, Label{Name: l.Name, Color: l.Color})
	}

	// Tags
	for _, t := range c.Tags {
		card.Tags = append(card.Tags, t.Name)
	}

	// Assignees (multiple)
	seen := map[uint]bool{}
	for _, a := range c.Assignees {
		if !seen[a.ID] {
			seen[a.ID] = true
			name := a.DisplayName
			if name == "" {
				name = a.Username
			}
			card.Assignees = append(card.Assignees, name)
		}
	}
	// Legacy single assignee fallback
	if c.Assignee != nil && !seen[c.Assignee.ID] {
		name := c.Assignee.DisplayName
		if name == "" {
			name = c.Assignee.Username
		}
		card.Assignees = append(card.Assignees, name)
	}

	// Attachments
	for _, a := range c.Attachments {
		card.Attachments = append(card.Attachments, Attachment{
			Filename: a.Filename,
			URL:      fmt.Sprintf("%s/api/v1/attachments/%d", baseURL, a.ID),
			MimeType: a.MimeType,
		})
	}

	// Comments
	commentURL := fmt.Sprintf("%s/api/v1/projects/%s/cards/%d/comments", baseURL, slug, c.ID)
	data, status, err := cwDo("GET", commentURL, hdrs, nil)
	if err != nil {
		return card, err
	}
	if status == http.StatusOK {
		var comments []apiComment
		if err := json.Unmarshal(data, &comments); err == nil {
			for _, cm := range comments {
				name := cm.User.DisplayName
				if name == "" {
					name = cm.User.Username
				}
				card.Comments = append(card.Comments, Comment{
					Author:    name,
					Body:      cm.Body,
					CreatedAt: cm.CreatedAt,
				})
			}
		}
	}

	// Checklist
	checkURL := fmt.Sprintf("%s/api/v1/projects/%s/cards/%d/checklist", baseURL, slug, c.ID)
	data, status, err = cwDo("GET", checkURL, hdrs, nil)
	if err != nil {
		return card, err
	}
	if status == http.StatusOK {
		var items []apiCheckItem
		if err := json.Unmarshal(data, &items); err == nil {
			for _, item := range items {
				card.Checklist = append(card.Checklist, CheckItem{
					Text: item.Body,
					Done: item.IsCompleted,
				})
			}
		}
	}

	return card, nil
}

func fetchTopicDetail(baseURL, slug, token string, topicID uint) (*Topic, error) {
	hdrs := map[string]string{"Authorization": "Bearer " + token}
	url := fmt.Sprintf("%s/api/v1/projects/%s/topics/%d", baseURL, slug, topicID)
	data, status, err := cwDo("GET", url, hdrs, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get topic %d (HTTP %d)", topicID, status)
	}

	var t apiTopicDetail
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse topic detail: %w", err)
	}

	author := t.User.DisplayName
	if author == "" {
		author = t.User.Username
	}

	topic := &Topic{
		Title:  t.Title,
		Body:   t.Body,
		Author: author,
	}
	for _, r := range t.Replies {
		rAuthor := r.User.DisplayName
		if rAuthor == "" {
			rAuthor = r.User.Username
		}
		topic.Replies = append(topic.Replies, TopicReply{
			Author: rAuthor,
			Body:   r.Body,
		})
	}
	return topic, nil
}

// ─── Write project ────────────────────────────────────────────────────────────

// WriteProject creates a project and all its content in Coworker using the
// REST API.
func WriteProject(baseURL, token string, p *Project, columnMap map[string]string) error {
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	// ── create project ────────────────────────────────────────────────────────
	slug := slugify(p.Name)
	type createProjReq struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Color       string `json:"color"`
		KeyPrefix   string `json:"key_prefix"`
	}
	prefix := strings.ToUpper(slug)
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	data, status, err := cwDo("POST", baseURL+"/api/v1/projects", hdrs, createProjReq{
		Name:        p.Name,
		Slug:        slug,
		Description: p.Description,
		Color:       "#6366f1",
		KeyPrefix:   prefix,
	})
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("create project (HTTP %d): %s", status, string(data))
	}
	var createdProj apiProject
	if err := json.Unmarshal(data, &createdProj); err != nil {
		return fmt.Errorf("parse created project: %w", err)
	}
	fmt.Printf("  → created project %q (slug=%s)\n", p.Name, createdProj.Slug)

	// ── collect unique labels ─────────────────────────────────────────────────
	labelIDMap := map[string]uint{} // label name → id in Coworker
	uniqueLabels := map[string]Label{}
	for _, col := range p.Columns {
		for _, card := range col.Cards {
			for _, l := range card.Labels {
				uniqueLabels[l.Name] = l
			}
		}
	}
	for _, l := range uniqueLabels {
		type createLabelReq struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		color := l.Color
		if color == "" {
			color = "#6366f1"
		}
		data, status, err := cwDo("POST",
			fmt.Sprintf("%s/api/v1/projects/%s/labels", baseURL, createdProj.Slug),
			hdrs,
			createLabelReq{Name: l.Name, Color: color},
		)
		if err != nil {
			return err
		}
		if status == http.StatusCreated || status == http.StatusOK {
			var lbl apiLabel
			if err := json.Unmarshal(data, &lbl); err == nil {
				labelIDMap[l.Name] = lbl.ID
			}
		}
	}

	// ── columns and cards ─────────────────────────────────────────────────────
	for _, col := range p.Columns {
		colName := MapColumn(col.Name, columnMap)
		type createColReq struct {
			Name string `json:"name"`
		}
		data, status, err := cwDo("POST",
			fmt.Sprintf("%s/api/v1/projects/%s/columns", baseURL, createdProj.Slug),
			hdrs,
			createColReq{Name: colName},
		)
		if err != nil {
			return err
		}
		if status != http.StatusCreated && status != http.StatusOK {
			return fmt.Errorf("create column %q (HTTP %d): %s", colName, status, string(data))
		}
		var createdCol apiColumn
		if err := json.Unmarshal(data, &createdCol); err != nil {
			return fmt.Errorf("parse created column: %w", err)
		}
		fmt.Printf("  → column %q\n", colName)

		for _, card := range col.Cards {
			if err := writeCard(baseURL, token, createdProj.Slug, createdCol.ID, card, labelIDMap); err != nil {
				return fmt.Errorf("write card %q: %w", card.Title, err)
			}
		}
	}

	// ── topics ────────────────────────────────────────────────────────────────
	for _, topic := range p.Topics {
		if err := writeTopic(baseURL, token, createdProj.Slug, topic); err != nil {
			return fmt.Errorf("write topic %q: %w", topic.Title, err)
		}
	}

	return nil
}

func writeCard(baseURL, token, slug string, columnID uint, card Card, labelIDMap map[string]uint) error {
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	type createCardReq struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Priority    string  `json:"priority"`
		DueDate     *string `json:"due_date,omitempty"`
	}
	req := createCardReq{
		Title:       card.Title,
		Description: card.Description,
		Priority:    card.Priority,
	}
	if card.DueDate != "" {
		req.DueDate = &card.DueDate
	}

	data, status, err := cwDo("POST",
		fmt.Sprintf("%s/api/v1/projects/%s/columns/%d/cards", baseURL, slug, columnID),
		hdrs, req,
	)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("create card (HTTP %d): %s", status, string(data))
	}
	var created apiCard
	if err := json.Unmarshal(data, &created); err != nil {
		return fmt.Errorf("parse created card: %w", err)
	}
	fmt.Printf("    → card %q\n", card.Title)

	// Assign labels
	for _, l := range card.Labels {
		if labelID, ok := labelIDMap[l.Name]; ok {
			url := fmt.Sprintf("%s/api/v1/projects/%s/cards/%d/labels/%d", baseURL, slug, created.ID, labelID)
			cwDo("POST", url, hdrs, nil) //nolint:errcheck
		}
	}

	// Checklist
	for _, item := range card.Checklist {
		type checkReq struct {
			Body        string `json:"body"`
			IsCompleted bool   `json:"is_completed"`
		}
		cwDo("POST", //nolint:errcheck
			fmt.Sprintf("%s/api/v1/projects/%s/cards/%d/checklist", baseURL, slug, created.ID),
			hdrs,
			checkReq{Body: item.Text, IsCompleted: item.Done},
		)
	}

	// Comments
	for _, cm := range card.Comments {
		body := cm.Body
		if cm.Author != "" {
			body = fmt.Sprintf("*[%s]* %s", cm.Author, cm.Body)
		}
		type commentReq struct {
			Body string `json:"body"`
		}
		cwDo("POST", //nolint:errcheck
			fmt.Sprintf("%s/api/v1/projects/%s/cards/%d/comments", baseURL, slug, created.ID),
			hdrs,
			commentReq{Body: body},
		)
	}

	return nil
}

func writeTopic(baseURL, token, slug string, topic Topic) error {
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	type createTopicReq struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	body := topic.Body
	if topic.Author != "" {
		body = fmt.Sprintf("*[%s]* %s", topic.Author, topic.Body)
	}
	data, status, err := cwDo("POST",
		fmt.Sprintf("%s/api/v1/projects/%s/topics", baseURL, slug),
		hdrs,
		createTopicReq{Title: topic.Title, Body: body},
	)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("create topic (HTTP %d): %s", status, string(data))
	}

	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(data, &created); err != nil {
		return fmt.Errorf("parse created topic: %w", err)
	}
	fmt.Printf("  → topic %q\n", topic.Title)

	// Replies
	for _, reply := range topic.Replies {
		replyBody := reply.Body
		if reply.Author != "" {
			replyBody = fmt.Sprintf("*[%s]* %s", reply.Author, reply.Body)
		}
		type replyReq struct {
			Body string `json:"body"`
		}
		cwDo("POST", //nolint:errcheck
			fmt.Sprintf("%s/api/v1/projects/%s/topics/%d/replies", baseURL, slug, created.ID),
			hdrs,
			replyReq{Body: replyBody},
		)
	}

	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// slugify converts a project name to a URL-safe slug.
func slugify(name string) string {
	s := strings.ToLower(name)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('-')
		}
	}
	// Trim leading/trailing dashes
	result := strings.Trim(b.String(), "-")
	// Collapse consecutive dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}
