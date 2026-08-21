import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

import {createActivityRepository} from '../infrastructure/activity-repository.js';
import {createConversationRepository} from '../infrastructure/conversation-repository.js';
import {createGroupRepository} from '../infrastructure/group-repository.js';
import {createJobRepository} from '../infrastructure/job-repository.js';
import {createLifeEventRepository} from '../infrastructure/life-event-repository.js';
import {createMemoryRepository} from '../infrastructure/memory-repository.js';
import {createPendingEventRepository} from '../infrastructure/pending-event-repository.js';
import {createPersonaRepository} from '../infrastructure/persona-repository.js';
import {createRelationshipRepository} from '../infrastructure/relationship-repository.js';
import {createSettingsRepository} from '../infrastructure/settings-repository.js';
import {createProviderRegistry} from '../infrastructure/provider-ports.js';
import {createStartupRuntime} from '../runtime/startup.js';

const DEFAULT_SETTINGS = Object.freeze({
    lmStudioUrl: 'http://127.0.0.1:8000/v1',
    lmStudioApiKey: '',
    model: '',
    comfyUrl: 'http://127.0.0.1:8188',
    imageWorkflow: '',
    videoWorkflow: '',
    imageProvider: 'comfyui',
    videoProvider: 'comfyui',
    h3Executable: 'h3.c',
    h3ModelDir: '',
    h3OutputDir: '',
    h3AllowedRoot: '',
    h3TimeoutMs: 15 * 60_000,
    h3Defaults: {},
    simplifiedMediaMode: false,
    activityReadAt: null
});

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function resolveClock(value) {
    const source = value ?? (() => new Date().toISOString());
    const callback = typeof source === 'function'
        ? source
        : isRecord(source) && typeof source.now === 'function'
            ? source.now.bind(source)
            : null;
    if (!callback) throw new TypeError('Companion test context clock must be a function or provide now()');
    return () => {
        const result = callback();
        if (result instanceof Date) return result.toISOString();
        if (typeof result === 'string' && result.trim() !== '') return result;
        throw new TypeError('Companion test context clock must return a timestamp');
    };
}

function resolveId(value) {
    const source = value ?? (prefix => `${prefix}_test`);
    const callback = typeof source === 'function'
        ? source
        : isRecord(source) && typeof source.next === 'function'
            ? source.next.bind(source)
            : null;
    if (!callback) throw new TypeError('Companion test context idGenerator must be a function or provide next()');
    return prefix => {
        const result = callback(prefix);
        if (typeof result !== 'string' || result.trim() === '') {
            throw new TypeError('Companion test context idGenerator must return a non-empty string');
        }
        return result;
    };
}

function resolveSettings(value) {
    const source = typeof value === 'function' ? value : () => (isRecord(value) ? value : {});
    return () => {
        const custom = source();
        return {...DEFAULT_SETTINGS, ...(isRecord(custom) ? custom : {})};
    };
}

function temporaryDataDir() {
    return mkdtempSync(join(tmpdir(), 'local-ai-companion-context-'));
}

/**
 * Build an isolated SQLite-backed fixture context without importing the
 * legacy server entrypoint. The factory owns only a default temporary data
 * directory; caller-provided paths remain caller-owned during cleanup.
 */
