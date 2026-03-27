package search

import "codecom/internal/sessionindex"

// Result is the normalized output for a conversation search query.
type Result struct {
	SessionIDs         map[string]struct{}
	FolderPaths        map[string]struct{}
	OffsetsBySessionID map[string][]int64
}

// Backend is a query-only search handle built from a concrete index backend.
type Backend interface {
	Search(query string) (Result, error)
	Close() error
}

// BuildSQLiteIndex builds/rebuilds the v1 SQLite sidecar index and returns a ready backend.
func BuildSQLiteIndex(codexRoot string, sessions []sessionindex.SessionRecord) (Backend, error) {
	return buildSQLiteIndex(codexRoot, sessions)
}
