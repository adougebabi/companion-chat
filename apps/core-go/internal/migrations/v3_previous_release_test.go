package migrations

import "testing"

func TestV3Upgrade0066ToHeadPreservesSavedWorldMediaSettingsAndPendingTasks(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyReleasedMigrationFixture(t, ctx, pool, "0066_life_place")
	if _, err := pool.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('v3-old-owner','human','active'),('v3-old-fl','fluctlight','active');
 INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,identity,personality,behavioral_policy,life_profile,provenance) VALUES('v3-old-fl','v3-old-owner','blank_slate','active','{"name":"Preserved identity"}','{}','{}','{}','{}');
 INSERT INTO public.life_worlds(id,fluctlight_id,name,background) VALUES('v3-old-world','v3-old-fl','Preserved world','Original world source');
 INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id) VALUES('v3-old-fl','v3-old-world');
 INSERT INTO public.world_places(id,world_id,name,description,details) VALUES('v3-old-place','v3-old-world','Preserved room','Old custom description','{"furniture":["Original desk"]}');
 INSERT INTO public.runtime_settings(key,value_json) VALUES('v3.fixture.setting','{"keep":true}');
 INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('v3.fixture.secret',decode('01020304','hex'),decode('05060708','hex'));
 INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,object_version,provider_request_id,workflow_id,status) VALUES('v3-old-asset','v3-old-fl','version-1','image','image/png',12,'preserved-content-digest','private-bucket','original/object','original-version-id','original-provider-request','original-media-workflow','ready');
 INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count) VALUES('v3-old-intent','v3-old-workflow','lifecycle','platform.control','{"owner_actor_id":"v3-old-owner","source":"original"}','pending',2);`); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal("0066 upgrade/rerun", n, err)
		}
	}
	var retained bool
	if err := pool.QueryRow(ctx, `SELECT
 (SELECT version_num=$1 FROM public.alembic_version)
 AND EXISTS(SELECT 1 FROM public.fluctlights WHERE id='v3-old-fl' AND identity->>'name'='Preserved identity')
 AND EXISTS(SELECT 1 FROM public.world_places WHERE id='v3-old-place' AND world_id='v3-old-world' AND description='Old custom description' AND details->'furniture'='["Original desk"]'::jsonb)
 AND EXISTS(SELECT 1 FROM public.fluctlight_world_bindings WHERE fluctlight_id='v3-old-fl' AND world_id='v3-old-world')
 AND EXISTS(SELECT 1 FROM public.runtime_settings WHERE key='v3.fixture.setting' AND value_json='{"keep":true}')
 AND EXISTS(SELECT 1 FROM public.setting_secrets WHERE purpose='v3.fixture.secret' AND ciphertext=decode('01020304','hex') AND nonce=decode('05060708','hex'))
 AND EXISTS(SELECT 1 FROM public.media_assets WHERE id='v3-old-asset' AND object_version='original-version-id' AND object_key='original/object' AND sha256='preserved-content-digest' AND status='ready')
 AND EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id='v3-old-intent' AND workflow_id='v3-old-workflow' AND attempt_count=2 AND status='pending' AND payload->>'source'='original')
 AND NOT EXISTS(SELECT 1 FROM public.world_place_defaults WHERE place_id='v3-old-place')
 AND to_regclass('public.business_backup_jobs') IS NOT NULL
 AND to_regclass('public.workflow_control_commands') IS NOT NULL`, Head).Scan(&retained); err != nil || !retained {
		t.Fatal("upgrade rewrote saved state, invented a baseline, or lost new schema", retained, err)
	}
}
