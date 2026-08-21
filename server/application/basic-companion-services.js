import {createConversationService} from './conversation-service.js';
import {createIdentitySettingsService} from './identity-settings-service.js';

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
export function createBasicCompanionServices({repositories, settings, providers, clock = () => new Date().toISOString(), debugInspector = false, identitySettingsService} = {}) {
    const personas = repository(repositories, ['persona', 'personas'], 'persona repository');
    const groups = repository(repositories, ['group', 'groups'], 'group repository');
    const conversation = repository(repositories, ['conversation', 'conversationRepository'], 'conversation repository');
    const activity = repository(repositories, ['activity', 'activityRepository'], 'activity repository');
    const conversationService = createConversationService({repository: conversation, clock});
    const settingsPort = settings ?? repositories?.settings;
    if (!isRecord(settingsPort) || typeof settingsPort.read !== 'function') throw new TypeError('Basic companion services require settings.read()');
    const identity = identitySettingsService ?? createIdentitySettingsService({
        repositories: {persona: personas, group: groups, activity, settings: settingsPort},
        providers,
        debugInspector
    });
    const useIdentitySettings = identitySettingsService !== undefined;

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
            list() { return providers?.summaries?.({detailed: true}) ?? []; }
        },
        conversations: {
            list(command) { return pageDto(conversationService.list(conversationCommand(command))); },
            appendMessage(command) { return conversationService.appendMessage(command); }
        },
        activities: {
            list(command) {
                return pageDto(activity.listActivities(command ?? {}));
            }
        }
    };
    return Object.freeze(service);
}

export default createBasicCompanionServices;
