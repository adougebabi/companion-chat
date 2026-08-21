import {randomUUID} from 'node:crypto';

/**
 * Application flow for persona-private relationship evidence and evolution.
 *
 * Only the relationship layer is mutable here. Foundation, life blueprint,
 * state, and provider/runtime concerns remain outside this flow.
 */
export const RELATIONSHIP_FLOW_VERSION = 1;
export const RELATIONSHIP_EVOLUTION_JOB_TYPE = 'relationship_evolution';
export const RELATIONSHIP_DEBOUNCE_MS = 10 * 60 * 1000;

const PRIVATE_PLANS = new WeakMap();
const PATCH_LIMITS = Object.freeze({communicationStyle: 240, relationshipNote: 400});
const MAX_EVIDENCE = 12;
const MAX_MEMORIES = 8;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function optionalText(value, field, maxLength = 240) {
    if (value === undefined || value === null || value === '') return null;
    return requiredText(value, field, maxLength);
}

function timestamp(value, field) {
    const normalized = value instanceof Date ? value.toISOString() : value;
    if (typeof normalized !== 'string' || !normalized.trim() || !Number.isFinite(Date.parse(normalized))) throw new TypeError(`${field} must be a valid timestamp`);
    return new Date(Date.parse(normalized)).toISOString();
}

