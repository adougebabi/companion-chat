import {createCapabilityDispatcher} from '../../server/application/capability-dispatcher.js';
import {createCompanionApplication} from '../../server/application/companion-application.js';
import {createMediaFlow} from '../../server/application/media-flow.js';
import {createPendingEventFlow} from '../../server/application/pending-event-flow.js';
import {createSceneEventFlow} from '../../server/application/scene-event-flow.js';
import {createCompanionTestContext} from '../../server/testing/companion-context.js';

const NOW = '2026-08-21T00:00:00.000Z';

function mediaConcept(kind) {
    return {
        schemaVersion: 1,
        mediaKind: kind,
        scene: 'test scene',
        action: 'test action',
        mood: 'calm',
        narrative: 'test media concept',
        humanSubjects: [{label: 'persona', role: 'subject', inFrame: true}],
        nonHumanObjects: [{label: 'window', kind: 'environment', inFrame: true}],
        capture: {mode: 'external_capture', operator: 'friend', deviceVisibility: 'out_of_frame', framingIntent: 'natural medium shot'},
        compositionIntent: 'preserve the relationship between subject and environment'
    };
}

export function mediaCall(request = 'a natural photo') {
    return {
        schemaVersion: 2,
        kind: 'image',
        request,
        count: 1,
        personaMediaConcept: mediaConcept('image'),
        currentEvent: null,
        temporaryAppearance: {}
    };
}

export function sceneCall() {
    return {
        operation: 'start',
        location: 'quiet cafe',
        room: 'window seat',
        activity: 'talking',
        situation: 'talking together by the cafe window',
        mood: 'calm',
        objects: ['tea'],
        participants: ['user', 'persona']
    };
}

export function pendingCall(suffix = 'chat-plan') {
    return {
        schemaVersion: 1,
        summary: 'follow up after the interview',
        notBefore: '2026-08-21T00:01:00.000Z',
        expiresAt: '2026-08-21T02:00:00.000Z',
        dedupeKey: `${suffix}-pending`
    };
}

function stateRepository(database) {
    return {
        read(personaId) {
            return database.prepare('SELECT * FROM companion_persona_states WHERE persona_id = ?').get(personaId);
        },
        updateProjection({personaId, situation, mood, checkpointAt, updatedAt, sourceEventId, sharedSceneJson, expected}) {
            return database.prepare(`
                UPDATE companion_persona_states
                SET situation = ?, mood = ?, checkpoint_at = ?, updated_at = ?, source_event_id = ?, shared_scene_json = ?
                WHERE persona_id = ? AND source_event_id IS ? AND shared_scene_json = ?
            `).run(
                situation,
                mood,
                checkpointAt,
                updatedAt,
                sourceEventId,
                sharedSceneJson,
                personaId,
                expected.sourceEventId || null,
                expected.sharedSceneJson || '{}'
            );
        }
    };
}

function capabilityProvenance(call, source) {
    return {
        source,
        ...(call.id ? {callId: call.id} : {}),
        ...(call.idempotencyKey ? {idempotencyKey: call.idempotencyKey} : {})
    };
}

function commandFor(call, source, personaId) {
    return {
        personaId: call.personaId ?? personaId,
        call: call.arguments,
        sourceMessageId: call.causationUserMessageId ?? source.id,
        provenance: capabilityProvenance(call, call.source === 'marker' ? 'marker' : 'native')
    };
}

function markerAdapter(name, token, valueFactory) {
    return (text, context) => {
        if (!text.includes(token)) return {text, arguments: null};
        return {
            text: text.replace(token, ''),
            arguments: valueFactory(),
            idempotencyKey: `marker-${name}-${context.personaId || 'unknown'}`
        };
    };
}

function identityResult({error, result}) {
    return error ? {error} : result;
}

function capabilityCall(name, argumentsValue, index, personaId, sourceMessageId, idempotencyKey, overrides = {}) {
    return {
        id: `call_${name}_${index}`,
        index,
        name,
        argumentsText: JSON.stringify(argumentsValue),
        arguments: argumentsValue,
        source: 'native',
        personaId,
        causationUserMessageId: sourceMessageId,
        idempotencyKey,
        ...overrides
    };
}

