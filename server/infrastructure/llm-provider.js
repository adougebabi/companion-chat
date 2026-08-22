import {createHash} from 'node:crypto';

import {cleanUrl, boundedProviderError} from './provider-ports.js';
import {
    STRUCTURED_TURN_SCHEMA_VERSION,
    STRUCTURED_TURN_LIMITS,
    normalizeStructuredTurnSafely
} from '../contracts/index.js';

const MAX_DIAGNOSTICS = 8;
const MAX_DIAGNOSTIC_LENGTH = 240;

function boundedText(value, fallback = '') {
    const text = String(value ?? fallback).replace(/\s+/g, ' ').trim();
    return text.length <= MAX_DIAGNOSTIC_LENGTH ? text : `${text.slice(0, MAX_DIAGNOSTIC_LENGTH - 3)}...`;
}

function diagnostic(list, value) {
    if (list.length < MAX_DIAGNOSTICS) list.push(boundedText(value));
}

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function sidecarCandidate(payload) {
    const choice = payload?.choices?.[0] || {};
    const delta = choice.delta || {};
    const message = choice.message || {};
    const fields = [
        isRecord(delta.content) ? delta.content : undefined,
        delta.structuredTurn,
        delta.structured_turn,
        delta.structuredSidecar,
        delta.structured,
        delta.controlPayload,
        delta.control_payload,
        delta.parsed,
        delta.control,
        message.structuredTurn,
        message.structured_turn,
        message.structuredSidecar,
        message.structured,
        message.controlPayload,
        message.control_payload,
        message.parsed,
        message.control,
        choice.structuredTurn,
        choice.structured_turn,
        choice.structuredSidecar,
        choice.structured,
        choice.controlPayload,
        choice.control_payload,
        choice.parsed,
        choice.control,
        payload?.structuredTurn,
        payload?.structured_turn,
        payload?.structuredSidecar,
        payload?.structured,
        payload?.controlPayload,
        payload?.control_payload,
        payload?.parsed,
        payload?.control,
        payload?.metadata?.structuredTurn,
        payload?.metadata?.control
    ];
    return fields.find(candidate => candidate !== undefined && candidate !== null) ?? null;
}

function parseSidecar(value, diagnostics) {
    if (value === null || value === undefined) return null;
    if (typeof value === 'string') {
        try { return JSON.parse(value); }
        catch { diagnostic(diagnostics, 'structured sidecar contains invalid JSON'); return null; }
    }
    if (!isRecord(value)) {
        diagnostic(diagnostics, 'structured sidecar must be an object');
        return null;
    }
    return value;
}

function sidecarText(sidecar, fallback) {
    if (typeof sidecar?.text === 'string') return sidecar.text;
    if (Array.isArray(sidecar?.messages)) {
        const text = sidecar.messages
            .filter(message => !message?.role || message.role === 'assistant')
            .map(message => message?.text ?? message?.content)
            .filter(value => typeof value === 'string')
            .join('');
        if (text) return text;
    }
    return fallback;
}

function normalizeTurnRecord({text, tokens, toolCalls, sidecar, parseErrors, doneSeen, personaId, causationId, sourceMode} = {}) {
    const candidateSidecar = isRecord(sidecar) ? sidecar : {};
    const control = isRecord(candidateSidecar.control)
        ? candidateSidecar.control
        : {
            affectEvents: candidateSidecar.affectEvents,
            driveSignals: candidateSidecar.driveSignals,
            memoryWrites: candidateSidecar.memoryWrites,
            capabilityCalls: candidateSidecar.capabilityCalls
        };
    const visible = sidecarText(candidateSidecar, text || '');
    const candidate = {
        schemaVersion: isRecord(sidecar) ? candidateSidecar.schemaVersion : STRUCTURED_TURN_SCHEMA_VERSION,
        text: visible,
        tokens: Array.isArray(candidateSidecar.tokens) ? candidateSidecar.tokens : (tokens || []),
        messages: Array.isArray(candidateSidecar.messages) ? candidateSidecar.messages : [],
        control: {
            affectEvents: control.affectEvents ?? [],
            driveSignals: control.driveSignals ?? [],
            memoryWrites: control.memoryWrites ?? [],
            capabilityCalls: [
                ...(Array.isArray(toolCalls) ? toolCalls : []),
                ...(Array.isArray(control.capabilityCalls) ? control.capabilityCalls : [])
            ].slice(0, STRUCTURED_TURN_LIMITS.capabilityCalls)
        },
        parseDiagnostics: Array.isArray(parseErrors) ? parseErrors.slice(0, MAX_DIAGNOSTICS) : [],
        sourceMode: sourceMode
            ?? (isRecord(sidecar) ? 'structured_sidecar' : toolCalls?.length ? 'native_tools' : 'text')
    };
    const result = normalizeStructuredTurnSafely(candidate, {personaId, causationId});
    if (result.ok) return result.value;
    return {
        ...result.value,
        parseDiagnostics: [...result.value.parseDiagnostics, ...(Array.isArray(parseErrors) ? parseErrors : [])].slice(0, MAX_DIAGNOSTICS),
        sourceMode: sourceMode ?? (toolCalls?.length ? 'native_tools' : 'text')
    };
}

