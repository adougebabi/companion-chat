import {createHash} from 'node:crypto';

export const CAPABILITY_DISPATCHER_VERSION = 1;
export const MAX_CAPABILITY_DIAGNOSTIC_LENGTH = 240;
export const MAX_CAPABILITY_DIAGNOSTICS = 8;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedText(value, field, maxLength = MAX_CAPABILITY_DIAGNOSTIC_LENGTH, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && !text) throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function errorText(error, fallback = 'Capability execution failed') {
    const source = error instanceof Error ? error.message : error;
    const text = String(source ?? fallback).replace(/\s+/g, ' ').trim() || fallback;
    return text.length <= MAX_CAPABILITY_DIAGNOSTIC_LENGTH
        ? text
        : `${text.slice(0, MAX_CAPABILITY_DIAGNOSTIC_LENGTH - 3)}...`;
}

function requiredName(value, field = 'Capability name') {
    return boundedText(value, field, 120);
}

function resolveRegistry(input) {
    if (input instanceof Map) return [...input.entries()];
    if (isRecord(input)) {
        if (typeof input.entries === 'function') return [...input.entries()];
        if (typeof input.list === 'function' && typeof input.get === 'function') {
            return input.list().map(name => [name, input.get(name)]);
        }
        return Object.entries(input);
    }
    throw new TypeError('Capability dispatcher registry must be a map or object');
}

function normalizeEntry(name, entry) {
    const capability = requiredName(name, 'Capability registry name');
    if (!isRecord(entry)) throw new TypeError(`Capability registry entry ${capability} must be an object`);
    const cardinality = entry.cardinality === undefined ? 1 : entry.cardinality;
    if (!Number.isInteger(cardinality) || cardinality < 1) throw new RangeError(`Capability ${capability} cardinality must be a positive integer`);
    if (typeof entry.execute !== 'function') throw new TypeError(`Capability ${capability} must provide execute()`);
    if (entry.markerAdapter !== undefined && entry.markerAdapter !== null && typeof entry.markerAdapter !== 'function') {
        throw new TypeError(`Capability ${capability} markerAdapter must be a function`);
    }
    if (entry.result !== undefined && typeof entry.result !== 'function') throw new TypeError(`Capability ${capability} result must be a function`);
    return Object.freeze({
        capability,
        cardinality,
        receiver: entry.receiver ?? entry,
        execute: entry.execute,
        markerAdapter: entry.markerAdapter || null,
        result: entry.result || null
    });
}

