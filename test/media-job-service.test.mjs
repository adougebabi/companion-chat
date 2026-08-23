import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    MEDIA_POLL_JOB_TYPES,
    MEDIA_SUBMIT_JOB_TYPES,
    MEDIA_COMPENSATION_JOB_TYPES,
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
        findByPayload(input) {
            return [...jobs.values()].find(candidate => {
                if (candidate.jobType !== input.jobType || candidate.personaId !== input.personaId) return false;
                const value = candidate.payload?.idempotencyKey
                    ?? candidate.payload?.capabilityCall?.idempotencyKey;
                return value === input.value;
            }) || null;
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
        targetStatus() {
            return 'processing';
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
            },
            ...overrides.acceptance
        },
        clock: () => NOW
    });
    const source = job();
    jobs.set(source.id, source);
    return {service, repo, jobs, calls, targets, source, provider};
}

test('media job service exposes one registration map for submit and poll jobs', () => {
    const {service} = fixture();
    assert.deepEqual(service.list(), [...MEDIA_SUBMIT_JOB_TYPES, ...MEDIA_POLL_JOB_TYPES, ...MEDIA_COMPENSATION_JOB_TYPES]);
    assert.deepEqual(service.list().map(type => service.operationFor(type)), [
        ...MEDIA_SUBMIT_JOB_TYPES.map(() => 'submit'),
        ...MEDIA_POLL_JOB_TYPES.map(() => 'poll'),
        ...MEDIA_COMPENSATION_JOB_TYPES.map(() => 'compensate')
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

test('poll enqueue failure leaves a durable compensation that repairs one poll without resubmitting', async () => {
    let failPollEnqueue = true;
    const fixtureValue = fixture({
        mediaFlow: {
            enqueuePoll({job: sourceJob, payload}) {
                if (failPollEnqueue) throw new Error('poll outbox unavailable');
                return fixtureValue.repo.enqueue({
                    jobType: 'chat_media_poll',
                    personaId: sourceJob.persona_id,
                    messageId: sourceJob.message_id,
                    payload,
                    runAfter: NOW,
                    now: NOW
                });
            }
        }
    });

    const first = await fixtureValue.service.submit(fixtureValue.source, {leaseOwner: 'worker_1'});
    assert.equal(first.status, 'complete');
    assert.equal(first.result.pollCompensationQueued, true);
    assert.equal(fixtureValue.source.status, 'complete');
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'submit').length, 1);
    assert.equal([...fixtureValue.jobs.values()].filter(item => item.jobType === 'chat_media_poll').length, 0);

    const compensation = [...fixtureValue.jobs.values()].find(item => item.jobType === 'media_poll_compensation');
    assert.ok(compensation);
    failPollEnqueue = false;
    compensation.status = 'leased';
    compensation.lease_owner = 'worker_2';
    compensation.lease_expires_at = '2026-08-21T00:01:00.000Z';
    const repaired = await fixtureValue.service.compensatePoll(compensation, {leaseOwner: 'worker_2'});

    assert.equal(repaired.status, 'complete');
    assert.equal([...fixtureValue.jobs.values()].filter(item => item.jobType === 'chat_media_poll').length, 1);
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'submit').length, 1);
});

test('terminal compensation failures retain target association and mark the target failed', async () => {
    const fixtureValue = fixture();
    fixtureValue.source.status = 'complete';
    fixtureValue.source.result_json = JSON.stringify({provider: 'fixture'});
    const compensation = {
        id: 'compensation_missing_locator',
        job_type: 'media_poll_compensation',
        status: 'leased',
        lease_owner: 'worker_2',
        lease_expires_at: '2026-08-21T00:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: fixtureValue.source.persona_id,
        message_id: fixtureValue.source.message_id,
        payload: {sourceJobId: fixtureValue.source.id, provider: 'fixture', kind: 'image'}
    };
    fixtureValue.jobs.set(compensation.id, compensation);

    const result = await fixtureValue.service.compensatePoll(compensation, {leaseOwner: 'worker_2'});

    assert.equal(result.status, 'failed');
    assert.equal(compensation.status, 'failed');
    assert.equal(fixtureValue.targets.at(-1).status, 'failed');
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'poll' || kind === 'submit'), false);
});

