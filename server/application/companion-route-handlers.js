import {COMPANION_ROUTE_CONTRACT} from '../http/route-registry.js';

const DEFAULT_HEALTH_RESPONSE = Object.freeze({ok: true, storage: 'companion-v2'});
const MAX_ID_LENGTH = 240;
const MAX_TEXT_LENGTH = 12_000;

const ROUTE_GROUPS = Object.freeze({
    bootstrap: [['bootstrap', 'read'], ['bootstrap', 'get']],
    settings: [['settings', 'update'], ['settings', 'save'], ['settings', 'write'], ['settings', 'read']],
    models: [['models', 'list'], ['models', 'get']],
    interviewPreview: [['interview', 'preview'], ['interviews', 'preview']],
    interviewAnalyze: [['interview', 'analyze'], ['interviews', 'analyze']],
    createInterview: [['interview', 'create'], ['interviews', 'create']],
    getInterview: [['interview', 'get'], ['interviews', 'get']],
    answerInterview: [['interview', 'answer'], ['interviews', 'answer']],
    activateInterview: [['interview', 'activate'], ['interviews', 'activate']],
    createPersona: [['persona', 'create'], ['personas', 'create'], ['identity', 'createPersona']],
    createGroup: [['group', 'create'], ['groups', 'create']],
    deletePersona: [['persona', 'delete'], ['personas', 'delete'], ['identity', 'deletePersona']],
    assignPersonaGroup: [['persona', 'assignGroup'], ['personas', 'assignGroup'], ['group', 'assignPersona']],
    updateImageGenerationPolicy: [['persona', 'updateImageGenerationPolicy'], ['personas', 'updateImageGenerationPolicy'], ['persona', 'setImageGenerationPolicy']],
    getPersona: [['persona', 'get'], ['personas', 'get'], ['identity', 'getPersona']],
    getFoundationDraft: [['foundation', 'draft'], ['persona', 'getFoundationDraft']],
    updateFoundation: [['foundation', 'update'], ['persona', 'updateFoundation']],
    restoreFoundationRevision: [['foundation', 'restoreRevision'], ['persona', 'restoreFoundationRevision']],
    rollbackEvolution: [['evolution', 'rollback'], ['relationship', 'rollbackEvolution']],
    screenPersona: [['persona', 'screen'], ['personas', 'screen']],
    createSchedule: [['schedule', 'create'], ['schedules', 'create']],
    rescheduleSchedule: [['schedule', 'reschedule'], ['schedules', 'reschedule']],
    cancelSchedule: [['schedule', 'cancel'], ['schedules', 'cancel']],
    deleteMemory: [['memory', 'delete'], ['memories', 'delete']],
    listConversations: [['conversation', 'list'], ['conversations', 'list']],
    appendConversationMessage: [['conversation', 'appendMessage'], ['conversations', 'appendMessage'], ['conversation', 'append']],
    chat: [['conversation', 'chat'], ['chat', 'run'], ['chat', 'stream']],
    listActivities: [['activity', 'list'], ['activities', 'list']],
    markActivitiesRead: [['activity', 'markRead'], ['activities', 'markRead']],
    commentActivity: [['activity', 'comment'], ['activities', 'comment']],
    likeActivity: [['activity', 'like'], ['activities', 'like']],
    hideActivity: [['activity', 'hide'], ['activities', 'hide']],
    getMedia: [['media', 'get'], ['media', 'read'], ['asset', 'read'], ['assetReader', 'readAsset']],
    listPromptRuns: [['debug', 'listPromptRuns'], ['promptRuns', 'list']],
    h3Preflight: [['debug', 'h3Preflight'], ['h3', 'preflight']],
    getDebugContext: [['debug', 'getContext'], ['debug', 'context']],
    getLifecycle: [['debug', 'getLifecycle'], ['lifecycle', 'get']],
    simulatePersona: [['debug', 'simulatePersona'], ['simulation', 'run']],
    debugMedia: [['debug', 'media'], ['media', 'debug']]
});

