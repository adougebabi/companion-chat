/**
 * Resolve the current life-world projection without consulting persistence,
 * providers, process state, or the wall clock.
 *
 * The resolver deliberately accepts both the normalized domain vocabulary and
 * row-shaped persistence aliases. This keeps the pure contract useful across
 * repository adapters while leaving all I/O at the callers.
 */

const GENERATED_SCHEDULE_SOURCES = new Set([
    'ai_daily_plan',
    'daily_plan',
    'daily_plan_baseline',
    'life_model_fixed',
    'life_model_flexible',
    'life_model_opportunity',
    'generated',
    'routine'
]);

const INACTIVE_STATUSES = new Set(['cancelled', 'canceled', 'complete', 'completed', 'expired', 'failed', 'skipped']);
const INACTIVE_PRESENCE_STATUSES = new Set(['ended', 'inactive', 'closed', 'expired', 'cancelled', 'canceled']);

function isRecord(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function clone(value) {
    if (Array.isArray(value)) return value.map(clone);
    if (isRecord(value)) return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clone(item)]));
    return value;
}

function text(value) {
    return typeof value === 'string' && value.trim() ? value.trim() : '';
}

function firstText(...values) {
    for (const value of values) {
        const result = text(value);
        if (result) return result;
    }
    return '';
}

function json(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function timeValue(value) {
    if (value instanceof Date) return Number.isFinite(value.getTime()) ? value.getTime() : null;
    if (typeof value === 'number' && Number.isFinite(value)) {
        const parsed = new Date(value);
        return Number.isFinite(parsed.getTime()) ? parsed.getTime() : null;
    }
    if (typeof value === 'string' && value.trim()) {
        const parsed = new Date(value);
        return Number.isFinite(parsed.getTime()) ? parsed.getTime() : null;
    }
    return null;
}

function isoTime(value) {
    const parsed = timeValue(value);
    return parsed === null ? null : new Date(parsed).toISOString();
}

function readTime(record, ...keys) {
    for (const key of keys) {
        if (record?.[key] !== undefined && record?.[key] !== null && record[key] !== '') return record[key];
    }
    return null;
}

function localParts(value, timezone) {
    const date = new Date(value);
    const options = {year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false, timeZone: timezone || 'UTC'};
    try {
        const parts = new Intl.DateTimeFormat('en-US', options).formatToParts(date);
        return Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, part.type === 'hour' && Number(part.value) === 24 ? 0 : Number(part.value)]));
    } catch {
        return Object.fromEntries(new Intl.DateTimeFormat('en-US', {...options, timeZone: 'UTC'}).formatToParts(date)
            .filter(part => part.type !== 'literal').map(part => [part.type, part.type === 'hour' && Number(part.value) === 24 ? 0 : Number(part.value)]));
    }
}

function localDate(value, timezone) {
    const parts = localParts(value, timezone);
    return `${String(parts.year).padStart(4, '0')}-${String(parts.month).padStart(2, '0')}-${String(parts.day).padStart(2, '0')}`;
}

function nextDate(dateText) {
    const date = new Date(`${dateText}T00:00:00.000Z`);
    if (!Number.isFinite(date.getTime())) return dateText;
    return new Date(date.getTime() + 86_400_000).toISOString().slice(0, 10);
}

function zonedInstant(planDate, clockText, timezone) {
    const match = /^(\d{2}):(\d{2})$/.exec(String(clockText || ''));
    if (!match || Number(match[1]) > 24 || Number(match[2]) > 59 || (Number(match[1]) === 24 && Number(match[2]) !== 0)) return null;
    if (Number(match[1]) === 24) return zonedInstant(nextDate(planDate), '00:00', timezone);
    const year = Number(planDate.slice(0, 4));
    const month = Number(planDate.slice(5, 7));
    const day = Number(planDate.slice(8, 10));
    if (![year, month, day].every(Number.isFinite)) return null;
    let candidate = Date.UTC(year, month - 1, day, Number(match[1]), Number(match[2]));
    for (let index = 0; index < 3; index += 1) {
        const parts = localParts(candidate, timezone);
        const represented = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute);
        const target = Date.UTC(year, month - 1, day, Number(match[1]), Number(match[2]));
        candidate += target - represented;
    }
    return new Date(candidate).toISOString();
}

