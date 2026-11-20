package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type scheduleEntry struct {
	index int // Original one-based model item number, retained after sorting.
	item  map[string]any
	start time.Time
	end   time.Time
}

func (a *App) initialSchedulePayload(ctx context.Context, fluctlightID, localDate, timezone string, identity, lifeProfile map[string]any) (map[string]any, error) {
	// Schedule semantics are owned by the cognitive-assessment role. Reflection
	// consumes an evidence window after the plan is accepted; using it here
	// returns a reflection proposal shape instead of the required {items,...}
	// schedule and leaves the lifecycle intent pending forever.
	var activeProfileID string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeProfileID); err != nil {
		return nil, err
	}
	habits, _, err := readProfileHabits(ctx, a.DB.Pool(), fluctlightID, activeProfileID)
	if err != nil {
		return nil, err
	}
	stableLife := map[string]any{}
	for _, key := range []string{"social_background", "preferences", "recurring_commitments", "character_constraints"} {
		if value, exists := lifeProfile[key]; exists {
			stableLife[key] = value
		}
	}
	stableLife["life_habits"] = habits
	goals, intentions, err := a.agencyProfile(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	inner, err := a.readInnerState(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	appearance, _, _, err := a.readEffectiveLifeSnapshot(ctx, fluctlightID, a.now().UTC())
	if err != nil {
		return nil, err
	}
	_, currentLife, err := a.readLifeContextSnapshotAt(ctx, fluctlightID, a.now().UTC())
	if err != nil {
		return nil, err
	}
	outcomes, err := a.readRecentActionOutcomes(ctx, fluctlightID, 6)
	if err != nil {
		return nil, err
	}
	places, err := a.readScheduleWorldPlaces(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	scheduleIdentity := scheduleGenerationIdentity(identity)
	taskInput := ScheduleGenerationTaskInput{LocalDate: localDate, Timezone: timezone, Identity: scheduleIdentity, LifeProfile: stableLife, WorldPlaces: places,
		CurrentState: map[string]any{"inner_state": inner, "appearance": appearance}, CurrentLife: currentLife,
		Goals: filterActiveProfileRows(goals, activeProfileID), Intentions: filterActiveProfileRows(intentions, activeProfileID), RecentOutcomes: outcomes}
	result, err := a.RunScheduleGenerationTask(ctx, taskInput)
	if err != nil {
		return nil, fmt.Errorf("initial schedule provider request failed: %w", err)
	}
	payload, err := normalizeInitialScheduleResponse(result, localDate, timezone, places)
	if err != nil {
		// A local model can stop midway through a long JSON schedule even when
		// the HTTP request itself succeeds. Retry the same factual context with
		// an explicit compact-output reminder; never salvage a partial array.
		taskInput.CompactOutputReminder = true
		if retryResult, retryErr := a.RunScheduleGenerationTask(ctx, taskInput); retryErr == nil {
			payload, err = normalizeInitialScheduleResponse(retryResult, localDate, timezone, places)
		} else {
			return nil, fmt.Errorf("initial schedule repair request failed: %w", retryErr)
		}
	}
	if err != nil {
		// Both model outputs failed validation. Accept an explicitly unplanned
		// default through the same CAS/world checks; never fabricate a routine.
		payload, err = defaultInitialSchedule(localDate, timezone, err)
		if err != nil {
			return nil, err
		}
	}
	places.bind(payload)
	return payload, nil
}

func scheduleGenerationIdentity(identity map[string]any) map[string]any {
	return scheduleIdentityPromptView(identity)
}

// Keep the automatic current-day lifecycle on its existing acceptance path.
func (a *App) generateInitialSchedule(ctx context.Context, ownerID, fluctlightID, localDate, timezone, expectedLifeContextRevision string, identity, lifeProfile map[string]any) (map[string]any, error) {
	payload, err := a.initialSchedulePayload(ctx, fluctlightID, localDate, timezone, identity, lifeProfile)
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
	for index, raw := range items {
		item := copyPromptData(mapValue(raw)).(map[string]any)
		if len(item) == 0 {
			return nil, fmt.Errorf("items[%d]: expected an object", index+1)
		}
		start, err := parseScheduleTimeInLocation(stringValue(firstValue(item["start_at"], item["startAt"])), day, location)
		if err != nil {
			return nil, fmt.Errorf("items[%d].start_at: expected RFC3339 or local YYYY-MM-DD HH:MM[:SS] / HH:MM[:SS]: %w", index+1, err)
		}
		end, err := parseScheduleTimeInLocation(stringValue(firstValue(item["end_at"], item["endAt"])), day, location)
		if err != nil {
			return nil, fmt.Errorf("items[%d].end_at: expected RFC3339 or local YYYY-MM-DD HH:MM[:SS] / HH:MM[:SS]: %w", index+1, err)
		}
		if !end.After(start) {
			return nil, fmt.Errorf("items[%d].end_at: expected a time after start_at %s", index+1, start.Format(time.RFC3339))
		}
		activity := stringValue(firstValue(item["activity"], item["task"]))
		scene := stringValue(firstValue(item["scene"], item["context"]))
		if activity == "" || scene == "" {
			return nil, fmt.Errorf("items[%d].activity/scene: expected nonempty activity and scene", index+1)
		}
		item["start_at"] = start.Format(time.RFC3339)
		item["end_at"] = end.Format(time.RFC3339)
		item["activity"] = activity
		item["scene"] = scene
		if location := strings.TrimSpace(stringValue(item["location"])); location != "" {
			item["location"] = location
		}
		entries = append(entries, scheduleEntry{index: index + 1, item: item, start: start, end: end})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].start.Before(entries[j].start) })
	dayStart := day.Truncate(24 * time.Hour)
	// time.Truncate is based on UTC, so construct local midnight explicitly.
	dayStart = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, location)
	nextDay := dayStart.AddDate(0, 0, 1)
	if !entries[0].start.Equal(dayStart) {
		return nil, fmt.Errorf("items[%d].start_at: expected local day start %s", entries[0].index, dayStart.Format(time.RFC3339))
	}
	last := entries[len(entries)-1]
	if !last.end.Equal(nextDay) {
		return nil, fmt.Errorf("items[%d].end_at: expected local day end %s", last.index, nextDay.Format(time.RFC3339))
	}
	resultItems := make([]any, 0, len(entries))
	for index := range entries {
		entry := &entries[index]
		if index > 0 && !entry.start.Equal(entries[index-1].end) {
			expected := entries[index-1].end
			delta := entry.start.Sub(expected)
			// At most five minutes, preserving the preceding end and a
			// positive following interval. Larger discrepancies need correction.
			if delta < -5*time.Minute || delta > 5*time.Minute || !entry.end.After(expected) {
				return nil, fmt.Errorf("items[%d].start_at: expected %s to match items[%d].end_at (maximum automatic alignment 5 minutes)", entry.index, expected.Format(time.RFC3339), entries[index-1].index)
			}
			entry.start = expected
			entry.item["start_at"] = expected.Format(time.RFC3339)
		}
		if entry.start.Before(dayStart) || entry.end.After(nextDay) {
			return nil, fmt.Errorf("items[%d].start_at/end_at: expected times within %s through %s", entry.index, dayStart.Format(time.RFC3339), nextDay.Format(time.RFC3339))
		}
		resultItems = append(resultItems, entry.item)
	}
	payload := map[string]any{"local_date": localDate, "timezone": timezone, "items": resultItems, "reschedule_policy": result["reschedule_policy"], "evidence_refs": []any{"foundation:" + "pending"}}
	if places, present := result["new_places"]; present {
		payload["new_places"] = copyPromptData(places)
	}
	return payload, nil
}

