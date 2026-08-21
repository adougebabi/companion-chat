import {cleanUrl, boundedProviderError} from './provider-ports.js';

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
export function createMtplxProvider({settings, fetchImpl} = {}) {
    const readSettings = assertSettings(settings);
    if (fetchImpl !== undefined && typeof fetchImpl !== 'function') throw new TypeError('MTPLX provider fetchImpl must be a function');

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
        stream(requestPayload = {}) {
            const {signal, ...body} = requestPayload;
            return request('/chat/completions', {method: 'POST', body, signal, allowErrorResponse: true});
        },
        async models() {
            const response = await request('/models');
            return response.json();
        }
    });
}

export default createMtplxProvider;
