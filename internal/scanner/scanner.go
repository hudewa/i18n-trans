package scanner

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Match represents a found Chinese text match
type Match struct {
	FilePath       string            `json:"file_path"`
	Line           int               `json:"line"`
	Column         int               `json:"column"`
	RawText        string            `json:"raw_text"`
	QuoteType      string            `json:"quote_type"` // " or '
	ChineseText    string            `json:"chinese_text"`
	ID             string            `json:"id"` // MD5 hash of Chinese text
	IsSprintf      bool              `json:"is_sprintf"`      // 是否被 fmt.Sprintf 包裹
	SprintfArgs    []string          `json:"sprintf_args"`    // Sprintf 的参数
	SprintfPrefix  string            `json:"sprintf_prefix"`  // Sprintf 前缀部分，如 "fmt.Sprintf("
	TemplateVars   []string          `json:"template_vars"`   // 模板变量如 ["{{.Name}}", "{{.Age}}"]
	TemplateMap    map[string]string `json:"template_map"`    // 模板变量到占位符的映射
	HasMapArg      bool              `json:"has_map_arg"`     // 是否有 map 参数（跨行）
}

// Scanner scans files for Chinese text
type Scanner struct {
	includeExts     []string
	excludeDirs     []string
	excludePatterns []string
}

// compile patterns - patterns match quoted strings, Chinese check is done separately
var (
	doubleQuotePattern = regexp.MustCompile(`"([^"]*)"`)
	singleQuotePattern = regexp.MustCompile(`'([^']*)'`)
	// Pattern to match function definitions: func (receiver) Name(params) returns {
	funcPattern = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(`)
	// Pattern to match fmt.Sprintf or sprintf with format string and arguments
	// Matches: fmt.Sprintf("...%d...", arg1, arg2) or sprintf("...%d...", arg)
	sprintfPattern = regexp.MustCompile(`(?i)(?:fmt\.)?sprintf\s*\(\s*["']`)
	// Pattern to match template variables like {{.Name}}, {{.User.Age}}
	templateVarPattern = regexp.MustCompile(`\{\{\.[A-Za-z_][A-Za-z0-9_\.]*\}\}`)
)

// containsChinese checks if a string contains Chinese characters
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// New creates a new Scanner
func New(includeExts, excludeDirs, excludePatterns []string) *Scanner {
	return &Scanner{
		includeExts:     includeExts,
		excludeDirs:     excludeDirs,
		excludePatterns: excludePatterns,
	}
}

// Scan scans a directory and returns all matches
func (s *Scanner) Scan(root string) ([]Match, error) {
	var matches []Match

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			if s.shouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be processed
		if !s.shouldProcessFile(path) {
			return nil
		}

		// Scan file
		fileMatches, err := s.scanFile(path)
		if err != nil {
			return fmt.Errorf("error scanning file %s: %w", path, err)
		}

		matches = append(matches, fileMatches...)
		return nil
	})

	return matches, err
}

// shouldSkipDir checks if a directory should be skipped
func (s *Scanner) shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	for _, dir := range s.excludeDirs {
		if base == dir {
			return true
		}
	}
	return false
}

