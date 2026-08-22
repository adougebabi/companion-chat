import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createMemoryEventFlow} from '../server/application/memory-flow.js';
import {createMemoryRepository} from '../server/infrastructure/memory-repository.js';

const NOW = '2026-08-20T00:00:00.000Z';

function databaseFor({withIdempotencyColumn = false} = {}) {
    const database = new Database(':memory:');
    database.exec(`
        CREATE TABLE companion_memories (
            id TEXT PRIMARY KEY, persona_id TEXT NOT NULL, memory_key TEXT NOT NULL,
            value TEXT NOT NULL, confidence REAL NOT NULL, status TEXT NOT NULL,
            source_type TEXT, source_id TEXT${withIdempotencyColumn ? ', idempotency_key TEXT UNIQUE' : ''},
            created_at TEXT NOT NULL, updated_at TEXT NOT NULL, superseded_at TEXT
        );
    `);
    return database;
}

function fixture(options = {}) {
    const database = databaseFor(options);
    const sourceRows = new Map([
        ['message_1', {id: 'message_1', personaId: 'persona_1', role: 'user'}],
        ['message_2', {id: 'message_2', personaId: 'persona_2', role: 'user'}],
        ['assistant_1', {id: 'assistant_1', personaId: 'persona_1', role: 'assistant'}]
    ]);
    const personas = new Map([['persona_1', {id: 'persona_1'}], ['persona_2', {id: 'persona_2'}]]);
    const memory = createMemoryRepository({database, clock: () => NOW});
    const flow = createMemoryEventFlow({
        repositories: {
            memoryRepository: memory,
            personaRepository: {findActive(id) { return personas.get(id) ?? null; }},
            messageRepository: {findById({id}) { return sourceRows.get(id) ?? null; }}
        },
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_allocated`
    });
    return {database, memory, flow};
}

function command(overrides = {}) {
    return {
        personaId: 'persona_1',
        sourceMessageId: 'message_1',
        call: {
            schemaVersion: 1,
            operation: 'upsert',
            memoryKey: 'favorite_drink',
            value: 'tea',
            confidence: 0.8,
            sourceType: 'structured_turn'
        },
        provenance: {source: 'native', callId: 'call_1', idempotencyKey: 'turn_1'},
        ...overrides
    };
}

test('memory_event plan validates ownership and is read-only', () => {
    const setup = fixture();
    try {
        const plan = setup.flow.plan(command());
        assert.equal(plan.type, 'memory_event_plan');
        assert.equal(plan.memoryId.startsWith('memory_event_'), true);
        assert.deepEqual(plan.preallocatedIds, {memoryId: plan.memoryId});
        assert.equal(plan.previewResult.memory.sourceId, 'message_1');
        assert.equal(setup.database.prepare('SELECT COUNT(*) AS count FROM companion_memories').get().count, 0);
        assert.throws(() => setup.flow.plan(command({
            sourceMessageId: 'assistant_1',
            call: {...command().call, sourceId: 'assistant_1'},
            provenance: {idempotencyKey: 'wrong_source'}
        })), /user-owned/);
        assert.throws(() => setup.flow.plan(command({
            call: {...command().call, confidence: 1.1},
            provenance: {idempotencyKey: 'bad_confidence'}
        })), /0 到 1|between 0 and 1/);
    } finally {
        setup.database.close();
    }
});

test('memory_event apply is persona-scoped and idempotent across fresh plans', () => {
    const setup = fixture();
    try {
        const firstPlan = setup.flow.plan(command());
        const first = setup.flow.apply(firstPlan);
        assert.equal(first.created, true);
        assert.equal(first.replayed, false);
        assert.equal(setup.database.prepare('SELECT COUNT(*) AS count FROM companion_memories WHERE persona_id = ?').get('persona_1').count, 1);

        const replayPlan = setup.flow.plan(command());
        assert.equal(replayPlan.replayed, true);
        const replay = setup.flow.apply(replayPlan);
        assert.equal(replay.replayed, true);
        assert.equal(replay.memoryId, first.memoryId);
        assert.equal(setup.database.prepare('SELECT COUNT(*) AS count FROM companion_memories').get().count, 1);

        const other = setup.flow.plan(command({
            personaId: 'persona_2',
            sourceMessageId: 'message_2'
        }));
        const otherResult = setup.flow.apply(other);
        assert.equal(otherResult.created, true);
        assert.equal(setup.database.prepare('SELECT COUNT(*) AS count FROM companion_memories WHERE persona_id = ?').get('persona_2').count, 1);
    } finally {
        setup.database.close();
    }
});

test('upsert changes only the selected memory while preserving auditable source fields', () => {
    const setup = fixture();
    try {
        setup.flow.apply(setup.flow.plan(command()));
        const next = setup.flow.plan(command({
            call: {...command().call, value: 'coffee', confidence: 0.9},
            provenance: {source: 'native', idempotencyKey: 'turn_2'}
        }));
        const result = setup.flow.apply(next);
        assert.equal(result.created, true);
        const rows = setup.database.prepare('SELECT * FROM companion_memories').all();
        assert.equal(rows.length, 1);
        assert.deepEqual({value: rows[0].value, confidence: rows[0].confidence, source_id: rows[0].source_id}, {
            value: 'coffee', confidence: 0.9, source_id: 'message_1'
        });
    } finally {
        setup.database.close();
    }
});

test('memory_event apply participates in caller transaction and rolls back on a failed write', () => {
    const setup = fixture();
    try {
        const plan = setup.flow.plan(command());
        assert.throws(() => setup.flow.apply(plan, work => setup.database.transaction(() => {
            work();
            throw new Error('forced rollback');
        })()), /forced rollback/);
        assert.equal(setup.database.prepare('SELECT COUNT(*) AS count FROM companion_memories').get().count, 0);
    } finally {
        setup.database.close();
    }
});
