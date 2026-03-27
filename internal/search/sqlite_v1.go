package search

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codecom/internal/sessionindex"
	_ "modernc.org/sqlite"
)

const v1IndexRelativePath = ".codecom/search/index-v1.sqlite"

type sqliteBackend struct {
	db *sql.DB
}

type clauseMatch struct {
	cwd     string
	offsets map[int64]struct{}
}

func buildSQLiteIndex(_ string, sessions []sessionindex.SessionRecord) (Backend, error) {
	indexPath, err := defaultIndexPath()
	if err != nil {
		return nil, err
	}
	indexDir := filepath.Dir(indexPath)
	if err := ensurePrivateDir(indexDir); err != nil {
		return nil, err
	}
	if err := ensureRegularPath(indexPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", indexPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite index: %w", err)
	}

	backend := &sqliteBackend{db: db}
	if err := backend.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := backend.rebuild(sessions); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(indexPath, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("chmod sqlite index: %w", err)
	}
	return backend, nil
}

func defaultIndexPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, v1IndexRelativePath), nil
}

func ensurePrivateDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("index directory is symlink: %s", path)
		}
		if !info.IsDir() {
			return fmt.Errorf("index directory is not a directory: %s", path)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("chmod index directory: %w", err)
		}
		return nil
	case os.IsNotExist(err):
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create index directory: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("stat index directory: %w", err)
	}
}

func ensureRegularPath(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("index file is symlink: %s", path)
		}
		if info.IsDir() {
			return fmt.Errorf("index file is a directory: %s", path)
		}
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("stat index path: %w", err)
}

func (b *sqliteBackend) Close() error {
	if b == nil || b.db == nil {
		return nil
	}
	return b.db.Close()
}

func (b *sqliteBackend) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			cwd TEXT NOT NULL,
			session_file TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			session_id TEXT NOT NULL,
			cwd TEXT NOT NULL,
			offset INTEGER NOT NULL,
			content_lc TEXT NOT NULL,
			PRIMARY KEY(session_id, offset)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);`,
	}
	for _, stmt := range stmts {
		if _, err := b.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (b *sqliteBackend) rebuild(sessions []sessionindex.SessionRecord) error {
	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("begin rebuild tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM messages;`); err != nil {
		return fmt.Errorf("clear messages: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM sessions;`); err != nil {
		return fmt.Errorf("clear sessions: %w", err)
	}

	insSession, err := tx.Prepare(`INSERT INTO sessions(session_id, cwd, session_file) VALUES(?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("prepare session insert: %w", err)
	}
	defer insSession.Close()

	insMessage, err := tx.Prepare(`INSERT INTO messages(session_id, cwd, offset, content_lc) VALUES(?, ?, ?, ?);`)
	if err != nil {
		return fmt.Errorf("prepare message insert: %w", err)
	}
	defer insMessage.Close()

	for _, s := range sessions {
		sid := strings.TrimSpace(s.SessionID)
		if sid == "" {
			sid = strings.TrimSuffix(filepath.Base(s.SessionFile), filepath.Ext(s.SessionFile))
		}
		if sid == "" {
			return fmt.Errorf("session missing id and filename: %q", s.SessionFile)
		}
		cwd := strings.TrimSpace(s.EffectiveCWD())
		if _, err := insSession.Exec(sid, cwd, s.SessionFile); err != nil {
			return fmt.Errorf("insert session %q: %w", sid, err)
		}

		lines, err := parseConversationMessages(s.SessionFile)
		if err != nil {
			return fmt.Errorf("parse session %q: %w", s.SessionFile, err)
		}
		for _, line := range lines {
			if _, err := insMessage.Exec(sid, cwd, line.Offset, strings.ToLower(line.Text)); err != nil {
				return fmt.Errorf("insert message %q offset %d: %w", sid, line.Offset, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rebuild tx: %w", err)
	}
	return nil
}

func (b *sqliteBackend) Search(query string) (Result, error) {
	pq, err := parseQuery(query)
	if err != nil {
		return Result{}, err
	}

	perClause := make([]map[string]*clauseMatch, 0, len(pq.Clauses))

	for _, clause := range pq.Clauses {
		rows, err := b.db.Query(`SELECT session_id, cwd, offset FROM messages WHERE instr(content_lc, ?) > 0;`, clause)
		if err != nil {
			return Result{}, fmt.Errorf("query clause %q: %w", clause, err)
		}

		matches := make(map[string]*clauseMatch)
		for rows.Next() {
			var sid, cwd string
			var offset int64
			if err := rows.Scan(&sid, &cwd, &offset); err != nil {
				rows.Close()
				return Result{}, fmt.Errorf("scan query rows: %w", err)
			}
			m, ok := matches[sid]
			if !ok {
				m = &clauseMatch{cwd: cwd, offsets: make(map[int64]struct{})}
				matches[sid] = m
			}
			if m.cwd == "" {
				m.cwd = cwd
			}
			m.offsets[offset] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return Result{}, fmt.Errorf("iterate query rows: %w", err)
		}
		rows.Close()
		if len(matches) == 0 {
			return emptyResult(), nil
		}
		perClause = append(perClause, matches)
	}

	finalSessionIDs := intersectSessionIDs(perClause)
	if len(finalSessionIDs) == 0 {
		return emptyResult(), nil
	}

	res := emptyResult()
	for _, sid := range finalSessionIDs {
		res.SessionIDs[sid] = struct{}{}
		merged := make(map[int64]struct{})
		cwd := ""
		for _, clauseMatches := range perClause {
			entry := clauseMatches[sid]
			if entry == nil {
				continue
			}
			if cwd == "" {
				cwd = entry.cwd
			}
			for off := range entry.offsets {
				merged[off] = struct{}{}
			}
		}
		if cwd != "" {
			res.FolderPaths[cwd] = struct{}{}
		}
		res.OffsetsBySessionID[sid] = sortedOffsets(merged)
	}
	return res, nil
}

func emptyResult() Result {
	return Result{
		SessionIDs:         make(map[string]struct{}),
		FolderPaths:        make(map[string]struct{}),
		OffsetsBySessionID: make(map[string][]int64),
	}
}

func intersectSessionIDs(perClause []map[string]*clauseMatch) []string {
	if len(perClause) == 0 {
		return nil
	}
	ids := make([]string, 0, len(perClause[0]))
	for sid := range perClause[0] {
		ok := true
		for i := 1; i < len(perClause); i++ {
			if _, exists := perClause[i][sid]; !exists {
				ok = false
				break
			}
		}
		if ok {
			ids = append(ids, sid)
		}
	}
	sort.Strings(ids)
	return ids
}

func sortedOffsets(m map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(m))
	for off := range m {
		out = append(out, off)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
