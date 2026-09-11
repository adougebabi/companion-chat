package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type scheduleEntry struct {
	item  map[string]any
	start time.Time
	end   time.Time
}

func (a *App) generateInitialSchedule(ctx context.Context, ownerID, fluctlightID, localDate, timezone, expectedLifeContextRevision string, identity, lifeProfile map[string]any) (map[string]any, error) {
	messages := []map[string]any{
		{"role": "system", "content": "Return one compact object with items and reschedule_policy. items must contain 8-16 objects covering the complete local day contiguously from 00:00 through the next 00:00 in the supplied timezone. Every item needs start_at, end_at, activity, scene, location, item_type, status, priority, flexibility, interruption_cost. Keep activity, scene, and location each under 80 Chinese characters; use one concrete activity and one concrete scene per item, never combine alternatives with '/', '／', '、', or '或'. Merge adjacent periods with the same activity and scene instead of producing many small segments. priority, flexibility, and interruption_cost are normalized numbers from 0 to 1 (never a 1-10 score). Use RFC3339 timestamps with the supplied timezone. Do not return markdown or foundation fields."},
		{"role": "user", "content": jsonString(map[string]any{
			"local_date":   localDate,
			"timezone":     timezone,
			"identity":     identity,
			"life_profile": lifeProfile,
		})},
	}
	// Schedule semantics are owned by the cognitive-assessment role. Reflection
	// consumes an evidence window after the plan is accepted; using it here
	// returns a reflection proposal shape instead of the required {items,...}
	// schedule and leaves the lifecycle intent pending forever.
	result, err := a.Provider.StructuredWithSchema(WithProviderScenario(ctx, "schedule_generation"), "cognitive_assessment", messages, "schedule_response", scheduleResponseSchema(), false)
	if err != nil {
		return nil, fmt.Errorf("initial schedule provider request failed: %w", err)
	}
	payload, err := normalizeScheduleResponse(result, localDate, timezone)
	if err != nil {
		// A local model can stop midway through a long JSON schedule even when
		// the HTTP request itself succeeds. Retry the same factual context with
		// an explicit compact-output reminder; never salvage a partial array.
		retryMessages := append(append([]map[string]any{}, messages...), map[string]any{"role": "user", "content": "上一个日程 JSON 不完整。请重新输出完整且紧凑的 8-16 个时段，必须覆盖从 00:00 到次日 00:00，不能截断，也不要附加解释。"})
		if retryResult, retryErr := a.Provider.StructuredWithSchema(ctx, "cognitive_assessment", retryMessages, "schedule_response", scheduleResponseSchema(), false); retryErr == nil {
			payload, err = normalizeScheduleResponse(retryResult, localDate, timezone)
		}
	}
	if err != nil {
		return nil, err
	}
	payload["evidence_refs"] = []any{"foundation:" + fluctlightID}
	payload["expected_revision"] = 0
	payload["expected_life_context_revision"] = expectedLifeContextRevision
	payload["idempotency_key"] = "schedule-current-day:" + stableDigest(fluctlightID+"\x1f"+localDate+"\x1f"+timezone+"\x1f"+expectedLifeContextRevision)
	accepted, err := a.AcceptSchedule(ctx, ownerID, fluctlightID, payload)
	if err != nil {
		return nil, fmt.Errorf("initial schedule persistence failed: %w", err)
	}
	accepted["timezone"] = timezone
	return accepted, nil
}

func normalizeScheduleResponse(result map[string]any, localDate, timezone string) (map[string]any, error) {
	items := arrayValue(result["items"])
	if len(items) == 0 {
		if nested := mapValue(result["schedule"]); len(nested) > 0 {
			items = arrayValue(nested["items"])
		}
	}
	if len(items) == 0 {
		return nil, errors.New("initial schedule response has no items")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule_timezone_invalid: %w", err)
	}
	day, err := time.ParseInLocation("2006-01-02", localDate, location)
	if err != nil {
		return nil, fmt.Errorf("schedule_local_date_invalid: %w", err)
	}
	entries := make([]scheduleEntry, 0, len(items))
	for _, raw := range items {
		item := mapValue(raw)
		if len(item) == 0 {
			return nil, errors.New("initial schedule items must be objects")
		}
		start, err := parseScheduleTimeInLocation(stringValue(firstValue(item["start_at"], item["startAt"])), day, location)
		if err != nil {
			return nil, fmt.Errorf("initial schedule start_at invalid: %w", err)
		}
		end, err := parseScheduleTimeInLocation(stringValue(firstValue(item["end_at"], item["endAt"])), day, location)
		if err != nil || !end.After(start) {
			return nil, errors.New("initial schedule end_at invalid")
		}
		activity := stringValue(firstValue(item["activity"], item["task"]))
		scene := stringValue(firstValue(item["scene"], item["context"]))
		if activity == "" || scene == "" {
			return nil, errors.New("initial schedule activity and scene are required")
		}
		item["start_at"] = start.Format(time.RFC3339)
		item["end_at"] = end.Format(time.RFC3339)
		item["activity"] = activity
		item["scene"] = scene
		if location := strings.TrimSpace(stringValue(item["location"])); location != "" {
			item["location"] = location
		}
		entries = append(entries, scheduleEntry{item: item, start: start, end: end})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].start.Before(entries[j].start) })
	dayStart := day.Truncate(24 * time.Hour)
	// time.Truncate is based on UTC, so construct local midnight explicitly.
	dayStart = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	nextDay := dayStart.AddDate(0, 0, 1)
	if !entries[0].start.Equal(dayStart) || !entries[len(entries)-1].end.Equal(nextDay) {
		return nil, errors.New("initial schedule must cover the complete local day")
	}
	resultItems := make([]any, 0, len(entries))
	for index, entry := range entries {
		if index > 0 && !entry.start.Equal(entries[index-1].end) {
			return nil, errors.New("initial schedule items must be contiguous")
		}
		resultItems = append(resultItems, entry.item)
	}
	return map[string]any{"local_date": localDate, "timezone": timezone, "items": resultItems, "reschedule_policy": result["reschedule_policy"], "evidence_refs": []any{"foundation:" + "pending"}}, nil
}

func parseScheduleTimeInLocation(value string, day time.Time, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("timestamp is required")
	}
	// Models commonly express the end of a local day as `24:00`, which is
	// valid ISO-8601 notation but rejected by Go's time parser. Normalize only
	// an exact midnight marker to the following calendar day; other malformed
	// times remain validation errors.
	for _, marker := range []string{"T24:00:00", " 24:00:00", "T24:00", " 24:00"} {
		if index := strings.Index(value, marker); index >= 0 {
			normalized := value[:index] + strings.Replace(marker, "24:", "00:", 1) + value[index+len(marker):]
			var parsed time.Time
			var err error
			if strings.Contains(normalized, "T") {
				parsed, err = time.Parse(time.RFC3339, normalized)
			} else {
				parsed, err = time.ParseInLocation("2006-01-02 15:04", normalized, location)
			}
			if err == nil {
				return parsed.AddDate(0, 0, 1), nil
			}
		}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			if layout == "15:04" {
				return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, location), nil
			}
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}

func firstValue(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
