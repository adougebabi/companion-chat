import assert from 'node:assert/strict';
import test from 'node:test';

import {createFlowEffectAdapter, registerFlowAdapter} from '../server/application/flow-effect-adapter.js';
import {createFlowRegistry} from '../server/application/flow-registry.js';
import {createContextPipeline} from '../server/application/context-pipeline.js';

test('flow effect adapter publishes one durable job for an idempotent effect', () => {
    const jobs = [];
    const repository = {
        findByPayload({value}) {
            return jobs.find(job => job.payload.idempotencyKey === value) ?? null;
        },
        enqueue(input) {
            jobs.push({id: input.id, job_type: input.jobType, payload: input.payload});
            return jobs.at(-1);
        }
    };
    const effects = createFlowEffectAdapter({
        jobRepository: repository,
        clock: () => '2026-08-22T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_test`
    });
    const intent = {
        effectId: 'effect_test',
        kind: 'pending_event',
        capability: 'pending',
        idempotencyKey: 'pending:one',
        causationId: 'message_test',
        payload: {personaId: 'persona_test', pendingEventId: 'pending_test'}
    };

    const first = effects.publish(intent, {personaId: 'persona_test'});
    const replay = effects.publish(intent, {personaId: 'persona_test'});

    assert.equal(first.created, true);
    assert.equal(replay.created, false);
    assert.strictEqual(replay.job, first.job);
    assert.equal(jobs.length, 1);
    assert.equal(jobs[0].payload.idempotencyKey, 'pending:one');
});

test('registered application flow exposes normalized facts, projections, effects and presentation', async () => {
    const registry = createFlowRegistry();
    registerFlowAdapter(registry, {
        id: 'timeline',
        flow: {version: 1},
        execute: async () => ({
            facts: [{type: 'timeline_fact'}],
            projections: [{type: 'timeline_projection'}],
            effects: [{effectId: 'effect_timeline', kind: 'timeline.activity_decision', idempotencyKey: 'timeline:one', causationId: 'decision_one', payload: {}}],
            presentation: [{type: 'timeline_event'}]
        })
    });

    const result = await registry.run('timeline', {personaId: 'persona_test'}, {});
    assert.deepEqual(result.facts, [{type: 'timeline_fact'}]);
    assert.deepEqual(result.projections, [{type: 'timeline_projection'}]);
    assert.equal(result.effects[0].capability, 'timeline');
    assert.deepEqual(result.presentation, [{type: 'timeline_event'}]);
});

test('context budget reserves required fragments and serializer only renders selected fragments', () => {
    const pipeline = createContextPipeline({maxChars: 512, maxFragments: 4});
    const fragments = pipeline.collect({fragments: [
        {section: 'optional_large', priority: 100, text: 'x'.repeat(600), required: false, provenance: {source: 'memory'}},
        {section: 'identity', priority: 100, text: 'confirmed identity', required: true, budget: 18, provenance: {source: 'identity'}},
        {section: 'life', priority: 90, text: 'confirmed life state', required: true, budget: 20, provenance: {source: 'life'}}
    ]});
    const budget = pipeline.budget(fragments);

    assert.equal(fragments[0].section, 'identity');
    assert.equal(fragments[0].source, 'identity');
    assert.equal(fragments[0].required, true);
    assert.deepEqual(fragments[0].provenance, {source: 'identity'});
    assert.deepEqual(budget.fragments.map(item => item.section), ['identity', 'life']);
    assert.equal(budget.required.length, 2);
    assert.equal(budget.optional.length, 0);
    assert.equal(budget.used, 38);
    assert.match(pipeline.serialize({budget}), /\[identity\]/);
    assert.doesNotMatch(pipeline.serialize({budget}), /optional_large/);
    // Without an explicit budget the serializer renders the supplied list;
    // selection is exclusively the budgeter's responsibility.
    assert.match(pipeline.serialize({fragments: fragments.slice(0, 1)}), /\[identity\]/);
});
