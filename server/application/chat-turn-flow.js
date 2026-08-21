import {
    emptyStepResult,
    normalizeCapabilityCall,
    normalizeChatResult,
    normalizeStepResult,
    sseToken
} from '../contracts/index.js';
import {createCapabilityHandoffStep, createFlowRegistry} from './flow-registry.js';
import {createFlowExecutor} from './flow-executor.js';

export const CHAT_TURN_FLOW_ID = 'chat-turn';
export const CHAT_TURN_FLOW_VERSION = 1;

const MAX_HISTORY = 18;
const MAX_MESSAGES = 20;
const MAX_COLLECTION_ITEMS = 50;
const MAX_MESSAGE_TEXT = 12_000;
const MAX_TOKEN_TEXT = 12_000;
const FLOW_RUNTIME = Symbol('chat-turn-runtime');
const CONTINUATION_FALLBACK = '我已经记下这件事，但暂时没能组织出更多回复。';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isPromiseLike(value) {
    return Boolean(value) && (typeof value === 'object' || typeof value === 'function') && typeof value.then === 'function';
}

function boundedText(value, field, maxLength, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    if (value.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    if (!allowEmpty && !value.trim()) throw new TypeError(`${field} must not be empty`);
    return value;
}

function requiredObject(value, field) {
    if (!isRecord(value)) throw new TypeError(`${field} must be an object`);
    return value;
}

function resolvePortMethod(port, methods, field, {optional = false} = {}) {
    if (typeof port === 'function') return port;
    if (isRecord(port)) {
        for (const method of methods) {
            if (typeof port[method] === 'function') return port[method].bind(port);
        }
    }
    if (optional) return null;
    throw new TypeError(`${field} must provide ${methods.join('() or ')}()`);
}

function normalizeCommand(value) {
    const command = requiredObject(value ?? {}, 'Chat turn command');
    const personaId = boundedText(command.personaId, 'Chat turn command personaId', 160);
    const text = typeof command.text === 'string'
        ? boundedText(command.text, 'Chat turn command text', MAX_MESSAGE_TEXT, {allowEmpty: true})
        : '';
    const attachments = command.attachments === undefined
        ? []
        : Array.isArray(command.attachments)
            ? command.attachments.slice(0, 8)
            : (() => { throw new TypeError('Chat turn command attachments must be an array'); })();
    return {...command, personaId, text, attachments};
}

function normalizeHistory(value) {
    if (Array.isArray(value)) return value.slice(-MAX_HISTORY);
    if (isRecord(value) && Array.isArray(value.items)) return value.items.slice(-MAX_HISTORY);
    return [];
}

function normalizeCallList(value) {
    if (value === undefined || value === null) return [];
    if (!Array.isArray(value)) throw new TypeError('Chat completion toolCalls must be an array');
    return value.map(normalizeCapabilityCall);
}

function normalizeCompletion(value = {}) {
    const completion = requiredObject(value, 'LLM completion');
    const tokens = Array.isArray(completion.tokens)
        ? completion.tokens.map((token, index) => boundedText(token, `LLM completion token ${index}`, MAX_TOKEN_TEXT, {allowEmpty: true}))
        : [];
    const text = completion.text === undefined || completion.text === null
        ? tokens.join('')
        : boundedText(completion.text, 'LLM completion text', MAX_MESSAGE_TEXT, {allowEmpty: true});
    const toolCalls = normalizeCallList(completion.toolCalls ?? completion.capabilityCalls);
    const stepResult = isRecord(completion.stepResult)
        ? normalizeStepResult(completion.stepResult)
        : emptyStepResult();
    return {
        ...completion,
        text,
        tokens,
        toolCalls,
        stepResult,
        doneSeen: completion.doneSeen === undefined ? true : Boolean(completion.doneSeen)
    };
}

/**
 * Consume an already-normalized LLM stream. The transport adapter owns SSE
 * decoding and provider chunk parsing; this helper only joins normalized
 * token/completion records exposed by the injected LlmStreamingPort.
 */
async function completionFromStream(value) {
    const stream = isPromiseLike(value) ? await value : value;
    if (!stream || typeof stream[Symbol.asyncIterator] !== 'function') return normalizeCompletion(stream);

    const tokens = [];
    let completion = {};
    for await (const item of stream) {
        const chunk = requiredObject(item, 'Normalized LLM stream item');
        if (chunk.type === 'token') {
            tokens.push(boundedText(chunk.token ?? '', 'Normalized LLM token', MAX_TOKEN_TEXT, {allowEmpty: true}));
            continue;
        }
        if (chunk.type === 'completion' && isRecord(chunk.completion)) {
            completion = {...completion, ...chunk.completion};
            continue;
        }
        completion = {...completion, ...chunk};
    }
    return normalizeCompletion({...completion, tokens: tokens.length ? tokens : completion.tokens});
}

function boundedValue(value, depth = 0) {
    if (value === null || value === undefined) return value;
    if (typeof value === 'string') return value.length <= MAX_MESSAGE_TEXT ? value : `${value.slice(0, MAX_MESSAGE_TEXT - 3)}...`;
    if (typeof value !== 'object') return value;
    if (depth > 4) return '[bounded]';
    if (Array.isArray(value)) return value.slice(0, MAX_COLLECTION_ITEMS).map(item => boundedValue(item, depth + 1));
    const result = {};
    for (const [key, child] of Object.entries(value).slice(0, MAX_COLLECTION_ITEMS)) {
        result[key] = boundedValue(child, depth + 1);
    }
    return result;
}

function boundedMessage(value) {
    if (!isRecord(value)) return boundedValue(value);
    const message = boundedValue(value);
    if (typeof message.text === 'string' && message.text.length > MAX_MESSAGE_TEXT) {
        message.text = `${message.text.slice(0, MAX_MESSAGE_TEXT - 3)}...`;
    }
    if (Array.isArray(message.attachments)) message.attachments = message.attachments.slice(0, 8);
    if (Array.isArray(message.jobs)) message.jobs = message.jobs.slice(0, MAX_COLLECTION_ITEMS);
    return message;
}

/**
 * Normalize the result presented to the transport layer. The authoritative
 * ordered collection is `messages`; `message` is always derived from its
 * first item for the legacy chat client.
 */
export function normalizeBoundedChatResult(value = {}) {
    const normalized = normalizeChatResult(requiredObject(value, 'ChatResult'));
    const messages = normalized.messages.slice(0, MAX_MESSAGES).map(boundedMessage);
    return {
        ...boundedValue(normalized),
        type: normalized.type ?? 'done',
        messages,
        message: messages[0] ?? null,
        learned: normalized.learned.slice(0, MAX_COLLECTION_ITEMS).map(item => boundedValue(item)),
        jobs: normalized.jobs.slice(0, MAX_COLLECTION_ITEMS).map(item => boundedValue(item))
    };
}

function resultChannels(value) {
    if (!isRecord(value)) return emptyStepResult();
    if (!['facts', 'projections', 'effects', 'presentation'].every(channel => Array.isArray(value[channel]))) {
        return emptyStepResult();
    }
    return normalizeStepResult(value);
}

function resolveChatPolicy(value) {
    if (value === undefined || value === null) return null;
    if (typeof value === 'function') return value;
    if (!isRecord(value)) throw new TypeError('Chat policy must be a function or object');
    for (const method of ['evaluate', 'apply', 'run', 'handle']) {
        if (value[method] !== undefined) {
            if (typeof value[method] !== 'function') throw new TypeError(`Chat policy.${method} must be a function`);
            return value[method].bind(value);
        }
    }
    throw new TypeError('Chat policy must provide evaluate()');
}

function policyHandled(runtime) {
    return runtime.policy?.handled === true;
}

function modelMessage(message) {
    if (!isRecord(message)) return null;
    return {
        role: message.role === 'assistant' ? 'assistant' : message.role === 'tool' ? 'tool' : 'user',
        content: message.content ?? message.text ?? ''
    };
}

function continuationMessages(history, completion, presentation) {
    const messages = Array.isArray(history) ? history.map(modelMessage).filter(Boolean) : [];
    const calls = Array.isArray(completion.toolCalls) ? completion.toolCalls : [];
    const results = Array.isArray(presentation)
        ? presentation
            .filter(event => event?.type === 'capability-result' && isRecord(event.result))
            .map(event => event.result)
        : [];
    const assistantToolCalls = calls.map((call, index) => ({
        id: call.id || `call_${call.index ?? index}`,
        type: 'function',
        function: {
            name: call.name,
            arguments: call.argumentsText || '{}'
        }
    }));
    if (!assistantToolCalls.length) return {messages, calls, results};
    messages.push({role: 'assistant', content: '', tool_calls: assistantToolCalls});
    for (const [index, call] of calls.entries()) {
        const id = call.id || `call_${call.index ?? index}`;
        const result = results.find(item => item.callId === call.id || item.name === call.name) ?? {
            name: call.name,
            ok: false,
            callId: call.id ?? null,
            error: 'Capability result unavailable'
        };
        messages.push({role: 'tool', tool_call_id: id, content: JSON.stringify(result)});
    }
    return {messages, calls, results};
}

function invokeMapper(mapper, input) {
    if (typeof mapper === 'function') return mapper(input);
    return mapper.map(input);
}

function invocationFor(first, second) {
    if (isRecord(first) && (Object.hasOwn(first, 'command') || Object.hasOwn(first, 'context'))) {
        return {context: first.context ?? {}, command: first.command ?? {}};
    }
    if (second === undefined && isRecord(first) && (Object.hasOwn(first, 'personaId') || Object.hasOwn(first, 'text'))) {
        return {context: {}, command: first};
    }
    return {context: first ?? {}, command: second ?? {}};
}

async function repositoryHistory(repository, input) {
    const reader = resolvePortMethod(repository, ['listMessages', 'readMessages', 'read'], 'ConversationRepository', {optional: true});
    if (!reader) return [];
    const value = await reader({
        personaId: input.command.personaId,
        conversationId: input.command.conversationId,
        limit: Math.min(Number(input.command.historyLimit) || MAX_HISTORY, MAX_HISTORY),
        cursor: null
    });
    // The table-scoped repository returns raw rows newest-first. The model
    // context must be chronological, matching the public conversation helper.
    const rows = Array.isArray(value) ? value : value?.items;
    return Array.isArray(rows) ? rows.slice(-MAX_HISTORY).reverse() : [];
}

function registerChatTurnFlow({registry, contextReader, llmStream, capabilityDispatcher, conversationRepository, presentationMapper, userMessageWriter, chatPolicy, enableContinuation = false, flowId}) {
    const capabilityHandoff = createCapabilityHandoffStep({dispatcher: capabilityDispatcher});
    registry.register({
        id: flowId,
        version: CHAT_TURN_FLOW_VERSION,
        layer: 'application',
        dependencies: [
            {id: 'backend-contracts', layer: 'contracts'},
            {id: 'flow-runtime', layer: 'domain'}
        ],
        steps: [
            ...(userMessageWriter ? [{
                id: 'user-message-boundary',
                layer: 'application',
                dependencies: [{id: 'conversation-repository', layer: 'contracts'}],
                async run(_context, command) {
                    const runtime = command[FLOW_RUNTIME];
                    const message = await userMessageWriter({
                        personaId: command.personaId,
                        text: command.text,
                        attachments: command.attachments,
                        command
                    });
                    runtime.userMessage = message ?? null;
                    if (message?.id && !command.causationId) command.causationId = message.id;
                    return emptyStepResult();
                }
            }] : []),
            ...(chatPolicy ? [{
                id: 'deferred-chat-policy',
                layer: 'application',
                dependencies: [{id: 'conversation-repository', layer: 'contracts'}],
                async run(context, command) {
                    const runtime = command[FLOW_RUNTIME];
                    const value = await chatPolicy({
                        context,
                        command,
                        personaId: command.personaId,
                        text: command.text,
                        chatAt: command.chatAt ?? context.chatAt,
                        userMessage: runtime.userMessage,
                        message: runtime.userMessage
                    });
                    if (!value || value.handled !== true) return emptyStepResult();
                    runtime.policy = value;
                    runtime.chatResult = normalizeBoundedChatResult(value.chatResult ?? {messages: value.messages ?? []});
                    runtime.messages = runtime.chatResult.messages.slice();
                    return resultChannels(value);
                }
            }] : []),
            {
                id: 'conversation-context',
                layer: 'application',
                dependencies: [{id: 'conversation-repository', layer: 'contracts'}],
                async run(context, command) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    runtime.history = normalizeHistory(await repositoryHistory(conversationRepository, {context, command}));
                    return emptyStepResult();
                }
            },
            {
                id: 'context-loader',
                layer: 'application',
                dependencies: [{id: 'context-reader', layer: 'domain'}],
                async run(context, command) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    runtime.context = await contextReader({
                        context,
                        command,
                        messages: runtime.history,
                        personaId: command.personaId,
                        requestId: context.requestId ?? null,
                        correlationId: context.correlationId ?? null,
                        causationId: context.causationId ?? command.causationId ?? null
                    });
                    return emptyStepResult();
                }
            },
            {
                id: 'llm-stream',
                layer: 'application',
                dependencies: [{id: 'llm-streaming-port', layer: 'contracts'}],
                async run(context, command) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    const response = await llmStream({
                        context: runtime.context ?? {},
                        messages: runtime.history,
                        command,
                        personaId: command.personaId,
                        requestId: context.requestId ?? null,
                        correlationId: context.correlationId ?? null,
                        causationId: context.causationId ?? command.causationId ?? null,
                        signal: command.signal
                    });
                    runtime.completion = await completionFromStream(response);
                    const tokens = runtime.completion.tokens.length
                        ? runtime.completion.tokens
                        : runtime.completion.text ? [runtime.completion.text] : [];
                    const markerText = /<(?:media-intent|pending-event|scene-event)>/i.test(runtime.completion.text || '');
                    const capabilityText = runtime.completion.toolCalls.length > 0;
                    return {
                        ...runtime.completion.stepResult,
                        presentation: runtime.completion.stepResult.presentation.concat(markerText || (capabilityText && enableContinuation) ? [] : tokens.map(token => sseToken(token)))
                    };
                }
            },
            {
                id: 'capability-handoff',
                layer: 'application',
                dependencies: [{id: 'capability-dispatcher', layer: 'contracts'}],
                async run(context, command, previous) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    const calls = runtime.completion?.toolCalls?.length
                        ? runtime.completion.toolCalls
                        : Array.isArray(command.capabilityCalls) ? command.capabilityCalls : [];
                    const causationId = calls[0]?.causationUserMessageId
                        ?? context.causationId
                        ?? command.causationId
                        ?? null;
                    const handoffContext = {...context, causationId};
                    const handoffCommand = {
                        ...command,
                        capabilityCalls: calls,
                        markerText: runtime.completion?.text ?? '',
                        completion: runtime.completion ?? {},
                        causationId
                    };
                    const result = await capabilityHandoff.run(handoffContext, handoffCommand, previous);
                    runtime.capabilityPresentation = result.presentation.slice();
                    const visible = result.presentation.find(event => event?.type === 'capability-visible-text');
                    if (visible) runtime.visibleText = visible.text;
                    return result;
                }
            },
            ...(enableContinuation ? [{
                id: 'capability-continuation',
                layer: 'application',
                dependencies: [{id: 'llm-streaming-port', layer: 'contracts'}],
                async run(context, command, previous) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    const completion = runtime.completion ?? {};
                    const calls = Array.isArray(completion.toolCalls)
                        ? completion.toolCalls.filter(call => call?.source === 'native' && call?.name)
                        : [];
                    if (!calls.length || completion.doneSeen !== true || completion.incompleteToolIndexes?.length) {
                        return emptyStepResult();
                    }

                    const continuation = continuationMessages(runtime.history, completion, previous.presentation);
                    if (!continuation.calls.length) return emptyStepResult();

                    try {
                        const response = await llmStream({
                            context: runtime.context ?? {},
                            messages: continuation.messages,
                            command: {...command, toolChoice: 'none', tool_choice: 'none'},
                            personaId: command.personaId,
                            requestId: context.requestId ?? null,
                            correlationId: context.correlationId ?? null,
                            causationId: context.causationId ?? command.causationId ?? null,
                            signal: command.signal,
                            toolChoice: 'none',
                            tool_choice: 'none',
                            continuation: true
                        });
                        const next = await completionFromStream(response);
                        if (next.toolCalls.length) {
                            runtime.completion = normalizeCompletion({
                                ...next,
                                text: CONTINUATION_FALLBACK,
                                tokens: [CONTINUATION_FALLBACK],
                                toolCalls: [],
                                doneSeen: true,
                                parseErrors: [...(next.parseErrors || []), 'Continuation attempted another capability call']
                            });
                        } else {
                            runtime.completion = next;
                        }
                    } catch (error) {
                        void error;
                        // Durable capability effects are already committed by
                        // their flow adapters; a continuation failure only
                        // changes the bounded user-visible fallback.
                        runtime.completion = normalizeCompletion({
                            text: CONTINUATION_FALLBACK,
                            tokens: [CONTINUATION_FALLBACK],
                            toolCalls: [],
                            doneSeen: true
                        });
                    }
                    return {
                        facts: [],
                        projections: [],
                        effects: [],
                        presentation: runtime.completion.tokens.map(token => sseToken(token))
                    };
                }
            }] : []),
            {
                id: 'conversation-result',
                layer: 'application',
                dependencies: [{id: 'conversation-repository', layer: 'contracts'}],
                async run(_context, command, previous) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    const completion = runtime.completion ?? {};
                    runtime.messages = Array.isArray(completion.messages)
                        ? completion.messages.slice(0, MAX_MESSAGES)
                        : completion.message ? [completion.message] : [];
                    runtime.presentationBeforeMapping = previous.presentation.slice();
                    return emptyStepResult();
                }
            },
            {
                id: 'presentation-mapper',
                layer: 'application',
                dependencies: [{id: 'presentation-mapper', layer: 'contracts'}],
                async run(context, command, previous) {
                    const runtime = command[FLOW_RUNTIME];
                    if (policyHandled(runtime)) return emptyStepResult();
                    const mapped = await invokeMapper(presentationMapper, {
                        context: runtime.context ?? {},
                        command,
                        history: runtime.history,
                        completion: runtime.completion ?? normalizeCompletion(),
                        capabilityPresentation: runtime.capabilityPresentation ?? [],
                        facts: previous.facts,
                        projections: previous.projections,
                        effects: previous.effects,
                        presentation: previous.presentation,
                        messages: runtime.messages ?? [],
                        ...(runtime.visibleText === undefined ? {} : {visibleText: runtime.visibleText})
                    });
                    const mappedResult = isRecord(mapped) && Object.hasOwn(mapped, 'chatResult')
                        ? mapped.chatResult
                        : mapped;
                    runtime.chatResult = normalizeBoundedChatResult(mappedResult ?? {messages: runtime.messages ?? []});
                    return resultChannels(isRecord(mapped) && Object.hasOwn(mapped, 'chatResult') ? mapped : {});
                }
            }
        ]
    });
    return registry;
}

