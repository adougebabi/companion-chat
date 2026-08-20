import {normalizeSseEvent, sseDone, sseError, sseToken} from '../contracts/index.js';
import {normalizeBoundedChatResult} from '../application/chat-turn-flow.js';

const MAX_ERROR_LENGTH = 480;
const INTERNAL_RESULT_KEYS = new Set([
    'chatResult',
    'effects',
    'facts',
    'presentation',
    'projections',
    'type',
    'message',
    'messages',
    'learned',
    'jobs'
]);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function compactText(value, fallback) {
    const text = String(value ?? '').replace(/\s+/g, ' ').trim();
    if (!text) return fallback;
    if (text.length <= MAX_ERROR_LENGTH) return text;
    return `${text.slice(0, MAX_ERROR_LENGTH - 3)}...`;
}

function errorText(value) {
    if (value instanceof Error) return value.message;
    if (isRecord(value)) return value.error ?? value.message ?? value.code ?? value;
    return value;
}

function sinkLooksClosed(sink) {
    return Boolean(sink?.closed)
        || Boolean(sink?.destroyed)
        || Boolean(sink?.writableEnded)
        || Boolean(sink?.writableFinished);
}

function resolveFlowRun(chatTurnFlow) {
    if (typeof chatTurnFlow === 'function') return chatTurnFlow;
    for (const method of ['run', 'execute', 'runFlow']) {
        if (typeof chatTurnFlow?.[method] === 'function') return chatTurnFlow[method].bind(chatTurnFlow);
    }
    throw new TypeError('ChatTurnSseAdapter requires ChatTurnFlow.run()');
}

function resolveInvocation(first, second, third) {
    if (third !== undefined) {
        return {context: first ?? {}, command: second ?? {}, sink: third, request: null};
    }

    const envelope = first ?? {};
    const request = isRecord(envelope) && isRecord(envelope.request) ? envelope.request : envelope;
    const context = request.context ?? {};
    const command = request.command ?? (Object.hasOwn(request, 'personaId') || Object.hasOwn(request, 'text') ? request : {});
    const sink = second ?? request.sink ?? request.response ?? request.res ?? envelope.sink ?? envelope.response ?? envelope.res;
    return {context, command, sink, request: isRecord(envelope.request) ? envelope : request};
}

function resolveAbortSignal({context, command, request}) {
    return command?.signal ?? context?.signal ?? request?.signal ?? null;
}

function resolveAbortCallback({context, command, request}, configuredCallback) {
    if (typeof configuredCallback === 'function') return configuredCallback;
    for (const value of [request?.isRequestAborted, request?.requestAborted, context?.isRequestAborted, context?.requestAborted, command?.isRequestAborted, command?.requestAborted]) {
        if (typeof value === 'function') return value;
    }
    return null;
}

function resolveRequestTarget({context, command, request}) {
    for (const value of [request?.req, request?.request, context?.req, context?.request, command?.req, command?.request]) {
        if (value && typeof value.on === 'function') return value;
    }
    return null;
}

function addListener(target, event, listener, listeners) {
    if (!target || typeof target.on !== 'function') return;
    target.on(event, listener);
    listeners.push([target, event, listener]);
}

function removeListeners(listeners) {
    for (const [target, event, listener] of listeners) {
        target.removeListener?.(event, listener);
        target.off?.(event, listener);
    }
}

function combineSignals(inputSignal, bridgeSignal) {
    if (!inputSignal) return bridgeSignal;
    if (!bridgeSignal) return inputSignal;
    if (typeof AbortSignal?.any === 'function') return AbortSignal.any([inputSignal, bridgeSignal]);
    const controller = new AbortController();
    const abort = () => controller.abort();
    if (inputSignal.aborted || bridgeSignal.aborted) abort();
    else {
        inputSignal.addEventListener?.('abort', abort, {once: true});
        bridgeSignal.addEventListener?.('abort', abort, {once: true});
    }
    return controller.signal;
}

function donePayload(result) {
    const source = isRecord(result?.chatResult) ? result.chatResult : result;
    const bounded = normalizeBoundedChatResult(isRecord(source) ? source : {messages: []});
    const payload = {
        type: 'done',
        messages: bounded.messages,
        message: bounded.message,
        learned: bounded.learned,
        jobs: bounded.jobs
    };

    // `chatResult` is the only flow channel allowed to add browser-facing
    // presentation fields. Aggregate facts/effects remain application data.
    for (const [key, value] of Object.entries(bounded)) {
        if (!INTERNAL_RESULT_KEYS.has(key)) payload[key] = value;
    }
    return sseDone(payload);
}

function tokenEvents(result) {
    if (!Array.isArray(result?.presentation)) return [];
    return result.presentation.filter(event => isRecord(event) && event.type === 'token');
}

