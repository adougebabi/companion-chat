package core

import (
	"testing"
	"time"
)

func TestTemporaryHairStyleChangesSharedBodyWithoutChangingLengthOrWardrobe(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	set, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(appearanceStyleCapabilityName, "tie-hair", map[string]any{
		"operation": "set", "style": "扎起头发", "reason": "明确决定暂时换发型",
	}))
	if err != nil || set.Result.Status != "completed" {
		t.Fatalf("temporary hair style: receipt=%#v err=%v", set, err)
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	fields := mapValue(appearance["body_fields"])
	if stringValue(mapValue(fields["hair_length"])["value"]) != "long" || stringValue(mapValue(fields["hair_style"])["value"]) != "扎起头发" {
		t.Fatalf("temporary style changed durable length or was lost: %#v", fields)
	}
	clear, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(appearanceStyleCapabilityName, "untie-hair", map[string]any{
		"operation": "clear", "reason": "决定放下头发",
	}))
	if err != nil || stringValue(mapValue(clear.Result.Output)["status"]) != "cleared" {
		t.Fatalf("clear style: receipt=%#v err=%v", clear, err)
	}
	appearance, _, _, err = fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	fields = mapValue(appearance["body_fields"])
	if stringValue(mapValue(fields["hair_length"])["value"]) != "long" || stringValue(mapValue(fields["hair_style"])["status"]) != "cleared" {
		t.Fatalf("clearing temporary style restored wrong current body: %#v", fields)
	}
}