const ROUTE_OPERATION_METHODS = Object.freeze([
    'execute', 'run', 'handle', 'invoke', 'call', 'stream', 'read', 'get', 'list',
    'create', 'update', 'save', 'write', 'delete', 'append', 'preview', 'analyze',
    'answer', 'activate', 'assignGroup', 'setImageGenerationPolicy', 'restoreRevision',
    'rollback', 'screen', 'reschedule', 'cancel', 'markRead', 'comment', 'like', 'hide',
    'preflight', 'context', 'simulate', 'debug', 'readAsset', 'readCandidate', 'submit',
    'generate', 'poll', 'status', 'handleChatTurn', 'handleChat', 'streamChat', 'streamCompletion'
]);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertDependencies(value, field) {
    if (value === undefined) return Object.freeze({});
    if (!isRecord(value)) throw new TypeError(`Companion route ${field} must be an object`);
    return value;
}

function nonEmptyText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function text(value, field, maxLength = MAX_TEXT_LENGTH, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    if (value.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    const normalized = value.trim();
    if (!allowEmpty && normalized === '') throw new TypeError(`${field} must not be empty`);
    return normalized;
}

function requestBody(req, {optional = false} = {}) {
    const body = req?.body;
    if (body === undefined && optional) return undefined;
    if (!isRecord(body)) throw new TypeError('请求体必须是 JSON 对象');
    return body;
}

function routeParams(req) {
    return isRecord(req?.params) ? req.params : {};
}

function routeQuery(req) {
    return isRecord(req?.query) ? req.query : {};
}

function requiredParam(req, name) {
    return nonEmptyText(routeParams(req)[name], `请求参数 ${name}`);
}

function requiredBodyParam(body, name, {maxLength = MAX_TEXT_LENGTH} = {}) {
    return nonEmptyText(body?.[name], `请求字段 ${name}`, maxLength);
}

function bodyCommand(req, {optional = false} = {}) {
    const body = requestBody(req, {optional});
    return body === undefined ? {} : {...body};
}

function commandWithParams(req, body = {}) {
    const params = routeParams(req);
    const query = routeQuery(req);
    return {...body, ...query, ...params};
}

function commandWithRequiredParams(req, names, body = {}) {
    for (const name of names) requiredParam(req, name);
    return commandWithParams(req, body);
}

function queryCommand(req) {
    return {...routeQuery(req)};
}

function responseJson(res, value, status = 200) {
    if (status !== undefined && typeof res?.status === 'function') res.status(status);
    if (typeof res?.json === 'function') {
        res.json(value);
        return value;
    }
    return value;
}

function responseEmpty(res, status = 204) {
    if (typeof res?.status === 'function') res.status(status);
    if (typeof res?.end === 'function') res.end();
    return undefined;
}

function responseEnded(res) {
    return Boolean(res?.headersSent || res?.writableEnded || res?.writableFinished || res?.destroyed);
}

function debugDisabledResponse(res) {
    return responseJson(res, {error: 'Debug inspector is disabled'}, 404);
}

function boundedNotImplemented(routeName, res) {
    const payload = {error: `Companion route "${routeName}" is not configured`};
    return responseJson(res, payload, 501);
}

function responseHeaders(res, headers) {
    if (typeof res?.set === 'function') {
        res.set(headers);
        return;
    }
    if (typeof res?.setHeader === 'function') {
        for (const [name, value] of Object.entries(headers)) res.setHeader(name, value);
    }
}

function prepareSseResponse(res) {
    if (typeof res?.status === 'function') res.status(200);
    responseHeaders(res, {
        'Content-Type': 'text/event-stream; charset=utf-8',
        'Cache-Control': 'no-cache',
        Connection: 'keep-alive'
    });
    res?.flushHeaders?.();
}

function methodCandidates(routeName, {includeGeneric = true} = {}) {
    const pairs = ROUTE_GROUPS[routeName] || [];
    return [
        routeName,
        ...pairs.map(([, method]) => method),
        ...(includeGeneric ? ROUTE_OPERATION_METHODS : [])
    ];
}

function findMethod(value, routeName, preferredMethod, {includeGeneric = true} = {}) {
    if (typeof value === 'function') return {invoke: value, receiver: null};
    if (!isRecord(value)) return null;
    const names = [preferredMethod, routeName, ...methodCandidates(routeName, {includeGeneric})]
        .filter(Boolean);
    for (const method of [...new Set(names)]) {
        if (typeof value[method] === 'function') return {invoke: value[method], receiver: value};
    }
    return null;
}

function resolveUseCase(routeName, containers) {
    for (const {value, source} of containers) {
        if (!isRecord(value)) continue;

        const direct = findMethod(value[routeName], routeName);
        if (direct) return {...direct, source};

        for (const [group, preferredMethod] of ROUTE_GROUPS[routeName] || []) {
            const grouped = findMethod(value[group], routeName, preferredMethod);
            if (grouped) return {...grouped, source};
        }

        const root = findMethod(value, routeName, undefined, {includeGeneric: false});
        if (root) return {...root, source};
    }
    return null;
}

function invocationContext({repositories, services, policies, adapters}, routeName, req, res) {
    return {
        route: routeName,
        request: req,
        response: res,
        repositories,
        services,
        policies,
        adapters
    };
}

// Keep the transport envelope discoverable to use-cases without changing the
// enumerable command DTO that gets validated, logged, or passed to mappers.
function attachRequestContext(command, context) {
    if (!isRecord(command)) return command;
    for (const [name, value] of Object.entries({
        request: context.request,
        response: context.response,
        context
    })) {
        if (Object.hasOwn(command, name)) continue;
        Object.defineProperty(command, name, {value, enumerable: false});
    }
    return command;
}

function invokeUseCase(resolved, command, context, {responseFirst = false} = {}) {
    if (!resolved) return undefined;
    const receiver = resolved.receiver;
    const args = responseFirst ? [command, context.response] : [command, context];
    return resolved.invoke.apply(receiver, args);
}

function mapDto(adapters, routeName, value, command, context) {
    const mappers = adapters.dtoMappers ?? adapters.dto ?? adapters.responseMappers;
    if (!isRecord(mappers)) return value;
    const mapper = mappers[routeName];
    if (typeof mapper !== 'function') return value;
    return mapper(value, command, context);
}

function presentResult({res, adapters, routeName, command, context, result, status, empty = false}) {
    if (result && isRecord(result) && result.http && isRecord(result.http)) {
        const http = result.http;
        if (http.status === 204 || http.empty === true) return responseEmpty(res, http.status ?? 204);
        return responseJson(res, mapDto(adapters, routeName, http.body, command, context), http.status ?? status);
    }
    if (empty || status === 204) return responseEmpty(res, status);
    if (result === undefined && routeName === 'getMedia') {
        // Asset providers stream directly and may resolve with no DTO. A
        // missing repository row, however, must not leave the HTTP request
        // without a response.
        return responseEnded(res) ? result : responseJson(res, {error: 'Media does not exist'}, 404);
    }
    let dto = result;
    if (routeName === 'appendConversationMessage' && Array.isArray(dto)) {
        dto = {message: dto[0] ?? null, messages: dto};
    } else if (routeName === 'listConversations' && Array.isArray(dto)) {
        dto = {items: dto, nextCursor: null};
    }
    const mappedStatus = routeName === 'restoreFoundationRevision' && dto?.restored === true ? 201 : status;
    return responseJson(res, mapDto(adapters, routeName, dto, command, context), mappedStatus);
}

function invokeJson({routeName, req, res, command, status = 200, empty = false, dependencies}) {
    const {repositories, services, policies, adapters} = dependencies;
    const context = invocationContext(dependencies, routeName, req, res);
    attachRequestContext(command, context);
    const resolved = resolveUseCase(routeName, [
        {value: services, source: 'services'},
        {value: adapters, source: 'adapters'}
    ]);
    if (!resolved) return boundedNotImplemented(routeName, res);
    const result = invokeUseCase(resolved, command, context);
    if (result && typeof result.then === 'function') {
        return result.then(value => presentResult({res, adapters, routeName, command, context, result: value, status, empty}));
    }
    return presentResult({res, adapters, routeName, command, context, result, status, empty});
}

function invokeStreaming({routeName, req, res, command, dependencies}) {
    const {services, adapters} = dependencies;
    const context = invocationContext(dependencies, routeName, req, res);
    const resolved = resolveUseCase(routeName, [
        {value: services, source: 'services'},
        {value: adapters, source: 'adapters'}
    ]);
    if (!resolved) return boundedNotImplemented(routeName, res);
    prepareSseResponse(res);
    const invocation = {context, command, req, res, request: req, response: res, sink: res};
    const result = invokeUseCase(resolved, invocation, context, {responseFirst: true});
    return result;
}

function validatePolicy(body, policies) {
    const policy = body.policy ?? body.imageGenerationPolicy;
    const allowed = policies.imageGenerationPolicies ?? policies.imagePolicies;
    if (!Array.isArray(allowed)) return nonEmptyText(policy, '请求字段 policy', 80);
    if (allowed.length === 0 || !allowed.includes(policy)) {
        throw Object.assign(new TypeError('人格生图频率无效'), {status: 400});
    }
    return policy;
}

function descriptor(routeName) {
    const descriptors = {
        health: ({req, res, dependencies}) => {
            const configured = dependencies.policies.healthResponse ?? dependencies.adapters.healthResponse;
            if (configured !== undefined) {
                const result = typeof configured === 'function'
                    ? configured(req, invocationContext(dependencies, routeName, req, res))
                    : configured;
                if (result && typeof result.then === 'function') return result.then(value => responseJson(res, value));
                return responseJson(res, result);
            }
            const resolved = resolveUseCase(routeName, [{value: dependencies.services, source: 'services'}]);
            if (resolved) {
                const context = invocationContext(dependencies, routeName, req, res);
                const result = invokeUseCase(resolved, {}, context);
                if (result && typeof result.then === 'function') return result.then(value => responseJson(res, value));
                return responseJson(res, result);
            }
            return responseJson(res, DEFAULT_HEALTH_RESPONSE);
        },
        bootstrap: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: queryCommand(req), dependencies}),
        settings: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            return invokeJson({routeName, req, res, command: body, dependencies});
        },
        models: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: queryCommand(req), dependencies}),

        interviewPreview: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: bodyCommand(req), dependencies}),
        interviewAnalyze: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: bodyCommand(req), status: 201, dependencies}),
        createInterview: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: bodyCommand(req, {optional: true}), status: 201, dependencies}),
        getInterview: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['interviewId']), dependencies}),
        answerInterview: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['interviewId'], bodyCommand(req)), dependencies}),
        activateInterview: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['interviewId'], bodyCommand(req, {optional: true})), status: 201, dependencies}),

        createPersona: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: bodyCommand(req), status: 201, dependencies}),
        createGroup: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            requiredBodyParam(body, 'name', {maxLength: 60});
            return invokeJson({routeName, req, res, command: body, status: 201, dependencies});
        },
        deletePersona: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        assignPersonaGroup: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            requiredBodyParam(body, 'groupId');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], body), dependencies});
        },
        updateImageGenerationPolicy: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            const policy = validatePolicy(body, dependencies.policies);
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], {...body, policy}), dependencies});
        },
        getPersona: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        getFoundationDraft: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        updateFoundation: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            requiredBodyParam(body, 'foundation', {maxLength: 6_000});
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], body), status: 201, dependencies});
        },
        restoreFoundationRevision: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId', 'revisionId']), dependencies}),
        rollbackEvolution: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId', 'evolutionId']), dependencies}),
        screenPersona: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            if (typeof body.screened !== 'boolean') throw new TypeError('screened 必须是布尔值');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], body), dependencies});
        },
        createSchedule: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            if (body.explicitlyAccepted !== true) throw new TypeError('只有明确、已接受且有具体时间的计划可以写入日程');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], body), status: 201, dependencies});
        },
        rescheduleSchedule: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId', 'scheduleId'], bodyCommand(req)), dependencies}),
        cancelSchedule: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId', 'scheduleId']), status: 204, empty: true, dependencies}),
        deleteMemory: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId', 'memoryId']), status: 204, empty: true, dependencies}),

        listConversations: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        appendConversationMessage: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            if (!['assistant', 'user'].includes(body.role)) throw new TypeError('消息角色无效');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], body), status: 201, dependencies});
        },
        chat: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            requiredBodyParam(body, 'personaId');
            if (body.text !== undefined) text(body.text, '请求字段 text', MAX_TEXT_LENGTH, {allowEmpty: false});
            if (body.attachments !== undefined && !Array.isArray(body.attachments)) throw new TypeError('attachments 必须是数组');
            return invokeStreaming({routeName, req, res, command: body, dependencies});
        },

        listActivities: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: queryCommand(req), dependencies}),
        markActivitiesRead: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: {}, status: 204, empty: true, dependencies}),
        commentActivity: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            requiredBodyParam(body, 'content');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['activityId'], body), status: 201, dependencies});
        },
        likeActivity: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            if (typeof body.liked !== 'boolean') throw new TypeError('liked 必须是布尔值');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['activityId'], body), dependencies});
        },
        hideActivity: ({req, res, dependencies}) => {
            const body = bodyCommand(req);
            if (typeof body.hidden !== 'boolean') throw new TypeError('hidden 必须是布尔值');
            return invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['activityId'], body), dependencies});
        },

        getMedia: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['mediaId']), dependencies}),
        listPromptRuns: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: queryCommand(req), dependencies}),
        h3Preflight: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: bodyCommand(req, {optional: true}), dependencies}),
        getDebugContext: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        getLifecycle: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId']), dependencies}),
        simulatePersona: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], bodyCommand(req)), status: 201, dependencies}),
        debugMedia: ({req, res, dependencies}) => invokeJson({routeName, req, res, command: commandWithRequiredParams(req, ['personaId'], bodyCommand(req)), status: 202, dependencies})
    };
    return descriptors[routeName];
}

