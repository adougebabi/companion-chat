import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createCompanionRuntime} from '../server/runtime/runtime.js';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-modular-life-'));
const BASE = '2026-08-21T08:00:00.000Z';
let now = BASE;
let idCounter = 0;
const providerCalls = [];
const queuedDecisions = [];

function id(prefix) {
    idCounter += 1;
    return `${prefix}_modular_${idCounter}`;
}

function decisionForRequest(input) {
    const content = input?.messages?.at(-1)?.content;
    let request = {};
    try { request = JSON.parse(content); } catch { /* The fixture only needs a bounded fallback. */ }

    if (queuedDecisions.length) return queuedDecisions.shift();
    if (request.source?.sourceType === 'pending_event') {
        return {schemaVersion: 1, send: true, reason: 'explicit follow-up', message: '我想起你刚才交代的那件事了。'};
    }
    if (request.source?.sourceType === 'life_event') {
        return {schemaVersion: 1, send: true, reason: 'worth sharing', message: '刚刚想和你分享一件小事。'};
    }
    return {publish: true, content: '今天的生活有一个值得记录的片段。', media: {kind: 'none'}};
}

const llmFixture = {
    id: 'mtplx',
    label: 'modular fixture',
    portType: 'llm-streaming',
    capabilities: ['stream'],
    async stream(input) {
        providerCalls.push(input);
        return {
            ok: true,
            json: async () => ({choices: [{message: {content: JSON.stringify(decisionForRequest(input))}}]})
        };
    }
};

function createRuntime(options = {}) {
    return createCompanionRuntime({
        Database,
        dataDir,
        environment: {DATA_DIR: dataDir},
        clock: () => now,
        idGenerator: id,
        providerAdapters: {mtplx: llmFixture},
        workerRuntime: false,
        ...options
    });
}

let runtime = createRuntime();

function transaction(work) {
    return runtime.database.transaction(work)();
}

function services() {
    return runtime.application.services;
}

function persona(input) {
    return services().persona.create({
        name: input.name,
        role: input.role ?? '陪伴者',
        foundation: input.foundation ?? `${input.name}的测试设定。`,
        ...(input.blueprint ? {blueprint: input.blueprint} : {})
    });
}

function userMessage(personaId, text = '今天也要记得告诉我你的近况。') {
    return services().conversations.appendMessage({personaId, role: 'user', text});
}

function lifeEvent(personaId, input) {
    return runtime.repositories.lifeEvent.createEvent({
        eventId: input.eventId,
        personaId,
        type: input.type ?? 'social',
        occurredAt: input.occurredAt ?? now,
        resolvesAt: input.resolvesAt ?? null,
        causationId: input.causationId ?? null,
        payload: input.payload ?? {}
    });
}

function enqueueJob(input) {
    return runtime.repositories.job.enqueue({
        id: input.id ?? id('job'),
        jobType: input.jobType,
        personaId: input.personaId,
        runAfter: input.runAfter ?? now,
        maxAttempts: input.maxAttempts ?? 3,
        payload: input.payload ?? {}
    });
}

test('native appearance_event persists the current outfit through the capability dispatcher', () => {
    const subject = persona({name: '换装测试人格'});
    const source = userMessage(subject.id, '我想看看你今天穿什么。');
    const output = runtime.application.capabilityDispatcher.dispatch({
        mode: 'plan',
        personaId: subject.id,
        causationUserMessageId: source.id,
        calls: [{
            id: 'appearance_call_1',
            index: 0,
            name: 'appearance_event',
            source: 'native',
            personaId: subject.id,
            causationUserMessageId: source.id,
            idempotencyKey: 'appearance_dispatch_1',
            argumentsText: JSON.stringify({operation: 'set', outfit: '白色衬衫和深色长裙'}),
            arguments: {operation: 'set', outfit: '白色衬衫和深色长裙'}
        }],
        completion: {doneSeen: true}
    });
    assert.equal(output.effects.length, 1);
    const result = output.effects[0].payload.apply();
    assert.equal(result.outfit, '白色衬衫和深色长裙');
    const state = runtime.repositories.state.read({personaId: subject.id});
    assert.equal(JSON.parse(state.appearance_json).outfit, '白色衬衫和深色长裙');
    const resolved = services().life.resolvedStateFor({personaId: subject.id, at: new Date(now)});
    assert.equal(JSON.parse(resolved.appearance_json).outfit, '白色衬衫和深色长裙');
    const shaped = services().life.stateShape({personaId: subject.id, at: new Date(now)});
    assert.equal(shaped.appearance.outfit, '白色衬衫和深色长裙');
    services().persona.delete({personaId: subject.id});
});

