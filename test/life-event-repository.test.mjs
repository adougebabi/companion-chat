import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createLifeEventRepository} from '../server/infrastructure/life-event-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_life_events (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL,
            type TEXT NOT NULL,
            occurred_at TEXT NOT NULL,
            resolves_at TEXT,
            causation_id TEXT,
            payload_json TEXT NOT NULL,
            created_at TEXT NOT NULL
        );
        CREATE TABLE companion_persona_states (
            persona_id TEXT PRIMARY KEY,
            situation TEXT NOT NULL DEFAULT '',
            mood TEXT NOT NULL DEFAULT '',
            appearance_json TEXT NOT NULL DEFAULT '{}',
            checkpoint_at TEXT NOT NULL,
            updated_at TEXT NOT NULL,
            source_event_id TEXT
        );
    `);
    database.prepare(`
        INSERT INTO companion_persona_states
            (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at, source_event_id)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `).run('persona_a', '原来的状态', '平静', '{}', '2026-08-20T00:00:00.000Z', '2026-08-20T00:00:00.000Z', null);
    return database;
}

function repositoryFor(database, overrides = {}) {
    return createLifeEventRepository({
        database,
        clock: () => '2026-08-20T01:02:03.000Z',
        id: prefix => `${prefix}_generated`,
        ...overrides
    });
}

test('life-event repository requires an already-open database', () => {
    assert.throws(() => createLifeEventRepository(), /open database/);
});

test('createEvent uses injected id/clock and returns the raw event row', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const event = repository.createEvent({
            personaId: 'persona_a',
            type: 'social',
            payload: {situation: '在公园散步'},
            occurredAt: '2026-08-20T01:00:00.000Z'
        });

        assert.deepEqual(event, {
            id: 'event_generated',
            persona_id: 'persona_a',
            type: 'social',
            occurred_at: '2026-08-20T01:00:00.000Z',
            resolves_at: null,
            causation_id: null,
            payload_json: '{"situation":"在公园散步"}',
            created_at: '2026-08-20T01:02:03.000Z'
        });
        assert.deepEqual(repository.findById({eventId: event.id, personaId: 'persona_a'}), event);
        assert.equal(repository.findById({eventId: event.id, personaId: 'persona_b'}), undefined);
    } finally {
        database.close();
    }
});

test('listActive is persona-scoped, time-aware, ordered, and limited', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        repository.insertEvent({id: 'event_old', personaId: 'persona_a', type: 'old', occurredAt: '2026-08-19T23:00:00.000Z', resolvesAt: '2026-08-20T00:30:00.000Z', payloadJson: '{}', createdAt: '2026-08-19T23:00:00.000Z'});
        repository.insertEvent({id: 'event_active_a', personaId: 'persona_a', type: 'active', occurredAt: '2026-08-20T00:30:00.000Z', resolvesAt: '2026-08-20T02:00:00.000Z', payloadJson: '{}', createdAt: '2026-08-20T00:30:00.000Z'});
        repository.insertEvent({id: 'event_open', personaId: 'persona_a', type: 'open', occurredAt: '2026-08-20T00:45:00.000Z', payloadJson: '{}', createdAt: '2026-08-20T00:45:00.000Z'});
        repository.insertEvent({id: 'event_future', personaId: 'persona_a', type: 'future', occurredAt: '2026-08-20T03:00:00.000Z', payloadJson: '{}', createdAt: '2026-08-20T03:00:00.000Z'});
        repository.insertEvent({id: 'event_other', personaId: 'persona_b', type: 'other', occurredAt: '2026-08-20T00:50:00.000Z', payloadJson: '{}', createdAt: '2026-08-20T00:50:00.000Z'});

        assert.deepEqual(repository.listActive({personaId: 'persona_a', at: '2026-08-20T01:00:00.000Z'}).map(row => row.id), ['event_open', 'event_active_a']);
        assert.deepEqual(repository.listActive({personaId: 'persona_a', at: '2026-08-20T01:00:00.000Z', limit: 1}).map(row => row.id), ['event_open']);
        assert.deepEqual(repository.list({personaId: 'persona_a'}).map(row => row.id), ['event_future', 'event_open', 'event_active_a', 'event_old']);
    } finally {
        database.close();
    }
});

test('updateEvent and updateState are guarded, parameterized raw-row updates', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        repository.insertEvent({id: 'event_update', personaId: 'persona_a', type: 'routine', payload: {before: true}, createdAt: '2026-08-20T00:00:00.000Z'});
        const event = repository.updateEvent({eventId: 'event_update', personaId: 'persona_a', type: 'recovery', resolvesAt: new Date('2026-08-20T02:00:00.000Z'), payload: {after: true}});
        assert.equal(event.type, 'recovery');
        assert.equal(event.resolves_at, '2026-08-20T02:00:00.000Z');
        assert.equal(event.payload_json, '{"after":true}');
        assert.equal(repository.updateEvent({eventId: 'event_update', personaId: 'persona_b', type: 'wrong'}), undefined);

        const state = repository.updateState({personaId: 'persona_a', situation: '正在恢复', mood: '安心', appearanceJson: {coat: '蓝色'}, checkpointAt: '2026-08-20T02:00:00.000Z', updatedAt: '2026-08-20T02:00:00.000Z', sourceEventId: 'event_update'});
        assert.deepEqual(state, {
            persona_id: 'persona_a',
            situation: '正在恢复',
            mood: '安心',
            appearance_json: '{"coat":"蓝色"}',
            checkpoint_at: '2026-08-20T02:00:00.000Z',
            updated_at: '2026-08-20T02:00:00.000Z',
            source_event_id: 'event_update'
        });
    } finally {
        database.close();
    }
});

test('life-event repository has no HTTP, provider, or browser DTO ownership', async () => {
    const source = await readFile(new URL('../server/infrastructure/life-event-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|res\.json|summary\s*\(/i);
});
