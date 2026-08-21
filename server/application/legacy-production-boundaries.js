/**
 * Machine-readable deletion-gate inventory for production behavior that still
 * has a legacy implementation in server.js.
 *
 * This is intentionally a map, not a compatibility implementation. It lets
 * the final cutover check distinguish a real modular target from a temporary
 * facade and keeps the remaining cross-layer blockers explicit.
 */

export const LEGACY_PRODUCTION_BOUNDARY_VERSION = 1;

const REQUIRED_SYMBOLS = Object.freeze([
    'createEvent',
    'contextFor',
    'streamPersonaChat',
    'mediaProvider',
    'jobHandlers',
    'debugRoutes'
]);

const STATUS_VALUES = new Set(['mapped', 'partial', 'blocked']);

function freezeEntry(entry) {
    if (!STATUS_VALUES.has(entry.status)) throw new TypeError(`Unknown legacy boundary status: ${entry.status}`);
    return Object.freeze({
        ...entry,
        legacy: Object.freeze({...entry.legacy}),
        targetModules: Object.freeze(entry.targetModules.map(value => Object.freeze({...value}))),
        adapters: Object.freeze(entry.adapters.map(value => Object.freeze({...value}))),
        blockers: Object.freeze(entry.blockers.map(value => Object.freeze({...value}))),
        deletionChecks: Object.freeze([...entry.deletionChecks])
    });
}

