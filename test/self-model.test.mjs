import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {
    SELF_MODEL_CLAIM_SCHEMA_VERSION,
    normalizeSelfModelClaim
} from '../server/contracts/index.js';
import {createStartupRuntime} from '../server/runtime/startup.js';
import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';
import {createInteractionFactRepository} from '../server/infrastructure/interaction-fact-repository.js';
import {createSelfModelRepository} from '../server/infrastructure/self-model-repository.js';
import {createSelfModelFlow} from '../server/application/self-model-flow.js';

const NOW = '2026-08-23T00:00:00.000Z';

function claim(overrides = {}) {
    return {
        schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
        category: 'preference',
        claim: '我更喜欢提前约定安排。',
        summary: '更偏好提前约定的安排。',
        confidence: 0.84,
        evidenceRefs: ['message_self_1'],
        source: 'llm',
        uncertainty: {kind: 'candidate', note: '仍需后续互动修正'},
        sourceMessageId: 'message_self_1',
        causationId: 'message_self_1',
        idempotencyKey: 'self-model-1',
        modelVersion: 'test-model-v1',
        ...overrides
    };
}

function fixture() {
    const startup = createStartupRuntime({Database, databasePath: ':memory:', dataDir: '/tmp'});
    const database = startup.database;
    database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_self', 'Self', 'companion', '#fff', ?, ?),
               ('persona_other_self', 'Other', 'companion', '#000', ?, ?)
    `).run(NOW, NOW, NOW, NOW);
    const conversation = createConversationRepository({database});
    const ownConversation = conversation.getOrCreateConversation({personaId: 'persona_self', id: 'conversation_self', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_self_1', conversationId: ownConversation.id, role: 'user', text: '我喜欢提前约定。', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    const otherConversation = conversation.getOrCreateConversation({personaId: 'persona_other_self', id: 'conversation_other_self', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_other_self', conversationId: otherConversation.id, role: 'user', text: 'other', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    let sequence = 0;
    const id = prefix => `${prefix}_${++sequence}`;
    const interactionFact = createInteractionFactRepository({database, clock: () => NOW, id});
    const selfModel = createSelfModelRepository({database, clock: () => NOW, id});
    const transaction = callback => database.inTransaction ? callback() : database.transaction(callback)();
    const flow = createSelfModelFlow({
        repositories: {selfModel, interactionFact, conversation},
        clock: () => NOW,
        idGenerator: id,
        transaction
    });
    return {startup, database, conversation, interactionFact, selfModel, flow};
}

test('self-model claims require LLM evidence and preserve uncertainty', () => {
    assert.throws(() => normalizeSelfModelClaim({
        ...claim(), evidenceRefs: []
    }, {personaId: 'persona_self', sourceMessageId: 'message_self_1'}), /requires evidenceRefs/);
    const normalized = normalizeSelfModelClaim(claim(), {personaId: 'persona_self', sourceMessageId: 'message_self_1'});
    assert.equal(normalized.schemaVersion, SELF_MODEL_CLAIM_SCHEMA_VERSION);
    assert.equal(normalized.summary, '更偏好提前约定的安排。');
    assert.deepEqual(normalized.uncertainty, {kind: 'candidate', note: '仍需后续互动修正'});
});
test('self-model flow is persona-scoped, idempotent, CAS guarded, and auto-activates only the LLM claim', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({
            personaId: 'persona_self',
            sourceMessageId: 'message_self_1',
            selfModelClaims: [claim()]
        });
        const result = value.flow.apply(plan);
        assert.equal(result.changed, true);
        assert.equal(value.selfModel.listActive({personaId: 'persona_self'}).length, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_persona_foundation_revisions').get().count, 0);

        const replay = value.flow.apply(value.flow.plan({
            personaId: 'persona_self',
            sourceMessageId: 'message_self_1',
            selfModelClaims: [claim({summary: '不同摘要'})]
        }));
        assert.equal(replay.changed, false);
        assert.equal(value.selfModel.listActive({personaId: 'persona_self'})[0].summary, '更偏好提前约定的安排。');

        const row = value.selfModel.list({personaId: 'persona_self'})[0];
        assert.equal(value.selfModel.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 0, status: 'rejected'}).updated, false);
        assert.equal(value.selfModel.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 1, status: 'deferred'}).updated, true);
        assert.equal(value.selfModel.listActive({personaId: 'persona_self'}).length, 0);

        assert.throws(() => value.flow.plan({
            personaId: 'persona_self',
            sourceMessageId: 'message_other_self',
            selfModelClaims: [claim({idempotencyKey: 'foreign-source'})]
        }), /does not (exist|belong)/);
        assert.throws(() => value.flow.plan({
            personaId: 'persona_other_self',
            sourceMessageId: 'message_self_1',
            selfModelClaims: [claim({idempotencyKey: 'foreign-persona'})]
        }), /does not (exist|belong)/);
    } finally {
        value.startup.close();
    }
});

test('self-model apply participates in caller transaction rollback', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({personaId: 'persona_self', sourceMessageId: 'message_self_1', selfModelClaims: [claim({idempotencyKey: 'rollback-self'})]});
        assert.throws(() => value.flow.apply(plan, {
            transaction(work) {
                return value.database.transaction(() => {
                    work();
                    throw new Error('caller rollback');
                })();
            }
        }), /caller rollback/);
        assert.equal(value.selfModel.list({personaId: 'persona_self'}).length, 0);
    } finally {
        value.startup.close();
    }
});
