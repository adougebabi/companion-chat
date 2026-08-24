export function clockFor(clock) {
    if (typeof clock === 'function') return clock;
    if (clock && typeof clock.now === 'function') return clock.now.bind(clock);
    return () => new Date().toISOString();
}

export function localDateFor(value, timezone = 'UTC') {
    try {
        const parts = new Intl.DateTimeFormat('en-CA', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit'}).formatToParts(new Date(value));
        const fields = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, part.value]));
        return `${fields.year}-${fields.month}-${fields.day}`;
    } catch {
        return new Date(value).toISOString().slice(0, 10);
    }
}

export function isPlanDate(value) {
    if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
    const parsed = new Date(`${value}T00:00:00.000Z`);
    return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value;
}

export function nextDate(planDate) {
    const value = new Date(`${planDate}T00:00:00.000Z`);
    value.setUTCDate(value.getUTCDate() + 1);
    return value.toISOString().slice(0, 10);
}

export function zonedInstant(planDate, clockText, timezone = 'UTC') {
    const match = /^(\d{2}):(\d{2})$/.exec(String(clockText || ''));
    if (!match) return null;
    const hour = Number(match[1]);
    const minute = Number(match[2]);
    if (hour > 24 || minute > 59 || (hour === 24 && minute !== 0)) return null;
    const target = hour === 24
        ? new Date(`${nextDate(planDate)}T00:00:00.000Z`).getTime()
        : Date.UTC(Number(planDate.slice(0, 4)), Number(planDate.slice(5, 7)) - 1, Number(planDate.slice(8, 10)), hour, minute);
    let candidate = target;
    for (let index = 0; index < 3; index += 1) {
        const parts = new Intl.DateTimeFormat('en-US', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false}).formatToParts(new Date(candidate));
        const fields = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, Number(part.value)]));
        const represented = Date.UTC(fields.year, fields.month - 1, fields.day, fields.hour === 24 ? 0 : fields.hour, fields.minute);
        candidate += target - represented;
    }
    return new Date(candidate).toISOString();
}

export function timezoneFor(blueprint) {
    return typeof blueprint?.timezone === 'string' && blueprint.timezone.trim() ? blueprint.timezone.trim() : 'UTC';
}

export function initialDailyBaseline({blueprint, planId, planDate}) {
    const world = blueprint?.world && typeof blueprint.world === 'object' ? blueprint.world : {};
    const locations = Array.isArray(world.locations) ? world.locations : [];
    const defaultRef = world.defaultSceneRef ?? blueprint.defaultSceneRef ?? null;
    const location = locations.find(item => item?.id === defaultRef?.locationId) ?? locations.find(item => item?.isDefault) ?? locations[0] ?? {};
    const rooms = Array.isArray(location.rooms) ? location.rooms : [];
    const room = rooms.find(item => item?.id === defaultRef?.roomId) ?? rooms.find(item => item?.isDefault) ?? rooms[0] ?? {};
    const scene = String(room.scene ?? location.scene ?? blueprint.defaultScene ?? '日常场景').trim() || '日常场景';
    const locationName = String(location.name ?? location.title ?? '家中').trim() || '家中';
    const roomName = String(room.name ?? room.title ?? '自己的房间').trim() || '自己的房间';
    const timezone = timezoneFor(blueprint);
    const startsAt = zonedInstant(planDate, '00:00', timezone) ?? `${planDate}T00:00:00.000Z`;
    const endsAt = zonedInstant(planDate, '24:00', timezone) ?? `${nextDate(planDate)}T00:00:00.000Z`;
    return {
        id: `${planId}:baseline:initial`,
        slotKey: `${planId}:baseline:initial`,
        slotKind: 'baseline_idle',
        title: '日常休息',
        situation: '正在自己的空间里休息',
        scene,
        sceneRef: defaultRef,
        location: locationName,
        room: roomName,
        startsAt,
        endsAt,
        planDate,
        source: 'daily_plan_baseline',
        status: 'confirmed',
        priority: 0,
        constraints: {title: '日常休息', situation: '正在自己的空间里休息', scene, sceneRef: defaultRef, location: locationName, room: roomName},
        outcome: {}
    };
}

export function dailyPlanJobKey(personaId, planDate) {
    return `daily-plan:${personaId}:${planDate}`;
}

export function dailyPlanJobPayload({personaId, dailyPlanId, planDate}) {
    return {dailyPlanId, personaId, planDate, idempotencyKey: dailyPlanJobKey(personaId, planDate)};
}

export function dailyPlanFor({blueprint, planId, planDate}) {
    const timezone = timezoneFor(blueprint);
    const baseline = initialDailyBaseline({blueprint, planId, planDate});
    return {schemaVersion: 1, timezone, planDate, items: [], timeline: [baseline]};
}
