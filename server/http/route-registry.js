/**
 * The public companion HTTP surface, expressed as data so the composition
 * root can provide the application handlers without making this module aware
 * of persistence, providers, or the legacy server entrypoint.
 */
const ROUTE_DEFINITIONS = [
    {id: 'health', method: 'GET', path: '/api/health', handler: 'health'},
    {id: 'bootstrap', method: 'GET', path: '/api/companion/bootstrap', handler: 'bootstrap'},
    {id: 'settings', method: 'PUT', path: '/api/companion/settings', handler: 'settings'},
    {id: 'models', method: 'GET', path: '/api/companion/models', handler: 'models'},

    {id: 'interviewPreview', method: 'POST', path: '/api/companion/interviews/preview', handler: 'interviewPreview'},
    {id: 'interviewAnalyze', method: 'POST', path: '/api/companion/interviews/analyze', handler: 'interviewAnalyze'},
    {id: 'createInterview', method: 'POST', path: '/api/companion/interviews', handler: 'createInterview'},
    {id: 'getInterview', method: 'GET', path: '/api/companion/interviews/:interviewId', handler: 'getInterview'},
    {id: 'answerInterview', method: 'POST', path: '/api/companion/interviews/:interviewId/answers', handler: 'answerInterview'},
    {id: 'activateInterview', method: 'POST', path: '/api/companion/interviews/:interviewId/activate', handler: 'activateInterview'},

    {id: 'createPersona', method: 'POST', path: '/api/companion/personas', handler: 'createPersona'},
    {id: 'createGroup', method: 'POST', path: '/api/companion/groups', handler: 'createGroup'},
    {id: 'deletePersona', method: 'DELETE', path: '/api/companion/personas/:personaId', handler: 'deletePersona'},
    {id: 'assignPersonaGroup', method: 'PUT', path: '/api/companion/personas/:personaId/group', handler: 'assignPersonaGroup'},
    {id: 'updateImageGenerationPolicy', method: 'PUT', path: '/api/companion/personas/:personaId/image-generation-policy', handler: 'updateImageGenerationPolicy'},
    {id: 'getPersona', method: 'GET', path: '/api/companion/personas/:personaId', handler: 'getPersona'},
    {id: 'getFoundationDraft', method: 'GET', path: '/api/companion/personas/:personaId/foundation/draft', handler: 'getFoundationDraft'},
    {id: 'updateFoundation', method: 'PUT', path: '/api/companion/personas/:personaId/foundation', handler: 'updateFoundation'},
    {id: 'restoreFoundationRevision', method: 'POST', path: '/api/companion/personas/:personaId/foundation-revisions/:revisionId/restore', handler: 'restoreFoundationRevision'},
    {id: 'rollbackEvolution', method: 'POST', path: '/api/companion/personas/:personaId/evolutions/:evolutionId/rollback', handler: 'rollbackEvolution'},
    {id: 'screenPersona', method: 'PUT', path: '/api/companion/personas/:personaId/screen', handler: 'screenPersona'},

    {id: 'createSchedule', method: 'POST', path: '/api/companion/personas/:personaId/schedule', handler: 'createSchedule'},
    {id: 'rescheduleSchedule', method: 'PATCH', path: '/api/companion/personas/:personaId/schedule/:scheduleId', handler: 'rescheduleSchedule'},
    {id: 'cancelSchedule', method: 'POST', path: '/api/companion/personas/:personaId/schedule/:scheduleId/cancel', handler: 'cancelSchedule'},
    {id: 'deleteMemory', method: 'DELETE', path: '/api/companion/personas/:personaId/memories/:memoryId', handler: 'deleteMemory'},

    {id: 'listConversations', method: 'GET', path: '/api/companion/conversations/:personaId', handler: 'listConversations'},
    {id: 'appendConversationMessage', method: 'POST', path: '/api/companion/conversations/:personaId/messages', handler: 'appendConversationMessage'},
    {id: 'chat', method: 'POST', path: '/api/companion/chat', handler: 'chat', sse: true},

    {id: 'listActivities', method: 'GET', path: '/api/companion/activities', handler: 'listActivities'},
    {id: 'markActivitiesRead', method: 'POST', path: '/api/companion/activities/read', handler: 'markActivitiesRead'},
    {id: 'commentActivity', method: 'POST', path: '/api/companion/activities/:activityId/comments', handler: 'commentActivity'},
    {id: 'likeActivity', method: 'PUT', path: '/api/companion/activities/:activityId/like', handler: 'likeActivity'},
    {id: 'hideActivity', method: 'PUT', path: '/api/companion/activities/:activityId/hide', handler: 'hideActivity'},

    {id: 'getMedia', method: 'GET', path: '/api/companion/media/:mediaId', handler: 'getMedia'},

    {id: 'listPromptRuns', method: 'GET', path: '/api/companion/prompt-runs', handler: 'listPromptRuns', debug: true},
    {id: 'h3Preflight', method: 'POST', path: '/api/companion/h3-preflight', handler: 'h3Preflight', debug: true},
    {id: 'getDebugContext', method: 'GET', path: '/api/companion/personas/:personaId/debug-context', handler: 'getDebugContext', debug: true},
    {id: 'getLifecycle', method: 'GET', path: '/api/companion/personas/:personaId/lifecycle', handler: 'getLifecycle', debug: true},
    {id: 'simulatePersona', method: 'POST', path: '/api/companion/personas/:personaId/simulate', handler: 'simulatePersona', debug: true},
    {id: 'debugMedia', method: 'POST', path: '/api/companion/personas/:personaId/debug-media', handler: 'debugMedia', debug: true}
];

