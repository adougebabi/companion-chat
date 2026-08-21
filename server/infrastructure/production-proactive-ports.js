import {randomUUID} from 'node:crypto';

import {
    createActivityDecisionFlow,
    createDeferredChatReplyFlow,
    createPendingEventWorkerFlow,
    createProactiveMessageFlow
} from '../application/proactive/worker-flows.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function valueFor(value, camel, snake) {
    return value?.[camel] === undefined ? value?.[snake] : value[camel];
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

function clockFor(clock) {
    if (typeof clock === 'function') return () => new Date(clock()).toISOString();
    if (isRecord(clock) && typeof clock.now === 'function') return () => new Date(clock.now()).toISOString();
    return () => new Date().toISOString();
}

function idFor(id) {
    if (typeof id === 'function') return id;
    if (isRecord(id) && typeof id.next === 'function') return id.next.bind(id);
    return prefix => `${prefix}_${randomUUID()}`;
}

function providerCompletion(provider, settings) {
    if (!provider || typeof provider.stream !== 'function') return null;
    return async ({messages, signal, temperature = 0.15} = {}) => {
        const config = typeof settings === 'function' ? settings() : {};
        const response = await provider.stream({
            model: config?.model,
            stream: false,
            temperature,
            messages,
            signal
        });
        if (!response?.ok) throw new Error(`MTPLX HTTP ${response?.status ?? 'error'}`);
        const body = await response.json();
        const content = body?.choices?.[0]?.message?.content;
        if (typeof content !== 'string' || !content.trim()) throw new Error('MTPLX returned an empty decision');
        const source = content.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
        try { return JSON.parse(source); } catch { throw new Error('MTPLX decision JSON is invalid'); }
    };
}

function providerReply(provider, settings) {
    if (!provider || typeof provider.stream !== 'function') return null;
    return async ({messages, signal, temperature = 0.7} = {}) => {
        const config = typeof settings === 'function' ? settings() : {};
        const response = await provider.stream({
            model: config?.model,
            stream: false,
            temperature,
            messages,
            signal
        });
        if (!response?.ok) throw new Error(`MTPLX HTTP ${response?.status ?? 'error'}`);
        const body = await response.json();
        const content = body?.choices?.[0]?.message?.content;
        if (typeof content !== 'string' || !content.trim()) throw new Error('MTPLX returned an empty reply');
        return content.trim();
    };
}

function requireDatabase(database) {
    if (!database || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Production proactive ports require an open database');
    }
    return database;
}

function createPendingPort(database, jobRepository, now) {
    return {
        findById({pendingEventId, personaId}) {
            return database.prepare('SELECT * FROM companion_pending_events WHERE id = ? AND persona_id = ?').get(pendingEventId, personaId);
        },
        transition({pendingEventId, personaId, from, to, at, reason}) {
            const fromStatus = from ? ' AND status = ?' : '';
            const values = [to, at ?? now(), ...(to === 'triggered' ? [at ?? now()] : []), ...(to === 'consumed' ? [at ?? now()] : []), ...(to === 'cancelled' || to === 'expired' ? [at ?? now()] : []), reason ? JSON.stringify({reason}) : null, pendingEventId, personaId];
            const sql = `
                UPDATE companion_pending_events
                SET status = ?, updated_at = ?,
                    triggered_at = CASE WHEN ? = 'triggered' THEN ? ELSE triggered_at END,
                    consumed_at = CASE WHEN ? = 'consumed' THEN ? ELSE consumed_at END,
                    cancelled_at = CASE WHEN ? IN ('cancelled', 'expired') THEN ? ELSE cancelled_at END,
                    payload_json = CASE WHEN ? IS NULL THEN payload_json ELSE json_set(payload_json, '$.lastTransition', json(?)) END
                WHERE id = ? AND persona_id = ?${fromStatus}
            `;
            // Keep transitions deliberately small and avoid trying to infer a
            // missing status from stale worker observations.
            const row = database.prepare('SELECT * FROM companion_pending_events WHERE id = ? AND persona_id = ?').get(pendingEventId, personaId);
            if (!row || (from && row.status !== from)) return null;
            database.prepare(`
                UPDATE companion_pending_events
                SET status = ?, updated_at = ?,
                    triggered_at = CASE WHEN ? = 'triggered' THEN ? ELSE triggered_at END,
                    consumed_at = CASE WHEN ? = 'consumed' THEN ? ELSE consumed_at END,
                    cancelled_at = CASE WHEN ? IN ('cancelled', 'expired') THEN ? ELSE cancelled_at END
                WHERE id = ? AND persona_id = ?${fromStatus}
            `).run(
                to, at ?? now(), to, at ?? now(), to, at ?? now(), to, at ?? now(), pendingEventId, personaId, ...(from ? [from] : [])
            );
            return database.prepare('SELECT * FROM companion_pending_events WHERE id = ?').get(pendingEventId);
        },
        findDelivery({personaId, pendingEventId}) {
            return database.prepare(`
                SELECT messages.* FROM companion_messages messages
                JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
                WHERE conversations.persona_id = ? AND messages.proactive_pending_event_id = ?
                LIMIT 1
            `).get(personaId, pendingEventId);
        },
        jobRepository
    };
}