function scopeId(value) {
    return text(value?.personaId ?? value?.persona_id ?? value?.subjectId ?? value?.subject_id);
}

function scopeMatches(value, personaId) {
    const candidate = scopeId(value);
    return !candidate || !personaId || candidate === personaId;
}

function payloadRecord(value) {
    const payload = json(value?.payload_json ?? value?.payload, {});
    const details = json(value?.details_json ?? value?.details, {});
    return {
        ...(isRecord(payload) ? payload : {}),
        ...(isRecord(details) ? details : {}),
        ...value
    };
}

function trustedTimes(record) {
    const startRaw = readTime(record, 'startsAt', 'starts_at', 'occurredAt', 'occurred_at', 'startedAt', 'started_at');
    const endRaw = readTime(record, 'endsAt', 'ends_at', 'resolvesAt', 'resolves_at', 'expiresAt', 'expires_at');
    const startMs = timeValue(startRaw);
    const endMs = timeValue(endRaw);
    const explicitlyUnknown = record?.timeFact === 'unknown' || record?.time_fact === 'unknown';
    const trustedEndMs = explicitlyUnknown ? null : endMs;
    return {
        startMs,
        endMs: trustedEndMs,
        startsAt: startMs === null ? null : new Date(startMs).toISOString(),
        endsAt: trustedEndMs === null ? null : new Date(trustedEndMs).toISOString(),
        timeFact: trustedEndMs === null ? 'unknown' : 'known'
    };
}

function defaultScene(blueprint = {}) {
    const world = isRecord(blueprint.world) ? blueprint.world : {};
    const defaultRef = world.defaultSceneRef ?? blueprint.defaultSceneRef ?? world.defaultRoom ?? blueprint.defaultRoom ?? null;
    const locations = Array.isArray(world.locations) ? world.locations : Array.isArray(blueprint.locations) ? blueprint.locations : [];
    let location = firstText(world.defaultLocation, blueprint.defaultLocation);
    let room = firstText(world.defaultRoom, blueprint.defaultRoom);
    let scene = firstText(world.defaultScene, blueprint.defaultScene);
    let sceneRef = typeof defaultRef === 'string' ? defaultRef : null;

    if (isRecord(defaultRef)) {
        sceneRef = firstText(defaultRef.locationId && defaultRef.roomId ? `${defaultRef.locationId}:${defaultRef.roomId}` : '', defaultRef.id);
        location = firstText(defaultRef.location, defaultRef.locationName, location);
        room = firstText(defaultRef.room, defaultRef.roomName, room);
        scene = firstText(defaultRef.scene, defaultRef.name, scene);
    }
    if (sceneRef && sceneRef.includes(':')) {
        const [locationId, roomId] = sceneRef.split(':');
        const locationRow = locations.find(item => String(item?.id || item?.locationId || '') === locationId);
        const rooms = Array.isArray(locationRow?.rooms) ? locationRow.rooms : [];
        const roomRow = rooms.find(item => String(item?.id || item?.roomId || '') === roomId);
        location = firstText(locationRow?.name, locationRow?.title, location);
        room = firstText(roomRow?.name, roomRow?.title, room);
        scene = firstText(roomRow?.scene, roomRow?.name, locationRow?.scene, scene);
    }
    scene = firstText(scene, room, location, '日常场景');
    return {scene, location, room, sceneRef};
}

