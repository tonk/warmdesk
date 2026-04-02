// coworker-export reads a Coworker project and pushes it to Jira, Trello,
// OpenProject, or Ryver.
//
// Usage:
//
//	coworker-export [--config FILE] [--dry-run]
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
	dryRun := flag.Bool("dry-run", false, "print what would be exported without making API calls")
	flag.Parse()

	cfg, err := migrate.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Prompt for any required fields still missing
	cfg.Platform.Name = promptPlatform(cfg.Platform.Name)

	fmt.Printf("Coworker export\n")
	fmt.Printf("  source  : %s (project: %s)\n", cfg.Coworker.URL, cfg.Coworker.Project)
	fmt.Printf("  target  : %s\n", strings.ToLower(cfg.Platform.Name))

	// Authenticate with Coworker
	fmt.Printf("\nConnecting to Coworker...\n")
	token, err := migrate.Login(cfg.Coworker.URL, cfg.Coworker.Username, cfg.Coworker.Password)
	if err != nil {
		log.Fatalf("login: %v", err)
	}

	// Read project
	fmt.Printf("Reading project %q...\n", cfg.Coworker.Project)
	project, err := migrate.ReadProject(cfg.Coworker.URL, token, cfg.Coworker.Project)
	if err != nil {
		log.Fatalf("read project: %v", err)
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
		fmt.Println("\n[dry-run] no changes made to target platform")
		return
	}

	// Export
	fmt.Printf("\nExporting to %s...\n", strings.Title(strings.ToLower(cfg.Platform.Name)))
	switch strings.ToLower(cfg.Platform.Name) {
	case "jira":
		err = migrate.ExportToJira(cfg.Platform, project, cfg.ColumnMap)
	case "trello":
		err = migrate.ExportToTrello(cfg.Platform, project, cfg.ColumnMap)
	case "openproject":
		err = migrate.ExportToOpenProject(cfg.Platform, project, cfg.ColumnMap)
	case "ryver":
		err = migrate.ExportToRyver(cfg.Platform, project, cfg.ColumnMap)
	default:
		log.Fatalf("unknown platform %q — must be jira, trello, openproject, or ryver", cfg.Platform.Name)
	}
	if err != nil {
		log.Fatalf("export: %v", err)
	}

	fmt.Printf("\n✓ export complete\n")
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
