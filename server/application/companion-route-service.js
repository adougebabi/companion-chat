const MAX_ID_LENGTH = 240;
const MAX_PERSONA_NAME_LENGTH = 160;
const MAX_PERSONA_ROLE_LENGTH = 160;
const MAX_FOUNDATION_LENGTH = 6_000;
const MAX_MESSAGE_LENGTH = 8_000;
const MAX_GROUP_NAME_LENGTH = 60;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertRecord(value, field) {
    if (!isRecord(value)) throw Object.assign(new TypeError(`${field}必须是 JSON 对象`), {status: 400});
    return value;
}

function requiredText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') {
        throw Object.assign(new TypeError(`${field}不能为空`), {status: 400});
    }
    const normalized = value.trim();
    if (normalized.length > maxLength) {
        throw Object.assign(new RangeError(`${field}不能超过 ${maxLength} 个字符`), {status: 400});
    }
    return normalized;
}

function badRequest(message) {
    return Object.assign(new TypeError(message), {status: 400});
}

function notFound(message) {
    return Object.assign(new Error(message), {status: 404});
}

function notConfigured(message) {
    return Object.assign(new Error(message), {status: 501});
}

function resolveClock(clock) {
    if (typeof clock === 'function') return clock;
    if (isRecord(clock) && typeof clock.now === 'function') return clock.now.bind(clock);
    return () => new Date().toISOString();
}

function repository(repositories, names, field) {
    for (const name of names) {
        if (isRecord(repositories?.[name])) return repositories[name];
    }
    throw new TypeError(`Companion route service requires ${field}`);
}

function candidate(value, names) {
    if (typeof value === 'function') return value;
    if (!isRecord(value)) return null;
    for (const name of names) {
        if (typeof value[name] === 'function') return value[name].bind(value);
    }
    return null;
}

function invokeConfigured(candidates, names, args, message) {
    for (const value of candidates) {
        const operation = candidate(value, names);
        if (operation) return operation(...args);
    }
    throw notConfigured(message);
}

function valueOr(value, fallback) {
    return value === undefined || value === null ? fallback : value;
}

