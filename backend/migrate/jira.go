package migrate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"bytes"
)

// ─── Jira HTTP helper ────────────────────────────────────────────────────────

func jiraDo(method, rawURL string, auth string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http %s %s: %w", method, rawURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func jiraAuth(email, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
}

// ─── Jira ADF helpers ─────────────────────────────────────────────────────────

// textToADF converts a plain-text or markdown string to Atlassian Document
// Format (ADF) using simple paragraph nodes split on newlines.
func textToADF(text string) map[string]interface{} {
	lines := strings.Split(text, "\n")
	var content []map[string]interface{}
	for _, line := range lines {
		if line == "" {
			line = " " // ADF requires non-empty text nodes
		}
		content = append(content, map[string]interface{}{
			"type": "paragraph",
			"content": []map[string]interface{}{
				{"type": "text", "text": line},
			},
		})
	}
	if len(content) == 0 {
		content = []map[string]interface{}{
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": " "},
				},
			},
		}
	}
	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// ─── Priority mapping ─────────────────────────────────────────────────────────

var warmDeskToJiraPriority = map[string]string{
	"none":     "Lowest",
	"low":      "Low",
	"medium":   "Medium",
	"high":     "High",
	"critical": "Highest",
}

var jiraToWarmDeskPriority = map[string]string{
	"Lowest":  "none",
	"Low":     "low",
	"Medium":  "medium",
	"High":    "high",
	"Highest": "critical",
}

// ─── Export to Jira ───────────────────────────────────────────────────────────

