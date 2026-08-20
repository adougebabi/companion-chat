function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Conversation repository requires an open database');
    }
    return database;
}

function inputFor(first, second = {}) {
    if (typeof first === 'string') return {...second, conversationId: first};
    if (!first || typeof first !== 'object' || Array.isArray(first)) {
        throw new TypeError('Conversation repository input must be an object');
    }
    return first;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || !value) throw new TypeError(`${field} must be a non-empty string`);
    return value;
}

function nullableText(value, field) {
    if (value === null || value === undefined) return null;
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string or null`);
    return value;
}

function jsonColumn(value, field, fallback) {
    const resolved = value === undefined ? fallback : value;
    if (typeof resolved !== 'string') throw new TypeError(`${field} must be a JSON string`);
    return resolved;
}

/**
 * Create a repository for persona conversations and their message rows.
 *
 * IDs, timestamps, normalization, cursor encoding/decoding, DTO shaping, and
 * unread policy remain with the caller. This adapter only issues parameterized
 * SQL against the existing conversation and message tables. Callers may wrap
 * multiple methods in their own better-sqlite3 transaction.
 */
export function createConversationRepository({database} = {}) {
    const openDatabase = assertOpenDatabase(database);

    function getConversation(personaId) {
        return openDatabase.prepare(`
            SELECT * FROM companion_conversations
            WHERE persona_id = ?
        `).get(personaId);
    }

    function getOrCreateConversation(first, second = {}) {
        const input = typeof first === 'string' ? {...second, personaId: first} : inputFor(first);
        const existing = getConversation(input.personaId);
        if (existing) return existing;

        const conversationId = input.id ?? input.conversationId;
        const createdAt = requiredText(input.createdAt, 'Conversation.createdAt');
        const updatedAt = input.updatedAt === undefined ? createdAt : requiredText(input.updatedAt, 'Conversation.updatedAt');
        requiredText(input.personaId, 'Conversation.personaId');
        requiredText(conversationId, 'Conversation.id');

        // The persona unique key makes this safe for concurrent first access.
        openDatabase.prepare(`
            INSERT INTO companion_conversations (id, persona_id, created_at, updated_at)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(persona_id) DO NOTHING
        `).run(conversationId, input.personaId, createdAt, updatedAt);
        return getConversation(input.personaId);
    }

    function appendMessage(first, second = {}) {
        const input = inputFor(first, second);
        const conversationId = requiredText(input.conversationId, 'Message.conversationId');
        const id = requiredText(input.id, 'Message.id');
        const role = requiredText(input.role, 'Message.role');
        const text = typeof input.text === 'string' ? input.text : requiredText(input.text, 'Message.text');
        const attachmentsJson = jsonColumn(input.attachmentsJson ?? input.attachments_json, 'Message.attachmentsJson', '[]');
        const generationJson = nullableText(input.generationJson ?? input.generation_json, 'Message.generationJson');
        const jobsJson = jsonColumn(input.jobsJson ?? input.jobs_json, 'Message.jobsJson', '[]');
        const proactiveEventId = nullableText(input.proactiveEventId ?? input.proactive_event_id, 'Message.proactiveEventId');
        const proactivePendingEventId = nullableText(input.proactivePendingEventId ?? input.proactive_pending_event_id, 'Message.proactivePendingEventId');
        const createdAt = requiredText(input.createdAt, 'Message.createdAt');
        const readAt = nullableText(input.readAt, 'Message.readAt');

        openDatabase.prepare(`
            INSERT INTO companion_messages (
                id, conversation_id, role, text, attachments_json, generation_json,
                jobs_json, proactive_event_id, proactive_pending_event_id, created_at, read_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            id, conversationId, role, text, attachmentsJson, generationJson,
            jobsJson, proactiveEventId, proactivePendingEventId, createdAt, readAt
        );
        return openDatabase.prepare('SELECT * FROM companion_messages WHERE id = ?').get(id);
    }

    function updateConversationTimestamp(first, second = {}) {
        const input = inputFor(first, second);
        const updatedAt = requiredText(input.updatedAt, 'Conversation.updatedAt');
        if (input.conversationId !== undefined) {
            requiredText(input.conversationId, 'Conversation.conversationId');
            return openDatabase.prepare(`
                UPDATE companion_conversations SET updated_at = ? WHERE id = ?
            `).run(updatedAt, input.conversationId);
        }
        requiredText(input.personaId, 'Conversation.personaId');
        return openDatabase.prepare(`
            UPDATE companion_conversations SET updated_at = ? WHERE persona_id = ?
        `).run(updatedAt, input.personaId);
    }

    function listMessages(first, second = {}) {
        const input = inputFor(first, second);
        const pageSize = Number(input.limit === undefined ? 50 : input.limit);
        if (!Number.isInteger(pageSize) || pageSize < 1) throw new RangeError('Message limit must be a positive integer');

        const values = [];
        let where = '';
        if (input.conversationId !== undefined) {
            requiredText(input.conversationId, 'Message.conversationId');
            where = 'conversation_id = ?';
            values.push(input.conversationId);
        } else {
            requiredText(input.personaId, 'Conversation.personaId');
            where = 'conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)';
            values.push(input.personaId);
        }
        if (input.cursor !== undefined && input.cursor !== null) {
            if (!input.cursor || typeof input.cursor !== 'object' || Array.isArray(input.cursor)) throw new TypeError('Message cursor must be an object');
            requiredText(input.cursor.createdAt, 'Message cursor.createdAt');
            requiredText(input.cursor.id, 'Message cursor.id');
            where += ' AND (created_at < ? OR (created_at = ? AND id < ?))';
            values.push(input.cursor.createdAt, input.cursor.createdAt, input.cursor.id);
        }
        return openDatabase.prepare(`
            SELECT * FROM companion_messages
            WHERE ${where}
            ORDER BY created_at DESC, id DESC
            LIMIT ?
        `).all(...values, pageSize);
    }

    // The route owns which roles are considered unread; this method only
    // performs the requested table update and leaves policy outside storage.
    function updateReadAt(first, second = {}) {
        const input = inputFor(first, second);
        const readAt = requiredText(input.readAt, 'Message.readAt');
        const values = [readAt];
        let where;
        if (input.conversationId !== undefined) {
            requiredText(input.conversationId, 'Message.conversationId');
            where = 'conversation_id = ?';
            values.push(input.conversationId);
        } else {
            requiredText(input.personaId, 'Conversation.personaId');
            where = 'conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)';
            values.push(input.personaId);
        }
        if (input.role !== undefined) {
            requiredText(input.role, 'Message.role');
            where += ' AND role = ?';
            values.push(input.role);
        }
        if (input.onlyUnread !== false) where += ' AND read_at IS NULL';
        return openDatabase.prepare(`UPDATE companion_messages SET read_at = ? WHERE ${where}`).run(...values);
    }

    return Object.freeze({
        getConversation,
        getOrCreateConversation,
        appendMessage,
        updateConversationTimestamp,
        listMessages,
        updateReadAt,
        markMessagesRead: updateReadAt
    });
}
