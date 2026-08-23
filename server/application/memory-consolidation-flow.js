import {
    MEMORY_CONSOLIDATION_SCHEMA_VERSION,
    normalizeMemoryConsolidationCandidate
} from '../contracts/index.js';

export const MEMORY_CONSOLIDATION_FLOW_VERSION = 1;

const PRIVATE_PLANS = new WeakMap();
const MAX_CANDIDATES_PER_TURN = 8;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, max = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const result = value.trim();
    if (result.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return result;
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Memory consolidation flow clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Memory consolidation flow clock value');
    throw new TypeError('Memory consolidation flow clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated memory consolidation id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated memory consolidation id');
    throw new TypeError('Memory consolidation flow id must be a function or provide next()');
}

function repositoryFor(repositories, names, field, {optional = false} = {}) {
    for (const name of names) {
        if (repositories?.[name] !== undefined) return repositories[name];
    }
    if (optional) return null;
    throw new TypeError(`Memory consolidation flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${names.join('() or ')}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`${field} must be synchronous`);
    return value;
}

function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') return transaction(work);
    if (transaction && typeof transaction.transaction === 'function') {
        if (transaction.inTransaction) return work();
        const result = transaction.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (transaction && typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Memory consolidation transaction must be a function or provide transaction()/run()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function rowPersonaId(row) {
    return row?.personaId ?? row?.persona_id ?? row?.ownerPersonaId;
}

function rowMessageId(row) {
    return row?.id ?? row?.messageId ?? row?.message_id;
}

function sourceMessageFor(sourceLookup, command, personaId, sourceMessageId) {
    if (!sourceMessageId) return null;
    if (!sourceLookup) {
        const source = command.sourceMessage;
        if (!isRecord(source) || rowMessageId(source) !== sourceMessageId) throw new Error('Memory consolidation source message does not exist');
        if (rowPersonaId(source) !== personaId) throw new Error('Memory consolidation source message does not belong to persona');
        if (source.role !== undefined && source.role !== 'user') throw new Error('Memory consolidation source message must be user-owned');
        return {id: sourceMessageId, personaId, role: source.role ?? 'user'};
    }
    let source = sourceLookup({id: sourceMessageId, messageId: sourceMessageId, personaId});
    if (!source && sourceLookup.length >= 2) source = sourceLookup(sourceMessageId, {personaId});
    const resolved = sync(source, 'Memory consolidation source lookup');
    if (!isRecord(resolved) || rowMessageId(resolved) !== sourceMessageId) throw new Error('Memory consolidation source message does not exist');
    if (rowPersonaId(resolved) !== personaId) throw new Error('Memory consolidation source message does not belong to persona');
    if (resolved.role !== undefined && resolved.role !== 'user') throw new Error('Memory consolidation source message must be user-owned');
    return {id: sourceMessageId, personaId, role: resolved.role ?? 'user'};
}

function sourceFactFor(sourceFactLookup, sourceMessageLookup, command, personaId, ref) {
    if (ref === command.sourceMessageId) return sourceMessageFor(sourceMessageLookup, command, personaId, ref);
    if (!sourceFactLookup) {
        const supplied = command.sourceFacts?.[ref] ?? command.sourceFacts?.find?.(item => rowMessageId(item) === ref || item?.id === ref);
        if (!isRecord(supplied)) throw new Error(`Memory consolidation source fact ${ref} does not exist`);
        if (rowPersonaId(supplied) !== personaId) throw new Error('Memory consolidation source fact does not belong to persona');
        return supplied;
    }
    let fact = sourceFactLookup({id: ref, factId: ref, personaId});
    if (!fact && sourceFactLookup.length >= 2) fact = sourceFactLookup(ref, {personaId});
    const resolved = sync(fact, 'Memory consolidation source fact lookup');
    if (!isRecord(resolved) || (resolved.id ?? resolved.factId) !== ref) throw new Error(`Memory consolidation source fact ${ref} does not exist`);
    if (rowPersonaId(resolved) !== personaId) throw new Error('Memory consolidation source fact does not belong to persona');
    return resolved;
}

function owned(existing, personaId) {
    if (!existing) return null;
    if (rowPersonaId(existing) !== personaId) throw new Error('Memory consolidation candidate does not belong to persona');
    return existing;
}

function sourceMatch(existing, candidate) {
    if (!existing) return null;
    if (existing.sourceMessageId && candidate.sourceMessageId && existing.sourceMessageId !== candidate.sourceMessageId) {
        throw new Error('Memory consolidation idempotency key is bound to a different source message');
    }
    const existingRefs = new Set(existing.sourceFactRefs ?? existing.source_fact_refs ?? []);
    for (const ref of candidate.sourceFactRefs ?? []) if (existingRefs.size && !existingRefs.has(ref)) {
        throw new Error('Memory consolidation idempotency key is bound to different source facts');
    }
    return existing;
}

function resultCandidate(value) {
    if (!value) return null;
    return value.candidate ?? value.row ?? value.result ?? value;
}

function assertPlan(value) {
    if (!isRecord(value) || !PRIVATE_PLANS.has(value)) throw new TypeError('Memory consolidation flow plan is invalid');
    return PRIVATE_PLANS.get(value);
}

/**
 * Plan and persist only LLM-produced consolidation candidates. Applying a
 * plan never touches companion_memories; promotion is a separate future
 * governance operation.
 */
export function createMemoryConsolidationFlow({repositories, clock, idGenerator, id, transaction} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Memory consolidation flow repositories must be an object');
    const candidateRepository = repositoryFor(repositories, ['memoryConsolidationRepository', 'memoryConsolidation', 'consolidation'], 'candidate repository');
    const sourceFactRepository = repositoryFor(repositories, ['interactionFactRepository', 'interactionFact', 'interactionFacts', 'sourceFactRepository'], 'source fact repository', {optional: true});
    const sourceMessageRepository = repositoryFor(repositories, ['conversationRepository', 'conversation', 'messageRepository', 'messages'], 'source message repository', {optional: true});
    const findCandidate = methodFor(candidateRepository, ['findByIdempotencyKey', 'findByIdempotency'], 'candidate repository');
    const insertCandidate = methodFor(candidateRepository, ['insert', 'record', 'create'], 'candidate repository');
    const compareAndSwap = methodFor(candidateRepository, ['compareAndSwap', 'update'], 'candidate repository', {optional: true});
    const sourceFactLookup = sourceFactRepository
        ? methodFor(sourceFactRepository, ['findById', 'findFact', 'getById'], 'source fact repository', {optional: true})
        : null;
    const sourceMessageLookup = sourceMessageRepository
        ? methodFor(sourceMessageRepository, ['findMessage', 'findById', 'findSourceMessage', 'getMessage', 'getById'], 'source message repository', {optional: true})
        : null;
    const now = clockFor(clock);
    const nextId = idFor(idGenerator ?? id);

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Memory consolidation command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Memory consolidation personaId', 160);
        const candidatesInput = Array.isArray(command.memoryConsolidations)
            ? command.memoryConsolidations
            : command.candidate ? [command.candidate] : [];
        if (candidatesInput.length > MAX_CANDIDATES_PER_TURN) throw new RangeError(`Memory consolidation flow accepts at most ${MAX_CANDIDATES_PER_TURN} candidates per turn`);
        if (!candidatesInput.length) throw new TypeError('Memory consolidation flow requires at least one candidate');
        const sourceMessageId = command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id ?? null;
        const source = sourceMessageFor(sourceMessageLookup, command, personaId, sourceMessageId);
        const candidates = [];
        const existingCandidates = [];
        const sourceFacts = [];
        const seen = new Set();
        for (const input of candidatesInput) {
            const candidateInput = {
                ...input,
                personaId,
                ...(sourceMessageId && input.sourceMessageId === undefined ? {sourceMessageId} : {}),
                ...(command.causationId && input.causationId === undefined ? {causationId: command.causationId} : {}),
                ...(command.modelVersion && input.modelVersion === undefined ? {modelVersion: command.modelVersion} : {})
            };
            const candidate = normalizeMemoryConsolidationCandidate(candidateInput, {
                personaId,
                sourceMessageId,
                causationId: command.causationId,
                modelVersion: command.modelVersion
            });
            if (seen.has(candidate.idempotencyKey)) continue;
            seen.add(candidate.idempotencyKey);
            if (candidate.interactionFactId !== null) {
                if (!sourceFactLookup) throw new TypeError('Memory consolidation interactionFactId cannot be verified');
                const linkedFact = sourceFactFor(sourceFactLookup, sourceMessageLookup, command, personaId, candidate.interactionFactId);
                if (linkedFact.sourceMessageId && candidate.sourceMessageId && linkedFact.sourceMessageId !== candidate.sourceMessageId) {
                    throw new Error('Memory consolidation interaction fact is bound to a different source message');
                }
            }
            const candidateSourceMessageId = candidate.sourceMessageId ?? sourceMessageId;
            if (!source && candidateSourceMessageId) sourceMessageFor(sourceMessageLookup, command, personaId, candidateSourceMessageId);
            const sourceCommand = {...command, sourceMessageId: candidateSourceMessageId};
            for (const ref of candidate.sourceFactRefs) sourceFacts.push(sourceFactFor(sourceFactLookup, sourceMessageLookup, sourceCommand, personaId, ref));
            const existing = sourceMatch(owned(sync(findCandidate({personaId, idempotencyKey: candidate.idempotencyKey}), 'Memory consolidation idempotency lookup'), personaId), candidate);
            candidates.push(candidate);
            existingCandidates.push(existing);
        }
        const planValue = deepFreeze({
            type: 'memory_consolidation_flow_plan',
            version: MEMORY_CONSOLIDATION_FLOW_VERSION,
            schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
            personaId,
            sourceMessageId,
            candidates,
            existingCandidates,
            sourceFacts,
            previewResult: {
                personaId,
                sourceMessageId,
                candidateCount: candidates.length,
                replayed: candidates.length > 0 && existingCandidates.every(Boolean),
                promotesMemory: false
            }
        });
        PRIVATE_PLANS.set(planValue, {source, candidates, existingCandidates, sourceFacts});
        return planValue;
    }

    function apply(planValue, options = {}) {
        const planState = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Memory consolidation apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => {
            const results = [];
            for (let index = 0; index < planState.candidates.length; index += 1) {
                const candidate = planState.candidates[index];
                const existing = sourceMatch(owned(sync(findCandidate({personaId: planValue.personaId, idempotencyKey: candidate.idempotencyKey}), 'Memory consolidation idempotency lookup'), planValue.personaId), candidate);
                if (existing) {
                    results.push({created: false, replayed: true, changed: false, candidate: existing});
                    continue;
                }
                const written = sync(insertCandidate({
                    ...candidate,
                    id: nextId('memory_consolidation'),
                    personaId: planValue.personaId,
                    status: 'candidate',
                    createdAt: candidate.createdAt ?? now(),
                    updatedAt: candidate.updatedAt ?? now(),
                    revision: 1
                }), 'Memory consolidation candidate write');
                const persisted = resultCandidate(written) ?? written;
                results.push({
                    created: written?.created !== false,
                    replayed: written?.replayed === true,
                    changed: written?.changed !== false,
                    candidate: persisted
                });
            }
            return {
                type: 'memory_consolidation_flow_result',
                version: MEMORY_CONSOLIDATION_FLOW_VERSION,
                schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
                personaId: planValue.personaId,
                sourceMessageId: planValue.sourceMessageId,
                candidates: results,
                changed: results.some(item => item.changed)
            };
        });
    }

    function close(planValue, {error, expectedRevision} = {}) {
        const planState = assertPlan(planValue);
        if (!compareAndSwap) throw new TypeError('Memory consolidation candidate repository does not provide CAS');
        const message = error === undefined ? 'Memory consolidation candidate closed' : requiredText(String(error), 'Memory consolidation error', 240);
        return planState.candidates.map(candidate => compareAndSwap({
            id: candidate.id,
            candidateId: candidate.id,
            personaId: planValue.personaId,
            status: 'rejected',
            error: message,
            expectedRevision: expectedRevision ?? candidate.revision ?? 1
        }));
    }

    return Object.freeze({
        version: MEMORY_CONSOLIDATION_FLOW_VERSION,
        schemaVersion: MEMORY_CONSOLIDATION_SCHEMA_VERSION,
        plan,
        apply,
        close,
        reject: close
    });
}

export const createMemoryConsolidationCandidateFlow = createMemoryConsolidationFlow;
export const createCompanionMemoryConsolidationFlow = createMemoryConsolidationFlow;
export default createMemoryConsolidationFlow;
