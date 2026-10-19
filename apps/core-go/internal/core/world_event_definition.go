package core

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Event definitions describe an explicit virtual-world simulation. They never
// classify an unstructured conversation or guess a character's mental state.
func validateWorldEventDefinition(d map[string]any) error {
	if containsPermanentHarm(d) {
		return harmError()
	}
	if !eventKeys(d, "schema_version", "description", "category", "intensity", "duration_minutes", "trigger", "condition", "effects") || d["schema_version"] != "world-event.v1" {
		return invalid("event schema_version must be world-event.v1")
	}
	if _, err := boundedWorldText(d, "description", 4000, true); err != nil {
		return err
	}
	if !eventEnum(d["category"], "environment", "visitor", "shop", "encounter", "temporary_state") || !eventEnum(d["intensity"], "low", "medium") {
		return invalid("event category or intensity is invalid")
	}
	if !eventInt(d["duration_minutes"], 1, 1440) {
		return invalid("duration_minutes must be 1..1440")
	}
	t := mapValue(d["trigger"])
	if !eventKeys(t, "mode", "interval_minutes", "cooldown_minutes", "probability") || !eventEnum(t["mode"], "manual", "automatic") || !eventInt(t["interval_minutes"], 1, 10080) || !eventInt(t["cooldown_minutes"], 1, 10080) {
		return invalid("event trigger requires a mode, interval and cooldown")
	}
	p, ok := eventNumber(t["probability"])
	if !ok || p < 0 || p > 1 {
		return invalid("event probability must be 0..1")
	}
	c := mapValue(d["condition"])
	if !eventKeys(c, "day", "start", "end", "place_ids", "require_presence") || !eventEnum(c["day"], "daily", "weekday", "weekend") {
		return invalid("event condition is invalid")
	}
	_, sok := parseNPCClock(stringValue(c["start"]))
	_, eok := parseNPCClock(stringValue(c["end"]))
	if !sok || !eok {
		return invalid("condition start/end require HH:mm clocks; equal clocks mean a full day")
	}
	if _, ok := c["require_presence"].(bool); !ok {
		return invalid("require_presence must be explicit")
	}
	places, ok := c["place_ids"].([]any)
	if !ok || len(places) > 32 {
		return invalid("condition place_ids must be an array of at most 32 IDs")
	}
	for _, v := range places {
		if strings.TrimSpace(stringValue(v)) == "" || len(stringValue(v)) > 128 {
			return invalid("condition place ID is invalid")
		}
	}
	effects, ok := d["effects"].([]any)
	if !ok || len(effects) < 1 || len(effects) > 8 {
		return invalid("event requires 1..8 effects")
	}
	injuryCount := 0
	seenEffects := map[string]bool{}
	for _, raw := range effects {
		e := mapValue(raw)
		if !eventKeys(e, "kind", "scope", "place_id", "npc_id", "value", "body_effect") {
			return invalid("unknown event effect field")
		}
		if !eventEnum(e["kind"], "weather", "atmosphere", "traffic", "place_availability", "npc_visit", "temporary_state", "temporary_injury") || !eventEnum(e["scope"], "world", "place", "character") {
			return invalid("event effect kind/scope is invalid")
		}
		if e["scope"] == "place" && strings.TrimSpace(stringValue(e["place_id"])) == "" {
			return invalid("place effect requires place_id")
		}
		if e["scope"] != "place" && stringValue(e["place_id"]) != "" {
			return invalid("place_id requires place scope")
		}
		identity := stringValue(e["kind"]) + ":" + stringValue(e["scope"]) + ":" + stringValue(e["place_id"])
		if e["kind"] == "npc_visit" {
			identity = "npc_visit:" + stringValue(e["npc_id"])
		}
		if seenEffects[identity] {
			return invalid("duplicate or contradictory event effect")
		}
		seenEffects[identity] = true
		if e["kind"] == "npc_visit" {
			if e["scope"] != "place" || strings.TrimSpace(stringValue(e["npc_id"])) == "" {
				return invalid("NPC visit requires NPC and place")
			}
		} else if stringValue(e["npc_id"]) != "" {
			return invalid("npc_id requires npc_visit")
		}
		if e["kind"] == "place_availability" && (e["scope"] != "place" || !eventEnum(e["value"], "open", "closed")) {
			return invalid("place_availability requires open/closed at a place")
		}
		if e["kind"] == "temporary_injury" {
			injuryCount++
			if injuryCount > 1 || e["scope"] != "character" || c["require_presence"] != true {
				return invalid("one temporary injury may affect a confirmed present character")
			}
			b := mapValue(e["body_effect"])
			if !eventKeys(b, "body_part", "description", "severity", "impact") || b["severity"] != "minor" {
				return invalid("world events admit only recoverable minor injury")
			}
			if _, err := validateBodyEventEffect("body_injury", b); err != nil {
				return err
			}
		} else if e["body_effect"] != nil {
			return invalid("body_effect requires temporary_injury")
		}
		if e["kind"] == "temporary_state" && (e["scope"] != "character" || c["require_presence"] != true) {
			return invalid("temporary_state requires actual character presence")
		}
		if _, err := boundedWorldText(e, "value", 1000, true); err != nil {
			return err
		}
	}
	return nil
}
func eventKeys(m map[string]any, keys ...string) bool {
	allowed := map[string]bool{}
	for _, k := range keys {
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func eventEnum(v any, values ...string) bool {
	for _, s := range values {
		if v == s {
			return true
		}
	}
	return false
}
func eventNumber(v any) (float64, bool) {
	switch v.(type) {
	case float64, int:
		return numberFloat(v)
	default:
		return 0, false
	}
}
func eventInt(v any, min, max int) bool {
	n, ok := eventNumber(v)
	return ok && n >= float64(min) && n <= float64(max) && n == float64(int(n))
}
func validateWorldEventReferencesTx(ctx context.Context, tx pgx.Tx, world string, d map[string]any) error {
	places := append([]any{}, arrayValue(mapValue(d["condition"])["place_ids"])...)
	for _, raw := range arrayValue(d["effects"]) {
		e := mapValue(raw)
		if e["scope"] == "place" {
			places = append(places, e["place_id"])
		}
		if e["kind"] == "npc_visit" {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_npcs WHERE world_id=$1 AND id=$2 AND status='active' AND enabled)`, world, e["npc_id"]).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return invalid("event NPC is not available in this world")
			}
		}
	}
	for _, id := range places {
		var valid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE world_id=$1 AND id=$2 AND status='active')`, world, id).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return invalid("event place is not available in this world")
		}
	}
	return nil
}
