import {randomUUID} from 'node:crypto';

function databaseFor(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('Timeline repository requires an open database');
    return database;
}
function required(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be non-empty`);
    return value.trim();
}
function json(value) { return typeof value === 'string' ? value : JSON.stringify(value ?? {}); }
function clockFor(clock) {
    if (typeof clock === 'function') return clock;
    if (clock?.now) return clock.now.bind(clock);
    return () => new Date().toISOString();
}

export function createTimelineRepository({database, clock, id} = {}) {
    const db = databaseFor(database);
    const now = clockFor(clock);
    const nextId = typeof id === 'function' ? id : prefix => `${prefix}_${randomUUID()}`;
    function findByDecisionKey({personaId, decisionKey} = {}) {
        return db.prepare('SELECT * FROM companion_event_decisions WHERE persona_id = ? AND decision_key = ?').get(required(personaId, 'Persona.id'), required(decisionKey, 'Timeline decisionKey'));
    }
    function insertDecision(input = {}) {
        const createdAt = input.createdAt ?? now();
        const idValue = required(input.id ?? nextId('decision'), 'Timeline decision.id');
        db.prepare(`
            INSERT INTO companion_event_decisions
                (id, persona_id, slot_id, decision_key, decision_type, status, run_at, expires_at, priority, preemption_mode, candidate_json, rationale_json, event_id, job_id, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(idValue, required(input.personaId, 'Persona.id'), input.slotId ?? null, required(input.decisionKey, 'Timeline decisionKey'), required(input.decisionType, 'Timeline decisionType'), required(input.status, 'Timeline decision.status'), input.runAt ?? null, input.expiresAt ?? null, Number(input.priority ?? 0), input.preemptionMode ?? 'none', json(input.candidate), json(input.rationale), input.eventId ?? null, input.jobId ?? null, createdAt, input.updatedAt ?? createdAt);
        return db.prepare('SELECT * FROM companion_event_decisions WHERE id = ? AND persona_id = ?').get(idValue, input.personaId);
    }
    function updateDecision(input = {}) {
        const idValue = required(input.id ?? input.decisionId, 'Timeline decision.id');
        const fields = [];
        const values = [];
        const mappings = [
            ['slot_id', 'slotId'], ['status', 'status'], ['run_at', 'runAt'], ['expires_at', 'expiresAt'],
            ['priority', 'priority'], ['preemption_mode', 'preemptionMode'], ['candidate_json', 'candidate'],
            ['rationale_json', 'rationale'], ['event_id', 'eventId'], ['job_id', 'jobId']
        ];
        for (const [column, key] of mappings) {
            if (input[key] === undefined) continue;
            fields.push(`${column} = ?`);
            values.push(column.endsWith('_json') ? json(input[key]) : input[key]);
        }
        fields.push('updated_at = ?');
        values.push(input.updatedAt ?? now());
        values.push(idValue, required(input.personaId, 'Persona.id'));
        db.prepare(`UPDATE companion_event_decisions SET ${fields.join(', ')} WHERE id = ? AND persona_id = ?`).run(...values);
        return db.prepare('SELECT * FROM companion_event_decisions WHERE id = ? AND persona_id = ?').get(idValue, input.personaId);
    }
    function findByKey({personaId, planDate, slotKey} = {}) {
        return db.prepare('SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? AND slot_key = ?').get(required(personaId, 'Persona.id'), required(planDate, 'Timeline planDate'), required(slotKey, 'Timeline slotKey'));
    }
    function list({personaId, at, planDate} = {}) {
        const owner = required(personaId, 'Persona.id');
        if (planDate) return db.prepare('SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? ORDER BY starts_at, id').all(owner, planDate);
        // Advancement must see expired confirmed/active rows so they can
        // converge to skipped/completed instead of disappearing first.
        return db.prepare("SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND status IN ('confirmed', 'active') ORDER BY starts_at, id LIMIT 100").all(owner);
    }
    function upsertSlot(input = {}) {
        const idValue = required(input.id ?? nextId('slot'), 'Timeline slot.id');
        const createdAt = input.createdAt ?? now();
        const constraints = {
            ...(typeof input.constraints === 'object' && input.constraints ? input.constraints : {}),
            ...(input.title ? {title: input.title} : {}), ...(input.situation ? {situation: input.situation} : {}),
            ...(input.scene ? {scene: input.scene} : {}), ...(input.sceneRef ? {sceneRef: input.sceneRef} : {}),
            ...(input.location ? {location: input.location} : {}), ...(input.room ? {room: input.room} : {})
        };
        db.prepare(`
            INSERT INTO companion_timeline_slots
                (id, persona_id, plan_date, slot_key, slot_kind, starts_at, ends_at, status, source, priority, schedule_id, plan_revision, constraints_json, outcome_json, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(persona_id, plan_date, slot_key) DO UPDATE SET
                slot_kind = excluded.slot_kind, starts_at = excluded.starts_at, ends_at = excluded.ends_at,
                status = excluded.status, source = excluded.source, priority = excluded.priority,
                schedule_id = excluded.schedule_id, plan_revision = excluded.plan_revision,
                constraints_json = excluded.constraints_json, outcome_json = excluded.outcome_json, updated_at = excluded.updated_at
        `).run(idValue, required(input.personaId, 'Persona.id'), required(input.planDate, 'Timeline planDate'), required(input.slotKey, 'Timeline slotKey'), input.slotKind ?? 'planned', input.startsAt ?? null, input.endsAt ?? null, input.status ?? 'confirmed', input.source ?? 'daily_plan', Number(input.priority ?? 0), input.scheduleId ?? null, input.planRevision ?? null, json(constraints), json(input.outcome), createdAt, input.updatedAt ?? createdAt);
        return findByKey({personaId: input.personaId, planDate: input.planDate, slotKey: input.slotKey});
    }
    function updateSlot(input = {}) {
        const idValue = required(input.id ?? input.slotId, 'Timeline slot.id');
        const fields = [];
        const values = [];
        const mappings = [['status', 'status'], ['outcome_json', 'outcome'], ['starts_at', 'startsAt'], ['ends_at', 'endsAt']];
        for (const [column, key] of mappings) {
            if (input[key] === undefined) continue;
            fields.push(`${column} = ?`);
            values.push(column === 'outcome_json' ? json(input[key]) : input[key]);
        }
        fields.push('updated_at = ?');
        values.push(input.updatedAt ?? now(), idValue);
        db.prepare(`UPDATE companion_timeline_slots SET ${fields.join(', ')} WHERE id = ? AND persona_id = ?`).run(...values, required(input.personaId, 'Persona.id'));
        return db.prepare('SELECT * FROM companion_timeline_slots WHERE id = ? AND persona_id = ?').get(idValue, input.personaId);
    }
    function createEventLink(input = {}) {
        const idValue = required(input.id ?? nextId('event_link'), 'Timeline event link.id');
        db.prepare('INSERT OR IGNORE INTO companion_event_links (id, persona_id, from_event_id, to_event_id, link_type, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)').run(idValue, required(input.personaId, 'Persona.id'), required(input.fromEventId, 'Timeline fromEventId'), required(input.toEventId ?? input.eventId, 'Timeline toEventId'), input.linkType ?? 'follows', json(input.metadata), input.createdAt ?? now());
        return db.prepare('SELECT * FROM companion_event_links WHERE id = ? AND persona_id = ?').get(idValue, input.personaId);
    }
    function linkEvents(input = {}) {
        if (input.slotId && input.eventId) {
            const slot = db.prepare('SELECT * FROM companion_timeline_slots WHERE id = ? AND persona_id = ?').get(required(input.slotId, 'Timeline slot.id'), required(input.personaId, 'Persona.id'));
            if (!slot) return null;
            let previous = {};
            try { previous = slot.outcome_json ? JSON.parse(slot.outcome_json) : {}; } catch { previous = {}; }
            const outcome = {...previous, eventId: input.eventId, ...(input.decisionId ? {decisionId: input.decisionId} : {})};
            return updateSlot({id: slot.id, personaId: input.personaId, outcome, updatedAt: input.updatedAt ?? now()});
        }
        return createEventLink(input);
    }
    function deleteGeneratedSlots({personaId, planDate, slotKeys = []} = {}) {
        const owner = required(personaId, 'Persona.id');
        const date = required(planDate, 'Timeline planDate');
        const keys = [...new Set((Array.isArray(slotKeys) ? slotKeys : []).filter(value => typeof value === 'string' && value.trim()).map(value => value.trim()))];
        const values = [owner, date];
        const keep = keys.length ? ` AND slot_key NOT IN (${keys.map(() => '?').join(', ')})` : '';
        values.push(...keys);
        return db.prepare(`
            DELETE FROM companion_timeline_slots
            WHERE persona_id = ? AND plan_date = ?
              AND source IN ('daily_plan', 'ai_daily_plan', 'daily_plan_baseline', 'life_model_fixed', 'life_model_flexible', 'life_model_opportunity')
              AND NOT EXISTS (SELECT 1 FROM companion_event_decisions WHERE companion_event_decisions.slot_id = companion_timeline_slots.id)
              AND COALESCE(json_extract(outcome_json, '$.eventId'), '') = ''${keep}
        `).run(...values);
    }
    return Object.freeze({findByDecisionKey, findByKey, list, insertDecision, createDecision: insertDecision, updateDecision, upsertSlot, insertSlot: upsertSlot, updateSlot, markStatus: updateSlot, linkEvents, createEventLink, deleteGeneratedSlots});
}

export default createTimelineRepository;
