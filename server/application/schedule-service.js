const MAX_ID_LENGTH = 240;
const MAX_TITLE_LENGTH = 120;
const MAX_SCENE_LENGTH = 120;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
}

function record(value, field) {
    if (!isRecord(value)) throw statusError(`${field}必须是 JSON 对象`, 400);
    return value;
}

function requiredText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') throw statusError(`${field}不能为空`, 400);
    const normalized = value.trim();
    if (normalized.length > maxLength) throw statusError(`${field}不能超过 ${maxLength} 个字符`, 400);
    return normalized;
}

function sourceFor(options, names) {
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    for (const source of [options, repositories]) {
        for (const name of names) if (source[name] !== undefined) return source[name];
    }
    return undefined;
}

function methodFor(port, names, field, {optional = false} = {}) {
    if (typeof port === 'function') return port;
    if (isRecord(port)) {
        for (const name of names) if (typeof port[name] === 'function') return port[name].bind(port);
    }
    if (optional) return null;
    throw statusError(`日程服务缺少 ${field}.${names[0]}()`, 501);
}

function clockFor(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.now === 'function') return value.now.bind(value);
    return () => new Date().toISOString();
}

function timestamp(value, field) {
    const normalized = value instanceof Date ? value.toISOString() : value;
    if (typeof normalized !== 'string' || !normalized.trim() || !Number.isFinite(Date.parse(normalized))) {
        throw statusError(`${field}必须是有效时间`, 400);
    }
    return new Date(normalized).toISOString();
}

function personaIdFor(command) {
    return requiredText(command.personaId ?? command.persona_id, '人格 ID');
}

function mapMaybe(value, mapper) {
    return value && typeof value.then === 'function' ? value.then(mapper) : mapper(value);
}

/**
 * Owns schedule command validation and persona ownership. Schedule writes,
 * life-event facts, and any accepted-plan lookup remain explicit repository
 * ports so this service cannot silently create an incomplete schedule.
 */
export function createScheduleService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Schedule service options must be an object');
    const schedules = sourceFor(options, ['schedule', 'schedules', 'scheduleRepository', 'schedulePort']);
    const personas = sourceFor(options, ['persona', 'personas', 'personaRepository', 'personaPort']);
    const conversations = sourceFor(options, ['conversation', 'conversationRepository', 'conversationPort']);
    const now = clockFor(options.clock ?? options.now);
    const findPersona = methodFor(personas, ['findActive', 'find', 'get'], 'persona port', {optional: true});
    const findSchedule = methodFor(schedules, ['findActive', 'find', 'get'], 'schedule port', {optional: true});

    function requirePersona(personaId) {
        if (!findPersona) return {id: personaId};
        const value = findPersona(personaId);
        if (value && typeof value.then === 'function') throw new TypeError('Schedule persona lookup must be synchronous');
        if (!value) throw statusError('人格不存在', 404);
        return value;
    }

    function currentSchedule(personaId, scheduleId) {
        if (!findSchedule) return null;
        const value = findSchedule({personaId, scheduleId, id: scheduleId});
        if (value && typeof value.then === 'function') throw new TypeError('Schedule lookup must be synchronous');
        if (!value) throw statusError('有效日程不存在', 404);
        return value;
    }

    function operation(names, field) {
        const result = methodFor(schedules, names, field, {optional: true});
        if (!result) throw statusError(`日程服务缺少 ${field} port`, 501);
        return result;
    }

    function normalizeWindow(input, existing = null) {
        const current = timestamp(now(), '当前时间');
        const startsAt = timestamp(input.startsAt, '计划开始时间');
        if (Date.parse(startsAt) <= Date.parse(current)) throw statusError('计划开始时间必须是未来的明确时间', 400);
        let endsAt = input.endsAt === undefined ? existing?.endsAt ?? existing?.ends_at : input.endsAt;
        if (input.endsAt === undefined && endsAt && existing) {
            const existingStartsAt = existing.startsAt ?? existing.starts_at;
            const duration = Date.parse(endsAt) - Date.parse(existingStartsAt);
            if (Number.isFinite(duration) && duration > 0) {
                endsAt = new Date(Date.parse(startsAt) + duration).toISOString();
            }
        }
        if (endsAt !== null && endsAt !== undefined) {
            endsAt = timestamp(endsAt, '计划结束时间');
            if (Date.parse(endsAt) <= Date.parse(startsAt)) throw statusError('计划结束时间无效', 400);
        }
        return {startsAt, endsAt: endsAt ?? null};
    }

    function create(command = {}) {
        const input = record(command, '日程创建请求');
        const personaId = personaIdFor(input);
        requirePersona(personaId);
        if (input.explicitlyAccepted !== true) throw statusError('只有明确、已接受且有具体时间的计划可以写入日程', 400);
        const title = requiredText(input.title, '计划标题', MAX_TITLE_LENGTH);
        const window = normalizeWindow(input);
        const sourceMessageId = input.sourceMessageId ?? input.source_message_id;
        const accepted = methodFor(conversations, ['verifiedAcceptedPlan', 'findAcceptedPlan', 'acceptedPlanFor'], 'conversation accepted-plan', {optional: true});
        const verifyResult = accepted && sourceMessageId
            ? accepted({personaId, sourceMessageId})
            : undefined;
        const payload = {
            ...input,
            personaId,
            title,
            kind: input.kind ?? 'plan',
            source: input.source ?? 'explicit_chat_plan',
            startsAt: window.startsAt,
            endsAt: window.endsAt,
            scene: input.scene === undefined ? undefined : requiredText(input.scene, '场景', MAX_SCENE_LENGTH),
            sourceMessageId: sourceMessageId ?? null,
            createdAt: input.createdAt ?? now()
        };
        return mapMaybe(verifyResult, acceptedPlan => operation(['createSchedule', 'create', 'insert'], 'create')({...payload, acceptedPlan}));
    }

    function reschedule(command = {}) {
        const input = record(command, '日程改期请求');
        const personaId = personaIdFor(input);
        const scheduleId = requiredText(input.scheduleId ?? input.schedule_id, '日程 ID');
        requirePersona(personaId);
        const existing = currentSchedule(personaId, scheduleId);
        const window = normalizeWindow(input, existing);
        const title = input.title === undefined
            ? existing?.title
            : requiredText(input.title, '计划标题', MAX_TITLE_LENGTH);
        if (!title) throw statusError('计划标题不能为空', 400);
        return operation(['rescheduleSchedule', 'reschedule', 'update'], 'reschedule')({
            ...input,
            personaId,
            scheduleId,
            title,
            startsAt: window.startsAt,
            endsAt: window.endsAt,
            updatedAt: input.updatedAt ?? now()
        });
    }

    function cancel(command = {}) {
        const input = record(command, '日程取消请求');
        const personaId = personaIdFor(input);
        const scheduleId = requiredText(input.scheduleId ?? input.schedule_id, '日程 ID');
        requirePersona(personaId);
        currentSchedule(personaId, scheduleId);
        return operation(['cancelSchedule', 'cancel', 'delete'], 'cancel')({
            ...input, personaId, scheduleId, cancelledAt: input.cancelledAt ?? now()
        });
    }

    return Object.freeze({
        create,
        createSchedule: create,
        reschedule,
        rescheduleSchedule: reschedule,
        cancel,
        cancelSchedule: cancel
    });
}

export const createScheduleApplicationService = createScheduleService;
export default createScheduleService;
