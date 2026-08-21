import {boundedProviderError} from './provider-ports.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function readSettings(settings) {
    if (typeof settings === 'function') return settings();
    if (isRecord(settings) && typeof settings.read === 'function') return settings.read();
    return isRecord(settings) ? settings : {};
}

function responseContent(body) {
    const content = body?.choices?.[0]?.message?.content;
    if (typeof content !== 'string' || !content.trim()) throw new Error('MTPLX returned no media prompt');
    return content.trim();
}

function parseModelJson(value) {
    const source = String(value || '').trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
    try {
        return JSON.parse(source);
    } catch {
        return null;
    }
}

function promptText(value) {
    if (typeof value === 'string' && value.trim()) return value.trim();
    if (!isRecord(value)) return '';
    if (typeof value.finalPrompt === 'string' && value.finalPrompt.trim()) return value.finalPrompt.trim();
    if (typeof value.prompt === 'string' && value.prompt.trim()) return value.prompt.trim();
    if (isRecord(value.sections)) {
        return Object.values(value.sections).filter(item => typeof item === 'string' && item.trim()).join('\n');
    }
    return '';
}

/**
 * LLM-backed prompt-master port. It asks MTPLX for the final provider prompt
 * while keeping visual semantics model-owned; the media worker never invents
 * subjects, camera relationships, or negative constraints itself.
 */
export function createMediaPromptMaster({provider, settings, model, timeoutMs = 30_000} = {}) {
    if (!provider || typeof provider.stream !== 'function') throw new TypeError('Media prompt master requires an MTPLX provider');
    const read = () => readSettings(settings);

    async function complete(messages, context = {}) {
        const config = read();
        const response = await provider.stream({
            model: model ?? config.model,
            stream: false,
            temperature: 0.25,
            messages,
            signal: context.signal ?? (typeof AbortSignal?.timeout === 'function' ? AbortSignal.timeout(timeoutMs) : undefined)
        });
        if (!response?.ok) {
            let body;
            try { body = await response.json(); } catch { body = null; }
            throw new Error(boundedProviderError(body?.error?.message ?? `MTPLX HTTP ${response?.status ?? 'error'}`, 'MTPLX prompt failed'));
        }
        return response.json();
    }

    return Object.freeze({
        async fill({envelope, concept, personaMediaConcept, priorAcceptance, context} = {}) {
            const frozenConcept = concept ?? personaMediaConcept;
            const response = await complete([
                {
                    role: 'system',
                    content: 'You are a media prompt master. Return strict JSON {"finalPrompt":"..."}. Preserve the frozen identity, scene, action, subjects, non-human objects, and capture relationship. Do not add people or infer visual facts. Keep the prompt concise and executable for the configured image/video provider.'
                },
                {
                    role: 'user',
                    content: JSON.stringify({envelope, personaMediaConcept: frozenConcept, priorAcceptance: priorAcceptance ?? null})
                }
            ], context);
            const body = await response;
            const parsed = parseModelJson(responseContent(body));
            const finalPrompt = promptText(parsed) || promptText(responseContent(body));
            if (!finalPrompt) throw new Error('MTPLX media prompt was malformed');
            return {finalPrompt: finalPrompt.slice(0, 32_000)};
        }
    });
}

export function createSkippedMediaAcceptance({clock = () => new Date().toISOString()} = {}) {
    return Object.freeze({
        async accept() {
            return {
                verdict: 'skipped',
                diagnostic: 'media acceptance is not configured',
                checkedAt: typeof clock === 'function' ? clock() : new Date().toISOString()
            };
        }
    });
}

export default createMediaPromptMaster;
