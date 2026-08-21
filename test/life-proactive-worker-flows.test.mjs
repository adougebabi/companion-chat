import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {
    createActivityDecisionFlow,
    createDeferredChatReplyFlow,
    createPendingEventWorkerFlow,
    createProactiveMessageFlow
} from '../server/application/proactive/worker-flows.js';

const NOW = '2026-08-21T08:00:00.000Z';
const LATER = '2026-08-21T09:00:00.000Z';

function command(type, payload, overrides = {}) {
    return {
        type,
        jobId: `job_${type}`,
        personaId: 'persona_1',
        payload,
        job: {id: `job_${type}`, job_type: type, payload_json: JSON.stringify(payload)},
        ...overrides
    };
}

function decisionPort({decision, calls}) {
    let frozen = null;
    return {
        readFrozen() {
            calls.push('readFrozen');
            return frozen;
        },
        evaluate(input) {
            calls.push(['evaluate', input.kind]);
            return decision;
        },
        freeze(input) {
            calls.push(['freeze', input.kind]);
            frozen = input.decision;
            return {changed: true, decision: frozen};
        }
    };
}

function replyPort(calls, {existing = null} = {}) {
    return {
        findDelivery(input) {
            calls.push(['findDelivery', input.source.sourceType || 'batch']);
            return existing;
        },
        project(input) {
            calls.push(['project', input.source]);
            return [
                {id: `message_${calls.filter(([name]) => name === 'project').length}_1`, role: 'assistant', text: input.text},
                {id: `message_${calls.filter(([name]) => name === 'project').length}_2`, role: 'assistant', text: '第二句。'}
            ];
        }
    };
}

test('pending worker transitions due events, freezes the decision, and projects ordered assistant messages', async () => {
    const calls = [];
    const pending = {id: 'pending_1', persona_id: 'persona_1', status: 'pending', summary: '下班后提醒', not_before: NOW, expires_at: LATER};
    const transitions = [];
    const pendingPort = {
        findById() { return pending; },
        transition(input) {
            transitions.push(input);
            pending.status = input.to;
            return pending;
        }
    };
    const flow = createPendingEventWorkerFlow({
        pendingEvent: pendingPort,
        decision: decisionPort({calls, decision: {schemaVersion: 1, send: true, reason: 'relevant', message: '我想起你刚才说的事了。'}}),
        reply: replyPort(calls),
        clock: () => NOW
    });

    const output = await flow.run(command('pending_event', {pendingEventId: pending.id}), {now: NOW});

    assert.equal(output.status, 'complete');
    assert.deepEqual(output.result.messageIds, ['message_1_1', 'message_1_2']);
    assert.equal(pending.status, 'consumed');
    assert.deepEqual(transitions.map(item => [item.from, item.to]), [['pending', 'triggered'], ['triggered', 'consumed']]);
    assert.deepEqual(calls.filter(item => Array.isArray(item) && ['evaluate', 'freeze'].includes(item[0])), [
        ['evaluate', 'proactive_message'], ['freeze', 'proactive_message']
    ]);
    assert.equal(output.projections.find(item => item.type === 'assistant_reply').messageId, 'message_1_1');
    assert.equal(output.presentation[0].messages.length, 2);
});

test('pending worker expires before evaluation and does not project a reply', async () => {
    const calls = [];
    const pending = {id: 'pending_expired', persona_id: 'persona_1', status: 'pending', summary: 'expired', not_before: '2026-08-20T08:00:00.000Z', expires_at: '2026-08-21T07:59:59.000Z'};
    const pendingPort = {
        findById() { return pending; },
        transition(input) { pending.status = input.to; return pending; }
    };
    const flow = createPendingEventWorkerFlow({
        pendingEvent: pendingPort,
        decision: decisionPort({calls, decision: {schemaVersion: 1, send: true, reason: 'must not run', message: '不可见。'}}),
        reply: replyPort(calls),
        clock: () => NOW
    });

    const output = await flow.run(command('pending_event', {pendingEventId: pending.id}), {now: NOW});

    assert.deepEqual(output.result, {skipped: 'expired', pendingEventId: pending.id});
    assert.equal(pending.status, 'expired');
    assert.equal(calls.some(item => Array.isArray(item) && item[0] === 'evaluate'), false);
    assert.equal(calls.some(item => Array.isArray(item) && item[0] === 'project'), false);
});

