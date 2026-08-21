import {createConversationService} from './conversation-service.js';
import {createIdentitySettingsService} from './identity-settings-service.js';
import {createActivityService} from './activity-service.js';
import {createCompanionRouteService} from './companion-route-service.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function repository(repositories, names, field) {
    for (const name of names) {
        if (isRecord(repositories?.[name])) return repositories[name];
    }
    throw new TypeError(`Basic companion services require ${field}`);
}

function conversationCommand(command) {
    if (typeof command === 'string') return {personaId: command};
    if (!isRecord(command)) return {};

    const normalized = {...command};
    if (normalized.personaId === undefined && normalized.persona_id !== undefined) {
        normalized.personaId = normalized.persona_id;
    }
    // Older callers used the response name as the request cursor. The
    // conversation application service still owns cursor decoding/validation.
    if (normalized.cursor === undefined && normalized.nextCursor !== undefined) {
        normalized.cursor = normalized.nextCursor;
    }
    return normalized;
}

function pageDto(result) {
    if (Array.isArray(result)) return {items: result, nextCursor: null};
    if (!isRecord(result)) return {items: [], nextCursor: null};

    const nextCursor = result.nextCursor ?? result.cursor ?? null;
    const {cursor: _legacyCursor, ...rest} = result;
    return {
        ...rest,
        items: Array.isArray(result.items) ? result.items : [],
        nextCursor
    };
}

/**
 * Small repository-backed services used by the modular runtime smoke path.
 * Feature-specific policy remains in application flows; unsupported routes
 * continue to report bounded 501 through the route-handler composition.
 */