async function dispatchClaimed({owner = 'modular-worker', at = now, jobType} = {}) {
    const job = runtime.repositories.job.claim({
        leaseOwner: owner,
        leaseMs: 60_000,
        now: at,
        ...(jobType ? {jobTypes: [jobType]} : {})
    });
    assert.ok(job, 'expected a due job to be claimed');
    return runtime.jobDispatcher.runJob(job, {
        owner,
        leaseOwner: owner,
        leaseMs: 60_000,
        now: at
    });
}

function createPending(personaId, {
    notBefore = new Date(Date.parse(now) + 60_000).toISOString(),
    expiresAt = new Date(Date.parse(now) + 3_600_000).toISOString(),
    dedupeKey = `follow-up-${idCounter}`,
    provenance = {}
} = {}) {
    const source = userMessage(personaId, '我稍后有一件事，届时请记得问问我。');
    const plan = runtime.application.pendingEventFlow.plan({
        personaId,
        sourceMessageId: source.id,
        call: {schemaVersion: 1, summary: '稍后问问这件事', notBefore, expiresAt, dedupeKey},
        provenance: {source: 'native', callId: `pending-call-${dedupeKey}`, ...provenance}
    });
    const applied = runtime.application.pendingEventFlow.apply(plan, {transaction});
    return {source, plan, ...applied};
}

function deletePersona(personaId) {
    services().persona.delete({personaId});
}

test.after(async () => {
    await runtime.stop();
    rmSync(dataDir, {recursive: true, force: true});
});

test('persona-private conversations, life events, activities, jobs, and relationship rollback stay isolated', () => {
    now = BASE;
    const first = persona({name: '模块人格甲'});
    const second = persona({name: '模块人格乙'});
    try {
        const firstMessage = userMessage(first.id, '甲的人格私有消息。');
        const secondMessage = userMessage(second.id, '乙的人格私有消息。');
        const firstEvent = lifeEvent(first.id, {
            eventId: 'event_modular_first',
            payload: {situation: '在图书馆整理笔记', scene: '图书馆', mood: '专注'}
        });
        const secondEvent = lifeEvent(second.id, {
            eventId: 'event_modular_second',
            payload: {situation: '在旧书店挑书', scene: '旧书店', mood: '愉快'}
        });
        runtime.repositories.activity.insertActivity({
            id: 'activity_modular_first', personaId: first.id, eventId: firstEvent.id,
            content: '甲记录了今天的片段。', mediaMode: 'none', mediaStatus: 'none', createdAt: now
        });
        runtime.repositories.activity.insertActivity({
            id: 'activity_modular_second', personaId: second.id, eventId: secondEvent.id,
            content: '乙记录了今天的片段。', mediaMode: 'none', mediaStatus: 'none', createdAt: now
        });
        enqueueJob({id: 'job_modular_first', jobType: 'pending_event', personaId: first.id, runAfter: now, payload: {pendingEventId: 'none'}});
        enqueueJob({id: 'job_modular_second', jobType: 'pending_event', personaId: second.id, runAfter: now, payload: {pendingEventId: 'none'}});

        const evolution = runtime.repositories.relationship.insertEvolution({
            id: 'evolution_modular_first', personaId: first.id, reason: '持续交流证据',
            evidence: [{source: 'conversation', messageId: firstMessage.id}],
            previousPatch: {trust: 0.2}, nextPatch: {trust: 0.7}, createdAt: now
        });
        assert.deepEqual(JSON.parse(runtime.repositories.relationship.activePatch({personaId: first.id}).next_patch), {trust: 0.7});
        const rolledBack = services().relationship.rollback({personaId: first.id, evolutionId: evolution.id});
        assert.equal(rolledBack.status, 'reverted');
        assert.equal(runtime.repositories.relationship.activePatch({personaId: first.id}), undefined);

        assert.deepEqual(services().activities.list({personaId: first.id, limit: 20}).items.map(item => item.id), ['activity_modular_first']);
        assert.deepEqual(services().activities.list({personaId: second.id, limit: 20}).items.map(item => item.id), ['activity_modular_second']);
        assert.deepEqual(services().conversations.list({personaId: first.id, markRead: false}).items.map(item => item.id), [firstMessage.id]);
        assert.deepEqual(services().conversations.list({personaId: second.id, markRead: false}).items.map(item => item.id), [secondMessage.id]);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ?').get(first.id).count, 2);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ?').get(second.id).count, 2);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_activities WHERE persona_id = ?').get(first.id).count, 1);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_activities WHERE persona_id = ?').get(second.id).count, 1);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(first.id).count, 1);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(second.id).count, 1);
    } finally {
        deletePersona(first.id);
        deletePersona(second.id);
    }
});

