import {randomUUID} from 'node:crypto';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') throw new TypeError('Interview repository requires an open database');
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

function decode(value, fallback = {}) {
    if (value && typeof value === 'object') return value;
    try { return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}

function record(value) {
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

function list(value) {
    return Array.isArray(value) ? value : [];
}

export function createInterviewRepository({database, clock, id, personaLifecycle} = {}) {
    const db = assertDatabase(database);
    const now = clockFor(clock);
    const nextId = idFor(id);

    function getInterview({interviewId, id: alias} = {}) {
        const row = db.prepare('SELECT * FROM companion_interview_sessions WHERE id = ?').get(text(interviewId ?? alias, 'Interview.id'));
        if (!row) return null;
        return {...row, answers: decode(row.answers_json, {}), skipped: decode(row.skipped_json, []), inferredFields: decode(row.inferred_fields_json, [])};
    }

    function preview({answers = {}, ...input} = {}) {
        const value = {...record(answers), ...input};
        const name = String(value.name || '新朋友').trim();
        const role = String(value.role || '陪伴者').trim();
        const foundation = String(value.foundation || value.description || `${name}是一位值得慢慢了解的${role}。`).trim().slice(0, 6000);
        const sourceBlueprint = record(value.blueprint);
        const inferredFields = list(value.inferredFields);
        const fieldSources = record(value.fieldSources);
        const interests = list(value.interests ?? sourceBlueprint.interests);
        const routine = list(value.routine ?? sourceBlueprint.routine);
        const supportingCast = list(value.supportingCast ?? sourceBlueprint.supportingCast);
        const blueprint = {
            schemaVersion: 2,
            timezone: sourceBlueprint.timezone || 'Asia/Shanghai',
            ...sourceBlueprint,
            identity: {...record(sourceBlueprint.identity), name, role},
            foundation,
            interests,
            routine,
            supportingCast,
            world: {
                defaultSceneRef: {locationId: 'home', roomId: 'private_room'},
                ...record(sourceBlueprint.world)
            },
            provenance: fieldSources,
            generation: {
                source: value.source === 'llm' ? 'llm' : (sourceBlueprint.generation?.source || 'modular-default'),
                usedFallback: value.source !== 'llm',
                validationWarnings: []
            }
        };
        return {answers: value, foundation, blueprint, inferredFields, fieldSources};
    }

    function createInterview(input = {}) {
        const sessionId = text(input.id ?? nextId('interview'), 'Interview.id');
        const at = input.createdAt ?? now();
        db.prepare(`INSERT INTO companion_interview_sessions (id, answers_json, skipped_json, status, created_at, updated_at, completed_at, source, inferred_fields_json) VALUES (?, ?, ?, 'draft', ?, ?, NULL, ?, ?)`)
            .run(sessionId, JSON.stringify(input.answers ?? {}), JSON.stringify(input.skipped ?? []), at, at, input.source ?? 'modular', JSON.stringify(input.inferredFields ?? []));
        return getInterview({interviewId: sessionId});
    }

    function createReadyInterview(input = {}) {
        const sessionId = text(input.id ?? nextId('interview'), 'Interview.id');
        const at = input.createdAt ?? now();
        const write = () => {
            db.prepare(`INSERT INTO companion_interview_sessions (id, answers_json, skipped_json, status, created_at, updated_at, completed_at, source, inferred_fields_json) VALUES (?, ?, ?, 'ready', ?, ?, NULL, ?, ?)`)
                .run(sessionId, JSON.stringify(input.answers ?? {}), JSON.stringify(input.skipped ?? []), at, at, input.source ?? 'llm', JSON.stringify(input.inferredFields ?? []));
        };
        if (typeof db.transaction === 'function') db.transaction(write)();
        else write();
        return getInterview({interviewId: sessionId});
    }

    function answerInterview(input = {}) {
        const sessionId = text(input.interviewId ?? input.id, 'Interview.id');
        const current = getInterview({interviewId: sessionId});
        if (!current) return null;
        const answers = {...current.answers, ...(input.answers && typeof input.answers === 'object' ? input.answers : {})};
        const at = input.updatedAt ?? now();
        db.prepare(`UPDATE companion_interview_sessions SET answers_json = ?, status = 'ready', updated_at = ?, inferred_fields_json = ? WHERE id = ?`).run(JSON.stringify(answers), at, JSON.stringify(input.inferredFields ?? current.inferredFields ?? []), sessionId);
        return getInterview({interviewId: sessionId});
    }

    function activateInterview(input = {}) {
        const sessionId = text(input.interviewId ?? input.id, 'Interview.id');
        const current = getInterview({interviewId: sessionId});
        if (!current) return null;
        if (current.status !== 'ready' && current.status !== 'draft') throw Object.assign(new Error('访谈尚未准备好激活'), {status: 409});
        if (!personaLifecycle || typeof personaLifecycle.createPersona !== 'function') throw Object.assign(new Error('访谈激活缺少 persona lifecycle port'), {status: 501});
        const overrides = record(input.overrides);
        const directAnswers = record(input.answers);
        const changedFields = new Set([...Object.keys(overrides), ...Object.keys(directAnswers)]);
        const answers = {...current.answers, ...overrides, ...directAnswers};
        const inferredFields = list(current.inferredFields).filter(field => !changedFields.has(field));
        const fieldSources = {...record(current.answers?.fieldSources)};
        for (const field of changedFields) if (fieldSources[field] !== undefined || ['name', 'role', 'foundation', 'interests', 'routine', 'visualBaseline', 'supportingCast', 'languageStyle', 'relationshipNote', 'relationshipKind', 'relationship', 'interactionBoundaries'].includes(field)) fieldSources[field] = 'user';
        answers.inferredFields = inferredFields;
        answers.fieldSources = fieldSources;
        const previewValue = preview({answers, source: current.source});
        const persona = personaLifecycle.createPersona({...previewValue.answers, foundation: previewValue.foundation, blueprint: previewValue.blueprint, color: input.color ?? overrides.color});
        const at = input.updatedAt ?? now();
        db.prepare(`UPDATE companion_interview_sessions SET answers_json = ?, inferred_fields_json = ?, status = 'activated', updated_at = ?, completed_at = ? WHERE id = ?`)
            .run(JSON.stringify(answers), JSON.stringify(inferredFields), at, at, sessionId);
        return persona;
    }

    return Object.freeze({getInterview, get: getInterview, find: getInterview, findById: getInterview, preview, createInterview, createReadyInterview, create: createInterview, start: createInterview, answerInterview, answer: answerInterview, update: answerInterview, activateInterview, activate: activateInterview});
}

export default createInterviewRepository;
