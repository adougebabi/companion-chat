function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    return value.trim();
}

function decodeCursor(value) {
    if (value === undefined || value === null || value === '') return null;
    try {
        const parsed = JSON.parse(Buffer.from(String(value), 'base64url').toString('utf8'));
        if (!isRecord(parsed) || typeof parsed.createdAt !== 'string' || typeof parsed.id !== 'string') throw new Error('invalid');
        return {createdAt: parsed.createdAt, id: parsed.id};
    } catch {
        throw Object.assign(new Error('会话游标无效'), {status: 400});
    }
}

function encodeCursor(row) {
    if (!row) return null;
    return Buffer.from(JSON.stringify({createdAt: row.created_at ?? row.createdAt, id: row.id})).toString('base64url');
}

function decodeJson(value, fallback) {
    if (value && typeof value === 'object') return value;
    try { return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}

function attachmentsDto(value) {
    const rows = Array.isArray(value) ? value : [];
    return rows.filter(isRecord).map(item => {
        const id = typeof item.id === 'string' ? item.id : '';
        return {
            ...item,
            ...(id && typeof item.url !== 'string' ? {url: `/api/companion/media/${encodeURIComponent(id)}`} : {})
        };
    });
}

function messageDto(row) {
    return {
        id: row.id,
        role: row.role,
        text: row.text,
        attachments: attachmentsDto(decodeJson(row.attachments ?? row.attachments_json, [])),
        generation: row.generation ?? (row.generation_json ? decodeJson(row.generation_json, {}) : undefined),
        jobs: decodeJson(row.jobs ?? row.jobs_json, []),
        proactiveEventId: row.proactive_event_id || row.proactiveEventId || undefined,
        proactivePendingEventId: row.proactive_pending_event_id || row.proactivePendingEventId || undefined,
        createdAt: row.created_at ?? row.createdAt,
        readAt: row.read_at || row.readAt || undefined
    };
}

/** Application owner for cursor/read/message DTO policy over raw conversation ports. */
export function createConversationService({repository, clock = () => new Date().toISOString(), idGenerator} = {}) {
    if (!isRecord(repository)) throw new TypeError('Conversation service requires a repository');
    if (typeof repository.getConversation !== 'function' || typeof repository.listMessages !== 'function') {
        throw new TypeError('Conversation service repository must provide getConversation() and listMessages()');
    }
    const now = typeof clock === 'function' ? clock : clock?.now?.bind(clock);
    if (typeof now !== 'function') throw new TypeError('Conversation service clock must be callable');
    const nextId = typeof idGenerator === 'function' ? idGenerator : prefix => `${prefix}_${Date.now()}`;

    function list(command = {}) {
        const personaId = requiredText(command.personaId, 'Conversation.personaId');
        const cursor = decodeCursor(command.cursor);
        const thread = repository.getConversation(personaId);
        if (!thread) return {items: [], nextCursor: null};
        const limit = Math.min(100, Math.max(1, Number(command.limit) || 50));
        const rows = repository.listMessages({conversationId: thread.id, cursor, limit});
        const items = rows.slice().reverse().map(messageDto);
        if (command.markRead !== false && typeof repository.updateReadAt === 'function') {
            repository.updateReadAt({conversationId: thread.id, role: 'assistant', readAt: now()});
        }
        return {items, nextCursor: rows.length === limit ? encodeCursor(rows.at(-1)) : null};
    }

    function appendMessage(command = {}) {
        const personaId = requiredText(command.personaId, 'Conversation.personaId');
        const role = requiredText(command.role, 'Message.role');
        if (!['assistant', 'user'].includes(role)) throw Object.assign(new Error('消息角色无效'), {status: 400});
        const createdAt = now();
        const thread = repository.getOrCreateConversation({personaId, id: nextId('conversation'), createdAt, updatedAt: createdAt});
        const row = repository.appendMessage({
            id: command.id || nextId('message'),
            conversationId: thread.id,
            role,
            text: String(command.text || ''),
            attachmentsJson: JSON.stringify(command.attachments || []),
            generationJson: command.generation ? JSON.stringify(command.generation) : null,
            jobsJson: JSON.stringify(command.jobs || []),
            proactiveEventId: command.proactiveEventId || null,
            proactivePendingEventId: command.proactivePendingEventId || null,
            createdAt,
            readAt: command.readAt ?? (role === 'user' ? createdAt : null)
        });
        return messageDto(row);
    }

    return Object.freeze({list, appendMessage, messageDto, decodeCursor, encodeCursor});
}

export default createConversationService;