/**
 * Frozen public route metadata. Consumers can use this to build contract
 * fixtures without importing or executing the application implementation.
 */
export const COMPANION_ROUTE_CONTRACT = Object.freeze(
    ROUTE_DEFINITIONS.map(route => Object.freeze({...route}))
);

// Descriptive aliases make the contract discoverable without introducing a
// second mutable route list.
export const COMPANION_ROUTES = COMPANION_ROUTE_CONTRACT;
export const companionRouteContract = COMPANION_ROUTE_CONTRACT;

const METHOD_NAMES = new Set(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']);
const MISSING_HANDLER_POLICIES = new Set(['skip', 'error']);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertApp(app) {
    if (!app || (typeof app !== 'object' && typeof app !== 'function')) {
        throw new TypeError('Companion route registry requires an app');
    }
    for (const method of METHOD_NAMES) {
        if (typeof app[method.toLowerCase()] !== 'function') {
            throw new TypeError(`Companion route registry app must provide ${method.toLowerCase()}()`);
        }
    }
    return app;
}

function assertHandlers(handlers) {
    if (handlers === undefined) return {};
    if (!isRecord(handlers)) throw new TypeError('Companion route handlers must be an object');
    return handlers;
}

function resolveHandler(handlers, definition) {
    const candidates = [definition.handler, definition.id];
    for (const name of candidates) {
        const value = handlers[name];
        if (value !== undefined) return value;
    }
    return undefined;
}

function assertRouteHandler(handler, definition) {
    if (typeof handler !== 'function') {
        throw new TypeError(`Companion route handler "${definition.handler}" must be a function`);
    }
    return handler;
}

function routeMetadata(definition, reason) {
    return Object.freeze({
        id: definition.id,
        method: definition.method,
        path: definition.path,
        handler: definition.handler,
        ...(reason ? {reason} : {})
    });
}

/**
 * Register the companion HTTP surface against an injected Express-like app.
 *
 * Handlers own validation, DTO mapping, persistence and provider behavior.
 * This module only selects the public method/path, applies the common route
 * wrapper, and gates diagnostics. Missing handlers are reported in the return
 * value by default; `missingHandler: 'error'` turns that into a configuration
 * error, which prevents an accidental partially-mounted API.
 */
export function registerCompanionRoutes({
    app,
    handlers,
    debugInspectorEnabled = false,
    wrapRoute,
    sendError,
    missingHandler = 'skip'
} = {}) {
    assertApp(app);
    const providedHandlers = assertHandlers(handlers);
    if (typeof wrapRoute !== 'function') throw new TypeError('Companion route registry requires wrapRoute()');
    if (sendError !== undefined && typeof sendError !== 'function') {
        throw new TypeError('Companion route registry sendError must be a function');
    }
    if (!MISSING_HANDLER_POLICIES.has(missingHandler)) {
        throw new TypeError('Companion route registry missingHandler must be "skip" or "error"');
    }

    const registered = [];
    const skipped = [];
    const debugEnabled = debugInspectorEnabled === true;

    for (const definition of ROUTE_DEFINITIONS) {
        if (definition.debug && !debugEnabled) {
            skipped.push(routeMetadata(definition, 'debug_disabled'));
            continue;
        }

        const handler = resolveHandler(providedHandlers, definition);
        if (handler === undefined) {
            const metadata = routeMetadata(definition, 'handler_missing');
            if (missingHandler === 'error') {
                throw new TypeError(`Companion route handler "${definition.handler}" is required for ${definition.method} ${definition.path}`);
            }
            skipped.push(metadata);
            continue;
        }

        const routeHandler = assertRouteHandler(handler, definition);
        // The injected wrapper owns sync/rejected-promise error conversion.
        // Passing the error helper through keeps custom composition roots on
        // the same bounded `{error}` contract as the default HTTP app.
        const wrapped = wrapRoute(routeHandler, {
            sendError,
            sse: Boolean(definition.sse),
            defaultStatus: definition.sse ? 400 : undefined
        });
        if (typeof wrapped !== 'function') {
            throw new TypeError(`Companion route wrapper did not return a function for ${definition.method} ${definition.path}`);
        }
        app[definition.method.toLowerCase()](definition.path, wrapped);
        registered.push(routeMetadata(definition));
    }

    return Object.freeze({
        debugInspectorEnabled: debugEnabled,
        registered: Object.freeze(registered),
        skipped: Object.freeze(skipped)
    });
}

export default registerCompanionRoutes;