function createDecisionPort({database, jobRepository, completion, now}) {
    return {
        readFrozen({jobId}) {
            const row = jobRepository.find({id: jobId});
            const decision = parseJson(row?.result_json, {}).decision;
            return decision ?? null;
        },
        async evaluate(input) {
            if (!completion) throw new Error('MTPLX proactive decision provider is unavailable');
            const source = input.proactive ?? input.event ?? {};
            return completion({
                messages: [
                    {role: 'system', content: 'Return strict JSON for this proactive decision. Use send/publish booleans as requested by the caller and never invent facts.'},
                    {role: 'user', content: JSON.stringify({source, recentMessages: input.recentMessages ?? [], lifeWorld: input.lifeWorld ?? null})}
                ],
                signal: input.context?.signal
            });
        },
        freeze({jobId, decision, leaseOwner, personaId}) {
            const row = jobRepository.find({id: jobId});
            if (!row) return {changed: false};
            return jobRepository.patchResult(row, {patch: {decision}, leaseOwner, personaId, now: now()});
        }
    };
}

function createReplyProjection({database, conversationRepository, id, now}) {
    function conversationFor(personaId) {
        return conversationRepository.getConversation(personaId);
    }
    return {
        findDelivery({personaId, source}) {
            if (source?.pendingEventId) return database.prepare('SELECT * FROM companion_messages WHERE proactive_pending_event_id = ? LIMIT 1').get(source.pendingEventId);
            if (source?.eventId) return database.prepare('SELECT * FROM companion_messages WHERE proactive_event_id = ? LIMIT 1').get(source.eventId);
            return null;
        },
        project({personaId, text, source, fallback}) {
            const conversation = conversationFor(personaId);
            if (!conversation) throw new Error('Conversation is unavailable for proactive reply');
            const createdAt = now();
            const value = typeof text === 'string' && text.trim() ? text.trim() : fallback;
            const row = conversationRepository.appendMessage({
                id: source?.replyMessageId ?? id('message'),
                conversationId: conversation.id,
                role: 'assistant',
                text: value,
                attachmentsJson: '[]',
                generationJson: null,
                jobsJson: '[]',
                proactiveEventId: source?.eventId ?? null,
                proactivePendingEventId: source?.pendingEventId ?? null,
                createdAt,
                readAt: null
            });
            return [{id: row.id, role: row.role, text: row.text, attachments: [], jobs: [], createdAt: row.created_at, readAt: row.read_at}];
        }
    };
}

function createActivityProjection({activityRepository, id, now}) {
    return {
        findByEvent({personaId, eventId}) {
            return activityRepository.findActivityByEvent?.({eventId, personaId})
                ?? activityRepository.findActivity({id: eventId, personaId})
                ?? activityRepository.findActivity({activityId: eventId, personaId});
        },
        publish({personaId, eventId, content, media, createdAt = now()}) {
            return activityRepository.insertActivity({
                id: id('activity'), personaId, eventId, content,
                mediaMode: media?.kind && media.kind !== 'none' ? media.kind : 'none',
                mediaStatus: media?.kind && media.kind !== 'none' ? 'queued' : 'none',
                createdAt
            });
        }
    };
}

