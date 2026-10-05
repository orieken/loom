package repodoc

import (
	"fmt"
	"path"
	"strings"

	"github.com/orieken/loom/internal/factsheet"
)

// factIndex answers "does the fact sheet say this?" without the verifier
// re-reading the sheet's slices.
type factIndex struct {
	sheet factsheet.FactSheet
	// lists maps a citable fact-sheet field to its values.
	lists map[string][]string
	// exactNames and approximateNames are the names a prose claim may use.
	exactNames       map[string]bool
	approximateNames map[string]bool
}

func newFactIndex(sheet factsheet.FactSheet) factIndex {
	facts := factIndex{
		sheet: sheet,
		lists: map[string][]string{
			"files": sheet.Files, "exports": sheet.Exports, "imports": sheet.Imports,
			"approximate_imports": sheet.ApproximateImports, "dependents": sheet.Dependents,
			"approximate_dependents": sheet.ApproximateDependents, "entrypoints": sheet.Entrypoints,
			"tests": sheet.Tests, "children": sheet.Children, "child_summaries": sheet.Children,
		},
		exactNames: map[string]bool{}, approximateNames: map[string]bool{},
	}
	facts.collectNames()
	return facts
}

// collectNames gathers every name a claim may legitimately use. A directory is
// known by its full path and its last segment; a file by its name and its
// repo-relative path.
func (f *factIndex) collectNames() {
	for _, group := range [][]string{f.sheet.Files, f.sheet.Exports, f.sheet.Entrypoints, f.sheet.Tests} {
		for _, name := range group {
			f.exactNames[name] = true
			f.exactNames[path.Join(f.sheet.Path, name)] = true
		}
	}
	f.exactNames[f.sheet.Path], f.exactNames[baseName(f.sheet.Path)] = true, true
	for _, dir := range append(append(append([]string{}, f.sheet.Imports...), f.sheet.Dependents...), f.sheet.Children...) {
		f.exactNames[dir], f.exactNames[baseName(dir)] = true, true
	}
	for _, dir := range append(append([]string{}, f.sheet.ApproximateImports...), f.sheet.ApproximateDependents...) {
		f.approximateNames[dir], f.approximateNames[baseName(dir)] = true, true
	}
}

func (f factIndex) hasSymbolOrEntrypoint(name string) bool {
	return contains(f.sheet.Exports, name) || contains(f.sheet.Entrypoints, name)
}

func (f factIndex) hasFile(file string) bool {
	return contains(f.sheet.Files, file) || contains(f.sheet.Files, strings.TrimPrefix(file, f.sheet.Path+"/"))
}

func (f factIndex) sameFile(a, b string) bool {
	return strings.TrimPrefix(a, f.sheet.Path+"/") == strings.TrimPrefix(b, f.sheet.Path+"/")
}

// checkCitation validates `root` or `root:value`. A root alone must name a
// non-empty field — citing `tests` for a directory with no tests cites
// nothing — and a value must be one the field holds.
func (f factIndex) checkCitation(citation string) *Violation {
	root, value, hasValue := strings.Cut(citation, ":")
	values, known := f.lists[root]
	if !known {
		return &Violation{Source: SourceVerifier, Code: CodeUnknownCitation, Message: fmt.Sprintf("%q does not start with a fact-sheet field", citation)}
	}
	if !hasValue {
		return emptyCitation(root, citation, values)
	}
	if !contains(values, value) && (root != "files" || !f.hasFile(value)) {
		return &Violation{Source: SourceVerifier, Code: CodeUnknownCitation,
			Message: fmt.Sprintf("%q cites %q, which the fact sheet's %s does not list", citation, value, root)}
	}
	return nil
}

func emptyCitation(root, citation string, values []string) *Violation {
	if len(values) > 0 {
		return nil
	}
	return &Violation{Source: SourceVerifier, Code: CodeEmptyCitation,
		Message: fmt.Sprintf("%q cites %s, but the fact sheet lists none — that supports nothing", citation, root)}
}

// checkName judges one backticked token inside a claim.
func (f factIndex) checkName(name string, in claim) *Violation {
	if f.exactNames[name] || f.qualifiedExport(name) {
		return nil
	}
	if f.approximateNames[name] {
		if hasHedge(in.text) {
			return nil
		}
		return &Violation{Source: SourceVerifier, Code: CodeApproximateAsFact, Where: in.where,
			Message: fmt.Sprintf("`%s` is known only through an approximate edge; state it as possible or unconfirmed, not as fact", name)}
	}
	return &Violation{Source: SourceVerifier, Code: CodeUnknownName, Where: in.where,
		Message: fmt.Sprintf("`%s` is not in the fact sheet. Backticks are reserved for names the sheet contains", name)}
}

// qualifiedExport accepts `pkg.Symbol` when Symbol is an export.
func (f factIndex) qualifiedExport(name string) bool {
	_, symbol, qualified := strings.Cut(name, ".")
	return qualified && contains(f.sheet.Exports, symbol)
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
