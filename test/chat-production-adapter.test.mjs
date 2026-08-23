import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createCompanionChatService} from '../server/application/chat-service.js';
import {
    ASSISTANT_MESSAGE_FACT_TYPE,
    createChatContextReader,
    createChatLlmStreamingPort,
    createChatPresentationMapper,
    createChatProductionPorts,
    splitChatAssistantReply
} from '../server/application/chat-production-adapter.js';
import {createConversationCommitAdapter} from '../server/infrastructure/conversation-commit-adapter.js';
import {createCompanionRuntime} from '../server/runtime/runtime.js';

function sink() {
    return {
        events: [],
        writableEnded: false,
        on() { return this; },
        removeListener() { return this; }
    };
}

function repositoryFixture() {
    const rows = [];
    const conversation = {id: 'conversation_test', persona_id: 'persona_test'};
    return {
        rows,
        getOrCreateConversation() {
            return conversation;
        },
        findMessage({id}) {
            return rows.find(row => row.id === id) ?? null;
        },
        appendMessages({conversationId, messages, updatedAt}) {
            const inserted = messages.map(message => ({
                id: message.id,
                conversation_id: conversationId,
                role: message.role,
                text: message.text,
                attachments_json: message.attachmentsJson,
                generation_json: message.generationJson,
                jobs_json: message.jobsJson,
                created_at: message.createdAt,
                read_at: message.readAt
            }));
            rows.push(...inserted);
            conversation.updated_at = updatedAt;
            return inserted;
        }
    };
}

test('context adapter preserves persona/time and limits model history', async () => {
    let received;
    const reader = createChatContextReader({
        contextFor(personaId, at) {
            received = {personaId, at};
            return {prompt: 'identity', layers: {systemCapability: 'capabilities'}};
        },
        clock: () => '2026-08-21T10:00:00.000Z'
    });
    const result = await reader.read({
        personaId: 'persona_test',
        command: {chatAt: '2026-08-21T12:00:00+08:00'},
        messages: Array.from({length: 25}, (_, index) => ({id: index}))
    });

    assert.equal(result.prompt, 'identity');
    assert.equal(received.personaId, 'persona_test');
    assert.equal(received.at.toISOString(), '2026-08-21T04:00:00.000Z');
});

test('stream adapter owns prompt assembly while leaving completion decoding injected', async () => {
    let request;
    const port = createChatLlmStreamingPort({
        stream(input) {
            request = input;
            return {tokens: ['ready'], toolCalls: []};
        },
        tools: [{name: 'scene_event'}]
    });
    const completion = await port.stream({
        context: {prompt: 'identity', layers: {systemCapability: 'capabilities'}},
        messages: [{role: 'user', text: 'hello'}],
        personaId: 'persona_test'
    });

    assert.deepEqual(completion, {tokens: ['ready'], toolCalls: []});
    assert.deepEqual(request.messages, [
        {role: 'system', content: 'identity\n\ncapabilities'},
        {role: 'user', content: 'hello'}
    ]);
    assert.deepEqual(request.tools, [{name: 'scene_event'}]);
});

test('presentation mapper creates ordered assistant facts and capability presentation', () => {
    const map = createChatPresentationMapper({
        clock: () => '2026-08-21T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_test`
    });
    const result = map({
        command: {personaId: 'persona_test'},
        completion: {text: '第一句。第二句！'},
        capabilityPresentation: [{
            type: 'capability-result',
            result: {name: 'scene_event', ok: true, result: {operation: 'switch'}}
        }]
    });

    assert.equal(result.facts[0].type, ASSISTANT_MESSAGE_FACT_TYPE);
    assert.deepEqual(result.chatResult.messages.map(message => message.text), ['第一句。', '第二句！']);
    assert.deepEqual(result.chatResult.sceneEvent, {operation: 'switch'});
    assert.equal(result.facts[0].messages[1].createdAt, '2026-08-21T00:00:00.001Z');
    assert.deepEqual(splitChatAssistantReply(''), []);
});

test('conversation commit adapter materializes assistant facts idempotently', () => {
    const repository = repositoryFixture();
    const adapter = createConversationCommitAdapter({
        repository,
        clock: () => '2026-08-21T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_test`,
        transaction: callback => callback()
    });
    const stepResult = {
        facts: [{
            type: ASSISTANT_MESSAGE_FACT_TYPE,
            personaId: 'persona_test',
            messages: [{
                id: 'message_test',
                role: 'assistant',
                text: '已提交。',
                createdAt: '2026-08-21T00:00:00.000Z',
                attachments: [],
                jobs: []
            }]
        }],
        projections: [], effects: [], presentation: []
    };

    assert.equal(adapter.commit(stepResult).messages.length, 1);
    assert.equal(adapter.commit(stepResult).messages.length, 0);
    assert.equal(repository.rows.length, 1);
    assert.equal(repository.rows[0].text, '已提交。');
});

