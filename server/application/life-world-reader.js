/**
 * Read the persistence-facing life-world rows and prepare the input contract
 * consumed by the pure life-state resolver.
 *
 * This module deliberately owns no life-state policy. It only validates the
 * persona/time boundary, calls injected readers, and decodes the JSON columns
 * used by the normalized SQLite rows. Repositories remain responsible for
 * persistence queries; the resolver remains responsible for precedence.
 */

const REPOSITORY_NAMES = Object.freeze({
    state: ['stateRepository', 'personaStateRepository', 'state'],
    schedule: ['scheduleRepository', 'schedule'],
    lifeEvent: ['lifeEventRepository', 'lifeEvent', 'life'],
    dailyPlan: ['dailyPlanRepository', 'dailyPlan', 'plan'],
    presence: ['presenceRepository', 'presence']
});

const READER_NAMES = Object.freeze({
    blueprint: ['read', 'readBlueprint', 'forPersona'],
    scene: ['read', 'readScene', 'forPersona']
});

const OBJECT_INPUT = Symbol('lifeWorldReaderObjectInput');

function isRecord(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function clone(value) {
    if (value instanceof Date) return new Date(value.getTime());
    if (Array.isArray(value)) return value.map(clone);
    if (isRecord(value)) return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clone(item)]));
    return value;
}

function fallbackBlueprint(personaId) {
    const label = personaId || '人格';
    return {
        schemaVersion: 2,
        timezone: 'Asia/Shanghai',
        identity: {name: label, role: '陪伴者'},
        interests: [],
        routine: [],
        world: {
            defaultSceneRef: {locationId: 'home', roomId: 'private_room'},
            locations: [{
                id: 'home', name: '家中', isDefault: true,
                rooms: [{id: 'private_room', name: '自己的宿舍房间', scene: `${label}在自己的宿舍房间里，保持自然的日常状态。`}]
            }]
        },
        fixedTimeEvents: [], dailyFlexibleEvents: [], randomPositiveEvents: [], randomNegativeEvents: [], supportingCast: [],
        generation: {source: 'safe-v2-fallback', usedFallback: true, validationWarnings: ['blueprint_reader_missing']}
    };
}

function text(value) {
    return typeof value === 'string' ? value.trim() : '';
}

function requiredPersonaId(value) {
    const personaId = text(value);
    if (!personaId) throw new TypeError('Life-world reader personaId must be a non-empty string');
    return personaId;
}

function parseJson(value, fallback) {
    if (isRecord(value) || Array.isArray(value)) return clone(value);
    if (typeof value !== 'string' || !value.trim()) return clone(fallback);
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? clone(fallback) : clone(parsed);
    } catch {
        return clone(fallback);
    }
}

function rowValue(row, camelName, snakeName) {
    if (!isRecord(row)) return undefined;
    return row[camelName] === undefined ? row[snakeName] : row[camelName];
}

function rowPersonaId(row) {
    return text(rowValue(row, 'personaId', 'persona_id')
        ?? rowValue(row, 'subjectId', 'subject_id')
        ?? rowValue(row, 'ownerPersonaId', 'owner_persona_id'));
}

function belongsToPersona(row, personaId) {
    if (!isRecord(row)) return false;
    const candidate = rowPersonaId(row);
    return !candidate || candidate === personaId;
}

function scopedRow(row, personaId, field) {
    if (row === null || row === undefined) return null;
    if (!isRecord(row)) throw new TypeError(`Life-world reader ${field} must return an object row`);
    if (!belongsToPersona(row, personaId)) return null;
    return clone(row);
}

function scopedRows(rows, personaId, field) {
    if (rows === null || rows === undefined) return [];
    if (!Array.isArray(rows)) throw new TypeError(`Life-world reader ${field} must return an array of rows`);
    return rows.map(row => {
        if (!isRecord(row)) throw new TypeError(`Life-world reader ${field} must return object rows`);
        return row;
    }).filter(row => belongsToPersona(row, personaId)).map(clone);
}

function addAlias(output, camelName, snakeName) {
    if (output[camelName] === undefined && output[snakeName] !== undefined) output[camelName] = clone(output[snakeName]);
}

