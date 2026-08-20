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
import {createFlowExecutor, FlowExecutionError} from '../server/application/flow-executor.js';
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

test('the flow executor aggregates multi-step results and commits once', async () => {
    const registry = createFlowRegistry();
    registry.register({
        id: 'aggregate-flow',
        version: 1,
        layer: 'application',
        steps: [
            {
                id: 'first',
                run: async () => ({
                    facts: [{type: 'first'}],
                    projections: [{type: 'state', value: 1}],
                    effects: [],
                    presentation: [{type: 'token', token: 'one'}]
                })
            },
            {
                id: 'second',
                run: async (_context, _command, previous) => ({
                    facts: [{type: 'second', previousFacts: previous.facts.length}],
                    projections: [],
                    effects: [],
                    presentation: [{type: 'done'}]
                })
            }
        ]
    });
    const commits = [];
    const executor = createFlowExecutor({registry, commitBoundary: async result => commits.push(result)});

    const result = await executor.run('aggregate-flow', {correlationId: 'correlation_test', causationId: 'message_test'});

    assert.deepEqual(result, {
        facts: [{type: 'first'}, {type: 'second', previousFacts: 1}],
        projections: [{type: 'state', value: 1}],
        effects: [],
        presentation: [{type: 'token', token: 'one'}, {type: 'done'}]
    });
    assert.equal(commits.length, 1);
    assert.strictEqual(commits[0], result);
});

test('a failed step propagates a bounded error without committing', async () => {
    const registry = createFlowRegistry();
    registry.register({
        id: 'failing-flow',
        version: 1,
        layer: 'application',
        steps: [{
            id: 'explode',
            run: async () => { throw new Error('x'.repeat(1_000)); }
        }]
    });
    let commitCount = 0;
    const executor = createFlowExecutor({registry, commitBoundary: async () => { commitCount += 1; }});

    await assert.rejects(
        executor.run('failing-flow'),
        error => error instanceof FlowExecutionError
            && error.code === 'FLOW_EXECUTION_FAILED'
            && error.stepId === 'explode'
            && error.message.length <= 240
    );
    assert.equal(commitCount, 0);
});

test('the executor passes deterministic correlation and causation context to every step', async () => {
    const registry = createFlowRegistry();
    const received = [];
    registry.register({
        id: 'context-flow',
        version: 1,
        layer: 'application',
        steps: [{
            id: 'observe-context',
            run: async (context, command) => {
                received.push({context, command});
                return {facts: [], projections: [], effects: [], presentation: []};
            }
        }]
    });
    const executor = createFlowExecutor({registry});
    const context = {requestId: 'request_test', correlationId: 'correlation_test', causationId: 'message_test', personaId: 'persona_test'};
    const command = {value: 'command_test'};

    await executor.run('context-flow', context, command);

    assert.deepEqual(received, [{context, command}]);
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
    const commits = [];
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
        },
        commitBoundary: async result => commits.push(result)
    });
    const result = await composition.flowExecutor.run(
        'chat-turn',
        {personaId: call.personaId, correlationId: 'request_test'},
        {capabilityCalls: [call], causationId: call.causationUserMessageId}
    );
    assert.deepEqual(received.calls, [call]);
    assert.deepEqual(received.context, {personaId: 'persona_test', causationId: 'message_test', correlationId: 'request_test'});
    assert.deepEqual(result.effects, [effect]);
    assert.deepEqual(result.presentation, [{type: 'capability-result', result: {name: 'scene_event', ok: true, callId: 'call_test', idempotencyKey: 'cap_test', result: {eventId: 'event_test'}, error: null}}]);
    assert.deepEqual(composition.contracts.capability.call.name, CAPABILITY_NAMES);
    assert.equal(BACKEND_CONTRACT_BASELINE.sse.done.message, 'Message|null');
    assert.equal(commits.length, 1);
    assert.strictEqual(commits[0], result);
});
