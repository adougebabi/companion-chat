import {createCompanionRouteHandlers} from './companion-route-handlers.js';
import {createMediaFlow} from './media-flow.js';
import {createPendingEventFlow} from './pending-event-flow.js';
import {createSceneEventFlow} from './scene-event-flow.js';
import {createAppearanceEventFlow} from './appearance-event-flow.js';
import {createLifeEventFlow} from './life-event-flow.js';
import {createTimelineFlow} from './timeline-flow.js';
import {createRelationshipFlow} from './relationship-flow.js';
import {createAppraisalFlow} from './appraisal-flow.js';
import {createMemoryConsolidationFlow} from './memory-consolidation-flow.js';
import {createAgencyIntentionFlow} from './agency-intention-flow.js';
import {createSelfModelFlow} from './self-model-flow.js';
import {createCompanionChatService} from './chat-service.js';
import {createCapabilityHandoffAdapter, createFlowCapabilityRegistry} from './capability-handoff-adapter.js';
import {createFlowEffectAdapter, registerFlowAdapter} from './flow-effect-adapter.js';
import {createFlowRegistry} from './flow-registry.js';

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
    'userMessageWriter',
    'chatPolicy',
    'deferredChatPolicy',
    'enableContinuation',
    'presentationMapper',
    'chatPresentationMapper',
    'commitBoundary',
    'commit',
    'conversationCommitAdapter',
    'assistantCommitAdapter',
    'chatCommitBoundary',
    'affectFlow',
    'appraisalFlow',
    'memoryConsolidationFlow',
    'selfModelFlow',
    'agencyIntentionFlow',
    'structuredTurnControl',
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
    'userMessageWriter',
    'chatPolicy',
    'deferredChatPolicy',
    'enableContinuation',
    'presentationMapper',
    'chatPresentationMapper',
    'conversationCommitAdapter',
    'assistantCommitAdapter',
    'chatCommitBoundary',
    'sendSse',
    'end'
]);

