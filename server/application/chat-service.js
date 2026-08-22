import {createChatTurnFlow} from './chat-turn-flow.js';
import {createChatTurnSseAdapter} from '../http/chat-turn-sse-adapter.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredCommand(value) {
    if (!isRecord(value)) throw new TypeError('Companion chat command must be an object');
    return value;
}

function requestTarget(value) {
    if (value && typeof value.on === 'function') return value;
    return null;
}

function resolveCommitBoundary(value) {
    if (value === undefined || value === null) return value;
    if (typeof value === 'function') return value;
    if (isRecord(value)) {
        if (typeof value.commit === 'function') return value.commit.bind(value);
        if (typeof value.commitStepResult === 'function') return value.commitStepResult.bind(value);
    }
    throw new TypeError('Chat commit boundary must provide commit()');
}

/**
 * Keep request-scoped transport state at the application boundary. The chat
 * flow receives the same stable context shape for direct callers and HTTP
 * callers, while the SSE adapter remains the owner of lifecycle observation.
 */
function normalizeRequestContext(command, response, suppliedContext = {}, transport = {}) {
    const supplied = isRecord(suppliedContext) ? suppliedContext : {};
    const request = requestTarget(transport.request)
        ?? requestTarget(command.request)
        ?? requestTarget(command.req)
        ?? requestTarget(supplied.request)
        ?? requestTarget(supplied.req)
        ?? null;
    const context = {
        ...supplied,
        request: supplied.request ?? transport.request ?? request,
        response: supplied.response ?? transport.response ?? response,
        req: supplied.req ?? transport.req ?? request,
        res: supplied.res ?? transport.res ?? response
    };

    for (const field of ['requestId', 'correlationId', 'causationId', 'subjectId', 'personaId']) {
        if (context[field] === undefined && command[field] !== undefined) context[field] = command[field];
    }
    for (const field of ['signal', 'isRequestAborted', 'requestAborted']) {
        if (context[field] === undefined && transport[field] !== undefined) context[field] = transport[field];
        if (context[field] === undefined && command[field] !== undefined) context[field] = command[field];
    }
    if (context.personaId === undefined && command.personaId !== undefined) context.personaId = command.personaId;
    return context;
}

function normalizeInvocation(value, response) {
    const source = requiredCommand(value);
    const isEnvelope = isRecord(source.command)
        && (isRecord(source.context)
            || source.request !== undefined
            || source.req !== undefined
            || source.response !== undefined
            || source.res !== undefined
            || source.sink !== undefined);
    const command = isEnvelope ? requiredCommand(source.command) : source;
    const suppliedContext = isEnvelope ? source.context : command.context;
    const transport = isEnvelope
        ? {
            request: source.request ?? source.req,
            req: source.req ?? source.request,
            response: source.response ?? source.res,
            res: source.res ?? source.response,
            signal: source.signal,
            isRequestAborted: source.isRequestAborted,
            requestAborted: source.requestAborted
        }
        : {};
    return {
        command,
        context: normalizeRequestContext(command, response, suppliedContext, transport)
    };
}

/**
 * Compose the application chat flow and its SSE transport boundary.
 *
 * Construction only validates and wires injected ports. Database access,
 * provider calls, network activity, and response writes happen in `handle`.
 */
export function createCompanionChatService({
    contextReader,
    llmStreamingPort,
    capabilityDispatcher,
    conversationRepository,
    registry,
    executor,
    presentationMapper,
    affectFlow,
    structuredTurnControl,
    userMessageWriter,
    chatPolicy,
    deferredChatPolicy,
    enableContinuation,
    commitBoundary,
    sendSse,
    end,
    errorMapper
} = {}) {
    const resolvedCommitBoundary = resolveCommitBoundary(commitBoundary);
    const chatTurnFlow = createChatTurnFlow({
        contextReader,
        llmStreamingPort,
        capabilityDispatcher,
        conversationRepository,
        presentationMapper,
        affectFlow,
        structuredTurnControl,
        registry,
        executor,
        userMessageWriter,
        chatPolicy: chatPolicy ?? deferredChatPolicy,
        enableContinuation,
        commitBoundary: resolvedCommitBoundary
    });
    const sseAdapter = createChatTurnSseAdapter({
        chatTurnFlow,
        sendSse,
        end,
        errorMapper
    });

    async function handle(command, response) {
        if (!response) throw new TypeError('Companion chat response is required');
        const normalized = normalizeInvocation(command, response);
        const context = normalized.context;
        return sseAdapter({
            context,
            command: normalized.command,
            req: context.req,
            response: context.response,
            res: context.res,
            sink: response
        }, response);
    }

    return Object.freeze({chatTurnFlow, sseAdapter, handle});
}

export default createCompanionChatService;
