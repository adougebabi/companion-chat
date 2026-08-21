import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createGroupRepository} from '../server/infrastructure/group-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_groups (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            is_default INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );
        CREATE TABLE companion_personas (
            id TEXT PRIMARY KEY,
            group_id TEXT,
            enabled INTEGER NOT NULL DEFAULT 1,
            deleted_at TEXT,
            updated_at TEXT NOT NULL
        );
    `);
    database.prepare('INSERT INTO companion_groups (id, name, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?)')
        .run('group_default', 'Default', 1, '2026-08-20T00:00:00.000Z', '2026-08-20T00:00:00.000Z');
    database.prepare('INSERT INTO companion_groups (id, name, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?)')
        .run('group_custom', 'Custom', 0, '2026-08-20T00:00:01.000Z', '2026-08-20T00:00:01.000Z');
    database.prepare('INSERT INTO companion_personas (id, group_id, enabled, deleted_at, updated_at) VALUES (?, ?, ?, ?, ?)')
        .run('persona_1', 'group_custom', 1, null, '2026-08-20T00:00:02.000Z');
    database.prepare('INSERT INTO companion_personas (id, group_id, enabled, deleted_at, updated_at) VALUES (?, ?, ?, ?, ?)')
        .run('persona_disabled', 'group_custom', 0, null, '2026-08-20T00:00:03.000Z');
    database.prepare('INSERT INTO companion_personas (id, group_id, enabled, deleted_at, updated_at) VALUES (?, ?, ?, ?, ?)')
        .run('persona_deleted', 'group_custom', 1, '2026-08-20T00:00:04.000Z', '2026-08-20T00:00:04.000Z');
    return database;
}

function repositoryFor(database, overrides = {}) {
    return createGroupRepository({
        database,
        id: prefix => `${prefix}_created`,
        clock: () => '2026-08-20T01:02:03.000Z',
        ...overrides
    });
}

test('group repository requires an already-open database', () => {
    assert.throws(() => createGroupRepository(), /open database/);
});

test('list returns raw groups with active persona counts and defaultGroup/find return raw rows', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        assert.deepEqual(repository.list().map(group => ({id: group.id, count: group.persona_count})), [
            {id: 'group_default', count: 0},
            {id: 'group_custom', count: 1}
        ]);
        assert.equal(repository.defaultGroup().id, 'group_default');
        assert.equal(repository.find('group_custom').name, 'Custom');
        assert.equal(repository.find('missing'), undefined);
    } finally {
        database.close();
    }
});

test('create uses injected id/clock, preserves the caller name, and returns a raw row', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const created = repository.create({name: '  Group with spaces  '});
        assert.equal(created.id, 'group_created');
        assert.equal(created.name, '  Group with spaces  ');
        assert.equal(created.is_default, 0);
        assert.equal(created.created_at, '2026-08-20T01:02:03.000Z');
        assert.equal(created.updated_at, '2026-08-20T01:02:03.000Z');
    } finally {
        database.close();
    }
});

test('assignPersona updates only persona group membership and returns the raw persona row', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const updated = repository.assignPersona({personaId: 'persona_1', groupId: 'group_default'});
        assert.equal(updated.id, 'persona_1');
        assert.equal(updated.group_id, 'group_default');
        assert.equal(updated.updated_at, '2026-08-20T01:02:03.000Z');
        assert.equal(database.prepare('SELECT group_id FROM companion_personas WHERE id = ?').get('persona_disabled').group_id, 'group_custom');
    } finally {
        database.close();
    }
});

test('group repository does not perform name policy, DTO shaping, or legacy/provider imports', async () => {
    const source = await readFile(new URL('../server/infrastructure/group-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|groupShape|personaCount/i);
});
