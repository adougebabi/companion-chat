import assert from 'node:assert/strict';
import test from 'node:test';

import {createLifeStateResolver, resolveLifeState} from '../server/domain/life-state-resolver.js';

const CURRENT_TIME = '2026-08-20T03:00:00.000Z';

function blueprint(overrides = {}) {
    return {
        personaId: 'persona-1',
        timezone: 'Asia/Shanghai',
        mood: '平静',
        world: {
            defaultSceneRef: 'home:bedroom',
            locations: [{
                id: 'home', name: '家中', rooms: [{id: 'bedroom', name: '卧室', scene: '卧室'}]
            }]
        },
        routine: [{from: 0, to: 24, label: '在自己的空间里休息', scene: '卧室'}],
        ...overrides
    };
}

function lifeEvent(overrides = {}) {
    return {
        id: 'event-1',
        type: 'social',
        occurredAt: '2026-08-20T02:00:00.000Z',
        resolvesAt: '2026-08-20T04:00:00.000Z',
        situation: '正在和朋友聊天',
        scene: '街角咖啡馆',
        location: '街角咖啡馆',
        room: '',
        mood: '放松',
        priority: 30,
        ...overrides
    };
}

function schedule(overrides = {}) {
    return {
        id: 'schedule-1',
        personaId: 'persona-1',
        status: 'active',
        source: 'explicit_chat_plan',
        title: '去图书馆',
        startsAt: '2026-08-20T02:30:00.000Z',
        endsAt: '2026-08-20T03:30:00.000Z',
        details: {situation: '正在图书馆整理笔记', scene: '图书馆', location: '学校', room: '阅览室'},
        ...overrides
    };
}

function readyPlan(overrides = {}) {
    return {
        id: 'plan-1',
        planDate: '2026-08-20',
        status: 'ready',
        items: [{
            title: '打游戏',
            situation: '在宿舍打游戏看番',
            scene: '宿舍',
            startsAt: '10:00',
            endsAt: '13:00'
        }],
        ...overrides
    };
}

function input(overrides = {}) {
    return {
        blueprint: blueprint(),
        currentTime: CURRENT_TIME,
        scheduleItems: [],
        lifeEvents: [],
        dailyPlan: null,
        ...overrides
    };
}

test('precedence is shared scene, life event, explicit schedule, ready plan, then routine', () => {
    const sharedScene = {
        personaId: 'persona-1',
        eventId: 'scene-1',
        location: '湖边公园',
        room: '',
        activity: '一起散步',
        situation: '正在湖边散步',
        startedAt: '2026-08-20T02:45:00.000Z'
    };
    const all = input({
        presence: sharedScene,
        lifeEvents: [lifeEvent()],
        scheduleItems: [schedule()],
        dailyPlan: readyPlan()
    });

    assert.equal(resolveLifeState(all).source, 'shared_scene');
    assert.equal(resolveLifeState({...all, presence: null}).source, 'event');
    assert.equal(resolveLifeState({...all, presence: null, lifeEvents: []}).source, 'schedule');
    assert.equal(resolveLifeState({...all, presence: null, lifeEvents: [], scheduleItems: []}).source, 'daily_plan');
    assert.equal(resolveLifeState({...all, presence: null, lifeEvents: [], scheduleItems: [], dailyPlan: null}).source, 'routine');
});

test('trusted boundaries are exposed consistently for event, schedule, and plan projections', () => {
    const event = resolveLifeState(input({lifeEvents: [lifeEvent()]}));
    assert.equal(event.startsAt, '2026-08-20T02:00:00.000Z');
    assert.equal(event.endsAt, '2026-08-20T04:00:00.000Z');
    assert.equal(event.timeFact, 'known');
    assert.equal(event.nextBoundaryAt, event.endsAt);
    assert.equal(event.sourceEvent.id, 'event-1');

    const plan = resolveLifeState(input({
        currentTime: '2026-08-20T03:30:00.000Z',
        dailyPlan: readyPlan({items: [{title: '学习', scene: '图书馆', situation: '在图书馆学习', startsAt: '10:00', endsAt: '12:00'}]})
    }));
    assert.equal(plan.source, 'daily_plan');
    assert.equal(plan.endsAt, '2026-08-20T04:00:00.000Z');
    assert.equal(plan.timeFact, 'known');

    const beforeFirstPlanSlot = resolveLifeState(input({
        currentTime: '2026-08-20T00:47:00.000Z',
        dailyPlan: readyPlan()
    }));
    assert.equal(beforeFirstPlanSlot.source, 'daily_plan_baseline');
    assert.equal(beforeFirstPlanSlot.situation, '在默认房间里休息，等待下一项安排');
    assert.equal(beforeFirstPlanSlot.endsAt, '2026-08-20T02:00:00.000Z');
    assert.equal(beforeFirstPlanSlot.timeFact, 'known');

    const scheduleState = resolveLifeState(input({scheduleItems: [schedule()]}));
    assert.equal(scheduleState.source, 'schedule');
    assert.equal(scheduleState.situation, '正在图书馆整理笔记');
    assert.equal(scheduleState.location, '学校');
    assert.equal(scheduleState.room, '阅览室');
    assert.equal(scheduleState.endsAt, '2026-08-20T03:30:00.000Z');
    assert.equal(scheduleState.timeFact, 'known');
});

