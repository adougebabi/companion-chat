import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createActivityRepository} from '../server/infrastructure/activity-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.pragma('foreign_keys = ON');
    database.exec(`
        CREATE TABLE companion_personas (id TEXT PRIMARY KEY);
        CREATE TABLE companion_life_events (id TEXT PRIMARY KEY);
        CREATE TABLE companion_activities (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL REFERENCES companion_personas(id),
            event_id TEXT REFERENCES companion_life_events(id),
            content TEXT NOT NULL,
            media_mode TEXT NOT NULL DEFAULT 'none',
            media_status TEXT NOT NULL DEFAULT 'none',
            created_at TEXT NOT NULL
        );
        CREATE INDEX companion_activities_persona_feed_idx
            ON companion_activities(persona_id, created_at DESC, id DESC);
        CREATE TABLE companion_activity_comments (
            id TEXT PRIMARY KEY,
            activity_id TEXT NOT NULL REFERENCES companion_activities(id),
            parent_comment_id TEXT REFERENCES companion_activity_comments(id),
            author_kind TEXT NOT NULL,
            author_persona_id TEXT REFERENCES companion_personas(id),
            supporting_character_id TEXT,
            content TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
        CREATE INDEX companion_activity_comments_activity_idx
            ON companion_activity_comments(activity_id, created_at, id);
        CREATE TABLE companion_activity_reactions (
            activity_id TEXT NOT NULL REFERENCES companion_activities(id),
            actor_kind TEXT NOT NULL,
            supporting_character_id TEXT,
            created_at TEXT NOT NULL
        );
        CREATE UNIQUE INDEX companion_user_reaction_once
            ON companion_activity_reactions(activity_id, actor_kind)
            WHERE actor_kind = 'user';
        CREATE TABLE companion_activity_visibility (
            activity_id TEXT PRIMARY KEY REFERENCES companion_activities(id),
            hidden_at TEXT,
            updated_at TEXT NOT NULL
        );
        CREATE TABLE companion_media_assets (id TEXT PRIMARY KEY);
        CREATE TABLE companion_activity_media (
            activity_id TEXT NOT NULL REFERENCES companion_activities(id),
            media_id TEXT NOT NULL REFERENCES companion_media_assets(id),
            position INTEGER NOT NULL,
            PRIMARY KEY(activity_id, media_id)
        );
    `);
    database.prepare('INSERT INTO companion_personas (id) VALUES (?)').run('persona_1');
    database.prepare('INSERT INTO companion_personas (id) VALUES (?)').run('persona_2');
    return database;
}

function activityInput(overrides = {}) {
    return {
        id: 'activity_1',
        personaId: 'persona_1',
        content: 'A day worth remembering.',
        mediaMode: 'none',
        mediaStatus: 'none',
        createdAt: '2026-08-20T00:00:01.000Z',
        ...overrides
    };
}

function commentInput(overrides = {}) {
    return {
        id: 'comment_1',
        activityId: 'activity_1',
        personaId: 'persona_1',
        authorKind: 'user',
        content: 'This is lovely.',
        createdAt: '2026-08-20T00:00:02.000Z',
        ...overrides
    };
}

test('activity repository requires an already-open database', () => {
    assert.throws(() => createActivityRepository(), /open database/);
});

test('activity feed uses stable descending cursor ordering and persona filters', () => {
    const database = createDatabase();
    try {
        const repository = createActivityRepository({database});
        repository.insertActivity(activityInput({id: 'activity_a', createdAt: '2026-08-20T00:00:01.000Z'}));
        repository.insertActivity(activityInput({id: 'activity_c', createdAt: '2026-08-20T00:00:03.000Z'}));
        repository.insertActivity(activityInput({id: 'activity_b', createdAt: '2026-08-20T00:00:02.000Z'}));
        repository.insertActivity(activityInput({id: 'activity_same_z', createdAt: '2026-08-20T00:00:02.000Z'}));
        repository.insertActivity(activityInput({id: 'other_activity', personaId: 'persona_2', createdAt: '2026-08-20T00:00:04.000Z'}));

        const firstPage = repository.listActivities({personaId: 'persona_1', limit: 2});
        assert.deepEqual(firstPage.map(row => row.id), ['activity_c', 'activity_same_z']);
        const secondPage = repository.listActivities({
            personaId: 'persona_1',
            cursor: {createdAt: firstPage.at(-1).created_at, id: firstPage.at(-1).id},
            limit: 2
        });
        assert.deepEqual(secondPage.map(row => row.id), ['activity_b', 'activity_a']);
        assert.deepEqual(repository.listActivities({personaId: 'persona_2', limit: 10}).map(row => row.id), ['other_activity']);
        assert.deepEqual(repository.listActivities({limit: 10, visibility: 'all'}).map(row => row.id), [
            'other_activity', 'activity_c', 'activity_same_z', 'activity_b', 'activity_a'
        ]);
    } finally {
        database.close();
    }
});