function appendToolCallFragment(toolCalls, fragment, diagnostics) {
    const explicitIndex = Number.isInteger(fragment?.index) && fragment.index >= 0 ? fragment.index : null;
    const fragmentId = String(fragment?.id || '').trim();
    const name = String(fragment?.function?.name || '').trim();
    const argumentsFragment = String(fragment?.function?.arguments || '');
    if (Number.isInteger(fragment?.index) && fragment.index < 0) {
        diagnostic(diagnostics, 'tool_call index must be a non-negative integer');
        toolCalls.push({index: explicitIndex, id: fragmentId, function: {name, arguments: argumentsFragment}, malformed: true});
        return toolCalls.at(-1);
    }

    let slot = null;
    if (fragmentId) {
        const idSlot = toolCalls.findIndex(call => call?.id === fragmentId);
        if (idSlot >= 0) {
            const existingIndex = toolCalls[idSlot]?.index;
            if (explicitIndex !== null && existingIndex !== null && existingIndex !== undefined && existingIndex !== explicitIndex) {
                diagnostic(diagnostics, 'tool_call index does not match its id');
                toolCalls.push({index: explicitIndex, id: fragmentId, function: {name, arguments: argumentsFragment}, malformed: true});
                return toolCalls.at(-1);
            }
            slot = idSlot;
        }
    }
    if (slot === null && explicitIndex !== null) {
        const existing = toolCalls[explicitIndex];
        if (existing?.id && fragmentId && existing.id !== fragmentId) {
            diagnostic(diagnostics, 'tool_call index contains multiple ids');
            toolCalls.push({index: explicitIndex, id: fragmentId, function: {name, arguments: argumentsFragment}, malformed: true});
            return toolCalls.at(-1);
        }
        slot = explicitIndex;
    }
    if (slot === null) {
        const unindexed = toolCalls.map((call, index) => call && call.index === null ? index : -1).filter(index => index >= 0);
        if (!fragmentId && unindexed.length > 1) {
            diagnostic(diagnostics, 'tool_call without index/id cannot be assigned');
            toolCalls.push({index: null, id: '', function: {name, arguments: argumentsFragment}, malformed: true});
            return toolCalls.at(-1);
        }
        slot = unindexed[0] ?? toolCalls.length;
    }

    const current = toolCalls[slot] || {index: explicitIndex, id: '', type: 'function', function: {name: '', arguments: ''}};
    if ((current.index === undefined || current.index === null) && explicitIndex !== null) current.index = explicitIndex;
    if (!current.id && fragmentId) current.id = fragmentId;
    current.type = fragment?.type || current.type || 'function';
    if (!current.function.name && name) current.function.name = name;
    current.function.arguments += argumentsFragment;
    toolCalls[slot] = current;
    return current;
}

function idempotencyKey({personaId, causationId, name, id, argumentsText}) {
    const digest = createHash('sha256').update(String(argumentsText || '')).digest('hex').slice(0, 24);
    return `native:${personaId || 'unknown'}:${causationId || 'unknown'}:${name}:${id || 'anonymous'}:${digest}`.slice(0, 240);
}

