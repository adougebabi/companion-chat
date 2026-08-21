import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';
import Database from 'better-sqlite3';
import {createCompanionRuntime} from '../server/runtime/runtime.js';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-life-state-'));
const runtime = createCompanionRuntime({Database, dataDir, workerRuntime: false, environment: {DATA_DIR: dataDir}});
const {database} = runtime;
const services = runtime.application.services;
const createPersona = input => services.persona.create(input);
const deletePersona = personaId => services.persona.delete({personaId});
const resolvedStateFor = (personaId, at) => services.life.resolvedStateFor({personaId, at});
const stateShape = (personaId, at) => services.life.stateShape({personaId, at});
const scheduledState = (persona, at) => services.life.scheduledState({personaId: persona.id, at});
const sleepAvailability = (persona, at) => services.life.sleepAvailability({personaId: persona.id, at});
const reconcilePersona = (personaId, options = {}) => services.life.reconcilePersona({personaId, at: options.at});
const recoverPersona = personaId => services.life.recoverPersona({personaId});

test.after(async () => {
    await runtime.stop();
    rmSync(dataDir, {recursive: true, force: true});
});

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

test('state and sleep projections use shared-scene precedence without changing scheduledState compatibility', () => {
    const persona = createPersona({name: 'Projection adapter', role: 'companion', foundation: 'Integration fixture.'});
    try {
        const startsAt = '2023-12-31T17:00:00.000Z';
        const endsAt = '2023-12-31T19:00:00.000Z';
        database.prepare('INSERT INTO companion_schedule_items (id, persona_id, kind, title, starts_at, ends_at, status, source, details_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-shared-schedule', persona.id, 'plan', 'legacy schedule', startsAt, endsAt, 'active', 'explicit_chat_plan',
            JSON.stringify({sceneRef: {locationId: 'home', roomId: 'private_room'}, situation: 'legacy schedule should yield to the shared scene'}),
            startsAt, startsAt
        );
        const at = new Date('2023-12-31T18:00:00.000Z');
        const legacy = scheduledState(persona, at);
        assert.equal(legacy.source, 'schedule');
        assert.equal(legacy.endsAt, endsAt);

        database.prepare('UPDATE companion_persona_states SET shared_scene_json = ? WHERE persona_id = ?').run(JSON.stringify({
            eventId: 'integration-shared-scene', location: '湖边公园', room: '湖边步道', activity: '一起散步',
            situation: '正在湖边公园一起散步', startedAt: '2023-12-31T17:30:00.000Z'
        }), persona.id);
        database.prepare('INSERT INTO companion_life_events (id, persona_id, type, occurred_at, resolves_at, causation_id, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)').run(
            'integration-shared-scene', persona.id, 'shared_scene', '2023-12-31T17:30:00.000Z', null, null,
            JSON.stringify({operation: 'start', location: '湖边公园', room: '湖边步道', activity: '一起散步', situation: '正在湖边公园一起散步'}),
            '2023-12-31T17:30:00.000Z'
        );
        database.prepare('UPDATE companion_persona_states SET source_event_id = ? WHERE persona_id = ?').run('integration-shared-scene', persona.id);

        const projected = stateShape(persona.id, at);
        assert.equal(projected.sharedScene.eventId, 'integration-shared-scene');
        assert.equal(projected.source.kind, 'shared_scene');
        assert.equal(projected.sourceId, 'integration-shared-scene');
        assert.equal(projected.location, '湖边公园');
        assert.equal(projected.timeFact, 'unknown');
        assert.equal(projected.endsAt, null);

        const availability = sleepAvailability(persona, at);
        assert.equal(availability.sleeping, true);
        assert.equal(availability.nextBoundaryAt, null);

        const eventCountBeforeReconcile = database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(persona.id).count;
        const reconciled = reconcilePersona(persona.id, {publish: false});
        assert.equal(reconciled.source, 'shared_scene');
        assert.equal(reconciled.sharedScene.eventId, 'integration-shared-scene');
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(persona.id).count, eventCountBeforeReconcile);

        database.prepare('UPDATE companion_persona_states SET checkpoint_at = ? WHERE persona_id = ?').run('2023-12-31T17:00:00.000Z', persona.id);
        const recovered = recoverPersona(persona.id);
        assert.equal(recovered.source, 'shared_scene');
        assert.equal(recovered.sharedScene.eventId, 'integration-shared-scene');
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ?').get(persona.id).count, eventCountBeforeReconcile);
    } finally {
        deletePersona(persona.id);
    }
});

test('stateShape keeps appearance expiry and trusted source metadata on read', () => {
    const persona = createPersona({name: 'Expiry projection', role: 'companion', foundation: 'Integration fixture.'});
    try {
        const eventId = 'integration-expiring-appearance';
        const startsAt = '2024-01-01T03:15:00.000Z';
        const endsAt = '2024-01-01T04:15:00.000Z';
        database.prepare('INSERT INTO companion_life_events (id, persona_id, type, occurred_at, resolves_at, causation_id, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)').run(
            eventId, persona.id, 'social', startsAt, endsAt, null,
            JSON.stringify({situation: 'wearing a temporary coat', mood: 'calm', scene: 'park', appearance: {coat: 'blue'}, rationale: 'integration fixture'}),
            startsAt
        );
        database.prepare('UPDATE companion_persona_states SET situation = ?, mood = ?, appearance_json = ?, source_event_id = ?, checkpoint_at = ?, updated_at = ? WHERE persona_id = ?').run(
            'wearing a temporary coat', 'calm', JSON.stringify({coat: 'blue'}), eventId, startsAt, startsAt, persona.id
        );

        const before = stateShape(persona.id, new Date('2024-01-01T03:30:00.000Z'));
        assert.deepEqual(before.appearance, {coat: 'blue'});
        assert.equal(before.source.kind, 'social');
        assert.equal(before.source.eventId, eventId);
        assert.equal(before.startsAt, startsAt);
        assert.equal(before.endsAt, endsAt);
        assert.equal(before.timeFact, 'known');

        const after = stateShape(persona.id, new Date('2024-01-01T04:30:00.000Z'));
        assert.deepEqual(after.appearance, {});
        assert.equal(database.prepare('SELECT appearance_json FROM companion_persona_states WHERE persona_id = ?').get(persona.id).appearance_json, '{}');
    } finally {
        deletePersona(persona.id);
    }
});
