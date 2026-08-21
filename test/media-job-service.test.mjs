import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    MEDIA_POLL_JOB_TYPES,
    MEDIA_SUBMIT_JOB_TYPES,
    createMediaJobService
} from '../server/application/media-job-service.js';

const NOW = '2026-08-21T00:00:00.000Z';

function job(overrides = {}) {
    return {
        id: 'job_media_1',
        job_type: 'chat_image',
        status: 'leased',
        lease_owner: 'worker_1',
        lease_expires_at: '2026-08-21T00:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: 'persona_1',
        message_id: 'message_1',
        payload_json: JSON.stringify({
            kind: 'image',
            provider: 'fixture',
            envelope: {schemaVersion: 1, mediaKind: 'image'},
            personaMediaConcept: {schemaVersion: 1, mediaKind: 'image'},
            capabilityCall: {idempotencyKey: 'media_1'}
        }),
        result_json: '{}',
        ...overrides
    };
}

function fixture(overrides = {}) {
    const jobs = new Map();
    const calls = [];
    const targets = [];
    let counter = 0;
    const repo = {
        find(input) {
            const id = typeof input === 'string' ? input : input.id;
            return jobs.get(id) || null;
        },
        findLeased(input) {
            calls.push(['findLeased', input]);
            const found = jobs.get(input.id);
            if (!found || found.status !== 'leased' || found.lease_owner !== input.leaseOwner) return null;
            if (Date.parse(found.lease_expires_at) <= Date.parse(input.now)) return null;
            return found;
        },
        patchResult(found, input) {
            calls.push(['patchResult', found.id, input.patch]);
            const current = JSON.parse(found.result_json || '{}');
            found.result_json = JSON.stringify({...current, ...input.patch});
            return {changed: true, result: JSON.parse(found.result_json), job: found};
        },
        settle(input) {
            calls.push(['settle', input]);
            const found = jobs.get(input.id);
            if (!found || found.status !== 'leased' || found.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            found.status = input.status;
            found.result_json = JSON.stringify(input.result || JSON.parse(found.result_json || '{}'));
            found.error = input.error || null;
            found.lease_owner = null;
            found.lease_expires_at = null;
            return {changed: true, status: input.status, job: found};
        },
        retry(input) {
            calls.push(['retry', input]);
            const found = jobs.get(input.id);
            if (!found || found.status !== 'leased' || found.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            found.status = 'queued';
            found.run_after = input.runAfter;
            found.error = input.error || null;
            found.lease_owner = null;
            found.lease_expires_at = null;
            return {changed: true, status: 'retry', job: found};
        },
        enqueue(input) {
            const next = {id: input.id || `job_poll_${++counter}`, status: 'queued', ...input};
            jobs.set(next.id, next);
            calls.push(['enqueue', next]);
            return next;
        }
    };
    const provider = {
        id: 'fixture',
        capabilities: ['image', 'video'],
        async submit(input) {
            calls.push(['submit', input]);
            return {externalId: 'external_1', pending: true};
        },
        async poll(input) {
            calls.push(['poll', input]);
            return {status: 'complete', files: [{filename: 'fixture.png', type: 'output'}]};
        },
        ...overrides.provider
    };
    const observability = {
        createReporter() {
            return {
                stage(stage) { calls.push(['stage', stage]); return {changed: true}; },
                output(stream, value) { calls.push(['output', stream, value]); return {changed: true}; },
                flush() { calls.push(['flush']); return {changed: true}; }
            };
        },
        settle(found, input) {
            calls.push(['observability.settle', found.id, input.status]);
            return input.status === 'retry' ? repo.retry(input) : repo.settle(input);
        },
        ...overrides.observability
    };
    const mediaFlow = {
        updateTarget(input) {
            targets.push(input);
            return {changed: true};
        },
        persistAssets(input) {
            calls.push(['persistAssets', input]);
            return input.files.map((file, index) => ({id: `asset_${index + 1}`, kind: 'image', file}));
        },
        ...overrides.mediaFlow
    };
    const service = createMediaJobService({
        providers: {fixture: provider},
        observability,
        repositories: {job: repo, mediaFlow},
        promptMaster: {
            fill(input) {
                calls.push(['prompt.fill', input]);
                return {template: {scene: input.envelope.mediaKind}};
            },
            render(template) {
                calls.push(['prompt.render', template]);
                return 'A bounded provider prompt';
            }
        },
        acceptance: {
            accept(input) {
                calls.push(['accept', input]);
                return {verdict: 'pass', observedFacts: {sceneMatches: true}};
            }
        },
        clock: () => NOW
    });
    const source = job();
    jobs.set(source.id, source);
    return {service, repo, jobs, calls, targets, source, provider};
}

test('media job service exposes one registration map for submit and poll jobs', () => {
    const {service} = fixture();
    assert.deepEqual(service.list(), [...MEDIA_SUBMIT_JOB_TYPES, ...MEDIA_POLL_JOB_TYPES]);
    assert.deepEqual(service.list().map(type => service.operationFor(type)), [
        ...MEDIA_SUBMIT_JOB_TYPES.map(() => 'submit'),
        ...MEDIA_POLL_JOB_TYPES.map(() => 'poll')
    ]);
    const registered = [];
    const target = {register(type, handler, receiver) { registered.push([type, handler, receiver]); }};
    assert.strictEqual(service.register(target), target);
    assert.deepEqual(registered.map(([type]) => type), service.list());
    assert.equal(registered.every(([, handler]) => typeof handler === 'function'), true);
});

test('submit freezes the prompt result, settles the source, and enqueues one poll job', async () => {
    const fixtureValue = fixture();
    const result = await fixtureValue.service.handlers.chat_image(fixtureValue.source, {leaseOwner: 'worker_1'});

    assert.equal(result.status, 'complete');
    assert.equal(fixtureValue.provider.submit ? fixtureValue.source.status : null, 'complete');
    assert.equal(fixtureValue.source.status, 'complete');
    assert.equal(fixtureValue.source.error, null);
    assert.equal(fixtureValue.jobs.size, 2);
    const pollJob = [...fixtureValue.jobs.values()].find(item => item.jobType === 'chat_media_poll');
    assert.ok(pollJob);
    assert.equal(pollJob.payload.externalId, 'external_1');
    const submitCall = fixtureValue.calls.find(([kind]) => kind === 'submit');
    assert.equal(submitCall[1].prompt, 'A bounded provider prompt');
    assert.equal(fixtureValue.targets.at(-1).status, 'processing');
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'observability.settle'), true);
});

