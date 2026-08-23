import {
    SELF_MODEL_CLAIM_SCHEMA_VERSION,
    normalizeSelfModelClaim
} from '../contracts/index.js';

export const SELF_MODEL_FLOW_VERSION = 1;

const PRIVATE_PLANS = new WeakMap();
const MAX_CLAIMS_PER_TURN = 8;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, max = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const result = value.trim();
    if (result.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return result;
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (clock && typeof clock.now === 'function') return () => new Date(clock.now()).toISOString();
    throw new TypeError('Self-model flow clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated self-model flow id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated self-model flow id');
    throw new TypeError('Self-model flow id must be a function or provide next()');
}

function repositoryFor(repositories, names, field, {optional = false} = {}) {
    for (const name of names) if (repositories?.[name] !== undefined) return repositories[name];
    if (optional) return null;
    throw new TypeError(`Self-model flow requires ${field}`);
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
    throw new TypeError('Self-model transaction must be a function or provide transaction()/run()');
}

function rowPersonaId(row) {
    return row?.personaId ?? row?.persona_id ?? row?.ownerPersonaId;
}

function rowMessageId(row) {
    return row?.id ?? row?.messageId ?? row?.message_id;
}

function rowFactId(row) {
    return row?.id ?? row?.factId ?? row?.fact_id;
}

function sourceMessageFor(sourceLookup, command, personaId, sourceMessageId) {
    if (!sourceMessageId) return null;
    if (!sourceLookup) {
        const source = command.sourceMessage;
        if (!isRecord(source) || rowMessageId(source) !== sourceMessageId) throw new Error('Self-model source message does not exist');
        if (rowPersonaId(source) !== personaId) throw new Error('Self-model source message does not belong to persona');
        if (source.role !== undefined && source.role !== 'user') throw new Error('Self-model source message must be user-owned');
        return source;
    }
    let source = sourceLookup({id: sourceMessageId, messageId: sourceMessageId, personaId});
    if (!source && sourceLookup.length >= 2) source = sourceLookup(sourceMessageId, {personaId});
    const resolved = sync(source, 'Self-model source message lookup');
    if (!isRecord(resolved) || rowMessageId(resolved) !== sourceMessageId) throw new Error('Self-model source message does not exist');
    if (rowPersonaId(resolved) !== personaId) throw new Error('Self-model source message does not belong to persona');
    if (resolved.role !== undefined && resolved.role !== 'user') throw new Error('Self-model source message must be user-owned');
    return resolved;
}

function sourceFactFor(sourceLookup, command, personaId, ref) {
    if (!sourceLookup) {
        const supplied = command.sourceFacts?.[ref] ?? command.sourceFacts?.find?.(item => rowFactId(item) === ref);
        if (!isRecord(supplied)) return null;
        if (rowPersonaId(supplied) !== personaId) throw new Error('Self-model evidence fact does not belong to persona');
        return supplied;
    }
    let fact = sourceLookup({id: ref, factId: ref, personaId});
    if (!fact && sourceLookup.length >= 2) fact = sourceLookup(ref, {personaId});
    const resolved = sync(fact, 'Self-model source fact lookup');
    if (!isRecord(resolved) || rowFactId(resolved) !== ref) return null;
    if (rowPersonaId(resolved) !== personaId) throw new Error('Self-model evidence fact does not belong to persona');
    return resolved;
}

function evidenceFor({ref, sourceMessageLookup, sourceFactLookup, command, personaId}) {
    if (ref === command.sourceMessageId) return sourceMessageFor(sourceMessageLookup, command, personaId, ref);
    const fact = sourceFactFor(sourceFactLookup, command, personaId, ref);
    if (fact) return fact;
    return sourceMessageFor(sourceMessageLookup, command, personaId, ref);
}

function owned(existing, personaId) {
    if (!existing) return null;
    if (rowPersonaId(existing) !== personaId) throw new Error('Self-model claim does not belong to persona');
    return existing;
}

function sourceMatch(existing, candidate) {
    if (!existing) return null;
    if (existing.sourceMessageId && candidate.sourceMessageId && existing.sourceMessageId !== candidate.sourceMessageId) {
        throw new Error('Self-model claim idempotency key is bound to a different source message');
    }
    return existing;
}

function resultClaim(value) {
    if (!value) return null;
    return value.claim ?? value.row ?? value.result ?? value;
}

function assertPlan(value) {
    if (!isRecord(value) || !PRIVATE_PLANS.has(value)) throw new TypeError('Self-model flow plan is invalid');
    return PRIVATE_PLANS.get(value);
}

/**
 * Plan and apply LLM-produced self-model claims. Applying a plan only writes
 * this claim ledger; it never changes foundation, relationship, or memory.
 */
