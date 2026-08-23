import assert from 'node:assert/strict';
import test from 'node:test';

import {createConversationService} from '../server/application/conversation-service.js';

test('conversation service owns cursor encoding, chronological DTOs, and read marking', () => {
    const readMarks = [];
    const repository = {
        getConversation: () => ({id: 'conversation_1'}),
        listMessages: ({limit}) => [
            {id: 'message_2', role: 'assistant', text: 'two', created_at: '2026-08-21T00:00:02.000Z', attachments_json: '[]', jobs_json: '[]'},
            {id: 'message_1', role: 'user', text: 'one', created_at: '2026-08-21T00:00:01.000Z', attachments_json: '[]', jobs_json: '[]'}
        ].slice(0, limit),
        updateReadAt: input => readMarks.push(input)
    };
    const service = createConversationService({repository, clock: () => '2026-08-21T00:00:03.000Z'});
    const result = service.list({personaId: 'persona_1', limit: 2});
    assert.deepEqual(result.items.map(item => item.id), ['message_1', 'message_2']);
    assert.ok(result.nextCursor);
    assert.equal(readMarks.length, 1);
});

test('conversation service rejects malformed cursors and invalid roles', () => {
    const repository = {getConversation: () => null, listMessages() { return []; }};
    const service = createConversationService({repository});
    assert.throws(() => service.list({personaId: 'p1', cursor: 'bad'}), /会话游标无效/);
    assert.throws(() => service.appendMessage({personaId: 'p1', role: 'system'}), /消息角色无效/);
});

test('conversation DTO restores the canonical media URL for legacy id-only attachments', () => {
    const repository = {
        getConversation: () => ({id: 'conversation_1'}),
        listMessages: () => [{
            id: 'message_media', role: 'assistant', text: '', created_at: '2026-08-21T00:00:00.000Z',
            attachments_json: JSON.stringify([{id: 'asset with space', kind: 'image'}]), jobs_json: '[]'
        }]
    };
    const service = createConversationService({repository});
    const [message] = service.list({personaId: 'persona_1', markRead: false}).items;
    assert.equal(message.attachments[0].url, '/api/companion/media/asset%20with%20space');
});
