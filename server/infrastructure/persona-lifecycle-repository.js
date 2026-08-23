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

function localDateFor(value, timezone = 'UTC') {
    try {
        const parts = new Intl.DateTimeFormat('en-CA', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit'}).formatToParts(new Date(value));
        const fields = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, part.value]));
        return `${fields.year}-${fields.month}-${fields.day}`;
    } catch {
        return new Date(value).toISOString().slice(0, 10);
    }
}

function nextDate(planDate) {
    const value = new Date(`${planDate}T00:00:00.000Z`);
    value.setUTCDate(value.getUTCDate() + 1);
    return value.toISOString().slice(0, 10);
}

function zonedInstant(planDate, clockText, timezone = 'UTC') {
    const match = /^(\d{2}):(\d{2})$/.exec(String(clockText || ''));
    if (!match) return null;
    const hour = Number(match[1]);
    const minute = Number(match[2]);
    if (hour > 24 || minute > 59 || (hour === 24 && minute !== 0)) return null;
    const target = hour === 24
        ? new Date(`${nextDate(planDate)}T00:00:00.000Z`).getTime()
        : Date.UTC(Number(planDate.slice(0, 4)), Number(planDate.slice(5, 7)) - 1, Number(planDate.slice(8, 10)), hour, minute);
    let candidate = target;
    for (let index = 0; index < 3; index += 1) {
        const parts = new Intl.DateTimeFormat('en-US', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false}).formatToParts(new Date(candidate));
        const fields = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, Number(part.value)]));
        const represented = Date.UTC(fields.year, fields.month - 1, fields.day, fields.hour === 24 ? 0 : fields.hour, fields.minute);
        candidate += target - represented;
    }
    return new Date(candidate).toISOString();
}

