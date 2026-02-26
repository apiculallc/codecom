package sessionindex

import (
	"testing"
	"time"
)

func TestSortSessionsByCWDThenNewest(t *testing.T) {
	records := []SessionRecord{
		{SessionFile: "2", SessionMetaCWD: "/b", SortTime: time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)},
		{SessionFile: "1", SessionMetaCWD: "/a", SortTime: time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)},
		{SessionFile: "3", SessionMetaCWD: "/a", SortTime: time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC)},
	}
	SortSessions(records)

	if records[0].SessionFile != "3" || records[1].SessionFile != "1" || records[2].SessionFile != "2" {
		t.Fatalf("unexpected order: %#v", []string{records[0].SessionFile, records[1].SessionFile, records[2].SessionFile})
	}
}
