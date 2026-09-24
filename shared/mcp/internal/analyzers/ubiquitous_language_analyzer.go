package analyzers

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type LanguageViolation struct {
	File        string `json:"file"`
	LineNumber  int    `json:"lineNumber"`
	InvalidTerm string `json:"invalidTerm"`
	Suggested   string `json:"suggested"`
}

type UbiquitousLanguageResult struct {
	Success         bool                `json:"success"`
	ProjectPath     string              `json:"projectPath"`
	ViolationsCount int                 `json:"violationsCount"`
	Violations      []LanguageViolation `json:"violations,omitempty"`
	Summary         string              `json:"summary"`
}

type UbiquitousLanguageAnalyzer struct{}

func NewUbiquitousLanguageAnalyzer() *UbiquitousLanguageAnalyzer {
	return &UbiquitousLanguageAnalyzer{}
}

var scannedSourceExtensions = map[string]struct{}{
	".go": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {}, ".py": {}, ".java": {}, ".cs": {},
}

var (
	backtickTermRe   = regexp.MustCompile("`([^`]+)`")
	sectionHeaderRe  = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	numberedHeaderRe = regexp.MustCompile(`^\d+\.\s`)
	synonymLineRe    = regexp.MustCompile(`(?i)^\s*\*\*(?:synonyms to avoid|synonyms|avoid|not)\*\*\s*:\s*(.+)$`)
	tableRowRe       = regexp.MustCompile(`^\|\s*\*\*([^*]+)\*\*\s*\|(.+)$`)
)

type termMatcher struct {
	re        *regexp.Regexp
	synonym   string
	canonical string
}

func (a *UbiquitousLanguageAnalyzer) Analyze(ctx context.Context, projectPath, dictionaryPath string) (*UbiquitousLanguageResult, error) {
	result := &UbiquitousLanguageResult{
		Success: true, ProjectPath: projectPath, Violations: []LanguageViolation{},
	}
	synonyms, err := loadDictionary(dictionaryPath)
	if err != nil {
		return nil, err
	}
	if len(synonyms) == 0 {
		result.Summary = "No ubiquitous language violations found"
		return result, nil
	}
	files, err := a.collectSourceFiles(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	matchers := compileMatchers(synonyms)
	if err := ForEachFile(ctx, files, func(file string) { a.scanFile(file, matchers, result) }); err != nil {
		return nil, err
	}
	result.ViolationsCount = len(result.Violations)
	if result.ViolationsCount == 0 {
		result.Summary = "No ubiquitous language violations found"
	} else {
		result.Summary = "Ubiquitous language violations found"
	}
	return result, nil
}

func loadDictionary(dictionaryPath string) (map[string]string, error) {
	f, err := os.Open(dictionaryPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	synonyms := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var currentTerm string
	for scanner.Scan() {
		line := scanner.Text()
		if m := sectionHeaderRe.FindStringSubmatch(line); m != nil {
			if !numberedHeaderRe.MatchString(m[1]) {
				currentTerm = m[1]
			} else {
				currentTerm = ""
			}
		}
		parseTableRow(line, synonyms)
		parseSynonymLine(line, currentTerm, synonyms)
	}
	return synonyms, nil
}

func parseTableRow(line string, synonyms map[string]string) {
	m := tableRowRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	canonical := strings.TrimSpace(m[1])
	parts := strings.Split(m[2], "|")
	for len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) < 2 {
		return
	}
	addSynonyms(parts[len(parts)-1], canonical, synonyms)
}

func parseSynonymLine(line, currentTerm string, synonyms map[string]string) {
	if currentTerm == "" {
		return
	}
	m := synonymLineRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	addSynonyms(m[1], currentTerm, synonyms)
}

func addSynonyms(raw, canonical string, synonyms map[string]string) {
	matches := backtickTermRe.FindAllStringSubmatch(raw, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			recordSynonym(m[1], canonical, synonyms)
		}
		return
	}
	for _, s := range strings.Split(raw, ",") {
		recordSynonym(s, canonical, synonyms)
	}
}

func recordSynonym(term, canonical string, synonyms map[string]string) {
	term = strings.Trim(strings.TrimSpace(term), "`*_ ")
	if idx := strings.Index(term, "("); idx > 0 {
		term = strings.TrimSpace(term[:idx])
	}
	if term != "" {
		synonyms[strings.ToLower(term)] = canonical
	}
}

func compileMatchers(synonyms map[string]string) []termMatcher {
	matchers := make([]termMatcher, 0, len(synonyms))
	for synonym, canonical := range synonyms {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(synonym) + `\b`)
		if err != nil {
			continue
		}
		matchers = append(matchers, termMatcher{re: re, synonym: synonym, canonical: canonical})
	}
	return matchers
}

func (a *UbiquitousLanguageAnalyzer) collectSourceFiles(ctx context.Context, root string) ([]string, error) {
	return CollectFiles(ctx, root, func(path string) bool {
		_, scanned := scannedSourceExtensions[strings.ToLower(filepath.Ext(path))]
		return scanned
	})
}

func (a *UbiquitousLanguageAnalyzer) scanFile(file string, matchers []termMatcher, result *UbiquitousLanguageResult) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, m := range matchers {
			if m.re.MatchString(line) {
				result.Violations = append(result.Violations, LanguageViolation{
					File: file, LineNumber: lineNum, InvalidTerm: m.synonym, Suggested: m.canonical,
				})
			}
		}
	}
}