test('life-world resolution gives life events and shared scenes authoritative state, with scene replay idempotency', () => {
    now = BASE;
    const subject = persona({name: '生活世界解析'});
    try {
        runtime.repositories.schedule.createSchedule({
            id: 'schedule_modular_state', personaId: subject.id, title: '日程基线',
            startsAt: '2026-08-21T07:00:00.000Z', endsAt: '2026-08-21T10:00:00.000Z',
            details: {location: '日程地点', situation: '日程中的普通安排'}, createdAt: BASE
        });
        lifeEvent(subject.id, {
            eventId: 'event_modular_state', occurredAt: '2026-08-21T08:30:00.000Z',
            resolvesAt: '2026-08-21T09:30:00.000Z',
            payload: {situation: '在图书馆整理笔记', scene: '图书馆', mood: '专注', appearance: {coat: 'blue'}}
        });

        const eventState = services().life.resolvedStateFor({personaId: subject.id, at: new Date('2026-08-21T08:45:00.000Z')});
        assert.equal(eventState.resolved_source, 'event');
        assert.equal(eventState.situation, '在图书馆整理笔记');
        assert.equal(eventState.resolved_scene, '图书馆');
        assert.equal(eventState.resolved_ends_at, '2026-08-21T09:30:00.000Z');
        assert.deepEqual(JSON.parse(eventState.appearance_json), {coat: 'blue'});

        const source = userMessage(subject.id, '我们一起去湖边走走。');
        const sceneFlow = runtime.application.sceneEventFlow;
        const sceneInput = {
            personaId: subject.id,
            sourceMessageId: source.id,
            call: {operation: 'start', location: '湖边公园', room: '湖边步道', activity: '一起散步', situation: '正在湖边公园一起散步', mood: '放松'},
            provenance: {source: 'native', callId: 'scene_modular_1', idempotencyKey: 'scene-modular-replay'}
        };
        const applied = sceneFlow.apply(sceneFlow.plan(sceneInput), {transaction});
        assert.equal(applied.operation, 'start');
        const shared = services().life.stateShape({personaId: subject.id, at: new Date('2026-08-21T08:45:00.000Z')});
        assert.equal(shared.source.kind, 'shared_scene');
        assert.equal(shared.sourceId, applied.eventId);
        assert.equal(shared.location, '湖边公园');
        assert.equal(shared.sharedScene.eventId, applied.eventId);

        const replayed = sceneFlow.apply(sceneFlow.plan(sceneInput), {transaction});
        assert.equal(replayed.replayed, true);
        assert.equal(replayed.eventId, applied.eventId);
        assert.equal(runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND json_extract(payload_json, '$.idempotencyKey') = ?").get(subject.id, 'scene-modular-replay').count, 1);

        const ended = sceneFlow.apply(sceneFlow.plan({
            personaId: subject.id,
            sourceMessageId: source.id,
            call: {operation: 'end'},
            provenance: {source: 'native', callId: 'scene_modular_end', idempotencyKey: 'scene-modular-end'}
        }), {transaction});
        assert.equal(ended.operation, 'end');
        const resumed = services().life.resolvedStateFor({personaId: subject.id, at: new Date('2026-08-21T08:45:00.000Z')});
        assert.equal(resumed.resolved_source, 'event');
        assert.equal(resumed.situation, '在图书馆整理笔记');
    } finally {
        deletePersona(subject.id);
    }
});

test('pending-event flow is persona-scoped, deduplicated, and due delivery freezes one decision', async () => {
    now = BASE;
    queuedDecisions.push({schemaVersion: 1, send: true, reason: 'explicit follow-up', message: '我想起你稍后要处理的那件事了。'});
    const first = persona({name: '待定事件甲'});
    const second = persona({name: '待定事件乙'});
    try {
        const notBefore = new Date(Date.parse(BASE) + 60_000).toISOString();
        const expiresAt = new Date(Date.parse(BASE) + 3_600_000).toISOString();
        const created = createPending(first.id, {notBefore, expiresAt, dedupeKey: 'same-follow-up'});
        const replayPlan = runtime.application.pendingEventFlow.plan({
            personaId: first.id,
            sourceMessageId: created.source.id,
            call: {schemaVersion: 1, summary: '稍后问问这件事', notBefore, expiresAt, dedupeKey: 'same-follow-up'},
            provenance: {source: 'native', callId: 'pending-replay'}
        });
        const replay = runtime.application.pendingEventFlow.apply(replayPlan, {transaction});
        assert.equal(replay.created, false);
        assert.equal(replay.pendingEvent.id, created.pendingEvent.id);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(first.id).count, 1);
        assert.equal(runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'pending_event'").get(first.id).count, 1);

        const otherMessage = userMessage(second.id);
        assert.throws(() => runtime.application.pendingEventFlow.plan({
            personaId: second.id,
            sourceMessageId: created.source.id,
            call: {schemaVersion: 1, summary: '越权注册', notBefore, expiresAt, dedupeKey: 'cross-persona'}
        }), /belong to persona|不属于人格|source message does not exist/);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(second.id).count, 0);
        assert.ok(otherMessage.id);

        now = notBefore;
        const output = await dispatchClaimed({owner: 'pending-due-worker', at: notBefore, jobType: 'pending_event'});
        assert.equal(output.status, 'complete');
        assert.equal(runtime.database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'consumed');
        const delivered = runtime.database.prepare('SELECT * FROM companion_messages WHERE proactive_pending_event_id = ?').get(created.pendingEvent.id);
        assert.equal(delivered.text, '我想起你稍后要处理的那件事了。');
        const job = runtime.repositories.job.find({id: created.jobId});
        assert.equal(JSON.parse(job.result_json).decision.message, '我想起你稍后要处理的那件事了。');
        assert.equal(providerCalls.length > 0, true);
    } finally {
        deletePersona(first.id);
        deletePersona(second.id);
    }
});

