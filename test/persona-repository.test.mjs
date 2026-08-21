import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createPersonaRepository} from '../server/infrastructure/persona-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_personas (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            role TEXT NOT NULL,
            color TEXT NOT NULL,
            enabled INTEGER NOT NULL DEFAULT 1,
            screened_at TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            deleted_at TEXT
        );
    `);
    database.prepare(`
        INSERT INTO companion_personas
            (id, name, role, color, enabled, screened_at, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('persona_a', 'A', 'companion', '#a', 1, null, '2026-08-20T00:00:02.000Z', '2026-08-20T00:00:02.000Z', null);
    database.prepare(`
        INSERT INTO companion_personas
            (id, name, role, color, enabled, screened_at, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('persona_b', 'B', 'companion', '#b', 1, '2026-08-20T00:00:03.000Z', '2026-08-20T00:00:01.000Z', '2026-08-20T00:00:03.000Z', null);
    database.prepare(`
        INSERT INTO companion_personas
            (id, name, role, color, enabled, screened_at, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('persona_disabled', 'Disabled', 'companion', '#c', 0, null, '2026-08-20T00:00:00.000Z', '2026-08-20T00:00:00.000Z', null);
    database.prepare(`
        INSERT INTO companion_personas
            (id, name, role, color, enabled, screened_at, created_at, updated_at, deleted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('persona_deleted', 'Deleted', 'companion', '#d', 1, null, '2026-08-19T00:00:00.000Z', '2026-08-19T00:00:00.000Z', '2026-08-20T00:00:00.000Z');
    return database;
}

test('persona repository requires an already-open database', () => {
    assert.throws(() => createPersonaRepository(), /open database/);
});

test('findActive and listActive exclude disabled/deleted personas and return raw rows', () => {
    const database = createDatabase();
    try {
        const repository = createPersonaRepository({database, clock: () => '2026-08-20T01:00:00.000Z', id: () => 'unused'});
        assert.equal(repository.findActive('persona_a').name, 'A');
        assert.equal(repository.findActive('persona_disabled'), undefined);
        assert.equal(repository.findActive('persona_deleted'), undefined);
        assert.deepEqual(repository.listActive().map(persona => persona.id), ['persona_b', 'persona_a']);
        assert.equal(Object.hasOwn(repository.listActive()[0], 'screened_at'), true);
    } finally {
        database.close();
    }
});

test('updateScreen parameterizes screen state, supports clearing it, and returns the raw row', () => {
    const database = createDatabase();
    try {
        const repository = createPersonaRepository({database, clock: () => '2026-08-20T01:02:03.000Z'});
        const screened = repository.updateScreen({personaId: 'persona_a', screenedAt: '2026-08-20T01:02:03.000Z'});
        assert.equal(screened.screened_at, '2026-08-20T01:02:03.000Z');
        assert.equal(screened.updated_at, '2026-08-20T01:02:03.000Z');
        const restored = repository.updateScreen({personaId: 'persona_a', screenedAt: null, updatedAt: new Date('2026-08-20T02:00:00.000Z')});
        assert.equal(restored.screened_at, null);
        assert.equal(restored.updated_at, '2026-08-20T02:00:00.000Z');
        assert.equal(database.prepare('SELECT screened_at FROM companion_personas WHERE id = ?').get('persona_b').screened_at, '2026-08-20T00:00:03.000Z');
    } finally {
        database.close();
    }
});

test('updateScreen leaves unrelated rows unchanged and validates the storage identity', () => {
    const database = createDatabase();
    try {
        const repository = createPersonaRepository({database, clock: () => '2026-08-20T01:00:00.000Z'});
        assert.equal(repository.updateScreen({personaId: 'missing', screenedAt: null}), undefined);
        assert.throws(() => repository.findActive(''), /Persona.id/);
        assert.throws(() => repository.updateScreen({screenedAt: null}), /Persona.id/);
    } finally {
        database.close();
    }
});

test('persona repository has no legacy, HTTP, provider, or DTO imports', async () => {
    const source = await readFile(new URL('../server/infrastructure/persona-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|summary\s*\(/i);
});
