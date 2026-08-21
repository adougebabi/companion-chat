import {createCapabilityDispatcher} from './capability-dispatcher.js';
import {createCompanionRouteHandlers} from './companion-route-handlers.js';
import {createMediaFlow} from './media-flow.js';
import {createPendingEventFlow} from './pending-event-flow.js';
import {createSceneEventFlow} from './scene-event-flow.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function optionalFactory(factory, options) {
    if (factory === undefined) return null;
    return factory(options);
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
    const routeHandlers = options.routeHandlers ?? createCompanionRouteHandlers({
        repositories,
        services: options.services,
        policies: options.policies,
        adapters: options.adapters
    });
    return Object.freeze({
        repositories,
        services: options.services ?? Object.freeze({}),
        policies: options.policies ?? Object.freeze({}),
        adapters: options.adapters ?? Object.freeze({}),
        pendingEventFlow,
        sceneEventFlow,
        mediaFlow,
        capabilityDispatcher,
        routeHandlers
    });
}

export const createProductionCompanionApplication = createCompanionApplication;
export default createCompanionApplication;
