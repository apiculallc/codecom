package search

import (
	"reflect"
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "tokens", input: "alpha beta", want: []string{"alpha", "beta"}},
		{name: "phrase and token", input: `"hello world" foo`, want: []string{"hello world", "foo"}},
		{name: "normalizes whitespace and case", input: `  "Hello   world"   FOO `, want: []string{"hello world", "foo"}},
		{name: "empty", input: "   ", wantErr: true},
		{name: "unterminated quote", input: `"alpha beta`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQuery(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQuery error: %v", err)
			}
			if !reflect.DeepEqual(got.Clauses, tt.want) {
				t.Fatalf("clauses mismatch\nwant=%v\n got=%v", tt.want, got.Clauses)
			}
		})
	}
}
