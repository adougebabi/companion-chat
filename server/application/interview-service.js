const MAX_DESCRIPTION_LENGTH = 6_000;
const MAX_ID_LENGTH = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
}

function requiredText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') throw statusError(`${field}不能为空`, 400);
    const normalized = value.trim();
    if (normalized.length > maxLength) throw statusError(`${field}不能超过 ${maxLength} 个字符`, 400);
    return normalized;
}

function record(value, field) {
    if (!isRecord(value)) throw statusError(`${field}必须是 JSON 对象`, 400);
    return value;
}

function sourceFor(options, names) {
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    for (const source of [options, repositories]) {
        for (const name of names) if (source[name] !== undefined) return source[name];
    }
    return undefined;
}

function methodFor(port, names, field, {optional = false} = {}) {
    if (typeof port === 'function') return port;
    if (isRecord(port)) {
        for (const name of names) if (typeof port[name] === 'function') return port[name].bind(port);
    }
    if (optional) return null;
    throw statusError(`访谈服务缺少 ${field}.${names[0]}()`, 501);
}

function mapMaybe(value, mapper) {
    return value && typeof value.then === 'function' ? value.then(mapper) : mapper(value);
}

function interviewId(command) {
    const input = record(command, '访谈请求');
    return requiredText(input.interviewId ?? input.interview_id ?? input.id, '访谈 ID');
}

function normalizeAnswers(value) {
    if (value === undefined) return {};
    if (!isRecord(value)) throw statusError('访谈答案必须是 JSON 对象', 400);
    return {...value};
}

function analysisResult(value) {
    if (!isRecord(value)) throw statusError('人格分析未返回有效结果', 502);
    const answers = normalizeAnswers(value.answers ?? value.preview?.answers);
    const blueprint = isRecord(value.blueprint)
        ? value.blueprint
        : isRecord(value.preview?.blueprint) ? value.preview.blueprint : undefined;
    const inferredFields = value.inferredFields === undefined ? [] : value.inferredFields;
    if (!Array.isArray(inferredFields) || inferredFields.some(field => typeof field !== 'string')) {
        throw statusError('人格分析字段来源无效', 502);
    }
    const fieldSources = value.fieldSources === undefined ? {} : value.fieldSources;
    if (!isRecord(fieldSources)) throw statusError('人格分析字段来源无效', 502);
    return {answers, blueprint, inferredFields: [...new Set(inferredFields)], fieldSources};
}

/**
 * Application boundary for persona interviews. Interview persistence,
 * analysis/model calls, and activation/life-model generation are all ports;
 * this module never invents a preview or a persona when a port is absent.
 */
