import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {createActivityService} from '../server/application/activity-service.js';

const NOW = '2026-08-21T00:00:00.000Z';

function fixture() {
    const calls = [];
    const personas = new Map([
        ['persona_1', {
            id: 'persona_1', name: 'Lin', role: 'tester', color: '#123456', group_id: 'group_1', group_name: 'Default',
            screened_at: null, situation: '在窗边', mood: '平静', unread_count: 2, updated_at: NOW
        }],
        ['persona_screened', {
            id: 'persona_screened', name: 'Screened', role: 'tester', color: '#654321',
            screened_at: '2026-08-20T00:00:02.500Z', situation: '', mood: '', unread_count: 0, updated_at: NOW
        }]
    ]);
    const activityRows = [
        {id: 'activity_old', persona_id: 'persona_1', content: 'Old', media_mode: 'none', media_status: 'none', created_at: '2026-08-20T00:00:01.000Z'},
        {id: 'activity_new', persona_id: 'persona_1', content: 'New', media_mode: 'image_set', media_status: 'ready', created_at: '2026-08-20T00:00:03.000Z'},
        {id: 'activity_screened', persona_id: 'persona_screened', content: 'Hidden by screen', media_mode: 'none', media_status: 'none', created_at: '2026-08-20T00:00:04.000Z'},
        {id: 'activity_hidden', persona_id: 'persona_1', content: 'Hidden', media_mode: 'none', media_status: 'none', created_at: '2026-08-20T00:00:05.000Z', hidden_at: NOW}
    ];
    const comments = new Map([
        ['activity_new', [{id: 'comment_1', activity_id: 'activity_new', author_kind: 'user', content: 'Nice.', created_at: NOW}]]
    ]);
    const mediaLinks = new Map([
        ['activity_new', [{activity_id: 'activity_new', media_id: 'asset_1', position: 0}]]
    ]);
    const assets = new Map([['asset_1', {id: 'asset_1', media_kind: 'image'}]]);
    const liked = new Set();
    const activity = {
        listActivities(input) {
            calls.push(['list', input]);
            const filtered = activityRows
                .filter(row => !input.personaId || row.persona_id === input.personaId)
                .filter(row => input.visibility === 'hidden' ? row.hidden_at : !row.hidden_at)
                .sort((left, right) => right.created_at.localeCompare(left.created_at) || right.id.localeCompare(left.id))
                .filter(row => !input.cursor || row.created_at < input.cursor.createdAt || (row.created_at === input.cursor.createdAt && row.id < input.cursor.id));
            return filtered.slice(0, input.limit);
        },
        findActivity({activityId, personaId}) {
            const row = activityRows.find(candidate => candidate.id === activityId);
            return row && (!personaId || row.persona_id === personaId) ? row : null;
        },
        listActivityComments(input) { return comments.get(input.activityId) || []; },
        insertActivityComment(input) {
            calls.push(['comment', input]);
            const row = {...input, activity_id: input.activityId, author_kind: input.authorKind, created_at: input.createdAt};
            comments.set(input.activityId, [...(comments.get(input.activityId) || []), row]);
            return row;
        },
        listActivityMedia(input) { return mediaLinks.get(input.activityId) || []; },
        getUserReaction({activityId}) { return liked.has(activityId); },
        setUserReaction(input) {
            calls.push(['like', input]);
            if (input.liked) liked.add(input.activityId); else liked.delete(input.activityId);
        },
        setActivityVisibility(input) {
            calls.push(['visibility', input]);
            const row = activityRows.find(candidate => candidate.id === input.activityId);
            if (row) row.hidden_at = input.hidden ? input.hiddenAt : null;
        }
    };
    const memory = {
        insert(input) { calls.push(['memory', input]); return input; }
    };
    const settings = {
        value: {model: 'fixture'},
        read() { return this.value; },
        write(value) { calls.push(['settings', value]); this.value = value; return value; }
    };
    const service = createActivityService({
        activity,
        memory,
        settings,
        media: {findAsset(id) { return assets.get(id); }},
        persona: {findActive(id) { return personas.get(id); }},
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_fixture`
    });
    return {service, calls, activity, settings};
}

test('activity service returns browser DTOs, applies screened visibility, and encodes stable cursors', () => {
    const {service, calls} = fixture();
    const first = service.list({limit: 1});
    assert.deepEqual(first.items.map(item => item.id), ['activity_new']);
    assert.ok(first.nextCursor);
    assert.deepEqual(first.items[0].persona, {
        id: 'persona_1', name: 'Lin', role: 'tester', color: '#123456', groupId: 'group_1', groupName: 'Default',
        screened: false, currentSituation: '在窗边', mood: '平静', unreadCount: 2, updatedAt: NOW
    });
    assert.equal(first.items[0].comments[0].authorName, '我');
    assert.equal(first.items[0].liked, false);
    assert.deepEqual(first.items[0].media, [{id: 'asset_1', kind: 'image', url: '/api/companion/media/asset_1'}]);

    const second = service.list({cursor: first.nextCursor, limit: 1});
    assert.deepEqual(second.items.map(item => item.id), ['activity_old']);
    assert.equal(second.nextCursor, null);
    assert.equal(calls.filter(([name]) => name === 'list').some(([, input]) => input.visibility === 'all'), false);
});

test('activity service supports hidden feed and rejects malformed cursors', () => {
    const {service} = fixture();
    assert.deepEqual(service.list({visibility: 'hidden'}).items.map(item => item.id), ['activity_hidden']);
    assert.throws(() => service.list({cursor: 'not-a-cursor'}), error => error.status === 400 && /游标/.test(error.message));
    assert.throws(() => service.list({visibility: 'unknown'}), /visibility/);
});

test('comments persist activity row and memory evidence through injected ports', () => {
    const {service, calls} = fixture();
    const result = service.comment({activityId: 'activity_new', content: '  Remember this.  '});
    assert.equal(result.content, 'Remember this.');
    assert.equal(result.authorKind, 'user');
    assert.deepEqual(calls.find(([name]) => name === 'memory')[1], {
        id: 'memory_fixture', personaId: 'persona_1', memoryKey: '动态互动', value: 'Remember this.', confidence: 0.7,
        status: 'active', sourceType: 'activity_comment', sourceId: 'comment_fixture', createdAt: NOW, updatedAt: NOW
    });
    assert.throws(() => service.comment({activityId: 'activity_new', content: '   '}), /must not be empty/);
    assert.throws(() => service.comment({activityId: 'missing', content: 'x'}), error => error.status === 404);
});

test('like, hide/restore, and mark-read preserve narrow application result shapes', () => {
    const {service, settings} = fixture();
    assert.deepEqual(service.like({activityId: 'activity_new', liked: true}), {liked: true});
    assert.equal(service.list({personaId: 'persona_1', limit: 1}).items[0].liked, true);
    assert.deepEqual(service.hide({activityId: 'activity_new', hidden: true}), {hidden: true});
    assert.equal(service.list({personaId: 'persona_1'}).items.some(item => item.id === 'activity_new'), false);
    assert.deepEqual(service.hide({activityId: 'activity_new', hidden: false}), {hidden: false});
    assert.equal(service.markRead(), undefined);
    assert.equal(settings.value.activityReadAt, NOW);
    assert.throws(() => service.like({activityId: 'activity_new', liked: 'yes'}), /boolean/);
    assert.throws(() => service.hide({activityId: 'activity_new', hidden: 'yes'}), /boolean/);
});

test('activity service is transport and storage independent', async () => {
    const source = await readFile(new URL('../server/application/activity-service.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(|child_process/);
});
