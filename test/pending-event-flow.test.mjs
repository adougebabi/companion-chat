import assert from 'node:assert/strict';
import test from 'node:test';

import {createPendingEventFlow} from '../server/application/pending-event-flow.js';

const NOW = '2026-08-20T00:00:00.000Z';

function sourceRepository(rows = {}) {
    return {
        calls: [],
        findById(input) {
            this.calls.push(input);
            return rows[input.id];
        }
    };
}

function pendingRepository() {
    const events = new Map();
    const jobs = new Map();
    return {
        events,
        jobs,
        reads: 0,
        writes: 0,
        findByDedupeKey({personaId, dedupeKey, notBefore}) {
            this.reads += 1;
            return [...events.values()].find(row => row.personaId === personaId && row.dedupeKey === dedupeKey && row.notBefore === notBefore);
        },
        insertPendingEvent(input) {
            this.writes += 1;
            const row = {...input, createdAt: input.createdAt, updatedAt: input.updatedAt};
            events.set(row.id, row);
            return row;
        },
        findLinkedJob({personaId, pendingEventId}) {
            return [...jobs.values()].find(job => job.personaId === personaId && job.payload.pendingEventId === pendingEventId);
        },
        ensureLinkedJob({personaId, pendingEventId, job}) {
            const existing = this.findLinkedJob({personaId, pendingEventId});
            if (existing) return {job: existing, created: false};
            this.writes += 1;
            jobs.set(job.id, job);
            return {job, created: true};
        }
    };
}

function command(overrides = {}) {
    return {
        personaId: 'persona_1',
        sourceMessageId: 'message_1',
        sourceMessage: {id: 'message_1', personaId: 'persona_1', role: 'user'},
        call: {
            schemaVersion: 1,
            summary: '面试结束后跟进',
            notBefore: '2026-08-20T00:05:00.000Z',
            expiresAt: '2026-08-20T02:00:00.000Z',
            dedupeKey: 'interview_followup'
        },
        provenance: {source: 'native', callId: 'call_1', idempotencyKey: 'cap_1'},
        ...overrides
    };
}

function flow({pending = pendingRepository(), source = sourceRepository({'message_1': command().sourceMessage}), normalizeCall} = {}) {
    let id = 0;
    const repositories = {
        pendingEventRepository: pending,
        sourceMessageRepository: source,
        jobRepository: {enqueue(input) { return input; }},
        lifeEventRepository: {findById() { return null; }}
    };
    return {pending, source, flow: createPendingEventFlow({repositories, clock: () => NOW, idGenerator: prefix => `${prefix}_${++id}`, normalizeCall})};
}

test('plan validates, scopes the source message, and preallocates without writes', () => {
    const setup = flow();
    const plan = setup.flow.plan(command());
    assert.equal(plan.type, 'pending_event_plan');
    assert.equal(plan.pendingEventId, 'pending_event_1');
    assert.equal(plan.jobId, 'job_2');
    assert.equal(plan.previewResult.pendingEvent.id, 'pending_event_1');
    assert.equal(plan.previewResult.job.id, 'job_2');
    assert.equal(setup.pending.writes, 0);
    assert.equal(Object.isFrozen(plan), true);
});

test('apply creates one pending row and durable job through the caller transaction', () => {
    const setup = flow();
    const plan = setup.flow.plan(command());
    let transactionCalls = 0;
    const result = setup.flow.apply(plan, {transaction(work) { transactionCalls += 1; return work(); }});
    assert.equal(transactionCalls, 1);
    assert.equal(result.created, true);
    assert.equal(result.pendingEvent.id, plan.pendingEventId);
    assert.equal(result.jobId, plan.jobId);
    assert.equal(setup.pending.events.size, 1);
    assert.equal(setup.pending.jobs.size, 1);
});

test('replay is idempotent and repair creates a replacement job without a second pending row', () => {
    const setup = flow();
    const firstPlan = setup.flow.plan(command());
    const first = setup.flow.apply(firstPlan);
    assert.equal(first.created, true);

    const replayPlan = setup.flow.plan(command());
    assert.equal(replayPlan.pendingEventId, first.pendingEvent.id);
    assert.equal(replayPlan.jobId, first.jobId);
    const replay = setup.flow.apply(replayPlan);
    assert.equal(replay.created, false);
    assert.equal(setup.pending.events.size, 1);
    assert.equal(setup.pending.jobs.size, 1);

    setup.pending.jobs.delete(first.jobId);
    const repairPlan = setup.flow.plan(command());
    assert.equal(repairPlan.pendingEventId, first.pendingEvent.id);
    assert.notEqual(repairPlan.jobId, first.jobId);
    const repaired = setup.flow.apply(repairPlan);
    assert.equal(repaired.created, true);
    assert.equal(setup.pending.events.size, 1);
    assert.equal(setup.pending.jobs.size, 1);
});

test('source persona and role mismatches fail before any write', () => {
    const wrongPersona = flow({source: sourceRepository({'message_1': {id: 'message_1', personaId: 'persona_2', role: 'user'}})});
    assert.throws(() => wrongPersona.flow.plan(command({sourceMessage: undefined})), /does not belong/);
    assert.equal(wrongPersona.pending.writes, 0);

    const assistant = flow({source: sourceRepository({'message_1': {id: 'message_1', personaId: 'persona_1', role: 'assistant'}})});
    assert.throws(() => assistant.flow.plan(command({sourceMessage: undefined})), /user-owned/);
    assert.equal(assistant.pending.writes, 0);
});

test('injected native normalizer is used but its output remains strictly bounded', () => {
    const calls = [];
    const setup = flow({normalizeCall(value, reference) {
        calls.push([value, reference]);
        return {...value, summary: value.summary.trim()};
    }});
    const plan = setup.flow.plan(command({call: {...command().call, summary: '  injected  '}}));
    assert.equal(plan.call.summary, 'injected');
    assert.equal(calls.length, 1);
    assert.equal(calls[0][1], Date.parse(NOW));
    assert.throws(() => setup.flow.plan(command({call: {...command().call, expiresAt: '2026-08-20T00:00:00'}})), /timezone|offset/i);
});