async function mappedError(error, errorMapper, invocation) {
    let mapped;
    try {
        mapped = typeof errorMapper === 'function'
            ? await errorMapper(error, invocation)
            : compactText(errorText(error), 'Chat turn failed');
    } catch {
        mapped = 'Chat turn failed';
    }
    return sseError(compactText(errorText(mapped), 'Chat turn failed'));
}

function isSignal(value) {
    return Boolean(value) && typeof value.aborted === 'boolean';
}

/**
 * Create the transport boundary for one chat turn.
 *
 * Route integration contract:
 *
 * ```js
 * const handleChat = createChatTurnSseAdapter({
 *   chatTurnFlow,
 *   sendSse: (res, event) => res.write(`data: ${JSON.stringify(event)}\\n\\n`),
 *   end: res => res.end(),
 *   errorMapper: error => ({error: userSafeMessage(error)})
 * });
 * await handleChat({context, command}, res);
 * ```
 *
 * `command` is passed to the injected flow as the command, `context` is
 * passed as flow context, and the sink is only written with normalized
 * `token`, `done`, or `error` events. The adapter never parses provider
 * chunks or dispatches capabilities.
 */
export function createChatTurnSseAdapter({chatTurnFlow, sendSse, end, errorMapper, isRequestAborted, requestAborted} = {}) {
    const runFlow = resolveFlowRun(chatTurnFlow);
    if (typeof sendSse !== 'function') throw new TypeError('ChatTurnSseAdapter requires sendSse()');
    if (typeof end !== 'function') throw new TypeError('ChatTurnSseAdapter requires end()');

    return async function handleChatTurn(first = {}, second, third) {
        const invocation = resolveInvocation(first, second, third);
        const {context, command, sink, request} = invocation;
        if (!isRecord(context)) throw new TypeError('Chat turn SSE context must be an object');
        if (!isRecord(command)) throw new TypeError('Chat turn SSE command must be an object');
        if (!sink) throw new TypeError('Chat turn SSE response sink is required');

        const listeners = [];
        const rawSignal = resolveAbortSignal(invocation);
        const inputSignal = isSignal(rawSignal) ? rawSignal : null;
        const lifecycleProbe = resolveAbortCallback(invocation, isRequestAborted ?? requestAborted);
        const requestTarget = resolveRequestTarget(invocation);
        const canObserveClose = typeof sink.on === 'function' || Boolean(requestTarget);
        const bridgeController = canObserveClose ? new AbortController() : null;
        const flowSignal = combineSignals(inputSignal, bridgeController?.signal);
        let clientClosed = sinkLooksClosed(sink);
        let ended = false;
        let transportFailure = null;

        const markClientClosed = () => {
            clientClosed = true;
            bridgeController?.abort();
        };
        addListener(sink, 'close', markClientClosed, listeners);
        addListener(sink, 'aborted', markClientClosed, listeners);
        addListener(requestTarget, 'aborted', markClientClosed, listeners);
        addListener(requestTarget, 'close', markClientClosed, listeners);
        if (isSignal(inputSignal)) inputSignal.addEventListener?.('abort', markClientClosed, {once: true});

        const clientHasClosed = () => {
            if (clientClosed || sinkLooksClosed(sink)) {
                markClientClosed();
                return true;
            }
            if (isSignal(inputSignal) && inputSignal.aborted) {
                markClientClosed();
                return true;
            }
            try {
                if (lifecycleProbe?.(invocation) === true) {
                    markClientClosed();
                    return true;
                }
            } catch {
                // A request lifecycle probe is advisory; it must not turn into
                // a browser-visible transport error.
            }
            return false;
        };

        const emit = async event => {
            if (clientHasClosed()) return false;
            const normalized = normalizeSseEvent(event);
            try {
                await sendSse(sink, normalized);
            } catch (error) {
                if (clientHasClosed()) return false;
                transportFailure = error;
                throw error;
            }
            return true;
        };

        const endOnce = async () => {
            if (ended) return;
            ended = true;
            removeListeners(listeners);
            if (isSignal(inputSignal)) inputSignal.removeEventListener?.('abort', markClientClosed);
            try {
                await end(sink);
            } catch {
                // The response may already be closed. Ending is best effort.
            }
        };

        try {
            if (clientHasClosed()) return null;
            const flowCommand = flowSignal && command.signal !== flowSignal
                ? {...command, signal: flowSignal}
                : command;
            const result = await runFlow({context, command: flowCommand});
            if (clientHasClosed()) return null;
            for (const event of tokenEvents(result)) {
                if (!(await emit(sseToken(event.token)))) return null;
            }
            if (clientHasClosed()) return null;
            await emit(donePayload(result));
            return result;
        } catch (error) {
            if (!clientHasClosed() && !transportFailure) await emit(await mappedError(error, errorMapper, invocation));
            return null;
        } finally {
            await endOnce();
        }
    };
}