function sceneForRef(blueprint, ref) {
    const fallback = defaultScene(blueprint);
    if (!ref) return fallback;
    const world = isRecord(blueprint.world) ? blueprint.world : {};
    const locations = Array.isArray(world.locations) ? world.locations : Array.isArray(blueprint.locations) ? blueprint.locations : [];
    const target = isRecord(ref)
        ? ref
        : typeof ref === 'string' && ref.includes(':')
            ? {locationId: ref.split(':')[0], roomId: ref.split(':')[1]}
            : null;
    if (!target) return fallback;
    const locationRow = locations.find(item => String(item?.id || item?.locationId || '') === String(target.locationId || ''));
    const rooms = Array.isArray(locationRow?.rooms) ? locationRow.rooms : [];
    const roomRow = rooms.find(item => String(item?.id || item?.roomId || '') === String(target.roomId || ''));
    return {
        scene: firstText(roomRow?.scene, roomRow?.name, locationRow?.scene, locationRow?.name, fallback.scene),
        location: firstText(locationRow?.name, locationRow?.title, fallback.location),
        room: firstText(roomRow?.name, roomRow?.title, fallback.room),
        sceneRef: isRecord(ref) ? clone(ref) : ref
    };
}

function sourceAppearance(blueprint, source) {
    const value = source?.appearance ?? source?.temporaryAppearance ?? source?.appearance_json;
    if (value !== undefined && value !== null && value !== '') return clone(json(value, value));
    const fallback = blueprint?.appearance ?? blueprint?.visualBaseline ?? blueprint?.characterCard?.appearanceCore;
    return fallback === undefined || fallback === null ? {} : clone(fallback);
}

function sourceEventMetadata(record, fallbackType = null) {
    if (!record) return null;
    const id = firstText(record.id, record.eventId, record.event_id, record.sourceId, record.source_id);
    const type = firstText(record.type, record.eventType, record.event_type, fallbackType);
    const occurredAt = isoTime(readTime(record, 'occurredAt', 'occurred_at', 'startsAt', 'starts_at'));
    const resolvesAt = isoTime(readTime(record, 'resolvesAt', 'resolves_at', 'endsAt', 'ends_at'));
    const causationId = firstText(record.causationId, record.causation_id, record.sourceMessageId, record.source_message_id) || null;
    return {id: id || null, type: type || null, occurredAt, resolvesAt, causationId, priority: Number(record.priority ?? 0) || 0};
}

function commonProjection({blueprint, source, sourceId = null, sourceEvent = null, record = {}, nowMs, fallback = {}}) {
    const defaults = defaultScene(blueprint);
    const merged = payloadRecord(record);
    const suppliedSceneRef = merged.sceneRef ?? merged.scene_ref ?? null;
    const sceneProjection = sceneForRef(blueprint, suppliedSceneRef || defaults.sceneRef);
    const sceneRef = sceneProjection.sceneRef || null;
    const situation = firstText(merged.situation, merged.label, merged.title, fallback.situation, '正在自己的空间里休息');
    const scene = firstText(merged.scene, merged.activity, fallback.scene, sceneProjection.scene, defaults.scene);
    const location = firstText(merged.location, fallback.location, sceneProjection.location, defaults.location);
    const room = firstText(merged.room, fallback.room, sceneProjection.room, defaults.room);
    const mood = firstText(merged.mood, fallback.mood, blueprint.mood, '平静');
    const appearance = sourceAppearance(blueprint, merged);
    const times = trustedTimes(merged);
    const explicitNext = isoTime(readTime(merged, 'nextBoundaryAt', 'next_boundary_at', 'nextBoundary'));
    const nextBoundaryAt = times.endsAt || explicitNext || null;
    const eventId = sourceEvent?.id || null;
    const eventMetadata = sourceEvent || null;
    return {
        source,
        sourceId: sourceId || eventId || null,
        situation,
        mood,
        scene,
        location,
        room,
        sceneRef,
        appearance,
        startsAt: times.startsAt,
        endsAt: times.endsAt,
        timeFact: times.timeFact,
        nextBoundaryAt,
        nextBoundary: nextBoundaryAt,
        sourceEvent: eventMetadata,
        sourceEventId: eventId,
        eventId,
        slotKind: firstText(merged.slotKind, merged.slot_kind, merged.kind) || null,
        sleeping: merged.sleeping === true
            || merged.isSleeping === true
            || merged.slotKind === 'baseline_sleep'
            || merged.slotKind === 'sleep'
            || merged.slot_kind === 'baseline_sleep'
            || merged.slot_kind === 'sleep',
        scheduleId: firstText(merged.scheduleId, merged.schedule_id) || null,
        slotId: firstText(merged.slotId, merged.slot_id) || null,
        planId: firstText(merged.planId, merged.plan_id) || null
    };
}

