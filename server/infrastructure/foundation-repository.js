import {randomUUID} from 'node:crypto';

function assertDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Foundation repository requires an open database');
    }
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

function row(database, personaId) {
    return database.prepare(`
        SELECT * FROM companion_persona_foundation_revisions
        WHERE persona_id = ? ORDER BY version DESC, created_at DESC, id DESC LIMIT 1
    `).get(personaId);
}

export function createFoundationRepository({database, clock, id} = {}) {
    const db = assertDatabase(database);
    const now = clockFor(clock);
    const nextId = idFor(id);

    function getDraft({personaId} = {}) {
        return row(db, text(personaId, 'Persona.id'));
    }

    function updateFoundation({personaId, foundation, reason = '用户修订基础人格', updatedAt} = {}) {
        const owner = text(personaId, 'Persona.id');
        const value = text(foundation, 'Foundation');
        const at = updatedAt ?? now();
        const current = row(db, owner);
        const version = Number(current?.version || 0) + 1;
        db.transaction(() => {
            db.prepare(`
                INSERT INTO companion_persona_foundation_revisions
                    (id, persona_id, version, foundation, reason, created_at)
                VALUES (?, ?, ?, ?, ?, ?)
            `).run(nextId('foundation'), owner, version, value, String(reason).slice(0, 240), at);
            db.prepare('UPDATE companion_personas SET updated_at = ? WHERE id = ?').run(at, owner);
        })();
        return row(db, owner);
    }

    function restoreFoundationRevision({personaId, revisionId, restoredAt} = {}) {
        const owner = text(personaId, 'Persona.id');
        const revision = db.prepare(`
            SELECT * FROM companion_persona_foundation_revisions
            WHERE id = ? AND persona_id = ?
        `).get(text(revisionId, 'Foundation revision.id'), owner);
        if (!revision) return null;
        const current = row(db, owner);
        if (current?.id === revision.id) return {version: current.version, foundation: current.foundation, restored: false};
        const at = restoredAt ?? now();
        const version = Number(current?.version || 0) + 1;
        db.transaction(() => {
            db.prepare(`
                INSERT INTO companion_persona_foundation_revisions
                    (id, persona_id, version, foundation, reason, created_at)
                VALUES (?, ?, ?, ?, ?, ?)
            `).run(nextId('foundation'), owner, version, revision.foundation, `用户恢复版本 ${revision.version}`, at);
            db.prepare('UPDATE companion_personas SET updated_at = ? WHERE id = ?').run(at, owner);
        })();
        return {version, foundation: revision.foundation, restored: true, createdAt: at};
    }

    return Object.freeze({
        getDraft,
        getCurrent: getDraft,
        draft: getDraft,
        updateFoundation,
        update: updateFoundation,
        createRevision: updateFoundation,
        restoreFoundationRevision,
        restoreRevision: restoreFoundationRevision,
        restore: restoreFoundationRevision
    });
}

export default createFoundationRepository;
