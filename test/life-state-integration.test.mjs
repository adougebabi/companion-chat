import assert from 'node:assert/strict';
import {mkdtempSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-life-state-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
const {companionTestHooks: hooks} = await import(`../server.js?life-state-integration=${Date.now()}`);
const {database, createPersona, deletePersona, resolvedStateFor, stateShape} = hooks;

test('resolvedStateFor uses explicit Asia/Shanghai time through the pure resolver adapter', () => {
    const persona = createPersona({name: 'Life resolver', role: 'companion', foundation: 'Integration fixture.'});
    try {
        const plan = database.prepare('SELECT id FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(persona.id);
        database.prepare("UPDATE companion_daily_plans SET plan_date = ?, status = 'ready', plan_json = ?, source = 'test', updated_at = ? WHERE id = ?").run(
            '2024-01-01',
            JSON.stringify([{title: 'sleep', situation: 'sleeping before the day begins', scene: '宿舍', startsAt: '10:00', endsAt: '13:00'}]),
            '2024-01-01T00:00:00.000Z',
            plan.id
        );
        database.prepare('INSERT INTO companion_timeline_slots (id, persona_id, plan_date, slot_key, slot_kind, starts_at, ends_at, status, source, priority, schedule_id, plan_revision, constraints_json, outcome_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-plan-slot', persona.id, '2024-01-01', `${plan.id}:baseline:before`, 'baseline_sleep',
            '2023-12-31T16:00:00.000Z', '2024-01-01T02:00:00.000Z', 'active', 'daily_plan_baseline', 1, null, 1, '{}', '{}',
            '2024-01-01T00:00:00.000Z', '2024-01-01T00:00:00.000Z'
        );

        const beforePlan = resolvedStateFor(persona.id, new Date('2024-01-01T00:47:00.000Z'));
        assert.equal(beforePlan.source, 'daily_plan_baseline');
        assert.equal(beforePlan.resolved_source_id, 'integration-plan-slot');
        assert.equal(beforePlan.endsAt, '2024-01-01T02:00:00.000Z');
        assert.equal(beforePlan.timeFact, 'known');

        database.prepare('INSERT INTO companion_schedule_items (id, persona_id, kind, title, starts_at, ends_at, status, source, details_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-schedule', persona.id, 'plan', 'explicit meeting',
            '2024-01-01T03:00:00.000Z', '2024-01-01T04:00:00.000Z', 'active', 'explicit_chat_plan',
            JSON.stringify({scene: 'custom cafe', situation: 'meeting at a custom cafe'}),
            '2024-01-01T00:00:00.000Z', '2024-01-01T00:00:00.000Z'
        );
        const scheduleState = resolvedStateFor(persona.id, new Date('2024-01-01T03:30:00.000Z'));
        assert.equal(scheduleState.source, 'schedule');
        assert.equal(scheduleState.location, 'custom cafe');
        assert.equal(scheduleState.endsAt, '2024-01-01T04:00:00.000Z');

        database.prepare('INSERT INTO companion_life_events (id, persona_id, type, occurred_at, resolves_at, causation_id, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-event', persona.id, 'social', '2024-01-01T03:15:00.000Z', '2024-01-01T04:15:00.000Z', null,
            JSON.stringify({situation: 'talking with a friend', mood: 'calm', scene: 'park', appearance: {coat: 'blue'}}),
            '2024-01-01T03:15:00.000Z'
        );
        const eventState = resolvedStateFor(persona.id, new Date('2024-01-01T03:30:00.000Z'));
        assert.equal(eventState.source, 'event');
        assert.equal(eventState.situation, 'talking with a friend');
        assert.deepEqual(JSON.parse(eventState.appearance_json), {coat: 'blue'});

        database.prepare('UPDATE companion_persona_states SET shared_scene_json = ? WHERE persona_id = ?').run(JSON.stringify({
            eventId: 'integration-scene', location: 'shared park', room: '', activity: 'walking together',
            situation: 'walking together in the shared park', startedAt: '2024-01-01T03:20:00.000Z'
        }), persona.id);
        const sharedState = resolvedStateFor(persona.id, new Date('2024-01-01T03:30:00.000Z'));
        assert.equal(sharedState.source, 'shared_scene');
        assert.equal(sharedState.location, 'shared park');
        assert.equal(sharedState.source_event_id, 'integration-scene');
    } finally {
        deletePersona(persona.id);
    }
});

test('resolvedStateFor preserves temporary appearance aliases at an explicit time', () => {
    const persona = createPersona({name: 'Appearance alias', role: 'companion', foundation: 'Integration fixture.'});
    try {
        database.prepare('INSERT INTO companion_life_events (id, persona_id, type, occurred_at, resolves_at, causation_id, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-alias-event', persona.id, 'social', '2024-01-01T03:15:00.000Z', '2024-01-01T04:15:00.000Z', null,
            JSON.stringify({situation: 'wearing a temporary coat', mood: 'calm', scene: 'park', temporaryAppearance: {coat: 'blue'}}),
            '2024-01-01T03:15:00.000Z'
        );
        const at = new Date('2024-01-01T03:30:00.000Z');
        const resolved = resolvedStateFor(persona.id, at);
        assert.deepEqual(JSON.parse(resolved.appearance_json), {coat: 'blue'});
        assert.deepEqual(stateShape(persona.id, at).appearance, {coat: 'blue'});
    } finally {
        deletePersona(persona.id);
    }
});
