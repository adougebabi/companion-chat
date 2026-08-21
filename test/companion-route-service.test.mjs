import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {createCompanionRouteService} from '../server/application/companion-route-service.js';

function fixture({lifecycle, assistantAppender} = {}) {
    const rows = new Map([
        ['persona_a', {
            id: 'persona_a', name: 'A', role: 'tester', color: '#ffffff', group_id: 'group_a',
            screened_at: null, situation: '在窗边', mood: '平静', unread_count: 2, updated_at: 'before'
        }],
        ['persona_b', {
            id: 'persona_b', name: 'B', role: 'tester', color: '#000000', group_id: 'group_default',
            screened_at: null, situation: '在路上', mood: '轻快', unread_count: 0, updated_at: 'before'
        }]
    ]);
    const groupRows = new Map([
        ['group_default', {id: 'group_default', name: 'Default', is_default: 1, persona_count: 1}],
        ['group_a', {id: 'group_a', name: '工作', is_default: 0, persona_count: 1}]
    ]);
    const calls = [];
    const persona = {
        findActive(id) {
            calls.push({method: 'findActive', id});
            return rows.get(id);
        },
        listActive() {
            return [...rows.values()];
        },
        updateScreen({personaId, screenedAt, updatedAt}) {
            calls.push({method: 'updateScreen', personaId, screenedAt, updatedAt});
            const row = rows.get(personaId);
            if (!row) return undefined;
            row.screened_at = screenedAt;
            row.updated_at = updatedAt;
            return row;
        }
    };
    const group = {
        find(id) {
            calls.push({method: 'findGroup', id});
            return groupRows.get(id);
        },
        create({name}) {
            calls.push({method: 'createGroup', name});
            if ([...groupRows.values()].some(row => row.name === name)) {
                throw Object.assign(new Error('duplicate group'), {code: 'SQLITE_CONSTRAINT_UNIQUE'});
            }
            const row = {id: 'group_new', name, is_default: 0, persona_count: 0};
            groupRows.set(row.id, row);
            return row;
        },
        assignPersona({personaId, groupId, updatedAt}) {
            calls.push({method: 'assignPersona', personaId, groupId, updatedAt});
            const row = rows.get(personaId);
            if (!row) return undefined;
            row.group_id = groupId;
            row.updated_at = updatedAt;
            return row;
        }
    };
    const conversationRepository = {
        updateReadAt(input) {
            calls.push({method: 'updateReadAt', ...input});
        }
    };
    const conversation = {
        list(input) {
            calls.push({method: 'listConversation', input});
            return {items: [{id: 'message_a', role: 'user', text: 'hello'}], nextCursor: 'cursor_next'};
        },
        appendMessage(input) {
            calls.push({method: 'appendMessage', input});
            return {id: 'message_new', role: input.role, text: input.text};
        },
        ...(assistantAppender ? {appendUserVisibleAssistantReply: assistantAppender} : {})
    };
    const identity = {
        bootstrap: {
            read() {
                return {
                    personas: [...rows.values()].map(row => ({
                        id: row.id, name: row.name, role: row.role, color: row.color,
                        groupId: row.group_id, groupName: groupRows.get(row.group_id)?.name ?? null,
                        screened: Boolean(row.screened_at), currentSituation: row.situation,
                        mood: row.mood, unreadCount: row.screened_at ? 0 : row.unread_count, updatedAt: row.updated_at
                    }))
                };
            }
        }
    };
    const service = createCompanionRouteService({
        repositories: {persona, group, conversation: conversationRepository},
        services: {identity},
        adapters: lifecycle ? {personaLifecycle: lifecycle} : {},
        conversationService: conversation,
        clock: () => '2026-08-21T00:00:00.000Z'
    });
    return {service, calls, rows};
}

test('group create and persona assignment validate input, map DTOs, and isolate missing owners', () => {
    const {service, calls} = fixture();
    assert.deepEqual(service.group.create({name: '  新组  '}), {
        id: 'group_new', name: '新组', isDefault: false, personaCount: 0
    });
    assert.throws(() => service.group.create({name: '新组'}), error => error.status === 400 && /已存在/.test(error.message));
    assert.throws(() => service.persona.assignGroup({personaId: 'missing', groupId: 'group_a'}), error => error.status === 404);
    assert.throws(() => service.persona.assignGroup({personaId: 'persona_a', groupId: 'missing'}), error => error.status === 404);
    assert.equal(calls.some(call => call.method === 'assignPersona' && call.personaId === 'missing'), false);

    const assigned = service.persona.assignGroup({personaId: 'persona_a', groupId: 'group_default'});
    assert.equal(assigned.groupId, 'group_default');
    assert.equal(assigned.groupName, 'Default');
    assert.equal(assigned.unreadCount, 2);
});

