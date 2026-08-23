import assert from 'node:assert/strict';
import test from 'node:test';

import {createDeferredChatPolicy} from '../server/application/deferred-chat-policy.js';

const NOW = '2026-08-23T15:00:00.000Z';

function fixture({sleepDecision, sleepAvailability = () => ({sleeping: true, intimacy: 2, draw: 50})} = {}) {
    let created = null;
    const policy = createDeferredChatPolicy({
        deferredBatch: {
            create(input) {
                created = input;
                return {batch: {id: input.id, deliverAt: input.deliverAt}, job: {id: input.job.id}};
            }
        },
        conversationRepository: {getConversation: () => ({id: 'conversation_1'})},
        sleepAvailability,
        sleepDecision,
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_1`
    });
    return {policy, get created() { return created; }};
}

test('ordinary rest does not invoke sleep decision or create a deferred batch', async () => {
    let decisions = 0;
    const fixtureValue = fixture({sleepAvailability: () => ({sleeping: false, immediate: true}), sleepDecision: async () => { decisions += 1; return {immediate: false}; }});
    const result = await fixtureValue.policy.evaluate({personaId: 'persona_1', text: '你好', userMessage: {id: 'message_1', text: '你好'}});
    assert.equal(result.handled, false);
    assert.equal(decisions, 0);
    assert.equal(fixtureValue.created, null);
});

test('sleeping chat delegates immediate-or-deferred choice to the LLM decision port', async () => {
    let received;
    const fixtureValue = fixture({sleepDecision: async input => {
        received = input;
        return {immediate: false, deliverAt: '2026-08-23T15:20:00.000Z'};
    }});
    const result = await fixtureValue.policy.evaluate({personaId: 'persona_1', text: '你醒了吗', userMessage: {id: 'message_1', text: '你醒了吗'}});
    assert.equal(result.kind, 'deferred');
    assert.equal(received.availability.intimacy, 2);
    assert.equal(fixtureValue.created.deliverAt, '2026-08-23T15:20:00.000Z');
});

test('sleep decision failure falls back to the bounded availability decision', async () => {
    const fixtureValue = fixture({sleepDecision: async () => { throw new Error('provider unavailable'); }});
    const result = await fixtureValue.policy.evaluate({personaId: 'persona_1', text: '晚安', userMessage: {id: 'message_1', text: '晚安'}});
    assert.equal(result.kind, 'deferred');
    assert.ok(fixtureValue.created);
});
