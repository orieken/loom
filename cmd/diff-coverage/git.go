package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout bounds each git call. They are local commands, but an unbounded
// one would hang CI rather than fail it.
const gitTimeout = 60 * time.Second

// gitDiff is the production diffSource: zero-context, no colour, no external
// diff driver, renames detected so a moved file contributes only its edits.
//
// `git diff <base>` does not show untracked files, so a new file not yet
// added would be silently skipped — a vacuous pass locally that CI would
// never see. Untracked Go files are appended as wholly added.
func gitDiff(base string) (io.Reader, error) {
	tracked, err := git("diff", "--unified=0", "--no-color", "--no-ext-diff", "--find-renames", base, "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", base, err)
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	additions, err := untrackedAsAdditions(untracked)
	if err != nil {
		return nil, err
	}
	return io.MultiReader(bytes.NewReader(tracked), strings.NewReader(additions)), nil
}

func git(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, "git", args...)
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// untrackedAsAdditions renders each listed file as a diff that adds every line.
func untrackedAsAdditions(listing []byte) (string, error) {
	var additions strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(listing))
	for scanner.Scan() {
		lines, err := countLines(scanner.Text())
		if err != nil {
			return "", err
		}
		if lines > 0 {
			fmt.Fprintf(&additions, "+++ b/%s\n@@ -0,0 +1,%d @@\n", scanner.Text(), lines)
		}
	}
	return additions.String(), scanner.Err()
}

func countLines(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := bytes.Count(content, []byte("\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return lines, nil
}
