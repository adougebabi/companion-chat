import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createConversationRepository} from '../server/infrastructure/conversation-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.pragma('foreign_keys = ON');
    database.exec(`
        CREATE TABLE companion_personas (id TEXT PRIMARY KEY);
        CREATE TABLE companion_conversations (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL UNIQUE REFERENCES companion_personas(id),
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );
        CREATE TABLE companion_messages (
            id TEXT PRIMARY KEY,
            conversation_id TEXT NOT NULL REFERENCES companion_conversations(id),
            role TEXT NOT NULL,
            text TEXT NOT NULL,
            attachments_json TEXT NOT NULL DEFAULT '[]',
            generation_json TEXT,
            jobs_json TEXT NOT NULL DEFAULT '[]',
            proactive_event_id TEXT,
            proactive_pending_event_id TEXT,
            created_at TEXT NOT NULL,
            read_at TEXT
        );
        CREATE INDEX companion_messages_conversation_time_idx ON companion_messages(conversation_id, created_at DESC, id DESC);
    `);
    database.prepare('INSERT INTO companion_personas (id) VALUES (?)').run('persona_1');
    database.prepare('INSERT INTO companion_personas (id) VALUES (?)').run('persona_2');
    return database;
}

function conversationInput(overrides = {}) {
    return {
        personaId: 'persona_1', id: 'conversation_1',
        createdAt: '2026-08-20T00:00:00.000Z',
        updatedAt: '2026-08-20T00:00:00.000Z', ...overrides
    };
}

function messageInput(overrides = {}) {
    return {
        id: 'message_1', conversationId: 'conversation_1', role: 'user', text: 'hello',
        attachmentsJson: '[]', generationJson: null, jobsJson: '[]',
        proactiveEventId: null, proactivePendingEventId: null,
        createdAt: '2026-08-20T00:00:01.000Z', readAt: '2026-08-20T00:00:01.000Z', ...overrides
    };
}

test('conversation repository requires an already-open database', () => {
    assert.throws(() => createConversationRepository(), /open database/);
});

test('get-or-create is idempotent and isolates personas', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        const first = repository.getOrCreateConversation(conversationInput());
        const second = repository.getOrCreateConversation(conversationInput({id: 'conversation_other', createdAt: '2026-08-20T01:00:00.000Z'}));
        const other = repository.getOrCreateConversation(conversationInput({personaId: 'persona_2', id: 'conversation_2'}));
        assert.equal(first.id, 'conversation_1');
        assert.equal(second.id, first.id);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get('persona_1').count, 1);
        assert.equal(other.persona_id, 'persona_2');
    } finally {
        database.close();
    }
});

test('message insertion and conversation timestamp update stay table-scoped', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        const inserted = repository.appendMessage(messageInput());
        repository.updateConversationTimestamp({conversationId: 'conversation_1', updatedAt: '2026-08-20T00:00:02.000Z'});
        assert.equal(inserted.text, 'hello');
        assert.equal(database.prepare('SELECT updated_at FROM companion_conversations WHERE id = ?').get('conversation_1').updated_at, '2026-08-20T00:00:02.000Z');
        assert.equal(repository.listMessages({personaId: 'persona_2', limit: 5}).length, 0);
    } finally {
        database.close();
    }
});

test('findMessage enforces persona ownership while returning the raw message row', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        repository.getOrCreateConversation(conversationInput({personaId: 'persona_2', id: 'conversation_2'}));
        repository.appendMessage(messageInput());
        assert.equal(repository.findMessage({id: 'message_1', personaId: 'persona_1'}).persona_id, 'persona_1');
        assert.equal(repository.findMessage({id: 'message_1', personaId: 'persona_2'}), undefined);
    } finally {
        database.close();
    }
});

test('batch append preserves input order and message-owned fields while updating once', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        database.exec(`
            CREATE TABLE conversation_timestamp_updates (conversation_id TEXT, updated_at TEXT);
            CREATE TRIGGER conversation_timestamp_update_log
            AFTER UPDATE OF updated_at ON companion_conversations
            BEGIN
                INSERT INTO conversation_timestamp_updates (conversation_id, updated_at) VALUES (NEW.id, NEW.updated_at);
            END;
        `);
        const inserted = repository.appendMessages({
            conversationId: 'conversation_1',
            updatedAt: '2026-08-20T00:00:03.000Z',
            messages: [
                messageInput({
                    id: 'assistant_1', role: 'assistant', text: 'first', createdAt: '2026-08-20T00:00:01.000Z', readAt: null,
                    generationJson: JSON.stringify({status: 'queued'}), jobsJson: JSON.stringify([{id: 'job_1'}]),
                    proactiveEventId: 'event_1', proactivePendingEventId: 'pending_1'
                }),
                messageInput({
                    id: 'assistant_2', role: 'assistant', text: 'second', createdAt: '2026-08-20T00:00:02.000Z', readAt: '2026-08-20T00:00:02.000Z',
                    generationJson: JSON.stringify({status: 'ready'}), jobsJson: JSON.stringify([{id: 'job_2'}]),
                    proactiveEventId: 'event_2', proactivePendingEventId: 'pending_2'
                })
            ]
        });

        assert.deepEqual(inserted.map(row => row.id), ['assistant_1', 'assistant_2']);
        assert.equal(inserted[0].generation_json, JSON.stringify({status: 'queued'}));
        assert.equal(inserted[0].jobs_json, JSON.stringify([{id: 'job_1'}]));
        assert.equal(inserted[0].proactive_event_id, 'event_1');
        assert.equal(inserted[0].proactive_pending_event_id, 'pending_1');
        assert.equal(database.prepare('SELECT updated_at FROM companion_conversations WHERE id = ?').get('conversation_1').updated_at, '2026-08-20T00:00:03.000Z');
        assert.deepEqual(database.prepare('SELECT conversation_id, updated_at FROM conversation_timestamp_updates').all(), [{conversation_id: 'conversation_1', updated_at: '2026-08-20T00:00:03.000Z'}]);
    } finally {
        database.close();
    }
});

