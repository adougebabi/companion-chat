function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('Schedule repository requires an open database');
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
    return prefix => `${prefix}_${crypto.randomUUID()}`;
}

function detailsFor(input) {
    return JSON.stringify({
        ...(input.details && typeof input.details === 'object' ? input.details : {}),
        ...(input.scene === undefined ? {} : {scene: input.scene}),
        ...(input.sourceMessageId === undefined ? {} : {sourceMessageId: input.sourceMessageId})
    });
}

export function createScheduleRepository({database, clock, id} = {}) {
    const db = assertDatabase(database);
    const now = clockFor(clock);
    const nextId = idFor(id);

    function findActive({personaId, scheduleId, id: alias} = {}) {
        const owner = text(personaId, 'Persona.id');
        const schedule = text(scheduleId ?? alias, 'Schedule.id');
        return db.prepare(`
            SELECT * FROM companion_schedule_items
            WHERE id = ? AND persona_id = ? AND status = 'active'
        `).get(schedule, owner);
    }

    function listActive({personaId, at} = {}) {
        const owner = text(personaId, 'Persona.id');
        return db.prepare(`
            SELECT * FROM companion_schedule_items
            WHERE persona_id = ? AND status = 'active'
            ORDER BY starts_at, id
        `).all(owner);
    }

    function createSchedule(input = {}) {
        const owner = text(input.personaId, 'Persona.id');
        const scheduleId = text(input.id ?? nextId('schedule'), 'Schedule.id');
        const createdAt = input.createdAt ?? now();
        db.prepare(`
            INSERT INTO companion_schedule_items
                (id, persona_id, kind, title, starts_at, ends_at, status, source, details_json, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)
        `).run(
            scheduleId, owner, text(input.kind ?? 'plan', 'Schedule.kind'), text(input.title, 'Schedule.title'),
            text(input.startsAt, 'Schedule.startsAt'), input.endsAt ?? null,
            text(input.source ?? 'explicit_chat_plan', 'Schedule.source'), detailsFor(input), createdAt, createdAt
        );
        return db.prepare('SELECT * FROM companion_schedule_items WHERE id = ?').get(scheduleId);
    }

    function rescheduleSchedule(input = {}) {
        const owner = text(input.personaId, 'Persona.id');
        const scheduleId = text(input.scheduleId ?? input.id, 'Schedule.id');
        const updatedAt = input.updatedAt ?? now();
        const result = db.prepare(`
            UPDATE companion_schedule_items
            SET title = COALESCE(?, title), starts_at = ?, ends_at = ?, updated_at = ?
            WHERE id = ? AND persona_id = ? AND status = 'active'
        `).run(input.title ?? null, text(input.startsAt, 'Schedule.startsAt'), input.endsAt ?? null, updatedAt, scheduleId, owner);
        if (!result.changes) return null;
        return db.prepare('SELECT * FROM companion_schedule_items WHERE id = ?').get(scheduleId);
    }

    function cancelSchedule(input = {}) {
        const owner = text(input.personaId, 'Persona.id');
        const scheduleId = text(input.scheduleId ?? input.id, 'Schedule.id');
        const at = input.cancelledAt ?? now();
        const result = db.prepare(`
            UPDATE companion_schedule_items SET status = 'cancelled', updated_at = ?
            WHERE id = ? AND persona_id = ? AND status = 'active'
        `).run(at, scheduleId, owner);
        return result.changes ? {id: scheduleId, cancelled: true, cancelledAt: at} : null;
    }

    return Object.freeze({findActive, find: findActive, listActive, list: listActive, createSchedule, create: createSchedule, insert: createSchedule, rescheduleSchedule, reschedule: rescheduleSchedule, update: rescheduleSchedule, cancelSchedule, cancel: cancelSchedule, delete: cancelSchedule});
}

export default createScheduleRepository;
