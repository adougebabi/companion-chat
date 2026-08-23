import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {AGENCY_INTENTION_SCHEMA_VERSION, normalizeAgencyIntention} from '../server/contracts/index.js';
import {createStartupRuntime} from '../server/runtime/startup.js';
import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';
import {createAgencyIntentionRepository} from '../server/infrastructure/agency-intention-repository.js';
import {createAgencyIntentionFlow} from '../server/application/agency-intention-flow.js';

const NOW = '2026-08-23T00:00:00.000Z';

function intention(overrides = {}) {
    return {
        schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION,
        intent: 'defer_response',
        topic: '当前话题',
        explanation: '我想先整理一下，再继续回答。',
        reasonCategory: 'needs_context',
        confidence: 0.78,
        evidenceRefs: ['message_agency_1'],
        source: 'llm',
        sourceMessageId: 'message_agency_1',
        causationId: 'message_agency_1',
        idempotencyKey: 'agency-intention-1',
        modelVersion: 'test-model-v1',
        ...overrides
    };
}

function fixture() {
    const startup = createStartupRuntime({Database, databasePath: ':memory:', dataDir: '/tmp'});
    const database = startup.database;
    database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_agency', 'Agency', 'companion', '#fff', ?, ?),
               ('persona_other_agency', 'Other', 'companion', '#000', ?, ?)
    `).run(NOW, NOW, NOW, NOW);
    const conversation = createConversationRepository({database});
    const own = conversation.getOrCreateConversation({personaId: 'persona_agency', id: 'conversation_agency', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_agency_1', conversationId: own.id, role: 'user', text: '先等等。', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    const other = conversation.getOrCreateConversation({personaId: 'persona_other_agency', id: 'conversation_other_agency', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_other_agency', conversationId: other.id, role: 'user', text: 'other', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    let sequence = 0;
    const id = prefix => `${prefix}_${++sequence}`;
    const repository = createAgencyIntentionRepository({database, clock: () => NOW, id});
    const transaction = callback => database.inTransaction ? callback() : database.transaction(callback)();
    const flow = createAgencyIntentionFlow({repositories: {agencyIntention: repository, conversation}, clock: () => NOW, idGenerator: id, transaction});
    return {startup, database, repository, flow};
}

test('agency intention is an evidence-backed LLM contract and fails closed without evidence', () => {
    assert.throws(() => normalizeAgencyIntention({...intention(), evidenceRefs: []}, {personaId: 'persona_agency', sourceMessageId: 'message_agency_1'}), /requires evidenceRefs/);
    const value = normalizeAgencyIntention(intention(), {personaId: 'persona_agency', sourceMessageId: 'message_agency_1'});
    assert.equal(value.schemaVersion, AGENCY_INTENTION_SCHEMA_VERSION);
    assert.equal(value.intent, 'defer_response');
});

test('agency intention flow stores a candidate without delivering a message and enforces scope/CAS', () => {
    const value = fixture();
    try {
        const applied = value.flow.apply(value.flow.plan({personaId: 'persona_agency', sourceMessageId: 'message_agency_1', agencyIntentions: [intention()]}));
        assert.equal(applied.changed, true);
        assert.equal(value.repository.list({personaId: 'persona_agency'})[0].status, 'candidate');
        assert.equal(value.database.prepare("SELECT COUNT(*) AS count FROM companion_messages WHERE role = 'assistant'").get().count, 0);
        const replay = value.flow.apply(value.flow.plan({personaId: 'persona_agency', sourceMessageId: 'message_agency_1', agencyIntentions: [intention({explanation: 'changed'})]}));
        assert.equal(replay.changed, false);
        const row = value.repository.list({personaId: 'persona_agency'})[0];
        assert.equal(value.repository.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 0, status: 'frozen'}).updated, false);
        assert.equal(value.repository.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 1, status: 'frozen', decision: {qualification: 'pending'}}).updated, true);
        assert.deepEqual(value.repository.list({personaId: 'persona_agency'})[0].decision, {qualification: 'pending'});
        assert.throws(() => value.flow.plan({personaId: 'persona_agency', sourceMessageId: 'message_other_agency', agencyIntentions: [intention({idempotencyKey: 'foreign-source'})]}), /does not (exist|belong)/);
        assert.throws(() => value.flow.plan({personaId: 'persona_other_agency', sourceMessageId: 'message_agency_1', agencyIntentions: [intention({idempotencyKey: 'foreign-persona'})]}), /does not (exist|belong)/);
    } finally {
        value.startup.close();
    }
});

test('agency intention apply rolls back with the caller transaction', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({personaId: 'persona_agency', sourceMessageId: 'message_agency_1', agencyIntentions: [intention({idempotencyKey: 'rollback-agency'})]});
        assert.throws(() => value.flow.apply(plan, {
            transaction(work) {
                return value.database.transaction(() => {
                    work();
                    throw new Error('caller rollback');
                })();
            }
        }), /caller rollback/);
        assert.equal(value.repository.list({personaId: 'persona_agency'}).length, 0);
    } finally {
        value.startup.close();
    }
});