export function createCompanionTestContext({
    dataDir,
    databasePath,
    clock: configuredClock,
    idGenerator: configuredIdGenerator,
    settings: configuredSettings,
    providerAdapters
} = {}) {
    const ownsDataDir = dataDir === undefined;
    const resolvedDataDir = dataDir ?? temporaryDataDir();
    const clock = resolveClock(configuredClock);
    const id = resolveId(configuredIdGenerator);
    const settings = resolveSettings(configuredSettings);
    let startup;
    let cleaned = false;

    try {
        startup = createStartupRuntime({
            dataDir: resolvedDataDir,
            databasePath,
            environment: {DATA_DIR: resolvedDataDir},
            clock,
            id,
            settings
        });

        const {database} = startup;
        const job = createJobRepository({database, clock, id});
        const pendingEvent = createPendingEventRepository({
            database,
            enqueueJob: job.enqueue.bind(job)
        });
        const lifeEvent = createLifeEventRepository({database, clock, id});
        const repositories = Object.freeze({
            conversation: createConversationRepository({database}),
            activity: createActivityRepository({database}),
            job,
            pending: pendingEvent,
            pendingEvent,
            lifeEvent,
            life: lifeEvent,
            persona: createPersonaRepository({database, clock, id}),
            group: createGroupRepository({database, clock, id}),
            memory: createMemoryRepository({database, clock}),
            relationship: createRelationshipRepository({database, clock, id}),
            settings: createSettingsRepository({database, defaults: settings, clock})
        });
        const providers = createProviderRegistry({dryRunAdapters: providerAdapters});

        function createPersona(input = {}) {
            if (!isRecord(input)) throw new TypeError('Companion test persona must be an object');
            const personaId = input.id ?? id('persona');
            const createdAt = input.createdAt ?? clock();
            const updatedAt = input.updatedAt ?? createdAt;
            const name = typeof input.name === 'string' && input.name.trim() ? input.name.trim() : '测试人格';
            const role = typeof input.role === 'string' && input.role.trim() ? input.role.trim() : '陪伴者';
            const color = typeof input.color === 'string' && input.color.trim() ? input.color.trim() : '#888888';
            database.prepare(`
                INSERT INTO companion_personas (id, name, role, color, enabled, created_at, updated_at, group_id)
                VALUES (?, ?, ?, ?, 1, ?, ?, (SELECT id FROM companion_groups WHERE is_default = 1 ORDER BY created_at, id LIMIT 1))
            `).run(personaId, name, role, color, createdAt, updatedAt);
            database.prepare(`
                INSERT INTO companion_persona_states (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at, source_event_id, shared_scene_json)
                VALUES (?, '', '平静', '{}', ?, ?, NULL, '{}')
            `).run(personaId, createdAt, updatedAt);
            return repositories.persona.findActive(personaId);
        }

        function deletePersona(personaId) {
            if (typeof personaId !== 'string' || !personaId.trim()) throw new TypeError('Companion test persona id must be a non-empty string');
            database.transaction(() => {
                database.prepare('DELETE FROM companion_jobs WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_pending_events WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_event_decisions WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_event_links WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_timeline_slots WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_daily_plans WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_activity_media WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)').run(personaId);
                database.prepare('DELETE FROM companion_activity_visibility WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)').run(personaId);
                database.prepare('DELETE FROM companion_activity_comments WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)').run(personaId);
                database.prepare('DELETE FROM companion_activity_reactions WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)').run(personaId);
                database.prepare('DELETE FROM companion_activities WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_messages WHERE conversation_id IN (SELECT id FROM companion_conversations WHERE persona_id = ?)').run(personaId);
                database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_memories WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_persona_evolutions WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_schedule_items WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_life_events WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_persona_life_blueprint_revisions WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_persona_life_blueprints WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_persona_foundation_revisions WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_persona_states WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_supporting_characters WHERE persona_id = ?').run(personaId);
                database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
            })();
            return true;
        }

        const cleanup = () => {
            if (cleaned) return false;
            cleaned = true;
            startup.close();
            if (ownsDataDir) rmSync(resolvedDataDir, {recursive: true, force: true});
            return true;
        };

        return Object.freeze({
            database,
            repositories,
            createPersona,
            deletePersona,
            clock,
            id,
            providers,
            providerRegistry: providers,
            dataDir: resolvedDataDir,
            databasePath: startup.databasePath,
            cleanup
        });
    } catch (error) {
        try {
            startup?.close();
        } finally {
            if (ownsDataDir) rmSync(resolvedDataDir, {recursive: true, force: true});
        }
        throw error;
    }
}

export default createCompanionTestContext;
