package analyzers

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DependencyBoundaryViolation struct {
	File       string `json:"file"`
	LineNumber int    `json:"lineNumber"`
	FromLayer  string `json:"fromLayer"`
	ToLayer    string `json:"toLayer"`
	ImportPath string `json:"importPath"`
}

type DependencyVerificationResult struct {
	Success         bool                          `json:"success"`
	ProjectPath     string                        `json:"projectPath"`
	ViolationsCount int                           `json:"violationsCount"`
	Violations      []DependencyBoundaryViolation `json:"violations,omitempty"`
	Summary         string                        `json:"summary"`
}

type DependencyBoundaryAnalyzer struct{}

func NewDependencyBoundaryAnalyzer() *DependencyBoundaryAnalyzer {
	return &DependencyBoundaryAnalyzer{}
}

const (
	layerUnknown    = "unknown"
	layerDomain     = "domain"
	layerUsecases   = "usecases"
	layerAdapters   = "adapters"
	layerFrameworks = "frameworks"
)

var layerRank = map[string]int{
	layerDomain: 0, layerUsecases: 1, layerAdapters: 2, layerFrameworks: 3,
}

var layerSegments = map[string]string{
	"domain": layerDomain, "entities": layerDomain, "entity": layerDomain,
	"usecases": layerUsecases, "use-cases": layerUsecases, "use_cases": layerUsecases,
	"usecase": layerUsecases, "application": layerUsecases,
	"adapters": layerAdapters, "adapter": layerAdapters,
	"interfaces": layerAdapters, "infrastructure": layerAdapters,
	"frameworks": layerFrameworks,
}

var scannedImportExtensions = map[string]struct{}{".go": {}, ".ts": {}, ".tsx": {}}

var (
	goSingleImportRe    = regexp.MustCompile(`^\s*import\s+(?:[A-Za-z_.][\w]*\s+)?"([^"]+)"`)
	goImportBlockOpenRe = regexp.MustCompile(`^\s*import\s*\(\s*$`)
	goImportBlockLineRe = regexp.MustCompile(`^\s*(?:[A-Za-z_.][\w]*\s+)?"([^"]+)"\s*(?://.*)?$`)
	tsFromImportRe      = regexp.MustCompile(`from\s+['"]([^'"]+)['"]`)
	tsSideImportRe      = regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]`)
)

type importRef struct {
	Path string
	Line int
}

func (a *DependencyBoundaryAnalyzer) Analyze(projectPath string) (*DependencyVerificationResult, error) {
	result := &DependencyVerificationResult{
		Success: true, ProjectPath: projectPath, Violations: []DependencyBoundaryViolation{},
	}
	files, err := a.collectSourceFiles(projectPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		a.scanFile(f, result)
	}
	result.ViolationsCount = len(result.Violations)
	if result.ViolationsCount == 0 {
		result.Summary = "No dependency boundary violations found"
	} else {
		result.Summary = "Dependency boundary violations found"
	}
	return result, nil
}

func classifyLayer(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for _, p := range parts {
		if l, ok := layerSegments[strings.ToLower(p)]; ok {
			return l
		}
	}
	return layerUnknown
}

func isBoundaryViolation(from, to string) bool {
	fromRank, ok1 := layerRank[from]
	toRank, ok2 := layerRank[to]
	return ok1 && ok2 && fromRank < toRank
}

func (a *DependencyBoundaryAnalyzer) collectSourceFiles(root string) ([]string, error) {
	return CollectFiles(root, func(path string) bool {
		_, scanned := scannedImportExtensions[strings.ToLower(filepath.Ext(path))]
		return scanned
	})
}

func (a *DependencyBoundaryAnalyzer) scanFile(file string, result *DependencyVerificationResult) {
	fromLayer := classifyLayer(file)
	if fromLayer == layerUnknown {
		return
	}
	imports, err := readImports(file)
	if err != nil {
		return
	}
	for _, imp := range imports {
		toLayer := classifyLayer(imp.Path)
		if isBoundaryViolation(fromLayer, toLayer) {
			result.Violations = append(result.Violations, DependencyBoundaryViolation{
				File: file, LineNumber: imp.Line, FromLayer: fromLayer, ToLayer: toLayer, ImportPath: imp.Path,
			})
		}
	}
}

func readImports(file string) ([]importRef, error) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".go":
		return readGoImports(file)
	case ".ts", ".tsx":
		return readTSImports(file)
	}
	return nil, nil
}

func readGoImports(file string) ([]importRef, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var imports []importRef
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	inBlock := false
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if inBlock {
			inBlock, imports = parseGoBlockLine(line, lineNum, imports)
			continue
		}
		if goImportBlockOpenRe.MatchString(line) {
			inBlock = true
			continue
		}
		if m := goSingleImportRe.FindStringSubmatch(line); m != nil {
			imports = append(imports, importRef{Path: m[1], Line: lineNum})
		}
	}
	return imports, nil
}

func parseGoBlockLine(line string, lineNum int, imports []importRef) (stillInBlock bool, out []importRef) {
	if strings.TrimSpace(line) == ")" {
		return false, imports
	}
	if m := goImportBlockLineRe.FindStringSubmatch(line); m != nil {
		return true, append(imports, importRef{Path: m[1], Line: lineNum})
	}
	return true, imports
}

func readTSImports(file string) ([]importRef, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var imports []importRef
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if m := tsFromImportRe.FindStringSubmatch(line); m != nil {
			imports = append(imports, importRef{Path: m[1], Line: lineNum})
		} else if m := tsSideImportRe.FindStringSubmatch(line); m != nil {
			imports = append(imports, importRef{Path: m[1], Line: lineNum})
		}
	}
	return imports, nil
}
