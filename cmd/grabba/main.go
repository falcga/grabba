package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/falcga/grabba/internal/analyzer"
)

var (
	// Version is set during build with -ldflags
	Version = "dev"
	// BuildTime is set during build with -ldflags
	BuildTime = "unknown"
)

func main() {
	var (
		filePath       string
		repoPath       string
		threshold      float64
		minLength      int
		maxLength      int
		confidence     float64
		showVersion    bool
		outputJSON     bool
		outputFile     string
		gitHistory     bool
		fileExtensions string
		excludeDirs    string
		maxFileSize    int64
	)

	flag.StringVar(&filePath, "file", "", "Analyze specific file")
	flag.StringVar(&repoPath, "repo", ".", "Path to Git repository for analysis")
	flag.Float64Var(&threshold, "threshold", 4.5, "Entropy threshold (bits)")
	flag.IntVar(&minLength, "min-length", 8, "Minimum secret length")
	flag.IntVar(&maxLength, "max-length", 256, "Maximum secret length")
	flag.Float64Var(&confidence, "confidence", 0.6, "Confidence threshold (0-1)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&outputJSON, "json", false, "Output in JSON format")
	flag.StringVar(&outputFile, "output", "", "Save results to file")
	flag.BoolVar(&gitHistory, "git-history", false, "Analyze Git history")
	flag.StringVar(&fileExtensions, "extensions", ".py,.js,.go,.java,.cpp,.env,.json,.yml,.yaml,.xml,.sh,.bash", "File extensions comma separated")
	flag.StringVar(&excludeDirs, "exclude", ".git,node_modules,__pycache__,.venv,venv,env,dist,build", "Directories to exclude comma separated")
	flag.Int64Var(&maxFileSize, "max-size", 1024*1024, "Maximum file size in bytes")

	flag.Parse()

	if showVersion {
		fmt.Printf("Grabba v%s\nBuilt: %s\n", Version, BuildTime)
		os.Exit(0)
	}

	if filePath != "" {
		abs, err := filepath.Abs(filePath)
		if err == nil {
			filePath = abs
		}
	}
	if repoPath != "" {
		abs, err := filepath.Abs(repoPath)
		if err == nil {
			repoPath = abs
		}
		if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
			fmt.Printf("[-] Repository path '%s' does not exist or is not a directory\n", repoPath)
			os.Exit(1)
		}
	}

	entropyAnalyzer := analyzer.NewAnalyzer(minLength, maxLength, threshold, confidence)

	var candidates []analyzer.SecretCandidate

	start := time.Now()

	switch {
	case filePath != "":
		fmt.Printf("[ ] Analyzing file: %s\n", filePath)
		// #nosec G304 – filePath is from trusted input (argument)
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("[-] Error reading file: %v\n", err)
			os.Exit(1)
		}
		candidates = entropyAnalyzer.AnalyzeText(string(content), filePath, true)
	case gitHistory:
		fmt.Printf("[ ] Analyzing Git history: %s\n", repoPath)
		candidates = entropyAnalyzer.AnalyzeGitHistory(repoPath)
	default:
		fmt.Printf("[ ] Analyzing repository: %s\n", repoPath)
		exts := strings.Split(fileExtensions, ",")
		excludes := strings.Split(excludeDirs, ",")
		candidates = entropyAnalyzer.AnalyzeRepository(
			repoPath,
			exts,
			maxFileSize,
			excludes,
		)
	}

	elapsed := time.Since(start)

	if outputJSON || outputFile != "" {
		result := map[string]interface{}{
			"candidates": candidates,
			"total":      len(candidates),
			"time":       elapsed.String(),
			"stats":      entropyAnalyzer.GetStats(),
		}

		if outputFile != "" {
			if err := entropyAnalyzer.ExportResults(candidates, outputFile); err != nil {
				fmt.Printf("[-] Error saving: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("[+] Results saved to %s\n", outputFile)
		} else {
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonData))
		}
	} else {
		if len(candidates) == 0 {
			fmt.Println("[+] No secrets found")
			entropyAnalyzer.PrintStats()
			os.Exit(0)
		}

		fmt.Printf("\n[-] Found %d potential secrets:\n\n", len(candidates))

		for idx, c := range candidates {
			if idx >= 20 {
				fmt.Printf("\n... and %d more results\n", len(candidates)-20)
				break
			}

			var levelColor string
			switch c.Level {
			case analyzer.LevelVeryHigh:
				levelColor = "\033[31m"
			case analyzer.LevelHigh:
				levelColor = "\033[33m"
			default:
				levelColor = "\033[36m"
			}

			fmt.Printf("%s[%d] Level: %s%s\033[0m\n", levelColor, idx+1, levelColor, c.Level)
			fmt.Printf("    File: %s:%d\n", c.FilePath, c.LineNumber)
			fmt.Printf("    Secret: %s\n", truncateString(c.Text, 60))
			fmt.Printf("    Entropy: %.2f bits | Confidence: %.1f%%\n", c.Entropy, c.Confidence*100)
			fmt.Printf("    Context: %s\n", c.Context)
			fmt.Println()
		}

		entropyAnalyzer.PrintStats()
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
