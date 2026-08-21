function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Settings repository requires an open database');
    }
    return database;
}

function resolveDefaults(defaults, defaultsFactory) {
    const source = defaultsFactory ?? defaults;
    if (typeof source !== 'function') {
        throw new TypeError('Settings repository requires a defaults factory');
    }
    return source;
}

function resolveClock(clock, now) {
    const source = clock ?? now;
    const callback = typeof source === 'function'
        ? source
        : isRecord(source) && typeof source.now === 'function'
            ? source.now.bind(source)
            : null;
    if (!callback) throw new TypeError('Settings repository requires a clock');
    return () => {
        const value = callback();
        if (value instanceof Date) return value.toISOString();
        if (typeof value === 'string' && value.trim()) return value;
        throw new TypeError('Settings repository clock must return a timestamp');
    };
}

function decodePayload(value) {
    if (typeof value !== 'string' || value.length === 0) return {};
    try {
        return JSON.parse(value);
    } catch {
        return {};
    }
}

/**
 * Create a table-scoped adapter for the single companion settings row.
 *
 * Defaults, validation, provider selection, and browser-safe DTO shaping stay
 * with the caller. This adapter only decodes/merges JSON and performs the
 * parameterized storage update against an already-open database.
 */
export function createSettingsRepository({database, defaults, defaultsFactory, clock, now} = {}) {
    const openDatabase = assertOpenDatabase(database);
    const createDefaults = resolveDefaults(defaults, defaultsFactory);
    const timestamp = resolveClock(clock, now);

    function getRawPayload() {
        const row = openDatabase.prepare(
            'SELECT payload_json FROM companion_settings WHERE id = 1'
        ).get();
        return decodePayload(row?.payload_json);
    }

    function read() {
        const defaultValue = createDefaults();
        const defaultsValue = isRecord(defaultValue) ? defaultValue : {};
        const rawValue = getRawPayload();
        return {...defaultsValue, ...(isRecord(rawValue) ? rawValue : {})};
    }

    function write(next) {
        if (!isRecord(next)) throw new TypeError('Settings repository value must be an object');
        openDatabase.prepare(
            'UPDATE companion_settings SET payload_json = ?, updated_at = ? WHERE id = 1'
        ).run(JSON.stringify(next), timestamp());
        return next;
    }

    return Object.freeze({
        read,
        write,
        getRawPayload,
        update: write
    });
}

export const createCompanionSettingsRepository = createSettingsRepository;
export default createSettingsRepository;
