package sessionindex

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Scan(codexRoot string) (ScanResult, error) {
	pattern := filepath.Join(codexRoot, "sessions", "*", "*", "*", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return ScanResult{}, err
	}
	sort.Strings(files)

	res := ScanResult{
		Sessions: make([]SessionRecord, 0, len(files)),
		Warnings: make([]Warning, 0),
	}

	for _, file := range files {
		rec, warnings, err := parseSessionFile(file)
		if err != nil {
			return ScanResult{}, err
		}
		rec.SortTime = parseSessionDay(file)
		res.Sessions = append(res.Sessions, rec)
		res.Warnings = append(res.Warnings, warnings...)
	}

	ApplyOrphanStatus(res.Sessions, 0)
	SortSessions(res.Sessions)
	return res, nil
}

func parseSessionDay(path string) time.Time {
	dir := filepath.Dir(path)
	parts := strings.Split(filepath.ToSlash(dir), "/")
	if len(parts) < 3 {
		return time.Time{}
	}
	n := len(parts)
	y, errY := strconv.Atoi(parts[n-3])
	m, errM := strconv.Atoi(parts[n-2])
	d, errD := strconv.Atoi(parts[n-1])
	if errY != nil || errM != nil || errD != nil {
		return time.Time{}
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}
