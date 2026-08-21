import assert from 'node:assert/strict';
import test from 'node:test';
import {createCompanionTestContext} from '../server/testing/companion-context.js';
import {createMediaFlow} from '../server/application/media-flow.js';

let idSequence = 0;
const context = createCompanionTestContext({
    clock: () => '2026-08-21T00:00:00.000Z',
    idGenerator: prefix => `${prefix}_media_test_${++idSequence}`
});
const {database, repositories} = context;

const mediaFlow = createMediaFlow({
    repositories,
    clock: context.clock,
    idGenerator: context.id,
    normalizeCall(value) {
        return {...value, count: value.count ?? 1};
    },
    mediaConceptEnvelopeFor(_persona, {kind}) {
        return {schemaVersion: 1, mediaKind: kind, scene: '测试场景', action: '测试动作'};
    },
    providerFor: () => 'comfyui',
    transaction: work => database.transaction(work)()
});

function createPersona(name) {
    const id = `persona_${name}`;
    database.prepare(`INSERT INTO companion_personas (id, name, role, color, enabled, created_at, updated_at) VALUES (?, ?, '学生', '#888888', 1, ?, ?)`)
        .run(id, name, context.clock(), context.clock());
    return {id, name, role: '学生'};
}

function deletePersona(personaId) {
    database.prepare('DELETE FROM companion_jobs WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_messages WHERE conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)').run(personaId);
    database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(personaId);
    database.prepare('DELETE FROM companion_personas WHERE id = ?').run(personaId);
}

function mediaConcept(kind) {
    return {
        schemaVersion: 1, mediaKind: kind, scene: '测试场景', action: '测试动作', mood: '平静', narrative: '测试媒体概念',
        humanSubjects: [{label: '人格本人', role: '主体', inFrame: true}],
        nonHumanObjects: [{label: '环境', kind: 'environment', inFrame: true}],
        capture: {mode: 'external_capture', operator: '画外朋友', deviceVisibility: 'out_of_frame', framingIntent: '自然中景'},
        compositionIntent: '保持概念中的主体与环境关系。'
    };
}

function mediaCall(kind = 'image', count = 1, request = '测试媒体请求') {
    return {
        schemaVersion: 2, kind, request, count,
        personaMediaConcept: mediaConcept(kind), currentEvent: null, temporaryAppearance: {}
    };
}

function chatCounts(personaId) {
    return {
        conversations: database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get(personaId).count,
        messages: database.prepare('SELECT COUNT(*) AS count FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ?').get(personaId).count,
        jobs: database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type IN ('chat_image', 'chat_video')").get(personaId).count
    };
}

test('media planning is read-only and does not create a missing conversation', () => {
    const persona = createPersona('媒体预览');
    database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(persona.id);
    const before = chatCounts(persona.id);

    const plan = mediaFlow.plan({personaId: persona.id, call: mediaCall('image', 3), provenance: {idempotencyKey: 'preview-only'}});

    assert.equal(plan.count, 3);
    assert.deepEqual(plan.assetKeys, ['preview-only:0', 'preview-only:1', 'preview-only:2']);
    assert.equal(plan.entries.length, 3);
    assert.equal(plan.entries.every(entry => entry.message && entry.job), true);
    assert.equal(plan.entries.every(entry => entry.message.id && entry.jobId), true);
    assert.deepEqual(chatCounts(persona.id), before);
});

test('applying a count plan is atomic and replay-safe even when reusing the same plan', () => {
    const persona = createPersona('媒体批量计划');
    const plan = mediaFlow.plan({personaId: persona.id, call: mediaCall('image', 3), provenance: {idempotencyKey: 'count-and-replay'}});
    const before = chatCounts(persona.id);

    const first = mediaFlow.apply(plan);
    const afterFirst = chatCounts(persona.id);
    assert.equal(first.replayed, false);
    assert.equal(first.jobIds.length, 3);
    assert.deepEqual(first.jobIds, plan.jobIds);
    assert.equal(afterFirst.messages - before.messages, 3);
    assert.equal(afterFirst.jobs - before.jobs, 3);

    const replay = mediaFlow.apply(plan);
    assert.equal(replay.replayed, true);
    assert.deepEqual(replay.jobIds, first.jobIds);
    assert.deepEqual(chatCounts(persona.id), afterFirst);
});

test('a later media job failure rolls back every applied placeholder and job', () => {
    const persona = createPersona('媒体计划回滚');
    repositories.conversation.getOrCreateConversation({personaId: persona.id, id: `conversation_${persona.id}`, createdAt: context.clock(), updatedAt: context.clock()});
    const plan = mediaFlow.plan({personaId: persona.id, call: mediaCall('image', 3), provenance: {idempotencyKey: 'rollback-plan'}});
    const conversationId = database.prepare('SELECT id FROM companion_conversations WHERE persona_id = ?').get(persona.id).id;
    database.exec(`
        CREATE TRIGGER fail_second_media_plan_job
        BEFORE INSERT ON companion_jobs
        WHEN NEW.job_type = 'chat_image'
            AND (SELECT COUNT(*) FROM companion_jobs WHERE persona_id = NEW.persona_id AND job_type = 'chat_image') >= 1
        BEGIN
            SELECT RAISE(ABORT, 'forced media plan rollback');
        END;
    `);
    try {
        assert.throws(() => mediaFlow.apply(plan), /forced media plan rollback/);
    } finally {
        database.exec('DROP TRIGGER fail_second_media_plan_job');
    }

    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE conversation_id = ?').get(conversationId).count, 0);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'chat_image'").get(persona.id).count, 0);
});

test.after(() => context.cleanup());
