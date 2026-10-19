package capture

import (
	"path/filepath"
	"time"
)

// These names are the only place these strings appear in non-test code
// (architecture-notes.md decision 12).
const (
	claudeDirName      = ".claude"
	memoryDirName      = "memory"
	sessionsDirName    = "sessions"
	saltFileName       = "capture.salt"
	failuresFileName   = "capture.failures"
	purgeStampFileName = "capture.purge"
	configFileName     = "capture.yaml"
	settingsLocalName  = "settings.local.json"
	databaseFileName   = "episodes.db"
	dayFileExtension   = ".jsonl"
	dayFileDateLayout  = "2006-01-02"
)

// Layout is the single source of truth for capture paths. It is a value
// object built from the project root. It does no file system access.
type Layout struct {
	root string
}

// NewLayout builds the Layout of one project root.
func NewLayout(root string) Layout {
	return Layout{root: root}
}

// Root returns the project root the Layout was built from.
func (l Layout) Root() string { return l.root }

func (l Layout) memoryDir() string {
	return filepath.Join(l.root, claudeDirName, memoryDirName)
}

// SessionsDir is the directory that holds the day files.
func (l Layout) SessionsDir() string {
	return filepath.Join(l.memoryDir(), sessionsDirName)
}

// DayFile is the day file of the UTC date of the given instant. The name
// never comes from a payload.
func (l Layout) DayFile(at time.Time) string {
	name := at.UTC().Format(dayFileDateLayout) + dayFileExtension
	return filepath.Join(l.SessionsDir(), name)
}

// SaltPath is the Capture salt file.
func (l Layout) SaltPath() string { return filepath.Join(l.memoryDir(), saltFileName) }

// FailuresPath is the append-only failure counter file.
func (l Layout) FailuresPath() string { return filepath.Join(l.memoryDir(), failuresFileName) }

// PurgeStampPath is the retention purge rate-limit stamp.
func (l Layout) PurgeStampPath() string { return filepath.Join(l.memoryDir(), purgeStampFileName) }

// DatabasePath is the episodes.db projection.
func (l Layout) DatabasePath() string { return filepath.Join(l.memoryDir(), databaseFileName) }

// ConfigPath is the optional capture configuration file.
func (l Layout) ConfigPath() string { return filepath.Join(l.root, claudeDirName, configFileName) }

// SettingsLocalPath is the file that holds the hook registration.
func (l Layout) SettingsLocalPath() string {
	return filepath.Join(l.root, claudeDirName, settingsLocalName)
}