function syncResult(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Capability dispatcher ${field} must be synchronous`);
    return value;
}

function callName(call) {
    return typeof call?.name === 'string' ? call.name.trim() : '';
}

function callIndex(call, fallback) {
    return Number.isInteger(call?.index) && call.index >= 0 ? call.index : fallback;
}

function callSource(call) {
    return call?.source === 'marker' ? 'marker' : 'native';
}

function stableMarkerKey({name, personaId, causationUserMessageId, index, argumentsValue}) {
    let serialized;
    try {
        serialized = JSON.stringify(argumentsValue ?? null);
    } catch {
        serialized = String(argumentsValue);
    }
    const digest = createHash('sha256').update(serialized).digest('hex').slice(0, 24);
    return `marker:${name}:${personaId || 'unknown'}:${causationUserMessageId || 'unknown'}:${index}:${digest}`.slice(0, 240);
}

function defaultMarkerCall({name, argumentsValue, index, personaId, causationUserMessageId, idempotencyKey}) {
    const argumentsText = (() => {
        try { return JSON.stringify(argumentsValue ?? null); } catch { return ''; }
    })();
    return {
        id: null,
        index,
        name,
        argumentsText,
        arguments: argumentsValue ?? null,
        source: 'marker',
        personaId: personaId || '',
        causationUserMessageId: causationUserMessageId || '',
        idempotencyKey: idempotencyKey || stableMarkerKey({name, personaId, causationUserMessageId, index, argumentsValue})
    };
}

function normalizeMarkerOutput(value, text, descriptor, context, index, markerCallFactory) {
    if (value === null || value === undefined || value === false) return {text, call: null};
    if (typeof value === 'string') return {text: value, call: null};
    if (!isRecord(value)) throw new TypeError(`Capability ${descriptor.capability} markerAdapter returned an invalid result`);
    const nextText = value.text === undefined ? text : String(value.text);
    const suppliedCall = value.call ?? value.capabilityCall;
    if (suppliedCall) return {text: nextText, call: suppliedCall};
    if (value.arguments === undefined || value.arguments === null || value.arguments === false) return {text: nextText, call: null};
    const details = {
        name: descriptor.capability,
        argumentsValue: value.arguments,
        index,
        personaId: context.personaId,
        causationUserMessageId: context.causationUserMessageId,
        idempotencyKey: value.idempotencyKey
    };
    const call = markerCallFactory ? syncResult(markerCallFactory(details), 'markerCallFactory') : defaultMarkerCall(details);
    return {text: nextText, call};
}

function normalizeNativeCall(call, firstSeen) {
    if (!isRecord(call)) throw new TypeError('CapabilityCall must be an object');
    const name = requiredName(callName(call), 'CapabilityCall.name');
    return {
        ...call,
        name,
        index: callIndex(call, firstSeen),
        source: callSource(call),
        firstSeen
    };
}

function modeFor(value) {
    const mode = value === undefined ? 'execute' : value;
    if (mode !== 'execute' && mode !== 'plan') throw new TypeError(`Unsupported capability dispatcher mode: ${String(mode)}`);
    return mode;
}

function diagnosticsFor(values) {
    return values
        .filter(Boolean)
        .map(value => errorText(value))
        .slice(0, MAX_CAPABILITY_DIAGNOSTICS);
}

function nativeStateFor(value) {
    if (value instanceof Set) return new Set(value);
    if (Array.isArray(value)) return new Set(value);
    return new Set();
}

/**
 * Dispatch normalized capability envelopes while leaving transport parsing,
 * capability persistence, and provider behavior to injected adapters.
 */
export function createCapabilityDispatcher({registry, capabilityRegistry, markerCallFactory} = {}) {
    const entries = new Map(resolveRegistry(registry ?? capabilityRegistry).map(([name, entry]) => {
        const normalized = normalizeEntry(name, entry);
        return [normalized.capability, normalized];
    }));
    if (!entries.size) throw new TypeError('Capability dispatcher registry must not be empty');

    function names() {
        return [...entries.keys()];
    }

    function resultFor(entry, execution) {
        if (entry.result) {
            try {
                return syncResult(entry.result.call(entry.receiver, execution), `capability ${entry.capability}.result`);
            } catch (error) {
                execution.error ||= errorText(error);
            }
        }
        return {
            ok: !execution.error && execution.result !== null && execution.result !== undefined,
            error: execution.error || null,
            result: execution.result ?? null
        };
    }

    function executeCall(entry, call, mode, context, duplicateError = null) {
        const execution = {
            call,
            result: null,
            plan: null,
            intent: null,
            error: duplicateError,
            source: callSource(call),
            mode
        };
        if (execution.error) return execution;
        if (call.error) {
            execution.error = errorText(call.error);
            return execution;
        }
        if (call.incomplete || context.incompleteIndexes.has(call.index) || context.completion?.doneSeen !== true) {
            execution.error = 'Native capability call did not complete';
            return execution;
        }
        try {
            const outcome = syncResult(entry.execute.call(entry.receiver, {
                call,
                mode,
                capability: entry.capability,
                personaId: call.personaId ?? context.personaId ?? null,
                causationUserMessageId: call.causationUserMessageId ?? context.causationUserMessageId ?? null
            }), `capability ${entry.capability}.execute`);
            if (mode === 'plan') {
                if (!outcome?.plan) throw new Error('Capability plan did not return a plan reference');
                execution.plan = outcome.plan;
                execution.intent = outcome.intent || null;
                execution.result = outcome.result ?? outcome.plan.previewResult ?? outcome.plan.preview ?? null;
            } else {
                execution.result = outcome;
            }
        } catch (error) {
            execution.error = errorText(error);
        }
        return execution;
    }

    function dispatch(options = {}) {
        if (!isRecord(options)) throw new TypeError('Capability dispatch options must be an object');
        const mode = modeFor(options.mode);
        const completion = isRecord(options.completion) ? options.completion : {};
        const calls = Array.isArray(options.calls) ? options.calls : [];
        const incompleteIndexes = new Set(Array.isArray(completion.incompleteToolIndexes) ? completion.incompleteToolIndexes : []);
        const context = {
            personaId: options.personaId,
            causationUserMessageId: options.causationUserMessageId ?? options.causationId,
            completion,
            incompleteIndexes
        };
        const attempts = [];
        const diagnostics = Array.isArray(completion.parseErrors) ? completion.parseErrors.slice() : [];
        let sawUnknownNative = Boolean(options.blockMarkers);
        const nativeGroups = new Map(names().map(name => [name, []]));
        const normalizedNative = [];
        calls.forEach((raw, firstSeen) => {
            const call = normalizeNativeCall(raw, firstSeen);
            if (call.source !== 'native') return;
            normalizedNative.push(call);
            if (!entries.has(call.name)) {
                sawUnknownNative = true;
                attempts.push({call: null, raw, name: call.name, source: 'native', error: 'Unknown native capability'});
                return;
            }
            nativeGroups.get(call.name).push(call);
        });
        const nativeCapabilities = nativeStateFor(options.nativeState);
        for (const [name, group] of nativeGroups) if (group.length) nativeCapabilities.add(name);
        const byCapability = Object.fromEntries(names().map(name => [name, null]));
        const sortedNative = normalizedNative
            .filter(call => entries.has(call.name))
            .sort((left, right) => left.index - right.index || left.firstSeen - right.firstSeen);
        for (const call of sortedNative) {
            const entry = entries.get(call.name);
            const group = nativeGroups.get(call.name);
            const duplicateError = group.length > entry.cardinality
                ? `Duplicate ${call.name} capability call exceeds cardinality`
                : null;
            if (!byCapability[call.name]) {
                const execution = executeCall(entry, call, mode, context, duplicateError);
                byCapability[call.name] = execution;
                attempts.push(execution);
            } else {
                attempts.push({call, result: null, error: duplicateError || `Duplicate ${call.name} capability call was not executed`, source: 'native', mode});
            }
        }

        let visibleText = String(options.markerText || '');
        const markerContext = {
            personaId: options.personaId,
            causationUserMessageId: options.causationUserMessageId ?? options.causationId,
            mode
        };
        let markerIndex = Math.max(-1, ...sortedNative.map(call => call.index)) + 1;
        const stripOrExtract = (name, adapter, {executeMarker = true} = {}) => {
            if (!adapter) return;
            const index = markerIndex;
            try {
                const output = normalizeMarkerOutput(adapter.call(entries.get(name).receiver, visibleText, {...markerContext, capability: name, index}), visibleText, entries.get(name), {...markerContext, capability: name}, index, markerCallFactory);
                visibleText = output.text;
                if (!executeMarker || !output.call) return;
                markerIndex += 1;
                const call = normalizeNativeCall({...output.call, name, source: 'marker', index}, index);
                const execution = executeCall(entries.get(name), call, mode, context);
                byCapability[name] ||= execution;
                attempts.push(execution);
            } catch (error) {
                diagnostics.push(errorText(error));
            }
        };
        if (!sawUnknownNative) {
            for (const name of names()) {
                const entry = entries.get(name);
                if (nativeCapabilities.has(name)) stripOrExtract(name, entry.markerAdapter, {executeMarker: false});
                else stripOrExtract(name, entry.markerAdapter);
            }
        } else {
            for (const name of names()) stripOrExtract(name, entries.get(name).markerAdapter, {executeMarker: false});
        }

        const continuationEntries = sortedNative.map(call => {
            const execution = attempts.find(attempt => attempt.call === call) || {call, result: null, error: 'Capability call was not executed', mode};
            return {call, result: resultFor(entries.get(call.name), execution)};
        });
        return {
            mode,
            attempts,
            byCapability,
            visibleText,
            nativeCapabilities,
            unknownNative: sawUnknownNative,
            continuationEntries,
            diagnostics: diagnosticsFor([...diagnostics, ...attempts.map(attempt => attempt.error)])
        };
    }

    return Object.freeze({
        version: CAPABILITY_DISPATCHER_VERSION,
        dispatch,
        names,
        has(name) { return entries.has(name); },
        get(name) { return entries.get(name) || null; }
    });
}

export const createNativeCapabilityDispatcher = createCapabilityDispatcher;
export default createCapabilityDispatcher;