function numberValue(value, fallback = 0) {
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

function booleanValue(value) {
    return value === true || value === 1 || value === '1' || value === 'true';
}

function groupDto(row) {
    if (!isRecord(row)) throw new TypeError('分组 repository 未返回分组');
    return {
        id: row.id,
        name: valueOr(row.name ?? row.group_name ?? row.groupName, null),
        isDefault: booleanValue(row.is_default ?? row.isDefault),
        personaCount: numberValue(row.persona_count ?? row.personaCount)
    };
}

function personaDto(row, group) {
    if (!isRecord(row)) throw new TypeError('人格 repository 未返回人格');
    const groupId = row.group_id ?? row.groupId ?? null;
    const screened = row.screened_at ?? row.screenedAt ?? row.screened;
    return {
        id: row.id,
        name: row.name,
        role: row.role,
        color: row.color,
        groupId,
        groupName: row.group_name ?? row.groupName ?? group?.name ?? null,
        screened: booleanValue(screened),
        currentSituation: String(row.current_situation ?? row.currentSituation ?? row.situation ?? ''),
        mood: String(row.mood ?? ''),
        unreadCount: numberValue(row.unread_count ?? row.unreadCount),
        updatedAt: row.updated_at ?? row.updatedAt
    };
}

function pageDto(result) {
    if (Array.isArray(result)) return {items: result, nextCursor: null};
    if (!isRecord(result)) return {items: [], nextCursor: null};
    return {
        ...result,
        items: Array.isArray(result.items) ? result.items : [],
        nextCursor: result.nextCursor ?? result.cursor ?? null
    };
}

function normalizePersonaId(command) {
    const input = assertRecord(command, '请求');
    return requiredText(input.personaId ?? input.persona_id, '人格 ID');
}

function normalizeConversationCommand(command) {
    const input = assertRecord(command, '会话请求');
    const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
    const cursor = input.cursor ?? input.nextCursor;
    return {
        ...input,
        personaId,
        ...(cursor === undefined ? {} : {cursor}),
        ...(input.limit === undefined ? {} : {limit: input.limit})
    };
}

function normalizeMessageCommand(command) {
    const input = assertRecord(command, '消息请求');
    const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
    if (!['assistant', 'user'].includes(input.role)) throw badRequest('消息角色无效');
    if (input.text !== undefined && typeof input.text !== 'string') throw badRequest('消息文本必须是字符串');
    if (typeof input.text === 'string' && input.text.length > MAX_MESSAGE_LENGTH) {
        throw badRequest(`消息文本不能超过 ${MAX_MESSAGE_LENGTH} 个字符`);
    }
    return {...input, personaId, text: typeof input.text === 'string' ? input.text.slice(0, MAX_MESSAGE_LENGTH) : ''};
}

function normalizeGroupName(value) {
    if (typeof value !== 'string') throw badRequest('分组名称必须是文本');
    const name = value.trim();
    if (!name) throw badRequest('分组名称不能为空');
    if (name.length > MAX_GROUP_NAME_LENGTH) throw badRequest(`分组名称不能超过 ${MAX_GROUP_NAME_LENGTH} 个字符`);
    return name;
}

function normalizePersonaCreate(command) {
    const input = assertRecord(command, '人格请求');
    const name = requiredText(input.name, '人格名称', MAX_PERSONA_NAME_LENGTH);
    const role = requiredText(input.role, '人格角色', MAX_PERSONA_ROLE_LENGTH);
    // Foundation/life-model defaults are lifecycle policy. Keep an optional
    // bounded field here, but let the explicit adapter derive or reject it.
    const foundation = input.foundation === undefined
        ? undefined
        : requiredText(input.foundation, '基础设定', MAX_FOUNDATION_LENGTH);
    if (input.color !== undefined && (typeof input.color !== 'string' || !/^#[0-9a-f]{6}$/i.test(input.color))) {
        throw badRequest('人格颜色无效');
    }
    return {...input, name, role, ...(foundation === undefined ? {} : {foundation})};
}

function findActivePersona(personas, personaId) {
    if (typeof personas.findActive === 'function') return personas.findActive(personaId);
    if (typeof personas.listActive === 'function') {
        const rows = personas.listActive();
        if (!Array.isArray(rows)) throw new TypeError('Persona repository listActive() must return an array');
        return rows.find(row => row?.id === personaId);
    }
    throw new TypeError('Persona repository must provide findActive() or listActive()');
}

function requireActivePersona(personas, personaId, {allowUnconfiguredEmpty = false} = {}) {
    const row = findActivePersona(personas, personaId);
    if (!row && allowUnconfiguredEmpty && typeof personas.findActive !== 'function' && typeof personas.listActive === 'function' && personas.listActive().length === 0) {
        return {id: personaId};
    }
    if (!row) throw notFound('人格不存在');
    return row;
}

function readBootstrapPersona(identity, personaId) {
    const read = identity?.bootstrap?.read ?? identity?.readBootstrap ?? identity?.bootstrap?.get;
    if (typeof read !== 'function') return undefined;
    const result = read.call(identity.bootstrap ?? identity, {});
    if (result && typeof result.then === 'function') {
        return result.then(value => value?.personas?.find(persona => persona.id === personaId));
    }
    return result?.personas?.find(persona => persona.id === personaId);
}

function mapMaybe(value, mapper) {
    return value && typeof value.then === 'function' ? value.then(mapper) : mapper(value);
}

/**
 * Application owner for the small companion route slice.
 *
 * The service validates route commands, enforces persona ownership, and maps
 * repository rows to browser DTOs. Persona creation/deletion/detail remain an
 * explicit lifecycle adapter because their complete behavior also initializes
 * or removes foundation, life-model, state, jobs, media, and conversation rows.
 */
export function createCompanionRouteService({
    repositories = {},
    services = {},
    adapters = {},
    identitySettingsService,
    conversationService,
    clock,
    idGenerator
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Companion route repositories must be an object');
    const personas = repository(repositories, ['persona', 'personas', 'personaRepository'], 'persona repository');
    const groups = repository(repositories, ['group', 'groups', 'groupRepository'], 'group repository');
    const conversationRepository = repository(repositories, ['conversation', 'conversationRepository'], 'conversation repository');
    const identity = identitySettingsService ?? services.identity ?? services.identitySettings;
    const conversation = conversationService
        ?? services.conversationService
        ?? services.conversations
        ?? services.conversation;
    if (!isRecord(conversation) || typeof conversation.list !== 'function' || typeof conversation.appendMessage !== 'function') {
        throw new TypeError('Companion route service requires conversation.list() and appendMessage()');
    }
    const now = resolveClock(clock);
    void idGenerator;

    function summaryFor(row) {
        const groupId = row.group_id ?? row.groupId;
        const group = groupId && typeof groups.find === 'function' ? groups.find(groupId) : undefined;
        const identityValue = readBootstrapPersona(identity, row.id);
        return mapMaybe(identityValue, value => value ?? personaDto(row, group));
    }

    function createPersona(command = {}) {
        const input = normalizePersonaCreate(command);
        const lifecycle = adapters.personaLifecycle ?? adapters.identity ?? services.personaLifecycle ?? services.identity;
        const result = invokeConfigured(
            [lifecycle, adapters],
            ['createPersona', 'create'],
            [input],
            '人格创建需要配置 identity lifecycle adapter'
        );
        return mapMaybe(result, value => value);
    }

    function deletePersona(command = {}) {
        const personaId = normalizePersonaId(command);
        // Resolve first so an unknown/deleted ID cannot cause an adapter side effect.
        requireActivePersona(personas, personaId);
        const lifecycle = adapters.personaLifecycle ?? adapters.identity ?? services.personaLifecycle ?? services.identity;
        const result = invokeConfigured(
            [lifecycle, adapters],
            ['deletePersona', 'delete'],
            [{...command, personaId}],
            '人格删除需要配置 identity lifecycle adapter'
        );
        return mapMaybe(result, value => value ?? {id: personaId, deleted: true, deletedMediaIds: []});
    }

    function getPersona(command = {}) {
        const personaId = normalizePersonaId(command);
        requireActivePersona(personas, personaId);
        const lifecycle = adapters.personaLifecycle ?? adapters.identity ?? services.personaLifecycle ?? services.identity;
        return invokeConfigured(
            [lifecycle, adapters],
            ['getPersona', 'readPersona', 'get'],
            [{...command, personaId}],
            '人格详情需要配置 identity lifecycle adapter'
        );
    }

    function createGroup(command = {}) {
        const input = assertRecord(command, '分组请求');
        const name = normalizeGroupName(input.name);
        let row;
        try {
            row = groups.create({name});
        } catch (error) {
            if (/SQLITE_CONSTRAINT|UNIQUE|duplicate|已存在/i.test(String(error?.code || error?.message || error))) {
                throw badRequest('分组名称已存在');
            }
            throw error;
        }
        return groupDto(row);
    }

    function assignPersonaGroup(command = {}) {
        const input = assertRecord(command, '分组归属请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        const groupId = requiredText(input.groupId ?? input.group_id, '分组 ID');
        const persona = requireActivePersona(personas, personaId);
        const group = typeof groups.find === 'function' ? groups.find(groupId) : undefined;
        if (!group) throw notFound('分组不存在');
        if (typeof groups.assignPersona !== 'function') throw new TypeError('Group repository must provide assignPersona()');
        const updated = groups.assignPersona({personaId: persona.id, groupId: group.id, updatedAt: now()});
        if (!updated) throw notFound('人格不存在');
        return summaryFor(updated);
    }

    function screenPersona(command = {}) {
        const input = assertRecord(command, '人格屏蔽请求');
        const personaId = requiredText(input.personaId ?? input.persona_id, '人格 ID');
        if (typeof input.screened !== 'boolean') throw badRequest('screened 必须是布尔值');
        const persona = requireActivePersona(personas, personaId);
        if (typeof personas.updateScreen !== 'function') throw new TypeError('Persona repository must provide updateScreen()');
        const updatedAt = now();
        const updated = personas.updateScreen({
            personaId: persona.id,
            screenedAt: input.screened ? updatedAt : null,
            updatedAt
        });
        if (!updated) throw notFound('人格不存在');
        if (input.screened && typeof conversationRepository.updateReadAt === 'function') {
            conversationRepository.updateReadAt({personaId: persona.id, role: 'assistant', readAt: updatedAt});
        }
        return summaryFor(updated);
    }

    function listConversations(command = {}) {
        const input = normalizeConversationCommand(command);
        const personaId = input.personaId;
        requireActivePersona(personas, personaId, {allowUnconfiguredEmpty: true});
        return mapMaybe(conversation.list(input), pageDto);
    }

    function appendConversationMessage(command = {}) {
        const input = normalizeMessageCommand(command);
        requireActivePersona(personas, input.personaId, {allowUnconfiguredEmpty: true});

        if (input.role === 'assistant') {
            const assistantAppender = candidate(conversation, [
                'appendUserVisibleAssistantReply',
                'appendAssistantReply',
                'appendAssistantMessage'
            ]) ?? candidate(adapters.conversation ?? adapters, [
                'appendUserVisibleAssistantReply',
                'appendAssistantReply'
            ]);
            if (assistantAppender) {
                return mapMaybe(
                    assistantAppender(input.personaId, input.text, input),
                    messages => {
                        const list = Array.isArray(messages) ? messages : [messages].filter(Boolean);
                        return {message: list[0] ?? null, messages: list};
                    }
                );
            }
        }

        return mapMaybe(conversation.appendMessage(input), message => message);
    }

    const personaApi = Object.freeze({
        create: createPersona,
        createPersona,
        delete: deletePersona,
        deletePersona,
        get: getPersona,
        getPersona,
        assignGroup: assignPersonaGroup,
        screen: screenPersona
    });
    const groupApi = Object.freeze({
        create: createGroup,
        assignPersona: assignPersonaGroup,
        assignGroup: assignPersonaGroup
    });
    const conversationApi = Object.freeze({
        list: listConversations,
        appendMessage: appendConversationMessage,
        append: appendConversationMessage
    });

    return Object.freeze({
        persona: personaApi,
        personas: personaApi,
        identity: personaApi,
        group: groupApi,
        groups: groupApi,
        conversations: conversationApi,
        conversation: conversationApi,
        createPersona,
        deletePersona,
        getPersona,
        createGroup,
        assignPersonaGroup,
        screenPersona,
        listConversations,
        appendConversationMessage
    });
}

export const createCompanionRouteApplicationService = createCompanionRouteService;
export default createCompanionRouteService;