test('expired pending events are consumed by the generic worker without an LLM call', async () => {
    now = BASE;
    const subject = persona({name: '过期待定事件'});
    try {
        const notBefore = new Date(Date.parse(BASE) + 60_000).toISOString();
        const expiresAt = new Date(Date.parse(BASE) + 120_000).toISOString();
        const created = createPending(subject.id, {notBefore, expiresAt, dedupeKey: 'expired-follow-up'});
        const callsBefore = providerCalls.length;
        now = new Date(Date.parse(BASE) + 180_000).toISOString();
        const output = await dispatchClaimed({owner: 'pending-expiry-worker', at: now, jobType: 'pending_event'});
        assert.equal(output.status, 'complete');
        assert.equal(output.result.skipped, 'expired');
        assert.equal(providerCalls.length, callsBefore);
        assert.equal(runtime.database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'expired');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE proactive_pending_event_id = ?').get(created.pendingEvent.id).count, 0);
    } finally {
        deletePersona(subject.id);
    }
});

test('activity publication is an application decision flow settled by the generic dispatcher', async () => {
    now = BASE;
    const subject = persona({name: '动态发布流程'});
    try {
        const event = lifeEvent(subject.id, {
            eventId: 'event_modular_activity',
            payload: {situation: '完成了今天的计划', scene: '家中', mood: '满足'}
        });
        const job = enqueueJob({
            id: 'job_modular_activity', jobType: 'activity_decision', personaId: subject.id,
            payload: {eventId: event.id}
        });
        const output = await dispatchClaimed({owner: 'activity-worker', at: now, jobType: 'activity_decision'});
        assert.equal(output.status, 'complete');
        assert.equal(output.result.published, true);
        const activity = runtime.database.prepare('SELECT * FROM companion_activities WHERE id = ?').get(output.result.activityId);
        assert.equal(activity.persona_id, subject.id);
        assert.equal(activity.event_id, event.id);
        assert.equal(activity.content, '今天的生活有一个值得记录的片段。');
        assert.equal(runtime.repositories.job.find({id: job.id}).status, 'complete');
        const callsBeforeReplay = providerCalls.length;

        const replayJob = enqueueJob({
            id: 'job_modular_activity_replay', jobType: 'activity_decision', personaId: subject.id,
            payload: {eventId: event.id}
        });
        const replay = await dispatchClaimed({owner: 'activity-replay-worker', at: now, jobType: 'activity_decision'});
        assert.equal(replay.status, 'complete');
        assert.equal(replay.result.skipped, 'already_published');
        assert.equal(providerCalls.length, callsBeforeReplay);
        assert.equal(runtime.repositories.job.find({id: replayJob.id}).status, 'complete');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_activities WHERE event_id = ?').get(event.id).count, 1);
    } finally {
        deletePersona(subject.id);
    }
});

