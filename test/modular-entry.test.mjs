import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {existsSync, mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {runModularCli, startModularRuntime} from '../server/index.js';

test('importing server/index.js does not create storage or install signal handlers', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-import-'));
    const signalCounts = {
        sigint: process.listenerCount('SIGINT'),
        sigterm: process.listenerCount('SIGTERM')
    };
    try {
        await import('../server/index.js?side-effect-free');
        assert.equal(process.listenerCount('SIGINT'), signalCounts.sigint);
        assert.equal(process.listenerCount('SIGTERM'), signalCounts.sigterm);
        assert.equal(existsSync(join(dataDir, 'companion.sqlite')), false);
    } finally {
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('server/index.js exposes a side-effect-free modular start factory', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-entry-'));
    const runtime = await startModularRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        startOptions: {listen: false, worker: false}
    });
    try {
        assert.equal(runtime.state, 'running');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 14);
        await runtime.stop();
        assert.equal(runtime.state, 'stopped');
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('CLI wiring forwards environment paths and cleans up on repeated signals', async () => {
    const signalSource = new EventEmitter();
    const calls = [];
    let runtimeOptions;
    const runtime = {
        async start(options) {
            calls.push(['start', options]);
        },
        async stop() {
            calls.push(['stop']);
        }
    };
    const result = await runModularCli({
        environment: {
            PORT: '0',
            HOST: '127.0.0.1',
            DATA_DIR: '/tmp/companion-cli-data',
            DATABASE_PATH: '/tmp/companion-cli.sqlite'
        },
        runtimeFactory(options) {
            runtimeOptions = options;
            return runtime;
        },
        startOptions: {listen: false, worker: false},
        signalSource
    });

    assert.strictEqual(result.runtime, runtime);
    assert.deepEqual(calls, [['start', {listen: false, worker: false}]]);
    assert.equal(runtimeOptions.port, '0');
    assert.equal(runtimeOptions.host, '127.0.0.1');
    assert.equal(runtimeOptions.dataDir, '/tmp/companion-cli-data');
    assert.equal(runtimeOptions.databasePath, '/tmp/companion-cli.sqlite');
    assert.strictEqual(runtimeOptions.environment.PORT, '0');

    signalSource.emit('SIGTERM');
    signalSource.emit('SIGINT');
    await result.cleanup();
    assert.deepEqual(calls, [
        ['start', {listen: false, worker: false}],
        ['stop']
    ]);
    assert.equal(signalSource.listenerCount('SIGINT'), 0);
    assert.equal(signalSource.listenerCount('SIGTERM'), 0);
    const cleanupPromise = result.cleanup();
    assert.strictEqual(result.cleanup(), cleanupPromise);
    await cleanupPromise;
});
