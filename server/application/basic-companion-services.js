function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function repository(repositories, names, field) {
    for (const name of names) {
        if (isRecord(repositories?.[name])) return repositories[name];
    }
    throw new TypeError(`Basic companion services require ${field}`);
}

function messageDto(row) {
    if (!row) return null;
    const decode = value => {
        if (value && typeof value === 'object') return value;
        try { return value ? JSON.parse(value) : []; } catch { return []; }
    };
    return {
        id: row.id,
        role: row.role,
        text: row.text,
        attachments: decode(row.attachments ?? row.attachments_json),
        generation: row.generation ?? (row.generation_json ? decode(row.generation_json) : undefined),
        jobs: decode(row.jobs ?? row.jobs_json),
        createdAt: row.createdAt ?? row.created_at,
        readAt: row.readAt ?? row.read_at ?? undefined
    };
}

function groupDto(row) {
    return {id: row.id, name: row.name, isDefault: Boolean(row.is_default), personaCount: Number(row.persona_count || 0)};
}

function personaDto(row) {
    return {
        id: row.id,
        name: row.name,
        role: row.role,
        color: row.color,
        groupId: row.group_id || null,
        screened: Boolean(row.screened_at),
        updatedAt: row.updated_at
    };
}

/**
 * Small repository-backed services used by the modular runtime smoke path.
 * Feature-specific policy remains in application flows; unsupported routes
 * continue to report bounded 501 through the route-handler composition.
 */
export function createBasicCompanionServices({repositories, settings, providers, clock = () => new Date().toISOString(), debugInspector = false} = {}) {
    const personas = repository(repositories, ['persona', 'personas'], 'persona repository');
    const groups = repository(repositories, ['group', 'groups'], 'group repository');
    const conversation = repository(repositories, ['conversation', 'conversationRepository'], 'conversation repository');
    const activity = repository(repositories, ['activity', 'activityRepository'], 'activity repository');
    const conversationService = createConversationService({repository: conversation, clock});
    const settingsPort = settings ?? repositories?.settings;
    if (!isRecord(settingsPort) || typeof settingsPort.read !== 'function') throw new TypeError('Basic companion services require settings.read()');

    const service = {
        bootstrap: {
            read() {
                return {
                    settings: settingsPort.read(),
                    personas: personas.listActive().map(personaDto),
                    groups: groups.list().map(groupDto),
                    activityUnread: false,
                    defaultTimezone: 'UTC',
                    debugInspector: debugInspector === true
                };
            }
        },
        settings: {
            read() { return settingsPort.read(); },
            update(command) {
                if (!isRecord(command)) throw new TypeError('Settings command must be an object');
                if (typeof settingsPort.write !== 'function') throw new TypeError('Settings service is read-only');
                settingsPort.write(command);
                return settingsPort.read();
            }
        },
        models: {
            list() { return providers?.summaries?.({detailed: true}) ?? []; }
        },
        conversations: {
            list(command) { return conversationService.list(command); },
            appendMessage(command) { return conversationService.appendMessage(command); }
        },
        activities: {
            list(command) {
                const result = activity.listActivities(command);
                if (Array.isArray(result)) return {items: result, nextCursor: null};
                return result;
            }
        }
    };
    return Object.freeze(service);
}

export default createBasicCompanionServices;
import {createConversationService} from './conversation-service.js';
