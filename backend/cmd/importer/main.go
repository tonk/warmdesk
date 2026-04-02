// coworker-import reads a project from Jira, Trello, OpenProject, or Ryver
// and creates it in Coworker.
//
// Usage:
//
//	coworker-import [--config FILE] [--dry-run]
//
// Required fields can be supplied in the config file, as environment variables,
// or interactively when the program prompts for them.
//
// Environment variable overrides:
//
//	COWORKER_URL, COWORKER_USERNAME, COWORKER_PASSWORD, COWORKER_PROJECT
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
	configFile := flag.String("config", "coworker-migrate.yaml", "path to migration config file")
	dryRun := flag.Bool("dry-run", false, "print what would be imported without writing to Coworker")
	flag.Parse()

	cfg, err := migrate.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Prompt for any required fields still missing
	cfg.Platform.Name = promptPlatform(cfg.Platform.Name)

	fmt.Printf("Coworker import\n")
	fmt.Printf("  source  : %s\n", strings.ToLower(cfg.Platform.Name))
	fmt.Printf("  target  : %s (project will be created)\n", cfg.Coworker.URL)

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
		fmt.Println("\n[dry-run] no changes made to Coworker")
		return
	}

	// Authenticate with Coworker
	fmt.Printf("\nConnecting to Coworker...\n")
	token, err := migrate.Login(cfg.Coworker.URL, cfg.Coworker.Username, cfg.Coworker.Password)
	if err != nil {
		log.Fatalf("login: %v", err)
	}

	// Write to Coworker
	fmt.Printf("Creating project in Coworker...\n")
	// For import, ColumnMap is used in reverse: reverse map was already applied
	// during ReadFrom*, so we pass nil here to preserve the column names as-is.
	if err := migrate.WriteProject(cfg.Coworker.URL, token, project, nil); err != nil {
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
