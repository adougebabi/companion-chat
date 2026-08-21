import {
    createActivityProjectionPort,
    createAssistantReplyProjectionPort,
    createClockPort,
    createConversationMessagePort,
    createDecisionPort,
    createDeferredBatchPort,
    createIdPort,
    createLifeEventPort,
    createLifeWorldPort,
    createPendingEventPort
} from './flow-ports.js';

export const PROACTIVE_FLOW_VERSION = 1;
export const PROACTIVE_DECISION_SCHEMA_VERSION = 1;
export const PROACTIVE_DECISION_MAX_REASON = 240;
export const PROACTIVE_DECISION_MAX_MESSAGE = 90;
export const ACTIVITY_DECISION_MAX_CONTENT = 900;

const TERMINAL_PENDING_STATUSES = new Set(['consumed', 'cancelled', 'expired']);
const PENDING_ACTIVE_STATUSES = new Set(['pending', 'triggered']);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function text(value, field, maxLength = 240, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const normalized = value.trim();
    if (!allowEmpty && !normalized) throw new TypeError(`${field} must not be empty`);
    if (normalized.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return normalized;
}

function requiredId(value, field) {
    return text(value, field, 160);
}

function valueOf(value, camel, snake, fallback) {
    if (value?.[camel] !== undefined) return value[camel];
    if (value?.[snake] !== undefined) return value[snake];
    return fallback;
}

function parseJson(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function frozenDecisionValue(value) {
    if (!isRecord(value)) return value;
    if (value.decision !== undefined) return value.decision;
    if (value.frozenDecision !== undefined) return value.frozenDecision;
    if (value.frozen_decision !== undefined) return value.frozen_decision;
    if (value.result !== undefined) return frozenDecisionValue(parseJson(value.result, value.result));
    if (value.result_json !== undefined) return frozenDecisionValue(parseJson(value.result_json, {}));
    if (value.decision_json !== undefined) return parseJson(value.decision_json, value.decision_json);
    return value;
}

function jobPayload(command) {
    if (isRecord(command.payload)) return command.payload;
    return parseJson(valueOf(command.job, 'payload', 'payload_json'), {});
}

function nowFor(command, context, clock) {
    const value = context?.now ?? command.now;
    if (value !== undefined && value !== null) {
        const iso = value instanceof Date ? value.toISOString() : value;
        if (typeof iso !== 'string' || !Number.isFinite(Date.parse(iso))) throw new TypeError('Proactive flow now must be a valid timestamp');
        return new Date(iso).toISOString();
    }
    return clock();
}

function personaFor(command) {
    return requiredId(command.personaId ?? command.persona_id, 'Proactive flow personaId');
}

function payloadId(command, key) {
    const payload = jobPayload(command);
    return payload[key] ?? payload[key.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`)];
}

function terminal(message, code = 'PROACTIVE_FLOW_INPUT_INVALID') {
    return Object.assign(new Error(message), {retryable: false, terminal: true, code});
}

function retryable(message, code = 'PROACTIVE_FLOW_RETRYABLE') {
    return Object.assign(new Error(message), {retryable: true, code});
}

function sourceId(source) {
    return source?.id ?? source?.eventId ?? source?.event_id ?? source?.pendingEventId ?? source?.pending_event_id ?? null;
}

function sourcePersonaId(source) {
    return source?.personaId ?? source?.persona_id;
}

function assertScoped(source, personaId, field) {
    if (!source) return null;
    if (sourcePersonaId(source) && sourcePersonaId(source) !== personaId) throw terminal(`${field} does not belong to persona`, 'PROACTIVE_FLOW_SCOPE_MISMATCH');
    return source;
}

function pendingStatus(row) {
    return valueOf(row, 'status', 'status', null);
}

function pendingId(row) {
    return row?.id ?? row?.pendingEventId ?? row?.pending_event_id;
}

function pendingAt(row, camel, snake) {
    return valueOf(row, camel, snake, null);
}

function normalizeProactiveDecision(value) {
    if (!isRecord(value) || value.schemaVersion !== PROACTIVE_DECISION_SCHEMA_VERSION || typeof value.send !== 'boolean') {
        throw terminal('Proactive decision schema is invalid');
    }
    const reason = text(value.reason, 'Proactive decision reason', PROACTIVE_DECISION_MAX_REASON);
    const message = text(value.message ?? '', 'Proactive decision message', PROACTIVE_DECISION_MAX_MESSAGE, {allowEmpty: true});
    if (!value.send && message) throw terminal('Proactive decision send=false cannot include a message');
    if (value.send && !message) throw terminal('Proactive decision send=true requires a message');
    return Object.freeze({schemaVersion: PROACTIVE_DECISION_SCHEMA_VERSION, send: value.send, reason, message});
}

function decodeDecision(value) {
    if (isRecord(value)) return normalizeProactiveDecision(value);
    if (typeof value !== 'string' || !value.trim()) throw terminal('Proactive decision is missing');
    const source = value.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
    try {
        return normalizeProactiveDecision(JSON.parse(source));
    } catch (error) {
        if (error?.terminal) throw error;
        throw terminal('Proactive decision JSON is invalid');
    }
}

function normalizeActivityDecision(value) {
    if (!isRecord(value) || typeof value.publish !== 'boolean') throw terminal('Activity decision schema is invalid');
    if (!value.publish) return Object.freeze({publish: false, content: '', media: null});
    const content = text(value.content, 'Activity decision content', ACTIVITY_DECISION_MAX_CONTENT);
    const media = value.media === undefined || value.media === null || value.media.kind === 'none' ? null : value.media;
    if (media !== null && (!isRecord(media) || !['image', 'video'].includes(media.kind))) throw terminal('Activity decision media kind is invalid');
    return Object.freeze({publish: true, content, media});
}

function decodeActivityDecision(value) {
    if (isRecord(value)) return normalizeActivityDecision(value);
    if (typeof value !== 'string' || !value.trim()) throw terminal('Activity decision is missing');
    const source = value.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
    try {
        return normalizeActivityDecision(JSON.parse(source));
    } catch (error) {
        if (error?.terminal) throw error;
        throw terminal('Activity decision JSON is invalid');
    }
}

function normalizeReplyMessages(value) {
    const messages = Array.isArray(value) ? value : value?.messages;
    if (!Array.isArray(messages) || !messages.length) throw new Error('Assistant reply projection returned no messages');
    return messages.map((message, index) => {
        if (!isRecord(message)) throw new Error(`Assistant reply message ${index} is invalid`);
        return Object.freeze({id: requiredId(message.id, `Assistant reply message ${index}.id`), ...message});
    });
}

function channelResult({result, facts = [], projections = [], effects = [], presentation = []}) {
    return {status: 'complete', result, facts, projections, effects, presentation};
}

function retryResult({result, facts = [], projections = [], effects = [], presentation = []}) {
    return {status: 'retry', result, facts, projections, effects, presentation};
}

function projection(type, value) {
    return {type, ...value};
}

function fact(type, value) {
    return {type, ...value};
}

function presentation(type, value) {
    return {type, ...value};
}

function resultMessageProjection(messages, source) {
    return projection('assistant_reply', {
        source,
        messageId: messages[0].id,
        messageIds: messages.map(message => message.id),
        messages
    });
}

async function readFrozenDecision({decision, kind, command, source, personaId, at, evaluatorInput, decode}) {
    const stored = decision.readFrozen
        ? await decision.readFrozen({kind, personaId, jobId: command.jobId, leaseOwner: command.context?.leaseOwner, sourceId: sourceId(source), source, at})
        : null;
    if (stored !== null && stored !== undefined) return {decision: decode(frozenDecisionValue(stored)), frozen: true, changed: false};

    const evaluated = await decision.evaluate({...evaluatorInput, kind, personaId, jobId: command.jobId, source, at});
    const normalized = decode(evaluated);
    const frozen = await decision.freeze({kind, personaId, jobId: command.jobId, leaseOwner: command.context?.leaseOwner, sourceId: sourceId(source), source, decision: normalized, at});
    const changed = frozen?.changed === undefined ? true : Boolean(frozen.changed);
    if (!changed) {
        const replay = decision.readFrozen ? await decision.readFrozen({kind, personaId, jobId: command.jobId, sourceId: sourceId(source), source, at}) : null;
        if (replay) return {decision: decode(frozenDecisionValue(replay)), frozen: true, changed: false};
        return {decision: normalized, frozen: false, changed: false};
    }
    return {decision: normalized, frozen: true, changed: true};
}

function flowDefinition(id, stepId, execute) {
    const run = (command, context = {}) => execute(command, context);
    return Object.freeze({
        id,
        version: PROACTIVE_FLOW_VERSION,
        steps: [{
            id: stepId,
            layer: 'application',
            dependencies: [{id: 'proactive-flow-contract', layer: 'contracts'}],
            async run(context, command) {
                const result = await execute(command, context);
                return {facts: result.facts, projections: result.projections, effects: result.effects, presentation: result.presentation};
            }
        }],
        run,
        execute: run,
        handle: run
    });
}

function sourceFromPayload(command, key) {
    const id = payloadId(command, key);
    if (!id) throw terminal(`Proactive job payload requires ${key}`);
    return requiredId(id, `Proactive job payload ${key}`);
}

function transitionInput({personaId, row, id, from, to, at, reason}) {
    return {personaId, pendingEventId: id, from, to, at, reason, current: row};
}

async function transitionPending(port, input) {
    const transitioned = await port.transition(input);
    return transitioned?.row ?? transitioned?.pendingEvent ?? transitioned ?? null;
}

function alreadyDelivered(replyPort, source) {
    return replyPort.findDelivery ? replyPort.findDelivery({personaId: source.personaId, source}) : null;
}

function deliveryResult(existing, source, reason = 'already_delivered') {
    return channelResult({
        result: {skipped: reason, sourceType: source.pendingEventId ? 'pending_event' : 'life_event', sourceId: source.pendingEventId ?? source.eventId},
        projections: [projection('assistant_reply_delivery', {source, existing})]
    });
}

function buildProactiveSource(kind, row, personaId) {
    if (kind === 'pending_event') {
        return {
            sourceType: 'pending_event',
            pendingEventId: pendingId(row),
            personaId,
            pendingEvent: {
                id: pendingId(row),
                summary: valueOf(row, 'summary', 'summary', ''),
                notBefore: pendingAt(row, 'notBefore', 'not_before'),
                expiresAt: pendingAt(row, 'expiresAt', 'expires_at'),
                sourceMessageId: valueOf(row, 'sourceMessageId', 'source_message_id', null)
            }
        };
    }
    return {
        sourceType: 'life_event',
        eventId: row.id,
        personaId,
        event: {...parseJson(valueOf(row, 'payload', 'payload_json'), {}), id: row.id, type: row.type}
    };
}

function buildProactiveMessageFlow({lifeEvent, pendingEvent, decision, reply, lifeWorld, conversation, clock, idGenerator, flowId = 'proactive-message', stepId = 'proactive-reply-projection'} = {}) {
    const events = lifeEvent ? createLifeEventPort(lifeEvent) : null;
    const pending = pendingEvent ? createPendingEventPort(pendingEvent) : null;
    const decisions = createDecisionPort(decision);
    const replies = createAssistantReplyProjectionPort(reply);
    const world = createLifeWorldPort(lifeWorld);
    const messages = conversation ? createConversationMessagePort(conversation) : null;
    const now = createClockPort(clock);

    async function execute(command, context = {}) {
        const kind = command.type === 'pending_event' ? 'pending_event' : 'proactive_message';
        const personaId = personaFor(command);
        const at = nowFor(command, context, now);
        let row;
        if (kind === 'pending_event') {
            const pendingIdValue = sourceFromPayload(command, 'pendingEventId');
            row = assertScoped(await pending.read({personaId, pendingEventId: pendingIdValue}), personaId, 'Pending event');
            if (!row) return channelResult({result: {skipped: 'pending_event_missing', pendingEventId: pendingIdValue}});
            const expiresAt = pendingAt(row, 'expiresAt', 'expires_at');
            const notBefore = pendingAt(row, 'notBefore', 'not_before');
            if (expiresAt && Date.parse(expiresAt) <= Date.parse(at)) {
                if (PENDING_ACTIVE_STATUSES.has(pendingStatus(row))) await transitionPending(pending, transitionInput({personaId, row, id: pendingId(row), from: pendingStatus(row), to: 'expired', at, reason: 'expired'}));
                return channelResult({result: {skipped: 'expired', pendingEventId: pendingIdValue}, projections: [projection('pending_event', {pendingEventId: pendingIdValue, status: 'expired'})]});
            }
            if (notBefore && Date.parse(notBefore) > Date.parse(at)) return channelResult({result: {skipped: 'not_due', pendingEventId: pendingIdValue}});
            if (TERMINAL_PENDING_STATUSES.has(pendingStatus(row))) return channelResult({result: {skipped: `pending_${pendingStatus(row)}`, pendingEventId: pendingIdValue}});
            if (pendingStatus(row) === 'pending') {
                const claimed = await transitionPending(pending, transitionInput({personaId, row, id: pendingId(row), from: 'pending', to: 'triggered', at, reason: 'due'}));
                if (claimed && pendingStatus(claimed) && pendingStatus(claimed) !== 'triggered') return channelResult({result: {skipped: 'claim_lost', pendingEventId: pendingIdValue}});
                row = claimed || {...row, status: 'triggered'};
            }
        } else {
            const eventId = sourceFromPayload(command, 'eventId');
            row = assertScoped(await events.read({personaId, eventId}), personaId, 'Life event');
            if (!row) return channelResult({result: {skipped: 'event_missing', eventId}});
        }

        const source = buildProactiveSource(kind, row, personaId);
        const existing = await alreadyDelivered(replies, source);
        if (existing) return deliveryResult(existing, source);
        const recentMessages = messages && source.pendingEvent?.sourceMessageId
            ? await messages.listByIds({personaId, ids: [source.pendingEvent.sourceMessageId]})
            : [];
        const worldContext = world ? await world.read({personaId, at}) : null;
        const frozen = await readFrozenDecision({
            decision: decisions,
            kind: 'proactive_message',
            command,
            source,
            personaId,
            at,
            evaluatorInput: {proactive: source, recentMessages, lifeWorld: worldContext},
            decode: decodeDecision
        });
        if (!frozen.frozen && !frozen.changed) {
            return retryResult({result: {skipped: 'decision_freeze_race', sourceType: source.sourceType, sourceId: sourceId(source)}});
        }
        const decisionValue = frozen.decision;
        if (!decisionValue.send) {
            if (kind === 'pending_event' && PENDING_ACTIVE_STATUSES.has(pendingStatus(row))) {
                await transitionPending(pending, transitionInput({personaId, row, id: pendingId(row), from: pendingStatus(row), to: 'consumed', at, reason: decisionValue.reason}));
            }
            return channelResult({
                result: {skipped: 'decision_send_false', sourceType: source.sourceType, sourceId: sourceId(source), reason: decisionValue.reason},
                projections: [projection('proactive_decision', {source, decision: decisionValue})]
            });
        }
        const payload = jobPayload(command);
        const projected = normalizeReplyMessages(await replies.project({personaId, text: decisionValue.message, source: {...source, ...(payload.replyMessageId ? {replyMessageId: payload.replyMessageId} : {})}, fallback: payload.fallbackText || '刚好想和你说一声。'}));
        if (kind === 'pending_event') await transitionPending(pending, transitionInput({personaId, row, id: pendingId(row), from: pendingStatus(row), to: 'consumed', at, reason: 'delivered'}));
        const result = {
            sourceType: source.sourceType,
            sourceId: sourceId(source),
            messageId: projected[0].id,
            messageIds: projected.map(message => message.id),
            reason: decisionValue.reason
        };
        if (kind === 'pending_event') result.pendingEventId = pendingId(row); else result.eventId = row.id;
        return channelResult({
            result,
            facts: [fact('proactive_reply_delivered', result)],
            projections: [projection('proactive_decision', {source, decision: decisionValue}), resultMessageProjection(projected, source)],
            presentation: [presentation('assistant_reply', {messages: projected, source})]
        });
    }

    return flowDefinition(flowId, stepId, execute);
}

export function createProactiveMessageFlow(options = {}) {
    return buildProactiveMessageFlow(options);
}

export function createPendingEventWorkerFlow(options = {}) {
    return buildProactiveMessageFlow({...options, flowId: options.flowId ?? 'pending-event', stepId: options.stepId ?? 'pending-event-reply-projection'});
}

export function createActivityDecisionFlow({lifeEvent, decision, activity, lifeWorld, clock, idGenerator} = {}) {
    const events = createLifeEventPort(lifeEvent);
    const decisions = createDecisionPort(decision);
    const activities = createActivityProjectionPort(activity);
    const world = createLifeWorldPort(lifeWorld);
    const now = createClockPort(clock);
    const nextId = createIdPort(idGenerator);

    async function execute(command, context = {}) {
        const personaId = personaFor(command);
        const eventId = sourceFromPayload(command, 'eventId');
        const at = nowFor(command, context, now);
        const event = assertScoped(await events.read({personaId, eventId}), personaId, 'Life event');
        if (!event) return channelResult({result: {skipped: 'event_missing', eventId}});
        const source = buildProactiveSource('life_event', event, personaId);
        const existing = activities.findByEvent ? await activities.findByEvent({personaId, eventId}) : null;
        if (existing) return channelResult({result: {skipped: 'already_published', eventId, activityId: existing.id}, projections: [projection('activity', {eventId, activityId: existing.id})]});
        const worldContext = world ? await world.read({personaId, at}) : null;
        const frozen = await readFrozenDecision({
            decision: decisions,
            kind: 'activity_decision',
            command,
            source,
            personaId,
            at,
            evaluatorInput: {event: source.event, lifeWorld: worldContext},
            decode: decodeActivityDecision
        });
        if (!frozen.frozen && !frozen.changed) return retryResult({result: {skipped: 'decision_freeze_race', eventId}});
        const decisionValue = frozen.decision;
        if (!decisionValue.publish) return channelResult({result: {eventId, published: false}, projections: [projection('activity_decision', {eventId, decision: decisionValue})]});
        const published = await activities.publish({
            personaId,
            eventId,
            content: decisionValue.content,
            media: decisionValue.media,
            createdAt: at,
            id: nextId('activity')
        });
        if (!published?.id) throw new Error('Activity projection must return an id');
        const effects = decisionValue.media
            ? [{
                effectId: nextId('effect'),
                kind: 'activity-media-request',
                capability: 'media_event',
                idempotencyKey: `activity:${published.id}:media`,
                causationId: eventId,
                payload: {activityId: published.id, eventId, personaId, media: decisionValue.media}
            }]
            : [];
        const result = {eventId, activityId: published.id, published: true, media: decisionValue.media?.kind || 'none'};
        return channelResult({
            result,
            facts: [fact('activity_published', result)],
            projections: [projection('activity', {activity: published}), projection('activity_decision', {eventId, decision: decisionValue})],
            effects,
            presentation: [presentation('activity', {activity: published})]
        });
    }

    return flowDefinition('activity-decision', 'activity-decision-projection', execute);
}

export function createDeferredChatReplyFlow({deferredBatch, conversation, reply, lifeWorld, replyComposer, clock} = {}) {
    const batches = createDeferredBatchPort(deferredBatch);
    const messages = createConversationMessagePort(conversation);
    const replies = createAssistantReplyProjectionPort(reply);
    const world = createLifeWorldPort(lifeWorld);
    const now = createClockPort(clock);
    if (!replyComposer || typeof replyComposer.compose !== 'function') throw new TypeError('deferred replyComposer must provide compose()');

    async function execute(command, context = {}) {
        const personaId = personaFor(command);
        const batchId = sourceFromPayload(command, 'batchId');
        const at = nowFor(command, context, now);
        const batch = await batches.read({personaId, batchId});
        if (!batch) return channelResult({result: {skipped: 'batch_missing', batchId}});
        const status = valueOf(batch, 'status', 'status', 'queued');
        if (['complete', 'expired', 'cancelled', 'failed'].includes(status)) return channelResult({result: {skipped: `batch_${status}`, batchId}});
        const deliverAt = valueOf(batch, 'deliverAt', 'deliver_at', null);
        if (deliverAt && Date.parse(deliverAt) > Date.parse(at)) return channelResult({result: {skipped: 'not_due', batchId}});
        const expiresAt = valueOf(batch, 'expiresAt', 'expires_at', null);
        if (expiresAt && Date.parse(expiresAt) <= Date.parse(at)) {
            if (batches.expire) await batches.expire({personaId, batchId, at, from: status});
            return channelResult({result: {skipped: 'expired', batchId}, projections: [projection('deferred_batch', {batchId, status: 'expired'})]});
        }
        const rawIds = valueOf(batch, 'messageIds', 'message_ids', null);
        const ids = Array.isArray(rawIds) ? rawIds : parseJson(rawIds ?? valueOf(batch, 'messageIdsJson', 'message_ids_json'), []);
        const pendingMessages = Array.isArray(batch.messages) ? batch.messages : await messages.listByIds({personaId, ids: Array.isArray(ids) ? ids.filter(Boolean) : []});
        const worldContext = world ? await world.read({personaId, at: deliverAt || at}) : null;
        let textValue;
        try {
            textValue = await replyComposer.compose({personaId, batch, messages: pendingMessages, lifeWorld: worldContext, at, command});
        } catch (error) {
            if (batches.recordFailure) await batches.recordFailure({personaId, batchId, at, error: String(error?.message || error).slice(0, 500)});
            throw retryable(error?.message || 'Deferred reply composition failed');
        }
        const projected = normalizeReplyMessages(await replies.project({personaId, text: textValue?.text ?? textValue, source: {batchId, personaId}, fallback: '刚刚看到你的消息了。'}));
        const completed = await batches.complete({personaId, batchId, at, resultMessageId: projected[0].id, messageIds: projected.map(message => message.id), status});
        const result = {batchId, messageId: projected[0].id, messageIds: projected.map(message => message.id)};
        return channelResult({
            result,
            facts: [fact('deferred_reply_completed', result)],
            projections: [projection('deferred_batch', {batchId, status: 'complete', completed}), resultMessageProjection(projected, {batchId, personaId})],
            presentation: [presentation('assistant_reply', {messages: projected, source: {batchId, personaId}})]
        });
    }

    return flowDefinition('deferred-chat-reply', 'deferred-reply-projection', execute);
}

export const createDeferredReplyFlow = createDeferredChatReplyFlow;

export default {
    createProactiveMessageFlow,
    createPendingEventWorkerFlow,
    createActivityDecisionFlow,
    createDeferredChatReplyFlow,
    createDeferredReplyFlow
};
