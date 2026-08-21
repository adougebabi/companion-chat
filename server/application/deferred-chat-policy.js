/**
 * Application policy for chat turns that intersect a sleeping life-world.
 *
 * The policy is intentionally transport and storage agnostic. Repositories
 * create/merge durable batches and enqueue their job through ports; chat flow
 * facts are committed by the normal conversation commit boundary.
 */

export const DEFERRED_CHAT_POLICY_VERSION = 1;
export const DEFERRED_CHAT_JOB_TYPE = 'deferred_chat_reply';
export const ASSISTANT_MESSAGE_FACT_TYPE = 'conversation.assistant_message';
export const DEFERRED_BATCH_ACTIVE_STATUSES = Object.freeze(['queued', 'leased', 'processing']);

const TRUSTED_TIME_PATTERN = /(?:什么时候|啥时候|何时|几点|多会儿|多会).{0,8}(?:下课|结束|忙完|完成)|(?:下课|结束).{0,8}(?:时间|时候|啥时候|几点)/;
const MAX_REPLY_LENGTH = 12_000;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function callable(value, names, field, {optional = false} = {}) {
    if (typeof value === 'function') return value;
    if (isRecord(value)) {
        for (const name of names) {
            if (typeof value[name] === 'function') return value[name].bind(value);
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${names.join('() or ')}()`);
}

function text(value, field, {allowEmpty = false, max = 240} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const normalized = value.trim();
    if (!allowEmpty && !normalized) throw new TypeError(`${field} must not be empty`);
    if (normalized.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    return normalized;
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function clockFor(clock) {
    const read = callable(clock ?? (() => new Date().toISOString()), ['now'], 'Deferred chat clock');
    return () => timestamp(read(), 'Deferred chat clock value');
}

function idFor(id) {
    if (typeof id === 'function') return id;
    if (isRecord(id) && typeof id.next === 'function') return id.next.bind(id);
    let sequence = 0;
    return prefix => `${prefix}_${Date.now()}_${sequence++}`;
}

function valueOf(value, camel, snake, fallback = undefined) {
    if (value?.[camel] !== undefined) return value[camel];
    if (value?.[snake] !== undefined) return value[snake];
    return fallback;
}

function parseDate(value) {
    const date = new Date(value);
    return Number.isFinite(date.getTime()) ? date : null;
}

function parseJson(value, fallback) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed ?? fallback;
    } catch {
        return fallback;
    }
}

function stateValue(state, camel, snake, fallback = undefined) {
    if (state?.[camel] !== undefined) return state[camel];
    if (state?.[snake] !== undefined) return state[snake];
    if (isRecord(state?.source) && state.source[camel] !== undefined) return state.source[camel];
    return fallback;
}

function timezoneFor(state, explicit) {
    return String(explicit || stateValue(state, 'timezone', 'timezone', '') || 'Asia/Shanghai');
}

function formatTime(value, timeZone) {
    const date = parseDate(value);
    if (!date) return '';
    try {
        return new Intl.DateTimeFormat('zh-CN', {
            hour: '2-digit', minute: '2-digit', hour12: false, timeZone
        }).format(date);
    } catch {
        return new Intl.DateTimeFormat('zh-CN', {hour: '2-digit', minute: '2-digit', hour12: false}).format(date);
    }
}

function normalizeTrustedTimeState(state = {}) {
    const source = stateValue(state, 'resolvedSource', 'resolved_source', stateValue(state, 'source', 'source', 'unknown'));
    const situation = String(stateValue(state, 'situation', 'situation', '') || '');
    const endsAt = stateValue(state, 'resolvedEndsAt', 'resolved_ends_at', stateValue(state, 'endsAt', 'ends_at', null));
    const timeFact = stateValue(state, 'resolvedTimeFact', 'resolved_time_fact', stateValue(state, 'timeFact', 'time_fact', endsAt ? 'known' : 'unknown'));
    return {source: typeof source === 'string' ? source : source?.kind || 'unknown', situation, endsAt, timeFact};
}

/**
 * Return the trusted time facts used by both the chat short-circuit and any
 * deferred composer. No timestamp is inferred from the user's wording.
 */
export function trustedTimeFacts({text: inputText, state = {}, timeZone} = {}) {
    const sourceText = String(inputText || '').trim();
    if (!TRUSTED_TIME_PATTERN.test(sourceText)) return null;

    const resolved = normalizeTrustedTimeState(state);
    const zone = timezoneFor(state, timeZone);
    const endText = resolved.timeFact === 'known' ? formatTime(resolved.endsAt, zone) : '';
    const isLesson = /上课|课程|课堂|老师/.test(resolved.situation);
    let reply;
    if (!isLesson) {
        if (resolved.source === 'daily_plan_baseline' && endText) {
            reply = `我现在不在上课，${resolved.situation || '正在休息'}，${endText}后才会开始下一项安排。`;
        } else if (endText) {
            reply = `我现在不在上课，${resolved.situation || '正在按自己的节奏安排'}，这段安排预计到${endText}结束。`;
        } else {
            reply = `我现在没有课程或可确认的结束时间，${resolved.situation || '正在按自己的节奏休息'}。`;
        }
    } else if (endText) {
        reply = `我这段课程安排预计到${endText}结束。`;
    } else {
        reply = '我现在没有可确认的下课时间，先按眼前的安排来。';
    }

    return Object.freeze({
        source: resolved.source,
        situation: resolved.situation,
        endsAt: resolved.endsAt || null,
        timeFact: resolved.timeFact === 'known' && endText ? 'known' : 'unknown',
        timeZone: zone,
        reply: reply.slice(0, MAX_REPLY_LENGTH),
        fact: Object.freeze({
            type: 'life.trusted_time_fact',
            source: resolved.source,
            situation: resolved.situation,
            endsAt: resolved.endsAt || null,
            timeFact: resolved.timeFact === 'known' && endText ? 'known' : 'unknown',
            timeZone: zone
        })
    });
}

export const trustedTimeReplyForMessage = trustedTimeFacts;
export const resolveTrustedTimeFacts = trustedTimeFacts;

function defaultSleepAvailability({state = {}, at} = {}) {
    const situation = String(stateValue(state, 'situation', 'situation', '') || '');
    const source = stateValue(state, 'resolvedSource', 'resolved_source', stateValue(state, 'source', 'source', ''));
    const sleeping = state?.sleeping === true
        || state?.isSleeping === true
        || (source === 'daily_plan_baseline' && /睡|赖床|自然醒|起床前/.test(situation));
    if (!sleeping) return {sleeping: false, immediate: true};
    return {
        sleeping: true,
        immediate: state?.immediate === true,
        intimacy: Number.isFinite(state?.intimacy) ? state.intimacy : 0,
        draw: Number.isFinite(state?.draw) ? state.draw : 0,
        nextBoundaryAt: stateValue(state, 'nextBoundaryAt', 'next_boundary_at', stateValue(state, 'resolvedNextBoundaryAt', 'resolved_next_boundary_at', null)),
        timezone: timezoneFor(state)
    };
}

function availabilityFor(source) {
    return callable(source, ['resolve', 'read', 'sleepAvailability'], 'Sleep availability port');
}

function activeBatchReader(source) {
    return callable(source, ['findActive', 'findActiveForPersona', 'active'], 'Deferred batch active reader', {optional: true});
}

function batchAppender(source) {
    return callable(source, ['appendMessage', 'append', 'mergeMessage'], 'Deferred batch append port', {optional: true});
}

function batchCreator(source) {
    return callable(source, ['create', 'createBatch', 'enqueue'], 'Deferred batch create port', {optional: true});
}

function conversationFor(source) {
    return callable(source, ['getConversation', 'findConversation'], 'Conversation repository', {optional: true});
}

function batchId(value) {
    return value?.id ?? value?.batchId ?? value?.batch_id ?? null;
}

function batchStatus(value) {
    return valueOf(value, 'status', 'status', null);
}

function messageIntent({id, text: messageText, createdAt}) {
    return {
        id,
        role: 'assistant',
        text: messageText,
        attachments: [],
        generation: null,
        jobs: [],
        createdAt,
        readAt: null,
        proactiveEventId: null,
        proactivePendingEventId: null
    };
}

function policyResult({kind, result = {}, facts = [], messages = []} = {}) {
    return Object.freeze({
        handled: true,
        kind,
        result: Object.freeze(result),
        facts: Object.freeze(facts),
        projections: Object.freeze([]),
        effects: Object.freeze([]),
        presentation: Object.freeze([]),
        chatResult: Object.freeze({type: 'done', messages, message: messages[0] || null, learned: [], jobs: []})
    });
}

function atFor(input, now) {
    const value = input.chatAt ?? input.at ?? input.context?.chatAt ?? input.context?.now ?? now();
    return timestamp(value, 'Deferred chat policy time');
}

function conversationIdFor(message, repository, personaId) {
    const direct = valueOf(message, 'conversationId', 'conversation_id', null);
    if (direct) return direct;
    const conversation = repository?.(personaId);
    return valueOf(conversation, 'id', 'conversation_id', null);
}

function deliveryAtFor(at, availability) {
    const explicit = parseDate(availability?.deliverAt);
    if (explicit && explicit.getTime() > Date.parse(at)) return explicit.toISOString();
    const boundary = parseDate(availability?.nextBoundaryAt);
    if (boundary && boundary.getTime() > Date.parse(at)) return boundary.toISOString();
    return new Date(Date.parse(at) + 5 * 60_000).toISOString();
}

function batchKeyFor(personaId, at, availability) {
    if (typeof availability?.batchKey === 'string' && availability.batchKey.trim()) return availability.batchKey.trim();
    return `${personaId}:${at.slice(0, 13)}:sleep`;
}

/**
 * Compose the chat preflight policy. The policy can be used as a flow step or
 * directly by a route adapter; both forms return the same short-circuit DTO.
 */
export function createDeferredChatPolicy({
    deferredBatch,
    batchRepository,
    jobRepository,
    conversationRepository,
    lifeWorld,
    sleepAvailability,
    clock,
    idGenerator,
    timezone,
    replyFallback = '刚刚看到你的消息了。'
} = {}) {
    const batches = deferredBatch ?? batchRepository;
    const active = activeBatchReader(batches);
    const append = batchAppender(batches);
    const create = batchCreator(batches);
    const readConversation = conversationFor(conversationRepository);
    const readWorld = callable(lifeWorld, ['read', 'readState', 'contextFor'], 'Deferred chat life-world port', {optional: true});
    const resolveSleep = sleepAvailability ? availabilityFor(sleepAvailability) : defaultSleepAvailability;
    const now = clockFor(clock);
    const nextId = idFor(idGenerator);
    const enqueue = callable(jobRepository, ['enqueue', 'create', 'enqueueJob'], 'Deferred chat job port', {optional: true});

    async function activeBatchFor(personaId, at) {
        if (!active) return null;
        return active({personaId, at, statuses: DEFERRED_BATCH_ACTIVE_STATUSES.slice()});
    }

    async function appendToBatch(batch, {personaId, messageId, at}) {
        if (!append || !batch || !messageId) return batch;
        return append({personaId, batchId: batchId(batch), messageId, at, status: batchStatus(batch)});
    }

    async function createDeferredBatch({personaId, userMessage, at, availability}) {
        if (!create) throw new Error('Deferred chat batch create port is unavailable');
        const conversationId = conversationIdFor(userMessage, readConversation, personaId);
        if (!conversationId) throw new Error('Deferred chat conversation is unavailable');
        const id = nextId('deferred_chat');
        const deliverAt = deliveryAtFor(at, availability);
        const key = batchKeyFor(personaId, at, availability);
        const input = {
            id,
            personaId,
            conversationId,
            batchKey: key,
            status: 'queued',
            deliverAt,
            decision: {
                intimacy: Number.isFinite(availability?.intimacy) ? availability.intimacy : 0,
                draw: Number.isFinite(availability?.draw) ? availability.draw : 0,
                reason: 'sleep_deferred'
            },
            messageIds: userMessage?.id ? [userMessage.id] : [],
            createdAt: at,
            updatedAt: at,
            job: {
                id: nextId('job'),
                jobType: DEFERRED_CHAT_JOB_TYPE,
                personaId,
                priority: 5,
                runAfter: deliverAt,
                maxAttempts: 4,
                payload: {batchId: id}
            }
        };
        const created = await create(input);
        // A simple repository may return the batch but expose job enqueue as a
        // separate port. Production adapters create both under one boundary.
        if (enqueue && created?.created !== false && !created?.job && !created?.jobId) {
            const job = await enqueue(input.job);
            return {batch: created, job};
        }
        return {batch: created?.batch ?? created, job: created?.job ?? null};
    }

    async function worldFor(personaId, at, input) {
        if (!readWorld) return input.state ?? input.context?.state ?? {};
        return readWorld({personaId, at, command: input.command ?? input, messages: input.messages ?? []});
    }

    async function evaluate(input = {}) {
        if (!isRecord(input)) throw new TypeError('Deferred chat policy input must be an object');
        const personaId = text(input.personaId ?? input.command?.personaId, 'Deferred chat personaId', {max: 160});
        const userMessage = input.userMessage ?? input.message ?? null;
        const messageId = valueOf(userMessage, 'id', 'message_id', null);
        const at = atFor(input, now);
        const state = await worldFor(personaId, at, input);
        const existing = await activeBatchFor(personaId, at);
        if (existing && batchStatus(existing) && !DEFERRED_BATCH_ACTIVE_STATUSES.includes(batchStatus(existing))) {
            // A repository may return a wider set than the port contract.
        } else if (existing) {
            const merged = await appendToBatch(existing, {personaId, messageId, at});
            return policyResult({
                kind: 'deferred',
                result: {suppressed: 'active_batch', batchId: batchId(existing), merged: Boolean(merged)},
                messages: []
            });
        }

        const availability = await resolveSleep({personaId, at, state, command: input.command ?? input, userMessage});
        if (availability?.sleeping === true && availability?.immediate !== true) {
            const created = await createDeferredBatch({personaId, userMessage, at, availability});
            return policyResult({
                kind: 'deferred',
                result: {suppressed: 'sleep', batchId: batchId(created?.batch ?? created), deliverAt: created?.batch?.deliver_at ?? created?.batch?.deliverAt ?? deliveryAtFor(at, availability), jobId: created?.job?.id ?? created?.jobId ?? null},
                messages: []
            });
        }

        const trusted = trustedTimeFacts({text: input.text ?? input.command?.text, state, timeZone: timezone});
        if (trusted) {
            const assistant = messageIntent({id: nextId('message'), text: trusted.reply || replyFallback, createdAt: at});
            return policyResult({
                kind: 'trusted_time',
                result: {trustedTime: trusted.fact},
                facts: [{type: trusted.fact.type, ...trusted.fact}, {type: ASSISTANT_MESSAGE_FACT_TYPE, personaId, messages: [assistant]}],
                messages: [assistant]
            });
        }
        return Object.freeze({handled: false, kind: 'continue', facts: [], projections: [], effects: [], presentation: []});
    }

    return Object.freeze({
        version: DEFERRED_CHAT_POLICY_VERSION,
        evaluate,
        apply: evaluate,
        run: evaluate,
        activeBatchFor,
        trustedTimeFacts,
        sleepAvailability: resolveSleep
    });
}

export const createChatDeferredPolicy = createDeferredChatPolicy;
export const createDeferredReplyPolicy = createDeferredChatPolicy;

export default createDeferredChatPolicy;
