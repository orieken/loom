package patternspec

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// specText is a valid hand-written spec with the given name and status.
func specText(name string, status Status) string {
	return strings.Join([]string{
		"schemaVersion: 1", "name: " + name, "version: 1.0.0", "intent: Do the thing.", "owner: team",
		"triggers: {keywords: [], intentPatterns: []}",
		"steps: [{id: run, description: Run it., executor: tool}]",
		"provenance: {source: hand-written, status: " + string(status) + "}",
	}, "\n")
}

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

func scan(project fs.FS) Report { return Scan(project, knownReferences) }

func issueWith(report Report, file, field, message string) (Issue, bool) {
	for _, issue := range report.Issues {
		if issue.File == file && issue.Field == field && strings.Contains(issue.Message, message) {
			return issue, true
		}
	}
	return Issue{}, false
}

// @issue F2 @ac a project without pattern specs reports zero specs
func TestAProjectWithoutPatternDirectoriesHasNoSpecsAndNoIssues(t *testing.T) {
	report := scan(fstest.MapFS{"README.md": file("hello")})

	if len(report.Documents) != 0 || len(report.Issues) != 0 {
		t.Errorf("want an empty report, got %+v", report)
	}
}

// @issue F2 @ac a valid local spec passes and is counted
func TestAValidLocalSpecIsCounted(t *testing.T) {
	report := scan(fstest.MapFS{".claude/patterns/do-thing.pattern.yaml": file(specText("do-thing", StatusCandidate))})

	if len(report.Documents) != 1 || report.Failures() != 0 {
		t.Fatalf("want one valid document, got %+v", report)
	}
	if document := report.Documents[0]; document.Location != LocationLocal || document.Path != ".claude/patterns/do-thing.pattern.yaml" {
		t.Errorf("document = %+v", document)
	}
}

// @issue F2 @ac a name that differs from the file name fails and shows both
func TestANameThatDiffersFromTheFileNameFails(t *testing.T) {
	report := scan(fstest.MapFS{".claude/patterns/do-thing.pattern.yaml": file(specText("other-thing", StatusCandidate))})

	issue, found := issueWith(report, ".claude/patterns/do-thing.pattern.yaml", "name", `name "other-thing" does not match the file name "do-thing"`)
	if !found || !issue.IsFailure() {
		t.Errorf("want a name failure, got %v", report.Issues)
	}
	if len(report.Documents) != 0 {
		t.Errorf("an invalid spec was counted: %+v", report.Documents)
	}
}

// @issue F2 @ac a candidate in shared/patterns fails
func TestACandidateInTheSharedTreeFails(t *testing.T) {
	for _, dir := range []string{SharedDir, ExamplesDir} {
		t.Run(dir, func(t *testing.T) {
			path := dir + "/do-thing.pattern.yaml"

			report := scan(fstest.MapFS{path: file(specText("do-thing", StatusCandidate))})

			if _, found := issueWith(report, path, "provenance.status", "MUST be approved"); !found {
				t.Errorf("want a status failure, got %v", report.Issues)
			}
		})
	}
}

func TestEveryPromotableStatusMayBeShared(t *testing.T) {
	for _, status := range []Status{StatusApproved, StatusForged, StatusRetired} {
		t.Run(string(status), func(t *testing.T) {
			report := scan(fstest.MapFS{SharedDir + "/do-thing.pattern.yaml": file(specText("do-thing", status))})

			if report.Failures() != 0 || len(report.Documents) != 1 {
				t.Errorf("status %s refused in shared: %v", status, report.Issues)
			}
		})
	}
}

// @issue F2 @ac a local spec that overrides a shared spec warns
func TestALocalSpecOverridingASharedSpecWarns(t *testing.T) {
	report := scan(fstest.MapFS{
		SharedDir + "/do-thing.pattern.yaml": file(specText("do-thing", StatusApproved)),
		LocalDir + "/do-thing.pattern.yaml":  file(specText("do-thing", StatusCandidate)),
	})

	issue, found := issueWith(report, LocalDir+"/do-thing.pattern.yaml", "name", `overrides the shared spec "do-thing"`)
	if !found || issue.IsFailure() {
		t.Errorf("want an override warning, got %v", report.Issues)
	}
	if report.Failures() != 0 || len(report.Documents) != 2 {
		t.Errorf("an override is not a failure: %+v", report)
	}
}

