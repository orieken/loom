package factory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/orieken/loom/internal/intake"
)

// Repository is the git the supervisor needs: reading a commit, and adding
// and removing worktrees. internal/worktree.Git satisfies it.
type Repository interface {
	ResolveCommit(ref string) (string, error)
	ListFiles(commit, dir string) ([]string, error)
	ShowFile(commit, path string) ([]byte, error)
	AddWorktree(path, branch, base string) error
	RemoveWorktree(path string) error
}

// notOrders are the files in intake/ that describe orders rather than being one.
var notOrders = map[string]bool{"TEMPLATE.md": true, "README.md": true}

// Candidate is one committed intake file, checked.
type Candidate struct {
	ID         string
	Path       string
	Content    []byte
	SHA256     string
	Document   intake.Document
	Violations []intake.Violation
}

// IsReady reports whether the order asks to be worked on now. Validity is a
// separate question: an order can be ready and broken.
func (c Candidate) IsReady() bool {
	return c.Document.Order.Status == intake.StatusReady
}

// IsClaimable reports whether the order may be claimed: ready and valid.
func (c Candidate) IsClaimable() bool {
	return c.IsReady() && len(c.Violations) == 0
}

// Intake is every order committed on a branch, at one commit.
type Intake struct {
	Commit string
	Orders map[string]Candidate
}

// ReadIntake reads intake/ as committed at the head of branch (ADR-010, 2B):
// a working-tree edit is invisible until it is committed, so an order
// reaches the factory reviewed, the way code reaches main.
func ReadIntake(repo Repository, branch string, plans []string) (Intake, error) {
	commit, err := repo.ResolveCommit(branch)
	if err != nil {
		return Intake{}, fmt.Errorf("read intake from %q: %w", branch, err)
	}
	files, err := orderFiles(repo, commit)
	if err != nil {
		return Intake{}, err
	}
	features, err := featureStems(repo, commit)
	if err != nil {
		return Intake{}, err
	}
	orders := map[string]Candidate{}
	for _, file := range files {
		candidate, err := readCandidate(repo, commit, file, intake.Context{Plans: plans, OtherSpecs: features})
		if err != nil {
			return Intake{}, err
		}
		orders[candidate.ID] = candidate
	}
	return Intake{Commit: commit, Orders: orders}, nil
}

func orderFiles(repo Repository, commit string) ([]string, error) {
	files, err := repo.ListFiles(commit, IntakeDir)
	if err != nil {
		return nil, fmt.Errorf("list %s/ at %s: %w", IntakeDir, commit, err)
	}
	var orders []string
	for _, file := range files {
		if strings.HasSuffix(file, ".md") && !notOrders[path.Base(file)] {
			orders = append(orders, file)
		}
	}
	return orders, nil
}

// featureStems are the stems of the committed features/*.md specs, which an
// order must not reuse (review item I-10).
func featureStems(repo Repository, commit string) (map[string]string, error) {
	files, err := repo.ListFiles(commit, FeaturesDir)
	if err != nil {
		return nil, fmt.Errorf("list %s/ at %s: %w", FeaturesDir, commit, err)
	}
	stems := map[string]string{}
	for _, file := range files {
		if strings.HasSuffix(file, ".md") && !notOrders[path.Base(file)] {
			stems[stem(file)] = file
		}
	}
	return stems, nil
}

func readCandidate(repo Repository, commit, file string, context intake.Context) (Candidate, error) {
	content, err := repo.ShowFile(commit, file)
	if err != nil {
		return Candidate{}, fmt.Errorf("read %s at %s: %w", file, commit, err)
	}
	context.FileStem = stem(file)
	document, _ := intake.Parse(content) // a parse failure is reported by Check below
	sum := sha256.Sum256(content)
	return Candidate{
		ID: context.FileStem, Path: file, Content: content, SHA256: hex.EncodeToString(sum[:]),
		Document: document, Violations: intake.Check(content, context),
	}, nil
}

func stem(file string) string {
	return strings.TrimSuffix(path.Base(file), path.Ext(file))
}

// Title is the spec's first heading, for commit messages and PR titles.
func Title(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}