test('activity compensation failures use the source activity association', async () => {
    const fixtureValue = fixture();
    const source = job({
        id: 'activity_source_1',
        job_type: 'activity_image',
        activity_id: 'activity_1',
        result_json: JSON.stringify({provider: 'fixture'})
    });
    source.status = 'complete';
    fixtureValue.jobs.set(source.id, source);
    const compensation = {
        id: 'activity_compensation_1',
        job_type: 'media_poll_compensation',
        status: 'leased',
        lease_owner: 'worker_2',
        lease_expires_at: '2026-08-21T00:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: source.persona_id,
        activity_id: source.activity_id,
        payload: {
            sourceJobId: source.id,
            sourceJobType: source.job_type,
            provider: 'fixture',
            externalId: 'external_1',
            kind: 'image'
        }
    };
    fixtureValue.jobs.set(compensation.id, compensation);

    const result = await fixtureValue.service.compensatePoll(compensation, {leaseOwner: 'worker_2'});

    assert.equal(result.status, 'failed');
    assert.equal(fixtureValue.targets.at(-1).job.activity_id, 'activity_1');
    assert.equal(fixtureValue.targets.at(-1).status, 'failed');
});

test('quality retry successor uses one deterministic key across repeated poll completion', async () => {
    const fixtureValue = fixture({acceptance: {
        accept() { return {verdict: 'retry', violations: ['scene'], retryGuidance: 'adjust scene'}; }
    }});
    const firstPoll = job({
        id: 'poll_quality_1',
        job_type: 'chat_media_poll',
        payload_json: JSON.stringify({kind: 'image', provider: 'fixture', externalId: 'external_1', sourceJobId: 'job_media_1'})
    });
    fixtureValue.jobs.set(firstPoll.id, firstPoll);
    const first = await fixtureValue.service.poll(firstPoll, {leaseOwner: 'worker_1'});
    assert.equal(first.status, 'complete');

    const secondPoll = job({
        id: 'poll_quality_2',
        job_type: 'chat_media_poll',
        payload_json: JSON.stringify({kind: 'image', provider: 'fixture', externalId: 'external_1', sourceJobId: 'job_media_1'})
    });
    fixtureValue.jobs.set(secondPoll.id, secondPoll);
    const second = await fixtureValue.service.poll(secondPoll, {leaseOwner: 'worker_1'});
    assert.equal(second.status, 'complete');

    const successors = [...fixtureValue.jobs.values()].filter(item => item.payload?.qualityRetryKey === 'media:quality-retry:job_media_1:1');
    assert.equal(successors.length, 1);
    assert.equal(successors[0].payload.idempotencyKey, 'media:quality-retry:job_media_1:1');
});

test('dispatcher-owned settlement defers the media repository transition', async () => {
    const fixtureValue = fixture();
    const result = await fixtureValue.service.handlers.chat_image(fixtureValue.source, {
        leaseOwner: 'worker_1',
        deferSettlement: true
    });

    assert.equal(result.status, 'complete');
    assert.equal(result.settlement.deferred, true);
    assert.equal(fixtureValue.source.status, 'leased');
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'observability.settle' || kind === 'settle').length, 0);
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

test('poll does not settle complete when the message target projection fails', async () => {
    const fixtureValue = fixture({mediaFlow: {
        updateTarget(input) {
            fixtureValue.targets.push(input);
            return {changed: false, reason: 'message_not_found'};
        }
    }});
    const poll = job({
        id: 'poll_missing_target',
        job_type: 'chat_media_poll',
        payload_json: JSON.stringify({kind: 'image', provider: 'fixture', externalId: 'external_1', sourceJobId: 'job_media_1'})
    });
    fixtureValue.jobs.set(poll.id, poll);

    const result = await fixtureValue.service.handlers.chat_media_poll(poll, {leaseOwner: 'worker_1'});
    assert.equal(result.status, 'retry');
    assert.equal(poll.status, 'queued');
    assert.equal(fixtureValue.targets.at(-1).status, 'ready');
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