function normalizeToolCalls(toolCalls, {personaId, causationId} = {}) {
    return toolCalls.filter(Boolean).map((call, index) => {
        const name = String(call.name || call.function?.name || '').trim();
        const argumentsText = String(call.argumentsText ?? call.function?.arguments ?? '');
        const normalizedIndex = Number.isInteger(call.index) && call.index >= 0 ? call.index : index;
        let args = null;
        if (argumentsText) {
            try { args = JSON.parse(argumentsText); } catch { /* dispatcher reports malformed JSON */ }
        }
        return {
            id: call.id || null,
            index: normalizedIndex,
            name,
            argumentsText,
            arguments: args,
            source: 'native',
            personaId: personaId || '',
            causationUserMessageId: causationId || '',
            idempotencyKey: idempotencyKey({personaId, causationId, name, id: call.id, argumentsText}),
            ...(call.malformed ? {incomplete: true, error: 'Malformed native tool call'} : {})
        };
    });
}

/**
 * Decode one OpenAI-compatible streaming response at the infrastructure
 * boundary. Application flows receive normalized token/tool records and never
 * need to know about SSE framing or provider-specific delta shapes.
 */
export async function consumeMtplxStream(response, {onText, personaId, causationId, signal} = {}) {
    if (!response?.body?.getReader) throw new Error('模型服务未返回可读取的流');
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    const toolCalls = [];
    const parseErrors = [];
    let buffer = '';
    let text = '';
    let finishReason = null;
    let doneSeen = false;
    let firstSeen = 0;
    let structuredSidecar = null;
    const processPayload = async raw => {
        if (!raw) return;
        if (raw === '[DONE]') { doneSeen = true; return; }
        let payload;
        try { payload = JSON.parse(raw); } catch { diagnostic(parseErrors, '模型流包含无效 SSE JSON'); return; }
        const sidecar = parseSidecar(sidecarCandidate(payload), parseErrors);
        if (sidecar) structuredSidecar = sidecar;
        const choice = payload?.choices?.[0] || {};
        const delta = choice.delta || {};
        if (choice.finish_reason) finishReason = boundedText(choice.finish_reason);
        const token = typeof delta.content === 'string' ? delta.content : '';
        if (token) { text += token; await onText?.(token); }
        for (const fragment of (Array.isArray(delta.tool_calls) ? delta.tool_calls : [])) {
            const call = appendToolCallFragment(toolCalls, fragment, parseErrors);
            if (call && call.firstSeen === undefined) call.firstSeen = firstSeen++;
        }
        for (const call of (Array.isArray(choice.message?.tool_calls) ? choice.message.tool_calls : [])) {
            const collected = appendToolCallFragment(toolCalls, call, parseErrors);
            if (collected && collected.firstSeen === undefined) collected.firstSeen = firstSeen++;
        }
    };
    const abort = () => { try { reader.cancel(); } catch {} };
    signal?.addEventListener?.('abort', abort, {once: true});
    try {
        while (true) {
            const {value, done} = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, {stream: true});
            const lines = buffer.split(/\r?\n/);
            buffer = lines.pop() || '';
            for (const line of lines) if (line.startsWith('data:')) await processPayload(line.slice(5).trim());
        }
        buffer += decoder.decode();
        if (buffer.startsWith('data:')) await processPayload(buffer.slice(5).trim());
    } finally {
        signal?.removeEventListener?.('abort', abort);
    }
    if (!doneSeen) diagnostic(parseErrors, '模型流缺少 [DONE]');
    const normalized = normalizeToolCalls(toolCalls, {personaId, causationId});
    const incompleteToolIndexes = normalized
        .filter(call => (!doneSeen || !call.name || !call.argumentsText || call.incomplete))
        .map(call => call.index);
    const turn = normalizeTurnRecord({
        text,
        tokens: text ? [text] : [],
        toolCalls: normalized,
        sidecar: structuredSidecar,
        parseErrors,
        doneSeen,
        personaId,
        causationId,
        sourceMode: structuredSidecar ? 'structured_sidecar' : normalized.length ? 'native_tools' : undefined
    });
    return {
        text: turn.text,
        tokens: text ? [text] : turn.tokens,
        toolCalls: normalized,
        finishReason,
        doneSeen,
        parseErrors: turn.parseDiagnostics,
        incompleteToolIndexes,
        structuredTurn: turn,
        control: turn.control,
        sourceMode: turn.sourceMode
    };
}

