import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createSettingsRepository} from '../server/infrastructure/settings-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_settings (
            id INTEGER PRIMARY KEY CHECK(id = 1),
            payload_json TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );
    `);
    database.prepare('INSERT INTO companion_settings (id, payload_json, updated_at) VALUES (1, ?, ?)')
        .run('{}', '2026-08-20T00:00:00.000Z');
    return database;
}

function repositoryFor(database, overrides = {}) {
    return createSettingsRepository({
        database,
        defaults: () => ({model: 'default-model', nested: {from: 'defaults'}}),
        clock: () => '2026-08-20T01:02:03.000Z',
        ...overrides
    });
}

test('settings repository requires an already-open database, defaults factory, and clock', () => {
    assert.throws(() => createSettingsRepository(), /open database/);
    const database = createDatabase();
    try {
        assert.throws(() => createSettingsRepository({database, clock: () => 'now'}), /defaults factory/);
        assert.throws(() => createSettingsRepository({database, defaults: () => ({} )}), /clock/);
    } finally {
        database.close();
    }
});

test('read merges persisted settings over a fresh defaults factory result', () => {
    const database = createDatabase();
    let defaultCalls = 0;
    try {
        database.prepare('UPDATE companion_settings SET payload_json = ? WHERE id = 1')
            .run(JSON.stringify({model: 'stored-model', custom: 'kept'}));
        const repository = repositoryFor(database, {
            defaults: () => {
                defaultCalls += 1;
                return {model: 'default-model', fallback: true};
            }
        });
        assert.deepEqual(repository.read(), {
            model: 'stored-model',
            fallback: true,
            custom: 'kept'
        });
        assert.deepEqual(repository.read(), {
            model: 'stored-model',
            fallback: true,
            custom: 'kept'
        });
        assert.equal(defaultCalls, 2);
    } finally {
        database.close();
    }
});

test('write performs a parameterized update and preserves the raw payload contract', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const next = {model: "model with 'quotes'", nested: {enabled: true}};
        assert.strictEqual(repository.write(next), next);
        assert.deepEqual(repository.getRawPayload(), next);
        assert.deepEqual(repository.read(), {
            model: "model with 'quotes'",
            nested: {enabled: true}
        });
        assert.equal(database.prepare('SELECT updated_at FROM companion_settings WHERE id = 1').get().updated_at, '2026-08-20T01:02:03.000Z');
        assert.strictEqual(repository.update, repository.write);
    } finally {
        database.close();
    }
});

test('malformed or non-object persisted payloads do not become settings fields', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        database.prepare('UPDATE companion_settings SET payload_json = ? WHERE id = 1').run('{malformed');
        assert.deepEqual(repository.getRawPayload(), {});
        assert.deepEqual(repository.read(), {model: 'default-model', nested: {from: 'defaults'}});
        database.prepare('UPDATE companion_settings SET payload_json = ? WHERE id = 1').run(JSON.stringify(['unexpected']));
        assert.deepEqual(repository.getRawPayload(), ['unexpected']);
        assert.deepEqual(repository.read(), {model: 'default-model', nested: {from: 'defaults'}});
    } finally {
        database.close();
    }
});

test('settings repository has no legacy, HTTP, or provider imports', async () => {
    const source = await readFile(new URL('../server/infrastructure/settings-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|\.exec\s*\(/i);
});
