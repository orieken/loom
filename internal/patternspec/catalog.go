package patternspec

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// Where pattern specs live, relative to the project root.
const (
	SharedDir   = "shared/patterns"
	ExamplesDir = "shared/patterns/examples"
	LocalDir    = ".claude/patterns"
	FileSuffix  = ".pattern.yaml"
)

// MaxSpecBytes caps one spec file. A larger file fails without a full read.
const MaxSpecBytes = 256 * 1024

// Location says which pattern directory a spec came from.
type Location string

// The pattern locations.
const (
	LocationShared  Location = "shared"
	LocationExample Location = "example"
	LocationLocal   Location = "local"
)

// isInSharedTree reports whether specs here MUST be promotable: both
// shared/patterns/ and its examples are part of the shared tree.
func (location Location) isInSharedTree() bool { return location != LocationLocal }

// patternDirectories are scanned in this order, so messages are stable.
var patternDirectories = []struct {
	dir      string
	location Location
}{
	{SharedDir, LocationShared},
	{ExamplesDir, LocationExample},
	{LocalDir, LocationLocal},
}

// Document is one valid spec and where it was found.
type Document struct {
	Path     string
	Location Location
	Spec     Spec
}

// Report is the result of scanning a project's pattern directories.
type Report struct {
	Documents []Document
	Issues    []Issue
}

// Failures counts the issues that fail validation.
func (report Report) Failures() int { return failureCount(report.Issues) }

// Scan validates every *.pattern.yaml in the pattern directories of
// project. A directory that does not exist holds no specs; one that cannot
// be read is a failure.
func Scan(project fs.FS, references ReferenceResolver) Report {
	var report Report
	for _, directory := range patternDirectories {
		scanDirectory(project, directory.dir, directory.location, references, &report)
	}
	report.Issues = append(report.Issues, overrideWarnings(report.Documents)...)
	return report
}

func scanDirectory(project fs.FS, dir string, location Location, references ReferenceResolver, report *Report) {
	entries, err := fs.ReadDir(project, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		report.Issues = append(report.Issues, Issue{File: dir, Field: rootField, Message: "the pattern directory cannot be read", Severity: SeverityFailure})
		return
	}
	for _, entry := range entries {
		if isPatternFile(entry) {
			report.add(loadFile(project, path.Join(dir, entry.Name()), location, references))
		}
	}
}

func isPatternFile(entry fs.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), FileSuffix)
}

// add records a file's issues, and the document when nothing failed.
func (report *Report) add(document Document, issues []Issue) {
	report.Issues = append(report.Issues, inFile(document.Path, issues)...)
	if failureCount(issues) == 0 {
		report.Documents = append(report.Documents, document)
	}
}

func loadFile(project fs.FS, file string, location Location, references ReferenceResolver) (Document, []Issue) {
	source, issue, isRead := readCapped(project, file)
	if !isRead {
		return Document{Path: file}, []Issue{issue}
	}
	spec, issues := Parse(source)
	if len(issues) > 0 {
		return Document{Path: file}, issues
	}
	issues = append(locationIssues(spec, file, location), Validate(spec, references)...)
	return Document{Path: file, Location: location, Spec: spec}, issues
}

// readCapped refuses a file over MaxSpecBytes from its size, before it
// reads it, and never reads more than the cap.
func readCapped(project fs.FS, file string) ([]byte, Issue, bool) {
	tooLarge := failure(rootField, fmt.Sprintf("the file is larger than %d KB", MaxSpecBytes/1024))
	if info, err := fs.Stat(project, file); err == nil && info.Size() > MaxSpecBytes {
		return nil, tooLarge, false
	}
	opened, err := project.Open(file)
	if err != nil {
		return nil, failure(rootField, "the file cannot be read"), false
	}
	defer opened.Close()
	source, err := io.ReadAll(io.LimitReader(opened, MaxSpecBytes+1))
	if err != nil {
		return nil, failure(rootField, "the file cannot be read"), false
	}
	if len(source) > MaxSpecBytes {
		return nil, tooLarge, false
	}
	return source, Issue{}, true
}

// locationIssues are the rules that need to know where the spec lives.
func locationIssues(spec Spec, file string, location Location) []Issue {
	var issues []Issue
	stem := strings.TrimSuffix(path.Base(file), FileSuffix)
	if spec.Name != stem {
		issues = append(issues, failure("name", fmt.Sprintf("name %q does not match the file name %q", spec.Name, stem)))
	}
	if location.isInSharedTree() && !spec.Provenance.Status.IsPromotable() {
		issues = append(issues, failure("provenance.status", "a spec in shared/patterns MUST be approved; a candidate belongs in "+LocalDir))
	}
	return issues
}

// overrideWarnings flags a local spec that shadows a shared one. Examples
// are not promoted specs, so a local spec never overrides one.
func overrideWarnings(documents []Document) []Issue {
	shared := map[string]bool{}
	for _, document := range documents {
		if document.Location == LocationShared {
			shared[document.Spec.Name] = true
		}
	}
	var issues []Issue
	for _, document := range documents {
		if document.Location == LocationLocal && shared[document.Spec.Name] {
			message := fmt.Sprintf("overrides the shared spec %q in %s", document.Spec.Name, SharedDir)
			issues = append(issues, Issue{File: document.Path, Field: "name", Message: message, Severity: SeverityWarning})
		}
	}
	return issues
}