const BOUNDARIES = [
    {
        key: 'createEvent',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['createEvent', 'instantiateTimelineEvent', 'reconcilePersona'],
            location: 'server.js:1754-1848'
        },
        targetModules: [
            {module: 'server/application/scene-event-flow.js', exports: 'createSceneEventFlow', role: 'shared-scene fact/projection flow'},
            {module: 'server/infrastructure/life-event-repository.js', exports: 'createLifeEventRepository', role: 'life-event persistence port'},
            {module: 'server/application/proactive/worker-flows.js', exports: 'createActivityDecisionFlow', role: 'post-commit activity flow'}
        ],
        adapters: [
            {module: 'server/application/scene-event-flow.js', export: 'createSceneEventFlow', role: 'scene_event-only compatibility target'}
        ],
        blockers: [
            {code: 'event-policy-mixed-with-persistence', detail: 'The legacy function validates automatic-event policy, resolves participants, writes life state, and updates persona state in one transaction.'},
            {code: 'event-fanout-not-effect-intent', detail: 'Activity creation, media enqueue, activity-decision enqueue, and proactive enqueue are still direct SQL/job side effects.'},
            {code: 'timeline-policy-parity-incomplete', detail: 'The modular life-event flow now owns generic fact/state/activity/job fanout, but timeline candidate policy, participant introduction, and recovery reconciliation still need normalized replay.'}
        ],
        deletionChecks: [
            'Register a generic life-event flow with facts, projections, and effect intents.',
            'Move participant validation and automatic-event policy behind injected domain ports.',
            'Replay routine, schedule, recovery, timeline, debug, media, and proactive producers before deleting createEvent.'
        ]
    },
    {
        key: 'contextFor',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['contextFor', 'userVisibleChatPrompt', 'resolvedStateFor'],
            location: 'server.js:2246-2293'
        },
        targetModules: [
            {module: 'server/application/life-world-reader.js', exports: 'createLifeWorldReader', role: 'normalized life-world input'},
            {module: 'server/application/chat-production-adapter.js', exports: 'createChatContextReader', role: 'chat context port adapter'},
            {module: 'server/domain/life-state-resolver.js', exports: 'createLifeStateResolver', role: 'pure state precedence'}
        ],
        adapters: [
            {module: 'server/application/chat-production-adapter.js', export: 'createChatContextReader', role: 'accepts an injected legacy reader during transition'},
            {module: 'server/runtime/runtime.js', export: 'createDefaultChatProductionPorts', role: 'modular production context composition'}
        ],
        blockers: [
            {code: 'prompt-layer-parity-incomplete', detail: 'The modular reader supplies normalized life-world data, but identity, memory, relationship, presence, capability, budget, and prompt serialization are not yet the complete legacy prompt contract.'},
            {code: 'legacy-callers-remain', detail: 'Deferred chat, proactive evaluation, lifecycle reconciliation, and debug paths still call the server.js contextFor directly.'}
        ],
        deletionChecks: [
            'Route every prompt producer through ContextFragment, budget, and serializer ports.',
            'Replay chat, media, proactive, deferred, and debug context against the same normalized fixture.',
            'Remove direct contextFor calls from server.js and test hooks.'
        ]
    },
    {
        key: 'streamPersonaChat',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['streamPersonaChat', 'createChatTurnIntegration'],
            location: 'server.js:3820-4129'
        },
        targetModules: [
            {module: 'server/application/chat-service.js', exports: 'createCompanionChatService', role: 'chat use-case composition'},
            {module: 'server/application/chat-turn-flow.js', exports: 'createChatTurnFlow', role: 'typed chat flow'},
            {module: 'server/http/chat-turn-sse-adapter.js', exports: 'createChatTurnSseAdapter', role: 'SSE transport boundary'}
        ],
        adapters: [
            {module: 'server/application/chat-production-adapter.js', export: 'createChatProductionPorts', role: 'production context/LLM/conversation ports'},
            {module: 'server/application/chat-service.js', export: 'createCompanionChatService', role: 'replacement for the route-level stream function'}
        ],
        blockers: [
            {code: 'legacy-policy-before-flow', detail: 'The legacy route owns user-message insertion, relationship enqueue, sleep/deferred batching, trusted-time replies, and persona timestamp updates before the modular chat flow.'},
            {code: 'legacy-capability-commit-path', detail: 'The legacy integration still plans/applies capability effects and schedule extraction with direct SQLite transaction ownership.'},
            {code: 'route-cutover-pending', detail: 'server.js still registers its chat handler and the modular runtime has not replaced that production entrypoint.'}
        ],
        deletionChecks: [
            'Move deferred/sleep/trusted-time policy into application flows or explicit policy ports.',
            'Use one capability dispatcher and one commit boundary for chat effects.',
            'Run SSE token/done/error and disconnect replay before removing streamPersonaChat.'
        ]
    },
    {
        key: 'mediaProvider',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['mediaProviders', 'providerFor', 'submitMediaJob', 'pollMedia'],
            location: 'server.js:119-136, 4474-4779, 5434-5620'
        },
        targetModules: [
            {module: 'server/infrastructure/provider-ports.js', exports: 'createProviderRegistry', role: 'provider identity/capability registry'},
            {module: 'server/infrastructure/production-media-providers.js', exports: 'createProductionProviderRegistry', role: 'ComfyUI/h3 production adapters'},
            {module: 'server/application/media-job-service.js', exports: 'createMediaJobService', role: 'provider-independent submit/poll lifecycle'}
        ],
        adapters: [
            {module: 'server.js', export: 'mediaProviders', role: 'legacy Map-shaped facade over provider registry'},
            {module: 'server/application/media-job-composition.js', export: 'createMediaJobApplication', role: 'modular provider/job composition'}
        ],
        blockers: [
            {code: 'duplicate-provider-registration', detail: 'server.js still creates and registers media adapters directly while the modular runtime can create a production provider registry.'},
            {code: 'legacy-job-boundary', detail: 'The old submit/poll helpers still own provider calls, progress, asset persistence, and lease settlement instead of delegating to media-job-service.'}
        ],
        deletionChecks: [
            'Use createProductionProviderRegistry for the production composition root.',
            'Register media submit and poll handlers only through createMediaJobService and the generic dispatcher.',
            'Verify provider failure, progress, asset settlement, and stale lease behavior on a temporary database.'
        ]
    },
    {
        key: 'jobHandlers',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['legacyWorkerRuntime', 'runProactiveMessageJob', 'runPendingEventJob', 'runActivityDecisionJob', 'runDeferredChatReplyJob', 'submitMediaJob', 'pollMedia'],
            location: 'server.js:4870-5620'
        },
        targetModules: [
            {module: 'server/runtime/job-dispatcher.js', exports: 'createJobDispatcher', role: 'lease/retry/settlement owner'},
            {module: 'server/application/media-job-service.js', exports: 'createMediaJobService', role: 'media registrations'},
            {module: 'server/application/proactive-job-service.js', exports: 'createProactiveJobService', role: 'proactive registrations'},
            {module: 'server/application/proactive/worker-flows.js', exports: 'createProactiveMessageFlow, createPendingEventWorkerFlow, createActivityDecisionFlow, createDeferredChatReplyFlow', role: 'feature flows'}
        ],
        adapters: [
            {module: 'server/runtime/runtime.js', export: 'resolveJobHandlers', role: 'registers media/proactive services with one dispatcher'},
            {module: 'server/application/proactive-job-service.js', export: 'createProactiveJobService', role: 'fail-closed registration seam when a flow is absent'}
        ],
        blockers: [
            {code: 'legacy-worker-still-active', detail: 'server.js supplies legacyWorkerRuntime to createRuntime and still dispatches job types through direct branches.'},
            {code: 'old-handler-exports', detail: 'The legacy test-hook surface still exports feature-specific worker functions, so deletion cannot be verified by production import scan alone.'}
        ],
        deletionChecks: [
            'Start the modular runtime with media and proactive services and assert the registration set is complete.',
            'Remove legacyWorkerRuntime injection and route every job through createJobDispatcher.',
            'Verify expired leases, retries, restart recovery, and terminal failures with temporary DATA_DIR.'
        ]
    },
    {
        key: 'debugRoutes',
        status: 'partial',
        legacy: {
            file: 'server.js',
            symbols: ['getDebugContext', 'getLifecycle', 'simulatePersona', 'debugMedia', 'debugContextFor'],
            location: 'server.js:5833-5864, 3060-3140'
        },
        targetModules: [
            {module: 'server/http/route-registry.js', exports: 'registerCompanionRoutes', role: 'explicit debug route gate'},
            {module: 'server/application/debug-service.js', exports: 'createDebugService', role: 'debug use cases and redaction'},
            {module: 'server/application/companion-route-handlers.js', exports: 'createCompanionRouteHandlers', role: 'HTTP validation and DTO mapping'}
        ],
        adapters: [
            {module: 'server/http/route-registry.js', export: 'registerCompanionRoutes', role: 'debugInspectorEnabled === true registration policy'},
            {module: 'server/application/debug-service.js', export: 'createDebugService', role: 'persona-scoped diagnostic adapter'}
        ],
        blockers: [
            {code: 'debug-dto-parity-incomplete', detail: 'The modular debug service currently exposes normalized context/lifecycle data, while the legacy inspector includes bounded media prompt/progress, pending, timeline, decision, and deferred-batch projections.'},
            {code: 'legacy-debug-handler-sql', detail: 'server.js debug handlers still query SQLite and invoke createEvent/createChatMediaRequest directly.'}
        ],
        deletionChecks: [
            'Match persona-scoped redacted debug DTOs and bounded media progress fields.',
            'Exercise route absence for debug flag values other than 1 and success with flag 1.',
            'Remove direct debug SQL and test-hook exports after route contract replay.'
        ]
    }
].map(freezeEntry);

