import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';

import {resolveLifeState} from '../server/domain/life-state-resolver.js';
import {createLifeWorldReader} from '../server/application/life-world-reader.js';

const NOW = '2026-08-20T03:00:00.000Z';

function fixture(overrides = {}) {
    const calls = [];
    const state = {
        persona_id: 'persona_a',
        situation: '原来的状态',
        mood: '平静',
        appearance_json: JSON.stringify({coat: 'blue'}),
        shared_scene_json: JSON.stringify({eventId: 'scene_a', location: '湖边', activity: '散步', situation: '正在湖边散步'}),
        ...overrides.state
    };
    const repositories = {
        state: {read(personaId, input) { calls.push(['state', personaId, input]); return state; }},
        schedule: {list(input) {
            calls.push(['schedule', input]);
            return [{
                id: 'schedule_a', persona_id: 'persona_a', status: 'active', source: 'explicit_chat_plan',
                title: '去图书馆', starts_at: '2026-08-20T02:30:00.000Z', ends_at: '2026-08-20T03:30:00.000Z',
                details_json: JSON.stringify({situation: '在图书馆整理笔记', scene: '图书馆'})
            }, {id: 'schedule_b', persona_id: 'persona_b', status: 'active'}];
        }},
        lifeEvent: {listActive(input) {
            calls.push(['lifeEvent', input]);
            return [{
                id: 'event_a', persona_id: 'persona_a', type: 'social', occurred_at: '2026-08-20T02:00:00.000Z',
                resolves_at: '2026-08-20T04:00:00.000Z', payload_json: JSON.stringify({situation: '和朋友聊天'})
            }, {id: 'event_b', persona_id: 'persona_b', type: 'social'}];
        }},
        dailyPlan: {findReady(input) {
            calls.push(['dailyPlan', input]);
            return {
                id: 'plan_a', persona_id: 'persona_a', plan_date: '2026-08-20', status: 'ready',
                plan_json: JSON.stringify([{title: '学习', situation: '在图书馆学习', startsAt: '10:00', endsAt: '12:00'}])
            };
        }},
        presence: {read(personaId, input) {
            calls.push(['presence', personaId, input]);
            return {persona_id: 'persona_a', shared_scene_json: state.shared_scene_json};
        }}
    };
    const reader = createLifeWorldReader({
        repositories: {...repositories, ...overrides.repositories},
        blueprintReader: overrides.blueprintReader ?? (() => ({
            personaId: 'persona_a', timezone: 'Asia/Shanghai', mood: '平静',
            world: {defaultSceneRef: 'home:bedroom', locations: [{id: 'home', name: '家中', rooms: [{id: 'bedroom', name: '卧室', scene: '卧室'}]}]},
            routine: [{from: 0, to: 24, label: '在自己的空间里休息'}]
        })),
        sceneReader: overrides.sceneReader ?? ((personaId, input) => {
            calls.push(['scene', personaId, input]);
            return input.row;
        }),
        clock: overrides.clock ?? (() => NOW)
    });
    return {reader, calls};
}

test('reader passes one explicit time and persona scope to every repository', () => {
    const {reader, calls} = fixture();
    const input = reader.readResolverInput('persona_a', new Date(NOW));

    assert.equal(input.personaId, 'persona_a');
    assert.equal(input.currentTime.toISOString(), NOW);
    assert.deepEqual(input.scheduleItems.map(row => row.id), ['schedule_a']);
    assert.deepEqual(input.lifeEvents.map(row => row.id), ['event_a']);
    assert.deepEqual(input.dailyPlan.items, [{title: '学习', situation: '在图书馆学习', startsAt: '10:00', endsAt: '12:00'}]);
    assert.equal(input.presence.eventId, 'scene_a');
    for (const call of calls.filter(item => ['schedule', 'lifeEvent', 'dailyPlan'].includes(item[0]))) {
        assert.equal(call[1].personaId, 'persona_a');
        assert.equal(call[1].at, NOW);
    }
    assert.equal(calls.filter(item => item[0] === 'state')[0][1], 'persona_a');
    assert.equal(calls.filter(item => item[0] === 'presence')[0][1], 'persona_a');
});

test('row normalization decodes state, schedule, event, plan, and presence JSON without mutating rows', () => {
    const rawState = {persona_id: 'persona_a', appearance_json: '{"hat":"red"}', shared_scene_json: '{}'};
    const rawSchedule = {persona_id: 'persona_a', details_json: '{"scene":"cafe"}', starts_at: NOW};
    const rawEvent = {persona_id: 'persona_a', payload_json: '{"situation":"reading"}', occurred_at: NOW};
    const rawPlan = {persona_id: 'persona_a', plan_date: '2026-08-20', status: 'ready', plan_json: '[{"title":"read"}]'};
    const {reader} = fixture({
        repositories: {
            state: {read: () => rawState},
            schedule: {list: () => [rawSchedule]},
            lifeEvent: {list: () => [rawEvent]},
            dailyPlan: {read: () => rawPlan},
            presence: {read: () => null}
        },
        sceneReader: () => null
    });

    assert.deepEqual(reader.readPersonaState('persona_a'), {
        ...rawState, personaId: 'persona_a', appearance: {hat: 'red'}, sharedScene: {}
    });
    assert.deepEqual(reader.readScheduleItems('persona_a'), [{
        ...rawSchedule, personaId: 'persona_a', startsAt: NOW, details: {scene: 'cafe'}
    }]);
    assert.deepEqual(reader.readLifeEvents('persona_a'), [{
        ...rawEvent, personaId: 'persona_a', occurredAt: NOW, payload: {situation: 'reading'}
    }]);
    assert.deepEqual(reader.readDailyPlan('persona_a'), {
        ...rawPlan, personaId: 'persona_a', planDate: '2026-08-20', items: [{title: 'read'}]
    });
    assert.deepEqual(rawState, {persona_id: 'persona_a', appearance_json: '{"hat":"red"}', shared_scene_json: '{}'});
});

test('reader clock is used only when no time is supplied and invalid times fail at the boundary', () => {
    let clockCalls = 0;
    const {reader} = fixture({clock: () => { clockCalls += 1; return NOW; }});
    reader.readScheduleItems('persona_a');
    assert.equal(clockCalls, 1);
    reader.readScheduleItems('persona_a', NOW);
    assert.equal(clockCalls, 1);
    assert.throws(() => reader.readResolverInput('persona_a', 'not-a-time'), /valid timestamp/);
});

test('resolver input remains policy-free and feeds the pure resolver', () => {
    const {reader} = fixture({sceneReader: () => null});
    const input = reader.readResolverInput('persona_a', NOW);
    assert.deepEqual(Object.keys(input), [
        'blueprint', 'personaId', 'scheduleItems', 'lifeEvents', 'dailyPlan', 'dailyPlanProjection', 'presence', 'state', 'currentTime'
    ]);
    assert.equal(resolveLifeState(input).source, 'event');
});

test('reader rejects async dependencies and has no legacy/transport/provider ownership', async () => {
    assert.throws(() => fixture({repositories: {schedule: {list: async () => []}}}).reader.readScheduleItems('persona_a'), /must be synchronous/);
    const source = await readFile(new URL('../server/application/life-world-reader.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /(?:from|import)\s*['"][^'"]*(?:server\.js|express|provider)/i);
    assert.doesNotMatch(source, /better-sqlite3|fetch\s*\(|child_process|res\.json/);
});
