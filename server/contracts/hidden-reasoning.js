const REASONING_NAME = '(?:think|thinking|analysis|reasoning)';
const OPEN_TAG = new RegExp(`<\\s*${REASONING_NAME}\\b[^>]*>`, 'i');
const CLOSE_TAG = new RegExp(`<\\s*/\\s*${REASONING_NAME}\\s*>`, 'i');
const BLOCK_PATTERN = new RegExp(`<\\s*${REASONING_NAME}\\b[^>]*>[\\s\\S]*?<\\s*/\\s*${REASONING_NAME}\\s*>`, 'gi');

function possibleTagPrefix(value) {
    const index = value.lastIndexOf('<');
    if (index < 0) return null;
    const suffix = value.slice(index);
    if (suffix.length > 80 || suffix.includes('>')) return null;
    const body = suffix.replace(/^<\s*\/?\s*/i, '');
    if (!/^[a-z][\w:-]*(?:\s+[^>]*)?$/i.test(body)) return null;
    const name = body.match(/^[a-z]+/i)?.[0]?.toLowerCase() || '';
    const names = ['think', 'thinking', 'analysis', 'reasoning'];
    return names.some(candidate => candidate.startsWith(name) || name.startsWith(candidate)) ? suffix : null;
}

/** Remove complete or unterminated provider reasoning wrappers from text. */
export function stripHiddenReasoning(value) {
    let text = String(value ?? '');
    for (let index = 0; index < 8; index += 1) {
        const leading = text.match(new RegExp(`^(\\s*)${OPEN_TAG.source}`, 'i'));
        if (!leading) break;
        const rest = text.slice(leading[0].length);
        const close = rest.match(CLOSE_TAG);
        if (!close) return leading[1];
        text = rest.slice(close.index + close[0].length);
    }
    // Only remove additional complete blocks when they begin the remaining
    // provider text; literal XML/HTML-like tags in ordinary prose are kept.
    text = text.replace(BLOCK_PATTERN, (block, offset, source) => {
        return source.slice(0, offset).trim() ? block : '';
    });
    const leadingClose = text.match(new RegExp(`^(\\s*)${CLOSE_TAG.source}`, 'i'));
    return leadingClose ? `${leadingClose[1]}${text.slice(leadingClose[0].length)}` : text;
}

/**
 * Filter hidden reasoning while a provider stream is still arriving. The
 * filter keeps a possible tag prefix between chunks so `<thi` + `nk>` never
 * becomes visible text before the opening tag is recognized.
 */
export function createHiddenReasoningFilter() {
    let pending = '';
    let inside = false;
    let seenVisible = false;

    function push(value = '', final = false) {
        pending += String(value ?? '');
        let visible = '';
        while (pending) {
            if (inside) {
                const close = pending.match(CLOSE_TAG);
                if (!close) {
                    if (final) pending = '';
                    break;
                }
                pending = pending.slice(close.index + close[0].length);
                inside = false;
                continue;
            }
            const open = pending.match(OPEN_TAG);
            if (open) {
                const prefix = pending.slice(0, open.index);
                if (seenVisible || prefix.trim()) {
                    visible += pending;
                    pending = '';
                    break;
                }
                pending = pending.slice(open.index + open[0].length);
                inside = true;
                continue;
            }
            const prefix = possibleTagPrefix(pending);
            if (prefix && !final) {
                const prefixText = pending.slice(0, pending.length - prefix.length);
                if (seenVisible || prefixText.trim()) {
                    visible += pending;
                    pending = '';
                    break;
                }
                visible += prefixText;
                pending = prefix;
                break;
            }
            visible += pending;
            pending = '';
        }
        if (visible.trim()) seenVisible = true;
        return visible;
    }

    return Object.freeze({
        push(value) { return push(value, false); },
        finish() { return push('', true); }
    });
}

export default stripHiddenReasoning;
