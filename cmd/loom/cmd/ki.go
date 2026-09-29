package cmd

// `loom ki scan` — the deterministic injection check in front of synced
// Knowledge Items (roadmap L3.7). scripts/sync-memory.sh runs it on every org
// KI before writing one; it is equally usable by hand on a KI under review.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/injectionscan"
	"github.com/spf13/cobra"
)

// Exit codes a caller branches on: sync-memory.sh skips a flagged KI but
// aborts the whole pull when a file could not be scanned at all.
const (
	ExitCodeKIFlagged   = 1
	ExitCodeKIScanError = 2
)

// errKIFlagged reports that at least one file matched a rule.
var errKIFlagged = errors.New("instruction-override patterns found")

// errKIScan reports a file that could not be scanned — a scan that did not
// happen must never read as a clean one.
var errKIScan = errors.New("could not scan")

var kiCmd = &cobra.Command{
	Use:   "ki",
	Short: "Checks on Knowledge Items",
	Args:  cobra.NoArgs,
}

var kiScanCmd = &cobra.Command{
	Use:   "scan <file|directory>...",
	Short: "Flag text in Knowledge Items that addresses a model instead of informing it",
	Long: `Scans markdown files — or every .md directly inside a directory — for
instruction-override patterns: "ignore your previous instructions", role
markers, requests to bypass a gate or reveal a system prompt, secrecy toward
the user, and invisible characters.

Exits 0 when nothing matched, 1 when something did, and 2 when a file could
not be scanned. It catches naive injection, not every injection: a match is a
reason for a person to read the file, and silence is not proof it is safe.

Rules: ` + strings.Join(injectionscan.Rules(), ", "),
	Args: cobra.MinimumNArgs(1),
	RunE: runKIScan,
}

func init() {
	rootCmd.AddCommand(kiCmd)
	kiCmd.AddCommand(kiScanCmd)
}

func runKIScan(cmd *cobra.Command, args []string) error {
	files, err := markdownFiles(args)
	if err != nil {
		return fmt.Errorf("%w: %v", errKIScan, err)
	}
	flagged := 0
	for _, file := range files {
		findings, err := scanKIFile(file)
		if err != nil {
			return fmt.Errorf("%w %s: %v", errKIScan, file, err)
		}
		printFindings(cmd, file, findings)
		if len(findings) > 0 {
			flagged++
		}
	}
	if flagged > 0 {
		return fmt.Errorf("%w in %d of %d file(s)", errKIFlagged, flagged, len(files))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "clean: %d file(s), no instruction-override patterns\n", len(files))
	return nil
}

func scanKIFile(file string) ([]injectionscan.Finding, error) {
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return injectionscan.Scan(string(body)), nil
}

func printFindings(cmd *cobra.Command, file string, findings []injectionscan.Finding) {
	for _, finding := range findings {
		fmt.Fprintf(cmd.OutOrStdout(), "%s:%d %s: %s\n", file, finding.Line, finding.Rule, finding.Excerpt)
	}
}

// markdownFiles expands each argument: a file stands for itself, a
// directory for the .md files directly inside it.
func markdownFiles(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		expanded, err := expandArgument(arg)
		if err != nil {
			return nil, err
		}
		files = append(files, expanded...)
	}
	return files, nil
}

func expandArgument(arg string) ([]string, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{arg}, nil
	}
	return filepath.Glob(filepath.Join(arg, "*.md"))
}

// exitOnKIScan gives `loom ki scan` its own exit codes; cobra's default would
// make an unscannable file indistinguishable from a flagged one.
func exitOnKIScan(err error) {
	if errors.Is(err, errKIFlagged) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(ExitCodeKIFlagged)
	}
	if errors.Is(err, errKIScan) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(ExitCodeKIScanError)
	}
}