function normalizeState(row, personaId) {
    const output = scopedRow(row, personaId, 'state');
    if (!output) return null;
    addAlias(output, 'personaId', 'persona_id');
    addAlias(output, 'checkpointAt', 'checkpoint_at');
    addAlias(output, 'updatedAt', 'updated_at');
    addAlias(output, 'sourceEventId', 'source_event_id');
    if (output.appearance === undefined) {
        output.appearance = parseJson(output.appearanceJson ?? output.appearance_json, {});
    }
    if (output.sharedScene === undefined) {
        output.sharedScene = parseJson(output.sharedSceneJson ?? output.shared_scene_json, null);
    }
    if (output.sharedScene && !belongsToPersona(output.sharedScene, personaId)) return null;
    return output;
}

function normalizeSchedule(row, personaId) {
    if (!belongsToPersona(row, personaId)) return null;
    const output = clone(row);
    addAlias(output, 'personaId', 'persona_id');
    addAlias(output, 'id', 'schedule_id');
    addAlias(output, 'startsAt', 'starts_at');
    addAlias(output, 'endsAt', 'ends_at');
    addAlias(output, 'source', 'source_type');
    if (output.details === undefined) output.details = parseJson(output.detailsJson ?? output.details_json, {});
    if (isRecord(output.details) && !belongsToPersona(output.details, personaId)) return null;
    return output;
}

function normalizeLifeEvent(row, personaId) {
    if (!belongsToPersona(row, personaId)) return null;
    const output = clone(row);
    addAlias(output, 'personaId', 'persona_id');
    addAlias(output, 'occurredAt', 'occurred_at');
    addAlias(output, 'resolvesAt', 'resolves_at');
    addAlias(output, 'causationId', 'causation_id');
    if (output.payload === undefined) output.payload = parseJson(output.payloadJson ?? output.payload_json, {});
    if (isRecord(output.payload) && !belongsToPersona(output.payload, personaId)) return null;
    return output;
}

function normalizePlan(row, personaId) {
    if (row === null || row === undefined) return null;
    if (Array.isArray(row)) return {items: clone(row)};
    if (!isRecord(row) || !belongsToPersona(row, personaId)) return null;
    const output = clone(row);
    addAlias(output, 'personaId', 'persona_id');
    addAlias(output, 'planDate', 'plan_date');
    addAlias(output, 'planId', 'plan_id');
    const parsed = parseJson(output.planJson ?? output.plan_json, undefined);
    if (isRecord(parsed) && !belongsToPersona(parsed, personaId)) return null;
    if (Array.isArray(parsed)) {
        if (output.items === undefined) output.items = parsed;
    } else if (isRecord(parsed)) {
        for (const [key, value] of Object.entries(parsed)) {
            if (output[key] === undefined) output[key] = value;
        }
    }
    if (output.items !== undefined) output.items = Array.isArray(output.items) ? clone(output.items) : [];
    if (output.timeline !== undefined) output.timeline = Array.isArray(output.timeline) ? clone(output.timeline) : [];
    if (output.slots !== undefined) output.slots = Array.isArray(output.slots) ? clone(output.slots) : [];
    return output;
}

function normalizePresence(value, personaId) {
    if (value === null || value === undefined) return null;
    if (!isRecord(value)) return null;
    if (!belongsToPersona(value, personaId)) return null;

    const outer = clone(value);
    let nested = outer.sharedScene ?? outer.shared_scene ?? outer.presence ?? outer.sceneSnapshot ?? outer.scene_snapshot;
    if (nested === undefined && (outer.sharedSceneJson !== undefined || outer.shared_scene_json !== undefined)) {
        nested = parseJson(outer.sharedSceneJson ?? outer.shared_scene_json, null);
    }
    if (nested !== undefined && nested !== null) {
        if (!isRecord(nested) || !belongsToPersona(nested, personaId)) return null;
        return {...outer, ...clone(nested)};
    }
    return outer;
}

function resolveClock(clock) {
    if (typeof clock === 'function') return clock;
    if (isRecord(clock) && typeof clock.now === 'function') return clock.now.bind(clock);
    if (clock === undefined) return () => new Date();
    throw new TypeError('Life-world reader clock must be a function or provide now()');
}

function normalizedTime(value, field, fallback) {
    const source = value === undefined ? fallback() : value;
    const date = source instanceof Date ? new Date(source.getTime()) : new Date(source);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`Life-world reader ${field} must be a valid timestamp`);
    return {date, iso: date.toISOString()};
}

