/**
 * Application boundary for media diagnostics and asset delivery.
 *
 * This module only coordinates injected repositories, observability ports,
 * media-flow ports, settings, and provider ports. It deliberately has no
 * knowledge of an HTTP framework, a storage engine, provider clients, or the
 * legacy server root.
 */

export const MEDIA_DEBUG_SERVICE_VERSION = 1;
export const MEDIA_SOURCE_JOB_TYPES = Object.freeze([
    'activity_image',
    'activity_video',
    'chat_image',
    'chat_video'
]);
export const MEDIA_POLL_JOB_TYPES = Object.freeze(['activity_media_poll', 'chat_media_poll']);

const MAX_DEBUG_ITEMS = 10;
const MAX_POLL_ITEMS = 80;
const MAX_SUMMARY_LENGTH = 2_000;
const MAX_ERROR_LENGTH = 500;
const MAX_OUTPUT_LENGTH = 480;
const MAX_ELAPSED_MS = 7 * 24 * 60 * 60 * 1_000;
const MAX_JOBS_LIMIT = 200;
const SENSITIVE_KEY = /api[-_]?key|authorization|token|secret|password|credential|cookie/i;
const SENSITIVE_VALUE = /Bearer\s+[^\s,;]+|((?:api[-_]?key|token|secret|password|authorization)\s*[:=]\s*)[^\s,;]+/gi;
const ABSOLUTE_PATH = /(^|[\s=:])\/(?:[^\s,;\/]+\/){1,}[^\s,;]*/g;
const PROGRESS_STAGES = Object.freeze({
    queued: 'Queued',
    preparing: 'Preparing',
    generating: 'Generating',
    validating_output: 'Validating output',
    waiting_provider: 'Waiting for provider',
    complete: 'Complete',
    failed: 'Failed',
    unknown: 'Unknown'
});

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isPromise(value) {
    return Boolean(value) && typeof value.then === 'function';
}

function mapMaybe(value, mapper) {
    return isPromise(value) ? value.then(mapper) : mapper(value);
}

function allMaybe(values, mapper) {
    return values.some(isPromise) ? Promise.all(values).then(mapper) : mapper(values);
}

function text(value, field, maxLength = 240, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw Object.assign(new TypeError(`${field} must be a string`), {status: 400});
    const normalized = value.trim();
    if (!allowEmpty && !normalized) throw Object.assign(new TypeError(`${field} must not be empty`), {status: 400});
    if (normalized.length > maxLength) throw Object.assign(new RangeError(`${field} exceeds ${maxLength} characters`), {status: 400});
    return normalized;
}

function valueFor(value, camel, snake) {
    return value?.[camel] === undefined ? value?.[snake] : value[camel];
}

function parseJson(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed === null ? fallback : parsed;
    } catch {
        return fallback;
    }
}

function nowIso(clock) {
    const value = typeof clock === 'function' ? clock() : clock?.now?.();
    const date = value instanceof Date ? value : new Date(value ?? Date.now());
    return Number.isFinite(date.getTime()) ? date.toISOString() : new Date().toISOString();
}

function boundedText(value, limit = MAX_SUMMARY_LENGTH) {
    const source = value instanceof Error ? value.message : value;
    const normalized = String(source ?? '').replace(/\s+/g, ' ').trim();
    return normalized.length <= limit ? normalized : `${normalized.slice(0, Math.max(0, limit - 3))}...`;
}

function redactString(value, limit = MAX_SUMMARY_LENGTH) {
    const redacted = String(value ?? '')
        .replace(SENSITIVE_VALUE, '$1[redacted]')
        .replace(ABSOLUTE_PATH, '$1[path redacted]');
    return boundedText(redacted, limit);
}

