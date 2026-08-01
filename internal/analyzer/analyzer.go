package analyzer

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type EntropyLevel string

const (
	LevelLow      EntropyLevel = "low"
	LevelMedium   EntropyLevel = "medium"
	LevelHigh     EntropyLevel = "high"
	LevelVeryHigh EntropyLevel = "very_high"
)

type SecretCandidate struct {
	Text              string       `json:"text"`
	Entropy           float64      `json:"entropy"`
	NormalizedEntropy float64      `json:"normalized_entropy"`
	Level             EntropyLevel `json:"level"`
	Alphabet          string       `json:"alphabet"`
	AlphabetSize      int          `json:"alphabet_size"`
	Length            int          `json:"length"`
	UniqueChars       int          `json:"unique_chars"`
	LineNumber        int          `json:"line_number"`
	FilePath          string       `json:"file_path"`
	Context           string       `json:"context"`
	Confidence        float64      `json:"confidence"`
}

type ByConfidence []SecretCandidate

func (a ByConfidence) Len() int           { return len(a) }
func (a ByConfidence) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByConfidence) Less(i, j int) bool { return a[i].Confidence > a[j].Confidence }

type ShannonEntropyAnalyzer struct {
	MinLength           int
	MaxLength           int
	EntropyThreshold    float64
	ConfidenceThreshold float64
	MaxWorkers          int

	alphabetCache      sync.Map
	entropyCache       sync.Map
	falsePositiveCache sync.Map
	mu                 sync.RWMutex
	stats              AnalyzerStats
}

type AnalyzerStats struct {
	FilesScanned    int64
	LinesProcessed  int64
	CandidatesFound int64
	CacheHits       int64
	CacheMisses     int64
	ProcessingTime  time.Duration
}

var Alphabets = map[string]string{
	"base64":       "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",
	"base64_url":   "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_",
	"base32":       "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567=",
	"hex":          "0123456789abcdefABCDEF",
	"lowercase":    "abcdefghijklmnopqrstuvwxyz",
	"uppercase":    "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"digits":       "0123456789",
	"alphanumeric": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789",
	"jwt":          "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_",
}

var (
	keywordPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:api[_\s-]?key|apikey|api-token|secret|token|password|passwd|pwd|bearer|access_key|private_key)`),
	}

	exactPatterns = map[string]*regexp.Regexp{
		"aws_key":     regexp.MustCompile(`AKIA[0-9A-Z]{16,}`),
		"private_key": regexp.MustCompile(`-----BEGIN (?:RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`),
		"jwt":         regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`),
		"url_secret":  regexp.MustCompile(`[?&](?:key|token|secret|api_key|apikey)=([a-zA-Z0-9_\-\.]{8,})`),
	}

	falsePositivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^[a-f0-9]{32}$`),
		regexp.MustCompile(`^[a-f0-9]{40}$`),
		regexp.MustCompile(`^[a-f0-9]{64}$`),
		regexp.MustCompile(`^[a-f0-9]{128}$`),
		regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`),
		regexp.MustCompile(`^[0-9]{6,}$`),
		regexp.MustCompile(`^(?:test|example|demo|sample|dummy)[a-z]*$`),
		regexp.MustCompile(`^(?:foo|bar|baz|qux)[a-z]*$`),
	}
)

func NewAnalyzer(minLength, maxLength int, entropyThreshold, confidenceThreshold float64) *ShannonEntropyAnalyzer {
	if minLength == 0 {
		minLength = 8
	}
	if maxLength == 0 {
		maxLength = 256
	}
	if entropyThreshold == 0 {
		entropyThreshold = 4.5
	}
	if confidenceThreshold == 0 {
		confidenceThreshold = 0.6
	}

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}

	return &ShannonEntropyAnalyzer{
		MinLength:           minLength,
		MaxLength:           maxLength,
		EntropyThreshold:    entropyThreshold,
		ConfidenceThreshold: confidenceThreshold,
		MaxWorkers:          workers,
	}
}

func (a *ShannonEntropyAnalyzer) AnalyzeText(text, filePath string, lineNumbers bool) []SecretCandidate {
	start := time.Now()
	defer func() {
		a.mu.Lock()
		a.stats.ProcessingTime += time.Since(start)
		a.mu.Unlock()
	}()

	var candidates []SecretCandidate
	lines := strings.Split(text, "\n")

	if a.MaxWorkers > 1 && len(lines) > 1000 {
		return a.analyzeTextParallel(lines, filePath, lineNumbers)
	}

	for i, line := range lines {
		if lineNumbers {
			a.analyzeLine(line, filePath, i+1, &candidates)
		} else {
			a.analyzeLine(line, filePath, 0, &candidates)
		}
	}

	sort.Sort(ByConfidence(candidates))
	return candidates
}