function persistedAppearanceProjection(projection, input) {
    const persisted = isRecord(input?.state?.appearance) ? input.state.appearance : {};
    if (!Object.keys(persisted).length) return projection;
    const source = isRecord(projection?.appearance) ? projection.appearance : {};
    return {...projection, appearance: {...persisted, ...source}};
}

function activePresence(input, personaId, nowMs) {
    const candidates = [input.presence, input.presenceSnapshot, input.sharedScene, input.sharedSceneSnapshot];
    for (const raw of candidates) {
        if (!raw) continue;
        const outer = isRecord(raw) ? raw : {};
        if (!scopeMatches(outer, personaId)) continue;
        if (outer.active === false || outer.isActive === false || INACTIVE_PRESENCE_STATUSES.has(String(outer.status || '').toLowerCase())) continue;
        const nestedCandidate = outer.sharedScene ?? outer.shared_scene ?? outer.sceneSnapshot ?? outer.scene;
        const nested = isRecord(nestedCandidate) ? nestedCandidate : outer;
        if (!isRecord(nested) || !scopeMatches(nested, personaId)) continue;
        const merged = {...outer, ...nested};
        const startMs = timeValue(readTime(merged, 'startsAt', 'startedAt', 'started_at', 'occurredAt', 'occurred_at'));
        const endMs = timeValue(readTime(merged, 'endsAt', 'expiresAt', 'expires_at', 'endedAt', 'ended_at', 'resolvesAt', 'resolves_at'));
        if (startMs !== null && startMs > nowMs) continue;
        if (endMs !== null && endMs <= nowMs) continue;
        if (!firstText(merged.situation, merged.activity, merged.scene, merged.location)) continue;
        return merged;
    }
    return null;
}

function activeLifeEvent(input, personaId, nowMs) {
    const events = Array.isArray(input.lifeEvents) ? input.lifeEvents : Array.isArray(input.events) ? input.events : [];
    return events.map((raw, index) => ({raw, index, merged: payloadRecord(raw)}))
        .filter(({raw, merged}) => scopeMatches(raw, personaId) && scopeMatches(merged, personaId))
        .filter(({merged}) => !['routine', 'schedule'].includes(String(merged.type || '').toLowerCase()))
        .filter(({merged}) => !INACTIVE_STATUSES.has(String(merged.status || '').toLowerCase()) && merged.active !== false)
        .map(item => {
            const event = sourceEventMetadata(item.merged, 'life_event');
            const times = trustedTimes(item.merged);
            const startMs = times.startMs;
            const endMs = times.endMs;
            return {...item, event, startMs, endMs};
        })
        .filter(({merged, startMs, endMs}) => (startMs === null ? merged.active === true || String(merged.status || '').toLowerCase() === 'active' : startMs <= nowMs) && (endMs === null || endMs > nowMs))
        .sort((left, right) => (right.event.priority - left.event.priority)
            || ((right.startMs ?? Number.NEGATIVE_INFINITY) - (left.startMs ?? Number.NEGATIVE_INFINITY))
            || String(left.event.id || left.index).localeCompare(String(right.event.id || right.index)))[0] || null;
}

function scheduleSource(item) {
    return firstText(item.source, item.sourceType, item.source_type).toLowerCase();
}

