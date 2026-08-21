import assert from 'node:assert/strict';
import test from 'node:test';

import {createProactiveSettlementFixture} from './fixtures/proactive-job-settlement-composition-fixture.mjs';
import {cleanup as cleanupLegacy, state as legacyState} from './fixtures/legacy-proactive-settlement-domain-fixture.mjs';

test('generic dispatcher settles proactive flows once and rejects stale leases', async () => {
    const fixture = createProactiveSettlementFixture();
    const completed = await fixture.dispatcher.runJob(fixture.job, fixture.context);
    assert.equal(completed.status, 'complete');
    assert.deepEqual(completed.result, {
        skipped: 'decision_send_false',
        causationId: 'cause_fixture'
    });
    assert.equal(fixture.job.status, 'complete');
    assert.equal(fixture.calls.filter(([kind]) => kind === 'settle').length, 1);
    assert.equal(fixture.flowCalls(), 1);

    const stale = await fixture.dispatcher.runJob({
        ...fixture.job,
        status: 'leased',
        lease_owner: 'stale_worker',
        lease_expires_at: '2026-08-21T00:01:00.000Z'
    }, {
        ...fixture.context,
        leaseOwner: 'stale_worker'
    });
    assert.equal(stale.status, 'stale');
    assert.equal(stale.changed, false);
    assert.equal(fixture.calls.filter(([kind]) => kind === 'settle').length, 1);
    assert.equal(fixture.flowCalls(), 1);
});

test('legacy proactive domain settlement remains an explicit comparison fixture', () => {
    const {database, createPersona, createEvent, deletePersona, appendMessage, completeProactiveMessageJob} = legacyState();
    const persona = createPersona({name: '租约回归', role: '测试人格', foundation: '用于验证主动消息作业的租约结算。'});
    try {
        const oldMessage = appendMessage(persona.id, {role: 'user', text: '今天见。'});
        database.prepare('UPDATE companion_messages SET created_at = ? WHERE id = ?').run(new Date(Date.now() - 11 * 60 * 1000).toISOString(), oldMessage.id);
        const event = createEvent(persona, {
            type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路'
        }, {proactive: true, publish: false, source: 'test'});
        const job = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(persona.id);
        assert.ok(job);
        const owner = 'lease_proactive_regression';
        const leaseExpiresAt = new Date(Date.now() + 60_000).toISOString();
        database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run(owner, leaseExpiresAt, job.id);

        const completed = completeProactiveMessageJob({...job, status: 'leased', lease_owner: owner, lease_expires_at: leaseExpiresAt}, {
            schemaVersion: 1, send: false, reason: 'regression', message: ''
        });
        assert.equal(completed.completed, true);
        assert.equal(completed.result.skipped, 'decision_send_false');
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'complete');
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE proactive_event_id = ?').get(event.eventId).count, 0);

        const stale = completeProactiveMessageJob({...job, status: 'leased', lease_owner: 'stale_owner', lease_expires_at: leaseExpiresAt}, {
            schemaVersion: 1, send: true, reason: 'stale', message: '不应发送。'
        });
        assert.equal(stale.completed, false);
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'complete');
    } finally {
        deletePersona(persona.id);
    }
});

test.after(() => cleanupLegacy());
