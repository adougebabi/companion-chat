import assert from 'node:assert/strict';
import {existsSync, mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import createSqliteRuntime, {SqliteRuntimeError} from '../server/runtime/sqlite-runtime.js';

function temporaryDirectory() {
    return mkdtempSync(join(tmpdir(), 'local-ai-companion-sqlite-runtime-'));
}

function withTemporaryDatabase(callback) {
    const dataDir = temporaryDirectory();
    try {
        return callback(dataDir, join(dataDir, 'companion.sqlite'));
    } finally {
        rmSync(dataDir, {recursive: true, force: true});
    }
}

function migrationsFor(order) {
    return [
        {
            version: 1,
            name: 'create-runtime-fixture',
            apply(database) {
                order.push('one');
                database.exec('CREATE TABLE runtime_fixture (id INTEGER PRIMARY KEY, value TEXT NOT NULL)');
                database.prepare('INSERT INTO runtime_fixture (value) VALUES (?)').run('one');
            }
        },
        {
            version: 2,
            name: 'append-runtime-fixture',
            apply(database) {
                order.push('two');
                database.prepare('INSERT INTO runtime_fixture (value) VALUES (?)').run('two');
            }
        }
    ];
}

test('opens the requested database, applies pragmas, and runs migrations in order', () => {
    withTemporaryDatabase((dataDir, databasePath) => {
        const order = [];
        const runtime = createSqliteRuntime({Database, dataDir, databasePath, migrations: migrationsFor(order)});

        assert.equal(runtime.databasePath, databasePath);
        assert.deepEqual(order, ['one', 'two']);
        assert.equal(runtime.database.pragma('journal_mode', {simple: true}).toLowerCase(), 'wal');
        assert.equal(runtime.database.pragma('foreign_keys', {simple: true}), 1);
        assert.equal(runtime.database.pragma('busy_timeout', {simple: true}), 5000);
        assert.deepEqual(
            runtime.database.prepare('SELECT value FROM runtime_fixture ORDER BY id').all().map(row => row.value),
            ['one', 'two']
        );
        assert.equal(existsSync(databasePath), true);
        runtime.close();
    });
});

test('migration execution is idempotent for repeated runs and reopening the file', () => {
    withTemporaryDatabase((dataDir, databasePath) => {
        const order = [];
        const migrations = migrationsFor(order);
        const first = createSqliteRuntime({Database, dataDir, databasePath, migrations});
        first.runMigrations();
        assert.deepEqual(order, ['one', 'two']);
        assert.equal(first.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 2);
        first.close();

        const second = createSqliteRuntime({Database, dataDir, databasePath, migrations});
        assert.deepEqual(order, ['one', 'two']);
        assert.equal(second.database.prepare('SELECT COUNT(*) AS count FROM runtime_fixture').get().count, 2);
        second.runMigrations();
        assert.deepEqual(order, ['one', 'two']);
        second.close();
    });
});

test('supports an in-memory database and closes idempotently', () => {
    let closeCalls = 0;
    class TrackingDatabase {
        constructor(path) {
            this.database = new Database(path);
        }

        pragma(...args) {
            return this.database.pragma(...args);
        }

        exec(...args) {
            return this.database.exec(...args);
        }

        prepare(...args) {
            return this.database.prepare(...args);
        }

        transaction(...args) {
            return this.database.transaction(...args);
        }

        close() {
            closeCalls += 1;
            return this.database.close();
        }
    }

    const runtime = createSqliteRuntime({Database: TrackingDatabase, databasePath: ':memory:', migrations: []});
    runtime.close();
    runtime.close();
    assert.equal(closeCalls, 1);
    assert.throws(() => runtime.runMigrations(), error => {
        assert.equal(error.code, 'SQLITE_RUNTIME_CLOSED');
        return true;
    });
});

test('rejects missing, malformed, duplicate, and gapped migration definitions', () => {
    const validApply = () => {};
    const invalidDefinitions = [
        undefined,
        [{version: 1, name: 'missing-apply'}],
        [{version: 1, name: '', apply: validApply}],
        [{version: 1, name: 'one', apply: validApply}, {version: 1, name: 'duplicate', apply: validApply}],
        [{version: 1, name: 'one', apply: validApply}, {version: 3, name: 'gap', apply: validApply}]
    ];

    for (const migrations of invalidDefinitions) {
        assert.throws(
            () => createSqliteRuntime({Database, databasePath: ':memory:', migrations}),
            error => error instanceof SqliteRuntimeError && error.code === 'SQLITE_RUNTIME_CONFIG'
        );
    }
});

test('bounds startup errors without retaining migration error details', () => {
    const secret = 'postgres://user:super-secret@example.invalid/database';
    assert.throws(() => createSqliteRuntime({
        Database,
        databasePath: ':memory:',
        migrations: [{
            version: 1,
            name: 'runtime-error',
            apply() {
                throw new Error(secret);
            }
        }]
    }), error => {
        assert.equal(error instanceof SqliteRuntimeError, true);
        assert.equal(error.message.includes(secret), false);
        assert.equal(error.message.length < 240, true);
        assert.equal('cause' in error, false);
        return true;
    });
});

test('has no legacy server, network, or provider imports', () => {
    const source = readFileSync(new URL('../server/runtime/sqlite-runtime.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|better-sqlite3|express|fetch\s*\(|https?:|child_process|provider/i);
});
