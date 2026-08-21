import {createMediaJobService} from './media-job-service.js';
import {createMediaObservability} from './media-observability.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function mergeOptions(primary, secondary) {
    return {
        ...(isRecord(primary) ? primary : {}),
        ...(isRecord(secondary) ? secondary : {})
    };
}

function hasMethod(value, names) {
    return names.some(name => typeof value?.[name] === 'function');
}

function isObservability(value) {
    return isRecord(value) && hasMethod(value, ['createReporter', 'recordProgress', 'settle']);
}

function isMediaJobService(value) {
    return isRecord(value) && (
        typeof value.register === 'function'
        || typeof value.submit === 'function'
        || isRecord(value.handlers)
        || isRecord(value.handlerMap)
    );
}

function observabilityPorts(options) {
    const nested = mergeOptions(options.observabilityOptions, options.observabilityPorts);
    const configured = options.observability;
    if (isObservability(configured)) return configured;

    const direct = {
        ...options,
        ...(isRecord(configured) ? configured : {}),
        ...(isRecord(options.mediaObservability) ? options.mediaObservability : {}),
        ...nested
    };
    const hasPorts = [
        'progressParser',
        'progressWriter',
        'leaseGuard',
        'settleJob',
        'debugProjector',
        'runProcess',
        'reporterFactory'
    ].some(name => direct[name] !== undefined);
    if (!hasPorts) return null;

    // Reporter creation is an application concern. The default keeps the
    // generic reporter returned by createMediaObservability while callers can
    // still inject a richer adapter when a provider needs one.
    return createMediaObservability({
        ...direct,
        reporterFactory: direct.reporterFactory ?? (({report}) => report)
    });
}

/**
 * Build the media observability application from explicit ports.
 *
 * This is intentionally a small fixture-friendly composition helper: it does
 * not open SQLite, create a provider, or recreate a debug projection. Legacy
 * integration tests can pass their existing progress/lease/settlement/debug
 * functions here while application tests use in-memory ports.
 */
export function createMediaObservabilityApplication(options = {}) {
    if (!isRecord(options)) throw new TypeError('Media observability application options must be an object');
    const resolved = observabilityPorts(options);
    if (!resolved) throw new TypeError('Media observability application requires observability ports');
    return Object.freeze({observability: resolved, ...resolved});
}

function repositoriesFor(options) {
    const repositories = isRecord(options.repositories)
        ? {...options.repositories}
        : {};
    const directJob = options.jobRepository ?? options.job;
    const mediaFlow = options.mediaFlow ?? repositories.mediaFlow ?? repositories.media ?? repositories.mediaRepository;
    if (directJob !== undefined && repositories.jobRepository === undefined && repositories.job === undefined) {
        repositories.jobRepository = directJob;
    }
    if (mediaFlow !== undefined && repositories.mediaFlow === undefined) repositories.mediaFlow = mediaFlow;
    return repositories;
}

/**
 * Compose a media job service from runtime-owned repositories, providers, and
 * application ports. An already-created service may be supplied by callers
 * that need complete control over registration or lifecycle.
 */
export function createMediaJobApplication(options = {}) {
    if (!isRecord(options)) throw new TypeError('Media job application options must be an object');
    if (isMediaJobService(options.service)) return options.service;

    const configuredService = isRecord(options.mediaJobService) && !isMediaJobService(options.mediaJobService)
        ? options.mediaJobService
        : null;
    const nested = mergeOptions(
        configuredService,
        mergeOptions(options.mediaJobServiceOptions, options.mediaJobOptions)
    );
    const configured = {...nested, ...options};
    const observability = options.observability ?? options.mediaObservability ?? observabilityPorts(configured);
    const providers = options.providers ?? options.providerRegistry ?? options.providerAdapters ?? options.mediaProviderAdapters;
    if (providers === undefined) throw new TypeError('Media job application requires providers or providerAdapters');

    return createMediaJobService({
        ...configured,
        providers,
        observability,
        repositories: repositoriesFor(configured),
        promptMaster: options.promptMaster ?? options.mediaPromptMaster ?? nested.promptMaster,
        acceptance: options.acceptance ?? options.mediaAcceptance ?? nested.acceptance,
        clock: options.clock ?? nested.clock
    });
}

export const createMediaJobComposition = createMediaJobApplication;
export const createMediaJobApplicationService = createMediaJobApplication;

export default createMediaJobApplication;
