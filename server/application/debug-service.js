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
            .replace(/((?:api[-_]?key|token|secret|password)\s*[:=]\s*)[^\s,;]+/gi, '$1[redacted]');
        return redacted.length > 2_000 ? `${redacted.slice(0, 1_997)}...` : redacted;
    }
    if (Array.isArray(value)) return value.slice(0, 50).map(item => redact(item, depth + 1));
    if (!isRecord(value)) return value;
    return Object.fromEntries(Object.entries(value).slice(0, 80).map(([key, child]) => /key|token|secret|password|authorization|credential|cookie/i.test(key) ? [key, '[redacted]'] : [key, redact(child, depth + 1)]));
}

export function createDebugService({repositories = {}, promptRuns, h3Preflight, contextReader, lifeWorldReader, lifeEventFlow, mediaJobService, clock = () => new Date().toISOString()} = {}) {
    const runs = promptRuns ?? repositories.promptRun;
    const lifeEvents = repositories.lifeEvent;
    const personas = repositories.persona;
    const jobs = repositories.job;
    const readContext = contextReader ?? lifeWorldReader;

    function listPromptRuns(command = {}) {
        if (!runs?.list) throw Object.assign(new Error('prompt runs 未配置'), {status: 501});
        return runs.list({personaId: command.personaId, limit: command.limit});
    }

    function h3PreflightRead(command = {}) {
        if (typeof h3Preflight !== 'function') throw Object.assign(new Error('H3 preflight 未配置'), {status: 501});
        return h3Preflight(command.settings ?? command);
    }

    function getContext(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
        if (!readContext?.readResolverInput && !readContext?.read) throw Object.assign(new Error('debug context 未配置'), {status: 501});
        const input = {personaId, at: command.at, command: {...command, personaId}};
        const result = readContext.readResolverInput
            ? readContext.readResolverInput(input)
            : readContext.read(input);
        return redact(sync(result, 'debug context'));
    }

    function getLifecycle(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
        const events = lifeEvents?.list ? lifeEvents.list({personaId, limit: 20}) : lifeEvents?.listActive?.({personaId, at: clock(), limit: 20}) ?? [];
        const jobsForPersona = jobs?.listForPersona ? jobs.listForPersona({personaId, limit: 50}) : [];
        return {personaId, events: redact(sync(events, 'lifecycle events')), jobs: redact(sync(jobsForPersona, 'lifecycle jobs'))};
    }

    function simulatePersona(command = {}) {
        const personaId = text(command.personaId, '人格 ID');
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
        h3Preflight: h3PreflightRead,
        getDebugContext: getContext,
        getLifecycle,
        simulatePersona,
        debugMedia,
        debug: {listPromptRuns, h3Preflight: h3PreflightRead, getDebugContext: getContext, getLifecycle, simulatePersona, debugMedia}
    });
}

export default createDebugService;
