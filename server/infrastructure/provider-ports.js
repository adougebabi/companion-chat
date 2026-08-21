/**
 * Provider boundaries used by the composition root and runtime adapters.
 *
 * This module intentionally contains no provider clients, filesystem access,
 * database access, or domain policy. A provider is an injected object whose
 * methods are called with the provider as `this`; the registry only validates
 * the boundary and keeps provider failures bounded.
 */

export const PROVIDER_PORT_TYPES = Object.freeze({
    LLM_STREAMING: 'llm-streaming',
    MEDIA: 'media',
    ASSET_READER: 'asset-reader'
});

export const PROVIDER_CAPABILITIES = Object.freeze({
    LLM_STREAMING: Object.freeze(['stream']),
    MEDIA: Object.freeze(['image', 'video']),
    ASSET_READER: Object.freeze(['asset', 'candidate'])
});

export const MAX_PROVIDER_ERROR_LENGTH = 240;
export const MAX_PROVIDER_ID_LENGTH = 120;
export const MAX_PROVIDER_LABEL_LENGTH = 160;
export const MAX_PROVIDER_CAPABILITY_LENGTH = 80;

const PORT_TYPE_ALIASES = new Map([
    ['llm', PROVIDER_PORT_TYPES.LLM_STREAMING],
    ['llm-stream', PROVIDER_PORT_TYPES.LLM_STREAMING],
    ['llm-streaming', PROVIDER_PORT_TYPES.LLM_STREAMING],
    ['llmStreaming', PROVIDER_PORT_TYPES.LLM_STREAMING],
    ['streaming', PROVIDER_PORT_TYPES.LLM_STREAMING],
    ['media', PROVIDER_PORT_TYPES.MEDIA],
    ['media-provider', PROVIDER_PORT_TYPES.MEDIA],
    ['asset', PROVIDER_PORT_TYPES.ASSET_READER],
    ['asset-reader', PROVIDER_PORT_TYPES.ASSET_READER],
    ['assetReader', PROVIDER_PORT_TYPES.ASSET_READER]
]);

const OPERATION_ALIASES = Object.freeze({
    stream: ['stream', 'streamCompletion', 'streamChat'],
    submit: ['submit', 'generate'],
    poll: ['poll', 'status'],
    readAsset: ['readAsset', 'read', 'readCandidate'],
    readCandidate: ['readCandidate', 'readAsset', 'read']
});

