const MAX_ROWS = 5_000;
const MAX_VALUE_LENGTH = 24_000;
const SENSITIVE_KEY = /api[_-]?key|authorization|token|secret|password|credential|cookie/i;
const SENSITIVE_VALUE = /Bearer\s+[^\s,;]+|(?:api[_-]?key|token|secret|password)=[^&\s]+/gi;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function bounded(value, limit = MAX_VALUE_LENGTH) {
    const text = String(value ?? '').replace(SENSITIVE_VALUE, '[redacted]');
    return text.length <= limit ? text : `${text.slice(0, limit - 14)}...[truncated]`;
}

function redact(value, key = '', depth = 0) {
    if (SENSITIVE_KEY.test(key)) return '[redacted]';
    if (depth > 8) return '[depth omitted]';
    if (typeof value === 'string') {
        const binary = value.match(/data:[^,]+;base64,[A-Za-z0-9+/=_-]+/i);
        if (binary) return bounded(value.replace(binary[0], `[binary omitted: ${binary[0].length} chars]`));
        return bounded(value);
    }
    if (Array.isArray(value)) return value.slice(0, 100).map(item => redact(item, '', depth + 1));
    if (isRecord(value)) return Object.fromEntries(Object.entries(value).slice(0, 100).map(([name, item]) => [name, redact(item, name, depth + 1)]));
    return value;
}

function json(value) {
    try { return JSON.stringify(redact(value)); } catch { return JSON.stringify({error: 'value_not_serializable'}); }
}

function parse(value, fallback = null) {
    if (value === null || value === undefined) return fallback;
    try { return redact(JSON.parse(value)); } catch { return redact(value); }
}

function requiredDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Prompt-run repository requires an open database');
    }
    return database;
}

/** Table-scoped, redacted prompt-run storage used by the debug inspector. */
export function createPromptRunRepository({database, clock = () => new Date().toISOString()} = {}) {
    const db = requiredDatabase(database);
    const now = typeof clock === 'function' ? clock : clock.now.bind(clock);

    function start(input = {}) {
        const id = String(input.id || '').trim();
        if (!id) throw new TypeError('Prompt-run id is required');
        const operation = String(input.operation || 'unknown').trim().slice(0, 80) || 'unknown';
        const createdAt = input.createdAt || now();
        db.prepare(`
            INSERT INTO companion_prompt_runs
                (id, persona_id, job_id, message_id, operation, status, model, request_json, error, created_at, completed_at)
            VALUES (?, ?, ?, ?, ?, 'running', ?, ?, NULL, ?, NULL)
        `).run(id, input.personaId || null, input.jobId || null, input.messageId || null, operation, String(input.model || '').slice(0, 160), json(input.request ?? {}), createdAt);
        db.prepare(`DELETE FROM companion_prompt_runs WHERE id IN (
            SELECT id FROM companion_prompt_runs ORDER BY created_at DESC, id DESC LIMIT -1 OFFSET ?
        )`).run(MAX_ROWS);
        return id;
    }

    function finish(id, input = {}) {
        if (!id) return null;
        const status = String(input.status || 'completed').slice(0, 40);
        const error = input.error === undefined || input.error === null ? null : bounded(input.error, 500);
        db.prepare(`
            UPDATE companion_prompt_runs
            SET status = ?, model = COALESCE(?, model), request_json = COALESCE(?, request_json),
                response_json = COALESCE(?, response_json), error = COALESCE(?, error), completed_at = ?
            WHERE id = ?
        `).run(status, input.model ? String(input.model).slice(0, 160) : null, input.request === undefined ? null : json(input.request), input.response === undefined ? null : json(input.response), error, input.completedAt || now(), id);
        return db.prepare('SELECT * FROM companion_prompt_runs WHERE id = ?').get(id) || null;
    }

    function list({personaId = null, limit = 50} = {}) {
        const boundedLimit = Math.min(100, Math.max(1, Math.floor(Number(limit) || 50)));
        const where = personaId ? 'WHERE runs.persona_id = ?' : '';
        const rows = db.prepare(`
            SELECT runs.id, runs.persona_id, personas.name AS persona_name, runs.job_id, runs.message_id,
                runs.operation, runs.status, runs.model, runs.request_json, runs.response_json,
                runs.error, runs.created_at, runs.completed_at
            FROM companion_prompt_runs runs
            LEFT JOIN companion_personas personas ON personas.id = runs.persona_id
            ${where}
            ORDER BY runs.created_at DESC, runs.id DESC LIMIT ?
        `).all(...(personaId ? [personaId, boundedLimit] : [boundedLimit]));
        return rows.map(row => ({
            id: row.id,
            personaId: row.persona_id,
            personaName: row.persona_name || '',
            jobId: row.job_id,
            messageId: row.message_id,
            operation: row.operation,
            status: row.status,
            model: row.model,
            request: parse(row.request_json, {}),
            response: parse(row.response_json, null),
            error: bounded(row.error || '', 2_000),
            createdAt: row.created_at,
            completedAt: row.completed_at
        }));
    }

    return Object.freeze({start, finish, list});
}

export default createPromptRunRepository;