test('poll accepts completed provider output and projects ready assets', async () => {
    const fixtureValue = fixture();
    const poll = job({
        id: 'poll_1',
        job_type: 'chat_media_poll',
        payload_json: JSON.stringify({kind: 'image', provider: 'fixture', externalId: 'external_1', sourceJobId: 'job_media_1'})
    });
    fixtureValue.jobs.set(poll.id, poll);

    const result = await fixtureValue.service.handlers.chat_media_poll(poll, {leaseOwner: 'worker_1'});

    assert.equal(result.status, 'complete');
    assert.equal(poll.status, 'complete');
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'poll'), true);
    assert.equal(fixtureValue.targets.at(-1).status, 'ready');
    assert.equal(fixtureValue.targets.at(-1).attachments[0].id, 'asset_1');
    assert.equal(fixtureValue.calls.find(([kind]) => kind === 'accept')[1].sourceJob.id, 'job_media_1');
});

test('provider failures share bounded retry and terminal settlement behavior', async () => {
    const fixtureValue = fixture({provider: {
        async submit() { throw new Error(`provider secret ${'x'.repeat(900)}`); }
    }});

    const retried = await fixtureValue.service.submit(fixtureValue.source, {leaseOwner: 'worker_1'});
    assert.equal(retried.status, 'retry');
    assert.equal(fixtureValue.source.status, 'queued');
    assert.equal(fixtureValue.source.error.length, 500);
    assert.equal(fixtureValue.calls.at(-1)[0], 'retry');

    const terminal = job({id: 'job_terminal', attempt_count: 3, max_attempts: 3});
    fixtureValue.jobs.set(terminal.id, terminal);
    const failed = await fixtureValue.service.submit(terminal, {leaseOwner: 'worker_1'});
    assert.equal(failed.status, 'failed');
    assert.equal(terminal.status, 'failed');
    assert.equal(fixtureValue.targets.at(-1).status, 'failed');
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'observability.settle').length >= 2, true);
});

test('expired or different leases fail closed before provider execution', async () => {
    const fixtureValue = fixture();
    const stale = job({id: 'stale', lease_owner: 'worker_other'});
    fixtureValue.jobs.set(stale.id, stale);
    const result = await fixtureValue.service.submit(stale, {leaseOwner: 'worker_1'});

    assert.equal(result.status, 'stale');
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'submit'), false);
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'settle' || kind === 'retry'), false);
});

test('service stays independent of the legacy root and runtime clients', async () => {
    const source = await readFile(new URL('../server/application/media-job-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|child_process|fetch\s*\(/i);
});
