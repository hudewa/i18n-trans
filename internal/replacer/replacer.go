package replacer

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/hudewa/i18n-trans/internal/scanner"
)

// Replacer replaces Chinese text with module.identification format
type Replacer struct {
	moduleName    string
	dryRun        bool
	replaceMode   string // "simple" or "i18n"
}

// New creates a new Replacer
func New(moduleName string, dryRun bool) *Replacer {
	return &Replacer{
		moduleName:  moduleName,
		dryRun:      dryRun,
		replaceMode: "i18n",
	}
}

// NewWithMode creates a new Replacer with specified replace mode
func NewWithMode(moduleName string, dryRun bool, replaceMode string) *Replacer {
	if replaceMode == "" {
		replaceMode = "i18n"
	}
	return &Replacer{
		moduleName:  moduleName,
		dryRun:      dryRun,
		replaceMode: replaceMode,
	}
}

// ReplaceResult holds the result of a replacement operation
type ReplaceResult struct {
	FilePath      string `json:"file_path"`
	Replacements  int    `json:"replacements"`
	Error         error  `json:"error,omitempty"`
}

// ReplaceAll replaces Chinese text in all files
func (r *Replacer) ReplaceAll(matches []scanner.Match) ([]ReplaceResult, error) {
	// Group matches by file
	fileMatches := make(map[string][]scanner.Match)
	for _, m := range matches {
		fileMatches[m.FilePath] = append(fileMatches[m.FilePath], m)
	}

	var results []ReplaceResult

	for filePath, fileMatchList := range fileMatches {
		result, err := r.replaceFile(filePath, fileMatchList)
		if err != nil {
			result.Error = err
		}
		results = append(results, result)
	}

	return results, nil
}

// replaceFile replaces Chinese text in a single file
func (r *Replacer) replaceFile(filePath string, matches []scanner.Match) (ReplaceResult, error) {
	result := ReplaceResult{
		FilePath: filePath,
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	replacements := 0

	// Sort matches by line number (descending) to avoid index shifting
	// when replacing multiple matches on the same line
	sortedMatches := sortMatchesByLineDesc(matches)

	// Track which lines have been modified
	modifiedLines := make(map[int]bool)

	for _, match := range sortedMatches {
		if match.Line < 1 || match.Line > len(lines) {
			continue
		}

		lineIdx := match.Line - 1
		line := lines[lineIdx]

		// Find and replace the specific occurrence
		newLine, replaced := r.replaceInLine(line, match)
		if replaced {
			lines[lineIdx] = newLine
			replacements++
			modifiedLines[match.Line] = true
		}
	}

	result.Replacements = replacements

	// Write file if not dry run and there were replacements
	if !r.dryRun && replacements > 0 {
		newContent := strings.Join(lines, "\n")
		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			return result, fmt.Errorf("failed to write file: %w", err)
		}
	}

	return result, nil
}

// replaceInLine replaces the Chinese text in a line
func (r *Replacer) replaceInLine(line string, match scanner.Match) (string, bool) {
	// Handle Sprintf case - need to replace the entire Sprintf call
	if match.IsSprintf {
		return r.replaceSprintfInLine(line, match)
	}

	// Build replacement string based on mode
	replacement := r.buildReplacement(match)

	// Find the exact occurrence
	idx := strings.Index(line, match.RawText)
	if idx == -1 {
		// Try to find it with some flexibility (whitespace differences)
		return r.replaceWithFlexibility(line, match)
	}

	return line[:idx] + replacement + line[idx+len(match.RawText):], true
}

// replaceSprintfInLine replaces the entire Sprintf call
func (r *Replacer) replaceSprintfInLine(line string, match scanner.Match) (string, bool) {
	// Build replacement
	replacement := r.buildReplacement(match)

	// Find the Sprintf prefix in the line
	sprintfIdx := strings.Index(strings.ToLower(line), "sprintf")
	if sprintfIdx == -1 {
		// Fallback to normal replacement
		idx := strings.Index(line, match.RawText)
		if idx == -1 {
			return r.replaceWithFlexibility(line, match)
		}
		return line[:idx] + replacement + line[idx+len(match.RawText):], true
	}

	// Find the start of Sprintf call (include fmt. prefix if present)
	startIdx := sprintfIdx
	if sprintfIdx >= 4 && line[sprintfIdx-4:sprintfIdx] == "fmt." {
		startIdx = sprintfIdx - 4
	}

	// Find the matching closing parenthesis
	parenStart := strings.Index(line[startIdx:], "(") + startIdx
	if parenStart < startIdx {
		return line, false
	}

	// Find the matching closing paren
	parenDepth := 0
	endIdx := -1
	for i := parenStart; i < len(line); i++ {
		if line[i] == '(' {
			parenDepth++
		} else if line[i] == ')' {
			parenDepth--
			if parenDepth == 0 {
				endIdx = i + 1
				break
			}
		}
	}

	if endIdx == -1 {
		return line, false
	}

	// Replace the entire Sprintf call
	return line[:startIdx] + replacement + line[endIdx:], true
}