function completionFromJson(payload, {personaId, causationId} = {}) {
    const choice = payload?.choices?.[0] || {};
    const message = choice.message || {};
    const parseErrors = [];
    let sidecar = parseSidecar(sidecarCandidate(payload), parseErrors);
    if (!sidecar && isRecord(message.content)
        && (message.content.schemaVersion || message.content.control || message.content.affectEvents || message.content.memoryWrites)) {
        sidecar = message.content;
    }
    if (!sidecar && typeof message.content === 'string' && message.content.trim().startsWith('{')) {
        try {
            const parsed = JSON.parse(message.content);
            if (isRecord(parsed) && (parsed.schemaVersion || parsed.control || parsed.affectEvents || parsed.memoryWrites || parsed.capabilityCalls)) {
                sidecar = parsed;
            }
        } catch {
            // A normal text completion may begin with a brace; only the
            // structured envelope shape is treated as a control sidecar.
        }
    }
    const messageText = typeof message.content === 'string' ? message.content : '';
    const text = sidecar ? sidecarText(sidecar, messageText) : messageText;
    const calls = (Array.isArray(message.tool_calls) ? message.tool_calls : []).map((call, index) => ({
        ...call,
        index: Number.isInteger(call?.index) ? call.index : index,
        name: call?.function?.name,
        argumentsText: call?.function?.arguments || ''
    }));
    const toolCalls = normalizeToolCalls(calls, {personaId, causationId});
    const turn = normalizeTurnRecord({
        text,
        tokens: text ? [text] : [],
        toolCalls,
        sidecar,
        parseErrors,
        doneSeen: true,
        personaId,
        causationId,
        sourceMode: sidecar ? 'structured_sidecar' : toolCalls.length ? 'native_tools' : undefined
    });
    return {
        text: turn.text,
        tokens: turn.tokens,
        toolCalls,
        finishReason: choice.finish_reason || null,
        doneSeen: true,
        parseErrors: turn.parseDiagnostics,
        incompleteToolIndexes: [],
        structuredTurn: turn,
        control: turn.control,
        sourceMode: turn.sourceMode
    };
}

function assertSettings(settings) {
    if (typeof settings !== 'function') throw new TypeError('MTPLX provider requires settings()');
    return settings;
}

function responseError(status, body) {
    const message = body?.error?.message || body?.message || `模型服务 HTTP ${status}`;
    return new Error(boundedProviderError(message, `模型服务 HTTP ${status}`));
}

/**
 * Adapter for the OpenAI-compatible local MTPLX endpoint. Construction is
 * side-effect free; network calls happen only from stream/models operations.
 */
export function createMtplxProvider({settings, fetchImpl, promptRuns, clock = () => new Date().toISOString(), idGenerator} = {}) {
    const readSettings = assertSettings(settings);
    if (fetchImpl !== undefined && typeof fetchImpl !== 'function') throw new TypeError('MTPLX provider fetchImpl must be a function');

    const nextId = typeof idGenerator === 'function'
        ? idGenerator
        : prefix => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;

    function startTrace(payload = {}) {
        if (!promptRuns || payload.trace === false) return null;
        try {
            const trace = payload.trace && typeof payload.trace === 'object' ? payload.trace : {};
            return promptRuns.start?.({
                id: nextId('prompt'),
                personaId: trace.personaId ?? null,
                jobId: trace.jobId ?? null,
                messageId: trace.messageId ?? null,
                operation: String(trace.operation || 'chat').slice(0, 80),
                model: String(payload.model || '').slice(0, 160),
                request: payload
            }) ?? null;
        } catch {
            return null;
        }
    }

    function finishTrace(runId, patch = {}) {
        if (!runId || !promptRuns) return;
        try { promptRuns.finish?.(runId, {...patch, completedAt: clock()}); } catch { /* diagnostics must not fail the provider */ }
    }

    async function captureResponse(runId, response) {
        if (!runId || !promptRuns) return;
        if (typeof response?.clone !== 'function') {
            finishTrace(runId, {status: 'submitted'});
            return;
        }
        try {
            const copy = response.clone();
            const contentType = copy.headers?.get?.('content-type') || '';
            const body = /json/i.test(contentType)
                ? await copy.json()
                : typeof copy.text === 'function' ? await copy.text() : null;
            finishTrace(runId, {status: 'completed', response: body});
        } catch {
            // A provider response may not support cloning (notably some local
            // test adapters). Keep the request trace without consuming the
            // caller-owned stream.
            finishTrace(runId, {status: 'submitted'});
        }
    }

    async function request(path, {body, signal, method = 'GET', allowErrorResponse = false} = {}) {
        const fetcher = fetchImpl ?? globalThis.fetch;
        if (typeof fetcher !== 'function') throw new TypeError('MTPLX provider requires fetch()');
        const config = readSettings();
        const headers = body === undefined ? {} : {'Content-Type': 'application/json'};
        if (config.lmStudioApiKey) headers.Authorization = `Bearer ${config.lmStudioApiKey}`;
        const response = await fetcher(`${cleanUrl(config.lmStudioUrl)}${path}`, {
            method,
            headers,
            ...(body === undefined ? {} : {body: JSON.stringify(body)}),
            ...(signal ? {signal} : {})
        });
        if (!response.ok && !allowErrorResponse) {
            let payload = null;
            try { payload = await response.json(); } catch { /* bounded fallback below */ }
            throw responseError(response.status, payload);
        }
        return response;
    }

    return Object.freeze({
        id: 'mtplx',
        label: 'MTPLX',
        portType: 'llm-streaming',
        capabilities: ['stream'],
        async stream(requestPayload = {}) {
            const {signal, trace: _trace, ...body} = requestPayload;
            const traceId = startTrace(requestPayload);
            try {
                const response = await request('/chat/completions', {method: 'POST', body, signal, allowErrorResponse: true});
                if (!response.ok) {
                    let payload = null;
                    try {
                        const copy = typeof response.clone === 'function' ? response.clone() : response;
                        payload = await copy?.json?.();
                    } catch { /* bounded HTTP fallback below */ }
                    const error = responseError(response.status, payload);
                    finishTrace(traceId, {status: 'failed', error: error.message, response: payload});
                    throw error;
                }
                void captureResponse(traceId, response);
                return response;
            } catch (error) {
                finishTrace(traceId, {status: 'failed', error: boundedProviderError(error)});
                throw error;
            }
        },
        async models() {
            const response = await request('/models');
            return response.json();
        }
    });
}

