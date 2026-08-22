import {
    normalizeAffectEventCandidate,
    normalizeDriveSignalCandidate
} from '../contracts/index.js';

export const AFFECT_FLOW_VERSION = 1;
export const MAX_AFFECT_EVENTS_PER_TURN = 8;

const PRIVATE_PLANS = new WeakMap();

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function clockFor(clock) {
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (isRecord(clock) && typeof clock.now === 'function') return () => new Date(clock.now()).toISOString();
    return () => new Date().toISOString();
}

function idFor(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.next === 'function') return value.next.bind(value);
    let sequence = 0;
    return prefix => `${prefix}_${Date.now()}_${sequence++}`;
}

function repositoryFor(repositories) {
    const repository = repositories?.affectRepository ?? repositories?.affect ?? repositories?.affectState;
    if (!repository || typeof repository.applyEvent !== 'function') {
        throw new TypeError('Affect flow requires repositories.affect.applyEvent()');
    }
    return repository;
}

function transactionRunner(transaction, work) {
    if (!transaction) return work();
    if (typeof transaction === 'function') return transaction(work);
    if (typeof transaction.transaction === 'function') return transaction.transaction(work);
    if (typeof transaction.run === 'function') return transaction.run(work);
    throw new TypeError('Affect flow transaction must be a function or provide transaction()/run()');
}

function candidateContext(command, personaId) {
    return {
        personaId,
        causationId: command.causationId ?? command.causation_id ?? command.sourceMessageId ?? command.source_message_id ?? null,
        sourceMessageId: command.sourceMessageId ?? command.source_message_id ?? null,
        modelVersion: command.modelVersion ?? command.model_version ?? null
    };
}

function planState(plan) {
    const state = PRIVATE_PLANS.get(plan);
    if (!state) throw new TypeError('Affect flow plan is invalid');
    return state;
}

/**
 * Plan and apply bounded affect/drives events without allowing model-supplied
 * numeric deltas. The repository owns the state reducer and transaction-safe
 * snapshot update; this flow owns candidate scoping and event provenance.
 */
export function createAffectFlow({repositories, clock, idGenerator, id, transaction} = {}) {
    const affect = repositoryFor(repositories);
    const now = clockFor(clock);
    const nextId = idFor(idGenerator ?? id);

    function plan(command = {}) {
        if (!isRecord(command)) throw new TypeError('Affect flow command must be an object');
        const personaId = requiredText(command.personaId ?? command.persona_id, 'Affect personaId', 160);
        const context = candidateContext(command, personaId);
        const affectCandidates = Array.isArray(command.affectEvents) ? command.affectEvents : [];
        const driveCandidates = Array.isArray(command.driveSignals) ? command.driveSignals : [];
        if (affectCandidates.length + driveCandidates.length > MAX_AFFECT_EVENTS_PER_TURN) {
            throw new RangeError(`Affect flow accepts at most ${MAX_AFFECT_EVENTS_PER_TURN} events per turn`);
        }
        const events = affectCandidates.map((candidate, index) => {
            const normalized = normalizeAffectEventCandidate(candidate, context);
            return {
                ...normalized,
                id: nextId('affect_event'),
                eventType: normalized.type,
                effectiveAt: command.effectiveAt ?? command.at ?? now(),
                createdAt: command.createdAt ?? command.at ?? now(),
                payload: {source: 'structured_turn', candidateIndex: index},
                modelVersion: normalized.modelVersion ?? command.modelVersion ?? null
            };
        });
        const driveEvents = driveCandidates.map((candidate, index) => {
            const normalized = normalizeDriveSignalCandidate(candidate, context);
            return {
                ...normalized,
                id: nextId('affect_event'),
                drive: normalized.drive,
                direction: normalized.direction,
                effectiveAt: command.effectiveAt ?? command.at ?? now(),
                createdAt: command.createdAt ?? command.at ?? now(),
                payload: {source: 'structured_turn', candidateIndex: affectCandidates.length + index},
                modelVersion: normalized.modelVersion ?? command.modelVersion ?? null
            };
        });
        const eventsToApply = [...events, ...driveEvents];
        const planValue = Object.freeze({
            type: 'affect_state_plan',
            version: AFFECT_FLOW_VERSION,
            personaId,
            events: Object.freeze(eventsToApply),
            previewResult: {
                personaId,
                eventCount: eventsToApply.length,
                eventTypes: eventsToApply.map(event => event.eventType ?? event.type)
            }
        });
        PRIVATE_PLANS.set(planValue, {events: eventsToApply});
        return planValue;
    }

    function apply(planValue, options = {}) {
        const state = planState(planValue);
        const runner = typeof options === 'function' ? options : options.transaction ?? transaction;
        return transactionRunner(runner, () => {
            const results = [];
            for (const event of state.events) {
                const result = event.drive !== undefined
                    ? affect.applyDriveSignal(event)
                    : affect.applyEvent(event);
                results.push(result);
            }
            return {
                type: 'affect_state_result',
                version: AFFECT_FLOW_VERSION,
                personaId: planValue.personaId,
                results,
                eventCount: results.length,
                changed: results.some(result => result?.created === true)
            };
        });
    }

    return Object.freeze({version: AFFECT_FLOW_VERSION, plan, apply});
}

export const createAffectStateFlow = createAffectFlow;
export const createCompanionAffectFlow = createAffectFlow;
export default createAffectFlow;
