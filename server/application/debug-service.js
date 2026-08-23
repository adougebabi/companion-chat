import {debugStateFor} from './debug-context.js';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function text(value, field, max = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw Object.assign(new TypeError(`${field}不能为空`), {status: 400});
    const result = value.trim();
    if (result.length > max) throw Object.assign(new RangeError(`${field}过长`), {status: 400});
    return result;
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`${field}必须同步返回`);
    return value;
}

function parse(value, fallback = {}) {
    if (isRecord(value) || Array.isArray(value)) return value;
    try { return value ? JSON.parse(value) : fallback; } catch { return fallback; }
}

function redact(value, depth = 0) {
    if (depth > 5) return '[bounded]';
    if (typeof value === 'string') {
        const redacted = value
            .replace(/Bearer\s+[^\s,;]+/gi, 'Bearer [redacted]')
            .replace(/((?:api[-_]?key|authorization|token|secret|password|credential|cookie)\s*[:=]\s*)[^\s,;]+/gi, '$1[redacted]');
        return redacted.length > 2_000 ? `${redacted.slice(0, 1_997)}...` : redacted;
    }
    if (Array.isArray(value)) return value.slice(0, 50).map(item => redact(item, depth + 1));
    if (!isRecord(value)) return value;
    return Object.fromEntries(Object.entries(value).slice(0, 80).map(([key, child]) => /key|token|secret|password|authorization|credential|cookie/i.test(key) ? [key, '[redacted]'] : [key, redact(child, depth + 1)]));
}

function summary(value, limit = 2_000) {
    const safe = redact(value);
    let serialized;
    try { serialized = typeof safe === 'string' ? safe : JSON.stringify(safe); } catch { serialized = '[unavailable]'; }
    const textValue = String(serialized ?? '');
    return textValue.length <= limit ? textValue : `${textValue.slice(0, Math.max(0, limit - 3))}...`;
}

function boundedRedacted(value, limit = 2_000) {
    const safe = redact(value);
    let serialized;
    try { serialized = JSON.stringify(safe); } catch { return '[unavailable]'; }
    if (serialized.length <= limit) return safe;
    return summary(safe, limit);
}

function jobDto(row) {
    const payload = parse(row?.payload_json ?? row?.payload, {});
    const result = parse(row?.result_json ?? row?.result, {});
    return {
        id: row?.id ?? null,
        jobType: row?.job_type ?? row?.jobType ?? 'unknown',
        status: row?.status ?? 'unknown',
        priority: row?.priority ?? null,
        runAfter: row?.run_after ?? row?.runAfter ?? null,
        leaseExpiresAt: row?.lease_expires_at ?? row?.leaseExpiresAt ?? null,
        attemptCount: Number(row?.attempt_count ?? row?.attemptCount ?? 0) || 0,
        maxAttempts: Number(row?.max_attempts ?? row?.maxAttempts ?? 0) || 0,
        personaId: row?.persona_id ?? row?.personaId ?? null,
        activityId: row?.activity_id ?? row?.activityId ?? null,
        messageId: row?.message_id ?? row?.messageId ?? null,
        traceId: row?.trace_id ?? row?.traceId ?? null,
        createdAt: row?.created_at ?? row?.createdAt ?? null,
        updatedAt: row?.updated_at ?? row?.updatedAt ?? null,
        completedAt: row?.completed_at ?? row?.completedAt ?? null,
        error: summary(row?.error ?? '', 500),
        payloadSummary: summary(payload, 900),
        resultSummary: summary(result, 900)
    };
}

function requirePersona(personas, personaId) {
    if (!personas) return null;
    const lookup = typeof personas.findActive === 'function'
        ? () => personas.findActive(personaId)
        : typeof personas.findById === 'function'
            ? () => personas.findById({personaId, id: personaId})
            : typeof personas.get === 'function'
                ? () => personas.get({personaId, id: personaId})
                : null;
    if (!lookup) throw Object.assign(new Error('persona repository 未配置查找方法'), {status: 501});
    const persona = sync(lookup(), 'debug persona');
    if (!persona) throw Object.assign(new Error('人格不存在'), {status: 404});
    return persona;
}

