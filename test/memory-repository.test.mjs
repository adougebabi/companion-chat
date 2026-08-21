import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createMemoryRepository} from '../server/infrastructure/memory-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_memories (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL,
            memory_key TEXT NOT NULL,
            value TEXT NOT NULL,
            confidence REAL NOT NULL,
            status TEXT NOT NULL,
            source_type TEXT,
            source_id TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            superseded_at TEXT
        );
    `);
    database.prepare(`
        INSERT INTO companion_memories
            (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at, superseded_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('memory_old', 'persona_1', 'old', 'old value', .8, 'active', 'test', null, '2026-08-20T00:00:01.000Z', '2026-08-20T00:00:01.000Z', null);
    database.prepare(`
        INSERT INTO companion_memories
            (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at, superseded_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('memory_new', 'persona_1', 'new', 'new value', .9, 'active', 'test', null, '2026-08-20T00:00:02.000Z', '2026-08-20T00:00:02.000Z', null);
    database.prepare(`
        INSERT INTO companion_memories
            (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at, superseded_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('memory_deleted', 'persona_1', 'deleted', 'deleted value', .9, 'deleted', 'test', null, '2026-08-20T00:00:03.000Z', '2026-08-20T00:00:03.000Z', null);
    database.prepare(`
        INSERT INTO companion_memories
            (id, persona_id, memory_key, value, confidence, status, source_type, source_id, created_at, updated_at, superseded_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run('memory_other', 'persona_2', 'other', 'other value', .9, 'active', 'test', null, '2026-08-20T00:00:04.000Z', '2026-08-20T00:00:04.000Z', null);
    return database;
}

function repositoryFor(database, overrides = {}) {
    return createMemoryRepository({
        database,
        clock: () => '2026-08-20T01:02:03.000Z',
        ...overrides
    });
}

test('memory repository requires an already-open database', () => {
    assert.throws(() => createMemoryRepository(), /open database/);
});

test('listActive is persona-scoped, status-scoped, limited, and returns raw rows', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        assert.deepEqual(repository.listActive({personaId: 'persona_1'}).map(row => row.id), ['memory_new', 'memory_old']);
        assert.deepEqual(repository.listActive({personaId: 'persona_1', limit: 1}).map(row => row.id), ['memory_new']);
        assert.equal(repository.listActive({personaId: 'persona_2'})[0].id, 'memory_other');
        assert.equal(Object.hasOwn(repository.listActive({personaId: 'persona_1'})[0], 'memory_key'), true);
    } finally {
        database.close();
    }
});

test('delete performs a guarded persona-scoped update and uses the injected clock', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const changed = repository.delete({personaId: 'persona_1', memoryId: 'memory_new'});
        assert.equal(changed.changes, 1);
        const deleted = database.prepare('SELECT status, updated_at FROM companion_memories WHERE id = ?').get('memory_new');
        assert.deepEqual(deleted, {status: 'deleted', updated_at: '2026-08-20T01:02:03.000Z'});
        assert.equal(repository.delete({personaId: 'persona_1', memoryId: 'memory_new'}).changes, 0);
        assert.equal(repository.delete({personaId: 'persona_2', memoryId: 'memory_old'}).changes, 0);
        assert.equal(database.prepare('SELECT status FROM companion_memories WHERE id = ?').get('memory_old').status, 'active');
    } finally {
        database.close();
    }
});

test('memory repository validates storage identity/limit but does not own DTO or evolution policy', async () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        assert.throws(() => repository.listActive(), /Persona.id/);
        assert.throws(() => repository.listActive({personaId: 'persona_1', limit: 0}), /positive integer/);
        assert.throws(() => repository.delete({personaId: 'persona_1'}), /Memory.id/);
    } finally {
        database.close();
    }
    const source = await readFile(new URL('../server/infrastructure/memory-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|messageShape|companion_persona_evolutions|findEvolutionPatch/i);
});
