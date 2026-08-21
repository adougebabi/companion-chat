import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {createCompanionChatService} from '../server/application/chat-service.js';

function responseSink() {
    const listeners = new Map();
    return {
        events: [],
        ended: 0,
        writableEnded: false,
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
        }
    };
}

function dependencies(overrides = {}) {
    const events = [];
    const service = createCompanionChatService({
        contextReader: {
            async read(input) {
                events.push(['context', input]);
                return {fragments: []};
            }
        },
        llmStreamingPort: {
            async stream(input) {
                events.push(['llm', input]);
                return {tokens: ['ready'], toolCalls: []};
            }
        },
        capabilityDispatcher: {
            async dispatch(input) {
                events.push(['dispatch', input]);
                return {results: [], effects: []};
            }
        },
        conversationRepository: {
            listMessages() {
                events.push(['conversation']);
                return {items: []};
            }
        },
        commitBoundary: async result => {
            events.push(['commit', result]);
        },
        sendSse: (sink, event) => sink.events.push(event),
        end: sink => {
            sink.ended += 1;
            sink.writableEnded = true;
        },
        ...overrides
    });
    return {service, events};
}

test('constructs the chat service without invoking injected ports', () => {
    const calls = [];
    const {service} = dependencies({
        contextReader: {read: () => calls.push('context')},
        llmStreamingPort: {stream: () => calls.push('llm')},
        capabilityDispatcher: {dispatch: () => calls.push('dispatch')},
        conversationRepository: {listMessages: () => calls.push('conversation')},
        commitBoundary: () => calls.push('commit'),
        sendSse: () => calls.push('send'),
        end: () => calls.push('end')
    });

    assert.equal(typeof service.chatTurnFlow.run, 'function');
    assert.equal(typeof service.sseAdapter, 'function');
    assert.equal(typeof service.handle, 'function');
    assert.deepEqual(calls, []);
});

test('handle normalizes request context and preserves the SSE contract', async () => {
    const {service, events} = dependencies();
    const sink = responseSink();
    const request = {on() {}};

    await service.handle({
        personaId: 'persona_test',
        text: 'hello',
        request,
        requestId: 'request_test',
        correlationId: 'correlation_test'
    }, sink);

    const contextInput = events.find(([type]) => type === 'context')[1];
    assert.strictEqual(contextInput.context.request, request);
    assert.strictEqual(contextInput.context.req, request);
    assert.strictEqual(contextInput.context.response, sink);
    assert.strictEqual(contextInput.context.res, sink);
    assert.equal(contextInput.requestId, 'request_test');
    assert.equal(contextInput.correlationId, 'correlation_test');
    assert.deepEqual(sink.events, [
        {type: 'token', token: 'ready'},
        {type: 'done', messages: [], message: null, learned: [], jobs: []}
    ]);
    assert.equal(sink.ended, 1);
    assert.deepEqual(events.map(([type]) => type), ['conversation', 'context', 'llm', 'dispatch', 'commit']);
});

test('handle accepts the existing route invocation envelope without leaking transport fields into the command', async () => {
    const {service, events} = dependencies();
    const sink = responseSink();
    const request = {on() {}};

    await service.handle({
        context: {requestId: 'request_envelope'},
        command: {personaId: 'persona_test', text: 'hello'},
        request,
        req: request,
        response: sink,
        res: sink,
        sink
    }, sink);

    const contextInput = events.find(([type]) => type === 'context')[1];
    const llmInput = events.find(([type]) => type === 'llm')[1];
    assert.equal(contextInput.requestId, 'request_envelope');
    assert.strictEqual(contextInput.context.request, request);
    assert.strictEqual(contextInput.context.response, sink);
    assert.equal(llmInput.command.personaId, 'persona_test');
    assert.equal(llmInput.command.command, undefined);
});

test('handle preserves done.messages as authoritative and message as its compatibility alias', async () => {
    const messages = [
        {id: 'message_1', role: 'assistant', text: 'First.'},
        {id: 'message_2', role: 'assistant', text: 'Second.'}
    ];
    const {service} = dependencies({
        presentationMapper: () => ({type: 'done', messages})
    });
    const sink = responseSink();

    await service.handle({personaId: 'persona_test', text: 'hello'}, sink);

    assert.deepEqual(sink.events.at(-1), {
        type: 'done',
        messages,
        message: messages[0],
        learned: [],
        jobs: []
    });
});

test('handle maps flow failures through the injected SSE error mapper and ends once', async () => {
    let mappedInvocation;
    const {service} = dependencies({
        llmStreamingPort: {
            async stream() {
                throw new Error('provider detail must stay private');
            }
        },
        errorMapper: (_error, invocation) => {
            mappedInvocation = invocation;
            return {error: 'safe chat failure'};
        }
    });
    const sink = responseSink();

    await service.handle({personaId: 'persona_test', text: 'hello'}, sink);

    assert.equal(mappedInvocation.command.personaId, 'persona_test');
    assert.strictEqual(mappedInvocation.context.response, sink);
    assert.deepEqual(sink.events, [{type: 'error', error: 'safe chat failure'}]);
    assert.equal(sink.ended, 1);
});

test('handle forwards abort and disconnect lifecycle to the flow and suppresses late SSE writes', async () => {
    const controller = new AbortController();
    let observedSignal;
    let release;
    let markStarted;
    const started = new Promise(resolve => {
        markStarted = resolve;
    });
    const {service} = dependencies({
        llmStreamingPort: {
            stream(input) {
                observedSignal = input.signal;
                markStarted();
                return new Promise(resolve => {
                    release = resolve;
                });
            }
        }
    });
    const sink = responseSink();
    const running = service.handle({personaId: 'persona_test', text: 'hello', signal: controller.signal}, sink);

    await started;
    sink.close();
    controller.abort();
    release({tokens: ['late'], toolCalls: []});
    await running;

    assert.equal(observedSignal.aborted, true);
    assert.deepEqual(sink.events, []);
    assert.equal(sink.ended, 1);
});

test('commit boundary is invoked by the flow exactly once after successful presentation', async () => {
    const {service, events} = dependencies();
    const sink = responseSink();

    await service.handle({personaId: 'persona_test', text: 'hello'}, sink);

    assert.equal(events.filter(([type]) => type === 'commit').length, 1);
    assert.equal(events.indexOf(events.find(event => event[0] === 'commit')), events.length - 1);
});

test('chat service has no legacy entrypoint, parser, provider, or database dependency', async () => {
    const source = await readFile(new URL('../server/application/chat-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|better-sqlite3|child_process|fetch\s*\(|comfy|mtplx|marker/i);
});
