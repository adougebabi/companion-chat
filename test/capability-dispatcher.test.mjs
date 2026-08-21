import assert from 'node:assert/strict';
import test from 'node:test';

import {createCapabilityDispatcher, MAX_CAPABILITY_DIAGNOSTIC_LENGTH} from '../server/application/capability-dispatcher.js';

function call(name, overrides = {}) {
    return {
        id: `${name}_call`, index: 0, name, argumentsText: '{}', arguments: {}, source: 'native',
        personaId: 'persona_1', causationUserMessageId: 'message_1', idempotencyKey: `${name}_key`, ...overrides
    };
}

function registry(calls) {
    return {
        scene_event: {
            cardinality: 1,
            execute({call: capabilityCall, mode}) {
                calls.push(['scene', capabilityCall.source, mode]);
                return mode === 'plan' ? {plan: {type: 'scene_plan', previewResult: {eventId: 'event_1'}}, result: {eventId: 'event_1'}} : {eventId: 'event_1'};
            },
            result(execution) { return {ok: !execution.error, error: execution.error || null, eventId: execution.result?.eventId || null}; }
        },
        media_event: {
            cardinality: 1,
            markerAdapter(text, context) {
                const marker = '<media-intent>';
                if (!text.includes(marker)) return {text, arguments: null};
                return {
                    text: text.replace(marker, ''),
                    call: call('media_event', {
                        id: null, index: context.index, source: 'marker', arguments: {kind: 'image'},
                        argumentsText: '{"kind":"image"}', idempotencyKey: 'marker_media_key'
                    })
                };
            },
            execute({call: capabilityCall}) {
                calls.push(['media', capabilityCall.source]);
                return {jobId: 'job_1'};
            },
            result(execution) { return {ok: !execution.error, error: execution.error || null, jobId: execution.result?.jobId || null}; }
        },
        pending_event: {
            cardinality: 1,
            markerAdapter(text) {
                if (!text.includes('<pending-event>')) return {text, arguments: null};
                return {text: text.replace('<pending-event>', ''), arguments: null};
            },
            execute() {
                calls.push(['pending']);
                return {pendingEventId: 'pending_1'};
            }
        }
    };
}

test('native calls execute in provider order and expose bounded continuation entries', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({
        mode: 'execute',
        calls: [call('media_event', {index: 2}), call('scene_event', {index: 0})],
        completion: {doneSeen: true},
        markerText: '<media-intent><pending-event>visible'
    });
    assert.deepEqual(calls, [['scene', 'native', 'execute'], ['media', 'native']]);
    assert.equal(result.visibleText, 'visible');
    assert.deepEqual(result.continuationEntries.map(entry => entry.call.name), ['scene_event', 'media_event']);
    assert.equal(result.byCapability.scene_event.error, null);
    assert.equal(result.byCapability.media_event.error, null);
});

test('plan mode passes mode to entries without invoking a second execution path', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({mode: 'plan', calls: [call('scene_event')], completion: {doneSeen: true}});
    assert.deepEqual(calls, [['scene', 'native', 'plan']]);
    assert.equal(result.byCapability.scene_event.plan.type, 'scene_plan');
    assert.equal(result.byCapability.scene_event.result.eventId, 'event_1');
});

test('unknown native calls fail closed and strip all marker side effects', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({
        calls: [call('unknown_event', {name: 'unknown_event'})],
        completion: {doneSeen: true},
        markerText: '<media-intent><pending-event>reply'
    });
    assert.equal(result.unknownNative, true);
    assert.equal(result.visibleText, 'reply');
    assert.deepEqual(calls, []);
    assert.match(result.diagnostics[0], /Unknown native/);
});

test('native presence blocks matching marker, duplicate cardinality prevents execution, and malformed native is non-fatal', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const duplicate = dispatcher.dispatch({
        calls: [call('media_event', {index: 0}), call('media_event', {id: 'media_2', index: 1})],
        completion: {doneSeen: true},
        markerText: '<media-intent>reply'
    });
    assert.deepEqual(calls, []);
    assert.match(duplicate.byCapability.media_event.error, /cardinality/);
    assert.equal(duplicate.visibleText, 'reply');

    calls.length = 0;
    const malformed = dispatcher.dispatch({
        calls: [call('media_event', {error: 'bad native arguments'})],
        completion: {doneSeen: true},
        markerText: '<media-intent>reply'
    });
    assert.deepEqual(calls, []);
    assert.equal(malformed.byCapability.media_event.error, 'bad native arguments');
    assert.equal(malformed.visibleText, 'reply');
});

test('marker adapter can return a normalized marker call and diagnostics remain bounded', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({
        personaId: 'persona_1', causationUserMessageId: 'message_1',
        calls: [], markerText: '<media-intent>reply', completion: {doneSeen: true, parseErrors: ['x'.repeat(MAX_CAPABILITY_DIAGNOSTIC_LENGTH + 40)]}
    });
    assert.deepEqual(calls, [['media', 'marker']]);
    assert.equal(result.byCapability.media_event.call.source, 'marker');
    assert.equal(result.diagnostics[0].length, MAX_CAPABILITY_DIAGNOSTIC_LENGTH);
});

test('native incomplete index blocks marker fallback and records one bounded diagnostic', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({
        calls: [call('media_event', {index: 4})],
        completion: {doneSeen: false, incompleteToolIndexes: [4]},
        markerText: '<media-intent>reply'
    });
    assert.deepEqual(calls, []);
    assert.equal(result.byCapability.media_event.error, 'Native capability call did not complete');
    assert.equal(result.continuationEntries.length, 1);
    assert.equal(result.diagnostics.length, 1);
});

test('missing completion boundary fails native calls closed instead of executing them', () => {
    const calls = [];
    const dispatcher = createCapabilityDispatcher({registry: registry(calls)});
    const result = dispatcher.dispatch({calls: [call('scene_event')], markerText: 'reply'});
    assert.deepEqual(calls, []);
    assert.equal(result.byCapability.scene_event.error, 'Native capability call did not complete');
});

test('registry entry receivers are retained for execute, marker, and result callbacks', () => {
    const receiver = {executed: 0, marked: 0, resulted: 0};
    const dispatcher = createCapabilityDispatcher({registry: {
        demo: {
            cardinality: 1,
            execute() { this.executed += 1; return {ok: true}; },
            markerAdapter(text) { this.marked += 1; return {text, arguments: null}; },
            result(execution) { this.resulted += 1; return {ok: !execution.error}; },
            receiver
        }
    }});
    const result = dispatcher.dispatch({calls: [call('demo')], completion: {doneSeen: true}});
    assert.equal(receiver.executed, 1);
    assert.equal(receiver.marked, 1);
    assert.equal(dispatcher.get('demo').receiver, receiver);
    assert.deepEqual(result.continuationEntries[0].result, {ok: true});
    assert.equal(receiver.resulted, 1);
});
