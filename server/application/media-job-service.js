/**
 * Application orchestration for durable media jobs.
 *
 * The service knows job lifecycle and media contracts, but it does not know
 * how a provider talks to an upstream service or how a repository persists
 * rows. Every side effect is supplied through a port so this module can be
 * imported by worker and application tests without starting the server.
 */

export const MEDIA_SUBMIT_JOB_TYPES = Object.freeze([
    'activity_image',
    'activity_video',
    'chat_image',
    'chat_video'
]);

export const MEDIA_POLL_JOB_TYPES = Object.freeze(['activity_media_poll', 'chat_media_poll']);
export const MEDIA_JOB_SERVICE_VERSION = 1;

const DEFAULT_RETRY_DELAY_MS = 1_000;
const MAX_RETRY_DELAY_MS = 15 * 60 * 1_000;
const MAX_ERROR_LENGTH = 500;
const MAX_EXTERNAL_ID_LENGTH = 2_048;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredRecord(value, field) {
    if (!isRecord(value)) throw new TypeError(`Media job service ${field} must be an object`);
    return value;
}

function requiredFunction(value, field) {
    if (typeof value !== 'function') throw new TypeError(`Media job service requires ${field}()`);
    return value;
}

function text(value, field, maxLength = 240, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const normalized = value.trim();
    if (!allowEmpty && !normalized) throw new TypeError(`${field} must not be empty`);
    if (normalized.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return normalized;
}

function errorText(value, fallback = 'Media job failed') {
    const source = value instanceof Error ? value.message : value;
    const normalized = String(source ?? fallback).replace(/\s+/g, ' ').trim() || fallback;
    if (normalized.length <= MAX_ERROR_LENGTH) return normalized;
    return `${normalized.slice(0, MAX_ERROR_LENGTH - 3)}...`;
}

function nowValue(clock) {
    const value = typeof clock === 'function' ? clock() : clock.now();
    const date = value instanceof Date ? value : new Date(value);
    if (!Number.isFinite(date.getTime())) throw new TypeError('Media job service clock must return a valid date');
    return date.toISOString();
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => nowValue(clock);
    if (isRecord(clock) && typeof clock.now === 'function') return () => nowValue(clock);
    throw new TypeError('Media job service clock must be a function or provide now()');
}

function valueFor(value, camel, snake) {
    return value?.[camel] === undefined ? value?.[snake] : value[camel];
}

function jobId(job) {
    return text(valueFor(job, 'id', 'id'), 'Media job id', 160);
}

function jobType(job) {
    return text(valueFor(job, 'jobType', 'job_type'), 'Media job type', 80);
}

function personaId(job) {
    const value = valueFor(job, 'personaId', 'persona_id');
    return value === null || value === undefined ? undefined : text(value, 'Media job personaId', 160);
}

function leaseOwner(job, context = {}) {
    const value = context.leaseOwner ?? context.owner ?? valueFor(job, 'leaseOwner', 'lease_owner');
    return value === null || value === undefined ? undefined : text(value, 'Media job leaseOwner', 160);
}

function attemptInfo(job) {
    const attempt = Number(valueFor(job, 'attemptCount', 'attempt_count'));
    const maximum = Number(valueFor(job, 'maxAttempts', 'max_attempts'));
    return {
        attempt: Number.isInteger(attempt) && attempt > 0 ? attempt : 1,
        maximum: Number.isInteger(maximum) && maximum > 0 ? maximum : 1
    };
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

function payloadFor(job) {
    return parseJson(job?.payload ?? job?.payload_json, {});
}

function resultFor(job) {
    return parseJson(job?.result ?? job?.result_json, {});
}

function kindFor(job, payload = payloadFor(job)) {
    if (payload.kind === 'image' || payload.kind === 'video') return payload.kind;
    return /video/.test(jobType(job)) ? 'video' : 'image';
}

function operationFor(type) {
    if (MEDIA_SUBMIT_JOB_TYPES.includes(type)) return 'submit';
    if (MEDIA_POLL_JOB_TYPES.includes(type)) return 'poll';
    return null;
}

function hasMethod(value, names) {
    if (typeof value === 'function') return value;
    if (!isRecord(value)) return null;
    for (const name of names) {
        if (typeof value[name] === 'function') return value[name].bind(value);
    }
    return null;
}

function resolveRepository(repositories, names, field, {optional = false} = {}) {
    for (const name of names) {
        if (repositories?.[name] !== undefined) {
            if (!isRecord(repositories[name])) throw new TypeError(`Media job service ${field} must be an object`);
            return repositories[name];
        }
    }
    if (optional) return null;
    throw new TypeError(`Media job service requires ${field}`);
}

function providerId(provider, configured) {
    const value = typeof configured === 'string' ? configured : configured?.id;
    return text(value ?? provider?.id, 'Media provider id', 80);
}

function providerFor(providers, kind, configured) {
    const requested = typeof configured === 'string' && configured.trim() ? configured.trim() : 'comfyui';
    let provider = null;
    if (providers && typeof providers.get === 'function') {
        provider = providers.get(requested, {portType: 'media', capability: kind});
    } else if (providers && typeof providers.find === 'function') {
        provider = providers.find(requested, {portType: 'media', capability: kind});
    } else if (providers instanceof Map) {
        provider = providers.get(requested) || null;
    } else if (isRecord(providers)) {
        provider = providers[requested];
    }
    if (!provider && providers && typeof providers.submit === 'function') provider = providers;
    if (!provider || typeof provider.submit !== 'function' || typeof provider.poll !== 'function') {
        throw new Error(`Media provider is unavailable: ${requested}`);
    }
    const id = providerId(provider, requested);
    const capabilities = Array.isArray(provider.capabilities) ? provider.capabilities : null;
    if (capabilities && !capabilities.includes(kind)) throw new Error(`Media provider ${id} does not support ${kind}`);
    return {id, provider};
}

function methodPort(value, names, field, {optional = false} = {}) {
    const method = hasMethod(value, names);
    if (!method && !optional) throw new TypeError(`Media job service ${field} must provide ${names[0]}()`);
    return method;
}

function normalizePromptResult(value) {
    if (typeof value === 'string') return {finalPrompt: text(value, 'Media provider prompt', 32_000)};
    requiredRecord(value, 'prompt master result');
    const finalPrompt = value.finalPrompt ?? value.prompt;
    if (finalPrompt !== undefined) {
        return {
            ...value,
            finalPrompt: text(finalPrompt, 'Media provider prompt', 32_000)
        };
    }
    if (value.template !== undefined) return {...value, template: value.template};
    if (value.promptTemplate !== undefined) return {...value, promptTemplate: value.promptTemplate};
    throw new TypeError('Media prompt master did not return a prompt or template');
}

function normalizeAcceptance(value) {
    if (value === undefined || value === null) return {verdict: 'pass'};
    if (typeof value === 'string') return {verdict: value.trim().toLowerCase()};
    requiredRecord(value, 'acceptance result');
    const verdict = String(value.verdict ?? value.status ?? 'pass').trim().toLowerCase();
    if (!['pass', 'retry', 'reject', 'skipped'].includes(verdict)) throw new Error('Media acceptance verdict is invalid');
    return {...value, verdict};
}

function outcome(status, result, error, extra = {}) {
    return {
        status,
        ...(result === undefined ? {} : {result}),
        ...(error ? {error: errorText(error)} : {}),
        ...extra
    };
}

function mergeResult(current, patch) {
    const base = isRecord(current) ? current : {};
    const next = isRecord(patch) ? patch : {};
    return {...base, ...Object.fromEntries(Object.entries(next).filter(([, value]) => value !== undefined))};
}

function externalIdOf(value) {
    const id = value?.externalId ?? value?.external_id ?? value?.promptId ?? value?.prompt_id;
    if (typeof id !== 'string' || !id.trim() || id.length > MAX_EXTERNAL_ID_LENGTH) {
        throw new Error('Media provider did not return a valid external id');
    }
    return id.trim();
}

function retryAtFor(job, now, retryDelayMs, maxRetryDelayMs) {
    const requested = job?.retryAt ?? job?.retry_at ?? job?.runAfter ?? job?.run_after;
    if (requested !== undefined && Number.isFinite(Date.parse(requested))) return new Date(requested).toISOString();
    const {attempt} = attemptInfo(job);
    const delay = Math.min(maxRetryDelayMs, retryDelayMs * (2 ** Math.min(Math.max(attempt - 1, 0), 8)));
    return new Date(Date.parse(now) + delay).toISOString();
}

function methodResult(value) {
    return value && typeof value.then === 'function' ? value : Promise.resolve(value);
}

/**
 * Compose media job handlers for the generic durable dispatcher.
 *
 * `repositories` must provide a job repository. A media-flow port may expose
 * target projection, asset persistence, prompt-result, and poll-enqueue
 * methods; each is optional because the worker can be tested with only a job
 * repository. Provider adapters and model-owned stages are always injected.
 */
export function createMediaJobService({
    providers,
    observability,
    repositories,
    promptMaster,
    acceptance,
    clock,
    retryDelayMs = DEFAULT_RETRY_DELAY_MS,
    maxRetryDelayMs = MAX_RETRY_DELAY_MS
} = {}) {
    if (!providers) throw new TypeError('Media job service requires providers');
    const repositoryGroup = requiredRecord(repositories, 'repositories');
    const jobRepository = resolveRepository(repositoryGroup, ['jobRepository', 'job', 'effectRepository'], 'job repository');
    const mediaFlow = resolveRepository(repositoryGroup, ['mediaFlow', 'media', 'mediaRepository'], 'media flow', {optional: true});
    const now = clockFor(clock);
    const retryBase = Number(retryDelayMs);
    const retryMax = Number(maxRetryDelayMs);
    if (!Number.isFinite(retryBase) || retryBase < 0) throw new RangeError('Media job service retryDelayMs must be non-negative');
    if (!Number.isFinite(retryMax) || retryMax < retryBase) throw new RangeError('Media job service maxRetryDelayMs must be >= retryDelayMs');

    const findLeased = methodPort(jobRepository, ['findLeased', 'getLeased', 'isClaimed'], 'job repository lease guard');
    const patchResult = methodPort(jobRepository, ['patchResult', 'recordResult'], 'job repository result writer', {optional: true});
    const findJob = methodPort(jobRepository, ['find', 'findById', 'get'], 'job repository lookup', {optional: true});
    const enqueue = methodPort(jobRepository, ['enqueue', 'create', 'insert'], 'job repository enqueue', {optional: true});
    const repositorySettle = methodPort(jobRepository, ['settle', 'settleJob'], 'job repository settlement', {optional: true});
    const repositoryRetry = methodPort(jobRepository, ['retry', 'retryJob'], 'job repository retry', {optional: true});
    const flowRecordResult = methodPort(mediaFlow, ['recordResult', 'recordJobResult', 'patchResult'], 'media flow result writer', {optional: true});
    const flowUpdateTarget = methodPort(mediaFlow, ['updateTarget', 'updateMediaTarget', 'settleTarget'], 'media flow target writer', {optional: true});
    const flowPersistAssets = methodPort(mediaFlow, ['persistAssets', 'mediaAssets', 'saveAssets'], 'media flow asset writer', {optional: true});
    const flowEnqueuePoll = methodPort(mediaFlow, ['enqueuePoll', 'schedulePoll', 'createPollJob'], 'media flow poll enqueue', {optional: true});
    const flowQualityRetry = methodPort(mediaFlow, ['enqueueQualityRetry', 'scheduleQualityRetry'], 'media flow quality retry', {optional: true});
    const observeSettle = methodPort(observability, ['settle'], 'observability settlement', {optional: true});
    const makeReporter = methodPort(observability, ['createReporter'], 'observability reporter', {optional: true});

    const promptFill = typeof promptMaster === 'function'
        ? promptMaster
        : methodPort(promptMaster, ['fill', 'fillMediaPromptTemplate', 'create'], 'prompt master', {optional: true});
    const promptRender = methodPort(promptMaster, ['render', 'renderMediaPromptTemplate'], 'prompt renderer', {optional: true});
    const accept = typeof acceptance === 'function'
        ? acceptance
        : methodPort(acceptance, ['accept', 'evaluate', 'check', 'run'], 'acceptance', {optional: true});

    async function activeLease(job, context = {}) {
        const id = jobId(job);
        const owner = leaseOwner(job, context);
        if (!owner) return job;
        const checked = await methodResult(findLeased({
            id,
            leaseOwner: owner,
            ...(personaId(job) === undefined ? {} : {personaId: personaId(job)}),
            now: context.now ?? now()
        }));
        return checked || null;
    }

    async function writeResult(job, patch, context = {}) {
        const active = await activeLease(job, context);
        if (!active) return {changed: false, reason: 'lease_rejected', result: null, job: null};
        const input = {patch, now: context.now ?? now(), leaseOwner: leaseOwner(job, context), personaId: personaId(job)};
        const writer = flowRecordResult || patchResult;
        if (!writer) return {changed: false, reason: 'result_writer_unavailable', result: null, job: active};
        const result = await methodResult(flowRecordResult
            ? flowRecordResult({job: active, ...input})
            : patchResult(active, input));
        return result ?? {changed: true, result: mergeResult(resultFor(active), patch), job: active};
    }

    function settlementInput(job, status, options, context = {}) {
        const settledAt = context.now ?? now();
        return {
            id: jobId(job),
            personaId: personaId(job),
            leaseOwner: leaseOwner(job, context),
            status,
            terminal: status === 'failed',
            runAfter: options.runAfter ?? (status === 'retry' ? retryAtFor(job, settledAt, retryBase, retryMax) : settledAt),
            result: options.result,
            error: options.error ? errorText(options.error) : null,
            now: settledAt
        };
    }

    async function settle(job, options = {}, context = {}) {
        requiredRecord(job, 'job');
        const status = options.status ?? (options.retry ? 'retry' : options.terminal ? 'failed' : 'complete');
        if (!['complete', 'retry', 'failed'].includes(status)) throw new TypeError('Media job settlement status is invalid');
        const active = await activeLease(job, context);
        if (!active) return {changed: false, reason: 'lease_rejected', status: null, job: null};
        const input = settlementInput(active, status, options, context);
        if (context.deferSettlement === true) {
            // A generic durable dispatcher can own the single repository
            // transition while the media service still guards the lease and
            // returns the intended outcome to that dispatcher.
            return {changed: true, deferred: true, status, job: active, input};
        }
        let result;
        if (observeSettle) {
            result = await methodResult(observeSettle(active, input));
        } else if (status === 'retry' && repositoryRetry) {
            result = await methodResult(repositoryRetry(input));
        } else if (status !== 'retry' && repositorySettle) {
            result = await methodResult(repositorySettle(input));
        } else {
            throw new TypeError('Media job service has no settlement port');
        }
        const changed = result?.changed === undefined ? Boolean(result) : Boolean(result.changed);
        return {...(result || {}), changed, status: result?.status ?? (changed ? status : null), job: result?.job ?? null};
    }

    async function fail(job, error, context = {}, extra = {}) {
        const {attempt, maximum} = attemptInfo(job);
        const terminal = extra.terminal === true || error?.terminal === true || error?.retryable === false || attempt >= maximum;
        const status = terminal ? 'failed' : 'retry';
        const result = mergeResult(extra.result, {failedStage: extra.failedStage, ...(extra.stages ? {stages: extra.stages} : {})});
        // Target projections are guarded with the same lease token as the
        // eventual transition. This keeps a stale worker from publishing a
        // terminal error after a later attempt has claimed the job.
        if (status === 'failed') await updateTarget(job, {status: 'failed', error: errorText(error)}, context);
        const transition = await settle(job, {status, result, error, terminal, runAfter: terminal ? undefined : retryAtFor(job, context.now ?? now(), retryBase, retryMax)}, context);
        return outcome(transition.status ?? status, result, error, {changed: transition.changed, settlement: transition});
    }

    async function updateTarget(job, patch, context = {}) {
        const active = await activeLease(job, context);
        if (!active) return {changed: false, reason: 'lease_rejected'};
        if (!flowUpdateTarget) return {changed: false, reason: 'target_writer_unavailable'};
        return await methodResult(flowUpdateTarget({job: active, ...patch, now: context.now ?? now()})) ?? {changed: true};
    }

    async function promptFor(job, payload, context = {}) {
        const stored = resultFor(job);
        if (typeof stored.finalPrompt === 'string' && stored.finalPrompt.trim() && !payload.priorAcceptance) {
            return {finalPrompt: stored.finalPrompt, ...(stored.promptTemplate ? {promptTemplate: stored.promptTemplate} : {})};
        }
        if (!promptFill) {
            if (typeof payload.finalPrompt === 'string' && payload.finalPrompt.trim()) return {finalPrompt: payload.finalPrompt.trim()};
            throw new Error('Media prompt master is unavailable');
        }
        const source = {
            envelope: payload.envelope,
            concept: payload.personaMediaConcept,
            personaMediaConcept: payload.personaMediaConcept,
            priorAcceptance: payload.priorAcceptance ?? null,
            kind: kindFor(job, payload),
            job,
            context
        };
        const filled = normalizePromptResult(await methodResult(promptFill(source)));
        if (filled.finalPrompt) return filled;
        const template = filled.promptTemplate ?? filled.template;
        if (!promptRender || template === undefined) throw new Error('Media prompt master did not return a renderable template');
        const rendered = await methodResult(promptRender(template));
        return {...filled, promptTemplate: template, finalPrompt: text(rendered, 'Media provider prompt', 32_000)};
    }

    async function evaluateAcceptance(job, provider, files, context = {}) {
        if (!accept) return {verdict: 'pass'};
        const result = await methodResult(accept({
            job,
            sourceJob: job,
            provider,
            files: Array.isArray(files) ? files : [],
            payload: payloadFor(job),
            context
        }));
        return normalizeAcceptance(result);
    }

    async function sourceJobFor(job) {
        const sourceId = payloadFor(job).sourceJobId ?? payloadFor(job).source_job_id;
        if (!sourceId || !findJob) return job;
        const source = await methodResult(findJob({id: sourceId, personaId: personaId(job)}));
        return source || job;
    }

    async function persistAssets(job, provider, files, context = {}) {
        if (flowPersistAssets) {
            return await methodResult(flowPersistAssets({job, provider: provider.id ?? provider, files: files || [], now: context.now ?? now()}));
        }
        return Array.isArray(files) ? files.slice(0, 3) : [];
    }

    async function queuePoll(job, provider, externalId, kind, context = {}) {
        const payload = payloadFor(job);
        const pollPayload = {
            sourceJobId: jobId(job),
            provider: provider.id,
            externalId,
            promptId: externalId,
            kind
        };
        if (flowEnqueuePoll) return methodResult(flowEnqueuePoll({job, payload: pollPayload, now: context.now ?? now()}));
        if (!enqueue) return null;
        const type = valueFor(job, 'jobType', 'job_type').startsWith('activity_') ? 'activity_media_poll' : 'chat_media_poll';
        return methodResult(enqueue({
            jobType: type,
            personaId: personaId(job),
            activityId: valueFor(job, 'activityId', 'activity_id') ?? null,
            messageId: valueFor(job, 'messageId', 'message_id') ?? null,
            priority: 4,
            maxAttempts: 60,
            runAfter: context.now ?? now(),
            payload: pollPayload,
            now: context.now ?? now()
        }));
    }

    async function qualityRetry(job, acceptanceResult, context = {}) {
        const payload = payloadFor(job);
        const used = Number(payload.qualityRetryCount || 0);
        const limit = Number(payload.maxQualityRetries ?? 1);
        if (acceptanceResult.verdict !== 'retry' || used >= limit) return null;
        const retryPayload = {
            ...payload,
            qualityRetryCount: used + 1,
            maxQualityRetries: limit,
            priorAcceptance: {
                violations: acceptanceResult.violations || [],
                retryGuidance: acceptanceResult.retryGuidance || ''
            }
        };
        if (flowQualityRetry) return methodResult(flowQualityRetry({job, payload: retryPayload, acceptance: acceptanceResult, now: context.now ?? now()}));
        if (!enqueue) return null;
        return methodResult(enqueue({
            jobType: jobType(job),
            personaId: personaId(job),
            activityId: valueFor(job, 'activityId', 'activity_id') ?? null,
            messageId: valueFor(job, 'messageId', 'message_id') ?? null,
            priority: valueFor(job, 'priority', 'priority') ?? 4,
            maxAttempts: valueFor(job, 'maxAttempts', 'max_attempts') ?? 3,
            payload: retryPayload,
            runAfter: context.now ?? now(),
            now: context.now ?? now()
        }));
    }

    async function completeGenerated(job, provider, externalId, files, context = {}, sourceJob = job) {
        // Poll children carry only the external locator. Acceptance and a
        // quality retry must still receive the frozen source envelope.
        const acceptanceResult = await evaluateAcceptance(sourceJob, provider.provider ?? provider, files, context);
        const acceptancePatch = {acceptance: [...(Array.isArray(resultFor(job).acceptance) ? resultFor(job).acceptance.slice(-2) : []), acceptanceResult]};
        await writeResult(job, acceptancePatch, context);
        if (acceptanceResult.verdict === 'retry' && Number(payloadFor(job).qualityRetryCount || 0) < Number(payloadFor(job).maxQualityRetries ?? 1)) {
            const queued = await qualityRetry(sourceJob, acceptanceResult, context);
            if (queued) {
                await updateTarget(job, {status: 'queued'}, context);
                const transition = await settle(job, {status: 'complete', result: {acceptance: acceptanceResult, qualityRetryQueued: true}}, context);
                return outcome(transition.status ?? 'complete', {acceptance: acceptanceResult, qualityRetryQueued: true}, null, {changed: transition.changed, settlement: transition});
            }
        }
        if (acceptanceResult.verdict === 'reject' || acceptanceResult.verdict === 'retry') {
            const safeError = 'Media result did not satisfy the requested intent';
            await updateTarget(job, {status: 'failed', error: safeError}, context);
            const transition = await settle(job, {status: 'failed', result: {acceptance: acceptanceResult, rejected: true}, error: safeError, terminal: true}, context);
            return outcome('failed', {acceptance: acceptanceResult, rejected: true}, safeError, {changed: transition.changed, settlement: transition});
        }
        const assets = await persistAssets(job, provider, files, context);
        const result = {provider: provider.id, externalId, promptId: externalId, pending: false, files: files || [], assets, acceptance: acceptanceResult};
        await writeResult(job, result, context);
        await updateTarget(job, {status: 'ready', provider: provider.id, externalId, promptId: externalId, attachments: assets}, context);
        const transition = await settle(job, {status: 'complete', result}, context);
        return outcome(transition.status ?? 'complete', result, null, {changed: transition.changed, settlement: transition});
    }

    async function submit(job, context = {}) {
        requiredRecord(job, 'job');
        const payload = payloadFor(job);
        const kind = kindFor(job, payload);
        if (!isRecord(payload.personaMediaConcept) || payload.personaMediaConcept.mediaKind !== kind) {
            return fail(job, new Error('Media job has no valid frozen media concept'), context, {terminal: true, failedStage: 'missing_frozen_media_concept'});
        }
        if (!(await activeLease(job, context))) return outcome('stale', null, null, {changed: false, reason: 'lease_rejected'});
        let provider;
        let reporter;
        let prompt;
        try {
            prompt = await promptFor(job, payload, context);
            provider = providerFor(providers, kind, payload.provider);
            const promptResult = {provider: provider.id, ...prompt, promptLength: prompt.finalPrompt.length, stages: {promptMaster: {status: 'complete'}}};
            if (makeReporter) {
                reporter = await methodResult(makeReporter(job));
                reporter?.stage?.('preparing');
            }
            const saved = await writeResult(job, promptResult, context);
            if (!saved.changed) return outcome('stale', null, null, {changed: false, reason: 'lease_rejected'});
            const submitted = await methodResult(provider.provider.submit({
                kind,
                prompt: prompt.finalPrompt,
                payload: {...payload, prompt: prompt.finalPrompt},
                settings: context.settings ?? payload.settings,
                progress: reporter,
                job
            }));
            const externalId = externalIdOf(submitted);
            if (submitted?.pending === false && Array.isArray(submitted.files)) {
                return completeGenerated(job, provider, externalId, submitted.files, context, job);
            }
            const result = {...promptResult, externalId, promptId: externalId, pending: true};
            await updateTarget(job, {status: 'processing', provider: provider.id, externalId, promptId: externalId}, context);
            const transition = await settle(job, {status: 'complete', result}, context);
            if (!transition.changed) return outcome('stale', result, null, {changed: false, reason: 'lease_rejected'});
            await queuePoll(job, provider, externalId, kind, context);
            return outcome('complete', result, null, {changed: true, settlement: transition});
        } catch (error) {
            reporter?.flush?.();
            return fail(job, error, context, {failedStage: provider ? 'provider' : 'prompt_master', result: prompt ? {finalPrompt: prompt.finalPrompt} : undefined});
        }
    }

    async function poll(job, context = {}) {
        requiredRecord(job, 'job');
        if (!(await activeLease(job, context))) return outcome('stale', null, null, {changed: false, reason: 'lease_rejected'});
        const payload = payloadFor(job);
        const kind = kindFor(job, payload);
        try {
            const selected = providerFor(providers, kind, payload.provider);
            const externalId = text(payload.externalId ?? payload.promptId, 'Media external id', MAX_EXTERNAL_ID_LENGTH);
            const polled = await methodResult(selected.provider.poll({kind, externalId, settings: context.settings ?? payload.settings, payload, job}));
            if (polled?.status === 'complete') return completeGenerated(job, selected, externalId, polled.files || [], context, await sourceJobFor(job));
            if (polled?.status === 'failed') throw new Error(polled.error || `Media provider ${selected.id} failed`);
            throw new Error(`Media provider ${selected.id} has not returned a result`);
        } catch (error) {
            return fail(job, error, context, {failedStage: 'provider'});
        }
    }

    function handlerFor(type, operation) {
        return async (job, context = {}) => (operation === 'submit' ? submit(job, context) : poll(job, context));
    }

    const handlerMap = {};
    for (const type of MEDIA_SUBMIT_JOB_TYPES) handlerMap[type] = handlerFor(type, 'submit');
    for (const type of MEDIA_POLL_JOB_TYPES) handlerMap[type] = handlerFor(type, 'poll');
    Object.freeze(handlerMap);

    function register(target, options = {}) {
        if (!target || typeof target.register !== 'function') throw new TypeError('Media job service register target must provide register()');
        const receiver = options.receiver ?? service;
        for (const [type, handler] of Object.entries(handlerMap)) target.register(type, handler, receiver);
        return target;
    }

    const service = {
        version: MEDIA_JOB_SERVICE_VERSION,
        handlers: handlerMap,
        handlerMap,
        submit,
        poll,
        settle,
        register,
        list() { return Object.keys(handlerMap); },
        operationFor,
        has(type) { return Object.hasOwn(handlerMap, type); },
        get(type) { return handlerMap[type] ?? null; },
        registrations() {
            return Object.freeze(Object.entries(handlerMap).map(([type, handler]) => Object.freeze({type, operation: operationFor(type), handler})));
        }
    };
    return Object.freeze(service);
}

export const createMediaJobApplicationService = createMediaJobService;
export default createMediaJobService;
