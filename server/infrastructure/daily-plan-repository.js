function dbFor(database) { if (!database || typeof database.prepare !== 'function') throw new TypeError('Daily-plan repository requires an open database'); return database; }
function parse(value) { try { return value ? JSON.parse(value) : []; } catch { return []; } }
export function createDailyPlanRepository({database} = {}) {
    const db = dbFor(database);
    function read({personaId, id, at} = {}) {
        const owner = personaId ?? id;
        const row = db.prepare(`SELECT * FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1`).get(owner);
        return row ? {...row, items: parse(row.plan_json), plan: parse(row.plan_json)} : null;
    }
    return Object.freeze({read, findReady: read, find: read, get: read});
}
export default createDailyPlanRepository;
