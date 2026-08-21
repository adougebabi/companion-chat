import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createRelationshipRepository} from '../server/infrastructure/relationship-repository.js';

function createDatabase() {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_persona_evolutions (
            id TEXT PRIMARY KEY,
            persona_id TEXT NOT NULL,
            reason TEXT NOT NULL,
            evidence_json TEXT NOT NULL,
            previous_patch TEXT NOT NULL,
            next_patch TEXT NOT NULL,
            status TEXT NOT NULL,
            created_at TEXT NOT NULL,
            reverted_at TEXT
        );
    `);
    database.prepare(`
        INSERT INTO companion_persona_evolutions
            (id, persona_id, reason, evidence_json, previous_patch, next_patch, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run('evolution_old', 'persona_1', 'old', '[]', '{}', JSON.stringify({communicationStyle: 'old'}), 'applied', '2026-08-20T00:00:01.000Z');
    database.prepare(`
        INSERT INTO companion_persona_evolutions
            (id, persona_id, reason, evidence_json, previous_patch, next_patch, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run('evolution_new', 'persona_1', 'new', JSON.stringify([{type: 'message'}]), JSON.stringify({communicationStyle: 'old'}), JSON.stringify({communicationStyle: 'new'}), 'applied', '2026-08-20T00:00:02.000Z');
    database.prepare(`
        INSERT INTO companion_persona_evolutions
            (id, persona_id, reason, evidence_json, previous_patch, next_patch, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run('evolution_reverted', 'persona_1', 'reverted', '[]', '{}', JSON.stringify({relationshipNote: 'ignored'}), 'reverted', '2026-08-20T00:00:03.000Z');
    database.prepare(`
        INSERT INTO companion_persona_evolutions
            (id, persona_id, reason, evidence_json, previous_patch, next_patch, status, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `).run('evolution_other', 'persona_2', 'other', '[]', '{}', '{}', 'applied', '2026-08-20T00:00:04.000Z');
    return database;
}

function repositoryFor(database, overrides = {}) {
    return createRelationshipRepository({
        database,
        id: prefix => `${prefix}_created`,
        clock: () => '2026-08-20T01:02:03.000Z',
        ...overrides
    });
}

test('relationship repository requires an already-open database', () => {
    assert.throws(() => createRelationshipRepository(), /open database/);
});

test('activePatch returns the latest applied raw JSON row and listRecent is persona-scoped', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        assert.deepEqual(repository.activePatch({personaId: 'persona_1'}), {next_patch: JSON.stringify({communicationStyle: 'new'})});
        assert.deepEqual(repository.listRecent({personaId: 'persona_1'}).map(row => row.id), ['evolution_reverted', 'evolution_new', 'evolution_old']);
        assert.deepEqual(repository.listRecent({personaId: 'persona_1', limit: 1}).map(row => row.id), ['evolution_reverted']);
        assert.equal(repository.activePatch({personaId: 'missing'}), undefined);
    } finally {
        database.close();
    }
});

test('insertEvolution serializes supplied JSON, uses injected id/clock, and returns a raw row', () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const inserted = repository.insertEvolution({
            personaId: 'persona_1',
            reason: 'new evidence',
            evidence: [{type: 'message', id: 'message_1'}],
            previousPatch: {communicationStyle: 'new'},
            nextPatch: {communicationStyle: 'warmer'},
            status: 'applied'
        });
        assert.equal(inserted.id, 'evolution_created');
        assert.equal(inserted.created_at, '2026-08-20T01:02:03.000Z');
        assert.deepEqual(JSON.parse(inserted.evidence_json), [{type: 'message', id: 'message_1'}]);
        assert.deepEqual(JSON.parse(inserted.previous_patch), {communicationStyle: 'new'});
        assert.deepEqual(JSON.parse(inserted.next_patch), {communicationStyle: 'warmer'});
    } finally {
        database.close();
    }
});

test('relationship repository preserves raw JSON strings and validates storage identity/limit', async () => {
    const database = createDatabase();
    try {
        const repository = repositoryFor(database);
        const inserted = repository.insertEvolution({
            id: 'evolution_raw', personaId: 'persona_1', reason: 'raw', evidence: '[1]',
            previousPatch: '{"x":1}', nextPatch: '{"x":2}', createdAt: new Date('2026-08-20T02:00:00.000Z'), status: 'applied'
        });
        assert.equal(inserted.evidence_json, '[1]');
        assert.equal(inserted.next_patch, '{"x":2}');
        assert.throws(() => repository.activePatch(), /Persona.id/);
        assert.throws(() => repository.listRecent({personaId: 'persona_1', limit: 0}), /positive integer/);
    } finally {
        database.close();
    }
    const source = await readFile(new URL('../server/infrastructure/relationship-repository.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|normalizeRelationshipPatch|relationshipPatchSummary|Object\.assign/i);
});
