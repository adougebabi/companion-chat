import assert from 'node:assert/strict';
import test from 'node:test';

import {createTimelineFlow} from '../server/application/timeline-flow.js';
import {createProactiveJobService} from '../server/application/proactive-job-service.js';

const NOW = '2026-08-23T08:00:00.000Z';
const SLOT_START = '2026-08-23T09:00:00.000Z';
const SLOT_END = '2026-08-23T10:00:00.000Z';

function timelineFixture() {
    const jobs = [];
    const decisions = new Map();
    const slots = new Map();
    const eventRecords = [];
    let idCounter = 0;
    const nextId = prefix => `${prefix}_fixture_${++idCounter}`;
    const jobRepository = {
        enqueue(input) {
            const job = {...input, payload: {...input.payload}};
            jobs.push(job);
            return job;
        },
        findByPayload({jobType, value}) {
            return jobs.find(job => job.jobType === jobType && job.payload?.idempotencyKey === value);
        }
    };
    const timelineRepository = {
        findByDecisionKey({personaId, decisionKey}) {
            return decisions.get(`${personaId}:${decisionKey}`) ?? null;
        },
        insertDecision(input) {
            const row = {...input};
            decisions.set(`${input.personaId}:${input.decisionKey}`, row);
            return row;
        },
        updateDecision(input) {
            const key = `${input.personaId}:${decisions.get(`${input.personaId}:${input.decisionId}`)?.decisionKey ?? ''}`;
            const row = [...decisions.values()].find(item => item.id === input.decisionId && item.personaId === input.personaId);
            if (!row) return null;
            Object.assign(row, input);
            decisions.set(key, row);
            return row;
        },
        findByKey({personaId, planDate, slotKey}) {
            return slots.get(`${personaId}:${planDate}:${slotKey}`) ?? null;
        },
        upsertSlot(input) {
            const row = {...input};
            slots.set(`${input.personaId}:${input.planDate}:${input.slotKey}`, row);
            return row;
        },
        deleteGeneratedSlots() {},
        updateSlot(input) {
            const row = [...slots.values()].find(item => item.id === input.id && item.personaId === input.personaId);
            if (!row) return null;
            Object.assign(row, input);
            return row;
        }
    };
    const lifeEventFlow = {
        record(input) {
            eventRecords.push(input);
            if (input.proactive) {
                jobRepository.enqueue({
                    id: nextId('proactive'),
                    jobType: 'proactive_message',
                    personaId: input.personaId,
                    runAfter: input.occurredAt,
                    payload: {eventId: input.eventId, personaId: input.personaId}
                });
            }
            return {eventId: input.eventId, event: {id: input.eventId, personaId: input.personaId, payload: input}};
        }
    };
    const flow = createTimelineFlow({
        repositories: {
            eventDecisionRepository: timelineRepository,
            timelineSlotRepository: timelineRepository,
            job: jobRepository
        },
        lifeEventFlow,
        clock: () => NOW,
        idGenerator: nextId
    });
    return {flow, jobs, eventRecords};
}

test('ready daily plans schedule one idempotent candidate and an LLM-gated proactive job at due time', async () => {
    const {flow, jobs, eventRecords} = timelineFixture();
    const input = {
        personaId: 'persona_fixture',
        planDate: '2026-08-23',
        at: NOW,
        plan: {
            status: 'ready',
            items: [{slotKey: 'slot-1', title: '普通安排', situation: '在工作室处理手边的事', scene: '工作室', startsAt: SLOT_START, endsAt: SLOT_END}]
        }
    };

    const first = flow.syncDailyPlanSlots(input);
    const replay = flow.syncDailyPlanSlots(input);
    assert.equal(first.effects.length, 1);
    assert.equal(replay.effects.length, 1);
    assert.equal(jobs.length, 1);
    assert.equal(jobs[0].jobType, 'timeline_candidate');
    assert.equal(jobs[0].runAfter, SLOT_START);
    assert.equal(jobs[0].payload.slotKey, 'slot-1');
    assert.equal(jobs[0].payload.candidate.situation, '在工作室处理手边的事');

    const result = await flow.handleJob(jobs[0], {now: SLOT_START});
    assert.equal(result.effects.length, 0);
    assert.equal(jobs.length, 2);
    assert.equal(eventRecords.length, 1);
    assert.equal(eventRecords[0].proactive, true);
    assert.equal(eventRecords[0].requestActivityDecision, false);
    assert.equal(jobs[1].jobType, 'proactive_message');
});

test('legacy timeline.activity_decision jobs use the canonical activity flow registration', async () => {
    const seen = [];
    const service = createProactiveJobService({
        flows: {
            activity_decision(command) {
                seen.push(command.type);
                return {status: 'complete', result: {eventId: command.payload.eventId}};
            }
        }
    });
    const result = await service.run('timeline.activity_decision', {
        id: 'job_legacy_timeline_activity',
        job_type: 'timeline.activity_decision',
        persona_id: 'persona_fixture',
        payload_json: JSON.stringify({eventId: 'event_fixture'})
    }, {now: NOW, leaseOwner: 'worker_fixture'});

    assert.deepEqual(result, {status: 'complete', result: {eventId: 'event_fixture'}});
    assert.deepEqual(seen, ['activity_decision']);
    assert.equal(service.has('timeline.activity_decision'), true);
});
