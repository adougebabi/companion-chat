import {mkdirSync} from 'node:fs';
import {dirname, join} from 'node:path';

const MIGRATION_TABLE = 'companion_schema_migrations';
const DEFAULT_PRAGMAS = Object.freeze({
    journal_mode: 'WAL',
    foreign_keys: 'ON',
    busy_timeout: 5000
});

const PRAGMA_ALIASES = Object.freeze({
    journalMode: 'journal_mode',
    journal_mode: 'journal_mode',
    foreignKeys: 'foreign_keys',
    foreign_keys: 'foreign_keys',
    busyTimeout: 'busy_timeout',
    busy_timeout: 'busy_timeout'
});

const SAFE_SQLITE_ERROR_CODE = /^SQLITE_[A-Z0-9_]{1,48}$/;
const SAFE_PRAGMA_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
const SAFE_PRAGMA_VALUE = /^[A-Za-z0-9_.-]+$/;

/**
 * Errors from the runtime intentionally contain only stable operational
 * context. The original SQLite error is not retained because it can include
 * a database path, SQL text, or configuration values.
 */
export class SqliteRuntimeError extends Error {
    constructor(message, {code = 'SQLITE_RUNTIME_ERROR', phase = 'startup'} = {}) {
        super(message);
        this.name = 'SqliteRuntimeError';
        this.code = code;
        this.phase = phase;
    }
}

function configError(message) {
    return new SqliteRuntimeError(message, {code: 'SQLITE_RUNTIME_CONFIG', phase: 'configuration'});
}

function safeSqliteCode(error) {
    return typeof error?.code === 'string' && SAFE_SQLITE_ERROR_CODE.test(error.code) ? error.code : null;
}

function runtimeFailure(phase, error, {version = null} = {}) {
    const code = safeSqliteCode(error);
    const migration = Number.isSafeInteger(version) ? ` for migration ${version}` : '';
    const detail = code ? ` (${code})` : '';
    return new SqliteRuntimeError(`SQLite runtime ${phase} failed${migration}${detail}`, {
        code: 'SQLITE_RUNTIME_FAILURE',
        phase
    });
}

function assertDatabaseConstructor(Database) {
    if (typeof Database !== 'function') {
        throw configError('SQLite runtime requires an injected Database constructor');
    }
}

function normalizeDataDir(dataDir) {
    if (dataDir === undefined || dataDir === null) return null;
    if (typeof dataDir !== 'string' || dataDir.trim() === '') {
        throw configError('SQLite runtime dataDir must be a non-empty string');
    }
    return dataDir;
}

function normalizeDatabasePath(dataDir, databasePath) {
    if (databasePath !== undefined && databasePath !== null) {
        if (typeof databasePath !== 'string' || databasePath.trim() === '') {
            throw configError('SQLite runtime databasePath must be a non-empty string');
        }
        return databasePath;
    }
    if (dataDir) return join(dataDir, 'companion.sqlite');
    throw configError('SQLite runtime requires databasePath or dataDir');
}

function validateMigrations(migrations) {
    if (!Array.isArray(migrations)) {
        throw configError('SQLite runtime requires an explicit migrations array');
    }

    const versions = new Set();
    const names = new Set();
    const normalized = migrations.map((migration, index) => {
        if (!migration || typeof migration !== 'object' || Array.isArray(migration)) {
            throw configError(`Invalid SQLite migration at index ${index}`);
        }
        if (!Number.isSafeInteger(migration.version) || migration.version < 1) {
            throw configError(`SQLite migration at index ${index} has an invalid version`);
        }
        if (versions.has(migration.version)) {
            throw configError(`Duplicate SQLite migration version ${migration.version}`);
        }
        versions.add(migration.version);

        if (typeof migration.name !== 'string' || migration.name.trim() === '') {
            throw configError(`SQLite migration at index ${index} has an invalid name`);
        }
        const name = migration.name.trim();
        if (names.has(name)) {
            throw configError(`Duplicate SQLite migration name at index ${index}`);
        }
        names.add(name);

        if (typeof migration.apply !== 'function') {
            throw configError(`SQLite migration at index ${index} has an invalid apply function`);
        }
        return {version: migration.version, name, apply: migration.apply};
    });

    normalized.sort((left, right) => left.version - right.version);
    for (let index = 0; index < normalized.length; index += 1) {
        const expectedVersion = index + 1;
        if (normalized[index].version !== expectedVersion) {
            throw configError(`SQLite migration versions must be contiguous starting at 1 (missing version ${expectedVersion})`);
        }
    }
    return normalized;
}

function normalizePragmaValue(value) {
    if (typeof value === 'boolean') return value ? 'ON' : 'OFF';
    if (typeof value === 'number' && Number.isFinite(value)) return String(value);
    if (typeof value === 'string' && SAFE_PRAGMA_VALUE.test(value)) return value;
    throw configError('SQLite pragma values must be booleans, finite numbers, or simple strings');
}

