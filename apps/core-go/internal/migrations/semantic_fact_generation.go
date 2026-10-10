package migrations

const SemanticFactGenerationPreviousHead = "0057_logical_agent_leases"

// Embeddings are a derived retrieval index. Their availability, retries and
// provider errors do not revise the memory facts or their provenance. Those
// continue to be fenced by the memories/memory_source_links triggers.
// Apply after the historical context-generation installer, including reruns.
const semanticFactGenerationSchemaSQL = `
DROP TRIGGER IF EXISTS context_generation_bump ON public.memory_embeddings;
`