// shouldProcessFile checks if a file should be processed
func (s *Scanner) shouldProcessFile(path string) bool {
	// Check extension
	ext := filepath.Ext(path)
	found := false
	for _, includeExt := range s.includeExts {
		if strings.EqualFold(ext, includeExt) {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	// Check exclude patterns
	base := filepath.Base(path)
	for _, pattern := range s.excludePatterns {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return false
		}
	}

	return true
}

// scanFile scans a single file for Chinese text
func (s *Scanner) scanFile(path string) ([]Match, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read all lines first to handle //noTrans + function skipping
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var matches []Match
	skipUntilBraceClose := false
	braceDepth := 0

	for lineNum, line := range lines {
		lineNum++ // 1-based line number

		// Handle //noTrans + function skipping
		if skipUntilBraceClose {
			// Count braces to track function depth
			braceDepth += strings.Count(line, "{")
			braceDepth -= strings.Count(line, "}")
			if braceDepth <= 0 {
				skipUntilBraceClose = false
				braceDepth = 0
			}
			continue
		}

		// Check if this line has //noTrans and next line looks like a function
		if hasNoTransComment(line) {
			// Check if next line exists and looks like a function definition
			if lineNum < len(lines) {
				nextLine := lines[lineNum]
				if looksLikeFunction(nextLine) {
					skipUntilBraceClose = true
					braceDepth = strings.Count(nextLine, "{")
					continue
				}
			}
		}

		// Skip lines with //noTrans or // notrans comment, or gorm struct tags, etc.
		if shouldSkipLine(line) {
			continue
		}

		// Find double quote matches
		doubleMatches := s.findMatchesInLine(path, lineNum, line, doubleQuotePattern, "\"")
		matches = append(matches, doubleMatches...)

		// Find single quote matches
		singleMatches := s.findMatchesInLine(path, lineNum, line, singleQuotePattern, "'")
		matches = append(matches, singleMatches...)
	}

	return matches, nil
}

// findMatchesInLine finds matches in a single line
func (s *Scanner) findMatchesInLine(filePath string, lineNum int, line string, pattern *regexp.Regexp, quoteType string) []Match {
	var matches []Match

	// Find comment start position (for Go // comments)
	commentStart := strings.Index(line, "//")

	// Check if this line contains fmt.Sprintf or sprintf
	isSprintfLine := sprintfPattern.MatchString(line)
	var sprintfInfo *SprintfInfo
	if isSprintfLine {
		sprintfInfo = parseSprintf(line)
	}

	for _, match := range pattern.FindAllStringIndex(line, -1) {
		start, end := match[0], match[1]

		// Skip if match is inside a comment (after //)
		if commentStart != -1 && start > commentStart {
			continue
		}

		fullMatch := line[start:end]
		innerText := line[start+1 : end-1] // Remove quotes

		// Skip if doesn't contain Chinese
		if !containsChinese(innerText) {
			continue
		}

		// Skip if contains image extensions
		if shouldSkipText(innerText) {
			continue
		}

		// Detect template variables like {{.Name}}
		templateVars := templateVarPattern.FindAllString(innerText, -1)

		// For ID generation, we need to handle template vars consistently
		// We'll create a version with template vars replaced by placeholders for ID generation
		idText := innerText
		templateMap := make(map[string]string)
		if len(templateVars) > 0 {
			for i, tv := range templateVars {
				placeholder := fmt.Sprintf("__VAR_%d__", i)
				templateMap[tv] = placeholder
				idText = strings.Replace(idText, tv, placeholder, 1)
			}
		}

		// Generate ID from Chinese text (with template vars normalized)
		id := generateID(idText)

		m := Match{
			FilePath:    filePath,
			Line:        lineNum,
			Column:      start + 1, // 1-based column
			RawText:     fullMatch,
			QuoteType:   quoteType,
			ChineseText: innerText,
			ID:          id,
			TemplateVars: templateVars,
			TemplateMap: templateMap,
		}

		// Check if this match is part of a Sprintf call
		if sprintfInfo != nil && start >= sprintfInfo.FormatStart && end <= sprintfInfo.FormatEnd {
			m.IsSprintf = true
			m.SprintfArgs = sprintfInfo.Args
			m.SprintfPrefix = sprintfInfo.Prefix
			// Check if argsStr contains map (for template variables)
			argsStr := line[sprintfInfo.FormatEnd:]
			if strings.Contains(argsStr, "map[string]any") || strings.Contains(argsStr, "map[string]interface") {
				m.HasMapArg = true
			}
		}

		matches = append(matches, m)
	}

	return matches
}

// SprintfInfo holds information about a Sprintf call
type SprintfInfo struct {
	Prefix      string   // e.g., "fmt.Sprintf(" or "sprintf("
	FormatStart int      // Start position of format string
	FormatEnd   int      // End position of format string
	Args        []string // Arguments after format string
}

// parseSprintf parses a Sprintf call and extracts format string and arguments
func parseSprintf(line string) *SprintfInfo {
	// Find sprintf position
	sprintfIdx := strings.Index(strings.ToLower(line), "sprintf")
	if sprintfIdx == -1 {
		return nil
	}

	// Find the opening parenthesis after sprintf
	parenStart := strings.Index(line[sprintfIdx:], "(")
	if parenStart == -1 {
		return nil
	}
	parenStart += sprintfIdx

	// Determine prefix (fmt.Sprintf or just sprintf)
	prefix := "sprintf("
	if sprintfIdx >= 4 && line[sprintfIdx-4:sprintfIdx] == "fmt." {
		prefix = "fmt.Sprintf("
		sprintfIdx -= 4
	}

	// Find the format string (first quoted string after opening paren)
	rest := line[parenStart+1:]

	// Skip whitespace
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}

	// Check if it starts with a quote
	if i >= len(rest) || (rest[i] != '"' && rest[i] != '\'') {
		return nil
	}

	quoteChar := rest[i]
	formatStart := parenStart + 1 + i

	// Find the end of the format string
	formatEnd := -1
	for j := i + 1; j < len(rest); j++ {
		if rest[j] == quoteChar && rest[j-1] != '\\' {
			formatEnd = parenStart + 1 + j + 1
			break
		}
	}

	if formatEnd == -1 {
		return nil
	}

	// Extract arguments after format string
	args := []string{}
	argsStr := rest[formatEnd-(parenStart+1):]

	// Check if this line contains map[string]any (template variable pattern)
	// If so, we need to handle it specially
	if strings.Contains(argsStr, "map[string]any") || strings.Contains(argsStr, "map[string]interface") {
		// For template variables with map, extract just the map part
		mapArg := extractMapArgument(argsStr)
		if mapArg != "" {
			args = append(args, mapArg)
		}
	} else {
		// Standard argument parsing
		args = parseStandardArgs(argsStr)
	}

	return &SprintfInfo{
		Prefix:      prefix,
		FormatStart: formatStart,
		FormatEnd:   formatEnd,
		Args:        args,
	}
}

