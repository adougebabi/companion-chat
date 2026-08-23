import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {
    APPRAISAL_SCHEMA_VERSION,
    INTERACTION_FACT_SCHEMA_VERSION,
    normalizeAppraisalCandidate,
    normalizeInteractionFact,
    normalizeStructuredTurnSafely,
    STRUCTURED_TURN_SCHEMA_VERSION
} from '../server/contracts/index.js';
import {createStartupRuntime} from '../server/runtime/startup.js';
import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';
import {createPersonaRepository} from '../server/infrastructure/persona-repository.js';
import {createAffectRepository} from '../server/infrastructure/affect-repository.js';
import {createInteractionFactRepository} from '../server/infrastructure/interaction-fact-repository.js';
import {createAppraisalRepository} from '../server/infrastructure/appraisal-repository.js';
import {createAffectFlow} from '../server/application/affect-flow.js';
import {createAppraisalFlow} from '../server/application/appraisal-flow.js';

const NOW = '2026-08-23T00:00:00.000Z';

function fixture() {
    const startup = createStartupRuntime({Database, databasePath: ':memory:', dataDir: '/tmp'});
    const database = startup.database;
    database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_appraisal', 'Appraisal', 'companion', '#fff', ?, ?),
               ('persona_other', 'Other', 'companion', '#000', ?, ?)
    `).run(NOW, NOW, NOW, NOW);
    const conversation = createConversationRepository({database});
    const first = conversation.getOrCreateConversation({personaId: 'persona_appraisal', id: 'conversation_appraisal', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_appraisal', conversationId: first.id, role: 'user', text: '今天聊得很顺利。', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    const second = conversation.getOrCreateConversation({personaId: 'persona_other', id: 'conversation_other', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_other', conversationId: second.id, role: 'user', text: '另一位用户。', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    let sequence = 0;
    const id = prefix => `${prefix}_${++sequence}`;
    const repositories = {
        persona: createPersonaRepository({database}),
        conversation,
        affect: createAffectRepository({database, clock: () => NOW, id}),
        interactionFact: createInteractionFactRepository({database, clock: () => NOW, id}),
        appraisal: createAppraisalRepository({database, clock: () => NOW, id})
    };
    const transaction = callback => database.inTransaction ? callback() : database.transaction(callback)();
    const affectFlow = createAffectFlow({repositories, clock: () => NOW, idGenerator: id, transaction});
    const flow = createAppraisalFlow({repositories, affectFlow, clock: () => NOW, idGenerator: id, transaction});
    return {startup, database, repositories, flow};
}

function appraisal(overrides = {}) {
    return {
        schemaVersion: APPRAISAL_SCHEMA_VERSION,
        category: 'comfort',
        confidence: 0.91,
        rationale: '模型认为本轮互动呈现出顺畅的连接感。',
        evidenceRefs: ['message_appraisal'],
        affectEvents: [{type: 'social_connection', confidence: 0.9, idempotencyKey: 'affect_appraisal_1'}],
        driveSignals: [{drive: 'social', direction: 'decrease_pressure', confidence: 0.8, idempotencyKey: 'drive_appraisal_1'}],
        sourceMessageId: 'message_appraisal',
        idempotencyKey: 'appraisal_1',
        modelVersion: 'test-model-v1',
        ...overrides
    };
}

test('interaction facts and appraisal signals use versioned LLM-first contracts', () => {
    const fact = normalizeInteractionFact({
        schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
        factType: 'user_message',
        personaId: 'persona_appraisal',
        sourceMessageId: 'message_appraisal',
        idempotencyKey: 'interaction_1',
        payload: {messageId: 'message_appraisal', role: 'user'},
        source: 'chat'
    }, {personaId: 'persona_appraisal', sourceMessageId: 'message_appraisal'});
    assert.equal(fact.schemaVersion, INTERACTION_FACT_SCHEMA_VERSION);
    const candidate = normalizeAppraisalCandidate(appraisal(), {personaId: 'persona_appraisal', sourceMessageId: 'message_appraisal'});
    assert.equal(candidate.schemaVersion, APPRAISAL_SCHEMA_VERSION);
    assert.equal(candidate.affectEvents[0].type, 'social_connection');
    assert.throws(() => normalizeAppraisalCandidate({...appraisal(), schemaVersion: 'companion.appraisal.v999'}, {personaId: 'persona_appraisal', sourceMessageId: 'message_appraisal'}), /schemaVersion/);
});

test('appraisal flow commits interaction fact, audit candidate, and existing affect reducer atomically', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({
            personaId: 'persona_appraisal',
            sourceMessageId: 'message_appraisal',
            causationId: 'message_appraisal',
            appraisals: [appraisal()],
            interactionFact: {idempotencyKey: 'interaction_1', payload: {messageId: 'message_appraisal', role: 'user'}}
        });
        assert.equal(plan.previewResult.appraisalCount, 1);
        const result = value.flow.apply(plan);
        assert.equal(result.changed, true);
        assert.equal(result.affect.eventCount, 2);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_interaction_facts').get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_appraisals WHERE status = \'applied\'').get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_persona_affect_events').get().count, 2);

        const replayPlan = value.flow.plan({
            personaId: 'persona_appraisal',
            sourceMessageId: 'message_appraisal',
            appraisals: [appraisal({category: 'different-model-label'})],
            interactionFact: {idempotencyKey: 'interaction_1'}
        });
        const replay = value.flow.apply(replayPlan);
        assert.equal(replay.changed, false);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_appraisals').get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_persona_affect_events').get().count, 2);
    } finally {
        value.startup.close();
    }
});

test('appraisal flow enforces persona-scoped source ownership and CAS revisions', () => {
    const value = fixture();
    try {
        assert.throws(() => value.flow.plan({
            personaId: 'persona_appraisal',
            sourceMessageId: 'message_other',
            appraisals: [appraisal({sourceMessageId: 'message_other'})]
        }), /does not (exist|belong)/);
        const foreignFact = value.repositories.interactionFact.record({
            schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
            factType: 'user_message',
            personaId: 'persona_other',
            sourceMessageId: 'message_other',
            idempotencyKey: 'linked_fact',
            payload: {messageId: 'message_other'},
            source: 'chat'
        }).fact;
        assert.throws(() => value.flow.plan({
            personaId: 'persona_appraisal',
            sourceMessageId: 'message_appraisal',
            appraisals: [appraisal({interactionFactId: foreignFact.id, idempotencyKey: 'missing-link'})]
        }), /interaction fact does not exist/);
        const fact = value.repositories.interactionFact.record({
            schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
            factType: 'user_message',
            personaId: 'persona_appraisal',
            sourceMessageId: 'message_appraisal',
            idempotencyKey: 'cas_fact',
            payload: {messageId: 'message_appraisal'},
            source: 'chat'
        }).fact;
        const stale = value.repositories.interactionFact.compareAndSwap({id: fact.id, personaId: fact.personaId, expectedRevision: 0, payload: {stale: true}});
        assert.equal(stale.updated, false);
        assert.equal(value.repositories.interactionFact.findById(fact.id, fact.personaId).revision, 1);
    } finally {
        value.startup.close();
    }
});

test('invalid appraisal sidecars fail closed without synthesizing affect signals', () => {
    const normalized = normalizeStructuredTurnSafely({
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text: '收到。',
        control: {appraisals: [appraisal({schemaVersion: 'companion.appraisal.v999'})]}
    }, {personaId: 'persona_appraisal', sourceMessageId: 'message_appraisal'});
    assert.equal(normalized.ok, false);
    assert.deepEqual(normalized.value.control.appraisals, []);
    assert.deepEqual(normalized.value.control.affectEvents, []);
    assert.ok(normalized.value.parseDiagnostics.length > 0);
});
