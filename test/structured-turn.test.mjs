import assert from 'node:assert/strict';
import test from 'node:test';

import {
    AFFECT_EVENT_TYPES,
    DRIVE_NAMES,
    STRUCTURED_TURN_SCHEMA_VERSION,
    normalizeAffectEventCandidate,
    normalizeDriveSignalCandidate,
    normalizeMemoryWriteCandidate,
    normalizeStructuredTurnEnvelope,
    validateStructuredTurn
} from '../server/contracts/index.js';
import {normalizeStructuredTurn} from '../server/application/structured-turn.js';

const context = {personaId: 'persona_1', causationId: 'message_1'};

function memory(overrides = {}) {
    return {
        operation: 'upsert',
        key: 'favorite.drink',
        value: 'tea',
        confidence: 0.9,
        sourceMessageId: 'message_1',
        idempotencyKey: 'memory_1',
        ...overrides
    };
}

test('structured turn contract normalizes text, native calls, and bounded control candidates', () => {
    const result = normalizeStructuredTurn({
        text: '我记住了。',
        tokens: ['我记住了。'],
        toolCalls: [{
            id: 'call_1', index: 0, name: 'scene_event',
            arguments: {operation: 'start'}, argumentsText: '{"operation":"start"}',
            source: 'native', personaId: 'persona_1',
            causationUserMessageId: 'message_1', idempotencyKey: 'call_1'
        }]
    }, context);

    assert.equal(result.schemaVersion, STRUCTURED_TURN_SCHEMA_VERSION);
    assert.equal(result.sourceMode, 'native_tools');
    assert.equal(result.text, '我记住了。');
    assert.equal(result.control.capabilityCalls[0].name, 'scene_event');
    assert.deepEqual(result.control.affectEvents, []);
    assert.deepEqual(result.control.memoryWrites, []);
});

test('structured sidecar normalizes affect, extensible drives, memory, and messages', () => {
    const result = normalizeStructuredTurn({
        text: '收到。',
        structuredSidecar: {
            schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
            messages: [{role: 'assistant', text: '收到。'}],
            control: {
                affectEvents: [{
                    type: AFFECT_EVENT_TYPES[0], confidence: 0.8,
                    sourceMessageId: 'message_1', idempotencyKey: 'affect_1'
                }],
                driveSignals: [{
                    drive: DRIVE_NAMES[0], direction: 'decrease_pressure', confidence: 0.7,
                    idempotencyKey: 'drive_1'
                }],
                memoryWrites: [memory()]
            }
        }
    }, context);

    assert.equal(result.sourceMode, 'structured_sidecar');
    assert.equal(result.control.affectEvents[0].personaId, 'persona_1');
    assert.equal(result.control.driveSignals[0].recognized, true);
    assert.equal(result.control.memoryWrites[0].sourceMessageId, 'message_1');
    assert.deepEqual(result.messages, [{role: 'assistant', text: '收到。'}]);
});

test('memory and state candidates reject invalid confidence, values, and unsupported affect types', () => {
    assert.throws(() => normalizeAffectEventCandidate({
        type: 'not-registered', confidence: 0.5, idempotencyKey: 'a'
    }, context), /not supported/);
    assert.throws(() => normalizeDriveSignalCandidate({
        drive: 'social', direction: 'decrease_pressure', confidence: 2, idempotencyKey: 'd'
    }, context), /between 0 and 1/);
    assert.throws(() => normalizeMemoryWriteCandidate(memory({value: undefined}), context), /value must be provided/);
    assert.throws(() => normalizeMemoryWriteCandidate(memory({key: 'x'.repeat(121)}), context), /exceeds/);
    assert.throws(() => normalizeMemoryWriteCandidate(memory({personaId: 'persona_other'}), context), /application scope/);
});

test('malformed optional sidecar fails closed while preserving visible text and legacy marker mode', () => {
    const malformed = normalizeStructuredTurn({
        text: '已经处理。<pending-event>{"summary":"later"}</pending-event>',
        structuredSidecar: {schemaVersion: 'companion.turn.v999', control: {memoryWrites: [memory()]}}
    }, context);
    assert.equal(malformed.text, '已经处理。<pending-event>{"summary":"later"}</pending-event>');
    assert.equal(malformed.sourceMode, 'legacy_marker');
    assert.deepEqual(malformed.control, {affectEvents: [], driveSignals: [], memoryWrites: [], capabilityCalls: []});
    assert.ok(malformed.parseDiagnostics.length > 0);
    assert.equal(validateStructuredTurn(malformed, context).ok, true);
});

test('strict envelope deduplicates direct memory writes by idempotency key', () => {
    const result = normalizeStructuredTurnEnvelope({
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text: '',
        control: {memoryWrites: [memory(), memory({value: 'coffee'})]}
    }, context);
    assert.equal(result.control.memoryWrites.length, 1);
    assert.equal(result.control.memoryWrites[0].value, 'tea');
    assert.throws(() => normalizeStructuredTurnEnvelope({
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION, text: '', control: {}, unexpected: true
    }, context), /not supported/);
});

test('memory_event capability arguments share the memory candidate validator', () => {
    const nestedMemory = memory();
    delete nestedMemory.idempotencyKey;
    const result = normalizeStructuredTurnEnvelope({
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text: '已记住。',
        control: {capabilityCalls: [{
            name: 'memory_event', index: 0, source: 'structured', idempotencyKey: 'call_memory_1',
            arguments: {memory: nestedMemory}
        }]}
    }, context);
    assert.equal(result.control.capabilityCalls[0].name, 'memory_event');
    assert.equal(result.control.memoryWrites[0].idempotencyKey, 'call_memory_1');
});

test('native affect and drive tools become bounded state candidates instead of executable effects', () => {
    const result = normalizeStructuredTurn({
        text: '我先缓一缓。',
        toolCalls: [
            {
                id: 'call_affect_1', index: 0, name: 'affect_event', source: 'native',
                argumentsText: JSON.stringify({event: {type: 'fatigue', confidence: 0.8, idempotencyKey: 'affect_native_1'}}),
                arguments: {event: {type: 'fatigue', confidence: 0.8, idempotencyKey: 'affect_native_1'}},
                personaId: 'persona_1', causationUserMessageId: 'message_1', idempotencyKey: 'call_affect_1'
            },
            {
                id: 'call_drive_1', index: 1, name: 'drive_signal', source: 'native',
                argumentsText: JSON.stringify({signal: {drive: 'rest', direction: 'increase_pressure', confidence: 0.7, idempotencyKey: 'drive_native_1'}}),
                arguments: {signal: {drive: 'rest', direction: 'increase_pressure', confidence: 0.7, idempotencyKey: 'drive_native_1'}},
                personaId: 'persona_1', causationUserMessageId: 'message_1', idempotencyKey: 'call_drive_1'
            }
        ]
    }, context);
    assert.deepEqual(result.control.capabilityCalls, []);
    assert.equal(result.control.affectEvents[0].type, 'fatigue');
    assert.equal(result.control.driveSignals[0].drive, 'rest');
});
