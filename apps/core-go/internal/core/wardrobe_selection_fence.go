package core

import "context"

// Only the typed preparation service supplies this application-owned authority.
// It is absent from model/public Tool arguments. Existing receipts replay first;
// new effects must still match the actual selection-time state under Life locks.
type wardrobeSelectionFenceKey struct{}
type wardrobeSelectionFence struct {
	Revision                int
	ProfileID, LifeRevision string
}

func withWardrobeSelectionFence(ctx context.Context, saved map[string]any) context.Context {
	fence := mapValue(saved["wear_fence"])
	return context.WithValue(ctx, wardrobeSelectionFenceKey{}, wardrobeSelectionFence{Revision: intValue(fence["revision"]), ProfileID: stringValue(fence["profile_id"]), LifeRevision: stringValue(fence["life_revision"])})
}