function readRequest(first, second, now) {
    const objectInput = isRecord(first) && (
        Object.hasOwn(first, 'personaId') || Object.hasOwn(first, 'persona_id')
        || Object.hasOwn(first, 'at') || Object.hasOwn(first, 'currentTime') || Object.hasOwn(first, 'time')
    );
    const personaId = requiredPersonaId(objectInput ? first.personaId ?? first.persona_id : first);
    const at = objectInput ? first.at ?? first.currentTime ?? first.time : second;
    return {personaId, ...normalizedTime(at, 'time', now)};
}

function resolveRepository(repositories, names, field, optional = true) {
    for (const name of names) {
        if (repositories[name] !== undefined) {
            const repository = repositories[name];
            if (!isRecord(repository) && typeof repository !== 'function') throw new TypeError(`Life-world reader ${field} must be an object or function`);
            return repository;
        }
    }
    if (optional) return null;
    throw new TypeError(`Life-world reader requires ${field}`);
}

function resolveMethod(source, names, field, {optional = false} = {}) {
    if (typeof source === 'function') {
        const callable = (...args) => source(...args);
        callable[OBJECT_INPUT] = /^\s*(?:async\s+)?(?:function\s*)?\(\s*\{|^\s*\{/.test(Function.prototype.toString.call(source));
        return callable;
    }
    if (isRecord(source)) {
        for (const name of names) {
            if (source[name] !== undefined) {
                if (typeof source[name] !== 'function') throw new TypeError(`Life-world reader ${field}.${name} must be a function`);
                const method = source[name];
                const callable = method.bind(source);
                callable[OBJECT_INPUT] = /\(\s*\{/.test(Function.prototype.toString.call(method));
                return callable;
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Life-world reader ${field} must provide ${names.join('() or ')}()`);
}

function syncResult(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Life-world reader ${field} must be synchronous`);
    return value;
}

function invoke(method, input, field, {positional = false} = {}) {
    if (!method) return undefined;
    const result = method[OBJECT_INPUT]
        ? method(input)
        : positional ? method(input.personaId, input) : method(input);
    return syncResult(result, field);
}

function unwrapRows(value, keys, fallback) {
    if (value === null || value === undefined) return fallback;
    if (Array.isArray(value)) return value;
    if (isRecord(value)) {
        for (const key of keys) {
            if (value[key] !== undefined) return value[key];
        }
    }
    return value;
}

function normalizeDailyPlanResult(value, personaId) {
    if (value === null || value === undefined) return {plan: null, projection: null};
    const wrapper = isRecord(value) && (
        value.plan !== undefined || value.dailyPlan !== undefined || value.projection !== undefined || value.dailyPlanProjection !== undefined
    );
    const rawPlan = wrapper ? value.plan ?? value.dailyPlan : value;
    const rawProjection = wrapper ? value.projection ?? value.dailyPlanProjection : null;
    return {
        plan: normalizePlan(rawPlan, personaId),
        projection: normalizePlan(rawProjection, personaId)
    };
}

/**
 * Build the application-side life-world reader.
 *
 * Repositories are intentionally duck-typed so this boundary can be adopted
 * before the schedule, daily-plan, and presence adapters are split out of the
 * legacy root. All methods are synchronous because the current SQLite
 * repositories and the pure resolver are synchronous.
 */
export function createLifeWorldReader({repositories = {}, blueprintReader, sceneReader, clock} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Life-world reader repositories must be an object');
    const readBlueprint = resolveMethod(blueprintReader, READER_NAMES.blueprint, 'blueprintReader');
    const readScene = resolveMethod(sceneReader, READER_NAMES.scene, 'sceneReader', {optional: true});
    const currentTime = resolveClock(clock);

    const stateRepository = resolveRepository(repositories, REPOSITORY_NAMES.state, 'state repository');
    const scheduleRepository = resolveRepository(repositories, REPOSITORY_NAMES.schedule, 'schedule repository');
    const lifeEventRepository = resolveRepository(repositories, REPOSITORY_NAMES.lifeEvent, 'life-event repository');
    const dailyPlanRepository = resolveRepository(repositories, REPOSITORY_NAMES.dailyPlan, 'daily-plan repository');
    const presenceRepository = resolveRepository(repositories, REPOSITORY_NAMES.presence, 'presence repository');

    const readState = resolveMethod(stateRepository, ['read', 'readProjection', 'findState', 'findByPersona', 'get'], 'state repository', {optional: true});
    const listSchedules = resolveMethod(scheduleRepository, ['listActive', 'list', 'read'], 'schedule repository', {optional: true});
    const listLifeEvents = resolveMethod(lifeEventRepository, ['listActive', 'list', 'read'], 'life-event repository', {optional: true});
    const readPlan = resolveMethod(dailyPlanRepository, ['read', 'findReady', 'findByDate', 'find', 'get'], 'daily-plan repository', {optional: true});
    const readPresenceRow = resolveMethod(presenceRepository, ['read', 'find', 'findByPersona', 'get'], 'presence repository', {optional: true});

    function contextFor(personaId, time) {
        return {personaId, at: time.iso, currentTime: clone(time.date)};
    }

    function personaState(request) {
        const row = invoke(readState, contextFor(request.personaId, request), 'state repository', {positional: true});
        const unwrapped = isRecord(row) && row.state !== undefined ? row.state : row;
        return normalizeState(unwrapped, request.personaId);
    }

    function scheduleItems(request) {
        const rows = unwrapRows(
            invoke(listSchedules, contextFor(request.personaId, request), 'schedule repository'),
            ['items', 'scheduleItems', 'schedules'],
            []
        );
        return scopedRows(rows, request.personaId, 'schedule repository')
            .map(row => normalizeSchedule(row, request.personaId))
            .filter(Boolean);
    }

    function lifeEvents(request) {
        const rows = unwrapRows(
            invoke(listLifeEvents, contextFor(request.personaId, request), 'life-event repository'),
            ['items', 'lifeEvents', 'events'],
            []
        );
        return scopedRows(rows, request.personaId, 'life-event repository')
            .map(row => normalizeLifeEvent(row, request.personaId))
            .filter(Boolean);
    }

    function dailyPlan(request) {
        const result = normalizeDailyPlanResult(
            invoke(readPlan, contextFor(request.personaId, request), 'daily-plan repository'),
            request.personaId
        );
        return result;
    }

    function presence(request, state) {
        const rawRepositoryPresence = invoke(
            readPresenceRow,
            {...contextFor(request.personaId, request), state: clone(state)},
            'presence repository',
            {positional: true}
        );
        const rawPresence = isRecord(rawRepositoryPresence) && rawRepositoryPresence.presence !== undefined
            ? rawRepositoryPresence.presence
            : rawRepositoryPresence;
        if (!readScene) return normalizePresence(rawPresence ?? state, request.personaId);
        const scene = invoke(
            readScene,
            {...contextFor(request.personaId, request), state: clone(state), row: clone(rawPresence), presence: clone(rawPresence)},
            'sceneReader',
            {positional: true}
        );
        return normalizePresence(scene, request.personaId);
    }

    function readPersonaState(first, second) {
        return personaState(readRequest(first, second, currentTime));
    }

    function readScheduleItems(first, second) {
        return scheduleItems(readRequest(first, second, currentTime));
    }

    function readLifeEvents(first, second) {
        return lifeEvents(readRequest(first, second, currentTime));
    }

    function readDailyPlan(first, second) {
        return dailyPlan(readRequest(first, second, currentTime)).plan;
    }

    function readPresence(first, second) {
        const request = readRequest(first, second, currentTime);
        return presence(request, personaState(request));
    }

    function readResolverInput(first, second) {
        const request = readRequest(first, second, currentTime);
        const blueprint = invoke(readBlueprint, contextFor(request.personaId, request), 'blueprintReader', {positional: true});
        const effectiveBlueprint = isRecord(blueprint) ? blueprint : fallbackBlueprint(request.personaId);
        const blueprintScope = rowPersonaId(effectiveBlueprint);
        if (blueprintScope && blueprintScope !== request.personaId) {
            throw new TypeError('Life-world reader blueprint does not belong to persona');
        }
        const state = personaState(request);
        const plan = dailyPlan(request);
        return {
            blueprint: clone(effectiveBlueprint),
            personaId: request.personaId,
            scheduleItems: scheduleItems(request),
            lifeEvents: lifeEvents(request),
            dailyPlan: clone(plan.plan),
            dailyPlanProjection: clone(plan.projection),
            presence: presence(request, state),
            currentTime: clone(request.date)
        };
    }

    return Object.freeze({
        readPersonaState,
        readScheduleItems,
        readLifeEvents,
        readDailyPlan,
        readPresence,
        readResolverInput
    });
}

export default createLifeWorldReader;