test('proactive decision is frozen before a retry and the retry does not call the model again', async () => {
    now = BASE;
    const subject = persona({name: '主动行为重试'});
    const collisionId = 'message_proactive_collision';
    try {
        const conversation = runtime.repositories.conversation.getConversation(subject.id);
        runtime.repositories.conversation.appendMessage({
            id: collisionId, conversationId: conversation.id, role: 'user', text: 'temporary id collision',
            attachmentsJson: '[]', generationJson: null, jobsJson: '[]', createdAt: now, readAt: now
        });
        const event = lifeEvent(subject.id, {
            eventId: 'event_modular_retry',
            payload: {situation: '在咖啡馆想起一件事', scene: '咖啡馆', mood: '平静'}
        });
        queuedDecisions.push({schemaVersion: 1, send: true, reason: 'frozen reason', message: '这是第一次冻结的主动消息。'});
        queuedDecisions.push({schemaVersion: 1, send: true, reason: 'must not be evaluated again', message: '不应该重新生成。'});
        const job = enqueueJob({
            id: 'job_modular_retry', jobType: 'proactive_message', personaId: subject.id,
            maxAttempts: 3, payload: {eventId: event.id, replyMessageId: collisionId}
        });
        const callsBefore = providerCalls.length;
        const first = await dispatchClaimed({owner: 'proactive-retry-worker', at: now, jobType: 'proactive_message'});
        assert.equal(first.status, 'retry');
        assert.equal(providerCalls.length, callsBefore + 1);
        const queued = runtime.repositories.job.find({id: job.id});
        assert.equal(queued.status, 'queued');
        assert.equal(JSON.parse(queued.result_json).decision.message, '这是第一次冻结的主动消息。');

        runtime.database.prepare('DELETE FROM companion_messages WHERE id = ?').run(collisionId);
        now = new Date(Date.parse(now) + 5_000).toISOString();
        const second = await dispatchClaimed({owner: 'proactive-retry-worker-2', at: now, jobType: 'proactive_message'});
        assert.equal(second.status, 'complete');
        assert.equal(providerCalls.length, callsBefore + 1);
        const delivered = runtime.database.prepare('SELECT * FROM companion_messages WHERE proactive_event_id = ?').get(event.id);
        assert.equal(delivered.text, '这是第一次冻结的主动消息。');
        assert.equal(runtime.repositories.job.find({id: job.id}).status, 'complete');
    } finally {
        deletePersona(subject.id);
    }
});

test('worker restart reclaims an expired lease and settles the durable pending flow once', async () => {
    now = BASE;
    const subject = persona({name: '重启恢复待定事件'});
    let created;
    try {
        created = createPending(subject.id, {
            notBefore: BASE,
            expiresAt: new Date(Date.parse(BASE) + 3_600_000).toISOString(),
            dedupeKey: 'restart-reclaim-follow-up'
        });
        const crashed = runtime.repositories.job.claim({leaseOwner: 'crashed-process', leaseMs: 1_000, now: BASE, jobTypes: ['pending_event']});
        assert.equal(crashed.id, created.jobId);
        now = new Date(Date.parse(BASE) + 5_000).toISOString();
        await runtime.stop();

        queuedDecisions.push({schemaVersion: 1, send: true, reason: 'recovered', message: '重启后我仍然记得这件事。'});
        runtime = createRuntime({
            workerRuntime: undefined,
            workerOptions: {leaseOwner: 'restarted-worker', clock: () => now, runOnStart: false, pollIntervalMs: 60_000}
        });
        await runtime.start({listen: false, worker: true});
        assert.equal(runtime.repositories.job.find({id: created.jobId}).status, 'queued');
        const claimed = runtime.repositories.job.claim({leaseOwner: 'restarted-worker', leaseMs: 60_000, now, jobTypes: ['pending_event']});
        const tick = await runtime.jobDispatcher.runJob(claimed, {leaseOwner: 'restarted-worker', leaseMs: 60_000, now});
        assert.equal(tick.status, 'complete');
        assert.equal(runtime.repositories.job.find({id: created.jobId}).status, 'complete');
        assert.equal(runtime.database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'consumed');
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE proactive_pending_event_id = ?').get(created.pendingEvent.id).count, 1);
        await runtime.stop();
    } finally {
        if (runtime.state !== 'stopped') await runtime.stop();
    }
});
