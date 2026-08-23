import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createCompanionRuntime, createRuntime} from '../server/runtime/runtime.js';
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
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_schema_migrations').get().count, 19);
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

test('runtime wires route handlers, provider registry, and registered job dispatcher', async () => {
    const dataDir = temporaryDirectory();
    const calls = [];
    const runtime = createRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        routeHandlers: {
            bootstrap(_req, res) {
                res.json({ok: true, source: 'fixture'});
            }
        },
        missingHandler: 'skip',
        providerAdapters: {
            fixture: {id: 'fixture', portType: 'llm-streaming', stream() { calls.push('provider'); }}
        },
        jobHandlers: {
            fixture_job: {run() { calls.push('job'); }}
        },
        jobRepository: {
            claim() {
                return {id: 'job_1', job_type: 'fixture_job', status: 'leased', lease_owner: 'fixture', lease_expires_at: '2999-01-01T00:00:00.000Z', attempt_count: 1, max_attempts: 1};
            },
            findLeased() {
                return {id: 'job_1', status: 'leased', lease_owner: 'fixture', lease_expires_at: '2999-01-01T00:00:00.000Z'};
            },
            settle() {
                calls.push('settle');
                return {changed: true};
            }
        }
    });
    try {
        assert.ok(runtime.providers.get('fixture'));
        assert.deepEqual(runtime.jobDispatcher.list(), ['fixture_job']);
        assert.equal(runtime.app, runtime.app);
        await runtime.start({listen: false, worker: false});
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('companion runtime registers every media handler, dispatches through the worker, and settles once', async () => {
    const dataDir = temporaryDirectory();
    const calls = [];
    const job = {
        id: 'media_runtime_job',
        job_type: 'chat_image',
        status: 'queued',
        lease_owner: null,
        lease_expires_at: null,
        attempt_count: 0,
        max_attempts: 2
    };
    const jobRepository = {
        claim(input) {
            calls.push(['claim', input.leaseOwner]);
            if (job.status !== 'queued') return null;
            job.status = 'leased';
            job.lease_owner = input.leaseOwner;
            job.lease_expires_at = '2999-01-01T00:00:00.000Z';
            job.attempt_count += 1;
            return job;
        },
        findLeased(input) {
            return job.status === 'leased' && job.lease_owner === input.leaseOwner ? job : null;
        },
        settle(input) {
            calls.push(['settle', input.status]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            job.status = input.status;
            job.lease_owner = null;
            job.lease_expires_at = null;
            return {changed: true, status: input.status, job};
        },
        retry(input) {
            calls.push(['retry', input.runAfter]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            job.status = 'queued';
            job.lease_owner = null;
            job.lease_expires_at = null;
            return {changed: true, status: 'queued', job};
        }
    };
    const mediaTypes = ['activity_image', 'activity_video', 'chat_image', 'chat_video', 'activity_media_poll', 'chat_media_poll'];
    const mediaJobService = {
        handlers: Object.fromEntries(mediaTypes.map(type => [type, async (receivedJob, context) => {
            calls.push(['media', type, context.deferSettlement]);
            if (type !== 'chat_image') return {status: 'complete', result: {type}};
            // This mirrors media-job-service: direct callers settle, but the
            // runtime adapter defers to the generic dispatcher.
            if (!context.deferSettlement) jobRepository.settle({id: receivedJob.id, status: 'complete', leaseOwner: context.leaseOwner});
            return {status: 'complete', result: {type}};
        }]) )
    };
    const timers = {
        setInterval() { return 1; },
        clearInterval() {},
        setTimeout() { return 1; },
        clearTimeout() {}
    };
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        repositories: {job: jobRepository},
        mediaJobService,
        handlers: {
            generic_job() { calls.push(['generic']); }
        },
        application: {routeHandlers: {}},
        missingHandler: 'skip',
        workerOptions: {timers, leaseOwner: 'runtime_worker'},
        environment: {DATA_DIR: dataDir}
    });
    try {
        assert.strictEqual(runtime.jobRepository, jobRepository);
        assert.strictEqual(runtime.repositories.jobRepository, jobRepository);
        assert.deepEqual(runtime.jobDispatcher.list(), [...mediaTypes, 'generic_job']);
        await runtime.start({listen: false});
        const result = await runtime.worker.tick();
        assert.equal(result.status, 'complete');
        assert.deepEqual(calls.filter(([kind]) => kind === 'media'), [['media', 'chat_image', true]]);
        assert.deepEqual(calls.filter(([kind]) => kind === 'settle'), [['settle', 'complete']]);
        assert.equal(job.status, 'complete');
    } finally {
        await runtime.stop();
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('companion runtime keeps stale media leases fail-closed and terminal/retry settlement single-owner', async () => {
    async function run(mode) {
        const calls = [];
        const job = {
            id: `media_${mode}`,
            job_type: 'chat_image',
            status: 'queued',
            lease_owner: null,
            lease_expires_at: null,
            attempt_count: 0,
            max_attempts: mode === 'terminal' ? 1 : 2
        };
        const repository = {
            claim(input) {
                job.status = 'leased';
                job.lease_owner = input.leaseOwner;
                job.lease_expires_at = '2999-01-01T00:00:00.000Z';
                job.attempt_count += 1;
                return job;
            },
            findLeased(input) {
                if (mode === 'stale') return null;
                return job.status === 'leased' && job.lease_owner === input.leaseOwner ? job : null;
            },
            settle(input) {
                calls.push(['settle', input.status]);
                job.status = input.status;
                job.lease_owner = null;
                job.lease_expires_at = null;
                return {changed: true, status: input.status, job};
            },
            retry(input) {
                calls.push(['retry', input.runAfter]);
                job.status = 'queued';
                job.lease_owner = null;
                job.lease_expires_at = null;
                return {changed: true, status: 'queued', job};
            }
        };
        const runtime = createCompanionRuntime({
            startupRuntime: {database: {}, close() {}},
            app: {},
            application: {},
            repositories: {job: repository},
            mediaJobService: {
                handlers: {
                    chat_image(_receivedJob, context) {
                        calls.push(['handler', context.deferSettlement]);
                        return mode === 'terminal'
                            ? {status: 'failed', terminal: true, error: 'terminal media failure'}
                            : {status: 'retry', error: 'temporary media failure'};
                    }
                }
            },
            workerOptions: {
                leaseOwner: 'media_runtime_worker',
                timers: {setInterval() { return 1; }, clearInterval() {}, setTimeout() { return 1; }, clearTimeout() {}}
            }
        });
        try {
            await runtime.start({listen: false});
            const result = await runtime.worker.tick();
            return {result, calls, job};
        } finally {
            await runtime.stop();
        }
    }

    const stale = await run('stale');
    assert.equal(stale.result.status, 'stale');
    assert.deepEqual(stale.calls, []);
    assert.equal(stale.job.status, 'leased');

    const retried = await run('retry');
    assert.equal(retried.result.status, 'retry');
    assert.deepEqual(retried.calls.map(([kind]) => kind), ['handler', 'retry']);
    assert.equal(retried.job.status, 'queued');

    const terminal = await run('terminal');
    assert.equal(terminal.result.status, 'failed');
    assert.deepEqual(terminal.calls, [['handler', true], ['settle', 'failed']]);
    assert.equal(terminal.job.status, 'failed');
});

test('companion runtime composes the activity application service for feed routes', async () => {
    const dataDir = temporaryDirectory();
    const runtime = createCompanionRuntime({Database, dataDir, workerRuntime: false, environment: {DATA_DIR: dataDir}});
    try {
        assert.equal(typeof runtime.application.services.activities.list, 'function');
        assert.equal(typeof runtime.application.services.activities.comment, 'function');
        const layer = runtime.app.router.stack.find(item => item.route?.path === '/api/companion/activities');
        assert.ok(layer);
        const response = {
            statusCode: 200,
            body: undefined,
            headersSent: false,
            status(code) { this.statusCode = code; return this; },
            json(value) { this.body = value; this.headersSent = true; return this; }
        };
        const result = layer.route.stack[0].handle({query: {}}, response);
        if (result?.then) await result;
        assert.equal(response.statusCode, 200);
        assert.deepEqual(response.body, {items: [], nextCursor: null});
    } finally {
        await runtime.stop();
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('runtime owns auxiliary task lifecycles in start/stop order', async () => {
    const dataDir = temporaryDirectory();
    const events = [];
    const first = {start: async () => events.push('first:start'), stop: async () => events.push('first:stop')};
    const second = {start: async () => events.push('second:start'), stop: async () => events.push('second:stop')};
    const runtime = createRuntime({Database, dataDir, workerRuntime: false, auxiliaryRuntimes: [first, second], environment: {DATA_DIR: dataDir}});
    try {
        await runtime.start({listen: false, worker: false});
        await runtime.stop();
        assert.deepEqual(events, ['first:start', 'second:start', 'second:stop', 'first:stop']);
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('companion runtime mounts the complete contract with bounded unconfigured handlers', async () => {
    const dataDir = temporaryDirectory();
    const runtime = createCompanionRuntime({Database, dataDir, workerRuntime: false, environment: {DATA_DIR: dataDir}});
    try {
        await runtime.start({listen: false, worker: false});
        const bootstrap = runtime.app.router.stack.find(layer => layer.route?.path === '/api/companion/bootstrap');
        assert.ok(bootstrap);
        assert.ok(runtime.application);
        assert.equal(typeof runtime.application.routeHandlers.bootstrap, 'function');
        assert.equal(typeof runtime.app, 'function');
    } finally {
        await runtime.stop();
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('persisted debug setting restores debug routes and bootstrap visibility after runtime recreation', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-debug-setting-'));
    const first = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        debugInspectorEnabled: false
    });
    try {
        first.repositories.settings.write({debugInspector: true});
    } finally {
        await first.stop();
    }

    const second = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        debugInspectorEnabled: false
    });
    try {
        assert.equal(second.application.services.bootstrap.read().debugInspector, true);
        const paths = second.app.router.stack.map(layer => layer.route?.path).filter(Boolean);
        assert.equal(paths.includes('/api/companion/personas/:personaId/debug-context'), true);
    } finally {
        await second.stop();
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('companion runtime provides a real repository-backed bootstrap slice', async () => {
    const dataDir = temporaryDirectory();
    const runtime = createCompanionRuntime({Database, dataDir, workerRuntime: false, environment: {DATA_DIR: dataDir}});
    try {
        await runtime.start({listen: false, worker: false});
        const layer = runtime.app.router.stack.find(item => item.route?.path === '/api/companion/bootstrap');
        assert.ok(layer);
        let payload;
        const response = {
            headersSent: false,
            status() { return this; },
            json(value) { payload = value; this.headersSent = true; return this; }
        };
        const result = layer.route.stack[0].handle({query: {}}, response);
        if (result?.then) await result;
        assert.equal(response.headersSent, true);
        assert.equal(Array.isArray(payload.groups), true);
        assert.equal(payload.storage, undefined);
    } finally {
        await runtime.stop();
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('companion runtime mounts one validated chat route from explicit chat ports', async () => {
    const dataDir = temporaryDirectory();
    const sent = [];
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        contextReader: {read: async () => ({fragments: []})},
        llmStreamingPort: {stream: async () => ({tokens: ['ready'], toolCalls: []})},
        capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
        conversationRepository: {listMessages: () => ({items: []})},
        commitBoundary: async () => {},
        sendSse: (_sink, event) => sent.push(event),
        end: sink => { sink.writableEnded = true; }
    });
    try {
        const chatRoutes = runtime.app.router.stack.filter(item => item.route?.path === '/api/companion/chat');
        assert.equal(chatRoutes.length, 1);
        assert.strictEqual(runtime.application.chatRoute, runtime.application.routeHandlers.chat);

        await runtime.start({listen: false, worker: false});
        const response = {
            writableEnded: false,
            status() { return this; },
            set() { return this; },
            flushHeaders() {},
            on() { return this; },
            removeListener() { return this; },
            end() { this.writableEnded = true; }
        };
        const result = chatRoutes[0].route.stack[0].handle({body: {personaId: 'persona_test', text: 'hello'}}, response);
        if (result?.then) await result;
        assert.deepEqual(sent, [
            {type: 'token', token: 'ready'},
            {type: 'done', messages: [], message: null, learned: [], jobs: []}
        ]);
        assert.equal(response.writableEnded, true);
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});
