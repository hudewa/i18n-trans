package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Doubao DoubaoConfig `mapstructure:"doubao"`
	Scan   ScanConfig   `mapstructure:"scan"`
	Output OutputConfig `mapstructure:"output"`
}

// DoubaoConfig holds Doubao API configuration
type DoubaoConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// ScanConfig holds scanner configuration
type ScanConfig struct {
	IncludeExt      []string `mapstructure:"include_ext"`
	ExcludeDirs     []string `mapstructure:"exclude_dirs"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
}

// OutputConfig holds output configuration
type OutputConfig struct {
	SQLDir     string `mapstructure:"sql_dir"`
	ModuleName string `mapstructure:"module_name"`
	UpdatedBy  string `mapstructure:"updated_by"`
}

// DefaultAPIKey 默认使用腾讯云提供的 API Key
//const DefaultAPIKey = ""

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Doubao: DoubaoConfig{
			APIKey:  "",
			BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			Model:   "doubao-1.5-pro-32k",
		},
		Scan: ScanConfig{
			IncludeExt:      []string{".go", ".js", ".vue", ".ts", ".tsx", ".jsx", ".json", ".yaml", ".yml"},
			ExcludeDirs:     []string{"node_modules", ".git", "dist", "build", "vendor", ".idea", ".vscode"},
			ExcludePatterns: []string{"*_test.go", "*.min.js"},
		},
		Output: OutputConfig{
			SQLDir:     "./sql",
			ModuleName: "doubao",
			UpdatedBy:  "doubao",
		},
	}
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigType("yaml")

	// Set defaults
	v.SetDefault("doubao", cfg.Doubao)
	v.SetDefault("scan", cfg.Scan)
	v.SetDefault("output", cfg.Output)

	// Try to load from config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Try default locations
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".gamoji-trans"))
		}
		v.AddConfigPath(".")
		v.SetConfigName("config")
	}

	// Read config file if exists
	if err := v.ReadInConfig(); err != nil {
		// It's ok if config file doesn't exist
		// Check for various "not found" error conditions
		configNotFound := false
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			configNotFound = true
		} else if os.IsNotExist(err) {
			configNotFound = true
		} else if err.Error() == "open config.yaml: no such file or directory" {
			configNotFound = true
		}

		if !configNotFound {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults
	}

	// Override with environment variables
	v.SetEnvPrefix("DOUBAO")
	v.AutomaticEnv()

	// Check for API key in environment
	if apiKey := os.Getenv("DOUBAO_API_KEY"); apiKey != "" {
		v.Set("doubao.api_key", apiKey)
	}

	// Unmarshal config
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Doubao.APIKey == "" {
		return fmt.Errorf("doubao API key is required (set via config file or DOUBAO_API_KEY environment variable)")
	}
	if c.Doubao.BaseURL == "" {
		return fmt.Errorf("doubao base URL is required")
	}
	if c.Doubao.Model == "" {
		return fmt.Errorf("doubao model is required")
	}
	return nil
}

// WriteExampleConfig writes an example configuration file
func WriteExampleConfig(path string) error {
	example := `# Gamoji Trans Configuration

# Doubao API Configuration
doubao:
  api_key: "your-api-key-here"
  base_url: "https://ark.cn-beijing.volces.com/api/v3"
  model: "doubao-1.5-pro-32k"

# Scanner Configuration
scan:
  include_ext:
    - ".go"
    - ".js"
    - ".vue"
    - ".ts"
    - ".tsx"
    - ".jsx"
    - ".json"
    - ".yaml"
    - ".yml"
  exclude_dirs:
    - "node_modules"
    - ".git"
    - "dist"
    - "build"
    - "vendor"
    - ".idea"
    - ".vscode"
  exclude_patterns:
    - "*_test.go"
    - "*.min.js"

# Output Configuration
output:
  sql_dir: "./sql"
  module_name: "doubao"
  updated_by: "doubao"
`
	return os.WriteFile(path, []byte(example), 0644)
}
