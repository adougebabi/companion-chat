import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {
    MEMORY_CONSOLIDATION_SCHEMA_VERSION,
    STRUCTURED_TURN_SCHEMA_VERSION,
    normalizeMemoryConsolidationCandidate,
    normalizeStructuredTurnSafely
} from '../server/contracts/index.js';
import {createStartupRuntime} from '../server/runtime/startup.js';
import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';
import {createInteractionFactRepository} from '../server/infrastructure/interaction-fact-repository.js';
import {createMemoryConsolidationRepository} from '../server/infrastructure/memory-consolidation-repository.js';
import {createMemoryConsolidationFlow} from '../server/application/memory-consolidation-flow.js';
import {createCompanionChatService} from '../server/application/chat-service.js';
import {createChatPresentationMapper} from '../server/application/chat-production-adapter.js';
import {createConversationCommitAdapter} from '../server/infrastructure/conversation-commit-adapter.js';

const NOW = '2026-08-23T00:00:00.000Z';

function candidate(overrides = {}) {
    return {
        schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
        layer: 'preference',
        key: 'favorite.drink',
        value: 'tea',
        confidence: 0.88,
        evidenceRefs: ['message_memory_1'],
        sourceFactRefs: ['message_memory_1'],
        sourceMessageId: 'message_memory_1',
        causationId: 'message_memory_1',
        idempotencyKey: 'memory-consolidation-1',
        source: 'llm',
        modelVersion: 'test-model-v1',
        ...overrides
    };
}

