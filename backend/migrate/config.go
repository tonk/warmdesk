package migrate

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for both export and import operations.
type Config struct {
	Coworker CoworkerConfig `yaml:"coworker"`
	Platform PlatformConfig `yaml:"platform"`
	// ColumnMap maps Coworker column names → external platform column/status names.
	// If empty or a name is not found, the Coworker column name is used as-is.
	ColumnMap map[string]string `yaml:"column_map"`
}

// CoworkerConfig holds connection details for the Coworker server.
type CoworkerConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Project  string `yaml:"project"` // project slug
}

// PlatformConfig holds connection details for the external platform.
type PlatformConfig struct {
	Name string `yaml:"name"` // jira | trello | openproject | ryver

	// Jira
	URL        string `yaml:"url"`
	Email      string `yaml:"email"`
	APIToken   string `yaml:"api_token"`
	ProjectKey string `yaml:"project_key"`
	IssueType  string `yaml:"issue_type"` // default: Task

	// Trello
	APIKey  string `yaml:"api_key"`
	Token   string `yaml:"token"`
	BoardID string `yaml:"board_id"`

	// OpenProject (reuses URL, APIKey)
	ProjectID string `yaml:"project_id"`

	// Ryver
	Org string `yaml:"org"`
	// api_token reuses APIToken above
	Team string `yaml:"team"`
}

// LoadConfig reads the YAML config file and applies environment variable
// overrides. Missing required fields are prompted interactively.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	// Load YAML file if it exists.
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Environment variable overrides.
	applyEnv(&cfg.Coworker.URL, "COWORKER_URL")
	applyEnv(&cfg.Coworker.Username, "COWORKER_USERNAME")
	applyEnv(&cfg.Coworker.Password, "COWORKER_PASSWORD")
	applyEnv(&cfg.Coworker.Project, "COWORKER_PROJECT")
	applyEnv(&cfg.Platform.APIToken, "PLATFORM_API_TOKEN")
	applyEnv(&cfg.Platform.APIKey, "PLATFORM_API_KEY")

	// Interactive prompts for required fields.
	promptIfEmpty(&cfg.Coworker.URL, "Coworker URL")
	promptIfEmpty(&cfg.Coworker.Username, "Coworker username")
	promptIfEmpty(&cfg.Coworker.Password, "Coworker password")
	promptIfEmpty(&cfg.Coworker.Project, "Coworker project slug")

	if cfg.Platform.IssueType == "" {
		cfg.Platform.IssueType = "Task"
	}

	return cfg, nil
}

// ReverseColumnMap returns a map from external platform names → Coworker column
// names (the inverse of cfg.ColumnMap).
func ReverseColumnMap(m map[string]string) map[string]string {
	rev := make(map[string]string, len(m))
	for k, v := range m {
		rev[v] = k
	}
	return rev
}

// MapColumn translates a Coworker column name to the external name using the
// column map. If no mapping exists the original name is returned unchanged.
func MapColumn(name string, columnMap map[string]string) string {
	if columnMap == nil {
		return name
	}
	if mapped, ok := columnMap[name]; ok {
		return mapped
	}
	return name
}

// MapColumnReverse translates an external column/status name back to a
// Coworker column name using the reversed column map.
func MapColumnReverse(name string, reverseMap map[string]string) string {
	if reverseMap == nil {
		return name
	}
	if mapped, ok := reverseMap[name]; ok {
		return mapped
	}
	return name
}

// ─── internal helpers ────────────────────────────────────────────────────────

func applyEnv(field *string, key string) {
	if v := os.Getenv(key); v != "" {
		*field = v
	}
}

var stdinReader = bufio.NewReader(os.Stdin)

func promptIfEmpty(field *string, label string) {
	if *field != "" {
		return
	}
	fmt.Printf("%s: ", label)
	val, _ := stdinReader.ReadString('\n')
	*field = strings.TrimSpace(val)
}
