const DEFAULT_MAX_CHARS = 12_000;
const DEFAULT_MAX_FRAGMENTS = 24;

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
