import assert from 'node:assert/strict';
import test from 'node:test';

import {createSceneEventFlow} from '../server/application/scene-event-flow.js';

const NOW = '2026-08-20T00:00:00.000Z';

function sceneCall(overrides = {}) {
    return {
        operation: 'start', location: 'quiet cafe', room: 'window seat', activity: 'talking together',
        situation: 'We are talking together by the cafe window.', mood: 'calm', objects: ['tea'], participants: ['user', 'persona'],
        ...overrides
    };
}

function setup({state = {}, events = [], source = {id: 'message_1', personaId: 'persona_1', role: 'user'}} = {}) {
    const stateRows = new Map([['persona_1', {sourceEventId: null, sharedSceneJson: '{}', sharedScene: null, ...state}]]);
    const lifeEvents = [...events];
    const calls = [];
    const repositories = {
        personaRepository: {findActive(id) { return id === 'persona_1' ? {id, name: 'Persona'} : null; }, touch(input) { calls.push(['touch', input]); return {id: input.personaId}; }},
        messageRepository: {findById(input) { return input.id === source.id ? source : null; }},
        lifeEventRepository: {
            findByIdempotencyKey({personaId, idempotencyKey}) {
                return lifeEvents.find(row => row.personaId === personaId && JSON.parse(row.payloadJson || row.payload_json || '{}').idempotencyKey === idempotencyKey);
            },
            createEvent(input) {
                calls.push(['event', input]);
                const row = {...input, payloadJson: JSON.stringify(input.payload)};
                lifeEvents.push(row);
                return row;
            }
        },
        stateRepository: {
            read(personaId) { return stateRows.get(personaId); },
            updateProjection(input) {
                calls.push(['state', input]);
                const row = stateRows.get(input.personaId);
                const expected = input.expected;
                if (row.sourceEventId !== (expected.sourceEventId || null) || row.sharedSceneJson !== (expected.sharedSceneJson || '{}')) return {changes: 0};
                row.sourceEventId = input.sourceEventId;
                row.sharedSceneJson = input.sharedSceneJson;
                row.sharedScene = input.sharedScene;
                row.situation = input.situation;
                row.mood = input.mood;
                return {changes: 1};
            }
        }
    };
    let nextId = 0;
    const flow = createSceneEventFlow({
        repositories,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_${++nextId}`,
        scheduledState: () => ({situation: 'fallback schedule', mood: 'steady'})
    });
    return {flow, calls, stateRows, lifeEvents, repositories};
}

function command(overrides = {}) {
    return {
        personaId: 'persona_1',
        sourceMessageId: 'message_1',
        call: sceneCall(),
        provenance: {source: 'native', callId: 'call_scene', idempotencyKey: 'scene_1'},
        ...overrides
    };
}

test('plan is read-only, validates source ownership, and preallocates one event id', () => {
    const fixture = setup();
    const before = fixture.calls.length;
    const plan = fixture.flow.plan(command());
    assert.equal(plan.type, 'scene_event_plan');
    assert.equal(plan.eventId, 'event_1');
    assert.equal(plan.preallocatedIds.eventId, plan.eventId);
    assert.equal(plan.previewResult.scene.eventId, plan.eventId);
    assert.equal(plan.previousScene, null);
    assert.equal(fixture.calls.length, before);
    assert.equal(fixture.lifeEvents.length, 0);
});

test('apply uses caller transaction and replay is idempotent by persona/idempotency key', () => {
    const fixture = setup();
    const plan = fixture.flow.plan(command());
    let transactions = 0;
    const first = fixture.flow.apply(plan, {callerTransaction(work) { transactions += 1; return work(); }});
    assert.equal(transactions, 1);
    assert.equal(first.eventId, plan.eventId);
    assert.equal(first.operation, 'start');
    assert.equal(fixture.lifeEvents.length, 1);
    assert.equal(fixture.stateRows.get('persona_1').sourceEventId, plan.eventId);

    const replayPlan = fixture.flow.plan(command());
    assert.equal(replayPlan.replayed, true);
    const replay = fixture.flow.apply(replayPlan);
    assert.equal(replay.replayed, true);
    assert.equal(replay.eventId, first.eventId);
    assert.equal(fixture.lifeEvents.length, 1);
});

test('expected projection guard rejects a stale plan before writing a second event', () => {
    const fixture = setup();
    const stale = fixture.flow.plan(command({provenance: {idempotencyKey: 'stale_1'}}));
    fixture.stateRows.get('persona_1').sourceEventId = 'event_other';
    fixture.stateRows.get('persona_1').sharedSceneJson = JSON.stringify({eventId: 'event_other', location: 'other', activity: 'else', situation: 'Elsewhere'});
    assert.throws(() => fixture.flow.apply(stale), /current scene projection/);
    assert.equal(fixture.lifeEvents.length, 0);
});

test('end and switch preserve the previous projection and scheduledState supplies only missing fallback facts', () => {
    const current = {eventId: 'event_existing', location: 'park', room: '', activity: 'walking', situation: 'Walking together', mood: 'calm', objects: [], participants: ['user', 'persona'], startedAt: NOW};
    const fixture = setup({state: {sourceEventId: 'event_existing', sharedSceneJson: JSON.stringify(current), sharedScene: current}});
    const end = fixture.flow.plan(command({call: sceneCall({operation: 'end'}), provenance: {idempotencyKey: 'end_1'}}));
    assert.equal(end.nextScene, null);
    assert.deepEqual(end.previousScene, current);
    const switched = fixture.flow.plan(command({call: sceneCall({operation: 'switch', location: '', activity: 'reading', situation: 'Reading together'}), provenance: {idempotencyKey: 'switch_1'}}));
    assert.equal(switched.nextScene.activity, 'reading');
    assert.equal(switched.nextScene.situation, 'Reading together');
});

test('injected normalizer is used and unsupported scene fields remain rejected', () => {
    const calls = [];
    const fixture = setup();
    const flow = createSceneEventFlow({
        repositories: fixture.repositories,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_custom`,
        scheduledState: () => ({situation: 'fallback', mood: 'steady'}),
        normalizeCall(value) { calls.push(value); return value; }
    });
    flow.plan(command({call: sceneCall({mood: ' calm '})}));
    assert.equal(calls.length, 1);
    assert.throws(() => flow.plan(command({call: {...sceneCall(), unsupported: true}, provenance: {idempotencyKey: 'bad'}})), /unsupported/);
});
