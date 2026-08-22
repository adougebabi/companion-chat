import assert from 'node:assert/strict';
import test from 'node:test';

import {createMtplxCompletionPort, createMtplxProvider, consumeMtplxStream} from '../server/infrastructure/llm-provider.js';

function jsonResponse(body) {
    return {
        ok: true,
        status: 200,
        headers: {get: () => 'application/json'},
        json: async () => body
    };
}

test('MTPLX provider is inert until stream/models are invoked', async () => {
    let calls = 0;
    const provider = createMtplxProvider({
        settings: () => ({lmStudioUrl: 'http://127.0.0.1:8000/v1/', lmStudioApiKey: 'secret'}),
        fetchImpl: async (url, options) => {
            calls += 1;
            assert.equal(url, 'http://127.0.0.1:8000/v1/chat/completions');
            assert.equal(options.headers.Authorization, 'Bearer secret');
            return {ok: true, json: async () => ({ok: true})};
        }
    });
    assert.equal(calls, 0);
    const result = await provider.stream({model: 'fixture', messages: []});
    assert.deepEqual(await result.json(), {ok: true});
    assert.equal(calls, 1);
});

test('MTPLX provider bounds upstream errors', async () => {
    const provider = createMtplxProvider({
        settings: () => ({lmStudioUrl: 'http://fixture/v1', lmStudioApiKey: ''}),
        fetchImpl: async () => ({ok: false, status: 502, json: async () => ({error: {message: `${'x'.repeat(600)} token=secret`}})})
    });
    await assert.rejects(() => provider.models(), error => {
        assert.ok(error.message.length <= 240);
        assert.doesNotMatch(error.message, /secret/);
        return true;
    });
});

test('completion port exposes one normalized turn for a parsed structured sidecar', async () => {
    const provider = {
        async stream() {
            return jsonResponse({choices: [{message: {
                content: '已记住。',
                structuredTurn: {
                    schemaVersion: 'companion.turn.v1',
                    control: {
                        affectEvents: [{type: 'social_connection', confidence: 0.8, idempotencyKey: 'affect_1'}],
                        driveSignals: [{drive: 'social', direction: 'decrease_pressure', confidence: 0.7, idempotencyKey: 'drive_1'}],
                        memoryWrites: [{operation: 'upsert', key: 'favorite.drink', value: 'tea', confidence: 0.9, idempotencyKey: 'memory_1'}]
                    }
                }
            }}]});
        }
    };
    const port = createMtplxCompletionPort({provider, settings: () => ({model: 'fixture'})});
    const result = await port.stream({personaId: 'persona_1', causationId: 'message_1', messages: []});

    assert.equal(result.text, '已记住。');
    assert.equal(result.sourceMode, 'structured_sidecar');
    assert.equal(result.structuredTurn.control.affectEvents[0].personaId, 'persona_1');
    assert.equal(result.control.driveSignals[0].drive, 'social');
    assert.equal(result.structuredTurn.control.memoryWrites[0].key, 'favorite.drink');
});

test('stream provider keeps visible text and native tool compatibility while rejecting malformed sidecar control', async () => {
    const response = new Response([
        'data: {"choices":[{"delta":{"content":"visible","structuredTurn":{"schemaVersion":"companion.turn.v999","control":{"memoryWrites":[]}}}}]}\n',
        'data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"scene_event","arguments":"{\\"operation\\":\\"start\\"}"}}]}}]}\n',
        'data: [DONE]\n\n'
    ].join(''), {headers: {'content-type': 'text/event-stream'}});
    const completion = await consumeMtplxStream(response, {personaId: 'persona_1', causationId: 'message_1'});

    assert.equal(completion.text, 'visible');
    assert.equal(completion.toolCalls[0].name, 'scene_event');
    assert.deepEqual(completion.structuredTurn.control.affectEvents, []);
    assert.deepEqual(completion.structuredTurn.control.memoryWrites, []);
    assert.ok(completion.parseErrors.length > 0);
});

test('strict JSON completion content becomes a control turn instead of visible JSON', async () => {
    const provider = {
        async stream() {
            return jsonResponse({choices: [{message: {content: JSON.stringify({
                schemaVersion: 'companion.turn.v1',
                text: '来自结构化回复。',
                control: {affectEvents: []}
            })}}]});
        }
    };
    const port = createMtplxCompletionPort({provider, settings: () => ({model: 'fixture'})});
    const result = await port.stream({personaId: 'persona_1', causationId: 'message_1'});

    assert.equal(result.text, '来自结构化回复。');
    assert.equal(result.sourceMode, 'structured_sidecar');
    assert.deepEqual(result.control.affectEvents, []);
});
