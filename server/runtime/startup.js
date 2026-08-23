import {randomUUID} from 'node:crypto';
import {dirname, join} from 'node:path';
import {fileURLToPath} from 'node:url';
import DefaultDatabase from 'better-sqlite3';

import {createSqliteRuntime, SQLITE_RUNTIME_DEFAULT_PRAGMAS} from './sqlite-runtime.js';

const PROJECT_ROOT = dirname(dirname(dirname(fileURLToPath(import.meta.url))));
const DEFAULT_DATA_DIR = join(PROJECT_ROOT, 'data');
const DEFAULT_DATABASE_FILENAME = 'companion.sqlite';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function nonEmptyString(value) {
    return typeof value === 'string' && value.trim() !== '' ? value : null;
}

function firstValue(...values) {
    for (const value of values) {
        const normalized = nonEmptyString(value);
        if (normalized !== null) return normalized;
    }
    return null;
}

function envValue(environment, ...names) {
    for (const name of names) {
        const value = nonEmptyString(environment?.[name]);
        if (value !== null) return value;
    }
    return null;
}

function resolvePaths({environment, dataDir, databasePath, rootDir = PROJECT_ROOT} = {}) {
    if (!isRecord(environment)) throw new TypeError('Startup environment must be an object');
    const resolvedRoot = firstValue(rootDir) || PROJECT_ROOT;
    const resolvedDataDir = firstValue(
        dataDir,
        envValue(environment, 'DATA_DIR', 'COMPANION_DATA_DIR'),
        join(resolvedRoot, 'data')
    );
    const resolvedDatabasePath = firstValue(
        databasePath,
        envValue(environment, 'DATABASE_PATH', 'COMPANION_DATABASE_PATH', 'COMPANION_DB_PATH'),
        join(resolvedDataDir, DEFAULT_DATABASE_FILENAME)
    );
    return Object.freeze({dataDir: resolvedDataDir, databasePath: resolvedDatabasePath});
}

function resolveNow(value) {
    const source = typeof value === 'function'
        ? value
        : isRecord(value) && typeof value.now === 'function'
            ? value.now.bind(value)
            : () => new Date().toISOString();
    return () => {
        const result = source();
        if (result instanceof Date) return result.toISOString();
        if (typeof result === 'string' && result.trim() !== '') return result;
        throw new TypeError('Startup clock must return a non-empty string or Date');
    };
}

function resolveId(value) {
    const source = typeof value === 'function'
        ? value
        : isRecord(value) && typeof value.next === 'function'
            ? value.next.bind(value)
            : prefix => `${prefix}_${randomUUID()}`;
    return prefix => {
        const result = source(prefix);
        if (typeof result !== 'string' || result.trim() === '') {
            throw new TypeError('Startup id generator must return a non-empty string');
        }
        return result;
    };
}

function defaultSettings({environment, dataDir}) {
    return {
        lmStudioUrl: envValue(environment, 'MTPLX_URL', 'LM_STUDIO_URL', 'COMPANION_MTPLX_URL', 'COMPANION_LM_STUDIO_URL') || 'http://127.0.0.1:8000/v1',
        lmStudioApiKey: envValue(environment, 'MTPLX_API_KEY', 'LM_STUDIO_API_KEY', 'COMPANION_MTPLX_API_KEY', 'COMPANION_LM_STUDIO_API_KEY') || '',
        model: envValue(environment, 'MTPLX_MODEL', 'LM_STUDIO_MODEL', 'COMPANION_MTPLX_MODEL', 'COMPANION_LM_STUDIO_MODEL') || '',
        comfyUrl: envValue(environment, 'COMFYUI_URL', 'COMPANION_COMFYUI_URL') || 'http://127.0.0.1:8188',
        imageWorkflow: '',
        videoWorkflow: '',
        imageProvider: 'comfyui',
        videoProvider: 'comfyui',
        h3Executable: envValue(environment, 'H3_EXECUTABLE', 'COMPANION_H3_EXECUTABLE') || 'h3.c',
        h3ModelDir: envValue(environment, 'H3_MODEL_DIR', 'COMPANION_H3_MODEL_DIR') || '',
        h3OutputDir: envValue(environment, 'H3_OUTPUT_DIR', 'COMPANION_H3_OUTPUT_DIR') || join(dataDir, 'media'),
        h3AllowedRoot: envValue(environment, 'H3_ALLOWED_ROOT', 'COMPANION_H3_ALLOWED_ROOT') || '',
        h3TimeoutMs: Number(envValue(environment, 'H3_TIMEOUT_MS', 'COMPANION_H3_TIMEOUT_MS') || 15 * 60_000),
        h3Defaults: {},
        simplifiedMediaMode: false,
        activityReadAt: null
    };
}

