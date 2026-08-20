import {normalizeStepResult} from '../contracts/index.js';

const CHANNELS = Object.freeze(['facts', 'projections', 'effects']);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function assertWriter(value, name) {
    if (typeof value !== 'function') throw new TypeError(`${name} writer must be a function`);
    return value;
}

function isDeclaredAsyncWriter(value) {
    const tag = Object.prototype.toString.call(value);
    return tag === '[object AsyncFunction]' || tag === '[object AsyncGeneratorFunction]';
}

function isPromiseLike(value) {
    return Boolean(value) && (typeof value === 'object' || typeof value === 'function') && typeof value.then === 'function';
}

function idFromItem(item, channel) {
    if (!isRecord(item)) return null;
    const candidates = channel === 'facts'
        ? ['id', 'factId']
        : channel === 'projections'
            ? ['id', 'projectionId']
            : ['effectId', 'id'];
    for (const key of candidates) {
        const value = item[key];
        if (typeof value === 'string' || typeof value === 'number') return value;
    }
    return null;
}

function idsFromWriterResult(value, item, channel) {
    if (value === null || value === undefined) {
        const fallback = idFromItem(item, channel);
        return fallback === null ? [] : [fallback];
    }
    if (Array.isArray(value)) return value.flatMap(itemValue => idsFromWriterResult(itemValue, null, channel));
    if (typeof value === 'string' || typeof value === 'number') return [value];
    if (isRecord(value)) {
        if (Array.isArray(value.ids)) return value.ids.flatMap(itemValue => idsFromWriterResult(itemValue, null, channel));
        for (const key of ['id', 'factId', 'projectionId', 'effectId', 'lastInsertRowid']) {
            const id = value[key];
            if (typeof id === 'string' || typeof id === 'number') return [id];
        }
    }
    const fallback = idFromItem(item, channel);
    return fallback === null ? [] : [fallback];
}

function normalizeWriterOutput(value, {channel, item}) {
    if (isPromiseLike(value)) {
        throw new SqliteCommitError({
            code: 'SQLITE_COMMIT_WRITER_ASYNC',
            phase: 'writer',
            channel,
            error: new Error(`The ${channel} writer must complete synchronously`)
        });
    }
    return idsFromWriterResult(value, item, channel);
}

function assertSynchronousWriters(writers) {
    for (const channel of CHANNELS) {
        if (isDeclaredAsyncWriter(writers[channel])) {
            throw new SqliteCommitError({
                code: 'SQLITE_COMMIT_WRITER_ASYNC',
                phase: 'writer',
                channel
            });
        }
    }
}

function normalizeWriters({writers, facts, projections, effects, effectIntents, writeFact, writeProjection, writeEffectIntent, writeFacts, writeProjections, writeEffects, factWriter, projectionWriter, effectIntentWriter} = {}) {
    const configured = isRecord(writers) ? writers : {};
    return {
        facts: assertWriter(configured.facts ?? configured.fact ?? facts ?? writeFacts ?? writeFact ?? factWriter, 'Facts'),
        projections: assertWriter(configured.projections ?? configured.projection ?? projections ?? writeProjections ?? writeProjection ?? projectionWriter, 'Projections'),
        effects: assertWriter(configured.effects ?? configured.effect ?? configured.effectIntents ?? effects ?? effectIntents ?? writeEffects ?? writeEffectIntent ?? effectIntentWriter, 'Effect intents')
    };
}

function resultWithAliases(counts, ids) {
    const result = {
        counts: Object.freeze({...counts}),
        ids: Object.freeze({
            facts: Object.freeze(ids.facts.slice()),
            projections: Object.freeze(ids.projections.slice()),
            effects: Object.freeze(ids.effects.slice())
        })
    };

    // Keep the canonical enumerable shape compact while making the per-channel
    // form convenient for callers migrating from bespoke repositories.
    for (const channel of ['facts', 'projections', 'effects']) {
        Object.defineProperty(result, channel, {
            enumerable: false,
            value: Object.freeze({count: counts[channel], ids: result.ids[channel]})
        });
    }
    return Object.freeze(result);
}

