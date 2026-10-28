package daterange

import (
	"testing"
	"time"
)

func now() time.Time {
	return time.Date(2026, 10, 28, 15, 0, 0, 0, time.UTC)
}

func TestDefaultLast28DaysEndingYesterday(t *testing.T) {
	r, err := Resolve(now(), "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.StartStr() != "2026-09-30" || r.EndStr() != "2026-10-27" {
		t.Fatalf("got %s → %s", r.StartStr(), r.EndStr())
	}
	if r.Days() != 28 {
		t.Fatalf("days=%d", r.Days())
	}
}

func TestExplicitSinceUntil(t *testing.T) {
	r, err := Resolve(now(), "2026-01-01", "2026-01-31", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.StartStr() != "2026-01-01" || r.EndStr() != "2026-01-31" {
		t.Fatalf("got %s → %s", r.StartStr(), r.EndStr())
	}
}

func TestDaysFlag(t *testing.T) {
	r, err := Resolve(now(), "", "", 7, "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if r.StartStr() != "2026-10-21" || r.EndStr() != "2026-10-27" {
		t.Fatalf("got %s → %s", r.StartStr(), r.EndStr())
	}
}

func TestIncrementalOverlap(t *testing.T) {
	r, err := Resolve(now(), "", "", 0, "2026-10-20")
	if err != nil {
		t.Fatal(err)
	}
	if r.StartStr() != "2026-10-18" || r.EndStr() != "2026-10-27" {
		t.Fatalf("got %s → %s (want lastEnd-2d through yesterday)", r.StartStr(), r.EndStr())
	}
}

func TestUntilBeforeSince(t *testing.T) {
	_, err := Resolve(now(), "2026-10-10", "2026-10-01", 0, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBadDate(t *testing.T) {
	_, err := Resolve(now(), "10/01/2026", "", 0, "")
	if err == nil {
		t.Fatal("expected error")
	}
}