test('proactive life-event retry reuses the frozen decision and does not evaluate again', async () => {
    const calls = [];
    const decisions = decisionPort({calls, decision: {schemaVersion: 1, send: true, reason: 'check in', message: '今天过得还顺利吗？'}});
    const flow = createProactiveMessageFlow({
        lifeEvent: {findById() { return {id: 'event_1', persona_id: 'persona_1', type: 'social', payload_json: '{"situation":"在咖啡馆"}'}; }},
        decision: decisions,
        reply: replyPort(calls),
        clock: () => NOW
    });
    const job = command('proactive_message', {eventId: 'event_1'});

    const first = await flow.run(job, {now: NOW});
    const second = await flow.run(job, {now: NOW});

    assert.equal(first.result.messageId, 'message_1_1');
    assert.equal(second.result.messageId, 'message_2_1');
    assert.equal(calls.filter(item => Array.isArray(item) && item[0] === 'evaluate').length, 1);
    assert.equal(calls.filter(item => Array.isArray(item) && item[0] === 'freeze').length, 1);
    assert.equal(calls.filter(item => Array.isArray(item) && item[0] === 'project').length, 2);
});

test('activity decision publishes a projection and emits a media effect intent without calling a provider', async () => {
    const calls = [];
    const flow = createActivityDecisionFlow({
        lifeEvent: {findById() { return {id: 'event_2', persona_id: 'persona_1', type: 'personal', payload_json: '{"situation":"完成了计划"}'}; }},
        decision: decisionPort({calls, decision: {publish: true, content: '今天终于完成计划了。', media: {kind: 'image', count: 1}}}),
        activity: {
            findByEvent() { return null; },
            publish(input) { calls.push(['publish', input]); return {id: input.id, ...input}; }
        },
        idGenerator: prefix => `${prefix}_1`,
        clock: () => NOW
    });

    const output = await flow.run(command('activity_decision', {eventId: 'event_2'}), {now: NOW});

    assert.deepEqual(output.result, {eventId: 'event_2', activityId: 'activity_1', published: true, media: 'image'});
    assert.equal(output.effects.length, 1);
    assert.equal(output.effects[0].capability, 'media_event');
    assert.equal(output.effects[0].payload.activityId, 'activity_1');
    assert.equal(calls.filter(item => Array.isArray(item) && item[0] === 'publish').length, 1);
});

test('deferred reply reads due batch and life-world context before completing the batch', async () => {
    const calls = [];
    let completed = null;
    const flow = createDeferredChatReplyFlow({
        deferredBatch: {
            findById() { return {id: 'batch_1', persona_id: 'persona_1', status: 'queued', deliver_at: NOW, message_ids_json: '["user_1"]'}; },
            complete(input) { completed = input; return {id: input.batchId, status: 'complete'}; }
        },
        conversation: {listByIds(input) { calls.push(['messages', input]); return [{id: 'user_1', role: 'user', text: '早上好'}]; }},
        lifeWorld: {read(input) { calls.push(['lifeWorld', input]); return {currentTime: input.at, situation: '在窗边'}; }},
        replyComposer: {compose(input) { calls.push(['compose', input]); return '醒来后第一句话就看到你了。'; }},
        reply: replyPort(calls),
        clock: () => NOW
    });

    const output = await flow.run(command('deferred_chat_reply', {batchId: 'batch_1'}), {now: NOW});

    assert.deepEqual(output.result.messageIds, ['message_1_1', 'message_1_2']);
    assert.equal(completed.status, 'queued');
    assert.equal(calls.some(item => item[0] === 'lifeWorld'), true);
    assert.equal(calls.find(item => item[0] === 'compose')[1].lifeWorld.situation, '在窗边');
    assert.equal(output.facts[0].type, 'deferred_reply_completed');
});

test('worker flow modules remain application-only and do not recreate SQL/provider/lease behavior', async () => {
    const [ports, flows] = await Promise.all([
        readFile(new URL('../server/application/proactive/flow-ports.js', import.meta.url), 'utf8'),
        readFile(new URL('../server/application/proactive/worker-flows.js', import.meta.url), 'utf8')
    ]);
    const source = `${ports}\n${flows}`;
    assert.doesNotMatch(source, /better-sqlite3|server\.js|fetch\s*\(|child_process|lease_owner|lease_expires_at|jobRepository\.settle/);
});

test('worker definitions use the canonical registry ids consumed by the generic job service', () => {
    const decision = decisionPort({calls: [], decision: {schemaVersion: 1, send: false, reason: 'fixture', message: ''}});
    assert.equal(createPendingEventWorkerFlow({pendingEvent: {findById() {}, transition() {}}, decision, reply: replyPort([])}).id, 'pending-event');
    assert.equal(createProactiveMessageFlow({lifeEvent: {findById() {}}, decision, reply: replyPort([])}).id, 'proactive-message');
    assert.equal(createActivityDecisionFlow({lifeEvent: {findById() {}}, decision, activity: {publish() {}}}).id, 'activity-decision');
    assert.equal(createDeferredChatReplyFlow({deferredBatch: {findById() {}, complete() {}}, conversation: {listByIds() {}}, reply: replyPort([]), replyComposer: {compose() { return ''; }}}).id, 'deferred-chat-reply');
});
