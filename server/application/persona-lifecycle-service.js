const MAX_ID_LENGTH = 240;
const MAX_NAME_LENGTH = 160;
const MAX_ROLE_LENGTH = 160;
const MAX_FOUNDATION_LENGTH = 6_000;
const IMAGE_POLICIES = new Set(['ask', 'always', 'important', 'user_only', 'autonomous']);
const INITIALIZATION_MODES = new Set(['llm_defined', 'blank_slate']);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
}

function requiredText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') throw statusError(`${field}不能为空`, 400);
    const normalized = value.trim();
    if (normalized.length > maxLength) throw statusError(`${field}不能超过 ${maxLength} 个字符`, 400);
    return normalized;
}

function record(value, field) {
    if (!isRecord(value)) throw statusError(`${field}必须是 JSON 对象`, 400);
    return value;
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
    throw statusError(`人格生命周期缺少 ${field}.${names[0]}()`, 501);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`人格生命周期 ${field}() 必须同步返回`);
    return value;
}

function clockFor(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.now === 'function') return value.now.bind(value);
    return () => new Date().toISOString();
}

function valueFor(row, names, fallback = undefined) {
    for (const name of names) if (row?.[name] !== undefined) return row[name];
    return fallback;
}

function booleanValue(value) {
    return value === true || value === 1 || value === '1' || value === 'true' || Boolean(value && value !== '0');
}

function summaryDto(row, group) {
    if (!isRecord(row)) throw new TypeError('人格 port 未返回人格');
    const groupId = valueFor(row, ['groupId', 'group_id'], null);
    const screenedAt = valueFor(row, ['screenedAt', 'screened_at', 'screened'], null);
    return {
        id: row.id,
        initializationMode: valueFor(row, ['initializationMode', 'initialization_mode'], 'llm_defined'),
        name: row.name,
        role: row.role,
        color: row.color,
        groupId,
        groupName: valueFor(row, ['groupName', 'group_name'], group?.name ?? null),
        screened: booleanValue(screenedAt),
        currentSituation: String(valueFor(row, ['currentSituation', 'current_situation', 'situation'], '')),
        mood: String(valueFor(row, ['mood'], '')),
        unreadCount: Math.max(0, Number(valueFor(row, ['unreadCount', 'unread_count'], 0)) || 0),
        updatedAt: valueFor(row, ['updatedAt', 'updated_at'])
    };
}

function mapMaybe(value, mapper) {
    return value && typeof value.then === 'function' ? value.then(mapper) : mapper(value);
}

/**
 * Owns persona lifecycle policy while delegating persistence and life-model
 * work to explicit ports. The service is intentionally usable with either
 * synchronous SQLite adapters or asynchronous test/application adapters.
 */
