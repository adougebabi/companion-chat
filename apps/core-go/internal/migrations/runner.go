package migrations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Head identifies the additive Go-owned schema bundle. Released identifiers
// are never rewritten; a new capability advances the head and preserves all
// existing facts.
const Head = "0025_llm_queue"

// Runner applies the clean-start schema without importing the legacy runtime.
// The statements are intentionally idempotent so an existing database keeps
// all facts/assets while a fresh database can boot without external tooling.
type Runner struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Runner { return &Runner{pool: pool} }

func (r *Runner) Apply(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight-go-migrations'))`); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if _, err := tx.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply Go Core schema: %w", err)
	}
	if _, err := tx.Exec(ctx, compatibilitySQL); err != nil {
		return fmt.Errorf("apply Go Core compatibility columns: %w", err)
	}
	var revisions []string
	rows, err := tx.Query(ctx, `SELECT version_num FROM public.alembic_version`)
	if err != nil {
		return fmt.Errorf("read migration ledger: %w", err)
	}
	for rows.Next() {
		var revision string
		if err := rows.Scan(&revision); err != nil {
			rows.Close()
			return err
		}
		revisions = append(revisions, revision)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(revisions) > 1 {
		return errors.New("migration ledger contains multiple heads")
	}
	if len(revisions) == 1 && strings.TrimSpace(revisions[0]) != Head {
		// The schema bundle is additive and preserves existing rows.  This is
		// the one-time bridge for databases created by the released schema chain
		// chain; future migrations must add a new Go bundle and head.
		if _, err := tx.Exec(ctx, `DELETE FROM public.alembic_version`); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES ($1) ON CONFLICT (version_num) DO NOTHING`, Head); err != nil {
		return fmt.Errorf("write migration head: %w", err)
	}
	return tx.Commit(ctx)
}

// schemaSQL contains the authoritative tables needed by the Go Core.  It is
// additive by design: existing PostgreSQL data is never dropped or rewritten.
const schemaSQL = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS public.alembic_version (version_num varchar(32) PRIMARY KEY);
CREATE TABLE IF NOT EXISTS public.actors (id varchar(128) PRIMARY KEY, actor_type varchar(16) NOT NULL, status varchar(16) NOT NULL DEFAULT 'active', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.owner_accounts (human_actor_id varchar(128) PRIMARY KEY, owner_key varchar(16) NOT NULL DEFAULT 'owner', credential_hash text NOT NULL, algorithm varchar(32) NOT NULL DEFAULT 'argon2id', parameters varchar(256) NOT NULL DEFAULT 'argon2id-default', credential_revision varchar(64) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.auth_sessions (id varchar(128) PRIMARY KEY, token_hash varchar(64) NOT NULL UNIQUE, human_actor_id varchar(128) NOT NULL, expires_at timestamptz NOT NULL, last_seen_at timestamptz, revoked_at timestamptz, user_agent_hash varchar(64), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.owner_setup_tokens (id varchar(128) PRIMARY KEY, token_hash varchar(64) NOT NULL UNIQUE, expires_at timestamptz NOT NULL, consumed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.auth_audit_log (id varchar(128) PRIMARY KEY, action varchar(64) NOT NULL, actor_id varchar(128), result varchar(16) NOT NULL, details text NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.provider_endpoints (id varchar(128) PRIMARY KEY, kind varchar(64) NOT NULL, base_url text NOT NULL, secret_purpose varchar(128) NOT NULL, capability_status varchar(32) NOT NULL DEFAULT 'unknown', checked_at timestamptz);
CREATE TABLE IF NOT EXISTS public.model_roles (role varchar(64) PRIMARY KEY, provider_endpoint_id varchar(128) NOT NULL, model_id varchar(256) NOT NULL, required_capabilities text NOT NULL DEFAULT '', token_budget integer NOT NULL DEFAULT 4096, timeout_seconds integer NOT NULL DEFAULT 120, retry_policy text NOT NULL DEFAULT '{}');
CREATE TABLE IF NOT EXISTS public.provider_preflights (id varchar(128) PRIMARY KEY, role varchar(64) NOT NULL, result varchar(32) NOT NULL, capability_version varchar(128), checked_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.provider_provenance (id varchar(128) PRIMARY KEY, role varchar(64) NOT NULL, endpoint_id varchar(128) NOT NULL, model_id varchar(256) NOT NULL, prompt_version varchar(128) NOT NULL, schema_version varchar(128) NOT NULL, correlation_id varchar(128) NOT NULL, token_budget integer NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.runtime_settings (key varchar(128) PRIMARY KEY, value_json text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.setting_secrets (purpose varchar(128) PRIMARY KEY, ciphertext bytea NOT NULL, nonce bytea NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.settings_audit (id varchar(128) PRIMARY KEY, actor_id varchar(128) NOT NULL, field varchar(128) NOT NULL, result varchar(16) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.actor_groups (id varchar(128) PRIMARY KEY, owner_actor_id varchar(128) NOT NULL, name varchar(128) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.actor_group_members (group_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(group_id, actor_id));
CREATE TABLE IF NOT EXISTS public.fluctlights (id varchar(128) PRIMARY KEY, created_by_actor_id varchar(128) NOT NULL, initialization_mode varchar(16) NOT NULL, status varchar(16) NOT NULL DEFAULT 'active', current_revision integer NOT NULL DEFAULT 0, lifecycle_revision integer NOT NULL DEFAULT 0, core_persona jsonb NOT NULL DEFAULT '{}', identity jsonb NOT NULL, personality jsonb NOT NULL, behavioral_policy jsonb NOT NULL, life_profile jsonb NOT NULL, provenance jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), retired_at timestamptz);
CREATE TABLE IF NOT EXISTS public.fluctlight_foundation_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, source varchar(32) NOT NULL, status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, initialization_mode varchar(16) NOT NULL, foundation_status varchar(16) NOT NULL, foundation_created_at timestamptz NOT NULL, confidence jsonb NOT NULL, changes jsonb NOT NULL, core_persona jsonb NOT NULL DEFAULT '{}', identity jsonb NOT NULL, personality jsonb NOT NULL, behavioral_policy jsonb NOT NULL, life_profile jsonb NOT NULL, provenance jsonb NOT NULL, evidence_refs jsonb NOT NULL, reason text, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), accepted_at timestamptz, rejected_at timestamptz);
CREATE TABLE IF NOT EXISTS public.fluctlight_inner_states (fluctlight_id varchar(128) PRIMARY KEY, revision integer NOT NULL DEFAULT 0, pad jsonb NOT NULL, mood jsonb NOT NULL, momentum jsonb NOT NULL, regulation jsonb NOT NULL, drives jsonb NOT NULL, conflicts jsonb NOT NULL, last_updated_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_state_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_event_id varchar(256) NOT NULL, expected_revision integer NOT NULL, resulting_revision integer NOT NULL, previous_state jsonb NOT NULL, resulting_state jsonb NOT NULL, requested_delta jsonb NOT NULL, applied_delta jsonb NOT NULL, result varchar(16) NOT NULL, reason_code varchar(128) NOT NULL, policy_version varchar(128) NOT NULL, model_version varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_goals (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source varchar(16) NOT NULL, description text NOT NULL, importance jsonb NOT NULL, urgency jsonb NOT NULL, progress jsonb NOT NULL, deadline timestamptz, status varchar(16) NOT NULL, evidence_refs jsonb NOT NULL, revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_goal_revisions (id varchar(128) PRIMARY KEY, goal_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_intentions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, goal_id varchar(128), action varchar(256) NOT NULL, preferred_time timestamptz, trigger jsonb NOT NULL, confidence jsonb NOT NULL, expiration timestamptz NOT NULL, evidence_refs jsonb NOT NULL, permission_snapshot jsonb NOT NULL, budget_snapshot jsonb NOT NULL, status varchar(16) NOT NULL, revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_intention_revisions (id varchar(128) PRIMARY KEY, intention_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.conversations (id varchar(128) PRIMARY KEY, created_by_actor_id varchar(128) NOT NULL, title varchar(256), revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.conversation_heads (conversation_id varchar(128) PRIMARY KEY, next_sequence integer NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS public.conversation_participants (conversation_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, role varchar(32) NOT NULL DEFAULT 'member', status varchar(32) NOT NULL DEFAULT 'active', joined_at timestamptz NOT NULL DEFAULT now(), left_at timestamptz, PRIMARY KEY(conversation_id, actor_id));
CREATE TABLE IF NOT EXISTS public.conversation_messages (id varchar(128) PRIMARY KEY, conversation_id varchar(128) NOT NULL, sequence integer NOT NULL, author_actor_id varchar(128) NOT NULL, kind varchar(32) NOT NULL, text text NOT NULL, attachment_refs jsonb NOT NULL DEFAULT '[]', idempotency_key varchar(256) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversation_message_idempotency ON public.conversation_messages(conversation_id,idempotency_key);
CREATE TABLE IF NOT EXISTS public.conversation_read_positions (conversation_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, last_read_sequence integer NOT NULL DEFAULT 0, last_delivered_sequence integer NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(conversation_id, actor_id));
CREATE TABLE IF NOT EXISTS public.fluctlight_direct_conversations (owner_actor_id varchar(128) NOT NULL, fluctlight_actor_id varchar(128) NOT NULL, conversation_id varchar(128) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(owner_actor_id, fluctlight_actor_id));
CREATE TABLE IF NOT EXISTS public.cognition_inbox_heads (fluctlight_id varchar(128) PRIMARY KEY, next_sequence integer NOT NULL DEFAULT 1, last_processed_sequence integer NOT NULL DEFAULT 0, writer_owner varchar(128), writer_lease_until timestamptz);
CREATE TABLE IF NOT EXISTS public.cognition_inbox (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, sequence integer NOT NULL, event_type varchar(128) NOT NULL, payload jsonb NOT NULL, causation_id varchar(128) NOT NULL, correlation_id varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL, occurred_at timestamptz NOT NULL, status varchar(32) NOT NULL DEFAULT 'pending', attempt_count integer NOT NULL DEFAULT 0, claimed_by varchar(128), claimed_at timestamptz, processed_at timestamptz, error_code varchar(128), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.cognition_assessments (id varchar(128) PRIMARY KEY, inbox_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, payload jsonb NOT NULL, schema_version varchar(64) NOT NULL, model varchar(256) NOT NULL, model_version varchar(256) NOT NULL, prompt_version varchar(256) NOT NULL, correlation_id varchar(128) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.cognition_decision_proposals (id varchar(128) PRIMARY KEY, assessment_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, action_type varchar(64) NOT NULL, payload jsonb NOT NULL, confidence text NOT NULL, evidence_refs jsonb NOT NULL, expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.cognition_frozen_actions (id varchar(128) PRIMARY KEY, decision_id varchar(128) NOT NULL, inbox_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, action_type varchar(64) NOT NULL, payload jsonb NOT NULL, state_revision integer NOT NULL, provider_request_id varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'frozen', realization_payload jsonb, error_code varchar(128), frozen_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz);
CREATE TABLE IF NOT EXISTS public.cognition_reflection_windows (fluctlight_id varchar(128) PRIMARY KEY, watermark integer NOT NULL DEFAULT 0, state_revision integer NOT NULL DEFAULT 0, status varchar(32) NOT NULL DEFAULT 'idle', updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.cognition_reflection_proposals (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, from_sequence integer NOT NULL, to_sequence integer NOT NULL, base_state_revision integer NOT NULL, payload jsonb NOT NULL, evidence_refs jsonb NOT NULL, correlation_id varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'proposed', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.cognition_wakeups (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, cycle integer NOT NULL, trigger_type varchar(64) NOT NULL DEFAULT 'periodic', occurred_at timestamptz NOT NULL DEFAULT now(), internal_dynamics jsonb NOT NULL DEFAULT '{}', attention jsonb NOT NULL, thought jsonb NOT NULL, desire jsonb NOT NULL, agency jsonb NOT NULL, action_type varchar(64) NOT NULL, action_id varchar(128), result jsonb NOT NULL DEFAULT '{}', reflection_intent_id varchar(128), status varchar(32) NOT NULL DEFAULT 'completed', UNIQUE(fluctlight_id,cycle));
CREATE TABLE IF NOT EXISTS public.cognition_appraisals (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, payload jsonb NOT NULL, schema_version varchar(64) NOT NULL, model varchar(256) NOT NULL, model_version varchar(256) NOT NULL, prompt_version varchar(256) NOT NULL, evidence_refs jsonb NOT NULL DEFAULT '[]', status varchar(32) NOT NULL DEFAULT 'accepted', revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_fact_id));
CREATE TABLE IF NOT EXISTS public.cognition_focus_cycles (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, appraisal_id varchar(128), attention jsonb NOT NULL, thought jsonb NOT NULL, desire jsonb NOT NULL, agency jsonb NOT NULL, action_type varchar(128) NOT NULL, action_id varchar(128), status varchar(32) NOT NULL DEFAULT 'proposed', revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_fact_id));
CREATE TABLE IF NOT EXISTS public.cognition_action_results (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, status varchar(32) NOT NULL, output jsonb NOT NULL DEFAULT '{}', expected jsonb NOT NULL DEFAULT '{}', observed jsonb NOT NULL DEFAULT '{}', error_code varchar(128), evidence_refs jsonb NOT NULL DEFAULT '[]', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(action_id));
CREATE TABLE IF NOT EXISTS public.cognition_internal_dynamics (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, previous_state jsonb NOT NULL, resulting_state jsonb NOT NULL, requested_delta jsonb NOT NULL, applied_delta jsonb NOT NULL, policy_version varchar(128) NOT NULL, model_version varchar(256) NOT NULL, evidence_refs jsonb NOT NULL DEFAULT '[]', status varchar(32) NOT NULL DEFAULT 'applied', revision integer NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_fact_id));
CREATE TABLE IF NOT EXISTS public.fluctlight_drive_slots (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, label varchar(256) NOT NULL, description text NOT NULL, value_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', decay_policy jsonb NOT NULL DEFAULT '{}', update_policy jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.fluctlight_drive_revisions (id varchar(128) PRIMARY KEY, slot_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_preference_slots (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, label varchar(256) NOT NULL, description text NOT NULL, value_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', update_policy jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.fluctlight_preference_revisions (id varchar(128) PRIMARY KEY, slot_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_trigger_preferences (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, trigger_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.capability_requests (id varchar(128) PRIMARY KEY, capability_key varchar(128) NOT NULL, title varchar(256) NOT NULL, description text NOT NULL, rationale text NOT NULL, desired_contract jsonb NOT NULL DEFAULT '{}', side_effect_class varchar(64) NOT NULL DEFAULT 'unknown', priority varchar(32) NOT NULL DEFAULT 'normal', fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, status varchar(32) NOT NULL DEFAULT 'proposed', review_note text, reviewer_actor_id varchar(128), capability_version varchar(128), idempotency_key varchar(256) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,idempotency_key));
CREATE TABLE IF NOT EXISTS public.cognition_claims (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, claim_type varchar(64) NOT NULL, content text NOT NULL, evidence_refs jsonb NOT NULL, confidence double precision NOT NULL, repetition_key varchar(256) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', expires_at timestamptz, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,repetition_key));
CREATE TABLE IF NOT EXISTS public.fluctlight_evolution_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, field varchar(128) NOT NULL, base_revision integer NOT NULL, revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,field,revision));
CREATE TABLE IF NOT EXISTS public.fluctlight_developing_self_claims (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, category varchar(64) NOT NULL, claim text NOT NULL, value jsonb NOT NULL DEFAULT '{}', confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1), evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', expires_at timestamptz, revision integer NOT NULL DEFAULT 1, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_developing_self_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, claim_id varchar(128), revision integer NOT NULL, base_revision integer NOT NULL, change_type varchar(32) NOT NULL, candidate jsonb NOT NULL DEFAULT '{}', before_value jsonb NOT NULL DEFAULT '{}', after_value jsonb NOT NULL DEFAULT '{}', confidence double precision, evidence_refs jsonb NOT NULL DEFAULT '[]', provenance jsonb NOT NULL DEFAULT '{}', source_window varchar(128), reason_code varchar(128) NOT NULL, status varchar(32) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,claim_id,revision));
CREATE TABLE IF NOT EXISTS public.relationships (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, target_actor_id varchar(128) NOT NULL, metrics jsonb NOT NULL, interaction_frequency double precision NOT NULL DEFAULT 0, last_interaction_at timestamptz, last_meaningful_interaction_at timestamptz, trend varchar(32) NOT NULL DEFAULT 'stable', summary text, emotional_association jsonb NOT NULL DEFAULT '{}', revision integer NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.relationship_revisions (id varchar(128) PRIMARY KEY, relationship_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, metrics jsonb NOT NULL, trend varchar(32) NOT NULL, summary text, emotional_association jsonb NOT NULL, evidence_refs jsonb NOT NULL, actor_id varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.relationship_governance (id varchar(128) PRIMARY KEY, relationship_id varchar(128) NOT NULL, revision_id varchar(128), action varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.memories (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, type varchar(32) NOT NULL, content text NOT NULL, actor_refs jsonb NOT NULL, conversation_id varchar(128), event_refs jsonb NOT NULL, evidence_refs jsonb NOT NULL, confidence double precision NOT NULL, importance double precision NOT NULL, emotional_significance double precision NOT NULL, visibility varchar(32) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, occurred_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), last_confirmed_at timestamptz, search_document tsvector);
CREATE TABLE IF NOT EXISTS public.memory_revisions (id varchar(128) PRIMARY KEY, memory_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, content text NOT NULL, status varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.memory_embeddings (id varchar(128) PRIMARY KEY, memory_id varchar(128) NOT NULL, memory_revision integer NOT NULL, model_id varchar(256) NOT NULL, dimensions integer NOT NULL, embedding jsonb NOT NULL, embedding_vector vector, status varchar(32) NOT NULL DEFAULT 'pending', error_code varchar(128), embedded_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.life_events (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, kind varchar(128) NOT NULL, start_at timestamptz NOT NULL, end_at timestamptz NOT NULL, scene varchar(512), activity varchar(512), location varchar(512), status varchar(32) NOT NULL DEFAULT 'confirmed', evidence_refs jsonb NOT NULL, idempotency_key varchar(256), expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.life_schedules (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, local_date date NOT NULL, timezone varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'proposed', generated_from varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, previous_version_id varchar(128), revision integer NOT NULL DEFAULT 0, generated_at timestamptz NOT NULL DEFAULT now(), reschedule_policy jsonb NOT NULL DEFAULT '{}');
CREATE TABLE IF NOT EXISTS public.life_schedule_items (id varchar(128) PRIMARY KEY, schedule_id varchar(128) NOT NULL, start_at timestamptz NOT NULL, end_at timestamptz NOT NULL, activity varchar(512) NOT NULL, scene varchar(512) NOT NULL, item_type varchar(64) NOT NULL, status varchar(32) NOT NULL, priority varchar(32) NOT NULL, flexibility varchar(32) NOT NULL, interruption_cost varchar(32) NOT NULL);
CREATE TABLE IF NOT EXISTS public.life_presence (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, current_task varchar(512), user_presence varchar(128), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.autonomy_policies (fluctlight_id varchar(128) PRIMARY KEY, mode varchar(32) NOT NULL DEFAULT 'active', allowed_actions jsonb NOT NULL, budget_remaining varchar(64) NOT NULL, quiet_hours jsonb NOT NULL, cooldown_until timestamptz, concurrency_limit integer NOT NULL DEFAULT 1, revision integer NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.autonomy_actions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_type varchar(64) NOT NULL, payload jsonb NOT NULL, policy_snapshot jsonb NOT NULL, expected_revisions jsonb NOT NULL, status varchar(32) NOT NULL DEFAULT 'frozen', workflow_id varchar(128) NOT NULL UNIQUE, provider_request_id varchar(128) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), settled_at timestamptz, error_code varchar(128));
CREATE TABLE IF NOT EXISTS public.autonomy_governance (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_id varchar(128) NOT NULL, from_status varchar(32) NOT NULL, to_status varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moments (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, author_actor_id varchar(128) NOT NULL, text text NOT NULL, visibility varchar(32) NOT NULL DEFAULT 'participants', status varchar(32) NOT NULL DEFAULT 'visible', media_asset_ids jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moment_comments (id varchar(128) PRIMARY KEY, moment_id varchar(128) NOT NULL, author_actor_id varchar(128) NOT NULL, text text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moment_reactions (moment_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, kind varchar(32) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(moment_id, actor_id));
CREATE TABLE IF NOT EXISTS public.moment_read_positions (owner_fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, last_seen_at timestamptz, PRIMARY KEY(owner_fluctlight_id, actor_id));
CREATE TABLE IF NOT EXISTS public.media_intents (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, kind varchar(32) NOT NULL, mime_type varchar(128) NOT NULL, prompt text NOT NULL, provider_request_id varchar(128) NOT NULL UNIQUE, provider_job_id varchar(256) UNIQUE, workflow_id varchar(128) NOT NULL UNIQUE, conversation_id varchar(128), message_id varchar(128), moment_id varchar(128), status varchar(32) NOT NULL DEFAULT 'pending', revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.media_assets (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, version varchar(128) NOT NULL, kind varchar(32) NOT NULL, mime_type varchar(128) NOT NULL, byte_size integer NOT NULL, sha256 varchar(128) NOT NULL, bucket varchar(256) NOT NULL, object_key text NOT NULL, object_version varchar(256), etag varchar(256), provider_request_id varchar(128) NOT NULL, workflow_id varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'pending', created_at timestamptz NOT NULL DEFAULT now(), ready_at timestamptz, tombstoned_at timestamptz, deleted_at timestamptz);
CREATE TABLE IF NOT EXISTS public.media_references (id varchar(128) PRIMARY KEY, asset_id varchar(128) NOT NULL, owner_fluctlight_id varchar(128) NOT NULL, target_type varchar(64) NOT NULL, target_id varchar(128) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.media_tombstones (id varchar(128) PRIMARY KEY, asset_id varchar(128) NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.platform_workflow_intents (intent_id varchar(128) PRIMARY KEY, workflow_id varchar(128) NOT NULL UNIQUE, task_queue varchar(32) NOT NULL, intent_type varchar(96) NOT NULL, payload jsonb NOT NULL, status varchar(32) NOT NULL DEFAULT 'pending', attempt_count integer NOT NULL DEFAULT 0, last_error text, next_attempt_at timestamptz, started_at timestamptz, completed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.platform_outbox_events (id varchar(128) PRIMARY KEY, kind varchar(128) NOT NULL, aggregate_type varchar(96) NOT NULL, aggregate_id varchar(128) NOT NULL, fluctlight_id varchar(128), causation_id varchar(128) NOT NULL, correlation_id varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, payload jsonb NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now(), available_at timestamptz NOT NULL DEFAULT now(), attempt_policy jsonb NOT NULL, published_at timestamptz, completed_at timestamptz, failed_at timestamptz, claim_owner varchar(128), claim_until timestamptz, attempt_count integer NOT NULL DEFAULT 0, last_error text);
CREATE TABLE IF NOT EXISTS public.platform_consumer_inbox (id bigserial PRIMARY KEY, consumer_group varchar(96) NOT NULL, event_id varchar(128) NOT NULL, result jsonb NOT NULL, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.diagnostic_events (id varchar(128) PRIMARY KEY, event_type varchar(128) NOT NULL, severity varchar(32) NOT NULL, fluctlight_id varchar(128), causation_id varchar(128), correlation_id varchar(128) NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.diagnostic_model_runs (id varchar(128) PRIMARY KEY, role varchar(64) NOT NULL, binding_role varchar(64) NOT NULL DEFAULT 'generic_llm', scenario varchar(128) NOT NULL DEFAULT '', priority integer NOT NULL DEFAULT 0, endpoint_id varchar(128), model_id varchar(256) NOT NULL, prompt jsonb NOT NULL, response jsonb, status varchar(32) NOT NULL, error_code varchar(128), correlation_id varchar(128) NOT NULL, queued_at timestamptz NOT NULL DEFAULT now(), started_at timestamptz, completed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.diagnostic_turns (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, conversation_id varchar(128), source_event_id varchar(128), correlation_id varchar(128) NOT NULL, status varchar(32) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.diagnostic_workflow_links (id varchar(128) PRIMARY KEY, correlation_id varchar(128) NOT NULL, workflow_id varchar(128) NOT NULL, intent_id varchar(128), event_id varchar(128), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.diagnostic_retention (id bigserial PRIMARY KEY, resource varchar(64) NOT NULL UNIQUE, retention_days integer NOT NULL, max_rows integer NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.platform_workflow_management_audit (id varchar(128) PRIMARY KEY, action varchar(32) NOT NULL, workflow_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, authorized varchar(8) NOT NULL, details jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_foundation_governance (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, revision_id varchar(128) NOT NULL, action varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_governance (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_goal_governance (id varchar(128) PRIMARY KEY, goal_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_intention_governance (id varchar(128) PRIMARY KEY, intention_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_inner_state_events (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, event_type varchar(64) NOT NULL, payload jsonb NOT NULL, revision integer NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.life_presence_overlays (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, scene varchar(512), activity varchar(512), location varchar(512), current_task varchar(512), user_presence varchar(128), expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moment_unread_markers (owner_fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, last_seen_at timestamptz, PRIMARY KEY(owner_fluctlight_id, actor_id));
CREATE TABLE IF NOT EXISTS public.platform_consumer_effects (id varchar(128) PRIMARY KEY, consumer_group varchar(96) NOT NULL, event_id varchar(128) NOT NULL, effect_type varchar(64) NOT NULL, aggregate_type varchar(96) NOT NULL, aggregate_id varchar(128) NOT NULL, aggregate_sequence integer NOT NULL, correlation_id varchar(128) NOT NULL, fluctlight_id varchar(128), payload_digest varchar(64) NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.platform_consumer_failures (id varchar(128) PRIMARY KEY, consumer_group varchar(96) NOT NULL, event_id varchar(128) NOT NULL, stream_id varchar(128) NOT NULL, attempt integer NOT NULL, max_attempts integer NOT NULL, status varchar(32) NOT NULL, error_code varchar(128) NOT NULL, details jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.platform_consumer_heads (consumer_group varchar(96) NOT NULL, aggregate_type varchar(96) NOT NULL, aggregate_id varchar(128) NOT NULL, last_sequence integer NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(consumer_group, aggregate_type, aggregate_id));
CREATE TABLE IF NOT EXISTS public.platform_object_grants (grant_id varchar(128) PRIMARY KEY, object_key text NOT NULL, object_version varchar(256), expires_at timestamptz NOT NULL, range_policy varchar(64) NOT NULL);
`

// compatibilitySQL covers columns/indexes introduced after the original
// clean-start tables. ADD COLUMN IF NOT EXISTS keeps existing facts intact and
// lets a database stopped at any released revision advance without Python.
const compatibilitySQL = `
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS reason text;
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS rejected_at timestamptz;
ALTER TABLE public.fluctlight_foundation_revisions ADD COLUMN IF NOT EXISTS core_persona jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.fluctlights ADD COLUMN IF NOT EXISTS lifecycle_revision integer NOT NULL DEFAULT 0;
ALTER TABLE public.fluctlights ADD COLUMN IF NOT EXISTS core_persona jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.relationship_governance ADD COLUMN IF NOT EXISTS revision_id varchar(128);
ALTER TABLE public.moment_unread_markers ADD COLUMN IF NOT EXISTS last_seen_at timestamptz;
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS current_task varchar(512);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS user_presence varchar(128);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS scene varchar(512);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS activity varchar(512);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS location varchar(512);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS expires_at timestamptz;
CREATE UNIQUE INDEX IF NOT EXISTS uq_life_schedule_revision ON public.life_schedules(fluctlight_id,local_date,revision);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS conversation_id varchar(128);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS message_id varchar(128);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS moment_id varchar(128);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS object_version varchar(256);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS etag varchar(256);
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS previous_version_id varchar(128);
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS last_confirmed_at timestamptz;
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE public.conversation_participants ADD COLUMN IF NOT EXISTS left_at timestamptz;
ALTER TABLE public.conversation_messages ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.conversation_read_positions ADD COLUMN IF NOT EXISTS last_delivered_sequence integer NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uq_media_intents_provider_job_id ON public.media_intents(provider_job_id) WHERE provider_job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_media_intents_message ON public.media_intents(message_id) WHERE message_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversation_messages_idempotency ON public.conversation_messages(conversation_id,idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS uq_cognition_inbox_idempotency ON public.cognition_inbox(fluctlight_id,idempotency_key);
CREATE INDEX IF NOT EXISTS ix_memories_search_document ON public.memories USING gin(search_document);
CREATE INDEX IF NOT EXISTS ix_cognition_claims_context ON public.cognition_claims(fluctlight_id,status,expires_at,created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_life_events_idempotency ON public.life_events(fluctlight_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_cognition_wakeups_fluctlight_occurred ON public.cognition_wakeups(fluctlight_id,occurred_at DESC);
CREATE INDEX IF NOT EXISTS ix_capability_requests_key_status ON public.capability_requests(capability_key,status,updated_at DESC);
CREATE INDEX IF NOT EXISTS ix_capability_requests_fluctlight ON public.capability_requests(fluctlight_id,created_at DESC);
CREATE INDEX IF NOT EXISTS ix_fluctlight_drive_slots_active ON public.fluctlight_drive_slots(fluctlight_id,status,updated_at DESC);
CREATE INDEX IF NOT EXISTS ix_fluctlight_preference_slots_active ON public.fluctlight_preference_slots(fluctlight_id,status,updated_at DESC);
CREATE INDEX IF NOT EXISTS ix_developing_self_claims_active ON public.fluctlight_developing_self_claims(fluctlight_id,status,updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_developing_self_claim_active ON public.fluctlight_developing_self_claims(fluctlight_id,category,claim) WHERE status IN ('active','uncertain');
CREATE INDEX IF NOT EXISTS ix_developing_self_revisions_fluctlight ON public.fluctlight_developing_self_revisions(fluctlight_id,created_at DESC);
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS status varchar(32) NOT NULL DEFAULT 'pending';
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS attempt_count integer NOT NULL DEFAULT 0;
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS last_error text;
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz;
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE public.platform_workflow_intents ADD COLUMN IF NOT EXISTS completed_at timestamptz;
ALTER TABLE public.platform_outbox_events ADD COLUMN IF NOT EXISTS claim_owner varchar(128);
ALTER TABLE public.platform_outbox_events ADD COLUMN IF NOT EXISTS claim_until timestamptz;
ALTER TABLE public.platform_outbox_events ADD COLUMN IF NOT EXISTS attempt_count integer NOT NULL DEFAULT 0;
ALTER TABLE public.platform_outbox_events ADD COLUMN IF NOT EXISTS last_error text;
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS binding_role varchar(64) NOT NULL DEFAULT 'generic_llm';
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS scenario varchar(128) NOT NULL DEFAULT '';
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS priority integer NOT NULL DEFAULT 0;
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS queued_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS started_at timestamptz;
ALTER TABLE public.diagnostic_model_runs ADD COLUMN IF NOT EXISTS completed_at timestamptz;
UPDATE public.diagnostic_model_runs SET binding_role=CASE WHEN role='embedding' THEN 'embedding' ELSE 'generic_llm' END WHERE binding_role IS NULL OR binding_role='' OR (binding_role='generic_llm' AND role='embedding');
INSERT INTO public.runtime_settings(key,value_json) VALUES ('llm.queue','{"generated_concurrency":2,"embedding_concurrency":1}') ON CONFLICT (key) DO NOTHING;
INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy)
SELECT 'generic_llm',provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy
FROM public.model_roles
WHERE role IN ('action_realization','cognitive_assessment','interaction','reflection','initialization','media_prompt')
ORDER BY CASE role WHEN 'action_realization' THEN 1 WHEN 'cognitive_assessment' THEN 2 WHEN 'interaction' THEN 3 WHEN 'reflection' THEN 4 WHEN 'initialization' THEN 5 ELSE 6 END
LIMIT 1 ON CONFLICT (role) DO NOTHING;
CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_consumer_inbox_group_event ON public.platform_consumer_inbox(consumer_group,event_id);
CREATE INDEX IF NOT EXISTS ix_platform_outbox_available ON public.platform_outbox_events(published_at,failed_at,available_at,claim_until,occurred_at);
`
