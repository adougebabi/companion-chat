import assert from 'node:assert/strict';
import {mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createCompanionRouteHandlers} from '../server/application/companion-route-handlers.js';
import {COMPANION_ROUTE_CONTRACT} from '../server/http/route-registry.js';
import {createRuntime} from '../server/runtime/runtime.js';

function response() {
    return {
        statusCode: 200,
        body: undefined,
        ended: false,
        headers: {},
        status(code) {
            this.statusCode = code;
            return this;
        },
        set(headers) {
            Object.assign(this.headers, headers);
            return this;
        },
        json(value) {
            this.body = value;
            return this;
        },
        end() {
            this.ended = true;
            return this;
        },
        flushHeaders() {
            this.headersSent = true;
        }
    };
}

function routeNames(app) {
    return (app.router?.stack || [])
        .map(layer => layer.route)
        .filter(Boolean)
        .map(route => `${Object.keys(route.methods || {})[0]?.toUpperCase()} ${route.path}`);
}

test('route handler composition returns exactly the public contract handlers', () => {
    const handlers = createCompanionRouteHandlers();
    assert.deepEqual(Object.keys(handlers), COMPANION_ROUTE_CONTRACT.map(route => route.handler));
    for (const route of COMPANION_ROUTE_CONTRACT) assert.equal(typeof handlers[route.handler], 'function', route.handler);

    const missing = response();
    handlers.bootstrap({query: {}}, missing);
    assert.equal(missing.statusCode, 501);
    assert.deepEqual(missing.body, {error: 'Companion route "bootstrap" is not configured'});
});

test('handlers validate at the transport boundary and pass use-case context without persistence logic', async () => {
    const calls = [];
    const handlers = createCompanionRouteHandlers({
        repositories: {persona: {findActive() { return null; }}},
        services: {
            bootstrap(command, context) {
                calls.push({command, context});
                return {settings: {}, personas: [], groups: [], activityUnread: false};
            },
            settings: {update(command) {
                calls.push({settings: command});
                return {saved: true};
            }},
            cancelSchedule() {
                return {cancelled: true};
            },
            restoreFoundationRevision() {
                return {restored: true, version: 2};
            },
            appendConversationMessage() {
                return [{id: 'message_1', role: 'user', text: 'hello'}];
            }
        },
        policies: {imageGenerationPolicies: ['ask']}
    });

    const bootstrapResponse = response();
    await handlers.bootstrap({query: {personaId: 'persona_1'}}, bootstrapResponse);
    assert.deepEqual(bootstrapResponse.body, {settings: {}, personas: [], groups: [], activityUnread: false});
    assert.deepEqual(calls[0].command, {personaId: 'persona_1'});
    assert.equal(calls[0].command.request.query.personaId, 'persona_1');
    assert.equal(calls[0].context.repositories.persona.findActive instanceof Function, true);
    assert.equal(calls[0].context.route, 'bootstrap');

    const settingsResponse = response();
    await handlers.settings({body: {model: 'fixture'}}, settingsResponse);
    assert.equal(settingsResponse.statusCode, 200);
    assert.deepEqual(calls[1].settings, {model: 'fixture'});

    const messageResponse = response();
    await handlers.appendConversationMessage({
        params: {personaId: 'persona_1'},
        body: {role: 'user', text: 'hello'}
    }, messageResponse);
    assert.equal(messageResponse.statusCode, 201);
    assert.deepEqual(messageResponse.body, {
        message: {id: 'message_1', role: 'user', text: 'hello'},
        messages: [{id: 'message_1', role: 'user', text: 'hello'}]
    });

    const cancelResponse = response();
    await handlers.cancelSchedule({params: {personaId: 'persona_1', scheduleId: 'schedule_1'}}, cancelResponse);
    assert.equal(cancelResponse.statusCode, 204);
    assert.equal(cancelResponse.ended, true);

    const restoreResponse = response();
    await handlers.restoreFoundationRevision({params: {personaId: 'persona_1', revisionId: 'revision_1'}}, restoreResponse);
    assert.equal(restoreResponse.statusCode, 201);

    assert.throws(
        () => handlers.createGroup({body: {name: '   '}}, response()),
        /请求字段 name must be a non-empty string/
    );
    assert.throws(
        () => handlers.getPersona({params: {}}, response()),
        /请求参数 personaId must be a non-empty string/
    );
});

test('chat delegates the request envelope and prepares the SSE response', async () => {
    let invocation;
    const handlers = createCompanionRouteHandlers({
        services: {
            chat(value, sink) {
                invocation = {value, sink};
                return Promise.resolve();
            }
        }
    });
    const sink = response();
    await handlers.chat({body: {personaId: 'persona_1', text: 'hello'}}, sink);
    assert.equal(sink.statusCode, 200);
    assert.equal(sink.headers['Content-Type'], 'text/event-stream; charset=utf-8');
    assert.deepEqual(invocation.value.command, {personaId: 'persona_1', text: 'hello'});
    assert.strictEqual(invocation.value.sink, sink);
    assert.strictEqual(invocation.sink, sink);
});

test('route handler module does not import the legacy root or perform infrastructure work', () => {
    const source = readFileSync(new URL('../server/application/companion-route-handlers.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*server\.js/i);
    assert.doesNotMatch(source, /better-sqlite3|child_process|\.prepare\s*\(|\bfetch\s*\(/i);
});

test('createRuntime mounts the composed route surface supplied as routeHandlers', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-route-handlers-'));
    const runtime = createRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        routeHandlers: createCompanionRouteHandlers({
            services: {
                bootstrap: () => ({settings: {}, personas: [], groups: [], activityUnread: false})
            }
        })
    });
    try {
        const routes = routeNames(runtime.app);
        assert.ok(routes.includes('GET /api/companion/bootstrap'));
        assert.ok(routes.includes('POST /api/companion/chat'));
        assert.ok(routes.includes('GET /api/companion/personas/:personaId'));
        assert.ok(routes.includes('GET /api/companion/personas/:personaId/debug-context') === false);
        await runtime.start({listen: false, worker: false});
        assert.equal(runtime.state, 'running');
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});
