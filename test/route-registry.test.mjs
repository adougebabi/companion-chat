import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    COMPANION_ROUTE_CONTRACT,
    registerCompanionRoutes
} from '../server/http/route-registry.js';

function fakeApp() {
    const calls = [];
    const app = {calls};
    for (const method of ['get', 'post', 'put', 'patch', 'delete']) {
        app[method] = (path, handler) => {
            calls.push({method: method.toUpperCase(), path, handler});
            return app;
        };
    }
    return app;
}

function allHandlers() {
    return Object.fromEntries(COMPANION_ROUTE_CONTRACT.map(route => [route.handler, () => {}]));
}

test('route contract covers the companion API with one injected handler per path', () => {
    const app = fakeApp();
    const wrapped = [];
    const result = registerCompanionRoutes({
        app,
        handlers: allHandlers(),
        debugInspectorEnabled: true,
        wrapRoute(handler, options) {
            wrapped.push({handler, options});
            return handler;
        },
        sendError() {}
    });

    assert.deepEqual(
        app.calls.map(({method, path}) => ({method, path})),
        COMPANION_ROUTE_CONTRACT.map(({method, path}) => ({method, path}))
    );
    assert.equal(result.registered.length, COMPANION_ROUTE_CONTRACT.length);
    assert.deepEqual(result.skipped, []);
    assert.equal(wrapped.length, COMPANION_ROUTE_CONTRACT.length);
    assert.equal(wrapped.find(({options}) => options.sse).options.sse, true);
});

test('debug routes are absent unless the caller passes the resolved boolean flag', () => {
    const app = fakeApp();
    const result = registerCompanionRoutes({
        app,
        handlers: allHandlers(),
        debugInspectorEnabled: false,
        wrapRoute: handler => handler
    });

    const debugRoutes = COMPANION_ROUTE_CONTRACT.filter(route => route.debug);
    assert.equal(result.registered.length, COMPANION_ROUTE_CONTRACT.length - debugRoutes.length);
    assert.deepEqual(
        result.skipped.filter(route => route.reason === 'debug_disabled').map(({id}) => id),
        debugRoutes.map(({id}) => id)
    );
    assert.equal(app.calls.some(call => debugRoutes.some(route => route.path === call.path)), false);

    const stringFlagApp = fakeApp();
    const stringFlagResult = registerCompanionRoutes({
        app: stringFlagApp,
        handlers: allHandlers(),
        debugInspectorEnabled: '1',
        wrapRoute: handler => handler
    });
    assert.equal(stringFlagResult.registered.some(route => route.debug), false);
});

test('missing handlers are reported explicitly or fail configuration in strict mode', () => {
    const app = fakeApp();
    const result = registerCompanionRoutes({
        app,
        handlers: {health: () => {}},
        wrapRoute: handler => handler
    });

    assert.deepEqual(result.registered.map(({id}) => id), ['health']);
    assert.equal(result.skipped.some(route => route.id === 'bootstrap' && route.reason === 'handler_missing'), true);
    assert.throws(
        () => registerCompanionRoutes({
            app: fakeApp(),
            handlers: {health: () => {}},
            wrapRoute: handler => handler,
            missingHandler: 'error'
        }),
        /handler "bootstrap" is required/
    );
});

test('chat preserves the direct request/response SSE handler interface and wrapper', async () => {
    const app = fakeApp();
    const calls = [];
    const req = {body: {personaId: 'persona_test'}};
    const res = {write() {}, end() {}};
    registerCompanionRoutes({
        app,
        handlers: {chat: (request, response) => calls.push({request, response})},
        wrapRoute: handler => async (request, response) => handler(request, response)
    });

    const chatRoute = app.calls.find(call => call.method === 'POST' && call.path === '/api/companion/chat');
    assert.ok(chatRoute);
    await chatRoute.handler(req, res);
    assert.deepEqual(calls, [{request: req, response: res}]);
});

test('route registry has no legacy, database, or provider imports', async () => {
    const source = await readFile(new URL('../server/http/route-registry.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|sqlite|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|child_process|\.prepare\s*\(|\bfetch\s*\(/i);
});
