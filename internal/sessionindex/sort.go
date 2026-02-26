package sessionindex

import "sort"

func SortSessions(records []SessionRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		ci := records[i].EffectiveCWD()
		cj := records[j].EffectiveCWD()
		if ci != cj {
			return ci < cj
		}
		if !records[i].SortTime.Equal(records[j].SortTime) {
			return records[i].SortTime.After(records[j].SortTime)
		}
		return records[i].SessionFile < records[j].SessionFile
	})
}
