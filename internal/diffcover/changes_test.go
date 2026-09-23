package diffcover_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
)

const sampleDiff = `diff --git a/internal/a/a.go b/internal/a/a.go
index 1111111..2222222 100644
--- a/internal/a/a.go
+++ b/internal/a/a.go
@@ -10,2 +10,3 @@ func Existing() {
@@ -20 +21 @@ func Other() {
@@ -30,4 +32,0 @@ func Deleted() {
diff --git a/internal/b/new.go b/internal/b/new.go
new file mode 100644
--- /dev/null
+++ b/internal/b/new.go
@@ -0,0 +1,2 @@
diff --git a/internal/c/gone.go b/internal/c/gone.go
deleted file mode 100644
--- a/internal/c/gone.go
+++ /dev/null
@@ -1,5 +0,0 @@
`

func TestParseDiffKeepsOnlyAddedLinesOfSurvivingFiles(t *testing.T) {
	changes, err := diffcover.ParseDiff(strings.NewReader(sampleDiff))
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	want := diffcover.Changes{
		// +10,3 is three lines; +21 has no count and is one; +32,0 only deletes.
		"internal/a/a.go": {10, 11, 12, 21},
		// A new file: every line is added.
		"internal/b/new.go": {1, 2},
		// internal/c/gone.go was deleted — nothing left to cover.
	}
	if !reflect.DeepEqual(changes, want) {
		t.Errorf("changes = %v, want %v", changes, want)
	}
}

func TestParseDiffRejectsAHunkItCannotRead(t *testing.T) {
	for name, hunk := range map[string]string{
		"no new range":  "@@ -1,2 @@",
		"bad start":     "@@ -1,2 +x,2 @@",
		"bad count":     "@@ -1,2 +3,y @@",
		"missing range": "@@",
	} {
		t.Run(name, func(t *testing.T) {
			diff := "+++ b/a.go\n" + hunk + "\n"
			if _, err := diffcover.ParseDiff(strings.NewReader(diff)); err == nil {
				t.Errorf("accepted %q — a hunk that cannot be read would drop changed lines from the measurement", hunk)
			}
		})
	}
}