func (a *ShannonEntropyAnalyzer) analyzeTextParallel(lines []string, filePath string, lineNumbers bool) []SecretCandidate {
	var (
		mu         sync.Mutex
		candidates []SecretCandidate
		wg         sync.WaitGroup
		ch         = make(chan struct {
			line string
			num  int
		}, len(lines))
	)

	for i := 0; i < a.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var local []SecretCandidate
			for item := range ch {
				if lineNumbers {
					a.analyzeLine(item.line, filePath, item.num, &local)
				} else {
					a.analyzeLine(item.line, filePath, 0, &local)
				}
			}
			mu.Lock()
			candidates = append(candidates, local...)
			mu.Unlock()
		}()
	}

	for i, line := range lines {
		if line == "" {
			continue
		}
		ch <- struct {
			line string
			num  int
		}{line, i + 1}
	}
	close(ch)

	wg.Wait()
	sort.Sort(ByConfidence(candidates))
	return candidates
}

func (a *ShannonEntropyAnalyzer) extractSecretFromLine(line string) (string, bool) {
	for _, kw := range keywordPatterns {
		loc := kw.FindStringIndex(line)
		if loc == nil {
			continue
		}
		startIdx := loc[1]
		remaining := line[startIdx:]

		trimmed := strings.TrimLeft(remaining, " \t:=:")

		if len(trimmed) == 0 {
			continue
		}

		var secret string
		if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, `'`) {
			quote := trimmed[0]
			endIdx := strings.IndexByte(trimmed[1:], quote)
			if endIdx != -1 {
				secret = trimmed[1 : endIdx+1]
			} else {
				secret = trimmed[1:]
			}
		} else {
			secretChars := regexp.MustCompile(`^[a-zA-Z0-9\+\/\=\-_\.]+`)
			match := secretChars.FindString(trimmed)
			if match != "" {
				secret = match
			} else {
				continue
			}
		}

		if secret != "" {
			return secret, true
		}
	}
	return "", false
}

func (a *ShannonEntropyAnalyzer) analyzeLine(line, filePath string, lineNum int, candidates *[]SecretCandidate) {
	atomic.AddInt64(&a.stats.LinesProcessed, 1)

	for patternName, pattern := range exactPatterns {
		matches := pattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			var secret string
			if len(match) > 1 {
				secret = match[1]
			} else {
				secret = match[0]
			}
			if len(secret) < a.MinLength || len(secret) > a.MaxLength {
				continue
			}
			if a.isFalsePositive(secret) {
				continue
			}
			if candidate := a.analyzeCandidate(secret, line, lineNum, filePath, patternName); candidate != nil {
				*candidates = append(*candidates, *candidate)
				atomic.AddInt64(&a.stats.CandidatesFound, 1)
			}
		}
	}

	if secret, found := a.extractSecretFromLine(line); found {
		if len(secret) < a.MinLength || len(secret) > a.MaxLength {
			return
		}
		if a.isFalsePositive(secret) {
			return
		}
		if candidate := a.analyzeCandidate(secret, line, lineNum, filePath, "keyword"); candidate != nil {
			*candidates = append(*candidates, *candidate)
			atomic.AddInt64(&a.stats.CandidatesFound, 1)
		}
	}
}

func (a *ShannonEntropyAnalyzer) analyzeCandidate(secret, line string, lineNum int, filePath, patternName string) *SecretCandidate {
	alphabet := a.determineBestAlphabet(secret)
	if alphabet == "" {
		return nil
	}

	entropy := a.calculateEntropy(secret, alphabet)
	if entropy < a.EntropyThreshold {
		return nil
	}

	level := getEntropyLevel(entropy)
	confidence := a.calculateConfidence(secret, entropy, patternName, alphabet)

	if confidence < a.ConfidenceThreshold {
		return nil
	}

	maxEntropy := math.Log2(float64(len(alphabet)))
	normalizedEntropy := entropy / maxEntropy

	return &SecretCandidate{
		Text:              secret,
		Entropy:           entropy,
		NormalizedEntropy: normalizedEntropy,
		Level:             level,
		Alphabet:          truncateString(alphabet, 50),
		AlphabetSize:      len(alphabet),
		Length:            len(secret),
		UniqueChars:       len(uniqueChars(secret)),
		LineNumber:        lineNum,
		FilePath:          filePath,
		Context:           getContext(line, secret),
		Confidence:        confidence,
	}
}

