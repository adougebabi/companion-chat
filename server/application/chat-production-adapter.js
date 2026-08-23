import {createCapabilityDispatcher} from './capability-dispatcher.js';
import {serializePromptMessages} from './context-pipeline.js';

const MAX_HISTORY = 18;
const MAX_MESSAGE_LENGTH = 8_000;
const MAX_SENTENCES = 20;
const ASSISTANT_MESSAGE_FACT = 'conversation.assistant_message';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function callable(value, methods, field, {optional = false} = {}) {
    if (typeof value === 'function') return value;
    if (isRecord(value)) {
        for (const method of methods) {
            if (typeof value[method] === 'function') return value[method].bind(value);
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${methods.join('() or ')}()`);
}

function text(value, field, {allowEmpty = false, max = 240} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    if (value.length > max) throw new RangeError(`${field} exceeds ${max} characters`);
    if (!allowEmpty && !value.trim()) throw new TypeError(`${field} must not be empty`);
    return value;
}

function clockFunction(value) {
    const clock = callable(value, ['now'], 'Chat production clock');
    return () => clock();
}

function idFunction(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.next === 'function') return value.next.bind(value);
    let sequence = 0;
    return prefix => `${prefix}_${Date.now()}_${sequence++}`;
}

function isoTimestamp(value, field = 'Chat message timestamp') {
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError(`${field} must be a valid timestamp`);
    return date.toISOString();
}

function timestampAt(base, index) {
    const date = new Date(base);
    date.setTime(date.getTime() + index);
    return date.toISOString();
}

function sentenceEnding(value) {
    return /[\u4e00-\u9fff]/.test(value) ? '。' : '.';
}

function ensureSentenceEnding(value) {
    const text = value.trim();
    return /[。！？!?。.!?]$/.test(text) ? text : `${text}${sentenceEnding(text)}`;
}

/**
 * Keep the user-visible reply contract in the application layer. This is
 * deliberately independent from persistence and only creates deterministic
 * message intents for the commit adapter to materialize.
 */
export function splitChatAssistantReply(value, fallback = '') {
    const source = String(value || '').replace(/\s+/g, ' ').trim() || fallback;
    const sentences = [];
    let remaining = source;
    const sentence = /^\s*([\s\S]*?[。！？!?]+(?:[”’」』）】]*)?)/;
    while (remaining && sentences.length < MAX_SENTENCES) {
        const match = remaining.match(sentence);
        if (!match || !match[1].trim()) {
            const trailing = remaining.trim();
            if (trailing) sentences.push(ensureSentenceEnding(trailing));
            break;
        }
        sentences.push(match[1].trim());
        remaining = remaining.slice(match[0].length).trimStart();
    }
    return sentences.filter(Boolean).map(item => item.slice(0, MAX_MESSAGE_LENGTH));
}

function contextDate(command = {}, context = {}, fallbackNow) {
    const candidate = command.chatAt ?? context.chatAt ?? fallbackNow();
    const date = candidate instanceof Date ? candidate : new Date(candidate);
    return Number.isFinite(date.getTime()) ? date : new Date(fallbackNow());
}

/**
 * Adapt the legacy context calculation as a pure application port. The
 * calculation itself is supplied by the composition root, so this module
 * owns only request timing and argument normalization.
 */
export function createChatContextReader({contextFor, read, clock = () => new Date().toISOString()} = {}) {
    const delegate = typeof contextFor === 'function'
        ? input => contextFor(input.personaId, input.at, input)
        : callable(read, ['read', 'readContext'], 'Chat context reader');
    const now = clockFunction(clock);
    return Object.freeze({
        read(input = {}) {
            const command = isRecord(input.command) ? input.command : {};
            const context = isRecord(input.context) ? input.context : {};
            const personaId = input.personaId ?? command.personaId ?? context.personaId;
            if (typeof personaId !== 'string' || !personaId.trim()) throw new TypeError('Chat context personaId must be a non-empty string');
            const at = contextDate(command, context, now);
            return delegate({
                ...input,
                personaId,
                at,
                chatAt: at.toISOString(),
                command,
                context,
                messages: Array.isArray(input.messages) ? input.messages.slice(-MAX_HISTORY) : []
            });
        }
    });
}

function defaultPrompt({context, messages}) {
    return serializePromptMessages({context, messages, maxHistory: MAX_HISTORY});
}

/**
 * Adapt a completion function into the normalized LlmStreamingPort. It does
 * not parse upstream payloads; the supplied completion function already owns
 * that transport concern and may return either normalized completion data or
 * an async iterable of normalized chunks.
 */
export function createChatLlmStreamingPort({stream, complete, llmStreamingPort, promptSerializer, tools = [], toolChoice = 'auto', temperature = 0.75} = {}) {
    const delegate = callable(stream ?? complete ?? llmStreamingPort, ['stream', 'streamCompletion', 'streamChat', 'complete'], 'Chat LLM completion');
    const serialize = typeof promptSerializer === 'function' ? promptSerializer : defaultPrompt;
    return Object.freeze({
        stream(input = {}) {
            const messages = serialize(input);
            return delegate({
                ...input,
                messages,
                ...(Array.isArray(tools) && tools.length ? {tools} : {}),
                ...(toolChoice !== undefined ? {tool_choice: toolChoice} : {}),
                ...(temperature !== undefined ? {temperature} : {})
            });
        }
    });
}

function capabilityPresentation(value) {
    if (!isRecord(value) || value.type !== 'capability-result' || !isRecord(value.result)) return null;
    const result = value.result;
    if (!result.ok) return {error: result.error || '能力调用未执行'};
    const raw = result.result;
    if (value.result.name === 'media_event' && isRecord(raw)) {
        return {
            ok: true,
            jobId: raw.jobId ?? null,
            jobIds: Array.isArray(raw.jobIds) ? raw.jobIds.slice(0, 3) : [],
            messageId: raw.message?.id ?? raw.messageId ?? null,
            kind: raw.kind ?? null,
            provider: raw.provider ?? null,
            count: raw.count ?? 1,
            replayed: raw.replayed === true
        };
    }
    if (value.result.name === 'pending_event' && isRecord(raw)) {
        return {ok: true, pendingEventId: raw.pendingEvent?.id ?? raw.pendingEventId ?? null, jobId: raw.jobId ?? null, created: raw.created === true};
    }
    return raw ?? {ok: true};
}

function presentationByCapability(value) {
    const result = {};
    for (const event of Array.isArray(value) ? value : []) {
        const capability = event?.result?.name;
        const presentation = capabilityPresentation(event);
        if (!capability || presentation === null) continue;
        if (capability === 'scene_event') result.sceneEvent = presentation;
        if (capability === 'appearance_event') result.appearanceEvent = presentation;
        if (capability === 'media_event') result.mediaEvent = presentation;
        if (capability === 'pending_event') result.pendingEvent = presentation;
    }
    return result;
}

function messageIntent({personaId, text: messageText, id, createdAt}) {
    return {
        id,
        role: 'assistant',
        text: messageText,
        attachments: [],
        generation: null,
        jobs: [],
        createdAt,
        readAt: null,
        proactiveEventId: null,
        proactivePendingEventId: null
    };
}

/**
 * Map a normalized completion to browser DTOs and commit facts. The mapper
 * never writes storage; its facts are consumed by the conversation adapter.
 */
export function createChatPresentationMapper({clock = () => new Date().toISOString(), idGenerator, fallback, splitReply = splitChatAssistantReply} = {}) {
    const now = clockFunction(clock);
    const nextId = idFunction(idGenerator);
    return function mapChatPresentation(input = {}) {
        const command = isRecord(input.command) ? input.command : {};
        const completion = isRecord(input.completion) ? input.completion : {};
        const personaId = command.personaId ?? input.personaId;
        if (typeof personaId !== 'string' || !personaId.trim()) throw new TypeError('Chat presentation personaId must be a non-empty string');
        const existing = Array.isArray(input.messages) ? input.messages.filter(isRecord) : [];
        const replyText = input.visibleText ?? completion.text ?? (Array.isArray(completion.tokens) ? completion.tokens.join('') : '');
        const base = isoTimestamp(now());
        const messages = existing.length
            ? existing.slice(0, MAX_SENTENCES)
            : splitReply(replyText, fallback).map((item, index) => messageIntent({
                personaId,
                text: item,
                id: nextId('message'),
                createdAt: timestampAt(base, index)
            }));
        const facts = existing.length ? [] : [{
            type: ASSISTANT_MESSAGE_FACT,
            personaId,
            messages
        }];
        return {
            chatResult: {
                type: 'done',
                messages,
                learned: [],
                jobs: [],
                ...presentationByCapability(input.capabilityPresentation)
            },
            facts,
            projections: [],
            effects: [],
            presentation: []
        };
    };
}

/**
 * Compose the safe application ports used by production chat. Callers still
 * inject the context calculation, normalized completion, and registry.
 */
export function createChatProductionPorts({
    contextFor,
    contextReader,
    stream,
    complete,
    llmStreamingPort,
    promptSerializer,
    tools,
    toolChoice,
    temperature,
    capabilityDispatcher,
    capabilityRegistry,
    registry,
    markerCallFactory,
    presentationMapper,
    clock,
    idGenerator,
    fallback
} = {}) {
    const resolvedContext = contextReader ?? (contextFor ? createChatContextReader({contextFor, clock}) : null);
    const resolvedLlm = llmStreamingPort ?? ((stream || complete) ? createChatLlmStreamingPort({stream, complete, promptSerializer, tools, toolChoice, temperature}) : null);
    const resolvedDispatcher = capabilityDispatcher ?? (capabilityRegistry || registry
        ? createCapabilityDispatcher({registry: capabilityRegistry ?? registry, markerCallFactory})
        : null);
    const resolvedPresentation = presentationMapper ?? createChatPresentationMapper({clock, idGenerator, fallback});
    return Object.freeze({
        ...(resolvedContext ? {contextReader: resolvedContext} : {}),
        ...(resolvedLlm ? {llmStreamingPort: resolvedLlm} : {}),
        ...(resolvedDispatcher ? {capabilityDispatcher: resolvedDispatcher} : {}),
        presentationMapper: resolvedPresentation
    });
}

export const ASSISTANT_MESSAGE_FACT_TYPE = ASSISTANT_MESSAGE_FACT;
export const createChatContextPort = createChatContextReader;
export const createChatStreamingPort = createChatLlmStreamingPort;
export const createChatPresentationPort = createChatPresentationMapper;
