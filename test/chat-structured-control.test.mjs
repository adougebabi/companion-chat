import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createStartupRuntime} from '../server/runtime/startup.js';
import {createMemoryRepository} from '../server/infrastructure/memory-repository.js';
import {createPersonaRepository} from '../server/infrastructure/persona-repository.js';
import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';
import {createAffectRepository} from '../server/infrastructure/affect-repository.js';
import {createMemoryService} from '../server/application/memory-service.js';
import {createMemoryEventFlow} from '../server/application/memory-flow.js';
import {createAffectFlow} from '../server/application/affect-flow.js';
import {createFlowCapabilityRegistry, createCapabilityHandoffAdapter} from '../server/application/capability-handoff-adapter.js';
import {createCompanionChatService} from '../server/application/chat-service.js';
import {createChatPresentationMapper} from '../server/application/chat-production-adapter.js';
import {createConversationCommitAdapter} from '../server/infrastructure/conversation-commit-adapter.js';

const NOW = '2026-08-22T00:00:00.000Z';

function fixture() {
    const startup = createStartupRuntime({Database, databasePath: ':memory:', dataDir: '/tmp'});
    const database = startup.database;
    database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_structured', '结构化人格', '陪伴者', '#fff', ?, ?)
    `).run(NOW, NOW);
    const blueprint = {read: () => ({personaId: 'persona_structured'})};
    let idSequence = 0;
    const nextId = prefix => `${prefix}_${++idSequence}`;
    const repositories = {
        memory: createMemoryRepository({database, clock: () => NOW}),
        persona: createPersonaRepository({database}),
        conversation: createConversationRepository({database}),
        affect: createAffectRepository({database, clock: () => NOW, id: nextId, blueprintRepository: blueprint})
    };
    const memoryService = createMemoryService({repositories, clock: () => NOW});
    const memoryEventFlow = createMemoryEventFlow({repositories, memoryService, clock: () => NOW, idGenerator: nextId});
    const affectFlow = createAffectFlow({repositories, clock: () => NOW, idGenerator: nextId});
    const capabilityDispatcher = createCapabilityHandoffAdapter({registry: createFlowCapabilityRegistry({memoryEventFlow})});
    const commitBoundary = createConversationCommitAdapter({
        repository: repositories.conversation,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_assistant`,
        transaction: callback => database.transaction(callback)()
    });
    const events = [];
    const service = createCompanionChatService({
        contextReader: {read: async () => ({prompt: 'test', layers: {systemCapability: 'test'}})},
        llmStreamingPort: {
            stream: async () => ({
                text: '我已经记住了。',
                tokens: ['我已经记住了。'],
                toolCalls: [],
                structuredSidecar: {
                    schemaVersion: 'companion.turn.v1',
                    text: '我已经记住了。',
                    control: {
                        affectEvents: [{type: 'social_connection', confidence: 0.9, idempotencyKey: 'affect_turn_1'}],
                        driveSignals: [{drive: 'social', direction: 'decrease_pressure', confidence: 0.8, idempotencyKey: 'drive_turn_1'}],
                        capabilityCalls: [{
                            name: 'memory_event',
                            source: 'structured',
                            idempotencyKey: 'memory_turn_1',
                            arguments: {memory: {
                                operation: 'upsert', key: '喜欢的饮料', value: '茶', confidence: 0.9,
                                idempotencyKey: 'memory_turn_1'
                            }}
                        }]
                    }
                }
            })
        },
        conversationRepository: repositories.conversation,
        userMessageWriter: ({personaId, text}) => {
            const conversation = repositories.conversation.getOrCreateConversation({
                personaId, id: 'conversation_structured', createdAt: NOW, updatedAt: NOW
            });
            return repositories.conversation.appendMessage({
                id: 'message_user_structured', conversationId: conversation.id, role: 'user', text,
                attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: NOW, readAt: NOW
            });
        },
        presentationMapper: createChatPresentationMapper({clock: () => NOW, idGenerator: prefix => `${prefix}_assistant`}),
        capabilityDispatcher,
        affectFlow,
        commitBoundary,
        sendSse: (_sink, event) => events.push(event),
        end: sink => { sink.writableEnded = true; }
    });
    return {startup, database, repositories, service, events};
}

test('structured turn commits explicit memory and affect/drives effects with the assistant message', async () => {
    const fixtureValue = fixture();
    const sink = {writableEnded: false, on() { return this; }, removeListener() {}};
    try {
        const result = await fixtureValue.service.handle({personaId: 'persona_structured', text: '我最近喜欢喝茶。'}, sink);
        assert.ok(result);
        assert.deepEqual(fixtureValue.events.map(event => event.type), ['token', 'done']);
        assert.deepEqual(fixtureValue.database.prepare('SELECT memory_key, value, source_id FROM companion_memories').all(), [{
            memory_key: '喜欢的饮料', value: '茶', source_id: 'message_user_structured'
        }]);
        const affect = fixtureValue.database.prepare('SELECT event_type, source_message_id FROM companion_persona_affect_events').all();
        assert.deepEqual(affect, [
            {event_type: 'social_connection', source_message_id: 'message_user_structured'},
            {event_type: 'drive_signal', source_message_id: 'message_user_structured'}
        ]);
        const snapshot = fixtureValue.database.prepare('SELECT pleasure, drives_json FROM companion_persona_affect_states').get();
        assert.equal(snapshot.pleasure > 0, true);
        assert.equal(JSON.parse(snapshot.drives_json).social < 0.5, true);
        assert.equal(fixtureValue.database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE role = \'assistant\'').get().count, 1);
    } finally {
        await fixtureValue.startup.close();
    }
});