// buildReplacement builds the replacement string based on replace mode
func (r *Replacer) buildReplacement(match scanner.Match) string {
	identifier := r.moduleName + "." + match.ID

	// Handle Sprintf case
	if match.IsSprintf && len(match.SprintfArgs) > 0 {
		return r.buildSprintfReplacement(match, identifier)
	}

	if r.replaceMode == "i18n" {
		// i18n mode: i18n.TextT(ctx, "module.id")
		return "i18n.TextT(ctx, " + match.QuoteType + identifier + match.QuoteType + ")"
	}

	// Simple mode: "module.id" or 'module.id'
	return match.QuoteType + identifier + match.QuoteType
}

// buildSprintfReplacement builds replacement for Sprintf pattern
// fmt.Sprintf("积分兑换商品只能全部退款%d", refundPrice) --> i18n.TextTf(ctx, "ai.id", refundPrice)
func (r *Replacer) buildSprintfReplacement(match scanner.Match, identifier string) string {
	args := strings.Join(match.SprintfArgs, ", ")

	if r.replaceMode == "i18n" {
		// i18n mode: i18n.TextTf(ctx, "module.id", args...)
		return "i18n.TextTf(ctx, " + match.QuoteType + identifier + match.QuoteType + ", " + args + ")"
	}

	// Simple mode: fmt.Sprintf("module.id", args...)
	return "fmt.Sprintf(" + match.QuoteType + identifier + match.QuoteType + ", " + args + ")"
}

// replaceWithFlexibility tries to find and replace with more flexibility
func (r *Replacer) replaceWithFlexibility(line string, match scanner.Match) (string, bool) {
	// Handle Sprintf case
	if match.IsSprintf {
		return r.replaceSprintfInLine(line, match)
	}

	// Extract the Chinese text
	chineseText := match.ChineseText

	// Try to find the pattern: quote + chinese + quote
	patterns := []string{
		`"` + chineseText + `"`,
		`'` + chineseText + `'`,
	}

	replacement := r.buildReplacement(match)

	for _, pattern := range patterns {
		if idx := strings.Index(line, pattern); idx != -1 {
			return line[:idx] + replacement + line[idx+len(pattern):], true
		}
	}

	return line, false
}

// sortMatchesByLineDesc sorts matches by line number in descending order
func sortMatchesByLineDesc(matches []scanner.Match) []scanner.Match {
	// Simple bubble sort for small lists
	sorted := make([]scanner.Match, len(matches))
	copy(sorted, matches)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Line > sorted[i].Line {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			} else if sorted[j].Line == sorted[i].Line && sorted[j].Column > sorted[i].Column {
				// Same line, sort by column descending
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// Preview shows what would be replaced without actually replacing
func (r *Replacer) Preview(matches []scanner.Match) string {
	var sb strings.Builder
	sb.WriteString("\n========== Replacement Preview ==========\n\n")

	fileMatches := make(map[string][]scanner.Match)
	for _, m := range matches {
		fileMatches[m.FilePath] = append(fileMatches[m.FilePath], m)
	}

	for filePath, fileMatchList := range fileMatches {
		sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		sb.WriteString(strings.Repeat("-", 50) + "\n")

		for _, m := range fileMatchList {
			replacement := r.buildReplacement(m)
			sb.WriteString(fmt.Sprintf("  Line %d, Col %d:\n", m.Line, m.Column))
			sb.WriteString(fmt.Sprintf("    Original: %s\n", m.RawText))
			sb.WriteString(fmt.Sprintf("    Replace:  %s\n", replacement))
			sb.WriteString(fmt.Sprintf("    Chinese:  %s\n\n", m.ChineseText))
		}
	}

	return sb.String()
}

// ReplaceInFile replaces text in a specific file (convenience method)
func (r *Replacer) ReplaceInFile(filePath string, oldText, newText string) error {
	if r.dryRun {
		fmt.Printf("[DRY RUN] Would replace in %s: %s -> %s\n", filePath, oldText, newText)
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	newContent := strings.ReplaceAll(string(content), oldText, newText)

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// ReadFileLines reads a file and returns its lines
func ReadFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}