function explicitSchedules(input, personaId, nowMs) {
    const items = Array.isArray(input.scheduleItems) ? input.scheduleItems : Array.isArray(input.schedules) ? input.schedules : [];
    return items.map((raw, index) => ({raw, index, merged: payloadRecord(raw)}))
        .filter(({raw, merged}) => scopeMatches(raw, personaId) && scopeMatches(merged, personaId))
        .filter(({merged}) => !INACTIVE_STATUSES.has(String(merged.status || 'active').toLowerCase()))
        .filter(({merged}) => !GENERATED_SCHEDULE_SOURCES.has(scheduleSource(merged)))
        .map(item => {
            const times = trustedTimes(item.merged);
            return {...item, times, startsMs: times.startMs, endsMs: times.endMs};
        })
        .filter(({startsMs, endsMs}) => startsMs !== null && startsMs <= nowMs && (endsMs === null || endsMs > nowMs));
}

function nextExplicitScheduleStart(input, personaId, nowMs) {
    const items = Array.isArray(input.scheduleItems) ? input.scheduleItems : Array.isArray(input.schedules) ? input.schedules : [];
    return items.map(raw => payloadRecord(raw))
        .filter(item => scopeMatches(item, personaId) && !INACTIVE_STATUSES.has(String(item.status || 'active').toLowerCase()) && !GENERATED_SCHEDULE_SOURCES.has(scheduleSource(item)))
        .map(item => timeValue(readTime(item, 'startsAt', 'starts_at')))
        .filter(value => value !== null && value > nowMs)
        .sort((left, right) => left - right)[0] ?? null;
}

function planInput(input) {
    const candidate = input.dailyPlanProjection ?? input.dailyPlan ?? input.plan ?? null;
    if (!isRecord(candidate) || !candidate.plan_json) return candidate;
    const serialized = json(candidate.plan_json, {});
    if (Array.isArray(serialized)) return {...candidate, items: serialized};
    return isRecord(serialized) ? {...serialized, ...candidate} : candidate;
}

function planReady(plan, projection) {
    if (!plan) return false;
    if (projection && projection === plan && (projection.source || projection.slotKind || projection.slot_kind)) return true;
    if (Array.isArray(plan)) return true;
    if (plan.ready === true || plan.status === 'ready' || plan.state === 'ready') return true;
    return Boolean((plan.timeline || plan.slots || plan.items) && plan.status === undefined && plan.ready === undefined);
}

function normalizePlanSlots(plan, blueprint, nowMs) {
    const direct = isRecord(plan) && (plan.source || plan.slotKind || plan.slot_kind) && !Array.isArray(plan.items) && !Array.isArray(plan.timeline) && !Array.isArray(plan.slots);
    const rows = direct ? [plan] : Array.isArray(plan) ? plan : Array.isArray(plan?.timeline) ? plan.timeline : Array.isArray(plan?.slots) ? plan.slots : Array.isArray(plan?.items) ? plan.items : [];
    const planDate = firstText(plan?.planDate, plan?.plan_date) || localDate(nowMs, blueprint.timezone);
    const timezone = blueprint.timezone;
    const planId = firstText(plan?.id, plan?.planId, plan?.plan_id) || null;
    return rows.map((raw, index) => {
        const merged = payloadRecord(raw);
        const startRaw = readTime(merged, 'startsAt', 'starts_at');
        const endRaw = readTime(merged, 'endsAt', 'ends_at');
        const startsAt = timeValue(startRaw) === null ? zonedInstant(planDate, startRaw, timezone) : isoTime(startRaw);
        const endsAt = timeValue(endRaw) === null ? zonedInstant(planDate, endRaw, timezone) : isoTime(endRaw);
        const slotKind = firstText(merged.slotKind, merged.slot_kind, merged.kind).toLowerCase();
        const source = slotKind.startsWith('baseline') || firstText(merged.source).toLowerCase() === 'daily_plan_baseline' ? 'daily_plan_baseline' : 'daily_plan';
        return {
            ...merged,
            slotKey: firstText(merged.slotKey, merged.slot_key, merged.id) || `${planId || 'daily-plan'}:slot:${index}`,
            slotKind: slotKind || (source === 'daily_plan_baseline' ? 'baseline_idle' : 'planned'),
            source,
            planId: firstText(merged.planId, merged.plan_id) || planId,
            startsAt,
            endsAt
        };
    }).filter(slot => slot.startsAt && (slot.endsAt === null || timeValue(slot.endsAt) > timeValue(slot.startsAt)))
        .sort((left, right) => (timeValue(left.startsAt) - timeValue(right.startsAt)) || String(left.slotKey).localeCompare(String(right.slotKey)));
}

