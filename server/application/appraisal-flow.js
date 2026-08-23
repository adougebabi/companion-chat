import {
    APPRAISAL_SCHEMA_VERSION,
    INTERACTION_FACT_SCHEMA_VERSION,
    normalizeAppraisalCandidate,
    normalizeInteractionFact
} from '../contracts/index.js';

export const APPRAISAL_FLOW_VERSION = 1;

const PRIVATE_PLANS = new WeakMap();
const MAX_APPRAISALS_PER_TURN = 8;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Appraisal flow clock value');
    if (clock && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Appraisal flow clock value');
    throw new TypeError('Appraisal flow clock must be a function or provide now()');
}

function idFor(value) {
    if (value === undefined) return prefix => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    if (typeof value === 'function') return prefix => requiredText(value(prefix), 'Generated appraisal flow id');
    if (value && typeof value.next === 'function') return prefix => requiredText(value.next(prefix), 'Generated appraisal flow id');
    throw new TypeError('Appraisal flow id must be a function or provide next()');
}

function repositoryFor(repositories, names, field, {optional = false} = {}) {
    for (const name of names) {
        if (repositories?.[name] !== undefined) return repositories[name];
    }
    if (optional) return null;
    throw new TypeError(`Appraisal flow requires ${field}`);
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
    if (typeof transaction === 'function') {
        const result = transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (typeof transaction.transaction === 'function') {
        const result = transaction.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Appraisal flow transaction must be a function or provide transaction()/run()');
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

function sourceFor(sourceLookup, command, personaId, sourceMessageId) {
    if (!sourceLookup) {
        const source = command.sourceMessage;
        if (!isRecord(source)) throw new TypeError('Appraisal flow requires a source message');
        if (rowMessageId(source) !== sourceMessageId) throw new Error('Appraisal source message does not exist');
        if (rowPersonaId(source) !== personaId) throw new Error('Appraisal source message does not belong to persona');
        if (source.role !== undefined && source.role !== 'user') throw new Error('Appraisal source message must be user-owned');
        return {id: sourceMessageId, personaId, role: source.role ?? 'user'};
    }
    let source = sourceLookup({id: sourceMessageId, messageId: sourceMessageId, personaId});
    if (!source && sourceLookup.length >= 2) source = sourceLookup(sourceMessageId, {personaId});
    const resolved = sync(source, 'Appraisal source lookup');
    if (!isRecord(resolved) || rowMessageId(resolved) !== sourceMessageId) throw new Error('Appraisal source message does not exist');
    if (rowPersonaId(resolved) !== personaId) throw new Error('Appraisal source message does not belong to persona');
    if (resolved.role !== undefined && resolved.role !== 'user') throw new Error('Appraisal source message must be user-owned');
    return {id: sourceMessageId, personaId, role: resolved.role ?? 'user'};
}

function owned(existing, personaId, field) {
    if (!existing) return null;
    if (rowPersonaId(existing) !== personaId) throw new Error(`Appraisal ${field} does not belong to persona`);
    return existing;
}

function sourceMatch(existing, sourceMessageId, field) {
    if (!existing) return null;
    const persistedSource = existing.sourceMessageId ?? existing.source_message_id;
    if (persistedSource && persistedSource !== sourceMessageId) {
        throw new Error(`Appraisal ${field} is bound to a different source message`);
    }
    return existing;
}

function assertPlan(value) {
    if (!isRecord(value) || !PRIVATE_PLANS.has(value)) throw new TypeError('Appraisal flow plan is invalid');
    return PRIVATE_PLANS.get(value);
}

function resultRow(value, field) {
    if (!value) return null;
    const row = value[field] ?? value.row ?? value.result ?? value;
    return row && (row.id || row.personaId || row.persona_id) ? row : null;
}

/**
 * Persist server-observed interaction facts and LLM-owned appraisal candidates
 * around the existing affect reducer. No semantic fallback is performed here:
 * only nested, validated model signals reach affectFlow.plan().
 */
export function createAppraisalFlow({repositories, affectFlow, clock, idGenerator, id, transaction} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Appraisal flow repositories must be an object');
    const interactionRepository = repositoryFor(repositories, ['interactionFactRepository', 'interactionFacts', 'interactionFact'], 'interaction fact repository');
    const appraisalRepository = repositoryFor(repositories, ['appraisalRepository', 'appraisals', 'appraisal'], 'appraisal repository');
    const conversationRepository = repositoryFor(repositories, ['conversationRepository', 'conversation', 'messageRepository', 'messages'], 'conversation repository', {optional: true});
    const sourceLookup = conversationRepository
        ? methodFor(conversationRepository, ['findMessage', 'findById', 'findSourceMessage', 'getMessage', 'getById'], 'Appraisal conversation repository', {optional: true})
        : null;
    const interactionFind = methodFor(interactionRepository, ['findByIdempotencyKey', 'findByIdempotency'], 'Interaction fact repository');
    const interactionFindById = methodFor(interactionRepository, ['findById', 'findFact', 'getById'], 'Interaction fact repository', {optional: true});
    const interactionRecord = methodFor(interactionRepository, ['record', 'insert', 'create'], 'Interaction fact repository');
    const appraisalFind = methodFor(appraisalRepository, ['findByIdempotencyKey', 'findByIdempotency'], 'Appraisal repository');
    const appraisalInsert = methodFor(appraisalRepository, ['insert', 'record', 'create'], 'Appraisal repository');
    const appraisalUpdate = methodFor(appraisalRepository, ['updateStatus', 'compareAndSwap', 'update'], 'Appraisal repository', {optional: true});
    const now = clockFor(clock);
    const nextId = idFor(idGenerator ?? id);

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Appraisal flow command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Appraisal personaId', 160);
        const sourceMessageId = requiredText(
            command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id,
            'Appraisal sourceMessageId', 160
        );
        const source = sourceFor(sourceLookup, command, personaId, sourceMessageId);
        const interactionInput = {
            schemaVersion: INTERACTION_FACT_SCHEMA_VERSION,
            factType: command.interactionFact?.factType ?? command.interactionFact?.type ?? 'user_message',
            personaId,
            sourceMessageId,
            causationId: command.causationId ?? sourceMessageId,
            idempotencyKey: command.interactionFact?.idempotencyKey ?? `interaction:${sourceMessageId}`,
            payload: command.interactionFact?.payload ?? {messageId: sourceMessageId, role: source.role},
            source: command.interactionFact?.source ?? 'chat',
            evidenceRefs: command.interactionFact?.evidenceRefs ?? []
        };
        const interactionFact = normalizeInteractionFact(interactionInput, {personaId, sourceMessageId});
        const existingInteraction = sourceMatch(
            owned(sync(interactionFind({personaId, idempotencyKey: interactionFact.idempotencyKey}), 'Interaction fact lookup'), personaId, 'interaction fact'),
            sourceMessageId,
            'interaction fact'
        );
        const appraisalCandidates = Array.isArray(command.appraisals)
            ? command.appraisals
            : command.appraisal ? [command.appraisal] : [];
        if (appraisalCandidates.length > MAX_APPRAISALS_PER_TURN) throw new RangeError(`Appraisal flow accepts at most ${MAX_APPRAISALS_PER_TURN} candidates per turn`);
        const appraisals = [];
        const existingAppraisals = [];
        const seen = new Set();
        for (const candidate of appraisalCandidates) {
            const appraisal = normalizeAppraisalCandidate(candidate, {personaId, sourceMessageId, causationId: command.causationId ?? sourceMessageId, modelVersion: command.modelVersion});
            if (seen.has(appraisal.idempotencyKey)) continue;
            seen.add(appraisal.idempotencyKey);
            if (appraisal.interactionFactId !== null) {
                if (!interactionFindById) throw new TypeError('Appraisal interactionFactId cannot be verified');
                const linkedFact = sync(interactionFindById({id: appraisal.interactionFactId, personaId}), 'Appraisal interaction fact lookup');
                if (!isRecord(linkedFact)) throw new Error('Appraisal interaction fact does not exist');
                owned(linkedFact, personaId, 'interaction fact');
                sourceMatch(linkedFact, sourceMessageId, 'interaction fact');
            }
            const existing = sourceMatch(
                owned(sync(appraisalFind({personaId, idempotencyKey: appraisal.idempotencyKey}), 'Appraisal lookup'), personaId, 'candidate'),
                sourceMessageId,
                'candidate'
            );
            appraisals.push(appraisal);
            existingAppraisals.push(existing);
        }
        const affectEvents = [];
        const driveSignals = [];
        for (const appraisal of appraisals) {
            for (const event of appraisal.affectEvents) {
                if (!affectEvents.some(existing => existing.idempotencyKey === event.idempotencyKey)) affectEvents.push(event);
            }
            for (const signal of appraisal.driveSignals) {
                if (!driveSignals.some(existing => existing.idempotencyKey === signal.idempotencyKey)) driveSignals.push(signal);
            }
        }
        for (const event of command.affectEvents ?? []) {
            if (!affectEvents.some(existing => existing.idempotencyKey === event.idempotencyKey)) affectEvents.push(event);
        }
        for (const signal of command.driveSignals ?? []) {
            if (!driveSignals.some(existing => existing.idempotencyKey === signal.idempotencyKey)) driveSignals.push(signal);
        }
        const affectPlan = affectFlow && (affectEvents.length || driveSignals.length)
            ? affectFlow.plan({
                personaId,
                sourceMessageId,
                causationId: command.causationId ?? sourceMessageId,
                modelVersion: command.modelVersion ?? appraisals.find(item => item.modelVersion)?.modelVersion ?? null,
                affectEvents,
                driveSignals,
                at: command.effectiveAt ?? command.at ?? now()
            })
            : null;
        const planValue = deepFreeze({
            type: 'appraisal_flow_plan',
            version: APPRAISAL_FLOW_VERSION,
            schemaVersion: APPRAISAL_SCHEMA_VERSION,
            personaId,
            sourceMessageId,
            interactionFact,
            appraisals,
            affectPlan,
            previewResult: {
                personaId,
                sourceMessageId,
                interactionFactId: existingInteraction?.id ?? null,
                appraisalCount: appraisals.length,
                affectEventCount: affectEvents.length,
                driveSignalCount: driveSignals.length,
                replayed: Boolean(existingInteraction) && existingAppraisals.every(Boolean)
            }
        });
        PRIVATE_PLANS.set(planValue, {source, interactionFact, existingInteraction, appraisals, existingAppraisals, affectPlan});
        return planValue;
    }

    function apply(planValue, options = {}) {
        const state = assertPlan(planValue);
        const settings = typeof options === 'function' ? {transaction: options} : options;
        if (!isRecord(settings)) throw new TypeError('Appraisal flow apply options must be an object');
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.runInTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => {
            const interactionResult = sync(interactionRecord({...state.interactionFact, id: state.existingInteraction?.id ?? nextId('interaction_fact')}), 'Interaction fact write');
            const persistedInteraction = resultRow(interactionResult, 'fact') ?? state.existingInteraction ?? state.interactionFact;
            const results = [];
            for (let index = 0; index < state.appraisals.length; index += 1) {
                const candidate = state.appraisals[index];
                const existing = state.existingAppraisals[index];
                if (existing) {
                    results.push({created: false, replayed: true, appraisal: existing});
                    continue;
                }
                const inserted = sync(appraisalInsert({
                    ...candidate,
                    id: nextId('appraisal'),
                    personaId: planValue.personaId,
                    interactionFactId: persistedInteraction.id ?? null,
                    status: 'candidate',
                    createdAt: candidate.createdAt ?? now(),
                    updatedAt: candidate.updatedAt ?? now()
                }), 'Appraisal write');
                const persisted = resultRow(inserted, 'appraisal') ?? inserted;
                results.push({created: inserted?.created !== false, replayed: false, appraisal: persisted});
            }
            const affectResult = state.affectPlan && affectFlow
                ? affectFlow.apply(state.affectPlan)
                : null;
            if (appraisalUpdate && affectResult) {
                for (const item of results) {
                    if (!item.created || !item.appraisal?.id) continue;
                    const revision = item.appraisal.revision ?? 1;
                    const status = affectResult.eventCount > 0 ? 'applied' : 'no_op';
                    const updated = sync(appraisalUpdate({
                        id: item.appraisal.id,
                        appraisalId: item.appraisal.id,
                        personaId: planValue.personaId,
                        status,
                        expectedRevision: revision
                    }), 'Appraisal status update');
                    item.appraisal = updated?.appraisal ?? item.appraisal;
                }
            }
            return {
                type: 'appraisal_flow_result',
                version: APPRAISAL_FLOW_VERSION,
                schemaVersion: APPRAISAL_SCHEMA_VERSION,
                personaId: planValue.personaId,
                sourceMessageId: planValue.sourceMessageId,
                interactionFact: persistedInteraction,
                appraisals: results,
                affect: affectResult,
                changed: Boolean(interactionResult?.created || results.some(item => item.created) || affectResult?.changed)
            };
        });
    }

    return Object.freeze({version: APPRAISAL_FLOW_VERSION, schemaVersion: APPRAISAL_SCHEMA_VERSION, plan, apply});
}

export const createInteractionAppraisalFlow = createAppraisalFlow;
export const createCompanionAppraisalFlow = createAppraisalFlow;
export default createAppraisalFlow;
