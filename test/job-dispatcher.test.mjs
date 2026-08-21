import assert from 'node:assert/strict';
import test from 'node:test';

import {createJobDispatcher, MAX_JOB_ERROR_LENGTH} from '../server/runtime/job-dispatcher.js';

const NOW = '2026-08-20T00:00:00.000Z';

function fakeRepository(initial = {}) {
    const jobs = new Map(Object.entries(initial));
    const calls = [];
    return {
        jobs,
        calls,
        claim(input) {
            calls.push(['claim', input]);
            return jobs.values().next().value ?? null;
        },
        findLeased(input) {
            calls.push(['findLeased', input]);
            const job = jobs.get(input.id);
            return job && job.status === 'leased' && job.lease_owner === input.leaseOwner && Date.parse(job.lease_expires_at) > Date.parse(input.now) ? job : null;
        },
        settle(input) {
            calls.push(['settle', input]);
            const job = jobs.get(input.id);
            if (!job || job.status !== 'leased' || job.lease_owner !== input.leaseOwner || Date.parse(job.lease_expires_at) <= Date.parse(input.now)) return {changed: false, status: null, job: null};
            job.status = input.status;
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.result = input.result ?? null;
            job.error = input.error ?? null;
            return {changed: true, status: input.status, job};
        },
        retry(input) {
            calls.push(['retry', input]);
            const job = jobs.get(input.id);
            if (!job || job.status !== 'leased' || job.lease_owner !== input.leaseOwner || Date.parse(job.lease_expires_at) <= Date.parse(input.now)) return {changed: false, status: null, job: null};
            job.status = 'queued';
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.run_after = input.runAfter;
            job.error = input.error ?? null;
            return {changed: true, status: 'queued', job};
        }
    };
}

function claimed(overrides = {}) {
    return {
        id: 'job_1', job_type: 'demo', status: 'leased', lease_owner: 'worker_1',
        lease_expires_at: '2026-08-20T00:01:00.000Z', attempt_count: 1, max_attempts: 3,
        ...overrides
    };
}

function context(overrides = {}) {
    return {leaseOwner: 'worker_1', leaseMs: 60_000, now: NOW, signal: new AbortController().signal, ...overrides};
}

test('registration is idempotent and preserves the handler receiver', async () => {
    const repository = fakeRepository({'job_1': claimed()});
    const receiver = {calls: 0, handle(job) { this.calls += 1; return {value: job.id}; }};
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    assert.strictEqual(dispatcher.register('demo', receiver.handle, receiver), dispatcher);
    assert.strictEqual(dispatcher.register('demo', receiver.handle, receiver), dispatcher);
    assert.deepEqual(dispatcher.list(), ['demo']);
    const result = await dispatcher.runJob(claimed(), context());
    assert.equal(result.status, 'complete');
    assert.equal(receiver.calls, 1);
    assert.deepEqual(result.result, {value: 'job_1'});
    assert.throws(() => dispatcher.register('demo', () => {}), /already registered/);
});

test('descriptor registrations retain the descriptor receiver', async () => {
    const repository = fakeRepository({'job_1': claimed()});
    const descriptor = {calls: 0, run() { this.calls += 1; }};
    const dispatcher = createJobDispatcher({repository, handlers: {demo: descriptor}, clock: () => NOW});
    await dispatcher.runJob(claimed(), context());
    assert.equal(descriptor.calls, 1);
});

test('expired leases and stale owners fail closed without invoking a handler or settlement', async () => {
    const repository = fakeRepository({'job_1': claimed()});
    let handled = 0;
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    dispatcher.register('demo', () => { handled += 1; });

    const expired = await dispatcher.runJob(claimed({lease_expires_at: NOW}), context());
    assert.deepEqual(expired, {status: 'stale', reason: 'expired_lease', changed: false, job: claimed({lease_expires_at: NOW}), settlement: null});
    const stale = await dispatcher.runJob(claimed(), context({leaseOwner: 'worker_other'}));
    assert.equal(stale.reason, 'stale_owner');
    assert.equal(handled, 0);
    assert.equal(repository.calls.filter(([kind]) => kind === 'settle' || kind === 'retry').length, 0);
});

test('handler failure retries with a bounded error, then terminally settles at max attempts', async () => {
    const retryRepository = fakeRepository({'job_1': claimed()});
    const dispatcher = createJobDispatcher({repository: retryRepository, clock: () => NOW, retryDelayMs: 100});
    dispatcher.register('demo', () => { throw new Error(`failure ${'x'.repeat(MAX_JOB_ERROR_LENGTH + 40)}`); });
    const retried = await dispatcher.runJob(claimed(), context());
    assert.equal(retried.status, 'retry');
    assert.equal(retried.error.length, MAX_JOB_ERROR_LENGTH);
    assert.equal(retryRepository.calls.at(-1)[0], 'retry');
    assert.equal(retryRepository.calls.at(-1)[1].runAfter, '2026-08-20T00:00:00.100Z');

    const terminalRepository = fakeRepository({'job_1': claimed({attempt_count: 3, max_attempts: 3})});
    const terminalDispatcher = createJobDispatcher({repository: terminalRepository, clock: () => NOW});
    terminalDispatcher.register('demo', () => { throw Object.assign(new Error('permanent'), {retryable: false}); });
    const terminal = await terminalDispatcher.runJob(claimed({attempt_count: 3, max_attempts: 3}), context());
    assert.equal(terminal.status, 'failed');
    assert.equal(terminalRepository.calls.at(-1)[0], 'settle');
    assert.equal(terminalRepository.calls.at(-1)[1].status, 'failed');
});

test('unknown job types fail closed through terminal settlement', async () => {
    const repository = fakeRepository({'job_1': claimed({job_type: 'unregistered'})});
    const terminal = [];
    const dispatcher = createJobDispatcher({repository, clock: () => NOW, onTerminal: result => terminal.push(result)});
    const result = await dispatcher.runJob(claimed({job_type: 'unregistered'}), context());
    assert.equal(result.status, 'failed');
    assert.match(result.error, /Unknown job type/);
    assert.equal(repository.calls.at(-1)[0], 'settle');
    assert.equal(repository.calls.at(-1)[1].status, 'failed');
    assert.equal(terminal.length, 1);
});

test('jobTick is a worker-runtime adapter that claims and dispatches one durable job', async () => {
    const repository = fakeRepository({'job_1': claimed()});
    const seen = [];
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    dispatcher.register('demo', (job, receivedContext) => {
        seen.push([job.id, receivedContext.leaseOwner, receivedContext.leaseMs, receivedContext.signal.aborted]);
        return {ok: true};
    });
    const result = await dispatcher.jobTick(context());
    assert.equal(result.status, 'complete');
    assert.deepEqual(seen, [['job_1', 'worker_1', 60_000, false]]);
    assert.equal(repository.calls[0][0], 'claim');
});

test('runJob also accepts a claimed envelope with lease metadata', async () => {
    const repository = fakeRepository({'job_1': claimed()});
    const seen = [];
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    dispatcher.register('demo', (_job, receivedContext) => {
        seen.push([receivedContext.leaseOwner, receivedContext.leaseMs, receivedContext.signal.aborted]);
    });
    const result = await dispatcher.runJob({job: claimed(), ...context()});
    assert.equal(result.status, 'complete');
    assert.deepEqual(seen, [['worker_1', 60_000, false]]);
});
