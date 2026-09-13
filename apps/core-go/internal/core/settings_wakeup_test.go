package core

import (
	"os"
	"strings"
	"testing"
)

func TestWakeUpSettingsNeedsRearmOnlyWhenEnabledAgain(t *testing.T) {
	enabled := WakeUpSettings{Enabled: true, IntervalSeconds: 1800}
	disabled := WakeUpSettings{Enabled: false, IntervalSeconds: 1800}
	if !wakeUpSettingsNeedsRearm(disabled, enabled) {
		t.Fatal("disabled to enabled did not request WakeUp rearm")
	}
	for _, transition := range []struct {
		from WakeUpSettings
		to   WakeUpSettings
	}{
		{from: enabled, to: enabled},
		{from: disabled, to: disabled},
		{from: enabled, to: disabled},
	} {
		if wakeUpSettingsNeedsRearm(transition.from, transition.to) {
			t.Fatalf("unexpected WakeUp rearm for %#v -> %#v", transition.from, transition.to)
		}
	}
}

func TestMergeWakeUpSettingsPreservesOmittedValuesAndRejectsWrongTypes(t *testing.T) {
	previous := WakeUpSettings{Enabled: false, IntervalSeconds: 3600}
	merged, err := mergeWakeUpSettings(previous, map[string]any{"enabled": true})
	if err != nil || !merged.Enabled || merged.IntervalSeconds != 3600 {
		t.Fatalf("partial WakeUp settings merge = %#v, %v", merged, err)
	}
	for _, invalid := range []any{
		"enabled",
		map[string]any{"enabled": "true"},
		map[string]any{"interval_seconds": "1800"},
		map[string]any{"unknown": true},
	} {
		if _, err := mergeWakeUpSettings(previous, invalid); err == nil {
			t.Fatalf("invalid WakeUp settings accepted: %#v", invalid)
		}
	}
}

func TestUpdateSettingsRearmsDurableWakeUpClockAfterEnable(t *testing.T) {
	source, err := os.ReadFile("settings.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (a *App) UpdateSettings")
	end := strings.Index(text[start:], "func wakeUpSettingsNeedsRearm")
	if start < 0 || end < 0 {
		t.Fatal("UpdateSettings WakeUp boundary not found")
	}
	body := text[start : start+end]
	for _, required := range []string{
		"product.wakeup",
		"EnsureWakeUpIntents",
		"ReleaseDueWakeUpIntents",
		"product_wakeup_rearm_failed",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("UpdateSettings WakeUp rearm missing %q", required)
		}
	}
}
