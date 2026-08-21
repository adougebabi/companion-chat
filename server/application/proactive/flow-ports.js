/**
 * Small application-side adapters for life/proactive worker flows.
 *
 * These adapters intentionally contain no persistence or provider policy. They
 * make the port names stable while the existing table repositories are being
 * migrated. A port may be synchronous or asynchronous; the flows await both.
 */

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function sourceObject(source, field) {
    if (!isRecord(source) && typeof source !== 'function') {
        throw new TypeError(`${field} port must be an object or function`);
    }
    return source;
}

function method(source, names, field, {optional = false} = {}) {
    if (typeof source === 'function') return {name: names[0], call: source};
    for (const name of names) {
        if (source[name] !== undefined) {
            if (typeof source[name] !== 'function') throw new TypeError(`${field}.${name} must be a function`);
            return {name, call: source[name].bind(source)};
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${names.join('() or ')}()`);
}

function invoke(entry, input) {
    if (!entry) return undefined;
    if (entry.name === 'appendUserVisibleAssistantReply') {
        return entry.call(input.personaId, input.text, {
            proactiveEventId: input.source?.eventId ?? undefined,
            proactivePendingEventId: input.source?.pendingEventId ?? undefined,
            fallback: input.fallback
        });
    }
    return entry.call(input);
}

function port(source, field, methods) {
    const resolved = sourceObject(source, field);
    const entries = Object.fromEntries(Object.entries(methods).map(([key, config]) => [
        key,
        method(resolved, config.names, `${field}.${key}`, {optional: config.optional === true})
    ]));
    return Object.freeze(Object.fromEntries(Object.entries(entries).map(([key, entry]) => [
        key,
        entry ? input => invoke(entry, input) : undefined
    ])));
}

export function createLifeEventPort(source) {
    return port(source, 'lifeEvent', {
        read: {names: ['findById', 'findEvent', 'get', 'read']}
    });
}

export function createPendingEventPort(source) {
    return port(source, 'pendingEvent', {
        read: {names: ['findById', 'findPendingEvent', 'get', 'read']},
        transition: {names: ['transition', 'updateStatus', 'markStatus']},
        findDelivery: {names: ['findDelivery', 'findMessage', 'findDelivered'], optional: true}
    });
}

export function createDecisionPort(source) {
    return port(source, 'decision', {
        readFrozen: {names: ['readFrozen', 'findFrozen', 'findByJob', 'find'], optional: true},
        evaluate: {names: ['evaluate', 'decide']},
        freeze: {names: ['freeze', 'persistFrozen', 'saveFrozen']}
    });
}

export function createAssistantReplyProjectionPort(source) {
    return port(source, 'assistantReplyProjection', {
        findDelivery: {names: ['findDelivery', 'findExisting', 'findBySource'], optional: true},
        project: {names: ['project', 'appendReply', 'appendUserVisibleAssistantReply']}
    });
}

export function createActivityProjectionPort(source) {
    return port(source, 'activityProjection', {
        findByEvent: {names: ['findByEvent', 'findActivityByEvent', 'find'], optional: true},
        publish: {names: ['publish', 'insertActivity', 'create']}
    });
}

export function createDeferredBatchPort(source) {
    return port(source, 'deferredBatch', {
        read: {names: ['findById', 'findBatch', 'find', 'read']},
        complete: {names: ['complete', 'markComplete', 'finish']},
        expire: {names: ['expire', 'markExpired'], optional: true},
        recordFailure: {names: ['recordFailure', 'markAttemptFailure'], optional: true}
    });
}

export function createConversationMessagePort(source) {
    return port(source, 'conversationMessages', {
        listByIds: {names: ['listByIds', 'listMessagesByIds', 'findByIds']}
    });
}

export function createLifeWorldPort(source) {
    if (source === undefined || source === null) return null;
    return port(source, 'lifeWorld', {
        read: {names: ['readResolverInput', 'read', 'contextFor']}
    });
}

export function createIdPort(source) {
    if (source === undefined || source === null) return prefix => `${prefix}_${Date.now().toString(36)}`;
    if (typeof source === 'function') return source;
    if (isRecord(source) && typeof source.next === 'function') return source.next.bind(source);
    throw new TypeError('proactive idGenerator must be a function or provide next()');
}

export function createClockPort(source) {
    if (source === undefined) return () => new Date().toISOString();
    const call = typeof source === 'function' ? source : source?.now?.bind(source);
    if (typeof call !== 'function') throw new TypeError('proactive clock must be a function or provide now()');
    return () => {
        const value = call();
        const iso = value instanceof Date ? value.toISOString() : value;
        if (typeof iso !== 'string' || !Number.isFinite(Date.parse(iso))) throw new TypeError('proactive clock must return a valid timestamp');
        return iso;
    };
}

export default {
    createLifeEventPort,
    createPendingEventPort,
    createDecisionPort,
    createAssistantReplyProjectionPort,
    createActivityProjectionPort,
    createDeferredBatchPort,
    createConversationMessagePort,
    createLifeWorldPort,
    createIdPort,
    createClockPort
};
