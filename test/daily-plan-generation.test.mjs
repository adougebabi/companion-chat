import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';

import {createPersonaLifecycleRepository} from '../server/infrastructure/persona-lifecycle-repository.js';
import {createCompanionRuntime} from '../server/runtime/runtime.js';

function createRuntime() {
    const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-daily-plan-'));
    const runtime = createCompanionRuntime({
        Database,
        dataDir,
        workerRuntime: false,
        environment: {DATA_DIR: dataDir}
    });
    return {runtime, dataDir};
}

async function closeRuntime({runtime, dataDir}) {
    await runtime.stop();
    rmSync(dataDir, {recursive: true, force: true});
}

function defaultPersona(runtime, input = {}) {
    return runtime.application.services.persona.create({
        name: input.name ?? 'Daily plan test',
        role: 'companion',
        foundation: 'Daily plan generation fixture.',
        createdAt: input.createdAt ?? '2026-08-22T01:00:00.000Z',
        ...(input.blueprint ? {blueprint: input.blueprint} : {})
    });
}

test('daily plan maintenance catches up local dates and replays ensure idempotently', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const persona = defaultPersona(runtime);
        const job = runtime.database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id);
        const claimed = runtime.jobRepository.claim({
            personaId: persona.id,
            jobTypes: ['daily_plan'],
            now: '2026-08-24T02:00:00.000Z',
            leaseOwner: 'daily-plan-test',
            leaseMs: 60_000
        });
        assert.equal(claimed.id, job.id);
        const result = await runtime.jobDispatcher.runJob(claimed, {
            now: '2026-08-24T02:00:00.000Z',
            leaseOwner: 'daily-plan-test',
            owner: 'daily-plan-test'
        });
        assert.equal(result.status, 'complete');
        assert.deepEqual(result.result.caughtUpDates, ['2026-08-22', '2026-08-23', '2026-08-24']);
        assert.equal(result.result.nextPlanDate, '2026-08-25');

        const plans = runtime.database.prepare('SELECT plan_date, status FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date').all(persona.id);
        assert.deepEqual(plans, [
            {plan_date: '2026-08-22', status: 'ready'},
            {plan_date: '2026-08-23', status: 'ready'},
            {plan_date: '2026-08-24', status: 'ready'},
            {plan_date: '2026-08-25', status: 'ready'}
        ]);
        assert.equal(runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_timeline_slots WHERE persona_id = ? AND source = 'daily_plan_baseline'").get(persona.id).count, 4);
        assert.equal(runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count, 4);
        const nextJob = runtime.database.prepare("SELECT run_after, payload_json FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan' AND json_extract(payload_json, '$.planDate') = '2026-08-25'").get(persona.id);
        assert.equal(nextJob.run_after, '2026-08-24T16:00:00.000Z');
        assert.equal(JSON.parse(nextJob.payload_json).idempotencyKey, `daily-plan:${persona.id}:2026-08-25`);

        const before = {
            plans: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get(persona.id).count,
            slots: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_timeline_slots WHERE persona_id = ?').get(persona.id).count,
            jobs: runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count
        };
        const replay = runtime.repositories.dailyPlan.ensure({personaId: persona.id, planDate: '2026-08-24', at: '2026-08-24T02:00:00.000Z'});
        assert.equal(replay.created, false);
        assert.deepEqual({
            plans: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get(persona.id).count,
            slots: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_timeline_slots WHERE persona_id = ?').get(persona.id).count,
            jobs: runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count
        }, before);
        assert.throws(() => runtime.repositories.dailyPlan.readById({personaId: '', dailyPlanId: planIdFor(runtime, persona.id)}), /Persona.id/);
    } finally {
        await closeRuntime(fixture);
    }
});