export function createPersonaLifecycleService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Persona lifecycle options must be an object');
    const personas = sourceFor(options, ['persona', 'personas', 'personaRepository', 'personaPort']);
    const groups = sourceFor(options, ['group', 'groups', 'groupRepository', 'groupPort']);
    const conversations = sourceFor(options, ['conversation', 'conversationRepository', 'conversationPort']);
    const lifecycle = sourceFor(options, ['personaLifecycle', 'lifecycle', 'identityPort', 'identity']);
    const clock = clockFor(options.clock ?? options.now);

    const findActive = methodFor(personas, ['findActive', 'find', 'get'], 'persona port', {optional: true});
    const listActive = methodFor(personas, ['listActive', 'list'], 'persona port', {optional: true});

    function requireActive(personaId) {
        const id = requiredText(personaId, '人格 ID');
        let row = findActive ? sync(findActive(id), 'persona lookup') : undefined;
        if (!row && listActive) {
            const rows = sync(listActive(), 'persona list');
            if (!Array.isArray(rows)) throw new TypeError('Persona list port must return an array');
            row = rows.find(item => item?.id === id);
        }
        if (!row) throw statusError('人格不存在', 404);
        return row;
    }

    function callLifecycle(names, input, field) {
        const operation = methodFor(lifecycle, names, field, {optional: true})
            ?? methodFor(personas, names, 'persona port', {optional: true});
        if (!operation) throw statusError(`人格生命周期缺少 ${field} port`, 501);
        return operation(input);
    }

    function normalizeCreate(command) {
        const input = record(command, '人格请求');
        const initializationMode = input.initializationMode ?? input.initialization_mode ?? 'llm_defined';
        if (!INITIALIZATION_MODES.has(initializationMode)) throw statusError('人格初始化模式无效', 400);
        const name = initializationMode === 'blank_slate'
            ? (input.name === undefined || input.name === null ? '' : String(input.name).trim().slice(0, MAX_NAME_LENGTH))
            : requiredText(input.name, '人格名称', MAX_NAME_LENGTH);
        const role = initializationMode === 'blank_slate'
            ? (input.role === undefined || input.role === null ? '' : String(input.role).trim().slice(0, MAX_ROLE_LENGTH))
            : requiredText(input.role, '人格角色', MAX_ROLE_LENGTH);
        const foundation = input.foundation === undefined
            ? undefined
            : requiredText(input.foundation, '基础设定', MAX_FOUNDATION_LENGTH);
        if (initializationMode === 'blank_slate' && foundation) throw statusError('白纸模式不能提供基础人格设定', 400);
        if (input.color !== undefined && (typeof input.color !== 'string' || !/^#[0-9a-f]{6}$/i.test(input.color))) {
            throw statusError('人格颜色无效', 400);
        }
        return { ...input, initializationMode, name, role, ...(foundation === undefined ? {} : {foundation}) };
    }

    function createPersona(command = {}) {
        return callLifecycle(['createPersona', 'create'], normalizeCreate(command), 'createPersona');
    }

    function deletePersona(command = {}) {
        const input = record(command, '人格删除请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        requireActive(personaId);
        return mapMaybe(callLifecycle(['deletePersona', 'delete'], {...input, personaId}, 'deletePersona'), value => {
            if (value === undefined) return {id: personaId, deleted: true, deletedMediaIds: []};
            return value;
        });
    }

    function getPersona(command = {}) {
        const input = record(command, '人格详情请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        requireActive(personaId);
        return callLifecycle(['getPersona', 'readPersona', 'get'], {...input, personaId}, 'getPersona');
    }

    function screenPersona(command = {}) {
        const input = record(command, '人格屏蔽请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        if (typeof input.screened !== 'boolean') throw statusError('screened 必须是布尔值', 400);
        const persona = requireActive(personaId);
        const updateScreen = methodFor(personas, ['updateScreen', 'screen'], 'persona port', {optional: true});
        if (!updateScreen) throw statusError('人格生命周期缺少 screen port', 501);
        const updatedAt = clock();
        const updated = sync(updateScreen({personaId, screenedAt: input.screened ? updatedAt : null, updatedAt}), 'screen');
        if (!updated) throw statusError('人格不存在', 404);
        if (input.screened && conversations) {
            const updateReadAt = methodFor(conversations, ['updateReadAt', 'markRead'], 'conversation port', {optional: true});
            if (updateReadAt) sync(updateReadAt({personaId, role: 'assistant', readAt: updatedAt}), 'read marker');
        }
        const groupId = valueFor(updated, ['groupId', 'group_id'], valueFor(persona, ['groupId', 'group_id'], null));
        const group = groups ? methodFor(groups, ['find', 'get'], 'group port', {optional: true}) : null;
        return mapMaybe(group && groupId ? sync(group(groupId), 'group lookup') : undefined, found => summaryDto(updated, found));
    }

    function updateImageGenerationPolicy(command = {}) {
        const input = record(command, '生图策略请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        const policy = requiredText(input.policy ?? input.imageGenerationPolicy, '生图策略', 40);
        if (!IMAGE_POLICIES.has(policy)) throw statusError('人格生图频率无效', 400);
        requireActive(personaId);
        const update = methodFor(personas, ['updateImageGenerationPolicy', 'setImageGenerationPolicy'], 'persona port', {optional: true});
        if (!update) throw statusError('人格生命周期缺少 image policy port', 501);
        const updated = sync(update({personaId, policy, updatedAt: input.updatedAt ?? clock()}), 'image policy');
        if (!updated) throw statusError('人格不存在', 404);
        const group = groups && valueFor(updated, ['groupId', 'group_id']) ? methodFor(groups, ['find', 'get'], 'group port', {optional: true}) : null;
        return group && valueFor(updated, ['groupId', 'group_id'])
            ? summaryDto(updated, sync(group(valueFor(updated, ['groupId', 'group_id'])), 'group lookup'))
            : summaryDto(updated);
    }

    function groupSummary(command = {}) {
        const input = isRecord(command) ? command : {};
        const groupPort = groups;
        const listGroups = methodFor(groupPort, ['list', 'listAll'], 'group port', {optional: true});
        if (!listGroups) throw statusError('人格生命周期缺少 group summary port', 501);
        const rows = sync(listGroups(input), 'group summary');
        if (!Array.isArray(rows)) throw new TypeError('Group list port must return an array');
        return rows.map(row => ({
            id: row.id,
            name: valueFor(row, ['name', 'groupName', 'group_name'], null),
            isDefault: booleanValue(valueFor(row, ['isDefault', 'is_default'], false)),
            personaCount: Math.max(0, Number(valueFor(row, ['personaCount', 'persona_count'], 0)) || 0)
        }));
    }

    const api = {
        create: createPersona,
        createPersona,
        delete: deletePersona,
        deletePersona,
        get: getPersona,
        getPersona,
        screen: screenPersona,
        screenPersona,
        updateImageGenerationPolicy,
        setImageGenerationPolicy: updateImageGenerationPolicy,
        groupSummary,
        summary: groupSummary
    };
    return Object.freeze(api);
}

export const createPersonaLifecycleApplicationService = createPersonaLifecycleService;
export default createPersonaLifecycleService;
