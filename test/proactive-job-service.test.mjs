import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {createJobDispatcher} from '../server/runtime/job-dispatcher.js';
import {
    PROACTIVE_JOB_TYPES,
    createProactiveJobService
} from '../server/application/proactive-job-service.js';

const NOW = '2026-08-21T00:00:00.000Z';

function leasedJob(type, overrides = {}) {
    return {
        id: `job_${type}`,
        job_type: type,
        status: 'leased',
        lease_owner: 'worker_1',
        lease_expires_at: '2026-08-21T00:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: 'persona_1',
        payload_json: JSON.stringify({source: 'fixture'}),
        ...overrides
    };
}

function repository(job) {
    const calls = [];
    return {
        calls,
        findLeased(input) {
            calls.push(['findLeased', input]);
            return job.status === 'leased' && job.lease_owner === input.leaseOwner ? job : null;
        },
        settle(input) {
            calls.push(['settle', input]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false};
            job.status = input.status;
            job.lease_owner = null;
            job.lease_expires_at = null;
            return {changed: true, status: input.status, job};
        },
        retry(input) {
            calls.push(['retry', input]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false};
            job.status = 'queued';
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.run_after = input.runAfter;
            return {changed: true, status: 'queued', job};
        }
    };
}

function context() {
    return {
        leaseOwner: 'worker_1',
        leaseMs: 60_000,
        now: NOW,
        signal: new AbortController().signal,
        correlationId: 'corr_1'
    };
}

test('the service is an inert registration seam with an explicit parity blocker list', () => {
    const service = createProactiveJobService({
        repositories: {
            conversation: {findMessage() {}},
            activity: {publish() {}},
            lifeEvent: {findById() {}},
            job: {settle() { throw new Error('must not be exposed'); }}
        },
        lifeWorld: {read() {}}
    });

    assert.deepEqual(service.list(), PROACTIVE_JOB_TYPES);
    assert.deepEqual(service.audit().registeredTypes, PROACTIVE_JOB_TYPES);
    assert.equal(service.audit().ready, false);
    assert.deepEqual(service.audit().availableTypes, []);
    assert.deepEqual(service.audit().blockers.map(blocker => blocker.type), PROACTIVE_JOB_TYPES);
    assert.match(service.audit().blockers[0].reason, /server\.js/);
    assert.equal(service.ports.repositories.job, undefined);
    assert.equal(service.ports.repositories.conversation.findMessage instanceof Function, true);
});

test('registered handlers adapt all four job types to application flows without exposing job settlement', async () => {
    const seen = [];
    const lifeWorld = {read() { return {source: 'life-world'}; }};
    const flows = Object.fromEntries(PROACTIVE_JOB_TYPES.map(type => [type, async command => {
        seen.push({type, command});
        return {type, payload: command.payload};
    }]));
    const service = createProactiveJobService({
        flows,
        repositories: {
            conversation: {findMessage() {}},
            activity: {publish() {}},
            lifeEvent: {findById() {}},
            pending: {findById() {}},
            effectRepository: {settle() { throw new Error('flow must not settle jobs'); }}
        },
        lifeWorld
    });

    for (const type of PROACTIVE_JOB_TYPES) {
        const job = leasedJob(type, {payload_json: JSON.stringify({source: type, causationId: `cause_${type}`})});
        const result = await service.run(type, job, context());
        assert.equal(result.status, 'complete');
        assert.deepEqual(result.result, {type, payload: {source: type, causationId: `cause_${type}`}});
    }

    assert.equal(seen.length, PROACTIVE_JOB_TYPES.length);
    for (const item of seen) {
        assert.equal(item.command.personaId, 'persona_1');
        assert.equal(item.command.correlationId, 'corr_1');
        assert.equal(item.command.ports.lifeWorld, lifeWorld);
        assert.equal(item.command.ports.repositories.effectRepository, undefined);
        assert.equal(item.command.ports.repositories.conversation.findMessage instanceof Function, true);
    }
    assert.equal(service.audit().ready, true);
    assert.deepEqual(service.audit().blockers, []);
});

