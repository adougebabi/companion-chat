import assert from 'node:assert/strict';
import {existsSync, mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import createStartupRuntime, {
    STARTUP_DEFAULT_PATHS,
    createCompanionMigrations,
    createRuntimeConfig,
    createServerStartup
} from '../server/runtime/startup.js';

function temporaryDirectory() {
    return mkdtempSync(join(tmpdir(), 'local-ai-companion-startup-'));
}

function withRuntime(options, callback) {
    const dataDir = temporaryDirectory();
    const runtime = createStartupRuntime({Database, dataDir, ...options});
    try {
        return callback(runtime, dataDir);
    } finally {
        runtime.close();
        rmSync(dataDir, {recursive: true, force: true});
    }
}

test('resolves DATA_DIR and DATABASE_PATH with COMPANION_* compatibility', () => {
    const dataDir = temporaryDirectory();
    const companionDataDir = join(dataDir, 'companion-data');
    const databasePath = join(dataDir, 'explicit.sqlite');
    try {
        const runtime = createStartupRuntime({
            Database,
            environment: {
                DATA_DIR: dataDir,
                DATABASE_PATH: databasePath,
                COMPANION_DATA_DIR: companionDataDir,
                COMPANION_DATABASE_PATH: join(dataDir, 'ignored.sqlite')
            },
            migrations: []
        });
        assert.equal(runtime.dataDir, dataDir);
        assert.equal(runtime.databasePath, databasePath);
        assert.equal(existsSync(databasePath), true);
        runtime.close();

        const companionRuntime = createStartupRuntime({
            Database,
            environment: {
                COMPANION_DATA_DIR: companionDataDir,
                COMPANION_DATABASE_PATH: join(companionDataDir, 'companion.sqlite')
            },
            migrations: []
        });
        assert.equal(companionRuntime.dataDir, companionDataDir);
        assert.equal(companionRuntime.databasePath, join(companionDataDir, 'companion.sqlite'));
        companionRuntime.close();
    } finally {
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('default startup applies the complete v1-v19 companion schema', () => {
    withRuntime({
        environment: {DATA_DIR: '/ignored-by-explicit-option'},
        now: () => '2026-01-01T00:00:00.000Z',
        id: prefix => `${prefix}_fixture`
    }, (runtime, dataDir) => {
        assert.equal(runtime.dataDir, dataDir);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 19);
        assert.deepEqual(
            runtime.database.prepare('SELECT version, name FROM companion_schema_migrations ORDER BY version').all(),
            [
                [1, 'initial-companion-domain'],
                [2, 'companion-settings-and-reaction-guard'],
                [3, 'state-source-and-user-reaction-guard'],
                [4, 'adaptive-persona-interviews'],
                [5, 'focus-and-proactive-query-indexes'],
                [6, 'persona-ai-daily-plans'],
                [7, 'persona-life-model-timeline'],
                [8, 'proactive-pending-events'],
                [9, 'persona-contact-groups'],
                [10, 'natural-language-interview-provenance'],
                [11, 'shared-scene-and-image-generation-policy'],
                [12, 'prompt-run-observability'],
                [13, 'prompt-run-response-observability'],
                [14, 'persona-affect-and-drive-state'],
                [15, 'interaction-facts-and-appraisals'],
                [16, 'memory-consolidation-candidate-ledger'],
                [17, 'self-model-claim-ledger'],
                [18, 'agency-intention-ledger'],
                [19, 'persona-initialization-mode']
            ].map(([version, name]) => ({version, name}))
        );
        assert.equal(runtime.database.prepare('PRAGMA table_info(companion_prompt_runs)').all().some(column => column.name === 'response_json'), true);
        assert.equal(runtime.database.prepare('PRAGMA table_info(companion_personas)').all().some(column => column.name === 'group_id'), true);
        assert.equal(runtime.database.prepare('PRAGMA table_info(companion_messages)').all().some(column => column.name === 'proactive_pending_event_id'), true);
        assert.equal(runtime.database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'companion_persona_affect_states'").get().name, 'companion_persona_affect_states');
        assert.equal(runtime.database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'companion_persona_affect_events'").get().name, 'companion_persona_affect_events');
        assert.equal(runtime.database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'companion_interaction_facts'").get().name, 'companion_interaction_facts');
        assert.equal(runtime.database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'companion_appraisals'").get().name, 'companion_appraisals');
        assert.equal(runtime.database.prepare("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'companion_memory_consolidation_candidates'").get().name, 'companion_memory_consolidation_candidates');
        assert.equal(runtime.database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json.includes('http://127.0.0.1:8000/v1'), true);
    });
});

test('startup exposes composition-root database/runtime wiring and idempotent reopen', () => {
    const dataDir = temporaryDirectory();
    const databasePath = join(dataDir, 'nested', 'runtime.sqlite');
    const options = {
        Database,
        dataDir,
        databasePath,
        now: () => '2026-01-01T00:00:00.000Z',
        id: prefix => `${prefix}_fixture`
    };
    try {
        const first = createStartupRuntime(options);
        assert.strictEqual(first.database, first.runtime.database);
        assert.strictEqual(first.database, first.runtimeConfig.database);
        assert.strictEqual(first.runMigrations, first.runtime.runMigrations);
        assert.strictEqual(first.close, first.runtime.close);
        assert.deepEqual(first.databaseConfig.migrationVersions, Array.from({length: 19}, (_, index) => index + 1));
        first.close();

        const second = createStartupRuntime(options);
        assert.equal(second.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 19);
        assert.equal(second.database.prepare('SELECT COUNT(*) AS count FROM companion_groups').get().count, 1);
        second.close();
    } finally {
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('v14 affect migration upgrades a database that already has v1-v13 applied', () => {
    const dataDir = temporaryDirectory();
    const databasePath = join(dataDir, 'upgrade.sqlite');
    const baseOptions = {
        Database,
        dataDir,
        databasePath,
        now: () => '2026-01-01T00:00:00.000Z',
        id: prefix => `${prefix}_fixture`
    };
    try {
        const before = createStartupRuntime({...baseOptions, migrations: createCompanionMigrations({environment: {}, dataDir}).slice(0, 13)});
        assert.equal(before.database.prepare('SELECT MAX(version) AS version FROM companion_schema_migrations').get().version, 13);
        before.close();

        const after = createStartupRuntime(baseOptions);
        assert.equal(after.database.prepare('SELECT MAX(version) AS version FROM companion_schema_migrations').get().version, 19);
        assert.equal(after.database.prepare('SELECT COUNT(*) AS count FROM companion_persona_affect_events').get().count, 0);
        after.close();
    } finally {
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('accepts explicit migrations, clock, and id dependencies without server side effects', () => {
    const order = [];
    const migrations = [
        {
            version: 1,
            name: 'startup-fixture',
            apply(database) {
                order.push('migration');
                database.exec('CREATE TABLE startup_fixture (id TEXT PRIMARY KEY, created_at TEXT NOT NULL)');
                database.prepare('INSERT INTO startup_fixture (id, created_at) VALUES (?, ?)').run('fixture', '2026-01-01T00:00:00.000Z');
            }
        }
    ];
    withRuntime({
        migrations,
        now: () => '2026-01-01T00:00:00.000Z',
        id: prefix => `${prefix}_fixture`
    }, runtime => {
        assert.deepEqual(order, ['migration']);
        assert.deepEqual(runtime.database.prepare('SELECT * FROM startup_fixture').all(), [{id: 'fixture', created_at: '2026-01-01T00:00:00.000Z'}]);
    });

    assert.strictEqual(createRuntimeConfig, createStartupRuntime);
    assert.strictEqual(createServerStartup, createStartupRuntime);
    assert.equal(createCompanionMigrations({environment: {}, dataDir: '/tmp/data'}).length, 19);
    assert.equal(STARTUP_DEFAULT_PATHS.databaseFilename, 'companion.sqlite');

    const source = readFileSync(new URL('../server/runtime/startup.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|listen\s*\(|fetch\s*\(|child_process/i);
});