function redact(value, key = '', depth = 0) {
    if (SENSITIVE_KEY.test(key)) return '[redacted]';
    if (depth > 8) return '[depth omitted]';
    if (typeof value === 'string') return redactString(value);
    if (value === null || value === undefined || typeof value === 'number' || typeof value === 'boolean') return value;
    if (Array.isArray(value)) return value.slice(0, 100).map(item => redact(item, '', depth + 1));
    if (!isRecord(value)) return redactString(value);
    return Object.fromEntries(Object.entries(value).slice(0, 100).map(([name, child]) => [name, redact(child, name, depth + 1)]));
}

export function redactMediaDebugValue(value) {
    return redact(value);
}

export function mediaDebugSummary(value, limit = MAX_SUMMARY_LENGTH) {
    const redacted = redact(value);
    const source = typeof redacted === 'string' ? redacted : JSON.stringify(redacted);
    return redactString(source ?? '', limit);
}

function method(value, names) {
    if (typeof value === 'function') return value;
    for (const name of names) if (typeof value?.[name] === 'function') return value[name].bind(value);
    return null;
}

function rows(value) {
    if (Array.isArray(value)) return value;
    if (isRecord(value) && Array.isArray(value.items)) return value.items;
    return [];
}

function jobType(job) {
    return valueFor(job, 'jobType', 'job_type');
}

function jobId(job) {
    return valueFor(job, 'id', 'job_id');
}

function personaIdFor(job) {
    return valueFor(job, 'personaId', 'persona_id');
}

function activityIdFor(job) {
    return valueFor(job, 'activityId', 'activity_id');
}

function messageIdFor(job) {
    return valueFor(job, 'messageId', 'message_id');
}

