import {createCapabilityDispatcher} from './capability-dispatcher.js';
import {createCompanionRouteHandlers} from './companion-route-handlers.js';
import {createMediaFlow} from './media-flow.js';
import {createPendingEventFlow} from './pending-event-flow.js';
import {createSceneEventFlow} from './scene-event-flow.js';
import {createCompanionChatService} from './chat-service.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function optionalFactory(factory, options) {
    if (factory === undefined) return null;
    return factory(options);
}

const CHAT_PORT_NAMES = Object.freeze([
    'contextReader',
    'llmStreamingPort',
    'llmStreamPort',
    'llm',
    'capabilityDispatcher',
    'conversationRepository',
    'conversation',
    'presentationMapper',
    'commitBoundary',
    'commit',
    'sendSse',
    'end',
    'errorMapper'
]);
const DIRECT_CHAT_MARKERS = Object.freeze([
    'contextReader',
    'llmStreamingPort',
    'llmStreamPort',
    'llm',
    'conversationRepository',
    'conversation',
    'sendSse',
    'end'
]);

function chatOptionsFrom(options) {
    const nested = isRecord(options.chatOptions) || isRecord(options.chatPorts)
        ? {
            ...(isRecord(options.chatPorts) ? options.chatPorts : {}),
            ...(isRecord(options.chatOptions) ? options.chatOptions : {})
        }
        : null;
    const direct = {};
    for (const name of CHAT_PORT_NAMES) {
        if (options[name] !== undefined) direct[name] = options[name];
    }
    const hasDirectPorts = DIRECT_CHAT_MARKERS.some(name => options[name] !== undefined);
    if (!nested && !hasDirectPorts) return null;
    const resolved = {...direct, ...(nested ?? {})};
    if (resolved.llmStreamingPort === undefined) {
        resolved.llmStreamingPort = resolved.llmStreamPort ?? resolved.llm;
    }
    if (resolved.conversationRepository === undefined) {
        resolved.conversationRepository = resolved.conversation;
    }
    if (resolved.commitBoundary === undefined) resolved.commitBoundary = resolved.commit;
    return resolved;
}

/**
 * Assemble application flows and transport handlers from injected ports.
 * Persistence, HTTP binding, providers, and worker lifecycle remain owned by
 * their respective runtime/composition layers.
 */
export function createCompanionApplication(options = {}) {
    if (!isRecord(options)) throw new TypeError('Companion application options must be an object');
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    const clock = options.clock;
    const idGenerator = options.idGenerator ?? options.id;
    const flowOptions = {repositories, clock, idGenerator};
    const pendingReady = repositories.pending || repositories.pendingEvent
        ? Boolean(repositories.job || repositories.jobRepository)
        : false;
    const pendingEventFlow = options.pendingEventFlow ?? (pendingReady
        ? optionalFactory(createPendingEventFlow, {
            ...flowOptions,
            normalizeCall: options.normalizePendingEventCall,
            transaction: options.transaction
        })
        : null);
    const sceneEventFlow = options.sceneEventFlow ?? (options.stateRepository || repositories.state
        ? optionalFactory(createSceneEventFlow, {
            ...flowOptions,
            normalizeCall: options.normalizeSceneEventCall,
            scheduledState: options.scheduledState,
            transaction: options.transaction
        })
        : null);
    const mediaFlow = options.mediaFlow ?? (options.normalizeMediaCapabilityCall || options.mediaNormalizer
        ? optionalFactory(createMediaFlow, {
            ...flowOptions,
            normalizeCall: options.normalizeMediaCapabilityCall ?? options.mediaNormalizer,
            mediaConceptEnvelopeFor: options.mediaConceptEnvelopeFor,
            mediaMessagePlaceholder: options.mediaMessagePlaceholder,
            messageShape: options.messageShape,
            providerFor: options.providerFor,
            transaction: options.transaction
        })
        : null);
    const capabilityDispatcher = options.capabilityDispatcher
        ?? (options.capabilityRegistry || options.registry
            ? createCapabilityDispatcher({
                registry: options.capabilityRegistry ?? options.registry,
                markerCallFactory: options.markerCallFactory
            })
            : null);
    const chatService = options.chatService ?? (() => {
        const chatOptions = chatOptionsFrom(options);
        return chatOptions ? createCompanionChatService(chatOptions) : null;
    })();
    const services = {
        ...(isRecord(options.services) ? options.services : {}),
        ...(chatService ? {chat: chatService} : {})
    };
    const defaultRouteHandlers = createCompanionRouteHandlers({
        repositories,
        services,
        policies: options.policies,
        adapters: options.adapters
    });
    const routeHandlers = options.routeHandlers ?? defaultRouteHandlers;
    // The generated handler owns body validation and SSE response preparation;
    // exposing it separately lets the HTTP composition mount one chat route
    // without relying on generic service method discovery or duplicating it.
    const chatRoute = options.chatRoute ?? (chatService ? routeHandlers.chat ?? defaultRouteHandlers.chat : null);
    return Object.freeze({
        repositories,
        services: Object.freeze(services),
        policies: options.policies ?? Object.freeze({}),
        adapters: options.adapters ?? Object.freeze({}),
        pendingEventFlow,
        sceneEventFlow,
        mediaFlow,
        capabilityDispatcher,
        chatService,
        chatRoute,
        routeHandlers
    });
}

export const createProductionCompanionApplication = createCompanionApplication;
export default createCompanionApplication;
