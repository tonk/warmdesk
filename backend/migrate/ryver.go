package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ─── Ryver HTTP helper ────────────────────────────────────────────────────────
//
// Ryver exposes an OData-based REST API at https://{org}.ryver.com/api/1/odata.svc
// Authentication uses a Bearer token obtained from the Ryver admin console.

func ryverDo(method, rawURL, token string, body interface{}) ([]byte, int, error) {
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
	req.Header.Set("Authorization", "Bearer "+token)
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

func ryverBase(org string) string {
	return fmt.Sprintf("https://%s.ryver.com/api/1/odata.svc", org)
}

// ─── Export to Ryver ──────────────────────────────────────────────────────────

// ExportToRyver exports a canonical project to Ryver as tasks in a team forum.
//
// Ryver does not have columns/statuses natively. The WarmDesk column name is
// appended as a tag on each task so the mapping can be reconstructed on import.
func ExportToRyver(cfg PlatformConfig, p *Project, columnMap map[string]string) error {
	base := ryverBase(cfg.Org)
	token := cfg.APIToken

	// Find the team by name
	teamID, err := ryverFindTeam(base, token, cfg.Team)
	if err != nil {
		return fmt.Errorf("find team %q: %w", cfg.Team, err)
	}
	fmt.Printf("  → team id: %d\n", teamID)

	for _, col := range p.Columns {
		columnTag := MapColumn(col.Name, columnMap)

		for _, card := range col.Cards {
			fmt.Printf("  → exporting card %s: %s\n", card.Ref, card.Title)

			// Build body: description + checklist
			body := card.Description
			if len(card.Checklist) > 0 {
				body += "\n\n**Checklist:**\n"
				for _, item := range card.Checklist {
					check := "[ ]"
					if item.Done {
						check = "[x]"
					}
					body += fmt.Sprintf("- %s %s\n", check, item.Text)
				}
			}
			if card.TimeMinutes > 0 {
				h := card.TimeMinutes / 60
				m := card.TimeMinutes % 60
				body += fmt.Sprintf("\n⏱ Time spent: %d:%02d\n", h, m)
			}

			// Build tags: column tag + card tags + labels
			tags := []string{columnTag}
			tags = append(tags, card.Tags...)
			for _, l := range card.Labels {
				tags = append(tags, l.Name)
			}

			taskBody := map[string]interface{}{
				"subject":  card.Title,
				"body":     body,
				"tags":     strings.Join(tags, ","),
			}
			if card.DueDate != "" {
				taskBody["dueDate"] = card.DueDate + "T00:00:00Z"
			}
			if card.Closed {
				taskBody["isComplete"] = true
			}

			// POST task to the team's task list
			taskURL := fmt.Sprintf("%s/Workrooms(%d)/Topic.TaskCreate()", base, teamID)
			data, status, err := ryverDo("POST", taskURL, token, taskBody)
			if err != nil {
				return fmt.Errorf("create task %q: %w", card.Title, err)
			}
			if status != http.StatusOK && status != http.StatusCreated {
				// Fall back: post as a topic if task API is not available
				fmt.Printf("    ⚠ task API returned %d, posting as topic\n", status)
				topicBody := map[string]interface{}{
					"subject": card.Title,
					"body":    body,
					"tags":    strings.Join(tags, ","),
				}
				topicURL := fmt.Sprintf("%s/Workrooms(%d)/Post.PostCreateTopic()", base, teamID)
				data, status, err = ryverDo("POST", topicURL, token, topicBody)
				if err != nil {
					return fmt.Errorf("create topic fallback %q: %w", card.Title, err)
				}
				if status != http.StatusOK && status != http.StatusCreated {
					fmt.Printf("    ⚠ could not export %q (HTTP %d): %s\n", card.Title, status, string(data))
					continue
				}
			}

			// Extract created entity id for comments
			var created struct {
				D struct {
					ID int `json:"id"`
				} `json:"d"`
			}
			var taskID int
			if err := json.Unmarshal(data, &created); err == nil {
				taskID = created.D.ID
			}

			// Post comments as replies
			if taskID > 0 {
				for _, cm := range card.Comments {
					replyBody := cm.Body
					if cm.Author != "" {
						replyBody = fmt.Sprintf("*[%s]* %s", cm.Author, cm.Body)
					}
					replyURL := fmt.Sprintf("%s/Posts(%d)/Post.Reply()", base, taskID)
					ryverDo("POST", replyURL, token, map[string]string{"body": replyBody}) //nolint:errcheck
				}
			}
		}
	}

	// Export topics as forum posts
	for _, topic := range p.Topics {
		body := topic.Body
		if topic.Author != "" {
			body = fmt.Sprintf("*[%s]* %s", topic.Author, topic.Body)
		}
		topicURL := fmt.Sprintf("%s/Workrooms(%d)/Post.PostCreateTopic()", base, teamID)
		data, status, err := ryverDo("POST", topicURL, token, map[string]interface{}{
			"subject": topic.Title,
			"body":    body,
		})
		if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
			fmt.Printf("  ⚠ could not export topic %q\n", topic.Title)
			continue
		}
		fmt.Printf("  → topic %q\n", topic.Title)

		var created struct {
			D struct{ ID int `json:"id"` } `json:"d"`
		}
		if err := json.Unmarshal(data, &created); err == nil && created.D.ID > 0 {
			for _, reply := range topic.Replies {
				replyBody := reply.Body
				if reply.Author != "" {
					replyBody = fmt.Sprintf("*[%s]* %s", reply.Author, reply.Body)
				}
				replyURL := fmt.Sprintf("%s/Posts(%d)/Post.Reply()", base, created.D.ID)
				ryverDo("POST", replyURL, token, map[string]string{"body": replyBody}) //nolint:errcheck
			}
		}
	}

	fmt.Printf("✓ export to Ryver complete\n")
	return nil
}

