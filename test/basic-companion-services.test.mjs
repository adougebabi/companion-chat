import assert from 'node:assert/strict';
import test from 'node:test';

import {createBasicCompanionServices} from '../server/application/basic-companion-services.js';

test('basic services return repository-backed bootstrap and settings DTOs', () => {
    const calls = [];
    const services = createBasicCompanionServices({
        repositories: {
            persona: {listActive: () => [{id: 'p1', name: 'P1', role: 'tester', color: '#fff', group_id: null, updated_at: 'now'}]},
            group: {list: () => [{id: 'g1', name: 'Default', is_default: 1, persona_count: 1}]},
            conversation: {getConversation: () => null, listMessages: () => [], getOrCreateConversation() {}, appendMessage() {}},
            activity: {listActivities: () => []},
            settings: {read: () => ({model: 'fixture'}), write: value => calls.push(value)}
        },
        providers: {summaries: () => [{id: 'mtplx'}]}
    });
    assert.equal(services.bootstrap.read().personas[0].id, 'p1');
    assert.equal(services.bootstrap.read().groups[0].isDefault, true);
    services.settings.update({model: 'next'});
    assert.deepEqual(calls, [{model: 'next'}]);
    assert.deepEqual(services.models.list(), [{id: 'mtplx'}]);
});

test('basic conversation/activity services preserve wrapper shapes', () => {
    const conversationCalls = [];
    const services = createBasicCompanionServices({
        repositories: {
            persona: {listActive: () => []},
            group: {list: () => []},
            conversation: {
                getConversation: personaId => {
                    conversationCalls.push({method: 'getConversation', personaId});
                    return {id: `conversation_${personaId}`};
                },
                listMessages: input => {
                    conversationCalls.push({method: 'listMessages', input});
                    return [{
                        id: input.cursor ? 'm0' : 'm1', role: 'user', text: 'hello',
                        attachments_json: '[]', jobs_json: '[]',
                        created_at: input.cursor ? '2026-08-20T00:00:00.000Z' : '2026-08-21T00:00:00.000Z'
                    }];
                }
            },
            activity: {listActivities: () => ({items: [{id: 'a1'}], cursor: 'activity_cursor'})},
            settings: {read: () => ({})}
        }
    });
    const firstPage = services.conversations.list({persona_id: 'p1', limit: 1});
    assert.deepEqual(firstPage.items.map(item => item.id), ['m1']);
    assert.ok(firstPage.nextCursor);
    const secondPage = services.conversations.list({personaId: 'p1', nextCursor: firstPage.nextCursor, limit: 1});
    assert.deepEqual(secondPage.items.map(item => item.id), ['m0']);
    assert.deepEqual(conversationCalls, [
        {method: 'getConversation', personaId: 'p1'},
        {method: 'listMessages', input: {conversationId: 'conversation_p1', cursor: null, limit: 1}},
        {method: 'getConversation', personaId: 'p1'},
        {method: 'listMessages', input: {
            conversationId: 'conversation_p1',
            cursor: {createdAt: '2026-08-21T00:00:00.000Z', id: 'm1'},
            limit: 1
        }}
    ]);
    assert.deepEqual(services.activities.list({}).items, [{id: 'a1'}]);
    assert.equal(services.activities.list({}).nextCursor, 'activity_cursor');
});

test('basic bootstrap maps legacy persona/group fields without inventing persistence reads', () => {
    const services = createBasicCompanionServices({
        repositories: {
            persona: {
                listActive: () => [{
                    id: 'p1', name: 'P1', role: 'tester', color: '#fff', group_id: 'g1', group_name: 'Default',
                    screened_at: null, situation: '在窗边看书', mood: '安静', unread_count: '2', updated_at: 'now'
                }]
            },
            group: {list: () => [{id: 'g1', name: 'Default', is_default: 1, persona_count: '1'}]},
            conversation: {getConversation: () => null, listMessages: () => []},
            activity: {listActivities: () => []},
            settings: {read: () => ({})}
        }
    });

    assert.deepEqual(services.bootstrap.read().personas[0], {
        id: 'p1', name: 'P1', role: 'tester', color: '#fff', groupId: 'g1', groupName: 'Default',
        screened: false, currentSituation: '在窗边看书', mood: '安静', unreadCount: 2, updatedAt: 'now'
    });
    assert.deepEqual(services.bootstrap.read().groups[0], {
        id: 'g1', name: 'Default', isDefault: true, personaCount: 1
    });
});

test('basic services have no legacy or transport imports', async () => {
    const {readFile} = await import('node:fs/promises');
    const source = await readFile(new URL('../server/application/basic-companion-services.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(/);
});