const DEFAULT_OPERATION_BY_TYPE = Object.freeze({
    [PROVIDER_PORT_TYPES.LLM_STREAMING]: 'stream',
    [PROVIDER_PORT_TYPES.MEDIA]: 'submit',
    [PROVIDER_PORT_TYPES.ASSET_READER]: 'readAsset'
});

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function text(value, field, maxLength, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const normalized = value.trim();
    if (!allowEmpty && !normalized) throw new TypeError(`${field} must not be empty`);
    if (normalized.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return normalized;
}

function errorMessage(error) {
    if (error === null || error === undefined) return '';
    if (error instanceof Error) return error.message;
    if (typeof error === 'string') return error;
    try {
        const serialized = JSON.stringify(error);
        return serialized === undefined ? String(error) : serialized;
    } catch {
        return String(error);
    }
}

/**
 * Keep provider errors suitable for logs and user-safe transport adapters.
 * Provider response bodies may contain credentials or very large diagnostics,
 * so only a compact, redacted message crosses this boundary.
 */
export function boundedProviderError(error, fallback = 'Provider operation failed', maxLength = MAX_PROVIDER_ERROR_LENGTH) {
    const limit = Number.isInteger(maxLength) && maxLength >= 32 ? maxLength : MAX_PROVIDER_ERROR_LENGTH;
    const source = errorMessage(error).replace(/\s+/g, ' ').trim();
    const candidate = source || String(fallback || 'Provider operation failed');
    const redacted = candidate
        .replace(/Bearer\s+[^\s,;]+/gi, 'Bearer [redacted]')
        .replace(/((?:api[_-]?key|token|secret|authorization|password)=)[^&\s]+/gi, '$1[redacted]');
    return redacted.length <= limit ? redacted : `${redacted.slice(0, limit - 3)}...`;
}

/**
 * Normalize a configured provider base URL before appending endpoint paths.
 * Empty values remain empty so configuration validation can report the
 * missing URL; a protocol-only URL is kept intact rather than becoming
 * `https:` after slash trimming.
 */
export function cleanUrl(value) {
    if (value === null || value === undefined) return '';
    const raw = String(value).trim();
    if (!raw) return '';
    if (/^[a-z][a-z\d+.-]*:\/\/$/i.test(raw)) return raw;
    return raw.replace(/\/+$/, '');
}

function normalizePortType(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback || null;
    if (typeof value !== 'string') throw new TypeError('Provider port type must be a string');
    const type = PORT_TYPE_ALIASES.get(value.trim());
    if (!type) throw new TypeError(`Unsupported provider port type: ${boundedProviderError(value, 'unknown')}`);
    return type;
}

function operationNames(port) {
    return Object.entries(OPERATION_ALIASES)
        .filter(([, aliases]) => aliases.some(name => typeof port[name] === 'function'))
        .map(([name]) => name);
}

function inferPortType(port) {
    if (OPERATION_ALIASES.stream.some(name => typeof port[name] === 'function')) {
        return PROVIDER_PORT_TYPES.LLM_STREAMING;
    }
    if (OPERATION_ALIASES.submit.some(name => typeof port[name] === 'function')
        || OPERATION_ALIASES.poll.some(name => typeof port[name] === 'function')) {
        return PROVIDER_PORT_TYPES.MEDIA;
    }
    if (Array.isArray(port.capabilities) && port.capabilities.some(capability => ['image', 'video'].includes(capability))) {
        return PROVIDER_PORT_TYPES.MEDIA;
    }
    if (OPERATION_ALIASES.readAsset.some(name => typeof port[name] === 'function')) {
        return PROVIDER_PORT_TYPES.ASSET_READER;
    }
    return null;
}

function normalizeCapabilities(port, portType) {
    const configured = port.capabilities;
    if (configured === undefined) {
        if (portType === PROVIDER_PORT_TYPES.LLM_STREAMING) return [...PROVIDER_CAPABILITIES.LLM_STREAMING];
        if (portType === PROVIDER_PORT_TYPES.ASSET_READER) return [...PROVIDER_CAPABILITIES.ASSET_READER];
        throw new TypeError('Media provider must declare capabilities');
    }
    if (!Array.isArray(configured)) throw new TypeError('Provider capabilities must be an array');
    const normalized = [];
    for (const [index, capability] of configured.entries()) {
        const value = text(capability, `Provider capabilities[${index}]`, MAX_PROVIDER_CAPABILITY_LENGTH);
        if (!normalized.includes(value)) normalized.push(value);
    }
    if (!normalized.length) throw new TypeError('Provider capabilities must not be empty');
    return normalized;
}

function requiredOperation(port, portType) {
    const operation = DEFAULT_OPERATION_BY_TYPE[portType];
    if (operationNames(port).includes(operation)) return operation;
    if (portType === PROVIDER_PORT_TYPES.MEDIA && operationNames(port).includes('readAsset')) return 'readAsset';
    const aliases = OPERATION_ALIASES[operation].join('(), ');
    throw new TypeError(`Provider ${port.id || 'port'} must provide ${aliases}()`);
}

function validatePort(port, {idOverride, expectedType} = {}) {
    if (!isRecord(port)) throw new TypeError('Provider port must be an object');
    const id = text(idOverride ?? port.id, 'Provider id', MAX_PROVIDER_ID_LENGTH);
    const declaredType = port.portType ?? port.providerType ?? port.type
        ?? (typeof port.kind === 'string' && PORT_TYPE_ALIASES.has(port.kind) ? port.kind : undefined);
    const portType = normalizePortType(declaredType, inferPortType(port));
    if (!portType) throw new TypeError(`Provider ${id} must declare a supported port type`);
    if (expectedType && portType !== normalizePortType(expectedType)) {
        throw new TypeError(`Provider ${id} is a ${portType} port, not ${normalizePortType(expectedType)}`);
    }
    const capabilities = normalizeCapabilities(port, portType);
    const operation = requiredOperation(port, portType);
    const label = port.label === undefined ? id : text(port.label, 'Provider label', MAX_PROVIDER_LABEL_LENGTH);
    return Object.freeze({id, label, portType, capabilities: Object.freeze(capabilities), operation, operations: Object.freeze(operationNames(port))});
}

/**
 * Assert an injected adapter at a provider boundary. The original object is
 * returned so callers retain its method receiver and can use dry-run objects
 * in tests without wrapping them.
 */
export function assertProviderPort(port, expectedType) {
    validatePort(port, {expectedType});
    return port;
}

export class ProviderPortError extends Error {
    constructor({providerId, operation, cause, code = 'PROVIDER_OPERATION_FAILED'} = {}) {
        const id = boundedProviderError(providerId, 'provider');
        const method = boundedProviderError(operation, 'operation');
        const detail = boundedProviderError(cause);
        // Do not retain the provider's raw error as `cause`: response bodies
        // can contain credentials and are outside this bounded contract.
        super(boundedProviderError(`Provider ${id} ${method} failed: ${detail}`));
        this.name = 'ProviderPortError';
        this.code = code;
        this.providerId = id;
        this.operation = method;
    }
}

function unknownProviderError(id) {
    const error = new ProviderPortError({
        providerId: id,
        operation: 'lookup',
        cause: 'provider is not registered',
        code: 'PROVIDER_NOT_FOUND'
    });
    return error;
}

function resolveOperation(port, requested) {
    const operation = requested || null;
    const aliases = OPERATION_ALIASES[operation] || (operation ? [operation] : []);
    const method = aliases.find(name => typeof port[name] === 'function');
    if (!method) {
        const name = boundedProviderError(operation, 'the requested operation');
        throw new TypeError(`Provider does not provide ${name}()`);
    }
    return method;
}

function normalizeRegisterInput(first, second, third) {
    if (typeof first === 'string') {
        if (!isRecord(second)) throw new TypeError('Provider registration adapter must be an object');
        return {port: second, idOverride: first, options: isRecord(third) ? third : {}};
    }
    if (!isRecord(first)) throw new TypeError('Provider registration requires an adapter object');
    return {port: first, idOverride: undefined, options: isRecord(second) ? second : {}};
}

function filterOptions(value) {
    if (typeof value === 'string') {
        const type = PORT_TYPE_ALIASES.get(value.trim());
        return type ? {portType: type} : {capability: value.trim()};
    }
    if (!isRecord(value)) return {};
    const rawType = value.portType ?? value.type ?? value.kind;
    const isPortType = typeof rawType === 'string' && PORT_TYPE_ALIASES.has(rawType.trim());
    return {
        portType: isPortType ? rawType : undefined,
        capability: value.capability ?? (!isPortType ? rawType : undefined),
        detailed: value.detailed === true
    };
}

/**
 * Build an in-process provider registry. `providers` and `dryRunAdapters` are
 * registration inputs only; no adapter is invoked while constructing it.
 */
export function createProviderRegistry(options = {}) {
    if (!isRecord(options)) throw new TypeError('Provider registry options must be an object');
    const entries = new Map();
    const initial = options.providers ?? options.dryRunAdapters ?? [];
    const initialList = Array.isArray(initial)
        ? initial.map(provider => ({provider}))
        : Object.entries(initial || {}).map(([id, provider]) => ({id, provider}));

    function register(first, second, third) {
        const {port, idOverride, options: registrationOptions} = normalizeRegisterInput(first, second, third);
        const metadata = validatePort(port, {
            idOverride,
            expectedType: registrationOptions.portType ?? registrationOptions.type
        });
        if (entries.has(metadata.id) && registrationOptions.replace !== true) {
            throw new Error(`Provider already registered: ${metadata.id}`);
        }
        entries.set(metadata.id, {port, metadata});
        return port;
    }

    function find(id, requirement) {
        const providerId = text(id, 'Provider id', MAX_PROVIDER_ID_LENGTH);
        const entry = entries.get(providerId);
        if (!entry) return null;
        const requested = filterOptions(requirement);
        if (requested.portType && entry.metadata.portType !== normalizePortType(requested.portType)) return null;
        if (requested.capability && !entry.metadata.capabilities.includes(requested.capability)) return null;
        return entry.port;
    }

    function get(id, requirement) {
        const providerId = text(id, 'Provider id', MAX_PROVIDER_ID_LENGTH);
        const provider = find(providerId, requirement);
        if (!provider) {
            const requested = filterOptions(requirement);
            if (requested.portType || requested.capability) {
                throw new ProviderPortError({
                    providerId,
                    operation: requested.portType || requested.capability,
                    cause: 'provider is not registered or does not declare this capability',
                    code: 'PROVIDER_CAPABILITY_UNAVAILABLE'
                });
            }
            throw unknownProviderError(providerId);
        }
        return provider;
    }

    function metadataFor(id) {
        const providerId = text(id, 'Provider id', MAX_PROVIDER_ID_LENGTH);
        const entry = entries.get(providerId);
        if (!entry) throw unknownProviderError(providerId);
        return entry.metadata;
    }

    function summaries(requirement) {
        const requested = filterOptions(requirement);
        return [...entries.values()]
            .filter(({metadata}) => !requested.portType || metadata.portType === normalizePortType(requested.portType))
            .filter(({metadata}) => !requested.capability || metadata.capabilities.includes(requested.capability))
            .map(({metadata}) => requested.detailed
                ? {id: metadata.id, label: metadata.label, portType: metadata.portType, capabilities: [...metadata.capabilities], operations: [...metadata.operations]}
                : {id: metadata.id, label: metadata.label, capabilities: [...metadata.capabilities]});
    }

    async function invoke(id, operation, input, receiver) {
        const provider = get(id);
        const metadata = metadataFor(id);
        const method = resolveOperation(provider, operation || metadata.operation);
        try {
            return await provider[method].call(provider, input, receiver);
        } catch (error) {
            if (error instanceof ProviderPortError) throw error;
            throw new ProviderPortError({providerId: metadata.id, operation: method, cause: error});
        }
    }

    const registry = {
        register,
        set(id, provider) {
            register(id, provider, {replace: true});
            return registry;
        },
        delete(id) {
            const providerId = text(id, 'Provider id', MAX_PROVIDER_ID_LENGTH);
            return entries.delete(providerId);
        },
        get,
        find,
        has(id, requirement) {
            return find(id, requirement) !== null;
        },
        metadata: metadataFor,
        summaries,
        invoke,
        stream(id, input, receiver) {
            return invoke(id, 'stream', input, receiver);
        },
        submit(id, input) {
            return invoke(id, 'submit', input);
        },
        poll(id, input) {
            return invoke(id, 'poll', input);
        },
        readAsset(id, input, receiver) {
            return invoke(id, 'readAsset', input, receiver);
        }
    };

    for (const {id, provider} of initialList) {
        if (id === undefined) register(provider);
        else register(id, provider);
    }
    return Object.freeze(registry);
}

// Convenience forms make the small registry API easy to use from composition
// code while keeping the stateful registry itself explicit and injectable.
export function register(registry, provider, adapter, options) {
    if (!registry || typeof registry.register !== 'function') throw new TypeError('Provider registry is required');
    return registry.register(provider, adapter, options);
}

export function get(registry, id, requirement) {
    if (!registry || typeof registry.get !== 'function') throw new TypeError('Provider registry is required');
    return registry.get(id, requirement);
}

export function summaries(registry, requirement) {
    if (!registry || typeof registry.summaries !== 'function') throw new TypeError('Provider registry is required');
    return registry.summaries(requirement);
}

export function invoke(registry, id, operation, input, receiver) {
    if (!registry || typeof registry.invoke !== 'function') throw new TypeError('Provider registry is required');
    return registry.invoke(id, operation, input, receiver);
}
