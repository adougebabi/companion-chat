import {randomUUID} from 'node:crypto';

import {
    createActivityDecisionFlow,
    createDeferredChatReplyFlow,
    createPendingEventWorkerFlow,
    createProactiveMessageFlow
} from '../application/proactive/worker-flows.js';
import {splitChatAssistantReply} from '../application/chat-production-adapter.js';
import {serializePromptMessages} from '../application/context-pipeline.js';

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

async function readContext(contextReader, input) {
    const reader = typeof contextReader === 'function'
        ? contextReader
        : contextReader?.read ?? contextReader?.readContext;
    return typeof reader === 'function' ? reader.call(contextReader, input) : null;
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
            // Keep transitions deliberately small and avoid trying to infer a
            // missing status from stale worker observations.
            const row = database.prepare('SELECT * FROM companion_pending_events WHERE id = ? AND persona_id = ?').get(pendingEventId, personaId);
            if (!row || (from && row.status !== from)) return null;
            const result = database.prepare(`
                UPDATE companion_pending_events
                SET status = ?, updated_at = ?,
                    triggered_at = CASE WHEN ? = 'triggered' THEN ? ELSE triggered_at END,
                    consumed_at = CASE WHEN ? = 'consumed' THEN ? ELSE consumed_at END,
                    cancelled_at = CASE WHEN ? IN ('cancelled', 'expired') THEN ? ELSE cancelled_at END,
                    payload_json = CASE WHEN ? IS NULL THEN payload_json ELSE json_set(payload_json, '$.lastTransition', json(?)) END
                WHERE id = ? AND persona_id = ?${fromStatus}
            `).run(
                to, at ?? now(), to, at ?? now(), to, at ?? now(), to, at ?? now(),
                reason ? JSON.stringify({reason}) : null, reason ? JSON.stringify({reason}) : null,
                pendingEventId, personaId, ...(from ? [from] : [])
            );
            if (!result.changes) return null;
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

function createDecisionPort({database, jobRepository, completion, now, contextReader}) {
    return {
        readFrozen({jobId}) {
            const row = jobRepository.find({id: jobId});
            const decision = parseJson(row?.result_json, {}).decision;
            return decision ?? null;
        },
        async evaluate(input) {
            if (!completion) throw new Error('MTPLX proactive decision provider is unavailable');
            const source = input.proactive ?? input.event ?? {};
            const context = await readContext(contextReader, {command: {personaId: input.personaId, chatAt: input.at}, messages: input.recentMessages ?? []});
            return completion({
                messages: serializePromptMessages({
                    context,
                    messages: input.recentMessages ?? [],
                    instruction: 'Return strict JSON for this proactive decision. Use send/publish booleans as requested by the caller and never invent facts.'
                }).concat([
                    {role: 'user', content: JSON.stringify({source, recentMessages: input.recentMessages ?? [], lifeWorld: input.lifeWorld ?? null})}
                ]),
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
            if (source?.batchId) {
                return database.prepare(`
                    SELECT messages.* FROM companion_messages messages
                    JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
                    WHERE conversations.persona_id = ?
                      AND json_extract(messages.jobs_json, '$[0].deferredBatchId') = ?
                    LIMIT 1
                `).get(personaId, source.batchId)
                    ?? database.prepare('SELECT * FROM companion_messages WHERE id = ? LIMIT 1').get(source.replyMessageId);
            }
            return null;
        },
        project({personaId, text, source, fallback}) {
            const conversation = conversationFor(personaId);
            if (!conversation) throw new Error('Conversation is unavailable for proactive reply');
            const createdAt = now();
            const values = splitChatAssistantReply(text, fallback);
            const rows = values.map((value, index) => ({
                id: index === 0 && source?.replyMessageId ? source.replyMessageId : id('message'),
                conversationId: conversation.id,
                role: 'assistant',
                text: value,
                attachmentsJson: '[]',
                generationJson: null,
                jobsJson: source?.batchId ? JSON.stringify([{type: 'deferred_chat_reply', deferredBatchId: source.batchId}]) : '[]',
                proactiveEventId: source?.eventId ?? null,
                proactivePendingEventId: source?.pendingEventId ?? null,
                createdAt: new Date(Date.parse(createdAt) + index).toISOString(),
                readAt: null
            }));
            const stored = typeof conversationRepository.appendMessages === 'function'
                ? conversationRepository.appendMessages({conversationId: conversation.id, messages: rows, updatedAt: rows.at(-1).createdAt})
                : rows.map(row => conversationRepository.appendMessage(row));
            return stored.map(row => ({id: row.id, role: row.role, text: row.text, attachments: [], jobs: [], createdAt: row.created_at ?? row.createdAt, readAt: row.read_at ?? row.readAt}));
        }
    };
}

function createActivityProjection({activityRepository, jobRepository, id, now}) {
    return {
        findByEvent({personaId, eventId}) {
            return activityRepository.findActivityByEvent?.({eventId, personaId})
                ?? activityRepository.findActivity({id: eventId, personaId})
                ?? activityRepository.findActivity({activityId: eventId, personaId});
        },
        publish({personaId, eventId, content, media, createdAt = now()}) {
            const activity = activityRepository.insertActivity({
                id: id('activity'), personaId, eventId, content,
                mediaMode: media?.kind && media.kind !== 'none' ? media.kind : 'none',
                mediaStatus: media?.kind && media.kind !== 'none' ? 'queued' : 'none',
                createdAt
            });
            const frozen = media?.kind && media.kind !== 'none'
                && isRecord(media.personaMediaConcept)
                && Object.hasOwn(media, 'currentEvent')
                && Object.hasOwn(media, 'temporaryAppearance');
            if (activity?.id && media?.kind && media.kind !== 'none' && !frozen) {
                activityRepository.updateActivity?.({id: activity.id, activityId: activity.id, personaId, mediaStatus: 'failed'});
            }
            if (activity?.id && frozen && jobRepository?.enqueue) {
                jobRepository.enqueue({
                    id: id('job'), jobType: media.kind === 'video' ? 'activity_video' : 'activity_image',
                    personaId, activityId: activity.id, priority: 3, maxAttempts: 4,
                    runAfter: createdAt,
                    payload: {
                        activityId: activity.id, eventId, personaId, kind: media.kind,
                        request: media.request ?? '', count: media.count ?? 1,
                        personaMediaConcept: media.personaMediaConcept, capabilityCall: media
                    }
                });
            }
            return activity;
        }
    };
}

function createDeferredBatchPort(database, now, {jobRepository, id} = {}) {
    const nextId = typeof id === 'function' ? id : prefix => `${prefix}_${randomUUID()}`;
    return {
        findById({batchId, personaId}) {
            const row = database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ? AND persona_id = ?').get(batchId, personaId);
            if (!row) return null;
            return {...row, messageIds: parseJson(row.message_ids_json, [])};
        },
        findActive({personaId}) {
            const row = database.prepare(`
                SELECT * FROM companion_chat_deferred_batches
                WHERE persona_id = ? AND status IN ('queued', 'leased', 'processing')
                ORDER BY created_at DESC, id DESC LIMIT 1
            `).get(personaId);
            return row ? {...row, messageIds: parseJson(row.message_ids_json, [])} : null;
        },
        appendMessage({batchId, personaId, messageId, at = now()}) {
            const row = database.prepare(`
                SELECT * FROM companion_chat_deferred_batches
                WHERE id = ? AND persona_id = ? AND status IN ('queued', 'leased', 'processing')
            `).get(batchId, personaId);
            if (!row) return null;
            const ids = parseJson(row.message_ids_json, []);
            if (!ids.includes(messageId)) ids.push(messageId);
            database.prepare(`
                UPDATE companion_chat_deferred_batches SET message_ids_json = ?, updated_at = ?
                WHERE id = ? AND persona_id = ? AND status IN ('queued', 'leased', 'processing')
            `).run(JSON.stringify(ids), at, batchId, personaId);
            return database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ?').get(batchId);
        },
        create(input = {}) {
            const existing = database.prepare(`
                SELECT * FROM companion_chat_deferred_batches
                WHERE persona_id = ? AND batch_key = ? LIMIT 1
            `).get(input.personaId, input.batchKey);
            if (existing) return {batch: {...existing, messageIds: parseJson(existing.message_ids_json, [])}, job: null, created: false};
            const createdAt = input.createdAt ?? now();
            const updatedAt = input.updatedAt ?? createdAt;
            const messageIds = Array.isArray(input.messageIds) ? input.messageIds.filter(Boolean) : [];
            const decision = input.decision && typeof input.decision === 'object' ? input.decision : {};
            let job = null;
            database.transaction(() => {
                database.prepare(`
                    INSERT INTO companion_chat_deferred_batches
                        (id, persona_id, conversation_id, batch_key, status, deliver_at, decision_json,
                         message_ids_json, created_at, updated_at)
                    VALUES (?, ?, ?, ?, 'queued', ?, ?, ?, ?, ?)
                `).run(
                    input.id, input.personaId, input.conversationId, input.batchKey,
                    input.deliverAt, JSON.stringify(decision), JSON.stringify(messageIds), createdAt, updatedAt
                );
                const jobInput = input.job;
                job = jobInput && jobRepository?.enqueue ? jobRepository.enqueue(jobInput) : null;
            })();
            const batch = database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ?').get(input.id);
            return {batch: {...batch, messageIds}, job, created: true};
        },
        complete({batchId, personaId, at = now(), resultMessageId, messageIds}) {
            const result = database.prepare(`
                UPDATE companion_chat_deferred_batches
                SET status = 'complete', result_message_id = ?, message_ids_json = ?, updated_at = ?, completed_at = ?
                WHERE id = ? AND persona_id = ? AND status IN ('queued', 'leased', 'processing')
            `).run(resultMessageId, JSON.stringify(messageIds || []), at, at, batchId, personaId);
            return result.changes ? database.prepare('SELECT * FROM companion_chat_deferred_batches WHERE id = ?').get(batchId) : null;
        },
        expire({batchId, personaId, at = now()}) {
            return database.prepare(`UPDATE companion_chat_deferred_batches SET status = 'expired', updated_at = ? WHERE id = ? AND persona_id = ? AND status IN ('queued', 'leased', 'processing')`).run(at, batchId, personaId);
        },
        recordFailure({batchId, personaId, at = now(), error}) {
            return database.prepare(`UPDATE companion_chat_deferred_batches SET error = ?, attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND persona_id = ?`).run(String(error || '').slice(0, 500), at, batchId, personaId);
        }
    };
}

export function createProductionDeferredChatBatchRepository({database, jobRepository, clock, id} = {}) {
    const now = clockFor(clock);
    return createDeferredBatchPort(requireDatabase(database), now, {jobRepository, id: idFor(id)});
}

function createConversationMessages(conversationRepository) {
    return {
        listByIds({personaId, ids}) {
            return (Array.isArray(ids) ? ids : []).map(id => conversationRepository.findMessage({id, personaId})).filter(Boolean);
        }
    };
}

/** Compose the application flows used by the default proactive worker. */
export function createProductionProactiveFlows({database, repositories, provider, settings, clock, id, lifeWorld, contextReader} = {}) {
    const openDatabase = requireDatabase(database);
    const now = clockFor(clock);
    const generateId = idFor(id);
    const completion = providerCompletion(provider, settings);
    const replyCompletion = providerReply(provider, settings);
    const conversation = repositories.conversation ?? repositories.conversationRepository;
    const lifeEvent = repositories.lifeEvent ?? repositories.lifeEventRepository;
    const activity = repositories.activity ?? repositories.activityRepository;
    const pending = createPendingPort(openDatabase, repositories.job ?? repositories.jobRepository, now);
    const decision = createDecisionPort({database: openDatabase, jobRepository: repositories.job ?? repositories.jobRepository, completion, now, contextReader});
    const reply = createReplyProjection({database: openDatabase, conversationRepository: conversation, id: generateId, now});
    const activityProjection = createActivityProjection({activityRepository: activity, jobRepository: repositories.job ?? repositories.jobRepository, id: generateId, now});
    const deferredBatch = createDeferredBatchPort(openDatabase, now, {
        jobRepository: repositories.job ?? repositories.jobRepository,
        id: generateId
    });
    const conversationMessages = createConversationMessages(conversation);
    const replyComposer = {
        async compose({personaId, batch, messages, lifeWorld, at, command, signal} = {}) {
            if (!replyCompletion) throw new Error('MTPLX deferred reply provider is unavailable');
            const context = await readContext(contextReader, {command: {personaId, chatAt: at}, messages});
            return replyCompletion({
                messages: serializePromptMessages({
                    context,
                    messages,
                    instruction: 'Return one concise user-visible assistant reply. Preserve the supplied life-world facts and do not invent events.'
                }).concat([
                    {role: 'user', content: JSON.stringify({personaId, batch, messages, lifeWorld, at, command: {type: command?.type, jobId: command?.jobId}})}
                ]),
                signal
            });
        }
    };
    return {
        proactive_message: createProactiveMessageFlow({lifeEvent, pendingEvent: pending, decision, reply, lifeWorld, clock: now, idGenerator: generateId}),
        pending_event: createPendingEventWorkerFlow({lifeEvent, pendingEvent: pending, decision, reply, lifeWorld, clock: now, idGenerator: generateId}),
        activity_decision: createActivityDecisionFlow({lifeEvent, decision, activity: activityProjection, lifeWorld, clock: now, idGenerator: generateId}),
        deferred_chat_reply: createDeferredChatReplyFlow({
            deferredBatch,
            conversation: conversationMessages,
            reply,
            lifeWorld,
            replyComposer,
            lease: repositories.job ?? repositories.jobRepository,
            clock: now,
            idGenerator: generateId
        })
    };
}

export default createProductionProactiveFlows;
