package jsonlstore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// trimFailureLog keeps the failure log inside its bounds. It runs inside the
// Stop-time purge, at most once per PurgeInterval, and never on the append
// path. It keeps the newest lines that fit in capture.MaxFailureLogBytes and
// drops every line older than the retention age. A line it cannot parse, such
// as a byte of the old counter format, is dropped too. It rewrites the file
// only when that changes the text.
//
// A hook may append between the read and the rename. That one line is lost.
// The log is a diagnostic history, so the loss is accepted.
func trimFailureLog(layout capture.Layout, now time.Time, retention capture.Retention) error {
	removeErr := removeLegacyFailures(layout)
	path := layout.FailureLogPath()
	original, err := readTailOf(path, capture.MaxFailureLogBytes)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegularFile) {
		return removeErr
	}
	if err != nil {
		return errors.Join(removeErr, err)
	}
	kept := keepNewest(parseEntries(original), now, retention)
	if bytes.Equal(kept, original) && !isOverCap(path) {
		return removeErr
	}
	return errors.Join(removeErr, replaceFile(path, kept))
}

// removeLegacyFailures deletes the one-byte-per-failure counter of an earlier
// install. It holds no timestamps, so no history can be kept from it.
func removeLegacyFailures(layout capture.Layout) error {
	err := os.Remove(layout.LegacyFailuresPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func isOverCap(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Size() > capture.MaxFailureLogBytes
}

// keepNewest renders the entries that are not older than the retention age.
// The input is oldest first and already fits the byte cap.
func keepNewest(entries []capture.FailureEntry, now time.Time, retention capture.Retention) []byte {
	oldest := now.AddDate(0, 0, -retention.MaxAgeDays)
	var text strings.Builder
	for _, entry := range entries {
		if !entry.At.Before(oldest) {
			text.WriteString(entry.Line())
		}
	}
	return []byte(text.String())
}

// replaceFile writes the text beside the log and renames it over the log, so
// a reader sees the old file or the new file and never half of one.
func replaceFile(path string, text []byte) error {
	temporary := path + capture.FailureLogTempSuffix
	if err := os.WriteFile(temporary, text, dayFileMode); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}