export function createCapabilityPlanFixture() {
    let sequence = 0;
    const context = createCompanionTestContext({
        clock: () => NOW,
        idGenerator: prefix => `${prefix}_capability_plan_${++sequence}`
    });
    const {database, repositories} = context;
    const sceneLifeEventRepository = {
        ...repositories.lifeEvent,
        findByIdempotencyKey({personaId, idempotencyKey}) {
            return database.prepare(`
                SELECT * FROM companion_life_events
                WHERE persona_id = ? AND json_extract(payload_json, '$.idempotencyKey') = ?
                ORDER BY created_at, id LIMIT 1
            `).get(personaId, idempotencyKey);
        }
    };
    const state = stateRepository(database);
    const flowRepositories = {
        ...repositories,
        sourceMessage: repositories.conversation,
        personaRepository: repositories.persona,
        conversationRepository: repositories.conversation,
        pendingEventRepository: repositories.pending,
        jobRepository: repositories.job,
        lifeEventRepository: sceneLifeEventRepository,
        stateRepository: state
    };
    const transaction = work => database.transaction(work)();
    const pendingEventFlow = createPendingEventFlow({
        repositories: flowRepositories,
        clock: context.clock,
        idGenerator: context.id,
        transaction
    });
    const sceneEventFlow = createSceneEventFlow({
        repositories: flowRepositories,
        clock: context.clock,
        idGenerator: context.id,
        scheduledState: () => ({situation: 'following the regular plan', mood: 'calm'}),
        transaction
    });
    const mediaFlow = createMediaFlow({
        repositories: flowRepositories,
        clock: context.clock,
        idGenerator: context.id,
        normalizeCall(value) {
            return {...value, count: value.count ?? 1};
        },
        mediaConceptEnvelopeFor(_persona, {kind}) {
            return {schemaVersion: 1, mediaKind: kind, scene: 'test scene', action: 'test action'};
        },
        providerFor: () => 'comfyui',
        transaction
    });

    const registry = {
        pending_event: {
            cardinality: 1,
            markerAdapter: markerAdapter('pending_event', '<pending-event>', () => pendingCall()),
            execute({call, mode, personaId, causationUserMessageId}) {
                const plan = pendingEventFlow.plan(commandFor(call, {id: causationUserMessageId}, personaId));
                return mode === 'plan' ? {plan, result: plan.previewResult} : pendingEventFlow.apply(plan);
            },
            result: identityResult
        },
        media_event: {
            cardinality: 1,
            markerAdapter: markerAdapter('media_event', '<media-intent>', () => mediaCall()),
            execute({call, mode, personaId, causationUserMessageId}) {
                const plan = mediaFlow.plan({
                    ...commandFor(call, {id: causationUserMessageId}, personaId),
                    provenance: {...capabilityProvenance(call, call.source === 'marker' ? 'marker' : 'native'), causationUserMessageId}
                });
                return mode === 'plan' ? {plan, result: plan.previewResult} : mediaFlow.apply(plan);
            },
            result: identityResult
        },
        scene_event: {
            cardinality: 1,
            markerAdapter: markerAdapter('scene_event', '<scene-event>', () => sceneCall()),
            execute({call, mode, personaId, causationUserMessageId}) {
                const plan = sceneEventFlow.plan(commandFor(call, {id: causationUserMessageId}, personaId));
                return mode === 'plan' ? {plan, result: plan.previewResult} : sceneEventFlow.apply(plan);
            },
            result: identityResult
        }
    };
    const capabilityDispatcher = createCapabilityDispatcher({registry});
    const application = createCompanionApplication({
        repositories: flowRepositories,
        pendingEventFlow,
        mediaFlow,
        sceneEventFlow,
        capabilityDispatcher,
        routeHandlers: {}
    });

    function createPersona(name = 'capability plan persona') {
        return context.createPersona({name, role: 'companion'});
    }

    function appendUserMessage(personaId, text = 'remember these capability calls') {
        const conversation = repositories.conversation.getOrCreateConversation({
            personaId,
            id: `conversation_${personaId}`,
            createdAt: context.clock(),
            updatedAt: context.clock()
        });
        return repositories.conversation.appendMessage({
            id: `message_${personaId}`,
            conversationId: conversation.id,
            role: 'user',
            text,
            attachmentsJson: '[]',
            generationJson: null,
            jobsJson: '[]',
            proactiveEventId: null,
            proactivePendingEventId: null,
            createdAt: context.clock(),
            readAt: context.clock()
        });
    }

    function callsFor(personaId, sourceMessageId, suffix = 'native') {
        return [
            capabilityCall('pending_event', pendingCall(`${suffix}-pending`), 0, personaId, sourceMessageId, `${suffix}-pending-key`),
            capabilityCall('media_event', mediaCall(), 1, personaId, sourceMessageId, `${suffix}-media-key`),
            capabilityCall('scene_event', sceneCall(), 2, personaId, sourceMessageId, `${suffix}-scene-key`)
        ];
    }

    function counts(personaId) {
        return {
            scenes: database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type IN ('shared_scene', 'shared_scene_end')").get(personaId).count,
            pending: database.prepare('SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?').get(personaId).count,
            jobs: database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type IN ('chat_image', 'chat_video', 'pending_event')").get(personaId).count,
            messages: database.prepare('SELECT COUNT(*) AS count FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ?').get(personaId).count
        };
    }

    function dispatch(options = {}) {
        return application.capabilityDispatcher.dispatch(options);
    }

    return Object.freeze({
        ...context,
        application,
        database,
        repositories,
        pendingEventFlow,
        mediaFlow,
        sceneEventFlow,
        capabilityDispatcher,
        createPersona,
        appendUserMessage,
        callsFor,
        counts,
        dispatch,
        markerText: '<pending-event><media-intent><scene-event>visible text'
    });
}

export {capabilityCall};

