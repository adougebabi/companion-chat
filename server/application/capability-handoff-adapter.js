import {createCapabilityDispatcher} from './capability-dispatcher.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function argumentsFor(call) {
    if (isRecord(call?.arguments)) return call.arguments;
    if (typeof call?.argumentsText === 'string' && call.argumentsText.trim()) {
        try { return JSON.parse(call.argumentsText); } catch { return null; }
    }
    return null;
}

function resultFor(attempt) {
    const call = attempt?.call || {};
    const name = call.name || attempt?.name || 'pending_event';
    return {
        name,
        ok: !attempt?.error,
        callId: call.id ?? null,
        idempotencyKey: call.idempotencyKey || `${name}:${call.index ?? 0}`,
        result: attempt?.result ?? null,
        error: attempt?.error ?? null
    };
}

function entryFor(flow, planCommand) {
    if (!flow || typeof flow.plan !== 'function' || typeof flow.apply !== 'function') return null;
    return {
        cardinality: 1,
        execute({call, mode, personaId, causationUserMessageId}) {
            const args = argumentsFor(call);
            if (!isRecord(args)) throw new Error('Capability arguments must be a JSON object');
            const command = planCommand({args, call, personaId, causationUserMessageId});
            const plan = flow.plan(command);
            if (mode === 'plan') return {plan, result: plan.previewResult ?? plan.preview ?? null};
            return flow.apply(plan);
        }
    };
}

/** Adapt the application capability parser/registry to the backend handoff port. */
export function createCapabilityHandoffAdapter({registry, capabilityRegistry} = {}) {
    const source = registry ?? capabilityRegistry;
    if (!source) return null;
    const dispatcher = createCapabilityDispatcher({registry: source});
    return Object.freeze({
        names: dispatcher.names,
        dispatch(input = {}) {
            const context = input.context ?? {};
            const output = dispatcher.dispatch({
                mode: input.mode ?? 'execute',
                calls: Array.isArray(input.calls) ? input.calls : [],
                markerText: input.markerText ?? '',
                completion: input.completion ?? {doneSeen: true},
                personaId: context.personaId ?? input.personaId,
                causationUserMessageId: context.causationId ?? input.causationUserMessageId
            });
            return {
                results: output.attempts.map(resultFor),
                effects: output.attempts.flatMap(attempt => attempt?.intent ? [attempt.intent] : []),
                visibleText: output.visibleText,
                diagnostics: output.diagnostics
            };
        }
    });
}

export function createFlowCapabilityRegistry({pendingEventFlow, sceneEventFlow, mediaFlow} = {}) {
    const registry = {};
    const pending = entryFor(pendingEventFlow, ({args, call, personaId, causationUserMessageId}) => ({
        personaId, call: args, sourceMessageId: causationUserMessageId ?? call.causationUserMessageId,
        provenance: {source: call.source ?? 'native', callId: call.id, idempotencyKey: call.idempotencyKey, causationUserMessageId: causationUserMessageId ?? call.causationUserMessageId}
    }));
    const scene = entryFor(sceneEventFlow, ({args, call, personaId, causationUserMessageId}) => ({
        personaId, call: args, sourceMessageId: causationUserMessageId ?? call.causationUserMessageId,
        provenance: {source: call.source ?? 'native', callId: call.id, idempotencyKey: call.idempotencyKey}
    }));
    const media = entryFor(mediaFlow, ({args, call, personaId, causationUserMessageId}) => ({
        personaId, call: args, sourceMessageId: causationUserMessageId ?? call.causationUserMessageId,
        provenance: {source: call.source ?? 'native', callId: call.id, idempotencyKey: call.idempotencyKey, causationUserMessageId: causationUserMessageId ?? call.causationUserMessageId}
    }));
    if (pending) registry.pending_event = pending;
    if (scene) registry.scene_event = scene;
    if (media) registry.media_event = media;
    return registry;
}

export default createCapabilityHandoffAdapter;
