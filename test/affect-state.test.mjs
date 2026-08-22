import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {
    AFFECT_EVENT_TYPES,
    applyAffectEvent,
    createInitialAffectState,
    decayAffectState,
    normalizeAffectPolicy,
    reduceAffectEvent
} from '../server/domain/affect-state.js';
import createAffectRepository from '../server/infrastructure/affect-repository.js';
import createStartupRuntime from '../server/runtime/startup.js';

const START = '2026-08-20T00:00:00.000Z';

function policy(overrides = {}) {
    return {
        baseline: {pleasure: 0.2, arousal: -0.1, dominance: 0.3},
        halfLife: {pleasure: 1000, arousal: 1000, dominance: 1000},
        drives: {
            social: {baseline: 0.2, weight: 0.8, halfLife: 1000},
            exploration: {baseline: 0.3, weight: 0.6, halfLife: 1000},
            rest: {baseline: 0.4, weight: 0.7, halfLife: 1000},
            futureNeed: {baseline: 0.9, halfLife: 500}
        },
        ...overrides
    };
}

function temporaryDirectory() {
    return mkdtempSync(join(tmpdir(), 'local-ai-companion-affect-'));
}

function withRuntime(callback) {
    const dataDir = temporaryDirectory();
    const runtime = createStartupRuntime({
        Database,
        dataDir,
        now: () => START,
        id: prefix => `${prefix}_fixture`
    });
    runtime.database.prepare(`
        INSERT INTO companion_personas (id, name, role, color, created_at, updated_at)
        VALUES ('persona_1', 'One', 'companion', '#fff', ?, ?),
               ('persona_2', 'Two', 'companion', '#000', ?, ?)
    `).run(START, START, START, START);
    try {
        return callback(runtime);
    } finally {
        runtime.close();
        rmSync(dataDir, {recursive: true, force: true});
    }
}

test('policy keeps only fixed drives active while preserving future drive configuration', () => {
    const normalized = normalizeAffectPolicy(policy());
    assert.deepEqual(normalized.baseline, {pleasure: 0.2, arousal: -0.1, dominance: 0.3});
    assert.equal(normalized.drives.social.weight, 0.8);
    assert.equal(normalized.futureDrives.futureNeed.baseline, 0.9);

    const initial = createInitialAffectState({personaId: 'persona_1', at: START, policy: policy()});
    assert.deepEqual(initial.drives, {social: 0.2, exploration: 0.3, rest: 0.4, futureNeed: 0.9});
    assert.equal(initial.revision, 0);
});

test('PAD and drive pressure decay lazily toward persona baselines at half-life', () => {
    const initial = createInitialAffectState({personaId: 'persona_1', at: START, policy: policy()});
    const changed = applyAffectEvent(initial, {type: 'stress', effectiveAt: START}, {policy: policy(), at: START}).state;
    const decayed = decayAffectState(changed, {
        policy: policy(),
        at: '2026-08-20T00:00:01.000Z'
    });
    assert.equal(changed.revision, 1);
    assert.equal(decayed.revision, 1);
    assert.ok(decayed.pleasure > changed.pleasure && decayed.pleasure < 0.2);
    assert.ok(decayed.arousal < changed.arousal && decayed.arousal > -0.1);
    assert.ok(decayed.effectiveAt.endsWith('00:00:01.000Z'));
});

test('allowlisted event types map to bounded server-owned deltas and reject arbitrary numbers', () => {
    assert.ok(AFFECT_EVENT_TYPES.includes('social_connection'));
    assert.deepEqual(reduceAffectEvent({type: 'connection'}), {
        eventType: 'social_connection', pleasureDelta: 0.12, arousalDelta: 0.04,
        dominanceDelta: 0.02, drivesDelta: {social: -0.25}
    });
    assert.throws(() => reduceAffectEvent({type: 'joy', pleasureDelta: 1}), /server-owned/);
    assert.throws(() => reduceAffectEvent({type: 'not-registered'}), /Unsupported affect event/);
});

test('drive signals use bounded pressure deltas and preserve unknown future keys without activating them', () => withRuntime(runtime => {
    const repository = createAffectRepository({database: runtime.database, clock: () => START, policy: policy()});
    const known = repository.applyDriveSignal({
        personaId: 'persona_1', drive: 'social', direction: 'increase_pressure',
        idempotencyKey: 'drive-1', effectiveAt: START
    });
    assert.equal(known.snapshot.drives.social, 0.38);
    assert.equal(known.event.eventType, 'drive_signal');

    const future = repository.applyDriveSignal({
        personaId: 'persona_1', drive: 'future_need', direction: 'increase_pressure',
        idempotencyKey: 'drive-2', effectiveAt: START
    });
    assert.equal(future.event.payload.recognized, false);
    assert.equal(Object.hasOwn(future.snapshot.drives, 'future_need'), false);
    assert.equal(future.snapshot.drives.futureNeed, 0.9);
    assert.equal(future.snapshot.drives.social, 0.38);
}));

test('repository applies event and snapshot atomically, with persona-scoped idempotency', () => withRuntime(runtime => {
    let idSequence = 0;
    const repository = createAffectRepository({
        database: runtime.database,
        clock: () => START,
        id: prefix => `${prefix}_${++idSequence}`,
        policyForPersona: () => policy()
    });
    const first = repository.applyEvent({
        personaId: 'persona_1', type: 'social_connection', idempotencyKey: 'turn-1',
        effectiveAt: START, causationId: 'message_1'
    });
    assert.equal(first.created, true);
    assert.equal(first.snapshot.revision, 1);
    assert.equal(first.snapshot.drives.social < 0.2, true);

    const duplicate = repository.applyEvent({
        personaId: 'persona_1', type: 'stress', idempotencyKey: 'turn-1', effectiveAt: START
    });
    assert.equal(duplicate.created, false);
    assert.equal(duplicate.event.eventType, 'social_connection');
    assert.equal(repository.listEvents({personaId: 'persona_1'}).length, 1);

    const otherPersona = repository.applyEvent({
        personaId: 'persona_2', type: 'stress', idempotencyKey: 'turn-1', effectiveAt: START
    });
    assert.equal(otherPersona.created, true);
    assert.equal(repository.listEvents({personaId: 'persona_2'}).length, 1);
}));

test('repository CAS refuses stale checkpoints and failed transactions leave no event or snapshot', () => withRuntime(runtime => {
    const repository = createAffectRepository({
        database: runtime.database,
        clock: () => START,
        policyForPersona: () => policy()
    });
    const first = repository.applyEvent({personaId: 'persona_1', type: 'joy', idempotencyKey: 'turn-1', effectiveAt: START});
    const stale = repository.compareAndSwapSnapshot({
        personaId: 'persona_1', expectedRevision: 0, pleasure: 0.9, arousal: 0.9, dominance: 0.9,
        drives: {social: 0.9}, effectiveAt: START
    });
    assert.equal(stale.updated, false);
    assert.equal(repository.findSnapshot({personaId: 'persona_1'}).revision, first.snapshot.revision);

    assert.throws(() => repository.applyEvent({
        personaId: 'persona_1', type: 'joy', idempotencyKey: 'turn-2', effectiveAt: START,
        payload: {notSerializable: BigInt(1)}
    }));
    assert.equal(repository.listEvents({personaId: 'persona_1'}).length, 1);
    assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_persona_affect_states').get().count, 1);
}));
