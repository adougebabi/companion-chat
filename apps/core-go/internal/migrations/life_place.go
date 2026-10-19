package migrations

// Stable place identity is explicit and nullable. Existing textual locations
// retain unknown identity; no name matching/backfill can create false presence.
const lifePlaceSchemaSQL = `
ALTER TABLE public.life_schedule_items ADD COLUMN IF NOT EXISTS place_id varchar(128) REFERENCES public.world_places(id);
CREATE INDEX IF NOT EXISTS ix_life_schedule_place ON public.life_schedule_items(place_id) WHERE place_id IS NOT NULL;
`
