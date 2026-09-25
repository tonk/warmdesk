// warmdesk-timer starts and stops WarmDesk's time-tracking timer from the
// command line. Stopping books the elapsed time — rounded up to whole 15
// minutes and split at midnight — as time entries on the chosen project.
//
// Usage:
//
//	warmdesk-timer start [-c CUSTOMER] [-m DESCRIPTION] [PROJECT]
//	warmdesk-timer stop [--at HH:MM]
//	warmdesk-timer status
//	warmdesk-timer cancel
//	warmdesk-timer projects
//
// Starting while a timer runs stops and books the running one first.
//
// Connection settings, first match wins: the --url/--key flags, the
// WARMDESK_URL/WARMDESK_API_KEY environment variables, then the config file
// (default: <user config dir>/warmdesk/timer.yaml) with "url:" and
// "api_key:". The key must be a personal API key (Settings → API Keys);
// project-scoped keys cannot use the timer.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const usage = `warmdesk-timer — start and stop WarmDesk time tracking

Usage:
  warmdesk-timer start [-c CUSTOMER] [-m DESCRIPTION] [PROJECT]
                               start a timer (stops and books a running one)
  warmdesk-timer stop [--at HH:MM]
                               stop and book the time (--at: when you actually stopped)
  warmdesk-timer status        show the running timer
  warmdesk-timer cancel        discard the running timer without booking
  warmdesk-timer projects      list the projects and customers you can pick
  warmdesk-timer version

Time is rounded up to whole 15 minutes and split at midnight.
Names match case-insensitively; a unique part of a name is enough.

Global flags (before the command):
  --url URL        WarmDesk base URL          (env WARMDESK_URL)
  --key KEY        personal API key           (env WARMDESK_API_KEY)
  --config FILE    config file with url: and api_key:
                   (default %s)
