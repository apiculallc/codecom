package sessionindex

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const sqliteStateFileName = "state_5.sqlite"

type sqliteThreadRow struct {
	ID          string
	CWD         string
	RolloutPath string
}

func attachSQLiteThreads(codexRoot string, records []SessionRecord) []Warning {
	dbPath := filepath.Join(codexRoot, sqliteStateFileName)
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}

	threads, err := readSQLiteThreads(dbPath)
	if err != nil {
		return []Warning{{
			SessionFile: dbPath,
			Line:        0,
			Message:     fmt.Sprintf("sqlite read skipped: %v", err),
		}}
	}

	for i := range records {
		row, ok := threads[records[i].SessionID]
		if !ok {
			continue
		}
		records[i].SQLiteThreadID = row.ID
		records[i].SQLiteThreadCWD = row.CWD
		records[i].SQLiteRollout = row.RolloutPath
		records[i].SQLiteMatched = true
	}

	return nil
}

func readSQLiteThreads(dbPath string) (map[string]sqliteThreadRow, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return readSQLiteThreadsDB(db)
}

// EnsureSQLiteThreadsReady verifies state_5.sqlite exists and threads is queryable.
func EnsureSQLiteThreadsReady(codexRoot string) error {
	dbPath := filepath.Join(codexRoot, sqliteStateFileName)
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return ensureSQLiteThreadsReadyDB(db)
}

func readSQLiteThreadsDB(db *sql.DB) (map[string]sqliteThreadRow, error) {
	rows := make(map[string]sqliteThreadRow)
	qrows, err := db.Query("SELECT id, cwd, COALESCE(rollout_path, '') FROM threads;")
	if err != nil {
		return nil, err
	}
	defer qrows.Close()

	for qrows.Next() {
		var row sqliteThreadRow
		if err := qrows.Scan(&row.ID, &row.CWD, &row.RolloutPath); err != nil {
			continue
		}
		if row.ID == "" {
			continue
		}
		rows[row.ID] = row
	}
	if err := qrows.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func ensureSQLiteThreadsReadyDB(db *sql.DB) error {
	var n int
	err := db.QueryRow("SELECT 1 FROM threads LIMIT 1;").Scan(&n)
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// RewriteSQLiteThreadPaths updates cwd and rollout_path for selected session ids.
func RewriteSQLiteThreadPaths(codexRoot, oldPrefix, newPrefix string, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	dbPath := filepath.Join(codexRoot, sqliteStateFileName)
	if _, err := os.Stat(dbPath); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return rewriteSQLiteThreadPathsDB(db, oldPrefix, newPrefix, sessionIDs)
}

func rewriteSQLiteThreadPathsDB(db *sql.DB, oldPrefix, newPrefix string, sessionIDs []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
UPDATE threads
SET cwd = REPLACE(cwd, ?, ?),
    rollout_path = CASE
      WHEN rollout_path IS NULL THEN NULL
      ELSE REPLACE(rollout_path, ?, ?)
    END
WHERE id = ?;
`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, id := range sessionIDs {
		if id == "" {
			continue
		}
		if _, err := stmt.Exec(oldPrefix, newPrefix, oldPrefix, newPrefix, id); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update sqlite thread %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