func (a *ShannonEntropyAnalyzer) calculateEntropy(text, alphabet string) float64 {
	cacheKey := text + "|" + alphabet
	if val, ok := a.entropyCache.Load(cacheKey); ok {
		atomic.AddInt64(&a.stats.CacheHits, 1)
		return val.(float64)
	}
	atomic.AddInt64(&a.stats.CacheMisses, 1)

	if len(text) < 256 {
		var freq [256]int
		validCount := 0

		for i := 0; i < len(text); i++ {
			c := text[i]
			if strings.ContainsRune(alphabet, rune(c)) {
				freq[c]++
				validCount++
			}
		}

		if validCount == 0 {
			return 0.0
		}

		entropy := 0.0
		for _, count := range freq {
			if count > 0 {
				prob := float64(count) / float64(validCount)
				entropy -= prob * math.Log2(prob)
			}
		}

		a.entropyCache.Store(cacheKey, entropy)
		return entropy
	}

	freq := make(map[rune]int)
	validCount := 0

	for _, ch := range text {
		if strings.ContainsRune(alphabet, ch) {
			freq[ch]++
			validCount++
		}
	}

	if validCount == 0 {
		return 0.0
	}

	entropy := 0.0
	for _, count := range freq {
		prob := float64(count) / float64(validCount)
		entropy -= prob * math.Log2(prob)
	}

	a.entropyCache.Store(cacheKey, entropy)
	return entropy
}

func (a *ShannonEntropyAnalyzer) determineBestAlphabet(text string) string {
	if val, ok := a.alphabetCache.Load(text); ok {
		return val.(string)
	}

	chars := uniqueChars(text)
	bestAlphabet := ""
	bestCoverage := 0.0

	for _, alphabet := range Alphabets {
		alphabetSet := make(map[rune]bool)
		for _, ch := range alphabet {
			alphabetSet[ch] = true
		}

		matchCount := 0
		for _, ch := range chars {
			if alphabetSet[ch] {
				matchCount++
			}
		}

		coverage := float64(matchCount) / float64(len(chars))
		if coverage > bestCoverage && coverage >= 0.8 {
			bestCoverage = coverage
			bestAlphabet = alphabet
		}
	}

	if bestAlphabet != "" {
		a.alphabetCache.Store(text, bestAlphabet)
		return bestAlphabet
	}

	custom := string(chars)
	a.alphabetCache.Store(text, custom)
	return custom
}

func (a *ShannonEntropyAnalyzer) isFalsePositive(text string) bool {
	if val, ok := a.falsePositiveCache.Load(text); ok {
		return val.(bool)
	}

	for _, pattern := range falsePositivePatterns {
		if pattern.MatchString(text) {
			a.falsePositiveCache.Store(text, true)
			return true
		}
	}

	if len(uniqueChars(text)) == 1 {
		a.falsePositiveCache.Store(text, true)
		return true
	}

	commonWords := map[string]bool{
		"password": true, "secret": true, "token": true,
		"key": true, "api": true, "admin": true,
	}
	if commonWords[strings.ToLower(text)] {
		a.falsePositiveCache.Store(text, true)
		return true
	}

	a.falsePositiveCache.Store(text, false)
	return false
}

func (a *ShannonEntropyAnalyzer) calculateConfidence(secret string, entropy float64, patternName, alphabet string) float64 {
	confidence := 0.0

	maxEntropy := math.Log2(float64(len(alphabet)))
	if maxEntropy > 0 {
		normalized := entropy / maxEntropy
		confidence += normalized * 0.4
	}

	lengthFactor := float64(len(secret)-a.MinLength) / float64(a.MaxLength-a.MinLength)
	if lengthFactor > 1.0 {
		lengthFactor = 1.0
	}
	if lengthFactor < 0 {
		lengthFactor = 0
	}
	confidence += lengthFactor * 0.2

	patternWeights := map[string]float64{
		"aws_key":       0.35,
		"private_key":   0.35,
		"jwt":           0.30,
		"keyword":       0.25,
		"url_secret":    0.15,
	}
	if weight, ok := patternWeights[patternName]; ok {
		confidence += weight
	} else {
		confidence += 0.10
	}

	uniqueRatio := float64(len(uniqueChars(secret))) / float64(len(secret))
	confidence += uniqueRatio * 0.05

	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

func (a *ShannonEntropyAnalyzer) AnalyzeRepository(repoPath string, fileExtensions []string, maxFileSize int64, excludeDirs []string) []SecretCandidate {
	start := time.Now()
	defer func() {
		a.mu.Lock()
		a.stats.ProcessingTime += time.Since(start)
		a.mu.Unlock()
	}()

	if len(fileExtensions) == 0 {
		fileExtensions = []string{".py", ".js", ".ts", ".java", ".cpp", ".go", ".rb", ".php", ".sh", ".bash", ".yml", ".yaml", ".json", ".xml", ".env", ".txt", ".md", ".conf", ".config", ".ini", ".toml", ".properties"}
	}

	if maxFileSize == 0 {
		maxFileSize = 1024 * 1024
	}

	if excludeDirs == nil {
		excludeDirs = []string{".git", "node_modules", "__pycache__", ".venv", "venv", "env", "dist", "build"}
	}

	var allCandidates []SecretCandidate
	var mu sync.Mutex
	var wg sync.WaitGroup

	var files []string
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			for _, exclude := range excludeDirs {
				if strings.Contains(path, exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := filepath.Ext(path)
		found := false
		for _, allowedExt := range fileExtensions {
			if ext == allowedExt {
				found = true
				break
			}
		}
		if !found {
			return nil
		}

		if info.Size() > maxFileSize {
			return nil
		}

		files = append(files, path)
		atomic.AddInt64(&a.stats.FilesScanned, 1)
		return nil
	})

	if err != nil {
		return allCandidates
	}

	fileChan := make(chan string, len(files))
	for i := 0; i < a.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				candidates := a.analyzeFile(filePath)
				if len(candidates) > 0 {
					mu.Lock()
					allCandidates = append(allCandidates, candidates...)
					mu.Unlock()
				}
			}
		}()
	}

	for _, filePath := range files {
		fileChan <- filePath
	}
	close(fileChan)

	wg.Wait()
	sort.Sort(ByConfidence(allCandidates))
	return allCandidates
}

