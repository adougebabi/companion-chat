package migrations

const selfGovernanceSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.fluctlight_self_owner_reviews (
 id varchar(128) PRIMARY KEY,
 fluctlight_id varchar(128) NOT NULL REFERENCES public.fluctlights(id),
 claim_id varchar(128) NOT NULL REFERENCES public.fluctlight_developing_self_claims(id),
 claim_revision integer NOT NULL,
 owner_actor_id varchar(128) NOT NULL REFERENCES public.actors(id),
 reason text NOT NULL,
 disposition varchar(32) NOT NULL CHECK(disposition='acknowledged'),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_self_owner_reviews_claim ON public.fluctlight_self_owner_reviews(claim_id,created_at DESC,id DESC);
`
