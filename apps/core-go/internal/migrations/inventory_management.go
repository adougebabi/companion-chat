package migrations

const inventoryManagementSchemaSQL = `
ALTER TABLE public.fluctlight_wardrobe_items ADD COLUMN IF NOT EXISTS archived_at timestamptz;
CREATE TABLE IF NOT EXISTS public.fluctlight_inventory_history (
 id varchar(128) PRIMARY KEY,fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 item_id varchar(128) NOT NULL,producer_table varchar(64) NOT NULL,operation varchar(16) NOT NULL,
 before_value jsonb,after_value jsonb,audit_id varchar(128),source_ref varchar(256),occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS public.fluctlight_inventory_repairs (repair_key varchar(128) PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS ix_inventory_history_item ON public.fluctlight_inventory_history(fluctlight_id,item_id,occurred_at DESC,id DESC);
CREATE OR REPLACE FUNCTION public.record_inventory_history() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE before_row jsonb;after_row jsonb;item_key text;fid text;
BEGIN
 IF TG_OP<>'INSERT' THEN before_row:=to_jsonb(OLD);END IF;
 IF TG_OP<>'DELETE' THEN after_row:=to_jsonb(NEW);END IF;
 fid:=COALESCE(after_row->>'fluctlight_id',before_row->>'fluctlight_id');
 IF TG_TABLE_NAME='fluctlight_wardrobe_items' THEN item_key:=COALESCE(after_row->>'id',before_row->>'id');ELSE item_key:=COALESCE(after_row->>'item_id',before_row->>'item_id');END IF;
 INSERT INTO public.fluctlight_inventory_history(id,fluctlight_id,item_id,producer_table,operation,before_value,after_value,audit_id,source_ref)
 VALUES('inventory_history_'||gen_random_uuid()::text,fid,item_key,TG_TABLE_NAME,TG_OP,before_row,after_row,NULLIF(current_setting('fluctlight.inventory.audit_id',true),''),NULLIF(current_setting('fluctlight.inventory.source_ref',true),''));
 IF TG_OP='UPDATE' AND TG_TABLE_NAME='fluctlight_worn_items' AND before_row->>'item_id' IS DISTINCT FROM after_row->>'item_id' THEN
  INSERT INTO public.fluctlight_inventory_history(id,fluctlight_id,item_id,producer_table,operation,before_value,after_value,audit_id,source_ref)
  VALUES('inventory_history_'||gen_random_uuid()::text,fid,before_row->>'item_id',TG_TABLE_NAME,'REPLACED',before_row,after_row,NULLIF(current_setting('fluctlight.inventory.audit_id',true),''),NULLIF(current_setting('fluctlight.inventory.source_ref',true),''));
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD;END IF;RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS inventory_history ON public.fluctlight_wardrobe_items;
CREATE TRIGGER inventory_history AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_wardrobe_items FOR EACH ROW EXECUTE FUNCTION public.record_inventory_history();
DROP TRIGGER IF EXISTS inventory_history ON public.fluctlight_worn_items;
CREATE TRIGGER inventory_history AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_worn_items FOR EACH ROW EXECUTE FUNCTION public.record_inventory_history();
DROP TRIGGER IF EXISTS inventory_history ON public.fluctlight_item_uses;
CREATE TRIGGER inventory_history AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_item_uses FOR EACH ROW EXECUTE FUNCTION public.record_inventory_history();
DROP TRIGGER IF EXISTS inventory_history ON public.fluctlight_wardrobe_outfit_items;
CREATE TRIGGER inventory_history AFTER INSERT OR UPDATE OR DELETE ON public.fluctlight_wardrobe_outfit_items FOR EACH ROW EXECUTE FUNCTION public.record_inventory_history();

`

const inventoryManagementDataRepairSQL = `
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.fluctlight_inventory_repairs WHERE repair_key='0073_eligibility') THEN RETURN;END IF;
-- Repair only rows that violate already-established use eligibility. This is a
-- sourced migration correction, never an autonomous role action or a purchase.
PERFORM set_config('fluctlight.inventory.source_ref','migration:0073_inventory_management',true);
WITH removed AS (
 DELETE FROM public.fluctlight_item_uses u USING public.fluctlight_wardrobe_items i
 WHERE i.fluctlight_id=u.fluctlight_id AND i.id=u.item_id
 AND (i.item_kind<>'object' OR i.availability<>'available' OR i.ownership NOT IN ('owned','borrowed') OR i.archived_at IS NOT NULL)
 RETURNING u.*
), removed_worn AS (
 DELETE FROM public.fluctlight_worn_items w USING public.fluctlight_wardrobe_items i WHERE i.fluctlight_id=w.fluctlight_id AND i.id=w.item_id
 AND (i.item_kind<>'wearable' OR i.availability<>'available' OR i.archived_at IS NOT NULL OR w.slot<>i.slot) RETURNING w.fluctlight_id
), history AS (
 INSERT INTO public.fluctlight_item_use_events(id,fluctlight_id,item_id,operation,activity,occurred_at,source_ref,item_revision)
 SELECT 'item_use_repair_'||md5(r.fluctlight_id||':'||r.item_id||':'||r.started_at::text),r.fluctlight_id,r.item_id,'stop',r.activity,now(),'migration:0073_inventory_management',i.revision
 FROM removed r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id ON CONFLICT(id) DO NOTHING
)
UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,updated_at=now() WHERE fluctlight_id IN (SELECT fluctlight_id FROM removed UNION SELECT fluctlight_id FROM removed_worn);
INSERT INTO public.fluctlight_inventory_repairs(repair_key)VALUES('0073_eligibility');

END $$;
`