function generatedPlanBaseline(slots, plan, blueprint, nowMs) {
    const planDate = firstText(plan?.planDate, plan?.plan_date) || localDate(nowMs, blueprint.timezone);
    const dayStart = zonedInstant(planDate, '00:00', blueprint.timezone);
    const dayEnd = zonedInstant(nextDate(planDate), '00:00', blueprint.timezone);
    const now = nowMs;
    if (!slots.length) {
        return {
            slotKey: `${firstText(plan?.id, plan?.planId, 'daily-plan')}:baseline:empty`,
            slotKind: 'baseline_idle',
            source: 'daily_plan_baseline',
            situation: '正在自己的空间里休息',
            scene: typeof blueprint.defaultScene === 'string' ? blueprint.defaultScene : typeof blueprint.world?.defaultScene === 'string' ? blueprint.world.defaultScene : '日常场景',
            location: typeof blueprint.world?.defaultLocation === 'string' ? blueprint.world.defaultLocation : '家中',
            room: typeof blueprint.world?.defaultRoom === 'string' ? blueprint.world.defaultRoom : '自己的房间',
            startsAt: dayStart,
            endsAt: dayEnd,
            timeFact: 'known'
        };
    }
    const previous = slots.filter(slot => timeValue(slot.startsAt) <= now && timeValue(slot.endsAt) <= now).at(-1);
    const next = slots.find(slot => timeValue(slot.startsAt) > now);
    const startsAt = previous?.endsAt || dayStart;
    const endsAt = next?.startsAt || dayEnd;
    if (!startsAt || !endsAt || timeValue(startsAt) > now || timeValue(endsAt) <= now) return null;
    const firstSlot = slots[0];
    const sleeping = !previous && firstSlot && (
        firstSlot.slotKind === 'baseline_sleep'
        || firstSlot.slotKind === 'sleep'
        || firstSlot.sleeping === true
        || firstSlot.isSleeping === true
    );
    return {
        slotKey: `${firstText(plan?.id, plan?.planId, 'daily-plan')}:baseline:resolver:${startsAt}`,
        slotKind: sleeping ? 'baseline_sleep' : previous ? 'baseline_idle' : 'baseline_idle',
        source: 'daily_plan_baseline',
        situation: sleeping ? '正在睡觉或赖床，等待自然醒' : previous ? '在默认房间里休息，等待下一项安排' : '在默认房间里休息，等待下一项安排',
        startsAt,
        endsAt,
        timeFact: 'known'
    };
}

function activeDailyPlan(input, blueprint, nowMs, personaId) {
    const plan = planInput(input);
    if (!plan || !planReady(plan, input.dailyPlanProjection)) return null;
    if (scopeId(plan) && !scopeMatches(plan, personaId)) return null;
    const planDate = firstText(plan?.planDate, plan?.plan_date);
    if (planDate && planDate !== localDate(nowMs, blueprint.timezone)) return null;
    const slots = normalizePlanSlots(plan, blueprint, nowMs);
    const active = slots.find(slot => {
        const start = timeValue(slot.startsAt);
        const end = timeValue(slot.endsAt);
        return start !== null && start <= nowMs && (end === null || end > nowMs);
    });
    if (active) {
        if (!active.endsAt) {
            const next = slots.find(slot => timeValue(slot.startsAt) > nowMs);
            if (next) active.nextBoundaryAt = next.startsAt;
        }
        return active;
    }
    return generatedPlanBaseline(slots, plan, blueprint, nowMs);
}

