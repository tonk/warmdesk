// warmdesk-import reads a project from Jira, Trello, OpenProject, or Ryver
// and creates it in WarmDesk.
//
// Usage:
//
//	warmdesk-import [--config FILE] [--dry-run]
//
// Required fields can be supplied in the config file, as environment variables,
// or interactively when the program prompts for them.
//
// Environment variable overrides:
//
//	WARMDESK_URL, WARMDESK_USERNAME, WARMDESK_PASSWORD, WARMDESK_PROJECT
//	PLATFORM_API_TOKEN, PLATFORM_API_KEY
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/tonk/warmdesk/migrate"
)

func main() {
	configFile := flag.String("config", "warmdesk-migrate.yaml", "path to migration config file")
	dryRun := flag.Bool("dry-run", false, "print what would be imported without writing to WarmDesk")
	flag.Parse()

	cfg, err := migrate.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Prompt for any required fields still missing
	cfg.Platform.Name = promptPlatform(cfg.Platform.Name)

	fmt.Printf("WarmDesk import\n")
	fmt.Printf("  source  : %s\n", strings.ToLower(cfg.Platform.Name))
	fmt.Printf("  target  : %s (project will be created)\n", cfg.WarmDesk.URL)

	// Read project from source platform
	fmt.Printf("\nReading from %s...\n", strings.Title(strings.ToLower(cfg.Platform.Name)))
	var project *migrate.Project
	switch strings.ToLower(cfg.Platform.Name) {
	case "jira":
		project, err = migrate.ImportFromJira(cfg.Platform, cfg.ColumnMap)
	case "trello":
		project, err = migrate.ImportFromTrello(cfg.Platform, cfg.ColumnMap)
	case "openproject":
		project, err = migrate.ImportFromOpenProject(cfg.Platform, cfg.ColumnMap)
	case "ryver":
		project, err = migrate.ImportFromRyver(cfg.Platform, cfg.ColumnMap)
	default:
		log.Fatalf("unknown platform %q — must be jira, trello, openproject, or ryver", cfg.Platform.Name)
	}
	if err != nil {
		log.Fatalf("read from %s: %v", cfg.Platform.Name, err)
	}

	// Summary
	totalCards := 0
	for _, col := range project.Columns {
		totalCards += len(col.Cards)
	}
	fmt.Printf("\nProject: %s\n", project.Name)
	fmt.Printf("  %d column(s), %d card(s), %d topic(s)\n",
		len(project.Columns), totalCards, len(project.Topics))
	for _, col := range project.Columns {
		fmt.Printf("  %-20s  %d cards\n", col.Name, len(col.Cards))
	}

	if *dryRun {
		fmt.Println("\n[dry-run] no changes made to WarmDesk")
		return
	}

	// Authenticate with WarmDesk
	fmt.Printf("\nConnecting to WarmDesk...\n")
	token, err := migrate.Login(cfg.WarmDesk.URL, cfg.WarmDesk.Username, cfg.WarmDesk.Password)
	if err != nil {
		log.Fatalf("login: %v", err)
	}

	// Write to WarmDesk
	fmt.Printf("Creating project in WarmDesk...\n")
	// For import, ColumnMap is used in reverse: reverse map was already applied
	// during ReadFrom*, so we pass nil here to preserve the column names as-is.
	if err := migrate.WriteProject(cfg.WarmDesk.URL, token, project, nil); err != nil {
		log.Fatalf("write project: %v", err)
	}

	fmt.Printf("\n✓ import complete\n")
}

func promptPlatform(current string) string {
	if current != "" {
		return current
	}
	fmt.Printf("Platform (jira|trello|openproject|ryver): ")
	var s string
	fmt.Scanln(&s)
	return strings.TrimSpace(s)
}
