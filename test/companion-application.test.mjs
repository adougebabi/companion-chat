import assert from 'node:assert/strict';
import test from 'node:test';

import {createCompanionApplication} from '../server/application/companion-application.js';
import {COMPANION_ROUTE_CONTRACT} from '../server/http/route-registry.js';

test('companion application composes route handlers without infrastructure side effects', () => {
    const application = createCompanionApplication({
        routeHandlers: Object.fromEntries(COMPANION_ROUTE_CONTRACT.map(route => [route.handler, () => {}]))
    });
    assert.equal(Object.keys(application.routeHandlers).length, COMPANION_ROUTE_CONTRACT.length);
    assert.equal(application.pendingEventFlow, null);
    assert.equal(application.sceneEventFlow, null);
    assert.equal(application.mediaFlow, null);
    assert.equal(application.capabilityDispatcher, null);
});

test('companion application keeps injected flows, services, and route handlers', () => {
    const flows = {pendingEventFlow: {plan() {}}, sceneEventFlow: {plan() {}}, mediaFlow: {plan() {}}};
    const handlers = {health() {}};
    const application = createCompanionApplication({...flows, routeHandlers: handlers, services: {chat() {}}});
    assert.strictEqual(application.pendingEventFlow, flows.pendingEventFlow);
    assert.strictEqual(application.sceneEventFlow, flows.sceneEventFlow);
    assert.strictEqual(application.mediaFlow, flows.mediaFlow);
    assert.strictEqual(application.routeHandlers, handlers);
});

test('companion application adds an injected chat service to the service map', () => {
    const chatService = {handle() {}};
    const application = createCompanionApplication({chatService, routeHandlers: {health() {}}});
    assert.strictEqual(application.chatService, chatService);
    assert.strictEqual(application.services.chat, chatService);
});

test('companion application exposes the validated chat route for explicit chat ports', () => {
    const calls = [];
    const application = createCompanionApplication({
        chatPorts: {
            contextReader: {read: async () => ({fragments: []})},
            llm: {stream: async () => ({tokens: [], toolCalls: []})},
            capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
            conversation: {listMessages: () => ({items: []})},
            commit: async () => calls.push('commit'),
            sendSse: (_sink, event) => calls.push(event),
            end: sink => { sink.ended = true; }
        }
    });

    assert.equal(typeof application.chatService.handle, 'function');
    assert.strictEqual(application.chatRoute, application.routeHandlers.chat);

    const sink = {
        events: [],
        writableEnded: false,
        on() { return this; },
        removeListener() { return this; }
    };
    const request = {body: {personaId: 'persona_test', text: 'hello'}};
    return application.chatRoute(request, sink).then(() => {
        assert.deepEqual(calls, [
            {type: 'error', error: 'step failed for flow chat-turn step llm-stream: 模型未返回可见回复'}
        ]);
        assert.equal(sink.ended, true);
    });
});
