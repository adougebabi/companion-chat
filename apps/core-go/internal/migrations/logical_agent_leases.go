package migrations

const LogicalAgentLeasesPreviousHead = "0056_kev_decisions"
const logicalAgentLeasesSchemaSQL = `
CREATE TABLE IF NOT EXISTS public.logical_agent_leases(
 fluctlight_id varchar(128) PRIMARY KEY REFERENCES public.fluctlights(id) ON DELETE CASCADE,
 owner_token varchar(128) NOT NULL,
 run_kind varchar(64) NOT NULL,
 expires_at timestamptz NOT NULL
);
`
