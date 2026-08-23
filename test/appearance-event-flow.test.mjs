import assert from 'node:assert/strict';
import test from 'node:test';

import {
    APPEARANCE_EVENT_MAX_OUTFIT_LENGTH,
    createAppearanceEventFlow,
    normalizeAppearanceEventCall
} from '../server/application/appearance-event-flow.js';

const NOW = '2026-08-23T00:00:00.000Z';

function setup({appearance = {coat: 'blue', shoes: 'white'}} = {}) {
    const stateRows = new Map([['persona_1', {
        sourceEventId: null,
        situation: 'reading',
        mood: 'calm',
        appearance: {...appearance},
        appearanceJson: JSON.stringify(appearance)
    }]]);
    const source = {id: 'message_1', personaId: 'persona_1', role: 'user'};
    const lifeEvents = [];
    const calls = [];
    const repositories = {
        personaRepository: {
            findActive(id) { return id === 'persona_1' ? {id, name: 'Persona'} : null; },
            touch(input) { calls.push(['touch', input]); return {id: input.personaId}; }
        },
        messageRepository: {findById(input) { return input.id === source.id ? source : null; }},
        lifeEventRepository: {
            findByIdempotencyKey({personaId, idempotencyKey}) {
                return lifeEvents.find(row => row.personaId === personaId
                    && JSON.parse(row.payloadJson).idempotencyKey === idempotencyKey);
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
                if (row.sourceEventId !== (input.expected.sourceEventId || null)
                    || JSON.stringify(row.appearance) !== JSON.stringify(input.expected.appearance)) return {changes: 0};
                row.situation = input.situation;
                row.mood = input.mood;
                row.appearance = {...input.appearance};
                row.appearanceJson = input.appearanceJson;
                row.sourceEventId = input.sourceEventId;
                return {changes: 1};
            }
        }
    };
    let sequence = 0;
    const flow = createAppearanceEventFlow({
        repositories,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_${++sequence}`
    });
    return {flow, stateRows, lifeEvents, calls, source};
}

function command(overrides = {}) {
    return {
        personaId: 'persona_1',
        sourceMessageId: 'message_1',
        call: {operation: 'set', outfit: 'a white linen shirt', reason: 'getting ready'},
        provenance: {source: 'native', callId: 'call_appearance_1', idempotencyKey: 'appearance_1'},
        ...overrides
    };
}

test('appearance call normalization enforces operation and bounded set outfit', () => {
    assert.deepEqual(normalizeAppearanceEventCall({operation: 'set', outfit: '  linen shirt  ', reason: '  change  '}), {
        operation: 'set', outfit: 'linen shirt', reason: 'change'
    });
    assert.deepEqual(normalizeAppearanceEventCall({operation: 'clear', reason: '  finished  '}), {
        operation: 'clear', reason: 'finished'
    });
    assert.throws(() => normalizeAppearanceEventCall({operation: 'set'}), /requires outfit/);
    assert.throws(() => normalizeAppearanceEventCall({operation: 'set', outfit: 'x'.repeat(APPEARANCE_EVENT_MAX_OUTFIT_LENGTH + 1)}), /exceeds/);
    assert.throws(() => normalizeAppearanceEventCall({operation: 'replace', outfit: 'coat'}), /operation is invalid/);
    assert.throws(() => normalizeAppearanceEventCall({operation: 'set', outfit: 'coat', unsupported: true}), /unsupported/);
});

test('planning is read-only, previews merged appearance, and apply uses the caller transaction', () => {
    const fixture = setup();
    const beforeCalls = fixture.calls.length;
    const plan = fixture.flow.plan(command());

    assert.equal(plan.type, 'appearance_event_plan');
    assert.equal(plan.eventId, 'event_1');
    assert.deepEqual(plan.previousAppearance, {coat: 'blue', shoes: 'white'});
    assert.deepEqual(plan.nextAppearance, {coat: 'blue', shoes: 'white', outfit: 'a white linen shirt'});
    assert.deepEqual(plan.previewResult.appearance, plan.nextAppearance);
    assert.equal(fixture.lifeEvents.length, 0);
    assert.equal(fixture.calls.length, beforeCalls);

    let transactions = 0;
    const result = fixture.flow.apply(plan, {
        callerTransaction(work) {
            transactions += 1;
            return work();
        }
    });
    assert.equal(transactions, 1);
    assert.deepEqual(result, {
        eventId: 'event_1',
        operation: 'set',
        outfit: 'a white linen shirt',
        appearance: {coat: 'blue', shoes: 'white', outfit: 'a white linen shirt'},
        reason: 'getting ready'
    });
    assert.equal(fixture.lifeEvents.length, 1);
    assert.equal(fixture.lifeEvents[0].type, 'appearance_change');
    assert.equal(fixture.lifeEvents[0].causationId, 'message_1');
    const payload = JSON.parse(fixture.lifeEvents[0].payloadJson);
    assert.equal(payload.idempotencyKey, 'appearance_1');
    assert.equal(payload.operation, 'set');
    assert.equal(payload.outfit, 'a white linen shirt');
    assert.deepEqual(fixture.stateRows.get('persona_1').appearance, {
        coat: 'blue', shoes: 'white', outfit: 'a white linen shirt'
    });
});

test('clear removes only outfit, preserves other appearance keys, and replays by life-event payload', () => {
    const fixture = setup({appearance: {coat: 'blue', outfit: 'old dress', accessories: ['scarf']}});
    const setPlan = fixture.flow.plan(command());
    fixture.flow.apply(setPlan);
    const beforeReplayCalls = fixture.calls.length;

    const clearPlan = fixture.flow.plan(command({
        call: {operation: 'clear', reason: 'changed for the evening'},
        provenance: {source: 'native', idempotencyKey: 'appearance_clear_1'}
    }));
    assert.deepEqual(clearPlan.nextAppearance, {coat: 'blue', accessories: ['scarf']});
    const cleared = fixture.flow.apply(clearPlan);
    assert.equal(cleared.operation, 'clear');
    assert.equal(cleared.outfit, null);
    assert.deepEqual(cleared.appearance, {coat: 'blue', accessories: ['scarf']});
    assert.deepEqual(fixture.stateRows.get('persona_1').appearance, {coat: 'blue', accessories: ['scarf']});
    assert.equal(JSON.parse(fixture.lifeEvents.at(-1).payloadJson).outfit, null);

    const replayPlan = fixture.flow.plan(command({
        call: {operation: 'set', outfit: 'different'},
        provenance: {source: 'native', idempotencyKey: 'appearance_clear_1'}
    }));
    assert.equal(replayPlan.replayed, true);
    const replay = fixture.flow.apply(replayPlan);
    assert.equal(replay.replayed, true);
    assert.equal(replay.eventId, cleared.eventId);
    assert.equal(fixture.lifeEvents.length, 2);
    assert.equal(fixture.calls.length, beforeReplayCalls + 3);
});

test('source ownership and stale projection guards reject writes', () => {
    const fixture = setup();
    assert.throws(() => fixture.flow.plan(command({sourceMessageId: 'missing'})), /source message does not exist/);
    assert.throws(() => fixture.flow.plan(command({personaId: 'persona_2'})), /persona does not exist/);

    const stale = fixture.flow.plan(command({provenance: {idempotencyKey: 'appearance_stale_1'}}));
    fixture.stateRows.get('persona_1').appearance = {coat: 'changed'};
    assert.throws(() => fixture.flow.apply(stale), /current appearance projection/);
    assert.equal(fixture.lifeEvents.length, 0);
});