function fixture() {
    const startup = createStartupRuntime({Database, databasePath: ':memory:', dataDir: '/tmp'});
    const database = startup.database;
    database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_memory', 'Memory', 'companion', '#fff', ?, ?),
               ('persona_other_memory', 'Other', 'companion', '#000', ?, ?)
    `).run(NOW, NOW, NOW, NOW);
    const conversation = createConversationRepository({database});
    const ownConversation = conversation.getOrCreateConversation({personaId: 'persona_memory', id: 'conversation_memory', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_memory_1', conversationId: ownConversation.id, role: 'user', text: '我喜欢茶。', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    const otherConversation = conversation.getOrCreateConversation({personaId: 'persona_other_memory', id: 'conversation_other_memory', createdAt: NOW, updatedAt: NOW});
    conversation.appendMessage({id: 'message_other_memory', conversationId: otherConversation.id, role: 'user', text: 'other', attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
    let sequence = 0;
    const id = prefix => `${prefix}_${++sequence}`;
    const sourceFacts = createInteractionFactRepository({database, clock: () => NOW, id});
    const candidates = createMemoryConsolidationRepository({database, clock: () => NOW, id});
    const transaction = callback => database.inTransaction ? callback() : database.transaction(callback)();
    const flow = createMemoryConsolidationFlow({
        repositories: {memoryConsolidation: candidates, interactionFact: sourceFacts, conversation},
        clock: () => NOW,
        idGenerator: id,
        transaction
    });
    return {startup, database, conversation, sourceFacts, candidates, flow, id};
}

test('memory consolidation contract is versioned and fails closed without evidence', () => {
    assert.throws(() => normalizeMemoryConsolidationCandidate({
        ...candidate(), schemaVersion: 'companion.memory-consolidation.v999'
    }, {personaId: 'persona_memory', sourceMessageId: 'message_memory_1'}), /schemaVersion/);
    assert.throws(() => normalizeMemoryConsolidationCandidate({
        ...candidate(), evidenceRefs: [], sourceFactRefs: []
    }, {personaId: 'persona_memory', sourceMessageId: 'message_memory_1'}), /requires evidenceRefs or sourceFactRefs/);
    assert.throws(() => normalizeMemoryConsolidationCandidate({
        ...candidate(), key: 'key', claim: 'claim'
    }, {personaId: 'persona_memory', sourceMessageId: 'message_memory_1'}), /exactly one/);
    const claim = normalizeMemoryConsolidationCandidate({
        ...candidate(), key: undefined, value: undefined, claim: 'a bounded model claim'
    }, {personaId: 'persona_memory', sourceMessageId: 'message_memory_1'});
    assert.equal(claim.claim, 'a bounded model claim');
    assert.equal(claim.value, null);
});

test('memory consolidation flow is persona-scoped, idempotent, CAS guarded, and never promotes active memory', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({personaId: 'persona_memory', sourceMessageId: 'message_memory_1', memoryConsolidations: [candidate()]});
        const applied = value.flow.apply(plan);
        assert.equal(applied.changed, true);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memory_consolidation_candidates').get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memories').get().count, 0);
        const replay = value.flow.apply(value.flow.plan({personaId: 'persona_memory', sourceMessageId: 'message_memory_1', memoryConsolidations: [candidate({value: 'coffee'})]}));
        assert.equal(replay.changed, false);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memory_consolidation_candidates').get().count, 1);

        const row = value.candidates.list({personaId: 'persona_memory'})[0];
        const stale = value.candidates.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 0, status: 'rejected'});
        assert.equal(stale.updated, false);
        const closed = value.candidates.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 1, status: 'deferred'});
        assert.equal(closed.updated, true);
        assert.equal(closed.candidate.revision, 2);
        assert.equal(value.candidates.compareAndSwap({id: row.id, personaId: row.personaId, expectedRevision: 1, status: 'rejected'}).updated, false);

        assert.throws(() => value.flow.plan({
            personaId: 'persona_memory', sourceMessageId: 'message_other_memory', memoryConsolidations: [candidate({idempotencyKey: 'other-source'})]
        }), /does not (exist|belong)/);
        assert.throws(() => value.flow.plan({
            personaId: 'persona_other_memory', sourceMessageId: 'message_memory_1', memoryConsolidations: [candidate({idempotencyKey: 'cross-persona'})]
        }), /does not (exist|belong)/);
        assert.throws(() => value.flow.plan({
            personaId: 'persona_memory',
            sourceMessageId: 'message_memory_1',
            memoryConsolidations: [candidate({sourceMessageId: 'message_other_memory', idempotencyKey: 'cross-source'})]
        }), /sourceMessageId does not match/);
        assert.throws(() => value.flow.plan({
            personaId: 'persona_memory',
            sourceMessageId: 'message_memory_1',
            memoryConsolidations: [candidate({interactionFactId: 'missing-fact', idempotencyKey: 'missing-interaction-fact'})]
        }), /source fact missing-fact does not exist/);
    } finally {
        value.startup.close();
    }
});

test('memory consolidation apply participates in caller transaction rollback', () => {
    const value = fixture();
    try {
        const plan = value.flow.plan({personaId: 'persona_memory', sourceMessageId: 'message_memory_1', memoryConsolidations: [candidate({idempotencyKey: 'rollback-candidate'})]});
        assert.throws(() => value.flow.apply(plan, {
            transaction(work) {
                return value.database.transaction(() => {
                    work();
                    throw new Error('caller rollback');
                })();
            }
        }), /caller rollback/);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memory_consolidation_candidates').get().count, 0);
    } finally {
        value.startup.close();
    }
});

test('invalid consolidation sidecars preserve visible text but produce no candidate', () => {
    const normalized = normalizeStructuredTurnSafely({
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text: '我收到了。',
        control: {memoryConsolidations: [candidate({schemaVersion: 'companion.memory-consolidation.v999'})]}
    }, {personaId: 'persona_memory', sourceMessageId: 'message_memory_1'});
    assert.equal(normalized.ok, false);
    assert.equal(normalized.value.text, '我收到了。');
    assert.deepEqual(normalized.value.control.memoryConsolidations, []);
});

test('chat sidecar candidates commit with assistant message in the caller transaction', async () => {
    const value = fixture();
    const events = [];
    const commit = createConversationCommitAdapter({
        repository: value.conversation,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_assistant`,
        transaction: callback => value.database.inTransaction ? callback() : value.database.transaction(callback)()
    });
    const service = createCompanionChatService({
        contextReader: {read: async () => ({prompt: 'test'})},
        llmStreamingPort: {
            stream: async () => ({
                text: '我记下了。',
                tokens: ['我记下了。'],
                toolCalls: [],
                structuredSidecar: {
                    schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
                    text: '我记下了。',
                    control: {memoryConsolidations: [candidate({
                        sourceMessageId: 'message_chat',
                        sourceFactRefs: ['message_chat'],
                        evidenceRefs: ['message_chat'],
                        causationId: 'message_chat',
                        idempotencyKey: 'memory-consolidation-chat'
                    })]}
                }
            })
        },
        conversationRepository: value.conversation,
        userMessageWriter: ({personaId, text}) => {
            const conversation = value.conversation.getOrCreateConversation({personaId, id: 'conversation_chat', createdAt: NOW, updatedAt: NOW});
            return value.conversation.appendMessage({id: 'message_chat', conversationId: conversation.id, role: 'user', text, attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW});
        },
        presentationMapper: createChatPresentationMapper({clock: () => NOW, idGenerator: prefix => `${prefix}_reply`}),
        capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
        memoryConsolidationFlow: value.flow,
        commitBoundary: commit,
        sendSse: (_sink, event) => events.push(event),
        end: sink => { sink.writableEnded = true; }
    });
    try {
        const result = await service.handle({personaId: 'persona_memory', text: '我喜欢茶。'}, {writableEnded: false, on() { return this; }, removeListener() {}});
        assert.ok(result);
        assert.deepEqual(events.map(event => event.type), ['token', 'done']);
        assert.equal(value.database.prepare("SELECT COUNT(*) AS count FROM companion_messages WHERE role = 'assistant'").get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memory_consolidation_candidates').get().count, 1);
        assert.equal(value.database.prepare('SELECT COUNT(*) AS count FROM companion_memories').get().count, 0);
    } finally {
        value.startup.close();
    }
});