function migrationDefinition(version, name, apply) {
    return Object.freeze({version, name, apply});
}

/**
 * Return the companion schema migrations without opening a database. The
 * migration callbacks receive the database from sqlite-runtime and close
 * over only deterministic startup helpers, so this list is independently
 * testable and does not import the legacy server entrypoint.
 */
export function createCompanionMigrations({environment = process.env, dataDir = DEFAULT_DATA_DIR, now, clock, id, idGenerator, settings} = {}) {
    if (!isRecord(environment)) throw new TypeError('Migration environment must be an object');
    const timestamp = resolveNow(now ?? clock);
    const createId = resolveId(id ?? idGenerator);
    const settingsFactory = typeof settings === 'function'
        ? settings
        : () => ({...defaultSettings({environment, dataDir}), ...(isRecord(settings) ? settings : {})});

    return [
        migrationDefinition(1, 'initial-companion-domain', database => {
            database.exec(`
                CREATE TABLE companion_personas (
                    id TEXT PRIMARY KEY, name TEXT NOT NULL, role TEXT NOT NULL, color TEXT NOT NULL,
                    enabled INTEGER NOT NULL DEFAULT 1, screened_at TEXT, created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL, deleted_at TEXT
                );
                CREATE TABLE companion_persona_foundation_revisions (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    version INTEGER NOT NULL, foundation TEXT NOT NULL, reason TEXT NOT NULL,
                    created_at TEXT NOT NULL, UNIQUE(persona_id, version)
                );
                CREATE TABLE companion_persona_life_blueprints (
                    persona_id TEXT PRIMARY KEY REFERENCES companion_personas(id), blueprint_json TEXT NOT NULL,
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
                );
                CREATE TABLE companion_persona_states (
                    persona_id TEXT PRIMARY KEY REFERENCES companion_personas(id), situation TEXT NOT NULL DEFAULT '',
                    mood TEXT NOT NULL DEFAULT '', appearance_json TEXT NOT NULL DEFAULT '{}',
                    checkpoint_at TEXT NOT NULL, updated_at TEXT NOT NULL, source_event_id TEXT
                );
                CREATE TABLE companion_supporting_characters (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id), name TEXT NOT NULL,
                    relationship_kind TEXT NOT NULL, profile_json TEXT NOT NULL DEFAULT '{}', introduced_event_id TEXT,
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
                );
                CREATE INDEX companion_supporting_characters_persona_idx ON companion_supporting_characters(persona_id, created_at);
                CREATE TABLE companion_schedule_items (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id), kind TEXT NOT NULL,
                    title TEXT NOT NULL, starts_at TEXT NOT NULL, ends_at TEXT, status TEXT NOT NULL, source TEXT NOT NULL,
                    details_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
                );
                CREATE INDEX companion_schedule_items_persona_start_idx ON companion_schedule_items(persona_id, starts_at);
                CREATE TABLE companion_life_events (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id), type TEXT NOT NULL,
                    occurred_at TEXT NOT NULL, resolves_at TEXT, causation_id TEXT, payload_json TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                CREATE INDEX companion_life_events_persona_time_idx ON companion_life_events(persona_id, occurred_at DESC, id DESC);
                CREATE TABLE companion_activities (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    event_id TEXT REFERENCES companion_life_events(id), content TEXT NOT NULL,
                    media_mode TEXT NOT NULL DEFAULT 'none', media_status TEXT NOT NULL DEFAULT 'none', created_at TEXT NOT NULL
                );
                CREATE INDEX companion_activities_feed_idx ON companion_activities(created_at DESC, id DESC);
                CREATE INDEX companion_activities_persona_feed_idx ON companion_activities(persona_id, created_at DESC, id DESC);
                CREATE TABLE companion_activity_comments (
                    id TEXT PRIMARY KEY, activity_id TEXT NOT NULL REFERENCES companion_activities(id),
                    parent_comment_id TEXT REFERENCES companion_activity_comments(id), author_kind TEXT NOT NULL,
                    author_persona_id TEXT REFERENCES companion_personas(id),
                    supporting_character_id TEXT REFERENCES companion_supporting_characters(id), content TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                CREATE INDEX companion_activity_comments_activity_idx ON companion_activity_comments(activity_id, created_at, id);
                CREATE TABLE companion_activity_reactions (
                    activity_id TEXT NOT NULL REFERENCES companion_activities(id), actor_kind TEXT NOT NULL,
                    supporting_character_id TEXT REFERENCES companion_supporting_characters(id), created_at TEXT NOT NULL
                );
                CREATE TABLE companion_activity_visibility (
                    activity_id TEXT PRIMARY KEY REFERENCES companion_activities(id), hidden_at TEXT, updated_at TEXT NOT NULL
                );
                CREATE TABLE companion_memories (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id), memory_key TEXT NOT NULL,
                    value TEXT NOT NULL, confidence REAL NOT NULL, status TEXT NOT NULL, source_type TEXT, source_id TEXT,
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, superseded_at TEXT
                );
                CREATE INDEX companion_memories_persona_status_idx ON companion_memories(persona_id, status, updated_at DESC);
                CREATE TABLE companion_persona_evolutions (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id), reason TEXT NOT NULL,
                    evidence_json TEXT NOT NULL, previous_patch TEXT NOT NULL, next_patch TEXT NOT NULL,
                    status TEXT NOT NULL, created_at TEXT NOT NULL, reverted_at TEXT
                );
                CREATE INDEX companion_persona_evolutions_persona_idx ON companion_persona_evolutions(persona_id, created_at DESC);
                CREATE TABLE companion_conversations (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL UNIQUE REFERENCES companion_personas(id),
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
                );
                CREATE TABLE companion_messages (
                    id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES companion_conversations(id), role TEXT NOT NULL,
                    text TEXT NOT NULL, attachments_json TEXT NOT NULL DEFAULT '[]', generation_json TEXT,
                    jobs_json TEXT NOT NULL DEFAULT '[]', proactive_event_id TEXT REFERENCES companion_life_events(id),
                    created_at TEXT NOT NULL, read_at TEXT
                );
                CREATE INDEX companion_messages_conversation_time_idx ON companion_messages(conversation_id, created_at DESC, id DESC);
                CREATE TABLE companion_media_assets (
                    id TEXT PRIMARY KEY, provider TEXT NOT NULL, media_kind TEXT NOT NULL, filename TEXT NOT NULL,
                    subfolder TEXT NOT NULL DEFAULT '', file_type TEXT NOT NULL DEFAULT 'output', locator_json TEXT NOT NULL,
                    created_at TEXT NOT NULL, unavailable_at TEXT, UNIQUE(provider, filename, subfolder, file_type)
                );
                CREATE TABLE companion_activity_media (
                    activity_id TEXT NOT NULL REFERENCES companion_activities(id), media_id TEXT NOT NULL REFERENCES companion_media_assets(id),
                    position INTEGER NOT NULL, PRIMARY KEY(activity_id, media_id)
                );
                CREATE TABLE companion_jobs (
                    id TEXT PRIMARY KEY, job_type TEXT NOT NULL, status TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 0,
                    run_after TEXT NOT NULL, lease_owner TEXT, lease_expires_at TEXT, attempt_count INTEGER NOT NULL DEFAULT 0,
                    max_attempts INTEGER NOT NULL DEFAULT 3, persona_id TEXT REFERENCES companion_personas(id),
                    activity_id TEXT REFERENCES companion_activities(id), message_id TEXT REFERENCES companion_messages(id),
                    trace_id TEXT, payload_json TEXT NOT NULL, result_json TEXT, error TEXT, created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL, completed_at TEXT
                );
                CREATE INDEX companion_jobs_ready_idx ON companion_jobs(status, run_after, priority DESC, created_at);
                CREATE INDEX companion_jobs_lease_idx ON companion_jobs(lease_expires_at);
            `);
        }),
        migrationDefinition(2, 'companion-settings-and-reaction-guard', database => {
            database.exec(`
                CREATE TABLE companion_settings (
                    id INTEGER PRIMARY KEY CHECK(id = 1), payload_json TEXT NOT NULL, updated_at TEXT NOT NULL
                );
                CREATE UNIQUE INDEX companion_user_reaction_once
                    ON companion_activity_reactions(activity_id, actor_kind)
                    WHERE actor_kind = 'user';
            `);
            database.prepare('INSERT OR IGNORE INTO companion_settings (id, payload_json, updated_at) VALUES (1, ?, ?)')
                .run(JSON.stringify(settingsFactory()), timestamp());
        }),
        migrationDefinition(3, 'state-source-and-user-reaction-guard', database => {
            const stateColumns = database.prepare('PRAGMA table_info(companion_persona_states)').all().map(column => column.name);
            if (!stateColumns.includes('source_event_id')) database.exec('ALTER TABLE companion_persona_states ADD COLUMN source_event_id TEXT');
            database.exec(`
                DELETE FROM companion_activity_reactions
                WHERE actor_kind = 'user'
                  AND rowid NOT IN (
                      SELECT MIN(rowid) FROM companion_activity_reactions
                      WHERE actor_kind = 'user' GROUP BY activity_id
                  );
                CREATE UNIQUE INDEX IF NOT EXISTS companion_user_reaction_once
                    ON companion_activity_reactions(activity_id, actor_kind)
                    WHERE actor_kind = 'user';
            `);
        }),
        migrationDefinition(4, 'adaptive-persona-interviews', database => {
            database.exec(`
                CREATE TABLE companion_interview_sessions (
                    id TEXT PRIMARY KEY, answers_json TEXT NOT NULL, skipped_json TEXT NOT NULL DEFAULT '[]',
                    status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT
                );
                CREATE INDEX companion_interview_sessions_status_idx
                    ON companion_interview_sessions(status, updated_at DESC);
            `);
        }),
        migrationDefinition(5, 'focus-and-proactive-query-indexes', database => {
            database.exec(`
                CREATE INDEX companion_messages_role_recent_idx
                    ON companion_messages(role, created_at DESC);
                CREATE INDEX companion_life_events_persona_type_time_idx
                    ON companion_life_events(persona_id, type, occurred_at DESC);
                CREATE INDEX companion_jobs_persona_type_status_idx
                    ON companion_jobs(persona_id, job_type, status, created_at DESC);
            `);
        }),
        migrationDefinition(6, 'persona-ai-daily-plans', database => {
            database.exec(`
                CREATE TABLE companion_daily_plans (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    plan_date TEXT NOT NULL, status TEXT NOT NULL, plan_json TEXT NOT NULL DEFAULT '[]',
                    source TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, plan_date)
                );
                CREATE INDEX companion_daily_plans_persona_date_idx ON companion_daily_plans(persona_id, plan_date DESC);
            `);
        }),
        migrationDefinition(7, 'persona-life-model-timeline', database => {
            database.exec(`
                CREATE TABLE companion_persona_life_blueprint_revisions (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    version INTEGER NOT NULL, blueprint_json TEXT NOT NULL, reason TEXT NOT NULL,
                    schema_version INTEGER NOT NULL, source TEXT NOT NULL, prompt_version TEXT,
                    model TEXT, used_fallback INTEGER NOT NULL DEFAULT 0,
                    validation_warnings_json TEXT NOT NULL DEFAULT '[]', created_at TEXT NOT NULL,
                    UNIQUE(persona_id, version)
                );
                CREATE INDEX companion_life_blueprint_revisions_persona_idx
                    ON companion_persona_life_blueprint_revisions(persona_id, version DESC);
                CREATE TABLE companion_timeline_slots (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    plan_date TEXT NOT NULL, slot_key TEXT NOT NULL, slot_kind TEXT NOT NULL,
                    starts_at TEXT, ends_at TEXT, status TEXT NOT NULL, source TEXT NOT NULL,
                    priority INTEGER NOT NULL DEFAULT 0, schedule_id TEXT REFERENCES companion_schedule_items(id),
                    plan_revision INTEGER, constraints_json TEXT NOT NULL DEFAULT '{}',
                    outcome_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, plan_date, slot_key)
                );
                CREATE INDEX companion_timeline_slots_persona_date_idx
                    ON companion_timeline_slots(persona_id, plan_date, starts_at);
                CREATE TABLE companion_event_decisions (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    slot_id TEXT REFERENCES companion_timeline_slots(id), decision_key TEXT NOT NULL,
                    decision_type TEXT NOT NULL, status TEXT NOT NULL, run_at TEXT, expires_at TEXT,
                    priority INTEGER NOT NULL DEFAULT 0, preemption_mode TEXT NOT NULL DEFAULT 'none',
                    candidate_json TEXT NOT NULL DEFAULT '{}', rationale_json TEXT NOT NULL DEFAULT '{}',
                    event_id TEXT REFERENCES companion_life_events(id), job_id TEXT REFERENCES companion_jobs(id),
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(persona_id, decision_key)
                );
                CREATE INDEX companion_event_decisions_persona_status_idx
                    ON companion_event_decisions(persona_id, status, run_at);
                CREATE TABLE companion_event_links (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    from_event_id TEXT NOT NULL REFERENCES companion_life_events(id),
                    to_event_id TEXT NOT NULL REFERENCES companion_life_events(id), link_type TEXT NOT NULL,
                    metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL,
                    UNIQUE(from_event_id, to_event_id, link_type)
                );
                CREATE INDEX companion_event_links_persona_from_idx
                    ON companion_event_links(persona_id, from_event_id, created_at);
                CREATE TABLE companion_chat_deferred_batches (
                    id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    conversation_id TEXT NOT NULL REFERENCES companion_conversations(id), batch_key TEXT NOT NULL,
                    status TEXT NOT NULL, deliver_at TEXT NOT NULL, decision_json TEXT NOT NULL DEFAULT '{}',
                    message_ids_json TEXT NOT NULL DEFAULT '[]', result_message_id TEXT REFERENCES companion_messages(id),
                    attempt_count INTEGER NOT NULL DEFAULT 0, error TEXT, created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL, completed_at TEXT, UNIQUE(persona_id, batch_key)
                );
                CREATE INDEX companion_chat_deferred_batches_due_idx
                    ON companion_chat_deferred_batches(status, deliver_at);
                CREATE INDEX companion_chat_deferred_batches_persona_idx
                    ON companion_chat_deferred_batches(persona_id, status, created_at DESC);
            `);
        }),
        migrationDefinition(8, 'proactive-pending-events', database => {
            database.exec(`
                CREATE TABLE companion_pending_events (
                    id TEXT PRIMARY KEY,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id),
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    status TEXT NOT NULL,
                    summary TEXT NOT NULL,
                    not_before TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    dedupe_key TEXT NOT NULL,
                    payload_json TEXT NOT NULL DEFAULT '{}',
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    triggered_at TEXT,
                    consumed_at TEXT,
                    cancelled_at TEXT,
                    UNIQUE(persona_id, dedupe_key, not_before)
                );
                CREATE INDEX companion_pending_events_due_idx
                    ON companion_pending_events(persona_id, status, not_before, expires_at);
                ALTER TABLE companion_messages ADD COLUMN proactive_pending_event_id TEXT
                    REFERENCES companion_pending_events(id) ON DELETE SET NULL;
                CREATE INDEX companion_messages_proactive_pending_idx
                    ON companion_messages(proactive_pending_event_id, created_at DESC);
            `);
        }),
        migrationDefinition(9, 'persona-contact-groups', database => {
            database.exec(`
                CREATE TABLE companion_groups (
                    id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE,
                    is_default INTEGER NOT NULL DEFAULT 0 CHECK(is_default IN (0, 1)),
                    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
                );
            `);
            const defaultGroupId = createId('group');
            const createdAt = timestamp();
            database.prepare('INSERT INTO companion_groups (id, name, is_default, created_at, updated_at) VALUES (?, ?, 1, ?, ?)')
                .run(defaultGroupId, '默认', createdAt, createdAt);
            database.exec('CREATE UNIQUE INDEX companion_groups_default_once ON companion_groups(is_default) WHERE is_default = 1');
            database.exec('ALTER TABLE companion_personas ADD COLUMN group_id TEXT REFERENCES companion_groups(id)');
            database.prepare('UPDATE companion_personas SET group_id = ? WHERE group_id IS NULL').run(defaultGroupId);
            database.exec('CREATE INDEX companion_personas_group_created_idx ON companion_personas(group_id, created_at)');
        }),
        migrationDefinition(10, 'natural-language-interview-provenance', database => {
            const columns = database.prepare('PRAGMA table_info(companion_interview_sessions)').all().map(column => column.name);
            if (!columns.includes('source')) database.exec("ALTER TABLE companion_interview_sessions ADD COLUMN source TEXT NOT NULL DEFAULT 'interview'");
            if (!columns.includes('inferred_fields_json')) database.exec("ALTER TABLE companion_interview_sessions ADD COLUMN inferred_fields_json TEXT NOT NULL DEFAULT '[]'");
        }),
        migrationDefinition(11, 'shared-scene-and-image-generation-policy', database => {
            const personaColumns = database.prepare('PRAGMA table_info(companion_personas)').all().map(column => column.name);
            if (!personaColumns.includes('image_generation_policy')) database.exec("ALTER TABLE companion_personas ADD COLUMN image_generation_policy TEXT NOT NULL DEFAULT 'autonomous'");
            database.exec("UPDATE companion_personas SET image_generation_policy = 'autonomous' WHERE image_generation_policy NOT IN ('ask', 'always', 'important', 'user_only', 'autonomous') OR image_generation_policy IS NULL OR image_generation_policy = ''");
            const stateColumns = database.prepare('PRAGMA table_info(companion_persona_states)').all().map(column => column.name);
            if (!stateColumns.includes('shared_scene_json')) database.exec("ALTER TABLE companion_persona_states ADD COLUMN shared_scene_json TEXT NOT NULL DEFAULT '{}'");
            database.exec("UPDATE companion_persona_states SET shared_scene_json = '{}' WHERE shared_scene_json IS NULL OR trim(shared_scene_json) = ''");
        }),
        migrationDefinition(12, 'prompt-run-observability', database => {
            database.exec(`
                CREATE TABLE companion_prompt_runs (
                    id TEXT PRIMARY KEY,
                    persona_id TEXT REFERENCES companion_personas(id) ON DELETE CASCADE,
                    job_id TEXT REFERENCES companion_jobs(id) ON DELETE SET NULL,
                    message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    operation TEXT NOT NULL,
                    status TEXT NOT NULL,
                    model TEXT NOT NULL DEFAULT '',
                    request_json TEXT NOT NULL,
                    error TEXT,
                    created_at TEXT NOT NULL,
                    completed_at TEXT
                );
                CREATE INDEX companion_prompt_runs_recent_idx
                    ON companion_prompt_runs(created_at DESC, id DESC);
                CREATE INDEX companion_prompt_runs_persona_recent_idx
                    ON companion_prompt_runs(persona_id, created_at DESC, id DESC);
                CREATE INDEX companion_prompt_runs_job_idx
                    ON companion_prompt_runs(job_id);
                CREATE INDEX companion_prompt_runs_message_idx
                    ON companion_prompt_runs(message_id);
            `);
        }),
        migrationDefinition(13, 'prompt-run-response-observability', database => {
            const columns = database.prepare('PRAGMA table_info(companion_prompt_runs)').all().map(column => column.name);
            if (!columns.includes('response_json')) database.exec('ALTER TABLE companion_prompt_runs ADD COLUMN response_json TEXT');
        }),
        migrationDefinition(14, 'persona-affect-and-drive-state', database => {
            database.exec(`
                CREATE TABLE companion_persona_affect_states (
                    persona_id TEXT PRIMARY KEY REFERENCES companion_personas(id) ON DELETE CASCADE,
                    pleasure REAL NOT NULL DEFAULT 0,
                    arousal REAL NOT NULL DEFAULT 0,
                    dominance REAL NOT NULL DEFAULT 0,
                    drives_json TEXT NOT NULL DEFAULT '{}',
                    revision INTEGER NOT NULL DEFAULT 0 CHECK(revision >= 0),
                    effective_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    source_event_id TEXT,
                    model_version TEXT NOT NULL DEFAULT 'affect.v1'
                );
                CREATE TABLE companion_persona_affect_events (
                    id TEXT PRIMARY KEY,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    event_type TEXT NOT NULL,
                    effective_at TEXT NOT NULL,
                    causation_id TEXT,
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    idempotency_key TEXT NOT NULL,
                    pleasure_delta REAL NOT NULL,
                    arousal_delta REAL NOT NULL,
                    dominance_delta REAL NOT NULL,
                    drives_delta_json TEXT NOT NULL DEFAULT '{}',
                    payload_json TEXT NOT NULL DEFAULT '{}',
                    model_version TEXT NOT NULL DEFAULT 'affect.v1',
                    created_at TEXT NOT NULL,
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_persona_affect_events_persona_time_idx
                    ON companion_persona_affect_events(persona_id, effective_at DESC, id DESC);
                CREATE INDEX companion_persona_affect_events_persona_causation_idx
                    ON companion_persona_affect_events(persona_id, causation_id, effective_at DESC);
                CREATE INDEX companion_persona_affect_events_source_message_idx
                    ON companion_persona_affect_events(source_message_id, effective_at DESC);
            `);
        }),
        migrationDefinition(15, 'interaction-facts-and-appraisals', database => {
            database.exec(`
                CREATE TABLE companion_interaction_facts (
                    id TEXT PRIMARY KEY,
                    schema_version TEXT NOT NULL,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    fact_type TEXT NOT NULL,
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    causation_id TEXT,
                    idempotency_key TEXT NOT NULL,
                    payload_json TEXT NOT NULL DEFAULT '{}',
                    source TEXT NOT NULL,
                    model_version TEXT,
                    evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_interaction_facts_persona_time_idx
                    ON companion_interaction_facts(persona_id, created_at DESC, id DESC);
                CREATE INDEX companion_interaction_facts_source_message_idx
                    ON companion_interaction_facts(source_message_id, created_at DESC);
                CREATE TABLE companion_appraisals (
                    id TEXT PRIMARY KEY,
                    schema_version TEXT NOT NULL,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    interaction_fact_id TEXT REFERENCES companion_interaction_facts(id) ON DELETE SET NULL,
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    causation_id TEXT,
                    category TEXT NOT NULL,
                    confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
                    rationale TEXT NOT NULL DEFAULT '',
                    evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                    affect_events_json TEXT NOT NULL DEFAULT '[]',
                    drive_signals_json TEXT NOT NULL DEFAULT '[]',
                    idempotency_key TEXT NOT NULL,
                    model_version TEXT,
                    source TEXT NOT NULL DEFAULT 'llm',
                    status TEXT NOT NULL DEFAULT 'candidate',
                    error TEXT,
                    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_appraisals_persona_time_idx
                    ON companion_appraisals(persona_id, created_at DESC, id DESC);
                CREATE INDEX companion_appraisals_interaction_idx
                    ON companion_appraisals(interaction_fact_id, created_at DESC);
                CREATE INDEX companion_appraisals_source_message_idx
                    ON companion_appraisals(source_message_id, created_at DESC);
            `);
        }),
        migrationDefinition(16, 'memory-consolidation-candidate-ledger', database => {
            database.exec(`
                CREATE TABLE companion_memory_consolidation_candidates (
                    id TEXT PRIMARY KEY,
                    schema_version TEXT NOT NULL,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    layer TEXT NOT NULL,
                    memory_key TEXT,
                    value_json TEXT,
                    claim TEXT,
                    confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
                    evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                    source_fact_refs_json TEXT NOT NULL DEFAULT '[]',
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    causation_id TEXT,
                    idempotency_key TEXT NOT NULL,
                    source TEXT NOT NULL DEFAULT 'llm',
                    model_version TEXT,
                    interaction_fact_id TEXT REFERENCES companion_interaction_facts(id) ON DELETE SET NULL,
                    status TEXT NOT NULL DEFAULT 'candidate',
                    error TEXT,
                    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    CHECK ((memory_key IS NOT NULL AND value_json IS NOT NULL AND claim IS NULL) OR (memory_key IS NULL AND value_json IS NULL AND claim IS NOT NULL)),
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_memory_consolidation_persona_status_idx
                    ON companion_memory_consolidation_candidates(persona_id, status, created_at DESC, id DESC);
                CREATE INDEX companion_memory_consolidation_source_message_idx
                    ON companion_memory_consolidation_candidates(source_message_id, created_at DESC);
                CREATE INDEX companion_memory_consolidation_layer_idx
                    ON companion_memory_consolidation_candidates(persona_id, layer, created_at DESC);
            `);
        }),
        migrationDefinition(17, 'self-model-claim-ledger', database => {
            database.exec(`
                CREATE TABLE companion_self_model_claims (
                    id TEXT PRIMARY KEY,
                    schema_version TEXT NOT NULL,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    category TEXT NOT NULL,
                    claim TEXT NOT NULL,
                    summary TEXT NOT NULL,
                    confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
                    evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                    source TEXT NOT NULL DEFAULT 'llm',
                    uncertainty_json TEXT,
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    causation_id TEXT,
                    idempotency_key TEXT NOT NULL,
                    model_version TEXT,
                    interaction_fact_id TEXT REFERENCES companion_interaction_facts(id) ON DELETE SET NULL,
                    decay_policy_json TEXT,
                    status TEXT NOT NULL DEFAULT 'candidate',
                    error TEXT,
                    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_self_model_claims_persona_status_idx
                    ON companion_self_model_claims(persona_id, status, created_at DESC, id DESC);
                CREATE INDEX companion_self_model_claims_persona_category_idx
                    ON companion_self_model_claims(persona_id, category, created_at DESC, id DESC);
                CREATE INDEX companion_self_model_claims_source_message_idx
                    ON companion_self_model_claims(source_message_id, created_at DESC);
                CREATE INDEX companion_self_model_claims_interaction_fact_idx
                    ON companion_self_model_claims(interaction_fact_id, created_at DESC);
            `);
        }),
        migrationDefinition(18, 'agency-intention-ledger', database => {
            database.exec(`
                CREATE TABLE companion_agency_intentions (
                    id TEXT PRIMARY KEY,
                    schema_version TEXT NOT NULL,
                    persona_id TEXT NOT NULL REFERENCES companion_personas(id) ON DELETE CASCADE,
                    intent TEXT NOT NULL,
                    topic TEXT,
                    explanation TEXT NOT NULL DEFAULT '',
                    reason_category TEXT,
                    confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
                    evidence_refs_json TEXT NOT NULL DEFAULT '[]',
                    source TEXT NOT NULL DEFAULT 'llm',
                    source_message_id TEXT REFERENCES companion_messages(id) ON DELETE SET NULL,
                    causation_id TEXT,
                    idempotency_key TEXT NOT NULL,
                    model_version TEXT,
                    interaction_fact_id TEXT REFERENCES companion_interaction_facts(id) ON DELETE SET NULL,
                    decision_json TEXT,
                    status TEXT NOT NULL DEFAULT 'candidate',
                    error TEXT,
                    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(persona_id, idempotency_key)
                );
                CREATE INDEX companion_agency_intentions_persona_status_idx
                    ON companion_agency_intentions(persona_id, status, created_at DESC, id DESC);
                CREATE INDEX companion_agency_intentions_source_message_idx
                    ON companion_agency_intentions(source_message_id, created_at DESC);
            `);
        }),
        migrationDefinition(19, 'persona-initialization-mode', database => {
            const columns = database.prepare('PRAGMA table_info(companion_personas)').all().map(column => column.name);
            if (!columns.includes('initialization_mode')) database.exec("ALTER TABLE companion_personas ADD COLUMN initialization_mode TEXT NOT NULL DEFAULT 'llm_defined'");
            database.exec("UPDATE companion_personas SET initialization_mode = 'llm_defined' WHERE initialization_mode IS NULL OR initialization_mode NOT IN ('llm_defined', 'blank_slate')");
        })
    ];
}