`

func main() {
	if err := run(os.Args[1:], os.Stdout, time.Now()); err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintf(os.Stderr, "warmdesk-timer: %v\n\nRun \"warmdesk-timer help\" for usage.\n", err)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "warmdesk-timer: %v\n", err)
		os.Exit(1)
	}
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...interface{}) error { return usageError{fmt.Sprintf(format, a...)} }

type config struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "timer.yaml"
	}
	return filepath.Join(dir, "warmdesk", "timer.yaml")
}

// loadConfig merges flags over environment variables over the config file.
// A missing config file is fine as long as url and key end up set.
func loadConfig(path, flagURL, flagKey string, explicitPath bool) (config, error) {
	var cfg config
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config file %s: %w", path, err)
		}
	case explicitPath || !errors.Is(err, os.ErrNotExist):
		return cfg, fmt.Errorf("config file: %w", err)
	}
	for _, o := range []struct {
		dst *string
		val string
	}{
		{&cfg.URL, os.Getenv("WARMDESK_URL")}, {&cfg.APIKey, os.Getenv("WARMDESK_API_KEY")},
		{&cfg.URL, flagURL}, {&cfg.APIKey, flagKey},
	} {
		if o.val != "" {
			*o.dst = o.val
		}
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.URL == "" || cfg.APIKey == "" {
		return cfg, fmt.Errorf("WarmDesk URL and API key are required: set WARMDESK_URL and WARMDESK_API_KEY, "+
			"use --url/--key, or put url: and api_key: in %s", path)
	}
	return cfg, nil
}

func run(args []string, out io.Writer, now time.Time) error {
	global := flag.NewFlagSet("warmdesk-timer", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	flagURL := global.String("url", "", "")
	flagKey := global.String("key", "", "")
	configPath := global.String("config", defaultConfigPath(), "")
	if err := global.Parse(args); err != nil {
		return usagef("%v", err)
	}
	explicitConfig := false
	global.Visit(func(f *flag.Flag) { explicitConfig = explicitConfig || f.Name == "config" })

	rest := global.Args()
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help" {
		fmt.Fprintf(out, usage, defaultConfigPath())
		return nil
	}
	cmd, cmdArgs := rest[0], rest[1:]
	if cmd == "version" {
		fmt.Fprintln(out, version)
		return nil
	}

	cfg, err := loadConfig(*configPath, *flagURL, *flagKey, explicitConfig)
	if err != nil {
		return err
	}
	api := newClient(cfg)

	switch cmd {
	case "start":
		return cmdStart(api, cmdArgs, out)
	case "stop":
		return cmdStop(api, cmdArgs, out, now)
	case "status":
		return cmdStatus(api, out, now)
	case "cancel":
		if err := api.do("DELETE", "/timer", nil, nil); err != nil {
			return err
		}
		fmt.Fprintln(out, "Timer discarded; nothing was booked.")
		return nil
	case "projects":
		return cmdProjects(api, out)
	default:
		return usagef("unknown command %q", cmd)
	}
}

// parseInterleaved lets flags and positional arguments appear in any order
// ("start Website -m x" as well as "start -m x Website").
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usagef("%v", err)
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func cmdStart(api *client, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	customer := fs.String("c", "", "")
	fs.StringVar(customer, "customer", "", "")
	desc := fs.String("m", "", "")
	fs.StringVar(desc, "message", "", "")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usagef("give one project name (quote names with spaces)")
	}
	project := ""
	if len(pos) == 1 {
		project = pos[0]
	}
	if project == "" && *customer == "" {
		return usagef("start needs a project, a customer (-c), or both")
	}

	var targets targetList
	if err := api.do("GET", "/timer/targets", nil, &targets); err != nil {
		return err
	}
	body := map[string]interface{}{"description": *desc}
	if zone := localZoneName(); zone != "" {
		body["time_zone"] = zone
	}
	var cust *target
	if *customer != "" {
		if cust, err = matchTarget(targets.Customers, *customer, "customer"); err != nil {
			return err
		}
		body["customer_id"] = cust.ID
	}
	if project != "" {
		candidates := targets.Projects
		if cust != nil {
			// Board projects of another customer can't be combined with it.
			candidates = nil
			for _, p := range targets.Projects {
				if p.CustomerID == nil || *p.CustomerID == cust.ID {
					candidates = append(candidates, p)
				}
			}
		}
		p, err := matchTarget(candidates, project, "project")
		if err != nil {
			return err
		}
		body["project_id"] = p.ID
	}

	var res timerState
	if err := api.do("POST", "/timer/start", body, &res); err != nil {
		return err
	}
	if len(res.StoppedEntries) > 0 {
		printBooked(out, res.StoppedEntries)
	}
	fmt.Fprintf(out, "Started %s at %s.\n", res.Timer.label(), res.Timer.StartedAt.Local().Format("15:04"))
	return nil
}

func cmdStop(api *client, args []string, out io.Writer, now time.Time) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	at := fs.String("at", "", "")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return usagef("stop takes no arguments")
	}
	var body interface{}
	if *at != "" {
		end, err := parseAt(*at, now)
		if err != nil {
			return err
		}
		body = map[string]string{"end": end.Format(time.RFC3339)}
	}
	var entries []entry
	if err := api.do("POST", "/timer/stop", body, &entries); err != nil {
		return err
	}
	printBooked(out, entries)
	return nil
}

// parseAt turns "HH:MM" into the most recent such moment not after now:
// today, or yesterday when that time hasn't come yet today.
func parseAt(s string, now time.Time) (time.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, usagef("--at wants HH:MM (e.g. 17:30), got %q", s)
	}
	end := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if end.After(now) {
		end = end.AddDate(0, 0, -1)
	}
	return end, nil
}

func cmdStatus(api *client, out io.Writer, now time.Time) error {
	var st timerState
	if err := api.do("GET", "/timer", nil, &st); err != nil {
		return err
	}
	if !st.Running || st.Timer == nil {
		fmt.Fprintln(out, "No timer running.")
		return nil
	}
	fmt.Fprintf(out, "Running: %s — since %s, %s\n", st.Timer.label(),
		st.Timer.StartedAt.Local().Format("15:04"), formatMinutes(int(now.Sub(st.Timer.StartedAt)/time.Minute)))
	return nil
}

func cmdProjects(api *client, out io.Writer) error {
	var targets targetList
	if err := api.do("GET", "/timer/targets", nil, &targets); err != nil {
		return err
	}
	fmt.Fprintln(out, "Projects:")
	if len(targets.Projects) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, p := range targets.Projects {
		switch {
		case p.TimeTrackingOnly:
			fmt.Fprintf(out, "  %s  (time tracking; any customer, -c)\n", p.Name)
		case p.CustomerName != "":
			fmt.Fprintf(out, "  %s  (%s)\n", p.Name, p.CustomerName)
		default:
			fmt.Fprintf(out, "  %s\n", p.Name)
		}
	}
	fmt.Fprintln(out, "Customers:")
	if len(targets.Customers) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, c := range targets.Customers {
		fmt.Fprintf(out, "  %s\n", c.Name)
	}
	return nil
}

func printBooked(out io.Writer, entries []entry) {
	if len(entries) == 0 {
		return
	}
	total := 0
	for _, e := range entries {
		total += e.Minutes
	}
	fmt.Fprintf(out, "Booked %s on %s:\n", formatMinutes(total), entries[0].label())
	for _, e := range entries {
		fmt.Fprintf(out, "  %s  %s–%s  %s\n", e.Date.Format("2006-01-02"), e.StartTime, e.EndTime, formatMinutes(e.Minutes))
	}
}

func formatMinutes(m int) string {
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}

// localZoneName returns this machine's IANA time zone, so the server books
// dates and times on the user's own wall clock. Empty when unknown (the
// server then uses the user's time zone setting).
func localZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}
	if name := time.Local.String(); name != "Local" && name != "" {
		return name
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i >= 0 {
			return link[i+len("zoneinfo/"):]
		}
	}
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}
