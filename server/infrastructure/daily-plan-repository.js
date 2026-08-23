function dbFor(database) { if (!database || typeof database.prepare !== 'function') throw new TypeError('Daily-plan repository requires an open database'); return database; }
function parse(value) { try { return value ? JSON.parse(value) : []; } catch { return []; } }
export function createDailyPlanRepository({database} = {}) {
    const db = dbFor(database);
    function hydrate(row) {
        if (!row) return null;
        const timeline = db.prepare(`SELECT * FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? ORDER BY starts_at, id`).all(row.persona_id, row.plan_date).map(slot => {
            let constraints = {};
            try { constraints = slot.constraints_json ? JSON.parse(slot.constraints_json) : {}; } catch { constraints = {}; }
            return {...constraints, ...slot, slotId: slot.id, slotKey: slot.slot_key, constraints};
        });
        const raw = parse(row.plan_json);
        const plan = Array.isArray(raw) ? {items: raw, timeline, planDate: row.plan_date, planId: row.id, status: row.status} : {...raw, timeline, planDate: row.plan_date, planId: row.id, status: row.status};
        return {...row, items: Array.isArray(raw) ? raw : raw.items ?? [], plan, dailyPlan: plan, timeline};
    }
    function read({personaId, id, dailyPlanId} = {}) {
        const owner = personaId ?? null;
        const planId = dailyPlanId ?? (id && owner ? null : id);
        const row = planId
            ? db.prepare(`SELECT * FROM companion_daily_plans WHERE id = ?${owner ? ' AND persona_id = ?' : ''}`).get(...(owner ? [planId, owner] : [planId]))
            : db.prepare(`SELECT * FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1`).get(owner);
        return hydrate(row);
    }
    function readById(input = {}) {
        return read({personaId: input.personaId ?? input.persona_id, dailyPlanId: input.dailyPlanId ?? input.daily_plan_id ?? input.id});
    }
    function markReady({dailyPlanId, id, updatedAt} = {}) {
        const planId = dailyPlanId ?? id;
        const at = updatedAt ?? new Date().toISOString();
        db.prepare(`UPDATE companion_daily_plans SET status = 'ready', updated_at = ? WHERE id = ? AND status IN ('queued', 'processing')`).run(at, planId);
        return db.prepare('SELECT * FROM companion_daily_plans WHERE id = ?').get(planId);
    }
    return Object.freeze({read, readById, findById: readById, findReady: read, find: read, get: read, markReady, complete: markReady});
}
export default createDailyPlanRepository;