test('explicit production ports complete assistant commit and SSE done contract', async () => {
    const repository = repositoryFixture();
    const commit = createConversationCommitAdapter({
        repository,
        clock: () => '2026-08-21T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_test`,
        transaction: callback => callback()
    });
    const ports = createChatProductionPorts({
        contextReader: {read: async () => ({prompt: 'identity', layers: {systemCapability: 'capabilities'}})},
        llmStreamingPort: {stream: async () => ({tokens: ['你好。'], toolCalls: []})},
        presentationMapper: createChatPresentationMapper({
            clock: () => '2026-08-21T00:00:00.000Z',
            idGenerator: prefix => `${prefix}_test`
        })
    });
    const events = [];
    const service = createCompanionChatService({
        ...ports,
        conversationRepository: {listMessages: () => ({items: []})},
        capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
        commitBoundary: commit,
        sendSse: (_sink, event) => events.push(event),
        end: response => { response.writableEnded = true; }
    });
    const response = sink();

    await service.handle({personaId: 'persona_test', text: 'hello'}, response);

    assert.deepEqual(events, [
        {type: 'token', token: '你好。'},
        {
            type: 'done',
            messages: [{
                id: 'message_test', role: 'assistant', text: '你好。', attachments: [], generation: null,
                jobs: [], createdAt: '2026-08-21T00:00:00.000Z', readAt: null,
                proactiveEventId: null, proactivePendingEventId: null
            }],
            message: {
                id: 'message_test', role: 'assistant', text: '你好。', attachments: [], generation: null,
                jobs: [], createdAt: '2026-08-21T00:00:00.000Z', readAt: null,
                proactiveEventId: null, proactivePendingEventId: null
            },
            learned: [], jobs: []
        }
    ]);
    assert.equal(repository.rows.length, 1);
    assert.equal(repository.rows[0].role, 'assistant');
    assert.equal(response.writableEnded, true);
});

test('createCompanionRuntime accepts explicit production ports for a repository-backed chat turn', async () => {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-chat-runtime-'));
    const repository = repositoryFixture();
    const commit = createConversationCommitAdapter({
        repository,
        clock: () => '2026-08-21T00:00:00.000Z',
        idGenerator: prefix => `${prefix}_runtime`,
        transaction: callback => callback()
    });
    const events = [];
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir},
        chatProductionPorts: {
            contextReader: {read: async () => ({prompt: 'identity', layers: {systemCapability: 'capabilities'}})},
            llmStreamingPort: {stream: async () => ({tokens: ['runtime reply.'], toolCalls: []})},
            presentationMapper: createChatPresentationMapper({
                clock: () => '2026-08-21T00:00:00.000Z',
                idGenerator: prefix => `${prefix}_runtime`
            }),
            capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
            conversationRepository: repository,
            commitBoundary: commit,
            sendSse: (_sink, event) => events.push(event),
            end: response => { response.writableEnded = true; }
        }
    });
    try {
        const route = runtime.app.router.stack.find(item => item.route?.path === '/api/companion/chat');
        assert.ok(route);
        const response = {
            writableEnded: false,
            status() { return this; },
            set() { return this; },
            flushHeaders() {},
            on() { return this; },
            removeListener() { return this; },
            end() { this.writableEnded = true; }
        };
        const result = route.route.stack[0].handle({body: {personaId: 'persona_test', text: 'hello'}}, response);
        if (result?.then) await result;
        assert.equal(events.at(-1).type, 'done');
        assert.equal(events.at(-1).messages[0].text, 'runtime reply.');
        assert.equal(repository.rows.length, 1);
        assert.equal(repository.rows[0].text, 'runtime reply.');
        assert.equal(response.writableEnded, true);
    } finally {
        await runtime.stop().catch(() => {});
        rmSync(dataDir, {recursive: true, force: true});
    }
});

test('production adapter modules stay independent from legacy transport and storage entrypoints', async () => {
    for (const path of [
        '../server/application/chat-production-adapter.js',
        '../server/infrastructure/conversation-commit-adapter.js'
    ]) {
        const source = await readFile(new URL(path, import.meta.url), 'utf8');
        assert.doesNotMatch(source, /server\.js|express|better-sqlite3|child_process|fetch\s*\(/i);
    }
});
