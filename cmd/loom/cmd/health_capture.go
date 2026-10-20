package cmd

// The capture line of `loom health`.
//
// The check is read only and cheap by design (architecture-notes.md
// decision 16). It lists a directory, stats files and parses one small
// settings file. It NEVER reads a day file and never opens episodes.db: a
// 50 MB store must not slow health down.

import (
	"errors"
	"fmt"
	"os"

	"github.com/orieken/loom/cmd/loom/internal/settings"
	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/jsonlstore"
	"github.com/orieken/loom/internal/capture/localfs"
)

const (
	// storeWarnPercent is the share of the store cap at which health warns.
	storeWarnPercent = 80
	percentBase      = 100
	// utcStampLayout shows the time of the last hook failure.
	utcStampLayout = dateLayout + " 15:04:05Z"
)

func (check *healthCheck) verifyCapture() {
	layout := capture.NewLayout(check.request.target)
	entries, err := settings.CountEntries(layout.SettingsLocalPath(), captureSelector(check.manifest.Capture))
	if err != nil {
		check.warn("capture: cannot read " + relativeSlash(layout.Root(), layout.SettingsLocalPath()) + ": " + err.Error())
		return
	}
	isLive := entries > 0
	check.reportCaptureState(entries)
	if !isLive && !captureLeftovers(layout) {
		return
	}
	check.verifyCaptureSalt(layout, isLive)
	check.verifyCaptureFailures(layout)
	check.verifyCaptureStore(layout)
}

func (check *healthCheck) reportCaptureState(entries int) {
	if entries == 0 {
		check.output.success("capture: off (enable with `loom install --capture`)")
		return
	}
	check.output.success(fmt.Sprintf("capture: live (%d hook entries registered)", entries))
}

// captureLeftovers tells whether data or a salt remains while the hooks are
// off, for example after a hand edit of the settings file.
func captureLeftovers(layout capture.Layout) bool {
	for _, path := range []string{layout.SessionsDir(), layout.SaltPath(), layout.FailuresPath()} {
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	return false
}

// verifyCaptureSalt reports a salt the hook cannot use. A hook with no
// usable salt writes nothing, so a live capture with a bad salt is a
// failure.
func (check *healthCheck) verifyCaptureSalt(layout capture.Layout, isLive bool) {
	_, err := localfs.NewSaltFile(layout.SaltPath()).Salt()
	message := saltProblem(err)
	if message == "" {
		return
	}
	if isLive {
		check.fail("capture: " + message)
		return
	}
	check.warn("capture: " + message)
}

func saltProblem(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, localfs.ErrSaltMissing):
		return "the salt file is missing, so the hooks record nothing. Run `loom install --capture` to create it"
	case errors.Is(err, localfs.ErrSaltMode):
		return "the salt file can be read by other users. Run `chmod 600` on it (the hooks record nothing until then)"
	default:
		return "the salt file is not 32 bytes, so the hooks record nothing. Restore it from a backup, or delete it and run `loom install --capture` (older hashes then stop matching)"
	}
}

func (check *healthCheck) verifyCaptureFailures(layout capture.Layout) {
	info, err := os.Stat(layout.FailuresPath())
	if err == nil && info.Size() > 0 {
		check.warn(fmt.Sprintf("capture: %s recorded (last at %s)", countLabel(int(info.Size()), "hook failure"), info.ModTime().UTC().Format(utcStampLayout)))
	}
	if jsonlstore.NewFailureLog(layout).IsStalled(localfs.SystemClock{}.Now()) {
		check.warn(fmt.Sprintf("capture: stall marker set; the hooks skip recording for up to %d s", int(capture.StallWindow.Seconds())))
	}
}

func (check *healthCheck) verifyCaptureStore(layout capture.Layout) {
	size := storeBytes(layout)
	capBytes, err := storeCap(layout)
	if err != nil {
		check.warn("capture: " + err.Error())
	}
	label := fmt.Sprintf("capture: store %.1f MB of %d MB", float64(size)/capture.BytesPerMegabyte, capBytes/capture.BytesPerMegabyte)
	if size*percentBase >= capBytes*storeWarnPercent {
		check.warn(label + fmt.Sprintf(" (at %d%% or more of the cap; the purge deletes the oldest day files)", storeWarnPercent))
		return
	}
	check.output.success(label)
}

// storeCap reads the cap from capture.yaml. A broken file gives the default
// cap and a message, because the hook fails closed on it.
func storeCap(layout capture.Layout) (int64, error) {
	config, err := localfs.NewConfigFile(layout.ConfigPath()).Load()
	if err != nil {
		return capture.DefaultConfig().Retention.MaxBytes, fmt.Errorf(".claude/capture.yaml is invalid, so the hooks record nothing: %w", err)
	}
	return config.Retention.MaxBytes, nil
}

// storeBytes adds up the sizes of the day files. It lists the directory and
// reads no file.
func storeBytes(layout capture.Layout) int64 {
	entries, err := os.ReadDir(layout.SessionsDir())
	if err != nil {
		return 0
	}
	var total int64
	for _, entry := range entries {
		if _, isDayFile := capture.DayFileDate(entry.Name()); !isDayFile || !entry.Type().IsRegular() {
			continue
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
	}
	return total
}
