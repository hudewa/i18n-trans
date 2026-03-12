package generator

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hudewa/i18n-trans/internal/translator"
)

// CSVGenerator generates CSV files
type CSVGenerator struct {
	outputDir string
}

// NewCSVGenerator creates a new CSVGenerator
func NewCSVGenerator(outputDir string) *CSVGenerator {
	return &CSVGenerator{
		outputDir: outputDir,
	}
}

// AppendToCSV appends translation results to aiAgent.csv
func (g *CSVGenerator) AppendToCSV(results []translator.TranslationResult) error {
	// Create output directory if not exists
	if err := os.MkdirAll(g.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	csvPath := filepath.Join(g.outputDir, "aiAgent.csv")

	// Check if file exists
	fileExists := false
	if _, err := os.Stat(csvPath); err == nil {
		fileExists = true
	}

	// Open file in append mode (create if not exists)
	f, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Write header if file is new
	if !fileExists {
		header := []string{"timestamp", "id", "zh", "en", "id_lang", "th", "vi", "ms", "status"}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	// Write data rows
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	for _, result := range results {
		status := "success"
		if result.Error != nil {
			status = "failed"
		}

		row := []string{
			timestamp,
			result.ID,
			result.Zh,
			result.En,
			result.Id,
			result.Th,
			result.Vi,
			result.Ms,
			status,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}
