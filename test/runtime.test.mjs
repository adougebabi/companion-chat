import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createRuntime} from '../server/runtime/runtime.js';
import {createHttpApp} from '../server/http/app.js';

function temporaryDirectory() {
    return mkdtempSync(join(tmpdir(), 'local-ai-companion-runtime-'));
}

test('runtime composes temporary SQLite, HTTP app, and worker lifecycle', async () => {
    const dataDir = temporaryDirectory();
    const calls = [];
    const worker = {
        async start() {
            calls.push('worker:start');
        },
        async stop() {
            calls.push('worker:stop');
        }
    };
    const app = createHttpApp({staticRoot: dataDir, healthResponse: {ok: true, storage: 'runtime-fixture'}});
    let closed = false;
    const listener = {
        listening: true,
        close(callback) {
            closed = true;
            this.listening = false;
            callback?.();
        },
        once() {
            return this;
        },
        address() {
            return {port: 4178};
        }
    };
    const originalListen = app.listen;
    app.listen = (port, host, callback) => {
        queueMicrotask(() => callback?.());
        return listener;
    };
    const runtime = createRuntime({
        Database,
        dataDir,
        app,
        workerRuntime: worker,
        environment: {PORT: '0', DATA_DIR: dataDir}
    });
    try {
        assert.equal(runtime.state, 'created');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 13);
        await runtime.start();
        assert.equal(runtime.state, 'running');
        assert.equal(runtime.server, listener);
        assert.equal(typeof app, 'function');
        assert.deepEqual(calls, ['worker:start']);
        await runtime.stop();
        assert.equal(runtime.state, 'stopped');
        assert.equal(closed, true);
        assert.deepEqual(calls, ['worker:start', 'worker:stop']);
        assert.equal(await runtime.stop(), false);
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop().catch(() => {});
        app.listen = originalListen;
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('runtime can start without binding HTTP or a worker for composition tests', async () => {
    const dataDir = temporaryDirectory();
    const runtime = createRuntime({Database, dataDir, workerRuntime: false, environment: {DATA_DIR: dataDir}});
    try {
        assert.equal(await runtime.start({listen: false, worker: false}), null);
        assert.equal(runtime.state, 'running');
        assert.equal(await runtime.stop(), true);
        assert.equal(runtime.state, 'stopped');
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});