function chatOptionsFrom(options) {
    const production = isRecord(options.chatProductionPorts)
        ? options.chatProductionPorts
        : isRecord(options.productionChatPorts) ? options.productionChatPorts : null;
    const nested = isRecord(options.chatOptions) || isRecord(options.chatPorts)
        ? {
            ...(production ?? {}),
            ...(isRecord(options.chatPorts) ? options.chatPorts : {}),
            ...(isRecord(options.chatOptions) ? options.chatOptions : {})
        }
        : production;
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
    if (resolved.presentationMapper === undefined) {
        resolved.presentationMapper = resolved.chatPresentationMapper;
    }
    if (resolved.commitBoundary === undefined) resolved.commitBoundary = resolved.commit;
    if (resolved.commitBoundary === undefined) {
        resolved.commitBoundary = resolved.chatCommitBoundary
            ?? resolved.conversationCommitAdapter
            ?? resolved.assistantCommitAdapter;
    }
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
    const flowRegistry = options.flowRegistry ?? createFlowRegistry();
    const jobRepository = repositories.job ?? repositories.jobRepository;
    const effectAdapter = options.effectAdapter
        ?? (jobRepository?.enqueue ? createFlowEffectAdapter({jobRepository, clock, idGenerator}) : null);
    const flowOptions = {repositories, clock, idGenerator, effectAdapter};
    const lifeEventFlow = options.lifeEventFlow ?? (repositories.lifeEvent || repositories.life
        ? optionalFactory(createLifeEventFlow, {...flowOptions, transaction: options.transaction})
        : null);
    const pendingReady = repositories.pending || repositories.pendingEvent
        ? Boolean(repositories.job || repositories.jobRepository)
        : false;
    const pendingEventFlow = options.pendingEventFlow ?? (pendingReady
        ? optionalFactory(createPendingEventFlow, {
            ...flowOptions,
            normalizeCall: options.normalizePendingEventCall,
            transaction: options.transaction,
            effectAdapter
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
    const appearanceEventFlow = options.appearanceEventFlow ?? (options.stateRepository || repositories.state
        ? optionalFactory(createAppearanceEventFlow, {...flowOptions, transaction: options.transaction})
        : null);
    const timelineRepositoryReady = repositories.eventDecisionRepository
        || repositories.timelineDecisionRepository
        || repositories.decisionRepository
        || repositories.eventDecision
        || repositories.decisions;
    const timelineFlow = options.timelineFlow ?? (timelineRepositoryReady && lifeEventFlow
        ? optionalFactory(createTimelineFlow, {...flowOptions, lifeEventFlow, transaction: options.transaction})
        : null);
    const relationshipFlow = options.relationshipFlow ?? (repositories.relationship || repositories.relationshipRepository
        ? optionalFactory(createRelationshipFlow, {...flowOptions, transaction: options.transaction})
        : null);
    const appraisalFlow = options.appraisalFlow ?? (repositories.interactionFact && repositories.appraisal
        ? optionalFactory(createAppraisalFlow, {
            ...flowOptions,
            affectFlow: options.affectFlow,
            transaction: options.transaction
        })
        : null);
    const memoryConsolidationFlow = options.memoryConsolidationFlow
        ?? (repositories.memoryConsolidation || repositories.memoryConsolidationRepository
            ? optionalFactory(createMemoryConsolidationFlow, {...flowOptions, transaction: options.transaction})
            : null);
    const selfModelFlow = options.selfModelFlow ?? (repositories.selfModel || repositories.selfModelRepository || repositories.selfModelClaims
        ? optionalFactory(createSelfModelFlow, {...flowOptions, transaction: options.transaction})
        : null);
    const agencyIntentionFlow = options.agencyIntentionFlow ?? (repositories.agencyIntention || repositories.agencyIntentionRepository
        ? optionalFactory(createAgencyIntentionFlow, {...flowOptions, transaction: options.transaction})
        : null);
    const mediaFlow = options.mediaFlow ?? (options.normalizeMediaCapabilityCall || options.mediaNormalizer
        ? optionalFactory(createMediaFlow, {
            ...flowOptions,
            normalizeCall: options.normalizeMediaCapabilityCall ?? options.mediaNormalizer,
            mediaConceptEnvelopeFor: options.mediaConceptEnvelopeFor,
            mediaMessagePlaceholder: options.mediaMessagePlaceholder,
            messageShape: options.messageShape,
            providerFor: options.providerFor,
            transaction: options.transaction,
            effectAdapter
        })
        : null);
    const capabilityRegistry = options.capabilityRegistry ?? options.registry
        ?? createFlowCapabilityRegistry({
            pendingEventFlow,
            sceneEventFlow,
            appearanceEventFlow,
            mediaFlow,
            memoryEventFlow: options.memoryEventFlow ?? options.memoryFlow
        });
    const capabilityDispatcher = options.capabilityDispatcher
        ?? (Object.keys(capabilityRegistry).length
            ? createCapabilityHandoffAdapter({registry: capabilityRegistry})
            : null);
    const chatService = options.chatService ?? (() => {
        const chatOptions = chatOptionsFrom(options);
        if (!chatOptions) return null;
        if (chatOptions.capabilityDispatcher === undefined && capabilityDispatcher) chatOptions.capabilityDispatcher = capabilityDispatcher;
        if (chatOptions.registry === undefined) chatOptions.registry = flowRegistry;
        return createCompanionChatService(chatOptions);
    })();
    registerFlowAdapter(flowRegistry, {id: 'life-event', flow: lifeEventFlow, execute: lifeEventFlow?.record?.bind(lifeEventFlow), version: lifeEventFlow?.version ?? 1});
    registerFlowAdapter(flowRegistry, {id: 'timeline', flow: timelineFlow, execute: timelineFlow?.evaluate?.bind(timelineFlow), version: timelineFlow?.version ?? 1});
    registerFlowAdapter(flowRegistry, {id: 'relationship', flow: relationshipFlow, execute: relationshipFlow?.evolve?.bind(relationshipFlow), version: relationshipFlow?.version ?? 1});
    registerFlowAdapter(flowRegistry, {
        id: 'appraisal',
        flow: appraisalFlow,
        execute: appraisalFlow ? command => appraisalFlow.apply(appraisalFlow.plan(command)) : null,
        version: appraisalFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'memory-consolidation',
        flow: memoryConsolidationFlow,
        execute: memoryConsolidationFlow ? command => memoryConsolidationFlow.apply(memoryConsolidationFlow.plan(command)) : null,
        version: memoryConsolidationFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'pending-event-capability',
        flow: pendingEventFlow,
        execute: pendingEventFlow ? command => pendingEventFlow.apply(pendingEventFlow.plan(command)) : null,
        version: pendingEventFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'scene-event',
        flow: sceneEventFlow,
        execute: sceneEventFlow ? command => sceneEventFlow.apply(sceneEventFlow.plan(command)) : null,
        version: sceneEventFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'appearance-event',
        flow: appearanceEventFlow,
        execute: appearanceEventFlow ? command => appearanceEventFlow.apply(appearanceEventFlow.plan(command)) : null,
        version: appearanceEventFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'media',
        flow: mediaFlow,
        execute: mediaFlow ? command => mediaFlow.apply(mediaFlow.plan(command)) : null,
        version: mediaFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'self-model',
        flow: selfModelFlow,
        execute: selfModelFlow ? command => selfModelFlow.apply(selfModelFlow.plan(command)) : null,
        version: selfModelFlow?.version ?? 1
    });
    registerFlowAdapter(flowRegistry, {
        id: 'agency-intention',
        flow: agencyIntentionFlow,
        execute: agencyIntentionFlow ? command => agencyIntentionFlow.apply(agencyIntentionFlow.plan(command)) : null,
        version: agencyIntentionFlow?.version ?? 1
    });
    const services = {
        ...(isRecord(options.services) ? options.services : {}),
        ...(chatService ? {chat: chatService} : {}),
        ...(lifeEventFlow ? {lifeEvent: lifeEventFlow, events: lifeEventFlow} : {}),
        ...(timelineFlow ? {timeline: timelineFlow, lifeTimeline: timelineFlow} : {}),
        ...(relationshipFlow ? {relationship: relationshipFlow, evolution: relationshipFlow} : {}),
        ...(appraisalFlow ? {appraisal: appraisalFlow} : {}),
        ...(memoryConsolidationFlow ? {memoryConsolidation: memoryConsolidationFlow} : {}),
        ...(selfModelFlow ? {selfModel: selfModelFlow} : {}),
        ...(agencyIntentionFlow ? {agencyIntention: agencyIntentionFlow} : {})
    };
    const defaultRouteHandlers = createCompanionRouteHandlers({
        repositories,
        services,
        policies: options.policies,
        adapters: options.adapters,
        debugInspectorEnabled: options.debugInspectorEnabled === true
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
        appearanceEventFlow,
        lifeEventFlow,
        timelineFlow,
        relationshipFlow,
        appraisalFlow,
        memoryConsolidationFlow,
        selfModelFlow,
        agencyIntentionFlow,
        mediaFlow,
        flowRegistry,
        flows: flowRegistry,
        effectAdapter,
        capabilityDispatcher,
        chatService,
        chatRoute,
        routeHandlers
    });
}

export const createProductionCompanionApplication = createCompanionApplication;
export default createCompanionApplication;