func TestALocalSpecNamedLikeAnExampleDoesNotWarn(t *testing.T) {
	report := scan(fstest.MapFS{
		ExamplesDir + "/do-thing.pattern.yaml": file(specText("do-thing", StatusApproved)),
		LocalDir + "/do-thing.pattern.yaml":    file(specText("do-thing", StatusCandidate)),
	})

	if len(report.Issues) != 0 {
		t.Errorf("an example is not a promoted spec: %v", report.Issues)
	}
}

func TestFilesWithoutThePatternSuffixAndDirectoriesAreIgnored(t *testing.T) {
	report := scan(fstest.MapFS{
		LocalDir + "/notes.md":              file("# not a spec"),
		LocalDir + "/do-thing.yaml":         file("not: a spec"),
		LocalDir + "/nested/x.pattern.yaml": file("not scanned"),
		LocalDir + "/do-thing.pattern.yaml": file(specText("do-thing", StatusCandidate)),
	})

	if len(report.Issues) != 0 || len(report.Documents) != 1 {
		t.Errorf("want only the one spec read, got %+v", report)
	}
}

// openTracking records which files a scan opened.
type openTracking struct {
	fstest.MapFS
	opened []string
}

func (tracking *openTracking) Open(name string) (fs.File, error) {
	tracking.opened = append(tracking.opened, name)
	return tracking.MapFS.Open(name)
}

// @issue F2 @ac a file over the size cap fails without a full read
func TestAFileOverTheCapFailsWithoutBeingOpened(t *testing.T) {
	path := LocalDir + "/huge.pattern.yaml"
	project := &openTracking{MapFS: fstest.MapFS{path: file(strings.Repeat("a", MaxSpecBytes+1))}}

	report := scan(project)

	if _, found := issueWith(report, path, rootField, "larger than 256 KB"); !found {
		t.Errorf("want a size failure, got %v", report.Issues)
	}
	for _, opened := range project.opened {
		if opened == path {
			t.Error("the oversized file was opened")
		}
	}
}

func TestAFileAtTheCapIsRead(t *testing.T) {
	text := specText("do-thing", StatusCandidate)
	padded := text + "\n#" + strings.Repeat(" ", MaxSpecBytes-len(text)-2)

	report := scan(fstest.MapFS{LocalDir + "/do-thing.pattern.yaml": file(padded)})

	if len(padded) != MaxSpecBytes || len(report.Documents) != 1 {
		t.Errorf("a file of exactly %d bytes (got %d) was refused: %v", MaxSpecBytes, len(padded), report.Issues)
	}
}

// failingFS fails one operation on one path.
type failingFS struct {
	fstest.MapFS
	readDirFails string
	openFails    string
}

func (failing failingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == failing.readDirFails {
		return nil, errors.New("permission denied")
	}
	return failing.MapFS.ReadDir(name)
}

func (failing failingFS) Open(name string) (fs.File, error) {
	if name == failing.openFails {
		return nil, errors.New("permission denied")
	}
	return failing.MapFS.Open(name)
}

// @issue F2 @ac an unreadable pattern directory fails and is named
func TestAnUnreadableDirectoryFailsAndIsNamed(t *testing.T) {
	project := failingFS{MapFS: fstest.MapFS{LocalDir + "/x.pattern.yaml": file("")}, readDirFails: LocalDir}

	report := scan(project)

	if _, found := issueWith(report, LocalDir, rootField, "cannot be read"); !found {
		t.Errorf("want a directory failure, got %v", report.Issues)
	}
}

func TestAnUnreadableFileFails(t *testing.T) {
	path := LocalDir + "/do-thing.pattern.yaml"
	project := failingFS{MapFS: fstest.MapFS{path: file(specText("do-thing", StatusCandidate))}, openFails: path}

	report := scan(project)

	if _, found := issueWith(report, path, rootField, "cannot be read"); !found {
		t.Errorf("want a file failure, got %v", report.Issues)
	}
}

func TestAnInvalidSpecIsReportedWithItsFileAndNotCounted(t *testing.T) {
	path := LocalDir + "/broken.pattern.yaml"
	source := readFixture(t, filepath.Join("testdata", "invalid", "step-without-executor"+FileSuffix))

	report := scan(fstest.MapFS{path: {Data: source}})

	if _, found := issueWith(report, path, "steps[0].executor", "is required"); !found || len(report.Documents) != 0 {
		t.Errorf("want the step failure in %s, got %+v", path, report)
	}
}

func TestIssueStringNamesTheFileTheFieldAndTheMessage(t *testing.T) {
	issue := Issue{File: "a.pattern.yaml", Field: "steps[0].executor", Message: "is required", Severity: SeverityFailure}

	if got := issue.String(); got != "a.pattern.yaml: steps[0].executor: is required" {
		t.Errorf("String() = %q", got)
	}
}
