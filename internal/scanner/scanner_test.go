package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestContainsChinese(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"纯中文", "你好世界", true},
		{"纯英文", "hello world", false},
		{"中英混合", "hello 世界", true},
		{"空字符串", "", false},
		{"数字和符号", "123!@#", false},
		{"中文标点", "你好，世界！", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsChinese(tt.input)
			if result != tt.expected {
				t.Errorf("containsChinese(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldSkipText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"PNG图片", "image.png", true},
		{"JPG图片", "photo.jpg", true},
		{"JPEG图片", "pic.jpeg", true},
		{"WebP图片", "img.webp", true},
		{"GIF图片", "anim.gif", true},
		{"SVG图片", "icon.svg", true},
		{"普通文本", "你好世界", false},
		{"带路径的图片", "/path/to/image.png", true},
		{"大写扩展名", "IMAGE.PNG", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipText(tt.input)
			if result != tt.expected {
				t.Errorf("shouldSkipText(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldSkipLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"包含//noTrans", `fmt.Println("你好") //noTrans`, true},
		{"包含// notrans", `fmt.Println("你好") // notrans`, true},
		{"包含//NOTRANS", `fmt.Println("你好") //NOTRANS`, true},
		{"不包含noTrans", `fmt.Println("你好") // 其他注释`, false},
		{"普通代码", `fmt.Println("你好")`, false},
		{"包含gorm标签", "UserID uint64 `gorm:\"column:user_id;comment:用户id\"`", true},
		{"包含gorm标签双引号", "gorm:\"column:user_id;comment:用户id\"", true},
		{"普通gorm变量", "gorm := \"test\"", false},
		{"以//开头", "// follower_count int", true},
		{"包含json和comment", "EventType string `json:\"event_type\" comment:\"是否预制事件\"`", true},
		{"只有json", "Name string `json:\"name\"`", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipLine(tt.input)
			if result != tt.expected {
				t.Errorf("shouldSkipLine(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"长文本", "这是一个很长的中文文本用于测试", 8},
		{"短文本", "你好", 2},
		{"8字符文本", "12345678", 8},
		{"空字符串", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateID(tt.input)
			if len(result) != tt.expected {
				t.Errorf("generateID(%q) length = %d, want %d", tt.input, len(result), tt.expected)
			}
		})
	}

	// 测试相同输入产生相同输出
	id1 := generateID("测试文本")
	id2 := generateID("测试文本")
	if id1 != id2 {
		t.Errorf("相同输入应产生相同ID: %q vs %q", id1, id2)
	}

	// 测试不同输入产生不同输出
	id3 := generateID("不同文本")
	if id1 == id3 {
		t.Errorf("不同输入应产生不同ID")
	}
}

func TestNew(t *testing.T) {
	includeExts := []string{".go", ".js"}
	excludeDirs := []string{"node_modules"}
	excludePatterns := []string{"*_test.go"}

	s := New(includeExts, excludeDirs, excludePatterns)

	if s == nil {
		t.Fatal("New() returned nil")
	}

	if len(s.includeExts) != 2 {
		t.Errorf("includeExts length = %d, want 2", len(s.includeExts))
	}

	if len(s.excludeDirs) != 1 {
		t.Errorf("excludeDirs length = %d, want 1", len(s.excludeDirs))
	}
}

func TestShouldSkipDir(t *testing.T) {
	s := New(nil, []string{"node_modules", ".git"}, nil)

	tests := []struct {
		path     string
		expected bool
	}{
		{"node_modules", true},
		{".git", true},
		{"src", false},
		{"/path/to/node_modules", true},
		{"/path/to/src", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := s.shouldSkipDir(tt.path)
			if result != tt.expected {
				t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestShouldProcessFile(t *testing.T) {
	s := New([]string{".go", ".js"}, nil, []string{"*_test.go"})

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"app.js", true},
		{"test.py", false},
		{"main_test.go", false},
		{"/path/to/main.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := s.shouldProcessFile(tt.path)
			if result != tt.expected {
				t.Errorf("shouldProcessFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestScanFile(t *testing.T) {
	// 创建临时目录和文件
	tmpDir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("你好世界")
	fmt.Println('单个中文')
	fmt.Println("hello world") // 无中文
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	s := New([]string{".go"}, nil, nil)
	matches, err := s.scanFile(testFile)
	if err != nil {
		t.Fatalf("scanFile failed: %v", err)
	}

	// 应该找到2个中文匹配（双引号和单引号各一个）
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}

	// 验证匹配内容
	foundDoubleQuote := false
	foundSingleQuote := false
	for _, m := range matches {
		if m.ChineseText == "你好世界" && m.QuoteType == `"` {
			foundDoubleQuote = true
		}
		if m.ChineseText == "单个中文" && m.QuoteType == `'` {
			foundSingleQuote = true
		}
	}

	if !foundDoubleQuote {
		t.Error("未找到双引号中的中文")
	}
	if !foundSingleQuote {
		t.Error("未找到单引号中的中文")
	}
}

func TestGroupByFile(t *testing.T) {
	matches := []Match{
		{FilePath: "/a/b/file1.go", Line: 1},
		{FilePath: "/a/b/file1.go", Line: 2},
		{FilePath: "/c/d/file2.go", Line: 1},
	}

	groups := GroupByFile(matches)

	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	if len(groups["/a/b/file1.go"]) != 2 {
		t.Errorf("Expected 2 matches in file1.go, got %d", len(groups["/a/b/file1.go"]))
	}

	if len(groups["/c/d/file2.go"]) != 1 {
		t.Errorf("Expected 1 match in file2.go, got %d", len(groups["/c/d/file2.go"]))
	}
}

func TestUniqueChineseTexts(t *testing.T) {
	matches := []Match{
		{ChineseText: "你好"},
		{ChineseText: "世界"},
		{ChineseText: "你好"}, // 重复
		{ChineseText: "测试"},
	}

	unique := UniqueChineseTexts(matches)

	if len(unique) != 3 {
		t.Errorf("Expected 3 unique texts, got %d", len(unique))
	}

	// 检查是否包含所有唯一值
	textMap := make(map[string]bool)
	for _, text := range unique {
		textMap[text] = true
	}

	if !textMap["你好"] || !textMap["世界"] || !textMap["测试"] {
		t.Error("Unique texts don't match expected values")
	}
}

func TestFindMatchesInLine(t *testing.T) {
	s := New(nil, nil, nil)

	tests := []struct {
		name      string
		line      string
		quoteType string
		expected  int
	}{
		{"双引号中文", `fmt.Println("你好")`, `"`, 1},
		{"单引号中文", `fmt.Println('你好')`, `'`, 1},
		{"无中文", `fmt.Println("hello")`, `"`, 0},
		{"多个中文", `a("你好", "世界")`, `"`, 2},
		{"图片路径", `img("icon.png")`, `"`, 0},
		{"注释中的中文", `// "注释中的中文"`, `"`, 0},
		{"代码和注释", `fmt.Println("你好") // 注释`, `"`, 1},
		{"只有注释", `// 这是注释`, `"`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern *regexp.Regexp
			if tt.quoteType == `"` {
				pattern = doubleQuotePattern
			} else {
				pattern = singleQuotePattern
			}

			matches := s.findMatchesInLine("test.go", 1, tt.line, pattern, tt.quoteType)
			if len(matches) != tt.expected {
				t.Errorf("Expected %d matches, got %d", tt.expected, len(matches))
			}
		})
	}
}
