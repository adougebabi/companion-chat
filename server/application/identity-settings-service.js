const SECRET_KEY_PATTERN = /(?:api[-_]?key|authorization|cookie|credential|password|secret|token)/i;
const KNOWN_PRIVATE_SETTINGS = new Set([
    'lmStudioApiKey',
    'h3Executable',
    'h3ModelDir',
    'h3OutputDir',
    'h3AllowedRoot'
]);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertRecord(value, field) {
    if (!isRecord(value)) throw new TypeError(`${field} must be an object`);
    return value;
}

function firstValue(row, names, fallback) {
    for (const name of names) {
        if (row?.[name] !== undefined && row?.[name] !== null) return row[name];
    }
    return fallback;
}

function textValue(value, fallback = '') {
    return typeof value === 'string' ? value : fallback;
}

function countValue(value, fallback = 0) {
    const count = Number(value);
    return Number.isFinite(count) && count >= 0 ? count : fallback;
}

function booleanValue(value) {
    return value === true || value === 1 || value === '1' || value === 'true';
}

function presentFlag(value) {
    if (typeof value === 'string') {
        const normalized = value.trim().toLowerCase();
        return normalized !== '' && normalized !== '0' && normalized !== 'false';
    }
    return Boolean(value);
}

function syncValue(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`${field} must be synchronous`);
    return value;
}

function listRows(repository, names, field) {
    for (const name of names) {
        if (typeof repository?.[name] !== 'function') continue;
        const rows = repository[name]();
        if (rows && typeof rows.then === 'function') throw new TypeError(`${field}.${name}() must be synchronous`);
        if (!Array.isArray(rows)) throw new TypeError(`${field}.${name}() must return an array`);
        return rows;
    }
    throw new TypeError(`${field} must provide ${names.join('() or ')}()`);
}

function replaceEmbeddedSecret(value) {
    return value
        .replace(/(bearer\s+)[^\s,;]+/gi, '$1[redacted]')
        .replace(/((?:api[-_]?key|token|secret|password)\s*[:=]\s*)[^\s,;]+/gi, '$1[redacted]')
        .replace(/(^|[\s=:])\/(?:[^\s,;\/]+\/)+[^\s,;]*/g, '$1[path redacted]');
}

function redactValue(value, {parentKey = ''} = {}) {
    if (Array.isArray(value)) return value.map(item => redactValue(item));
    if (!isRecord(value)) return typeof value === 'string' ? replaceEmbeddedSecret(value) : value;

    const output = {};
    for (const [key, child] of Object.entries(value)) {
        if (KNOWN_PRIVATE_SETTINGS.has(key) || SECRET_KEY_PATTERN.test(key)) continue;
        if (parentKey === 'h3Defaults' && key === 'profile') continue;
        output[key] = redactValue(child, {parentKey: key});
    }
    return output;
}

function safeH3Summary(value) {
    if (value === undefined || value === null) return undefined;
    const summary = redactValue(value);
    if (!isRecord(summary)) return undefined;
    return summary;
}

/**
 * Convert internal settings into the browser-safe settings contract.
 * Derived fields are supplied by application policies or composition code;
 * this function never inspects storage or provider configuration itself.
 */
export function publicSettings(value, {h3ConfigSummary, mediaProviders} = {}) {
    const settings = assertRecord(value, 'Settings');
    const output = redactValue(settings);
    const defaults = isRecord(settings.h3Defaults) ? redactValue(settings.h3Defaults, {parentKey: 'h3Defaults'}) : {};
    output.h3Defaults = defaults;
    if (settings.h3TimeoutMs !== undefined) output.h3TimeoutMs = settings.h3TimeoutMs;
    output.hasH3Configuration = Boolean(
        settings.h3Executable && settings.h3ModelDir && settings.h3OutputDir
    );
    output.hasLmStudioApiKey = Boolean(settings.lmStudioApiKey);

    const summary = syncValue(
        typeof h3ConfigSummary === 'function' ? h3ConfigSummary(settings) : h3ConfigSummary,
        'h3ConfigSummary'
    );
    const safeSummary = safeH3Summary(summary);
    if (safeSummary !== undefined) output.h3ConfigSummary = safeSummary;

    const summaries = syncValue(
        typeof mediaProviders === 'function' ? mediaProviders({detailed: true}) : mediaProviders,
        'mediaProviders'
    );
    if (summaries !== undefined) output.mediaProviders = redactValue(summaries);
    return output;
}

export const redactSettings = publicSettings;

function personaDto(row, groupsById) {
    assertRecord(row, 'Persona row');
    const groupId = firstValue(row, ['group_id', 'groupId'], null);
    const group = groupId ? groupsById.get(groupId) : null;
    const screened = firstValue(row, ['screened_at', 'screenedAt', 'screened'], false);
    return {
        id: row.id,
        initializationMode: firstValue(row, ['initialization_mode', 'initializationMode'], 'llm_defined'),
        name: row.name,
        role: row.role,
        color: row.color,
        groupId: groupId || null,
        groupName: firstValue(row, ['group_name', 'groupName'], group?.name ?? null),
        screened: presentFlag(screened),
        currentSituation: textValue(firstValue(row, ['current_situation', 'currentSituation', 'situation'], '')),
        mood: textValue(firstValue(row, ['mood'], '')),
        unreadCount: countValue(firstValue(row, ['unread_count', 'unreadCount'], 0)),
        updatedAt: firstValue(row, ['updated_at', 'updatedAt'], undefined)
    };
}

