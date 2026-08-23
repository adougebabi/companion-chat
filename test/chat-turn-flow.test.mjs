import assert from 'node:assert/strict';
import test from 'node:test';

import {CAPABILITY_NAMES} from '../server/contracts/index.js';
import {createFlowRegistry} from '../server/application/flow-registry.js';
import {createChatTurnFlow, normalizeBoundedChatResult} from '../server/application/chat-turn-flow.js';

function call(overrides = {}) {
    return {
        id: 'call_test',
        index: 0,
        name: 'scene_event',
        argumentsText: '{"operation":"start"}',
        arguments: {operation: 'start'},
        source: 'native',
        personaId: 'persona_test',
        causationUserMessageId: 'message_test',
        idempotencyKey: 'capability_test',
        ...overrides
    };
}

function dependencies({events = [], dispatcher, mapper, repository} = {}) {
    const capability = dispatcher ?? {
        async dispatch(input) {
            events.push(['dispatch', input]);
            return {results: [], effects: []};
        }
    };
    return {
        contextReader: async input => {
            events.push(['context', input]);
            return {fragments: [{kind: 'life', text: 'at home'}]};
        },
        llmStreamingPort: {
            async stream(input) {
                events.push(['llm', input]);
                return {text: 'ready', toolCalls: []};
            }
        },
        capabilityDispatcher: capability,
        conversationRepository: repository ?? {
            listMessages(input) {
                events.push(['conversation', input]);
                return {items: [{id: 'history_1', role: 'user', text: 'hello'}]};
            }
        },
        presentationMapper: mapper ?? (input => {
            events.push(['presentation', input]);
            return {messages: [{id: 'assistant_test', role: 'assistant', text: 'ready', attachments: [], jobs: []}]};
        })
    };
}

test('ChatTurnFlow injects all ports without opening a database or provider', async () => {
    const events = [];
    let commitCount = 0;
    const flow = createChatTurnFlow({
        ...dependencies({events}),
        commitBoundary: async result => {
            events.push(['commit', result]);
            commitCount += 1;
        }
    });

    assert.deepEqual(flow.registry.list()[0], {
        id: 'chat-turn',
        version: 1,
        layer: 'application',
        stepIds: ['conversation-context', 'context-loader', 'llm-stream', 'capability-handoff', 'conversation-result', 'presentation-mapper']
    });
    assert.equal(commitCount, 0);

    const result = await flow.run({
        context: {requestId: 'request_test', correlationId: 'correlation_test'},
        command: {personaId: 'persona_test', text: 'hello'}
    });

    assert.equal(commitCount, 1);
    assert.deepEqual(result.messages, [{id: 'assistant_test', role: 'assistant', text: 'ready', attachments: [], jobs: []}]);
    assert.deepEqual(result.message, result.messages[0]);
    assert.equal(result.type, 'done');
    assert.deepEqual(result.facts, []);
    assert.deepEqual(result.projections, []);
    assert.deepEqual(result.effects, []);
    assert.deepEqual(events.map(([type]) => type), ['conversation', 'context', 'llm', 'dispatch', 'presentation', 'commit']);

    const singleArgumentResult = await flow.run({personaId: 'persona_test', text: 'single argument'});
    assert.equal(singleArgumentResult.type, 'done');
});

test('ChatTurnFlow preserves the ordered context, stream, native handoff, and presentation steps', async () => {
    const events = [];
    const nativeCall = call();
    const flow = createChatTurnFlow({
        ...dependencies({events, dispatcher: {
            async dispatch(input) {
                events.push(['dispatch', input]);
                return {
                    results: [{
                        name: nativeCall.name,
                        ok: true,
                        callId: nativeCall.id,
                        idempotencyKey: nativeCall.idempotencyKey,
                        result: {eventId: 'event_test'},
                        error: null
                    }],
                    effects: [{
                        effectId: 'effect_test',
                        kind: 'scene-event',
                        capability: nativeCall.name,
                        idempotencyKey: nativeCall.idempotencyKey,
                        causationId: nativeCall.causationUserMessageId,
                        payload: {operation: 'start'}
                    }]
                };
            }
        }}),
        llmStreamingPort: {
            async stream(input) {
                events.push(['llm', input]);
                return {tokens: ['tool response'], toolCalls: [nativeCall]};
            }
        },
        commitBoundary: async () => events.push(['commit'])
    });

    const result = await flow.run({personaId: 'persona_test', causationId: 'message_fallback'}, {personaId: 'persona_test', text: 'start'});

    assert.deepEqual(events.map(([type]) => type), ['conversation', 'context', 'llm', 'dispatch', 'presentation', 'commit']);
    const dispatchInput = events.find(([type]) => type === 'dispatch')[1];
    assert.deepEqual(dispatchInput.calls, [nativeCall]);
    assert.deepEqual(dispatchInput.context, {
        personaId: 'persona_test',
        causationId: 'message_test',
        correlationId: null
    });
    assert.equal(result.effects[0].causationId, 'message_test');
    assert.deepEqual(result.presentation[0], {type: 'token', token: 'tool response'});
    assert.deepEqual(result.presentation[1].result, {name: 'scene_event', ok: true, callId: 'call_test', idempotencyKey: 'capability_test', result: {eventId: 'event_test'}, error: null});
});

