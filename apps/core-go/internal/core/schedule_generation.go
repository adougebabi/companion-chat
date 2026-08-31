package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (a *App) generateInitialSchedule(ctx context.Context, ownerID, fluctlightID, localDate, timezone string, identity, lifeProfile map[string]any) (map[string]any, error) {
	messages := []map[string]any{
		{"role": "system", "content": "Return one JSON object with items and reschedule_policy. items must be a non-empty array of objects covering the complete local day contiguously from 00:00 through the next 00:00 in the supplied timezone. Every item needs start_at, end_at, activity, scene, item_type, status, priority, flexibility, interruption_cost. Use RFC3339 timestamps with the supplied timezone. Do not return markdown or foundation fields."},
		{"role": "user", "content": jsonString(map[string]any{
			"fluctlight_id": fluctlightID,
			"local_date":    localDate,
			"timezone":      timezone,
			"identity":      identity,
			"life_profile":  lifeProfile,
		})},
	}
	// The existing provider contract assigns schedule planning to the
	// cognitive-assessment role; initialization is reserved for foundation
	// extraction and may use a different response schema.
	result, err := a.Provider.Structured(ctx, "cognitive_assessment", messages)
	if err != nil {
		return nil, fmt.Errorf("initial schedule provider request failed: %w", err)
	}
	payload, err := normalizeScheduleResponse(result, localDate, timezone)
	if err != nil {
		return nil, err
	}
	payload["evidence_refs"] = []any{"foundation:" + fluctlightID}
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
	type scheduleEntry struct {
		item  map[string]any
		start time.Time
		end   time.Time
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
	return map[string]any{"local_date": localDate, "timezone": timezone, "items": resultItems, "evidence_refs": []any{"foundation:" + "pending"}}, nil
}

func parseScheduleTimeInLocation(value string, day time.Time, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("timestamp is required")
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