export function createDebugService({repositories = {}, promptRuns, settings, h3Preflight, contextReader, lifeWorldReader, lifeEventFlow, mediaJobService, clock = () => new Date().toISOString()} = {}) {
    const runs = promptRuns ?? repositories.promptRun;
    const lifeEvents = repositories.lifeEvent;
    const personas = repositories.persona;
    const jobs = repositories.job;
    const readContext = contextReader ?? lifeWorldReader;
    const settingsPort = settings ?? repositories.settings;

    function listPromptRuns(command = {}) {
        if (!runs?.list) throw Object.assign(new Error('prompt runs 未配置'), {status: 501});
        const limit = command.limit === undefined ? 50 : Math.min(100, Math.max(1, Math.trunc(Number(command.limit) || 50)));
        return runs.list({personaId: command.personaId ?? command.persona_id ?? null, limit});
    }

    function h3PreflightRead(command = {}) {
        if (typeof h3Preflight !== 'function') throw Object.assign(new Error('H3 preflight 未配置'), {status: 501});
        return h3Preflight(command.settings ?? command);
    }

    function getContext(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
        requirePersona(personas, personaId);
        if (typeof readContext !== 'function' && !readContext?.readResolverInput && !readContext?.read) throw Object.assign(new Error('debug context 未配置'), {status: 501});
        const input = {personaId, at: command.at, command: {...command, personaId}};
        const result = typeof readContext === 'function'
            ? readContext(input)
            : readContext.readResolverInput
                ? readContext.readResolverInput(input)
                : readContext.read(input);
        const context = sync(result, 'debug context');
        const settingsValue = sync(settingsPort?.read?.() ?? (typeof settingsPort === 'function' ? settingsPort() : {}), 'debug settings') ?? {};
        const recentRows = sync(repositories.conversation?.listMessages?.({personaId, limit: 10}) ?? [], 'debug messages');
        const messages = Array.isArray(recentRows) ? recentRows : recentRows?.items ?? [];
        const recentRequests = messages.map(row => ({
            id: row.id,
            createdAt: row.created_at ?? row.createdAt,
            status: row.role === 'assistant' ? 'response' : 'request',
            promptSummary: row.role === 'user' ? summary(row.text || '', 240) : '',
            responseSummary: row.role === 'assistant' ? summary(row.text || '', 240) : '',
            error: ''
        }));
        const mediaRows = sync(repositories.job?.listForPersona?.({
            personaId,
            jobTypes: ['activity_image', 'activity_video', 'chat_image', 'chat_video', 'activity_media_poll', 'chat_media_poll'],
            limit: 10
        }) ?? [], 'debug media jobs');
        const mediaJobs = (Array.isArray(mediaRows) ? mediaRows : mediaRows?.items ?? [])
            .filter(row => row.persona_id === undefined || row.persona_id === personaId || row.personaId === personaId)
            .slice(0, 10).map(row => {
            const payload = parse(row.payload_json ?? row.payload, {});
            const resultValue = parse(row.result_json ?? row.result, {});
            return {
                id: row.id,
                kind: payload.kind ?? (/video/i.test(row.job_type ?? '') ? 'video' : 'image'),
                status: row.status,
                createdAt: row.created_at ?? row.createdAt,
                provider: payload.provider ?? resultValue.provider ?? 'comfyui',
                trigger: payload.trigger ?? 'unknown',
                promptSummary: summary(resultValue.finalPrompt || '', 480),
                progress: redact(resultValue.progress ?? {}),
                error: summary(row.error || '', 500)
            };
            });
        const affectSnapshot = repositories.affect?.readSnapshot
            ? sync(repositories.affect.readSnapshot({personaId, at: command.at ?? clock()}), 'debug affect snapshot')
            : null;
        const affectEvents = repositories.affect?.listEvents
            ? sync(repositories.affect.listEvents({personaId, limit: 10}), 'debug affect events')
            : [];
        const appraisals = repositories.appraisal?.list
            ? sync(repositories.appraisal.list({personaId, limit: 10}), 'debug appraisals')
            : [];
        const memoryConsolidations = repositories.memoryConsolidation?.list
            ? sync(repositories.memoryConsolidation.list({personaId, limit: 10}), 'debug memory consolidations')
            : [];
        const selfModelClaims = repositories.selfModel?.list
            ? sync(repositories.selfModel.list({personaId, limit: 10}), 'debug self-model claims')
            : [];
        const agencyIntentions = repositories.agencyIntention?.list
            ? sync(repositories.agencyIntention.list({personaId, limit: 10}), 'debug agency intentions')
            : [];
        const layers = {
            identity: summary(context?.layers?.immutableIdentity ?? context?.layers?.identity ?? ''),
            immutableIdentity: summary(context?.layers?.immutableIdentity ?? ''),
            lifeState: summary(context?.layers?.lifeState ?? context?.state ?? ''),
            relationship: summary(context?.layers?.relationship ?? ''),
            affect: boundedRedacted({snapshot: affectSnapshot, recentEvents: affectEvents}),
            appraisal: boundedRedacted(appraisals),
            memoryConsolidation: boundedRedacted(memoryConsolidations),
            selfModel: boundedRedacted(selfModelClaims),
            agency: boundedRedacted(agencyIntentions),
            systemCapability: summary(context?.layers?.systemCapability ?? ''),
            provider: {model: summary(settingsValue.model || '自动选择', 240), lmStudioConfigured: Boolean(settingsValue.lmStudioUrl), comfyConfigured: Boolean(settingsValue.comfyUrl)}
        };
        return {
            layers,
            state: debugStateFor(context),
            recentRequests,
            mediaJobs,
            emergence: boundedRedacted({appraisals, memoryConsolidations, selfModelClaims, agencyIntentions}, 8_000)
        };
    }

    function getLifecycle(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
        requirePersona(personas, personaId);
        const events = lifeEvents?.list ? lifeEvents.list({personaId, limit: 20}) : lifeEvents?.listActive?.({personaId, at: clock(), limit: 20}) ?? [];
        const jobsForPersona = jobs?.listForPersona ? jobs.listForPersona({personaId, limit: 50}) : [];
        const scopedJobs = sync(jobsForPersona, 'lifecycle jobs');
        const rows = Array.isArray(scopedJobs) ? scopedJobs.filter(row => row.persona_id === undefined || row.persona_id === personaId || row.personaId === personaId) : scopedJobs;
        const affectEvents = repositories.affect?.listEvents
            ? repositories.affect.listEvents({personaId, limit: 20})
            : [];
        const appraisals = repositories.appraisal?.list ? repositories.appraisal.list({personaId, limit: 20}) : [];
        const memoryConsolidations = repositories.memoryConsolidation?.list ? repositories.memoryConsolidation.list({personaId, limit: 20}) : [];
        const selfModelClaims = repositories.selfModel?.list ? repositories.selfModel.list({personaId, limit: 20}) : [];
        const agencyIntentions = repositories.agencyIntention?.list ? repositories.agencyIntention.list({personaId, limit: 20}) : [];
        return {
            personaId,
            events: redact(sync(events, 'lifecycle events')),
            affectEvents: redact(affectEvents),
            emergence: boundedRedacted({appraisals, memoryConsolidations, selfModelClaims, agencyIntentions}, 8_000),
            jobs: rows.map(jobDto)
        };
    }

    function simulatePersona(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
        requirePersona(personas, personaId);
        if (lifeEventFlow?.record) return lifeEventFlow.record({
            ...command,
            personaId,
            source: 'debug',
            simulated: true,
            publish: command.publish === true
        });
        if (!lifeEvents?.createEvent && !lifeEvents?.insertEvent) throw Object.assign(new Error('life event repository 未配置'), {status: 501});
        const create = lifeEvents.createEvent ?? lifeEvents.insertEvent;
        const event = create.call(lifeEvents, {
            id: command.id,
            personaId,
            type: command.type ?? 'debug_simulation',
            occurredAt: command.occurredAt ?? clock(),
            resolvesAt: command.resolvesAt ?? null,
            causationId: command.causationId ?? null,
            payload: redact(command.payload ?? {situation: command.situation ?? '开发检查器模拟状态', mood: command.mood ?? '平静', scene: command.scene ?? '日常场景'})
        });
        return redact(event);
    }

    function debugMedia(command = {}) {
        const job = command.job;
        if (job && mediaJobService?.submit) return mediaJobService.submit(job, command.context ?? {});
        if (!jobs?.enqueue) throw Object.assign(new Error('debug media job service 未配置'), {status: 501});
        const personaId = text(command.personaId, '人格 ID');
        requirePersona(personas, personaId);
        const kind = command.kind === 'video' ? 'video' : 'image';
        const concept = isRecord(command.personaMediaConcept) ? command.personaMediaConcept : {
            schemaVersion: 1,
            mediaKind: kind,
            scene: String(command.request || '开发检查器媒体任务').slice(0, 800),
            action: String(command.request || '开发检查器媒体任务').slice(0, 800),
            mood: '', narrative: '', humanSubjects: [], nonHumanObjects: [],
            capture: {mode: 'other', operator: '', deviceVisibility: 'unspecified', framingIntent: ''},
            compositionIntent: ''
        };
        return jobs.enqueue({
            jobType: kind === 'video' ? 'chat_video' : 'chat_image',
            personaId,
            priority: 4,
            maxAttempts: 3,
            payload: {kind, provider: command.provider ?? (kind === 'video' ? 'comfyui' : 'comfyui'), request: command.request ?? '', personaMediaConcept: concept},
            runAfter: command.runAfter ?? clock()
        });
    }

    return Object.freeze({
        listPromptRuns,
        promptRuns: listPromptRuns,
        promptRunsFor: listPromptRuns,
        h3Preflight: h3PreflightRead,
        getDebugContext: getContext,
        getLifecycle,
        simulatePersona,
        debugMedia,
        debug: {listPromptRuns, promptRuns: listPromptRuns, promptRunsFor: listPromptRuns, h3Preflight: h3PreflightRead, getDebugContext: getContext, getLifecycle, simulatePersona, debugMedia}
    });
}

export default createDebugService;
