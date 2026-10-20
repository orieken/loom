package capture

import (
	"path/filepath"
	"strings"
	"time"
)

// These names are the only place these strings appear in non-test code
// (architecture-notes.md decision 12).
const (
	claudeDirName      = ".claude"
	memoryDirName      = "memory"
	sessionsDirName    = "sessions"
	saltFileName       = "capture.salt"
	failureLogFileName = "capture.failures.log"
	// legacyFailuresFileName is the one-byte-per-failure counter of an earlier
	// install. Nothing writes it. Purge and uninstall remove it.
	legacyFailuresFileName = "capture.failures"
	purgeStampFileName     = "capture.purge"
	configFileName         = "capture.yaml"
	settingsLocalName      = "settings.local.json"
	databaseFileName       = "episodes.db"
	dayFileExtension       = ".jsonl"
	dayFileDateLayout      = "2006-01-02"
)

// FailureLogTempSuffix names the scratch file of the failure-log trim. The
// trim and Layout.FailureLogTempPath both use it, so they cannot drift apart.
const FailureLogTempSuffix = ".tmp"

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

// DayFileDate returns the UTC date that a day-file name stands for. It is
// the inverse of DayFile and reports false for any other name, so a foreign
// file in the sessions directory is never taken for a day file.
func DayFileDate(name string) (time.Time, bool) {
	stem, hasExtension := strings.CutSuffix(name, dayFileExtension)
	date, err := time.Parse(dayFileDateLayout, stem)
	if !hasExtension || err != nil || date.Format(dayFileDateLayout) != stem {
		return time.Time{}, false
	}
	return date, true
}

// SaltPath is the Capture salt file.
func (l Layout) SaltPath() string { return filepath.Join(l.memoryDir(), saltFileName) }

// FailureLogPath is the append-only failure log: one line per hook failure.
func (l Layout) FailureLogPath() string { return filepath.Join(l.memoryDir(), failureLogFileName) }

// FailureLogTempPath is the scratch file that the failure-log trim writes
// before it renames it over the log. A crash between the write and the rename
// leaves it behind, so uninstall removes it too.
func (l Layout) FailureLogTempPath() string { return l.FailureLogPath() + FailureLogTempSuffix }

// LegacyFailuresPath is the old counter file of an earlier install. The hook
// and health never read it; the purge and uninstall remove it.
func (l Layout) LegacyFailuresPath() string {
	return filepath.Join(l.memoryDir(), legacyFailuresFileName)
}

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