test('registration dispatches through the generic lease/retry/settlement owner exactly once', async () => {
    const job = leasedJob('pending_event');
    const repo = repository(job);
    const calls = [];
    const service = createProactiveJobService({
        flows: {
            pending_event(command) {
                calls.push(command);
                return {pendingEventId: command.payload.pendingEventId};
            }
        }
    });
    const dispatcher = createJobDispatcher({repository: repo, clock: () => NOW});
    assert.strictEqual(service.register(dispatcher), dispatcher);

    const result = await dispatcher.runJob(job, context());
    assert.equal(result.status, 'complete');
    assert.deepEqual(result.result, {pendingEventId: undefined});
    assert.equal(calls.length, 1);
    assert.equal(repo.calls.filter(([kind]) => kind === 'settle').length, 1);
    assert.equal(repo.calls.filter(([kind]) => kind === 'retry').length, 0);
});

test('flow retry outcome is handed to generic dispatcher rather than settling inside the adapter', async () => {
    const job = leasedJob('activity_decision');
    const repo = repository(job);
    const service = createProactiveJobService({
        flows: {
            activity_decision() {
                return {status: 'retry', error: 'model temporarily unavailable', retryAt: '2026-08-21T00:00:05.000Z'};
            }
        }
    });
    const dispatcher = createJobDispatcher({repository: repo, clock: () => NOW});
    service.register(dispatcher);

    const result = await dispatcher.runJob(job, context());
    assert.equal(result.status, 'retry');
    assert.equal(repo.calls.filter(([kind]) => kind === 'retry').length, 1);
    assert.equal(repo.calls.filter(([kind]) => kind === 'settle').length, 0);
    assert.equal(repo.calls.at(-1)[1].runAfter, '2026-08-21T00:00:05.000Z');
});

test('missing flow and malformed payload fail closed as terminal blockers', async () => {
    const missing = createProactiveJobService();
    await assert.rejects(
        () => missing.run('deferred_chat_reply', leasedJob('deferred_chat_reply'), context()),
        error => error.code === 'missing_deferred_chat_reply_flow' && error.retryable === false
    );

    const malformedJob = leasedJob('pending_event', {payload_json: '{not-json'});
    const calls = [];
    const service = createProactiveJobService({flows: {pending_event: () => { calls.push('flow'); }}});
    await assert.rejects(
        () => service.run('pending_event', malformedJob, context()),
        error => error.code === 'PROACTIVE_JOB_INPUT_INVALID' && error.retryable === false
    );
    assert.deepEqual(calls, []);
});

test('malformed payload terminally settles through generic dispatcher and never invokes the flow', async () => {
    const job = leasedJob('proactive_message', {payload_json: '[]'});
    const repo = repository(job);
    let invoked = false;
    const service = createProactiveJobService({
        flows: {proactive_message() { invoked = true; }}
    });
    const dispatcher = createJobDispatcher({repository: repo, clock: () => NOW});
    service.register(dispatcher);

    const result = await dispatcher.runJob(job, context());
    assert.equal(result.status, 'failed');
    assert.equal(invoked, false);
    assert.equal(repo.calls.filter(([kind]) => kind === 'settle').length, 1);
    assert.equal(repo.calls.at(-1)[1].status, 'failed');
});

test('application flow registry can supply the canonical hyphenated flow ids', async () => {
    const seen = [];
    const registry = {
        get(id) {
            return ['pending-event'].includes(id) ? {id} : null;
        },
        run(id, contextValue, command) {
            seen.push([id, contextValue, command.type]);
            return {id};
        }
    };
    const service = createProactiveJobService({flowRegistry: registry});
    const result = await service.run('pending_event', leasedJob('pending_event'), context());
    assert.deepEqual(result, {status: 'complete', result: {id: 'pending-event'}});
    assert.deepEqual(seen.map(item => item[0]), ['pending-event']);
    assert.equal(seen[0][2], 'pending_event');
});

test('the adapter has no direct database, provider, or legacy-entrypoint dependency', async () => {
    const source = await readFile(new URL('../server/application/proactive-job-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s+['"][^'"]*(?:better-sqlite3|server\.js|child_process|express)[^'"]*['"]/i);
    assert.doesNotMatch(source, /fetch\s*\(/i);
});