/**
 * Convert the raw MTPLX provider port into the normalized application stream
 * port. The provider remains responsible for HTTP/SSE parsing; the
 * application sees only completion records and token callbacks.
 */
export function createMtplxCompletionPort({provider, settings, tools = [], toolChoice = 'auto', temperature = 0.75} = {}) {
    const port = provider;
    if (!port || typeof port.stream !== 'function') throw new TypeError('MTPLX completion port requires provider.stream()');
    const readSettings = typeof settings === 'function' ? settings : () => ({});
    return Object.freeze({
        async stream(input = {}) {
            const personaId = input.personaId || input.command?.personaId || '';
            const causationId = input.causationId || input.command?.causationId || '';
            const model = input.model || readSettings().model || '';
            const requestedTools = input.tools ?? tools;
            const requestedToolChoice = input.toolChoice ?? input.tool_choice ?? toolChoice;
            const payload = {
                ...input,
                model,
                stream: true,
                ...(Array.isArray(requestedTools) && requestedTools.length ? {tools: requestedTools} : {}),
                ...(requestedToolChoice === undefined ? {} : {tool_choice: requestedToolChoice}),
                ...(temperature === undefined ? {} : {temperature})
            };
            delete payload.context;
            delete payload.command;
            delete payload.personaId;
            delete payload.requestId;
            delete payload.correlationId;
            delete payload.causationId;
            delete payload.onToken;
            delete payload.toolChoice;
            const response = await port.stream({...payload, signal: input.signal});
            if (!response?.ok) {
                let body = null;
                try { body = await response?.json?.(); } catch {}
                throw responseError(response?.status || 502, body);
            }
            const contentType = response.headers?.get?.('content-type') || '';
            if (response.body?.getReader && /text\/event-stream/i.test(contentType)) {
                return consumeMtplxStream(response, {
                    personaId,
                    causationId,
                    signal: input.signal,
                    onText: input.onToken
                });
            }
            return completionFromJson(await response.json(), {personaId, causationId});
        }
    });
}

export const createMtplxStreamingPort = createMtplxCompletionPort;

const JSON_COMPLETION_TIMEOUT_MS = 20_000;
const MAX_JSON_COMPLETION_CONTENT = 24_000;

function jsonCompletionError(message, status = 502, code = 'MODEL_JSON_COMPLETION_FAILED') {
    return Object.assign(new Error(boundedText(message, '模型服务请求失败')), {status, code});
}

