package workflow

import (
	"testing"
	"time"
)

func TestNextLocalMidnightDelayUsesConfiguredTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 31, 23, 30, 0, 0, location)
	if got, want := nextLocalMidnightDelay(now, "Asia/Shanghai"), 30*time.Minute; got != want {
		t.Fatalf("delay = %s, want %s", got, want)
	}
}

func TestNextLocalMidnightDelayFallsBackForUnknownTimezone(t *testing.T) {
	if got, want := nextLocalMidnightDelay(time.Now(), "not/a/zone"), 24*time.Hour; got != want {
		t.Fatalf("delay = %s, want %s", got, want)
	}
}