test('ChatTurnFlow presents raw repository history oldest-first to the model', async () => {
    let modelHistory;
    const flow = createChatTurnFlow({
        ...dependencies({repository: {
            listMessages() {
                return {items: [
                    {id: 'newest', role: 'user', text: 'newest'},
                    {id: 'older', role: 'assistant', text: 'older'}
                ]};
            }
        }}),
        llmStreamingPort: {
            async stream(input) {
                modelHistory = input.messages.map(message => message.text);
                return {text: 'ready', toolCalls: []};
            }
        }
    });

    await flow.run({context: {}, command: {personaId: 'persona_test', text: 'hello'}});

    assert.deepEqual(modelHistory, ['older', 'newest']);
});

test('ChatTurnFlow does not commit or persist a partial result after a provider failure', async () => {
    const events = [];
    let commits = 0;
    const flow = createChatTurnFlow({
        ...dependencies({events}),
        llmStreamingPort: {async stream() {
            events.push(['llm']);
            throw new Error('provider credentials must stay bounded');
        }},
        commitBoundary: async () => { commits += 1; }
    });

    await assert.rejects(
        flow.run({personaId: 'persona_test'}, {personaId: 'persona_test', text: 'hello'}),
        error => error.code === 'FLOW_EXECUTION_FAILED' && error.stepId === 'llm-stream' && error.message.length <= 240
    );
    assert.equal(commits, 0);
    assert.deepEqual(events.map(([type]) => type), ['conversation', 'context', 'llm']);
});

test('ChatTurnFlow rejects an empty model completion instead of fabricating an assistant reply', async () => {
    let commits = 0;
    const flow = createChatTurnFlow({
        ...dependencies(),
        llmStreamingPort: {async stream() { return {text: '', tokens: [], toolCalls: [], doneSeen: true}; }},
        commitBoundary: async () => { commits += 1; }
    });

    await assert.rejects(
        flow.run({personaId: 'persona_test'}, {personaId: 'persona_test', text: 'hello'}),
        error => error.code === 'FLOW_EXECUTION_FAILED' && error.stepId === 'llm-stream' && /模型未返回可见回复/.test(error.message)
    );
    assert.equal(commits, 0);
});

test('ChatTurnFlow propagates commit failure after successful steps without rerunning provider or dispatcher', async () => {
    const events = [];
    const flow = createChatTurnFlow({
        ...dependencies({events}),
        commitBoundary: async () => {
            events.push(['commit']);
            throw new Error('commit failure');
        }
    });

    await assert.rejects(
        flow.run({personaId: 'persona_test'}, {personaId: 'persona_test', text: 'hello'}),
        error => error.code === 'FLOW_COMMIT_FAILED' && error.message.length <= 240
    );
    assert.deepEqual(events.map(([type]) => type), ['conversation', 'context', 'llm', 'dispatch', 'presentation', 'commit']);
    assert.equal(events.filter(([type]) => type === 'llm').length, 1);
    assert.equal(events.filter(([type]) => type === 'dispatch').length, 1);
});

test('bounded ChatResult keeps done.messages authoritative and derives the compatibility alias', () => {
    const result = normalizeBoundedChatResult({
        messages: Array.from({length: 40}, (_, index) => ({id: `message_${index}`, role: 'assistant', text: 'x'.repeat(13_000)})),
        message: {id: 'stale'},
        learned: Array.from({length: 70}, () => ({value: 'learned'})),
        jobs: Array.from({length: 70}, () => ({id: 'job'}))
    });

    assert.equal(result.type, 'done');
    assert.equal(result.messages.length, 20);
    assert.equal(result.messages[0].text.length, 12_000);
    assert.equal(result.learned.length, 50);
    assert.equal(result.jobs.length, 50);
    assert.strictEqual(result.message, result.messages[0]);
});

test('ChatTurnFlow rejects missing ports before any side effect can occur', () => {
    assert.throws(() => createChatTurnFlow(), /ChatContextReader/);
    assert.throws(() => createChatTurnFlow({
        contextReader: async () => ({}),
        llmStreamingPort: {stream() {}},
        conversationRepository: {},
        presentationMapper: () => ({messages: []})
    }), /CapabilityDispatcherPort/);
    assert.deepEqual(CAPABILITY_NAMES, ['scene_event', 'media_event', 'pending_event']);
});
