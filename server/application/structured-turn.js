import {
    STRUCTURED_TURN_SCHEMA_VERSION,
    STRUCTURED_TURN_LIMITS,
    normalizeStructuredCapabilityCall,
    normalizeStructuredTurnEnvelope,
    normalizeStructuredTurnSafely
} from '../contracts/index.js';

const MARKER_PATTERN = /<(?:media-intent|pending-event|scene-event)>[\s\S]*?<\/(?:media-intent|pending-event|scene-event)>/i;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedDiagnostic(value) {
    const text = String(value ?? 'Structured turn control was rejected').replace(/\s+/g, ' ').trim();
    return text.length <= STRUCTURED_TURN_LIMITS.diagnostic
        ? text
        : `${text.slice(0, STRUCTURED_TURN_LIMITS.diagnostic - 3)}...`;
}

function diagnosticsFrom(value) {
    const values = [];
    for (const item of Array.isArray(value) ? value : []) {
        if (values.length >= STRUCTURED_TURN_LIMITS.diagnostics) break;
        values.push(boundedDiagnostic(item));
    }
    return values;
}

function visibleTextFrom(completion, sidecar) {
    if (typeof sidecar?.text === 'string') return sidecar.text;
    if (typeof completion?.text === 'string') return completion.text;
    if (typeof completion?.content === 'string') return completion.content;
    if (typeof completion?.message?.text === 'string') return completion.message.text;
    if (typeof completion?.message?.content === 'string') return completion.message.content;
    if (Array.isArray(completion?.tokens)) return completion.tokens.filter(token => typeof token === 'string').join('');
    if (Array.isArray(sidecar?.messages)) {
        return sidecar.messages
            .filter(message => message?.role === undefined || message?.role === 'assistant')
            .map(message => typeof message?.text === 'string' ? message.text : message?.content)
            .filter(text => typeof text === 'string')
            .join('');
    }
    return '';
}

function textTokens(completion, sidecar, text) {
    const tokens = Array.isArray(completion?.tokens)
        ? completion.tokens.filter(token => typeof token === 'string').slice(0, STRUCTURED_TURN_LIMITS.tokens)
        : [];
    return tokens.length ? tokens : text ? [text] : [];
}

/**
 * Read the optional provider sidecar without interpreting arbitrary model
 * text. Providers may expose a parsed object or a JSON string under one of
 * the conventional fields; the application still validates the envelope.
 */
export function readStructuredSidecar(value = {}) {
    if (!isRecord(value)) return {present: false, value: null, error: null};
    const candidates = [
        ['structuredTurn', value.structuredTurn],
        ['structured_turn', value.structured_turn],
        ['structuredSidecar', value.structuredSidecar],
        ['structured', value.structured],
        ['controlPayload', value.controlPayload],
        ['control_payload', value.control_payload],
        ['parsed', value.parsed],
        ['control', value.control],
        ['sidecar', value.sidecar]
    ];
    for (const [field, candidate] of candidates) {
        if (candidate === undefined || candidate === null) continue;
        if (typeof candidate === 'string') {
            try { return {present: true, value: JSON.parse(candidate), error: null, field}; }
            catch { return {present: true, value: null, error: `${field} is not valid JSON`, field}; }
        }
        return {present: true, value: candidate, error: null, field};
    }
    return {present: false, value: null, error: null};
}

function sidecarEnvelope(sidecar, completion) {
    if (!isRecord(sidecar)) throw new TypeError('Structured sidecar must be an object');
    return {
        ...sidecar,
        schemaVersion: sidecar.schemaVersion,
        text: sidecar.text ?? visibleTextFrom(completion, sidecar),
        tokens: sidecar.tokens ?? [],
        messages: sidecar.messages ?? [],
        control: sidecar.control ?? {
            affectEvents: sidecar.affectEvents,
            driveSignals: sidecar.driveSignals,
            memoryWrites: sidecar.memoryWrites,
            capabilityCalls: sidecar.capabilityCalls
        }
    };
}

