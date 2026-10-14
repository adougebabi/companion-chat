package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
)

type contextReferenceKindCapacity struct {
	Count         int `json:"count"`
	SnapshotBytes int `json:"snapshot_bytes"`
}

type contextReferenceSnapshotCapacity struct {
	Ref      string               `json:"ref"`
	Kind     ContextReferenceKind `json:"kind"`
	EntityID string               `json:"entity_id"`
	Bytes    int                  `json:"bytes"`
}

// Capacity is operational metadata, not part of reference authorization or its
// immutable hash. Thresholds explain optimization debt; they never reject data.
type contextReferenceCapacityReport struct {
	ReferenceCount       int                                                   `json:"reference_count"`
	EncodedBytes         int                                                   `json:"encoded_bytes"`
	SnapshotBytes        int                                                   `json:"snapshot_bytes"`
	LargestSnapshotBytes int                                                   `json:"largest_snapshot_bytes"`
	ByKind               map[ContextReferenceKind]contextReferenceKindCapacity `json:"by_kind"`
	LargestSnapshots     []contextReferenceSnapshotCapacity                    `json:"largest_snapshots"`
	Exceeded             []string                                              `json:"advisory_thresholds_exceeded"`
}

func (index ContextReferenceIndex) capacityReport() contextReferenceCapacityReport {
	report := contextReferenceCapacityReport{ReferenceCount: len(index.ByRef), ByKind: map[ContextReferenceKind]contextReferenceKindCapacity{}, Exceeded: []string{}}
	encoded, err := json.Marshal(index)
	if err == nil {
		report.EncodedBytes = len(encoded)
	}
	for ref, entry := range index.ByRef {
		bytes := len(entry.Snapshot)
		report.SnapshotBytes += bytes
		report.LargestSnapshotBytes = max(report.LargestSnapshotBytes, bytes)
		kind := report.ByKind[entry.Kind]
		kind.Count++
		kind.SnapshotBytes += bytes
		report.ByKind[entry.Kind] = kind
		report.LargestSnapshots = append(report.LargestSnapshots, contextReferenceSnapshotCapacity{Ref: ref, Kind: entry.Kind, EntityID: entry.EntityID, Bytes: bytes})
	}
	sort.Slice(report.LargestSnapshots, func(i, j int) bool {
		if report.LargestSnapshots[i].Bytes != report.LargestSnapshots[j].Bytes {
			return report.LargestSnapshots[i].Bytes > report.LargestSnapshots[j].Bytes
		}
		return report.LargestSnapshots[i].Ref < report.LargestSnapshots[j].Ref
	})
	if len(report.LargestSnapshots) > 5 {
		report.LargestSnapshots = report.LargestSnapshots[:5]
	}
	if report.ReferenceCount > contextReferenceCountWarningThreshold {
		report.Exceeded = append(report.Exceeded, "reference_count")
	}
	if report.EncodedBytes > contextReferenceIndexWarningBytes {
		report.Exceeded = append(report.Exceeded, "encoded_bytes")
	}
	if report.LargestSnapshotBytes > contextReferenceSnapshotWarningBytes {
		report.Exceeded = append(report.Exceeded, "snapshot_bytes")
	}
	return report
}

func (a *App) recordContextReferenceCapacity(ctx context.Context, projection ContextProjection, stage string) {
	report := projection.ReferenceIndex.capacityReport()
	if len(report.Exceeded) == 0 {
		return
	}
	correlation := firstString(providerCorrelation(ctx), firstString(projection.SourceFactID, "context:"+stableDigest(projection.FluctlightID+projection.AsOf)))
	payload := map[string]any{
		"stage": stage, "decision": "continue_complete_context", "as_of": projection.AsOf, "capacity": report,
		"advisory_thresholds": map[string]int{"reference_count": contextReferenceCountWarningThreshold, "encoded_bytes": contextReferenceIndexWarningBytes, "snapshot_bytes": contextReferenceSnapshotWarningBytes},
	}
	// Always emit safe metadata to the worker console. The existing best-effort
	// sink may be unavailable before any ModelRun exists; it cannot hide this.
	slog.Default().WarnContext(ctx, "context reference capacity advisory; continuing with complete context",
		"event_type", "context.reference.capacity_warning", "stage", stage, "correlation_id", correlation,
		"fluctlight_id", projection.FluctlightID, "source_fact_id", projection.SourceFactID,
		"decision", payload["decision"], "as_of", projection.AsOf, "advisory_thresholds", payload["advisory_thresholds"],
		"reference_count", report.ReferenceCount, "encoded_bytes", report.EncodedBytes,
		"snapshot_bytes", report.SnapshotBytes, "largest_snapshot_bytes", report.LargestSnapshotBytes, "by_kind", report.ByKind,
		"largest_snapshots", report.LargestSnapshots, "advisory_thresholds_exceeded", report.Exceeded)
	if a != nil {
		a.recordDiagnosticEvent(ctx, "context.reference.capacity_warning", "warning", projection.FluctlightID, projection.SourceFactID, correlation, payload)
	}
}
