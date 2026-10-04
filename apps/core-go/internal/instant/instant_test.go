package instant

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestMarshalCanonicalInstantsPreservesSourcesDatesAndLargeNumbers(t *testing.T) {
	at, _ := time.Parse(time.RFC3339Nano, "2026-10-04T08:10:11.123456789+08:00")
	value := map[string]any{"created_at": at, "nested": map[string]any{"finishedAt": "2026-10-04T00:10:11.123456789Z", "birthday": "2000-05-06", "text": "2026-10-04T08:10:11+08:00"}, "sequence": json.Number("9007199254740993"), "snapshot": map[string]any{"created_at": "2026-10-04T00:10:11.123456789Z"}}
	encoded, err := Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	wire := string(encoded)
	if !strings.Contains(wire, `"created_at":"2026-10-04T00:10:11.123+00:00"`) || !strings.Contains(wire, `"finishedAt":"2026-10-04T00:10:11.123+00:00"`) || !strings.Contains(wire, `9007199254740993`) || !strings.Contains(wire, `"text":"2026-10-04T08:10:11+08:00"`) || !strings.Contains(wire, `"birthday":"2000-05-06"`) || !strings.Contains(wire, `"snapshot":{"created_at":"2026-10-04T00:10:11.123456789Z"}`) {
		t.Fatal(wire)
	}
	if at.Nanosecond() != 123456789 {
		t.Fatal("formatter changed stored precision")
	}
}

func TestStructuredOperationalTimeUsesNumericMillisecondsWithoutRewritingText(t *testing.T) {
	at := time.Date(2026, 10, 4, 8, 10, 11, 123456789, time.FixedZone("offset", 8*3600))
	formatted := LogAttribute(nil, slog.Time(slog.TimeKey, at))
	if formatted.Value.String() != "2026-10-04T00:10:11.123+00:00" {
		t.Fatal(formatted)
	}
	source := slog.String("source_text", "2026-10-04T08:10:11Z")
	if got := LogAttribute(nil, source); got.Value.String() != source.Value.String() {
		t.Fatal(got)
	}
}