function normalizeNativeCalls(calls, context, diagnostics) {
    const result = [];
    const source = Array.isArray(calls) ? calls : [];
    for (const call of source.slice(0, STRUCTURED_TURN_LIMITS.capabilityCalls)) {
        try {
            result.push(normalizeStructuredCapabilityCall({...call, source: call?.source ?? 'native'}, context));
        } catch (error) {
            diagnostics.push(boundedDiagnostic(error?.message));
            if (diagnostics.length >= STRUCTURED_TURN_LIMITS.diagnostics) break;
        }
    }
    return result;
}

/**
 * Normalize every supported completion form into one application-owned turn:
 * old text, existing native tool calls, a complete structured sidecar, and
 * legacy capability marker text. No provider stream decoding or second token
 * accumulator belongs here.
 */
export function normalizeStructuredTurn(completion = {}, context = {}) {
    const input = isRecord(completion) ? completion : {};
    const diagnostics = diagnosticsFrom(input.parseDiagnostics ?? input.parseErrors);
    const sidecarResult = readStructuredSidecar(input);
    if (sidecarResult.error) diagnostics.push(boundedDiagnostic(sidecarResult.error));
    const nativeCalls = normalizeNativeCalls(input.toolCalls ?? input.capabilityCalls, context, diagnostics);
    let sidecar = null;
    let sidecarControl = {affectEvents: [], driveSignals: [], memoryWrites: [], capabilityCalls: []};
    if (sidecarResult.present && !sidecarResult.error) {
        try {
            const candidate = sidecarEnvelope(sidecarResult.value, input);
            sidecar = normalizeStructuredTurnEnvelope(candidate, context);
            sidecarControl = sidecar.control;
        } catch (error) {
            diagnostics.push(boundedDiagnostic(error?.message));
        }
    }
    const text = visibleTextFrom(input, sidecar);
    const capabilityCalls = [...nativeCalls, ...sidecarControl.capabilityCalls]
        .slice(0, STRUCTURED_TURN_LIMITS.capabilityCalls);
    const sourceMode = sidecar
        ? 'structured_sidecar'
        : nativeCalls.length
            ? 'native_tools'
            : MARKER_PATTERN.test(text)
                ? 'legacy_marker'
                : 'text';
    const candidate = {
        schemaVersion: STRUCTURED_TURN_SCHEMA_VERSION,
        text,
        tokens: textTokens(input, sidecar, text),
        messages: sidecar?.messages ?? (Array.isArray(input.messages) ? input.messages : []),
        control: {
            affectEvents: sidecar?.control.affectEvents ?? [],
            driveSignals: sidecar?.control.driveSignals ?? [],
            memoryWrites: sidecar?.control.memoryWrites ?? [],
            capabilityCalls
        },
        parseDiagnostics: diagnostics,
        sourceMode
    };
    const normalized = normalizeStructuredTurnSafely(candidate, context);
    if (normalized.ok) return normalized.value;
    return {
        ...normalized.value,
        // Native/legacy candidates are independently bounded above. A bad
        // sidecar must not erase a valid visible reply or old tool call.
        control: {
            affectEvents: [],
            driveSignals: [],
            memoryWrites: [],
            capabilityCalls: nativeCalls
        },
        parseDiagnostics: [...diagnostics, boundedDiagnostic(normalized.error)].slice(0, STRUCTURED_TURN_LIMITS.diagnostics),
        sourceMode
    };
}

export const normalizeCompanionTurn = normalizeStructuredTurn;
export const normalizeCompanionTurnResult = normalizeStructuredTurn;
export const normalizeTurnResult = normalizeStructuredTurn;
export const normalizeTurn = normalizeStructuredTurn;

export function validateStructuredTurnResult(value, context = {}) {
    const normalized = normalizeStructuredTurn(value, context);
    return {
        ok: normalized.parseDiagnostics.length === 0,
        value: normalized,
        errors: normalized.parseDiagnostics.slice()
    };
}

export default normalizeStructuredTurn;
