package memory

// Removal of the session table by `loom uninstall`.
//
// The human accepted this DROP under guardrail #2 (2026-10-18), with these
// limits: only `loom uninstall` calls it; it is not in any code that runs on
// Open; dry-run lists and drops nothing; it touches no other table; it does
// nothing when the file or the table is absent.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
)

// uninstallDropTimeout bounds one plan or one drop.
const uninstallDropTimeout = 30 * time.Second

// PlanSessionTableDrop lists the session tables that DropSessionTables would
// drop. It changes nothing and does not create the file. Use it for a
// dry run.
func PlanSessionTableDrop(path string) ([]string, error) {
	return withExistingStore(path, func(ctx context.Context, db *sql.DB) ([]string, error) {
		return presentSessionTables(ctx, db)
	})
}

// DropSessionTables drops the session table and returns the names it
// dropped. It touches no other table. A missing file or table is not an
// error and drops nothing.
func DropSessionTables(path string) ([]string, error) {
	return withExistingStore(path, func(ctx context.Context, db *sql.DB) ([]string, error) {
		present, err := presentSessionTables(ctx, db)
		if err != nil || len(present) == 0 {
			return nil, err
		}
		// The name is the package constant, never input.
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS `+sessionTableName); err != nil {
			return nil, fmt.Errorf("drop %s: %w", sessionTableName, err)
		}
		return present, nil
	})
}

// withExistingStore runs work against a store that is already on disk. It
// never creates the file.
func withExistingStore(path string, work func(context.Context, *sql.DB) ([]string, error)) ([]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("look for %s: %w", path, err)
	}
	db, err := openSessionDB(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), uninstallDropTimeout)
	defer cancel()
	return work(ctx, db)
}

func presentSessionTables(ctx context.Context, db *sql.DB) ([]string, error) {
	hasTable, err := hasSessionTable(ctx, db)
	if err != nil || !hasTable {
		return nil, err
	}
	return []string{sessionTableName}, nil
}
