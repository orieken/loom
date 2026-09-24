package analyzers

import (
	"bufio"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type FunctionComplexity struct {
	File         string `json:"file"`
	FunctionName string `json:"functionName"`
	LineNumber   int    `json:"lineNumber"`
	Complexity   int    `json:"complexity"`
	LineCount    int    `json:"lineCount"`
	Status       string `json:"status"`
}

type ComplexityAnalysisResult struct {
	Success         bool                 `json:"success"`
	ProjectPath     string               `json:"projectPath"`
	TotalFiles      int                  `json:"totalFiles"`
	TotalFunctions  int                  `json:"totalFunctions"`
	ViolationsCount int                  `json:"violationsCount"`
	Violations      []FunctionComplexity `json:"violations,omitempty"`
	Summary         string               `json:"summary"`
}

type ComplexityAnalyzer struct{}

func NewComplexityAnalyzer() *ComplexityAnalyzer { return &ComplexityAnalyzer{} }

func (a *ComplexityAnalyzer) Analyze(ctx context.Context, projectPath string, maxComplexity, maxLines int) (*ComplexityAnalysisResult, error) {
	if maxComplexity <= 0 {
		maxComplexity = 7
	}
	if maxLines <= 0 {
		maxLines = 30
	}
	result := &ComplexityAnalysisResult{
		Success:     true,
		ProjectPath: projectPath,
		Violations:  []FunctionComplexity{},
	}
	files, err := collectSourceFiles(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	result.TotalFiles = len(files)
	if err := a.analyzeAll(ctx, files, maxComplexity, maxLines, result); err != nil {
		return nil, err
	}
	result.Summary = complexitySummary(result.ViolationsCount)
	return result, nil
}

func collectSourceFiles(ctx context.Context, projectPath string) ([]string, error) {
	return CollectFiles(ctx, projectPath, isAnalyzableExtension)
}

func isAnalyzableExtension(path string) bool {
	return strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".ts")
}

func complexitySummary(violationCount int) string {
	if violationCount > 0 {
		return "Complexity threshold violations found"
	}
	return "All functions pass complexity and LOC checks"
}

func (a *ComplexityAnalyzer) analyzeAll(ctx context.Context, files []string, maxComplexity, maxLines int, result *ComplexityAnalysisResult) error {
	return ForEachFile(ctx, files, func(file string) {
		if strings.HasSuffix(file, ".go") {
			a.analyzeGoFile(file, maxComplexity, maxLines, result)
			return
		}
		a.analyzeGenericFile(file, maxComplexity, maxLines, result)
	})
}

func (a *ComplexityAnalyzer) analyzeGoFile(file string, maxComplexity, maxLines int, result *ComplexityAnalysisResult) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		a.analyzeGenericFile(file, maxComplexity, maxLines, result)
		return
	}
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		result.TotalFunctions++
		startPos := fset.Position(fn.Pos())
		lineCount := fset.Position(fn.End()).Line - startPos.Line + 1
		complexity := calcCyclomaticComplexity(fn)
		if complexity > maxComplexity || lineCount > maxLines {
			result.ViolationsCount++
			result.Violations = append(result.Violations, FunctionComplexity{
				File: file, FunctionName: fn.Name.Name,
				LineNumber: startPos.Line, Complexity: complexity,
				LineCount: lineCount, Status: "VIOLATION",
			})
		}
	}
}

func calcCyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			_ = v
			complexity++
		case *ast.BinaryExpr:
			if v.Op == token.LAND || v.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func (a *ComplexityAnalyzer) analyzeGenericFile(file string, _ int, maxLines int, result *ComplexityAnalysisResult) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	result.TotalFunctions++
	if lines > maxLines {
		result.ViolationsCount++
		result.Violations = append(result.Violations, FunctionComplexity{
			File:         file,
			FunctionName: filepath.Base(file),
			LineNumber:   1,
			Complexity:   1,
			LineCount:    lines,
			Status:       "VIOLATION",
		})
	}
}