// Only fixed-width date/hour/minute fields and one or two numeric seconds
// are accepted here. A missing separator and a single seconds digit are
// mechanical; letters, invalid fields and ambiguous dates remain errors.
var mechanicalScheduleTimestamp = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})(?:T| )?(\d{2}:\d{2})(?::(\d{1,2}))?$`)

func parseScheduleTimeInLocation(value string, day time.Time, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parts := mechanicalScheduleTimestamp.FindStringSubmatch(value); parts != nil {
		value = parts[1] + "T" + parts[2]
		if parts[3] != "" {
			seconds := parts[3]
			if len(seconds) == 1 {
				seconds = "0" + seconds
			}
			value += ":" + seconds
		}
	}
	if value == "" {
		return time.Time{}, errors.New("timestamp is required")
	}
	if value == "24:00" || value == "24:00:00" {
		return resolveScheduleWallTime(time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, time.UTC), location)
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
				if err != nil {
					for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
						if nominal, e := time.Parse(layout, normalized); e == nil {
							return resolveScheduleWallTime(nominal.AddDate(0, 0, 1), location)
						}
					}
				}
			} else {
				for _, layout := range []string{"2006-01-02 15:04", "2006-01-02 15:04:05"} {
					if nominal, e := time.Parse(layout, normalized); e == nil {
						return resolveScheduleWallTime(nominal.AddDate(0, 0, 1), location)
					}
				}
				err = ErrInvalidArguments
			}
			if err == nil {
				return parsed.AddDate(0, 0, 1), nil
			}
		}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "15:04", "15:04:05"} {
		// Parse the nominal wall fields in UTC first: ParseInLocation silently
		// normalizes nonexistent times and may select either fold occurrence.
		if nominal, err := time.Parse(layout, value); err == nil {
			if layout == "15:04" || layout == "15:04:05" {
				nominal = time.Date(day.Year(), day.Month(), day.Day(), nominal.Hour(), nominal.Minute(), nominal.Second(), 0, time.UTC)
			}
			return resolveScheduleWallTime(nominal, location)
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

// The supplied numeric offset is authoritative for ISO instants; this helper
// applies only to unoffset local wall times. Gaps reject, folds select the
// earliest matching instant consistently.
func resolveScheduleWallTime(wall time.Time, location *time.Location) (time.Time, error) {
	offsets := map[int]bool{}
	for hour := -48; hour <= 48; hour++ {
		_, offset := wall.Add(time.Duration(hour) * time.Hour).In(location).Zone()
		offsets[offset] = true
	}
	var earliest time.Time
	for offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if local.Year() != wall.Year() || local.Month() != wall.Month() || local.Day() != wall.Day() || local.Hour() != wall.Hour() || local.Minute() != wall.Minute() || local.Second() != wall.Second() {
			continue
		}
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}
	if earliest.IsZero() {
		return time.Time{}, errors.New("schedule_local_time_nonexistent")
	}
	return earliest.In(location), nil
}

// Preflight only the immutable model snapshot. Transactional acceptance still
// owns world revisions, proposal creation and reference resolution.
func normalizeInitialScheduleResponse(result map[string]any, date, zone string, places *scheduleWorldPlaceContext) (map[string]any, error) {
	payload, err := normalizeScheduleResponse(result, date, zone)
	if err != nil {
		return nil, err
	}
	if places != nil && places.WorldID != "" {
		// Refer to the original model order, even when normalization sorts items.
		sourceItems := arrayValue(result["items"])
		if len(sourceItems) == 0 {
			sourceItems = arrayValue(mapValue(result["schedule"])["items"])
		}
		for index, raw := range sourceItems {
			item := mapValue(raw)
			ref, id := stringValue(item["place_ref"]), stringValue(item["place_id"])
			if stringValue(item["location_kind"]) == "transit" {
				if ref != "" || id != "" {
					return nil, fmt.Errorf("items[%d].place_ref/place_id: schedule_transit_place_invalid; expected no place reference in transit", index+1)
				}
			} else if stringValue(item["location"]) != "" && ref == "" && id == "" {
				return nil, fmt.Errorf("items[%d].place_ref: schedule_place_reference_required; expected an existing world place id or new_places key for location %q", index+1, item["location"])
			}
			if ref != "" && id != "" && ref != id {
				return nil, fmt.Errorf("items[%d].place_ref/place_id: schedule_place_reference_conflict; expected the same existing place id or only place_ref for a proposal", index+1)
			}
		}
	}
	return payload, nil
}

func defaultInitialSchedule(date, zone string, cause error) (map[string]any, error) {
	payload, err := normalizeScheduleResponse(map[string]any{"items": []any{map[string]any{
		"start_at": "00:00", "end_at": "24:00", "activity": "默认未安排", "scene": "未安排",
		"location_kind": "unknown", "item_type": "free", "status": "planned",
		"priority": 0.0, "flexibility": 1.0, "interruption_cost": 0.0,
	}}, "reschedule_policy": map[string]any{"default_initial_schedule": true, "replaceable": true, "validation_error": cause.Error()}}, date, zone)
	if err != nil {
		return nil, err
	}
	payload["generated_from"] = "default_initial_schedule"
	return payload, nil
}