function routineProjection(blueprint, nowMs) {
    const routine = Array.isArray(blueprint.routine) ? blueprint.routine : [];
    const parts = localParts(nowMs, blueprint.timezone);
    const minute = parts.hour * 60 + parts.minute;
    const match = routine.find(item => {
        const from = Number(item?.from ?? item?.startHour);
        const to = Number(item?.to ?? item?.endHour);
        return Number.isFinite(from) && Number.isFinite(to) && minute >= from * 60 && minute < to * 60;
    });
    if (match) return {record: match, source: 'routine', sourceId: firstText(match.id, match.sourceId) || null};
    return {record: {situation: '正在自己的空间里休息'}, source: 'baseline', sourceId: null};
}

/**
 * @typedef {Object} LifeStateResolverInput
 * @property {Object} blueprint
 * @property {Array<Object>} [scheduleItems]
 * @property {Array<Object>} [lifeEvents]
 * @property {Object|Array<Object>} [dailyPlan]
 * @property {Object} [presence]
 * @property {Object} [sharedScene]
 * @property {Date|string|number} currentTime
 * @property {string} [personaId]
 */

/**
 * @param {LifeStateResolverInput} input
 * @returns {Object}
 */
export function resolveLifeState(input = {}) {
    if (!isRecord(input)) throw new TypeError('LifeStateResolver input must be an object');
    const nowMs = timeValue(input.currentTime ?? input.at ?? input.time);
    if (nowMs === null) throw new TypeError('LifeStateResolver requires a valid currentTime');
    const blueprint = isRecord(input.blueprint) ? input.blueprint : {};
    const personaId = text(input.personaId)
        || text(blueprint.personaId ?? blueprint.persona_id ?? blueprint.subjectId ?? blueprint.subject_id ?? blueprint.id)
        || null;
    const fallback = defaultScene(blueprint);

    const presence = activePresence(input, personaId, nowMs);
    if (presence) {
        const metadata = sourceEventMetadata(presence, 'shared_scene');
        return persistedAppearanceProjection(commonProjection({blueprint, source: 'shared_scene', sourceId: metadata?.id, sourceEvent: metadata, record: presence, nowMs, fallback}), input);
    }

    const event = activeLifeEvent(input, personaId, nowMs);
    if (event) {
        return persistedAppearanceProjection(commonProjection({blueprint, source: 'event', sourceId: event.event?.id, sourceEvent: event.event, record: event.merged, nowMs, fallback}), input);
    }

    const schedules = explicitSchedules(input, personaId, nowMs);
    if (schedules.length) {
        const selected = schedules.sort((left, right) => (Number(right.merged.priority || 0) - Number(left.merged.priority || 0))
            || (right.startsMs - left.startsMs)
            || String(left.merged.id || left.index).localeCompare(String(right.merged.id || right.index)))[0];
        const next = selected.endsMs === null ? nextExplicitScheduleStart(input, personaId, nowMs) : null;
        const record = {...selected.merged, scheduleId: firstText(selected.merged.scheduleId, selected.merged.schedule_id, selected.merged.id) || null, nextBoundaryAt: next === null ? null : new Date(next).toISOString()};
        return persistedAppearanceProjection(commonProjection({blueprint, source: 'schedule', sourceId: firstText(record.scheduleId) || null, record, nowMs, fallback}), input);
    }

    const planSlot = activeDailyPlan(input, blueprint, nowMs, personaId);
    if (planSlot) {
        const source = planSlot.source === 'daily_plan_baseline' ? 'daily_plan_baseline' : 'daily_plan';
        return persistedAppearanceProjection(commonProjection({blueprint, source, sourceId: firstText(planSlot.slotId, planSlot.slot_id, planSlot.slotKey) || null, record: planSlot, nowMs, fallback}), input);
    }

    const routine = routineProjection(blueprint, nowMs);
    return persistedAppearanceProjection(commonProjection({blueprint, source: routine.source, sourceId: routine.sourceId, record: routine.record, nowMs, fallback}), input);
}

/**
 * Factory form for composition roots that prefer an injected pure capability.
 * No state is captured and no clock is read.
 */
export function createLifeStateResolver() {
    return resolveLifeState;
}

export const lifeStateResolverSources = Object.freeze([
    'shared_scene',
    'event',
    'schedule',
    'daily_plan',
    'daily_plan_baseline',
    'routine',
    'baseline'
]);