function groupDto(row) {
    assertRecord(row, 'Group row');
    return {
        id: row.id,
        name: firstValue(row, ['name', 'group_name', 'groupName'], null),
        isDefault: booleanValue(firstValue(row, ['is_default', 'isDefault'], false)),
        personaCount: countValue(firstValue(row, ['persona_count', 'personaCount'], 0))
    };
}

export function createIdentitySettingsService({
    repositories = {},
    personaRepository,
    groupRepository,
    activityRepository,
    settings,
    settingsPort,
    settingsRepository,
    settingsPolicy,
    providers,
    providerSummaries,
    h3ConfigSummary,
    mediaProviders,
    defaultTimezone,
    debugInspector = false,
    activityUnread
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Identity settings repositories must be an object');
    const personas = personaRepository ?? repositories.persona ?? repositories.personas;
    const groups = groupRepository ?? repositories.group ?? repositories.groups;
    const activity = activityRepository ?? repositories.activity ?? repositories.activities;
    const settingsAdapter = settings ?? settingsPort ?? settingsRepository ?? repositories.settings ?? repositories.settingsRepository;
    if (!isRecord(personas)) throw new TypeError('Identity settings service requires a persona repository');
    if (!isRecord(groups)) throw new TypeError('Identity settings service requires a group repository');
    if (!isRecord(settingsAdapter)) throw new TypeError('Identity settings service requires a settings port');
    if (!['listActive', 'list'].some(name => typeof personas[name] === 'function')) {
        throw new TypeError('Identity settings persona repository must provide listActive() or list()');
    }
    if (!['list', 'listAll'].some(name => typeof groups[name] === 'function')) {
        throw new TypeError('Identity settings group repository must provide list() or listAll()');
    }
    if (typeof settingsAdapter.read !== 'function') throw new TypeError('Identity settings settings port must provide read()');

    const summaryPort = h3ConfigSummary
        ?? (typeof settingsPolicy?.h3ConfigSummary === 'function' ? settingsPolicy.h3ConfigSummary.bind(settingsPolicy) : settingsPolicy?.h3ConfigSummary);
    const providerSummaryPort = mediaProviders
        ?? providerSummaries
        ?? (typeof providers?.summaries === 'function' ? providers.summaries.bind(providers) : undefined);

    function readRawSettings() {
        const value = settingsAdapter.read();
        if (value && typeof value.then === 'function') throw new TypeError('Settings port read() must be synchronous');
        return assertRecord(value, 'Settings port read() result');
    }

    function readSettings() {
        const value = readRawSettings();
        return publicSettings(value, {
            h3ConfigSummary: summaryPort,
            mediaProviders: providerSummaryPort
        });
    }

    function updateSettings(command) {
        const patch = assertRecord(command, 'Settings command');
        const current = readRawSettings();
        const candidate = {...current, ...patch};
        if (current.lmStudioApiKey !== undefined && (patch.lmStudioApiKey === undefined || patch.lmStudioApiKey === '' || patch.lmStudioApiKey === 'configured')) {
            candidate.lmStudioApiKey = current.lmStudioApiKey;
        }

        const validator = settingsPolicy?.validate ?? settingsPolicy?.validateUpdate;
        const next = syncValue(
            typeof validator === 'function' ? validator(candidate, {current, patch}) ?? candidate : candidate,
            'Settings policy'
        );
        assertRecord(next, 'Validated settings');

        const writer = typeof settingsAdapter.update === 'function' ? settingsAdapter.update : settingsAdapter.write;
        if (typeof writer !== 'function') throw new TypeError('Settings port must provide update() or write()');
        const persisted = writer.call(settingsAdapter, next);
        if (persisted && typeof persisted.then === 'function') throw new TypeError('Settings port update() must be synchronous');
        return readSettings();
    }

    function readBootstrap(command = {}) {
        assertRecord(command, 'Bootstrap command');
        const groupRows = listRows(groups, ['list', 'listAll'], 'Group repository');
        const groupsDto = groupRows.map(groupDto);
        const groupsById = new Map(groupsDto.filter(group => group.id !== undefined).map(group => [group.id, group]));
        const personasDto = listRows(personas, ['listActive', 'list'], 'Persona repository')
            .map(row => personaDto(row, groupsById));
        const rawSettings = readRawSettings();
        const configuredUnread = activityUnread ?? activity?.hasUnread ?? activity?.hasUnreadActivities;
        const unreadResult = typeof configuredUnread === 'function'
            ? configuredUnread.call(activity, {readAt: rawSettings.activityReadAt})
            : false;
        if (unreadResult && typeof unreadResult.then === 'function') throw new TypeError('Activity unread port must be synchronous');
        const timezone = syncValue(
            typeof defaultTimezone === 'function' ? defaultTimezone() : defaultTimezone,
            'defaultTimezone'
        );
        return {
            settings: publicSettings(rawSettings, {
                h3ConfigSummary: summaryPort,
                mediaProviders: providerSummaryPort
            }),
            personas: personasDto,
            groups: groupsDto,
            activityUnread: Boolean(unreadResult),
            defaultTimezone: typeof timezone === 'string' && timezone.trim() ? timezone : 'UTC',
            debugInspector: debugInspector === true
        };
    }

    const bootstrap = Object.freeze({read: readBootstrap, get: readBootstrap});
    const settingsService = Object.freeze({read: readSettings, get: readSettings, update: updateSettings, write: updateSettings, save: updateSettings});
    return Object.freeze({
        bootstrap,
        settings: settingsService,
        readBootstrap,
        readSettings,
        updateSettings
    });
}

export const createIdentitySettingsApplicationService = createIdentitySettingsService;
export default createIdentitySettingsService;