export function createInterviewService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Interview service options must be an object');
    const repository = sourceFor(options, ['repository', 'interview', 'interviews', 'interviewRepository', 'interviewPort']);
    const previewPort = sourceFor(options, ['preview', 'interviewPreview', 'previewPort']);
    const analyzer = sourceFor(options, ['analyzer', 'analysis', 'analysisPort', 'interviewAnalyzer']);
    const activation = sourceFor(options, ['activation', 'activationPort', 'personaLifecycle', 'personaLifecycleService']);

    function configured(names, portNames, field) {
        for (const port of portNames) {
            const operation = methodFor(port, names, field, {optional: true});
            if (operation) return operation;
        }
        throw statusError(`访谈服务缺少 ${field} port`, 501);
    }

    function preview(command = {}) {
        const input = record(command, '访谈预览请求');
        const answers = normalizeAnswers(input.answers ?? input);
        const operation = configured(['preview', 'previewInterview', 'buildPreview'], [previewPort, repository], 'preview');
        return operation({answers, ...input});
    }

    function analyze(command = {}) {
        const input = record(command, '人格分析请求');
        const description = requiredText(input.description, '人格描述', MAX_DESCRIPTION_LENGTH);
        const operation = configured(['analyze', 'analyzeDescription', 'extract'], [analyzer], 'analyze');
        const persist = methodFor(repository, ['createReadyInterview'], 'createReadyInterview', {optional: true});
        const create = methodFor(repository, ['createInterview', 'create', 'start'], 'create', {optional: true});
        const answer = methodFor(repository, ['answerInterview', 'answer', 'saveAnswer', 'update'], 'answer', {optional: true});
        if (!persist && (!create || !answer)) throw statusError('访谈服务缺少 ready session persistence port', 501);
        return mapMaybe(operation({description, signal: input.signal, personaId: input.personaId, trace: input.trace}), value => {
            const normalized = analysisResult(value);
            const storedAnswers = {
                ...normalized.answers,
                ...(normalized.blueprint ? {blueprint: normalized.blueprint} : {}),
                fieldSources: normalized.fieldSources,
                source: 'llm'
            };
            const inputForRepository = {
                id: input.interviewId ?? input.id,
                answers: storedAnswers,
                source: 'llm',
                inferredFields: normalized.inferredFields
            };
            const ready = persist
                ? persist(inputForRepository)
                : mapMaybe(create(inputForRepository), draft => answer({
                    interviewId: draft?.id ?? draft?.interviewId,
                    id: draft?.id ?? draft?.interviewId,
                    answers: storedAnswers,
                    inferredFields: normalized.inferredFields
                }));
            return mapMaybe(ready, session => {
                if (!session) throw statusError('访谈 session 创建失败', 502);
                const interviewIdValue = session.interviewId ?? session.id;
                if (typeof interviewIdValue !== 'string' || !interviewIdValue) throw statusError('访谈 session 缺少 ID', 502);
                const preview = value.preview ?? {
                    ...normalized.answers,
                    ...(normalized.blueprint ? {blueprint: normalized.blueprint} : {}),
                    fieldSources: normalized.fieldSources,
                    inferredFields: normalized.inferredFields
                };
                return {
                    ...session,
                    id: session.id ?? interviewIdValue,
                    interviewId: interviewIdValue,
                    status: 'ready',
                    source: 'llm',
                    answers: normalized.answers,
                    inferredFields: normalized.inferredFields,
                    fieldSources: normalized.fieldSources,
                    preview,
                    ...(normalized.blueprint ? {blueprint: normalized.blueprint} : {})
                };
            });
        });
    }

    function create(command = {}) {
        const input = record(command, '访谈创建请求');
        const answers = normalizeAnswers(input.answers ?? input);
        const operation = configured(['createInterview', 'create', 'start'], [repository], 'create');
        return operation({...input, answers});
    }

    function get(command = {}) {
        const id = interviewId(command);
        const operation = configured(['getInterview', 'findById', 'find', 'get', 'read'], [repository], 'get');
        return mapMaybe(operation({interviewId: id, id}), value => {
            if (!value) throw statusError('访谈不存在', 404);
            return value;
        });
    }

    function answer(command = {}) {
        const input = record(command, '访谈回答请求');
        const id = interviewId(input);
        const operation = configured(['answerInterview', 'answer', 'saveAnswer', 'update'], [repository], 'answer');
        return operation({...input, interviewId: id, id});
    }

    function activate(command = {}) {
        const input = record(command, '访谈激活请求');
        const id = interviewId(input);
        const operation = configured(['activateInterview', 'activate'], [activation, repository], 'activate');
        return mapMaybe(operation({...input, interviewId: id, id}), value => {
            if (value === undefined || value === null) throw new Error('访谈激活未返回人格');
            return value;
        });
    }

    return Object.freeze({
        preview,
        analyze,
        create,
        get,
        answer,
        activate,
        previewInterview: preview,
        analyzeInterview: analyze,
        createInterview: create,
        getInterview: get,
        answerInterview: answer,
        activateInterview: activate
    });
}

export const createInterviewApplicationService = createInterviewService;
export default createInterviewService;
