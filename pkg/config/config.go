package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Trans  TransConfig  `mapstructure:"trans"`
	Doubao DoubaoConfig `mapstructure:"doubao"`
	Scan   ScanConfig   `mapstructure:"scan"`
	Output OutputConfig `mapstructure:"output"`
}

// TransConfig holds translation API configuration (new format)
type TransConfig struct {
	KeyID   string `mapstructure:"keyId"`
	BaseURL string `mapstructure:"baseURL"`
	Model   string `mapstructure:"model"`
}

// DoubaoConfig holds Doubao API configuration
type DoubaoConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// ScanConfig holds scanner configuration
type ScanConfig struct {
	Dir             []string `mapstructure:"dir"`              // 要扫描的目录列表，为空则扫描当前目录
	IncludeExt      []string `mapstructure:"include_ext"`      // 包含的文件扩展名
	ExcludeDirs     []string `mapstructure:"exclude_dirs"`     // 排除的目录
	ExcludePatterns []string `mapstructure:"exclude_patterns"` // 排除的文件模式
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
		Trans: TransConfig{
			KeyID:   "",
			BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			Model:   "doubao-1-5-pro-32k-250115",
		},
		Doubao: DoubaoConfig{
			APIKey:  "",
			BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			Model:   "doubao-1-5-pro-32k-250115",
		},
		Scan: ScanConfig{
			Dir:             []string{}, // 为空时默认扫描当前目录
			IncludeExt:      []string{".go"}, // 只扫描 Go 文件
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
// Priority: 1. 指定路径 2. configs/config.yaml (当前目录) 3. config.yaml (当前目录) 4. ~/.gamoji-trans/config 5. 环境变量
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigType("yaml")

	// Set defaults
	v.SetDefault("trans", cfg.Trans)
	v.SetDefault("doubao", cfg.Doubao)
	v.SetDefault("scan", cfg.Scan)
	v.SetDefault("scan.dir", cfg.Scan.Dir)
	v.SetDefault("output", cfg.Output)

	configFileUsed := ""

	// Try to load from config file
	if configPath != "" {
		// 使用指定的配置文件路径
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			if !isConfigNotFound(err) {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
			// 文件不存在，继续
		} else {
			configFileUsed = configPath
		}
	} else {
		// Priority 1: configs/config.yaml (当前目录下的 configs 文件夹)
		if _, err := os.Stat("configs/config.yaml"); err == nil {
			v.SetConfigFile("configs/config.yaml")
			if err := v.ReadInConfig(); err == nil {
				configFileUsed = "configs/config.yaml"
			}
		}

		// Priority 2: config.yaml (当前目录)
		if configFileUsed == "" {
			if _, err := os.Stat("config.yaml"); err == nil {
				// 创建新的 viper 实例，避免之前的设置干扰
				v = viper.New()
				v.SetConfigType("yaml")
				v.SetConfigFile("config.yaml")
				if err := v.ReadInConfig(); err == nil {
					configFileUsed = "config.yaml"
				}
			}
		}

		// Priority 3: ~/.gamoji-trans/config
		if configFileUsed == "" {
			home, err := os.UserHomeDir()
			if err == nil {
				globalConfig := filepath.Join(home, ".gamoji-trans/config")
				if _, err := os.Stat(globalConfig); err == nil {
					v = viper.New()
					v.SetConfigType("yaml")
					v.SetConfigFile(globalConfig)
					if err := v.ReadInConfig(); err == nil {
						configFileUsed = globalConfig
					}
				}
			}
		}
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

	// 如果没有找到配置文件，返回特殊标记（需要在同步前设置）
	if configFileUsed == "" {
		cfg.Trans.KeyID = "__NO_CONFIG_FILE__"
	} else {
		// 确保配置文件的值被正确读取
		// 如果 trans.keyId 为空但 doubao.api_key 有值，同步过去
		if cfg.Trans.KeyID == "" && cfg.Doubao.APIKey != "" {
			cfg.Trans.KeyID = cfg.Doubao.APIKey
		}
	}

	// 将 trans 配置同步到 doubao（保持兼容性）
	// 注意：如果 KeyID 是 "__NO_CONFIG_FILE__"，则不会覆盖
	if cfg.Trans.KeyID != "" && cfg.Trans.KeyID != "__NO_CONFIG_FILE__" {
		cfg.Doubao.APIKey = cfg.Trans.KeyID
	}
	if cfg.Trans.BaseURL != "" {
		cfg.Doubao.BaseURL = cfg.Trans.BaseURL
	}
	if cfg.Trans.Model != "" {
		cfg.Doubao.Model = cfg.Trans.Model
	}

	return cfg, nil
}

// isConfigNotFound checks if the error is a "config file not found" error
func isConfigNotFound(err error) bool {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	if os.IsNotExist(err) {
		return true
	}
	if err.Error() == "open config.yaml: no such file or directory" {
		return true
	}
	return false
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// 检查是否没有找到配置文件
	if c.Trans.KeyID == "__NO_CONFIG_FILE__" {
		return fmt.Errorf(`未找到配置文件

请在以下位置之一创建配置文件：
  1. configs/config.yaml  (推荐，项目级配置)
  2. config.yaml          (当前目录)
  3. ~/.gamoji-trans/config  (用户级配置)

配置格式如下：
─────────────────────────────────
trans:
  keyId: "4f9f124d-a094-4fea-9967-843e5994161a"
  baseURL: "https://ark.cn-beijing.volces.com/api/v3"
  model: "doubao-1-5-pro-32k-250115"
─────────────────────────────────

或者设置环境变量：
  export DOUBAO_API_KEY="your-api-key"`)
	}

	if c.Doubao.APIKey == "" {
		return fmt.Errorf("API key 未配置\n\n请在配置文件中设置 trans.keyId，或设置环境变量 DOUBAO_API_KEY")
	}
	if c.Doubao.BaseURL == "" {
		return fmt.Errorf("baseURL 未配置\n\n请在配置文件中设置 trans.baseURL")
	}
	if c.Doubao.Model == "" {
		return fmt.Errorf("model 未配置\n\n请在配置文件中设置 trans.model")
	}
	return nil
}

// WriteExampleConfig writes an example configuration file
func WriteExampleConfig(path string) error {
	example := `# i18n-trans Configuration
# 配置文件支持以下位置（按优先级排序）：
#   1. configs/config.yaml  (项目级配置，推荐)
#   2. config.yaml          (当前目录)
#   3. ~/.gamoji-trans/config  (用户级全局配置)

# 翻译 API 配置（必填）
trans:
  keyId: "your-api-key-here"                    # 豆包 API Key
  baseURL: "https://ark.cn-beijing.volces.com/api/v3"  # API 基础地址
  model: "doubao-1-5-pro-32k-250115"            # 模型名称

# 扫描器配置（可选，使用默认值可省略）
scan:
  # 要扫描的目录列表（可选）
  # 如果为空或未设置，默认扫描当前目录 "."
  # 可以指定多个目录，如: ["./src", "./components", "./pages"]
  dir: []
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

# 输出配置（可选，使用默认值可省略）
output:
  sql_dir: "./sql"
  module_name: "doubao"
  updated_by: "doubao"
`
	return os.WriteFile(path, []byte(example), 0644)
}