function rowTime(row, field, fallback = '') {
    const value = valueFor(row, field, field.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`));
    return typeof value === 'string' && Number.isFinite(Date.parse(value)) ? value : fallback;
}

function sortNewest(left, right) {
    const leftTime = Date.parse(rowTime(left, 'updatedAt', rowTime(left, 'createdAt', ''))) || 0;
    const rightTime = Date.parse(rowTime(right, 'updatedAt', rowTime(right, 'createdAt', ''))) || 0;
    if (leftTime !== rightTime) return rightTime - leftTime;
    return String(jobId(right) || right?.id || '').localeCompare(String(jobId(left) || left?.id || ''));
}

function targetKey(row) {
    const messageId = messageIdFor(row);
    if (messageId) return `message:${messageId}`;
    const activityId = activityIdFor(row);
    if (activityId) return `activity:${activityId}`;
    return `job:${jobId(row)}`;
}

function pollSourceId(row) {
    const payload = parseJson(valueFor(row, 'payload', 'payload_json'), {});
    const result = parseJson(valueFor(row, 'result', 'result_json'), {});
    return payload.sourceJobId ?? payload.source_job_id ?? result.sourceJobId ?? result.source_job_id ?? null;
}

function boolValue(value) {
    return value === true || value === 1 || value === '1' || String(value).toLowerCase() === 'true';
}

function kindFor(source, payload, pollPayload = {}) {
    if (payload.kind === 'image' || payload.kind === 'video') return payload.kind;
    if (pollPayload.kind === 'image' || pollPayload.kind === 'video') return pollPayload.kind;
    return /video/i.test(String(jobType(source) || '')) ? 'video' : 'image';
}

function fallbackStage(status) {
    if (status === 'queued') return 'queued';
    if (status === 'leased') return 'waiting_provider';
    if (status === 'complete') return 'complete';
    if (status === 'failed') return 'failed';
    return 'unknown';
}

function progressSnapshot(value, row, at, observability) {
    const source = isRecord(value) ? value : {};
    const injected = method(observability, ['progressSnapshot', 'projectProgress', 'progressForDebug']);
    let projected = source;
    if (injected) {
        try {
            const candidate = injected({progress: source, job: row, at});
            if (isRecord(candidate)) projected = candidate;
        } catch {
            projected = source;
        }
    }
    const status = valueFor(row, 'status', 'status') || 'unknown';
    const stageCandidate = String(projected.stage || '');
    const stage = Object.hasOwn(PROGRESS_STAGES, stageCandidate) ? stageCandidate : fallbackStage(status);
    const startedAt = rowTime(projected, 'startedAt', rowTime(row, 'createdAt', at));
    const updatedAt = rowTime(projected, 'updatedAt', rowTime(row, 'updatedAt', startedAt));
    const startedMs = Date.parse(startedAt);
    const persistedElapsed = Number(projected.elapsedMs);
    const elapsedMs = status === 'leased' && Number.isFinite(startedMs)
        ? Math.min(MAX_ELAPSED_MS, Math.max(0, Date.parse(at) - startedMs))
        : Math.min(MAX_ELAPSED_MS, Math.max(0, Number.isFinite(persistedElapsed) ? persistedElapsed : 0));
    const percentValue = Number(projected.percent);
    const percent = Number.isFinite(percentValue) ? Math.round(Math.min(100, Math.max(0, percentValue)) * 100) / 100 : null;
    const latestOutput = redactString(projected.latestOutput ?? projected.output ?? '', MAX_OUTPUT_LENGTH);
    const attempt = Number(valueFor(projected, 'attempt', 'attempt_count') ?? valueFor(row, 'attemptCount', 'attempt_count'));
    const outputLineCount = Number(projected.outputLineCount);
    return {
        schemaVersion: Number(projected.schemaVersion) || 1,
        attempt: Number.isFinite(attempt) ? Math.max(0, Math.floor(attempt)) : 0,
        stage,
        stageLabel: PROGRESS_STAGES[stage],
        percent,
        startedAt,
        updatedAt,
        elapsedMs,
        latestOutput,
        latestStream: projected.latestStream === 'stdout' || projected.latestStream === 'stderr' ? projected.latestStream : null,
        outputSeen: Boolean(projected.outputSeen || latestOutput),
        outputLineCount: Number.isFinite(outputLineCount) ? Math.max(0, Math.floor(outputLineCount)) : 0
    };
}

function messageDto(row) {
    if (!row) return null;
    const attachments = parseJson(valueFor(row, 'attachments', 'attachments_json'), []);
    const generation = parseJson(valueFor(row, 'generation', 'generation_json'), {});
    const jobs = parseJson(valueFor(row, 'jobs', 'jobs_json'), []);
    return {
        id: row.id,
        role: row.role,
        text: mediaDebugSummary(row.text || ''),
        attachments: redact(attachments),
        generation: redact(generation),
        jobs: redact(jobs),
        createdAt: valueFor(row, 'createdAt', 'created_at'),
        readAt: valueFor(row, 'readAt', 'read_at') ?? null
    };
}

function activityDto(row) {
    if (!row) return null;
    return {
        id: row.id,
        personaId: valueFor(row, 'personaId', 'persona_id'),
        eventId: valueFor(row, 'eventId', 'event_id') ?? null,
        content: mediaDebugSummary(row.content || ''),
        mediaMode: valueFor(row, 'mediaMode', 'media_mode') ?? 'none',
        mediaStatus: valueFor(row, 'mediaStatus', 'media_status') ?? 'none',
        createdAt: valueFor(row, 'createdAt', 'created_at')
    };
}

function assetDto(value) {
    if (!value) return null;
    const locator = parseJson(value.locator ?? value.locator_json, {});
    const id = value.id ?? value.mediaId ?? value.media_id;
    return {
        id,
        provider: value.provider ?? null,
        kind: value.kind ?? value.mediaKind ?? value.media_kind ?? 'image',
        mediaKind: value.mediaKind ?? value.media_kind ?? value.kind ?? 'image',
        filename: redactString(value.filename ?? '', 240),
        subfolder: redactString(value.subfolder ?? '', 240),
        fileType: value.fileType ?? value.file_type ?? 'output',
        locator: redact(locator),
        url: value.url ?? (id ? `/api/companion/media/${id}` : null),
        createdAt: value.createdAt ?? value.created_at ?? null,
        unavailableAt: value.unavailableAt ?? value.unavailable_at ?? null
    };
}

function targetDto({source, status, activity, message}) {
    const activityId = activityIdFor(source);
    const messageId = messageIdFor(source);
    return {
        type: activityId ? 'activity' : 'message',
        id: activityId || messageId || jobId(source),
        activityId: activityId || null,
        messageId: messageId || null,
        status,
        activity: activityDto(activity),
        message: messageDto(message)
    };
}

function providerIdFor(source, payload, result, pollPayload, pollResult) {
    return payload.provider || result.provider || pollPayload.provider || pollResult.provider || 'comfyui';
}

function externalIdFor(result, pollPayload, pollResult) {
    return result.externalId || result.external_id || pollResult.externalId || pollResult.external_id
        || pollPayload.externalId || pollPayload.external_id || result.promptId || pollResult.promptId || pollPayload.promptId || '';
}

function safeError(value) {
    return redactString(value, MAX_ERROR_LENGTH);
}

function failureDiagnostics({source, poll, result, pollResult, status, attempt, maxAttempts}) {
    const sourceError = result.providerError || result.providerFailure?.error || result.failedStage && result.error || source?.error || '';
    const pollError = pollResult.providerError || pollResult.providerFailure?.error || poll?.error || '';
    const error = safeError(pollError || sourceError);
    const stage = result.failedStage || pollResult.failedStage || result.providerFailure?.stage || pollResult.providerFailure?.stage || (pollError ? 'poll' : sourceError ? 'submit' : null);
    if (!error && !stage && status !== 'failed') return null;
    const attemptValue = Number(attempt);
    const maxValue = Number(maxAttempts);
    return {
        stage: stage || (status === 'failed' ? 'provider' : 'unknown'),
        error,
        sourceError: safeError(sourceError),
        pollError: safeError(pollError),
        retryable: status === 'queued' || status === 'leased' || (Number.isFinite(maxValue) && Number.isFinite(attemptValue) && attemptValue < maxValue),
        attempt: Number.isFinite(attemptValue) ? attemptValue : 0,
        maxAttempts: Number.isFinite(maxValue) ? maxValue : 0,
        stages: redact(result.stages || pollResult.stages || {})
    };
}

function settingReader(settings) {
    if (typeof settings === 'function') return settings;
    if (typeof settings?.read === 'function') return settings.read.bind(settings);
    if (typeof settings?.get === 'function') return settings.get.bind(settings);
    if (isRecord(settings)) return () => settings;
    return () => ({});
}

function boolSetting(value) {
    return value === true || value === 1 || value === '1' || ['true', 'on'].includes(String(value).toLowerCase());
}

function debugDisabledError() {
    return Object.assign(new Error('Debug inspector is disabled'), {status: 404, code: 'DEBUG_INSPECTOR_DISABLED'});
}

function notFoundError(message) {
    return Object.assign(new Error(message), {status: 404});
}

function providerError(providerId, cause) {
    return Object.assign(new Error(`Media provider ${redactString(providerId, 120)} is unavailable: ${safeError(cause || 'asset read failed')}`), {
        status: 502,
        code: 'MEDIA_PROVIDER_FAILED',
        providerId: redactString(providerId, 120),
        diagnostic: safeError(cause || 'asset read failed')
    });
}

function providerFor(providers, id) {
    if (providers?.find) return providers.find(id, {portType: 'media'});
    if (providers?.get) return providers.get(id, {portType: 'media'});
    if (providers instanceof Map) return providers.get(id) || null;
    return providers?.[id] ?? (providers?.readAsset ? providers : null);
}

function invokeReadAsset(providers, provider, providerId, input, response) {
    if (typeof providers?.readAsset === 'function') return providers.readAsset(providerId, input, response);
    const read = method(provider, ['readAsset', 'read', 'readCandidate']);
    if (!read) throw providerError(providerId, 'readAsset is unavailable');
    return read({...input, res: response});
}

function findPersona(personas, personaId) {
    if (typeof personas?.findActive === 'function') return personas.findActive(personaId);
    const lookup = method(personas, ['findById', 'get', 'find']);
    return lookup ? lookup({personaId, id: personaId}) : null;
}

function requireDebugPersona(personas, personaId) {
    if (!personas) return null;
    const value = findPersona(personas, personaId);
    if (isPromise(value)) return value.then(persona => persona || Promise.reject(notFoundError('Persona does not exist')));
    if (!value) throw notFoundError('Persona does not exist');
    return value;
}

function normalizeJobList(value, types) {
    return rows(value).filter(row => types.includes(jobType(row)));
}

function listJobs(jobRepository, personaId, types, limit) {
    const list = method(jobRepository, ['listMediaDebugJobs', 'listForPersona', 'list']);
    if (!list) return [];
    return mapMaybe(list({personaId, jobTypes: types, types, limit}), value => normalizeJobList(value, types).slice(0, limit));
}

function listMessages(conversationRepository, personaId, limit) {
    const list = method(conversationRepository, ['listMessages', 'listForPersona', 'recentForPersona']);
    if (!list) return [];
    return mapMaybe(list({personaId, limit}), rows);
}

function readMessage(conversationRepository, messageId, personaId, messages) {
    if (!messageId) return null;
    const cached = messages.find(row => row.id === messageId);
    if (cached) return cached;
    const find = method(conversationRepository, ['findMessage', 'findById', 'getMessage', 'get']);
    return find ? find({id: messageId, messageId, personaId}) : null;
}

function readActivity(activityRepository, activityId, personaId) {
    if (!activityId) return null;
    const find = method(activityRepository, ['findActivity', 'findById', 'get']);
    return find ? find({id: activityId, activityId, personaId}) : null;
}

function readAssets({assetRepository, activityRepository, source, result, message, personaId}) {
    const explicit = result.assets || result.attachments || parseJson(valueFor(message, 'attachments', 'attachments_json'), []);
    const output = Array.isArray(explicit) ? explicit.map(assetDto).filter(Boolean) : [];
    const listLinks = method(activityRepository, ['listActivityMedia', 'mediaForActivity']);
    const findAsset = method(assetRepository, ['find', 'findById', 'get']);
    if (!listLinks || !findAsset || !activityIdFor(source)) return output;
    const linked = listLinks({activityId: activityIdFor(source), personaId});
    return mapMaybe(linked, links => {
        const values = rows(links).map(link => findAsset({id: link.mediaId ?? link.media_id, personaId}));
        return allMaybe(values, resolved => {
            const merged = [...output, ...resolved.map(assetDto).filter(Boolean)];
            return [...new Map(merged.filter(asset => asset.id).map(asset => [asset.id, asset])).values()];
        });
    });
}

function sourceMediaDto({source, poll, message, activity, assets, settings, at, observability}) {
    const payload = parseJson(valueFor(source, 'payload', 'payload_json'), {});
    const result = parseJson(valueFor(source, 'result', 'result_json'), {});
    const pollPayload = parseJson(valueFor(poll, 'payload', 'payload_json'), {});
    const pollResult = parseJson(valueFor(poll, 'result', 'result_json'), {});
    const effective = poll ? {...source, status: valueFor(poll, 'status', 'status'), attempt_count: valueFor(poll, 'attemptCount', 'attempt_count'), updated_at: valueFor(poll, 'updatedAt', 'updated_at') || valueFor(source, 'updatedAt', 'updated_at')} : source;
    const status = valueFor(effective, 'status', 'status') || 'unknown';
    const kind = kindFor(source, payload, pollPayload);
    const provider = providerIdFor(source, payload, result, pollPayload, pollResult);
    const externalId = externalIdFor(result, pollPayload, pollResult);
    const progressSource = result.progress || null;
    const progressPoll = pollResult.progress || null;
    const snapshots = [
        {source: 'source', progress: progressSnapshot(progressSource, source, at, observability)},
        progressPoll ? {source: 'poll', progress: progressSnapshot(progressPoll, effective, at, observability)} : null
    ].filter(Boolean);
    const progress = progressSnapshot(progressPoll || progressSource, effective, at, observability);
    const finalPrompt = mediaDebugSummary(result.finalPrompt || pollResult.finalPrompt || '');
    const promptTemplate = mediaDebugSummary(result.promptTemplate || result.prompt_master_template || pollResult.promptTemplate || '');
    const personaConcept = mediaDebugSummary(result.personaConcept || result.personaMediaConcept || payload.personaMediaConcept || '');
    const envelope = mediaDebugSummary(payload.envelope || {kind});
    const diagnostic = failureDiagnostics({
        source,
        poll,
        result,
        pollResult,
        status,
        attempt: valueFor(effective, 'attemptCount', 'attempt_count'),
        maxAttempts: valueFor(source, 'maxAttempts', 'max_attempts')
    });
    return {
        id: jobId(source),
        sourceJobId: jobId(source),
        kind,
        type: kind,
        status,
        createdAt: valueFor(source, 'createdAt', 'created_at'),
        updatedAt: valueFor(effective, 'updatedAt', 'updated_at'),
        trigger: payload.trigger || payload.envelope?.trigger || 'unknown',
        provider,
        externalId: mediaDebugSummary(externalId, 240),
        envelope,
        capabilityCall: mediaDebugSummary(payload.capabilityCall || ''),
        personaConcept,
        promptTemplate,
        finalPrompt,
        promptMaster: {template: promptTemplate, finalPrompt},
        acceptance: mediaDebugSummary(result.acceptance || pollResult.acceptance || []),
        promptSummary: finalPrompt,
        progress,
        progressSnapshots: snapshots,
        workflowSummary: mediaDebugSummary({
            kind,
            provider,
            externalId,
            promptLength: result.promptLength || pollResult.promptLength || 0,
            stages: result.stages || pollResult.stages || {},
            failedStage: result.failedStage || pollResult.failedStage || '',
            workflowError: result.workflowError || pollResult.workflowError || ''
        }),
        error: diagnostic?.error || safeError(poll?.error || source.error || ''),
        providerFailure: diagnostic,
        providerDiagnostics: diagnostic,
        diagnostics: diagnostic,
        message: messageDto(message),
        target: targetDto({source, status, activity, message}),
        assets: Array.isArray(assets) ? assets : [],
        settings: {simplifiedMediaMode: boolSetting(settings?.simplifiedMediaMode)}
    };
}

function buildMediaDtos({sources, polls, messages, personaId, repositories, settings, at, observability}) {
    const pollBySource = new Map();
    const pollByTarget = new Map();
    for (const poll of [...polls].sort(sortNewest)) {
        const sourceId = pollSourceId(poll);
        if (sourceId && !pollBySource.has(sourceId)) pollBySource.set(sourceId, poll);
        const key = targetKey(poll);
        if (!pollByTarget.has(key)) pollByTarget.set(key, poll);
    }
    const values = sources.slice().sort(sortNewest).slice(0, MAX_DEBUG_ITEMS).map(source => {
        const poll = pollBySource.get(jobId(source)) || pollByTarget.get(targetKey(source)) || null;
        const message = readMessage(repositories.conversation, messageIdFor(source), personaId, messages);
        const activity = readActivity(repositories.activity, activityIdFor(source), personaId);
        return allMaybe([message, activity], ([resolvedMessage, resolvedActivity]) => {
            const assets = readAssets({
                assetRepository: repositories.mediaAsset || repositories.asset || repositories.assets,
                activityRepository: repositories.activity,
                source,
                result: parseJson(valueFor(source, 'result', 'result_json'), {}),
                message: resolvedMessage,
                personaId
            });
            return mapMaybe(assets, resolvedAssets => sourceMediaDto({
                source,
                poll,
                message: resolvedMessage,
                activity: resolvedActivity,
                assets: resolvedAssets,
                settings,
                at,
                observability
            }));
        });
    });
    return allMaybe(values, resolved => resolved);
}

export function createMediaDebugService({
    repositories = {},
    settings,
    providers,
    mediaFlow,
    mediaRequestPort,
    mediaJobService,
    observability,
    clock = () => new Date().toISOString(),
    enabled,
    debugInspectorEnabled,
    maxJobs = MAX_DEBUG_ITEMS,
    maxRecentRequests = MAX_DEBUG_ITEMS
} = {}) {
    if (!isRecord(repositories)) throw new TypeError('Media debug service repositories must be an object');
    const debugEnabled = enabled === undefined
        ? debugInspectorEnabled === true || debugInspectorEnabled === '1'
        : enabled === true || enabled === '1';
    const readSettings = settingReader(settings ?? repositories.settings);
    const jobRepository = repositories.jobRepository ?? repositories.job ?? repositories.effectRepository;
    const conversationRepository = repositories.conversationRepository ?? repositories.conversation;
    const personaRepository = repositories.personaRepository ?? repositories.persona ?? repositories.personas;
    const at = () => nowIso(clock);
    const boundedJobLimit = Math.min(MAX_DEBUG_ITEMS, Math.max(1, Math.floor(Number(maxJobs) || MAX_DEBUG_ITEMS)));
    const boundedRequestLimit = Math.min(MAX_DEBUG_ITEMS, Math.max(1, Math.floor(Number(maxRecentRequests) || MAX_DEBUG_ITEMS)));

    function requireEnabled() {
        if (!debugEnabled) throw debugDisabledError();
    }

    function readSafeSettings() {
        return mapMaybe(readSettings(), value => ({simplifiedMediaMode: boolSetting(value?.simplifiedMediaMode)}));
    }

    function getSettings() {
        return readSafeSettings();
    }

    function getDebugContext(command = {}) {
        requireEnabled();
        const personaId = text(command.personaId ?? command.persona_id, 'Persona id', 240);
        const persona = requireDebugPersona(personaRepository, personaId);
        const recent = listMessages(conversationRepository, personaId, boundedRequestLimit);
        const source = listJobs(jobRepository, personaId, MEDIA_SOURCE_JOB_TYPES, MAX_JOBS_LIMIT);
        const polls = listJobs(jobRepository, personaId, MEDIA_POLL_JOB_TYPES, MAX_POLL_ITEMS);
        const safeSettings = readSafeSettings();
        return allMaybe([persona, recent, source, polls, safeSettings], ([resolvedPersona, recentRows, sourceRows, pollRows, resolvedSettings]) => {
            const mediaJobs = buildMediaDtos({
                sources: sourceRows,
                polls: pollRows,
                messages: recentRows,
                personaId,
                repositories,
                settings: resolvedSettings,
                at: at(),
                observability
            });
            const recentRequests = recentRows.slice(0, boundedRequestLimit).map(row => ({
                id: row.id,
                createdAt: valueFor(row, 'createdAt', 'created_at'),
                status: row.role === 'assistant' ? 'response' : 'request',
                promptSummary: row.role === 'user' ? mediaDebugSummary(row.text || '') : '',
                responseSummary: row.role === 'assistant' ? mediaDebugSummary(row.text || '') : '',
                error: safeError(parseJson(valueFor(row, 'generation', 'generation_json'), {})?.error || '')
            }));
            return mapMaybe(mediaJobs, jobs => ({
                version: MEDIA_DEBUG_SERVICE_VERSION,
                personaId,
                persona: resolvedPersona ? {id: resolvedPersona.id, name: mediaDebugSummary(resolvedPersona.name || '', 240)} : null,
                settings: resolvedSettings,
                layers: {
                    identity: mediaDebugSummary(resolvedPersona || {}),
                    provider: mediaDebugSummary({configured: Boolean(providers), providers: providers?.summaries?.({portType: 'media', detailed: true}) || []})
                },
                recentRequests,
                mediaJobs: jobs
            }));
        });
    }

    function debugMedia(command = {}) {
        requireEnabled();
        const personaId = text(command.personaId ?? command.persona_id, 'Persona id', 240);
        const kind = command.kind === 'video' ? 'video' : command.kind === 'image' ? 'image' : null;
        if (!kind) throw Object.assign(new TypeError('Media kind must be image or video'), {status: 400});
        const request = command.request === undefined ? '' : text(command.request, 'Media request', 500, {allowEmpty: true});
        const count = command.count === undefined ? 1 : Number(command.count);
        if (!Number.isInteger(count) || count < 1 || count > 3) throw Object.assign(new RangeError('Media count must be between 1 and 3'), {status: 400});
        const persona = requireDebugPersona(personaRepository, personaId);
        const call = {
            kind,
            count,
            request,
            ...(isRecord(command.personaMediaConcept) ? {personaMediaConcept: command.personaMediaConcept} : {})
        };
        const input = {
            personaId,
            call,
            source: 'debug',
            trigger: 'debug_inspector',
            provenance: {source: 'debug', ...(command.causationUserMessageId ? {causationUserMessageId: command.causationUserMessageId} : {})}
        };
        const flow = mediaRequestPort ?? mediaFlow;
        let queued;
        if (flow?.plan && flow?.apply) {
            const plan = flow.plan(input, undefined, input.provenance);
            queued = mapMaybe(plan, value => flow.apply(value, command.transaction ? {transaction: command.transaction} : {}));
        } else {
            const requestMethod = method(flow, ['createChatMediaRequest', 'create', 'queue', 'submit', 'request'])
                || method(mediaJobService, ['debugMedia', 'createChatMediaRequest', 'queue']);
            if (!requestMethod) throw Object.assign(new Error('Media debug request port is unavailable'), {status: 501});
            queued = requestMethod(input);
        }
        return mapMaybe(allMaybe([persona, queued], ([resolvedPersona, result]) => result), result => result);
    }

    function getMedia(command = {}, context = {}) {
        const mediaId = text(command.mediaId ?? command.media_id ?? command.id, 'Media id', 240);
        const assetRepository = repositories.mediaAsset ?? repositories.asset ?? repositories.assets;
        const findAsset = method(assetRepository, ['find', 'findById', 'get']);
        if (!findAsset) throw Object.assign(new Error('Media asset repository is unavailable'), {status: 501});
        const found = findAsset({id: mediaId, mediaId, personaId: command.personaId ?? command.persona_id});
        return mapMaybe(found, asset => {
            if (!asset) throw notFoundError('Media does not exist');
            const shaped = assetDto(asset);
            const providerId = shaped.provider;
            const provider = providerFor(providers, providerId);
            const response = context.response ?? context.res ?? command.response ?? command.res;
            try {
                const result = invokeReadAsset(providers, provider, providerId, {
                    asset: {...asset, locator: parseJson(asset.locator ?? asset.locator_json, {})},
                    settings: readSettings()
                }, response);
                const output = mapMaybe(result, value => value === undefined ? undefined : value);
                return isPromise(output) ? output.catch(error => { throw providerError(providerId, error); }) : output;
            } catch (error) {
                throw providerError(providerId, error);
            }
        });
    }

    const aliases = {
        getDebugContext,
        debugContextFor: getDebugContext,
        getContext: getDebugContext,
        debugMedia,
        getMedia,
        readAsset: getMedia,
        getSettings,
        simplifiedMediaMode() { return mapMaybe(readSafeSettings(), value => value.simplifiedMediaMode); }
    };
    return Object.freeze({
        version: MEDIA_DEBUG_SERVICE_VERSION,
        enabled: debugEnabled,
        ...aliases,
        debug: Object.freeze({...aliases})
    });
}

export const createMediaDebugApplicationService = createMediaDebugService;
export default createMediaDebugService;
