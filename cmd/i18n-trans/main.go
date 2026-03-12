package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hudewa/i18n-trans/internal/generator"
	"github.com/hudewa/i18n-trans/internal/logger"
	"github.com/hudewa/i18n-trans/internal/replacer"
	"github.com/hudewa/i18n-trans/internal/scanner"
	"github.com/hudewa/i18n-trans/internal/translator"
	"github.com/hudewa/i18n-trans/pkg/config"
)

var (
	cfgFile      string
	initOutput   string // init 命令的输出文件路径
	scanDir      string
	outputDir    string
	replace      bool
	dryRun       bool
	apiKey       string
	baseURL      string
	model        string
	moduleName   string
	replaceMode  string
)

var rootCmd = &cobra.Command{
	Use:   "gamoji-trans",
	Short: "A CLI tool for translating Chinese text in code to multiple languages",
	Long: `gamoji-trans scans code for Chinese text, translates it using Doubao AI,
and generates SQL files for i18n. It can also replace the original Chinese
text with module.identification format.`,
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan directory for Chinese text",
	Long:  `Scan the specified directory for Chinese text wrapped in quotes.`,
	RunE:  runScan,
}

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Scan and translate Chinese text",
	Long:  `Scan the directory for Chinese text and translate it using Doubao AI.`,
	RunE:  runTranslate,
}

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Full process: scan, translate, generate SQL, and optionally replace",
	Long: `Complete workflow: scan for Chinese text, translate using Doubao AI,
generate SQL file, and optionally replace original text with module.identification format.

Examples:
  # Scan, translate, generate SQL only (no replacement)
  i18n-trans process

  # Scan, translate, generate SQL, and replace Chinese text in files
  i18n-trans process --replace

  # Preview what would be replaced without actually replacing
  i18n-trans process --dry-run`,
	RunE: runProcess,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an example configuration file",
	Long:  `Create an example configuration file at the specified path.`,
	RunE:  runInit,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	// Scan command flags
	scanCmd.Flags().StringVarP(&scanDir, "dir", "d", ".", "directory to scan")

	// Translate command flags
	translateCmd.Flags().StringVarP(&scanDir, "dir", "d", ".", "directory to scan")
	translateCmd.Flags().StringVarP(&outputDir, "output", "o", "./sql", "output directory for SQL files")
	translateCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "Doubao API key (or set DOUBAO_API_KEY env var)")
	translateCmd.Flags().StringVar(&baseURL, "base-url", "", "Doubao API base URL")
	translateCmd.Flags().StringVarP(&model, "model", "m", "", "Doubao model name")
	translateCmd.Flags().StringVar(&moduleName, "module", "", "module name for identification")

	// Process command flags
	processCmd.Flags().StringVarP(&scanDir, "dir", "d", ".", "directory to scan")
	processCmd.Flags().StringVarP(&outputDir, "output", "o", "./sql", "output directory for SQL files")
	processCmd.Flags().BoolVar(&replace, "replace", false, "replace original text with module.identification")
	processCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be replaced without making changes")
	processCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "Doubao API key (or set DOUBAO_API_KEY env var)")
	processCmd.Flags().StringVar(&baseURL, "base-url", "", "Doubao API base URL")
	processCmd.Flags().StringVarP(&model, "model", "m", "", "Doubao model name")
	processCmd.Flags().StringVar(&moduleName, "module", "", "module name for identification")
	processCmd.Flags().StringVar(&replaceMode, "replace-mode", "", "replace mode: simple (default) or i18n")

	// Init command flags
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "config.yaml", "output file path")

	// Add commands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(translateCmd)
	rootCmd.AddCommand(processCmd)
	rootCmd.AddCommand(initCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScan(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 确定扫描目录：命令行参数优先，其次配置文件，最后默认当前目录
	scanDirs := getScanDirs(cfg.Scan.Dir, scanDir)

	// Create scanner
	s := scanner.New(cfg.Scan.IncludeExt, cfg.Scan.ExcludeDirs, cfg.Scan.ExcludePatterns)

	fmt.Printf("Scanning directories: %v\n", scanDirs)
	fmt.Println("This may take a while...")

	// Scan directories
	var allMatches []scanner.Match
	for _, dir := range scanDirs {
		matches, err := s.Scan(dir)
		if err != nil {
			return fmt.Errorf("scan failed for %s: %w", dir, err)
		}
		allMatches = append(allMatches, matches...)
	}
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Display results
	fmt.Printf("\nFound %d Chinese text occurrences:\n", len(allMatches))
	fmt.Println(strings.Repeat("=", 60))

	files := scanner.GroupByFile(allMatches)
	for filePath, fileMatches := range files {
		fmt.Printf("\nFile: %s\n", filePath)
		for _, m := range fileMatches {
			fmt.Printf("  Line %d, Col %d: %s\n", m.Line, m.Column, m.RawText)
		}
	}

	return nil
}

func runTranslate(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with command line flags
	if apiKey != "" {
		cfg.Doubao.APIKey = apiKey
	}
	if baseURL != "" {
		cfg.Doubao.BaseURL = baseURL
	}
	if model != "" {
		cfg.Doubao.Model = model
	}
	if moduleName != "" {
		cfg.Output.ModuleName = moduleName
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return err
	}

	// 确定扫描目录：命令行参数优先，其次配置文件，最后默认当前目录
	scanDirs := getScanDirs(cfg.Scan.Dir, scanDir)

	// Create scanner
	s := scanner.New(cfg.Scan.IncludeExt, cfg.Scan.ExcludeDirs, cfg.Scan.ExcludePatterns)

	fmt.Printf("Scanning directories: %v\n", scanDirs)

	// Scan directories
	var allMatches []scanner.Match
	for _, dir := range scanDirs {
		matches, err := s.Scan(dir)
		if err != nil {
			return fmt.Errorf("scan failed for %s: %w", dir, err)
		}
		allMatches = append(allMatches, matches...)
	}

	if len(allMatches) == 0 {
		fmt.Println("No Chinese text found.")
		return nil
	}

	fmt.Printf("Found %d Chinese text occurrences\n", len(allMatches))

	// Get unique Chinese texts
	uniqueTexts := scanner.UniqueChineseTexts(allMatches)
	fmt.Printf("Unique texts to translate: %d\n", len(uniqueTexts))

	// Create translator
	t := translator.New(cfg.Doubao.APIKey, cfg.Doubao.BaseURL, cfg.Doubao.Model)

	fmt.Println("Translating...")

	// Translate
	ctx := context.Background()
	results, err := t.TranslateTexts(ctx, uniqueTexts)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	// Create generator
	g := generator.New(outputDir, cfg.Output.ModuleName, cfg.Output.UpdatedBy)

	// Generate SQL
	sqlPath, err := g.GenerateSQL(results)
	if err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	fmt.Printf("\nSQL file generated: %s\n", sqlPath)

	// Print report
	report := g.GenerateReport(results)
	fmt.Println(report)

	return nil
}

func runProcess(cmd *cobra.Command, args []string) error {
	// Initialize logger
	log := logger.New("logs", "aiAgent_i18n.log")
	log.LogInfo("=== Starting i18n translation process ===")

	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.LogError(fmt.Sprintf("Failed to load config: %v", err))
		return fmt.Errorf("failed to load config: %w", err)
	}
	log.LogInfo("Config loaded successfully")

	// Override config with command line flags
	if apiKey != "" {
		cfg.Doubao.APIKey = apiKey
	}
	if baseURL != "" {
		cfg.Doubao.BaseURL = baseURL
	}
	if model != "" {
		cfg.Doubao.Model = model
	}
	if moduleName != "" {
		cfg.Output.ModuleName = moduleName
	}
	if replaceMode != "" {
		cfg.Output.ReplaceMode = replaceMode
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		log.LogError(fmt.Sprintf("Config validation failed: %v", err))
		return err
	}
	log.LogInfo("Config validated successfully")

	// 确定扫描目录：命令行参数优先，其次配置文件，最后默认当前目录
	scanDirs := getScanDirs(cfg.Scan.Dir, scanDir)
	log.LogInfo(fmt.Sprintf("Scanning directories: %v", scanDirs))

	// Create scanner
	s := scanner.New(cfg.Scan.IncludeExt, cfg.Scan.ExcludeDirs, cfg.Scan.ExcludePatterns)

	fmt.Printf("Scanning directories: %v\n", scanDirs)

	// Scan directories
	var allMatches []scanner.Match
	for _, dir := range scanDirs {
		matches, err := s.Scan(dir)
		if err != nil {
			log.LogError(fmt.Sprintf("Scan failed for %s: %v", dir, err))
			return fmt.Errorf("scan failed for %s: %w", dir, err)
		}
		allMatches = append(allMatches, matches...)
	}

	if len(allMatches) == 0 {
		msg := "No Chinese text found."
		fmt.Println(msg)
		log.LogInfo(msg)
		return nil
	}

	msg := fmt.Sprintf("Found %d Chinese text occurrences", len(allMatches))
	fmt.Println(msg)
	log.LogInfo(msg)

	// Get unique Chinese texts and their IDs
	uniqueTexts := scanner.UniqueChineseTexts(allMatches)
	msg = fmt.Sprintf("Unique texts to translate: %d", len(uniqueTexts))
	fmt.Println(msg)
	log.LogInfo(msg)

	// Create translator
	t := translator.New(cfg.Doubao.APIKey, cfg.Doubao.BaseURL, cfg.Doubao.Model)

	fmt.Println("Translating...")
	log.LogInfo("Starting translation...")

	// Translate
	ctx := context.Background()
	results, err := t.TranslateTexts(ctx, uniqueTexts)
	if err != nil {
		log.LogError(fmt.Sprintf("Translation failed: %v", err))
		return fmt.Errorf("translation failed: %w", err)
	}
	log.LogInfo(fmt.Sprintf("Translation completed: %d results", len(results)))

	// Create generators
	g := generator.New(outputDir, cfg.Output.ModuleName, cfg.Output.UpdatedBy)
	csvGen := generator.NewCSVGenerator(outputDir)

	// Generate SQL
	sqlPath, err := g.GenerateSQL(results)
	if err != nil {
		log.LogError(fmt.Sprintf("Failed to generate SQL: %v", err))
		return fmt.Errorf("failed to generate SQL: %w", err)
	}
	log.LogInfo(fmt.Sprintf("SQL file generated: %s", sqlPath))

	// Generate CSV
	if err := csvGen.AppendToCSV(results); err != nil {
		log.LogError(fmt.Sprintf("Failed to generate CSV: %v", err))
		// Don't return error, just log it
		fmt.Printf("Warning: Failed to generate CSV: %v\n", err)
	} else {
		csvPath := outputDir + "/aiAgent.csv"
		log.LogInfo(fmt.Sprintf("CSV file updated: %s", csvPath))
	}

	fmt.Printf("\nSQL file generated: %s\n", sqlPath)

	// Print report
	report := g.GenerateReport(results)
	fmt.Println(report)
	log.LogInfo(report)

	// Create a map of Chinese text to ID for replacement
	textToID := make(map[string]string)
	for _, result := range results {
		if result.Error == nil {
			textToID[result.Text] = result.ID
		}
	}

	// Update match IDs from translation results
	for i := range allMatches {
		if id, ok := textToID[allMatches[i].ChineseText]; ok {
			allMatches[i].ID = id
		}
	}

	// Replace if requested
	if replace || dryRun {
		r := replacer.NewWithMode(cfg.Output.ModuleName, dryRun, cfg.Output.ReplaceMode)

		if dryRun {
			preview := r.Preview(allMatches)
			fmt.Println(preview)
			log.LogInfo("Dry run preview generated")
		}

		if replace {
			fmt.Println("Replacing Chinese text in files...")
			log.LogInfo("Starting replacement...")
			replaceResults, err := r.ReplaceAll(allMatches)
			if err != nil {
				log.LogError(fmt.Sprintf("Replacement failed: %v", err))
				return fmt.Errorf("replacement failed: %w", err)
			}

			totalReplacements := 0
			for _, rr := range replaceResults {
				if rr.Error != nil {
					fmt.Printf("  Error in %s: %v\n", rr.FilePath, rr.Error)
					log.LogError(fmt.Sprintf("Error in %s: %v", rr.FilePath, rr.Error))
				} else if rr.Replacements > 0 {
					totalReplacements += rr.Replacements
					fmt.Printf("  %s: %d replacements\n", rr.FilePath, rr.Replacements)
					log.LogInfo(fmt.Sprintf("%s: %d replacements", rr.FilePath, rr.Replacements))
				}
			}
			fmt.Printf("\nTotal replacements: %d\n", totalReplacements)
			log.LogInfo(fmt.Sprintf("Total replacements: %d", totalReplacements))
		}
	}

	log.LogInfo("=== i18n translation process completed ===")
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := config.WriteExampleConfig(initOutput); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	fmt.Printf("Example configuration file created: %s\n", initOutput)
	return nil
}

// getScanDirs 确定要扫描的目录列表
//
// 优先级：
// 1. 如果命令行指定了 -d 参数（且不是默认值 "."），使用命令行参数
// 2. 如果配置文件中有 scan.dir 设置，使用配置文件的设置
// 3. 否则使用默认值 ["."]（当前目录）
//
// 参数说明：
//   - cfgDirs: 配置文件中 scan.dir 的值
//   - cmdDir: 命令行 -d 参数的值
//
// 返回值：要扫描的目录列表
func getScanDirs(cfgDirs []string, cmdDir string) []string {
	// 如果命令行参数不是默认值，优先使用命令行参数
	if cmdDir != "." {
		return []string{cmdDir}
	}

	// 如果配置文件中有设置，使用配置文件的设置
	if len(cfgDirs) > 0 {
		return cfgDirs
	}

	// 默认扫描当前目录
	return []string{"."}
}
