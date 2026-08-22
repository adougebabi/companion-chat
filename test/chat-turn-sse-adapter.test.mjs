import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {createChatTurnSseAdapter} from '../server/http/chat-turn-sse-adapter.js';

function responseSink() {
    const listeners = new Map();
    return {
        writableEnded: false,
        events: [],
        ended: 0,
        on(event, listener) {
            listeners.set(event, listener);
            return this;
        },
        removeListener(event, listener) {
            if (listeners.get(event) === listener) listeners.delete(event);
            return this;
        },
        close() {
            listeners.get('close')?.();
        },
        abort() {
            listeners.get('aborted')?.();
        }
    };
}

function adapterFor(flow, overrides = {}) {
    return createChatTurnSseAdapter({
        chatTurnFlow: flow,
        sendSse: (sink, event) => sink.events.push(event),
        end: sink => { sink.ended += 1; sink.writableEnded = true; },
        ...overrides
    });
}

test('emits token presentation in order and sends one done with an authoritative alias', async () => {
    const sink = responseSink();
    const message = {id: 'message_1', role: 'assistant', text: 'Ready.', attachments: [], jobs: []};
    const flow = {
        async run() {
            return {
                presentation: [
                    {type: 'token', token: 'One'},
                    {type: 'capability-result', result: {secret: 'internal'}},
                    {type: 'token', token: ' two'}
                ],
                messages: [message],
                message: {id: 'stale'},
                facts: [{type: 'internal-fact'}],
                effects: [{effectId: 'internal-effect'}],
                pendingEvent: {id: 'pending_1'}
            };
        }
    };

    await adapterFor(flow)({context: {requestId: 'request_1'}, command: {personaId: 'persona_1'}}, sink);

    assert.deepEqual(sink.events, [
        {type: 'token', token: 'One'},
        {type: 'token', token: 'two'},
        {type: 'done', messages: [message], message, learned: [], jobs: [], pendingEvent: {id: 'pending_1'}}
    ]);
    assert.equal(sink.events.filter(event => event.type === 'done').length, 1);
    assert.equal(sink.ended, 1);
});

test('forwards live token callbacks before the flow resolves', async () => {
    const sink = responseSink();
    let flowObservedToken = false;
    const adapter = adapterFor({
        async run({command}) {
            await command.onToken('live');
            flowObservedToken = sink.events[0]?.type === 'token' && sink.events[0]?.token === 'live';
            return {messages: [{id: 'message_live', role: 'assistant', text: 'done'}]};
        }
    });

    await adapter({context: {}, command: {}}, sink);

    assert.equal(flowObservedToken, true);
    assert.deepEqual(sink.events, [
        {type: 'token', token: 'live'},
        {type: 'done', messages: [{id: 'message_live', role: 'assistant', text: 'done'}], message: {id: 'message_live', role: 'assistant', text: 'done'}, learned: [], jobs: []}
    ]);
});

test('maps a flow failure to one bounded error event before ending', async () => {
    const sink = responseSink();
    const adapter = adapterFor({
        async run() {
            throw new Error('provider detail should be mapped ' + 'x'.repeat(700));
        }
    }, {errorMapper: () => ({error: 'safe failure'})});

    await adapter({context: {}, command: {}}, sink);

    assert.deepEqual(sink.events, [{type: 'error', error: 'safe failure'}]);
    assert.equal(sink.ended, 1);
});

test('bounds an error mapper result and never emits a second terminal error', async () => {
    const sink = responseSink();
    const adapter = adapterFor({
        async run() {
            throw new Error('failed');
        }
    }, {errorMapper: () => ' mapped ' + 'x'.repeat(700)});

    await adapter({context: {}, command: {}}, sink);

    assert.equal(sink.events.length, 1);
    assert.equal(sink.events[0].type, 'error');
    assert.equal(sink.events[0].error.length, 480);
    assert.match(sink.events[0].error, /\.\.\.$/);
    assert.equal(sink.ended, 1);
});

test('does not write after a client close while the flow is settling', async () => {
    const sink = responseSink();
    let release;
    const flow = {
        run() {
            return new Promise(resolve => {
                release = resolve;
            });
        }
    };
    const running = adapterFor(flow)({context: {}, command: {}}, sink);
    sink.close();
    release({presentation: [{type: 'token', token: 'late'}], messages: [{id: 'late'}]});

    await running;
    assert.deepEqual(sink.events, []);
    assert.equal(sink.ended, 1);
});

test('does not treat a normal request close as an SSE client disconnect', async () => {
    const sink = responseSink();
    const request = new EventEmitter();
    let receivedSignal;
    const adapter = adapterFor({
        async run({command}) {
            receivedSignal = command.signal;
            return {messages: [{id: 'message_request_close', role: 'assistant', text: 'done'}]};
        }
    });

    const running = adapter({context: {request}, command: {}, request}, sink);
    request.emit('close');
    await running;

    assert.equal(receivedSignal.aborted, false);
    assert.equal(sink.events.at(-1).type, 'done');
    assert.equal(sink.ended, 1);
});

test('keeps a slow provider connection alive with SSE comments', async () => {
    const sink = responseSink();
    const writes = [];
    sink.write = value => { writes.push(String(value)); return true; };
    const adapter = adapterFor({
        async run() {
            await new Promise(resolve => setTimeout(resolve, 25));
            return {messages: [{id: 'message_heartbeat', role: 'assistant', text: 'done'}]};
        }
    }, {heartbeatIntervalMs: 5});

    await adapter({context: {}, command: {}}, sink);

    assert.ok(writes.some(value => value === ': keep-alive\n\n'));
    assert.equal(sink.ended, 1);
});

test('forwards an abort signal to the flow and stops terminal presentation after abort', async () => {
    const sink = responseSink();
    const controller = new AbortController();
    let receivedSignal;
    const flow = {
        async run({command}) {
            receivedSignal = command.signal;
            controller.abort();
            return {presentation: [{type: 'token', token: 'late'}], messages: [{id: 'late'}]};
        }
    };

    await adapterFor(flow)({context: {}, command: {signal: controller.signal}}, sink);

    assert.equal(receivedSignal.aborted, true);
    assert.deepEqual(sink.events, []);
    assert.equal(sink.ended, 1);
});

test('transport adapter has no infrastructure or legacy entrypoint imports', async () => {
    const source = await readFile(new URL('../server/http/chat-turn-sse-adapter.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /better-sqlite3|server\.js|fetch\s*\(|child_process|comfy|mtplx/i);
});
