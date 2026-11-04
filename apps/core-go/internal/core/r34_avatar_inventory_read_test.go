package core

import (
	"net/url"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestR34DirectoryDisplaysSelectedReadyAvatarAndWardrobeKeepsOwnMedia(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "r34_media")
	seedUnknownEffectiveLifeForTest(t, f.ctx, f.repository, f.fluctlightID)
	// These projections check storage configuration only; no object I/O occurs.
	f.app.Storage = &minio.Client{}
	asset := "r34_avatar_" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES($1,$2,'v1','image','image/png',68,'r34-digest','isolated','not-fetched','r34-provider','r34-workflow','ready',now())`, asset, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	rows := func() []map[string]any {
		items, err := f.repository.ListFluctlights(f.ctx, f.ownerID)
		if err != nil {
			t.Fatal(err)
		}
		return f.app.DecorateFluctlightList(f.ctx, f.ownerID, items).([]map[string]any)
	}
	before := rows()
	if len(before) != 1 || before[0]["avatar"] != nil {
		t.Fatal("unselected generated image became an avatar", before)
	}
	reference := "r34_reference_" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES($1,$2,'v1','image','image/png',68,'reference-digest','isolated','not-fetched','r34-reference-provider','r34-reference-workflow','ready',now())`, reference, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,canonical_asset_id) VALUES($1,$2,'active',$3)`, "r34_visual_"+f.suffix, f.fluctlightID, reference); err != nil {
		t.Fatal(err)
	}
	if rows()[0]["avatar"] != nil {
		t.Fatal("reference image was reused as an avatar")
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identities SET generated_avatar_asset_id=$2 WHERE fluctlight_id=$1`, f.fluctlightID, asset); err != nil {
		t.Fatal(err)
	}
	after := rows()
	if mapValue(after[0]["avatar"])["asset_id"] != asset || mapValue(after[0]["avatar"])["url"] != "/api/media/"+asset || mapValue(after[0]["avatar_state"])["status"] != "ready" {
		t.Fatal("directory dropped the selected ready avatar", after)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_assets SET status='failed' WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	unavailable := rows()
	if unavailable[0]["avatar"] != nil || mapValue(unavailable[0]["avatar_state"])["status"] != "failed" {
		t.Fatal("unavailable selected image remained visible", unavailable)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_assets SET status='ready' WHERE id=$1`, asset); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"r34-shirt", "r34-coat"} {
		if _, err := f.app.AddWardrobeItems(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"id": id, "item_kind": "wearable", "category": "top", "slot": "upper_body", "description": id, "ownership": "owned", "availability": "available"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES($1,$2,$3,'item','r34-shirt')`, "r34_ref_"+f.suffix, asset, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	page, err := f.app.OwnerInventoryList(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(page["items"]) {
		item := mapValue(raw)
		media := arrayValue(item["media"])
		if item["id"] == "r34-shirt" && (len(media) != 1 || mapValue(media[0])["asset_id"] != asset) {
			t.Fatal("linked wardrobe image was lost", item)
		}
		if item["id"] == "r34-coat" && len(media) != 0 {
			t.Fatal("wardrobe image leaked into another item", item)
		}
	}
}
