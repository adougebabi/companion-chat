import assert from 'node:assert/strict';
import test from 'node:test';

import {createHttpApp, sendHttpError, wrapHttpRoute} from '../server/http/app.js';

function fakeExpress() {
    const calls = [];
    const app = {
        calls,
        set(name, value) {
            calls.push(['set', name, value]);
        },
        use(...args) {
            calls.push(['use', ...args]);
        },
        get(...args) {
            calls.push(['get', ...args]);
        },
        post(...args) {
            calls.push(['post', ...args]);
        },
        listen() {
            throw new Error('createHttpApp must not listen');
        }
    };
    const factory = () => app;
    factory.json = options => ({type: 'json', options});
    factory.static = (root, options) => ({type: 'static', root, options});
    return {factory, app};
}

function response() {
    return {
        headers: {},
        statusCode: 200,
        headersSent: false,
        writableEnded: false,
        body: null,
        set(headers) {
            Object.assign(this.headers, headers);
            return this;
        },
        status(statusCode) {
            this.statusCode = statusCode;
            return this;
        },
        json(value) {
            this.body = value;
            this.headersSent = true;
            this.writableEnded = true;
            return this;
        },
        end(value) {
            this.body = value;
            this.headersSent = true;
            this.writableEnded = true;
            return this;
        },
        flushHeaders() {
            this.headersSent = true;
        }
    };
}

function routeCall(app, method, path) {
    const call = app.calls.find(entry => entry[0] === method && entry[1] === path);
    assert.ok(call, `${method.toUpperCase()} ${path} should be registered`);
    return call.at(-1);
}

test('assembles the HTTP seams without opening a listener', async () => {
    const {factory, app} = fakeExpress();
    const registrarCalls = [];
    const adapterCalls = [];
    const httpApp = createHttpApp({
        expressFactory: factory,
        staticRoot: '/tmp/companion-src',
        routeRegistrar(dependencies) {
            registrarCalls.push(dependencies);
            dependencies.app.get('/api/fake', dependencies.wrapRoute((_req, res) => res.json({ok: true})));
        },
        chatSseAdapter: async (invocation, sink) => {
            adapterCalls.push({invocation, sink});
        }
    });

    assert.strictEqual(httpApp, app);
    assert.deepEqual(app.calls.slice(0, 4).map(call => call[0]), ['set', 'use', 'use', 'use']);
    assert.equal(app.calls[0][1], 'etag');
    assert.deepEqual(app.calls[1][1], {type: 'json', options: {limit: '12mb'}});
    assert.equal(app.calls[2][1], '/api');
    assert.equal(app.calls[3][1].type, 'static');
    assert.equal(app.calls[3][1].root, '/tmp/companion-src');
    assert.equal(app.calls[3][1].options.setHeaders instanceof Function, true);
    assert.deepEqual(registrarCalls[0].app, app);
    assert.equal(typeof registrarCalls[0].registerChatRoute, 'function');

    const staticResponse = response();
    app.calls[3][1].options.setHeaders(staticResponse, '/tmp/companion-src/index.html');
    assert.equal(staticResponse.headers['Cache-Control'], 'no-store, max-age=0');
    const assetResponse = response();
    app.calls[3][1].options.setHeaders(assetResponse, '/tmp/companion-src/image.png');
    assert.equal(assetResponse.headers['Cache-Control'], undefined);

    const health = routeCall(app, 'get', '/api/health');
    const healthResponse = response();
    await health({}, healthResponse);
    assert.deepEqual(healthResponse.body, {ok: true, storage: 'companion-v2'});

    const chat = routeCall(app, 'post', '/api/companion/chat');
    const chatResponse = response();
    const req = {body: {personaId: 'persona_1', text: 'hello'}};
    await chat(req, chatResponse);
    assert.equal(chatResponse.statusCode, 200);
    assert.equal(chatResponse.headers['Content-Type'], 'text/event-stream; charset=utf-8');
    assert.equal(chatResponse.headers['Cache-Control'], 'no-cache');
    assert.equal(chatResponse.headers.Connection, 'keep-alive');
    assert.strictEqual(adapterCalls[0].sink, chatResponse);
    assert.deepEqual(adapterCalls[0].invocation.command, req.body);
    assert.strictEqual(adapterCalls[0].invocation.req, req);

    assert.equal(app.calls.some(call => call[0] === 'listen'), false);
});

test('wrapHttpRoute and the final middleware preserve bounded error responses', async () => {
    const syncResponse = response();
    wrapHttpRoute(() => {
        throw Object.assign(new Error('bad input'), {status: 422});
    })({}, syncResponse);
    assert.equal(syncResponse.statusCode, 422);
    assert.deepEqual(syncResponse.body, {error: 'bad input'});

    const asyncResponse = response();
    await wrapHttpRoute(async () => {
        throw new Error('async failure');
    })({}, asyncResponse);
    assert.equal(asyncResponse.statusCode, 400);
    assert.deepEqual(asyncResponse.body, {error: 'async failure'});

    const fallbackResponse = response();
    assert.equal(sendHttpError(fallbackResponse, {statusCode: 503, error: 'unavailable'}), true);
    assert.equal(fallbackResponse.statusCode, 503);
    assert.deepEqual(fallbackResponse.body, {error: 'unavailable'});

    const alreadySent = response();
    alreadySent.headersSent = true;
    assert.equal(sendHttpError(alreadySent, new Error('late')), false);
});

test('route registrar can add routes and explicitly mount a chat route', () => {
    const {factory, app} = fakeExpress();
    let customChat;
    createHttpApp({
        expressFactory: factory,
        routeRegistrar({app: target, registerChatRoute}) {
            customChat = () => {};
            target.get('/api/custom', customChat);
            registerChatRoute();
        },
        chatSseAdapter: () => {}
    });

    assert.strictEqual(routeCall(app, 'get', '/api/custom'), customChat);
    assert.equal(app.calls.filter(call => call[0] === 'post' && call[1] === '/api/companion/chat').length, 1);
});
