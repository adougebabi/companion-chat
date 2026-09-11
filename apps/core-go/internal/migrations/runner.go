package migrations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Head identifies the Go-owned schema bundle. Released identifiers are never
// rewritten; the bounded capability-runtime reconciliation below is the one
// explicitly allowed active-payload migration and preserves audit history.
const Head = "0031_evolution_authority"
const PreviousHead = "0030_life_context_revision"
const LifeContextRevisionHead = "0030_life_context_revision"
const MemoryLifecycleHead = "0029_memory_lifecycle"
const AffectCanonicalHead = "0028_affect_canonical"
const ProjectHealthHead = "0027_project_health_evolution"
const CapabilityRuntimeHead = "0026_capability_runtime"
const CapabilityRuntimePreviousHead = "0025_llm_queue"
const ReleasedHead = "0020_media_provider_job"

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
	current := ""
	if len(revisions) == 1 {
		current = strings.TrimSpace(revisions[0])
		if current != revisions[0] {
			return fmt.Errorf("migration ledger head %q is not canonical", revisions[0])
		}
	}
	applyCapabilityRuntime := len(revisions) == 0 || current == ReleasedHead || current == CapabilityRuntimePreviousHead
	applyProjectHealth := len(revisions) == 0 || current == ReleasedHead || current == CapabilityRuntimePreviousHead || current == CapabilityRuntimeHead
	applyAffectCanonical := applyProjectHealth || current == ProjectHealthHead
	applyMemoryLifecycle := applyAffectCanonical || current == AffectCanonicalHead
	applyLifeContextRevision := applyMemoryLifecycle || current == MemoryLifecycleHead
	applyEvolutionAuthority := applyLifeContextRevision || current == LifeContextRevisionHead
	if len(revisions) == 1 && current != Head {
		if current != ReleasedHead && current != CapabilityRuntimePreviousHead && current != CapabilityRuntimeHead && current != ProjectHealthHead && current != AffectCanonicalHead && current != MemoryLifecycleHead && current != LifeContextRevisionHead {
			return fmt.Errorf("unsupported migration head %q; expected %s, %s, %s, %s, %s, %s, %s, or %s", revisions[0], ReleasedHead, CapabilityRuntimePreviousHead, CapabilityRuntimeHead, ProjectHealthHead, AffectCanonicalHead, MemoryLifecycleHead, LifeContextRevisionHead, Head)
		}
	}
	if applyCapabilityRuntime {
		if _, err := tx.Exec(ctx, capabilityRuntimeMigrationSQL); err != nil {
			return fmt.Errorf("apply capability runtime migration: %w", err)
		}
	}
	if applyProjectHealth {
		if _, err := tx.Exec(ctx, projectHealthEvolutionMigrationSQL); err != nil {
			return fmt.Errorf("apply project health evolution migration: %w", err)
		}
	}
	if applyAffectCanonical {
		if _, err := tx.Exec(ctx, affectCanonicalMigrationSQL); err != nil {
			return fmt.Errorf("apply canonical affect migration: %w", err)
		}
	}
	if applyMemoryLifecycle {
		if _, err := tx.Exec(ctx, memoryLifecycleMigrationSQL); err != nil {
			return fmt.Errorf("apply Memory lifecycle migration: %w", err)
		}
	}
	if applyLifeContextRevision {
		if _, err := tx.Exec(ctx, lifeContextRevisionMigrationSQL); err != nil {
			return fmt.Errorf("apply Life Context revision migration: %w", err)
		}
	}
	if applyEvolutionAuthority {
		if _, err := tx.Exec(ctx, evolutionAuthorityMigrationSQL); err != nil {
			return fmt.Errorf("apply evolution authority migration: %w", err)
		}
	}
	if len(revisions) == 1 && strings.TrimSpace(revisions[0]) != Head {
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
CREATE TABLE IF NOT EXISTS public.fluctlight_affect_profiles (fluctlight_id varchar(128) PRIMARY KEY, baseline_pad jsonb NOT NULL DEFAULT '{"pleasure":0,"arousal":0,"dominance":0}', decay_policy jsonb NOT NULL DEFAULT '{"pad_half_life_seconds":21600,"momentum_half_life_seconds":3600,"mood_half_life_seconds":7200,"drive_half_life_seconds":14400}', regulation_policy jsonb NOT NULL DEFAULT '{"strength":0}', emotional_summary jsonb NOT NULL DEFAULT '{}', evidence_refs jsonb NOT NULL DEFAULT '[]', revision integer NOT NULL DEFAULT 0, policy_version varchar(128) NOT NULL DEFAULT 'affect.reducer.v2', updated_at timestamptz NOT NULL DEFAULT now(), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_affect_reconciliations (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_head varchar(64) NOT NULL, mapping_policy varchar(128) NOT NULL, before_state jsonb NOT NULL, after_state jsonb NOT NULL, disposition varchar(32) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_head));
CREATE TABLE IF NOT EXISTS public.fluctlight_personality_runtime (fluctlight_id varchar(128) PRIMARY KEY, active_profile_id varchar(128) NOT NULL, previous_profile_id varchar(128), revision integer NOT NULL DEFAULT 0, switch_reason text NOT NULL DEFAULT '', switched_at timestamptz, cooldown_until timestamptz, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_state_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_event_id varchar(256) NOT NULL, expected_revision integer NOT NULL, resulting_revision integer NOT NULL, previous_state jsonb NOT NULL, resulting_state jsonb NOT NULL, requested_delta jsonb NOT NULL, applied_delta jsonb NOT NULL, result varchar(16) NOT NULL, reason_code varchar(128) NOT NULL, policy_version varchar(128) NOT NULL, model_version varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_goals (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, profile_id varchar(128), source varchar(16) NOT NULL, scope varchar(32) NOT NULL DEFAULT 'general', target_actor_id varchar(128), description text NOT NULL, importance jsonb NOT NULL, urgency jsonb NOT NULL, progress jsonb NOT NULL, deadline timestamptz, status varchar(16) NOT NULL, evidence_refs jsonb NOT NULL, revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_goal_revisions (id varchar(128) PRIMARY KEY, goal_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, from_status varchar(16) NOT NULL, to_status varchar(16) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_intentions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, profile_id varchar(128), goal_id varchar(128), action varchar(256) NOT NULL, preferred_time timestamptz, trigger jsonb NOT NULL, confidence jsonb NOT NULL, expiration timestamptz NOT NULL, evidence_refs jsonb NOT NULL, permission_snapshot jsonb NOT NULL, budget_snapshot jsonb NOT NULL, status varchar(16) NOT NULL, revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
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
CREATE TABLE IF NOT EXISTS public.cognition_action_outcomes (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_id varchar(128) NOT NULL, call_id varchar(128) NOT NULL, capability_name varchar(128) NOT NULL DEFAULT '', status varchar(32) NOT NULL, success_boundary varchar(128) NOT NULL, completion_boundary varchar(128), external_ref varchar(256), expected jsonb NOT NULL DEFAULT '{}', observed jsonb NOT NULL DEFAULT '{}', error_code varchar(128), goal_refs jsonb NOT NULL DEFAULT '[]', intention_refs jsonb NOT NULL DEFAULT '[]', evidence_refs jsonb NOT NULL DEFAULT '[]', context_references jsonb NOT NULL DEFAULT '{}', revision integer NOT NULL DEFAULT 1, request_digest varchar(64) NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(action_id,call_id));
CREATE INDEX IF NOT EXISTS ix_cognition_action_outcomes_fluctlight_recent ON public.cognition_action_outcomes(fluctlight_id,occurred_at DESC,id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_cognition_action_outcomes_external_ref ON public.cognition_action_outcomes(external_ref) WHERE external_ref IS NOT NULL AND external_ref <> '';
CREATE TABLE IF NOT EXISTS public.cognition_internal_dynamics (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, previous_state jsonb NOT NULL, resulting_state jsonb NOT NULL, requested_delta jsonb NOT NULL, applied_delta jsonb NOT NULL, policy_version varchar(128) NOT NULL, model_version varchar(256) NOT NULL, evidence_refs jsonb NOT NULL DEFAULT '[]', status varchar(32) NOT NULL DEFAULT 'applied', revision integer NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,source_fact_id));
CREATE TABLE IF NOT EXISTS public.fluctlight_drive_slots (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, label varchar(256) NOT NULL, description text NOT NULL, value_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', decay_policy jsonb NOT NULL DEFAULT '{}', update_policy jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.fluctlight_drive_revisions (id varchar(128) PRIMARY KEY, slot_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_preference_slots (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, label varchar(256) NOT NULL, description text NOT NULL, value_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', update_policy jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.fluctlight_preference_revisions (id varchar(128) PRIMARY KEY, slot_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_trigger_preferences (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, key varchar(128) NOT NULL, trigger_schema varchar(64) NOT NULL, value jsonb NOT NULL, confidence jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,key));
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_identities (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL UNIQUE, status varchar(32) NOT NULL DEFAULT 'missing', current_revision integer NOT NULL DEFAULT 0, identity_snapshot jsonb NOT NULL DEFAULT '{}', renderer_constraints jsonb NOT NULL DEFAULT '{}', canonical_asset_id varchar(128), character_sheet_asset_id varchar(128), adapter_version varchar(128) NOT NULL DEFAULT 'chest-cup-adapter.v1', active_session_id varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_identity_revisions (id varchar(128) PRIMARY KEY, visual_identity_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, identity_snapshot jsonb NOT NULL, renderer_constraints jsonb NOT NULL, canonical_asset_id varchar(128) NOT NULL, character_sheet_asset_id varchar(128), adapter_version varchar(128) NOT NULL, source varchar(64) NOT NULL, evidence_refs jsonb NOT NULL DEFAULT '[]', idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(visual_identity_id,revision));
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_identity_sessions (id varchar(128) PRIMARY KEY, visual_identity_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, trigger_type varchar(32) NOT NULL, workflow_id varchar(128) NOT NULL UNIQUE, source_fact_id varchar(128), max_attempts integer NOT NULL DEFAULT 3, current_attempt integer NOT NULL DEFAULT 1, character_sheet_media_intent_id varchar(128), status varchar(32) NOT NULL DEFAULT 'queued', last_error text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE UNIQUE INDEX IF NOT EXISTS uq_visual_identity_active_session ON public.fluctlight_visual_identity_sessions(fluctlight_id) WHERE status IN ('queued','running');
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_identity_attempts (id varchar(128) PRIMARY KEY, session_id varchar(128) NOT NULL, visual_identity_id varchar(128) NOT NULL, fluctlight_id varchar(128) NOT NULL, attempt_number integer NOT NULL, status varchar(32) NOT NULL DEFAULT 'queued', seed_prompt text, input_snapshot jsonb NOT NULL DEFAULT '{}', renderer_constraints jsonb NOT NULL DEFAULT '{}', media_intent_id varchar(128), candidate_asset_id varchar(128), vision_result jsonb NOT NULL DEFAULT '{}', patch_result jsonb NOT NULL DEFAULT '{}', decision varchar(32), feedback text, error_code varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(session_id,attempt_number));
CREATE TABLE IF NOT EXISTS public.fluctlight_visual_identity_timeline (id varchar(128) PRIMARY KEY, session_id varchar(128) NOT NULL, attempt_id varchar(128), fluctlight_id varchar(128) NOT NULL, stage varchar(64) NOT NULL, stage_order integer NOT NULL DEFAULT 0, status varchar(32) NOT NULL, summary text NOT NULL DEFAULT '', asset_ids jsonb NOT NULL DEFAULT '[]', metadata jsonb NOT NULL DEFAULT '{}', correlation_id varchar(128) NOT NULL, occurred_at timestamptz NOT NULL DEFAULT now(), UNIQUE(session_id,attempt_id,stage,occurred_at));
CREATE TABLE IF NOT EXISTS public.capability_requests (id varchar(128) PRIMARY KEY, capability_key varchar(128) NOT NULL, title varchar(256) NOT NULL, description text NOT NULL, rationale text NOT NULL, desired_contract jsonb NOT NULL DEFAULT '{}', side_effect_class varchar(64) NOT NULL DEFAULT 'unknown', priority varchar(32) NOT NULL DEFAULT 'normal', fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, status varchar(32) NOT NULL DEFAULT 'proposed', review_note text, reviewer_actor_id varchar(128), capability_version varchar(128), idempotency_key varchar(256) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,idempotency_key));
CREATE TABLE IF NOT EXISTS public.cognition_claims (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, source_fact_id varchar(128) NOT NULL, claim_type varchar(64) NOT NULL, content text NOT NULL, evidence_refs jsonb NOT NULL, confidence double precision NOT NULL, repetition_key varchar(256) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', expires_at timestamptz, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,repetition_key));
CREATE TABLE IF NOT EXISTS public.fluctlight_evolution_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, field varchar(128) NOT NULL, base_revision integer NOT NULL, revision integer NOT NULL, candidate_type varchar(64) NOT NULL, before_value jsonb NOT NULL, after_value jsonb NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'accepted', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,field,revision));
CREATE TABLE IF NOT EXISTS public.fluctlight_developing_self_claims (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, category varchar(64) NOT NULL, claim text NOT NULL, value jsonb NOT NULL DEFAULT '{}', confidence double precision NOT NULL CHECK (confidence >= 0 AND confidence <= 1), evidence_refs jsonb NOT NULL, provenance jsonb NOT NULL DEFAULT '{}', status varchar(32) NOT NULL DEFAULT 'active', expires_at timestamptz, revision integer NOT NULL DEFAULT 1, superseded_by varchar(128), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.fluctlight_developing_self_revisions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, claim_id varchar(128), revision integer NOT NULL, base_revision integer NOT NULL, change_type varchar(32) NOT NULL, candidate jsonb NOT NULL DEFAULT '{}', before_value jsonb NOT NULL DEFAULT '{}', after_value jsonb NOT NULL DEFAULT '{}', confidence double precision, evidence_refs jsonb NOT NULL DEFAULT '[]', provenance jsonb NOT NULL DEFAULT '{}', source_window varchar(128), reason_code varchar(128) NOT NULL, status varchar(32) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,claim_id,revision));
CREATE TABLE IF NOT EXISTS public.relationships (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, profile_id varchar(128), target_actor_id varchar(128) NOT NULL, role jsonb NOT NULL DEFAULT '{}', metrics jsonb NOT NULL, interaction_frequency double precision NOT NULL DEFAULT 0, last_interaction_at timestamptz, last_meaningful_interaction_at timestamptz, trend varchar(32) NOT NULL DEFAULT 'stable', summary text, emotional_association jsonb NOT NULL DEFAULT '{}', provenance jsonb NOT NULL DEFAULT '{}', revision integer NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.relationship_revisions (id varchar(128) PRIMARY KEY, relationship_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, role jsonb NOT NULL DEFAULT '{}', metrics jsonb NOT NULL, trend varchar(32) NOT NULL, summary text, emotional_association jsonb NOT NULL, evidence_refs jsonb NOT NULL, actor_id varchar(128) NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.relationship_governance (id varchar(128) PRIMARY KEY, relationship_id varchar(128) NOT NULL, revision_id varchar(128), action varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.memories (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, type varchar(32) NOT NULL, content text NOT NULL, actor_refs jsonb NOT NULL, conversation_id varchar(128), event_refs jsonb NOT NULL, evidence_refs jsonb NOT NULL, personality_perspectives jsonb NOT NULL DEFAULT '[]', confidence double precision NOT NULL, importance double precision NOT NULL, emotional_significance double precision NOT NULL, visibility varchar(32) NOT NULL, status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 0, canonical_key varchar(128), request_digest varchar(128), superseded_by_memory_id varchar(128), supersedes_memory_id varchar(128), occurred_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), last_confirmed_at timestamptz, deprecated_at timestamptz, search_document tsvector);
CREATE TABLE IF NOT EXISTS public.memory_revisions (id varchar(128) PRIMARY KEY, memory_id varchar(128) NOT NULL, revision integer NOT NULL, base_revision integer NOT NULL, operation varchar(32) NOT NULL DEFAULT 'legacy', snapshot jsonb NOT NULL DEFAULT '{}', content text NOT NULL, personality_perspectives jsonb NOT NULL DEFAULT '[]', status varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, source_window varchar(128), proposal_id varchar(128), candidate_index integer, semantic_reason text, related_memory_ids jsonb NOT NULL DEFAULT '[]', request_digest varchar(128), reason_code varchar(128), schema_version varchar(64) NOT NULL DEFAULT 'legacy.v1', idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(memory_id,revision));
CREATE TABLE IF NOT EXISTS public.memory_governance (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, proposal_id varchar(128), source_window varchar(128), candidate_index integer, operation varchar(32) NOT NULL, target_memory_id varchar(128), related_memory_ids jsonb NOT NULL DEFAULT '[]', base_revisions jsonb NOT NULL DEFAULT '{}', resulting_revisions jsonb NOT NULL DEFAULT '{}', actor_id varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, semantic_reason text NOT NULL, disposition varchar(32) NOT NULL, reason_code varchar(128) NOT NULL, policy_version varchar(128) NOT NULL, request_digest varchar(128) NOT NULL, result jsonb NOT NULL, idempotency_key varchar(256) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(proposal_id,candidate_index));
CREATE TABLE IF NOT EXISTS public.memory_lifecycle_repairs (memory_id varchar(128) PRIMARY KEY, repair_version varchar(64) NOT NULL, source_snapshot jsonb NOT NULL, canonical_key varchar(128) NOT NULL, request_digest varchar(128) NOT NULL, repaired_by_actor_id varchar(128) NOT NULL, reason text NOT NULL, repaired_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.memory_embeddings (id varchar(128) PRIMARY KEY, memory_id varchar(128) NOT NULL, memory_revision integer NOT NULL, provider_endpoint_id varchar(128), model_id varchar(256) NOT NULL, dimensions integer NOT NULL, embedding jsonb NOT NULL, embedding_vector vector, status varchar(32) NOT NULL DEFAULT 'pending', error_code varchar(128), embedded_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(memory_id,memory_revision,model_id));
CREATE TABLE IF NOT EXISTS public.life_events (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, kind varchar(128) NOT NULL, start_at timestamptz NOT NULL, end_at timestamptz NOT NULL, scene varchar(512), activity varchar(512), location varchar(512), status varchar(32) NOT NULL DEFAULT 'confirmed', revision integer NOT NULL DEFAULT 1, evidence_refs jsonb NOT NULL, idempotency_key varchar(256) NOT NULL, request_digest varchar(128) NOT NULL, result jsonb NOT NULL DEFAULT '{}', expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.life_schedules (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, local_date date NOT NULL, timezone varchar(128) NOT NULL, status varchar(32) NOT NULL DEFAULT 'proposed', generated_from varchar(128) NOT NULL, evidence_refs jsonb NOT NULL, previous_version_id varchar(128), revision integer NOT NULL DEFAULT 0, generated_at timestamptz NOT NULL DEFAULT now(), reschedule_policy jsonb NOT NULL DEFAULT '{}', idempotency_key varchar(256) NOT NULL, request_digest varchar(128) NOT NULL, result jsonb NOT NULL DEFAULT '{}', updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.life_schedule_items (id varchar(128) PRIMARY KEY, schedule_id varchar(128) NOT NULL, start_at timestamptz NOT NULL, end_at timestamptz NOT NULL, activity varchar(512) NOT NULL, scene varchar(512) NOT NULL, location varchar(512), item_type varchar(64) NOT NULL, status varchar(32) NOT NULL, priority varchar(32) NOT NULL, flexibility varchar(32) NOT NULL, interruption_cost varchar(32) NOT NULL);
CREATE TABLE IF NOT EXISTS public.life_context_commands (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, command_type varchar(64) NOT NULL, target_id varchar(128), idempotency_key varchar(256) NOT NULL, request_digest varchar(128) NOT NULL, result jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(fluctlight_id,idempotency_key));
CREATE TABLE IF NOT EXISTS public.life_presence (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, current_task varchar(512), user_presence varchar(128), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.autonomy_policies (fluctlight_id varchar(128) PRIMARY KEY, mode varchar(32) NOT NULL DEFAULT 'active', allowed_actions jsonb NOT NULL, budget_remaining varchar(64) NOT NULL, quiet_hours jsonb NOT NULL, cooldown_until timestamptz, concurrency_limit integer NOT NULL DEFAULT 1, revision integer NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.autonomy_actions (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_type varchar(64) NOT NULL, payload jsonb NOT NULL, policy_snapshot jsonb NOT NULL, expected_revisions jsonb NOT NULL, status varchar(32) NOT NULL DEFAULT 'frozen', workflow_id varchar(128) NOT NULL UNIQUE, provider_request_id varchar(128) NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now(), settled_at timestamptz, error_code varchar(128));
CREATE TABLE IF NOT EXISTS public.autonomy_governance (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, action_id varchar(128) NOT NULL, from_status varchar(32) NOT NULL, to_status varchar(32) NOT NULL, actor_id varchar(128) NOT NULL, reason text, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moments (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, author_actor_id varchar(128) NOT NULL, text text NOT NULL, visibility varchar(32) NOT NULL DEFAULT 'participants', status varchar(32) NOT NULL DEFAULT 'visible', media_asset_ids jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moment_comments (id varchar(128) PRIMARY KEY, moment_id varchar(128) NOT NULL, author_actor_id varchar(128) NOT NULL, text text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS public.moment_reactions (moment_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, kind varchar(32) NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(moment_id, actor_id));
CREATE TABLE IF NOT EXISTS public.moment_read_positions (owner_fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, last_seen_at timestamptz, PRIMARY KEY(owner_fluctlight_id, actor_id));
CREATE TABLE IF NOT EXISTS public.media_intents (id varchar(128) PRIMARY KEY, owner_fluctlight_id varchar(128) NOT NULL, kind varchar(32) NOT NULL, mime_type varchar(128) NOT NULL, prompt text NOT NULL, provider_prompt text NOT NULL DEFAULT '', provider_request_id varchar(128) NOT NULL UNIQUE, provider_job_id varchar(256) UNIQUE, workflow_id varchar(128) NOT NULL UNIQUE, conversation_id varchar(128), message_id varchar(128), moment_id varchar(128), status varchar(32) NOT NULL DEFAULT 'pending', quality_retry_count integer NOT NULL DEFAULT 0, quality_retry_guidance text NOT NULL DEFAULT '', quality_verdict varchar(16) NOT NULL DEFAULT '', quality_candidate_sha256 varchar(128), quality_checked_at timestamptz, revision integer NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now());
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
CREATE TABLE IF NOT EXISTS public.life_presence_overlays (id varchar(128) PRIMARY KEY, fluctlight_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, scene varchar(512), activity varchar(512), location varchar(512), current_task varchar(512), user_presence varchar(128), status varchar(32) NOT NULL DEFAULT 'active', revision integer NOT NULL DEFAULT 1, superseded_by_overlay_id varchar(128), idempotency_key varchar(256) NOT NULL, request_digest varchar(128) NOT NULL, result jsonb NOT NULL DEFAULT '{}', expires_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
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
ALTER TABLE public.relationships ADD COLUMN IF NOT EXISTS role jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.relationships ADD COLUMN IF NOT EXISTS profile_id varchar(128);
ALTER TABLE public.relationships ADD COLUMN IF NOT EXISTS provenance jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.relationship_revisions ADD COLUMN IF NOT EXISTS role jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS personality_perspectives jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS personality_perspectives jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS scope varchar(32) NOT NULL DEFAULT 'general';
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS target_actor_id varchar(128);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS profile_id varchar(128);
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS profile_id varchar(128);
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
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS provider_prompt text NOT NULL DEFAULT '';
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS quality_retry_count integer NOT NULL DEFAULT 0;
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS quality_retry_guidance text NOT NULL DEFAULT '';
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS quality_verdict varchar(16) NOT NULL DEFAULT '';
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS quality_candidate_sha256 varchar(128);
ALTER TABLE public.media_intents ADD COLUMN IF NOT EXISTS quality_checked_at timestamptz;
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS object_version varchar(256);
ALTER TABLE public.media_assets ADD COLUMN IF NOT EXISTS etag varchar(256);
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS previous_version_id varchar(128);
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS request_digest varchar(128);
CREATE UNIQUE INDEX IF NOT EXISTS uq_life_schedules_idempotency ON public.life_schedules(fluctlight_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS last_confirmed_at timestamptz;
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS expires_at timestamptz;
ALTER TABLE public.conversation_participants ADD COLUMN IF NOT EXISTS left_at timestamptz;
ALTER TABLE public.conversation_messages ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.conversation_read_positions ADD COLUMN IF NOT EXISTS last_delivered_sequence integer NOT NULL DEFAULT 0;
ALTER TABLE public.fluctlight_visual_identity_sessions ADD COLUMN IF NOT EXISTS character_sheet_media_intent_id varchar(128);
ALTER TABLE public.fluctlight_visual_identity_timeline ADD COLUMN IF NOT EXISTS stage_order integer NOT NULL DEFAULT 0;
UPDATE public.fluctlight_visual_identity_timeline SET stage_order = CASE stage
    WHEN 'session_created' THEN 10
    WHEN 'seed_requested' THEN 20
    WHEN 'seed_ready' THEN 30
    WHEN 'image_requested' THEN 40
    WHEN 'image_ready' THEN 50
    WHEN 'vision_requested' THEN 60
    WHEN 'vision_ready' THEN 70
    WHEN 'patch_requested' THEN 80
    WHEN 'patch_ready' THEN 90
    WHEN 'regenerate' THEN 100
    WHEN 'accepted' THEN 110
    WHEN 'character_sheet_requested' THEN 120
    WHEN 'character_sheet_ready' THEN 130
    WHEN 'completed' THEN 140
    WHEN 'failed' THEN 150
    ELSE 1000
END WHERE stage_order = 0;
CREATE UNIQUE INDEX IF NOT EXISTS uq_visual_identity_active_session ON public.fluctlight_visual_identity_sessions(fluctlight_id) WHERE status IN ('queued','running');
CREATE INDEX IF NOT EXISTS ix_visual_identity_timeline_fluctlight ON public.fluctlight_visual_identity_timeline(fluctlight_id,occurred_at,id);
CREATE INDEX IF NOT EXISTS ix_visual_identity_timeline_stage_order ON public.fluctlight_visual_identity_timeline(fluctlight_id,occurred_at,stage_order,id);
CREATE INDEX IF NOT EXISTS ix_visual_identity_attempts_session ON public.fluctlight_visual_identity_attempts(session_id,attempt_number);
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
CREATE INDEX IF NOT EXISTS ix_relationships_profile_target ON public.relationships(owner_fluctlight_id,profile_id,target_actor_id,updated_at DESC);
CREATE INDEX IF NOT EXISTS ix_fluctlight_goals_profile ON public.fluctlight_goals(fluctlight_id,profile_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS ix_fluctlight_intentions_profile ON public.fluctlight_intentions(fluctlight_id,profile_id,status,created_at DESC);
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
INSERT INTO public.runtime_settings(key,value_json) VALUES ('llm.queue','{"generated_concurrency":1,"embedding_concurrency":1}') ON CONFLICT (key) DO NOTHING;
UPDATE public.runtime_settings SET value_json='{"generated_concurrency":1,"embedding_concurrency":1}',updated_at=now() WHERE key='llm.queue' AND value_json='{"generated_concurrency":2,"embedding_concurrency":1}';
INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy)
SELECT 'generic_llm',provider_endpoint_id,model_id,token_budget,timeout_seconds,required_capabilities,retry_policy
FROM public.model_roles
WHERE role IN ('action_realization','cognitive_assessment','interaction','reflection','initialization','media_prompt')
ORDER BY CASE role WHEN 'action_realization' THEN 1 WHEN 'cognitive_assessment' THEN 2 WHEN 'interaction' THEN 3 WHEN 'reflection' THEN 4 WHEN 'initialization' THEN 5 ELSE 6 END
LIMIT 1 ON CONFLICT (role) DO NOTHING;
CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_consumer_inbox_group_event ON public.platform_consumer_inbox(consumer_group,event_id);
CREATE INDEX IF NOT EXISTS ix_platform_outbox_available ON public.platform_outbox_events(published_at,failed_at,available_at,claim_until,occurred_at);
`

const capabilityRuntimeMigrationSQL = `
CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_snapshot(
    projection jsonb,
    fluctlight_id text,
    conversation_id text,
    source_fact_id text,
    action_id text,
    owner_actor_id text
) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $fn$
SELECT CASE WHEN jsonb_typeof(projection) IS DISTINCT FROM 'object' THEN '{}'::jsonb ELSE
  jsonb_strip_nulls(jsonb_build_object(
    'identity', jsonb_strip_nulls(jsonb_build_object(
      'fluctlight_id', NULLIF(fluctlight_id,''), 'conversation_id', NULLIF(conversation_id,''),
      'source_fact_id', NULLIF(source_fact_id,''), 'action_id', NULLIF(action_id,'')
    )),
    'core_persona', projection->'core_persona',
    'current_state', COALESCE(projection->'inner_state', projection #> '{current_state,data,inner_state}'),
    'current_life', COALESCE(projection->'life_context', projection #> '{current_state,data,life_context}'),
    'schedule', projection->'schedule',
    'visual_identity', projection->'visual_identity',
    'appearance', projection #> '{identity,appearance}',
    'relationship_scope', jsonb_build_object(
      'authorized_actor_ids', COALESCE((
        SELECT jsonb_agg(to_jsonb(actor_id)) FROM (
          SELECT DISTINCT rel->>'target_actor_id' AS actor_id
          FROM jsonb_array_elements(CASE WHEN jsonb_typeof(projection->'relationships')='array' THEN projection->'relationships' ELSE '[]'::jsonb END) rel
          WHERE NULLIF(rel->>'target_actor_id','') IS NOT NULL
          UNION
          SELECT projection #>> '{current_speaker,actor_id}' WHERE NULLIF(projection #>> '{current_speaker,actor_id}','') IS NOT NULL
        ) authorized
      ), '[]'::jsonb),
      'relationships', CASE WHEN jsonb_typeof(projection->'relationships')='array' THEN projection->'relationships' ELSE '[]'::jsonb END
    ),
    'memory_scope', jsonb_build_object(
      'owner_actor_id', NULLIF(owner_actor_id,''),
      'memories', CASE WHEN jsonb_typeof(projection->'memories')='array' THEN projection->'memories' ELSE '[]'::jsonb END
    ),
    'agency', jsonb_build_object(
      'goals', CASE WHEN jsonb_typeof(projection->'goals')='array' THEN projection->'goals' ELSE '[]'::jsonb END,
      'intentions', CASE WHEN jsonb_typeof(projection->'intentions')='array' THEN projection->'intentions' ELSE '[]'::jsonb END
    )
  ))
END
$fn$;

CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_thin_arguments(call jsonb)
RETURNS jsonb LANGUAGE sql IMMUTABLE AS $fn$
SELECT CASE COALESCE(NULLIF(call->>'capability_name',''),call->>'name')
  WHEN 'conversation.reply' THEN jsonb_strip_nulls(jsonb_build_object('text',call #> '{arguments,text}'))
  WHEN 'moment.publish' THEN jsonb_strip_nulls(jsonb_build_object('text',call #> '{arguments,text}'))
  WHEN 'media.image.generate' THEN jsonb_strip_nulls(jsonb_build_object('intent',call #> '{arguments,intent}'))
  WHEN 'visual_identity.initialize' THEN '{}'::jsonb
  WHEN 'scene_event' THEN jsonb_strip_nulls(jsonb_build_object(
    'operation',call #> '{arguments,operation}','scene',call #> '{arguments,scene}',
    'activity',call #> '{arguments,activity}','location',call #> '{arguments,location}',
    'confidence',call #> '{arguments,confidence}'
  ))
  WHEN 'presence_event' THEN jsonb_strip_nulls(jsonb_build_object(
    'user_presence',call #> '{arguments,user_presence}','current_task',call #> '{arguments,current_task}',
    'expires_at',call #> '{arguments,expires_at}','confidence',call #> '{arguments,confidence}'
  ))
  WHEN 'schedule.replan' THEN jsonb_strip_nulls(jsonb_build_object('intent',call #> '{arguments,intent}'))
  WHEN 'memory_event' THEN jsonb_strip_nulls(jsonb_build_object(
    'content',call #> '{arguments,content}','type',call #> '{arguments,type}',
    'confidence',call #> '{arguments,confidence}','importance',call #> '{arguments,importance}',
    'emotional_significance',call #> '{arguments,emotional_significance}'
  ))
  WHEN 'affect_event' THEN jsonb_build_object('event',jsonb_strip_nulls(jsonb_build_object(
    'type',call #> '{arguments,event,type}','confidence',call #> '{arguments,event,confidence}'
  )))
  WHEN 'relationship.lookup' THEN jsonb_strip_nulls(jsonb_build_object('target_actor_id',call #> '{arguments,target_actor_id}'))
  WHEN 'capability.request' THEN jsonb_strip_nulls(jsonb_build_object(
    'capability_key',call #> '{arguments,capability_key}','title',call #> '{arguments,title}',
    'description',call #> '{arguments,description}','rationale',call #> '{arguments,rationale}',
    'desired_contract',call #> '{arguments,desired_contract}','priority',call #> '{arguments,priority}'
  ))
  ELSE call->'arguments'
END
$fn$;

CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_required_arguments_present(call jsonb)
RETURNS boolean LANGUAGE sql IMMUTABLE AS $fn$
SELECT CASE COALESCE(NULLIF(call->>'capability_name',''),call->>'name')
  WHEN 'conversation.reply' THEN jsonb_typeof(call #> '{arguments,text}')='string' AND NULLIF(btrim(call #>> '{arguments,text}'),'') IS NOT NULL
  WHEN 'moment.publish' THEN jsonb_typeof(call #> '{arguments,text}')='string' AND NULLIF(btrim(call #>> '{arguments,text}'),'') IS NOT NULL
  WHEN 'media.image.generate' THEN jsonb_typeof(call #> '{arguments,intent}')='string' AND NULLIF(btrim(call #>> '{arguments,intent}'),'') IS NOT NULL
  WHEN 'visual_identity.initialize' THEN true
  WHEN 'scene_event' THEN
    call #>> '{arguments,operation}' IN ('start','switch','end')
    AND CASE WHEN jsonb_typeof(call #> '{arguments,confidence}')='number' THEN (call #>> '{arguments,confidence}')::numeric BETWEEN 0 AND 1 ELSE false END
    AND (call #>> '{arguments,operation}'='end' OR (
      jsonb_typeof(call #> '{arguments,scene}')='string' AND NULLIF(btrim(call #>> '{arguments,scene}'),'') IS NOT NULL
      AND jsonb_typeof(call #> '{arguments,activity}')='string' AND NULLIF(btrim(call #>> '{arguments,activity}'),'') IS NOT NULL
    ))
  WHEN 'presence_event' THEN
    CASE WHEN jsonb_typeof(call #> '{arguments,confidence}')='number' THEN (call #>> '{arguments,confidence}')::numeric BETWEEN 0 AND 1 ELSE false END
    AND ((jsonb_typeof(call #> '{arguments,user_presence}')='string' AND NULLIF(btrim(call #>> '{arguments,user_presence}'),'') IS NOT NULL)
      OR (jsonb_typeof(call #> '{arguments,current_task}')='string' AND NULLIF(btrim(call #>> '{arguments,current_task}'),'') IS NOT NULL))
  WHEN 'schedule.replan' THEN jsonb_typeof(call #> '{arguments,intent}')='string' AND NULLIF(btrim(call #>> '{arguments,intent}'),'') IS NOT NULL
  WHEN 'memory_event' THEN
    jsonb_typeof(call #> '{arguments,content}')='string' AND NULLIF(btrim(call #>> '{arguments,content}'),'') IS NOT NULL
    AND call #>> '{arguments,type}' IN ('working','episodic','semantic','relationship','autobiographical')
    AND CASE WHEN jsonb_typeof(call #> '{arguments,confidence}')='number' THEN (call #>> '{arguments,confidence}')::numeric BETWEEN 0 AND 1 ELSE false END
    AND CASE WHEN jsonb_typeof(call #> '{arguments,importance}')='number' THEN (call #>> '{arguments,importance}')::numeric BETWEEN 0 AND 1 ELSE false END
  WHEN 'affect_event' THEN
    jsonb_typeof(call #> '{arguments,event}')='object'
    AND jsonb_typeof(call #> '{arguments,event,type}')='string' AND NULLIF(btrim(call #>> '{arguments,event,type}'),'') IS NOT NULL
    AND CASE WHEN jsonb_typeof(call #> '{arguments,event,confidence}')='number' THEN (call #>> '{arguments,event,confidence}')::numeric BETWEEN 0 AND 1 ELSE false END
  WHEN 'relationship.lookup' THEN jsonb_typeof(call #> '{arguments,target_actor_id}')='string' AND NULLIF(btrim(call #>> '{arguments,target_actor_id}'),'') IS NOT NULL
  WHEN 'capability.request' THEN
    jsonb_typeof(call #> '{arguments,capability_key}')='string' AND NULLIF(btrim(call #>> '{arguments,capability_key}'),'') IS NOT NULL
    AND jsonb_typeof(call #> '{arguments,title}')='string' AND NULLIF(btrim(call #>> '{arguments,title}'),'') IS NOT NULL
    AND jsonb_typeof(call #> '{arguments,description}')='string' AND NULLIF(btrim(call #>> '{arguments,description}'),'') IS NOT NULL
    AND jsonb_typeof(call #> '{arguments,rationale}')='string' AND NULLIF(btrim(call #>> '{arguments,rationale}'),'') IS NOT NULL
  ELSE false
END
$fn$;

CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_prepared_payload(call jsonb, source_fact_id text, call_id text)
RETURNS jsonb LANGUAGE sql IMMUTABLE AS $fn$
SELECT jsonb_build_object(
  'schema_version','fluctlight.capability-prepared.v1',
  'data', COALESCE(call #> '{prepared_payload,data}', CASE
    WHEN COALESCE(NULLIF(call->>'capability_name',''),call->>'name')='media.image.generate' AND call->'arguments' ? 'prepared_concept'
      THEN jsonb_build_object('media_concept',call->'arguments'->'prepared_concept')
    WHEN COALESCE(NULLIF(call->>'capability_name',''),call->>'name')='schedule.replan' AND call->'arguments' ? 'items'
      THEN jsonb_build_object('schedule_plan',call->'arguments')
    ELSE '{}'::jsonb END),
  'provenance', COALESCE(call #> '{prepared_payload,provenance}','{}'::jsonb) || jsonb_strip_nulls(jsonb_build_object(
    'evidence_refs', COALESCE(call #> '{arguments,evidence_refs}',call #> '{arguments,event,evidence_refs}',CASE WHEN NULLIF(source_fact_id,'') IS NULL THEN NULL ELSE jsonb_build_array(source_fact_id) END),
    'idempotency_key', COALESCE(call #> '{arguments,idempotency_key}',call #> '{arguments,event,idempotency_key}',CASE WHEN NULLIF(call_id,'') IS NULL THEN NULL ELSE to_jsonb('capability:'||call_id) END)
  ))
)
$fn$;

CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_context_complete(call jsonb)
RETURNS boolean LANGUAGE sql IMMUTABLE AS $fn$
SELECT CASE COALESCE(NULLIF(call->>'capability_name',''),call->>'name')
  WHEN 'media.image.generate' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','visual_identity','current_life','appearance','current_state']
  WHEN 'visual_identity.initialize' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','core_persona','visual_identity']
  WHEN 'scene_event' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','current_life','schedule']
  WHEN 'presence_event' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','current_life']
  WHEN 'schedule.replan' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','schedule','current_life','agency']
  WHEN 'memory_event' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','core_persona','memory_scope']
  WHEN 'affect_event' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','current_state']
  WHEN 'relationship.lookup' THEN jsonb_typeof(call->'context_snapshot')='object' AND call->'context_snapshot' ?& ARRAY['identity','relationship_scope']
  ELSE true
END
$fn$;

CREATE OR REPLACE FUNCTION pg_temp.fluctlight_capability_prepared_complete(call jsonb)
RETURNS boolean LANGUAGE sql IMMUTABLE AS $fn$
SELECT CASE COALESCE(NULLIF(call->>'capability_name',''),call->>'name')
  WHEN 'media.image.generate' THEN CASE
    WHEN call #> '{prepared_payload,data,media_concept}' IS NULL THEN true
    ELSE jsonb_typeof(call #> '{prepared_payload,data,media_concept}')='object'
      AND call #>> '{prepared_payload,data,media_concept,intent}' = call #>> '{arguments,intent}'
      AND jsonb_typeof(call #> '{prepared_payload,data,media_concept,context_binding}')='object'
      AND (call #> '{prepared_payload,data,media_concept,context_binding}') ?& ARRAY['visual_identity','current_life','appearance','current_state']
    END
  WHEN 'schedule.replan' THEN CASE
    WHEN call #> '{prepared_payload,data,schedule_plan}' IS NULL THEN true
    ELSE jsonb_typeof(call #> '{prepared_payload,data,schedule_plan}')='object'
      AND call #>> '{prepared_payload,data,schedule_plan,intent}' = call #>> '{arguments,intent}'
      AND jsonb_typeof(call #> '{prepared_payload,data,schedule_plan,items}')='array'
      AND jsonb_array_length(call #> '{prepared_payload,data,schedule_plan,items}') > 0
      AND jsonb_typeof(call #> '{prepared_payload,data,schedule_plan,reschedule_policy}')='object'
      AND NULLIF(call #>> '{prepared_payload,data,schedule_plan,local_date}','') IS NOT NULL
      AND NULLIF(call #>> '{prepared_payload,data,schedule_plan,timezone}','') IS NOT NULL
      AND NULLIF(call #>> '{prepared_payload,data,schedule_plan,completed_before}','') IS NOT NULL
      AND jsonb_typeof(call #> '{prepared_payload,data,schedule_plan,expected_revision}')='number'
    END
  ELSE true
END
$fn$;

-- Capability Runtime cutover. Active rows are preflighted and converted in
-- this transaction; any malformed payload aborts the migration instead of
-- silently remaining executable under an unknown shape. Completed/failed
-- audit rows are intentionally untouched.
DO $$
DECLARE malformed bigint;
BEGIN
    SELECT count(*) INTO malformed
    FROM public.cognition_frozen_actions f
	    WHERE f.status IN ('frozen','pending','claimed','started','running')
		      AND (
		        (f.payload ? 'capability_runtime_version' AND f.payload->>'capability_runtime_version' <> 'v2')
		        OR (f.payload->>'capability_runtime_version' = 'v2' AND (jsonb_typeof(f.payload->'capability_invocations') IS DISTINCT FROM 'array' OR jsonb_typeof(f.payload->'capability_results') IS DISTINCT FROM 'array'))
		        OR
		        (f.payload ? 'capability_runtime_version' AND (f.payload ? 'tool_calls' OR (f.payload->'decision') ? 'tool_calls'))
	        OR
	        (f.payload ? 'capability_invocations' AND f.payload ? 'tool_calls')
	        OR (f.payload ? 'capability_invocations' AND jsonb_typeof(f.payload #> '{decision,tool_calls}') = 'array')
	        OR (f.payload ? 'capability_invocations' AND jsonb_typeof(f.payload->'capability_invocations') <> 'array')
		        OR (f.payload ? 'tool_calls' AND jsonb_typeof(f.payload->'tool_calls') <> 'array')
	        OR (f.payload ? 'tool_results' AND jsonb_typeof(f.payload->'tool_results') <> 'array')
        OR (jsonb_typeof(f.payload #> '{decision,tool_calls}') IS NOT NULL AND jsonb_typeof(f.payload #> '{decision,tool_calls}') <> 'array')
			OR EXISTS (
          SELECT 1 FROM jsonb_array_elements(
            CASE WHEN jsonb_typeof(f.payload->'capability_invocations')='array' THEN f.payload->'capability_invocations'
                 WHEN jsonb_typeof(f.payload #> '{decision,tool_calls}')='array' THEN f.payload #> '{decision,tool_calls}'
                 WHEN jsonb_typeof(f.payload->'tool_calls')='array' THEN f.payload->'tool_calls' ELSE '[]'::jsonb END
          ) AS call
	          WHERE COALESCE(NULLIF(call->>'call_id',''),NULLIF(call->>'id','')) IS NULL
	             OR COALESCE(NULLIF(call->>'capability_name',''),NULLIF(call->>'name','')) IS NULL
		       OR COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(f.payload->>'source_fact_id',''),f.inbox_id) IS NULL
		       OR COALESCE(NULLIF(call->>'provider_request_id',''),NULLIF(f.payload->>'provider_request_id',''),f.provider_request_id) IS NULL
	             OR jsonb_typeof(call->'arguments') IS DISTINCT FROM 'object'
	             OR NOT pg_temp.fluctlight_capability_required_arguments_present(call)
	             OR ((COALESCE(NULLIF(call->>'capability_name',''),call->>'name') IN ('media.image.generate','schedule.replan')) AND NOT (call->'arguments' ? 'intent'))
	             OR (call ? 'prepared_payload' AND (jsonb_typeof(call->'prepared_payload') IS DISTINCT FROM 'object' OR call #>> '{prepared_payload,schema_version}' <> 'fluctlight.capability-prepared.v1'))
        )
		OR EXISTS (
		  SELECT 1
		  FROM jsonb_array_elements(CASE WHEN jsonb_typeof(f.payload->'capability_invocations')='array' THEN f.payload->'capability_invocations' WHEN jsonb_typeof(f.payload #> '{decision,tool_calls}')='array' THEN f.payload #> '{decision,tool_calls}' ELSE COALESCE(f.payload->'tool_calls','[]'::jsonb) END) AS duplicate_call
		  GROUP BY COALESCE(NULLIF(duplicate_call->>'call_id',''),NULLIF(duplicate_call->>'id',''))
			  HAVING count(*) > 1
			)
		OR EXISTS (
		  SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(f.payload->'tool_results')='array' THEN f.payload->'tool_results' ELSE '[]'::jsonb END) AS result
		  WHERE COALESCE(NULLIF(result->>'call_id',''),NULLIF(result->>'tool_call_id','')) IS NULL
		     OR COALESCE(NULLIF(result->>'capability_name',''),NULLIF(result->>'name','')) IS NULL
		     OR COALESCE(NULLIF(result->>'status',''),'') NOT IN ('completed','failed','rejected','deferred')
		     OR (result ? 'retryable' AND jsonb_typeof(result->'retryable') IS DISTINCT FROM 'boolean')
		)
	      );
    IF malformed > 0 THEN
        RAISE EXCEPTION 'capability runtime migration found % malformed active frozen payload(s)', malformed;
    END IF;

    SELECT count(*) INTO malformed
    FROM public.autonomy_actions a
	    WHERE a.status IN ('frozen','pending','claimed','started','running')
		      AND (
		        (a.payload ? 'capability_runtime_version' AND a.payload->>'capability_runtime_version' <> 'v2')
		        OR (a.payload->>'capability_runtime_version' = 'v2' AND (jsonb_typeof(a.payload->'capability_invocations') IS DISTINCT FROM 'array' OR jsonb_typeof(a.payload->'capability_results') IS DISTINCT FROM 'array'))
		        OR
		        (a.payload ? 'capability_runtime_version' AND (a.payload ? 'tool_calls' OR (a.payload->'decision') ? 'tool_calls'))
	        OR
	        (a.payload ? 'capability_invocations' AND a.payload ? 'tool_calls')
	        OR (a.payload ? 'capability_invocations' AND jsonb_typeof(a.payload->'capability_invocations') <> 'array')
	        OR (a.payload ? 'tool_calls' AND jsonb_typeof(a.payload->'tool_calls') <> 'array')
	        OR (a.payload ? 'tool_results' AND jsonb_typeof(a.payload->'tool_results') <> 'array')
			OR EXISTS (
          SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'capability_invocations')='array' THEN a.payload->'capability_invocations' ELSE COALESCE(a.payload->'tool_calls','[]'::jsonb) END) AS call
	          WHERE COALESCE(NULLIF(call->>'call_id',''),NULLIF(call->>'id','')) IS NULL
             OR COALESCE(NULLIF(call->>'capability_name',''),NULLIF(call->>'name','')) IS NULL
		       OR COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(a.payload->>'source_fact_id','')) IS NULL
		       OR COALESCE(NULLIF(call->>'provider_request_id',''),NULLIF(a.payload->>'provider_request_id',''),a.provider_request_id) IS NULL
	             OR jsonb_typeof(call->'arguments') IS DISTINCT FROM 'object'
	             OR NOT pg_temp.fluctlight_capability_required_arguments_present(call)
	             OR ((COALESCE(NULLIF(call->>'capability_name',''),call->>'name') IN ('media.image.generate','schedule.replan')) AND NOT (call->'arguments' ? 'intent'))
	             OR (call ? 'prepared_payload' AND (jsonb_typeof(call->'prepared_payload') IS DISTINCT FROM 'object' OR call #>> '{prepared_payload,schema_version}' <> 'fluctlight.capability-prepared.v1'))
        )
		OR EXISTS (
		  SELECT 1
		  FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'capability_invocations')='array' THEN a.payload->'capability_invocations' ELSE COALESCE(a.payload->'tool_calls','[]'::jsonb) END) AS duplicate_call
		  GROUP BY COALESCE(NULLIF(duplicate_call->>'call_id',''),NULLIF(duplicate_call->>'id',''))
			  HAVING count(*) > 1
			)
		OR EXISTS (
		  SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'tool_results')='array' THEN a.payload->'tool_results' ELSE '[]'::jsonb END) AS result
		  WHERE COALESCE(NULLIF(result->>'call_id',''),NULLIF(result->>'tool_call_id','')) IS NULL
		     OR COALESCE(NULLIF(result->>'capability_name',''),NULLIF(result->>'name','')) IS NULL
		     OR COALESCE(NULLIF(result->>'status',''),'') NOT IN ('completed','failed','rejected','deferred')
		     OR (result ? 'retryable' AND jsonb_typeof(result->'retryable') IS DISTINCT FROM 'boolean')
		)
	      );
    IF malformed > 0 THEN
        RAISE EXCEPTION 'capability runtime migration found % malformed active autonomy payload(s)', malformed;
    END IF;
END $$;

UPDATE public.cognition_frozen_actions f
SET payload = jsonb_set(
    jsonb_set((f.payload - 'tool_calls' - 'tool_results'), '{decision}', COALESCE((f.payload->'decision') - 'tool_calls' - 'context_projection' - 'media_concept' - 'visual_concept' - 'media_request', '{}'::jsonb), true),
    '{capability_runtime_version}',
    '"v2"'::jsonb,
    true
) || jsonb_build_object(
    'capability_invocations', COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'call_id', COALESCE(NULLIF(call->>'call_id',''),call->>'id'),
        'capability_name', COALESCE(NULLIF(call->>'capability_name',''),call->>'name'),
		'arguments', pg_temp.fluctlight_capability_thin_arguments(call),
		'prepared_payload', pg_temp.fluctlight_capability_prepared_payload(call,COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(f.payload->>'source_fact_id',''),f.inbox_id),COALESCE(NULLIF(call->>'call_id',''),call->>'id')),
        'source_fact_id', COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(f.payload->>'source_fact_id',''),f.inbox_id),
        'action_id', NULLIF(call->>'action_id',''),
        'provider_request_id', COALESCE(NULLIF(call->>'provider_request_id',''),NULLIF(f.payload->>'provider_request_id',''),f.provider_request_id),
        'sequence', COALESCE(NULLIF(call->>'sequence','')::integer, ord::integer-1),
        'schema_version', 'fluctlight.capability-invocation.v2',
        'metadata', jsonb_build_object('surface', COALESCE(NULLIF(call->'metadata'->>'surface',''), CASE WHEN f.payload ? 'decision' THEN 'conversation' ELSE 'autonomy' END), 'fluctlight_id', f.fluctlight_id, 'conversation_id', COALESCE(f.payload->>'conversation_id','')),
	        'context_snapshot', COALESCE(
	          CASE WHEN jsonb_typeof(call->'context_snapshot')='object' THEN CASE WHEN call->'context_snapshot' ? 'identity' THEN call->'context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	            call->'context_snapshot', f.fluctlight_id, COALESCE(f.payload->>'conversation_id',''),
	            COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(f.payload->>'source_fact_id',''),f.inbox_id), COALESCE(NULLIF(call->>'action_id',''),f.id),
	            COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=f.fluctlight_id),'')
	          ) END END,
	          CASE WHEN jsonb_typeof(f.payload->'capability_context_snapshot')='object' THEN CASE WHEN f.payload->'capability_context_snapshot' ? 'identity' THEN f.payload->'capability_context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	            f.payload->'capability_context_snapshot', f.fluctlight_id, COALESCE(f.payload->>'conversation_id',''), f.inbox_id, f.id,
	            COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=f.fluctlight_id),'')
	          ) END END,
	          pg_temp.fluctlight_capability_snapshot(
	          f.payload #> '{decision,context_projection}', f.fluctlight_id, COALESCE(f.payload->>'conversation_id',''),
	          COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(f.payload->>'source_fact_id',''),f.inbox_id),
	          COALESCE(NULLIF(call->>'action_id',''),f.id),
	          COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=f.fluctlight_id),'')
	        ))
    ))) FROM jsonb_array_elements(CASE WHEN jsonb_typeof(f.payload->'capability_invocations')='array' THEN f.payload->'capability_invocations' WHEN jsonb_typeof(f.payload #> '{decision,tool_calls}')='array' THEN f.payload #> '{decision,tool_calls}' ELSE COALESCE(f.payload->'tool_calls','[]'::jsonb) END) WITH ORDINALITY AS e(call,ord)), '[]'::jsonb),
	    'capability_context_snapshot', COALESCE(
	      CASE WHEN jsonb_typeof(f.payload->'capability_context_snapshot')='object' THEN CASE WHEN f.payload->'capability_context_snapshot' ? 'identity' THEN f.payload->'capability_context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	        f.payload->'capability_context_snapshot', f.fluctlight_id, COALESCE(f.payload->>'conversation_id',''), f.inbox_id, f.id,
	        COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=f.fluctlight_id),'')
	      ) END END,
	      pg_temp.fluctlight_capability_snapshot(
	      f.payload #> '{decision,context_projection}', f.fluctlight_id, COALESCE(f.payload->>'conversation_id',''), f.inbox_id, f.id,
	      COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=f.fluctlight_id),'')
	    ), '{}'::jsonb),
	    'capability_results', CASE
	      WHEN jsonb_typeof(f.payload->'capability_results')='array' THEN f.payload->'capability_results'
	      WHEN jsonb_typeof(f.payload->'tool_results')='array' THEN COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
	        'call_id', COALESCE(NULLIF(result->>'call_id',''),result->>'tool_call_id'),
	        'capability_name', COALESCE(NULLIF(result->>'capability_name',''),result->>'name'),
	        'status', result->>'status', 'output', result->'output', 'error_code', NULLIF(result->>'error_code',''),
	        'retryable', CASE WHEN jsonb_typeof(result->'retryable')='boolean' THEN (result->>'retryable')::boolean ELSE false END,
	        'provider_request_id', NULLIF(result->>'provider_request_id',''), 'correlation_id', NULLIF(result->>'correlation_id','')
	      ))) FROM jsonb_array_elements(f.payload->'tool_results') AS result), '[]'::jsonb)
	      ELSE '[]'::jsonb END
)
WHERE f.status IN ('frozen','pending','claimed','started','running')
	  AND (
	    NOT (f.payload ? 'capability_runtime_version')
	    OR (jsonb_typeof(f.payload->'capability_context_snapshot')='object' AND NOT (f.payload->'capability_context_snapshot' ? 'identity'))
	    OR EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(f.payload->'capability_invocations')='array' THEN f.payload->'capability_invocations' ELSE '[]'::jsonb END) call
	      WHERE pg_temp.fluctlight_capability_thin_arguments(call) IS DISTINCT FROM call->'arguments'
	         OR (jsonb_typeof(call->'context_snapshot')='object' AND NOT (call->'context_snapshot' ? 'identity')))
	  );

UPDATE public.autonomy_actions a
SET payload = jsonb_set((a.payload - 'tool_calls' - 'tool_results'), '{decision}', COALESCE((a.payload->'decision') - 'tool_calls' - 'context_projection' - 'media_concept' - 'visual_concept' - 'media_request', '{}'::jsonb), true) || jsonb_build_object(
    'capability_runtime_version','v2',
    'capability_invocations', COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
        'call_id', COALESCE(NULLIF(call->>'call_id',''),call->>'id'),
        'capability_name', COALESCE(NULLIF(call->>'capability_name',''),call->>'name'),
		'arguments', pg_temp.fluctlight_capability_thin_arguments(call),
		'prepared_payload', pg_temp.fluctlight_capability_prepared_payload(call,COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(a.payload->>'source_fact_id','')),COALESCE(NULLIF(call->>'call_id',''),call->>'id')),
        'source_fact_id', COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(a.payload->>'source_fact_id','')),
        'action_id', NULLIF(call->>'action_id',''),
        'provider_request_id', COALESCE(NULLIF(call->>'provider_request_id',''),NULLIF(a.payload->>'provider_request_id',''),a.provider_request_id),
        'sequence', COALESCE(NULLIF(call->>'sequence','')::integer, ord::integer-1),
        'schema_version', 'fluctlight.capability-invocation.v2',
        'metadata', jsonb_build_object('surface', COALESCE(NULLIF(call->'metadata'->>'surface',''),'autonomy'), 'fluctlight_id', a.fluctlight_id, 'conversation_id', COALESCE(a.payload->>'conversation_id','')),
	        'context_snapshot', COALESCE(
	          CASE WHEN jsonb_typeof(call->'context_snapshot')='object' THEN CASE WHEN call->'context_snapshot' ? 'identity' THEN call->'context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	            call->'context_snapshot', a.fluctlight_id, COALESCE(a.payload->>'conversation_id',''),
	            COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(a.payload->>'source_fact_id','')), COALESCE(NULLIF(call->>'action_id',''),a.id),
	            COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=a.fluctlight_id),'')
	          ) END END,
	          CASE WHEN jsonb_typeof(a.payload->'capability_context_snapshot')='object' THEN CASE WHEN a.payload->'capability_context_snapshot' ? 'identity' THEN a.payload->'capability_context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	            a.payload->'capability_context_snapshot', a.fluctlight_id, COALESCE(a.payload->>'conversation_id',''), COALESCE(a.payload->>'source_fact_id',''), a.id,
	            COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=a.fluctlight_id),'')
	          ) END END,
	          pg_temp.fluctlight_capability_snapshot(
	          a.payload->'context_projection', a.fluctlight_id, COALESCE(a.payload->>'conversation_id',''),
	          COALESCE(NULLIF(call->>'source_fact_id',''),NULLIF(a.payload->>'source_fact_id','')),
	          COALESCE(NULLIF(call->>'action_id',''),a.id),
	          COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=a.fluctlight_id),'')
	        ))
    ))) FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'capability_invocations')='array' THEN a.payload->'capability_invocations' ELSE COALESCE(a.payload->'tool_calls','[]'::jsonb) END) WITH ORDINALITY AS e(call,ord)), '[]'::jsonb),
	    'capability_context_snapshot', COALESCE(
	      CASE WHEN jsonb_typeof(a.payload->'capability_context_snapshot')='object' THEN CASE WHEN a.payload->'capability_context_snapshot' ? 'identity' THEN a.payload->'capability_context_snapshot' ELSE pg_temp.fluctlight_capability_snapshot(
	        a.payload->'capability_context_snapshot', a.fluctlight_id, COALESCE(a.payload->>'conversation_id',''), COALESCE(a.payload->>'source_fact_id',''), a.id,
	        COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=a.fluctlight_id),'')
	      ) END END,
	      pg_temp.fluctlight_capability_snapshot(
	      a.payload->'context_projection', a.fluctlight_id, COALESCE(a.payload->>'conversation_id',''), COALESCE(a.payload->>'source_fact_id',''), a.id,
	      COALESCE((SELECT created_by_actor_id FROM public.fluctlights WHERE id=a.fluctlight_id),'')
	    ), '{}'::jsonb),
	    'capability_results', CASE
	      WHEN jsonb_typeof(a.payload->'capability_results')='array' THEN a.payload->'capability_results'
	      WHEN jsonb_typeof(a.payload->'tool_results')='array' THEN COALESCE((SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
	        'call_id', COALESCE(NULLIF(result->>'call_id',''),result->>'tool_call_id'),
	        'capability_name', COALESCE(NULLIF(result->>'capability_name',''),result->>'name'),
	        'status', result->>'status', 'output', result->'output', 'error_code', NULLIF(result->>'error_code',''),
	        'retryable', CASE WHEN jsonb_typeof(result->'retryable')='boolean' THEN (result->>'retryable')::boolean ELSE false END,
	        'provider_request_id', NULLIF(result->>'provider_request_id',''), 'correlation_id', NULLIF(result->>'correlation_id','')
	      ))) FROM jsonb_array_elements(a.payload->'tool_results') AS result), '[]'::jsonb)
	      ELSE '[]'::jsonb END
)
WHERE a.status IN ('frozen','pending','claimed','started','running')
	  AND (
	    NOT (a.payload ? 'capability_runtime_version')
	    OR (jsonb_typeof(a.payload->'capability_context_snapshot')='object' AND NOT (a.payload->'capability_context_snapshot' ? 'identity'))
	    OR EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'capability_invocations')='array' THEN a.payload->'capability_invocations' ELSE '[]'::jsonb END) call
	      WHERE pg_temp.fluctlight_capability_thin_arguments(call) IS DISTINCT FROM call->'arguments'
	         OR (jsonb_typeof(call->'context_snapshot')='object' AND NOT (call->'context_snapshot' ? 'identity')))
	  );

DO $$
DECLARE malformed bigint;
BEGIN
  SELECT count(*) INTO malformed FROM (
    SELECT f.id
    FROM public.cognition_frozen_actions f
    WHERE f.status IN ('frozen','pending','claimed','started','running')
      AND (
        f.payload->>'capability_runtime_version' <> 'v2'
        OR jsonb_typeof(f.payload->'capability_invocations') IS DISTINCT FROM 'array'
        OR jsonb_typeof(f.payload->'capability_results') IS DISTINCT FROM 'array'
	        OR EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(f.payload->'capability_invocations')='array' THEN f.payload->'capability_invocations' ELSE '[]'::jsonb END) call
	          WHERE NOT pg_temp.fluctlight_capability_required_arguments_present(call)
	             OR NOT pg_temp.fluctlight_capability_context_complete(call)
	             OR NOT pg_temp.fluctlight_capability_prepared_complete(call))
      )
    UNION ALL
    SELECT a.id
    FROM public.autonomy_actions a
    WHERE a.status IN ('frozen','pending','claimed','started','running')
      AND (
        a.payload->>'capability_runtime_version' <> 'v2'
        OR jsonb_typeof(a.payload->'capability_invocations') IS DISTINCT FROM 'array'
        OR jsonb_typeof(a.payload->'capability_results') IS DISTINCT FROM 'array'
	        OR EXISTS (SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.payload->'capability_invocations')='array' THEN a.payload->'capability_invocations' ELSE '[]'::jsonb END) call
	          WHERE NOT pg_temp.fluctlight_capability_required_arguments_present(call)
	             OR NOT pg_temp.fluctlight_capability_context_complete(call)
	             OR NOT pg_temp.fluctlight_capability_prepared_complete(call))
      )
  ) invalid_active;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'capability runtime migration left % invalid active payload(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.platform_workflow_intents i
  LEFT JOIN public.autonomy_actions a ON a.id=i.payload->>'action_id'
  WHERE i.status IN ('pending','retry','started','running','cancel_requested')
    AND i.intent_type IN ('autonomy.action','capability.action')
    AND (a.id IS NULL OR a.payload->>'capability_runtime_version' <> 'v2');
  IF malformed > 0 THEN
    RAISE EXCEPTION 'capability runtime migration found % active workflow intent(s) without v2 action authority', malformed;
  END IF;
END $$;

`

// projectHealthEvolutionMigrationSQL is a separate cutover after the direct
// Capability Runtime. Causality cannot be reconstructed from historical prose,
// so executable pre-cutover actions must be drained instead of receiving
// fabricated empty influence metadata. Completed audit history remains in its
// original shape and is never re-executed.
const projectHealthEvolutionMigrationSQL = `
DO $$
DECLARE malformed bigint;
BEGIN
  SELECT count(*) INTO malformed FROM (
    SELECT f.id
    FROM public.cognition_frozen_actions f
    WHERE f.status IN ('frozen','pending','claimed','started','running')
      AND (
        f.payload->>'context_reference_version' <> 'fluctlight.context-reference-index.v1'
        OR jsonb_typeof(f.payload->'context_reference_index') IS DISTINCT FROM 'object'
        OR jsonb_typeof(f.payload->'context_reference_index'->'by_ref') IS DISTINCT FROM 'object'
        OR jsonb_typeof(f.payload->'influences') IS DISTINCT FROM 'array'
        OR jsonb_typeof(f.payload->'goal_refs') IS DISTINCT FROM 'array'
        OR jsonb_typeof(f.payload->'intention_refs') IS DISTINCT FROM 'array'
      )
    UNION ALL
    SELECT a.id
    FROM public.autonomy_actions a
    WHERE a.status IN ('frozen','pending','claimed','started','running')
      AND (
        a.payload->>'context_reference_version' <> 'fluctlight.context-reference-index.v1'
        OR jsonb_typeof(a.payload->'context_reference_index') IS DISTINCT FROM 'object'
        OR jsonb_typeof(a.payload->'context_reference_index'->'by_ref') IS DISTINCT FROM 'object'
        OR jsonb_typeof(a.payload->'influences') IS DISTINCT FROM 'array'
        OR jsonb_typeof(a.payload->'goal_refs') IS DISTINCT FROM 'array'
        OR jsonb_typeof(a.payload->'intention_refs') IS DISTINCT FROM 'array'
      )
  ) invalid_active;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'project health evolution cutover requires draining % active pre-reference action(s)', malformed;
  END IF;
END $$;

DO $$
DECLARE malformed bigint;
BEGIN
  SELECT count(*) INTO malformed
  FROM public.fluctlight_inner_states s
  WHERE jsonb_typeof(s.pad->'pleasure') IS DISTINCT FROM 'number'
     OR jsonb_typeof(s.pad->'arousal') IS DISTINCT FROM 'number'
     OR jsonb_typeof(s.pad->'dominance') IS DISTINCT FROM 'number'
     OR (s.pad->>'pleasure')::double precision NOT BETWEEN -1 AND 1
     OR (s.pad->>'arousal')::double precision NOT BETWEEN -1 AND 1
     OR (s.pad->>'dominance')::double precision NOT BETWEEN -1 AND 1
     OR (s.mood ? 'intensity' AND (jsonb_typeof(s.mood->'intensity') IS DISTINCT FROM 'number' OR (s.mood->>'intensity')::double precision NOT BETWEEN 0 AND 1))
     OR (s.momentum ? 'value' AND (jsonb_typeof(s.momentum->'value') IS DISTINCT FROM 'number' OR (s.momentum->>'value')::double precision NOT BETWEEN -1 AND 1))
     OR (s.regulation ? 'stress' AND (jsonb_typeof(s.regulation->'stress') IS DISTINCT FROM 'number' OR (s.regulation->>'stress')::double precision NOT BETWEEN 0 AND 1))
     OR (s.regulation ? 'stability' AND (jsonb_typeof(s.regulation->'stability') IS DISTINCT FROM 'number' OR (s.regulation->>'stability')::double precision NOT BETWEEN 0 AND 1));
  IF malformed > 0 THEN
    RAISE EXCEPTION 'project health affect reconciliation found % malformed state row(s)', malformed;
  END IF;
END $$;

INSERT INTO public.fluctlight_affect_profiles(fluctlight_id)
SELECT f.id FROM public.fluctlights f
ON CONFLICT(fluctlight_id) DO NOTHING;

INSERT INTO public.fluctlight_affect_reconciliations(id,fluctlight_id,source_head,mapping_policy,before_state,after_state,disposition)
SELECT 'affect_reconcile_' || substr(encode(digest(s.fluctlight_id || ':0027_project_health_evolution','sha256'),'hex'),1,32),
       s.fluctlight_id,
       '0026_capability_runtime',
       'preserve_numeric_subset_no_semantic_inference',
       jsonb_build_object('pad',s.pad,'mood',s.mood,'momentum',s.momentum,'regulation',s.regulation),
       jsonb_build_object('pad',s.pad,'mood',s.mood,'momentum',s.momentum,'regulation',s.regulation),
       'preserved'
FROM public.fluctlight_inner_states s
ON CONFLICT(fluctlight_id,source_head) DO NOTHING;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_inner_state_affect_ranges') THEN
    ALTER TABLE public.fluctlight_inner_states ADD CONSTRAINT ck_fluctlight_inner_state_affect_ranges CHECK (
      jsonb_typeof(pad->'pleasure')='number' AND (pad->>'pleasure')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(pad->'arousal')='number' AND (pad->>'arousal')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(pad->'dominance')='number' AND (pad->>'dominance')::double precision BETWEEN -1 AND 1
      AND (NOT (mood ? 'intensity') OR (jsonb_typeof(mood->'intensity')='number' AND (mood->>'intensity')::double precision BETWEEN 0 AND 1))
      AND (NOT (momentum ? 'value') OR (jsonb_typeof(momentum->'value')='number' AND (momentum->>'value')::double precision BETWEEN -1 AND 1))
      AND (NOT (regulation ? 'stress') OR (jsonb_typeof(regulation->'stress')='number' AND (regulation->>'stress')::double precision BETWEEN 0 AND 1))
      AND (NOT (regulation ? 'stability') OR (jsonb_typeof(regulation->'stability')='number' AND (regulation->>'stability')::double precision BETWEEN 0 AND 1))
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_affect_profile_baseline') THEN
    ALTER TABLE public.fluctlight_affect_profiles ADD CONSTRAINT ck_fluctlight_affect_profile_baseline CHECK (
      jsonb_typeof(baseline_pad->'pleasure')='number' AND (baseline_pad->>'pleasure')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(baseline_pad->'arousal')='number' AND (baseline_pad->>'arousal')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(baseline_pad->'dominance')='number' AND (baseline_pad->>'dominance')::double precision BETWEEN -1 AND 1
    );
  END IF;
END $$;
`

// memoryLifecycleMigrationSQL adds the V2 authority without rewriting legacy
// Memory facts or rebuildable embeddings. Existing rows that cannot prove the
// new canonical identity block cutover and require an explicit audited repair.
const memoryLifecycleMigrationSQL = `
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS canonical_key varchar(128);
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS superseded_by_memory_id varchar(128);
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS supersedes_memory_id varchar(128);
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE public.memories ADD COLUMN IF NOT EXISTS deprecated_at timestamptz;

ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS operation varchar(32) NOT NULL DEFAULT 'legacy';
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS snapshot jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS source_window varchar(128);
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS proposal_id varchar(128);
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS candidate_index integer;
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS semantic_reason text;
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS related_memory_ids jsonb NOT NULL DEFAULT '[]';
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS reason_code varchar(128);
ALTER TABLE public.memory_revisions ADD COLUMN IF NOT EXISTS schema_version varchar(64) NOT NULL DEFAULT 'legacy.v1';
ALTER TABLE public.memory_embeddings ADD COLUMN IF NOT EXISTS provider_endpoint_id varchar(128);

CREATE TABLE IF NOT EXISTS public.memory_governance (
  id varchar(128) PRIMARY KEY,
  fluctlight_id varchar(128) NOT NULL,
  proposal_id varchar(128),
  source_window varchar(128),
  candidate_index integer,
  operation varchar(32) NOT NULL,
  target_memory_id varchar(128),
  related_memory_ids jsonb NOT NULL DEFAULT '[]',
  base_revisions jsonb NOT NULL DEFAULT '{}',
  resulting_revisions jsonb NOT NULL DEFAULT '{}',
  actor_id varchar(128) NOT NULL,
  evidence_refs jsonb NOT NULL,
  semantic_reason text NOT NULL,
  disposition varchar(32) NOT NULL,
  reason_code varchar(128) NOT NULL,
  policy_version varchar(128) NOT NULL,
  request_digest varchar(128) NOT NULL,
  result jsonb NOT NULL,
  idempotency_key varchar(256) NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(proposal_id,candidate_index)
);

CREATE TABLE IF NOT EXISTS public.memory_lifecycle_repairs (
  memory_id varchar(128) PRIMARY KEY,
  repair_version varchar(64) NOT NULL,
  source_snapshot jsonb NOT NULL,
  canonical_key varchar(128) NOT NULL,
  request_digest varchar(128) NOT NULL,
  repaired_by_actor_id varchar(128) NOT NULL,
  reason text NOT NULL,
  repaired_at timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE malformed bigint;
BEGIN
  SELECT count(*) INTO malformed
  FROM public.memories m
  WHERE m.type NOT IN ('episodic','semantic','relationship','autobiographical')
     OR m.status NOT IN ('active','superseded','deprecated','forgotten')
     OR m.visibility NOT IN ('private','owner','participants')
     OR btrim(m.content)=''
     OR m.confidence NOT BETWEEN 0 AND 1
     OR m.importance NOT BETWEEN 0 AND 1
     OR m.emotional_significance NOT BETWEEN 0 AND 1
     OR m.revision < 0
     OR jsonb_typeof(m.actor_refs) IS DISTINCT FROM 'array'
     OR jsonb_typeof(m.event_refs) IS DISTINCT FROM 'array'
     OR jsonb_typeof(m.evidence_refs) IS DISTINCT FROM 'array'
     OR jsonb_array_length(m.evidence_refs)=0
     OR jsonb_typeof(m.personality_perspectives) IS DISTINCT FROM 'array'
     OR COALESCE(m.canonical_key,'') !~ '^[a-f0-9]{32}$'
     OR COALESCE(m.request_digest,'') !~ '^[a-f0-9]{32}$'
     OR m.superseded_by_memory_id=m.id
     OR m.supersedes_memory_id=m.id;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle cutover requires audited repair of % pre-lifecycle or malformed Memory row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.memories m
  WHERE m.canonical_key<>substring(encode(digest(
    m.type || chr(31) || btrim(m.content) || chr(31) || COALESCE(m.conversation_id,'') || chr(31) || m.visibility || chr(31) ||
    COALESCE((SELECT string_agg(actor_ref,chr(30) ORDER BY actor_ref) FROM jsonb_array_elements_text(m.actor_refs) AS actor_refs(actor_ref)),'') || chr(31) ||
    COALESCE((SELECT string_agg(event_ref,chr(30) ORDER BY event_ref) FROM jsonb_array_elements_text(m.event_refs) AS event_refs(event_ref)),'')
  ,'sha256'),'hex') FROM 1 FOR 32);
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % Memory row(s) with an incorrect canonical key', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.memories m
  LEFT JOIN public.memory_lifecycle_repairs repair ON repair.memory_id=m.id
  WHERE repair.memory_id IS NULL
     OR repair.repair_version<>'memory.lifecycle.repair.v1'
     OR repair.canonical_key<>m.canonical_key
     OR repair.request_digest<>m.request_digest
     OR btrim(repair.repaired_by_actor_id)=''
     OR btrim(repair.reason)=''
     OR repair.source_snapshot<>jsonb_build_object(
       'type',m.type,
       'content',btrim(m.content),
       'conversation_id',COALESCE(m.conversation_id,''),
       'visibility',m.visibility,
       'actor_refs',COALESCE((SELECT jsonb_agg(actor_ref ORDER BY actor_ref) FROM jsonb_array_elements_text(m.actor_refs) AS actor_refs(actor_ref)),'[]'::jsonb),
       'event_refs',COALESCE((SELECT jsonb_agg(event_ref ORDER BY event_ref) FROM jsonb_array_elements_text(m.event_refs) AS event_refs(event_ref)),'[]'::jsonb)
     );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % Memory row(s) without a verifiable audited repair', malformed;
  END IF;

  SELECT count(*) INTO malformed FROM (
    SELECT owner_fluctlight_id,canonical_key
    FROM public.memories
    WHERE status='active'
    GROUP BY owner_fluctlight_id,canonical_key
    HAVING count(*) > 1
  ) duplicate_active;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % duplicate active canonical Memory key(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed FROM (
    SELECT memory_id,revision
    FROM public.memory_revisions
    GROUP BY memory_id,revision
    HAVING count(*) > 1
  ) duplicate_revision;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % duplicate Memory revision(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed FROM (
    SELECT memory_id,memory_revision,model_id
    FROM public.memory_embeddings
    GROUP BY memory_id,memory_revision,model_id
    HAVING count(*) > 1
  ) duplicate_embedding;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % duplicate canonical embedding tuple(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.memory_embeddings e
  LEFT JOIN public.memory_revisions r ON r.memory_id=e.memory_id AND r.revision=e.memory_revision
  LEFT JOIN public.memories m ON m.id=e.memory_id
  WHERE r.memory_id IS NULL OR m.id IS NULL
     OR btrim(e.model_id)=''
     OR e.memory_revision < 0
     OR e.status NOT IN ('pending','ready','failed','stale')
     OR e.dimensions < 0
     OR jsonb_typeof(e.embedding) IS DISTINCT FROM 'array'
     OR EXISTS (SELECT 1 FROM jsonb_array_elements(e.embedding) value WHERE jsonb_typeof(value) IS DISTINCT FROM 'number')
     OR (e.status='ready' AND (e.dimensions<=0 OR e.embedding_vector IS NULL OR jsonb_array_length(e.embedding)<>e.dimensions OR vector_dims(e.embedding_vector)<>e.dimensions))
     OR (e.status<>'stale' AND btrim(COALESCE(e.provider_endpoint_id,''))='')
     OR (e.status='ready' AND (m.status<>'active' OR m.revision<>e.memory_revision));
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % malformed or orphan embedding row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.platform_workflow_intents i
  LEFT JOIN public.memories m ON m.id=i.payload->>'memory_id'
  LEFT JOIN public.memory_revisions r ON r.memory_id=i.payload->>'memory_id'
    AND r.revision::numeric=CASE WHEN jsonb_typeof(i.payload->'revision')='number' THEN (i.payload->>'revision')::numeric ELSE -1 END
  WHERE i.intent_type='memory.embedding'
    AND i.status IN ('pending','retry','started','cancel_requested')
    AND (
      jsonb_typeof(i.payload) IS DISTINCT FROM 'object'
      OR btrim(COALESCE(i.payload->>'memory_id',''))=''
      OR jsonb_typeof(i.payload->'revision') IS DISTINCT FROM 'number'
      OR (i.payload->>'revision')::numeric <> trunc((i.payload->>'revision')::numeric)
      OR (i.payload->>'revision')::numeric < 0
      OR btrim(COALESCE(i.payload->>'request_digest',''))=''
      OR m.id IS NULL OR r.memory_id IS NULL
      OR i.payload->>'request_digest'<>m.request_digest
      OR ((btrim(COALESCE(i.payload->>'provider_endpoint_id',''))='') <> (btrim(COALESCE(i.payload->>'model_id',''))=''))
    );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Memory lifecycle migration found % malformed active embedding workflow intent(s)', malformed;
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_memory_revisions_identity
ON public.memory_revisions(memory_id,revision);
CREATE INDEX IF NOT EXISTS ix_memories_authorized_retrieval
ON public.memories(owner_fluctlight_id,status,conversation_id,created_at DESC,id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_memories_active_canonical
ON public.memories(owner_fluctlight_id,canonical_key)
WHERE status='active';
CREATE UNIQUE INDEX IF NOT EXISTS uq_memory_embeddings_current_tuple
ON public.memory_embeddings(memory_id,memory_revision,model_id);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_memories_lifecycle_v2' AND conrelid='public.memories'::regclass
  ) THEN
    ALTER TABLE public.memories ADD CONSTRAINT ck_memories_lifecycle_v2 CHECK (
      type IN ('episodic','semantic','relationship','autobiographical')
      AND status IN ('active','superseded','deprecated','forgotten')
      AND visibility IN ('private','owner','participants')
      AND btrim(content)<>''
      AND confidence BETWEEN 0 AND 1
      AND importance BETWEEN 0 AND 1
      AND emotional_significance BETWEEN 0 AND 1
      AND revision >= 0
      AND jsonb_typeof(actor_refs)='array'
      AND jsonb_typeof(event_refs)='array'
      AND jsonb_typeof(evidence_refs)='array' AND jsonb_array_length(evidence_refs)>0
      AND jsonb_typeof(personality_perspectives)='array'
      AND canonical_key ~ '^[a-f0-9]{32}$'
      AND request_digest ~ '^[a-f0-9]{32}$'
      AND (superseded_by_memory_id IS NULL OR superseded_by_memory_id<>id)
      AND (supersedes_memory_id IS NULL OR supersedes_memory_id<>id)
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='fk_memory_embeddings_revision' AND conrelid='public.memory_embeddings'::regclass
  ) THEN
    ALTER TABLE public.memory_embeddings ADD CONSTRAINT fk_memory_embeddings_revision
      FOREIGN KEY(memory_id,memory_revision) REFERENCES public.memory_revisions(memory_id,revision);
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_memory_revisions_lifecycle_v2' AND conrelid='public.memory_revisions'::regclass
  ) THEN
    ALTER TABLE public.memory_revisions ADD CONSTRAINT ck_memory_revisions_lifecycle_v2 CHECK (
      revision >= 0 AND base_revision >= 0
      AND ((revision=0 AND base_revision=0) OR (revision>0 AND base_revision=revision-1))
      AND operation IN ('legacy','create','confirm','revise','merge','supersede','deprecate','forget','rollback')
      AND status IN ('active','superseded','deprecated','forgotten')
      AND jsonb_typeof(personality_perspectives)='array'
      AND jsonb_typeof(evidence_refs)='array'
      AND jsonb_typeof(related_memory_ids)='array'
      AND (schema_version='legacy.v1' OR (schema_version='memory.lifecycle.v2' AND jsonb_typeof(snapshot)='object' AND request_digest ~ '^[a-f0-9]{32}$'))
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_memory_governance_v2' AND conrelid='public.memory_governance'::regclass
  ) THEN
    ALTER TABLE public.memory_governance ADD CONSTRAINT ck_memory_governance_v2 CHECK (
      operation IN ('create','confirm','revise','merge','supersede','deprecate','forget','rollback')
      AND disposition IN ('applied','no_change','rejected','deferred')
      AND jsonb_typeof(related_memory_ids)='array'
      AND jsonb_typeof(base_revisions)='object'
      AND jsonb_typeof(resulting_revisions)='object'
      AND jsonb_typeof(evidence_refs)='array' AND jsonb_array_length(evidence_refs)>0
      AND jsonb_typeof(result)='object'
      AND btrim(semantic_reason)<>''
      AND request_digest ~ '^[a-f0-9]{32}$'
      AND btrim(idempotency_key)<>''
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_memory_embeddings_lifecycle_v2' AND conrelid='public.memory_embeddings'::regclass
  ) THEN
    ALTER TABLE public.memory_embeddings ADD CONSTRAINT ck_memory_embeddings_lifecycle_v2 CHECK (
      memory_revision >= 0
      AND btrim(model_id)<>''
      AND status IN ('pending','ready','failed','stale')
      AND dimensions >= 0
      AND jsonb_typeof(embedding)='array'
      AND NOT jsonb_path_exists(embedding, '$[*] ? (@.type() != "number")')
      AND (status='stale' OR btrim(COALESCE(provider_endpoint_id,''))<>'')
      AND (status<>'ready' OR (dimensions>0 AND embedding_vector IS NOT NULL AND jsonb_array_length(embedding)=dimensions AND vector_dims(embedding_vector)=dimensions))
    );
  END IF;
END $$;
`

const lifeContextRevisionMigrationSQL = `
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1;
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS result jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.life_events ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS status varchar(32) NOT NULL DEFAULT 'active';
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS revision integer NOT NULL DEFAULT 1;
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS superseded_by_overlay_id varchar(128);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS result jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.life_presence_overlays ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS result jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.life_schedules ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE public.life_schedule_items ADD COLUMN IF NOT EXISTS location varchar(512);
CREATE TABLE IF NOT EXISTS public.life_context_commands (
  id varchar(128) PRIMARY KEY,
  fluctlight_id varchar(128) NOT NULL,
  command_type varchar(64) NOT NULL,
  target_id varchar(128),
  idempotency_key varchar(256) NOT NULL,
  request_digest varchar(128) NOT NULL,
  result jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(fluctlight_id,idempotency_key)
);
DO $$
DECLARE
  malformed bigint;
  app_table record;
  table_rows bigint;
BEGIN
  malformed := 0;
  FOR app_table IN
    SELECT tablename
    FROM pg_catalog.pg_tables
	WHERE schemaname='public' AND tablename NOT IN ('alembic_version','runtime_settings')
    ORDER BY tablename
  LOOP
    EXECUTE format('SELECT count(*) FROM public.%I', app_table.tablename) INTO table_rows;
    malformed := malformed + table_rows;
  END LOOP;
  IF malformed > 0 THEN
    RAISE EXCEPTION '0030_life_context_revision is a clean-start cutover; rebuild the database instead of migrating % existing business row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.life_events e
  WHERE revision < 1
     OR status NOT IN ('confirmed','inferred','cancelled')
     OR end_at<=start_at
     OR jsonb_typeof(evidence_refs) IS DISTINCT FROM 'array'
     OR jsonb_typeof(result) IS DISTINCT FROM 'object'
     OR (status IN ('confirmed','inferred')
       AND end_at>now() AND (expires_at IS NULL OR expires_at>now())
       AND (
         btrim(COALESCE(idempotency_key,''))=''
         OR COALESCE(request_digest,'') !~ '^[a-f0-9]{32}$'
         OR result='{}'::jsonb
         OR COALESCE(result->>'id',result->>'event_id','')<>id
         OR COALESCE(result->>'revision',result->>'event_revision','')<>revision::text
         OR btrim(COALESCE(result->>'expected_context_revision',''))=''
         OR btrim(COALESCE(result->>'resulting_context_revision',''))=''
         OR result->'replayed' IS DISTINCT FROM 'false'::jsonb
       )
     );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context migration found % malformed Event row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.life_presence_overlays p
  WHERE revision < 1
     OR status NOT IN ('active','superseded','cleared')
     OR (status='active' AND current_task IS NULL AND user_presence IS NULL)
     OR (status<>'active' AND superseded_by_overlay_id=id)
     OR jsonb_typeof(result) IS DISTINCT FROM 'object'
     OR (status='active' AND (expires_at IS NULL OR expires_at>now()) AND (
       btrim(COALESCE(idempotency_key,''))=''
       OR COALESCE(request_digest,'') !~ '^[a-f0-9]{32}$'
       OR result='{}'::jsonb
       OR COALESCE(result->>'id',result->>'overlay_id','')<>id
       OR COALESCE(result->>'revision',result->>'overlay_revision','')<>revision::text
       OR btrim(COALESCE(result->>'expected_context_revision',''))=''
       OR btrim(COALESCE(result->>'resulting_context_revision',''))=''
       OR result->'replayed' IS DISTINCT FROM 'false'::jsonb
     ));
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context clean-start migration requires rebuilding % Presence row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed FROM (
    SELECT fluctlight_id FROM public.life_presence_overlays
    WHERE status='active'
    GROUP BY fluctlight_id HAVING count(*)>1
  ) duplicate_presence;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context migration found % Fluctlight(s) with multiple active Presence rows', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.life_schedules s
  WHERE status='accepted' AND local_date >= (now() AT TIME ZONE timezone)::date
    AND (
      btrim(COALESCE(idempotency_key,''))=''
      OR COALESCE(request_digest,'') !~ '^[a-f0-9]{32}$'
      OR jsonb_typeof(result) IS DISTINCT FROM 'object'
      OR result='{}'::jsonb
      OR COALESCE(result->>'id','')<>id
      OR COALESCE(result->>'revision','')<>revision::text
      OR btrim(COALESCE(result->>'expected_context_revision',''))=''
      OR btrim(COALESCE(result->>'resulting_context_revision',''))=''
      OR result->'replayed' IS DISTINCT FROM 'false'::jsonb
    );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context clean-start migration requires rebuilding % active Schedule row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.autonomy_actions
  WHERE status IN ('frozen','running','pending','started')
    AND btrim(COALESCE(expected_revisions->>'life_context_revision',''))='';
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context migration requires draining % active autonomy action(s) without Life Context revision', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.cognition_frozen_actions
  WHERE status IN ('frozen','pending','claimed','started','running')
    AND btrim(COALESCE(payload#>>'{decision,context_projection,life_context_revision}',''))='';
  IF malformed > 0 THEN
    RAISE EXCEPTION 'Life Context migration requires draining % active cognition action(s) without Life Context revision', malformed;
  END IF;
END $$;

ALTER TABLE public.life_events ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.life_events ALTER COLUMN request_digest SET NOT NULL;
ALTER TABLE public.life_presence_overlays ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.life_presence_overlays ALTER COLUMN request_digest SET NOT NULL;
ALTER TABLE public.life_schedules ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.life_schedules ALTER COLUMN request_digest SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_life_presence_idempotency
ON public.life_presence_overlays(fluctlight_id,idempotency_key)
WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_life_presence_active
ON public.life_presence_overlays(fluctlight_id)
WHERE status='active';
CREATE INDEX IF NOT EXISTS ix_life_events_context_authority
ON public.life_events(fluctlight_id,status,start_at,end_at,expires_at,revision);
CREATE UNIQUE INDEX IF NOT EXISTS uq_life_schedules_idempotency_required
ON public.life_schedules(fluctlight_id,idempotency_key)
WHERE idempotency_key IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_life_events_revision_v1' AND conrelid='public.life_events'::regclass
  ) THEN
    ALTER TABLE public.life_events ADD CONSTRAINT ck_life_events_revision_v1 CHECK (
      revision>=1 AND status IN ('confirmed','inferred','cancelled') AND end_at>start_at
      AND jsonb_typeof(evidence_refs)='array'
      AND jsonb_typeof(result)='object'
      AND request_digest ~ '^[a-f0-9]{32}$'
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_life_presence_revision_v1' AND conrelid='public.life_presence_overlays'::regclass
  ) THEN
    ALTER TABLE public.life_presence_overlays ADD CONSTRAINT ck_life_presence_revision_v1 CHECK (
      revision>=1
      AND status IN ('active','superseded','cleared')
      AND (status<>'active' OR current_task IS NOT NULL OR user_presence IS NOT NULL)
      AND (superseded_by_overlay_id IS NULL OR superseded_by_overlay_id<>id)
      AND btrim(COALESCE(idempotency_key,''))<>''
      AND request_digest ~ '^[a-f0-9]{32}$'
      AND jsonb_typeof(result)='object'
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_life_schedules_active_authority_v1' AND conrelid='public.life_schedules'::regclass
  ) THEN
    ALTER TABLE public.life_schedules ADD CONSTRAINT ck_life_schedules_active_authority_v1 CHECK (
      status<>'accepted' OR (
        revision>=1
        AND btrim(COALESCE(idempotency_key,''))<>''
        AND COALESCE(request_digest,'') ~ '^[a-f0-9]{32}$'
        AND jsonb_typeof(result)='object'
      )
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_life_context_commands_v1' AND conrelid='public.life_context_commands'::regclass
  ) THEN
    ALTER TABLE public.life_context_commands ADD CONSTRAINT ck_life_context_commands_v1 CHECK (
      btrim(command_type)<>''
      AND btrim(idempotency_key)<>''
      AND COALESCE(request_digest,'') ~ '^[a-f0-9]{32}$'
      AND jsonb_typeof(result)='object' AND result<>'{}'::jsonb
      AND COALESCE(result->>'id','')=COALESCE(target_id,'')
      AND btrim(COALESCE(result->>'status',''))<>''
      AND CASE WHEN jsonb_typeof(result->'revision')='number' THEN (result->>'revision')::integer>=1 ELSE false END
      AND btrim(COALESCE(result->>'expected_context_revision',''))<>''
      AND btrim(COALESCE(result->>'resulting_context_revision',''))<>''
      AND COALESCE(result->>'idempotency_key','')=idempotency_key
      AND result->'replayed' IS NOT DISTINCT FROM 'false'::jsonb
    );
  END IF;
END $$;

CREATE OR REPLACE FUNCTION public.fluctlight_life_authority_replay_ready_v1()
RETURNS trigger LANGUAGE plpgsql AS $fn$
DECLARE
  entity_id text;
  entity_revision text;
  requires_result boolean;
  row_status text;
  row_revision integer;
  row_idempotency_key text;
  row_request_digest text;
  row_result jsonb;
  row_end_at timestamptz;
  row_expires_at timestamptz;
  row_local_date date;
  row_timezone text;
BEGIN
  requires_result := false;
  IF TG_TABLE_NAME='life_events' THEN
    SELECT status,revision,idempotency_key,request_digest,result,end_at,expires_at
    INTO row_status,row_revision,row_idempotency_key,row_request_digest,row_result,row_end_at,row_expires_at
    FROM public.life_events WHERE id=NEW.id;
    requires_result := TG_OP='INSERT' OR (row_status IN ('confirmed','inferred')
      AND row_end_at>clock_timestamp()
      AND (row_expires_at IS NULL OR row_expires_at>clock_timestamp()));
    entity_id := COALESCE(row_result->>'id',row_result->>'event_id','');
    entity_revision := COALESCE(row_result->>'revision',row_result->>'event_revision','');
  ELSIF TG_TABLE_NAME='life_presence_overlays' THEN
    SELECT status,revision,idempotency_key,request_digest,result,expires_at
    INTO row_status,row_revision,row_idempotency_key,row_request_digest,row_result,row_expires_at
    FROM public.life_presence_overlays WHERE id=NEW.id;
    requires_result := TG_OP='INSERT' OR (row_status='active'
      AND (row_expires_at IS NULL OR row_expires_at>clock_timestamp()));
    entity_id := COALESCE(row_result->>'id',row_result->>'overlay_id','');
    entity_revision := COALESCE(row_result->>'revision',row_result->>'overlay_revision','');
  ELSIF TG_TABLE_NAME='life_schedules' THEN
    SELECT status,revision,idempotency_key,request_digest,result,local_date,timezone
    INTO row_status,row_revision,row_idempotency_key,row_request_digest,row_result,row_local_date,row_timezone
    FROM public.life_schedules WHERE id=NEW.id;
    requires_result := TG_OP='INSERT' OR (row_status='accepted'
      AND row_local_date >= (clock_timestamp() AT TIME ZONE row_timezone)::date);
    entity_id := COALESCE(row_result->>'id','');
    entity_revision := COALESCE(row_result->>'revision','');
  END IF;
  IF NOT requires_result THEN
    RETURN NEW;
  END IF;
  IF btrim(COALESCE(row_idempotency_key,''))=''
     OR COALESCE(row_request_digest,'') !~ '^[a-f0-9]{32}$'
     OR entity_id<>NEW.id OR entity_revision<>row_revision::text
     OR btrim(COALESCE(row_result->>'status',''))=''
     OR btrim(COALESCE(row_result->>'expected_context_revision',''))=''
     OR btrim(COALESCE(row_result->>'resulting_context_revision',''))=''
     OR row_result->'replayed' IS DISTINCT FROM 'false'::jsonb THEN
    RAISE EXCEPTION '% authority row % is not replay-ready', TG_TABLE_NAME, NEW.id;
  END IF;
  RETURN NEW;
END
$fn$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='ct_life_events_replay_ready_v1' AND tgrelid='public.life_events'::regclass) THEN
    CREATE CONSTRAINT TRIGGER ct_life_events_replay_ready_v1
    AFTER INSERT OR UPDATE ON public.life_events DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION public.fluctlight_life_authority_replay_ready_v1();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='ct_life_presence_replay_ready_v1' AND tgrelid='public.life_presence_overlays'::regclass) THEN
    CREATE CONSTRAINT TRIGGER ct_life_presence_replay_ready_v1
    AFTER INSERT OR UPDATE ON public.life_presence_overlays DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION public.fluctlight_life_authority_replay_ready_v1();
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='ct_life_schedules_replay_ready_v1' AND tgrelid='public.life_schedules'::regclass) THEN
    CREATE CONSTRAINT TRIGGER ct_life_schedules_replay_ready_v1
    AFTER INSERT OR UPDATE ON public.life_schedules DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION public.fluctlight_life_authority_replay_ready_v1();
  END IF;
END $$;
`

const evolutionAuthorityMigrationSQL = `
DO $$
DECLARE
  business_rows bigint := 0;
  app_table record;
  table_rows bigint;
BEGIN
  FOR app_table IN
    SELECT tablename
    FROM pg_catalog.pg_tables
    WHERE schemaname='public' AND tablename NOT IN ('alembic_version','runtime_settings')
    ORDER BY tablename
  LOOP
    EXECUTE format('SELECT count(*) FROM public.%I', app_table.tablename) INTO table_rows;
    business_rows := business_rows + table_rows;
  END LOOP;
  IF business_rows > 0 THEN
    RAISE EXCEPTION '0031_evolution_authority is a clean-start cutover; rebuild the database instead of migrating % existing business row(s)', business_rows;
  END IF;
END $$;

ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS desired_outcome text;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS success_criteria jsonb;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS motivation text;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS needs_reflection boolean;
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.fluctlight_goals ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.fluctlight_goals ALTER COLUMN desired_outcome SET NOT NULL;
ALTER TABLE public.fluctlight_goals ALTER COLUMN success_criteria SET NOT NULL;
ALTER TABLE public.fluctlight_goals ALTER COLUMN motivation SET NOT NULL;
ALTER TABLE public.fluctlight_goals ALTER COLUMN needs_reflection SET NOT NULL;
ALTER TABLE public.fluctlight_goals ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.fluctlight_goals ALTER COLUMN request_digest SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_fluctlight_goal_authority_idempotency ON public.fluctlight_goals(fluctlight_id,idempotency_key);

ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS operation varchar(32);
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS base_revision integer;
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS snapshot jsonb;
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS evidence_refs jsonb;
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS policy_version varchar(128);
ALTER TABLE public.fluctlight_goal_revisions ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN operation SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN base_revision SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN snapshot SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN evidence_refs SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN policy_version SET NOT NULL;
ALTER TABLE public.fluctlight_goal_revisions ALTER COLUMN request_digest SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_fluctlight_goal_revision_authority_idempotency ON public.fluctlight_goal_revisions(idempotency_key);

ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS action_intent text;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS expected_outcome text;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS capability_constraints jsonb;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS current_attempt_id varchar(128);
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS trigger_cursor_sequence integer NOT NULL DEFAULT 0;
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.fluctlight_intentions ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.fluctlight_intentions ALTER COLUMN action_intent SET NOT NULL;
ALTER TABLE public.fluctlight_intentions ALTER COLUMN expected_outcome SET NOT NULL;
ALTER TABLE public.fluctlight_intentions ALTER COLUMN capability_constraints SET NOT NULL;
ALTER TABLE public.fluctlight_intentions ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.fluctlight_intentions ALTER COLUMN request_digest SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_fluctlight_intention_authority_idempotency ON public.fluctlight_intentions(fluctlight_id,idempotency_key);

ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS operation varchar(32);
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS base_revision integer;
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS snapshot jsonb;
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS evidence_refs jsonb;
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS policy_version varchar(128);
ALTER TABLE public.fluctlight_intention_revisions ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN operation SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN base_revision SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN snapshot SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN evidence_refs SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN idempotency_key SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN policy_version SET NOT NULL;
ALTER TABLE public.fluctlight_intention_revisions ALTER COLUMN request_digest SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_fluctlight_intention_revision_authority_idempotency ON public.fluctlight_intention_revisions(idempotency_key);

CREATE TABLE IF NOT EXISTS public.fluctlight_intention_attempts (
  attempt_id varchar(128) PRIMARY KEY,
  fluctlight_id varchar(128) NOT NULL,
  intention_ref varchar(96) NOT NULL,
  goal_ref varchar(96) NOT NULL,
  action_id varchar(128) NOT NULL UNIQUE,
  outcome_id varchar(128) NOT NULL UNIQUE,
  outcome_digest varchar(128) NOT NULL,
  status varchar(32) NOT NULL,
  result jsonb NOT NULL,
  occurred_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE public.cognition_reflection_windows ADD COLUMN IF NOT EXISTS evolution_revisions jsonb NOT NULL DEFAULT '{}';
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS schema_version varchar(64);
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS context_snapshot jsonb;
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS expected_revisions jsonb;
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS result jsonb;
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS model_version varchar(128);
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS prompt_version varchar(128);
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS policy_version varchar(128);
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS request_digest varchar(128);
ALTER TABLE public.cognition_reflection_proposals ADD COLUMN IF NOT EXISTS idempotency_key varchar(256);
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN schema_version SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN context_snapshot SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN expected_revisions SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN result SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN model_version SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN prompt_version SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN policy_version SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN request_digest SET NOT NULL;
ALTER TABLE public.cognition_reflection_proposals ALTER COLUMN idempotency_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_reflection_proposal_v2_idempotency ON public.cognition_reflection_proposals(fluctlight_id,idempotency_key);

CREATE TABLE IF NOT EXISTS public.cognition_reflection_candidate_dispositions (
  id varchar(128) PRIMARY KEY,
  proposal_id varchar(128) NOT NULL,
  fluctlight_id varchar(128) NOT NULL,
  domain varchar(64) NOT NULL,
  candidate_id varchar(128) NOT NULL,
  candidate_index integer NOT NULL,
  disposition varchar(32) NOT NULL,
  reason_code varchar(128) NOT NULL,
  target_ref varchar(96),
  changed_ref varchar(96),
  base_revision integer NOT NULL,
  resulting_revision integer NOT NULL,
  evidence_refs jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(proposal_id,candidate_id)
);

CREATE TABLE IF NOT EXISTS public.fluctlight_evolution_states (
  fluctlight_id varchar(128) NOT NULL,
  profile_id varchar(128) NOT NULL,
  profile_ref varchar(96) NOT NULL,
  revision integer NOT NULL DEFAULT 0,
  domain_revisions jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(fluctlight_id,profile_id)
);

CREATE TABLE IF NOT EXISTS public.fluctlight_evolution_overlays (
  id varchar(128) PRIMARY KEY,
  ref varchar(96) NOT NULL UNIQUE,
  fluctlight_id varchar(128) NOT NULL,
  profile_id varchar(128) NOT NULL,
  kind varchar(32) NOT NULL,
  field_path varchar(128) NOT NULL,
  value_kind varchar(32) NOT NULL,
  semantic_direction varchar(32) NOT NULL,
  requested_delta double precision,
  applied_delta double precision,
  before_value jsonb NOT NULL,
  after_value jsonb NOT NULL,
  confidence double precision NOT NULL,
  evidence_refs jsonb NOT NULL,
  evidence_windows jsonb NOT NULL,
  policy_version varchar(128) NOT NULL,
  base_revision integer NOT NULL,
  revision integer NOT NULL,
  status varchar(32) NOT NULL,
  supersedes varchar(128),
  rollback_of varchar(128),
  cooldown_until timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  UNIQUE(fluctlight_id,profile_id,revision)
);
CREATE INDEX IF NOT EXISTS ix_evolution_overlays_effective ON public.fluctlight_evolution_overlays(fluctlight_id,profile_id,status,kind,field_path,revision);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_goal_authority_v2' AND conrelid='public.fluctlight_goals'::regclass) THEN
    ALTER TABLE public.fluctlight_goals ADD CONSTRAINT ck_fluctlight_goal_authority_v2 CHECK (
	  btrim(desired_outcome)<>'' AND jsonb_typeof(success_criteria)='array'
	  AND (needs_reflection OR jsonb_array_length(success_criteria)>0) AND btrim(motivation)<>''
      AND status IN ('candidate','active','paused','completed','abandoned','cancelled')
      AND revision>=1 AND btrim(idempotency_key)<>'' AND request_digest ~ '^[a-f0-9]{32}$'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_goal_revision_authority_v2' AND conrelid='public.fluctlight_goal_revisions'::regclass) THEN
    ALTER TABLE public.fluctlight_goal_revisions ADD CONSTRAINT ck_fluctlight_goal_revision_authority_v2 CHECK (
      operation IN ('create','update','pause','resume','complete','abandon','cancel')
      AND base_revision>=0 AND jsonb_typeof(snapshot)='object' AND jsonb_typeof(evidence_refs)='array'
      AND btrim(idempotency_key)<>'' AND btrim(policy_version)<>'' AND request_digest ~ '^[a-f0-9]{32}$'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_intention_authority_v2' AND conrelid='public.fluctlight_intentions'::regclass) THEN
    ALTER TABLE public.fluctlight_intentions ADD CONSTRAINT ck_fluctlight_intention_authority_v2 CHECK (
      btrim(action_intent)<>'' AND btrim(expected_outcome)<>'' AND jsonb_typeof(capability_constraints)='array'
      AND jsonb_typeof(trigger)='object' AND trigger->>'type' IN ('time','event','semantic')
      AND status IN ('candidate','qualified','due','in_progress','paused','completed','expired','cancelled')
      AND revision>=1 AND trigger_cursor_sequence>=0 AND btrim(idempotency_key)<>'' AND request_digest ~ '^[a-f0-9]{32}$'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_fluctlight_intention_revision_authority_v2' AND conrelid='public.fluctlight_intention_revisions'::regclass) THEN
    ALTER TABLE public.fluctlight_intention_revisions ADD CONSTRAINT ck_fluctlight_intention_revision_authority_v2 CHECK (
      operation IN ('create','update','qualify','mark_due','pause','resume','complete','expire','cancel')
      AND base_revision>=0 AND jsonb_typeof(snapshot)='object' AND jsonb_typeof(evidence_refs)='array'
      AND btrim(idempotency_key)<>'' AND btrim(policy_version)<>'' AND request_digest ~ '^[a-f0-9]{32}$'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_intention_attempt_authority_v2' AND conrelid='public.fluctlight_intention_attempts'::regclass) THEN
    ALTER TABLE public.fluctlight_intention_attempts ADD CONSTRAINT ck_intention_attempt_authority_v2 CHECK (
      status IN ('succeeded','failed','cancelled','suppressed') AND outcome_digest ~ '^[a-f0-9]{32}$'
      AND jsonb_typeof(result)='object'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_reflection_proposal_authority_v2' AND conrelid='public.cognition_reflection_proposals'::regclass) THEN
    ALTER TABLE public.cognition_reflection_proposals ADD CONSTRAINT ck_reflection_proposal_authority_v2 CHECK (
      schema_version='fluctlight.reflection-proposal.v2' AND jsonb_typeof(context_snapshot)='object'
      AND jsonb_typeof(expected_revisions)='object' AND jsonb_typeof(result)='object'
      AND status IN ('proposed','applied','no_change','invalid','deferred','failed')
      AND request_digest ~ '^[a-f0-9]{32}$' AND btrim(idempotency_key)<>''
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_reflection_candidate_disposition_v2' AND conrelid='public.cognition_reflection_candidate_dispositions'::regclass) THEN
    ALTER TABLE public.cognition_reflection_candidate_dispositions ADD CONSTRAINT ck_reflection_candidate_disposition_v2 CHECK (
      candidate_index>=0 AND disposition IN ('accepted','rejected','deferred','no_change')
      AND base_revision>=0 AND resulting_revision>=base_revision AND jsonb_typeof(evidence_refs)='array'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_evolution_state_authority_v1' AND conrelid='public.fluctlight_evolution_states'::regclass) THEN
    ALTER TABLE public.fluctlight_evolution_states ADD CONSTRAINT ck_evolution_state_authority_v1 CHECK (
      revision>=0 AND jsonb_typeof(domain_revisions)='object'
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_evolution_overlay_authority_v1' AND conrelid='public.fluctlight_evolution_overlays'::regclass) THEN
    ALTER TABLE public.fluctlight_evolution_overlays ADD CONSTRAINT ck_evolution_overlay_authority_v1 CHECK (
      kind IN ('personality','behavior_policy') AND value_kind IN ('numeric','categorical')
      AND btrim(field_path)<>'' AND confidence BETWEEN 0 AND 1
      AND jsonb_typeof(evidence_refs)='array' AND jsonb_typeof(evidence_windows)='array'
      AND base_revision>=0 AND revision=base_revision+1 AND status IN ('active','superseded')
      AND ref ~ '^evolution_overlay:ctx_[a-f0-9]{32}$'
    );
  END IF;
END $$;
`

// affectCanonicalMigrationSQL is deliberately a later immutable revision.
// Earlier disposable/project-health databases may already record 0027; this
// revision guarantees that S04 backfill and full numeric constraints are not
// silently skipped when those databases are upgraded.
const affectCanonicalMigrationSQL = `
INSERT INTO public.fluctlight_affect_profiles(fluctlight_id)
SELECT f.id FROM public.fluctlights f
ON CONFLICT(fluctlight_id) DO NOTHING;

DO $$
DECLARE malformed bigint;
BEGIN
  SELECT count(*) INTO malformed
  FROM public.fluctlight_inner_states s
  WHERE jsonb_typeof(s.pad) IS DISTINCT FROM 'object'
     OR CASE WHEN jsonb_typeof(s.pad->'pleasure')='number' THEN (s.pad->>'pleasure')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR CASE WHEN jsonb_typeof(s.pad->'arousal')='number' THEN (s.pad->>'arousal')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR CASE WHEN jsonb_typeof(s.pad->'dominance')='number' THEN (s.pad->>'dominance')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR jsonb_typeof(s.mood) IS DISTINCT FROM 'object'
     OR (s.mood ? 'intensity' AND CASE WHEN jsonb_typeof(s.mood->'intensity')='number' THEN (s.mood->>'intensity')::double precision NOT BETWEEN 0 AND 1 ELSE true END)
     OR jsonb_typeof(s.momentum) IS DISTINCT FROM 'object'
     OR (CASE WHEN jsonb_typeof(s.momentum)='object' THEN s.momentum - ARRAY['value','trend','pleasure_momentum','arousal_momentum','dominance_momentum'] ELSE '{}'::jsonb END) <> '{}'::jsonb
     OR EXISTS (
       SELECT 1 FROM jsonb_each(CASE WHEN jsonb_typeof(s.momentum)='object' THEN s.momentum ELSE '{}'::jsonb END) field
       WHERE CASE WHEN jsonb_typeof(field.value)='number' THEN (field.value #>> '{}')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     )
     OR jsonb_typeof(s.regulation) IS DISTINCT FROM 'object'
     OR (s.regulation ? 'stress' AND CASE WHEN jsonb_typeof(s.regulation->'stress')='number' THEN (s.regulation->>'stress')::double precision NOT BETWEEN 0 AND 1 ELSE true END)
     OR (s.regulation ? 'stability' AND CASE WHEN jsonb_typeof(s.regulation->'stability')='number' THEN (s.regulation->>'stability')::double precision NOT BETWEEN 0 AND 1 ELSE true END)
     OR jsonb_typeof(s.drives) IS DISTINCT FROM 'array'
     OR EXISTS (
       SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(s.drives)='array' THEN s.drives ELSE '[]'::jsonb END) drive
       WHERE jsonb_typeof(drive) IS DISTINCT FROM 'object'
          OR btrim(COALESCE(drive->>'key',''))=''
          OR CASE WHEN jsonb_typeof(drive->'pressure')='number' THEN (drive->>'pressure')::double precision NOT BETWEEN 0 AND 1 ELSE true END
          OR CASE WHEN jsonb_typeof(drive->'salience')='number' THEN (drive->>'salience')::double precision NOT BETWEEN 0 AND 1 ELSE true END
     )
     OR jsonb_typeof(s.conflicts) IS DISTINCT FROM 'array'
     OR EXISTS (
       SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(s.conflicts)='array' THEN s.conflicts ELSE '[]'::jsonb END) conflict
       WHERE jsonb_typeof(conflict) IS DISTINCT FROM 'object'
          OR btrim(COALESCE(conflict->>'key',''))=''
          OR CASE WHEN jsonb_typeof(conflict->'pressure')='number' THEN (conflict->>'pressure')::double precision NOT BETWEEN 0 AND 1 ELSE true END
     );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'canonical affect migration found % malformed inner-state row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.fluctlight_affect_profiles p
  WHERE jsonb_typeof(p.baseline_pad) IS DISTINCT FROM 'object'
     OR CASE WHEN jsonb_typeof(p.baseline_pad->'pleasure')='number' THEN (p.baseline_pad->>'pleasure')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR CASE WHEN jsonb_typeof(p.baseline_pad->'arousal')='number' THEN (p.baseline_pad->>'arousal')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR CASE WHEN jsonb_typeof(p.baseline_pad->'dominance')='number' THEN (p.baseline_pad->>'dominance')::double precision NOT BETWEEN -1 AND 1 ELSE true END
     OR jsonb_typeof(p.decay_policy) IS DISTINCT FROM 'object'
     OR CASE WHEN jsonb_typeof(p.decay_policy->'pad_half_life_seconds')='number' THEN (p.decay_policy->>'pad_half_life_seconds')::double precision NOT BETWEEN 60 AND 2592000 ELSE true END
     OR CASE WHEN jsonb_typeof(p.decay_policy->'momentum_half_life_seconds')='number' THEN (p.decay_policy->>'momentum_half_life_seconds')::double precision NOT BETWEEN 60 AND 2592000 ELSE true END
     OR CASE WHEN jsonb_typeof(p.decay_policy->'mood_half_life_seconds')='number' THEN (p.decay_policy->>'mood_half_life_seconds')::double precision NOT BETWEEN 60 AND 2592000 ELSE true END
     OR (p.decay_policy ? 'drive_half_life_seconds' AND CASE WHEN jsonb_typeof(p.decay_policy->'drive_half_life_seconds')='number' THEN (p.decay_policy->>'drive_half_life_seconds')::double precision NOT BETWEEN 60 AND 2592000 ELSE true END)
     OR jsonb_typeof(p.regulation_policy) IS DISTINCT FROM 'object'
     OR CASE WHEN jsonb_typeof(p.regulation_policy->'strength')='number' THEN (p.regulation_policy->>'strength')::double precision NOT BETWEEN 0 AND 1 ELSE true END
     OR jsonb_typeof(p.emotional_summary) IS DISTINCT FROM 'object'
     OR jsonb_typeof(p.evidence_refs) IS DISTINCT FROM 'array'
     OR p.revision < 0
	     OR p.policy_version <> 'affect.reducer.v2';
  IF malformed > 0 THEN
    RAISE EXCEPTION 'canonical affect migration found % malformed affect-profile row(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed
  FROM public.fluctlight_drive_slots d
  WHERE d.revision < 0
     OR CASE WHEN jsonb_typeof(d.confidence)='number' THEN (d.confidence #>> '{}')::double precision NOT BETWEEN 0 AND 1 ELSE true END
     OR (
       d.value_schema='pressure' AND (
         jsonb_typeof(d.value) IS DISTINCT FROM 'object'
         OR CASE WHEN jsonb_typeof(d.value->'pressure')='number' THEN (d.value->>'pressure')::double precision NOT BETWEEN 0 AND 1 ELSE true END
         OR CASE WHEN jsonb_typeof(d.value->'salience')='number' THEN (d.value->>'salience')::double precision NOT BETWEEN 0 AND 1 ELSE true END
         OR (d.value ? 'direction' AND (jsonb_typeof(d.value->'direction') IS DISTINCT FROM 'string' OR length(d.value->>'direction') > 256))
       )
     );
  IF malformed > 0 THEN
    RAISE EXCEPTION 'canonical affect migration found % malformed typed Drive slot(s)', malformed;
  END IF;

  SELECT count(*) INTO malformed FROM (
    SELECT fluctlight_id,source_event_id
    FROM public.fluctlight_state_revisions
    GROUP BY fluctlight_id,source_event_id
    HAVING count(*) > 1
  ) duplicate_source;
  IF malformed > 0 THEN
    RAISE EXCEPTION 'canonical affect migration found % duplicate state-transition source(s)', malformed;
  END IF;
END $$;

UPDATE public.fluctlight_affect_profiles
SET decay_policy=jsonb_set(decay_policy,'{drive_half_life_seconds}','14400'::jsonb,true)
WHERE NOT (decay_policy ? 'drive_half_life_seconds');

INSERT INTO public.fluctlight_affect_reconciliations(id,fluctlight_id,source_head,mapping_policy,before_state,after_state,disposition)
SELECT 'affect_reconcile_' || substr(encode(digest(s.fluctlight_id || ':0028_affect_canonical','sha256'),'hex'),1,32),
       s.fluctlight_id,
       '0027_project_health_evolution',
       'preserve_numeric_subset_no_semantic_inference',
       jsonb_build_object('pad',s.pad,'mood',s.mood,'momentum',s.momentum,'regulation',s.regulation,'drives',s.drives,'conflicts',s.conflicts),
       jsonb_build_object('pad',s.pad,'mood',s.mood,'momentum',s.momentum,'regulation',s.regulation,'drives',s.drives,'conflicts',s.conflicts),
       'preserved'
FROM public.fluctlight_inner_states s
ON CONFLICT(fluctlight_id,source_head) DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS uq_fluctlight_state_revisions_source
ON public.fluctlight_state_revisions(fluctlight_id,source_event_id);

CREATE OR REPLACE FUNCTION public.fluctlight_drive_state_valid(value jsonb)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
  SELECT jsonb_typeof(value)='array'
     AND NOT EXISTS (
       SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(value)='array' THEN value ELSE '[]'::jsonb END) drive
       WHERE jsonb_typeof(drive) IS DISTINCT FROM 'object'
          OR btrim(COALESCE(drive->>'key',''))=''
          OR jsonb_typeof(drive->'pressure') IS DISTINCT FROM 'number'
          OR (drive->>'pressure')::double precision NOT BETWEEN 0 AND 1
          OR jsonb_typeof(drive->'salience') IS DISTINCT FROM 'number'
          OR (drive->>'salience')::double precision NOT BETWEEN 0 AND 1
     )
$$;

CREATE OR REPLACE FUNCTION public.fluctlight_drive_conflicts_valid(value jsonb)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
  SELECT jsonb_typeof(value)='array'
     AND NOT EXISTS (
       SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(value)='array' THEN value ELSE '[]'::jsonb END) conflict
       WHERE jsonb_typeof(conflict) IS DISTINCT FROM 'object'
          OR btrim(COALESCE(conflict->>'key',''))=''
          OR jsonb_typeof(conflict->'pressure') IS DISTINCT FROM 'number'
          OR (conflict->>'pressure')::double precision NOT BETWEEN 0 AND 1
     )
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_fluctlight_inner_state_affect_ranges_v2'
      AND conrelid='public.fluctlight_inner_states'::regclass
  ) THEN
	    ALTER TABLE public.fluctlight_inner_states ADD CONSTRAINT ck_fluctlight_inner_state_affect_ranges_v2 CHECK (
	      jsonb_typeof(pad)='object'
	      AND pad ?& ARRAY['pleasure','arousal','dominance']
	      AND jsonb_typeof(pad->'pleasure')='number' AND (pad->>'pleasure')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(pad->'arousal')='number' AND (pad->>'arousal')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(pad->'dominance')='number' AND (pad->>'dominance')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(mood)='object'
      AND (NOT (mood ? 'intensity') OR (jsonb_typeof(mood->'intensity')='number' AND (mood->>'intensity')::double precision BETWEEN 0 AND 1))
      AND jsonb_typeof(momentum)='object'
      AND (momentum - ARRAY['value','trend','pleasure_momentum','arousal_momentum','dominance_momentum'])='{}'::jsonb
      AND (NOT (momentum ? 'value') OR (jsonb_typeof(momentum->'value')='number' AND (momentum->>'value')::double precision BETWEEN -1 AND 1))
      AND (NOT (momentum ? 'trend') OR (jsonb_typeof(momentum->'trend')='number' AND (momentum->>'trend')::double precision BETWEEN -1 AND 1))
      AND (NOT (momentum ? 'pleasure_momentum') OR (jsonb_typeof(momentum->'pleasure_momentum')='number' AND (momentum->>'pleasure_momentum')::double precision BETWEEN -1 AND 1))
      AND (NOT (momentum ? 'arousal_momentum') OR (jsonb_typeof(momentum->'arousal_momentum')='number' AND (momentum->>'arousal_momentum')::double precision BETWEEN -1 AND 1))
      AND (NOT (momentum ? 'dominance_momentum') OR (jsonb_typeof(momentum->'dominance_momentum')='number' AND (momentum->>'dominance_momentum')::double precision BETWEEN -1 AND 1))
      AND jsonb_typeof(regulation)='object'
      AND (NOT (regulation ? 'stress') OR (jsonb_typeof(regulation->'stress')='number' AND (regulation->>'stress')::double precision BETWEEN 0 AND 1))
      AND (NOT (regulation ? 'stability') OR (jsonb_typeof(regulation->'stability')='number' AND (regulation->>'stability')::double precision BETWEEN 0 AND 1))
      AND public.fluctlight_drive_state_valid(drives)
      AND public.fluctlight_drive_conflicts_valid(conflicts)
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_fluctlight_affect_profile_policy_v2'
      AND conrelid='public.fluctlight_affect_profiles'::regclass
  ) THEN
	    ALTER TABLE public.fluctlight_affect_profiles ADD CONSTRAINT ck_fluctlight_affect_profile_policy_v2 CHECK (
	      jsonb_typeof(baseline_pad)='object'
	      AND baseline_pad ?& ARRAY['pleasure','arousal','dominance']
      AND jsonb_typeof(baseline_pad->'pleasure')='number' AND (baseline_pad->>'pleasure')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(baseline_pad->'arousal')='number' AND (baseline_pad->>'arousal')::double precision BETWEEN -1 AND 1
      AND jsonb_typeof(baseline_pad->'dominance')='number' AND (baseline_pad->>'dominance')::double precision BETWEEN -1 AND 1
	      AND jsonb_typeof(decay_policy)='object'
	      AND decay_policy ?& ARRAY['pad_half_life_seconds','momentum_half_life_seconds','mood_half_life_seconds','drive_half_life_seconds']
      AND jsonb_typeof(decay_policy->'pad_half_life_seconds')='number' AND (decay_policy->>'pad_half_life_seconds')::double precision BETWEEN 60 AND 2592000
      AND jsonb_typeof(decay_policy->'momentum_half_life_seconds')='number' AND (decay_policy->>'momentum_half_life_seconds')::double precision BETWEEN 60 AND 2592000
      AND jsonb_typeof(decay_policy->'mood_half_life_seconds')='number' AND (decay_policy->>'mood_half_life_seconds')::double precision BETWEEN 60 AND 2592000
      AND jsonb_typeof(decay_policy->'drive_half_life_seconds')='number' AND (decay_policy->>'drive_half_life_seconds')::double precision BETWEEN 60 AND 2592000
	      AND jsonb_typeof(regulation_policy)='object'
	      AND regulation_policy ? 'strength'
      AND jsonb_typeof(regulation_policy->'strength')='number' AND (regulation_policy->>'strength')::double precision BETWEEN 0 AND 1
      AND jsonb_typeof(emotional_summary)='object'
      AND jsonb_typeof(evidence_refs)='array'
      AND revision >= 0
	      AND policy_version='affect.reducer.v2'
    );
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='ck_fluctlight_drive_slot_pressure_v2'
      AND conrelid='public.fluctlight_drive_slots'::regclass
  ) THEN
    ALTER TABLE public.fluctlight_drive_slots ADD CONSTRAINT ck_fluctlight_drive_slot_pressure_v2 CHECK (
      revision >= 0
      AND jsonb_typeof(confidence)='number' AND (confidence #>> '{}')::double precision BETWEEN 0 AND 1
      AND (
        value_schema <> 'pressure' OR (
          jsonb_typeof(value)='object'
          AND value ?& ARRAY['pressure','salience']
          AND jsonb_typeof(value->'pressure')='number' AND (value->>'pressure')::double precision BETWEEN 0 AND 1
          AND jsonb_typeof(value->'salience')='number' AND (value->>'salience')::double precision BETWEEN 0 AND 1
          AND (NOT (value ? 'direction') OR (jsonb_typeof(value->'direction')='string' AND length(value->>'direction') <= 256))
        )
      )
    );
  END IF;
END $$;
`
