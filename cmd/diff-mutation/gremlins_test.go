package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/mutation"
)

func TestArgumentsCarryTheSpikeSettingsAndNeverDiff(t *testing.T) {
	args := gremlinsArguments(mutation.Target{Dir: "internal/pkg", Exclude: []string{`^b\.go$`, "/"}}, "/tmp/r.json")
	want := []string{"unleash", "--workers", "1", "--timeout-coefficient", "20", "--output", "/tmp/r.json",
		"--exclude-files", `^b\.go$`, "--exclude-files", "/", "./internal/pkg"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args = %q\nwant   %q", args, want)
	}
	for _, arg := range args {
		if arg == "--diff" || arg == "-D" {
			t.Error("--diff passed: it does not scope in gremlins v0.6.0, and scoping it anyway would hide that")
		}
	}
}

func TestIntegrationTargetsAskForTheWholeSuite(t *testing.T) {
	args := gremlinsArguments(mutation.Target{Dir: "cmd/tool", Integration: true}, "/tmp/r.json")
	if !strings.Contains(strings.Join(args, " "), "--timeout-coefficient 150 ") {
		t.Errorf("args = %q, want the integration timeout coefficient", args)
	}
	if !strings.Contains(strings.Join(args, " "), "--integration ./cmd/tool") {
		t.Errorf("args = %q, want --integration before the target", args)
	}
	if plain := gremlinsArguments(mutation.Target{Dir: "pkg"}, "/tmp/r.json"); strings.Contains(strings.Join(plain, " "), "--integration") {
		t.Errorf("a package named like its directory asked for integration mode: %q", plain)
	}
}

// The adapter drives a stand-in binary so the real exec path — arguments in,
// report file out, failure surfaced with its output — is exercised without
// downloading gremlins into the test.
func TestRunGremlinsExecutesTheBinaryAndSurfacesItsFailure(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "gremlins")
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do if [ \"$prev\" = --output ]; then out=$a; fi; prev=$a; done\n" +
		"case \"$*\" in *./fail*) echo 'tests failed before mutation'; exit 3;; esac\n" +
		"printf '{\"files\": []}' > \"$out\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake gremlins: %v", err)
	}
	output := filepath.Join(dir, "r.json")

	if err := runGremlins(fake, mutation.Target{Dir: "pkg"}, output); err != nil {
		t.Fatalf("runGremlins: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Errorf("no report written where --output pointed: %v", err)
	}

	err := runGremlins(fake, mutation.Target{Dir: "fail"}, output)
	if err == nil || !strings.Contains(err.Error(), "tests failed before mutation") {
		t.Errorf("err = %v, want the binary's own output", err)
	}
}