func (a *ShannonEntropyAnalyzer) analyzeFile(filePath string) []SecretCandidate {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	if !isTextFile(file) {
		return nil
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil
	}

	return a.AnalyzeText(string(content), filePath, true)
}

func (a *ShannonEntropyAnalyzer) AnalyzeGitHistory(repoPath string) []SecretCandidate {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil
	}

	var candidates []SecretCandidate
	var mu sync.Mutex

	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil
	}

	err = commitIter.ForEach(func(commit *object.Commit) error {
		msgCandidates := a.AnalyzeText(commit.Message, commit.Hash.String()+".commit", false)

		tree, err := commit.Tree()
		if err == nil {
			tree.Files().ForEach(func(file *object.File) error {
				content, err := file.Contents()
				if err == nil {
					fileCandidates := a.AnalyzeText(content, file.Name, true)
					mu.Lock()
					candidates = append(candidates, fileCandidates...)
					mu.Unlock()
				}
				return nil
			})
		}

		mu.Lock()
		candidates = append(candidates, msgCandidates...)
		mu.Unlock()

		return nil
	})

	if err != nil {
		return candidates
	}

	sort.Sort(ByConfidence(candidates))
	return candidates
}

func uniqueChars(s string) []rune {
	seen := make(map[rune]bool)
	var result []rune
	for _, ch := range s {
		if !seen[ch] {
			seen[ch] = true
			result = append(result, ch)
		}
	}
	return result
}

func getEntropyLevel(entropy float64) EntropyLevel {
	if entropy < 3.0 {
		return LevelLow
	} else if entropy < 4.5 {
		return LevelMedium
	} else if entropy < 5.5 {
		return LevelHigh
	}
	return LevelVeryHigh
}

func getContext(line, secret string) string {
	start := strings.Index(line, secret)
	if start == -1 {
		if len(line) > 100 {
			return line[:100] + "..."
		}
		return line
	}

	ctxStart := start - 50
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := start + len(secret) + 50
	if ctxEnd > len(line) {
		ctxEnd = len(line)
	}

	context := line[ctxStart:ctxEnd]
	masked := strings.Replace(context, secret, strings.Repeat("*", min(10, len(secret))), -1)
	return masked
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isTextFile(file *os.File) bool {
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	file.Seek(0, 0)

	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return false
		}
	}
	return true
}

func (a *ShannonEntropyAnalyzer) ExportResults(candidates []SecretCandidate, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"results":   candidates,
		"stats":     a.GetStats(),
		"total":     len(candidates),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (a *ShannonEntropyAnalyzer) GetStats() AnalyzerStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats
}

func (a *ShannonEntropyAnalyzer) ClearCache() {
	a.entropyCache = sync.Map{}
	a.alphabetCache = sync.Map{}
	a.falsePositiveCache = sync.Map{}
}

func (a *ShannonEntropyAnalyzer) PrintStats() {
	stats := a.GetStats()
	fmt.Printf("Statistics:\n")
	fmt.Printf("  files processed: %d\n", stats.FilesScanned)
	fmt.Printf("  lines processed: %d\n", stats.LinesProcessed)
	fmt.Printf("  candidates: %d\n", stats.CandidatesFound)
	fmt.Printf("  cache hits: %d\n", stats.CacheHits)
	fmt.Printf("  cache miss: %d\n", stats.CacheMisses)
	fmt.Printf("  time: %v\n", stats.ProcessingTime)
}