test('screening updates only the selected persona and marks assistant messages read when enabled', () => {
    const {service, calls, rows} = fixture();
    const screened = service.persona.screen({personaId: 'persona_a', screened: true});
    assert.equal(screened.screened, true);
    assert.equal(screened.unreadCount, 0);
    assert.deepEqual(calls.filter(call => call.method === 'updateReadAt').at(-1), {
        method: 'updateReadAt', personaId: 'persona_a', role: 'assistant',
        readAt: '2026-08-21T00:00:00.000Z'
    });
    assert.equal(rows.get('persona_b').screened_at, null);

    service.persona.screen({personaId: 'persona_a', screened: false});
    assert.equal(rows.get('persona_a').screened_at, null);
    assert.equal(calls.filter(call => call.method === 'updateReadAt').length, 1);
    assert.throws(() => service.persona.screen({personaId: 'persona_a', screened: 'true'}), error => error.status === 400);
});

test('conversation list and append enforce persona scope and preserve cursor/read DTO semantics', () => {
    const {service, calls} = fixture({
        assistantAppender(personaId, text) {
            calls.push({method: 'assistantAppender', personaId, text});
            return [
                {id: 'assistant_1', role: 'assistant', text: `${text}。`},
                {id: 'assistant_2', role: 'assistant', text: '还有一句。'}
            ];
        }
    });
    const page = service.conversations.list({persona_id: 'persona_a', nextCursor: 'cursor_prev', limit: 1});
    assert.deepEqual(page, {items: [{id: 'message_a', role: 'user', text: 'hello'}], nextCursor: 'cursor_next'});
    assert.deepEqual(calls.find(call => call.method === 'listConversation').input, {
        persona_id: 'persona_a', nextCursor: 'cursor_prev', limit: 1,
        personaId: 'persona_a', cursor: 'cursor_prev'
    });

    const userMessage = service.conversations.appendMessage({personaId: 'persona_a', role: 'user', text: 'hi'});
    assert.deepEqual(userMessage, {id: 'message_new', role: 'user', text: 'hi'});
    const assistantMessages = service.conversations.appendMessage({personaId: 'persona_a', role: 'assistant', text: 'one'});
    assert.deepEqual(assistantMessages, {
        message: {id: 'assistant_1', role: 'assistant', text: 'one。'},
        messages: [
            {id: 'assistant_1', role: 'assistant', text: 'one。'},
            {id: 'assistant_2', role: 'assistant', text: '还有一句。'}
        ]
    });
    assert.throws(() => service.conversations.list({personaId: 'missing'}), error => error.status === 404);
    assert.throws(() => service.conversations.appendMessage({personaId: 'persona_a', role: 'system', text: 'bad'}), error => error.status === 400);
    assert.equal(calls.some(call => call.method === 'appendMessage' && call.input.personaId === 'missing'), false);
});

test('persona lifecycle operations use an explicit adapter and do not fake foundation/life behavior', () => {
    const lifecycleCalls = [];
    const {service} = fixture({
        lifecycle: {
            createPersona(input) {
                lifecycleCalls.push({method: 'createPersona', input});
                return {id: 'persona_new', name: input.name, role: input.role, foundation: input.foundation ?? 'adapter default'};
            },
            deletePersona(input) {
                lifecycleCalls.push({method: 'deletePersona', input});
                return {id: input.personaId, deleted: true, deletedMediaIds: ['media_1']};
            },
            getPersona(input) {
                lifecycleCalls.push({method: 'getPersona', input});
                return {persona: {id: input.personaId}};
            }
        }
    });
    assert.deepEqual(service.persona.create({name: 'New', role: 'companion', foundation: 'Stable'}), {
        id: 'persona_new', name: 'New', role: 'companion', foundation: 'Stable'
    });
    assert.deepEqual(service.persona.create({name: 'Derived', role: 'companion'}), {
        id: 'persona_new', name: 'Derived', role: 'companion', foundation: 'adapter default'
    });
    assert.deepEqual(service.persona.delete({personaId: 'persona_a'}), {
        id: 'persona_a', deleted: true, deletedMediaIds: ['media_1']
    });
    assert.deepEqual(service.persona.get({personaId: 'persona_a'}), {persona: {id: 'persona_a'}});
    assert.equal(lifecycleCalls.length, 4);
    assert.throws(() => service.persona.create({name: 'Missing role'}), error => error.status === 400);
    assert.throws(() => fixture().service.persona.create({name: 'New', role: 'companion', foundation: 'x'}), error => error.status === 501);
    assert.throws(() => fixture().service.persona.delete({personaId: 'missing'}), error => error.status === 404);
});

test('route service has no legacy root, transport, database, or provider ownership', async () => {
    const source = await readFile(new URL('../server/application/companion-route-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(|child_process|provider/i);
});