/**
 * Bounded error exposed by the SQLite commit boundary. It intentionally omits
 * the original error/cause so provider, SQL and filesystem details cannot leak
 * through a transport layer by accident.
 */
export class SqliteCommitError extends Error {
    constructor({code = 'SQLITE_COMMIT_FAILED', phase = 'commit', channel = null}) {
        const location = channel ? ` for ${channel}` : '';
        // Keep persistence/provider/SQL details behind this boundary. The code,
        // phase, and channel fields provide safe diagnostics for callers.
        super(`SQLite commit ${phase}${location} failed`);
        this.name = 'SqliteCommitError';
        this.code = code;
        this.phase = phase;
        this.channel = channel;
    }
}

/**
 * Create a commit-only adapter for an already-open better-sqlite3-compatible
 * database. Writers are called as `(item, database, {channel, index})` and
 * must finish synchronously. They may return an id, `{id}`, a SQLite run
 * result (`{lastInsertRowid}`), an id array, or nothing when the item itself
 * contains a usable id.
 *
 * The adapter records effect intents only. It never invokes an effect handler
 * or provider, and it never opens/configures a database or runs migrations.
 */
export function createSqliteCommitAdapter({database, db, writers, facts, projections, effects, effectIntents, writeFact, writeProjection, writeEffectIntent, writeFacts, writeProjections, writeEffects, factWriter, projectionWriter, effectIntentWriter} = {}) {
    const openDatabase = database ?? db;
    if (!isRecord(openDatabase) || typeof openDatabase.transaction !== 'function') {
        throw new TypeError('SQLite commit adapter requires an open database with transaction()');
    }
    const writerSet = normalizeWriters({writers, facts, projections, effects, effectIntents, writeFact, writeProjection, writeEffectIntent, writeFacts, writeProjections, writeEffects, factWriter, projectionWriter, effectIntentWriter});

    function commit(stepResult) {
        let normalized;
        try {
            normalized = normalizeStepResult(stepResult);
        } catch (error) {
            throw new SqliteCommitError({
                code: 'SQLITE_COMMIT_INVALID_STEP_RESULT',
                phase: 'validation',
                error
            });
        }
        assertSynchronousWriters(writerSet);

        const counts = {
            facts: normalized.facts.length,
            projections: normalized.projections.length,
            effects: normalized.effects.length,
            presentation: normalized.presentation.length
        };
        const ids = {facts: [], projections: [], effects: []};

        try {
            openDatabase.transaction(() => {
                for (const channel of CHANNELS) {
                    const values = normalized[channel];
                    const writer = writerSet[channel];
                    for (let index = 0; index < values.length; index += 1) {
                        const item = values[index];
                        let writeResult;
                        try {
                            writeResult = writer(item, openDatabase, {channel, index});
                            ids[channel].push(...normalizeWriterOutput(writeResult, {channel, item}));
                        } catch (error) {
                            if (error instanceof SqliteCommitError) throw error;
                            throw new SqliteCommitError({
                                code: 'SQLITE_COMMIT_WRITER_FAILED',
                                phase: 'writer',
                                channel,
                                error
                            });
                        }
                    }
                }
            })();
        } catch (error) {
            if (error instanceof SqliteCommitError) throw error;
            throw new SqliteCommitError({code: 'SQLITE_COMMIT_FAILED', phase: 'transaction', error});
        }

        return resultWithAliases(counts, ids);
    }

    return Object.freeze({commit, commitStepResult: commit});
}

// The boundary name is useful to callers that refer to the operation rather
// than the adapter object; both exports intentionally share one implementation.
export const createSqliteCommitBoundary = createSqliteCommitAdapter;
