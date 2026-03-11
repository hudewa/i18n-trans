package translator

import (
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "普通JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "带json标记的代码块",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "无标记代码块",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "带前后空格",
			input:    "  {\"key\": \"value\"}  ",
			expected: `{"key": "value"}`,
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildBatchPrompt(t *testing.T) {
	texts := []string{"你好", "世界"}
	prompt := buildBatchPrompt(texts)

	// 检查提示词包含关键信息
	if prompt == "" {
		t.Error("buildBatchPrompt returned empty string")
	}

	// 检查包含所有文本
	for _, text := range texts {
		if !containsSubstring(prompt, text) {
			t.Errorf("Prompt should contain %q", text)
		}
	}

	// 检查包含JSON格式说明
	if !containsSubstring(prompt, "JSON") {
		t.Error("Prompt should mention JSON")
	}
}

func containsSubstring(s, substr string) bool {
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

func TestNew(t *testing.T) {
	apiKey := "test-api-key"
	baseURL := "https://test.api.com"
	model := "test-model"

	trans := New(apiKey, baseURL, model)

	if trans == nil {
		t.Fatal("New() returned nil")
	}

	if trans.model != model {
		t.Errorf("model = %q, want %q", trans.model, model)
	}
}

func TestTranslationResultStructure(t *testing.T) {
	result := TranslationResult{
		Text:  "你好",
		ID:    "abc123",
		Zh:    "你好",
		En:    "Hello",
		Id:    "Halo",
		Th:    "สวัสดี",
		Vi:    "Xin chào",
		Ms:    "Hai",
		Error: nil,
	}

	if result.Text != "你好" {
		t.Error("Text field mismatch")
	}

	if result.En != "Hello" {
		t.Error("En field mismatch")
	}
}