export function createBasicCompanionServices({
    repositories,
    settings,
    providers,
    clock = () => new Date().toISOString(),
    idGenerator,
    debugInspector = false,
    identitySettingsService,
    activityService,
    routeService,
    adapters,
    personaLifecycle,
    personaLifecycleService,
    interviewService,
    foundationService,
    scheduleService,
    memoryService,
    debugService,
    mediaDebugService,
    mediaService,
    lifeStateService,
    lifeEventFlow,
    timelineFlow,
    relationshipFlow
} = {}) {
    const personas = repository(repositories, ['persona', 'personas', 'personaRepository'], 'persona repository');
    const groups = repository(repositories, ['group', 'groups', 'groupRepository'], 'group repository');
    const conversation = repository(repositories, ['conversation', 'conversationRepository'], 'conversation repository');
    const activity = repository(repositories, ['activity', 'activityRepository'], 'activity repository');
    const conversationService = createConversationService({repository: conversation, clock, idGenerator});
    const settingsPort = settings ?? repositories?.settings;
    if (!isRecord(settingsPort) || typeof settingsPort.read !== 'function') throw new TypeError('Basic companion services require settings.read()');
    const identity = identitySettingsService ?? createIdentitySettingsService({
        repositories: {persona: personas, group: groups, activity, settings: settingsPort},
        providers,
        debugInspector
    });
    const useIdentitySettings = identitySettingsService !== undefined;
    const activityApplication = activityService ?? null;
    const mediaAssetCandidate = repositories.mediaAsset ?? repositories.mediaAssetRepository
        ?? repositories.asset ?? repositories.assets;
    const mediaAsset = isRecord(mediaAssetCandidate) && typeof mediaAssetCandidate.find === 'function'
        ? mediaAssetCandidate : null;
    const mediaApplication = mediaDebugService ?? mediaService ?? (mediaAsset ? {
        get: command => mediaAsset.find(command),
        read: command => mediaAsset.find(command),
        getMedia: command => mediaAsset.find(command)
    } : null);
    const debugApplication = debugInspector === true
        ? mediaDebugService
            ? Object.freeze({
                ...(debugService ?? {}),
                ...mediaDebugService,
                debug: Object.freeze({...((debugService ?? {}).debug ?? {}), ...(mediaDebugService.debug ?? {})})
            })
            : debugService
        : null;
    const routeApplication = routeService ?? createCompanionRouteService({
        repositories,
        services: {identity: identity},
        adapters: {
            ...(isRecord(adapters) ? adapters : {}),
            ...(personaLifecycle === undefined ? {} : {personaLifecycle})
        },
        identitySettingsService: identity,
        conversationService,
        clock,
        idGenerator,
        personaLifecycleService: personaLifecycleService ?? personaLifecycle,
        interviewService,
        foundationService,
        scheduleService,
        memoryService
    });

    const service = {
        bootstrap: {
            read() { return identity.bootstrap.read(); }
        },
        settings: {
            read() { return useIdentitySettings ? identity.settings.read() : settingsPort.read(); },
            update(command) {
                if (!useIdentitySettings) {
                    if (!isRecord(command)) throw new TypeError('Settings command must be an object');
                    if (typeof settingsPort.write !== 'function') throw new TypeError('Settings service is read-only');
                    settingsPort.write(command);
                    return settingsPort.read();
                }
                return identity.settings.update(command);
            }
        },
        models: {
            list(command = {}) {
                const summaries = providers?.summaries?.({detailed: true}) ?? [];
                const providerId = command.provider ?? command.providerId ?? 'mtplx';
                const provider = providers?.find?.(providerId, {portType: 'llm-streaming'}) ?? null;
                if (provider?.models) {
                    return Promise.resolve(provider.models(command)).then(value => ({provider: providerId, models: value?.data ?? value?.models ?? value ?? [], providers: summaries}));
                }
                return summaries;
            }
        },
        persona: routeApplication.persona,
        personas: routeApplication.personas,
        identity: routeApplication.identity,
        group: routeApplication.group,
        groups: routeApplication.groups,
        interview: routeApplication.interview,
        interviews: routeApplication.interviews,
        foundation: routeApplication.foundation,
        schedule: routeApplication.schedule,
        schedules: routeApplication.schedules,
        memory: routeApplication.memory,
        memories: routeApplication.memories,
        lifecycle: routeApplication.identity,
        ...(mediaApplication ? {media: mediaApplication, assets: mediaApplication} : {}),
        ...(debugApplication ? {debug: debugApplication} : {}),
        ...(lifeStateService ? {life: lifeStateService, lifeState: lifeStateService} : {}),
        ...(lifeEventFlow ? {lifeEvent: lifeEventFlow, events: lifeEventFlow} : {}),
        ...(timelineFlow ? {timeline: timelineFlow, lifeTimeline: timelineFlow} : {}),
        ...(repositories.relationship ? {
            relationship: {
                rollbackEvolution(command) {
                    const result = relationshipFlow
                        ? relationshipFlow.rollback(command)
                        : repositories.relationship.rollbackEvolution(command);
                    if (!result) throw Object.assign(new Error('关系演化不存在或已回滚'), {status: 404});
                    return result;
                },
                rollback: command => {
                    const result = relationshipFlow
                        ? relationshipFlow.rollback(command)
                        : repositories.relationship.rollbackEvolution(command);
                    if (!result) throw Object.assign(new Error('关系演化不存在或已回滚'), {status: 404});
                    return result;
                }
            }
        } : {}),
        conversations: routeApplication.conversations,
        conversation: routeApplication.conversation,
        activities: {
            list(command) {
                if (activityApplication) return activityApplication.activities?.list?.(command ?? {}) ?? activityApplication.list(command ?? {});
                return pageDto(activity.listActivities(command ?? {}));
            },
            comment(command) {
                if (!activityApplication) throw new TypeError('Activity service is not configured');
                return activityApplication.activities?.comment?.(command) ?? activityApplication.comment(command);
            },
            like(command) {
                if (!activityApplication) throw new TypeError('Activity service is not configured');
                return activityApplication.activities?.like?.(command) ?? activityApplication.like(command);
            },
            hide(command) {
                if (!activityApplication) throw new TypeError('Activity service is not configured');
                return activityApplication.activities?.hide?.(command) ?? activityApplication.hide(command);
            },
            markRead(command) {
                if (!activityApplication) throw new TypeError('Activity service is not configured');
                return activityApplication.activities?.markRead?.(command) ?? activityApplication.markRead(command);
            }
        }
    };
    return Object.freeze({...service, routeService: routeApplication});
}

export default createBasicCompanionServices;
