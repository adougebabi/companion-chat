import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

import {
    CAPABILITY_EFFECT_STATUSES,
    createCapabilityEffectPlan,
    normalizeCapabilityEffectEntry,
    normalizeCapabilityEffectPlan,
    projectCapabilityEffectPlan,
    serializeCapabilityEffectPlan
} from '../server/application/capability-effect-plan.js';

function entry(capability, overrides = {}) {
    return {
        capability,
        order: {scene_event: 2, media_event: 1, pending_event: 0}[capability],
        call: {
            id: `call_${capability}`,
            index: {scene_event: 2, media_event: 1, pending_event: 0}[capability],
            source: 'native',
            personaId: 'persona_test',
            causationUserMessageId: 'message_test',
            idempotencyKey: `dedupe_${capability}`
        },
        causationId: 'message_test',
        previewResult: {ok: true, [`${capability}_id`]: `${capability}_preview`},
        ...overrides
    };
}

test('normalizes bounded staged entries and keeps the handler outside serialized data', () => {
    const apply = async () => ({eventId: 'event_committed'});
    const normalized = normalizeCapabilityEffectEntry({...entry('scene_event'), apply});
    const plan = normalizeCapabilityEffectPlan({entries: [normalized]});

    assert.equal(plan.status, 'preview');
    assert.equal(plan.entries[0].capability, 'scene_event');
    assert.equal(plan.entries[0].status, 'preview');
    assert.equal(plan.entries[0].provenance.idempotencyKey, 'dedupe_scene_event');
    assert.equal(JSON.stringify(plan).includes('apply'), false);
    assert.equal(JSON.stringify(plan).includes('arguments'), false);
    assert.deepEqual(serializeCapabilityEffectPlan(plan).entries[0].preallocatedIds, {});
    assert.equal(normalizeCapabilityEffectPlan(plan.serialize()).status, 'preview');
    assert.throws(() => normalizeCapabilityEffectEntry({...entry('unknown') , capability: 'unknown'}), /Unknown capability/);
    assert.throws(() => createCapabilityEffectPlan({entries: [entry('scene_event'), entry('scene_event', {order: 3, call: {...entry('scene_event').call, id: 'call_scene_2', idempotencyKey: 'dedupe_scene_2'}})]}), /Duplicate capability effect capability/);
});

test('sorts entries deterministically by explicit order and rejects duplicate provenance', () => {
    const plan = createCapabilityEffectPlan({entries: [entry('scene_event'), entry('pending_event'), entry('media_event')]});
    assert.deepEqual(plan.entries.map(item => item.capability), ['pending_event', 'media_event', 'scene_event']);
    assert.deepEqual(plan.entries.map(item => item.order), [0, 1, 2]);
    assert.throws(() => createCapabilityEffectPlan({entries: [
        entry('scene_event', {order: 0}),
        entry('media_event', {order: 0, call: {...entry('media_event').call, idempotencyKey: 'other_key'}})
    ]}), /Duplicate capability effect order/);
    assert.throws(() => createCapabilityEffectPlan({entries: [
        entry('scene_event'),
        entry('media_event', {order: 3, call: {...entry('media_event').call, id: 'call_scene_event', idempotencyKey: 'other_key'}})
    ]}), /Duplicate capability effect callId/);
});

test('projects bounded continuation/browser results without raw arguments or sensitive provenance', () => {
    const plan = createCapabilityEffectPlan({entries: [entry('media_event', {
        preallocatedIds: {jobId: 'job_preview'},
        arguments: {prompt: 'private prompt', apiKey: 'provider-secret'},
        previewResult: {
            ok: true,
            mediaId: 'media_preview',
            dedupeKey: 'private-dedupe',
            arguments: {token: 'private-token'},
            providerSecret: 'private-provider-secret',
            text: 'x'.repeat(2_000)
        }
    })]});
    const projection = projectCapabilityEffectPlan(plan);
    assert.deepEqual(Object.keys(projection), ['version', 'status', 'results']);
    assert.equal(projection.status, 'preview');
    assert.equal(projection.results.length, 1);
    assert.deepEqual(projection.results[0].preallocatedIds, {jobId: 'job_preview'});
    assert.equal(projection.results[0].result.dedupeKey, undefined);
    assert.equal(projection.results[0].result.arguments, undefined);
    assert.equal(projection.results[0].result.providerSecret, undefined);
    assert.equal(projection.results[0].result.text.length, 480);
    assert.equal(JSON.stringify(projection).includes('private-dedupe'), false);
    assert.equal(JSON.stringify(projection).includes('private-token'), false);
    assert.equal(JSON.stringify(projection).includes('provider-secret'), false);
    assert.strictEqual(projection.entries, projection.results);
});

test('commits preview entries in deterministic order and records bounded failure state', async () => {
    const events = [];
    const plan = createCapabilityEffectPlan({entries: [
        entry('scene_event', {apply: async ({capability}) => { events.push(capability); return {eventId: 'event_committed'}; }}),
        entry('media_event', {apply: async ({capability}) => { events.push(capability); return {jobId: 'job_committed'}; }, order: 1}),
        entry('pending_event', {apply: async ({capability}) => { events.push(capability); return {pendingId: 'pending_committed'}; }, order: 0})
    ]});
    const committed = await plan.commit();
    assert.deepEqual(events, ['pending_event', 'media_event', 'scene_event']);
    assert.equal(plan.status, 'committed');
    assert.equal(committed.status, 'committed');
    assert.deepEqual(committed.results.map(result => result.status), ['committed', 'committed', 'committed']);
    assert.deepEqual(committed.results.map(result => result.preallocatedIds), [{}, {}, {}]);
    const replayed = normalizeCapabilityEffectPlan(plan.serialize());
    assert.equal(replayed.status, 'committed');
    await plan.commit();
    assert.deepEqual(events, ['pending_event', 'media_event', 'scene_event']);

    const failing = createCapabilityEffectPlan({entries: [
        entry('scene_event', {apply: async () => { throw new Error(`provider token=super-secret ${'x'.repeat(400)}`); }})
    ]});
    const failed = await failing.apply();
    assert.equal(failing.status, 'failed');
    assert.equal(failed.results[0].status, 'failed');
    assert.equal(failed.results[0].ok, false);
    assert.equal(failed.results[0].error.includes('super-secret'), false);
    assert.equal(failed.results[0].error.length <= 240, true);
    assert.equal(normalizeCapabilityEffectPlan(failing.serialize()).status, 'failed');
    await assert.rejects(failing.commit(), /Cannot commit a failed/);
});

test('the contract module has no runtime adapter imports', () => {
    const source = readFileSync(new URL('../server/application/capability-effect-plan.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /from ['"].*(?:infrastructure|server\.js|better-sqlite|express)/);
    assert.deepEqual(CAPABILITY_EFFECT_STATUSES, ['preview', 'committed', 'failed']);
});
