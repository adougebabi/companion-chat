import {contextPromptFor} from './context-contracts.js';

const DEFAULT_MAX_CHARS = 12_000;
const DEFAULT_MAX_FRAGMENTS = 24;
const DEFAULT_MAX_HISTORY = 18;
const MAX_SECTION_LENGTH = 120;
const MAX_PROVENANCE_KEYS = 8;

function isRecord(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function boundedText(value, limit = 2_000) {
    const text = String(value ?? '').replace(/\s+/g, ' ').trim();
    return text.length <= limit ? text : `${text.slice(0, limit - 3)}...`;
}
function boundedSection(value, fallback) {
    const section = boundedText(value || fallback, MAX_SECTION_LENGTH);
    return section || fallback;
}
function normalizeProvenance(value, fallbackSource) {
    if (isRecord(value)) {
        return Object.freeze(Object.fromEntries(Object.entries(value).slice(0, MAX_PROVENANCE_KEYS).map(([key, item]) => [String(key), boundedText(item, 240)])));
    }
    return Object.freeze({source: boundedSection(fallbackSource, 'context')});
}
function fragment(value, source, priority = 0, options = {}) {
    if (value === undefined || value === null || value === '') return null;
    const sourceValue = isRecord(value) ? value : null;
    const suppliedText = sourceValue?.text ?? sourceValue?.value ?? value;
    const text = typeof suppliedText === 'string' ? suppliedText : JSON.stringify(suppliedText);
    if (!text || !text.trim()) return null;
    const bounded = boundedText(text);
    const configuredBudget = sourceValue?.budget ?? options.budget;
    const budget = configuredBudget === undefined || configuredBudget === null || configuredBudget === ''
        ? bounded.length
        : Math.max(0, Math.floor(Number(configuredBudget) || 0));
    const section = boundedSection(sourceValue?.section ?? options.section ?? source, source || 'context');
    const numericPriority = Number(sourceValue?.priority ?? options.priority ?? priority);
    return Object.freeze({
        // `source` is retained as a compatibility alias while `section` is the
        // canonical structured context location.
        source: section,
        section,
        priority: Number.isFinite(numericPriority) ? numericPriority : 0,
        required: sourceValue?.required === true || options.required === true,
        budget,
        provenance: normalizeProvenance(sourceValue?.provenance ?? options.provenance, source),
        text: bounded
    });
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

/** Serialize ordered context and history into provider messages. */
export function serializePromptMessages({context, messages = [], instruction, maxHistory = DEFAULT_MAX_HISTORY} = {}) {
    const system = contextPromptFor(context);
    const output = system ? [{role: 'system', content: system}] : [];
    if (typeof instruction === 'string' && instruction.trim()) output.push({role: 'system', content: instruction.trim()});
    const limit = Math.max(1, Math.min(DEFAULT_MAX_HISTORY, Number(maxHistory) || DEFAULT_MAX_HISTORY));
    output.push(...(Array.isArray(messages) ? messages.slice(-limit).map(modelMessage).filter(Boolean) : []));
    return output;
}

/**
 * Shared prompt context pipeline. Collection normalizes metadata, budget()
 * selects required sections first and optional sections within the total
 * budget, and serialize() only renders the already-selected sections.
 */
export function createContextPipeline({maxChars = DEFAULT_MAX_CHARS, maxFragments = DEFAULT_MAX_FRAGMENTS, serializer} = {}) {
    const charLimit = Math.max(512, Number(maxChars) || DEFAULT_MAX_CHARS);
    const fragmentLimit = Math.max(1, Math.min(DEFAULT_MAX_FRAGMENTS, Number(maxFragments) || DEFAULT_MAX_FRAGMENTS));
    function collect(input = {}) {
        const values = sourceList(input.fragments ?? input.sources);
        return values.map((item, index) => {
            const supplied = isRecord(item) && (item.text !== undefined || item.value !== undefined) ? item : {value: item};
            const source = supplied.source ?? supplied.section ?? `fragment_${index}`;
            return fragment(supplied, source, supplied.priority, supplied);
        }).filter(Boolean).sort((left, right) => Number(right.required) - Number(left.required) || right.priority - left.priority || left.section.localeCompare(right.section));
    }
    function budget(fragments = []) {
        if (!Array.isArray(fragments)) throw new TypeError('Context fragments must be an array');
        const normalized = fragments.map((item, index) => fragment(item, item?.section ?? item?.source ?? `fragment_${index}`, item?.priority, item)).filter(Boolean);
        const required = normalized.filter(item => item.required);
        const optional = normalized.filter(item => !item.required);
        const selected = [];
        let used = 0;
        for (const item of required) {
            if (selected.length >= fragmentLimit && required.length > fragmentLimit) {
                // Required context is never discarded because the optional
                // cap is a performance guard, not a correctness policy.
            }
            selected.push(item);
            used += item.budget;
        }
        for (const item of optional) {
            if (selected.length >= fragmentLimit) break;
            if (used + item.budget > charLimit) continue;
            selected.push(item);
            used += item.budget;
        }
        return Object.freeze({
            fragments: Object.freeze(selected),
            required: Object.freeze(required.slice()),
            optional: Object.freeze(selected.filter(item => !item.required)),
            used,
            limit: charLimit,
            remaining: Math.max(0, charLimit - used)
        });
    }
    function serialize(input = {}) {
        const selected = Array.isArray(input?.budget?.fragments)
            ? input.budget.fragments
            : Array.isArray(input?.fragments) ? input.fragments : [];
        if (typeof serializer === 'function') return serializer(selected, input);
        return selected.map(item => `[${item.section ?? item.source}]\n${item.text}`).join('\n\n');
    }
    return Object.freeze({collect, budget, serialize, maxChars: charLimit, maxFragments: fragmentLimit});
}

export const createContextFragmentPipeline = createContextPipeline;
export default createContextPipeline;
