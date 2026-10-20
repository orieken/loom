package jsonlstore

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var trimRetention = capture.DefaultConfig().Retention

// numberedLog writes count lines one second apart, the newest one second
// before logNow. It returns the lines, oldest first.
func numberedLog(t *testing.T, layout capture.Layout, count int) []string {
	t.Helper()
	lines := make([]string, count)
	for i := range lines {
		lines[i] = lineAt(logNow.Add(-time.Duration(count-i)*time.Second), capture.CauseWrite)
	}
	writeFile(t, layout.FailureLogPath(), strings.Join(lines, ""))
	return lines
}

func trim(t *testing.T, layout capture.Layout) {
	t.Helper()
	if err := trimFailureLog(layout, logNow, trimRetention); err != nil {
		t.Fatal(err)
	}
}

func TestTrimKeepsTheNewestLinesWhenTheLogIsOverTheCap(t *testing.T) {
	layout := newLayout(t)
	lines := numberedLog(t, layout, capture.MaxFailureLogBytes/len(lineAt(logNow, capture.CauseWrite))+500)

	trim(t, layout)

	got := readFile(t, layout.FailureLogPath())
	if len(got) > capture.MaxFailureLogBytes || len(got) < capture.MaxFailureLogBytes-len(lines[0]) {
		t.Fatalf("size after trim = %d, want just under %d", len(got), capture.MaxFailureLogBytes)
	}
	if !strings.HasSuffix(strings.Join(lines, ""), got) || !strings.HasSuffix(got, lines[len(lines)-1]) {
		t.Fatal("the trim kept lines other than the newest")
	}
	if strings.HasPrefix(got, lines[0]) {
		t.Fatal("the oldest line survived a trim of an over-cap log")
	}
}

func TestTrimDropsLinesOlderThanTheRetentionAge(t *testing.T) {
	layout := newLayout(t)
	tooOld := lineAt(logNow.AddDate(0, 0, -91), capture.CauseSalt)
	atEdge := lineAt(logNow.AddDate(0, 0, -90), capture.CauseDecode)
	recent := lineAt(logNow.Add(-time.Hour), capture.CausePanic)
	writeFile(t, layout.FailureLogPath(), tooOld+atEdge+recent)

	trim(t, layout)

	equal(t, "log after trim", readFile(t, layout.FailureLogPath()), atEdge+recent)
}

func TestTrimLeavesAnInBoundsLogByteForByteAsItWas(t *testing.T) {
	layout := newLayout(t)
	numberedLog(t, layout, 10)
	before, _ := os.Stat(layout.FailureLogPath())
	stamp := before.ModTime().Add(-time.Hour)
	_ = os.Chtimes(layout.FailureLogPath(), stamp, stamp)

	trim(t, layout)

	after, _ := os.Stat(layout.FailureLogPath())
	if !after.ModTime().Equal(stamp) {
		t.Fatal("the trim rewrote a log that was within its bounds")
	}
}

func TestTrimDropsTheBytesOfTheOldFormatFromTheNewLog(t *testing.T) {
	layout := newLayout(t)
	keep := lineAt(logNow, capture.CauseFlag)
	writeFile(t, layout.FailureLogPath(), "dda"+"\n"+keep)

	trim(t, layout)

	equal(t, "log after trim", readFile(t, layout.FailureLogPath()), keep)
}

func TestTrimKeepsTheLogPrivate(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.FailureLogPath(), lineAt(logNow.AddDate(0, 0, -200), capture.CauseSalt)+lineAt(logNow, capture.CauseSalt))

	trim(t, layout)

	assertMode(t, layout.FailureLogPath(), 0o600)
	if exists(layout.FailureLogPath() + capture.FailureLogTempSuffix) {
		t.Fatal("the trim left its temporary file behind")
	}
}

func TestTrimRemovesTheLegacyCounterOfAnEarlierInstall(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.LegacyFailuresPath(), "ddt")

	trim(t, layout)

	if exists(layout.LegacyFailuresPath()) {
		t.Fatal("the one-byte-per-failure file outlived the trim")
	}
}

func TestTrimOfAMissingLogIsNotAnError(t *testing.T) {
	layout := newLayout(t)

	trim(t, layout)

	if exists(layout.FailureLogPath()) {
		t.Fatal("the trim created a log")
	}
}

// The append path never trims: only the hourly purge does.
func TestRecordDoesNotTrimAnOverCapLog(t *testing.T) {
	layout := newLayout(t)
	lines := numberedLog(t, layout, capture.MaxFailureLogBytes/len(lineAt(logNow, capture.CauseWrite))+500)
	before := len(strings.Join(lines, ""))

	logAt(layout, logNow).Record(capture.CauseDecode)

	after := len(readFile(t, layout.FailureLogPath()))
	equal(t, "size after Record", after, before+len(lineAt(logNow, capture.CauseDecode)))
}

func TestPurgeTrimsTheFailureLogOncePerInterval(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.FailureLogPath(), lineAt(logNow.AddDate(0, 0, -200), capture.CauseSalt))

	if err := purge(t, layout, logNow, trimRetention); err != nil {
		t.Fatal(err)
	}
	equal(t, "log after the first purge", readFile(t, layout.FailureLogPath()), "")
	writeFile(t, layout.FailureLogPath(), lineAt(logNow.AddDate(0, 0, -200), capture.CauseSalt))
	if err := purge(t, layout, logNow.Add(time.Minute), trimRetention); err != nil {
		t.Fatal(err)
	}

	if readFile(t, layout.FailureLogPath()) == "" {
		t.Fatal("a purge inside the interval trimmed the log")
	}
}

func TestPurgeTrimsTheFailureLogEvenWhenTheDayFilePurgeFails(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.SessionsDir(), "not a directory")
	writeFile(t, layout.FailureLogPath(), lineAt(logNow.AddDate(0, 0, -200), capture.CauseSalt))

	err := purge(t, layout, logNow, trimRetention)

	if err == nil {
		t.Fatal("want the error of the day-file purge")
	}
	equal(t, "log after the purge", readFile(t, layout.FailureLogPath()), "")
}

func TestTrimReportsAnErrorWhenItCannotWriteAndKeepsTheLog(t *testing.T) {
	layout := newLayout(t)
	original := lineAt(logNow.AddDate(0, 0, -200), capture.CauseSalt) + lineAt(logNow, capture.CauseSalt)
	writeFile(t, layout.FailureLogPath(), original)
	if err := os.Mkdir(layout.FailureLogPath()+capture.FailureLogTempSuffix, 0o700); err != nil {
		t.Fatal(err)
	}

	err := trimFailureLog(layout, logNow, trimRetention)

	if err == nil {
		t.Fatal("want an error: the temporary file cannot be written")
	}
	equal(t, "log after a failed trim", readFile(t, layout.FailureLogPath()), original)
}
