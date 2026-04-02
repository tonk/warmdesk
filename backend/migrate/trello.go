package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const trelloBase = "https://api.trello.com"

// ─── Trello HTTP helper ───────────────────────────────────────────────────────

func trelloDo(method, path string, apiKey, token string, body interface{}) ([]byte, int, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	rawURL := trelloBase + path + sep + "key=" + url.QueryEscape(apiKey) + "&token=" + url.QueryEscape(token)

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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

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

// ─── Export to Trello ─────────────────────────────────────────────────────────

// ExportToTrello exports a canonical project to a Trello board.
func ExportToTrello(cfg PlatformConfig, p *Project, columnMap map[string]string) error {
	apiKey := cfg.APIKey
	token := cfg.Token
	boardID := cfg.BoardID

	// Get existing lists on the board
	data, status, err := trelloDo("GET", fmt.Sprintf("/1/boards/%s/lists", boardID), apiKey, token, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("get board lists (HTTP %d): %s", status, string(data))
	}
	var existingLists []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &existingLists); err != nil {
		return fmt.Errorf("parse lists: %w", err)
	}
	listIDByName := map[string]string{}
	for _, l := range existingLists {
		listIDByName[l.Name] = l.ID
	}

	// Get board members
	mData, mStatus, err := trelloDo("GET", fmt.Sprintf("/1/boards/%s/members", boardID), apiKey, token, nil)
	if err != nil {
		return err
	}
	memberIDByName := map[string]string{}
	if mStatus == http.StatusOK {
		var members []struct {
			ID       string `json:"id"`
			FullName string `json:"fullName"`
			Username string `json:"username"`
		}
		if err := json.Unmarshal(mData, &members); err == nil {
			for _, m := range members {
				memberIDByName[m.FullName] = m.ID
				memberIDByName[m.Username] = m.ID
			}
		}
	}

	for _, col := range p.Columns {
		listName := MapColumn(col.Name, columnMap)

		// Create list if it doesn't exist
		listID, ok := listIDByName[listName]
		if !ok {
			lData, lStatus, err := trelloDo("POST",
				fmt.Sprintf("/1/boards/%s/lists", boardID),
				apiKey, token,
				map[string]string{"name": listName},
			)
			if err != nil {
				return fmt.Errorf("create list %q: %w", listName, err)
			}
			if lStatus != http.StatusOK && lStatus != http.StatusCreated {
				return fmt.Errorf("create list %q (HTTP %d): %s", listName, lStatus, string(lData))
			}
			var newList struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(lData, &newList); err != nil {
				return fmt.Errorf("parse new list: %w", err)
			}
			listID = newList.ID
			listIDByName[listName] = listID
		}

		for _, card := range col.Cards {
			fmt.Printf("  → exporting card %s: %s\n", card.Ref, card.Title)

			cardBody := map[string]interface{}{
				"idList": listID,
				"name":   card.Title,
			}
			if card.Description != "" {
				cardBody["desc"] = card.Description
			}
			if card.DueDate != "" {
				cardBody["due"] = card.DueDate + "T00:00:00.000Z"
			}
			if card.Closed {
				cardBody["closed"] = true
			}

			// Member IDs
			var memberIDs []string
			for _, a := range card.Assignees {
				if id, ok := memberIDByName[a]; ok {
					memberIDs = append(memberIDs, id)
				}
			}
			if len(memberIDs) > 0 {
				cardBody["idMembers"] = strings.Join(memberIDs, ",")
			}

			cData, cStatus, err := trelloDo("POST", "/1/cards", apiKey, token, cardBody)
			if err != nil {
				return fmt.Errorf("create card %q: %w", card.Title, err)
			}
			if cStatus != http.StatusOK && cStatus != http.StatusCreated {
				return fmt.Errorf("create card %q (HTTP %d): %s", card.Title, cStatus, string(cData))
			}
			var createdCard struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(cData, &createdCard); err != nil {
				return fmt.Errorf("parse created card: %w", err)
			}

			// Labels
			for _, lbl := range card.Labels {
				color := trelloColor(lbl.Color)
				lbData, lbStatus, err := trelloDo("POST",
					fmt.Sprintf("/1/boards/%s/labels", boardID),
					apiKey, token,
					map[string]string{"name": lbl.Name, "color": color},
				)
				if err == nil && (lbStatus == http.StatusOK || lbStatus == http.StatusCreated) {
					var newLabel struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(lbData, &newLabel); err == nil {
						trelloDo("POST", //nolint:errcheck
							fmt.Sprintf("/1/cards/%s/idLabels", createdCard.ID),
							apiKey, token,
							map[string]string{"value": newLabel.ID},
						)
					}
				}
			}

			// Checklist
			if len(card.Checklist) > 0 {
				clData, clStatus, err := trelloDo("POST", "/1/checklists", apiKey, token,
					map[string]string{"idCard": createdCard.ID, "name": "Checklist"},
				)
				if err == nil && (clStatus == http.StatusOK || clStatus == http.StatusCreated) {
					var newCL struct {
						ID string `json:"id"`
					}
					if err := json.Unmarshal(clData, &newCL); err == nil {
						for _, item := range card.Checklist {
							state := "incomplete"
							if item.Done {
								state = "complete"
							}
							trelloDo("POST", //nolint:errcheck
								fmt.Sprintf("/1/checklists/%s/checkItems", newCL.ID),
								apiKey, token,
								map[string]string{"name": item.Text, "checked": state},
							)
						}
					}
				}
			}

			// Comments
			for _, cm := range card.Comments {
				text := cm.Body
				if cm.Author != "" {
					text = fmt.Sprintf("[%s] %s", cm.Author, cm.Body)
				}
				trelloDo("POST", //nolint:errcheck
					fmt.Sprintf("/1/cards/%s/actions/comments", createdCard.ID),
					apiKey, token,
					map[string]string{"text": text},
				)
			}

			// Time tracking (not natively supported — add as comment)
			if card.TimeMinutes > 0 {
				h := card.TimeMinutes / 60
				m := card.TimeMinutes % 60
				trelloDo("POST", //nolint:errcheck
					fmt.Sprintf("/1/cards/%s/actions/comments", createdCard.ID),
					apiKey, token,
					map[string]string{
						"text": fmt.Sprintf("⏱ Time spent: %d:%02d", h, m),
					},
				)
			}
		}
	}

	fmt.Printf("✓ export to Trello complete\n")
	return nil
}

