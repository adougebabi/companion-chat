import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createJobRepository, MAX_JOB_ERROR_LENGTH} from '../server/infrastructure/job-repository.js';

const T0 = '2026-08-20T00:00:00.000Z';
const T1 = '2026-08-20T00:01:00.000Z';
const T2 = '2026-08-20T00:02:00.000Z';
const T3 = '2026-08-20T00:03:00.000Z';
const T4 = '2026-08-20T00:04:00.000Z';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_jobs (
            id TEXT PRIMARY KEY,
            job_type TEXT NOT NULL,
            status TEXT NOT NULL,
            priority INTEGER NOT NULL DEFAULT 0,
            run_after TEXT NOT NULL,
            lease_owner TEXT,
            lease_expires_at TEXT,
            attempt_count INTEGER NOT NULL DEFAULT 0,
            max_attempts INTEGER NOT NULL DEFAULT 3,
            persona_id TEXT,
            activity_id TEXT,
            message_id TEXT,
            trace_id TEXT,
            payload_json TEXT NOT NULL,
            result_json TEXT,
            error TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            completed_at TEXT
        );
        CREATE INDEX companion_jobs_ready_idx ON companion_jobs(status, run_after, priority DESC, created_at);
        CREATE INDEX companion_jobs_lease_idx ON companion_jobs(lease_expires_at);
    `);
    return database;
}

function createRepository(database) {
    return createJobRepository({
        database,
        id: prefix => `${prefix}_generated`,
        clock: () => T0
    });
}

function enqueue(repository, overrides = {}) {
    return repository.enqueue({
        id: overrides.id || 'job_1',
        jobType: 'media_image',
        personaId: 'persona_1',
        runAfter: T0,
        createdAt: T0,
        payload: {schemaVersion: 1, request: '雨后的街道'},
        ...overrides
    });
}

test('job repository requires an already-open database and accepts injected helpers', () => {
    assert.throws(() => createJobRepository(), /open database/);
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        const row = repository.enqueue({jobType: 'test', payload: {ok: true}});
        assert.equal(row.id, 'job_generated');
        assert.equal(row.created_at, T0);
    } finally {
        database.close();
    }
});

test('enqueue persists a parameterized payload and find is persona-scoped', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        const row = enqueue(repository, {
            id: 'job_payload',
            jobType: 'pending_event',
            priority: 4,
            maxAttempts: 5,
            activityId: 'activity_1',
            messageId: 'message_1',
            traceId: 'trace_1',
            payload: {pendingEventId: 'pending_1', quote: "x'); DROP TABLE companion_jobs; --"}
        });

        assert.equal(row.status, 'queued');
        assert.equal(row.priority, 4);
        assert.equal(row.max_attempts, 5);
        assert.equal(row.attempt_count, 0);
        assert.equal(row.lease_owner, null);
        assert.deepEqual(JSON.parse(row.payload_json), {
            pendingEventId: 'pending_1',
            quote: "x'); DROP TABLE companion_jobs; --"
        });
        assert.equal(repository.find({id: row.id, personaId: 'persona_1'}).id, row.id);
        assert.equal(repository.find({id: row.id, personaId: 'persona_2'}), undefined);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, 1);
    } finally {
        database.close();
    }
});

test('claim assigns ownership, increments attempts, and rejects a wrong-owner settlement', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_claim'});

        const claimed = repository.claim({
            personaId: 'persona_1',
            leaseOwner: 'worker_a',
            leaseExpiresAt: T2,
            now: T1
        });
        assert.equal(claimed.id, 'job_claim');
        assert.equal(claimed.status, 'leased');
        assert.equal(claimed.lease_owner, 'worker_a');
        assert.equal(claimed.lease_expires_at, T2);
        assert.equal(claimed.attempt_count, 1);

        const rejected = repository.settle({
            id: 'job_claim',
            personaId: 'persona_1',
            leaseOwner: 'worker_b',
            status: 'complete',
            result: {shouldNotWrite: true},
            now: T1
        });
        assert.deepEqual(rejected, {changed: false, status: null, job: null});
        const stillLeased = repository.find({id: 'job_claim', personaId: 'persona_1'});
        assert.equal(stillLeased.status, 'leased');
        assert.equal(stillLeased.lease_owner, 'worker_a');
        assert.equal(stillLeased.result_json, null);
    } finally {
        database.close();
    }
});

test('claim accepts a caller lease policy for the selected job', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_h3', payload: {provider: 'h3'}});

        const claimed = repository.claim({
            leaseOwner: 'worker_h3',
            now: T1,
            leaseMs: job => job.id === 'job_h3' && JSON.parse(job.payload_json).provider === 'h3' ? 120_000 : 90_000
        });
        assert.equal(claimed.lease_expires_at, '2026-08-20T00:03:00.000Z');
    } finally {
        database.close();
    }
});

test('claim reclaims an expired lease while preserving persona isolation', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_expired', personaId: 'persona_1'});
        enqueue(repository, {id: 'job_other', personaId: 'persona_2'});

        const first = repository.claim({personaId: 'persona_1', leaseOwner: 'worker_a', leaseExpiresAt: T2, now: T1});
        assert.equal(first.attempt_count, 1);
        assert.equal(repository.claim({personaId: 'persona_1', leaseOwner: 'worker_c', leaseExpiresAt: T4, now: T2}), null);
        assert.equal(repository.retry({id: 'job_expired', personaId: 'persona_1', leaseOwner: 'worker_a', runAfter: T4, now: T2}).changed, false);
        assert.equal(repository.find({id: 'job_expired', personaId: 'persona_1'}).status, 'leased');

        const reclaimed = repository.claim({personaId: 'persona_1', leaseOwner: 'worker_b', leaseExpiresAt: T4, now: T3});
        assert.equal(reclaimed.id, 'job_expired');
        assert.equal(reclaimed.lease_owner, 'worker_b');
        assert.equal(reclaimed.attempt_count, 2);
        assert.equal(repository.find({id: 'job_other', personaId: 'persona_1'}), undefined);

        const other = repository.claim({personaId: 'persona_2', leaseOwner: 'worker_c', leaseExpiresAt: T4, now: T1});
        assert.equal(other.id, 'job_other');
    } finally {
        database.close();
    }
});

test('settle performs terminal completion under the active unexpired lease', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_complete'});
        const claimed = repository.claim({leaseOwner: 'worker_a', leaseExpiresAt: T2, now: T1});

        const settled = repository.settle(claimed, {
            result: {externalId: 'provider_1'},
            now: T1
        });
        assert.equal(settled.changed, true);
        assert.equal(settled.status, 'complete');
        assert.equal(settled.job.status, 'complete');
        assert.equal(settled.job.lease_owner, null);
        assert.equal(settled.job.lease_expires_at, null);
        assert.equal(settled.job.completed_at, T1);
        assert.deepEqual(JSON.parse(settled.job.result_json), {externalId: 'provider_1'});
    } finally {
        database.close();
    }
});

test('retry conditionally clears the lease and schedules the caller-provided time', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_retry'});
        repository.claim({leaseOwner: 'worker_a', leaseExpiresAt: T2, now: T1});

        const retry = repository.retry({
            id: 'job_retry',
            leaseOwner: 'worker_a',
            runAfter: T4,
            error: 'temporary provider failure',
            result: {attempt: 1},
            now: T1
        });
        assert.equal(retry.changed, true);
        assert.equal(retry.status, 'queued');
        assert.equal(retry.job.status, 'queued');
        assert.equal(retry.job.run_after, T4);
        assert.equal(retry.job.lease_owner, null);
        assert.equal(retry.job.lease_expires_at, null);
        assert.equal(retry.job.attempt_count, 1);
        assert.equal(retry.job.error, 'temporary provider failure');
        assert.equal(repository.claim({leaseOwner: 'worker_b', leaseExpiresAt: T4, now: T3}), null);
        const reclaimed = repository.claim({leaseOwner: 'worker_b', leaseExpiresAt: '2026-08-20T00:05:00.000Z', now: T4});
        assert.equal(reclaimed.attempt_count, 2);
    } finally {
        database.close();
    }
});

test('terminal failure is guarded and stores bounded diagnostics without retry policy', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        enqueue(repository, {id: 'job_failed'});
        repository.claim({leaseOwner: 'worker_a', leaseExpiresAt: T2, now: T1});

        const settled = repository.settle({
            id: 'job_failed',
            leaseOwner: 'worker_a',
            status: 'failed',
            error: 'e'.repeat(MAX_JOB_ERROR_LENGTH + 100),
            now: T1
        });
        assert.equal(settled.status, 'failed');
        assert.equal(settled.job.status, 'failed');
        assert.equal(settled.job.completed_at, null);
        assert.equal(settled.job.error.length, MAX_JOB_ERROR_LENGTH);
        assert.equal(settled.job.lease_owner, null);
    } finally {
        database.close();
    }
});

test('enqueue participates in the caller transaction and invalid payloads leave no partial row', () => {
    const database = createDatabase();
    try {
        const repository = createRepository(database);
        assert.throws(() => database.transaction(() => {
            enqueue(repository, {id: 'job_rollback'});
            throw new Error('caller rollback');
        })(), /caller rollback/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, 0);

        const cyclic = {};
        cyclic.self = cyclic;
        assert.throws(() => repository.enqueue({id: 'job_invalid_payload', jobType: 'test', payload: cyclic}), /could not be serialized/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, 0);

        assert.throws(() => repository.enqueue({id: 'job_bad_json', jobType: 'test', payloadJson: '{'}), /valid JSON/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, 0);
    } finally {
        database.close();
    }
});
