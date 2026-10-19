package migrations

const ownerMediaRequestsSchemaSQL = `
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS owner_request_id varchar(128) UNIQUE REFERENCES public.owner_surface_records(id);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS owner_item_id varchar(128) REFERENCES public.fluctlight_wardrobe_items(id);
DO $$BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='media_owner_request_target_check') THEN
 ALTER TABLE public.media_intents ADD CONSTRAINT media_owner_request_target_check CHECK(
 (owner_request_id IS NULL AND owner_item_id IS NULL) OR
 (owner_request_id IS NOT NULL AND media_scope='fluctlight' AND conversation_id IS NULL AND message_id IS NULL AND moment_id IS NULL));
 END IF;
END$$;
CREATE INDEX IF NOT EXISTS ix_owner_media_item ON public.media_intents(owner_fluctlight_id,owner_item_id,created_at DESC,id DESC) WHERE owner_request_id IS NOT NULL;
`
