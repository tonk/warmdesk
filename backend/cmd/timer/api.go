package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type client struct {
	cfg  config
	http *http.Client
}

func newClient(cfg config) *client {
	return &client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
}

// do sends a JSON request to /api/v1<path> and decodes the response into out
// (when non-nil). Server errors come back as the API's "error" message.
func (c *client) do(method, path string, body, out interface{}) error {
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.cfg.URL+"/api/v1"+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach WarmDesk at %s: %w", c.cfg.URL, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		msg := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			msg = apiErr.Error
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("not authorised (%s) — check the API key", msg)
		case http.StatusForbidden:
			return fmt.Errorf("forbidden: %s — the timer needs a personal API key and time tracking enabled for your account", msg)
		}
		return fmt.Errorf("%s", msg)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("unexpected response from WarmDesk: %w", err)
		}
	}
	return nil
}

type target struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	TimeTrackingOnly bool   `json:"time_tracking_only"`
	CustomerID       *uint  `json:"customer_id"`
	CustomerName     string `json:"customer_name"`
}

type targetList struct {
	Projects  []target `json:"projects"`
	Customers []target `json:"customers"`
}

type named struct {
	Name string `json:"name"`
}

type timer struct {
	Customer    *named    `json:"customer"`
	Project     *named    `json:"project"`
	Description string    `json:"description"`
	StartedAt   time.Time `json:"started_at"`
}

func (t *timer) label() string {
	return describe(t.Project, t.Customer, t.Description)
}

type timerState struct {
	Running        bool    `json:"running"`
	Timer          *timer  `json:"timer"`
	StoppedEntries []entry `json:"stopped_entries"`
}

type entry struct {
	Date        time.Time `json:"date"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	Minutes     int       `json:"minutes"`
	Description string    `json:"description"`
	Customer    *named    `json:"customer"`
	Project     *named    `json:"project"`
}

func (e entry) label() string {
	return describe(e.Project, e.Customer, e.Description)
}

// describe renders "Project (Customer) — description".
func describe(project, customer *named, desc string) string {
	var s string
	switch {
	case project != nil && customer != nil:
		s = project.Name + " (" + customer.Name + ")"
	case project != nil:
		s = project.Name
	case customer != nil:
		s = customer.Name
	default:
		s = "(no project)"
	}
	if desc != "" {
		s += " — " + desc
	}
	return s
}

// matchTarget finds the one target named by query: an exact
// (case-insensitive) name wins, else a unique name containing query. Errors
// list the candidates so the user can be more specific.
func matchTarget(list []target, query, kind string) (*target, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	var exact, partial []target
	for _, t := range list {
		name := strings.ToLower(t.Name)
		switch {
		case name == q:
			exact = append(exact, t)
		case strings.Contains(name, q):
			partial = append(partial, t)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = partial
	}
	switch len(matches) {
	case 1:
		return &matches[0], nil
	case 0:
		return nil, fmt.Errorf("no %s matches %q; run \"warmdesk-timer projects\" to see what you can pick", kind, query)
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Name
		if m.CustomerName != "" {
			names[i] += " (" + m.CustomerName + ")"
		}
	}
	sort.Strings(names)
	hint := ""
	if kind == "project" {
		hint = " (add -c CUSTOMER to pick one)"
	}
	return nil, fmt.Errorf("%q matches several %ss%s: %s", query, kind, hint, strings.Join(names, ", "))
}
