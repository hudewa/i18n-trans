package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hudewa/i18n-trans/internal/translator"
)

func TestEscapeSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"it's", "it''s"},
		{"don't worry", "don''t worry"},
		{"", ""},
		{"no quotes", "no quotes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeSQL(tt.input)
			if result != tt.expected {
				t.Errorf("escapeSQL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"短文本", "短文本"},
		{"这是一个超过八个字符的长文本", "这是一个超过八个字符的长文"},
		{"", ""},
		{"正好八个", "正好八个"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := generateID(tt.input)
			if result != tt.expected {
				t.Errorf("generateID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	g := New("./output", "testmodule", "testuser")

	if g == nil {
		t.Fatal("New() returned nil")
	}

	if g.outputDir != "./output" {
		t.Errorf("outputDir = %q, want %q", g.outputDir, "./output")
	}

	if g.moduleName != "testmodule" {
		t.Errorf("moduleName = %q, want %q", g.moduleName, "testmodule")
	}
}

func TestGenerateSQLContent(t *testing.T) {
	g := New("./sql", "test", "user")

	results := []translator.TranslationResult{
		{
			Text: "你好",
			ID:   "abc123",
			Zh:   "你好",
			En:   "Hello",
			Id:   "Halo",
			Th:   "สวัสดี",
			Vi:   "Xin chào",
			Ms:   "Hai",
		},
		{
			Text:  "失败文本",
			Error: assert.AnError,
		},
	}

	content := g.generateSQLContent(results)

	// 检查包含 INSERT 语句
	if !strings.Contains(content, "INSERT INTO") {
		t.Error("SQL content should contain INSERT INTO")
	}

	// 检查包含成功翻译的文本
	if !strings.Contains(content, "你好") {
		t.Error("SQL content should contain Chinese text")
	}

	// 检查包含英文翻译
	if !strings.Contains(content, "Hello") {
		t.Error("SQL content should contain English translation")
	}

	// 检查包含失败注释
	if !strings.Contains(content, "-- FAILED:") {
		t.Error("SQL content should contain failure comment")
	}

	// 检查包含时间戳
	if !strings.Contains(content, "Generated at:") {
		t.Error("SQL content should contain generation timestamp")
	}
}

func TestGenerateSQL(t *testing.T) {
	tmpDir := t.TempDir()
	g := New(tmpDir, "test", "user")

	results := []translator.TranslationResult{
		{
			Text: "测试",
			ID:   "test123",
			Zh:   "测试",
			En:   "Test",
			Id:   "Tes",
			Th:   "ทดสอบ",
			Vi:   "Kiểm tra",
			Ms:   "Ujian",
		},
	}

	filepath, err := g.GenerateSQL(results)
	if err != nil {
		t.Fatalf("GenerateSQL failed: %v", err)
	}

	// 检查文件是否创建
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		t.Error("SQL file was not created")
	}

	// 检查文件路径包含时间戳格式
	if !strings.Contains(filepath, "i18n_") || !strings.Contains(filepath, ".sql") {
		t.Error("Filename should contain i18n_ prefix and .sql extension")
	}

	// 读取并验证内容
	content, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("Failed to read SQL file: %v", err)
	}

	if !strings.Contains(string(content), "测试") {
		t.Error("SQL file should contain the Chinese text")
	}
}

func TestGenerateReport(t *testing.T) {
	g := New("./sql", "test", "user")

	results := []translator.TranslationResult{
		{Text: "成功1", Error: nil},
		{Text: "成功2", Error: nil},
		{Text: "失败1", Error: assert.AnError},
	}

	report := g.GenerateReport(results)

	// 检查包含报告标题
	if !strings.Contains(report, "Translation Report") {
		t.Error("Report should contain title")
	}

	// 检查包含总数
	if !strings.Contains(report, "Total: 3") {
		t.Error("Report should show total count")
	}

	// 检查包含成功数
	if !strings.Contains(report, "Success: 2") {
		t.Error("Report should show success count")
	}

	// 检查包含失败数
	if !strings.Contains(report, "Failed: 1") {
		t.Error("Report should show failed count")
	}
}

func TestGenerateReportAllSuccess(t *testing.T) {
	g := New("./sql", "test", "user")

	results := []translator.TranslationResult{
		{Text: "成功1", Error: nil},
		{Text: "成功2", Error: nil},
	}

	report := g.GenerateReport(results)

	// 全部成功时不应显示失败项列表
	if strings.Contains(report, "Failed items:") {
		t.Error("Report should not show failed items section when all succeed")
	}
}

// assert 包模拟
var assert = &Assert{}

type Assert struct{}

func (a *Assert) Errorf(t testing.TB, format string, args ...interface{}) {
	t.Errorf(format, args...)
}

var AnError = &testError{}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