function resolveEnvironment(environment) {
    if (environment === undefined) return process.env;
    if (!isRecord(environment)) throw new TypeError('Startup environment must be an object');
    return environment;
}

function resolveDatabaseConstructor(Database) {
    const constructor = Database ?? DefaultDatabase;
    if (typeof constructor !== 'function') throw new TypeError('Startup requires a SQLite Database constructor');
    return constructor;
}

/**
 * Build and open the single companion SQLite runtime for the composition
 * root. This factory does not listen, start workers, load providers, or
 * import the legacy application entrypoint. Pass an explicit Database and
 * migration list in tests.
 */
export function createStartupRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Startup options must be an object');
    const environment = resolveEnvironment(options.environment ?? options.env ?? process.env);
    const paths = resolvePaths({
        environment,
        dataDir: options.dataDir,
        databasePath: options.databasePath,
        rootDir: options.rootDir ?? options.root
    });
    const migrations = options.migrations ?? createCompanionMigrations({
        environment,
        dataDir: paths.dataDir,
        now: options.now,
        clock: options.clock,
        id: options.id,
        idGenerator: options.idGenerator,
        settings: options.settings
    });
    if (!Array.isArray(migrations)) throw new TypeError('Startup migrations must be an array');

    const sqliteRuntime = createSqliteRuntime({
        Database: resolveDatabaseConstructor(options.Database ?? options.databaseConstructor),
        dataDir: paths.dataDir,
        databasePath: paths.databasePath,
        migrations,
        pragmas: options.pragmas ?? SQLITE_RUNTIME_DEFAULT_PRAGMAS
    });
    const databaseConfig = Object.freeze({
        dataDir: paths.dataDir,
        databasePath: paths.databasePath,
        pragmas: options.pragmas ?? SQLITE_RUNTIME_DEFAULT_PRAGMAS,
        migrationVersions: Object.freeze(migrations.map(migration => migration.version))
    });
    const runtimeConfig = Object.freeze({
        kind: 'sqlite',
        dataDir: paths.dataDir,
        databasePath: paths.databasePath,
        database: sqliteRuntime.database,
        migrations,
        runMigrations: sqliteRuntime.runMigrations,
        close: sqliteRuntime.close
    });

    return Object.freeze({
        dataDir: paths.dataDir,
        databasePath: paths.databasePath,
        database: sqliteRuntime.database,
        migrations,
        databaseConfig,
        runtime: sqliteRuntime,
        sqliteRuntime,
        runtimeConfig,
        runMigrations: sqliteRuntime.runMigrations,
        close: sqliteRuntime.close
    });
}

export const createServerStartup = createStartupRuntime;
export const createCompanionStartup = createStartupRuntime;
export const createRuntimeConfig = createStartupRuntime;

export const STARTUP_DEFAULT_PATHS = Object.freeze({
    dataDir: DEFAULT_DATA_DIR,
    databaseFilename: DEFAULT_DATABASE_FILENAME
});

export default createStartupRuntime;