/**
 * Build the application-only chat turn use case. Construction performs no
 * database/provider work; all infrastructure is supplied through ports.
 *
 * @param {object} options
 * @returns {{flowId: string, registry: object, executor: object, run: Function, execute: Function, runFlow: Function}}
 */
export function createChatTurnFlow({
    registry = createFlowRegistry(),
    executor,
    contextReader,
    llmStreamingPort,
    llm,
    capabilityDispatcher,
    conversationRepository,
    presentationMapper,
    commit,
    commitBoundary,
    userMessageWriter,
    chatPolicy,
    enableContinuation = false,
    flowId = CHAT_TURN_FLOW_ID
} = {}) {
    const readContext = resolvePortMethod(contextReader, ['read', 'readContext'], 'ChatContextReader');
    const streamLlm = resolvePortMethod(llmStreamingPort ?? llm, ['stream', 'streamCompletion', 'streamChat'], 'LlmStreamingPort');
    requiredObject(conversationRepository, 'ConversationRepository');
    const mapPresentation = presentationMapper === undefined
        ? ({messages}) => ({type: 'done', messages})
        : presentationMapper;
    if (typeof mapPresentation !== 'function' && (!isRecord(mapPresentation) || typeof mapPresentation.map !== 'function')) {
        throw new TypeError('PresentationMapper must provide map()');
    }
    if (typeof flowId !== 'string' || !flowId.trim()) throw new TypeError('Chat turn flowId must be a non-empty string');
    if (!registry || typeof registry.register !== 'function' || typeof registry.get !== 'function') {
        throw new TypeError('ChatTurnFlow requires a flow registry');
    }
    if (registry.has?.(flowId)) throw new Error(`Chat turn flow already registered: ${flowId}`);

    registerChatTurnFlow({
        registry,
        contextReader: readContext,
        llmStream: streamLlm,
        capabilityDispatcher,
        conversationRepository,
        presentationMapper: mapPresentation,
        userMessageWriter,
        chatPolicy: resolveChatPolicy(chatPolicy),
        enableContinuation,
        flowId
    });

    const flowExecutor = executor ?? createFlowExecutor({registry, commit, commitBoundary});
    if (!flowExecutor || typeof flowExecutor.run !== 'function') throw new TypeError('ChatTurnFlow executor must provide run()');

    async function run(first = {}, second) {
        const invocation = invocationFor(first, second);
        const command = normalizeCommand(invocation.command);
        const runtimeCommand = {...command};
        const runtime = {};
        Object.defineProperty(runtimeCommand, FLOW_RUNTIME, {value: runtime});
        const aggregate = normalizeStepResult(await flowExecutor.run(flowId, invocation.context, runtimeCommand));
        const chatResult = runtime.chatResult ?? normalizeBoundedChatResult({messages: runtime.messages ?? []});
        return Object.freeze({
            ...aggregate,
            ...chatResult,
            chatResult
        });
    }

    return Object.freeze({
        flowId,
        registry,
        executor: flowExecutor,
        run,
        execute: run,
        runFlow: run
    });
}

export const createChatTurnUseCase = createChatTurnFlow;
