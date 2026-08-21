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

    /**
     * Append prepared message rows within the caller's transaction and update
     * the owning conversation once. The repository intentionally returns raw
     * SQLite rows; sentence policy, unread policy, and DTO shaping stay above
     * this table boundary.
     */
    function appendMessages(first, second = {}) {
        const input = inputFor(first, second);
        const messages = input.messages ?? input.rows;
        if (!Array.isArray(messages)) throw new TypeError('Conversation messages must be an array');
        if (!messages.length) return [];

        let conversationId = input.conversationId;
        if (conversationId !== undefined) requiredText(conversationId, 'Message.conversationId');
        const prepared = messages.map((message, index) => {
            if (!message || typeof message !== 'object' || Array.isArray(message)) throw new TypeError(`Message row ${index} must be an object`);
            const rowConversationId = message.conversationId ?? message.conversation_id ?? conversationId;
            requiredText(rowConversationId, `Message[${index}].conversationId`);
            if (conversationId === undefined) conversationId = rowConversationId;
            if (rowConversationId !== conversationId) throw new TypeError('Conversation messages must share one conversation');
            return {
                id: requiredText(message.id, `Message[${index}].id`),
                conversationId,
                role: requiredText(message.role, `Message[${index}].role`),
                text: typeof message.text === 'string' ? message.text : requiredText(message.text, `Message[${index}].text`),
                attachmentsJson: jsonColumn(message.attachmentsJson ?? message.attachments_json, `Message[${index}].attachmentsJson`, '[]'),
                generationJson: nullableText(message.generationJson ?? message.generation_json, `Message[${index}].generationJson`),
                jobsJson: jsonColumn(message.jobsJson ?? message.jobs_json, `Message[${index}].jobsJson`, '[]'),
                proactiveEventId: nullableText(message.proactiveEventId ?? message.proactive_event_id, `Message[${index}].proactiveEventId`),
                proactivePendingEventId: nullableText(message.proactivePendingEventId ?? message.proactive_pending_event_id, `Message[${index}].proactivePendingEventId`),
                createdAt: requiredText(message.createdAt ?? message.created_at, `Message[${index}].createdAt`),
                readAt: nullableText(message.readAt ?? message.read_at, `Message[${index}].readAt`)
            };
        });
        const updatedAt = input.updatedAt === undefined
            ? prepared.at(-1).createdAt
            : requiredText(input.updatedAt, 'Conversation.updatedAt');
        const insert = openDatabase.prepare(`
            INSERT INTO companion_messages (
                id, conversation_id, role, text, attachments_json, generation_json,
                jobs_json, proactive_event_id, proactive_pending_event_id, created_at, read_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `);
        const select = openDatabase.prepare('SELECT * FROM companion_messages WHERE id = ?');
        const inserted = [];
        for (const message of prepared) {
            insert.run(
                message.id, message.conversationId, message.role, message.text,
                message.attachmentsJson, message.generationJson, message.jobsJson,
                message.proactiveEventId, message.proactivePendingEventId,
                message.createdAt, message.readAt
            );
            inserted.push(select.get(message.id));
        }
        openDatabase.prepare(`
            UPDATE companion_conversations SET updated_at = ? WHERE id = ?
        `).run(updatedAt, conversationId);
        return inserted;
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

    function findMessage(first, second = {}) {
        const input = inputFor(first, second);
        const messageId = requiredText(input.id ?? input.messageId ?? input.message_id, 'Message.id');
        const values = [messageId];
        let scope = '';
        if (input.personaId !== undefined || input.persona_id !== undefined) {
            const personaId = requiredText(input.personaId ?? input.persona_id, 'Message.personaId');
            scope = ' AND conversations.persona_id = ?';
            values.push(personaId);
        }
        return openDatabase.prepare(`
            SELECT messages.*, conversations.persona_id AS persona_id FROM companion_messages messages
            JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
            WHERE messages.id = ?${scope}
        `).get(...values);
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
        appendMessages,
        updateConversationTimestamp,
        listMessages,
        findMessage,
        updateReadAt,
        markMessagesRead: updateReadAt
    });
}
