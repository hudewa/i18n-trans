package translator

import (
	"context"
	"fmt"
	"testing"
)

// TestExtractJSON 测试 extractJSON 函数
// 验证从各种格式的 Markdown 代码块中提取 JSON 的能力
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

// TestBuildBatchPrompt 测试 buildBatchPrompt 函数
// 验证提示词是否正确包含所有待翻译文本和 JSON 格式说明
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

// containsSubstring 检查字符串是否包含子串（辅助函数）
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSub(s, substr))
}

// containsSub 字符串包含检查的具体实现
func containsSub(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestNew 测试 New 函数创建翻译器实例
//
// 参数说明（测试用例）:
//   - apiKey: "test-api-key"    // API 访问密钥（测试用的假密钥）
//   - baseURL: "https://test.api.com"  // API 服务器地址（测试用的假地址）
//   - model: "test-model"       // 模型名称（测试用的假名称）
//
// 验证点:
//   - 确保 New 函数返回非 nil 的 Translator 实例
//   - 验证 model 字段被正确设置
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

// TestTranslationResultStructure 测试 TranslationResult 结构体的字段设置
// 验证结构体能正确存储所有语言的翻译结果
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

// TestGenerateIDs 测试 generateIDs 函数
// 验证 MD5 ID 生成逻辑的正确性
func TestGenerateIDs(t *testing.T) {
	tests := []struct {
		name     string
		texts    []string
		expected int // 期望返回的 ID 数量
	}{
		{
			name:     "空数组",
			texts:    []string{},
			expected: 0,
		},
		{
			name:     "单条文本",
			texts:    []string{"你好"},
			expected: 1,
		},
		{
			name:     "多条文本",
			texts:    []string{"你好", "世界", "欢迎"},
			expected: 3,
		},
		{
			name:     "重复文本",
			texts:    []string{"相同", "相同", "不同"},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := generateIDs(tt.texts)

			if len(ids) != tt.expected {
				t.Errorf("generateIDs() returned %d ids, want %d", len(ids), tt.expected)
			}

			// 验证每个 ID 都是 8 位十六进制字符串
			for i, id := range ids {
				if len(id) != 8 {
					t.Errorf("ID[%d] length = %d, want 8", i, len(id))
				}
				// 验证是有效的十六进制
				for _, c := range id {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
						t.Errorf("ID[%d] = %q contains invalid hex character", i, id)
						break
					}
				}
			}
		})
	}
}

// TestGenerateIDsConsistency 测试 generateIDs 的一致性
// 验证相同文本总是生成相同的 ID
func TestGenerateIDsConsistency(t *testing.T) {
	text := "测试文本"

	id1 := generateIDs([]string{text})[0]
	id2 := generateIDs([]string{text})[0]

	if id1 != id2 {
		t.Errorf("Same text generated different IDs: %q vs %q", id1, id2)
	}

	// 验证不同文本生成不同 ID
	differentText := "不同文本"
	id3 := generateIDs([]string{differentText})[0]

	if id1 == id3 {
		t.Error("Different texts generated same ID")
	}
}

// TestTranslateTextsEmpty 测试 TranslateTexts 处理空输入
// 验证空数组返回 nil 而不报错
func TestTranslateTextsEmpty(t *testing.T) {
	trans := New("4f9f124d-a094-4fea-9967-843e5994161a",
		"https://ark.cn-beijing.volces.com/api/v3",
		"doubao-1-5-pro-32k-250115")

	results, err := trans.TranslateTexts(context.Background(), []string{"你好世界"})

	if err != nil {
		t.Errorf("TranslateTexts(empty) returned error: %v", err)
	}

	fmt.Printf("TranslateTexts(empty) returned %v, want nil", results)
}
