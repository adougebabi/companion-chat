package migrations

// Reuse the existing item authority for wearable and ordinary objects. Only
// wearable items have a wearing slot; ordinary use is a separate operation.
const inventoryUsageSchemaSQL = `
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS item_kind varchar(16) NOT NULL DEFAULT 'wearable';
ALTER TABLE public.fluctlight_wardrobe_items DROP CONSTRAINT IF EXISTS fluctlight_wardrobe_items_slot_check;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='ck_inventory_item_kind') THEN
  ALTER TABLE public.fluctlight_wardrobe_items ADD CONSTRAINT ck_inventory_item_kind CHECK
   ((item_kind='wearable' AND length(trim(slot))>0) OR (item_kind='object' AND slot=''));
 END IF;
END $$;
CREATE TABLE IF NOT EXISTS public.fluctlight_item_uses (
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 item_id varchar(128) NOT NULL,
 revision integer NOT NULL DEFAULT 1,
 activity varchar(512) NOT NULL,
 started_at timestamptz NOT NULL,
 source_event_id varchar(128) NOT NULL,
 PRIMARY KEY(fluctlight_id,item_id),
 FOREIGN KEY(fluctlight_id,item_id) REFERENCES public.fluctlight_wardrobe_items(fluctlight_id,id)
);
CREATE TABLE IF NOT EXISTS public.fluctlight_item_use_events (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 item_id varchar(128) NOT NULL,
 operation varchar(16) NOT NULL CHECK (operation IN ('start','stop')),
 activity varchar(512) NOT NULL,
 occurred_at timestamptz NOT NULL,
 source_ref varchar(256) NOT NULL,
 item_revision integer NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(fluctlight_id,item_id) REFERENCES public.fluctlight_wardrobe_items(fluctlight_id,id)
);
DROP TRIGGER IF EXISTS context_generation_bump ON public.fluctlight_item_uses;
CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_item_uses
 FOR EACH ROW EXECUTE FUNCTION public.bump_context_direct_trigger();
`
