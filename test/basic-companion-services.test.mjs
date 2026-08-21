import assert from 'node:assert/strict';
import test from 'node:test';

import {createBasicCompanionServices} from '../server/application/basic-companion-services.js';

test('basic services return repository-backed bootstrap and settings DTOs', () => {
    const calls = [];
    const services = createBasicCompanionServices({
        repositories: {
            persona: {listActive: () => [{id: 'p1', name: 'P1', role: 'tester', color: '#fff', group_id: null, updated_at: 'now'}]},
            group: {list: () => [{id: 'g1', name: 'Default', is_default: 1, persona_count: 1}]},
            conversation: {},
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
    const services = createBasicCompanionServices({
        repositories: {
            persona: {listActive: () => []},
            group: {list: () => []},
            conversation: {
                getConversation: personaId => ({id: `conversation_${personaId}`}),
                listMessages: () => [{id: 'm1', role: 'user', text: 'hello', attachments_json: '[]', jobs_json: '[]', created_at: 'now'}]
            },
            activity: {listActivities: () => [{id: 'a1'}]},
            settings: {read: () => ({})}
        }
    });
    assert.deepEqual(services.conversations.list({personaId: 'p1'}).items.map(item => item.id), ['m1']);
    assert.deepEqual(services.activities.list({}).items, [{id: 'a1'}]);
});

test('basic services have no legacy or transport imports', async () => {
    const {readFile} = await import('node:fs/promises');
    const source = await readFile(new URL('../server/application/basic-companion-services.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(/);
});
