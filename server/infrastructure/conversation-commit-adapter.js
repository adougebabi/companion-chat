const ASSISTANT_MESSAGE_FACT = 'conversation.assistant_message';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function callable(value, methods, field, {optional = false} = {}) {
    if (typeof value === 'function') return value;
    if (isRecord(value)) {
        for (const method of methods) {
            if (typeof value[method] === 'function') return value[method].bind(value);
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${methods.join('() or ')}()`);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError(`${field} must be a non-empty string`);
    return value;
}

function timestamp(value, field) {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function idFunction(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.next === 'function') return value.next.bind(value);
    let sequence = 0;
    return prefix => `${prefix}_${Date.now()}_${sequence++}`;
}

function transactionFunction(value) {
    if (value === undefined || value === null) return callback => callback();
    const transaction = callable(value, ['transaction', 'runInTransaction', 'run'], 'Conversation commit transaction');
    return callback => transaction(callback);
}

function assistantFacts(result) {
    const facts = Array.isArray(result?.facts) ? result.facts : [];
    return facts.filter(fact => fact?.type === ASSISTANT_MESSAGE_FACT && Array.isArray(fact.messages));
}

function messageRow(message, conversationId) {
    if (!isRecord(message)) throw new TypeError('Assistant message fact must contain message objects');
    const createdAt = timestamp(message.createdAt ?? message.created_at, 'Assistant message.createdAt');
    const attachments = message.attachments ?? message.attachments_json ?? [];
    const jobs = message.jobs ?? message.jobs_json ?? [];
    const generation = message.generation ?? message.generation_json ?? null;
    return {
        id: requiredText(message.id, 'Assistant message.id'),
        conversationId,
        role: 'assistant',
        text: typeof message.text === 'string' ? message.text : '',
        attachmentsJson: typeof attachments === 'string' ? attachments : JSON.stringify(attachments),
        generationJson: generation === null || generation === undefined
            ? null
            : typeof generation === 'string' ? generation : JSON.stringify(generation),
        jobsJson: typeof jobs === 'string' ? jobs : JSON.stringify(jobs),
        proactiveEventId: message.proactiveEventId ?? message.proactive_event_id ?? null,
        proactivePendingEventId: message.proactivePendingEventId ?? message.proactive_pending_event_id ?? null,
        createdAt,
        readAt: message.readAt ?? message.read_at ?? null
    };
}

/**
 * Persist assistant message facts through the table-scoped conversation port.
 * This adapter intentionally knows neither the storage driver nor the HTTP
 * transport. The caller may inject a transaction wrapper for atomic writes.
 */
export function createConversationCommitAdapter({repository, conversationRepository, clock = () => new Date().toISOString(), idGenerator, id, transaction} = {}) {
    const conversations = repository ?? conversationRepository;
    if (!isRecord(conversations)) throw new TypeError('Conversation commit adapter requires a repository');
    if (typeof conversations.getOrCreateConversation !== 'function') throw new TypeError('Conversation commit adapter repository must provide getOrCreateConversation()');
    if (typeof conversations.appendMessages !== 'function' && typeof conversations.appendMessage !== 'function') {
        throw new TypeError('Conversation commit adapter repository must provide appendMessages() or appendMessage()');
    }
    const now = callable(clock, ['now'], 'Conversation commit clock');
    const nextId = idFunction(idGenerator ?? id);
    const runTransaction = transactionFunction(transaction);

    function commit(stepResult = {}) {
        const facts = assistantFacts(stepResult);
        if (!facts.length) return {messages: [], facts: 0};
        const committed = [];
        runTransaction(() => {
            for (const fact of facts) {
                const personaId = requiredText(fact.personaId, 'Assistant message fact personaId');
                const first = fact.messages[0];
                const createdAt = timestamp(first?.createdAt ?? now(), 'Conversation.createdAt');
                const conversation = conversations.getOrCreateConversation({
                    personaId,
                    id: fact.conversationId ?? nextId('conversation'),
                    createdAt,
                    updatedAt: timestamp(fact.updatedAt ?? fact.messages.at(-1)?.createdAt ?? createdAt, 'Conversation.updatedAt')
                });
                if (!conversation?.id) throw new Error('Conversation repository did not return a conversation');
                const rows = fact.messages.map(message => messageRow(message, conversation.id));
                const pending = rows.filter(row => {
                    if (typeof conversations.findMessage !== 'function') return true;
                    return !conversations.findMessage({id: row.id, personaId});
                });
                if (!pending.length) continue;
                let inserted;
                if (typeof conversations.appendMessages === 'function') {
                    inserted = conversations.appendMessages({
                        conversationId: conversation.id,
                        messages: pending,
                        updatedAt: rows.at(-1).createdAt
                    });
                } else {
                    inserted = pending.map(row => conversations.appendMessage(row));
                    conversations.updateConversationTimestamp?.({conversationId: conversation.id, updatedAt: rows.at(-1).createdAt});
                }
                committed.push(...(Array.isArray(inserted) ? inserted : [inserted]).filter(Boolean));
            }
        });
        return {messages: committed, facts: facts.length};
    }

    return Object.freeze({commit, commitStepResult: commit});
}

export const createConversationCommitBoundary = createConversationCommitAdapter;
export const createChatConversationCommitAdapter = createConversationCommitAdapter;
export const ASSISTANT_MESSAGE_FACT_TYPE = ASSISTANT_MESSAGE_FACT;