// extractMapArgument extracts map argument from args string
// Handles both inline map and multiline map starting on the same line
func extractMapArgument(argsStr string) string {
	// Find map[string]any or map[string]interface{}
	mapIdx := strings.Index(argsStr, "map[string]")
	if mapIdx == -1 {
		return ""
	}

	// First, find the opening brace after map[...]{
	braceStart := -1
	for i := mapIdx; i < len(argsStr); i++ {
		if argsStr[i] == '{' {
			braceStart = i
			break
		}
	}

	if braceStart == -1 {
		return strings.TrimSpace(argsStr[mapIdx:])
	}

	// Extract from opening brace to the matching closing brace
	braceDepth := 1  // Start at 1 since we found the opening brace
	inString := false
	stringChar := byte(0)

	for i := braceStart + 1; i < len(argsStr); i++ {
		ch := argsStr[i]

		if !inString && (ch == '"' || ch == '\'') {
			inString = true
			stringChar = ch
		} else if inString && ch == stringChar && (i > 0 && argsStr[i-1] != '\\') {
			inString = false
		} else if !inString {
			if ch == '{' {
				braceDepth++
			} else if ch == '}' {
				braceDepth--
				if braceDepth == 0 {
					// End of map
					return strings.TrimSpace(argsStr[mapIdx : i+1])
				}
			}
		}
	}

	// If we didn't find the closing brace, return what we have (multiline case)
	return strings.TrimSpace(argsStr[mapIdx:])
}

// parseStandardArgs parses standard Sprintf arguments
func parseStandardArgs(argsStr string) []string {
	args := []string{}
	parenDepth := 0
	currentArg := ""
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(argsStr); i++ {
		ch := argsStr[i]

		if !inString && (ch == '"' || ch == '\'') {
			inString = true
			stringChar = ch
			currentArg += string(ch)
		} else if inString && ch == stringChar && (i == 0 || argsStr[i-1] != '\\') {
			inString = false
			currentArg += string(ch)
		} else if !inString {
			if ch == '(' {
				parenDepth++
				currentArg += string(ch)
			} else if ch == ')' {
				if parenDepth == 0 {
					// End of sprintf call
					if strings.TrimSpace(currentArg) != "" {
						args = append(args, strings.TrimSpace(currentArg))
					}
					break
				}
				parenDepth--
				currentArg += string(ch)
			} else if ch == ',' && parenDepth == 0 {
				// End of current argument
				if strings.TrimSpace(currentArg) != "" {
					args = append(args, strings.TrimSpace(currentArg))
				}
				currentArg = ""
			} else {
				currentArg += string(ch)
			}
		} else {
			currentArg += string(ch)
		}
	}

	return args
}

// shouldSkipText checks if text should be skipped (e.g., image paths)
func shouldSkipText(text string) bool {
	lower := strings.ToLower(text)
	return strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".svg")
}

// hasNoTransComment checks if line contains //noTrans comment
func hasNoTransComment(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "//notrans") ||
		strings.Contains(lower, "// notrans")
}

// looksLikeFunction checks if a line looks like a function definition
func looksLikeFunction(line string) bool {
	return funcPattern.MatchString(line)
}

// shouldSkipLine checks if a line should be skipped
// Skips lines containing:
// - //noTrans or // notrans comments
// - gorm struct tags
// - lines starting with //
// - lines containing both json and comment tags
func shouldSkipLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(line)

	// Skip lines starting with //
	if strings.HasPrefix(trimmed, "//") {
		return true
	}

	// Skip gorm struct tags
	if strings.Contains(line, "gorm:\"") {
		return true
	}

	// Skip lines with both json and comment tags
	if strings.Contains(lower, "json:\"") && strings.Contains(lower, "comment:\"") {
		return true
	}

	// Skip lines with //noTrans comment
	return strings.Contains(lower, "//notrans") ||
		strings.Contains(lower, "// notrans")
}

// generateID generates an MD5 hash of the text (first 8 characters)
func generateID(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])[:8]
}

// GroupByFile groups matches by file path
func GroupByFile(matches []Match) map[string][]Match {
	groups := make(map[string][]Match)
	for _, m := range matches {
		groups[m.FilePath] = append(groups[m.FilePath], m)
	}
	return groups
}

// UniqueChineseTexts returns unique Chinese texts from matches
func UniqueChineseTexts(matches []Match) []string {
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		if !seen[m.ChineseText] {
			seen[m.ChineseText] = true
			unique = append(unique, m.ChineseText)
		}
	}
	return unique
}
