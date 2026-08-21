import {randomUUID} from 'node:crypto';

/**
 * Application flow for the persona life-model timeline.
 *
 * The flow owns candidate policy and decision semantics. It deliberately knows
 * nothing about SQLite, HTTP, or a provider. Repositories are narrow ports and
 * the generic life-event flow remains the only fact-creation boundary.
 */
export const TIMELINE_FLOW_VERSION = 1;
export const TIMELINE_DECISION_TYPES = Object.freeze(['start_event', 'no_event', 'defer_slot']);
export const TIMELINE_DECISION_STATUSES = Object.freeze(['proposed', 'accepted', 'suppressed', 'executed', 'expired', 'skipped']);
export const TIMELINE_SLOT_STATUSES = Object.freeze(['confirmed', 'active', 'completed', 'skipped']);

const PRIVATE_PLANS = new WeakMap();
const MAX_CANDIDATES = 32;
const MAX_SLOTS = 32;
const MAX_TEXT = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = MAX_TEXT) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function optionalText(value, field, maxLength = MAX_TEXT) {
    if (value === undefined || value === null || value === '') return null;
    return requiredText(value, field, maxLength);
}

function timestamp(value, field) {
    const normalized = value instanceof Date ? value.toISOString() : value;
    if (typeof normalized !== 'string' || !normalized.trim() || !Number.isFinite(Date.parse(normalized))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return new Date(Date.parse(normalized)).toISOString();
}

function clockFunction(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Timeline clock value');
    if (isRecord(clock) && typeof clock.now === 'function') return () => timestamp(clock.now(), 'Timeline clock value');
    throw new TypeError('Timeline flow clock must be a function or provide now()');
}

function idFunction(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') return prefix => requiredText(idGenerator(prefix), 'Generated timeline id');
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return prefix => requiredText(idGenerator.next(prefix), 'Generated timeline id');
    throw new TypeError('Timeline flow idGenerator must be a function or provide next()');
}

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    const source = isRecord(repositories) ? repositories : {};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (!isRecord(source[name]) && typeof source[name] !== 'function') throw new TypeError(`Timeline flow ${field} must be an object`);
            return source[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Timeline flow requires ${field}`);
}

function methodFor(repository, names, field, {optional = false} = {}) {
    if (typeof repository === 'function') return repository;
    if (isRecord(repository)) {
        for (const name of names) {
            if (repository[name] !== undefined) {
                if (typeof repository[name] !== 'function') throw new TypeError(`Timeline flow ${field}.${name} must be a function`);
                return repository[name].bind(repository);
            }
        }
    }
    if (optional) return null;
    throw new TypeError(`Timeline flow ${field} must provide ${names.join('() or ')}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Timeline flow ${field} must be synchronous`);
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
    throw new TypeError('Timeline transaction must be a function or provide transaction()/run()');
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

function rowDecision(row) {
    if (!row) return null;
    return {
        ...row,
        id: valueFor(row, 'id', 'decision_id'),
        personaId: valueFor(row, 'personaId', 'persona_id'),
        slotId: valueFor(row, 'slotId', 'slot_id') ?? null,
        decisionKey: valueFor(row, 'decisionKey', 'decision_key'),
        decisionType: valueFor(row, 'decisionType', 'decision_type'),
        status: row.status,
        runAt: valueFor(row, 'runAt', 'run_at') ?? null,
        expiresAt: valueFor(row, 'expiresAt', 'expires_at') ?? null,
        priority: Number(valueFor(row, 'priority', 'priority') || 0),
        preemptionMode: valueFor(row, 'preemptionMode', 'preemption_mode') || 'none',
        candidate: json(valueFor(row, 'candidate', 'candidate_json'), {}),
        rationale: json(valueFor(row, 'rationale', 'rationale_json'), {}),
        eventId: valueFor(row, 'eventId', 'event_id') ?? null,
        jobId: valueFor(row, 'jobId', 'job_id') ?? null,
        createdAt: valueFor(row, 'createdAt', 'created_at') ?? null,
        updatedAt: valueFor(row, 'updatedAt', 'updated_at') ?? null
    };
}

function rowSlot(row) {
    if (!row) return null;
    return {
        ...row,
        id: valueFor(row, 'id', 'slot_id'),
        slotId: valueFor(row, 'slotId', 'slot_id') ?? valueFor(row, 'id', 'slot_id'),
        personaId: valueFor(row, 'personaId', 'persona_id'),
        planDate: valueFor(row, 'planDate', 'plan_date'),
        slotKey: valueFor(row, 'slotKey', 'slot_key'),
        slotKind: valueFor(row, 'slotKind', 'slot_kind') || 'planned',
        startsAt: valueFor(row, 'startsAt', 'starts_at') ?? null,
        endsAt: valueFor(row, 'endsAt', 'ends_at') ?? null,
        status: row.status,
        source: row.source,
        priority: Number(row.priority || 0),
        constraints: json(valueFor(row, 'constraints', 'constraints_json'), {}),
        outcome: json(valueFor(row, 'outcome', 'outcome_json'), {})
    };
}

function personaIdFor(command) {
    return requiredText(command.personaId ?? command.persona_id, 'Timeline personaId', 160);
}

function personaFor(lookup, personaId, supplied) {
    if (!lookup) return supplied ?? {id: personaId};
    const row = sync(lookup(personaId), 'persona lookup');
    if (!row) throw new Error('Timeline persona does not exist');
    return row;
}

function localDateFor(at, timezone = 'UTC') {
    try {
        const parts = new Intl.DateTimeFormat('en-CA', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit'}).formatToParts(new Date(at));
        const values = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, part.value]));
        return `${values.year}-${values.month}-${values.day}`;
    } catch {
        return new Date(at).toISOString().slice(0, 10);
    }
}

function localHourFor(at, timezone = 'UTC') {
    try { return Number(new Intl.DateTimeFormat('en-US', {timeZone: timezone, hour: '2-digit', hour12: false}).format(new Date(at))); } catch { return new Date(at).getUTCHours(); }
}

function planDateFor(value, at, timezone = 'UTC') {
    if (value === undefined || value === null || value === '') return localDateFor(at, timezone);
    const date = requiredText(value, 'Timeline planDate', 32);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new TypeError('Timeline planDate must use YYYY-MM-DD');
    return date;
}

function focusTierFor(value, persona, atMs) {
    const raw = value?.tier ?? value?.focusTier ?? value ?? persona?.focusTier ?? persona?.focus_tier;
    if (['active', 'recent', 'idle'].includes(raw)) return raw;
    const engagedAt = value?.lastEngagedAt ?? persona?.lastEngagedAt ?? persona?.last_engaged_at;
    const elapsed = engagedAt ? atMs - Date.parse(engagedAt) : Number.POSITIVE_INFINITY;
    if (elapsed <= 30 * 60 * 1000) return 'active';
    if (elapsed <= 24 * 60 * 60 * 1000) return 'recent';
    return 'idle';
}

function budgetLimit(value, fallback = null) {
    if (Array.isArray(value)) return Number.isFinite(Number(value[1])) ? Math.max(0, Number(value[1])) : fallback;
    if (value === undefined || value === null || value === '') return fallback;
    return Number.isFinite(Number(value)) ? Math.max(0, Number(value)) : fallback;
}

function budgetFor(command, persona, blueprint) {
    const configured = command.budget ?? command.attentionBudget ?? blueprint?.attentionBudget?.dailyActivities;
    const object = isRecord(configured) ? configured : {};
    const limit = budgetLimit(object.limit ?? object.maximum ?? configured, null);
    const used = Number(object.used ?? command.budgetUsed ?? command.used ?? 0);
    return {kind: object.kind ?? 'dailyActivities', limit, used: Number.isFinite(used) ? Math.max(0, used) : 0, counted: object.counted === true};
}

function candidateValue(value) {
    if (!isRecord(value)) return null;
    const templateId = optionalText(value.templateId ?? value.template_id ?? value.id, 'Timeline candidate.templateId', 120);
    const family = optionalText(value.family ?? value.eventFamily ?? value.event_family, 'Timeline candidate.family', 80);
    const type = optionalText(value.type ?? value.eventType ?? family ?? 'personal_project', 'Timeline candidate.type', 80);
    const situation = optionalText(value.situation ?? value.title ?? value.summary, 'Timeline candidate.situation', 240);
    if (!templateId && !situation) return null;
    const duration = Array.isArray(value.durationMinutes) ? value.durationMinutes.map(Number).filter(Number.isFinite) : Number(value.durationMinutes);
    const sceneRef = value.sceneRef ?? value.scene_ref ?? (Array.isArray(value.sceneRefs) ? value.sceneRefs[0] : undefined);
    const recovery = optionalText(value.recovery, 'Timeline candidate.recovery', 240);
    return {
        ...value,
        ...(templateId ? {templateId} : {}),
        ...(family ? {family, eventFamily: family} : {}),
        type,
        ...(situation ? {situation} : {}),
        ...(sceneRef !== undefined ? {sceneRef} : {}),
        ...(recovery ? {recovery} : {}),
        ...(Number.isFinite(duration) ? {durationMinutes: duration} : Array.isArray(duration) && duration.length ? {durationMinutes: duration} : {}),
        priority: Number.isFinite(Number(value.priority)) ? Number(value.priority) : 0,
        preemptionMode: optionalText(value.preemptionMode, 'Timeline candidate.preemptionMode', 32) || 'none',
        reversible: value.reversible !== false
    };
}

function normalizeCandidates(command) {
    const values = command.candidates ?? (command.candidate === undefined ? [] : [command.candidate]);
    if (!Array.isArray(values)) throw new TypeError('Timeline candidates must be an array');
    return values.slice(0, MAX_CANDIDATES).map(candidateValue).filter(Boolean);
}

function deterministicIndex(key, length) {
    if (!length) return -1;
    let seed = 7;
    for (const character of key) seed = (seed * 31 + character.charCodeAt(0)) >>> 0;
    return seed % length;
}

function zonedInstant(planDate, clockText, timezone = 'UTC') {
    const match = /^(\d{2}):(\d{2})$/.exec(String(clockText || ''));
    if (!match || Number(match[1]) > 24 || Number(match[2]) > 59 || (Number(match[1]) === 24 && Number(match[2]) !== 0)) return null;
    const nextDate = Number(match[1]) === 24
        ? new Date(`${planDate}T00:00:00.000Z`).getTime() + 86_400_000
        : Date.UTC(Number(planDate.slice(0, 4)), Number(planDate.slice(5, 7)) - 1, Number(planDate.slice(8, 10)), Number(match[1]), Number(match[2]));
    const target = nextDate;
    let candidate = target;
    for (let index = 0; index < 3; index += 1) {
        const parts = new Intl.DateTimeFormat('en-US', {timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false}).formatToParts(new Date(candidate));
        const values = Object.fromEntries(parts.filter(part => part.type !== 'literal').map(part => [part.type, Number(part.value)]));
        const represented = Date.UTC(values.year, values.month - 1, values.day, values.hour === 24 ? 0 : values.hour, values.minute);
        candidate += target - represented;
    }
    return new Date(candidate).toISOString();
}

function normalizePlanSlot(value, index, planDate, timezone = 'UTC') {
    if (!isRecord(value)) return null;
    const title = optionalText(value.title ?? value.situation ?? value.label, 'Daily-plan slot.title', 160);
    let startsAt = value.startsAt ?? value.starts_at ?? null;
    let endsAt = value.endsAt ?? value.ends_at ?? null;
    const localTime = /^\d{2}:\d{2}$/;
    if (localTime.test(String(startsAt || ''))) startsAt = zonedInstant(planDate, startsAt, timezone);
    if (localTime.test(String(endsAt || ''))) endsAt = zonedInstant(planDate, endsAt, timezone);
    if (!title || !startsAt || !endsAt) return null;
    startsAt = timestamp(startsAt, 'Daily-plan slot.startsAt');
    endsAt = timestamp(endsAt, 'Daily-plan slot.endsAt');
    if (Date.parse(endsAt) <= Date.parse(startsAt)) return null;
    const slotKey = optionalText(value.slotKey ?? value.slot_key ?? value.id, 'Daily-plan slot.slotKey', 160) || `${planDate}:slot:${index}`;
    return {
        ...value,
        slotKey,
        slotKind: optionalText(value.slotKind ?? value.slot_kind, 'Daily-plan slot.slotKind', 60) || 'planned',
        title,
        situation: optionalText(value.situation ?? title, 'Daily-plan slot.situation', 240) || title,
        scene: optionalText(value.scene, 'Daily-plan slot.scene', 160) || '日常场景',
        sceneRef: optionalText(value.sceneRef ?? value.scene_ref, 'Daily-plan slot.sceneRef', 120),
        location: optionalText(value.location, 'Daily-plan slot.location', 160) || '',
        room: optionalText(value.room, 'Daily-plan slot.room', 120) || '',
        startsAt,
        endsAt,
        planDate,
        source: optionalText(value.source, 'Daily-plan slot.source', 80) || 'daily_plan',
        status: optionalText(value.status, 'Daily-plan slot.status', 40) || 'confirmed',
        priority: Number.isFinite(Number(value.priority)) ? Number(value.priority) : 0,
        constraints: isRecord(value.constraints) ? {...value.constraints} : {},
        outcome: isRecord(value.outcome) ? {...value.outcome} : {}
    };
}

function normalizeDailyPlan(plan, planDate) {
    const raw = plan?.timeline ?? plan?.plan?.timeline ?? plan?.items ?? plan?.plan?.items ?? [];
    if (!Array.isArray(raw)) throw new TypeError('Daily plan slots must be an array');
    const timezone = plan?.timezone ?? plan?.timeZone ?? plan?.plan?.timezone ?? 'UTC';
    const slots = raw.slice(0, MAX_SLOTS).map((item, index) => normalizePlanSlot(item, index, planDate, timezone)).filter(Boolean)
        .sort((left, right) => Date.parse(left.startsAt) - Date.parse(right.startsAt) || left.slotKey.localeCompare(right.slotKey));
    for (let index = 1; index < slots.length; index += 1) {
        if (Date.parse(slots[index].startsAt) < Date.parse(slots[index - 1].endsAt)) {
            throw new Error('Daily plan slots must not overlap');
        }
    }
    return slots;
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

export function createTimelineFlow({
    repositories,
    lifeEventFlow,
    clock,
    idGenerator,
    candidateSelector,
    transaction
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Timeline flow repositories must be an object');

    const personaRepository = resolveRepository(repositories, ['personaRepository', 'personas', 'persona'], 'persona repository', {optional: true});
    const decisionRepository = resolveRepository(repositories, ['eventDecisionRepository', 'timelineDecisionRepository', 'decisionRepository', 'eventDecision', 'decisions'], 'event-decision repository');
    const slotRepository = resolveRepository(repositories, ['timelineSlotRepository', 'timelineSlot', 'timelineSlots', 'timelineRepository', 'slots'], 'timeline-slot repository', {optional: true});
    const dailyPlanRepository = resolveRepository(repositories, ['dailyPlanRepository', 'dailyPlan', 'plan'], 'daily-plan repository', {optional: true});
    const blueprintRepository = resolveRepository(repositories, ['blueprintRepository', 'blueprint', 'lifeBlueprint'], 'blueprint repository', {optional: true});
    const lifeEventRepository = resolveRepository(repositories, ['lifeEventRepository', 'lifeEvent', 'life'], 'life-event repository', {optional: true});
    const focusRepository = resolveRepository(repositories, ['focusRepository', 'focus', 'attention'], 'focus repository', {optional: true});
    const budgetRepository = resolveRepository(repositories, ['budgetRepository', 'budget', 'attentionBudget'], 'budget repository', {optional: true});
    const lifeEvent = lifeEventFlow ?? repositories.lifeEventFlow;
    const personaLookup = personaRepository ? methodFor(personaRepository, ['findActive', 'findById', 'get'], 'persona repository', {optional: true}) : null;
    const decisionFind = methodFor(decisionRepository, ['findByDecisionKey', 'findByKey', 'find', 'get'], 'event-decision repository');
    const decisionInsert = methodFor(decisionRepository, ['insertDecision', 'createDecision', 'recordDecision', 'upsertDecision', 'insert'], 'event-decision repository', {optional: true});
    const decisionUpdate = methodFor(decisionRepository, ['updateDecision', 'markExecuted', 'completeDecision', 'update'], 'event-decision repository', {optional: true});
    const slotFind = slotRepository ? methodFor(slotRepository, ['findByKey', 'findSlot', 'getSlot'], 'timeline-slot repository', {optional: true}) : null;
    const slotList = slotRepository ? methodFor(slotRepository, ['list', 'listSlots', 'forPersona'], 'timeline-slot repository', {optional: true}) : null;
    const slotInsert = slotRepository ? methodFor(slotRepository, ['upsertSlot', 'insertSlot', 'createSlot'], 'timeline-slot repository', {optional: true}) : null;
    const slotUpdate = slotRepository ? methodFor(slotRepository, ['updateSlot', 'markStatus', 'update'], 'timeline-slot repository', {optional: true}) : null;
    const slotLink = slotRepository ? methodFor(slotRepository, ['linkEvents', 'linkEvent', 'createEventLink'], 'timeline-slot repository', {optional: true}) : null;
    const slotPrune = slotRepository ? methodFor(slotRepository, ['deleteGeneratedSlots', 'removeStaleSlots', 'pruneGeneratedSlots'], 'timeline-slot repository', {optional: true}) : null;
    const eventLink = slotRepository ? methodFor(slotRepository, ['createEventLink'], 'timeline-slot repository', {optional: true}) : null;
    const lifeEventList = lifeEventRepository ? methodFor(lifeEventRepository, ['list', 'listActive'], 'life-event repository', {optional: true}) : null;
    const planRead = dailyPlanRepository ? methodFor(dailyPlanRepository, ['read', 'findReady', 'find', 'get'], 'daily-plan repository', {optional: true}) : null;
    const blueprintRead = blueprintRepository ? methodFor(blueprintRepository, ['read', 'findCurrent', 'find', 'get'], 'blueprint repository', {optional: true}) : null;
    const focusRead = focusRepository ? methodFor(focusRepository, ['read', 'resolve', 'find', 'get'], 'focus repository', {optional: true}) : null;
    const budgetRead = budgetRepository ? methodFor(budgetRepository, ['read', 'count', 'resolve', 'find', 'get'], 'budget repository', {optional: true}) : null;
    const now = clockFunction(clock);
    const nextId = idFunction(idGenerator);
    if (!lifeEvent || !methodFor(lifeEvent, ['record', 'createEvent', 'insertEvent'], 'life-event flow', {optional: true})) {
        throw new TypeError('Timeline flow requires the generic life-event flow');
    }
    const lifeEventRecord = methodFor(lifeEvent, ['record', 'createEvent', 'insertEvent'], 'life-event flow');

    function existingDecision(personaId, decisionKey) {
        const result = decisionFind.length >= 2 ? decisionFind(personaId, decisionKey) : decisionFind({personaId, decisionKey});
        return rowDecision(sync(result, 'decision lookup'));
    }

    function findSlot(personaId, planDate, slotKey) {
        if (!slotFind || !slotKey) return null;
        const result = slotFind.length >= 2 ? slotFind(personaId, planDate, slotKey) : slotFind({personaId, planDate, slotKey});
        return rowSlot(sync(result, 'slot lookup'));
    }

    function policyFor(command, personaId, persona, blueprint, at) {
        const atMs = Date.parse(at);
        const focusValue = command.focus ?? command.focusTier ?? (focusRead ? sync(focusRead({personaId, persona, at}), 'focus lookup') : null);
        const budgetValue = command.budget ?? (budgetRead ? sync(budgetRead({personaId, persona, at}), 'budget lookup') : null);
        const focusTier = focusTierFor(focusValue, persona, atMs);
        const budget = budgetFor({...command, ...(budgetValue === null || budgetValue === undefined ? {} : {budget: budgetValue})}, persona, blueprint);
        const screened = command.screened !== undefined
            ? Boolean(command.screened)
            : Boolean(persona?.screened ?? persona?.screened_at ?? persona?.screenedAt);
        const reasons = [];
        if (focusTier === 'idle' && command.allowIdle !== true) reasons.push('not_recently_engaged');
        if (budget.limit !== null && budget.used >= budget.limit && command.ignoreBudget !== true) reasons.push('daily_budget');
        if (screened && command.allowScreened !== true) reasons.push('screened');
        return {allowed: reasons.length === 0, reasons, reason: reasons[0] ?? null, focusTier, budget, screened};
    }

    function chooseCandidate(candidates, command, decisionKey) {
        if (command.candidate === null || command.noEvent === true) return null;
        if (typeof candidateSelector === 'function') {
            const selected = sync(candidateSelector(candidates, {command, decisionKey}), 'candidate selector');
            return typeof selected === 'number' ? candidates[selected] ?? null : candidateValue(selected);
        }
        if (command.selectedCandidate !== undefined) return candidateValue(command.selectedCandidate);
        return candidates[deterministicIndex(decisionKey, candidates.length)] ?? null;
    }

    function decisionShape(command, personaId, at, persona, blueprint) {
        const planDate = planDateFor(command.planDate ?? command.plan_date, at, blueprint?.timezone);
        const bucket = optionalText(command.bucket ?? command.timeBucket, 'Timeline bucket', 80) || String(localHourFor(at, blueprint?.timezone)).padStart(2, '0');
        const slotKey = optionalText(command.slotKey ?? command.slot_key, 'Timeline slotKey', 160);
        const decisionKey = requiredText(command.decisionKey ?? command.decision_key ?? `${personaId}:${planDate}:${slotKey || 'opportunity'}:${bucket}`, 'Timeline decisionKey', 240);
        const candidates = normalizeCandidates(command);
        const policy = policyFor(command, personaId, persona, blueprint, at);
        const chosen = policy.allowed ? chooseCandidate(candidates, command, decisionKey) : null;
        const noEvent = !chosen;
        const rationale = {
            ...(isRecord(command.rationale) ? command.rationale : {}),
            ...(noEvent ? {reason: policy.reason || (candidates.length ? 'no_event_selected' : 'no_eligible_candidate')} : {reason: 'eligible_template_selected'}),
            candidateCount: candidates.length,
            focusTier: policy.focusTier,
            budget: policy.budget,
            screened: policy.screened,
            ...(policy.reasons.length ? {policyReasons: policy.reasons} : {})
        };
        return {
            personaId,
            planDate,
            decisionKey,
            slotKey,
            at,
            causationId: command.causationId ?? command.sourceMessageId ?? command.source_message_id ?? null,
            jobId: command.jobId ?? command.job_id ?? null,
            candidate: chosen,
            candidates,
            policy,
            decisionType: noEvent ? 'no_event' : 'start_event',
            status: noEvent ? 'suppressed' : 'accepted',
            runAt: timestamp(command.runAt ?? at, 'Timeline decision.runAt'),
            expiresAt: command.expiresAt ? timestamp(command.expiresAt, 'Timeline decision.expiresAt') : null,
            priority: Number.isFinite(Number(chosen?.priority ?? command.priority)) ? Number(chosen?.priority ?? command.priority) : 0,
            preemptionMode: optionalText(chosen?.preemptionMode ?? command.preemptionMode, 'Timeline decision.preemptionMode', 32) || 'none',
            rationale
        };
    }

    function plan(input = {}) {
        if (!isRecord(input)) throw new TypeError('Timeline command must be an object');
        const personaId = personaIdFor(input);
        const at = timestamp(input.at ?? now(), 'Timeline evaluation time');
        const persona = personaFor(personaLookup, personaId, input.persona);
        const blueprint = blueprintRead ? sync(blueprintRead({personaId}), 'blueprint lookup') : input.blueprint ?? {};
        const shape = decisionShape(input, personaId, at, persona, blueprint ?? {});
        const existing = existingDecision(personaId, shape.decisionKey);
        const resumable = existing && ['accepted', 'proposed'].includes(existing.status)
            && existing.decisionType === 'start_event' && !existing.eventId && existing.candidate;
        if (existing && !resumable) {
            const replay = {
                type: 'timeline_decision_plan',
                version: TIMELINE_FLOW_VERSION,
                personaId,
                planDate: shape.planDate,
                decisionKey: shape.decisionKey,
                decisionId: existing.id,
                eventId: existing.eventId,
                slotId: existing.slotId,
                decision: existing,
                candidate: existing.candidate,
                status: existing.status,
                decisionType: existing.decisionType,
                noEvent: existing.decisionType === 'no_event',
                replayed: true,
                previewResult: {decisionId: existing.id, eventId: existing.eventId, noEvent: existing.decisionType === 'no_event', status: existing.status}
            };
            const frozen = deepFreeze(replay);
            PRIVATE_PLANS.set(frozen, {shape, existing});
            return frozen;
        }
        const effectiveShape = resumable ? {
            ...shape,
            at: existing.runAt ?? shape.at,
            runAt: existing.runAt ?? shape.runAt,
            expiresAt: existing.expiresAt ?? shape.expiresAt,
            candidate: candidateValue(existing.candidate),
            candidates: [candidateValue(existing.candidate)].filter(Boolean),
            decisionType: 'start_event',
            status: existing.status,
            priority: existing.priority,
            preemptionMode: existing.preemptionMode,
            rationale: existing.rationale
        } : shape;
        const slot = findSlot(personaId, effectiveShape.planDate, effectiveShape.slotKey);
        const decisionId = existing?.id ?? nextId('decision');
        const eventId = effectiveShape.candidate ? (existing?.eventId ?? nextId('event')) : null;
        const preview = {
            decisionId,
            eventId,
            slotId: slot?.id ?? input.slotId ?? effectiveShape.candidate?.slotId ?? null,
            noEvent: effectiveShape.decisionType === 'no_event',
            status: effectiveShape.status,
            decisionType: effectiveShape.decisionType,
            focusTier: effectiveShape.policy.focusTier,
            screened: effectiveShape.policy.screened,
            budget: effectiveShape.policy.budget
        };
        const result = {
            type: 'timeline_decision_plan',
            version: TIMELINE_FLOW_VERSION,
            personaId,
            planDate: shape.planDate,
            decisionKey: shape.decisionKey,
            decisionId,
            eventId,
            slotId: preview.slotId,
            candidate: effectiveShape.candidate,
            candidates: effectiveShape.candidates,
            decisionType: effectiveShape.decisionType,
            status: effectiveShape.status,
            noEvent: preview.noEvent,
            policy: shape.policy,
            rationale: shape.rationale,
            jobId: shape.jobId,
            preallocatedIds: {decisionId, ...(eventId ? {eventId} : {})},
            previewResult: preview,
            replayed: false
        };
        const frozen = deepFreeze(result);
        PRIVATE_PLANS.set(frozen, {shape: effectiveShape, existing: resumable ? null : null, resume: Boolean(resumable), persona, decision: resumable ? existing : null});
        return frozen;
    }

    function applyWithin(planValue) {
        const privateState = PRIVATE_PLANS.get(planValue);
        if (!privateState) throw new TypeError('Timeline decision plan is invalid');
        if ((privateState.existing || planValue.replayed) && !privateState.resume) {
            return resultEnvelope({
                type: 'timeline_decision_result',
                version: TIMELINE_FLOW_VERSION,
                personaId: planValue.personaId,
                decisionId: planValue.decisionId,
                eventId: planValue.eventId ?? null,
                slotId: planValue.slotId ?? null,
                status: planValue.status,
                decisionType: planValue.decisionType,
                noEvent: planValue.noEvent,
                replayed: true,
                decision: privateState.existing ?? planValue.decision
            });
        }
        const {shape} = privateState;
        const createdAt = shape.at;
        const decisionInput = {
            id: planValue.decisionId,
            personaId: planValue.personaId,
            slotId: planValue.slotId,
            decisionKey: planValue.decisionKey,
            decisionType: planValue.decisionType,
            status: planValue.status,
            runAt: shape.runAt,
            expiresAt: shape.expiresAt,
            priority: shape.priority,
            preemptionMode: shape.preemptionMode,
            candidate: shape.candidate ?? {kind: 'no_event'},
            rationale: shape.rationale,
            jobId: shape.jobId,
            createdAt,
            updatedAt: createdAt
        };
        let decisionRow;
        if (privateState.resume) {
            decisionRow = privateState.decision;
        } else if (decisionInsert) {
            try {
                decisionRow = sync(decisionInsert(decisionInput), 'decision insert');
            } catch (error) {
                // A concurrent evaluator may have won the unique decision key
                // between plan() and apply(). Re-read and return that durable
                // decision instead of creating a second life fact.
                const replay = existingDecision(planValue.personaId, planValue.decisionKey);
                if (!replay) throw error;
                return resultEnvelope({
                    type: 'timeline_decision_result',
                    version: TIMELINE_FLOW_VERSION,
                    personaId: planValue.personaId,
                    decisionId: replay.id,
                    eventId: replay.eventId ?? null,
                    slotId: replay.slotId ?? planValue.slotId ?? null,
                    decision: replay,
                    status: replay.status,
                    decisionType: replay.decisionType,
                    noEvent: replay.decisionType === 'no_event',
                    replayed: true,
                    projections: [{type: 'timeline_decision', decision: replay}],
                    presentation: []
                });
            }
        } else {
            decisionRow = decisionInput;
        }
        decisionRow = rowDecision(decisionRow) ?? rowDecision(decisionInput);
        if (shape.decisionType === 'no_event') {
            return resultEnvelope({
                type: 'timeline_decision_result',
                version: TIMELINE_FLOW_VERSION,
                personaId: planValue.personaId,
                decisionId: planValue.decisionId,
                eventId: null,
                slotId: planValue.slotId ?? null,
                decision: decisionRow,
                status: 'suppressed',
                decisionType: 'no_event',
                noEvent: true,
                replayed: false,
                facts: [],
                projections: [{type: 'timeline_decision', decision: decisionRow}],
                presentation: [{type: 'timeline_decision', decisionId: planValue.decisionId, noEvent: true, reason: shape.rationale.reason}]
            });
        }
        if (!lifeEventRecord) throw new TypeError('Timeline flow requires life-event flow record()');
        const candidate = shape.candidate;
        const duration = Array.isArray(candidate.durationMinutes)
            ? Number(candidate.durationMinutes[0])
            : Number(candidate.durationMinutes);
        const resolvesAt = Number.isFinite(duration) && duration > 0
            ? new Date(Date.parse(shape.at) + Math.min(duration, 24 * 60) * 60_000).toISOString()
            : candidate.resolvesAt ?? null;
        const eventResult = sync(lifeEventRecord({
            id: planValue.eventId,
            eventId: planValue.eventId,
            personaId: planValue.personaId,
            type: candidate.type || candidate.eventFamily || 'personal_project',
            occurredAt: shape.at,
            resolvesAt,
            causationId: shape.causationId ?? null,
            idempotencyKey: planValue.decisionKey,
            source: 'timeline',
            rationale: shape.rationale.reason,
            situation: candidate.situation || candidate.title || '正在忙自己的事',
            mood: candidate.mood || '平静',
            scene: candidate.scene || '日常场景',
            sceneRef: candidate.sceneRef,
            appearance: candidate.appearance,
            eventFamily: candidate.family || candidate.eventFamily,
            templateId: candidate.templateId,
            priority: shape.priority,
            preemptionMode: shape.preemptionMode,
            reversible: candidate.reversible,
            recovery: candidate.recovery,
            decisionId: planValue.decisionId,
            slotId: planValue.slotId,
            publish: false,
            requestActivityDecision: false
        }), 'life-event creation');
        const eventId = eventResult?.eventId ?? eventResult?.event?.id ?? planValue.eventId;
        if (decisionUpdate) sync(decisionUpdate({id: planValue.decisionId, decisionId: planValue.decisionId, personaId: planValue.personaId, status: 'executed', eventId, updatedAt: shape.at}), 'decision update');
        if (slotUpdate && planValue.slotId) sync(slotUpdate({id: planValue.slotId, slotId: planValue.slotId, personaId: planValue.personaId, status: 'active', outcome: {eventId, decisionId: planValue.decisionId}, updatedAt: shape.at}), 'slot update');
        if (slotLink && planValue.slotId && eventId) sync(slotLink({personaId: planValue.personaId, slotId: planValue.slotId, eventId, decisionId: planValue.decisionId}), 'slot event link');
        if (eventLink && eventId && lifeEventList) {
            const rows = sync(lifeEventList({personaId: planValue.personaId, limit: 20}), 'timeline event lookup') || [];
            const previous = rows.find(row => (row.id ?? row.eventId ?? row.event_id) !== eventId);
            if (previous?.id ?? previous?.eventId ?? previous?.event_id) {
                sync(eventLink({
                    personaId: planValue.personaId,
                    fromEventId: previous.id ?? previous.eventId ?? previous.event_id,
                    toEventId: eventId,
                    linkType: 'follows',
                    metadata: {decisionId: planValue.decisionId, templateId: candidate.templateId ?? null},
                    createdAt: shape.at
                }), 'timeline event link');
            }
        }
        const event = eventResult?.event ?? eventResult;
        return resultEnvelope({
            type: 'timeline_decision_result',
            version: TIMELINE_FLOW_VERSION,
            personaId: planValue.personaId,
            decisionId: planValue.decisionId,
            eventId,
            slotId: planValue.slotId ?? null,
            decision: {...decisionRow, status: 'executed', eventId},
            event,
            status: 'executed',
            decisionType: 'start_event',
            noEvent: false,
            replayed: false,
            facts: [{type: 'life_event', event}, {type: 'timeline_decision_executed', decisionId: planValue.decisionId, eventId}],
            projections: [{type: 'timeline_decision', decisionId: planValue.decisionId, status: 'executed', eventId}, ...(planValue.slotId ? [{type: 'timeline_slot', slotId: planValue.slotId, status: 'active', eventId}] : [])],
            effects: [{effectId: `effect_timeline_activity_${planValue.decisionId}`, kind: 'timeline.activity_decision', capability: 'timeline', idempotencyKey: `timeline:${planValue.decisionId}`, causationId: planValue.decisionId, payload: {eventId, decisionId: planValue.decisionId}}],
            presentation: [{type: 'timeline_event', eventId, decisionId: planValue.decisionId}]
        });
    }

    function apply(planValue, options = {}) {
        if (!isRecord(options) && typeof options !== 'function') throw new TypeError('Timeline apply options must be an object');
        const runner = typeof options === 'function' ? options : options.transaction ?? options.commit ?? transaction;
        return transactionRunner(runner, () => applyWithin(planValue));
    }

    function syncDailyPlanSlots(input = {}) {
        if (!isRecord(input)) throw new TypeError('Daily-plan sync command must be an object');
        const personaId = personaIdFor(input);
        const at = timestamp(input.at ?? now(), 'Daily-plan sync time');
        const blueprint = blueprintRead ? sync(blueprintRead({personaId}), 'blueprint lookup') : {};
        const planDate = planDateFor(input.planDate ?? input.plan_date, at, blueprint?.timezone);
        const plan = input.plan ?? (planRead ? sync(planRead({personaId, planDate, at: input.at}), 'daily-plan lookup') : null);
        if (!plan) return resultEnvelope({type: 'daily_plan_slots', personaId, planDate, slots: [], skipped: 'plan_missing'});
        if (plan.status && plan.status !== 'ready' && input.allowUnready !== true) return resultEnvelope({type: 'daily_plan_slots', personaId, planDate, slots: [], skipped: 'plan_not_ready'});
        const slots = normalizeDailyPlan({...plan, timezone: plan.timezone ?? plan.timeZone ?? blueprint?.timezone}, planDate);
        if (!slotInsert) throw new TypeError('Timeline flow requires a timeline-slot upsert port to sync daily plans');
        const persist = () => {
            if (slotPrune) sync(slotPrune({personaId, planDate, slotKeys: slots.map(slot => slot.slotKey)}), 'daily-plan stale-slot prune');
            const persisted = slots.map(slot => rowSlot(sync(slotInsert({
                id: slot.id ?? nextId('slot'),
                personaId,
                planDate,
                slotKey: slot.slotKey,
                slotKind: slot.slotKind,
                title: slot.title,
                situation: slot.situation,
                scene: slot.scene,
                sceneRef: slot.sceneRef,
                location: slot.location,
                room: slot.room,
                startsAt: slot.startsAt,
                endsAt: slot.endsAt,
                status: slot.status,
                source: slot.source,
                priority: slot.priority,
                constraints: slot.constraints,
                outcome: slot.outcome,
                planRevision: plan.revision ?? plan.planRevision ?? null,
                updatedAt: input.updatedAt ?? now(),
                createdAt: input.createdAt ?? now()
            }), 'daily-plan slot upsert')));
            return resultEnvelope({
                type: 'daily_plan_slots', personaId, planDate, slots: persisted,
                facts: [],
                projections: persisted.map(slot => ({type: 'timeline_slot', slot})),
                presentation: [{type: 'daily_plan_slots', personaId, planDate, count: persisted.length}]
            });
        };
        return transactionRunner(input.transaction ?? transaction, persist);
    }

    function advanceSlots(input = {}) {
        if (!isRecord(input)) throw new TypeError('Timeline advancement command must be an object');
        const personaId = personaIdFor(input);
        const at = timestamp(input.at ?? now(), 'Timeline advancement time');
        if (!slotList || !slotUpdate) return resultEnvelope({type: 'timeline_slots_advanced', personaId, at, slots: []});
        const advance = () => {
            const rows = sync(slotList({personaId, at}), 'slot list') || [];
            const changed = [];
            for (const raw of rows) {
                const slot = rowSlot(raw);
                let status = slot.status;
                let reason = null;
                if (slot.status === 'active' && slot.endsAt && Date.parse(slot.endsAt) <= Date.parse(at)) {
                    status = 'completed'; reason = 'ended_at_boundary';
                } else if (slot.status === 'confirmed' && slot.endsAt && Date.parse(slot.endsAt) <= Date.parse(at)) {
                    status = 'skipped'; reason = 'expired_before_execution';
                } else if (slot.status === 'confirmed' && slot.startsAt && Date.parse(slot.startsAt) <= Date.parse(at) && (!slot.endsAt || Date.parse(slot.endsAt) > Date.parse(at))) {
                    status = 'active';
                }
                if (status !== slot.status) {
                    const updated = sync(slotUpdate({id: slot.id, slotId: slot.id, personaId, status, outcome: {...slot.outcome, ...(reason ? {reason} : {})}, updatedAt: at}), 'slot status update');
                    changed.push(rowSlot(updated) ?? {...slot, status});
                }
            }
            return resultEnvelope({type: 'timeline_slots_advanced', personaId, at, slots: changed, projections: changed.map(slot => ({type: 'timeline_slot', slot}))});
        };
        return transactionRunner(input.transaction ?? transaction, advance);
    }

    async function handleJob(job, context = {}) {
        const raw = job?.payload ?? job?.payload_json;
        const payload = isRecord(raw) ? raw : json(raw, {});
        const type = job?.jobType ?? job?.job_type ?? payload.type ?? 'timeline_candidate';
        if (type === 'daily_plan') return syncDailyPlanSlots({personaId: job.personaId ?? job.persona_id, planDate: payload.planDate ?? payload.plan_date, plan: payload.plan, at: context.now});
        if (type === 'timeline_reconcile' || type === 'timeline_slots') return advanceSlots({personaId: job.personaId ?? job.persona_id, at: context.now});
        const command = {...payload, personaId: payload.personaId ?? job?.personaId ?? job?.persona_id, jobId: payload.jobId ?? payload.job_id ?? job?.id ?? job?.jobId, at: payload.at ?? context.now};
        const planned = plan(command);
        return apply(planned);
    }

    function evaluate(command) {
        const planned = plan(command);
        return apply(planned);
    }

    const flow = {
        version: TIMELINE_FLOW_VERSION,
        plan,
        decide: plan,
        evaluate,
        execute: evaluate,
        run: evaluate,
        apply,
        syncDailyPlanSlots,
        ensureDailyPlanSlots: syncDailyPlanSlots,
        advanceSlots,
        reconcile: advanceSlots,
        handleJob
    };
    return Object.freeze(flow);
}

export const createTimelineApplicationFlow = createTimelineFlow;
export const createLifeTimelineFlow = createTimelineFlow;
export default createTimelineFlow;