export const LEGACY_PRODUCTION_BOUNDARIES = Object.freeze(BOUNDARIES);
export const LEGACY_PRODUCTION_SYMBOLS = REQUIRED_SYMBOLS;

export function getLegacyProductionBoundary(key) {
    return LEGACY_PRODUCTION_BOUNDARIES.find(entry => entry.key === key) ?? null;
}

export function auditLegacyProductionBoundaries() {
    const statusCounts = Object.fromEntries([...STATUS_VALUES].map(status => [status, 0]));
    const blockers = [];
    for (const boundary of LEGACY_PRODUCTION_BOUNDARIES) {
        statusCounts[boundary.status] += 1;
        for (const blocker of boundary.blockers) blockers.push({boundary: boundary.key, ...blocker});
    }
    return Object.freeze({
        version: LEGACY_PRODUCTION_BOUNDARY_VERSION,
        requiredSymbols: LEGACY_PRODUCTION_SYMBOLS,
        statusCounts: Object.freeze(statusCounts),
        readyForLegacyDeletion: statusCounts.partial === 0 && statusCounts.blocked === 0,
        blockers: Object.freeze(blockers.map(value => Object.freeze(value))),
        boundaries: LEGACY_PRODUCTION_BOUNDARIES
    });
}

export default LEGACY_PRODUCTION_BOUNDARIES;
