import assert from 'node:assert/strict';
import test from 'node:test';

import {createMediaFlow} from '../server/application/media-flow.js';

function clone(value) {
    return structuredClone(value);
}

function createFixture({failAfterJobs = null} = {}) {
    const state = {
        conversations: [],
        messages: [],
        jobs: [],
        nextId: 0,
        clock: '2026-08-20T12:00:00.000Z'
    };
    const persona = {id: 'persona_1', name: '媒体测试人格'};
    const ids = prefix => `${prefix}_${++state.nextId}`;
    const clock = () => state.clock;

    const personaRepository = {
        findActive(personaId) {
            return personaId === persona.id ? persona : null;
        }
    };

    const conversationRepository = {
        getOrCreateConversation({personaId, id, createdAt, updatedAt = createdAt}) {
            let conversation = state.conversations.find(item => item.personaId === personaId);
            if (!conversation) {
                conversation = {id, personaId, createdAt, updatedAt};
                state.conversations.push(conversation);
            }
            return conversation;
        },
        appendMessages({conversationId, messages, updatedAt}) {
            const rows = messages.map(message => ({
                id: message.id,
                conversation_id: conversationId,
                role: message.role,
                text: message.text,
                attachments_json: message.attachmentsJson,
                generation_json: message.generationJson,
                jobs_json: message.jobsJson,
                proactive_event_id: message.proactiveEventId,
                proactive_pending_event_id: message.proactivePendingEventId,
                created_at: message.createdAt,
                read_at: message.readAt
            }));
            state.messages.push(...rows);
            const conversation = state.conversations.find(item => item.id === conversationId);
            conversation.updatedAt = updatedAt;
            return rows;
        },
        findMessage({id, personaId}) {
            const row = state.messages.find(item => item.id === id);
            const conversation = row && state.conversations.find(item => item.id === row.conversation_id);
            return conversation?.personaId === personaId ? row : null;
        }
    };

    const jobRepository = {
        findByPayload({personaId, jobType, value}) {
            return state.jobs.find(job => job.personaId === personaId
                && job.jobType === jobType
                && job.payload.capabilityCall.idempotencyKey === value) || null;
        },
        enqueue(input) {
            if (failAfterJobs !== null && state.jobs.length >= failAfterJobs) throw new Error('forced media job failure');
            const job = {
                id: input.id,
                jobType: input.jobType,
                status: 'queued',
                personaId: input.personaId,
                messageId: input.messageId,
                payload: clone(input.payload),
                createdAt: input.createdAt,
                runAfter: input.runAfter
            };
            state.jobs.push(job);
            return job;
        }
    };

    const transaction = work => () => {
        const snapshot = clone(state);
        try {
            return work();
        } catch (error) {
            state.conversations = snapshot.conversations;
            state.messages = snapshot.messages;
            state.jobs = snapshot.jobs;
            state.nextId = snapshot.nextId;
            state.clock = snapshot.clock;
            throw error;
        }
    };

    const normalizeMediaCapabilityCall = value => {
        const normalized = {
            schemaVersion: 2,
            kind: value.kind,
            request: value.request || '',
            ...(value.count === undefined || value.count === 1 ? {} : {count: value.count}),
            personaMediaConcept: value.personaMediaConcept || {schemaVersion: 1, mediaKind: value.kind},
            currentEvent: value.currentEvent ?? null,
            temporaryAppearance: value.temporaryAppearance ?? {}
        };
        return normalized;
    };
    const mediaConceptEnvelopeFor = (resolvedPersona, input) => ({
        schemaVersion: 1,
        mediaKind: input.kind,
        count: input.count,
        request: input.request,
        trigger: input.trigger,
        personaId: resolvedPersona.id,
        event: input.event
    });

    const flow = createMediaFlow({
        repositories: {personaRepository, conversationRepository, jobRepository},
        clock,
        idGenerator: ids,
        normalizeMediaCapabilityCall,
        mediaConceptEnvelopeFor
    });

    return {state, persona, flow, transaction};
}

function command(overrides = {}) {
    return {
        personaId: 'persona_1',
        call: {
            kind: 'image',
            request: '三张测试图',
            count: 3,
            personaMediaConcept: {schemaVersion: 1, mediaKind: 'image'},
            currentEvent: null,
            temporaryAppearance: {}
        },
        provenance: {idempotencyKey: 'media_capability_1', callId: 'call_1'},
        ...overrides
    };
}

