package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ResidentMemoryTrace struct {
	Generation     int64  `json:"generation"`
	SourceDigest   string `json:"source_digest"`
	Status         string `json:"status"`
	CandidateCount int    `json:"candidate_count"`
	SelectedCount  int    `json:"selected_count"`
}

type residentMemorySnapshot struct {
	Memories []map[string]any
	Active   []map[string]any
	Trace    ResidentMemoryTrace
}

// Resident Memory is published by the owning source transaction. Reading it
// never writes a Prompt side effect. Row status and scope are checked again at
// the viewer boundary; full Long-term and Raw searches remain independent.
func (a *App) readResidentMemorySnapshot(ctx context.Context, ownerActorID, speakerActorID, fluctlightID string, at time.Time) (residentMemorySnapshot, error) {
	if a == nil || a.DB == nil || strings.TrimSpace(ownerActorID) == "" || strings.TrimSpace(fluctlightID) == "" {
		return residentMemorySnapshot{}, errors.New("resident_memory_scope_invalid")
	}
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, ownerActorID); err != nil {
		return residentMemorySnapshot{}, err
	}
	var generation int64
	var digest, status, actualDigest string
	var encoded []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT generation,source_digest,items_json,status,md5(items_json::text) FROM public.resident_memory_snapshots WHERE fluctlight_id=$1`, fluctlightID).Scan(&generation, &digest, &encoded, &status, &actualDigest); err != nil {
		return residentMemorySnapshot{}, err
	}
	trace := ResidentMemoryTrace{Generation: generation, SourceDigest: digest, Status: status}
	if status != "active" || digest != actualDigest {
		return residentMemorySnapshot{Trace: trace}, nil
	}
	items := decodeArray(encoded)
	trace.CandidateCount = len(items)
	result := residentMemorySnapshot{Memories: make([]map[string]any, 0), Active: make([]map[string]any, 0), Trace: trace}
	for _, raw := range items {
		item := mapValue(raw)
		switch stringValue(item["source_kind"]) {
		case "memory":
			if !memoryVisibleToActor(stringValue(item["visibility"]), fluctlightID, ownerActorID, speakerActorID, arrayValue(item["actor_refs"])) {
				continue
			}
			if status := stringValue(item["provenance_status"]); status != "verified" && status != "partial" {
				continue
			}
			result.Memories = append(result.Memories, cloneMap(item))
		case "active_memory":
			if speakerActorID != ownerActorID {
				continue
			}
			if from := stringValue(item["valid_from"]); from != "" {
				when, err := time.Parse(time.RFC3339Nano, from)
				if err != nil || at.Before(when) {
					continue
				}
			}
			if until := stringValue(item["valid_until"]); until != "" {
				when, err := time.Parse(time.RFC3339Nano, until)
				if err != nil || !at.Before(when) {
					continue
				}
			}
			result.Active = append(result.Active, cloneMap(item))
		}
	}
	result.Trace.SelectedCount = len(result.Memories) + len(result.Active)
	return result, nil
}