test('message pages use descending cursor order while returning rows oldest-first in the server', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        for (const [id, createdAt] of [['message_1', '2026-08-20T00:00:01.000Z'], ['message_2', '2026-08-20T00:00:02.000Z'], ['message_3', '2026-08-20T00:00:03.000Z']]) {
            repository.appendMessage(messageInput({id, createdAt, readAt: null, text: id}));
        }
        const firstPage = repository.listMessages({personaId: 'persona_1', limit: 2});
        assert.deepEqual(firstPage.map(row => row.id), ['message_3', 'message_2']);
        const serverItems = [...firstPage].reverse();
        assert.deepEqual(serverItems.map(row => row.id), ['message_2', 'message_3']);
        const secondPage = repository.listMessages({personaId: 'persona_1', cursor: {createdAt: firstPage.at(-1).created_at, id: firstPage.at(-1).id}, limit: 2});
        assert.deepEqual(secondPage.map(row => row.id), ['message_1']);
        assert.equal(repository.listMessages({personaId: 'persona_2', limit: 2}).length, 0);
    } finally {
        database.close();
    }
});

test('read marking stays caller-controlled, persona-scoped, and foreign-key protected', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        repository.getOrCreateConversation(conversationInput({personaId: 'persona_2', id: 'conversation_2'}));
        repository.appendMessage(messageInput({id: 'assistant_1', role: 'assistant', readAt: null}));
        repository.appendMessage(messageInput({id: 'user_1', role: 'user', readAt: null}));
        repository.appendMessage(messageInput({id: 'assistant_2', conversationId: 'conversation_2', role: 'assistant', readAt: null}));

        const readAt = '2026-08-20T00:00:10.000Z';
        const result = repository.updateReadAt({personaId: 'persona_1', role: 'assistant', readAt});
        assert.equal(result.changes, 1);
        assert.equal(database.prepare('SELECT read_at FROM companion_messages WHERE id = ?').get('assistant_1').read_at, readAt);
        assert.equal(database.prepare('SELECT read_at FROM companion_messages WHERE id = ?').get('user_1').read_at, null);
        assert.equal(database.prepare('SELECT read_at FROM companion_messages WHERE id = ?').get('assistant_2').read_at, null);

        assert.throws(() => repository.appendMessage(messageInput({id: 'orphan', conversationId: 'missing_conversation'})), /FOREIGN KEY/);
        assert.throws(() => repository.getOrCreateConversation(conversationInput({personaId: 'missing_persona', id: 'conversation_missing'})), /FOREIGN KEY/);
    } finally {
        database.close();
    }
});

test('caller transaction rolls back a failed message insert and timestamp update', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        assert.throws(() => database.transaction(() => {
            repository.appendMessage(messageInput({id: 'message_1'}));
            repository.updateConversationTimestamp({conversationId: 'conversation_1', updatedAt: '2026-08-20T00:00:02.000Z'});
            repository.appendMessage(messageInput({id: 'message_1', text: 'duplicate'}));
        })(), /UNIQUE constraint failed/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE conversation_id = ?').get('conversation_1').count, 0);
        assert.equal(database.prepare('SELECT updated_at FROM companion_conversations WHERE id = ?').get('conversation_1').updated_at, '2026-08-20T00:00:00.000Z');
    } finally {
        database.close();
    }
});

test('batch append rolls back every row and its timestamp when a later insert fails', () => {
    const database = createDatabase();
    try {
        const repository = createConversationRepository({database});
        repository.getOrCreateConversation(conversationInput());
        assert.throws(() => database.transaction(() => repository.appendMessages({
            conversationId: 'conversation_1',
            updatedAt: '2026-08-20T00:00:03.000Z',
            messages: [
                messageInput({id: 'assistant_batch_1', role: 'assistant', text: 'first', readAt: null}),
                messageInput({id: 'assistant_batch_1', role: 'assistant', text: 'duplicate', readAt: null})
            ]
        }))(), /UNIQUE constraint failed/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE conversation_id = ?').get('conversation_1').count, 0);
        assert.equal(database.prepare('SELECT updated_at FROM companion_conversations WHERE id = ?').get('conversation_1').updated_at, '2026-08-20T00:00:00.000Z');
    } finally {
        database.close();
    }
});
