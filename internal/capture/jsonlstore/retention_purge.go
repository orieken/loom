package jsonlstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/capture"
)

const hoursPerDay = 24

var _ capture.RetentionPurger = Purger{}

// Purger deletes old day files (architecture-notes.md decision 10). It takes
// no lock. A double purge is harmless, and a file that vanished is ignored.
type Purger struct {
	layout capture.Layout
}

// NewPurger builds the purger of one project layout.
func NewPurger(layout capture.Layout) Purger {
	return Purger{layout: layout}
}

// dayFile is a day file as the purge sees it.
type dayFile struct {
	path string
	date time.Time
	size int64
}

// PurgeIfDue does nothing when the stamp is younger than PurgeInterval. It
// touches the stamp before it deletes anything, so two sessions that pass
// the check together purge twice at most. It deletes by age first. It then
// deletes the oldest files while the store is over the cap. The files of
// today and yesterday (UTC) are never deleted, because another session may
// still append to them.
func (p Purger) PurgeIfDue(ctx context.Context, now time.Time, retention capture.Retention) error {
	if !p.isDue(now) {
		return nil
	}
	if err := p.touchStamp(now); err != nil {
		return err
	}
	files, err := p.dayFiles()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return removeAll(selectForPurge(files, now, retention))
}

func (p Purger) isDue(now time.Time) bool {
	info, err := os.Stat(p.layout.PurgeStampPath())
	return err != nil || now.Sub(info.ModTime()) >= capture.PurgeInterval
}

func (p Purger) touchStamp(now time.Time) error {
	path := p.layout.PurgeStampPath()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, dayFileMode)
	if err != nil {
		return err
	}
	return errors.Join(file.Close(), os.Chtimes(path, now, now))
}

// dayFiles lists the files whose name is a day-file name, oldest first. A
// missing directory is an empty store.
func (p Purger) dayFiles() ([]dayFile, error) {
	entries, err := os.ReadDir(p.layout.SessionsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []dayFile
	for _, entry := range entries {
		date, isDayFile := capture.DayFileDate(entry.Name())
		info, infoErr := entry.Info()
		if isDayFile && entry.Type().IsRegular() && infoErr == nil {
			path := filepath.Join(p.layout.SessionsDir(), entry.Name())
			files = append(files, dayFile{path: path, date: date, size: info.Size()})
		}
	}
	return files, nil
}

// selectForPurge picks the files to delete. The input is oldest first.
func selectForPurge(files []dayFile, now time.Time, retention capture.Retention) []dayFile {
	today := now.UTC().Truncate(hoursPerDay * time.Hour)
	total := totalSize(files)
	var doomed []dayFile
	for _, file := range files {
		isProtected := !file.date.Before(today.AddDate(0, 0, -1))
		if isProtected {
			continue
		}
		if isTooOld(file, today, retention) || total > retention.MaxBytes {
			doomed = append(doomed, file)
			total -= file.size
		}
	}
	return doomed
}

func isTooOld(file dayFile, today time.Time, retention capture.Retention) bool {
	ageDays := int(today.Sub(file.date) / (hoursPerDay * time.Hour))
	return ageDays > retention.MaxAgeDays
}

func totalSize(files []dayFile) int64 {
	var total int64
	for _, file := range files {
		total += file.size
	}
	return total
}

func removeAll(files []dayFile) error {
	var failures []error
	for _, file := range files {
		if err := os.Remove(file.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
