const ANALYZER_PROMPT_VERSION = 'persona-description-v2';
const MAX_NAME_LENGTH = 160;
const MAX_ROLE_LENGTH = 160;
const MAX_FOUNDATION_LENGTH = 6_000;
const MAX_VISUAL_BASELINE_LENGTH = 240;
const MAX_LANGUAGE_STYLE_LENGTH = 240;
const MAX_RELATIONSHIP_NOTE_LENGTH = 400;
const MAX_PROFILE_TEXT_LENGTH = 160;
const MAX_BACKGROUND_LENGTH = 1_200;
const MAX_MAJOR_EVENT_LENGTH = 320;
const MAX_COORDINATE_KEY_LENGTH = 64;
const MAX_COORDINATE_VALUE_LENGTH = 80;
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
    interactionBoundaries: MAX_RELATIONSHIP_NOTE_LENGTH,
    gender: MAX_PROFILE_TEXT_LENGTH,
    occupation: MAX_PROFILE_TEXT_LENGTH,
    growthExperience: MAX_BACKGROUND_LENGTH,
    tone: MAX_PROFILE_TEXT_LENGTH
});

const ANSWER_KEYS = new Set([
    'name', 'role', 'foundation', 'interests', 'visualBaseline', 'supportingCast',
    'routine', 'languageStyle', 'relationshipNote', 'relationshipKind', 'relationship', 'interactionBoundaries',
    'age', 'gender', 'occupation', 'growthExperience', 'majorEvents', 'personalityCoordinates',
    'strengths', 'weaknesses', 'quirks', 'obsessions', 'toneAndVocabulary', 'catchphrases',
    'signatureBehaviors', 'coreBeliefs', 'boundariesAndTaboos'
]);

const BLUEPRINT_KEYS = new Set([
    'name', 'role', 'foundation', 'interests', 'visualBaseline', 'supportingCast',
    'routine', 'languageStyle', 'relationshipNote', 'relationshipKind', 'relationship', 'interactionBoundaries', 'identity',
    'age', 'gender', 'occupation', 'growthExperience', 'majorEvents', 'personalityCoordinates',
    'strengths', 'weaknesses', 'quirks', 'obsessions', 'toneAndVocabulary', 'catchphrases',
    'signatureBehaviors', 'coreBeliefs', 'boundariesAndTaboos'
]);

const NEW_STRING_LIST_SCHEMAS = Object.freeze({
    majorEvents: Object.freeze({maxLength: MAX_MAJOR_EVENT_LENGTH, label: '重大事件'}),
    strengths: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '优点'}),
    weaknesses: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '缺点'}),
    quirks: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '怪癖'}),
    obsessions: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '执念'}),
    catchphrases: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '口癖'}),
    signatureBehaviors: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '标志性行为'}),
    coreBeliefs: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '核心信仰'}),
    boundariesAndTaboos: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '底线与禁忌'})
});

const LIST_ITEM_SCHEMAS = Object.freeze({
    interests: Object.freeze({
        textKeys: Object.freeze(['name', 'label', 'interest', 'topic', 'text', 'value']),
        fields: Object.freeze({
            name: 'text', label: 'text', interest: 'text', topic: 'text', text: 'text', value: 'text', category: 'text'
        })
    }),
    routine: Object.freeze({
        textKeys: Object.freeze(['label', 'activity', 'description', 'text', 'value']),
        fields: Object.freeze({
            label: 'text', activity: 'text', description: 'text', text: 'text', value: 'text',
            from: 'time', to: 'time', start: 'time', end: 'time', time: 'text', scene: 'text'
        })
    }),
    supportingCast: Object.freeze({
        textKeys: Object.freeze(['name', 'label', 'person', 'character', 'text', 'value']),
        fields: Object.freeze({
            name: 'text', label: 'text', person: 'text', character: 'text', text: 'text', value: 'text',
            role: 'text', relationship: 'text', description: 'text'
        })
    })
});

