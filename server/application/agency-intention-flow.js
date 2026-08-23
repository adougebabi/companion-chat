import {AGENCY_INTENTION_SCHEMA_VERSION, normalizeAgencyIntention} from '../contracts/index.js';

export const AGENCY_INTENTION_FLOW_VERSION = 1;
const PRIVATE_PLANS = new WeakMap();

function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function requiredText(value, field, max = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const result = value.trim();
    if (result.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return result;
}
function methodFor(repository, names, field) {
    for (const name of names) if (typeof repository?.[name] === 'function') return repository[name].bind(repository);
    throw new TypeError(`${field} must provide ${names.join('() or ')}()`);
}
function sync(value, field) { if (value && typeof value.then === 'function') throw new TypeError(`${field} must be synchronous`); return value; }
function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') return transaction(work);
    if (transaction && typeof transaction.transaction === 'function') {
        if (transaction.inTransaction) return work();
        const result = transaction.transaction(work);
        return typeof result === 'function' ? result() : result;
    }
    if (transaction && typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Agency intention transaction must be a function or provide transaction()/run()');
}
function rowPersonaId(row) { return row?.personaId ?? row?.persona_id; }
function rowMessageId(row) { return row?.id ?? row?.messageId ?? row?.message_id; }
function rowFactId(row) { return row?.id ?? row?.factId ?? row?.fact_id; }

function sourceMessageFor(findMessage, command, personaId, sourceMessageId) {
    if (!sourceMessageId) return null;
    if (!findMessage) {
        const source = command.sourceMessage;
        if (!isRecord(source) || rowMessageId(source) !== sourceMessageId) throw new Error('Agency source message does not exist');
        if (rowPersonaId(source) !== personaId) throw new Error('Agency source message does not belong to persona');
        if (source.role !== undefined && source.role !== 'user') throw new Error('Agency source message must be user-owned');
        return source;
    }
    const source = sync(findMessage({id: sourceMessageId, messageId: sourceMessageId, personaId}), 'Agency source message lookup');
    if (!source || rowMessageId(source) !== sourceMessageId) throw new Error('Agency source message does not exist');
    if (rowPersonaId(source) !== personaId) throw new Error('Agency source message does not belong to persona');
    if (source.role !== undefined && source.role !== 'user') throw new Error('Agency source message must be user-owned');
    return source;
}

export function createAgencyIntentionFlow({repositories, clock, idGenerator, transaction} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Agency intention flow repositories must be an object');
    const repository = repositories.agencyIntention ?? repositories.agencyIntentions ?? repositories.agency;
    if (!repository) throw new TypeError('Agency intention flow requires agency intention repository');
    const find = methodFor(repository, ['findByIdempotencyKey', 'findByIdempotency'], 'Agency intention repository');
    const insert = methodFor(repository, ['insert', 'record', 'create'], 'Agency intention repository');
    const conversation = repositories.conversation ?? repositories.conversationRepository;
    const findMessage = conversation ? methodFor(conversation, ['findMessage', 'findById'], 'Agency conversation repository') : null;
    const interactionFacts = repositories.interactionFact ?? repositories.interactionFactRepository ?? repositories.interactionFacts;
    const findFact = interactionFacts ? methodFor(interactionFacts, ['findById', 'findFact', 'getById'], 'Agency interaction fact repository') : null;
    const now = typeof clock === 'function' ? clock : () => new Date().toISOString();
    const nextId = typeof idGenerator === 'function' ? idGenerator : prefix => `${prefix}_${Date.now()}`;

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Agency intention command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Agency intention personaId', 160);
        const sourceMessageId = command.sourceMessageId ?? command.source_message_id ?? command.causationId ?? command.causation_id;
        sourceMessageFor(findMessage, command, personaId, sourceMessageId);
        const values = Array.isArray(command.agencyIntentions) ? command.agencyIntentions : command.intention ? [command.intention] : [];
        if (!values.length) throw new TypeError('Agency intention flow requires at least one intention');
        const intentions = [];
        const existing = [];
        const seen = new Set();
        for (const value of values) {
            const intention = normalizeAgencyIntention({...value, personaId, ...(sourceMessageId && value.sourceMessageId === undefined ? {sourceMessageId} : {})}, {personaId, sourceMessageId, causationId: command.causationId, modelVersion: command.modelVersion});
            if (seen.has(intention.idempotencyKey)) continue;
            seen.add(intention.idempotencyKey);
            const candidateSourceMessageId = intention.sourceMessageId ?? sourceMessageId;
            sourceMessageFor(findMessage, command, personaId, candidateSourceMessageId);
            for (const evidenceRef of intention.evidenceRefs) {
                if (evidenceRef === candidateSourceMessageId) continue;
                if (!findFact) throw new Error(`Agency evidence ${evidenceRef} cannot be verified`);
                const fact = sync(findFact({id: evidenceRef, factId: evidenceRef, personaId}), 'Agency interaction fact lookup');
                if (!fact || rowFactId(fact) !== evidenceRef) throw new Error(`Agency evidence ${evidenceRef} does not exist`);
                if (rowPersonaId(fact) !== personaId) throw new Error('Agency evidence does not belong to persona');
            }
            if (intention.interactionFactId !== null) {
                if (!findFact) throw new Error('Agency interaction fact cannot be verified');
                const fact = sync(findFact({id: intention.interactionFactId, factId: intention.interactionFactId, personaId}), 'Agency interaction fact lookup');
                if (!fact || rowFactId(fact) !== intention.interactionFactId) throw new Error('Agency interaction fact does not exist');
                if (rowPersonaId(fact) !== personaId) throw new Error('Agency interaction fact does not belong to persona');
                if (fact.sourceMessageId && candidateSourceMessageId && fact.sourceMessageId !== candidateSourceMessageId) {
                    throw new Error('Agency interaction fact is bound to a different source message');
                }
            }
            const found = sync(find({personaId, idempotencyKey: intention.idempotencyKey}), 'Agency intention idempotency lookup');
            if (found && rowPersonaId(found) !== personaId) throw new Error('Agency intention does not belong to persona');
            intentions.push(intention);
            existing.push(found ?? null);
        }
        const planValue = Object.freeze({type: 'agency_intention_flow_plan', version: AGENCY_INTENTION_FLOW_VERSION, schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION, personaId, sourceMessageId: sourceMessageId ?? null, intentions, existing, previewResult: {personaId, intentionCount: intentions.length, replayed: intentions.length > 0 && existing.every(Boolean)}});
        PRIVATE_PLANS.set(planValue, {intentions, existing});
        return planValue;
    }

    function apply(planValue, options = {}) {
        const state = PRIVATE_PLANS.get(planValue);
        if (!state) throw new TypeError('Agency intention flow plan is invalid');
        const settings = typeof options === 'function' ? {transaction: options} : options;
        const runner = settings.transaction ?? settings.callerTransaction ?? settings.commit ?? transaction;
        return transactionRunner(runner, () => {
            const results = state.intentions.map(intention => {
                const found = sync(find({personaId: planValue.personaId, idempotencyKey: intention.idempotencyKey}), 'Agency intention idempotency lookup');
                if (found) return {created: false, replayed: true, changed: false, intention: found};
                const written = sync(insert({...intention, id: nextId('agency_intention'), personaId: planValue.personaId, status: 'candidate', createdAt: now(), updatedAt: now()}), 'Agency intention write');
                return {created: written?.created !== false, replayed: written?.replayed === true, changed: written?.changed !== false, intention: written?.intention ?? written};
            });
            return {type: 'agency_intention_flow_result', version: AGENCY_INTENTION_FLOW_VERSION, schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION, personaId: planValue.personaId, intentions: results, changed: results.some(item => item.changed)};
        });
    }

    return Object.freeze({version: AGENCY_INTENTION_FLOW_VERSION, schemaVersion: AGENCY_INTENTION_SCHEMA_VERSION, plan, apply});
}

export const createAgencyFlow = createAgencyIntentionFlow;
export const createCompanionAgencyIntentionFlow = createAgencyIntentionFlow;
export default createAgencyIntentionFlow;