// ExportToJira exports a canonical project to Jira Cloud.
func ExportToJira(cfg PlatformConfig, p *Project, columnMap map[string]string) error {
	auth := jiraAuth(cfg.Email, cfg.APIToken)
	base := strings.TrimRight(cfg.URL, "/")

	issueType := cfg.IssueType
	if issueType == "" {
		issueType = "Task"
	}

	// Build user cache: display name → accountId
	userCache := map[string]string{}

	lookupUser := func(name string) string {
		if id, ok := userCache[name]; ok {
			return id
		}
		searchURL := fmt.Sprintf("%s/rest/api/3/user/search?query=%s", base, url.QueryEscape(name))
		data, status, err := jiraDo("GET", searchURL, auth, nil)
		if err != nil || status != http.StatusOK {
			return ""
		}
		var users []struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
		}
		if err := json.Unmarshal(data, &users); err != nil || len(users) == 0 {
			return ""
		}
		id := users[0].AccountID
		userCache[name] = id
		return id
	}

	// Cache transitions per issue key
	transitionCache := map[string]string{} // status name → transition id for current issue

	for _, col := range p.Columns {
		targetStatus := MapColumn(col.Name, columnMap)

		for _, card := range col.Cards {
			fmt.Printf("  → exporting card %s: %s\n", card.Ref, card.Title)

			// Build issue fields
			fields := map[string]interface{}{
				"project":   map[string]string{"key": cfg.ProjectKey},
				"summary":   card.Title,
				"issuetype": map[string]string{"name": issueType},
			}

			if card.Description != "" {
				fields["description"] = textToADF(card.Description)
			}

			if p, ok := warmDeskToJiraPriority[card.Priority]; ok {
				fields["priority"] = map[string]string{"name": p}
			}

			if card.DueDate != "" {
				fields["duedate"] = card.DueDate
			}

			// Labels (free-form in Jira)
			if len(card.Tags) > 0 {
				fields["labels"] = card.Tags
			}

			// Assignee (first one)
			if len(card.Assignees) > 0 {
				if accountID := lookupUser(card.Assignees[0]); accountID != "" {
					fields["assignee"] = map[string]string{"accountId": accountID}
				}
			}

			data, status, err := jiraDo("POST", base+"/rest/api/3/issue", auth, map[string]interface{}{
				"fields": fields,
			})
			if err != nil {
				return fmt.Errorf("create issue %q: %w", card.Title, err)
			}
			if status != http.StatusCreated && status != http.StatusOK {
				return fmt.Errorf("create issue %q (HTTP %d): %s", card.Title, status, string(data))
			}

			var created struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal(data, &created); err != nil {
				return fmt.Errorf("parse created issue: %w", err)
			}

			// Comments
			for _, cm := range card.Comments {
				body := cm.Body
				if cm.Author != "" {
					body = fmt.Sprintf("[%s] %s", cm.Author, cm.Body)
				}
				jiraDo("POST", //nolint:errcheck
					fmt.Sprintf("%s/rest/api/3/issue/%s/comment", base, created.Key),
					auth,
					map[string]interface{}{"body": textToADF(body)},
				)
			}

			// Checklist items as sub-tasks
			for _, item := range card.Checklist {
				checkFields := map[string]interface{}{
					"project":   map[string]string{"key": cfg.ProjectKey},
					"summary":   item.Text,
					"issuetype": map[string]string{"name": "Subtask"},
					"parent":    map[string]string{"key": created.Key},
				}
				jiraDo("POST", base+"/rest/api/3/issue", auth, map[string]interface{}{ //nolint:errcheck
					"fields": checkFields,
				})
			}

			// Log time
			if card.TimeMinutes > 0 {
				jiraDo("POST", //nolint:errcheck
					fmt.Sprintf("%s/rest/api/3/issue/%s/worklog", base, created.Key),
					auth,
					map[string]interface{}{
						"timeSpentSeconds": card.TimeMinutes * 60,
					},
				)
			}

			// Transition to match column status
			if targetStatus != "" {
				// Get available transitions
				tData, tStatus, err := jiraDo("GET",
					fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", base, created.Key),
					auth, nil)
				if err == nil && tStatus == http.StatusOK {
					var tr struct {
						Transitions []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
							To   struct {
								Name string `json:"name"`
							} `json:"to"`
						} `json:"transitions"`
					}
					if err := json.Unmarshal(tData, &tr); err == nil {
						_ = transitionCache
						for _, t := range tr.Transitions {
							if strings.EqualFold(t.To.Name, targetStatus) || strings.EqualFold(t.Name, targetStatus) {
								jiraDo("POST", //nolint:errcheck
									fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", base, created.Key),
									auth,
									map[string]interface{}{
										"transition": map[string]string{"id": t.ID},
									},
								)
								break
							}
						}
					}
				}
			}

			// Mark closed
			if card.Closed {
				tData, tStatus, err := jiraDo("GET",
					fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", base, created.Key),
					auth, nil)
				if err == nil && tStatus == http.StatusOK {
					var tr struct {
						Transitions []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"transitions"`
					}
					if err := json.Unmarshal(tData, &tr); err == nil {
						for _, t := range tr.Transitions {
							name := strings.ToLower(t.Name)
							if strings.Contains(name, "done") || strings.Contains(name, "close") || strings.Contains(name, "resolve") {
								jiraDo("POST", //nolint:errcheck
									fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", base, created.Key),
									auth,
									map[string]interface{}{
										"transition": map[string]string{"id": t.ID},
									},
								)
								break
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("✓ export to Jira complete\n")
	return nil
}

// ─── Import from Jira ─────────────────────────────────────────────────────────

// ImportFromJira reads a Jira project and returns the canonical representation.
func ImportFromJira(cfg PlatformConfig, columnMap map[string]string) (*Project, error) {
	auth := jiraAuth(cfg.Email, cfg.APIToken)
	base := strings.TrimRight(cfg.URL, "/")
	reverseMap := ReverseColumnMap(columnMap)

	// Project meta
	data, status, err := jiraDo("GET",
		fmt.Sprintf("%s/rest/api/3/project/%s", base, cfg.ProjectKey),
		auth, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get project (HTTP %d): %s", status, string(data))
	}
	var jiraProj struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &jiraProj); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}

	proj := &Project{
		Name:        jiraProj.Name,
		Description: jiraProj.Description,
	}

	// Columns indexed by status name
	colIndex := map[string]*Column{}

	// Paginate issues
	startAt := 0
	maxResults := 50
	for {
		searchURL := fmt.Sprintf("%s/rest/api/3/search?jql=project=%s+ORDER+BY+created&startAt=%d&maxResults=%d&expand=renderedFields",
			base, url.QueryEscape(cfg.ProjectKey), startAt, maxResults)
		data, status, err := jiraDo("GET", searchURL, auth, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("search issues (HTTP %d): %s", status, string(data))
		}

		var result struct {
			Total      int `json:"total"`
			StartAt    int `json:"startAt"`
			MaxResults int `json:"maxResults"`
			Issues     []struct {
				Key    string `json:"key"`
				Fields struct {
					Summary     string `json:"summary"`
					Description interface{} `json:"description"`
					Status      struct {
						Name string `json:"name"`
					} `json:"status"`
					Priority struct {
						Name string `json:"name"`
					} `json:"priority"`
					DueDate  string `json:"duedate"`
					Assignee *struct {
						DisplayName string `json:"displayName"`
					} `json:"assignee"`
					Labels  []string `json:"labels"`
					Subtasks []struct {
						Key    string `json:"key"`
						Fields struct {
							Summary string `json:"summary"`
							Status  struct{ Name string `json:"name"` } `json:"status"`
						} `json:"fields"`
					} `json:"subtasks"`
					Resolution *struct {
						Name string `json:"name"`
					} `json:"resolution"`
				} `json:"fields"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse issues: %w", err)
		}

		for _, issue := range result.Issues {
			statusName := issue.Fields.Status.Name
			colName := MapColumnReverse(statusName, reverseMap)

			col, ok := colIndex[colName]
			if !ok {
				proj.Columns = append(proj.Columns, Column{Name: colName})
				col = &proj.Columns[len(proj.Columns)-1]
				colIndex[colName] = col
			}

			priority := jiraToWarmDeskPriority[issue.Fields.Priority.Name]
			if priority == "" {
				priority = "none"
			}

			closed := false
			if issue.Fields.Resolution != nil {
				closed = true
			}

			card := Card{
				Ref:      issue.Key,
				Title:    issue.Fields.Summary,
				Priority: priority,
				DueDate:  issue.Fields.DueDate,
				Closed:   closed,
				Tags:     issue.Fields.Labels,
			}

			// Description — ADF or plain text
			if issue.Fields.Description != nil {
				card.Description = extractADFText(issue.Fields.Description)
			}

			// Assignee
			if issue.Fields.Assignee != nil {
				card.Assignees = []string{issue.Fields.Assignee.DisplayName}
			}

			// Subtasks → checklist
			for _, st := range issue.Fields.Subtasks {
				card.Checklist = append(card.Checklist, CheckItem{
					Text: st.Fields.Summary,
					Done: strings.EqualFold(st.Fields.Status.Name, "done"),
				})
			}

			// Comments
			cmData, cmStatus, err := jiraDo("GET",
				fmt.Sprintf("%s/rest/api/3/issue/%s/comment", base, issue.Key),
				auth, nil)
			if err == nil && cmStatus == http.StatusOK {
				var cmResp struct {
					Comments []struct {
						Author struct{ DisplayName string `json:"displayName"` } `json:"author"`
						Body   interface{} `json:"body"`
					} `json:"comments"`
				}
				if err := json.Unmarshal(cmData, &cmResp); err == nil {
					for _, cm := range cmResp.Comments {
						card.Comments = append(card.Comments, Comment{
							Author: cm.Author.DisplayName,
							Body:   extractADFText(cm.Body),
						})
					}
				}
			}

			// Worklogs → time
			wlData, wlStatus, err := jiraDo("GET",
				fmt.Sprintf("%s/rest/api/3/issue/%s/worklog", base, issue.Key),
				auth, nil)
			if err == nil && wlStatus == http.StatusOK {
				var wlResp struct {
					Worklogs []struct {
						TimeSpentSeconds int `json:"timeSpentSeconds"`
					} `json:"worklogs"`
				}
				if err := json.Unmarshal(wlData, &wlResp); err == nil {
					for _, wl := range wlResp.Worklogs {
						card.TimeMinutes += wl.TimeSpentSeconds / 60
					}
				}
			}

			col.Cards = append(col.Cards, card)
		}

		startAt += len(result.Issues)
		if startAt >= result.Total {
			break
		}
	}

	return proj, nil
}

// extractADFText extracts plain text from an ADF node or returns a string
// representation. This is best-effort.
func extractADFText(v interface{}) string {
	if v == nil {
		return ""
	}
	// If it's already a string just return it
	if s, ok := v.(string); ok {
		return s
	}
	// Try to marshal and walk the ADF tree
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var node map[string]interface{}
	if err := json.Unmarshal(data, &node); err != nil {
		return string(data)
	}
	return walkADFNode(node)
}

func walkADFNode(node map[string]interface{}) string {
	var sb strings.Builder
	if text, ok := node["text"].(string); ok {
		sb.WriteString(text)
	}
	if content, ok := node["content"].([]interface{}); ok {
		for _, child := range content {
			if childMap, ok := child.(map[string]interface{}); ok {
				sb.WriteString(walkADFNode(childMap))
			}
		}
	}
	if t, ok := node["type"].(string); ok && (t == "paragraph" || t == "heading") {
		sb.WriteString("\n")
	}
	return sb.String()
}
