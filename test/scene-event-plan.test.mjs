import assert from 'node:assert/strict';
import {mkdtempSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-scene-plan-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';

const {companionTestHooks} = await import(`../server.js?scene-plan-test=${Date.now()}`);
const {database, createPersona, deletePersona, appendMessage, planSceneEvent, applySceneEventPlan, applySceneEvent} = companionTestHooks;

function stateSnapshot(personaId) {
    return database.prepare(`
        SELECT source_event_id, shared_scene_json, situation, mood, checkpoint_at, updated_at
        FROM companion_persona_states WHERE persona_id = ?
    `).get(personaId);
}

function eventCount(personaId) {
    return database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(personaId).count;
}

function sceneCall() {
    return {
        operation: 'start',
        location: 'quiet cafe',
        room: 'window seat',
        activity: 'talking together',
        situation: 'We are talking together by the cafe window.',
        mood: 'calm',
        objects: ['tea'],
        participants: ['user', 'persona']
    };
}

test('scene planning is read-only, apply commits, and replay is idempotent', () => {
    const persona = createPersona({name: 'Scene plan preview', role: 'companion', foundation: 'Scene plan test persona.'});
    try {
        const source = appendMessage(persona.id, {role: 'user', text: 'Let us sit by the window.'});
        const beforeState = stateSnapshot(persona.id);
        const beforeEvents = eventCount(persona.id);
        const plan = planSceneEvent(persona, sceneCall(), source.id, {
            source: 'native', callId: 'call_scene_plan', idempotencyKey: 'scene-plan-preview-1'
        });

        assert.equal(plan.type, 'scene_event_plan');
        assert.equal(plan.preallocatedIds.eventId, plan.eventId);
        assert.equal(plan.previewResult.eventId, plan.eventId);
        assert.equal(plan.previewResult.scene, plan.scene);
        assert.equal(plan.provenance.source, 'native');
        assert.equal(plan.previousScene, null);
        assert.equal(eventCount(persona.id), beforeEvents);
        assert.deepEqual(stateSnapshot(persona.id), beforeState);

        const applied = database.transaction(() => applySceneEventPlan(plan))();
        assert.equal(applied.eventId, plan.eventId);
        assert.equal(applied.operation, 'start');
        assert.equal(applied.replayed, undefined);
        assert.equal(eventCount(persona.id), beforeEvents + 1);
        const persisted = database.prepare('SELECT type, causation_id, payload_json FROM companion_life_events WHERE id = ?').get(plan.eventId);
        assert.equal(persisted.type, 'shared_scene');
        assert.equal(persisted.causation_id, source.id);
        assert.equal(JSON.parse(persisted.payload_json).idempotencyKey, 'scene-plan-preview-1');
        assert.equal(JSON.parse(stateSnapshot(persona.id).shared_scene_json).eventId, plan.eventId);

        const replayed = database.transaction(() => applySceneEventPlan(plan))();
        assert.equal(replayed.eventId, applied.eventId);
        assert.equal(replayed.replayed, true);
        assert.equal(eventCount(persona.id), beforeEvents + 1);
    } finally {
        deletePersona(persona.id);
    }
});

test('scene apply rolls back its event and state projection when the caller transaction fails', () => {
    const persona = createPersona({name: 'Scene plan rollback', role: 'companion', foundation: 'Scene rollback test persona.'});
    try {
        const source = appendMessage(persona.id, {role: 'user', text: 'Start a scene.'});
        const plan = planSceneEvent(persona.id, sceneCall(), source.id, {
            source: 'native', callId: 'call_scene_rollback', idempotencyKey: 'scene-plan-rollback-1'
        });
        const beforeState = stateSnapshot(persona.id);
        const beforeEvents = eventCount(persona.id);

        assert.throws(() => database.transaction(() => {
            applySceneEventPlan(plan);
            throw new Error('force scene transaction rollback');
        })(), /force scene transaction rollback/);

        assert.equal(eventCount(persona.id), beforeEvents);
        assert.deepEqual(stateSnapshot(persona.id), beforeState);
        assert.equal(database.prepare('SELECT id FROM companion_life_events WHERE id = ?').get(plan.eventId), undefined);
    } finally {
        deletePersona(persona.id);
    }
});

test('apply rejects a plan after the expected scene source has changed', () => {
    const persona = createPersona({name: 'Scene plan stale', role: 'companion', foundation: 'Scene stale plan test persona.'});
    try {
        const firstSource = appendMessage(persona.id, {role: 'user', text: 'Start the first scene.'});
        const stalePlan = planSceneEvent(persona.id, sceneCall(), firstSource.id, {idempotencyKey: 'scene-plan-stale-1'});
        const secondSource = appendMessage(persona.id, {role: 'user', text: 'Switch scenes.'});
        applySceneEvent(persona.id, {...sceneCall(), operation: 'switch', location: 'bookshop'}, secondSource.id, {idempotencyKey: 'scene-plan-stale-2'});
        const beforeEvents = eventCount(persona.id);

        assert.throws(() => database.transaction(() => applySceneEventPlan(stalePlan))(), /当前场景不一致/);
        assert.equal(eventCount(persona.id), beforeEvents);
    } finally {
        deletePersona(persona.id);
    }
});
