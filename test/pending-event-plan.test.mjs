import assert from 'node:assert/strict';
import test from 'node:test';
import {createCompanionTestContext} from '../server/testing/companion-context.js';
import {createPendingEventFlow} from '../server/application/pending-event-flow.js';

let idSequence = 0;
const context = createCompanionTestContext({
    clock: () => '2026-08-21T00:00:00.000Z',
    idGenerator: prefix => `${prefix}_pending_test_${++idSequence}`
});
const {database, repositories} = context;
const pendingFlow = createPendingEventFlow({
    repositories: {...repositories, sourceMessage: repositories.conversation},
    clock: context.clock,
    idGenerator: context.id,
    transaction: work => database.transaction(work)()
});

function createPersona(name) {
    const id = `persona_${name}`;
    database.prepare(`INSERT INTO companion_personas (id, name, role, color, enabled, created_at, updated_at) VALUES (?, ?, '陪伴者', '#888888', 1, ?, ?)`)
        .run(id, name, context.clock(), context.clock());
    return {id, name};
}

function appendMessage(personaId, text) {
    const conversation = repositories.conversation.getOrCreateConversation({personaId, id: `conversation_${personaId}`, createdAt: context.clock()});
    return repositories.conversation.appendMessage({
        id: `message_${personaId}`,
        conversationId: conversation.id,
        role: 'user',
        text,
        attachmentsJson: '[]',
        generationJson: null,
        jobsJson: '[]',
        proactiveEventId: null,
        proactivePendingEventId: null,
        createdAt: context.clock(),
        readAt: context.clock()
    });
}

function deletePersona(personaId) {
    database.prepare('DELETE FROM companion_jobs WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_messages WHERE conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)').run(personaId);
    database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_pending_events WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
}

function pendingCall(suffix = 'followup') {
    const base = Date.parse(context.clock());
    const notBefore = new Date(base + 90_000).toISOString();
    const expiresAt = new Date(base + 2 * 60 * 60_000).toISOString();
    return {schemaVersion: 1, summary: '面试结束后跟进', notBefore, expiresAt, dedupeKey: `${suffix}-${base}`};
}

function transactionApply(plan) {
    return pendingFlow.apply(plan);
}

function pendingCounts(personaId) {
    return {
        events: database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(personaId).count,
        jobs: database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'pending_event'").get(personaId).count
    };
}

function setup(name = '待定事件计划测试') {
    const persona = createPersona(name);
    const source = appendMessage(persona.id, '这件事稍后再聊。');
    return {persona, source};
}

test('planning validates and preallocates without writing rows or invoking a provider', () => {
    const {persona, source} = setup();
    try {
        const before = pendingCounts(persona.id);
        const plan = pendingFlow.plan(persona.id, pendingCall(), source.id, {source: 'native', callId: 'call_preview'});
        assert.equal(plan.type, 'pending_event_plan');
        assert.equal(plan.pendingEventId, plan.preallocatedIds.pendingEventId);
        assert.equal(plan.jobId, plan.preallocatedIds.jobId);
        assert.equal(plan.previewResult.pendingEvent.id, plan.pendingEventId);
        assert.equal(plan.previewResult.job.id, plan.jobId);
        assert.equal(plan.previewResult.pendingEvent.summary, '面试结束后跟进');
        assert.deepEqual(pendingCounts(persona.id), before);
    } finally {
        deletePersona(persona.id);
    }
});

test('apply commits the plan, replays idempotently, and uses the planned job ID', () => {
    const {persona, source} = setup('待定事件回放测试');
    try {
        const call = pendingCall('replay');
        const plan = pendingFlow.plan(persona.id, call, source.id, {source: 'native', idempotencyKey: 'cap_replay'});
        const first = transactionApply(plan);
        assert.equal(first.created, true);
        assert.equal(first.pendingEvent.id, plan.pendingEventId);
        assert.equal(first.jobId, plan.jobId);
        assert.equal(database.prepare('SELECT id FROM companion_jobs WHERE id = ?').get(plan.jobId).id, plan.jobId);

        const replayPlan = pendingFlow.plan(persona.id, call, source.id, {source: 'native', idempotencyKey: 'cap_replay'});
        assert.equal(replayPlan.pendingEventId, plan.pendingEventId);
        assert.equal(replayPlan.jobId, plan.jobId);
        assert.equal(replayPlan.previewResult.replayed, true);
        const replay = transactionApply(replayPlan);
        assert.equal(replay.created, false);
        assert.deepEqual(pendingCounts(persona.id), {events: 1, jobs: 1});
    } finally {
        deletePersona(persona.id);
    }
});

test('apply repairs a pending event whose linked job was removed', () => {
    const {persona, source} = setup('待定事件修复测试');
    try {
        const call = pendingCall('repair');
        const firstPlan = pendingFlow.plan(persona.id, call, source.id);
        const first = transactionApply(firstPlan);
        database.prepare('DELETE FROM companion_jobs WHERE id = ?').run(first.jobId);
        const repairPlan = pendingFlow.plan(persona.id, call, source.id);
        assert.equal(repairPlan.pendingEventId, first.pendingEvent.id);
        assert.notEqual(repairPlan.jobId, first.jobId);
        const repaired = transactionApply(repairPlan);
        assert.equal(repaired.created, true);
        assert.equal(repaired.jobId, repairPlan.jobId);
        assert.deepEqual(pendingCounts(persona.id), {events: 1, jobs: 1});
    } finally {
        deletePersona(persona.id);
    }
});

test('caller transaction rolls back the pending row when the planned job ID cannot be inserted', () => {
    const {persona, source} = setup('待定事件回滚测试');
    try {
        const plan = pendingFlow.plan(persona.id, pendingCall('rollback'), source.id);
        const createdAt = new Date().toISOString();
        database.prepare(`
            INSERT INTO companion_jobs (id, job_type, status, run_after, payload_json, created_at, updated_at)
            VALUES (?, 'other_job', 'queued', ?, '{}', ?, ?)
        `).run(plan.jobId, createdAt, createdAt, createdAt);
        assert.throws(() => transactionApply(plan), /UNIQUE|constraint/i);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE id = ?').get(plan.pendingEventId).count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE id = ?').get(plan.jobId).count, 1);
    } finally {
        deletePersona(persona.id);
    }
});

test.after(() => context.cleanup());
