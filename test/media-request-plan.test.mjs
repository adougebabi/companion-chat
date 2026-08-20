import assert from 'node:assert/strict';
import {mkdtempSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-media-plan-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';

const {companionTestHooks} = await import(`../server.js?media-plan-test=${Date.now()}`);
const {
    database, createPersona, planChatMediaRequest, applyChatMediaRequestPlan,
    normalizeMediaCapabilityCall
} = companionTestHooks;

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
    return normalizeMediaCapabilityCall({
        schemaVersion: 2, kind, request, count,
        personaMediaConcept: mediaConcept(kind), currentEvent: null, temporaryAppearance: {}
    });
}

function chatCounts(personaId) {
    return {
        conversations: database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get(personaId).count,
        messages: database.prepare('SELECT COUNT(*) AS count FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ?').get(personaId).count,
        jobs: database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type IN ('chat_image', 'chat_video')").get(personaId).count
    };
}

test('media planning is read-only and does not create a missing conversation', () => {
    const persona = createPersona({name: '媒体预览', role: '学生', foundation: '用于验证媒体预览不会写入状态。'});
    database.prepare('DELETE FROM companion_conversations WHERE persona_id = ?').run(persona.id);
    const before = chatCounts(persona.id);

    const plan = planChatMediaRequest(persona.id, mediaCall('image', 3), {idempotencyKey: 'preview-only'});

    assert.equal(plan.count, 3);
    assert.deepEqual(plan.assetKeys, ['preview-only:0', 'preview-only:1', 'preview-only:2']);
    assert.equal(plan.entries.length, 3);
    assert.equal(plan.entries.every(entry => entry.message && entry.job), true);
    assert.equal(plan.entries.every(entry => entry.message.id && entry.jobId), true);
    assert.deepEqual(chatCounts(persona.id), before);
});

test('applying a count plan is atomic and replay-safe even when reusing the same plan', () => {
    const persona = createPersona({name: '媒体批量计划', role: '学生', foundation: '用于验证媒体计划的数量和重放语义。'});
    const plan = planChatMediaRequest(persona.id, mediaCall('image', 3), {idempotencyKey: 'count-and-replay'});
    const before = chatCounts(persona.id);

    const first = database.transaction(() => applyChatMediaRequestPlan(plan))();
    const afterFirst = chatCounts(persona.id);
    assert.equal(first.replayed, false);
    assert.equal(first.jobIds.length, 3);
    assert.deepEqual(first.jobIds, plan.jobIds);
    assert.equal(afterFirst.messages - before.messages, 3);
    assert.equal(afterFirst.jobs - before.jobs, 3);

    const replay = database.transaction(() => applyChatMediaRequestPlan(plan))();
    assert.equal(replay.replayed, true);
    assert.deepEqual(replay.jobIds, first.jobIds);
    assert.deepEqual(chatCounts(persona.id), afterFirst);
});

test('a later media job failure rolls back every applied placeholder and job', () => {
    const persona = createPersona({name: '媒体计划回滚', role: '学生', foundation: '用于验证媒体计划的事务回滚。'});
    const plan = planChatMediaRequest(persona.id, mediaCall('image', 3), {idempotencyKey: 'rollback-plan'});
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
        assert.throws(() => database.transaction(() => applyChatMediaRequestPlan(plan))(), /forced media plan rollback/);
    } finally {
        database.exec('DROP TRIGGER fail_second_media_plan_job');
    }

    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE conversation_id = ?').get(conversationId).count, 0);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'chat_image'").get(persona.id).count, 0);
});
