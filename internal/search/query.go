package search

import (
	"fmt"
	"strings"
)

type parsedQuery struct {
	Clauses []string
}

func parseQuery(raw string) (parsedQuery, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedQuery{}, fmt.Errorf("empty query")
	}

	clauses := make([]string, 0, 8)
	var token strings.Builder
	inQuote := false

	flush := func() {
		if token.Len() == 0 {
			return
		}
		norm := normalizeSearchText(token.String())
		if norm != "" {
			clauses = append(clauses, strings.ToLower(norm))
		}
		token.Reset()
	}

	for _, r := range raw {
		switch {
		case r == '"':
			if inQuote {
				flush()
				inQuote = false
			} else {
				flush()
				inQuote = true
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if inQuote {
				token.WriteRune(r)
			} else {
				flush()
			}
		default:
			token.WriteRune(r)
		}
	}

	if inQuote {
		return parsedQuery{}, fmt.Errorf("unterminated quoted phrase")
	}
	flush()
	if len(clauses) == 0 {
		return parsedQuery{}, fmt.Errorf("empty query")
	}
	return parsedQuery{Clauses: clauses}, nil
}