// ─── Import from Trello ───────────────────────────────────────────────────────

// ImportFromTrello reads a Trello board and returns the canonical representation.
func ImportFromTrello(cfg PlatformConfig, columnMap map[string]string) (*Project, error) {
	apiKey := cfg.APIKey
	token := cfg.Token
	boardID := cfg.BoardID
	reverseMap := ReverseColumnMap(columnMap)

	// Board meta
	bData, bStatus, err := trelloDo("GET",
		fmt.Sprintf("/1/boards/%s?fields=name,desc", boardID),
		apiKey, token, nil)
	if err != nil {
		return nil, err
	}
	if bStatus != http.StatusOK {
		return nil, fmt.Errorf("get board (HTTP %d): %s", bStatus, string(bData))
	}
	var board struct {
		Name string `json:"name"`
		Desc string `json:"desc"`
	}
	if err := json.Unmarshal(bData, &board); err != nil {
		return nil, fmt.Errorf("parse board: %w", err)
	}

	proj := &Project{
		Name:        board.Name,
		Description: board.Desc,
	}

	// Lists
	lData, lStatus, err := trelloDo("GET",
		fmt.Sprintf("/1/boards/%s/lists", boardID),
		apiKey, token, nil)
	if err != nil {
		return nil, err
	}
	if lStatus != http.StatusOK {
		return nil, fmt.Errorf("get lists (HTTP %d): %s", lStatus, string(lData))
	}
	var lists []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(lData, &lists); err != nil {
		return nil, fmt.Errorf("parse lists: %w", err)
	}
	colByListID := map[string]*Column{}
	for _, l := range lists {
		colName := MapColumnReverse(l.Name, reverseMap)
		proj.Columns = append(proj.Columns, Column{Name: colName})
		colByListID[l.ID] = &proj.Columns[len(proj.Columns)-1]
	}

	// All cards with embedded data
	cData, cStatus, err := trelloDo("GET",
		fmt.Sprintf("/1/boards/%s/cards?checklists=all&members=true&attachments=true&actions=commentCard", boardID),
		apiKey, token, nil)
	if err != nil {
		return nil, err
	}
	if cStatus != http.StatusOK {
		return nil, fmt.Errorf("get cards (HTTP %d): %s", cStatus, string(cData))
	}

	var tCards []struct {
		ID          string `json:"id"`
		IDList      string `json:"idList"`
		Name        string `json:"name"`
		Desc        string `json:"desc"`
		Closed      bool   `json:"closed"`
		Due         string `json:"due"`
		IDMembers   []string `json:"idMembers"`
		Labels      []struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		} `json:"labels"`
		Checklists []struct {
			Name       string `json:"name"`
			CheckItems []struct {
				Name  string `json:"name"`
				State string `json:"state"` // "complete" | "incomplete"
			} `json:"checkItems"`
		} `json:"checklists"`
		Actions []struct {
			Type string `json:"type"`
			Data struct {
				Text string `json:"text"`
			} `json:"data"`
			MemberCreator struct {
				FullName string `json:"fullName"`
			} `json:"memberCreator"`
		} `json:"actions"`
		Members []struct {
			FullName string `json:"fullName"`
		} `json:"members"`
		Attachments []struct {
			Name     string `json:"name"`
			URL      string `json:"url"`
			MimeType string `json:"mimeType"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(cData, &tCards); err != nil {
		return nil, fmt.Errorf("parse cards: %w", err)
	}

	for _, tc := range tCards {
		col, ok := colByListID[tc.IDList]
		if !ok {
			continue
		}

		dueDate := ""
		if tc.Due != "" && len(tc.Due) >= 10 {
			dueDate = tc.Due[:10]
		}

		card := Card{
			Title:       tc.Name,
			Description: tc.Desc,
			DueDate:     dueDate,
			Closed:      tc.Closed,
		}

		for _, m := range tc.Members {
			card.Assignees = append(card.Assignees, m.FullName)
		}

		for _, l := range tc.Labels {
			card.Labels = append(card.Labels, Label{Name: l.Name, Color: l.Color})
		}

		for _, cl := range tc.Checklists {
			for _, item := range cl.CheckItems {
				card.Checklist = append(card.Checklist, CheckItem{
					Text: item.Name,
					Done: item.State == "complete",
				})
			}
		}

		for _, a := range tc.Actions {
			if a.Type == "commentCard" {
				card.Comments = append(card.Comments, Comment{
					Author: a.MemberCreator.FullName,
					Body:   a.Data.Text,
				})
			}
		}

		for _, att := range tc.Attachments {
			card.Attachments = append(card.Attachments, Attachment{
				Filename: att.Name,
				URL:      att.URL,
				MimeType: att.MimeType,
			})
		}

		col.Cards = append(col.Cards, card)
	}

	return proj, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// trelloColor maps a hex color to the nearest Trello named color.
func trelloColor(hex string) string {
	h := strings.ToLower(strings.TrimPrefix(hex, "#"))
	switch {
	case strings.HasPrefix(h, "ef"), strings.HasPrefix(h, "dc"), strings.HasPrefix(h, "b9"):
		return "red"
	case strings.HasPrefix(h, "f5"), strings.HasPrefix(h, "f9"), strings.HasPrefix(h, "fb"):
		return "orange"
	case strings.HasPrefix(h, "fc"), strings.HasPrefix(h, "fa"):
		return "yellow"
	case strings.HasPrefix(h, "10"), strings.HasPrefix(h, "22"), strings.HasPrefix(h, "16"):
		return "green"
	case strings.HasPrefix(h, "3b"), strings.HasPrefix(h, "06"), strings.HasPrefix(h, "0e"):
		return "blue"
	case strings.HasPrefix(h, "8b"), strings.HasPrefix(h, "7c"), strings.HasPrefix(h, "6d"):
		return "purple"
	default:
		return "blue"
	}
}
