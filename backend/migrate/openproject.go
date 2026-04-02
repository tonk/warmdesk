package migrate

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ─── OpenProject HTTP helper ──────────────────────────────────────────────────

func opDo(method, rawURL, apiKey string, body interface{}) ([]byte, int, error) {
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
	// Basic auth: username="apikey", password=<api_key>
	creds := base64.StdEncoding.EncodeToString([]byte("apikey:" + apiKey))
	req.Header.Set("Authorization", "Basic "+creds)
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

// opHref builds an HAL href link object.
func opHref(href string) map[string]interface{} {
	return map[string]interface{}{"href": href}
}

// ─── Priority mapping ─────────────────────────────────────────────────────────

var warmDeskToOPPriority = map[string]string{
	"none":     "Low",
	"low":      "Low",
	"medium":   "Normal",
	"high":     "High",
	"critical": "Immediate",
}

var opToWarmDeskPriority = map[string]string{
	"Low":       "low",
	"Normal":    "medium",
	"High":      "high",
	"Urgent":    "high",
	"Immediate": "critical",
}

// ─── Export to OpenProject ────────────────────────────────────────────────────

// ExportToOpenProject exports a canonical project to OpenProject via its API v3.
func ExportToOpenProject(cfg PlatformConfig, p *Project, columnMap map[string]string) error {
	base := strings.TrimRight(cfg.URL, "/")
	apiKey := cfg.APIToken
	projectID := cfg.ProjectID

	// Load statuses → id map
	statusIDByName, err := opLoadStatuses(base, apiKey)
	if err != nil {
		fmt.Printf("  ⚠ could not load statuses: %v\n", err)
	}

	// Load priorities → id map
	priorityIDByName, err := opLoadPriorities(base, apiKey)
	if err != nil {
		fmt.Printf("  ⚠ could not load priorities: %v\n", err)
	}

	// Find "Task" type id
	taskTypeHref, err := opFindTypeHref(base, apiKey, "Task")
	if err != nil {
		fmt.Printf("  ⚠ could not find Task type: %v\n", err)
		taskTypeHref = ""
	}

	// Load assignees → id map
	userHrefByName, err := opLoadAssignees(base, apiKey, projectID)
	if err != nil {
		fmt.Printf("  ⚠ could not load assignees: %v\n", err)
	}

	for _, col := range p.Columns {
		targetStatus := MapColumn(col.Name, columnMap)
		statusHref := ""
		if id, ok := statusIDByName[targetStatus]; ok {
			statusHref = fmt.Sprintf("/api/v3/statuses/%d", id)
		}

		for _, card := range col.Cards {
			fmt.Printf("  → exporting card %s: %s\n", card.Ref, card.Title)

			links := map[string]interface{}{}
			if taskTypeHref != "" {
				links["type"] = opHref(taskTypeHref)
			}
			if statusHref != "" {
				links["status"] = opHref(statusHref)
			}

			opPriority := warmDeskToOPPriority[card.Priority]
			if opPriority == "" {
				opPriority = "Normal"
			}
			if id, ok := priorityIDByName[opPriority]; ok {
				links["priority"] = opHref(fmt.Sprintf("/api/v3/priorities/%d", id))
			}

			if len(card.Assignees) > 0 {
				if href, ok := userHrefByName[card.Assignees[0]]; ok {
					links["assignee"] = opHref(href)
				}
			}

			wpBody := map[string]interface{}{
				"subject": card.Title,
				"description": map[string]string{
					"format": "markdown",
					"raw":    card.Description,
				},
				"_links": links,
			}
			if card.DueDate != "" {
				wpBody["dueDate"] = card.DueDate
			}

			wpURL := fmt.Sprintf("%s/api/v3/projects/%s/work_packages", base, projectID)
			data, status, err := opDo("POST", wpURL, apiKey, wpBody)
			if err != nil {
				return fmt.Errorf("create work package %q: %w", card.Title, err)
			}
			if status != http.StatusCreated && status != http.StatusOK {
				return fmt.Errorf("create work package %q (HTTP %d): %s", card.Title, status, string(data))
			}

			var created struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(data, &created); err != nil {
				return fmt.Errorf("parse work package response: %w", err)
			}

			// Comments via activities
			for _, cm := range card.Comments {
				body := cm.Body
				if cm.Author != "" {
					body = fmt.Sprintf("*[%s]* %s", cm.Author, cm.Body)
				}
				opDo("POST", //nolint:errcheck
					fmt.Sprintf("%s/api/v3/work_packages/%d/activities", base, created.ID),
					apiKey,
					map[string]interface{}{
						"comment": map[string]string{
							"format": "markdown",
							"raw":    body,
						},
					},
				)
			}

			// Time entries
			if card.TimeMinutes > 0 {
				hours := float64(card.TimeMinutes) / 60.0
				opDo("POST", //nolint:errcheck
					fmt.Sprintf("%s/api/v3/time_entries", base),
					apiKey,
					map[string]interface{}{
						"hours": fmt.Sprintf("PT%.4fH", hours),
						"_links": map[string]interface{}{
							"workPackage": opHref(fmt.Sprintf("/api/v3/work_packages/%d", created.ID)),
							"project":     opHref(fmt.Sprintf("/api/v3/projects/%s", projectID)),
						},
					},
				)
			}

			// Checklist items as child work packages
			for _, item := range card.Checklist {
				childLinks := map[string]interface{}{
					"parent": opHref(fmt.Sprintf("/api/v3/work_packages/%d", created.ID)),
				}
				if taskTypeHref != "" {
					childLinks["type"] = opHref(taskTypeHref)
				}
				childBody := map[string]interface{}{
					"subject": item.Text,
					"_links":  childLinks,
				}
				// If done, try to use a "Closed" or "Done" status
				if item.Done {
					if id, ok := statusIDByName["Closed"]; ok {
						childLinks["status"] = opHref(fmt.Sprintf("/api/v3/statuses/%d", id))
					} else if id, ok := statusIDByName["Done"]; ok {
						childLinks["status"] = opHref(fmt.Sprintf("/api/v3/statuses/%d", id))
					}
				}
				opDo("POST", wpURL, apiKey, childBody) //nolint:errcheck
			}
		}
	}

	fmt.Printf("✓ export to OpenProject complete\n")
	return nil
}

// ─── Import from OpenProject ──────────────────────────────────────────────────

// ImportFromOpenProject reads an OpenProject project and returns the canonical form.
func ImportFromOpenProject(cfg PlatformConfig, columnMap map[string]string) (*Project, error) {
	base := strings.TrimRight(cfg.URL, "/")
	apiKey := cfg.APIToken
	projectID := cfg.ProjectID
	reverseMap := ReverseColumnMap(columnMap)

	// Project meta
	data, status, err := opDo("GET", fmt.Sprintf("%s/api/v3/projects/%s", base, projectID), apiKey, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get project (HTTP %d): %s", status, string(data))
	}
	var opProj struct {
		Name        string `json:"name"`
		Description struct {
			Raw string `json:"raw"`
		} `json:"description"`
	}
	if err := json.Unmarshal(data, &opProj); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}

	proj := &Project{
		Name:        opProj.Name,
		Description: opProj.Description.Raw,
	}

	colIndex := map[string]*Column{}

	// Paginate work packages
	offset := 1
	pageSize := 50
	for {
		wpURL := fmt.Sprintf("%s/api/v3/projects/%s/work_packages?pageSize=%d&offset=%d",
			base, projectID, pageSize, offset)
		data, status, err := opDo("GET", wpURL, apiKey, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("get work packages (HTTP %d): %s", status, string(data))
		}

		var page struct {
			Total    int `json:"total"`
			PageSize int `json:"pageSize"`
			Elements []struct {
				ID      int    `json:"id"`
				Subject string `json:"subject"`
				Description struct {
					Raw string `json:"raw"`
				} `json:"description"`
				DueDate string `json:"dueDate"`
				Links   struct {
					Status struct {
						Title string `json:"title"`
					} `json:"status"`
					Priority struct {
						Title string `json:"title"`
					} `json:"priority"`
					Assignee struct {
						Title string `json:"title"`
					} `json:"assignee"`
					Parent struct {
						Href string `json:"href"`
					} `json:"parent"`
				} `json:"_links"`
			} `json:"_embedded"`
		}

		// OpenProject wraps results in _embedded.elements
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse page: %w", err)
		}

		var totalCount int
		var elements []struct {
			ID      int    `json:"id"`
			Subject string `json:"subject"`
			Description struct {
				Raw string `json:"raw"`
			} `json:"description"`
			DueDate string `json:"dueDate"`
			Links   struct {
				Status struct {
					Title string `json:"title"`
				} `json:"status"`
				Priority struct {
					Title string `json:"title"`
				} `json:"priority"`
				Assignee struct {
					Title string `json:"title"`
					Href  string `json:"href"`
				} `json:"assignee"`
				Parent struct {
					Href string `json:"href"`
				} `json:"parent"`
			} `json:"_links"`
		}

		if v, ok := raw["total"]; ok {
			json.Unmarshal(v, &totalCount) //nolint:errcheck
		}
		if embedded, ok := raw["_embedded"]; ok {
			var embMap map[string]json.RawMessage
			if err := json.Unmarshal(embedded, &embMap); err == nil {
				if elems, ok := embMap["elements"]; ok {
					json.Unmarshal(elems, &elements) //nolint:errcheck
				}
			}
		}
		_ = page

		for _, wp := range elements {
			// Skip child work packages (have a parent link)
			if wp.Links.Parent.Href != "" {
				continue
			}

			statusName := wp.Links.Status.Title
			colName := MapColumnReverse(statusName, reverseMap)
			col, ok := colIndex[colName]
			if !ok {
				proj.Columns = append(proj.Columns, Column{Name: colName})
				col = &proj.Columns[len(proj.Columns)-1]
				colIndex[colName] = col
			}

			priority := opToWarmDeskPriority[wp.Links.Priority.Title]
			if priority == "" {
				priority = "none"
			}

			card := Card{
				Title:       wp.Subject,
				Description: wp.Description.Raw,
				DueDate:     wp.DueDate,
				Priority:    priority,
			}

			if wp.Links.Assignee.Title != "" {
				card.Assignees = []string{wp.Links.Assignee.Title}
			}

			// Child WPs as checklist
			childURL := fmt.Sprintf("%s/api/v3/work_packages?filters=%%5B%%7B%%22parent%%22%%3A%%7B%%22operator%%22%%3A%%22%%3D%%22%%2C%%22values%%22%%3A%%5B%%22%d%%22%%5D%%7D%%7D%%5D", base, wp.ID)
			if cData, cStatus, err := opDo("GET", childURL, apiKey, nil); err == nil && cStatus == http.StatusOK {
				var childPage map[string]json.RawMessage
				if err := json.Unmarshal(cData, &childPage); err == nil {
					if embedded, ok := childPage["_embedded"]; ok {
						var embMap map[string]json.RawMessage
						if err := json.Unmarshal(embedded, &embMap); err == nil {
							if elems, ok := embMap["elements"]; ok {
								var children []struct {
									Subject string `json:"subject"`
									Links   struct {
										Status struct{ Title string `json:"title"` } `json:"status"`
									} `json:"_links"`
								}
								if err := json.Unmarshal(elems, &children); err == nil {
									for _, ch := range children {
										done := strings.EqualFold(ch.Links.Status.Title, "closed") ||
											strings.EqualFold(ch.Links.Status.Title, "done")
										card.Checklist = append(card.Checklist, CheckItem{
											Text: ch.Subject,
											Done: done,
										})
									}
								}
							}
						}
					}
				}
			}

			// Activities (comments)
			actURL := fmt.Sprintf("%s/api/v3/work_packages/%d/activities", base, wp.ID)
			if aData, aStatus, err := opDo("GET", actURL, apiKey, nil); err == nil && aStatus == http.StatusOK {
				var actPage map[string]json.RawMessage
				if err := json.Unmarshal(aData, &actPage); err == nil {
					if embedded, ok := actPage["_embedded"]; ok {
						var embMap map[string]json.RawMessage
						if err := json.Unmarshal(embedded, &embMap); err == nil {
							if elems, ok := embMap["elements"]; ok {
								var activities []struct {
									Comment struct {
										Raw string `json:"raw"`
									} `json:"comment"`
									Links struct {
										User struct{ Title string `json:"title"` } `json:"user"`
									} `json:"_links"`
								}
								if err := json.Unmarshal(elems, &activities); err == nil {
									for _, act := range activities {
										if act.Comment.Raw != "" {
											card.Comments = append(card.Comments, Comment{
												Author: act.Links.User.Title,
												Body:   act.Comment.Raw,
											})
										}
									}
								}
							}
						}
					}
				}
			}

			col.Cards = append(col.Cards, card)
		}

		offset += len(elements)
		if offset > totalCount || len(elements) == 0 {
			break
		}
	}

	return proj, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func opLoadStatuses(base, apiKey string) (map[string]int, error) {
	data, status, err := opDo("GET", base+"/api/v3/statuses", apiKey, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	return opParseIDMap(data, "name")
}

func opLoadPriorities(base, apiKey string) (map[string]int, error) {
	data, status, err := opDo("GET", base+"/api/v3/priorities", apiKey, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	return opParseIDMap(data, "name")
}

func opFindTypeHref(base, apiKey, typeName string) (string, error) {
	data, status, err := opDo("GET", base+"/api/v3/types", apiKey, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", status)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(data, &page); err != nil {
		return "", err
	}
	if embedded, ok := page["_embedded"]; ok {
		var embMap map[string]json.RawMessage
		if err := json.Unmarshal(embedded, &embMap); err == nil {
			if elems, ok := embMap["elements"]; ok {
				var types []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(elems, &types); err == nil {
					for _, t := range types {
						if strings.EqualFold(t.Name, typeName) {
							return fmt.Sprintf("/api/v3/types/%d", t.ID), nil
						}
					}
					// Fall back to first type
					if len(types) > 0 {
						return fmt.Sprintf("/api/v3/types/%d", types[0].ID), nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("type %q not found", typeName)
}

func opLoadAssignees(base, apiKey, projectID string) (map[string]string, error) {
	url := fmt.Sprintf("%s/api/v3/projects/%s/available_assignees", base, projectID)
	data, status, err := opDo("GET", url, apiKey, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", status)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	result := map[string]string{}
	if embedded, ok := page["_embedded"]; ok {
		var embMap map[string]json.RawMessage
		if err := json.Unmarshal(embedded, &embMap); err == nil {
			if elems, ok := embMap["elements"]; ok {
				var users []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(elems, &users); err == nil {
					for _, u := range users {
						result[u.Name] = fmt.Sprintf("/api/v3/users/%d", u.ID)
					}
				}
			}
		}
	}
	return result, nil
}

// opParseIDMap extracts name → id from a standard OpenProject collection.
func opParseIDMap(data []byte, nameField string) (map[string]int, error) {
	var page map[string]json.RawMessage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	result := map[string]int{}
	if embedded, ok := page["_embedded"]; ok {
		var embMap map[string]json.RawMessage
		if err := json.Unmarshal(embedded, &embMap); err == nil {
			if elems, ok := embMap["elements"]; ok {
				var items []map[string]interface{}
				if err := json.Unmarshal(elems, &items); err == nil {
					for _, item := range items {
						name, _ := item[nameField].(string)
						idF, _ := item["id"].(float64)
						if name != "" {
							result[name] = int(idF)
						}
					}
				}
			}
		}
	}
	return result, nil
}