// ─── Import from Ryver ────────────────────────────────────────────────────────

// ImportFromRyver reads tasks and topics from a Ryver team and returns the
// canonical project representation.
func ImportFromRyver(cfg PlatformConfig, columnMap map[string]string) (*Project, error) {
	base := ryverBase(cfg.Org)
	token := cfg.APIToken
	reverseMap := ReverseColumnMap(columnMap)

	// Find team
	teamID, err := ryverFindTeam(base, token, cfg.Team)
	if err != nil {
		return nil, fmt.Errorf("find team %q: %w", cfg.Team, err)
	}

	proj := &Project{Name: cfg.Team}
	colIndex := map[string]*Column{}

	// Get tasks
	tasksURL := fmt.Sprintf("%s/Tasks?$filter=workroom/id+eq+%d&$expand=createUser,tags", base, teamID)
	data, status, err := ryverDo("GET", tasksURL, token, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get tasks (HTTP %d): %s", status, string(data))
	}

	var tasksResp struct {
		D struct {
			Results []struct {
				ID         int    `json:"id"`
				Subject    string `json:"subject"`
				Body       string `json:"body"`
				IsComplete bool   `json:"isComplete"`
				DueDate    string `json:"dueDate"`
				Tags       string `json:"tags"`
				CreateUser struct {
					Username    string `json:"username"`
					DisplayName string `json:"displayName"`
				} `json:"createUser"`
			} `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &tasksResp); err != nil {
		return nil, fmt.Errorf("parse tasks: %w", err)
	}

	for _, t := range tasksResp.D.Results {
		// Parse column name from tags (first tag that matches the reverse map or any tag)
		colName := "Tasks"
		tags := []string{}
		for _, tag := range strings.Split(t.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			mapped := MapColumnReverse(tag, reverseMap)
			if mapped != tag {
				colName = mapped
			} else {
				tags = append(tags, tag)
			}
		}

		col, ok := colIndex[colName]
		if !ok {
			proj.Columns = append(proj.Columns, Column{Name: colName})
			col = &proj.Columns[len(proj.Columns)-1]
			colIndex[colName] = col
		}

		dueDate := ""
		if len(t.DueDate) >= 10 {
			dueDate = t.DueDate[:10]
		}

		card := Card{
			Title:       t.Subject,
			Description: t.Body,
			DueDate:     dueDate,
			Closed:      t.IsComplete,
			Tags:        tags,
		}

		col.Cards = append(col.Cards, card)
	}

	// Get forum topics
	topicsURL := fmt.Sprintf("%s/Posts?$filter=workroom/id+eq+%d+and+recordType+eq+1&$expand=createUser", base, teamID)
	tData, tStatus, err := ryverDo("GET", topicsURL, token, nil)
	if err == nil && tStatus == http.StatusOK {
		var topicsResp struct {
			D struct {
				Results []struct {
					ID      int    `json:"id"`
					Subject string `json:"subject"`
					Body    string `json:"body"`
					CreateUser struct {
						DisplayName string `json:"displayName"`
						Username    string `json:"username"`
					} `json:"createUser"`
				} `json:"results"`
			} `json:"d"`
		}
		if err := json.Unmarshal(tData, &topicsResp); err == nil {
			for _, t := range topicsResp.D.Results {
				author := t.CreateUser.DisplayName
				if author == "" {
					author = t.CreateUser.Username
				}
				topic := Topic{
					Title:  t.Subject,
					Body:   t.Body,
					Author: author,
				}
				// Get replies
				repliesURL := fmt.Sprintf("%s/Posts(%d)/replies?$expand=createUser", base, t.ID)
				rData, rStatus, err := ryverDo("GET", repliesURL, token, nil)
				if err == nil && rStatus == http.StatusOK {
					var repliesResp struct {
						D struct {
							Results []struct {
								Body string `json:"body"`
								CreateUser struct {
									DisplayName string `json:"displayName"`
									Username    string `json:"username"`
								} `json:"createUser"`
							} `json:"results"`
						} `json:"d"`
					}
					if err := json.Unmarshal(rData, &repliesResp); err == nil {
						for _, r := range repliesResp.D.Results {
							rAuthor := r.CreateUser.DisplayName
							if rAuthor == "" {
								rAuthor = r.CreateUser.Username
							}
							topic.Replies = append(topic.Replies, TopicReply{
								Author: rAuthor,
								Body:   r.Body,
							})
						}
					}
				}
				proj.Topics = append(proj.Topics, topic)
			}
		}
	}

	return proj, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func ryverFindTeam(base, token, teamName string) (int, error) {
	url := fmt.Sprintf("%s/Workrooms?$filter=name+eq+'%s'&$select=id,name", base, teamName)
	data, status, err := ryverDo("GET", url, token, nil)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", status, string(data))
	}

	var resp struct {
		D struct {
			Results []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parse teams: %w", err)
	}
	if len(resp.D.Results) == 0 {
		return 0, fmt.Errorf("team %q not found", teamName)
	}
	return resp.D.Results[0].ID, nil
}
