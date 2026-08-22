import assert from 'node:assert/strict';
import test from 'node:test';

import {createMediaObservabilityApplication} from '../server/application/media-job-composition.js';
import {createRuntime} from '../server/runtime/runtime.js';

const NOW = '2026-08-21T00:00:00.000Z';

function makePorts() {
    const jobs = new Map();
    const calls = [];
    const repository = {
        findLeased(input) {
            const job = jobs.get(input.id);
            if (!job || job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return null;
            return job;
        },
        patchResult(job, input) {
            calls.push(['patch', job.id, input.patch]);
            const current = JSON.parse(job.result_json || '{}');
            job.result_json = JSON.stringify({...current, ...input.patch});
            return {changed: true, result: JSON.parse(job.result_json), job};
        },
        settle(input) {
            calls.push(['settle', input.status]);
            const job = jobs.get(input.id);
            if (!job || job.status !== 'leased' || job.lease_owner !== input.leaseOwner) {
                return {changed: false, status: null, job: null};
            }
            job.status = input.status;
            job.lease_owner = null;
            return {changed: true, status: input.status, job};
        },
        retry(input) {
            calls.push(['retry', input.status]);
            const job = jobs.get(input.id);
            if (!job || job.status !== 'leased' || job.lease_owner !== input.leaseOwner) {
                return {changed: false, status: null, job: null};
            }
            job.status = 'queued';
            job.lease_owner = null;
            return {changed: true, status: 'retry', job};
        },
        find(input) {
            return jobs.get(input.id) || null;
        },
        enqueue() {
            throw new Error('poll enqueue is not part of this fixture');
        }
    };
    const observabilityApplication = createMediaObservabilityApplication({
        progressParser(value) {
            const output = String(value);
            const percent = Number(output.match(/(\d+(?:\.\d+)?)%/)?.[1]);
            return {output, percent: Number.isFinite(percent) ? percent : null};
        },
        progressWriter(job, patch) {
            return repository.patchResult(job, {patch: {progress: patch}});
        },
        leaseGuard(job) {
            return repository.findLeased({id: job.id, leaseOwner: job.lease_owner, now: NOW});
        },
        settleJob(job, input) {
            return input.status === 'retry' ? repository.retry(input) : repository.settle(input);
        },
        debugProjector(value) {
            return value;
        },
        runProcess() {
            return {ok: true};
        },
        now: () => NOW
    });
    const provider = {
        id: 'fixture',
        portType: 'media',
        capabilities: ['image'],
        async submit() {
            calls.push(['provider']);
            throw new Error('fixture provider failure');
        },
        async poll() {
            return {status: 'pending'};
        }
    };
    return {jobs, calls, repository, provider, observability: observabilityApplication.observability};
}

function mediaJob(id, overrides = {}) {
    return {
        id,
        job_type: 'chat_image',
        status: 'leased',
        lease_owner: 'runtime_worker',
        attempt_count: 1,
        max_attempts: 1,
        persona_id: 'persona_fixture',
        payload_json: JSON.stringify({
            kind: 'image',
            provider: 'fixture',
            envelope: {schemaVersion: 1, mediaKind: 'image'},
            personaMediaConcept: {schemaVersion: 1, mediaKind: 'image'}
        }),
        result_json: '{}',
        ...overrides
    };
}

test('application fixture keeps debug, process, lease, and settlement ports injected', () => {
    const ports = makePorts();
    const projected = ports.observability.observeDebug({secret: 'bounded'});
    assert.deepEqual(projected, {secret: 'bounded'});
    assert.deepEqual(ports.observability.runProcess('fixture', []), {ok: true});
});

test('runtime composes media service from observability, repositories, and provider adapters', async () => {
    const ports = makePorts();
    const job = mediaJob('media_terminal');
    ports.jobs.set(job.id, job);
    const runtime = createRuntime({
        startupRuntime: {database: {}, close() {}},
        app: {},
        workerRuntime: false,
        environment: {DATA_DIR: '/tmp/media-job-composition-fixture'},
        repositories: {job: ports.repository},
        mediaObservability: ports.observability,
        providerAdapters: {fixture: ports.provider},
        promptMaster: {
            fill() {
                return {finalPrompt: 'fixture prompt'};
            }
        },
        acceptance: {accept() { return {verdict: 'pass'}; }}
    });
    try {
        assert.ok(runtime.mediaJobService);
        assert.strictEqual(runtime.mediaJobService.observability, ports.observability);
        assert.deepEqual(runtime.jobDispatcher.list(), ['activity_image', 'activity_video', 'chat_image', 'chat_video', 'activity_media_poll', 'chat_media_poll', 'media_poll_compensation']);

        const result = await runtime.mediaJobService.submit(job, {leaseOwner: 'runtime_worker', now: NOW});
        assert.equal(result.status, 'failed');
        assert.equal(job.status, 'failed');
        assert.deepEqual(ports.calls.map(([kind]) => kind), ['patch', 'patch', 'provider', 'settle']);

        const stale = mediaJob('media_stale', {lease_owner: 'other_worker'});
        ports.jobs.set(stale.id, stale);
        const staleResult = await runtime.mediaJobService.submit(stale, {leaseOwner: 'runtime_worker', now: NOW});
        assert.equal(staleResult.status, 'stale');
        assert.equal(ports.calls.filter(([kind]) => kind === 'provider').length, 1);
    } finally {
        await runtime.stop();
    }
});