function clockFunction(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Relationship clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Relationship clock value');
    throw new TypeError('Relationship flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated relationship id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return prefix => requiredText(idGenerator.next(prefix), 'Generated relationship id');
    throw new TypeError('Relationship flow idGenerator must be a function or provide next()');
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') throw new TypeError(`Relationship flow ${field} must be an object`);
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Relationship flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Relationship flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Relationship flow ${field} must provide ${names.join('() or ')}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Relationship flow ${field} must be synchronous`);
    return value;
}

function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') {
        const result = transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (isRecord(transaction) && typeof transaction.transaction === 'function') {
        const result = transaction.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (isRecord(transaction) && typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Relationship transaction must be a function or provide transaction()/run()');
}

function json(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed ?? fallback;
    } catch {
        return fallback;
    }
}

function valueFor(row, camel, snake) {
    return row?.[camel] === undefined ? row?.[snake] : row[camel];
}

function patchFromRow(row) {
    return json(row?.nextPatch ?? row?.next_patch, {});
}

function activePatchFromRow(row) {
    const value = row?.nextPatch ?? row?.next_patch ?? row?.patch;
    return json(value, {});
}

function normalizePatch(value) {
    if (!isRecord(value)) return {};
    const patch = {};
    if (typeof value.communicationStyle === 'string' && value.communicationStyle.trim()) patch.communicationStyle = value.communicationStyle.trim().slice(0, PATCH_LIMITS.communicationStyle);
    if (typeof value.relationshipNote === 'string' && value.relationshipNote.trim()) patch.relationshipNote = value.relationshipNote.trim().slice(0, PATCH_LIMITS.relationshipNote);
    if (Array.isArray(value.sharedTopics)) {
        patch.sharedTopics = [...new Set(value.sharedTopics.map(item => String(item).trim()).filter(Boolean))].slice(0, 8).map(item => item.slice(0, 48));
    }
    return patch;
}

function patchEqual(left, right) {
    return JSON.stringify(left || {}) === JSON.stringify(right || {});
}

function evidenceKey(item) {
    return `${item.type}:${item.id ?? item.sourceId ?? item.content ?? JSON.stringify(item)}`;
}

function normalizeEvidence(value, personaId) {
    const rows = value === undefined || value === null ? [] : Array.isArray(value) ? value : [value];
    const seen = new Set();
    const result = [];
    for (const raw of rows.slice(0, MAX_EVIDENCE * 2)) {
        if (!isRecord(raw)) continue;
        const owner = raw.personaId ?? raw.persona_id;
        if (owner !== undefined && owner !== null && owner !== personaId) throw new Error('Relationship evidence does not belong to persona');
        const type = requiredText(raw.type ?? raw.kind ?? 'interaction', 'Relationship evidence.type', 60);
        const id = optionalText(raw.id ?? raw.sourceId ?? raw.source_id, 'Relationship evidence.id', 160);
        const content = optionalText(raw.content ?? raw.summary ?? raw.text, 'Relationship evidence.content', 400);
        if (!id && !content) throw new TypeError('Relationship evidence requires id or content');
        const normalized = {type, ...(id ? {id, sourceId: id} : {}), ...(content ? {content} : {}), personaId};
        const key = evidenceKey(normalized);
        if (seen.has(key)) continue;
        seen.add(key);
        result.push(normalized);
        if (result.length >= MAX_EVIDENCE) break;
    }
    return result;
}

function evidenceKeys(evidence) {
    return new Set((Array.isArray(evidence) ? evidence : []).map(evidenceKey));
}

function overlap(left, right) {
    const rhs = evidenceKeys(right);
    return (Array.isArray(left) ? left : []).some(item => rhs.has(evidenceKey(item)));
}

function rowAge(row, nowMs) {
    const created = Date.parse(valueFor(row, 'createdAt', 'created_at') || '');
    return Number.isFinite(created) ? nowMs - created : Number.POSITIVE_INFINITY;
}

function normalizeEvolutionRow(row) {
    if (!row) return null;
    return {
        ...row,
        id: row.id,
        personaId: valueFor(row, 'personaId', 'persona_id'),
        reason: row.reason,
        evidence: json(valueFor(row, 'evidence', 'evidence_json'), []),
        previousPatch: json(valueFor(row, 'previousPatch', 'previous_patch'), {}),
        nextPatch: json(valueFor(row, 'nextPatch', 'next_patch'), {}),
        status: row.status,
        createdAt: valueFor(row, 'createdAt', 'created_at'),
        revertedAt: valueFor(row, 'revertedAt', 'reverted_at') ?? null
    };
}

function resultEnvelope(result) {
    return {
        ...result,
        facts: Array.isArray(result.facts) ? result.facts : [],
        projections: Array.isArray(result.projections) ? result.projections : [],
        effects: Array.isArray(result.effects) ? result.effects : [],
        presentation: Array.isArray(result.presentation) ? result.presentation : []
    };
}

export function createRelationshipFlow({
    repositories,
    clock,
    idGenerator,
    debounceMs = RELATIONSHIP_DEBOUNCE_MS,
    evaluator,
    transaction
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Relationship flow repositories must be an object');
    const relationshipRepository = resolveRepository(repositories, ['relationshipRepository', 'relationship', 'evolutionRepository', 'evolution'], 'relationship repository');
    const personaRepository = resolveRepository(repositories, ['personaRepository', 'personas', 'persona'], 'persona repository', {optional: true});
    const jobRepository = resolveRepository(repositories, ['jobRepository', 'job', 'effectRepository'], 'job repository', {optional: true});
    const personaLookup = personaRepository ? methodFor(personaRepository, ['findActive', 'findById', 'get'], 'persona repository', {optional: true}) : null;
    const activePatchLookup = methodFor(relationshipRepository, ['activePatch', 'findActivePatch', 'currentPatch'], 'relationship repository', {optional: true});
    const insertEvolution = methodFor(relationshipRepository, ['insertEvolution', 'createEvolution', 'recordEvolution', 'insert'], 'relationship repository', {optional: true});
    const recentEvolutionLookup = methodFor(relationshipRepository, ['listRecent', 'list', 'recent'], 'relationship repository', {optional: true});
    const rollbackEvolution = methodFor(relationshipRepository, ['rollbackEvolution', 'rollback', 'revert'], 'relationship repository', {optional: true});
    const findQueuedJob = jobRepository ? methodFor(jobRepository, ['findQueued', 'findPending', 'findLatestQueued', 'findBySource'], 'job repository', {optional: true}) : null;
    const enqueueJob = jobRepository ? methodFor(jobRepository, ['enqueue', 'create', 'insert'], 'job repository', {optional: true}) : null;
    const supersedeJobs = jobRepository ? methodFor(jobRepository, ['completeQueued', 'supersedeQueued', 'cancelQueued'], 'job repository', {optional: true}) : null;
    const now = clockFunction(clock);
    const nextId = idFunction(idGenerator);
    const debounce = Number.isFinite(Number(debounceMs)) && Number(debounceMs) >= 0 ? Number(debounceMs) : RELATIONSHIP_DEBOUNCE_MS;

    function personaFor(personaId, supplied) {
        if (!personaLookup) return supplied ?? {id: personaId};
        const row = sync(personaLookup(personaId), 'persona lookup');
        if (!row) throw new Error('Relationship persona does not exist');
        return row;
    }

    function currentPatch(personaId) {
        if (!activePatchLookup) return {};
        const row = sync(activePatchLookup({personaId}), 'active relationship lookup');
        return activePatchFromRow(row);
    }

    function recentRows(personaId) {
        if (!recentEvolutionLookup) return [];
        const rows = recentEvolutionLookup({personaId, limit: 32});
        return (sync(rows, 'relationship history lookup') || []).map(normalizeEvolutionRow);
    }

    function isDebounced(personaId, evidence, at, force) {
        if (force || debounce === 0 || evidence.length === 0) return null;
        const currentMs = Date.parse(at);
        const recent = recentRows(personaId).find(row => row.status === 'applied' && rowAge(row, currentMs) >= 0 && rowAge(row, currentMs) < debounce && overlap(row.evidence, evidence));
        return recent ?? null;
    }

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Relationship command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Relationship personaId', 160);
        const at = timestamp(command.at ?? now(), 'Relationship evaluation time');
        personaFor(personaId, command.persona);
        const evidence = normalizeEvidence(command.evidence ?? command.evidenceItems, personaId);
        const previousPatch = currentPatch(personaId);
        const incomingPatch = normalizePatch(command.patch ?? command.relationshipPatch);
        const nextPatch = {...previousPatch, ...incomingPatch};
        const unchanged = patchEqual(previousPatch, nextPatch);
        const debounced = isDebounced(personaId, evidence, at, command.force === true);
        const status = unchanged ? 'skipped' : debounced ? 'suppressed' : 'accepted';
        const evolutionId = status === 'accepted' ? nextId('evolution') : null;
        const result = {
            type: 'relationship_evolution_plan',
            version: RELATIONSHIP_FLOW_VERSION,
            personaId,
            reason: optionalText(command.reason, 'Relationship reason', 300) || '基于近期互动的关系层更新',
            evidence,
            previousPatch,
            incomingPatch,
            nextPatch,
            status,
            debounced: Boolean(debounced),
            noChange: unchanged,
            suppressedReason: unchanged ? 'no_change' : debounced ? 'debounced' : null,
            evolutionId,
            preallocatedIds: evolutionId ? {evolutionId} : {},
            previewResult: {evolutionId, status, debounced: Boolean(debounced), noChange: unchanged, evidenceCount: evidence.length}
        };
        const frozen = Object.freeze(result);
        PRIVATE_PLANS.set(frozen, {command, at, personaId, evidence, previousPatch, nextPatch, debounced: Boolean(debounced), debouncedBy: debounced, unchanged});
        return frozen;
    }

    function applyWithin(planValue) {
        const state = PRIVATE_PLANS.get(planValue);
        if (!state) throw new TypeError('Relationship evolution plan is invalid');
        if (state.unchanged || state.debounced) {
            return resultEnvelope({
                type: 'relationship_evolution_result',
                version: RELATIONSHIP_FLOW_VERSION,
                personaId: state.personaId,
                evolutionId: null,
                status: state.debounced ? 'suppressed' : 'skipped',
                debounced: state.debounced,
                noChange: state.unchanged,
                evidence: state.evidence,
                facts: [],
                projections: [{type: 'relationship_evolution', status: state.debounced ? 'debounced' : 'no_change', personaId: state.personaId}],
                presentation: []
            });
        }
        if (!insertEvolution) throw new TypeError('Relationship flow requires an evolution insert port');
        const row = sync(insertEvolution({
            id: planValue.evolutionId,
            personaId: state.personaId,
            reason: planValue.reason,
            evidence: state.evidence,
            previousPatch: state.previousPatch,
            nextPatch: state.nextPatch,
            status: 'applied',
            createdAt: state.at,
            updatedAt: state.at
        }), 'evolution insert');
        const evolution = normalizeEvolutionRow(row) ?? {
            id: planValue.evolutionId,
            personaId: state.personaId,
            reason: planValue.reason,
            evidence: state.evidence,
            previousPatch: state.previousPatch,
            nextPatch: state.nextPatch,
            status: 'applied',
            createdAt: state.at
        };
        return resultEnvelope({
            type: 'relationship_evolution_result',
            version: RELATIONSHIP_FLOW_VERSION,
            personaId: state.personaId,
            evolutionId: evolution.id,
            evolution,
            status: 'applied',
            debounced: false,
            noChange: false,
            evidence: state.evidence,
            facts: [{type: 'relationship_evolution_applied', evolutionId: evolution.id, personaId: state.personaId, evidence: state.evidence}],
            projections: [{type: 'relationship_patch', personaId: state.personaId, patch: state.nextPatch, evolutionId: evolution.id}],
            presentation: [{type: 'relationship_evolution', evolutionId: evolution.id}]
        });
    }

    function apply(planValue, options = {}) {
        if (!isRecord(options) && typeof options !== 'function') throw new TypeError('Relationship apply options must be an object');
        const runner = typeof options === 'function' ? options : options.transaction ?? options.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue));
    }

    function evolve(command = {}) {
        return apply(plan(command));
    }

    function submitEvidence(command = {}) {
        if (!isRecord(command)) throw new TypeError('Relationship evidence command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Relationship personaId', 160);
        const at = timestamp(command.at ?? now(), 'Relationship evidence time');
        personaFor(personaId, command.persona);
        const evidence = normalizeEvidence(command.evidence ?? command.evidenceItems, personaId);
        if (!evidence.length) return resultEnvelope({type: 'relationship_evidence', personaId, status: 'skipped', reason: 'no_evidence', evidence: []});
        const recent = isDebounced(personaId, evidence, at, command.force === true);
        if (recent) return resultEnvelope({type: 'relationship_evidence', personaId, status: 'suppressed', reason: 'debounced', evidence, evolutionId: recent.id, debounced: true});
        const queuedExisting = findQueuedJob
            ? sync(findQueuedJob({personaId, jobType: RELATIONSHIP_EVOLUTION_JOB_TYPE, evidence, sourceMessageId: command.sourceMessageId ?? command.messageId ?? null}), 'relationship queued-job lookup')
            : null;
        if (queuedExisting) return resultEnvelope({type: 'relationship_evidence', personaId, status: 'suppressed', reason: 'debounced', evidence, queued: true, job: queuedExisting, jobId: queuedExisting.id ?? queuedExisting.jobId, debounced: true});
        if (!enqueueJob) return resultEnvelope({type: 'relationship_evidence', personaId, status: 'accepted', evidence, queued: false, effects: [{effectId: `effect_relationship_${personaId}_${Date.parse(at)}`, kind: RELATIONSHIP_EVOLUTION_JOB_TYPE, capability: 'relationship', idempotencyKey: `relationship:${personaId}:${evidence.map(evidenceKey).join('|')}`, causationId: command.causationId ?? personaId, payload: {personaId, evidence}}]});
        const job = {
            id: command.jobId ?? nextId('job'),
            jobType: RELATIONSHIP_EVOLUTION_JOB_TYPE,
            personaId,
            messageId: command.messageId ?? command.sourceMessageId ?? evidence.find(item => item.type === 'message')?.id ?? null,
            priority: Number.isFinite(Number(command.priority)) ? Number(command.priority) : 1,
            runAfter: command.runAfter ?? new Date(Date.parse(at) + debounce).toISOString(),
            maxAttempts: Number.isFinite(Number(command.maxAttempts)) ? Number(command.maxAttempts) : 4,
            payload: {personaId, sourceMessageId: command.sourceMessageId ?? command.messageId ?? null, evidence, reason: command.reason ?? null, patch: command.patch ?? command.relationshipPatch ?? null, causationId: command.causationId ?? null}
        };
        if (supersedeJobs && command.supersede !== false) sync(supersedeJobs({personaId, jobType: RELATIONSHIP_EVOLUTION_JOB_TYPE, excludeId: job.id, result: {skipped: 'superseded_by_newer_evidence', supersededByJobId: job.id}, now: at}), 'relationship job debounce');
        const queued = sync(enqueueJob({...job, createdAt: at, updatedAt: at}), 'relationship job enqueue') ?? job;
        return resultEnvelope({type: 'relationship_evidence', personaId, status: 'accepted', evidence, queued: true, job: queued, jobId: queued.id ?? job.id, effects: [], presentation: [{type: 'relationship_evidence_queued', personaId, jobId: queued.id ?? job.id}]});
    }

    function rollback(command = {}) {
        if (!isRecord(command)) throw new TypeError('Relationship rollback command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Relationship personaId', 160);
        const evolutionId = requiredText(command.evolutionId ?? command.evolution_id ?? command.id, 'Relationship evolutionId', 160);
        personaFor(personaId, command.persona);
        const rows = recentRows(personaId).filter(row => row.status === 'applied');
        if (!rows.length || rows[0].id !== evolutionId) throw Object.assign(new Error('只能回滚当前最新的关系层演化'), {status: 400});
        if (!rollbackEvolution) throw new TypeError('Relationship repository requires rollbackEvolution()');
        const updated = sync(rollbackEvolution({personaId, evolutionId, updatedAt: command.updatedAt ?? now()}), 'relationship rollback');
        if (!updated) throw Object.assign(new Error('关系演化不存在或已回滚'), {status: 404});
        return resultEnvelope({type: 'relationship_rollback_result', version: RELATIONSHIP_FLOW_VERSION, personaId, evolutionId, status: 'reverted', evolution: normalizeEvolutionRow(updated) ?? updated, projections: [{type: 'relationship_patch_rollback', personaId, evolutionId}], presentation: [{type: 'relationship_rollback', personaId, evolutionId}]});
    }

    async function handleJob(job, context = {}) {
        const raw = job?.payload ?? job?.payload_json;
        let payload = raw;
        if (!isRecord(payload)) {
            try { payload = JSON.parse(payload || '{}'); } catch { throw Object.assign(new Error('Relationship job payload is invalid'), {retryable: false, terminal: true}); }
        }
        const personaId = payload.personaId ?? job?.personaId ?? job?.persona_id;
        const command = {...payload, personaId, at: payload.at ?? context.now};
        if (!command.patch && typeof evaluator === 'function') {
            const raw = await evaluator(Object.freeze(command), context);
            let evaluated = raw;
            if (typeof raw === 'string') {
                try { evaluated = JSON.parse(raw); } catch { evaluated = null; }
            }
            if (evaluated && typeof evaluated === 'object') {
                Object.assign(command, evaluated);
                if (!command.patch && evaluated.relationshipPatch) command.patch = evaluated.relationshipPatch;
            }
        }
        if (!command.patch && !command.relationshipPatch) return {status: 'complete', result: {skipped: 'evolution_not_evaluated', personaId}};
        const result = evolve(command);
        return {status: 'complete', result};
    }

    const flow = {
        version: RELATIONSHIP_FLOW_VERSION,
        plan,
        preview: plan,
        apply,
        evolve,
        execute: evolve,
        run: evolve,
        submitEvidence,
        recordEvidence: submitEvidence,
        enqueueEvidence: submitEvidence,
        rollback,
        rollbackEvolution: rollback,
        handleJob,
        runJob: handleJob,
        normalizePatch,
        normalizeEvidence
    };
    return Object.freeze(flow);
}

export const createRelationshipApplicationFlow = createRelationshipFlow;
export const createPersonaRelationshipFlow = createRelationshipFlow;
export default createRelationshipFlow;