function initialDailyBaseline({blueprint, planId, planDate}) {
    const world = blueprint?.world && typeof blueprint.world === 'object' ? blueprint.world : {};
    const locations = Array.isArray(world.locations) ? world.locations : [];
    const defaultRef = world.defaultSceneRef ?? blueprint.defaultSceneRef ?? null;
    const location = locations.find(item => item?.id === defaultRef?.locationId) ?? locations.find(item => item?.isDefault) ?? locations[0] ?? {};
    const rooms = Array.isArray(location.rooms) ? location.rooms : [];
    const room = rooms.find(item => item?.id === defaultRef?.roomId) ?? rooms.find(item => item?.isDefault) ?? rooms[0] ?? {};
    const scene = String(room.scene ?? location.scene ?? blueprint.defaultScene ?? '日常场景').trim() || '日常场景';
    const locationName = String(location.name ?? location.title ?? '家中').trim() || '家中';
    const roomName = String(room.name ?? room.title ?? '自己的房间').trim() || '自己的房间';
    const timezone = typeof blueprint?.timezone === 'string' && blueprint.timezone.trim() ? blueprint.timezone : 'UTC';
    const startsAt = zonedInstant(planDate, '00:00', timezone) ?? `${planDate}T00:00:00.000Z`;
    const endsAt = zonedInstant(planDate, '24:00', timezone) ?? `${nextDate(planDate)}T00:00:00.000Z`;
    return {
        id: `${planId}:baseline:initial`,
        slotKey: `${planId}:baseline:initial`,
        slotKind: 'baseline_idle',
        title: '日常休息',
        situation: '正在自己的空间里休息',
        scene,
        sceneRef: defaultRef,
        location: locationName,
        room: roomName,
        startsAt,
        endsAt,
        planDate,
        source: 'daily_plan_baseline',
        status: 'confirmed',
        priority: 0,
        constraints: {title: '日常休息', situation: '正在自己的空间里休息', scene, sceneRef: defaultRef, location: locationName, room: roomName},
        outcome: {}
    };
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
        const dailyPlanId = nextId('daily_plan');
        const group = db.prepare(`SELECT * FROM companion_groups WHERE is_default = 1 ORDER BY created_at, id LIMIT 1`).get();
        if (!group) throw new Error('默认分组不存在');
        const blueprintValue = input.blueprint && typeof input.blueprint === 'object' ? input.blueprint : (blueprintFactory?.(input) ?? defaultBlueprint(name, role, foundationText));
        const timezone = typeof blueprintValue.timezone === 'string' && blueprintValue.timezone.trim() ? blueprintValue.timezone : 'Asia/Shanghai';
        const planDate = localDateFor(createdAt, timezone);
        const initialBaseline = initialDailyBaseline({blueprint: blueprintValue, planId: dailyPlanId, planDate});
        const initialPlan = {
            schemaVersion: 1,
            timezone,
            planDate,
            items: [],
            timeline: [initialBaseline]
        };
        db.transaction(() => {
            db.prepare(`INSERT INTO companion_personas (id, name, role, color, group_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`).run(personaId, name, role, color, group.id, createdAt, createdAt);
            db.prepare(`INSERT INTO companion_persona_foundation_revisions (id, persona_id, version, foundation, reason, created_at) VALUES (?, ?, 1, ?, ?, ?)`).run(nextId('foundation'), personaId, foundationText, '初始化人格', createdAt);
            db.prepare(`INSERT INTO companion_persona_life_blueprints (persona_id, blueprint_json, created_at, updated_at) VALUES (?, ?, ?, ?)`).run(personaId, JSON.stringify(blueprintValue), createdAt, createdAt);
            if (db.prepare('SELECT name FROM sqlite_master WHERE type = \'table\' AND name = \'companion_persona_life_blueprint_revisions\'').get()) {
                db.prepare(`INSERT INTO companion_persona_life_blueprint_revisions (id, persona_id, version, blueprint_json, reason, schema_version, source, prompt_version, model, used_fallback, validation_warnings_json, created_at) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`).run(nextId('life_blueprint'), personaId, JSON.stringify(blueprintValue), '初始化生活模型', Number(blueprintValue.schemaVersion || 2), 'modular-default', null, null, 1, JSON.stringify([]), createdAt);
            }
            db.prepare(`INSERT INTO companion_persona_states (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at) VALUES (?, ?, ?, '{}', ?, ?)`).run(personaId, '正在开始自己的日常', '平静', createdAt, createdAt);
            db.prepare(`INSERT INTO companion_conversations (id, persona_id, created_at, updated_at) VALUES (?, ?, ?, ?)`).run(nextId('conversation'), personaId, createdAt, createdAt);
            // A new persona must have a durable day-one life-world projection.
            // The maintenance job may later replace the timeline with an LLM
            // plan, but initialization itself cannot leave state resolution
            // dependent on a worker tick.
            db.prepare(`INSERT OR IGNORE INTO companion_daily_plans (id, persona_id, plan_date, status, plan_json, source, created_at, updated_at) VALUES (?, ?, ?, 'ready', ?, 'modular-default', ?, ?)`).run(dailyPlanId, personaId, planDate, JSON.stringify(initialPlan), createdAt, createdAt);
            db.prepare(`INSERT OR IGNORE INTO companion_timeline_slots (id, persona_id, plan_date, slot_key, slot_kind, starts_at, ends_at, status, source, priority, schedule_id, plan_revision, constraints_json, outcome_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`).run(
                initialBaseline.id, personaId, planDate, initialBaseline.slotKey, initialBaseline.slotKind,
                initialBaseline.startsAt, initialBaseline.endsAt, initialBaseline.status, initialBaseline.source,
                initialBaseline.priority, null, 1, JSON.stringify(initialBaseline.constraints), JSON.stringify(initialBaseline.outcome), createdAt, createdAt
            );
        })();
        jobRepository?.enqueue?.({id: nextId('job'), jobType: 'daily_plan', personaId, priority: 2, maxAttempts: 12, runAfter: createdAt, payload: {dailyPlanId, planDate}});
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
