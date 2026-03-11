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
	FilePath    string `json:"file_path"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	RawText     string `json:"raw_text"`
	QuoteType   string `json:"quote_type"` // " or '
	ChineseText string `json:"chinese_text"`
	ID          string `json:"id"` // MD5 hash of Chinese text
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

	for _, match := range pattern.FindAllStringIndex(line, -1) {
		start, end := match[0], match[1]
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

		// Generate ID from Chinese text
		id := generateID(innerText)

		matches = append(matches, Match{
			FilePath:    filePath,
			Line:        lineNum,
			Column:      start + 1, // 1-based column
			RawText:     fullMatch,
			QuoteType:   quoteType,
			ChineseText: innerText,
			ID:          id,
		})
	}

	return matches
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
