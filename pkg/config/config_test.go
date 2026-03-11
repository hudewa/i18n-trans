package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// 检查 Doubao 配置
	if cfg.Doubao.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}

	if cfg.Doubao.Model == "" {
		t.Error("Model should not be empty")
	}

	// 检查 Scan 配置
	if len(cfg.Scan.IncludeExt) == 0 {
		t.Error("IncludeExt should not be empty")
	}

	if len(cfg.Scan.ExcludeDirs) == 0 {
		t.Error("ExcludeDirs should not be empty")
	}

	// 检查 Output 配置
	if cfg.Output.SQLDir == "" {
		t.Error("SQLDir should not be empty")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "完整配置",
			cfg: &Config{
				Doubao: DoubaoConfig{
					APIKey:  "test-key",
					BaseURL: "https://test.com",
					Model:   "test-model",
				},
			},
			wantErr: false,
		},
		{
			name: "缺少 API Key",
			cfg: &Config{
				Doubao: DoubaoConfig{
					APIKey:  "",
					BaseURL: "https://test.com",
					Model:   "test-model",
				},
			},
			wantErr: true,
		},
		{
			name: "缺少 BaseURL",
			cfg: &Config{
				Doubao: DoubaoConfig{
					APIKey:  "test-key",
					BaseURL: "",
					Model:   "test-model",
				},
			},
			wantErr: true,
		},
		{
			name: "缺少 Model",
			cfg: &Config{
				Doubao: DoubaoConfig{
					APIKey:  "test-key",
					BaseURL: "https://test.com",
					Model:   "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteExampleConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := WriteExampleConfig(configPath)
	if err != nil {
		t.Fatalf("WriteExampleConfig failed: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// 读取并验证内容
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)

	// 检查包含关键配置项
	if !contains(contentStr, "doubao:") {
		t.Error("Config should contain doubao section")
	}

	if !contains(contentStr, "api_key:") {
		t.Error("Config should contain api_key")
	}

	if !contains(contentStr, "scan:") {
		t.Error("Config should contain scan section")
	}

	if !contains(contentStr, "output:") {
		t.Error("Config should contain output section")
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
doubao:
  api_key: "test-api-key"
  base_url: "https://custom.api.com"
  model: "custom-model"
scan:
  include_ext:
    - ".go"
output:
  sql_dir: "./custom-sql"
  module_name: "custom"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Doubao.APIKey != "test-api-key" {
		t.Errorf("APIKey = %q, want %q", cfg.Doubao.APIKey, "test-api-key")
	}

	if cfg.Doubao.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.Doubao.BaseURL, "https://custom.api.com")
	}

	if cfg.Output.SQLDir != "./custom-sql" {
		t.Errorf("SQLDir = %q, want %q", cfg.Output.SQLDir, "./custom-sql")
	}
}

func TestLoadWithEnvVar(t *testing.T) {
	// 设置环境变量
	os.Setenv("DOUBAO_API_KEY", "env-api-key")
	defer os.Unsetenv("DOUBAO_API_KEY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Doubao.APIKey != "env-api-key" {
		t.Errorf("APIKey = %q, want %q", cfg.Doubao.APIKey, "env-api-key")
	}
}

func TestLoadNoConfigFile(t *testing.T) {
	// 加载不存在的配置文件应该使用默认值
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load should not fail when config file doesn't exist: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load should return default config when file doesn't exist")
	}

	// 验证使用了默认值
	if cfg.Doubao.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" {
		t.Error("Should use default BaseURL")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSub(s, substr))
}

func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
