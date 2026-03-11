package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"i18n-trans/internal/translator"
)

// Generator generates SQL files
type Generator struct {
	outputDir  string
	moduleName string
	updatedBy  string
}

// New creates a new Generator
func New(outputDir, moduleName, updatedBy string) *Generator {
	return &Generator{
		outputDir:  outputDir,
		moduleName: moduleName,
		updatedBy:  updatedBy,
	}
}

// GenerateSQL generates SQL file from translation results
func (g *Generator) GenerateSQL(results []translator.TranslationResult) (string, error) {
	// Create output directory if not exists
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("i18n_%s.sql", timestamp)
	filepath := filepath.Join(g.outputDir, filename)

	// Generate SQL content
	content := g.generateSQLContent(results)

	// Write to file
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write SQL file: %w", err)
	}

	return filepath, nil
}

// generateSQLContent generates the SQL content
func (g *Generator) generateSQLContent(results []translator.TranslationResult) string {
	var sb strings.Builder

	// Add header comment
	sb.WriteString(fmt.Sprintf("-- Auto-generated i18n SQL\n"))
	sb.WriteString(fmt.Sprintf("-- Generated at: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("-- Total translations: %d\n", len(results)))
	sb.WriteString("\n")

	// Generate INSERT statements
	for _, result := range results {
		if result.Error != nil {
			// Skip failed translations but add comment
			sb.WriteString(fmt.Sprintf("-- FAILED: %s (error: %v)\n", result.Text, result.Error))
			continue
		}

		// Escape single quotes for SQL
		zh := escapeSQL(result.Zh)
		en := escapeSQL(result.En)
		id := escapeSQL(result.Id)
		th := escapeSQL(result.Th)
		vi := escapeSQL(result.Vi)
		ms := escapeSQL(result.Ms)

		// Use ID from result or generate from Chinese text
		identification := result.ID
		if identification == "" {
			identification = generateID(result.Zh)
		}

		stmt := fmt.Sprintf(
			"INSERT INTO gamoji.i18n (type, module, identification, zh_lan, en_lan, id_lan, th_lan, vi_lan, ms_lan, updated_by) "+
				"VALUES (1, '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s');\n",
			g.moduleName,
			identification,
			zh,
			en,
			id,
			th,
			vi,
			ms,
			g.updatedBy,
		)
		sb.WriteString(stmt)
	}

	return sb.String()
}

// escapeSQL escapes single quotes for SQL
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// generateID generates a simple hash for identification
func generateID(text string) string {
	if len(text) > 8 {
		return text[:8]
	}
	return text
}

// GenerateReport generates a summary report
func (g *Generator) GenerateReport(results []translator.TranslationResult) string {
	var sb strings.Builder

	successCount := 0
	failCount := 0

	for _, r := range results {
		if r.Error != nil {
			failCount++
		} else {
			successCount++
		}
	}

	sb.WriteString("\n========== Translation Report ==========\n")
	sb.WriteString(fmt.Sprintf("Total: %d\n", len(results)))
	sb.WriteString(fmt.Sprintf("Success: %d\n", successCount))
	sb.WriteString(fmt.Sprintf("Failed: %d\n", failCount))

	if failCount > 0 {
		sb.WriteString("\nFailed items:\n")
		for _, r := range results {
			if r.Error != nil {
				sb.WriteString(fmt.Sprintf("  - %s: %v\n", r.Text, r.Error))
			}
		}
	}

	return sb.String()
}