test('daily plan uses the blueprint timezone across DST and preserves explicit slots on replay', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const persona = defaultPersona(runtime, {
            name: 'DST daily plan',
            createdAt: '2026-10-31T15:00:00.000Z',
            blueprint: {
                timezone: 'America/New_York',
                world: {
                    defaultSceneRef: {locationId: 'home', roomId: 'room'},
                    locations: [{id: 'home', isDefault: true, name: 'Home', rooms: [{id: 'room', isDefault: true, name: 'Room', scene: 'A quiet room'}]}]
                }
            }
        });
        const plan = runtime.repositories.dailyPlan.ensure({personaId: persona.id, planDate: '2026-11-01', at: '2026-11-01T16:00:00.000Z'});
        const slot = runtime.database.prepare("SELECT starts_at, ends_at FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? AND source = 'daily_plan_baseline'").get(persona.id, '2026-11-01');
        assert.equal(slot.starts_at, '2026-11-01T04:00:00.000Z');
        assert.equal(slot.ends_at, '2026-11-02T05:00:00.000Z');
        assert.equal(runtime.repositories.dailyPlan.read({personaId: persona.id, at: '2026-11-02T04:30:00.000Z'}).plan_date, '2026-11-01');
        assert.equal(plan.status, 'ready');

        runtime.database.prepare(`INSERT INTO companion_timeline_slots
            (id, persona_id, plan_date, slot_key, slot_kind, starts_at, ends_at, status, source, priority, schedule_id, plan_revision, constraints_json, outcome_json, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run('explicit-daily-slot', persona.id, '2026-11-01', 'explicit-daily-slot', 'planned', '2026-11-01T15:00:00.000Z', '2026-11-01T16:00:00.000Z', 'confirmed', 'explicit_chat_plan', 3, null, null, '{}', '{}', '2026-11-01T16:00:00.000Z', '2026-11-01T16:00:00.000Z');
        runtime.application.timelineFlow.syncDailyPlanSlots({personaId: persona.id, planDate: '2026-11-01', plan: plan.plan, at: '2026-11-01T16:00:00.000Z'});
        assert.ok(runtime.database.prepare('SELECT id FROM companion_timeline_slots WHERE id = ? AND persona_id = ?').get('explicit-daily-slot', persona.id));
    } finally {
        await closeRuntime(fixture);
    }
});

test('persona creation applies its local timezone default to both plan and baseline', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const persona = defaultPersona(runtime, {
            name: 'Default timezone daily plan',
            createdAt: '2026-08-22T23:00:00.000Z',
            blueprint: {
                world: {
                    defaultSceneRef: {locationId: 'home', roomId: 'room'},
                    locations: [{id: 'home', isDefault: true, name: 'Home', rooms: [{id: 'room', isDefault: true, name: 'Room', scene: 'A quiet room'}]}]
                }
            }
        });
        const plan = runtime.database.prepare('SELECT id, plan_date, plan_json FROM companion_daily_plans WHERE persona_id = ?').get(persona.id);
        const slot = runtime.database.prepare("SELECT starts_at, ends_at FROM companion_timeline_slots WHERE persona_id = ? AND plan_date = ? AND source = 'daily_plan_baseline'").get(persona.id, plan.plan_date);
        const storedPlan = JSON.parse(plan.plan_json);

        assert.equal(plan.plan_date, '2026-08-23');
        assert.equal(storedPlan.timezone, 'Asia/Shanghai');
        assert.equal(storedPlan.timeline[0].startsAt, '2026-08-22T16:00:00.000Z');
        assert.equal(storedPlan.timeline[0].endsAt, '2026-08-23T16:00:00.000Z');
        assert.equal(slot.starts_at, storedPlan.timeline[0].startsAt);
        assert.equal(slot.ends_at, storedPlan.timeline[0].endsAt);
    } finally {
        await closeRuntime(fixture);
    }
});

test('invalid plan dates are rejected before a new plan or job is written', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const persona = defaultPersona(runtime);
        const before = {
            plans: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get(persona.id).count,
            jobs: runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count
        };

        assert.throws(
            () => runtime.repositories.dailyPlan.ensure({personaId: persona.id, planDate: '2026-02-31', at: '2026-02-01T00:00:00.000Z'}),
            /valid YYYY-MM-DD/
        );
        assert.deepEqual({
            plans: runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get(persona.id).count,
            jobs: runtime.database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count
        }, before);
    } finally {
        await closeRuntime(fixture);
    }
});

test('day-one plan and daily-plan job roll back together when enqueue fails', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const lifecycle = createPersonaLifecycleRepository({
            database: runtime.database,
            clock: () => '2026-08-24T00:00:00.000Z',
            id: prefix => `atomic-${prefix}`,
            jobRepository: {enqueue() { throw new Error('daily plan queue unavailable'); }}
        });

        assert.throws(
            () => lifecycle.createPersona({id: 'atomic-persona', name: 'Atomic plan', role: 'companion', foundation: 'fixture'}),
            /queue unavailable/
        );
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_personas WHERE id = ?').get('atomic-persona').count, 0);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get('atomic-persona').count, 0);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_timeline_slots WHERE persona_id = ?').get('atomic-persona').count, 0);
    } finally {
        await closeRuntime(fixture);
    }
});

test('an excessive catch-up span remains retryable without generating an unbounded chain', async () => {
    const fixture = createRuntime();
    const {runtime} = fixture;
    try {
        const persona = defaultPersona(runtime);
        const job = runtime.database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id);
        const payload = JSON.parse(job.payload_json);
        payload.planDate = '2020-01-01';
        runtime.database.prepare('UPDATE companion_jobs SET payload_json = ? WHERE id = ?').run(JSON.stringify(payload), job.id);

        const claimed = runtime.jobRepository.claim({
            personaId: persona.id,
            jobTypes: ['daily_plan'],
            now: '2026-08-24T02:00:00.000Z',
            leaseOwner: 'bounded-catch-up-test',
            leaseMs: 60_000
        });
        const result = await runtime.jobDispatcher.runJob(claimed, {
            now: '2026-08-24T02:00:00.000Z',
            leaseOwner: 'bounded-catch-up-test',
            owner: 'bounded-catch-up-test'
        });

        assert.equal(result.status, 'retry');
        assert.match(result.error, /catch-up span exceeds limit/);
        assert.equal(runtime.database.prepare('SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ?').get(persona.id).count, 1);
        assert.equal(runtime.database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'queued');
    } finally {
        await closeRuntime(fixture);
    }
});
