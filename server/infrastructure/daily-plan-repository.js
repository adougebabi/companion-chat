import {randomUUID} from 'node:crypto';
import {dailyPlanFor, dailyPlanJobKey, dailyPlanJobPayload, clockFor, isPlanDate, localDateFor, timezoneFor} from '../domain/daily-plan-defaults.js';

function dbFor(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('Daily-plan repository requires an open database');
    return database;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    return value.trim();
}

function parse(value) {
    try { return value ? JSON.parse(value) : []; } catch { return []; }
}

function idFor(id) {
    if (typeof id === 'function') return id;
    if (id && typeof id.next === 'function') return id.next.bind(id);
    return prefix => `${prefix}_${randomUUID()}`;
}

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

export function createDailyPlanRepository({database, blueprintRepository, blueprint, jobRepository, clock, id} = {}) {
    const db = dbFor(database);
    const now = clockFor(clock);
    const nextId = idFor(id);

    function readBlueprint(personaId) {
        if (isRecord(blueprint)) return blueprint;
        const reader = blueprintRepository?.read ?? blueprintRepository?.get ?? blueprintRepository?.find;
        if (typeof reader !== 'function') return {};
        return reader.call(blueprintRepository, {personaId}) ?? {};
    }

    function timezoneForPersona(personaId) {
        const value = readBlueprint(personaId);
        if (timezoneFor(value) !== 'UTC' || value?.timezone === 'UTC') return timezoneFor(value);
        const latest = db.prepare('SELECT plan_json FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(personaId);
        const raw = parse(latest?.plan_json);
        return timezoneFor(raw);
    }

    function hydrate(row) {
        if (!row) return null;
        const timeline = db.prepare(`SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? ORDER BY starts_at, id`).all(row.persona_id, row.plan_date).map(slot => {
            let constraints = {};
            try { constraints = slot.constraints_json ? JSON.parse(slot.constraints_json) : {}; } catch { constraints = {}; }
            return {...constraints, ...slot, slotId: slot.id, slotKey: slot.slot_key, constraints};
        });
        const raw = parse(row.plan_json);
        const plan = Array.isArray(raw)
            ? {items: raw, timeline, planDate: row.plan_date, planId: row.id, status: row.status}
            : {...(isRecord(raw) ? raw : {}), timeline, planDate: row.plan_date, planId: row.id, status: row.status};
        return {...row, items: Array.isArray(raw) ? raw : plan.items ?? [], plan, dailyPlan: plan, timeline};
    }

    function planRow({personaId, planDate}) {
        return db.prepare('SELECT * FROM companion_daily_plans WHERE persona_id = ? AND plan_date = ?').get(personaId, planDate);
    }

    function read({personaId, id, dailyPlanId, planDate, plan_date, at} = {}) {
        const owner = personaId === undefined || personaId === null ? null : requiredText(personaId, 'Persona.id');
        const requestedDate = planDate ?? plan_date ?? (at !== undefined && owner
            ? localDateFor(at, timezoneForPersona(owner))
            : null);
        const planId = dailyPlanId ?? (id && owner ? null : id);
        const row = planId
            ? db.prepare(`SELECT * FROM companion_daily_plans WHERE id = ?${owner ? ' AND persona_id = ?' : ''}`).get(...(owner ? [planId, owner] : [planId]))
            : requestedDate
                ? db.prepare('SELECT * FROM companion_daily_plans WHERE persona_id = ? AND plan_date = ?').get(owner, requestedDate)
                : db.prepare('SELECT * FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(owner);
        return hydrate(row);
    }

    function readById(input = {}) {
        return read({personaId: input.personaId ?? input.persona_id, dailyPlanId: input.dailyPlanId ?? input.daily_plan_id ?? input.id});
    }

    function readByDate(input = {}) {
        return read({personaId: input.personaId ?? input.persona_id, planDate: input.planDate ?? input.plan_date, at: input.at});
    }

    function markReady({personaId, dailyPlanId, id, updatedAt} = {}) {
        const planId = requiredText(dailyPlanId ?? id, 'DailyPlan.id');
        const at = updatedAt ?? now();
        const owner = personaId === undefined || personaId === null ? null : requiredText(personaId, 'Persona.id');
        db.prepare(`UPDATE companion_daily_plans SET status = 'ready', updated_at = ? WHERE id = ?${owner ? ' AND persona_id = ?' : ''} AND status IN ('queued', 'processing')`).run(at, planId, ...(owner ? [owner] : []));
        const row = owner
            ? db.prepare('SELECT * FROM companion_daily_plans WHERE id = ? AND persona_id = ?').get(planId, owner)
            : db.prepare('SELECT * FROM companion_daily_plans WHERE id = ?').get(planId);
        return hydrate(row);
    }

    function existingJob({personaId, planDate}) {
        if (!jobRepository) return null;
        const key = dailyPlanJobKey(personaId, planDate);
        if (typeof jobRepository.findByPayload === 'function') {
            const byKey = jobRepository.findByPayload({personaId, jobType: 'daily_plan', path: '$.idempotencyKey', value: key});
            if (byKey) return byKey;
            // Jobs created before the stable key was introduced still carry the
            // target plan date, so replaying them must not create a successor.
            const byDate = jobRepository.findByPayload({personaId, jobType: 'daily_plan', path: '$.planDate', value: planDate});
            if (byDate) return byDate;
        }
        if (typeof jobRepository.listForPersona === 'function') {
            const jobs = jobRepository.listForPersona({personaId, jobTypes: ['daily_plan']}) ?? [];
            return jobs.find(job => {
                try {
                    const payload = typeof job.payload_json === 'string' ? JSON.parse(job.payload_json) : job.payload ?? {};
                    return payload.planDate === planDate || payload.plan_date === planDate;
                } catch { return false; }
            }) ?? null;
        }
        return null;
    }

    function ensureJob({personaId, dailyPlanId, planDate, runAfter}) {
        if (typeof jobRepository?.enqueue !== 'function') return null;
        const existing = existingJob({personaId, planDate});
        if (existing) return existing;
        return jobRepository.enqueue({
            id: nextId('job'),
            jobType: 'daily_plan',
            personaId,
            priority: 2,
            maxAttempts: 12,
            runAfter,
            payload: dailyPlanJobPayload({personaId, dailyPlanId, planDate})
        });
    }

    function insertBaseline({personaId, planDate, planId, blueprintValue, at}) {
        const existing = db.prepare(`
            SELECT * FROM companion_timeline_slots
            WHERE persona_id = ? AND plan_date = ?
              AND (source = 'daily_plan_baseline' OR slot_kind LIKE 'baseline_%')
            ORDER BY starts_at, id LIMIT 1
        `).get(personaId, planDate);
        if (existing) return existing;
        const baseline = dailyPlanFor({blueprint: blueprintValue, planId, planDate}).timeline[0];
        db.prepare(`INSERT OR IGNORE INTO companion_timeline_slots
            (id, persona_id, plan_date, slot_key, slot_kind, starts_at, ends_at, status, source, priority, schedule_id, plan_revision, constraints_json, outcome_json, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            baseline.id, personaId, planDate, baseline.slotKey, baseline.slotKind, baseline.startsAt, baseline.endsAt,
            baseline.status, baseline.source, baseline.priority, null, 1, JSON.stringify(baseline.constraints), JSON.stringify(baseline.outcome), at, at
        );
        return db.prepare('SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? AND slot_key = ?').get(personaId, planDate, baseline.slotKey);
    }

    function ensure({personaId, planDate, plan_date, at, runAfter, blueprint: suppliedBlueprint} = {}) {
        const owner = requiredText(personaId, 'Persona.id');
        const persona = db.prepare('SELECT id FROM companion_personas WHERE id = ? AND enabled = 1 AND deleted_at IS NULL').get(owner);
        if (!persona) throw new Error(`Persona not found: ${owner}`);
        const blueprintValue = isRecord(suppliedBlueprint) ? suppliedBlueprint : readBlueprint(owner);
        const timezone = timezoneFor(blueprintValue);
        const targetDate = planDate ?? plan_date ?? localDateFor(at ?? now(), timezone);
        if (!isPlanDate(targetDate)) throw new TypeError('DailyPlan.planDate must be a valid YYYY-MM-DD date');
        const timestamp = at ?? now();
        let row = null;
        let created = false;
        let job = null;
        const transaction = () => {
            row = planRow({personaId: owner, planDate: targetDate});
            if (!row) {
                const planId = nextId('daily_plan');
                const plan = dailyPlanFor({blueprint: blueprintValue, planId, planDate: targetDate});
                db.prepare(`INSERT OR IGNORE INTO companion_daily_plans (id, persona_id, plan_date, status, plan_json, source, created_at, updated_at)
                    VALUES (?, ?, ?, 'ready', ?, 'modular-default', ?, ?)
                `).run(planId, owner, targetDate, JSON.stringify(plan), timestamp, timestamp);
                row = planRow({personaId: owner, planDate: targetDate});
                created = row?.id === planId;
            }
            if (row?.status === 'ready') insertBaseline({personaId: owner, planDate: targetDate, planId: row.id, blueprintValue, at: timestamp});
            job = row ? ensureJob({personaId: owner, dailyPlanId: row.id, planDate: targetDate, runAfter: runAfter ?? timestamp}) : null;
        };
        if (db.inTransaction || typeof db.transaction !== 'function') transaction();
        else db.transaction(transaction)();
        return {...hydrate(row), created, job};
    }

    return Object.freeze({
        read,
        readByDate,
        readForDate: readByDate,
        readById,
        findById: readById,
        findReady: read,
        find: read,
        get: read,
        ensure,
        ensureDailyPlan: ensure,
        markReady,
        complete: markReady
    });
}

export default createDailyPlanRepository;
