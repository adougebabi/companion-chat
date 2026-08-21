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
        const value = {...answers, ...input};
        const name = String(value.name || '新朋友').trim();
        const role = String(value.role || '陪伴者').trim();
        const foundation = String(value.foundation || value.description || `${name}是一位值得慢慢了解的${role}。`).trim().slice(0, 6000);
        return {answers: value, foundation, blueprint: {schemaVersion: 2, timezone: 'Asia/Shanghai', identity: {name, role}, foundation, interests: [], routine: [], world: {defaultSceneRef: {locationId: 'home', roomId: 'private_room'}}}};
    }

    function analyze({description, ...input} = {}) {
        const value = text(description, 'Interview.description');
        const name = input.name ?? value.match(/(?:叫|名为|名字是)\s*([^，。,.]+)/)?.[1] ?? '新朋友';
        const role = input.role ?? '陪伴者';
        return {name: String(name).trim().slice(0, 160), role, foundation: value.slice(0, 6000), confidence: 0.5, inferredFields: ['name', 'role', 'foundation']};
    }

    function createInterview(input = {}) {
        const sessionId = text(input.id ?? nextId('interview'), 'Interview.id');
        const at = input.createdAt ?? now();
        db.prepare(`INSERT INTO companion_interview_sessions (id, answers_json, skipped_json, status, created_at, updated_at, completed_at, source, inferred_fields_json) VALUES (?, ?, ?, 'draft', ?, ?, NULL, ?, ?)`)
            .run(sessionId, JSON.stringify(input.answers ?? {}), JSON.stringify(input.skipped ?? []), at, at, input.source ?? 'modular', JSON.stringify(input.inferredFields ?? []));
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
        const previewValue = preview({answers: current.answers});
        const persona = personaLifecycle.createPersona({...previewValue.answers, foundation: previewValue.foundation, blueprint: previewValue.blueprint, color: input.color});
        const at = input.updatedAt ?? now();
        db.prepare(`UPDATE companion_interview_sessions SET status = 'activated', updated_at = ?, completed_at = ? WHERE id = ?`).run(at, at, sessionId);
        return persona;
    }

    return Object.freeze({getInterview, get: getInterview, find: getInterview, findById: getInterview, preview, analyze, createInterview, create: createInterview, start: createInterview, answerInterview, answer: answerInterview, update: answerInterview, activateInterview, activate: activateInterview});
}

export default createInterviewRepository;
