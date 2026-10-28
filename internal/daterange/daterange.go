package daterange

import (
	"fmt"
	"time"
)

const DefaultDays = 28
const OverlapDays = 2

// Range is an inclusive [Start, End] calendar-date window in UTC.
type Range struct {
	Start time.Time
	End   time.Time
}

func (r Range) StartStr() string { return r.Start.Format(time.DateOnly) }
func (r Range) EndStr() string   { return r.End.Format(time.DateOnly) }

func (r Range) Days() int {
	return int(r.End.Sub(r.Start).Hours()/24) + 1
}

func (r Range) Validate() error {
	if r.Start.IsZero() || r.End.IsZero() {
		return fmt.Errorf("date range is incomplete")
	}
	if r.End.Before(r.Start) {
		return fmt.Errorf("until %s is before since %s", r.EndStr(), r.StartStr())
	}
	return nil
}

// Resolve picks the fetch window.
//
// Precedence:
//  1. explicit since/until (YYYY-MM-DD)
//  2. last --days ending at until (or yesterday)
//  3. incremental: lastEnd minus OverlapDays through yesterday, if lastEnd is set
//  4. default: last DefaultDays ending yesterday
//
// YouTube Analytics for "today" is incomplete, so the default end is yesterday.
func Resolve(now time.Time, since, until string, days int, lastEnd string) (Range, error) {
	now = now.UTC()
	yesterday := truncate(now.Add(-24 * time.Hour))

	end := yesterday
	if until != "" {
		t, err := parse(until)
		if err != nil {
			return Range{}, fmt.Errorf("until: %w", err)
		}
		end = t
	}

	if since != "" {
		start, err := parse(since)
		if err != nil {
			return Range{}, fmt.Errorf("since: %w", err)
		}
		r := Range{Start: start, End: end}
		return r, r.Validate()
	}

	if days > 0 {
		start := end.AddDate(0, 0, -(days - 1))
		r := Range{Start: start, End: end}
		return r, r.Validate()
	}

	if lastEnd != "" {
		le, err := parse(lastEnd)
		if err != nil {
			return Range{}, fmt.Errorf("last end date: %w", err)
		}
		start := le.AddDate(0, 0, -OverlapDays)
		if start.After(end) {
			start = end.AddDate(0, 0, -(DefaultDays - 1))
		}
		r := Range{Start: start, End: end}
		return r, r.Validate()
	}

	start := end.AddDate(0, 0, -(DefaultDays - 1))
	r := Range{Start: start, End: end}
	return r, r.Validate()
}

func parse(s string) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("want YYYY-MM-DD, got %q", s)
	}
	return t, nil
}

func truncate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
