import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createPendingEventRepository} from '../server/infrastructure/pending-event-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_pending_events (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL,
            source_message_id TEXT,
            status TEXT NOT NULL,
            summary TEXT NOT NULL,
            not_before TEXT NOT NULL,
            expires_at TEXT NOT NULL,
            dedupe_key TEXT NOT NULL,
            payload_json TEXT NOT NULL DEFAULT '{}',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            triggered_at TEXT,
            consumed_at TEXT,
            cancelled_at TEXT,
            UNIQUE(persona_id, dedupe_key, not_before)
        );
        CREATE TABLE companion_jobs (
            id TEXT PRIMARY KEY,
            job_type TEXT NOT NULL,
            persona_id TEXT,
            payload_json TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
    `);
    return database;
}

function enqueueJob(database, calls) {
    return input => {
        calls.push(input);
        const job = {
            id: `job_${calls.length}`,
            jobType: input.jobType,
            personaId: input.personaId,
            payload: input.payload,
            createdAt: `2026-08-20T00:00:0${calls.length}.000Z`
        };
        database.prepare('INSERT INTO companion_jobs (id, job_type, persona_id, payload_json, created_at) VALUES (?, ?, ?, ?, ?)')
            .run(job.id, job.jobType, job.personaId, JSON.stringify(job.payload), job.createdAt);
        return job;
    };
}

function pendingInput(overrides = {}) {
    return {
        id: 'pending_1',
        personaId: 'persona_1',
        sourceMessageId: 'message_1',
        status: 'pending',
        summary: '面试结束后跟进',
        notBefore: '2026-08-20T10:00:00.000Z',
        expiresAt: '2026-08-20T12:00:00.000Z',
        dedupeKey: 'interview_followup',
        payload: {schemaVersion: 1, source: 'native'},
        createdAt: '2026-08-20T00:00:00.000Z',
        updatedAt: '2026-08-20T00:00:00.000Z',
        ...overrides
    };
}

test('pending repository requires an existing database and injected enqueue function', () => {
    assert.throws(() => createPendingEventRepository(), /open database/);

    const database = createDatabase();
    try {
        assert.throws(() => createPendingEventRepository({database}), /enqueueJob function/);
    } finally {
        database.close();
    }
});

test('pending repository inserts and finds rows by persona, dedupe key, and not-before', () => {
    const database = createDatabase();
    try {
        const repository = createPendingEventRepository({database, enqueueJob: () => { throw new Error('unexpected enqueue'); }});
        assert.equal(repository.findByDedupeKey({personaId: 'persona_1', dedupeKey: 'interview_followup', notBefore: pendingInput().notBefore}), undefined);

        const first = repository.insertPendingEvent(pendingInput());
        assert.equal(first.id, 'pending_1');
        assert.deepEqual(JSON.parse(first.payload_json), {schemaVersion: 1, source: 'native'});
        assert.equal(repository.findByDedupeKey({personaId: 'persona_1', dedupeKey: 'interview_followup', notBefore: first.not_before}).id, first.id);

        repository.insertPendingEvent(pendingInput({id: 'pending_2', dedupeKey: 'different_followup'}));
        repository.insertPendingEvent(pendingInput({id: 'pending_3', personaId: 'persona_2'}));
        assert.equal(repository.findByDedupeKey({personaId: 'persona_1', dedupeKey: 'different_followup', notBefore: first.not_before}).id, 'pending_2');
        assert.equal(repository.findByDedupeKey({personaId: 'persona_2', dedupeKey: 'interview_followup', notBefore: first.not_before}).id, 'pending_3');
        assert.equal(repository.findByDedupeKey({personaId: 'persona_1', dedupeKey: 'interview_followup', notBefore: first.not_before}).id, 'pending_1');
    } finally {
        database.close();
    }
});

test('pending repository finds linked jobs and repairs a missing link idempotently', () => {
    const database = createDatabase();
    const calls = [];
    try {
        const repository = createPendingEventRepository({database, enqueueJob: enqueueJob(database, calls)});
        repository.insertPendingEvent(pendingInput());
        assert.equal(repository.findLinkedJob({personaId: 'persona_1', pendingEventId: 'pending_1'}), undefined);

        const jobInput = {
            jobType: 'pending_event',
            personaId: 'persona_1',
            runAfter: '2026-08-20T10:00:00.000Z',
            payload: {pendingEventId: 'pending_1', idempotencyKey: 'cap_test'}
        };
        const first = repository.ensureLinkedJob({personaId: 'persona_1', pendingEventId: 'pending_1', job: jobInput});
        assert.equal(first.created, true);
        assert.equal(first.job.id, 'job_1');
        assert.equal(repository.findLinkedJob({personaId: 'persona_1', pendingEventId: 'pending_1'}).id, 'job_1');

        const second = repository.ensureLinkedJob({personaId: 'persona_1', pendingEventId: 'pending_1', job: jobInput});
        assert.equal(second.created, false);
        assert.equal(second.job.id, 'job_1');
        assert.equal(calls.length, 1);

        assert.equal(repository.findLinkedJob({personaId: 'persona_2', pendingEventId: 'pending_1'}), undefined);
        assert.throws(() => repository.ensureLinkedJob({personaId: 'persona_2', pendingEventId: 'pending_1', job: jobInput}), /does not belong to persona/);
        assert.equal(calls.length, 1);
    } finally {
        database.close();
    }
});

test('caller transaction rolls back the pending row when linked-job enqueue fails', () => {
    const database = createDatabase();
    try {
        const repository = createPendingEventRepository({database, enqueueJob: () => { throw new Error('queue unavailable'); }});
        assert.throws(() => database.transaction(() => {
            repository.insertPendingEvent(pendingInput());
            repository.ensureLinkedJob({
                personaId: 'persona_1',
                pendingEventId: 'pending_1',
                job: {jobType: 'pending_event', personaId: 'persona_1', payload: {pendingEventId: 'pending_1'}}
            });
        })(), /queue unavailable/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events').get().count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, 0);
    } finally {
        database.close();
    }
});