export function createSelfModelFlow({repositories, clock, idGenerator, id, transaction} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Self-model flow repositories must be an object');
    const claimRepository = repositoryFor(repositories, ['selfModelRepository', 'selfModel', 'selfModelClaimRepository', 'selfModelClaims'], 'claim repository');
    const sourceFactRepository = repositoryFor(repositories, ['interactionFactRepository', 'interactionFact', 'interactionFacts', 'sourceFactRepository'], 'source fact repository', {optional: true});
    const sourceMessageRepository = repositoryFor(repositories, ['conversationRepository', 'conversation', 'messageRepository', 'messages'], 'source message repository', {optional: true});
    const findClaim = methodFor(claimRepository, ['findByIdempotencyKey', 'findByIdempotency'], 'claim repository');
    const insertClaim = methodFor(claimRepository, ['insert', 'record', 'create'], 'claim repository');
    const compareAndSwap = methodFor(claimRepository, ['compareAndSwap', 'update'], 'claim repository', {optional: true});
    const sourceFactLookup = sourceFactRepository ? methodFor(sourceFactRepository, ['findById', 'findFact', 'getById'], 'source fact repository', {optional: true}) : null;
    const sourceMessageLookup = sourceMessageRepository ? methodFor(sourceMessageRepository, ['findMessage', 'findById', 'findSourceMessage', 'getMessage', 'getById'], 'source message repository', {optional: true}) : null;
    const now = clockFor(clock);
    const nextId = idFor(idGenerator ?? id);

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Self-model command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Self-model personaId', 160);
        const claimsInput = Array.isArray(command.selfModelClaims)
            ? command.selfModelClaims
            : Array.isArray(command.claims) ? command.claims : command.claim ? [command.claim] : [];
        if (claimsInput.length > MAX_CLAIMS_PER_TURN) throw new RangeError(`Self-model flow accepts at most ${MAX_CLAIMS_PER_TURN} claims per turn`);
        if (!claimsInput.length) throw new TypeError('Self-model flow requires at least one claim');
        const sourceMessageId = command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id ?? null;
        const source = sourceMessageFor(sourceMessageLookup, command, personaId, sourceMessageId);
        const claims = [];
        const existingClaims = [];
        const seen = new Set();
        for (const input of claimsInput) {
            const claim = normalizeSelfModelClaim({
                ...input,
                personaId,
                ...(sourceMessageId && input.sourceMessageId === undefined ? {sourceMessageId} : {}),
                ...(command.causationId && input.causationId === undefined ? {causationId: command.causationId} : {}),
                ...(command.modelVersion && input.modelVersion === undefined ? {modelVersion: command.modelVersion} : {})
            }, {personaId, sourceMessageId, causationId: command.causationId, modelVersion: command.modelVersion});
            if (seen.has(claim.idempotencyKey)) continue;
            seen.add(claim.idempotencyKey);
            const candidateSource = claim.sourceMessageId ?? sourceMessageId;
            if (!candidateSource) throw new Error('Self-model claim requires a source message');
            sourceMessageFor(sourceMessageLookup, command, personaId, candidateSource);
            for (const ref of claim.evidenceRefs) evidenceFor({ref, sourceMessageLookup, sourceFactLookup, command: {...command, sourceMessageId: candidateSource}, personaId});
            if (claim.interactionFactId !== null) {
                const fact = sourceFactFor(sourceFactLookup, command, personaId, claim.interactionFactId);
                if (!fact) throw new Error(`Self-model interaction fact ${claim.interactionFactId} does not exist`);
                if (fact.sourceMessageId && candidateSource && fact.sourceMessageId !== candidateSource) throw new Error('Self-model interaction fact is bound to a different source message');
            }
            const existing = sourceMatch(owned(sync(findClaim({personaId, idempotencyKey: claim.idempotencyKey}), 'Self-model claim idempotency lookup'), personaId), claim);
            claims.push(claim);
            existingClaims.push(existing);
        }
        const planValue = Object.freeze({
            type: 'self_model_flow_plan',
            version: SELF_MODEL_FLOW_VERSION,
            schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
            personaId,
            sourceMessageId,
            claims,
            existingClaims,
            previewResult: {
                personaId,
                sourceMessageId,
                claimCount: claims.length,
                replayed: claims.length > 0 && existingClaims.every(Boolean)
            }
        });
        PRIVATE_PLANS.set(planValue, {claims, existingClaims});
        return planValue;
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Self-model apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => {
            const results = [];
            for (const candidate of state.claims) {
                const existing = sourceMatch(owned(sync(findClaim({personaId: planValue.personaId, idempotencyKey: candidate.idempotencyKey}), 'Self-model claim idempotency lookup'), planValue.personaId), candidate);
                if (existing) {
                    results.push({created: false, replayed: true, changed: false, claim: existing});
                    continue;
                }
                const written = sync(insertClaim({
                    ...candidate,
                    id: nextId('self_model_claim'),
                    personaId: planValue.personaId,
                    status: 'active',
                    createdAt: now(),
                    updatedAt: now(),
                    revision: 1
                }), 'Self-model claim write');
                results.push({
                    created: written?.created !== false,
                    replayed: written?.replayed === true,
                    changed: written?.changed !== false,
                    claim: resultClaim(written) ?? written
                });
            }
            return {
                type: 'self_model_flow_result',
                version: SELF_MODEL_FLOW_VERSION,
                schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
                personaId: planValue.personaId,
                sourceMessageId: planValue.sourceMessageId,
                claims: results,
                changed: results.some(item => item.changed)
            };
        });
    }

    function close(planValue, {error, expectedRevision} = {}) {
        const state = assertPlan(planValue);
        if (!compareAndSwap) throw new TypeError('Self-model claim repository does not provide CAS');
        const message = error === undefined ? 'Self-model claim closed' : requiredText(String(error), 'Self-model error', 240);
        return state.claims.map(claim => compareAndSwap({
            id: claim.id,
            claimId: claim.id,
            personaId: planValue.personaId,
            status: 'rejected',
            error: message,
            expectedRevision: expectedRevision ?? claim.revision ?? 1
        }));
    }

    return Object.freeze({
        version: SELF_MODEL_FLOW_VERSION,
        schemaVersion: SELF_MODEL_CLAIM_SCHEMA_VERSION,
        plan,
        apply,
        close,
        reject: close
    });
}

export const createSelfModelClaimFlow = createSelfModelFlow;
export const createCompanionSelfModelFlow = createSelfModelFlow;
export default createSelfModelFlow;
