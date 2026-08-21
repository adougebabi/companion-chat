import assert from 'node:assert/strict';
import test from 'node:test';
import {createCompanionTestContext} from '../server/testing/companion-context.js';
import {createSceneEventFlow} from '../server/application/scene-event-flow.js';

let idSequence = 0;
const context = createCompanionTestContext({
    clock: () => '2026-08-21T00:00:00.000Z',
    idGenerator: prefix => `${prefix}_scene_test_${++idSequence}`
});
const {database, repositories} = context;

const lifeEventRepository = {
    ...repositories.lifeEvent,
    findByIdempotencyKey({personaId, idempotencyKey}) {
        return database.prepare("SELECT * FROM companion_life_events WHERE persona_id = ? AND json_extract(payload_json, '$.idempotencyKey') = ? ORDER BY created_at, id LIMIT 1").get(personaId, idempotencyKey);
    }
};
const stateRepository = {
    read(personaId) {
        return database.prepare('SELECT * FROM companion_persona_states WHERE persona_id = ?').get(personaId);
    },
    updateProjection({personaId, situation, mood, checkpointAt, updatedAt, sourceEventId, sharedSceneJson, expected}) {
        return database.prepare(`
            UPDATE companion_persona_states
            SET situation = ?, mood = ?, checkpoint_at = ?, updated_at = ?, source_event_id = ?, shared_scene_json = ?
            WHERE persona_id = ? AND source_event_id IS ? AND shared_scene_json = ?
        `).run(situation, mood, checkpointAt, updatedAt, sourceEventId, sharedSceneJson, personaId, expected.sourceEventId || null, expected.sharedSceneJson || '{}');
    }
};
const sceneFlow = createSceneEventFlow({
    repositories: {persona: repositories.persona, conversationRepository: repositories.conversation, lifeEvent: lifeEventRepository, state: stateRepository},
    clock: context.clock,
    idGenerator: context.id,
    scheduledState: () => ({situation: '正在按自己的安排活动', mood: '平静'}),
    transaction: work => database.transaction(work)()
});

function createPersona(name) {
    const id = `persona_${name}`;
    database.prepare(`INSERT INTO companion_personas (id, name, role, color, enabled, created_at, updated_at) VALUES (?, ?, '陪伴者', '#888888', 1, ?, ?)`)
        .run(id, name, context.clock(), context.clock());
    database.prepare(`INSERT INTO companion_persona_states (persona_id, situation, mood, appearance_json, checkpoint_at, updated_at, source_event_id, shared_scene_json) VALUES (?, '', '平静', '{}', ?, ?, NULL, '{}')`)
        .run(id, context.clock(), context.clock());
    return {id, name, role: '陪伴者'};
}

function appendMessage(personaId, text, suffix = '') {
    const conversation = repositories.conversation.getOrCreateConversation({personaId, id: `conversation_${personaId}`, createdAt: context.clock(), updatedAt: context.clock()});
    return repositories.conversation.appendMessage({id: `message_${personaId}${suffix}`, conversationId: conversation.id, role: 'user', text, attachmentsJson: '[]', generationJson: null, jobsJson: '[]', proactiveEventId: null, proactivePendingEventId: null, createdAt: context.clock(), readAt: context.clock()});
}

function deletePersona(personaId) {
    database.prepare('DELETE FROM companion_jobs WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_messages WHERE conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)').run(personaId);
    database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_life_events WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_persona_states WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
}

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
    const persona = createPersona('Scene plan preview');
    try {
        const source = appendMessage(persona.id, 'Let us sit by the window.');
        const beforeState = stateSnapshot(persona.id);
        const beforeEvents = eventCount(persona.id);
        const plan = sceneFlow.plan({personaId: persona.id, call: sceneCall(), sourceMessageId: source.id, provenance: {
            source: 'native', callId: 'call_scene_plan', idempotencyKey: 'scene-plan-preview-1'
        }});

        assert.equal(plan.type, 'scene_event_plan');
        assert.equal(plan.preallocatedIds.eventId, plan.eventId);
        assert.equal(plan.previewResult.eventId, plan.eventId);
        assert.equal(plan.previewResult.scene, plan.scene);
        assert.equal(plan.provenance.source, 'native');
        assert.equal(plan.previousScene, null);
        assert.equal(eventCount(persona.id), beforeEvents);
        assert.deepEqual(stateSnapshot(persona.id), beforeState);

        const applied = sceneFlow.apply(plan);
        assert.equal(applied.eventId, plan.eventId);
        assert.equal(applied.operation, 'start');
        assert.equal(applied.replayed, undefined);
        assert.equal(eventCount(persona.id), beforeEvents + 1);
        const persisted = database.prepare('SELECT type, causation_id, payload_json FROM companion_life_events WHERE id = ?').get(plan.eventId);
        assert.equal(persisted.type, 'shared_scene');
        assert.equal(persisted.causation_id, source.id);
        assert.equal(JSON.parse(persisted.payload_json).idempotencyKey, 'scene-plan-preview-1');
        assert.equal(JSON.parse(stateSnapshot(persona.id).shared_scene_json).eventId, plan.eventId);

        const replayed = sceneFlow.apply(plan);
        assert.equal(replayed.eventId, applied.eventId);
        assert.equal(replayed.replayed, true);
        assert.equal(eventCount(persona.id), beforeEvents + 1);
    } finally {
        deletePersona(persona.id);
    }
});

test('scene apply rolls back its event and state projection when the caller transaction fails', () => {
    const persona = createPersona('Scene plan rollback');
    try {
        const source = appendMessage(persona.id, 'Start a scene.');
        const plan = sceneFlow.plan({personaId: persona.id, call: sceneCall(), sourceMessageId: source.id, provenance: {
            source: 'native', callId: 'call_scene_rollback', idempotencyKey: 'scene-plan-rollback-1'
        }});
        const beforeState = stateSnapshot(persona.id);
        const beforeEvents = eventCount(persona.id);

        assert.throws(() => database.transaction(() => {
            sceneFlow.apply(plan);
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
    const persona = createPersona('Scene plan stale');
    try {
        const firstSource = appendMessage(persona.id, 'Start the first scene.');
        const stalePlan = sceneFlow.plan({personaId: persona.id, call: sceneCall(), sourceMessageId: firstSource.id, provenance: {idempotencyKey: 'scene-plan-stale-1'}});
        const secondSource = appendMessage(persona.id, 'Switch scenes.', '_second');
        sceneFlow.apply(sceneFlow.plan({personaId: persona.id, call: {...sceneCall(), operation: 'switch', location: 'bookshop'}, sourceMessageId: secondSource.id, provenance: {idempotencyKey: 'scene-plan-stale-2'}}));
        const beforeEvents = eventCount(persona.id);

        assert.throws(() => sceneFlow.apply(stalePlan), /current scene projection/);
        assert.equal(eventCount(persona.id), beforeEvents);
    } finally {
        deletePersona(persona.id);
    }
});

test.after(() => context.cleanup());
