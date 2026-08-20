import assert from 'node:assert/strict';
import test from 'node:test';

import {
    BACKEND_CONTRACT_BASELINE,
    CAPABILITY_NAMES,
    normalizeCapabilityCall,
    normalizeSseEvent,
    sseDone,
    sseError,
    sseToken,
    validateLayerDependencies,
    validatePorts
} from '../server/contracts/index.js';
import {createFlowRegistry} from '../server/application/flow-registry.js';
import {createCompositionRoot} from '../server/index.js';

function ports() {
    return {
        clock: {now() { return new Date(0); }},
        idGenerator: {next() { return 'id_test'; }},
        llm: {complete() {}, stream() {}},
        conversationRepository: {appendMessage() {}, listMessages() {}},
        identityRepository: {findById() {}},
        memoryRepository: {listActive() {}},
        lifeEventRepository: {record() {}},
        scheduleRepository: {list() {}},
        presenceRepository: {read() {}},
        activityRepository: {publish() {}},
        effectRepository: {record() {}, settle() {}},
        mediaProvider: {submit() {}},
        assetRepository: {find() {}}
    };
}

function capabilityCall(overrides = {}) {
    return normalizeCapabilityCall({
        id: 'call_test',
        index: 0,
        name: 'scene_event',
        argumentsText: '{"operation":"start"}',
        arguments: {operation: 'start'},
        source: 'native',
        personaId: 'persona_test',
        causationUserMessageId: 'message_test',
        idempotencyKey: 'cap_test',
        ...overrides
    });
}

test('the flow registry registers and runs typed step results once', async () => {
    const registry = createFlowRegistry();
    registry.register({
        id: 'contract-test-flow',
        version: 1,
        layer: 'application',
        dependencies: [{id: 'contracts', layer: 'contracts'}],
        steps: [{
            id: 'emit-contract-fixture',
            layer: 'application',
            dependencies: [{id: 'step-contract', layer: 'contracts'}],
            run: async () => ({facts: [{type: 'test'}], projections: [], effects: [], presentation: [{type: 'test'}]})
        }]
    });
    assert.equal(registry.has('contract-test-flow'), true);
    assert.deepEqual(registry.list(), [{id: 'contract-test-flow', version: 1, layer: 'application', stepIds: ['emit-contract-fixture']}]);
    assert.deepEqual(await registry.run('contract-test-flow'), {facts: [{type: 'test'}], projections: [], effects: [], presentation: [{type: 'test'}]});
    assert.throws(() => registry.register({id: 'contract-test-flow', version: 1, layer: 'application', steps: []}), /already registered/);
});

test('layer direction and required backend ports are explicit', () => {
    assert.equal(validateLayerDependencies('application', [{layer: 'domain'}]), true);
    assert.equal(validateLayerDependencies('domain', [{layer: 'contracts'}]), true);
    assert.throws(() => validateLayerDependencies('domain', [{layer: 'infrastructure'}]), /cannot depend/);
    assert.doesNotThrow(() => validatePorts(ports()));
    const missing = ports();
    delete missing.effectRepository.settle;
    assert.throws(() => validatePorts(missing), /effectRepository\.settle/);
});

test('SSE compatibility keeps done.messages authoritative and done.message as alias', () => {
    const message = {id: 'message_test', role: 'assistant', text: 'Ready.', attachments: [], jobs: []};
    const done = sseDone({messages: [message], message: {id: 'stale_alias'}, learned: [], jobs: []});
    assert.equal(done.type, 'done');
    assert.deepEqual(done.messages, [message]);
    assert.deepEqual(done.message, message);
    assert.deepEqual(sseDone({message}).messages, [message]);
    assert.equal(sseDone({messages: []}).message, null);
    assert.deepEqual(sseToken('visible'), {type: 'token', token: 'visible'});
    assert.deepEqual(sseError('provider failed'), {type: 'error', error: 'provider failed'});
    assert.deepEqual(normalizeSseEvent({type: 'done', messages: []}).message, null);
    assert.throws(() => normalizeSseEvent({type: 'tool', arguments: '{}'}), /not supported/);
});

test('native CapabilityCall hands off to CapabilityResult and EffectIntent', async () => {
    const call = capabilityCall();
    const effect = {
        effectId: 'effect_test',
        kind: 'scene-event',
        capability: 'scene_event',
        idempotencyKey: call.idempotencyKey,
        causationId: call.causationUserMessageId,
        payload: {operation: 'start'}
    };
    let received;
    const composition = createCompositionRoot({
        ports: ports(),
        capabilityDispatcher: {
            async dispatch(input) {
                received = input;
                return {
                    results: [{name: 'scene_event', ok: true, callId: call.id, idempotencyKey: call.idempotencyKey, result: {eventId: 'event_test'}, error: null}],
                    effects: [effect]
                };
            }
        }
    });
    const result = await composition.flows.run('chat-turn', {personaId: call.personaId, correlationId: 'request_test'}, {capabilityCalls: [call], causationId: call.causationUserMessageId});
    assert.deepEqual(received.calls, [call]);
    assert.deepEqual(received.context, {personaId: 'persona_test', causationId: 'message_test', correlationId: 'request_test'});
    assert.deepEqual(result.effects, [effect]);
    assert.deepEqual(result.presentation, [{type: 'capability-result', result: {name: 'scene_event', ok: true, callId: 'call_test', idempotencyKey: 'cap_test', result: {eventId: 'event_test'}, error: null}}]);
    assert.deepEqual(composition.contracts.capability.call.name, CAPABILITY_NAMES);
    assert.equal(BACKEND_CONTRACT_BASELINE.sse.done.message, 'Message|null');
});