test('media plan is read-only and preallocates one placeholder/job per asset', () => {
    const fixture = createFixture();
    const plan = fixture.flow.plan(command());

    assert.equal(plan.count, 3);
    assert.deepEqual(plan.assetKeys, ['media_capability_1:0', 'media_capability_1:1', 'media_capability_1:2']);
    assert.deepEqual(plan.messageIds.length, 3);
    assert.deepEqual(plan.jobIds.length, 3);
    assert.equal(fixture.state.conversations.length, 0);
    assert.equal(fixture.state.messages.length, 0);
    assert.equal(fixture.state.jobs.length, 0);
});

test('media flow defaults an omitted normalized count to one asset', () => {
    const fixture = createFixture();
    const plan = fixture.flow.plan(command({call: {...command().call, count: undefined}}));

    assert.equal(plan.count, 1);
    assert.equal(plan.entries.length, 1);
    assert.equal(fixture.state.messages.length, 0);
    assert.equal(fixture.state.jobs.length, 0);
});

test('media apply is atomic and replay-safe for capability idempotency', () => {
    const fixture = createFixture();
    const plan = fixture.flow.plan(command());
    const first = fixture.flow.apply(plan, {transaction: fixture.transaction});
    assert.equal(first.type, 'done');
    assert.equal(first.replayed, false);
    assert.deepEqual(first.jobIds, plan.jobIds);
    assert.equal(first.messages.length, 3);
    assert.deepEqual(first.messages.map(message => message.jobs[0].id), plan.jobIds);
    assert.equal(fixture.state.messages.length, 3);
    assert.equal(fixture.state.jobs.length, 3);

    const replayPlan = fixture.flow.plan(command());
    assert.equal(replayPlan.replayed, true);
    const replay = fixture.flow.apply(replayPlan, {transaction: fixture.transaction});
    assert.equal(replay.replayed, true);
    assert.deepEqual(replay.jobIds, first.jobIds);
    assert.deepEqual(replay.messages.map(message => message.id), first.messages.map(message => message.id));
    assert.equal(fixture.state.messages.length, 3);
    assert.equal(fixture.state.jobs.length, 3);
});

test('media apply repairs a durable job whose placeholder is missing', () => {
    const fixture = createFixture();
    const first = fixture.flow.apply(fixture.flow.plan(command({call: {...command().call, count: 1}})), {transaction: fixture.transaction});
    const job = fixture.state.jobs[0];
    fixture.state.messages.length = 0;
    const repairedPlan = fixture.flow.plan(command({call: {...command().call, count: 1}}));
    assert.equal(repairedPlan.entries[0].repairMessage, true);
    const repaired = fixture.flow.apply(repairedPlan, {transaction: fixture.transaction});
    assert.equal(repaired.replayed, false);
    assert.equal(repaired.jobIds[0], job.id);
    assert.equal(fixture.state.jobs.length, 1);
    assert.equal(fixture.state.messages.length, 1);
    assert.equal(fixture.state.messages[0].id, job.messageId);
    assert.equal(first.jobIds[0], repaired.jobIds[0]);
});

test('media flow repairs a missing job on a later plan without duplicating its placeholder', () => {
    const fixture = createFixture();
    const firstPlan = fixture.flow.plan(command({call: {...command().call, count: 1}}));
    const first = fixture.flow.apply(firstPlan, {transaction: fixture.transaction});
    fixture.state.jobs.length = 0;

    const repairPlan = fixture.flow.plan(command({call: {...command().call, count: 1}}));
    assert.equal(repairPlan.messageIds[0], first.messages[0].id);
    const repaired = fixture.flow.apply(repairPlan, {transaction: fixture.transaction});
    assert.equal(repaired.replayed, false);
    assert.deepEqual(repaired.messages.map(message => message.id), [first.messages[0].id]);
    assert.deepEqual(repaired.jobIds, repairPlan.jobIds);
    assert.equal(fixture.state.messages.length, 1);
    assert.equal(fixture.state.jobs.length, 1);
});

test('media apply rolls back every placeholder and job when a later job fails', () => {
    const fixture = createFixture({failAfterJobs: 1});
    const plan = fixture.flow.plan(command());

    assert.throws(() => fixture.flow.apply(plan, {transaction: fixture.transaction}), /forced media job failure/);
    assert.equal(fixture.state.conversations.length, 0);
    assert.equal(fixture.state.messages.length, 0);
    assert.equal(fixture.state.jobs.length, 0);
});