function normalizePragmas(pragmas) {
    if (pragmas !== undefined && pragmas !== null && !Array.isArray(pragmas) && (typeof pragmas !== 'object')) {
        throw configError('SQLite pragmas must be an object or an array');
    }

    const entries = new Map(Object.entries(DEFAULT_PRAGMAS));
    if (Array.isArray(pragmas)) {
        for (const [index, pragma] of pragmas.entries()) {
            if (typeof pragma !== 'string' || pragma.trim() === '') {
                throw configError(`SQLite pragma at index ${index} must be a non-empty string`);
            }
            const match = /^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z0-9_.-]+)$/.exec(pragma.trim());
            if (!match) throw configError(`SQLite pragma at index ${index} has an invalid format`);
            entries.set(match[1], match[2]);
        }
    } else if (pragmas) {
        for (const [inputName, value] of Object.entries(pragmas)) {
            const name = PRAGMA_ALIASES[inputName] || inputName;
            if (!SAFE_PRAGMA_NAME.test(name)) throw configError('SQLite pragma name is invalid');
            entries.set(name, value);
        }
    }

    return [...entries.entries()].map(([name, value]) => {
        if (!SAFE_PRAGMA_NAME.test(name)) throw configError('SQLite pragma name is invalid');
        return {name, value: normalizePragmaValue(value)};
    });
}

function isMemoryDatabase(databasePath) {
    return databasePath === ':memory:' || databasePath.startsWith('file::memory:');
}

function ensureDirectories(dataDir, databasePath) {
    if (dataDir) mkdirSync(dataDir, {recursive: true});
    if (isMemoryDatabase(databasePath)) return;
    const parent = dirname(databasePath);
    if (parent && parent !== '.') mkdirSync(parent, {recursive: true});
}

function isPromiseLike(value) {
    return Boolean(value) && (typeof value === 'object' || typeof value === 'function') && typeof value.then === 'function';
}

function assertOpen(closed) {
    if (closed()) throw new SqliteRuntimeError('SQLite runtime is closed', {code: 'SQLITE_RUNTIME_CLOSED', phase: 'lifecycle'});
}

function validateAppliedLedger(rows, migrationsByVersion) {
    const applied = new Map();
    for (const row of rows) {
        if (!Number.isSafeInteger(row?.version) || row.version < 1 || applied.has(row.version)) {
            throw new SqliteRuntimeError('SQLite migration ledger is invalid', {code: 'SQLITE_RUNTIME_LEDGER', phase: 'ledger'});
        }
        const migration = migrationsByVersion.get(row.version);
        if (!migration || typeof row.name !== 'string' || row.name !== migration.name) {
            throw new SqliteRuntimeError('SQLite migration ledger does not match the configured migrations', {
                code: 'SQLITE_RUNTIME_LEDGER',
                phase: 'ledger'
            });
        }
        applied.set(row.version, row.name);
    }

    const highestApplied = Math.max(0, ...applied.keys());
    for (let version = 1; version <= highestApplied; version += 1) {
        if (!applied.has(version)) {
            throw new SqliteRuntimeError('SQLite migration ledger contains a gap', {code: 'SQLITE_RUNTIME_LEDGER', phase: 'ledger'});
        }
    }
    return applied;
}

function createMigrationRunner({database, migrations, isClosed}) {
    const migrationsByVersion = new Map(migrations.map(migration => [migration.version, migration]));

    return function runMigrations() {
        assertOpen(isClosed);
        try {
            database.exec(`CREATE TABLE IF NOT EXISTS ${MIGRATION_TABLE} (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`);
            const rows = database.prepare(`SELECT version, name FROM ${MIGRATION_TABLE} ORDER BY version`).all();
            const applied = validateAppliedLedger(rows, migrationsByVersion);

            for (const migration of migrations) {
                if (applied.has(migration.version)) continue;
                const transaction = database.transaction(() => {
                    const result = migration.apply(database);
                    if (isPromiseLike(result)) {
                        throw new Error('SQLite migrations must be synchronous');
                    }
                    database.prepare(`INSERT INTO ${MIGRATION_TABLE} (version, name, applied_at) VALUES (?, ?, ?)`)
                        .run(migration.version, migration.name, new Date().toISOString());
                });
                try {
                    transaction();
                } catch (error) {
                    throw runtimeFailure('migration', error, {version: migration.version});
                }
            }
        } catch (error) {
            if (error instanceof SqliteRuntimeError) throw error;
            throw runtimeFailure('migration ledger', error);
        }
    };
}

/**
 * Open one injected SQLite database, configure its connection, and apply the
 * explicit ordered migration list. No application or network modules are
 * loaded by this adapter.
 */
export function createSqliteRuntime({Database, dataDir, databasePath, migrations, pragmas} = {}) {
    assertDatabaseConstructor(Database);
    const normalizedDataDir = normalizeDataDir(dataDir);
    const normalizedDatabasePath = normalizeDatabasePath(normalizedDataDir, databasePath);
    const normalizedMigrations = validateMigrations(migrations);
    const normalizedPragmas = normalizePragmas(pragmas);

    let database;
    let closed = false;
    try {
        ensureDirectories(normalizedDataDir, normalizedDatabasePath);
        database = new Database(normalizedDatabasePath);
        for (const pragma of normalizedPragmas) database.pragma(`${pragma.name} = ${pragma.value}`);

        const runMigrations = createMigrationRunner({database, migrations: normalizedMigrations, isClosed: () => closed});
        runMigrations();

        const close = () => {
            if (closed) return;
            closed = true;
            try {
                database.close();
            } catch (error) {
                throw runtimeFailure('close', error);
            }
        };

        return {database, databasePath: normalizedDatabasePath, runMigrations, close};
    } catch (error) {
        if (database && typeof database.close === 'function') {
            try {
                database.close();
            } catch {
                // Preserve the startup failure; the original close error may contain a path.
            }
        }
        if (error instanceof SqliteRuntimeError) throw error;
        throw runtimeFailure('startup', error);
    }
}

export const SQLITE_RUNTIME_DEFAULT_PRAGMAS = DEFAULT_PRAGMAS;

export default createSqliteRuntime;
