package harness

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// run executes a command in dir under a deadline, returning what it printed.
// A failure carries the output, because the output is the evidence.
func run(dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = isolatedEnv()
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, output)
	}
	return string(output), nil
}

// isolatedEnv keeps the harness's own Go workspace and flags out of the
// fixture: it is a separate module, and must build as one.
func isolatedEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOFLAGS=")
}

// copyTree copies every file under source into destination.
func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(destination, relative))
	})
}

func copyFile(source, destination string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o644)
}
