const ANALYZER_PROMPT_VERSION = 'persona-description-v2';
const MAX_NAME_LENGTH = 160;
const MAX_ROLE_LENGTH = 160;
const MAX_FOUNDATION_LENGTH = 6_000;
const MAX_VISUAL_BASELINE_LENGTH = 240;
const MAX_LANGUAGE_STYLE_LENGTH = 240;
const MAX_RELATIONSHIP_NOTE_LENGTH = 400;
const MAX_ROUTINE_ITEM_LENGTH = 160;
const MAX_INTEREST_LENGTH = 80;
const MAX_SUPPORTING_CAST_ITEM_LENGTH = 160;
const MAX_LIST_ITEMS = 12;

const ANSWER_LIMITS = Object.freeze({
    name: MAX_NAME_LENGTH,
    role: MAX_ROLE_LENGTH,
    foundation: MAX_FOUNDATION_LENGTH,
    visualBaseline: MAX_VISUAL_BASELINE_LENGTH,
    languageStyle: MAX_LANGUAGE_STYLE_LENGTH,
    relationshipNote: MAX_RELATIONSHIP_NOTE_LENGTH,
    relationshipKind: 80,
    relationship: MAX_RELATIONSHIP_NOTE_LENGTH,
    interactionBoundaries: MAX_RELATIONSHIP_NOTE_LENGTH
});

const ANSWER_KEYS = new Set([
    'name', 'role', 'foundation', 'interests', 'visualBaseline', 'supportingCast',
    'routine', 'languageStyle', 'relationshipNote', 'relationshipKind', 'relationship', 'interactionBoundaries'
]);

const BLUEPRINT_KEYS = new Set([
    'name', 'role', 'foundation', 'interests', 'visualBaseline', 'supportingCast',
    'routine', 'languageStyle', 'relationshipNote', 'relationshipKind', 'relationship', 'interactionBoundaries', 'identity'
]);

const ANALYZER_PROMPT = [
    'You extract a bounded companion persona blueprint from the user description.',
    'Return only one JSON object with exactly these top-level keys: answers, inferredFields, blueprint.',
    'answers must include non-empty name, role, and foundation strings.',
    'Use only allowed fields: name, role, foundation, interests, visualBaseline, supportingCast, routine, languageStyle, relationshipNote, relationshipKind, relationship, interactionBoundaries.',
    'inferredFields is an array of allowed field names that the model inferred rather than directly stated.',
    'interests, routine, and supportingCast must be arrays of concise strings; all scalar persona fields must be strings.',
    'blueprint is an object containing only the same allowed persona fields and may be {}.',
    'Do not include the original description, explanations, markdown, or any other keys.',
    `Prompt contract version: ${ANALYZER_PROMPT_VERSION}.`
].join('\n');

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedText(value, field, maxLength, {required = true} = {}) {
    if (typeof value !== 'string') throw analysisError(`${field}必须是字符串`);
    const normalized = value.trim();
    if (required && !normalized) throw analysisError(`${field}不能为空`);
    if (normalized.length > maxLength) throw analysisError(`${field}不能超过 ${maxLength} 个字符`);
    return normalized;
}

function analysisError(message, cause) {
    const error = Object.assign(new Error(`人格分析失败：${String(message).slice(0, 200)}`), {
        status: 502,
        code: 'INTERVIEW_ANALYSIS_INVALID'
    });
    if (cause) error.cause = cause;
    return error;
}

function unknownKeys(value, allowed, field) {
    if (!isRecord(value)) throw analysisError(`${field}必须是 JSON 对象`);
    const unknown = Object.keys(value).filter(key => !allowed.has(key));
    if (unknown.length) throw analysisError(`${field}包含不支持的字段`);
}

function stringList(value, field, maxLength) {
    if (!Array.isArray(value)) throw analysisError(`${field}必须是字符串数组`);
    if (value.length > MAX_LIST_ITEMS) throw analysisError(`${field}项目过多`);
    return value.map(item => boundedText(item, `${field}项目`, maxLength));
}

function normalizeAnswers(value, field = 'answers') {
    unknownKeys(value, ANSWER_KEYS, field);
    const answers = {};
    for (const key of Object.keys(value)) {
        const raw = value[key];
        if (key === 'interests') answers.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH);
        else if (key === 'routine') answers.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH);
        else if (key === 'supportingCast') answers.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH);
        else answers[key] = boundedText(raw, `${field}.${key}`, ANSWER_LIMITS[key]);
    }
    for (const key of ['name', 'role', 'foundation']) {
        if (!answers[key]) throw analysisError(`${field}.${key}缺失`);
    }
    return answers;
}

function normalizeBlueprint(value) {
    if (value === undefined) return {};
    unknownKeys(value, BLUEPRINT_KEYS, 'blueprint');
    const source = {...value};
    if (isRecord(source.identity)) {
        unknownKeys(source.identity, new Set(['name', 'role']), 'blueprint.identity');
        source.name = source.name ?? source.identity.name;
        source.role = source.role ?? source.identity.role;
        delete source.identity;
    }
    return normalizeOptionalFields(source, 'blueprint');
}

function normalizeOptionalFields(value, field) {
    const normalized = {};
    for (const key of Object.keys(value)) {
        const raw = value[key];
        if (key === 'interests') normalized.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH);
        else if (key === 'routine') normalized.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH);
        else if (key === 'supportingCast') normalized.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH);
        else normalized[key] = boundedText(raw, `${field}.${key}`, ANSWER_LIMITS[key]);
    }
    return normalized;
}

