package replacer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hudewa/i18n-trans/internal/scanner"
)

func TestNew(t *testing.T) {
	r := New("testmodule", true)

	if r == nil {
		t.Fatal("New() returned nil")
	}

	if r.moduleName != "testmodule" {
		t.Errorf("moduleName = %q, want %q", r.moduleName, "testmodule")
	}

	if !r.dryRun {
		t.Error("dryRun should be true")
	}
}

func TestSortMatchesByLineDesc(t *testing.T) {
	matches := []scanner.Match{
		{Line: 1, Column: 10},
		{Line: 3, Column: 5},
		{Line: 2, Column: 20},
		{Line: 3, Column: 15},
	}

	sorted := sortMatchesByLineDesc(matches)

	// 验证降序排列
	expected := []scanner.Match{
		{Line: 3, Column: 15},
		{Line: 3, Column: 5},
		{Line: 2, Column: 20},
		{Line: 1, Column: 10},
	}

	for i, m := range sorted {
		if m.Line != expected[i].Line || m.Column != expected[i].Column {
			t.Errorf("Position %d: got Line %d Col %d, want Line %d Col %d",
				i, m.Line, m.Column, expected[i].Line, expected[i].Column)
		}
	}
}

func TestReplaceInLine(t *testing.T) {
	r := New("myapp", false)

	tests := []struct {
		name       string
		line       string
		match      scanner.Match
		expected   string
		shouldReplace bool
	}{
		{
			name: "双引号替换",
			line: `fmt.Println("你好世界")`,
			match: scanner.Match{
				RawText:     `"你好世界"`,
				QuoteType:   `"`,
				ChineseText: "你好世界",
				ID:          "abc123",
			},
			expected:      `fmt.Println("myapp.abc123")`,
			shouldReplace: true,
		},
		{
			name: "单引号替换",
			line: `print('你好')`,
			match: scanner.Match{
				RawText:     `'你好'`,
				QuoteType:   `'`,
				ChineseText: "你好",
				ID:          "xyz789",
			},
			expected:      `print('myapp.xyz789')`,
			shouldReplace: true,
		},
		{
			name: "找不到匹配",
			line: `fmt.Println("不存在")`,
			match: scanner.Match{
				RawText:     `"你好"`,
				QuoteType:   `"`,
				ChineseText: "你好",
				ID:          "abc",
			},
			expected:      `fmt.Println("不存在")`,
			shouldReplace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, replaced := r.replaceInLine(tt.line, tt.match)
			if replaced != tt.shouldReplace {
				t.Errorf("replaceInLine() replaced = %v, want %v", replaced, tt.shouldReplace)
			}
			if replaced && result != tt.expected {
				t.Errorf("replaceInLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReplaceWithFlexibility(t *testing.T) {
	r := New("app", false)

	tests := []struct {
		name       string
		line       string
		match      scanner.Match
		expected   string
		shouldReplace bool
	}{
		{
			name: "灵活匹配双引号",
			line: `fmt.Println("你好")`,
			match: scanner.Match{
				RawText:     `"你好"`,
				QuoteType:   `"`,
				ChineseText: "你好",
				ID:          "id1",
			},
			expected:      `fmt.Println("app.id1")`,
			shouldReplace: true,
		},
		{
			name: "灵活匹配单引号",
			line: `print('测试')`,
			match: scanner.Match{
				RawText:     `'测试'`,
				QuoteType:   `'`,
				ChineseText: "测试",
				ID:          "id2",
			},
			expected:      `print('app.id2')`,
			shouldReplace: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, replaced := r.replaceWithFlexibility(tt.line, tt.match)
			if replaced != tt.shouldReplace {
				t.Errorf("replaceWithFlexibility() replaced = %v, want %v", replaced, tt.shouldReplace)
			}
			if replaced && result != tt.expected {
				t.Errorf("replaceWithFlexibility() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReplaceFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

func main() {
	print("你好")
	print("世界")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	r := New("myapp", false)
	matches := []scanner.Match{
		{
			FilePath:    testFile,
			Line:        4,
			RawText:     `"你好"`,
			QuoteType:   `"`,
			ChineseText: "你好",
			ID:          "id1",
		},
		{
			FilePath:    testFile,
			Line:        5,
			RawText:     `"世界"`,
			QuoteType:   `"`,
			ChineseText: "世界",
			ID:          "id2",
		},
	}

	result, err := r.replaceFile(testFile, matches)
	if err != nil {
		t.Fatalf("replaceFile failed: %v", err)
	}

	if result.Replacements != 2 {
		t.Errorf("Expected 2 replacements, got %d", result.Replacements)
	}

	// 验证文件内容
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(newContent), `"myapp.id1"`) {
		t.Error("File should contain replaced text for 你好")
	}

	if !strings.Contains(string(newContent), `"myapp.id2"`) {
		t.Error("File should contain replaced text for 世界")
	}
}

func TestReplaceFileDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package main

func main() {
	print("你好")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	r := New("myapp", true) // dry run
	matches := []scanner.Match{
		{
			FilePath:    testFile,
			Line:        4,
			RawText:     `"你好"`,
			QuoteType:   `"`,
			ChineseText: "你好",
			ID:          "id1",
		},
	}

	result, err := r.replaceFile(testFile, matches)
	if err != nil {
		t.Fatalf("replaceFile failed: %v", err)
	}

	if result.Replacements != 1 {
		t.Errorf("Expected 1 replacement count, got %d", result.Replacements)
	}

	// 验证文件内容未改变
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(newContent) != content {
		t.Error("File should not be modified in dry run mode")
	}
}

func TestPreview(t *testing.T) {
	r := New("testapp", false)
	matches := []scanner.Match{
		{
			FilePath:    "/path/to/file.go",
			Line:        10,
			Column:      20,
			RawText:     `"你好"`,
			QuoteType:   `"`,
			ChineseText: "你好",
			ID:          "abc123",
		},
	}

	preview := r.Preview(matches)

	// 检查包含预览标题
	if !strings.Contains(preview, "Replacement Preview") {
		t.Error("Preview should contain title")
	}

	// 检查包含文件路径
	if !strings.Contains(preview, "/path/to/file.go") {
		t.Error("Preview should contain file path")
	}

	// 检查包含原始文本
	if !strings.Contains(preview, `"你好"`) {
		t.Error("Preview should contain original text")
	}

	// 检查包含替换后的文本
	if !strings.Contains(preview, "testapp.abc123") {
		t.Error("Preview should contain replacement text")
	}
}

func TestReplaceInFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "hello world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	r := New("app", false)
	err := r.ReplaceInFile(testFile, "hello", "hi")
	if err != nil {
		t.Fatalf("ReplaceInFile failed: %v", err)
	}

	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(newContent) != "hi world" {
		t.Errorf("File content = %q, want %q", string(newContent), "hi world")
	}
}

func TestReadFileLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	lines, err := ReadFileLines(testFile)
	if err != nil {
		t.Fatalf("ReadFileLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Error("Lines don't match expected content")
	}
}
