package migrations

// Additive metadata and cardinality changes. Existing item/place identities,
// source snapshots and histories stay intact; no preference becomes ownership.
const wardrobeWorldStructureSQL = `
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS clothing_type text;
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS seasons text[] NOT NULL DEFAULT '{}';
ALTER TABLE public.fluctlight_wardrobe_outfits ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';
ALTER TABLE public.fluctlight_wardrobe_states ADD COLUMN IF NOT EXISTS outfit_tags text[] NOT NULL DEFAULT '{}';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='wardrobe_clothing_type_check') THEN
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT wardrobe_clothing_type_check CHECK(clothing_type IS NULL OR clothing_type IN ('outer','upper_body','underwear','panties','lower_body','socks','shoes','accessory'));
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='wardrobe_seasons_check') THEN
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT wardrobe_seasons_check CHECK(seasons <@ ARRAY['spring','summer','autumn','winter']::text[]);
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='wardrobe_worn_item_identity') THEN
  ALTER TABLE public.fluctlight_worn_items DROP CONSTRAINT fluctlight_worn_items_pkey;
  ALTER TABLE public.fluctlight_worn_items ADD CONSTRAINT wardrobe_worn_item_identity PRIMARY KEY(fluctlight_id,item_id);
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='wardrobe_outfit_item_identity') THEN
  ALTER TABLE public.fluctlight_wardrobe_outfit_items DROP CONSTRAINT fluctlight_wardrobe_outfit_items_pkey;
  ALTER TABLE public.fluctlight_wardrobe_outfit_items ADD CONSTRAINT wardrobe_outfit_item_identity PRIMARY KEY(fluctlight_id,outfit_id,item_id);
 END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS wardrobe_single_body_slot ON public.fluctlight_worn_items(fluctlight_id,slot) WHERE slot NOT IN ('accessory','accessories');
CREATE UNIQUE INDEX IF NOT EXISTS wardrobe_outfit_single_body_slot ON public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot) WHERE slot NOT IN ('accessory','accessories');
CREATE TABLE IF NOT EXISTS public.fluctlight_wardrobe_preparations(
 id text PRIMARY KEY,fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),profile_id text NOT NULL,
 intent_id text NOT NULL,attempt integer NOT NULL DEFAULT 1,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','generating','awaiting_wear','completed','failed')),
 proposal jsonb,result jsonb NOT NULL DEFAULT '{}',error_code text,
 expected_revision integer NOT NULL,source_ref text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(fluctlight_id)
);
ALTER TABLE public.life_worlds ADD COLUMN IF NOT EXISTS venue_categories jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.world_places ADD COLUMN IF NOT EXISTS framework_key text;
CREATE UNIQUE INDEX IF NOT EXISTS world_framework_place_key ON public.world_places(world_id,framework_key) WHERE framework_key IS NOT NULL;
`