test('sleep availability follows explicit slot facts instead of situation wording', () => {
    const wordingOnly = resolveLifeState(input({
        currentTime: '2026-08-20T00:47:00.000Z',
        dailyPlan: readyPlan({items: [{title: 'sleeping', situation: 'sleeping before the day begins', startsAt: '10:00', endsAt: '13:00'}]})
    }));
    assert.equal(wordingOnly.source, 'daily_plan_baseline');
    assert.equal(wordingOnly.slotKind, 'baseline_idle');
    assert.equal(wordingOnly.sleeping, false);

    const explicit = resolveLifeState(input({
        currentTime: '2026-08-20T00:47:00.000Z',
        dailyPlan: readyPlan({items: [{slotKind: 'baseline_sleep', title: 'sleeping', situation: 'sleeping before the day begins', startsAt: '10:00', endsAt: '13:00'}]})
    }));
    assert.equal(explicit.source, 'daily_plan_baseline');
    assert.equal(explicit.slotKind, 'baseline_sleep');
    assert.equal(explicit.sleeping, true);
});

test('row-shaped daily plans decode array payloads before resolving slots', () => {
    const state = resolveLifeState(input({
        dailyPlan: {
            id: 'plan-row',
            persona_id: 'persona-1',
            plan_date: '2026-08-20',
            status: 'ready',
            plan_json: JSON.stringify([{
                title: '学习',
                situation: '在图书馆学习',
                scene: '图书馆',
                startsAt: '10:00',
                endsAt: '12:00'
            }])
        }
    }));

    assert.equal(state.source, 'daily_plan');
    assert.equal(state.situation, '在图书馆学习');
    assert.equal(state.startsAt, '2026-08-20T02:00:00.000Z');
    assert.equal(state.endsAt, '2026-08-20T04:00:00.000Z');
});

test('missing event end time remains unknown and does not invent a boundary', () => {
    const state = resolveLifeState(input({lifeEvents: [lifeEvent({resolvesAt: null, endsAt: null})]}));
    assert.equal(state.source, 'event');
    assert.equal(state.startsAt, '2026-08-20T02:00:00.000Z');
    assert.equal(state.endsAt, null);
    assert.equal(state.timeFact, 'unknown');
    assert.equal(state.nextBoundaryAt, null);
});

test('expired events are excluded before lower-precedence sources are selected', () => {
    const state = resolveLifeState(input({lifeEvents: [lifeEvent({resolvesAt: '2026-08-20T02:59:59.999Z'})]}));
    assert.equal(state.source, 'routine');
    assert.equal(state.situation, '在自己的空间里休息');
});

test('expired event aliases are excluded using the same trusted boundary rules', () => {
    const state = resolveLifeState(input({lifeEvents: [{
        id: 'event-expired-alias',
        type: 'social',
        startedAt: '2026-08-20T02:00:00.000Z',
        expiresAt: '2026-08-20T02:59:59.999Z',
        active: true,
        situation: '已经结束的活动'
    }]}));

    assert.equal(state.source, 'routine');
});

test('presence snapshots are persona-scoped', () => {
    const foreign = {
        personaId: 'persona-2', eventId: 'foreign-scene', location: '别人的房间',
        activity: '不应投影', situation: '不应成为当前状态'
    };
    const foreignState = resolveLifeState(input({presence: foreign}));
    assert.equal(foreignState.source, 'routine');
    assert.notEqual(foreignState.sourceId, 'foreign-scene');

    const ownState = resolveLifeState(input({presence: {...foreign, personaId: 'persona-1', eventId: 'own-scene', situation: '共同在场'}}));
    assert.equal(ownState.source, 'shared_scene');
    assert.equal(ownState.sourceId, 'own-scene');
    assert.equal(ownState.situation, '共同在场');
});

test('factory returns the same pure resolver without reading wall-clock time', () => {
    const resolver = createLifeStateResolver();
    assert.equal(resolver, resolveLifeState);
    assert.throws(() => resolver(input({currentTime: undefined})), /requires a valid currentTime/);
});

test('missing timezone uses deterministic UTC rather than the host timezone', () => {
    const state = resolveLifeState(input({
        blueprint: blueprint({
            timezone: undefined,
            routine: [{from: 0, to: 1, label: '午夜状态'}]
        }),
        currentTime: '2026-08-20T00:30:00.000Z'
    }));

    assert.equal(state.source, 'routine');
    assert.equal(state.situation, '午夜状态');
});
