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
    'background', 'tone',
    'age', 'gender', 'occupation', 'growthExperience', 'majorEvents', 'personalityCoordinates',
    'strengths', 'weaknesses', 'quirks', 'obsessions', 'toneAndVocabulary', 'catchphrases',
    'signatureBehaviors', 'coreBeliefs', 'boundariesAndTaboos'
]);

const ANSWER_INPUT_KEYS = new Set([...ANSWER_KEYS, 'background', 'tone']);

const FIELD_ALIASES = Object.freeze({
    background: 'growthExperience',
    tone: 'languageStyle'
});

const NEW_STRING_LIST_SCHEMAS = Object.freeze({
    majorEvents: Object.freeze({maxLength: MAX_MAJOR_EVENT_LENGTH, label: '重大事件', textKeys: ['event', 'milestone', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    strengths: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '优点', textKeys: ['strength', 'trait', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    weaknesses: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '缺点', textKeys: ['weakness', 'trait', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    quirks: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '怪癖', textKeys: ['quirk', 'trait', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    obsessions: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '执念', textKeys: ['obsession', 'interest', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    catchphrases: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '口癖', textKeys: ['catchphrase', 'phrase', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    signatureBehaviors: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '标志性行为', textKeys: ['behavior', 'action', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    coreBeliefs: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '核心信仰', textKeys: ['belief', 'principle', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    boundariesAndTaboos: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '底线与禁忌', textKeys: ['boundary', 'taboo', 'rule', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    interactionBoundaries: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '互动边界', textKeys: ['boundary', 'rule', 'summary', 'description', 'text', 'value', 'name', 'label']}),
    vocabulary: Object.freeze({maxLength: MAX_PROFILE_TEXT_LENGTH, label: '词汇', textKeys: ['word', 'term', 'phrase', 'summary', 'description', 'text', 'value', 'name', 'label']})
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
    }),
    ...Object.fromEntries(Object.entries(NEW_STRING_LIST_SCHEMAS).map(([key, schema]) => [key, Object.freeze({
        textKeys: Object.freeze(schema.textKeys),
        fields: Object.freeze(Object.fromEntries(schema.textKeys.map(textKey => [textKey, 'text'])))
    })])),
    vocabulary: Object.freeze({
        textKeys: Object.freeze(NEW_STRING_LIST_SCHEMAS.vocabulary.textKeys),
        fields: Object.freeze(Object.fromEntries(NEW_STRING_LIST_SCHEMAS.vocabulary.textKeys.map(textKey => [textKey, 'text'])))
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

function normalizeScalarText(value, field, maxLength) {
    if (typeof value === 'string') return boundedText(value, field, maxLength);
    if (Array.isArray(value)) {
        const list = stringList(value, field, maxLength, 'majorEvents');
        return boundedText(list.join('；'), field, maxLength);
    }
    if (isRecord(value)) {
        const key = ['summary', 'description', 'text', 'value', 'narrative', 'note'].find(name => typeof value[name] === 'string');
        if (key) return boundedText(value[key], `${field}.${key}`, maxLength);
    }
    throw analysisError(`${field}必须是字符串、字符串数组或带明确文本的对象`);
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
    unknownKeys(value, new Set(['framework', 'values', 'model', 'type', 'traits', 'scores', 'mbti', 'bigFive']), field);
    const framework = boundedText(value.framework ?? value.model ?? 'custom', `${field}.framework`, 64);
    const sourceValues = value.values ?? value.traits ?? value.scores ?? {};
    const values = normalizeCoordinateValues(sourceValues, `${field}.values`);
    if (value.type !== undefined) values.type = boundedText(value.type, `${field}.type`, MAX_COORDINATE_VALUE_LENGTH);
    if (value.mbti !== undefined) values.mbti = boundedText(value.mbti, `${field}.mbti`, MAX_COORDINATE_VALUE_LENGTH);
    if (value.bigFive !== undefined) values.bigFive = normalizeCoordinateValues(value.bigFive, `${field}.bigFive`);
    return {framework, values};
}

function normalizeToneAndVocabulary(value, field) {
    unknownKeys(value, new Set(['tone', 'style', 'vocabulary', 'words']), field);
    const normalized = {};
    const tone = value.tone ?? value.style;
    const vocabulary = value.vocabulary ?? value.words;
    if (tone !== undefined) normalized.tone = normalizeScalarText(tone, `${field}.tone`, MAX_PROFILE_TEXT_LENGTH);
    if (vocabulary !== undefined) normalized.vocabulary = stringList(vocabulary, `${field}.vocabulary`, MAX_PROFILE_TEXT_LENGTH, 'vocabulary');
    return normalized;
}

function normalizeNewField(key, raw, field) {
    if (key === 'age') return boundedAge(raw, `${field}.age`);
    if (key === 'personalityCoordinates') return normalizePersonalityCoordinates(raw, `${field}.personalityCoordinates`);
    if (key === 'toneAndVocabulary') return normalizeToneAndVocabulary(raw, `${field}.toneAndVocabulary`);
    const listSchema = NEW_STRING_LIST_SCHEMAS[key];
    if (listSchema) return stringList(raw, `${field}.${key}`, listSchema.maxLength, key);
    return undefined;
}

function normalizeRelationshipObject(value, field) {
    unknownKeys(value, new Set(['note', 'kind', 'relationship', 'summary', 'description', 'text', 'value']), field);
    const note = value.note ?? value.relationship ?? value.summary ?? value.description ?? value.text ?? value.value;
    if (typeof note !== 'string') throw analysisError(`${field}缺少可显示文本`);
    return {
        note: boundedText(note, `${field}.note`, MAX_RELATIONSHIP_NOTE_LENGTH),
        ...(value.kind === undefined ? {} : {kind: boundedText(value.kind, `${field}.kind`, 80)})
    };
}

function canonicalizeAliases(value, field) {
    const source = {...value};
    for (const [alias, canonical] of Object.entries(FIELD_ALIASES)) {
        if (source[alias] !== undefined) {
            if (source[canonical] === undefined) source[canonical] = source[alias];
            delete source[alias];
        }
    }
    if (isRecord(source.identity)) {
        unknownKeys(source.identity, new Set(['name', 'role', 'age', 'gender', 'occupation']), `${field}.identity`);
        for (const key of ['name', 'role', 'age', 'gender', 'occupation']) {
            if (source[key] === undefined && source.identity[key] !== undefined) source[key] = source.identity[key];
        }
        delete source.identity;
    }
    if (isRecord(source.relationship)) {
        const relationship = normalizeRelationshipObject(source.relationship, `${field}.relationship`);
        if (source.relationshipNote === undefined) source.relationshipNote = relationship.note;
        if (source.relationshipKind === undefined && relationship.kind !== undefined) source.relationshipKind = relationship.kind;
        delete source.relationship;
    }
    if (isRecord(source.languageStyle)) {
        if (source.toneAndVocabulary === undefined) source.toneAndVocabulary = normalizeToneAndVocabulary(source.languageStyle, `${field}.languageStyle`);
        delete source.languageStyle;
    }
    return source;
}

function normalizeAnswers(value, field = 'answers') {
    unknownKeys(value, ANSWER_INPUT_KEYS, field);
    const source = canonicalizeAliases(value, field);
    const answers = {};
    for (const key of Object.keys(source)) {
        const raw = source[key];
        if (key === 'interests') answers.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH, key);
        else if (key === 'routine') answers.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH, key);
        else if (key === 'supportingCast') answers.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH, key);
        else if (key === 'interactionBoundaries') answers.interactionBoundaries = Array.isArray(raw)
            ? stringList(raw, `${field}.interactionBoundaries`, MAX_PROFILE_TEXT_LENGTH, key)
            : boundedText(raw, `${field}.interactionBoundaries`, MAX_RELATIONSHIP_NOTE_LENGTH);
        else if (key === 'growthExperience') answers.growthExperience = normalizeScalarText(raw, `${field}.growthExperience`, MAX_BACKGROUND_LENGTH);
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
    unknownKeys(value, new Set([...BLUEPRINT_KEYS, ...ANSWER_INPUT_KEYS]), 'blueprint');
    const source = canonicalizeAliases(value, 'blueprint');
    return normalizeOptionalFields(source, 'blueprint');
}

function normalizeOptionalFields(value, field) {
    const normalized = {};
    for (const key of Object.keys(value)) {
        const raw = value[key];
        if (key === 'interests') normalized.interests = stringList(raw, `${field}.interests`, MAX_INTEREST_LENGTH, key);
        else if (key === 'routine') normalized.routine = stringList(raw, `${field}.routine`, MAX_ROUTINE_ITEM_LENGTH, key);
        else if (key === 'supportingCast') normalized.supportingCast = stringList(raw, `${field}.supportingCast`, MAX_SUPPORTING_CAST_ITEM_LENGTH, key);
        else if (key === 'interactionBoundaries') normalized.interactionBoundaries = Array.isArray(raw)
            ? stringList(raw, `${field}.interactionBoundaries`, MAX_PROFILE_TEXT_LENGTH, key)
            : boundedText(raw, `${field}.interactionBoundaries`, MAX_RELATIONSHIP_NOTE_LENGTH);
        else if (key === 'growthExperience') normalized.growthExperience = normalizeScalarText(raw, `${field}.growthExperience`, MAX_BACKGROUND_LENGTH);
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