function createDeferredBatchPort(database, now) {
    return {
        findById({batchId, personaId}) {
            const row = database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ? AND persona_id = ?').get(batchId, personaId);
            if (!row) return null;
            return {...row, messageIds: parseJson(row.message_ids_json, [])};
        },
        complete({batchId, personaId, at = now(), resultMessageId, messageIds}) {
            const result = database.prepare(`
                UPDATE companion_chat_deferred_batches
                SET status = 'complete', result_message_id = ?, message_ids_json = ?, updated_at = ?, completed_at = ?
                WHERE id = ? AND persona_id = ? AND status IN ('queued', 'processing')
            `).run(resultMessageId, JSON.stringify(messageIds || []), at, at, batchId, personaId);
            return result.changes ? database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ?').get(batchId) : null;
        },
        expire({batchId, personaId, at = now()}) {
            return database.prepare(`UPDATE companion_chat_deferred_batches SET status = 'expired', updated_at = ? WHERE id = ? AND persona_id = ? AND status = 'queued'`).run(at, batchId, personaId);
        },
        recordFailure({batchId, personaId, at = now(), error}) {
            return database.prepare(`UPDATE companion_chat_deferred_batches SET error = ?, attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND persona_id = ?`).run(String(error || '').slice(0, 500), at, batchId, personaId);
        }
    };
}

function createConversationMessages(conversationRepository) {
    return {
        listByIds({personaId, ids}) {
            return (Array.isArray(ids) ? ids : []).map(id => conversationRepository.findMessage({id, personaId})).filter(Boolean);
        }
    };
}

/** Compose the application flows used by the default proactive worker. */
export function createProductionProactiveFlows({database, repositories, provider, settings, clock, id} = {}) {
    const openDatabase = requireDatabase(database);
    const now = clockFor(clock);
    const generateId = idFor(id);
    const completion = providerCompletion(provider, settings);
    const replyCompletion = providerReply(provider, settings);
    const conversation = repositories.conversation ?? repositories.conversationRepository;
    const lifeEvent = repositories.lifeEvent ?? repositories.lifeEventRepository;
    const activity = repositories.activity ?? repositories.activityRepository;
    const pending = createPendingPort(openDatabase, repositories.job ?? repositories.jobRepository, now);
    const decision = createDecisionPort({database: openDatabase, jobRepository: repositories.job ?? repositories.jobRepository, completion, now});
    const reply = createReplyProjection({database: openDatabase, conversationRepository: conversation, id: generateId, now});
    const activityProjection = createActivityProjection({activityRepository: activity, id: generateId, now});
    const deferredBatch = createDeferredBatchPort(openDatabase, now);
    const conversationMessages = createConversationMessages(conversation);
    const replyComposer = {
        async compose({personaId, batch, messages, lifeWorld, at, command, signal} = {}) {
            if (!replyCompletion) throw new Error('MTPLX deferred reply provider is unavailable');
            return replyCompletion({
                messages: [
                    {role: 'system', content: 'Return one concise user-visible assistant reply. Preserve the supplied life-world facts and do not invent events.'},
                    {role: 'user', content: JSON.stringify({personaId, batch, messages, lifeWorld, at, command: {type: command?.type, jobId: command?.jobId}})}
                ],
                signal
            });
        }
    };
    return {
        proactive_message: createProactiveMessageFlow({lifeEvent, pendingEvent: pending, decision, reply, clock: now, idGenerator: generateId}),
        pending_event: createPendingEventWorkerFlow({lifeEvent, pendingEvent: pending, decision, reply, clock: now, idGenerator: generateId}),
        activity_decision: createActivityDecisionFlow({lifeEvent, decision, activity: activityProjection, clock: now, idGenerator: generateId}),
        deferred_chat_reply: createDeferredChatReplyFlow({deferredBatch, conversation: conversationMessages, reply, replyComposer, clock: now})
    };
}

export default createProductionProactiveFlows;