test('activity and comment inserts return raw rows and preserve persona ownership', () => {
    const database = createDatabase();
    try {
        const repository = createActivityRepository({database});
        const activity = repository.insertActivity(activityInput());
        assert.equal(activity.persona_id, 'persona_1');
        assert.equal(activity.media_mode, 'none');

        const comment = repository.insertComment(commentInput());
        assert.equal(comment.author_kind, 'user');
        assert.equal(comment.content, 'This is lovely.');
        assert.deepEqual(repository.listActivityComments({activityId: activity.id, personaId: 'persona_1'}).map(row => row.id), ['comment_1']);

        assert.throws(() => repository.insertComment(commentInput({id: 'cross_persona', personaId: 'persona_2'})), /does not belong/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_comments WHERE id = ?').get('cross_persona').count, 0);

        database.prepare('INSERT INTO companion_media_assets (id) VALUES (?)').run('media_1');
        const mediaLink = repository.insertActivityMedia({activityId: activity.id, personaId: 'persona_1', mediaId: 'media_1', position: 0});
        assert.equal(mediaLink.position, 0);
        assert.equal(repository.listActivityMedia({activityId: activity.id, personaId: 'persona_1'})[0].media_id, 'media_1');
        database.prepare('INSERT INTO companion_media_assets (id) VALUES (?)').run('media_2');
        assert.throws(() => repository.insertActivityMedia({activityId: activity.id, personaId: 'persona_2', mediaId: 'media_2', position: 1}), /does not belong/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_media WHERE media_id = ?').get('media_2').count, 0);
    } finally {
        database.close();
    }
});

test('user reactions are idempotent and persona-scoped', () => {
    const database = createDatabase();
    try {
        const repository = createActivityRepository({database});
        repository.insertActivity(activityInput());

        const first = repository.setUserReaction({
            activityId: 'activity_1', personaId: 'persona_1', liked: true,
            createdAt: '2026-08-20T00:00:03.000Z'
        });
        const second = repository.setUserReaction({
            activityId: 'activity_1', personaId: 'persona_1', liked: true,
            createdAt: '2026-08-20T00:00:04.000Z'
        });
        assert.equal(first.activity_id, 'activity_1');
        assert.equal(second.created_at, '2026-08-20T00:00:04.000Z');
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_activity_reactions WHERE activity_id = ? AND actor_kind = 'user'").get('activity_1').count, 1);

        assert.equal(repository.setUserReaction({
            activityId: 'activity_1', personaId: 'persona_1', liked: false,
            createdAt: '2026-08-20T00:00:05.000Z'
        }), null);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_reactions').get().count, 0);
        assert.throws(() => repository.setUserReaction({
            activityId: 'activity_1', personaId: 'persona_2', liked: true,
            createdAt: '2026-08-20T00:00:06.000Z'
        }), /does not belong/);
    } finally {
        database.close();
    }
});

test('hide and restore update visibility rows without changing activities', () => {
    const database = createDatabase();
    try {
        const repository = createActivityRepository({database});
        const activity = repository.insertActivity(activityInput());
        const hidden = repository.hideActivity({
            activityId: activity.id, personaId: 'persona_1',
            hiddenAt: '2026-08-20T00:00:03.000Z', updatedAt: '2026-08-20T00:00:03.000Z'
        });
        assert.equal(hidden.hidden_at, '2026-08-20T00:00:03.000Z');
        assert.equal(repository.listActivities({personaId: 'persona_1', limit: 10}).length, 0);
        assert.equal(repository.listActivities({personaId: 'persona_1', visibility: 'hidden', limit: 10})[0].id, activity.id);

        const restored = repository.restoreActivity({
            activityId: activity.id, personaId: 'persona_1',
            updatedAt: '2026-08-20T00:00:04.000Z'
        });
        assert.equal(restored.hidden_at, null);
        assert.equal(restored.updated_at, '2026-08-20T00:00:04.000Z');
        assert.equal(repository.listActivities({personaId: 'persona_1', limit: 10})[0].content, activity.content);
        assert.throws(() => repository.hideActivity({
            activityId: activity.id, personaId: 'persona_2',
            hiddenAt: '2026-08-20T00:00:05.000Z', updatedAt: '2026-08-20T00:00:05.000Z'
        }), /does not belong/);
    } finally {
        database.close();
    }
});

test('caller transactions roll back activity and comment writes together', () => {
    const database = createDatabase();
    try {
        const repository = createActivityRepository({database});
        assert.throws(() => database.transaction(() => {
            repository.insertActivity(activityInput());
            repository.insertComment(commentInput());
            repository.insertActivity(activityInput({content: 'duplicate id'}));
        })(), /UNIQUE constraint failed/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activities').get().count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_comments').get().count, 0);

        repository.insertActivity(activityInput());
        assert.throws(() => database.transaction(() => {
            repository.setUserReaction({
                activityId: 'activity_1', personaId: 'persona_1', liked: true,
                createdAt: '2026-08-20T00:00:06.000Z'
            });
            repository.insertComment(commentInput({id: 'comment_2'}));
            repository.insertComment(commentInput({id: 'comment_2', content: 'duplicate id'}));
        })(), /UNIQUE constraint failed/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_reactions').get().count, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activity_comments').get().count, 0);
    } finally {
        database.close();
    }
});
