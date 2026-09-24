package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrOutsideWorkspace reports a path argument that resolves outside the
// workspace root (roadmap L2.3).
var ErrOutsideWorkspace = errors.New("resolves outside the workspace root")

// WorkspaceRoot confines the paths a model passes as tool arguments to one
// directory, supplied by server configuration and never by an argument.
// Before it, `projectPath: "/"` walked the disk and `../../etc` read outside
// the project.
//
// Every argument is resolved with its symbolic links followed, then compared
// with the root's own resolved path, so a link inside the workspace that
// points outside it is rejected like the path it points to. Walks below a
// resolved path never follow links (analyzers.CollectFiles), which closes the
// same door one level down.
//
// Residual, stated rather than hidden: the check happens when the tool is
// called, and the analyzers then open paths by name, so a link swapped in
// between the two would be followed. Opening through os.Root end to end would
// close that; it is not worth rewriting five analyzers for while the server
// runs locally over stdio for one user.
type WorkspaceRoot struct {
	dir string
}

// NewWorkspaceRoot resolves dir, which must be an existing directory.
func NewWorkspaceRoot(dir string) (WorkspaceRoot, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return WorkspaceRoot{}, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return WorkspaceRoot{}, fmt.Errorf("workspace root %q: %w", dir, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return WorkspaceRoot{}, err
	}
	if !info.IsDir() {
		return WorkspaceRoot{}, fmt.Errorf("workspace root %q is not a directory", dir)
	}
	return WorkspaceRoot{dir: resolved}, nil
}

// Dir is the resolved root directory.
func (r WorkspaceRoot) Dir() string { return r.dir }

// Resolve turns a path argument into the absolute, link-free path it names,
// or an error when it is empty, does not exist, or lands outside the root. A
// relative argument is taken relative to the root, not to the server's
// working directory.
func (r WorkspaceRoot) Resolve(argument string) (string, error) {
	if strings.TrimSpace(argument) == "" {
		return "", errors.New("path is empty")
	}
	candidate := argument
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(r.dir, candidate)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("path %q does not exist under the workspace root", argument)
	}
	if !r.contains(resolved) {
		return "", fmt.Errorf("path %q %w %s", argument, ErrOutsideWorkspace, r.dir)
	}
	return resolved, nil
}

func (r WorkspaceRoot) contains(resolved string) bool {
	relative, err := filepath.Rel(r.dir, resolved)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
