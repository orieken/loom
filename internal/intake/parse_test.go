package intake

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const validFile = `---
intake: 1
id: user-auth-lockout
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-23
requested-by: oscar
source: manual
source-ref: ""
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: [session-store]
budget:
  max-attempts: 2
  max-usd: 10
  max-hours: 4
scope:
  paths: ["src/auth/**"]
---

# Lock accounts after repeated failures
`

// Every field arrives, including an unquoted YAML date as the text written.
func TestParseReadsEveryOrderField(t *testing.T) {
	document, err := Parse([]byte(validFile))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := Order{
		Intake: 1, ID: "user-auth-lockout", Type: "feature", Plan: "deliver-feature", Status: "ready",
		Priority: 2, Created: "2026-10-23", RequestedBy: "oscar", Source: "manual", SourceRef: "",
		Trust: "internal", BaseBranch: "main", Finish: "pr", Autonomy: "inherit",
		DependsOn: []string{"session-store"},
		Budget:    Budget{MaxAttempts: 2, MaxUSD: 10, MaxHours: 4},
		Scope:     Scope{Paths: []string{"src/auth/**"}},
	}
	if !reflect.DeepEqual(document.Order, want) {
		t.Errorf("order = %+v\nwant    %+v", document.Order, want)
	}
}

func TestParseSeparatesTheBodyAndKnowsWhereItStarts(t *testing.T) {
	document, err := Parse([]byte(validFile))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(document.Body, "\n# Lock accounts") || strings.Contains(document.Body, "intake:") {
		t.Errorf("Body = %q, want only what follows the closing fence", document.Body)
	}
	if document.BodyLine != 24 {
		t.Errorf("BodyLine = %d, want 24 (the line after the closing fence)", document.BodyLine)
	}
}

// A misspelt key dropped silently is a setting the author believes they made.
func TestParseRejectsAnUnknownKey(t *testing.T) {
	source := strings.Replace(validFile, "priority: 2", "prority: 2", 1)
	if _, err := Parse([]byte(source)); err == nil || !strings.Contains(err.Error(), "prority") {
		t.Errorf("err = %v, want the unknown key named", err)
	}
}

func TestParseRefusesAFileWithNoOrder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   error
	}{
		{"no frontmatter", "# Just a spec\n", ErrNoFrontmatter},
		{"empty file", "", ErrNoFrontmatter},
		{"never closed", "---\nid: x\n# spec\n", ErrUnclosedFrontmatter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.source)); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestParseReportsMalformedYAML(t *testing.T) {
	if _, err := Parse([]byte("---\nid: [unclosed\n---\n")); err == nil || !strings.HasPrefix(err.Error(), "frontmatter:") {
		t.Errorf("err = %v, want a frontmatter error", err)
	}
}

// Empty frontmatter parses to an empty order, whose every rule then fails —
// rather than an error that hides which fields are missing.
func TestParseTreatsEmptyFrontmatterAsAnEmptyOrder(t *testing.T) {
	document, err := Parse([]byte("---\n---\nbody\n"))
	if err != nil || document.Order.ID != "" || document.Body != "body\n" {
		t.Errorf("document = %+v, err = %v", document, err)
	}
}

func TestParseAcceptsWindowsLineEndings(t *testing.T) {
	source := strings.ReplaceAll(validFile, "\n", "\r\n")
	if document, err := Parse([]byte(source)); err != nil || document.Order.ID != "user-auth-lockout" {
		t.Errorf("err = %v, id = %q", err, document.Order.ID)
	}
}

// The shipped template must stay parseable, so a copy of it is a valid
// starting point; its placeholders are expected to fail validation instead.
func TestTheShippedTemplateParses(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	template := filepath.Join(filepath.Dir(here), "..", "..", "templates", "claude-feature-team", "intake", "TEMPLATE.md")
	source, err := os.ReadFile(template)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	document, err := Parse(source)
	if err != nil {
		t.Fatalf("template does not parse: %v", err)
	}
	if document.Order.Intake != SchemaVersion || document.Order.Status != "draft" {
		t.Errorf("template order = %+v, want schema %d and status draft", document.Order, SchemaVersion)
	}
}
