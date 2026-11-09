package intake

import (
	"errors"
	"strings"
	"testing"
)

// fakeFileReader serves files from an in-memory map, the seam every slice
// test uses instead of touching a real filesystem.
func fakeFileReader(files map[string][]byte) FileReader {
	return func(path string) ([]byte, error) {
		content, ok := files[path]
		if !ok {
			return nil, errors.New("no such file")
		}
		return content, nil
	}
}

func TestIsWithinProjectRoot(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"docs/specs/x/acceptance-criteria.json", true},
		{"a/b/../c", true},
		{"../x", false},
		{"a/../../x", false},
		{"/etc/passwd", false},
		{"", false},
		{"..", false},
	} {
		if got := isWithinProjectRoot(tc.path); got != tc.want {
			t.Errorf("isWithinProjectRoot(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestReadRootedRefusesAnEscapingPathWithoutCallingRead(t *testing.T) {
	calls := 0
	read := func(string) ([]byte, error) { calls++; return nil, nil }
	if _, err := readRooted("../x", read); err == nil {
		t.Error("want an error for an escaping path")
	}
	if calls != 0 {
		t.Errorf("read called %d times, want zero", calls)
	}
}

func TestReadRootedWrapsAReadError(t *testing.T) {
	wantErr := errors.New("boom")
	read := func(string) ([]byte, error) { return nil, wantErr }
	if _, err := readRooted("a/b.json", read); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want it to wrap %v", err, wantErr)
	}
}

func TestReadSpecPackageParsesCriteria(t *testing.T) {
	read := fakeFileReader(map[string][]byte{
		"spec.json": []byte(`{"criteria":[{"id":"AC1","observableVia":"both"}]}`),
	})
	pkg, err := readSpecPackage("spec.json", read)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(pkg.Criteria) != 1 || pkg.Criteria[0].ID != "AC1" || pkg.Criteria[0].ObservableVia != "both" {
		t.Errorf("pkg = %+v", pkg)
	}
}

func TestReadSpecPackageReportsInvalidJSON(t *testing.T) {
	read := fakeFileReader(map[string][]byte{"spec.json": []byte("not json")})
	if _, err := readSpecPackage("spec.json", read); err == nil {
		t.Error("want an error for invalid JSON")
	}
}

func TestReadSpecPackageRefusesAnEscapingPath(t *testing.T) {
	calls := 0
	read := func(string) ([]byte, error) { calls++; return nil, nil }
	if _, err := readSpecPackage("../spec.json", read); err == nil {
		t.Error("want an error for an escaping path")
	}
	if calls != 0 {
		t.Errorf("read called %d times, want zero", calls)
	}
}

func TestReadManifestParsesShaLines(t *testing.T) {
	read := fakeFileReader(map[string][]byte{
		"m.sha256": []byte("abc123  path/a.txt\ndef456  path/b.txt\n"),
	})
	m, err := readManifest("m.sha256", read)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if m.entries["path/a.txt"] != "abc123" || m.entries["path/b.txt"] != "def456" {
		t.Errorf("entries = %v", m.entries)
	}
}

func TestReadManifestRejectsAMalformedLine(t *testing.T) {
	read := fakeFileReader(map[string][]byte{"m.sha256": []byte("not-a-valid-line\n")})
	if _, err := readManifest("m.sha256", read); err == nil {
		t.Error("want an error for a malformed line")
	}
}

func TestLevelsOf(t *testing.T) {
	for _, tc := range []struct {
		via  string
		want []string
		ok   bool
	}{
		{"api", []string{"api"}, true},
		{"ui", []string{"ui"}, true},
		{"both", []string{"api", "ui"}, true},
		{"", nil, false},
		{"web", nil, false},
	} {
		levels, ok := levelsOf(Criterion{ObservableVia: tc.via})
		if ok != tc.ok || strings.Join(levels, ",") != strings.Join(tc.want, ",") {
			t.Errorf("levelsOf(%q) = %v, %v; want %v, %v", tc.via, levels, ok, tc.want, tc.ok)
		}
	}
}

func TestParseCoversEntry(t *testing.T) {
	if got := parseCoversEntry("AC1:ui"); got.criterionID != "AC1" || got.level != "ui" {
		t.Errorf("parseCoversEntry(%q) = %+v", "AC1:ui", got)
	}
	if got := parseCoversEntry("AC2"); got.criterionID != "AC2" || got.level != "" {
		t.Errorf("parseCoversEntry(%q) = %+v", "AC2", got)
	}
}

func TestSha256DigestAndHex(t *testing.T) {
	content := []byte("hello")
	hex := sha256Hex(content)
	if len(hex) != 64 {
		t.Errorf("sha256Hex length = %d, want 64", len(hex))
	}
	if got := sha256Digest(content); got != "sha256:"+hex {
		t.Errorf("sha256Digest = %q, want sha256: + sha256Hex", got)
	}
}
