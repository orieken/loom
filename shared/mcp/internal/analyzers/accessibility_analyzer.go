package analyzers

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type AccessibilityViolation struct {
	File        string `json:"file"`
	LineNumber  int    `json:"lineNumber"`
	Element     string `json:"element"`
	Rule        string `json:"rule"`
	Description string `json:"description"`
}

type AccessibilityReportResult struct {
	Success         bool                     `json:"success"`
	Path            string                   `json:"path"`
	TotalFiles      int                      `json:"totalFiles"`
	ViolationsCount int                      `json:"violationsCount"`
	Violations      []AccessibilityViolation `json:"violations,omitempty"`
	Summary         string                   `json:"summary"`
}

type AccessibilityAnalyzer struct{}

func NewAccessibilityAnalyzer() *AccessibilityAnalyzer { return &AccessibilityAnalyzer{} }

var scannedExtensions = map[string]struct{}{
	".html": {}, ".htm": {}, ".vue": {}, ".jsx": {}, ".tsx": {}, ".svelte": {},
}

const (
	ruleClickableNonInteractive = "clickable-non-interactive"
	ruleImgAlt                  = "img-alt"
	ruleAnchorName              = "anchor-name"
	ruleInputLabel              = "input-label"
	ruleHTMLLang                = "html-lang"
)

var (
	clickableNonInteractiveRe = regexp.MustCompile(`(?i)<(div|span)\b[^>]*\bon[a-z]+\s*=`)
	imgTagRe                  = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	imgHasAltRe               = regexp.MustCompile(`(?i)\balt\s*=`)
	anchorOpenRe              = regexp.MustCompile(`(?i)<a\b[^>]*>`)
	ariaLabelRe               = regexp.MustCompile(`(?i)\baria-label\s*=\s*["'][^"']+["']`)
	inputTagRe                = regexp.MustCompile(`(?i)<input\b[^>]*>`)
	inputHasIDRe              = regexp.MustCompile(`(?i)\bid\s*=\s*["']([^"']+)["']`)
	inputTypeHiddenRe         = regexp.MustCompile(`(?i)\btype\s*=\s*["']hidden["']`)
	labelForRe                = regexp.MustCompile(`(?i)<label\b[^>]*\bfor\s*=\s*["']([^"']+)["']`)
	htmlTagRe                 = regexp.MustCompile(`(?i)<html\b[^>]*>`)
	htmlHasLangRe             = regexp.MustCompile(`(?i)\blang\s*=`)
)

func (a *AccessibilityAnalyzer) Analyze(ctx context.Context, path string) (*AccessibilityReportResult, error) {
	result := &AccessibilityReportResult{Success: true, Path: path, Violations: []AccessibilityViolation{}}

	files, err := a.collectFiles(ctx, path)
	if err != nil {
		return nil, err
	}
	result.TotalFiles = len(files)
	if err := ForEachFile(ctx, files, func(file string) { a.analyzeFile(file, result) }); err != nil {
		return nil, err
	}
	result.ViolationsCount = len(result.Violations)
	if result.ViolationsCount == 0 {
		result.Summary = "No accessibility violations found"
	} else {
		result.Summary = "Accessibility violations found"
	}
	return result, nil
}

func (a *AccessibilityAnalyzer) collectFiles(ctx context.Context, path string) ([]string, error) {
	return CollectFiles(ctx, path, func(candidate string) bool {
		_, scanned := scannedExtensions[strings.ToLower(filepath.Ext(candidate))]
		return scanned
	})
}

func (a *AccessibilityAnalyzer) analyzeFile(file string, result *AccessibilityReportResult) {
	labels := collectLabelIDs(file)
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
		checkHTMLLang(file, lineNum, line, result)
		checkClickableNonInteractive(file, lineNum, line, result)
		checkImgAlt(file, lineNum, line, result)
		checkAnchorName(file, lineNum, line, result)
		checkInputLabel(file, lineNum, line, labels, result)
	}
}

func checkHTMLLang(file string, lineNum int, line string, result *AccessibilityReportResult) {
	if !htmlTagRe.MatchString(line) || htmlHasLangRe.MatchString(line) {
		return
	}
	result.Violations = append(result.Violations, AccessibilityViolation{
		File: file, LineNumber: lineNum, Element: "html",
		Rule: ruleHTMLLang, Description: "<html> element is missing a lang attribute",
	})
}

func checkClickableNonInteractive(file string, lineNum int, line string, result *AccessibilityReportResult) {
	m := clickableNonInteractiveRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	el := strings.ToLower(m[1])
	result.Violations = append(result.Violations, AccessibilityViolation{
		File: file, LineNumber: lineNum, Element: el,
		Rule:        ruleClickableNonInteractive,
		Description: "Non-interactive element (<" + el + ">) has an event handler; use <button> or <a> instead",
	})
}

func checkImgAlt(file string, lineNum int, line string, result *AccessibilityReportResult) {
	tag := imgTagRe.FindString(line)
	if tag == "" || imgHasAltRe.MatchString(tag) {
		return
	}
	result.Violations = append(result.Violations, AccessibilityViolation{
		File: file, LineNumber: lineNum, Element: "img",
		Rule: ruleImgAlt, Description: "<img> element is missing an alt attribute",
	})
}

func checkAnchorName(file string, lineNum int, line string, result *AccessibilityReportResult) {
	openTag := anchorOpenRe.FindString(line)
	if openTag == "" || ariaLabelRe.MatchString(openTag) {
		return
	}
	rest := line[strings.Index(line, openTag)+len(openTag):]
	closeIdx := strings.Index(strings.ToLower(rest), "</a>")
	hasText := closeIdx >= 0 && strings.TrimSpace(rest[:closeIdx]) != "" ||
		closeIdx < 0 && strings.TrimSpace(rest) != ""
	if hasText {
		return
	}
	result.Violations = append(result.Violations, AccessibilityViolation{
		File: file, LineNumber: lineNum, Element: "a",
		Rule: ruleAnchorName, Description: "<a> element has no visible text and no aria-label",
	})
}

func checkInputLabel(file string, lineNum int, line string, labels map[string]struct{}, result *AccessibilityReportResult) {
	tag := inputTagRe.FindString(line)
	if tag == "" || inputTypeHiddenRe.MatchString(tag) || ariaLabelRe.MatchString(tag) {
		return
	}
	if idMatch := inputHasIDRe.FindStringSubmatch(tag); idMatch != nil {
		if _, ok := labels[idMatch[1]]; ok {
			return
		}
	}
	result.Violations = append(result.Violations, AccessibilityViolation{
		File: file, LineNumber: lineNum, Element: "input",
		Rule: ruleInputLabel, Description: "<input> element has no associated <label for=...> and no aria-label",
	})
}

func collectLabelIDs(file string) map[string]struct{} {
	ids := map[string]struct{}{}
	f, err := os.Open(file)
	if err != nil {
		return ids
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		for _, m := range labelForRe.FindAllStringSubmatch(scanner.Text(), -1) {
			ids[m[1]] = struct{}{}
		}
	}
	return ids
}
