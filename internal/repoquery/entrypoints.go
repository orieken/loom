package repoquery

import (
	"path"
	"sort"

	"github.com/orieken/loom/internal/factsheet"
)

// Entrypoint is a file that starts a program.
type Entrypoint struct {
	Path      string `json:"path"`
	Directory string `json:"directory"`
}

// Entrypoints lists every entrypoint the fact sheets record, sorted by path:
// a Go `func main`, a Python `__main__`, a Java `main`, a package.json
// `main` or `bin`.
func (s *Snapshot) Entrypoints() ([]Entrypoint, error) {
	facts, err := s.Facts()
	if err != nil {
		return nil, err
	}
	var found []Entrypoint
	for _, sheet := range facts {
		for _, file := range sheet.Entrypoints {
			found = append(found, Entrypoint{Path: joinRepoPath(sheet.Path, file), Directory: sheet.Path})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found, nil
}

func joinRepoPath(dir, name string) string {
	if dir == factsheet.RootPath {
		return name
	}
	return path.Join(dir, name)
}
