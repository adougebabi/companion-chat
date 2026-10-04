package core

import (
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/instant"
	"time"
)

// Instant strings are display/transport values. Storage and pagination keep
// their original precision; dates, durations and recurring wall clocks are
// deliberately not represented by this format.
const instantLayout = instant.Layout

func formatInstant(at time.Time) string { return instant.Format(at) }

func formatLocalInstant(at time.Time, zone *time.Location) string {
	if zone == nil {
		zone = time.UTC
	}
	return instant.FormatLocal(at, zone)
}

func projectionTimeView(projection ContextProjection) map[string]any {
	zoneName := firstString(projection.ReferenceTimezone, stringValue(projection.LifeContext["timezone"]))
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		zoneName, zone = "UTC", time.UTC
	}
	at, err := time.Parse(time.RFC3339Nano, projection.AsOf)
	if err != nil {
		at, err = time.Parse(time.RFC3339Nano, stringValue(projection.LifeContext["instant"]))
	}
	result := map[string]any{
		"reference_timezone":     zoneName,
		"history_time_semantics": "Display timezone does not identify the sender's location.",
		"actor_user_timezone":    nil, "actor_user_local_time": nil,
	}
	if err == nil {
		result["as_of"] = formatLocalInstant(at, zone)
		result["actor_self_local_time"] = formatLocalInstant(at, zone)
	}
	if err == nil {
		for _, fact := range projection.ActorFacts {
			actor := actorRefForID(projection.Actors, stringValue(fact["actor_id"]))
			if stringValue(actor["ref"]) != "actor_user" || fact["attribute"] != "timezone" || fact["status"] != "active" {
				continue
			}
			name := stringValue(fact["value"])
			if userZone, loadErr := time.LoadLocation(name); name != "" && loadErr == nil {
				result["actor_user_timezone"] = name
				result["actor_user_local_time"] = formatLocalInstant(at, userZone)
			}
		}
	}
	return result
}