/**
 * Compose the production companion HTTP handlers from application use-cases.
 *
 * This module intentionally owns no persistence, provider, or Express setup.
 * Repositories, policies, and adapters are carried in the use-case context so
 * route handlers remain a transport boundary rather than a second domain
 * implementation. Every contract route receives a function; an unconfigured
 * use-case responds with a bounded 501 instead of disappearing from the API.
 */
export function createCompanionRouteHandlers({repositories, services, policies, adapters, debugInspectorEnabled = false} = {}) {
    const dependencies = Object.freeze({
        repositories: assertDependencies(repositories, 'repositories'),
        services: assertDependencies(services, 'services'),
        policies: assertDependencies(policies, 'policies'),
        adapters: assertDependencies(adapters, 'adapters'),
        debugInspectorEnabled: debugInspectorEnabled === true
    });

    const handlers = {};
    for (const definition of COMPANION_ROUTE_CONTRACT) {
        const routeName = definition.handler;
        const route = descriptor(routeName);
        if (typeof route !== 'function') throw new Error(`No companion route descriptor for ${routeName}`);
        handlers[routeName] = (req, res) => {
            if (definition.debug && dependencies.debugInspectorEnabled !== true) {
                return debugDisabledResponse(res);
            }
            return route({req, res, dependencies});
        };
    }
    return Object.freeze(handlers);
}

export const createProductionCompanionRouteHandlers = createCompanionRouteHandlers;
export default createCompanionRouteHandlers;