function stripJsonFence(value) {
    const text = String(value ?? '').trim();
    const match = text.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i);
    return (match ? match[1] : text).trim();
}

function parseModelPayload(value) {
    const content = typeof value === 'string'
        ? value
        : value?.content ?? value?.text
            ?? value?.choices?.[0]?.message?.content
            ?? (isRecord(value) && (value.answers || value.inferredFields || value.blueprint) ? JSON.stringify(value) : undefined);
    if (typeof content !== 'string' || !content.trim()) throw analysisError('模型返回空内容');
    let parsed;
    try { parsed = JSON.parse(stripJsonFence(content)); }
    catch (error) { throw analysisError('模型返回的 JSON 无效', error); }
    if (!isRecord(parsed)) throw analysisError('模型返回必须是 JSON 对象');
    unknownKeys(parsed, new Set(['answers', 'inferredFields', 'blueprint']), '模型结果');
    if (!Object.hasOwn(parsed, 'answers')) throw analysisError('模型结果缺少 answers');
    const blueprint = normalizeBlueprint(parsed.blueprint);
    const answerSource = {...(isRecord(parsed.answers) ? parsed.answers : {})};
    for (const [key, valueForKey] of Object.entries(blueprint)) {
        if (answerSource[key] === undefined) answerSource[key] = valueForKey;
    }
    const answers = normalizeAnswers(answerSource);
    if (!Array.isArray(parsed.inferredFields) || parsed.inferredFields.some(field => typeof field !== 'string' || !ANSWER_KEYS.has(field))) {
        throw analysisError('inferredFields 必须是允许字段名数组');
    }
    const inferredFields = [...new Set(parsed.inferredFields)];
    const fieldSources = Object.fromEntries(Object.keys(answers).map(key => [key, inferredFields.includes(key) ? 'inferred' : 'llm']))
    return {answers, inferredFields, fieldSources};
}

function blueprintFor(answers, fieldSources) {
    return {
        schemaVersion: 2,
        timezone: 'Asia/Shanghai',
        foundation: answers.foundation,
        identity: {name: answers.name, role: answers.role},
        ...(answers.interests ? {interests: answers.interests} : {interests: []}),
        ...(answers.routine ? {routine: answers.routine} : {routine: []}),
        ...(answers.visualBaseline ? {visualBaseline: answers.visualBaseline} : {}),
        ...(answers.languageStyle ? {languageStyle: answers.languageStyle} : {}),
        ...(answers.supportingCast ? {supportingCast: answers.supportingCast} : {supportingCast: []}),
        ...(answers.relationshipNote || answers.relationshipKind || answers.relationship ? {
            relationship: {
                ...(answers.relationshipNote || answers.relationship ? {note: answers.relationshipNote || answers.relationship} : {}),
                ...(answers.relationshipKind ? {kind: answers.relationshipKind} : {})
            }
        } : {}),
        ...(answers.interactionBoundaries ? {interactionBoundaries: answers.interactionBoundaries} : {}),
        provenance: fieldSources,
        world: {defaultSceneRef: {locationId: 'home', roomId: 'private_room'}},
        generation: {source: 'llm', usedFallback: false, validationWarnings: []}
    };
}

function completionMethod(port) {
    if (typeof port === 'function') return port;
    for (const name of ['complete', 'completeJson', 'json']) {
        if (typeof port?.[name] === 'function') return port[name].bind(port);
    }
    return null;
}

/**
 * Application-owned persona analyzer. The completion port only transports
 * assistant JSON; this module owns the allowlist and blueprint contract.
 */
export function createInterviewAnalyzer({jsonCompletion, completion, model, promptVersion = ANALYZER_PROMPT_VERSION} = {}) {
    const complete = completionMethod(jsonCompletion ?? completion);
    if (!complete) throw new TypeError('Interview analyzer requires a JSON completion port');
    async function analyze({description, signal, personaId, trace} = {}) {
        const text = boundedText(description, '人格描述', 6_000);
        let result;
        try {
            result = await complete({
                model,
                stream: false,
                temperature: 0,
                messages: [
                    {role: 'system', content: `${ANALYZER_PROMPT}\nVersion: ${promptVersion}`},
                    {role: 'user', content: text}
                ],
                signal,
                trace: false
            });
        } catch (error) {
            if (error?.status === 502) throw error;
            throw analysisError(error?.message || '模型服务请求失败', error);
        }
        const parsed = parseModelPayload(result);
        const blueprint = blueprintFor(parsed.answers, parsed.fieldSources);
        const preview = {
            ...parsed.answers,
            blueprint,
            fieldSources: parsed.fieldSources,
            inferredFields: parsed.inferredFields
        };
        return Object.freeze({
            source: 'llm',
            status: 'ready',
            answers: parsed.answers,
            inferredFields: parsed.inferredFields,
            fieldSources: parsed.fieldSources,
            blueprint,
            preview
        });
    }
    return Object.freeze({
        promptVersion,
        analyze,
        extract: analyze
    });
}

export const INTERVIEW_ANALYZER_PROMPT_VERSION = ANALYZER_PROMPT_VERSION;
export const normalizeInterviewAnalysis = parseModelPayload;
export default createInterviewAnalyzer;
