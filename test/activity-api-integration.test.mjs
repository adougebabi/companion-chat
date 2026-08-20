import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-activity-api-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';
const {companionApp, companionTestHooks} = await import(`../server.js?activity-api=${Date.now()}`);
const {database, createPersona, deletePersona} = companionTestHooks;

function invokeRoute(path, method, {params = {}, query = {}, body} = {}) {
    const layer = (companionApp.router?.stack || []).find(item => item.route?.path === path && item.route.methods?.[method.toLowerCase()]);
    assert.ok(layer, `${method} ${path} route is registered`);
    const response = {
        statusCode: 200, body: undefined, headersSent: false,
        status(code) { this.statusCode = code; return this; },
        json(value) { this.body = value; this.headersSent = true; return this; },
        end() { this.headersSent = true; return this; }
    };
    layer.route.stack[0].handle({params, query, body}, response);
    return response;
}

function insertActivity({id, personaId, content, createdAt}) {
    database.prepare(`
        INSERT INTO companion_activities (id, persona_id, content, media_mode, media_status, created_at)
        VALUES (?, ?, ?, 'none', 'none', ?)
    `).run(id, personaId, content, createdAt);
}

test('activity API keeps cursor, ownership, visibility, interactions, and read behavior', () => {
    const persona = createPersona({name: 'Activity API', role: 'Tester', foundation: 'Activity API integration fixture.'});
    const screenedPersona = createPersona({name: 'Screened Activity API', role: 'Tester', foundation: 'Screened activity fixture.'});
    try {
        insertActivity({id: 'activity_old', personaId: persona.id, content: 'Old', createdAt: '2026-08-20T00:00:01.000Z'});
        insertActivity({id: 'activity_mid', personaId: persona.id, content: 'Middle', createdAt: '2026-08-20T00:00:02.000Z'});
        insertActivity({id: 'activity_new', personaId: persona.id, content: 'New', createdAt: '2026-08-20T00:00:03.000Z'});
        insertActivity({id: 'activity_screened', personaId: screenedPersona.id, content: 'Screened', createdAt: '2026-08-20T00:00:04.000Z'});
        database.prepare('UPDATE companion_personas SET screened_at = ? WHERE id = ?').run('2026-08-20T00:00:03.500Z', screenedPersona.id);

        const firstPage = invokeRoute('/api/companion/activities', 'GET', {query: {personaId: persona.id, limit: '2'}});
        assert.equal(firstPage.statusCode, 200);
        assert.deepEqual(firstPage.body.items.map(item => item.id), ['activity_new', 'activity_mid']);
        assert.ok(firstPage.body.nextCursor);

        const secondPage = invokeRoute('/api/companion/activities', 'GET', {query: {personaId: persona.id, cursor: firstPage.body.nextCursor, limit: '2'}});
        assert.deepEqual(secondPage.body.items.map(item => item.id), ['activity_old']);
        assert.equal(secondPage.body.nextCursor, null);

        const allVisible = invokeRoute('/api/companion/activities', 'GET', {query: {limit: '20'}});
        assert.equal(allVisible.body.items.some(item => item.id === 'activity_screened'), false);

        const comment = invokeRoute('/api/companion/activities/:activityId/comments', 'POST', {
            params: {activityId: 'activity_new'}, body: {content: '  Nice moment.  '}
        });
        assert.equal(comment.statusCode, 201);
        assert.equal(comment.body.content, 'Nice moment.');
        assert.equal(database.prepare("SELECT persona_id FROM companion_memories WHERE source_id = ?").get(comment.body.id).persona_id, persona.id);
        const invalidComment = invokeRoute('/api/companion/activities/:activityId/comments', 'POST', {
            params: {activityId: 'activity_new'}, body: {content: {unexpected: true}}
        });
        assert.equal(invalidComment.statusCode, 400);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_comments WHERE activity_id = ?').get('activity_new').count, 1);

        const liked = invokeRoute('/api/companion/activities/:activityId/like', 'PUT', {
            params: {activityId: 'activity_new'}, body: {liked: true}
        });
        assert.deepEqual(liked.body, {liked: true});
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_activity_reactions WHERE activity_id = 'activity_new' AND actor_kind = 'user'").get().count, 1);

        const hidden = invokeRoute('/api/companion/activities/:activityId/hide', 'PUT', {
            params: {activityId: 'activity_new'}, body: {hidden: true}
        });
        assert.deepEqual(hidden.body, {hidden: true});
        assert.equal(invokeRoute('/api/companion/activities', 'GET', {query: {personaId: persona.id}}).body.items.some(item => item.id === 'activity_new'), false);
        assert.equal(invokeRoute('/api/companion/activities', 'GET', {query: {personaId: persona.id, visibility: 'hidden'}}).body.items[0].id, 'activity_new');

        const restored = invokeRoute('/api/companion/activities/:activityId/hide', 'PUT', {
            params: {activityId: 'activity_new'}, body: {hidden: false}
        });
        assert.deepEqual(restored.body, {hidden: false});

        database.prepare(`
            INSERT INTO companion_media_assets (id, provider, media_kind, filename, subfolder, file_type, locator_json, created_at)
            VALUES ('activity_media_asset', 'test', 'image', 'activity.png', '', 'output', '{}', ?)
        `).run('2026-08-20T00:00:05.000Z');
        database.prepare('INSERT INTO companion_activity_media (activity_id, media_id, position) VALUES (?, ?, ?)').run('activity_new', 'activity_media_asset', 0);

        const read = invokeRoute('/api/companion/activities/read', 'POST');
        assert.equal(read.statusCode, 204);
        assert.equal(invokeRoute('/api/companion/bootstrap', 'GET').body.activityUnread, false);

        const shaped = invokeRoute('/api/companion/activities', 'GET', {query: {personaId: persona.id, limit: '1'}}).body.items[0];
        assert.equal(shaped.id, 'activity_new');
        assert.equal(shaped.comments[0].content, 'Nice moment.');
        assert.equal(shaped.liked, true);
        assert.deepEqual(shaped.media, [{id: 'activity_media_asset', kind: 'image', url: '/api/companion/media/activity_media_asset'}]);
    } finally {
        deletePersona(persona.id);
        deletePersona(screenedPersona.id);
        database.close();
        rmSync(dataDir, {recursive: true, force: true});
    }
});
