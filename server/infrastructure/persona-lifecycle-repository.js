import {randomUUID} from 'node:crypto';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') throw new TypeError('Persona lifecycle repository requires an open database');
    return database;
}

function text(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be non-empty`);
    return value.trim();
}

function clockFor(clock) {
    if (typeof clock === 'function') return clock;
    if (clock && typeof clock.now === 'function') return clock.now.bind(clock);
    return () => new Date().toISOString();
}

function idFor(id) {
    if (typeof id === 'function') return id;
    if (id && typeof id.next === 'function') return id.next.bind(id);
    return prefix => `${prefix}_${randomUUID()}`;
}

function defaultBlueprint(name, role, foundation) {
    return {
        schemaVersion: 2,
        timezone: 'Asia/Shanghai',
        foundation,
        identity: {name, role},
        interests: [],
        routine: [],
        world: {
            defaultSceneRef: {locationId: 'home', roomId: 'private_room'},
            locations: [{id: 'home', kind: 'home', name: '家中', isDefault: true, rooms: [{id: 'private_room', kind: 'private_room', name: '自己的房间', scene: `${name}在自己的房间里，保持自然的日常状态。`, activityTags: ['rest', 'chat']}]}]
        },
        fixedTimeEvents: [],
        dailyFlexibleEvents: [],
        randomPositiveEvents: [],
        randomNegativeEvents: [],
        supportingCast: [],
        generation: {source: 'modular-default', usedFallback: true, validationWarnings: []}
    };
}

export function createPersonaLifecycleRepository({database, clock, id, foundation, blueprintFactory, jobRepository} = {}) {
    const db = assertDatabase(database);
    const now = clockFor(clock);
    const nextId = idFor(id);

    function getPersona({personaId, id: alias} = {}) {
        return db.prepare(`SELECT * FROM companion_personas WHERE id = ? AND enabled = 1 AND deleted_at IS NULL`).get(text(personaId ?? alias, 'Persona.id'));
    }

    function createPersona(input = {}) {
        const name = text(input.name, 'Persona.name');
        const role = text(input.role, 'Persona.role');
        const foundationText = text(input.foundation ?? `${name}的基础人格设定`, 'Persona.foundation');
        const color = /^#[0-9a-f]{6}$/i.test(String(input.color || '')) ? input.color : '#3593d2';
        const createdAt = input.createdAt ?? now();
        const personaId = text(input.id ?? nextId('persona'), 'Persona.id');
        const group = db.prepare(`SELECT * FROM companion_groups WHERE is_default = 1 ORDER BY created_at, id LIMIT 1`).get();
        if (!group) throw new Error('默认分组不存在');
        const blueprintValue = input.blueprint && typeof input.blueprint === 'object' ? input.blueprint : (blueprintFactory?.(input) ?? defaultBlueprint(name, role, foundationText));
        const planDate = new Date(createdAt).toISOString().slice(0, 10);
        db.transaction(() => {
            db.prepare(`INSERT INTO companion_personas (id, name, role, color, group_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`).run(personaId, name, role, color, group.id, createdAt, createdAt);
            db.prepare(`INSERT INTO companion_persona_foundation_revisions (id, persona_id, version, foundation, reason, created_at) VALUES (?, ?, 1, ?, ?, ?)`).run(nextId('foundation'), personaId, foundationText, '初始化人格', createdAt);
            db.prepare(`INSERT INTO companion_persona_life_blueprints (persona_id, blueprint_json, created_at, updated_at) VALUES (?, ?, ?, ?)`).run(personaId, JSON.stringify(blueprintValue), createdAt, createdAt);
            if (db.prepare('SELECT name FROM sqlite_master WHERE type = \'table\' AND name = \'companion_persona_life_blueprint_revisions\'').get()) {
                db.prepare(`INSERT INTO companion_persona_life_blueprint_revisions (id, persona_id, version, blueprint_json, reason, schema_version, source, prompt_version, model, used_fallback, validation_warnings_json, created_at) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`).run(nextId('life_blueprint'), personaId, JSON.stringify(blueprintValue), '初始化生活模型', Number(blueprintValue.schemaVersion || 2), 'modular-default', null, null, 1, JSON.stringify([]), createdAt);
            }
            db.prepare(`INSERT INTO companion_persona_states (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at) VALUES (?, ?, ?, '{}', ?, ?)`).run(personaId, '正在开始自己的日常', '平静', createdAt, createdAt);
            db.prepare(`INSERT INTO companion_conversations (id, persona_id, created_at, updated_at) VALUES (?, ?, ?, ?)`).run(nextId('conversation'), personaId, createdAt, createdAt);
            db.prepare(`INSERT OR IGNORE INTO companion_daily_plans (id, persona_id, plan_date, status, plan_json, source, created_at, updated_at) VALUES (?, ?, ?, 'queued', '[]', 'modular-default', ?, ?)`).run(nextId('daily_plan'), personaId, planDate, createdAt, createdAt);
        })();
        jobRepository?.enqueue?.({id: nextId('job'), jobType: 'daily_plan', personaId, priority: 2, maxAttempts: 12, runAfter: createdAt, payload: {planDate}});
        return getPersona({personaId});
    }

    function deletePersona({personaId, id: alias} = {}) {
        const owner = text(personaId ?? alias, 'Persona.id');
        const persona = getPersona({personaId: owner});
        if (!persona) return {id: owner, deleted: false, deletedMediaIds: []};
        const media = db.prepare(`SELECT DISTINCT media_id FROM companion_activity_media WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)`).all(owner).map(item => item.media_id);
        db.transaction(() => {
            const statements = [
                'DELETE FROM companion_chat_deferred_batches WHERE persona_id = ?',
                'DELETE FROM companion_event_links WHERE persona_id = ?',
                'DELETE FROM companion_event_decisions WHERE persona_id = ?',
                'DELETE FROM companion_timeline_slots WHERE persona_id = ?',
                'DELETE FROM companion_pending_events WHERE persona_id = ?',
                'DELETE FROM companion_jobs WHERE persona_id = ?',
                'DELETE FROM companion_activity_comments WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)',
                'DELETE FROM companion_activity_reactions WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)',
                'DELETE FROM companion_activity_visibility WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)',
                'DELETE FROM companion_activity_media WHERE activity_id IN (SELECT id FROM companion_activities WHERE persona_id = ?)',
                'DELETE FROM companion_activities WHERE persona_id = ?',
                'DELETE FROM companion_messages WHERE conversation_id IN (SELECT id FROM companion_conversations WHERE persona_id = ?)',
                'DELETE FROM companion_conversations WHERE persona_id = ?',
                'DELETE FROM companion_memories WHERE persona_id = ?',
                'DELETE FROM companion_persona_evolutions WHERE persona_id = ?',
                'DELETE FROM companion_supporting_characters WHERE persona_id = ?',
                'DELETE FROM companion_daily_plans WHERE persona_id = ?',
                'DELETE FROM companion_schedule_items WHERE persona_id = ?',
                'DELETE FROM companion_persona_states WHERE persona_id = ?',
                'DELETE FROM companion_persona_life_blueprints WHERE persona_id = ?',
                'DELETE FROM companion_persona_life_blueprint_revisions WHERE persona_id = ?',
                'DELETE FROM companion_persona_foundation_revisions WHERE persona_id = ?',
                'DELETE FROM companion_life_events WHERE persona_id = ?',
                'DELETE FROM companion_personas WHERE id = ?'
            ];
            for (const sql of statements) db.prepare(sql).run(owner);
            for (const mediaId of media) {
                const references = db.prepare(`SELECT COUNT(*) AS count FROM companion_activity_media WHERE media_id = ?`).get(mediaId).count;
                if (!references) db.prepare('DELETE FROM companion_media_assets WHERE id = ?').run(mediaId);
            }
        })();
        return {id: owner, deleted: true, deletedMediaIds: media};
    }

    return Object.freeze({getPersona, get: getPersona, readPersona: getPersona, createPersona, create: createPersona, deletePersona, delete: deletePersona});
}

export default createPersonaLifecycleRepository;
