import {contextPromptFor} from './context-contracts.js';

const DEFAULT_MAX_CHARS = 12_000;
const DEFAULT_MAX_FRAGMENTS = 24;
const DEFAULT_MAX_HISTORY = 18;

function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function boundedText(value, limit = 2_000) { const text = String(value ?? '').replace(/\s+/g, ' ').trim(); return text.length <= limit ? text : `${text.slice(0, limit - 3)}...`; }
function fragment(value, source, priority = 0) {
    if (value === undefined || value === null || value === '') return null;
    const text = typeof value === 'string' ? value : JSON.stringify(value);
    if (!text.trim()) return null;
    return Object.freeze({source, priority: Number(priority) || 0, text: boundedText(text)});
}
function sourceList(sources) {
    if (Array.isArray(sources)) return sources;
    if (!isRecord(sources)) return [];
    return Object.entries(sources).map(([source, value]) => ({source, value}));
}

function modelMessage(message) {
    if (!isRecord(message)) return null;
    const suppliedRole = message.role;
    const role = suppliedRole === 'assistant' || suppliedRole === 'tool' || suppliedRole === 'system'
        ? suppliedRole : 'user';
    const mapped = {
        role,
        content: message.content ?? message.text ?? (Array.isArray(message.attachments) && message.attachments.length ? '[用户发送了媒体附件]' : '')
    };
    if (role === 'assistant' && Array.isArray(message.tool_calls)) {
        mapped.tool_calls = message.tool_calls.slice(0, 8).map(call => {
            if (!isRecord(call)) return call;
            const functionValue = isRecord(call.function) ? {
                ...(call.function.name === undefined ? {} : {name: call.function.name}),
                ...(call.function.arguments === undefined ? {} : {arguments: call.function.arguments})
            } : undefined;
            return {
                ...(call.id === undefined ? {} : {id: call.id}),
                ...(call.type === undefined ? {} : {type: call.type}),
                ...(functionValue ? {function: functionValue} : {})
            };
        });
    }
    if (role === 'tool') {
        if (message.tool_call_id !== undefined) mapped.tool_call_id = message.tool_call_id;
        if (message.name !== undefined) mapped.name = message.name;
    }
    return mapped;
}

/**
 * Serialize the context reader result and ordered history into provider
 * messages. In particular, assistant tool-call metadata and tool result
 * correlation are retained for the single continuation request.
 */
export function serializePromptMessages({context, messages = [], instruction, maxHistory = DEFAULT_MAX_HISTORY} = {}) {
    const system = contextPromptFor(context);
    const output = system ? [{role: 'system', content: system}] : [];
    if (typeof instruction === 'string' && instruction.trim()) output.push({role: 'system', content: instruction.trim()});
    const limit = Math.max(1, Math.min(DEFAULT_MAX_HISTORY, Number(maxHistory) || DEFAULT_MAX_HISTORY));
    output.push(...(Array.isArray(messages) ? messages.slice(-limit).map(modelMessage).filter(Boolean) : []));
    return output;
}

/** Shared prompt context pipeline: normalize fragments, budget them, serialize once. */
export function createContextPipeline({maxChars = DEFAULT_MAX_CHARS, maxFragments = DEFAULT_MAX_FRAGMENTS, serializer} = {}) {
    const charLimit = Math.max(512, Number(maxChars) || DEFAULT_MAX_CHARS);
    const fragmentLimit = Math.max(1, Math.min(DEFAULT_MAX_FRAGMENTS, Number(maxFragments) || DEFAULT_MAX_FRAGMENTS));
    function collect(input = {}) {
        const values = sourceList(input.fragments ?? input.sources);
        return values.map((item, index) => {
            if (isRecord(item) && item.text !== undefined) return fragment(item.text, item.source ?? `fragment_${index}`, item.priority);
            return fragment(item.value, item.source ?? `fragment_${index}`, item.priority);
        }).filter(Boolean).sort((left, right) => right.priority - left.priority || left.source.localeCompare(right.source));
    }
    function budget(fragments) {
        const selected = [];
        let used = 0;
        for (const item of fragments.slice(0, fragmentLimit)) {
            const extra = item.text.length + (selected.length ? 2 : 0);
            if (used + extra > charLimit) continue;
            selected.push(item);
            used += extra;
        }
        return Object.freeze({fragments: Object.freeze(selected), used, limit: charLimit});
    }
    function serialize(input = {}) {
        const selected = input.budget?.fragments ?? budget(collect(input)).fragments;
        if (typeof serializer === 'function') return serializer(selected, input);
        return selected.map(item => `[${item.source}]\n${item.text}`).join('\n\n');
    }
    return Object.freeze({collect, budget, serialize, maxChars: charLimit, maxFragments: fragmentLimit});
}

export const createContextFragmentPipeline = createContextPipeline;
export default createContextPipeline;