function jsonContentFromResponse(payload) {
    if (typeof payload === 'string') return payload;
    const choice = payload?.choices?.[0] ?? {};
    const message = choice.message ?? {};
    if (typeof message.content === 'string') return message.content;
    if (typeof choice.content === 'string') return choice.content;
    if (typeof payload?.content === 'string') return payload.content;
    if (isRecord(message.content)) return JSON.stringify(message.content);
    if (isRecord(payload) && (payload.answers || payload.inferredFields || payload.blueprint)) return JSON.stringify(payload);
    return '';
}

/**
 * Dedicated non-streaming JSON transport for application-owned extraction
 * ports. It deliberately returns assistant content and leaves schema parsing
 * to the application layer.
 */
export function createMtplxJsonCompletionPort({provider, settings, timeoutMs = JSON_COMPLETION_TIMEOUT_MS} = {}) {
    if (!provider || typeof provider.stream !== 'function') throw new TypeError('MTPLX JSON completion port requires provider.stream()');
    const readSettings = typeof settings === 'function' ? settings : () => ({});
    const boundedTimeout = Number.isFinite(Number(timeoutMs)) && Number(timeoutMs) > 0
        ? Math.min(Number(timeoutMs), 120_000)
        : JSON_COMPLETION_TIMEOUT_MS;

    async function complete(input = {}) {
        const controller = new AbortController();
        const callerSignal = input.signal;
        let callerAbort;
        let callerAbortListener;
        if (callerSignal?.aborted) controller.abort(callerSignal.reason);
        else if (callerSignal?.addEventListener) {
            callerAbort = () => controller.abort(callerSignal.reason);
            callerSignal.addEventListener('abort', callerAbort, {once: true});
        }
        let timeoutHandle;
        const timeoutPromise = new Promise((_, reject) => {
            timeoutHandle = setTimeout(() => {
                controller.abort();
                reject(jsonCompletionError('模型分析请求超时', 502, 'MODEL_JSON_COMPLETION_TIMEOUT'));
            }, boundedTimeout);
        });
        const abortPromise = callerSignal?.aborted
            ? Promise.reject(jsonCompletionError('模型分析请求已取消', 502, 'MODEL_JSON_COMPLETION_ABORTED'))
            : callerSignal?.addEventListener
                ? new Promise((_, reject) => {
                    callerAbortListener = () => reject(jsonCompletionError('模型分析请求已取消', 502, 'MODEL_JSON_COMPLETION_ABORTED'));
                    callerSignal.addEventListener('abort', callerAbortListener, {once: true});
                })
                : null;
        try {
            const {signal: _signal, trace, stream: _stream, ...rest} = input;
            const payload = {
                ...rest,
                model: input.model || readSettings().model || '',
                stream: false,
                signal: controller.signal,
                trace: trace === undefined ? false : trace
            };
            const operation = (async () => {
                const response = await provider.stream(payload);
                if (response?.ok === false) {
                    let body = null;
                    try { body = await response.json?.(); } catch { /* bounded fallback */ }
                    throw jsonCompletionError(responseError(response.status || 502, body).message);
                }
                const body = typeof response?.json === 'function' ? await response.json() : response;
                const content = jsonContentFromResponse(body).trim();
                if (!content) throw jsonCompletionError('模型服务返回空内容');
                if (content.length > MAX_JSON_COMPLETION_CONTENT) throw jsonCompletionError('模型服务返回内容过长');
                return content;
            })();
            const content = await Promise.race([
                operation,
                timeoutPromise,
                ...(abortPromise ? [abortPromise] : [])
            ]);
            return Object.freeze({content, text: content});
        } catch (error) {
            if (controller.signal.aborted) {
                if (callerSignal?.aborted) throw jsonCompletionError('模型分析请求已取消', 502, 'MODEL_JSON_COMPLETION_ABORTED');
                throw jsonCompletionError('模型分析请求超时', 502, 'MODEL_JSON_COMPLETION_TIMEOUT');
            }
            if (error?.status === 502) throw error;
            throw jsonCompletionError(error?.message || '模型服务请求失败');
        } finally {
            clearTimeout(timeoutHandle);
            if (callerAbort && callerSignal?.removeEventListener) callerSignal.removeEventListener('abort', callerAbort);
            if (callerAbortListener && callerSignal?.removeEventListener) callerSignal.removeEventListener('abort', callerAbortListener);
        }
    }

    return Object.freeze({complete, completeJson: complete, json: complete});
}

export default createMtplxProvider;