const ANALYZER_PROMPT = [
    'You extract a bounded companion persona blueprint from the user description.',
    'Return only one JSON object with exactly these top-level keys: answers, inferredFields, blueprint.',
    'answers must include non-empty name, role, and foundation strings.',
    'Use only allowed fields: name, role, foundation, age, gender, occupation, growthExperience, majorEvents, personalityCoordinates, strengths, weaknesses, quirks, obsessions, toneAndVocabulary, catchphrases, signatureBehaviors, coreBeliefs, boundariesAndTaboos, interests, visualBaseline, supportingCast, routine, languageStyle, relationshipNote, relationshipKind, relationship, interactionBoundaries.',
    'inferredFields is an array of allowed field names that the model inferred rather than directly stated.',
    'majorEvents, strengths, weaknesses, quirks, obsessions, catchphrases, signatureBehaviors, coreBeliefs, and boundariesAndTaboos must be bounded arrays of concise strings.',
    'personalityCoordinates must be an extensible object {framework, values}; framework identifies Big Five, MBTI, or another declared coordinate system, and values is an object of bounded numeric or string coordinates. Do not flatten or infer coordinates.',
    'toneAndVocabulary must be an object with optional bounded string tone and vocabulary string array.',
    'interests, routine, supportingCast, and interactionBoundaries may be arrays of concise strings; structured list items may use their explicit name/label/activity fields and are normalized to strings; all other scalar persona fields must be strings, except age which may be a finite integer or bounded string.',
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

function structuredField(value, field, type) {
    if (type === 'text') return boundedText(value, field, 160);
    if ((typeof value === 'number' && Number.isFinite(value)) || (typeof value === 'string' && value.trim())) {
        if (typeof value === 'string' && value.trim().length > 80) throw analysisError(`${field}不能超过 80 个字符`);
        return value;
    }
    throw analysisError(`${field}必须是字符串或有限数字`);
}

function structuredListItem(item, field, maxLength, listType) {
    const schema = LIST_ITEM_SCHEMAS[listType];
    unknownKeys(item, new Set(Object.keys(schema.fields)), `${field}项目`);
    for (const [key, value] of Object.entries(item)) {
        structuredField(value, `${field}项目.${key}`, schema.fields[key]);
    }
    const textKey = schema.textKeys.find(key => Object.hasOwn(item, key));
    if (!textKey) throw analysisError(`${field}项目缺少可显示文本`);
    return boundedText(item[textKey], `${field}项目.${textKey}`, maxLength);
}

function stringList(value, field, maxLength, listType) {
    if (!Array.isArray(value)) throw analysisError(`${field}必须是字符串数组`);
    if (value.length > MAX_LIST_ITEMS) throw analysisError(`${field}项目过多`);
    return value.map(item => {
        if (typeof item === 'string') return boundedText(item, `${field}项目`, maxLength);
        if (!isRecord(item)) throw analysisError(`${field}项目必须是字符串或支持的结构化对象`);
        return structuredListItem(item, field, maxLength, listType);
    });
}

function boundedAge(value, field) {
    if (typeof value === 'number') {
        if (!Number.isInteger(value) || value < 0 || value > 150) throw analysisError(`${field}必须是 0 到 150 的整数`);
        return value;
    }
    return boundedText(value, field, 32);
}

function boundedStringList(value, field, maxLength, label) {
    if (!Array.isArray(value)) throw analysisError(`${label}必须是字符串数组`);
    if (value.length > MAX_LIST_ITEMS) throw analysisError(`${label}项目过多`);
    return value.map(item => boundedText(item, `${field}项目`, maxLength));
}

function normalizeCoordinateValues(value, field) {
    if (!isRecord(value)) throw analysisError(`${field}必须是 JSON 对象`);
    const entries = Object.entries(value);
    if (entries.length > MAX_LIST_ITEMS) throw analysisError(`${field}项目过多`);
    return Object.fromEntries(entries.map(([key, coordinate]) => {
        const normalizedKey = boundedText(key, `${field}键`, MAX_COORDINATE_KEY_LENGTH);
        if (typeof coordinate === 'number') {
            if (!Number.isFinite(coordinate)) throw analysisError(`${field}.${normalizedKey}必须是有限数字或字符串`);
            return [normalizedKey, coordinate];
        }
        return [normalizedKey, boundedText(coordinate, `${field}.${normalizedKey}`, MAX_COORDINATE_VALUE_LENGTH)];
    }));
}

function normalizePersonalityCoordinates(value, field) {
    unknownKeys(value, new Set(['framework', 'values']), field);
    const framework = boundedText(value.framework, `${field}.framework`, 64);
    const values = normalizeCoordinateValues(value.values, `${field}.values`);
    return {framework, values};
}

function normalizeToneAndVocabulary(value, field) {
    unknownKeys(value, new Set(['tone', 'vocabulary']), field);
    const normalized = {};
    if (value.tone !== undefined) normalized.tone = boundedText(value.tone, `${field}.tone`, MAX_PROFILE_TEXT_LENGTH);
    if (value.vocabulary !== undefined) normalized.vocabulary = boundedStringList(value.vocabulary, `${field}.vocabulary`, MAX_PROFILE_TEXT_LENGTH, '词汇');
    return normalized;
}

function normalizeNewField(key, raw, field) {
    if (key === 'age') return boundedAge(raw, `${field}.age`);
    if (key === 'personalityCoordinates') return normalizePersonalityCoordinates(raw, `${field}.personalityCoordinates`);
    if (key === 'toneAndVocabulary') return normalizeToneAndVocabulary(raw, `${field}.toneAndVocabulary`);
    const listSchema = NEW_STRING_LIST_SCHEMAS[key];
    if (listSchema) return boundedStringList(raw, `${field}.${key}`, listSchema.maxLength, listSchema.label);
    return undefined;
}

function normalizeInteractionBoundaries(raw, field) {
    return Array.isArray(raw)
        ? boundedStringList(raw, `${field}.interactionBoundaries`, MAX_PROFILE_TEXT_LENGTH, '互动边界')
        : boundedText(raw, `${field}.interactionBoundaries`, MAX_RELATIONSHIP_NOTE_LENGTH);
}

function normalizeAnswers(value, field = 'answers') {
    unknownKeys(value, ANSWER_KEYS, field);
    const answers = {};
    for (const key of Object.keys(value)) {
        const raw = value[key];
        if (key === 'interests') answers.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH, key);
        else if (key === 'routine') answers.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH, key);
        else if (key === 'supportingCast') answers.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH, key);
        else if (key === 'interactionBoundaries') answers.interactionBoundaries = normalizeInteractionBoundaries(raw, field);
        else if (ANSWER_KEYS.has(key) && (key === 'age' || NEW_STRING_LIST_SCHEMAS[key] || key === 'personalityCoordinates' || key === 'toneAndVocabulary')) answers[key] = normalizeNewField(key, raw, field);
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
        if (key === 'interests') normalized.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH, key);
        else if (key === 'routine') normalized.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH, key);
        else if (key === 'supportingCast') normalized.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH, key);
        else if (key === 'interactionBoundaries') normalized.interactionBoundaries = normalizeInteractionBoundaries(raw, field);
        else if (ANSWER_KEYS.has(key) && (key === 'age' || NEW_STRING_LIST_SCHEMAS[key] || key === 'personalityCoordinates' || key === 'toneAndVocabulary')) normalized[key] = normalizeNewField(key, raw, field);
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
    const structuredFields = [
        'age', 'gender', 'occupation', 'growthExperience', 'majorEvents', 'personalityCoordinates',
        'strengths', 'weaknesses', 'quirks', 'obsessions', 'toneAndVocabulary', 'catchphrases',
        'signatureBehaviors', 'coreBeliefs', 'boundariesAndTaboos'
    ];
    return {
        schemaVersion: 2,
        timezone: 'Asia/Shanghai',
        foundation: answers.foundation,
        identity: {name: answers.name, role: answers.role},
        ...Object.fromEntries(structuredFields.filter(key => answers[key] !== undefined).map(key => [key, answers[key]])),
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
