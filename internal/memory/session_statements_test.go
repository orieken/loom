package memory

// Internal test: it swaps the SQL driver of the session handles for a
// wrapper that records every statement, to prove the report runs a constant
// number of statements and no write statement.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"modernc.org/sqlite"
)

const countingDriverName = "sqlite-counting"

type statementLog struct {
	mutex      sync.Mutex
	statements []string
}

func (l *statementLog) add(statement string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.statements = append(l.statements, statement)
}

func (l *statementLog) snapshot() []string {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return append([]string(nil), l.statements...)
}

var activeLog = &statementLog{}

type countingDriver struct{}

func (countingDriver) Open(name string) (driver.Conn, error) {
	inner, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: inner}, nil
}

type countingConn struct{ driver.Conn }

func (c *countingConn) Prepare(query string) (driver.Stmt, error) {
	activeLog.add(query)
	return c.Conn.Prepare(query)
}

func (c *countingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	activeLog.add(query)
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
}

func (c *countingConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
}

func init() { sql.Register(countingDriverName, countingDriver{}) }

type stubSource struct {
	events []capture.SessionEvent
}

func (s stubSource) Stream(_ context.Context, visit func(capture.SessionEvent) error) (SessionReadStats, error) {
	for _, event := range s.events {
		if err := visit(event); err != nil {
			return SessionReadStats{}, err
		}
	}
	return SessionReadStats{}, nil
}
func (stubSource) HasDayFiles() (bool, error) { return true, nil }
func (stubSource) NewestEventAt(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func editEvents(count int) []capture.SessionEvent {
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	var events []capture.SessionEvent
	for i := 0; i < count; i++ {
		path, ref := fmt.Sprintf("src/f%d.go", i), fmt.Sprintf("ref%d", i)
		at := base.Add(time.Duration(i) * time.Hour)
		display := path
		events = append(events,
			capture.NewSessionEvent("w", at, capture.EventToolResult).WithTool(capture.ToolEdit).WithPath(ref, &display).
				WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 2, PreHash: "a", PostHash: "b"}),
			capture.NewSessionEvent("o", at.Add(time.Second), capture.EventHunkObserved).WithPath(ref, nil).
				WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 2, PostHash: "c"}))
	}
	return events
}

// reportStatements ingests rowCount corrections, then records the statements
// of one report.
func reportStatements(t *testing.T, rowCount int, filter SessionFilter) []string {
	t.Helper()
	store, err := Open(DefaultPath(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	source := stubSource{events: editEvents(rowCount)}
	if _, err := store.IngestSessions(context.Background(), source, capture.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	sessionDriverName = countingDriverName
	defer func() { sessionDriverName = defaultSessionDriver }()
	activeLog.statements = nil
	if _, err := store.SessionReport(context.Background(), source, capture.DefaultConfig(), filter); err != nil {
		t.Fatal(err)
	}
	return activeLog.snapshot()
}

func TestReportRunsAConstantNumberOfStatements(t *testing.T) {
	for name, filter := range map[string]SessionFilter{
		"grouped": {}, "glob": {Glob: "src/**"}, "all filters": {Glob: "**", Tool: "Edit", Since: time.Unix(0, 0)},
	} {
		small := reportStatements(t, 10, filter)
		large := reportStatements(t, 10000, filter)
		if len(small) != len(large) {
			t.Errorf("%s: %d statements for 10 rows, %d for 10,000 rows", name, len(small), len(large))
		}
		if len(small) != 2 {
			t.Errorf("%s: %d statements, want an existence check and one select: %v", name, len(small), small)
		}
	}
}

func TestReportRunsNoWriteStatement(t *testing.T) {
	writes := []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "REPLACE", "PRAGMA", "VACUUM"}
	for _, filter := range []SessionFilter{{}, {Glob: "**"}} {
		for _, statement := range reportStatements(t, 10, filter) {
			first := strings.ToUpper(strings.Fields(statement)[0])
			for _, write := range writes {
				if first == write {
					t.Errorf("the report ran a write statement: %s", statement)
				}
			}
		}
	}
}

func TestTheGlobReportUsesOneSelectNotOnePerRow(t *testing.T) {
	statements := reportStatements(t, 50, SessionFilter{Glob: "**"})

	selects := 0
	for _, statement := range statements {
		if strings.Contains(statement, "FROM session_corrections") {
			selects++
		}
	}
	if selects != 1 {
		t.Errorf("%d selects on session_corrections, want 1: %v", selects, statements)
	}
	if !strings.Contains(statements[len(statements)-1], "ORDER BY at, seq") {
		t.Errorf("the glob select must read in the total order at, seq: %v", statements)
	}
}

func TestSessionDSNSetsTheBusyTimeout(t *testing.T) {
	dsn, err := sessionDSN("/tmp/x/episodes.db")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(dsn, "busy_timeout") || !strings.Contains(dsn, "5000") {
		t.Errorf("dsn %q lacks busy_timeout(5000)", dsn)
	}
}

func TestSessionTimeIsFixedWidthUTC(t *testing.T) {
	zone := time.FixedZone("test", 2*3600)

	got := formatSessionTime(time.Date(2026, 10, 18, 14, 0, 0, 0, zone))

	if got != "2026-10-18T12:00:00.000Z" {
		t.Errorf("formatSessionTime = %q", got)
	}
}
