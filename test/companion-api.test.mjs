import assert from 'node:assert/strict';
import {chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-test-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';
const {companionApp, companionTestHooks} = await import(`../server.js?test=${Date.now()}`);
const {database, createPersona, createEvent, requirePersona, deletePersona, listGroups, listActivities, listMessages, appendMessage, appendUserVisibleAssistantReply, splitUserVisibleAssistantReply, userVisibleChatPrompt, extractMediaIntent, extractPendingEventIntent, createVisibleMarkerRedactor, createPendingEvent, normalizePendingEventCall, normalizeProactiveDecision, freezeProactiveDecision, mediaRequestFromText, mediaCommitmentFromText, normalizeMediaRequest, normalizeMediaConceptEnvelope, normalizePersonaMediaConcept, normalizeMediaPromptTemplate, mediaConceptEnvelopeFor, renderMediaPromptTemplate, mediaPromptTemplateSections, systemCapabilityReplyForm, systemCapabilityMediaContract, systemCapabilityPendingEventContract, personaMediaConceptContract, imagePromptMasterContract, addActivityComment, setUserReaction, activeMemories, stateFor, resolvedStateFor, stateShape, scheduledState, contextFor, applyRelationshipEvolution, activeRelationshipPatch, explicitPlanFromMessage, createScheduleItem, rescheduleScheduleItem, createChatMediaRequest, completePolledMediaJob, completeGeneratedMedia, completeProactiveMessageJob, runPendingEventJob, completeActivityDecisionJob, parseActivityDecision, proactiveEligibility, personaFocusTier, publicBlueprint, restoreFoundationRevision, recoverPersona, reconcilePersona, buildInitialBlueprint, normalizeLifeBlueprint, validateLifeBlueprint, finalizeLifeBlueprint, generateInitialLifeBlueprint, lifeModelSchemaVersion, zonedPlanInstant, localDayBounds, normalizeDailyPlan, chooseTimelineTemplate, instantiateTimelineEvent, sleepAvailability, deferredBatchForMessage, trustedTimeReplyForMessage, createInterview, answerInterview, activateInterview, debugContextFor, redactDebugValue, debugSummary, debugInspectorEnabled, enqueueRelationshipEvolutionJob, providerFor, providerSummaries, mediaProviders, validateH3Configuration, h3ConfigSummary, h3Preflight, h3Args, h3OutputFile, leaseDurationForJob, submitMediaJob, saveSettings, publicSettings} = companionTestHooks;

const {interviewView, previewInterviewAnswers, validatePersonaDescription, normalizePersonaDescriptionExtraction, analyzePersonaDescription, createNaturalLanguageInterview, naturalLanguageDescriptionMaxLength} = companionTestHooks;
const {systemCapabilitySceneContract, imageGenerationPolicies, normalizeImageGenerationPolicy, imageGenerationPolicyFor, normalizeSceneEventCall, sharedSceneFor, applySceneEvent, sceneEventTool, consumeStreamedCompletion} = companionTestHooks;

const mediaConcept = (kind, overrides = {}) => ({
    schemaVersion: 1, mediaKind: kind, scene: '测试场景', action: '测试动作', mood: '平静', narrative: '测试媒体概念',
    humanSubjects: [{label: '人格本人', role: '主体', inFrame: true}],
    nonHumanObjects: [{label: '环境', kind: 'environment', inFrame: true}],
    capture: {mode: 'external_capture', operator: '画外朋友', deviceVisibility: 'out_of_frame', framingIntent: '自然中景'},
    compositionIntent: '保持概念中的主体与环境关系。', ...overrides
});
const mediaCall = (kind, request, overrides = {}) => ({
    schemaVersion: 2, kind, request, count: 1, personaMediaConcept: mediaConcept(kind, overrides.personaMediaConcept),
    currentEvent: null, temporaryAppearance: {hair: '自然短发'}, ...(overrides.trigger ? {trigger: overrides.trigger} : {}), ...overrides,
    personaMediaConcept: overrides.personaMediaConcept || mediaConcept(kind)
});

const routePaths = app => (app.router?.stack || []).flatMap(layer => layer.route ? [layer.route.path] : []);

function invokeRoute(path, method, {params = {}, body} = {}) {
    const layer = (companionApp.router?.stack || []).find(item => item.route?.path === path && item.route.methods?.[method.toLowerCase()]);
    assert.ok(layer, `${method} ${path} route is registered`);
    const response = {
        statusCode: 200, body: undefined, headersSent: false,
        status(code) { this.statusCode = code; return this; },
        json(value) { this.body = value; this.headersSent = true; return this; },
        end() { this.headersSent = true; return this; }
    };
    layer.route.stack[0].handle({params, body}, response);
    return response;
}

async function invokeRouteAsync(path, method, {params = {}, body} = {}) {
    const layer = (companionApp.router?.stack || []).find(item => item.route?.path === path && item.route.methods?.[method.toLowerCase()]);
    assert.ok(layer, `${method} ${path} route is registered`);
    const response = {
        statusCode: 200, body: undefined, headersSent: false,
        status(code) { this.statusCode = code; return this; },
        json(value) { this.body = value; this.headersSent = true; return this; },
        end() { this.headersSent = true; return this; }
    };
    const output = layer.route.stack[0].handle({params, body}, response);
    if (output?.then) await output.catch(() => {});
    return response;
}

test('contact groups seed a default, assign new personas, and persist route changes', () => {
    const migration = database.prepare("SELECT name FROM companion_schema_migrations WHERE version = 9").get();
    assert.equal(migration.name, 'persona-contact-groups');
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM sqlite_master WHERE type = 'table' AND name = 'companion_groups'").get().count, 1);

    const initialGroups = listGroups();
    assert.deepEqual(initialGroups.map(group => ({name: group.name, isDefault: group.isDefault, personaCount: group.personaCount})), [{name: '默认', isDefault: true, personaCount: 0}]);
    const persona = createPersona({name: '分组测试', role: '测试人格', foundation: '分组测试用于验证联系人分组。'});
    try {
        const defaultGroup = listGroups()[0];
        assert.equal(persona.groupId, defaultGroup.id);
        assert.equal(persona.groupName, '默认');
        assert.equal(defaultGroup.personaCount, 1);

        const initialBootstrap = invokeRoute('/api/companion/bootstrap', 'GET');
        assert.equal(initialBootstrap.statusCode, 200);
        assert.deepEqual(initialBootstrap.body.groups[0], defaultGroup);
        assert.equal(initialBootstrap.body.personas.find(item => item.id === persona.id).groupId, defaultGroup.id);
        assert.equal(initialBootstrap.body.personas.find(item => item.id === persona.id).groupName, '默认');

        const created = invokeRoute('/api/companion/groups', 'POST', {body: {name: '  工作  '}});
        assert.equal(created.statusCode, 201);
        assert.match(created.body.id, /^group_/);
        assert.equal(created.body.name, '工作');
        assert.equal(created.body.isDefault, false);
        assert.equal(created.body.personaCount, 0);
        const createdGroupId = created.body.id;

        const assigned = invokeRoute('/api/companion/personas/:personaId/group', 'PUT', {params: {personaId: persona.id}, body: {groupId: createdGroupId}});
        assert.equal(assigned.statusCode, 200);
        assert.equal(assigned.body.id, persona.id);
        assert.equal(assigned.body.groupId, createdGroupId);
        assert.equal(assigned.body.groupName, '工作');
        assert.equal(database.prepare('SELECT group_id FROM companion_personas WHERE id = ?').get(persona.id).group_id, createdGroupId);

        const persistedBootstrap = invokeRoute('/api/companion/bootstrap', 'GET');
        const persistedPersona = persistedBootstrap.body.personas.find(item => item.id === persona.id);
        assert.equal(persistedPersona.groupId, createdGroupId);
        assert.equal(persistedPersona.groupName, '工作');
        assert.equal(persistedBootstrap.body.groups.find(group => group.id === defaultGroup.id).personaCount, 0);
        assert.equal(persistedBootstrap.body.groups.find(group => group.id === createdGroupId).personaCount, 1);

        const duplicate = invokeRoute('/api/companion/groups', 'POST', {body: {name: '工作'}});
        assert.equal(duplicate.statusCode, 400);
        assert.equal(duplicate.body.error, '分组名称已存在');
        const blank = invokeRoute('/api/companion/groups', 'POST', {body: {name: '   '}});
        assert.equal(blank.statusCode, 400);
        const tooLong = invokeRoute('/api/companion/groups', 'POST', {body: {name: 'x'.repeat(61)}});
        assert.equal(tooLong.statusCode, 400);
        const malformed = invokeRoute('/api/companion/groups', 'POST', {body: []});
        assert.equal(malformed.statusCode, 400);

        const unknownGroup = invokeRoute('/api/companion/personas/:personaId/group', 'PUT', {params: {personaId: persona.id}, body: {groupId: 'group_missing'}});
        assert.equal(unknownGroup.statusCode, 404);
        const unknownPersona = invokeRoute('/api/companion/personas/:personaId/group', 'PUT', {params: {personaId: 'persona_missing'}, body: {groupId: createdGroupId}});
        assert.equal(unknownPersona.statusCode, 404);
        const malformedAssignment = invokeRoute('/api/companion/personas/:personaId/group', 'PUT', {params: {personaId: persona.id}, body: {}});
        assert.equal(malformedAssignment.statusCode, 400);
        assert.equal(database.prepare('SELECT group_id FROM companion_personas WHERE id = ?').get(persona.id).group_id, createdGroupId);
    } finally {
        deletePersona(persona.id);
    }
});

test('shared scene events persist continuity and image policy stays persona-scoped', () => {
    assert.deepEqual(imageGenerationPolicies, ['ask', 'always', 'important', 'user_only', 'autonomous']);
    assert.equal(normalizeImageGenerationPolicy('unknown'), 'autonomous');
    assert.throws(() => normalizeSceneEventCall({operation: 'switch', situation: '缺地点'}), /必须包含地点或活动/);
    assert.match(systemCapabilitySceneContract, /scene_event/);

    const persona = createPersona({name: '同场景测试', role: '测试人格', foundation: '用于验证共同场景连续性。'});
    try {
        const detail = invokeRoute('/api/companion/personas/:personaId', 'GET', {params: {personaId: persona.id}});
        assert.equal(detail.body.persona.imageGenerationPolicy, 'autonomous');
        const policy = invokeRoute('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {params: {personaId: persona.id}, body: {policy: 'user_only'}});
        assert.equal(policy.body.imageGenerationPolicy, 'user_only');
        const invalidPolicy = invokeRoute('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {params: {personaId: persona.id}, body: {policy: '拍照就生成'}});
        assert.equal(invalidPolicy.statusCode, 400);

        const startMessage = appendMessage(persona.id, {role: 'user', text: '我们去湖边走走。'});
        const started = applySceneEvent(persona, {operation: 'start', location: '湖边公园', activity: '沿湖散步', situation: '和用户一起沿湖边散步', mood: '放松', objects: ['雨伞']}, startMessage.id);
        assert.equal(started.operation, 'start');
        let state = resolvedStateFor(persona.id);
        assert.equal(state.source, 'shared_scene');
        assert.equal(state.location, '湖边公园');
        assert.equal(state.sharedScene.activity, '沿湖散步');
        assert.match(contextFor(persona.id).layers.lifeState, /湖边公园/);

        const ordinaryEvent = createEvent(persona, {type: 'social', situation: '和朋友聊了几句', mood: '开心', scene: '校园'}, {publish: false, source: 'chat'});
        assert.ok(ordinaryEvent.eventId);
        assert.equal(resolvedStateFor(persona.id).location, '湖边公园');

        const switchMessage = appendMessage(persona.id, {role: 'user', text: '那我们去咖啡馆坐坐。'});
        const switched = applySceneEvent(persona, {operation: 'switch', location: '街角咖啡馆', activity: '靠窗聊天', situation: '和用户在咖啡馆靠窗的位置聊天', mood: '安静'}, switchMessage.id);
        assert.equal(switched.previousScene.location, '湖边公园');
        assert.equal(resolvedStateFor(persona.id).location, '街角咖啡馆');
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type = 'shared_scene'").get(persona.id).count, 2);

        const endMessage = appendMessage(persona.id, {role: 'user', text: '今天先到这里。'});
        const ended = applySceneEvent(persona, {operation: 'end'}, endMessage.id);
        assert.equal(ended.operation, 'end');
        assert.notEqual(resolvedStateFor(persona.id).source, 'shared_scene');
        assert.equal(resolvedStateFor(persona.id).sharedScene, null);
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type = 'shared_scene_end'").get(persona.id).count, 1);
    } finally {
        deletePersona(persona.id);
    }
});

test('life model v2 fallback supplies a default room, safe event templates, and v7 tables', () => {
    const requiredTables = [
        'companion_persona_life_blueprint_revisions', 'companion_timeline_slots', 'companion_event_decisions',
        'companion_event_links', 'companion_chat_deferred_batches'
    ];
    for (const name of requiredTables) assert.equal(database.prepare("SELECT COUNT(*) AS count FROM sqlite_master WHERE type = 'table' AND name = ?").get(name).count, 1);

    const fallback = buildInitialBlueprint({name: '阿遥', role: '在读大学生', interests: '摄影、旧书店'});
    assert.equal(fallback.schemaVersion, lifeModelSchemaVersion);
    assert.equal(fallback.timezone, 'Asia/Shanghai');
    assert.deepEqual(fallback.world.defaultSceneRef, {locationId: 'home', roomId: 'private_room'});
    const defaultRoom = fallback.world.locations.find(location => location.id === 'home').rooms.find(room => room.id === 'private_room');
    assert.match(defaultRoom.scene, /自己的宿舍房间/);
    assert.equal(validateLifeBlueprint(fallback).ok, true);
    assert.equal(fallback.fixedTimeEvents.length > 0, true);
    assert.equal(fallback.dailyFlexibleEvents.length > 0, true);
    assert.equal(fallback.randomPositiveEvents.length > 0, true);
    assert.equal(fallback.randomNegativeEvents.length > 0, true);

    const unsafe = structuredClone(fallback);
    unsafe.randomNegativeEvents[0] = {...unsafe.randomNegativeEvents[0], situation: '遭遇严重伤害', recovery: ''};
    const validation = validateLifeBlueprint(normalizeLifeBlueprint(unsafe));
    assert.equal(validation.ok, false);
    assert.match(validation.errors.join('；'), /负向事件/);
    assert.equal(finalizeLifeBlueprint(unsafe, fallback).generation.usedFallback, true);
    const unsafePositive = structuredClone(fallback);
    unsafePositive.randomPositiveEvents[0] = {...unsafePositive.randomPositiveEvents[0], title: '需要用户借钱处理债务'};
    assert.equal(validateLifeBlueprint(normalizeLifeBlueprint(unsafePositive)).ok, false);
});

test('persona local-day bounds preserve an Asia/Shanghai morning that belongs to the prior UTC date', () => {
    const instant = zonedPlanInstant('2026-08-19', '06:00', 'Asia/Shanghai');
    assert.equal(instant.startsWith('2026-08-18T22:00:00'), true);
    const bounds = localDayBounds('2026-08-19', 'Asia/Shanghai');
    assert.equal(instant >= bounds.start && instant < bounds.end, true);
});

test('initial life-model generation falls back safely when the local model request fails', async () => {
    const baseline = buildInitialBlueprint({name: '回退', role: '学生', foundation: '回退保持稳定的日常。'});
    const originalFetch = globalThis.fetch;
    const previous = publicSettings();
    saveSettings({model: 'test-life-model'});
    globalThis.fetch = async () => { throw new Error('mock timeout'); };
    try {
        const generated = await generateInitialLifeBlueprint({name: '回退', role: '学生', foundation: baseline.foundation}, baseline);
        assert.equal(generated.generation.usedFallback, true);
        assert.equal(validateLifeBlueprint(generated).ok, true);
        assert.match(generated.generation.validationWarnings.join(' '), /mock timeout/);
    } finally {
        globalThis.fetch = originalFetch;
        saveSettings({model: previous.model, lmStudioUrl: previous.lmStudioUrl});
    }
});

test('clean-start companion flow isolates persona data and keeps reactions idempotent', () => {
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_personas').get().count, 0);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM sqlite_master WHERE type = 'table' AND name = 'app_state'").get().count, 0);

    const persona = createPersona({name: '林晚', role: '在读大学生', foundation: '林晚学习视觉设计，性格开朗而有主见。', supportingCast: [{name: '小柯', relationshipKind: '室友'}]});
    assert.equal(Object.hasOwn(publicBlueprint(persona.id), 'foundation'), false);
    assert.deepEqual(publicBlueprint(persona.id).world.defaultSceneRef, {locationId: 'home', roomId: 'private_room'});
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_life_blueprint_revisions WHERE persona_id = ?').get(persona.id).count, 1);
    const source = requirePersona(persona.id);
    const result = createEvent(source, {
        type: 'shopping', situation: '在商场挑衣服', mood: '开心', scene: '商场',
        content: '今天挑到一件很喜欢的外套。', resolvesAt: new Date(Date.now() + 3_600_000).toISOString()
    }, {publish: true, source: 'test'});

    const activityId = result.activityId;
    const comment = addActivityComment(activityId, '很适合你。');
    assert.equal(comment.authorKind, 'user');
    setUserReaction(activityId, true);
    setUserReaction(activityId, true);

    const feed = listActivities({limit: 20});
    assert.equal(feed.items.length, 1);
    assert.equal(feed.items[0].liked, true);
    assert.equal(feed.items[0].comments.length, 1);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_activity_reactions WHERE activity_id = ? AND actor_kind = 'user'").get(activityId).count, 1);
    assert.equal(activeMemories(persona.id)[0].value, '很适合你。');
    assert.equal(stateFor(persona.id).situation, '在商场挑衣服');
    assert.equal(stateShape(persona.id).source.kind, 'shopping');

    const social = createEvent(source, {type: 'social', situation: '和室友散步', mood: '轻松', scene: '校园', content: '和小柯散步聊了很久。'}, {publish: true, source: 'test'});
    const socialActivity = listActivities({limit: 20}).items.find(item => item.id === social.activityId);
    assert.equal(socialActivity.comments[0].authorKind, 'supporting_character');
    assert.equal(socialActivity.comments[0].authorName, '小柯');

    const introduction = createEvent(source, {type: 'shopping', situation: '在独立书店挑选画册', mood: '愉快', scene: '书店', content: '在书店遇到了一位聊得来的同好。', introducedCharacter: {name: '顾栖', relationshipKind: '新认识的同好'}}, {publish: true, source: 'test'});
    assert.equal(database.prepare('SELECT introduced_event_id FROM companion_supporting_characters WHERE persona_id = ? AND name = ?').get(persona.id, '顾栖').introduced_event_id, introduction.eventId);
    const introductionActivity = listActivities({limit: 20}).items.find(item => item.id === introduction.activityId);
    assert.equal(introductionActivity.comments[0].authorName, '顾栖');

    const evolution = applyRelationshipEvolution(persona.id, {reason: '用户持续分享摄影计划', evidence: [{type: 'message', id: 'message_example'}], patch: {communicationStyle: '可以自然聊摄影计划', sharedTopics: ['摄影']}});
    assert.equal(activeRelationshipPatch(persona.id).communicationStyle, '可以自然聊摄影计划');
    applyRelationshipEvolution(persona.id, {reason: '用户又提到摄影', evidence: [{type: 'message', id: 'message_followup'}], patch: {sharedTopics: ['摄影', '展览']}});
    assert.deepEqual(activeRelationshipPatch(persona.id), {communicationStyle: '可以自然聊摄影计划', sharedTopics: ['摄影', '展览']});
    database.prepare("UPDATE companion_persona_evolutions SET status = 'reverted' WHERE id = ?").run(evolution.id);
    assert.deepEqual(activeRelationshipPatch(persona.id), {communicationStyle: '可以自然聊摄影计划', sharedTopics: ['摄影', '展览']});

    database.prepare('INSERT INTO companion_activity_visibility (activity_id, hidden_at, updated_at) VALUES (?, ?, ?)').run(activityId, new Date().toISOString(), new Date().toISOString());
    assert.equal(listActivities({limit: 20}).items.some(item => item.id === activityId), false);
    assert.equal(listActivities({limit: 20, visibility: 'hidden'}).items.some(item => item.id === activityId), true);

    const plan = explicitPlanFromMessage('我们约好了明天下午3点去看展。');
    assert.ok(plan);
    const schedule = createScheduleItem(persona.id, {...plan, source: 'test'});
    assert.equal(database.prepare('SELECT status FROM companion_schedule_items WHERE id = ?').get(schedule.id).status, 'active');
    assert.equal(explicitPlanFromMessage('明天也许去看展吧。'), null);
    createEvent(source, {type: 'schedule', scheduleId: schedule.id, situation: schedule.title, mood: '期待', scene: '美术馆'}, {publish: false, source: 'test'});
    assert.equal(stateShape(persona.id).source.scheduleId, schedule.id);

    database.prepare('UPDATE companion_persona_states SET checkpoint_at = ? WHERE persona_id = ?').run(new Date(Date.now() - 31 * 60 * 1000).toISOString(), persona.id);
    recoverPersona(persona.id);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type = 'recovery'").get(persona.id).count, 1);
});

test('timeline decisions persist no-event outcomes and sleep batches merge without waking again', () => {
    const persona = createPersona({name: '时间线', role: '学生', foundation: '时间线有自己的生活节奏。'});
    const source = requirePersona(persona.id);
    const at = new Date();
    const first = chooseTimelineTemplate(source, at);
    assert.equal(first.decisionKey, chooseTimelineTemplate(source, at).decisionKey);
    instantiateTimelineEvent(source, at);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_event_decisions WHERE persona_id = ? AND decision_key = ?').get(persona.id, first.decisionKey).count, 1);
    const midnight = new Date();
    midnight.setHours(0, 30, 0, 0);
    assert.equal(sleepAvailability(source, midnight).sleeping, true);
    const firstMessage = appendMessage(persona.id, {role: 'user', text: '睡了吗？'});
    const firstBatch = deferredBatchForMessage(source, firstMessage.id, midnight);
    const secondMessage = appendMessage(persona.id, {role: 'user', text: '明天再回也没关系。'});
    const secondBatch = deferredBatchForMessage(source, secondMessage.id, midnight);
    assert.equal(secondBatch.id, firstBatch.id);
    assert.deepEqual(JSON.parse(secondBatch.message_ids_json), [firstMessage.id, secondMessage.id]);
});

test('relationship evolution jobs debounce to the newest user message per persona', () => {
    const persona = createPersona({name: '关系去重', role: '学生', foundation: '关系去重会在安静一段时间后再整理记忆。'});
    const otherPersona = createPersona({name: '另一人格', role: '学生', foundation: '不同人格的关系演化作业互不影响。'});
    const firstMessage = appendMessage(persona.id, {role: 'user', text: '第一条消息'});
    const first = enqueueRelationshipEvolutionJob(persona.id, firstMessage.id);
    const secondMessage = appendMessage(persona.id, {role: 'user', text: '第二条消息'});
    const second = enqueueRelationshipEvolutionJob(persona.id, secondMessage.id);
    const otherMessage = appendMessage(otherPersona.id, {role: 'user', text: '另一人格的消息'});
    const other = enqueueRelationshipEvolutionJob(otherPersona.id, otherMessage.id);

    const firstRow = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(first.id);
    const secondRow = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(second.id);
    const queuedForPersona = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'relationship_evolution' AND status = 'queued'").all(persona.id);
    const queuedForOtherPersona = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'relationship_evolution' AND status = 'queued'").all(otherPersona.id);

    assert.equal(firstRow.status, 'complete');
    assert.deepEqual(JSON.parse(firstRow.result_json), {skipped: 'superseded_by_newer_message', supersededByJobId: second.id, supersededByMessageId: secondMessage.id});
    assert.ok(firstRow.completed_at);
    assert.equal(secondRow.status, 'queued');
    assert.equal(secondRow.message_id, secondMessage.id);
    assert.equal(new Date(secondRow.run_after).getTime() - new Date(secondRow.created_at).getTime() >= 10 * 60 * 1000 - 1_000, true);
    assert.deepEqual(queuedForPersona.map(job => job.id), [second.id]);
    assert.deepEqual(queuedForOtherPersona.map(job => job.id), [other.id]);
});

test('a life event waits for the persona activity decision and respects a no-post decision', () => {
    const persona = createPersona({name: '动态决定', role: '学生', foundation: '动态决定会自己判断什么值得分享。'});
    const event = createEvent(requirePersona(persona.id), {type: 'social', situation: '和朋友在校园散步', mood: '轻松', scene: '校园'}, {requestActivityDecision: true, source: 'test'});
    assert.equal(listActivities({personaId: persona.id, limit: 20}).items.length, 0);
    const job = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'activity_decision' ORDER BY created_at DESC LIMIT 1").get(persona.id);
    assert.ok(job);
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run('activity_decision_lease', new Date(Date.now() + 60_000).toISOString(), job.id);
    const declined = completeActivityDecisionJob({...job, lease_owner: 'activity_decision_lease'}, parseActivityDecision('{"publish":false,"content":"","media":{"kind":"none"}}'));
    assert.equal(declined.completed, true);
    assert.equal(declined.result.reason, 'persona_decided_not_to_publish');
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activities WHERE event_id = ?').get(event.eventId).count, 0);
});

test('adaptive interviews skip known facts and preserve inferred blueprint provenance', () => {
    let interview = createInterview({name: '余真', role: '独立插画师', foundation: '余真安静、细致，喜欢把普通生活画成小画。', interests: '速写、旧书店', supportingCast: '同学阿澄'});
    assert.equal(interview.question.key, 'ageBand');
    while (interview.status !== 'ready') interview = answerInterview(interview.id, {key: interview.question.key, answer: ''});
    assert.equal(interview.status, 'ready');
    assert.equal(interview.preview.blueprint.provenance.foundation, 'user');
    assert.equal(interview.preview.blueprint.provenance.interests, 'user');
    assert.equal(interview.preview.blueprint.provenance.visualBaseline, 'inferred');
    assert.equal(interview.preview.blueprint.supportingCast[0].name, '同学阿澄');
    assert.equal(interview.preview.blueprint.characterCard.interactionRules.userIdentity, '');
    assert.equal(interview.preview.blueprint.provenance['interactionRules.userIdentity'], 'inferred');

    const activated = activateInterview(interview.id, {overrides: {visualBaseline: '短发，常穿宽松衬衫', userIdentity: '一位刚认识、愿意慢慢熟悉的朋友'}});
    assert.equal(activated.name, '余真');
    assert.equal(database.prepare('SELECT status FROM companion_interview_sessions WHERE id = ?').get(interview.id).status, 'activated');
    assert.equal(publicBlueprint(activated.id).characterCard.interactionRules.userIdentity, '一位刚认识、愿意慢慢熟悉的朋友');

    const readyAtStart = createInterview({name: '安禾', role: '学生', foundation: '安禾喜欢观察校园里的普通片刻。', ageBand: '20 岁出头', occupation: '学生', socialIdentity: '摄影社成员', householdContext: '住校', initialRelationships: '室友小满', personalityTraits: '细腻', socialAttitude: '友善', languageStyle: '自然简短', specialSetting: '周末会拍照', interests: '摄影', culturalPresentation: '自然校园感', faceBuild: '圆脸', complexionAura: '清爽', hair: '短发', everydayWardrobe: '宽松衬衫', distinguishingFeatures: '常带相机', visualBaseline: '短发', supportingCast: '室友小满', userIdentity: '新朋友', communicationDistance: '自然慢慢熟悉', interactionBoundaries: '尊重彼此生活节奏'});
    assert.equal(readyAtStart.status, 'ready');
});

test('natural-language analysis stores bounded provenance and activation preserves edits', async () => {
    const migration = database.prepare('SELECT name FROM companion_schema_migrations WHERE version = 10').get();
    assert.equal(migration.name, 'natural-language-interview-provenance');
    const columns = database.prepare('PRAGMA table_info(companion_interview_sessions)').all().map(column => column.name);
    assert.equal(columns.includes('source'), true);
    assert.equal(columns.includes('inferred_fields_json'), true);
    const defaults = normalizePersonaDescriptionExtraction({answers: {personalityTraits: '温和'}, inferredFields: []});
    assert.deepEqual(defaults.answers, {personalityTraits: '温和', name: '新朋友', role: '陪伴者', foundation: '新朋友是一位陪伴者。'});
    assert.deepEqual(defaults.inferredFields, ['name', 'role', 'foundation']);

    const description = '她叫林晚，是在读设计专业的学生，喜欢摄影和旧书。她性格细腻慢热，希望和我从自然、尊重边界的朋友关系开始。';
    const originalFetch = globalThis.fetch;
    const previous = publicSettings();
    const modelAnswers = {name: '林晚', role: '在读设计专业学生', foundation: '林晚性格细腻慢热，喜欢摄影和旧书。', personalityTraits: '细腻、慢热', interests: ['摄影', '旧书']};
    saveSettings({model: 'test-persona-description'});
    globalThis.fetch = async (url, options) => {
        assert.match(String(url), /chat\/completions$/);
        assert.equal(JSON.parse(options.body).messages[1].content, description);
        return {ok: true, json: async () => ({choices: [{message: {content: `\`\`\`json\n${JSON.stringify({answers: modelAnswers, inferredFields: ['personalityTraits']})}\n\`\`\``}}]})};
    };
    let created;
    try {
        created = await createNaturalLanguageInterview(description);
        assert.equal(created.status, 'ready');
        assert.equal(created.source, 'natural-language');
        assert.deepEqual(created.inferredFields, ['personalityTraits']);
        assert.equal(created.answers.name, '林晚');
        assert.equal(created.preview.blueprint.provenance.name, 'user');
        assert.equal(created.preview.blueprint.provenance.foundation, 'user');
        assert.equal(created.preview.blueprint.provenance.personalityTraits, 'inferred');
        assert.equal(created.preview.blueprint.provenance['personalityCore.traits'], 'inferred');
        assert.equal(JSON.stringify(created).includes(description), false);
        const row = database.prepare('SELECT source, inferred_fields_json, answers_json FROM companion_interview_sessions WHERE id = ?').get(created.id);
        assert.equal(row.source, 'natural-language');
        assert.deepEqual(JSON.parse(row.inferred_fields_json), ['personalityTraits']);
        assert.equal(row.answers_json.includes(description), false);

        const untouched = await createNaturalLanguageInterview(description);
        const untouchedPersona = activateInterview(untouched.id);
        assert.equal(publicBlueprint(untouchedPersona.id).provenance.personalityTraits, 'inferred');
        deletePersona(untouchedPersona.id);

        const activated = activateInterview(created.id, {overrides: {name: '林晚（确认）', foundation: '确认后的基础人格。'}});
        assert.equal(activated.name, '林晚（确认）');
        assert.equal(publicBlueprint(activated.id).provenance.name, 'user');
        assert.equal(publicBlueprint(activated.id).provenance.foundation, 'user');
        assert.equal(publicBlueprint(activated.id).provenance.personalityTraits, 'inferred');
        deletePersona(activated.id);
    } finally {
        globalThis.fetch = originalFetch;
        saveSettings({model: previous.model, lmStudioUrl: previous.lmStudioUrl});
    }
});

test('natural-language analyze route rejects invalid model/input without creating sessions', async () => {
    const before = database.prepare('SELECT COUNT(*) AS count FROM companion_interview_sessions').get().count;
    const originalFetch = globalThis.fetch;
    const previous = publicSettings();
    let calls = 0;
    globalThis.fetch = async () => {
        calls += 1;
        return {ok: true, json: async () => ({choices: [{message: {content: '{"answers":{"name":"坏结果","unknown":"不应保存"},"inferredFields":[]}'}}]})};
    };
    try {
        const missing = await invokeRouteAsync('/api/companion/interviews/analyze', 'POST', {body: {}});
        assert.equal(missing.statusCode, 400);
        assert.equal(missing.body.error, '人格描述不能为空');
        const oversized = await invokeRouteAsync('/api/companion/interviews/analyze', 'POST', {body: {description: 'x'.repeat(naturalLanguageDescriptionMaxLength + 1)}});
        assert.equal(oversized.statusCode, 400);
        assert.equal(oversized.body.error, `人格描述不能超过 ${naturalLanguageDescriptionMaxLength} 个字符`);
        assert.equal(calls, 0);

        saveSettings({model: 'test-persona-description'});
        const unknown = await invokeRouteAsync('/api/companion/interviews/analyze', 'POST', {body: {description: '请分析这段描述'}});
        assert.equal(unknown.statusCode, 502);
        assert.match(unknown.body.error, /^人格分析失败：/);
        assert.equal(calls, 1);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_interview_sessions').get().count, before);

        globalThis.fetch = async () => { calls += 1; throw new Error('mock provider timeout'); };
        const failed = await invokeRouteAsync('/api/companion/interviews/analyze', 'POST', {body: {description: '请分析后模拟 provider 失败'}});
        assert.equal(failed.statusCode, 502);
        assert.match(failed.body.error, /人格分析失败：mock provider timeout/);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_interview_sessions').get().count, before);
    } finally {
        globalThis.fetch = originalFetch;
        saveSettings({model: previous.model, lmStudioUrl: previous.lmStudioUrl});
    }
});

test('user-visible assistant replies are sentence-scoped, ordered, and isolated from JSON-only prompts', () => {
    const persona = createPersona({name: '句读', role: '学生', foundation: '句读喜欢用简洁的话分享日常。'});
    assert.deepEqual(splitUserVisibleAssistantReply('第一句。第二句没有结尾'), ['第一句。', '第二句没有结尾。']);
    const persisted = appendUserVisibleAssistantReply(persona.id, '第一句。第二句没有结尾');
    assert.deepEqual(persisted.map(message => message.text), ['第一句。', '第二句没有结尾。']);
    assert.deepEqual(listMessages(persona.id, {markRead: false}).items.map(message => message.text), ['第一句。', '第二句没有结尾。']);
    const prompt = userVisibleChatPrompt(persona.id, '写一条普通的聊天回复。');
    assert.equal(prompt.endsWith(systemCapabilityReplyForm), true);
    assert.match(systemCapabilityReplyForm, /恰好是一句完整的话/);
    assert.match(systemCapabilityMediaContract, /<media-intent>/);
    const marker = mediaCall('image', '咖啡馆窗边的自然照片', {temporaryAppearance: {hair: '银灰短发'}});
    const parsedMarker = extractMediaIntent(`我来找一张图。<media-intent>${JSON.stringify(marker)}</media-intent>`);
    assert.equal(parsedMarker.text, '我来找一张图。');
    assert.equal(parsedMarker.media.schemaVersion, 2);
    assert.equal(parsedMarker.media.personaMediaConcept.capture.mode, 'external_capture');
    assert.deepEqual(parsedMarker.media.temporaryAppearance, {hair: '银灰短发'});
    assert.equal(extractMediaIntent('好的，我这就给你生成一张海边日落的图片。').media, null);
    assert.equal(extractMediaIntent('<media-intent>{"kind":"audio","request":"不支持"}</media-intent>').media, null);
    assert.equal(mediaCommitmentFromText('我待会拍完发你。'), null);
    assert.equal(mediaRequestFromText('请给我生成一段在雨中散步的视频。'), null);
    assert.equal(mediaRequestFromText('不要生成图片，我们继续聊天。'), null);
    assert.equal(extractMediaIntent('今天聊聊天。').media, null);
});

test('media envelopes and model-owned schemas are structurally validated without visual inference', () => {
    const persona = createPersona({name: '契约', role: '学生', foundation: '契约会遵守当前生活状态。'});
    assert.deepEqual(normalizeMediaRequest({kind: 'image'}), {kind: 'image'});
    assert.deepEqual(normalizeMediaRequest({kind: 'video', request: '傍晚散步', count: 9}), {kind: 'video', request: '傍晚散步', count: 3});
    const envelope = mediaConceptEnvelopeFor(requirePersona(persona.id), {kind: 'image', request: '手持手机自拍，举高45度角比心', trigger: 'test'});
    assert.equal(envelope.facts.currentState.situation.length > 0, true);
    assert.equal(Object.hasOwn(envelope, 'people'), false);
    assert.equal(Object.hasOwn(envelope, 'camera'), false);
    assert.throws(() => normalizeMediaConceptEnvelope({...envelope, inferredPeople: ['不允许']}), /不支持字段/);
    assert.throws(() => normalizePersonaMediaConcept({schemaVersion: 1, mediaKind: 'image', scene: '公园', action: '散步', mood: '开心', narrative: '日常散步', humanSubjects: [], nonHumanObjects: [], capture: {mode: 'server_guess', operator: '', deviceVisibility: 'unspecified', framingIntent: '中景'}, compositionIntent: ''}), /拍摄方式无效/);
    assert.throws(() => normalizeMediaPromptTemplate({schemaVersion: 1, sections: {capture: 'x'}}), /缺少固定段落/);
    const masterOwnedLength = Object.fromEntries(mediaPromptTemplateSections.map(section => [section, 'x'.repeat(1_200)]));
    assert.equal(normalizeMediaPromptTemplate({schemaVersion: 1, sections: masterOwnedLength}).sections.capture.length, 1_200);
    assert.throws(() => createChatMediaRequest(persona.id, {kind: 'audio', schemaVersion: 2, personaMediaConcept: mediaConcept('audio'), currentEvent: null, temporaryAppearance: {}}), /媒体请求/);
});

test('fixed prompt templates preserve master-selected selfie, photographer POV, and non-human object separation', () => {
    const template = normalizeMediaPromptTemplate({
        schemaVersion: 1,
        sections: {
            capture: '由林晚手持前置手机自拍，镜头略高于眼睛，取景合理。',
            humanSubjects: '入镜的人类主体只有林晚和小周，两人自然比心。',
            identityAndContinuity: '保持林晚既有短发与日常气质。',
            sceneAndAction: '河边步道上的轻松自拍。',
            wardrobeAndNonHumanProps: '蓝色外套、手机、两套待比较的服装和一只小狗均为非人物对象。',
            lightingAndMood: '左侧自然光，开心放松。',
            photographyStyleAndColor: '真实手机摄影，干净自然色彩。',
            constraints: '不要增加未声明的人类主体；服装、手机和小狗不得变成人。'
        }
    });
    const prompt = renderMediaPromptTemplate(template);
    const labels = ['拍摄方式与镜头关系', '明确人类主体', '身份与外观连续性', '场景与动作', '穿搭与非人物道具', '光线与情绪', '摄影风格与色调', '约束与排除项'];
    const positions = labels.map(label => prompt.indexOf(`【${label}】`));
    assert.equal(positions.every((position, index) => position >= 0 && (!index || position > positions[index - 1])), true);
    assert.match(prompt, /入镜的人类主体只有林晚和小周/);
    assert.match(prompt, /服装、手机和小狗不得变成人/);
    assert.doesNotMatch(prompt, /共\s*\d+\s*人/);
    assert.match(personaMediaConceptContract, /nonHumanObjects/);
    assert.match(imagePromptMasterContract, /固定的生图模板/);
});

test('media providers validate capabilities, persist selection, and keep h3 paths private', () => {
    assert.deepEqual(providerSummaries().map(provider => provider.id), ['comfyui', 'h3']);
    assert.throws(() => providerFor('image', 'h3'), /不支持图片/);
    const h3Root = join(dataDir, 'h3-config-fixture');
    const modelDir = join(h3Root, 'MiniMax-H3');
    const outputDir = join(h3Root, 'outputs');
    mkdirSync(modelDir, {recursive: true});
    saveSettings({videoProvider: 'h3', h3Executable: process.execPath, h3ModelDir: modelDir, h3OutputDir: outputDir, h3AllowedRoot: h3Root, h3Defaults: {width: 720, reuse: 2}});
    assert.equal(publicSettings().videoProvider, 'h3');
    assert.equal(Object.hasOwn(publicSettings(), 'h3Executable'), false);
    assert.equal(publicSettings().h3ConfigSummary.executable.displayName.startsWith('…/'), true);
    assert.equal(JSON.stringify(publicSettings()).includes(h3Root), false);
    assert.equal(Object.hasOwn(publicSettings().h3Defaults, 'profile'), false);
    assert.deepEqual(h3Args({prompt: '已编译提示词'}, {h3ModelDir: '/private/models', h3Defaults: {width: 720, reuse: 2}}, '/private/output/video.mp4'), ['-d', '/private/models', '-p', '已编译提示词', '--width', '720', '--reuse', '2', '-o', '/private/output/video.mp4']);
    assert.equal(h3OutputFile({}, {h3OutputDir: '/private/output', h3AllowedRoot: '/private'}).startsWith('/private/output/'), true);
    assert.throws(() => h3OutputFile({outputPath: '/outside/video.mp4'}, {h3OutputDir: '/private/output', h3AllowedRoot: '/private'}), /路径无效/);
    assert.equal(leaseDurationForJob({payload_json: JSON.stringify({provider: 'h3'})}) > 90_000, true);
});

test('h3 save-time validation rejects invalid paths without persisting them', () => {
    const h3Root = join(dataDir, 'h3-validation-fixture');
    const modelDir = join(h3Root, 'MiniMax-H3');
    const outputDir = join(h3Root, 'outputs');
    mkdirSync(modelDir, {recursive: true});
    saveSettings({videoProvider: 'h3', h3Executable: process.execPath, h3ModelDir: modelDir, h3OutputDir: outputDir, h3AllowedRoot: h3Root});
    const before = database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json;
    assert.throws(() => saveSettings({h3Executable: 'h3.c'}), /绝对路径/);
    assert.equal(database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json, before);
    const nonExecutable = join(h3Root, 'not-executable');
    writeFileSync(nonExecutable, '#!/bin/sh\n');
    chmodSync(nonExecutable, 0o600);
    assert.throws(() => saveSettings({h3Executable: nonExecutable}), /执行权限/);
    assert.equal(database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json, before);
    assert.throws(() => saveSettings({h3ModelDir: join(h3Root, 'missing-model')}), /模型目录/);
    assert.equal(database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json, before);
    const blockedOutput = join(h3Root, 'blocked-output');
    writeFileSync(blockedOutput, 'not a directory');
    assert.throws(() => saveSettings({h3OutputDir: blockedOutput}), /输出目录/);
    assert.equal(database.prepare('SELECT payload_json FROM companion_settings WHERE id = 1').get().payload_json, before);
    assert.throws(() => validateH3Configuration({h3Executable: process.execPath, h3ModelDir: modelDir, h3OutputDir: join(h3Root, 'outside'), h3AllowedRoot: join(h3Root, 'allowed')}), /允许根目录/);
    const summary = h3ConfigSummary({h3Executable: '/private/secret/h3', h3ModelDir: '/private/secret/models', h3OutputDir: '/private/secret/outputs'});
    assert.equal(JSON.stringify(summary).includes('/private/secret'), false);
});

test('an active event overrides routine state consistently for UI and chat context', () => {
    const persona = createPersona({name: '状态', role: '学生', foundation: '状态会如实描述当前所在的位置。'});
    createEvent(requirePersona(persona.id), {type: 'study', situation: '正在图书馆整理笔记', mood: '专注', scene: '图书馆'}, {publish: false, source: 'test'});
    assert.equal(scheduledState(requirePersona(persona.id)).situation, '正在图书馆整理笔记');
    assert.equal(stateShape(persona.id).situation, '正在图书馆整理笔记');
    assert.match(contextFor(persona.id).prompt, /【当前真实状态】正在图书馆整理笔记/);
    assert.equal(resolvedStateFor(persona.id).situation, '正在图书馆整理笔记');
});

test('four layers keep an active schedule coherent for chat and media, and persist an AI daily-plan job', () => {
    const persona = createPersona({name: '计划', role: '学生', foundation: '计划会按自己的当天安排生活。'});
    const today = new Date();
    const startsAt = new Date(today.getTime() - 10 * 60_000).toISOString();
    const endsAt = new Date(today.getTime() + 50 * 60_000).toISOString();
    database.prepare('INSERT INTO companion_schedule_items (id, persona_id, kind, title, starts_at, ends_at, status, source, details_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run('schedule_live_plan', persona.id, 'daily_plan', '整理课程笔记', startsAt, endsAt, 'active', 'ai_daily_plan', JSON.stringify({scene: '图书馆自习区', situation: '正在图书馆整理课程笔记'}), startsAt, startsAt);
    const context = contextFor(persona.id);
    assert.match(context.layers.immutableIdentity, /不可变身份层/);
    assert.match(context.layers.lifeState, /正在图书馆整理课程笔记/);
    assert.match(context.layers.relationship, /人格私有关系层/);
    assert.match(context.layers.systemCapability, /系统能力层/);
    const media = createChatMediaRequest(persona.id, mediaCall('image', '拍一张现在的自然照片'));
    const envelope = JSON.parse(database.prepare('SELECT payload_json FROM companion_jobs WHERE id = ?').get(media.jobId).payload_json).envelope;
    assert.equal(envelope.facts.currentState.location, '图书馆自习区');
    assert.equal(envelope.facts.currentState.situation, '正在图书馆整理课程笔记');
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ? AND status = 'queued'").get(persona.id).count, 1);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count, 1);
});

test('a ready daily plan owns the pre-first-slot state and its trusted chat time facts', () => {
    const persona = createPersona({name: '苏芷柠', role: '学生', foundation: '苏芷柠住在学校附近，喜欢按自己的节奏休息和娱乐。'});
    const planDate = '2026-08-19';
    const planJson = [{title: '睡到自然醒，宿舍打游戏看番', scene: '宿舍', situation: '睡醒后在宿舍打游戏看番', startsAt: '10:00', endsAt: '13:00'}];
    const createdAt = new Date('2026-08-18T16:00:00.000Z').toISOString();
    const existingPlan = database.prepare('SELECT id, plan_date FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(persona.id);
    const planId = existingPlan?.id || `daily_plan_suzhinong_${persona.id}`;
    if (existingPlan) database.prepare("UPDATE companion_daily_plans SET plan_date = ?, status = 'ready', plan_json = ?, source = 'test', updated_at = ? WHERE id = ?").run(planDate, JSON.stringify(planJson), createdAt, planId);
    else database.prepare('INSERT INTO companion_daily_plans (id, persona_id, plan_date, status, plan_json, source, created_at, updated_at) VALUES (?, ?, ?, \'ready\', ?, \'test\', ?, ?)').run(planId, persona.id, planDate, JSON.stringify(planJson), createdAt, createdAt);

    const beforeFirstSlot = new Date('2026-08-19T00:47:00.000Z'); // 08:47 in Asia/Shanghai.
    const projected = scheduledState(requirePersona(persona.id), beforeFirstSlot);
    assert.equal(projected.source, 'daily_plan_baseline');
    assert.match(projected.situation, /睡|赖床|休息/);
    assert.doesNotMatch(projected.situation, /上课/);
    assert.equal(projected.startsAt, '2026-08-18T16:00:00.000Z');
    assert.equal(projected.endsAt, '2026-08-19T02:00:00.000Z');
    assert.equal(projected.timeFact, 'known');
    assert.equal(projected.location, '住处');
    assert.equal(projected.room, '自己的宿舍房间');

    const shape = stateShape(persona.id, beforeFirstSlot);
    assert.equal(shape.source.kind, 'daily_plan_baseline');
    assert.equal(shape.startsAt, projected.startsAt);
    assert.equal(shape.endsAt, projected.endsAt);
    assert.equal(shape.timeFact, 'known');
    assert.equal(shape.room, '自己的宿舍房间');

    const context = contextFor(persona.id, beforeFirstSlot);
    assert.match(context.layers.lifeState, /当前主状态来源：daily_plan_baseline/);
    assert.match(context.layers.lifeState, /可信结束时间：2026-08-19T02:00:00\.000Z/);
    assert.doesNotMatch(context.layers.lifeState, /稳定作息.*上课中/);
    assert.match(context.layers.lifeState, /当天计划已就绪/);
    assert.match(context.layers.lifeState, /正在睡眠状态/);
    const planSleep = sleepAvailability(requirePersona(persona.id), beforeFirstSlot, context.state);
    assert.equal(planSleep.sleeping, true);
    assert.equal(planSleep.nextBoundaryAt, '2026-08-19T02:00:00.000Z');
    assert.match(context.layers.systemCapability, /只有 timeFact=known/);
    const prompt = userVisibleChatPrompt(persona.id, '用户问：什么时候下课？', beforeFirstSlot);
    assert.match(prompt, /不得根据“学生”“上课”等身份猜测课程/);
    assert.match(prompt, /不得编造“十点半”等具体时间/);
    assert.doesNotMatch(prompt, /稳定作息.*上课中/);
    assert.match(trustedTimeReplyForMessage(requirePersona(persona.id), '啥时候下课呀宝宝', context.state), /我现在不在上课.*10:00/);
    const baselineEnvelope = mediaConceptEnvelopeFor(requirePersona(persona.id), {kind: 'image', trigger: 'test', at: beforeFirstSlot});
    assert.match(baselineEnvelope.facts.currentState.room, /自己的宿舍房间/);
    assert.match(baselineEnvelope.facts.currentState.situation, /睡觉|赖床/);

    const activeSlot = scheduledState(requirePersona(persona.id), new Date('2026-08-19T02:30:00.000Z'));
    assert.equal(activeSlot.source, 'daily_plan');
    assert.equal(activeSlot.startsAt, '2026-08-19T02:00:00.000Z');
    assert.equal(activeSlot.endsAt, '2026-08-19T05:00:00.000Z');
    assert.equal(activeSlot.timeFact, 'known');
    reconcilePersona(persona.id, {publish: false});
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_life_events WHERE persona_id = ? AND type IN ('daily_plan', 'daily_plan_baseline')").get(persona.id).count, 0);
});

test('partial explicit schedule overlays only its own interval and daily plans reject overlapping blocks', () => {
    const persona = createPersona({name: '局部冲突', role: '学生', foundation: '局部冲突按明确计划安排一天。'});
    const planDate = '2026-08-19';
    const existing = database.prepare('SELECT id FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(persona.id);
    database.prepare("UPDATE companion_daily_plans SET plan_date = ?, status = 'ready', plan_json = ?, source = 'test', updated_at = ? WHERE id = ?").run(
        planDate,
        JSON.stringify([{title: '宿舍打游戏', scene: '宿舍', situation: '在宿舍打游戏放松', startsAt: '10:00', endsAt: '13:00'}]),
        '2026-08-18T16:00:00.000Z',
        existing.id
    );
    database.prepare('INSERT INTO companion_schedule_items (id, persona_id, kind, title, starts_at, ends_at, status, source, details_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run(
        'schedule_partial_' + persona.id, persona.id, 'plan', '和用户确认的午间安排',
        '2026-08-19T03:00:00.000Z', '2026-08-19T04:00:00.000Z', 'active', 'explicit_chat_plan',
        JSON.stringify({scene: '校园咖啡馆', situation: '正在和用户确认的午间安排'}), '2026-08-18T16:00:00.000Z', '2026-08-18T16:00:00.000Z'
    );
    assert.equal(scheduledState(requirePersona(persona.id), new Date('2026-08-19T02:15:00.000Z')).source, 'daily_plan');
    const explicitState = scheduledState(requirePersona(persona.id), new Date('2026-08-19T03:30:00.000Z'));
    assert.equal(explicitState.source, 'schedule');
    assert.equal(explicitState.location, '校园咖啡馆');
    assert.equal(explicitState.room, '');
    assert.equal(scheduledState(requirePersona(persona.id), new Date('2026-08-19T04:15:00.000Z')).source, 'daily_plan');
    assert.equal(normalizeDailyPlan({items: [
        {title: '甲', scene: '宿舍', situation: '甲', startsAt: '10:00', endsAt: '12:00'},
        {title: '乙', scene: '图书馆', situation: '乙', startsAt: '11:00', endsAt: '13:00'}
    ]}, planDate), null);
});

test('generic rest baseline remains awake and unknown time facts do not claim a precise end time', () => {
    const persona = createPersona({name: '晨间休息', role: '学生', foundation: '晨间休息按自己的安排慢慢开始一天。'});
    const planDate = '2026-08-19';
    const plan = database.prepare('SELECT id FROM companion_daily_plans WHERE persona_id = ? ORDER BY plan_date DESC LIMIT 1').get(persona.id);
    database.prepare("UPDATE companion_daily_plans SET plan_date = ?, status = 'ready', plan_json = ?, source = 'test', updated_at = ? WHERE id = ?").run(
        planDate,
        JSON.stringify([{title: '晨间休息', scene: '宿舍', situation: '在宿舍休息一会儿', startsAt: '10:00', endsAt: '11:00'}]),
        '2026-08-18T16:00:00.000Z',
        plan.id
    );
    const at = new Date('2026-08-19T00:47:00.000Z');
    const context = contextFor(persona.id, at);
    assert.equal(context.state.resolved_source, 'daily_plan_baseline');
    assert.doesNotMatch(context.state.situation, /睡觉|赖床/);
    assert.equal(sleepAvailability(requirePersona(persona.id), at, context.state).sleeping, false);
    const unknownState = {...context.state, resolved_time_fact: 'unknown', resolved_ends_at: '2026-08-19T02:00:00.000Z'};
    const reply = trustedTimeReplyForMessage(requirePersona(persona.id), '啥时候下课呀？', unknownState);
    assert.doesNotMatch(reply, /10:00|十点/);
    assert.match(reply, /没有课程或可确认的结束时间/);
});

test('legacy blueprint reads receive an effective safe v2 room without a migration write', () => {
    const persona = createPersona({name: '旧设定', role: '学生', foundation: '旧设定有稳定的生活习惯。'});
    const original = database.prepare('SELECT blueprint_json FROM companion_persona_life_blueprints WHERE persona_id = ?').get(persona.id).blueprint_json;
    database.prepare('UPDATE companion_persona_life_blueprints SET blueprint_json = ?, updated_at = ? WHERE persona_id = ?').run(JSON.stringify({routine: [{label: '旧作息', from: 0, to: 24, scene: '旧房间'}], interests: ['旧兴趣']}), new Date().toISOString(), persona.id);

    const effective = publicBlueprint(persona.id);
    assert.equal(effective.schemaVersion, lifeModelSchemaVersion);
    assert.deepEqual(effective.world.defaultSceneRef, {locationId: 'home', roomId: 'private_room'});
    assert.equal(validateLifeBlueprint(normalizeLifeBlueprint(effective)).ok, true);
    assert.deepEqual(effective.interests, ['旧兴趣']);
    assert.deepEqual(JSON.parse(database.prepare('SELECT blueprint_json FROM companion_persona_life_blueprints WHERE persona_id = ?').get(persona.id).blueprint_json), {routine: [{label: '旧作息', from: 0, to: 24, scene: '旧房间'}], interests: ['旧兴趣']});
    assert.notEqual(original, database.prepare('SELECT blueprint_json FROM companion_persona_life_blueprints WHERE persona_id = ?').get(persona.id).blueprint_json);
    const legacyState = scheduledState(requirePersona(persona.id), new Date('2026-08-19T00:47:00.000Z'));
    assert.equal(legacyState.source, 'routine');
    assert.equal(legacyState.endsAt, null);
    assert.equal(legacyState.timeFact, 'unknown');
});

test('model-authorized media markers are transport-only and all producers persist a factual envelope', () => {
    const parsed = extractMediaIntent(`我晚点把这组街拍发给你。<media-intent>${JSON.stringify(mediaCall('image', '傍晚街拍', {count: 2}))}</media-intent>`);
    assert.equal(parsed.media.schemaVersion, 2);
    assert.equal(parsed.media.count, 2);
    assert.match(imagePromptMasterContract, /固定的生图模板/);
    assert.doesNotMatch(systemCapabilityMediaContract, /creativeDirection/);

    const persona = createPersona({name: '画面', role: '学生', foundation: '画面喜欢与朋友分享校园生活。'});
    const chat = createChatMediaRequest(persona.id, mediaCall('image', '河边自拍和两套衣服对比', {trigger: 'test'}));
    const chatPayload = JSON.parse(database.prepare('SELECT payload_json FROM companion_jobs WHERE id = ?').get(chat.jobId).payload_json);
    assert.equal(chatPayload.envelope.schemaVersion, 1);
    assert.equal(chatPayload.envelope.trigger, 'test');
    assert.equal(Object.hasOwn(chatPayload, 'prompt'), false);
    assert.equal(Object.hasOwn(chatPayload, 'mediaIntent'), false);
    assert.equal(Object.hasOwn(chat.message.generation, 'envelope'), false);

    const activityCall = mediaCall('image', '和朋友在公园聊天', {personaMediaConcept: mediaConcept('image', {scene: '公园', action: '和朋友聊天'})});
    const activity = createEvent(requirePersona(persona.id), {type: 'social', situation: '和朋友在公园聊天', mood: '轻松', scene: '公园', visual: true, mediaCapabilityCall: activityCall}, {publish: true, source: 'test'});
    const activityPayload = JSON.parse(database.prepare("SELECT payload_json FROM companion_jobs WHERE activity_id = ? AND job_type = 'activity_image' ORDER BY created_at DESC LIMIT 1").get(activity.activityId).payload_json);
    assert.equal(activityPayload.envelope.schemaVersion, 1);
    assert.equal(activityPayload.envelope.trigger, 'activity_event');
    assert.equal(Object.hasOwn(activityPayload, 'prompt'), false);
});

test('chat media jobs persist a placeholder and replace that exact message on completion', () => {
    const persona = createPersona({name: '沈青', role: '在读大学生', foundation: '沈青是摄影社成员，喜欢记录普通日常。'});
    const request = createChatMediaRequest(persona.id, mediaCall('image', '今天在校园里的自然照片'));
    assert.equal(request.message.generation.status, 'queued');
    assert.equal(request.message.generation.kind, 'image');
    assert.equal(request.message.attachments.length, 0);

    let job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const payload = JSON.parse(job.payload_json);
    assert.equal(payload.envelope.mediaKind, 'image');
    assert.equal(payload.envelope.request, '今天在校园里的自然照片');
    assert.equal(Object.hasOwn(payload, 'prompt'), false);
    const leaseOwner = 'lease_test_media';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), job.id);
    const completed = completePolledMediaJob({...job, lease_owner: leaseOwner}, 'comfy_prompt_1', [{filename: 'campus.png', subfolder: '', type: 'output', format: 'png'}]);
    assert.equal(completed.completed, true);

    const message = listMessages(persona.id, {markRead: false}).items.find(item => item.id === request.message.id);
    assert.equal(message.generation.status, 'ready');
    assert.equal(message.generation.kind, 'image');
    assert.equal(message.attachments.length, 1);
    assert.match(message.attachments[0].url, /^\/api\/companion\/media\//);
    assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'complete');
    assert.throws(() => createChatMediaRequest(persona.id, mediaCall('image,video', '非法媒体类型')), /媒体类型/);
});

test('frozen persona concept and prompt master preserve capture, people, and object semantics before provider submission', async () => {
    const persona = createPersona({name: '若岚', role: '摄影爱好者', foundation: '若岚是一位女性摄影爱好者，常带相机记录朋友。'});
    const request = createChatMediaRequest(persona.id, mediaCall('image', '给闺蜜拍照，旁边有两套衣服和一只小狗', {personaMediaConcept: {
        ...mediaConcept('image'), scene: '公园花墙前', action: '为闺蜜拍摄自然肖像', mood: '开心', narrative: '若岚在公园为闺蜜拍照。',
        humanSubjects: [{label: '闺蜜', role: '被摄主体', inFrame: true}, {label: '若岚', role: '摄影者', inFrame: false}],
        nonHumanObjects: [{label: '两套待比较的服装', kind: 'clothing', inFrame: true}, {label: '一只小狗', kind: 'animal', inFrame: true}],
        capture: {mode: 'operator_pov', operator: '若岚（画外摄影者）', deviceVisibility: 'out_of_frame', framingIntent: '从若岚的取景位置拍摄闺蜜的中景'}, compositionIntent: '闺蜜自然面对镜头站立。'
    }}));
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE id = ?").run('lease_model_media', new Date(Date.now() + 60_000).toISOString(), request.jobId);
    let job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const concept = JSON.parse(job.payload_json).personaMediaConcept;
    const template = {
        schemaVersion: 1,
        sections: {
            capture: '摄影者若岚位于画面外，镜头采用她的自然第一视角，拍摄设备不入画。',
            humanSubjects: '画面内唯一的人类主体是闺蜜，若岚不入镜。',
            identityAndContinuity: '闺蜜保持自然生活化外观与真实肤色。',
            sceneAndAction: '公园花墙前，闺蜜正在接受朋友拍摄。',
            wardrobeAndNonHumanProps: '两套待比较的服装和一只小狗都是非人物对象，按真实比例出现。',
            lightingAndMood: '柔和自然光，开心放松。',
            photographyStyleAndColor: '真实日常摄影，干净自然色彩。',
            constraints: '不要增加其他人类；服装和小狗不得被生成成人。'
        }
    };
    const originalFetch = globalThis.fetch;
    const previous = publicSettings();
    let providerCalls = 0;
    let providerPrompt = '';
    mediaProviders.set('fixture-image', {id: 'fixture-image', label: 'fixture', capabilities: ['image'], async submit({prompt}) { providerCalls += 1; providerPrompt = prompt; return {externalId: 'fixture_media', pending: false, files: [{filename: 'fixture.png', type: 'output'}]}; }});
    saveSettings({imageProvider: 'fixture-image', model: 'fixture-model'});
    const fixturePayload = {...JSON.parse(job.payload_json), provider: 'fixture-image'};
    database.prepare('UPDATE companion_jobs SET payload_json = ? WHERE id = ?').run(JSON.stringify(fixturePayload), job.id);
    job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const responses = [JSON.stringify(template)];
    globalThis.fetch = async () => ({ok: true, json: async () => ({choices: [{message: {content: responses.shift()}}]})});
    try {
        await submitMediaJob(job);
    } finally {
        globalThis.fetch = originalFetch;
        mediaProviders.delete('fixture-image');
        saveSettings({imageProvider: previous.imageProvider || 'comfyui', model: previous.model || ''});
    }
    const settled = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const result = JSON.parse(settled.result_json);
    assert.equal(settled.status, 'complete', settled.error);
    assert.equal(providerCalls, 1);
    assert.deepEqual(result.personaConcept.humanSubjects.map(item => [item.label, item.inFrame]), [['闺蜜', true], ['若岚', false]]);
    assert.deepEqual(result.personaConcept.nonHumanObjects.map(item => item.label), ['两套待比较的服装', '一只小狗']);
    assert.equal(result.promptTemplate.sections.capture, template.sections.capture);
    assert.equal(providerPrompt, result.finalPrompt);
    assert.match(result.finalPrompt, /画面内唯一的人类主体是闺蜜/);
    assert.match(result.finalPrompt, /服装和小狗不得被生成成人/);
    assert.doesNotMatch(result.finalPrompt, /共\s*\d+\s*人/);
});

test('legacy job without frozen concept fails without concept LLM, B, or provider fallback', async () => {
    const persona = createPersona({name: '失败边界', role: '学生', foundation: '失败边界会如实处理生成失败。'});
    const request = createChatMediaRequest(persona.id, mediaCall('image', '一张普通照片'));
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1, max_attempts = 2 WHERE id = ?").run('lease_bad_concept_one', new Date(Date.now() + 60_000).toISOString(), request.jobId);
    let job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const originalFetch = globalThis.fetch;
    let providerCalls = 0;
    mediaProviders.set('never-submit', {id: 'never-submit', label: 'never', capabilities: ['image'], async submit() { providerCalls += 1; return {externalId: 'never', pending: true}; }});
    saveSettings({imageProvider: 'never-submit'});
    const legacyPayload = JSON.parse(job.payload_json);
    delete legacyPayload.personaMediaConcept;
    database.prepare('UPDATE companion_jobs SET payload_json = ? WHERE id = ?').run(JSON.stringify(legacyPayload), request.jobId);
    job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    globalThis.fetch = async () => { throw new Error('concept B must not be called'); };
    try {
        await submitMediaJob(job);
    } finally {
        globalThis.fetch = originalFetch;
        mediaProviders.delete('never-submit');
        saveSettings({imageProvider: 'comfyui'});
    }
    const settled = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const result = JSON.parse(settled.result_json);
    const message = listMessages(persona.id, {markRead: false}).items.find(item => item.id === request.message.id);
    assert.equal(providerCalls, 0);
    assert.equal(settled.status, 'failed');
    assert.equal(result.failedStage, 'missing_frozen_media_concept');
    assert.equal(result.migrationFailure, 'missing_frozen_media_concept');
    assert.equal(message.generation.status, 'failed');
    assert.equal(Object.hasOwn(result, 'finalPrompt'), false);
});

test('C acceptance gates pass, one retry, reject, and infrastructure skip', async () => {
    const previous = publicSettings();
    const originalFetch = globalThis.fetch;
    const fixtureId = 'fixture-acceptance';
    const bytes = Buffer.from('fixture-image-bytes');
    mediaProviders.set(fixtureId, {
        id: fixtureId, label: 'fixture acceptance', capabilities: ['image'],
        async readCandidate() { return {bytes, mimeType: 'image/png'}; }
    });
    saveSettings({imageProvider: fixtureId, model: 'fixture-acceptance-model'});
    const lease = request => {
        database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE id = ?")
            .run(`lease_${request.jobId}`, new Date(Date.now() + 60_000).toISOString(), request.jobId);
        return database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    };
    const verdict = (value, violations = []) => JSON.stringify({schemaVersion: 1, verdict: value, violations, observedFacts: {sceneMatches: value === 'pass'}, retryGuidance: value === 'retry' ? '保持冻结的场景与镜头关系。' : ''});
    try {
        const passPersona = createPersona({name: '验收通过', role: '学生', foundation: '验收通过。'});
        const passRequest = createChatMediaRequest(passPersona.id, mediaCall('image', '通过图片'));
        globalThis.fetch = async () => ({ok: true, json: async () => ({choices: [{message: {content: verdict('pass')}}]})});
        await completeGeneratedMedia(lease(passRequest), 'pass-prompt', [{filename: 'pass.png', type: 'output'}], fixtureId);
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(passRequest.jobId).status, 'complete');
        assert.equal(listMessages(passPersona.id, {markRead: false}).items.find(item => item.id === passRequest.message.id).generation.status, 'ready');

        const retryPersona = createPersona({name: '验收重试', role: '学生', foundation: '验收重试。'});
        const retryRequest = createChatMediaRequest(retryPersona.id, mediaCall('image', '重试图片'));
        globalThis.fetch = async () => ({ok: true, json: async () => ({choices: [{message: {content: verdict('retry', [{code: 'scene_mismatch', severity: 'hard', detail: '场景不符'}])}}]})});
        await completeGeneratedMedia(lease(retryRequest), 'retry-prompt', [{filename: 'retry.png', type: 'output'}], fixtureId);
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(retryRequest.jobId).status, 'complete');
        assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'chat_image' AND status = 'queued'").get(retryPersona.id).count, 1);
        assert.equal(listMessages(retryPersona.id, {markRead: false}).items.find(item => item.id === retryRequest.message.id).generation.status, 'queued');

        const rejectPersona = createPersona({name: '验收拒绝', role: '学生', foundation: '验收拒绝。'});
        const rejectRequest = createChatMediaRequest(rejectPersona.id, mediaCall('image', '拒绝图片'));
        globalThis.fetch = async () => ({ok: true, json: async () => ({choices: [{message: {content: verdict('reject', [{code: 'unsafe', severity: 'hard', detail: '安全问题'}])}}]})});
        await completeGeneratedMedia(lease(rejectRequest), 'reject-prompt', [{filename: 'reject.png', type: 'output'}], fixtureId);
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(rejectRequest.jobId).status, 'failed');
        assert.equal(listMessages(rejectPersona.id, {markRead: false}).items.find(item => item.id === rejectRequest.message.id).generation.status, 'failed');
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_media_assets WHERE filename = ?').get('reject.png').count, 0);

        const skippedPersona = createPersona({name: '验收降级', role: '学生', foundation: '验收降级。'});
        const skippedRequest = createChatMediaRequest(skippedPersona.id, mediaCall('image', '降级图片'));
        mediaProviders.get(fixtureId).readCandidate = async () => null;
        let acceptanceCalls = 0;
        globalThis.fetch = async () => { acceptanceCalls += 1; throw new Error('C must be skipped before model call'); };
        await completeGeneratedMedia(lease(skippedRequest), 'skipped-prompt', [{filename: 'skipped.png', type: 'output'}], fixtureId);
        assert.equal(acceptanceCalls, 0);
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(skippedRequest.jobId).status, 'complete');
        const skippedResult = JSON.parse(database.prepare('SELECT result_json FROM companion_jobs WHERE id = ?').get(skippedRequest.jobId).result_json);
        assert.equal(skippedResult.acceptance.at(-1).verdict, 'skipped');
    } finally {
        globalThis.fetch = originalFetch;
        mediaProviders.delete(fixtureId);
        saveSettings({imageProvider: previous.imageProvider || 'comfyui', model: previous.model || ''});
    }
});

test('debug context is redacted, bounded, and persona-scoped', () => {
    assert.equal(debugInspectorEnabled, false);
    const first = createPersona({name: '调试一号', role: '学生', foundation: '调试一号喜欢记录日常。'});
    const second = createPersona({name: '调试二号', role: '编辑', foundation: '调试二号喜欢收集旧书。'});
    appendMessage(first.id, {role: 'user', text: `Bearer secret-token-${'x'.repeat(80)} ${'a'.repeat(2600)}`});
    appendMessage(second.id, {role: 'user', text: '这条内容不能泄露到另一人格。'});
    const firstMedia = createChatMediaRequest(first.id, mediaCall('image', `apiKey=super-secret ${'p'.repeat(2600)}`));
    const secondMedia = createChatMediaRequest(second.id, mediaCall('video', '只属于第二人格的媒体意图'));

    const context = debugContextFor(first.id);
    assert.equal(context.recentRequests.length <= 10, true);
    assert.equal(context.mediaJobs.length <= 10, true);
    assert.equal(context.recentRequests.some(item => item.promptSummary.includes('super-secret')), false);
    assert.equal(context.recentRequests.some(item => item.promptSummary.includes('另一人格')), false);
    assert.equal(context.mediaJobs.some(item => item.id === secondMedia.jobId), false);
    assert.equal(context.mediaJobs.some(item => item.id === firstMedia.jobId), true);
    assert.equal(context.mediaJobs[0].promptSummary.length <= 2000, true);
    assert.equal(context.mediaJobs[0].envelope.length <= 2000, true);
    assert.equal(Object.hasOwn(context.mediaJobs[0], 'personaConcept'), true);
    assert.equal(Object.hasOwn(context.mediaJobs[0], 'promptTemplate'), true);
    assert.deepEqual(redactDebugValue({authorization: 'Bearer abc', nested: {apiKey: 'secret'}}), {authorization: '[redacted]', nested: {apiKey: '[redacted]'}});
    assert.equal(debugSummary(`Bearer abcdefghijklmnopqrstuvwxyz ${'z'.repeat(2100)}`).includes('abcdefghijklmnopqrstuvwxyz'), false);
});

test('debug routes are not registered unless the explicit local flag is enabled', async () => {
    assert.equal(routePaths(companionApp).some(path => String(path).includes('debug-context')), false);
    assert.equal(routePaths(companionApp).some(path => String(path).includes('debug-media')), false);
    assert.equal(routePaths(companionApp).some(path => String(path).includes('h3-preflight')), false);

    const debugDataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-debug-test-'));
    const originalDataDir = process.env.DATA_DIR;
    const originalFlag = process.env.COMPANION_DEBUG_INSPECTOR;
    try {
        process.env.DATA_DIR = debugDataDir;
        delete process.env.COMPANION_DEBUG_INSPECTOR;
        const unsetFlagModule = await import(`../server.js?debug-unset=${Date.now()}`);
        assert.equal(unsetFlagModule.companionTestHooks.debugInspectorEnabled, false);
        assert.equal(routePaths(unsetFlagModule.companionApp).some(path => String(path).includes('debug-context')), false);
        assert.equal(routePaths(unsetFlagModule.companionApp).some(path => String(path).includes('h3-preflight')), false);

        process.env.COMPANION_DEBUG_INSPECTOR = '1';
        const debugModule = await import(`../server.js?debug=${Date.now()}`);
        const persona = debugModule.companionTestHooks.createPersona({name: '本地检查', role: '开发测试', foundation: '只在本地检查器中查看。'});
        assert.equal(debugModule.companionTestHooks.debugInspectorEnabled, true);
        assert.equal(routePaths(debugModule.companionApp).includes('/api/companion/personas/:personaId/debug-context'), true);
        assert.equal(routePaths(debugModule.companionApp).includes('/api/companion/personas/:personaId/debug-media'), true);
        assert.equal(routePaths(debugModule.companionApp).includes('/api/companion/h3-preflight'), true);
        const context = debugModule.companionTestHooks.debugContextFor(persona.id);
        assert.equal(context.layers.identity.includes('本地检查'), true);
        const h3Root = join(debugDataDir, 'h3-preflight-fixture');
        const modelDir = join(h3Root, 'MiniMax-H3');
        const executable = join(h3Root, 'h3-preflight-fixture.sh');
        mkdirSync(modelDir, {recursive: true});
        writeFileSync(executable, `#!/bin/sh\nprintf 'first ${h3Root}\\nsecond\\nthird\\nfourth\\nfifth\\n'\n`);
        chmodSync(executable, 0o700);
        debugModule.companionTestHooks.saveSettings({videoProvider: 'h3', h3Executable: executable, h3ModelDir: modelDir, h3OutputDir: join(h3Root, 'outputs'), h3AllowedRoot: h3Root});
        const jobsBefore = debugModule.companionTestHooks.database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count;
        const assetsBefore = debugModule.companionTestHooks.database.prepare('SELECT COUNT(*) AS count FROM companion_media_assets').get().count;
        const preflight = await debugModule.companionTestHooks.h3Preflight();
        assert.equal(preflight.ok, true);
        assert.equal(preflight.process.started, true);
        assert.equal(preflight.process.output.length <= 4, true);
        assert.equal(JSON.stringify(preflight).includes(h3Root), false);
        assert.equal(debugModule.companionTestHooks.database.prepare('SELECT COUNT(*) AS count FROM companion_jobs').get().count, jobsBefore);
        assert.equal(debugModule.companionTestHooks.database.prepare('SELECT COUNT(*) AS count FROM companion_media_assets').get().count, assetsBefore);
        const dispatch = debugModule.companionTestHooks.createChatMediaRequest(persona.id, mediaCall('image', '本地测试图片'));
        assert.equal(dispatch.message.generation.status, 'queued');
    } finally {
        if (originalDataDir === undefined) delete process.env.DATA_DIR; else process.env.DATA_DIR = originalDataDir;
        if (originalFlag === undefined) delete process.env.COMPANION_DEBUG_INSPECTOR; else process.env.COMPANION_DEBUG_INSPECTOR = originalFlag;
        rmSync(debugDataDir, {recursive: true, force: true});
    }
});

test('foundation recovery creates a new immutable revision instead of rewriting history', () => {
    const persona = createPersona({name: '苏遥', role: '学生', foundation: '苏遥最初喜欢安静地画画。'});
    const original = database.prepare('SELECT * FROM companion_persona_foundation_revisions WHERE persona_id = ? AND version = 1').get(persona.id);
    const changedAt = new Date().toISOString();
    database.prepare('INSERT INTO companion_persona_foundation_revisions (id, persona_id, version, foundation, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)').run('foundation_test_changed', persona.id, 2, '苏遥后来更喜欢热闹的派对。', '测试修改', changedAt);

    const restored = restoreFoundationRevision(persona.id, original.id);
    assert.equal(restored.restored, true);
    assert.equal(restored.version, 3);
    assert.equal(restored.foundation, original.foundation);
    assert.equal(database.prepare('SELECT foundation FROM companion_persona_foundation_revisions WHERE id = ?').get('foundation_test_changed').foundation, '苏遥后来更喜欢热闹的派对。');
});

test('two personas keep activity, messages, state, and media jobs isolated', () => {
    const first = createPersona({name: '乔安', role: '学生', foundation: '乔安喜欢在校园里拍照。'});
    const second = createPersona({name: '闻溪', role: '编辑', foundation: '闻溪喜欢逛旧书店。'});

    const firstEvent = createEvent(requirePersona(first.id), {
        type: 'study', situation: '在图书馆整理摄影笔记', mood: '专注', scene: '图书馆', content: '把下午拍的照片都整理好了。'
    }, {publish: true, source: 'test'});
    const secondEvent = createEvent(requirePersona(second.id), {
        type: 'shopping', situation: '在旧书店挑书', mood: '愉快', scene: '旧书店', content: '找到一本很喜欢的散文集。'
    }, {publish: true, source: 'test'});
    const firstMedia = createChatMediaRequest(first.id, mediaCall('image', '图书馆里的自然照片'));
    const secondMedia = createChatMediaRequest(second.id, mediaCall('video', '旧书店里翻书的短视频'));

    assert.deepEqual(listActivities({personaId: first.id, limit: 20}).items.map(item => item.id), [firstEvent.activityId]);
    assert.deepEqual(listActivities({personaId: second.id, limit: 20}).items.map(item => item.id), [secondEvent.activityId]);
    assert.equal(listMessages(first.id, {markRead: false}).items.some(item => item.id === secondMedia.message.id), false);
    assert.equal(listMessages(second.id, {markRead: false}).items.some(item => item.id === firstMedia.message.id), false);
    assert.equal(stateFor(first.id).situation, '在图书馆整理摄影笔记');
    assert.equal(stateFor(second.id).situation, '在旧书店挑书');
    assert.equal(database.prepare('SELECT persona_id FROM companion_jobs WHERE id = ?').get(firstMedia.jobId).persona_id, first.id);
    assert.equal(database.prepare('SELECT persona_id FROM companion_jobs WHERE id = ?').get(secondMedia.jobId).persona_id, second.id);
});

test('permanently deleting a test persona removes its private rows without touching another persona', () => {
    const removable = createPersona({name: '待删除', role: '测试人格', foundation: '这条人格将被彻底删除。'});
    const survivor = createPersona({name: '保留', role: '测试人格', foundation: '这条人格必须保留。'});
    appendMessage(removable.id, {role: 'user', text: '需要清理这段测试对话。'});
    const event = createEvent(requirePersona(removable.id), {type: 'shopping', situation: '测试商店', mood: '平静', scene: '测试场景', content: '用于删除测试的动态。'}, {publish: true, source: 'test'});
    const media = createChatMediaRequest(removable.id, mediaCall('image', '删除测试图片'));
    appendMessage(survivor.id, {role: 'user', text: '这段对话必须保留。'});

    const result = deletePersona(removable.id);
    assert.equal(result.deleted, true);
    assert.throws(() => requirePersona(removable.id), /人格不存在/);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ?').get(removable.id).count, 0);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_activities WHERE persona_id = ?').get(removable.id).count, 0);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_conversations WHERE persona_id = ?').get(removable.id).count, 0);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_persona_foundation_revisions WHERE persona_id = ?').get(removable.id).count, 0);
    for (const table of ['companion_persona_life_blueprint_revisions', 'companion_timeline_slots', 'companion_event_decisions', 'companion_event_links', 'companion_chat_deferred_batches']) {
        assert.equal(database.prepare(`SELECT COUNT(*) AS count FROM ${table} WHERE persona_id = ?`).get(removable.id).count, 0);
    }
    assert.equal(listActivities({limit: 100}).items.some(item => item.id === event.activityId), false);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE id = ?').get(media.message.id).count, 0);
    assert.deepEqual(listMessages(survivor.id, {markRead: false}).items.map(message => message.text), ['这段对话必须保留。']);
    assert.equal(requirePersona(survivor.id).name, '保留');
});

test('screened personas suppress proactive unread messages without affecting other personas', () => {
    const visible = createPersona({name: '陈岚', role: '学生', foundation: '陈岚喜欢和朋友分享校园生活。'});
    const screened = createPersona({name: '陆遥', role: '学生', foundation: '陆遥习惯安静地记录日常。'});
    const visiblePersona = requirePersona(visible.id);
    const visibleUser = appendMessage(visible.id, {role: 'user', text: '今天还好吗？'});
    database.prepare('UPDATE companion_messages SET created_at = ? WHERE id = ?').run(new Date(Date.now() - 11 * 60 * 1000).toISOString(), visibleUser.id);

    const visibleEvent = createEvent(visiblePersona, {
        type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路', content: '晚风很舒服。', proactiveText: '刚散完步，想和你说一声。'
    }, {publish: false, proactive: true, source: 'test'});
    const visibleJob = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(visible.id);
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run('lease_visible_proactive', new Date(Date.now() + 60_000).toISOString(), visibleJob.id);
    assert.equal(completeProactiveMessageJob({...visibleJob, lease_owner: 'lease_visible_proactive'}, '刚散完步，想和你说一声。').completed, true);
    const visibleUnread = database.prepare(`
        SELECT COUNT(*) AS count FROM companion_messages messages
        JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
        WHERE conversations.persona_id = ? AND messages.role = 'assistant' AND messages.read_at IS NULL
    `).get(visible.id).count;
    assert.equal(visibleUnread, 1);

    database.prepare('UPDATE companion_personas SET screened_at = ? WHERE id = ?').run(new Date().toISOString(), screened.id);
    const screenedEvent = createEvent(requirePersona(screened.id), {
        type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路', content: '晚风很舒服。', proactiveText: '刚散完步，想和你说一声。'
    }, {publish: false, proactive: true, source: 'test'});
    const screenedJob = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(screened.id);
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run('lease_screened_proactive', new Date(Date.now() + 60_000).toISOString(), screenedJob.id);
    const screenedCompletion = completeProactiveMessageJob({...screenedJob, lease_owner: 'lease_screened_proactive'}, '刚散完步，想和你说一声。');
    assert.equal(screenedCompletion.result.skipped, 'screened');
    assert.ok(visibleEvent.eventId);
    assert.ok(screenedEvent.eventId);

    assert.equal(database.prepare(`
        SELECT COUNT(*) AS count FROM companion_messages messages
        JOIN companion_conversations conversations ON conversations.id = messages.conversation_id
        WHERE conversations.persona_id = ? AND messages.role = 'assistant'
    `).get(screened.id).count, 0);
    assert.equal(requirePersona(visible.id).screened_at, null);
    assert.ok(requirePersona(screened.id).screened_at);
    createEvent(requirePersona(screened.id), {type: 'shopping', situation: '在书店挑明信片', mood: '愉快', scene: '书店', content: '刚挑好一张明信片。'}, {publish: true, source: 'test'});
    assert.equal(listActivities({personaId: screened.id, limit: 20}).items.length, 0);
    assert.equal(listMessages(visible.id, {markRead: false}).items.length, 2);
    assert.equal(listMessages(screened.id, {markRead: false}).items.length, 0);
});

test('proactive delivery uses focus, screen, budget, and lease guards', () => {
    const persona = createPersona({name: '乔宁', role: '学生', foundation: '乔宁喜欢和熟悉的人分享校园日常。'});
    const source = requirePersona(persona.id);
    assert.equal(personaFocusTier(source), 'idle');
    assert.equal(proactiveEligibility(source, {eventType: 'social'}).reason, 'not_recently_engaged');
    appendMessage(persona.id, {role: 'user', text: '下午见。'});
    assert.equal(personaFocusTier(requirePersona(persona.id)), 'active');
    assert.equal(proactiveEligibility(requirePersona(persona.id), {eventType: 'mild_setback'}).reason, 'active_chat');

    database.prepare('UPDATE companion_messages SET created_at = ? WHERE conversation_id = (SELECT id FROM companion_conversations WHERE persona_id = ?)').run(new Date(Date.now() - 11 * 60 * 1000).toISOString(), persona.id);
    assert.equal(proactiveEligibility(requirePersona(persona.id), {eventType: 'mild_setback'}).allowed, true);

    createEvent(requirePersona(persona.id), {type: 'mild_setback', situation: '因小插曲有点低落', mood: '低落', scene: '校园', proactiveText: '想和你说说今天的小插曲。'}, {proactive: true, source: 'test'});
    const job = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(persona.id);
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run('lease_proactive_once', new Date(Date.now() + 60_000).toISOString(), job.id);
    const stale = completeProactiveMessageJob({...job, lease_owner: 'stale_lease'}, '不会写入');
    assert.equal(stale.completed, false);
    database.prepare('UPDATE companion_jobs SET lease_expires_at = ? WHERE id = ?').run(new Date(Date.now() - 1_000).toISOString(), job.id);
    assert.equal(completeProactiveMessageJob({...job, lease_owner: 'lease_proactive_once'}, '过期租约').completed, false);
    database.prepare('UPDATE companion_jobs SET lease_expires_at = ? WHERE id = ?').run(new Date(Date.now() + 60_000).toISOString(), job.id);
    const delivered = completeProactiveMessageJob({...job, lease_owner: 'lease_proactive_once'}, '想和你说说今天的小插曲。晚点再和你聊。');
    assert.equal(delivered.completed, true);
    assert.deepEqual(delivered.result.messageIds.length, 2);
    assert.deepEqual(listMessages(persona.id, {markRead: false}).items.filter(message => message.proactiveEventId).map(message => message.text), ['想和你说说今天的小插曲。', '晚点再和你聊。']);
    assert.equal(completeProactiveMessageJob({...job, lease_owner: 'lease_proactive_once'}, '重复消息').completed, false);

    database.prepare('UPDATE companion_personas SET screened_at = ? WHERE id = ?').run(new Date().toISOString(), persona.id);
    assert.equal(proactiveEligibility(requirePersona(persona.id), {eventType: 'social'}).reason, 'screened');
});

test('pending-event markers are strict, bounded, and deduplicated into one durable job', () => {
    const persona = createPersona({name: '闻夏', role: '学生', foundation: '闻夏会记得朋友重要的日子。'});
    const source = appendMessage(persona.id, {role: 'user', text: '我下午要去面试。'});
    const notBefore = new Date(Date.now() + 60_000).toISOString();
    const expiresAt = new Date(Date.now() + 2 * 60 * 60_000).toISOString();
    const call = {schemaVersion: 1, summary: '下午面试结束后可以问问结果', notBefore, expiresAt, dedupeKey: '下午面试'};
    assert.match(systemCapabilityPendingEventContract, /<pending-event>/);
    assert.throws(() => normalizePendingEventCall({...call, expiresAt: '2026-08-19T12:00:00'}), /带时区/);
    assert.equal(extractPendingEventIntent(`普通回复。<pending-event>${JSON.stringify(call)}</pending-event>`).text, '普通回复。');
    const first = createPendingEvent(persona.id, call, source.id);
    const second = createPendingEvent(persona.id, call, source.id);
    assert.equal(first.created, true);
    assert.equal(second.created, false);
    assert.equal(first.pendingEvent.id, second.pendingEvent.id);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_pending_events WHERE persona_id = ?").get(persona.id).count, 1);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'pending_event'").get(persona.id).count, 1);
    const oversized = `<pending-event>${'x'.repeat(2_000)}</pending-event>`;
    assert.equal(extractPendingEventIntent(`正常文字。${oversized}`).text, '正常文字。');
    assert.equal(extractPendingEventIntent('<pending-event>{"schemaVersion":1').text, '');
});

test('stream marker redactor never emits internal capability tags', () => {
    const redactor = createVisibleMarkerRedactor();
    const chunks = [
        '先说一句。<pend',
        'ing-event>{"schemaVersion":1}</pending-event>然后继续。',
        '<media-intent>{"kind":"image"}</media-intent>结束。'
    ];
    const visible = chunks.map(chunk => redactor.push(chunk)).join('') + redactor.flush();
    assert.equal(visible, '先说一句。然后继续。结束。');
    assert.equal(/<\/?(?:pending-event|media-intent)/i.test(visible), false);
});

test('pending-event due worker evaluates current chat context and can intervene during active chat', async () => {
    const persona = createPersona({name: '沈宁', role: '学生', foundation: '沈宁会在重要事情之后认真关心朋友。'});
    const source = appendMessage(persona.id, {role: 'user', text: '我刚结束面试，现在有点紧张。'});
    const call = {
        schemaVersion: 1,
        summary: '面试结束后关心用户的感受',
        notBefore: new Date(Date.now() + 60_000).toISOString(),
        expiresAt: new Date(Date.now() + 2 * 60 * 60_000).toISOString(),
        dedupeKey: '面试结束关心'
    };
    const created = createPendingEvent(persona.id, call, source.id);
    const dueAt = new Date(Date.now() - 1_000).toISOString();
    const expiresAt = new Date(Date.now() + 60 * 60_000).toISOString();
    database.prepare('UPDATE companion_pending_events SET not_before = ?, expires_at = ? WHERE id = ?').run(dueAt, expiresAt, created.pendingEvent.id);
    const job = database.prepare("SELECT * FROM companion_jobs WHERE id = ? AND job_type = 'pending_event'").get(created.jobId);
    const leaseOwner = 'lease_pending_due';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, run_after = ? WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), dueAt, job.id);

    const previousSettings = publicSettings();
    const previousFetch = globalThis.fetch;
    let calls = 0;
    saveSettings({model: 'pending-test-model'});
    globalThis.fetch = async (_url, request) => {
        calls += 1;
        const body = JSON.parse(request.body);
        assert.equal(body.stream, false);
        assert.match(body.messages[0].content, /主动私聊结构化决策/);
        assert.match(body.messages[1].content, /面试结束后关心/);
        return new Response(JSON.stringify({choices: [{message: {content: JSON.stringify({schemaVersion: 1, send: true, reason: '当前聊天适合自然关心', message: '面试结束了吗？现在感觉怎么样？'})}}]}), {status: 200, headers: {'content-type': 'application/json'}});
    };
    try {
        const result = await runPendingEventJob({...job, lease_owner: leaseOwner});
        assert.equal(result.completed, true);
        assert.equal(calls, 1);
        assert.equal(database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'consumed');
        const message = listMessages(persona.id, {markRead: false}).items.find(item => item.proactivePendingEventId === created.pendingEvent.id);
        assert.equal(message.text, '面试结束了吗？');
    } finally {
        globalThis.fetch = previousFetch;
        saveSettings({model: previousSettings.model || ''});
    }
});

test('expired pending events finish without an LLM call', async () => {
    const persona = createPersona({name: '顾遥', role: '学生', foundation: '顾遥不会错过已经过期的提醒。'});
    const source = appendMessage(persona.id, {role: 'user', text: '下周有一件事。'});
    const created = createPendingEvent(persona.id, {
        schemaVersion: 1,
        summary: '过期后不再跟进',
        notBefore: new Date(Date.now() + 60_000).toISOString(),
        expiresAt: new Date(Date.now() + 2 * 60 * 60_000).toISOString(),
        dedupeKey: '过期提醒'
    }, source.id);
    const past = new Date(Date.now() - 60_000).toISOString();
    database.prepare('UPDATE companion_pending_events SET not_before = ?, expires_at = ? WHERE id = ?').run(past, past, created.pendingEvent.id);
    const job = database.prepare("SELECT * FROM companion_jobs WHERE id = ? AND job_type = 'pending_event'").get(created.jobId);
    const leaseOwner = 'lease_pending_expired';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, run_after = ? WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), past, job.id);
    const previousFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = async () => { calls += 1; throw new Error('expired event must not call model'); };
    try {
        const result = await runPendingEventJob({...job, lease_owner: leaseOwner});
        assert.equal(result.status, 'complete');
        assert.equal(calls, 0);
        assert.equal(database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'expired');
    } finally {
        globalThis.fetch = previousFetch;
    }
});

test('terminal pending-event evaluation failure closes the pending lifecycle', async () => {
    const persona = createPersona({name: '苏禾', role: '学生', foundation: '苏禾会谨慎处理未完成的提醒。'});
    const source = appendMessage(persona.id, {role: 'user', text: '我有一件稍后再说的事。'});
    const created = createPendingEvent(persona.id, {
        schemaVersion: 1,
        summary: '模型失败时也要结束生命周期',
        notBefore: new Date(Date.now() + 60_000).toISOString(),
        expiresAt: new Date(Date.now() + 60 * 60_000).toISOString(),
        dedupeKey: '模型失败生命周期'
    }, source.id);
    const dueAt = new Date(Date.now() - 1_000).toISOString();
    database.prepare('UPDATE companion_pending_events SET not_before = ? WHERE id = ?').run(dueAt, created.pendingEvent.id);
    const previousFetch = globalThis.fetch;
    globalThis.fetch = async () => new Response(JSON.stringify({choices: [{message: {content: '不是 JSON'}}]}), {status: 200, headers: {'content-type': 'application/json'}});
    try {
        for (let attempt = 1; attempt <= 4; attempt += 1) {
            const owner = `lease_pending_failure_${attempt}`;
            database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, run_after = ?, attempt_count = ? WHERE id = ?").run(owner, new Date(Date.now() + 60_000).toISOString(), dueAt, attempt, created.jobId);
            const job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(created.jobId);
            await runPendingEventJob({...job, lease_owner: owner});
        }
        assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(created.jobId).status, 'failed');
        assert.equal(database.prepare('SELECT status FROM companion_pending_events WHERE id = ?').get(created.pendingEvent.id).status, 'cancelled');
        assert.equal(listMessages(persona.id, {markRead: false}).items.some(message => message.proactivePendingEventId === created.pendingEvent.id), false);
    } finally {
        globalThis.fetch = previousFetch;
    }
});

test('life-event proactive delivery is suppressed while the user is actively chatting', () => {
    const persona = createPersona({name: '许安', role: '学生', foundation: '许安只在合适的时候打扰朋友。'});
    appendMessage(persona.id, {role: 'user', text: '我正在和你聊天。'});
    const event = createEvent(requirePersona(persona.id), {type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路'}, {proactive: true, source: 'test'});
    assert.ok(event.eventId);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message'").get(persona.id).count, 0);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_life_events WHERE id = ?').get(event.eventId).count, 1);
});

test('life-event worker rechecks active chat before making its LLM call', async () => {
    const persona = createPersona({name: '许真', role: '学生', foundation: '许真不会在用户正在说话时另起话题。'});
    const oldMessage = appendMessage(persona.id, {role: 'user', text: '刚刚聊完。'});
    database.prepare('UPDATE companion_messages SET created_at = ? WHERE id = ?').run(new Date(Date.now() - 11 * 60 * 1000).toISOString(), oldMessage.id);
    const event = createEvent(requirePersona(persona.id), {type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路'}, {proactive: true, source: 'test'});
    const job = database.prepare("SELECT * FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(persona.id);
    appendMessage(persona.id, {role: 'user', text: '现在继续聊天。'});
    const owner = 'lease_life_event_active_race';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run(owner, new Date(Date.now() + 60_000).toISOString(), job.id);
    const previousFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = async () => { calls += 1; throw new Error('active life event must not call model'); };
    try {
        const result = await companionTestHooks.runProactiveMessageJob({...job, lease_owner: owner});
        assert.equal(result.result.skipped, 'active_chat');
        assert.equal(calls, 0);
        assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_messages WHERE proactive_event_id = ?').get(event.eventId).count, 0);
    } finally {
        globalThis.fetch = previousFetch;
    }
});

test('proactive decisions are frozen before delivery retries', () => {
    const persona = createPersona({name: '林澄', role: '学生', foundation: '林澄会把重要的关心说得自然。'});
    const source = appendMessage(persona.id, {role: 'user', text: '今天见。'});
    database.prepare('UPDATE companion_messages SET created_at = ? WHERE id = ?').run(new Date(Date.now() - 11 * 60 * 1000).toISOString(), source.id);
    const event = createEvent(requirePersona(persona.id), {type: 'social', situation: '和朋友散步', mood: '轻松', scene: '校园小路'}, {proactive: true, source: 'test'});
    const job = database.prepare("SELECT * FROM companion_jobs WHERE id = ? AND job_type = 'proactive_message'").get(database.prepare("SELECT id FROM companion_jobs WHERE persona_id = ? AND job_type = 'proactive_message' ORDER BY created_at DESC LIMIT 1").get(persona.id).id);
    const leaseOwner = 'lease_frozen_decision';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), job.id);
    const decision = {schemaVersion: 1, send: true, reason: '冻结测试', message: '这是冻结后的主动消息。'};
    assert.equal(freezeProactiveDecision({...job, lease_owner: leaseOwner}, decision).changed, true);
    const completed = completeProactiveMessageJob({...job, lease_owner: leaseOwner});
    assert.equal(completed.completed, true);
    assert.equal(listMessages(persona.id, {markRead: false}).items.find(message => message.proactiveEventId === event.eventId).text, '这是冻结后的主动消息。');
});

test('rescheduling keeps one schedule and records an audited life event', () => {
    const persona = createPersona({name: '季遥', role: '学生', foundation: '季遥喜欢和朋友提前约好看展。'});
    const original = createScheduleItem(persona.id, {
        title: '周末看展', startsAt: new Date(Date.now() + 4 * 3_600_000).toISOString(), endsAt: new Date(Date.now() + 5 * 3_600_000).toISOString(), source: 'explicit_chat_plan', scene: '美术馆'
    });
    const movedStart = new Date(Date.now() + 7 * 3_600_000).toISOString();
    const moved = rescheduleScheduleItem(persona.id, original.id, {startsAt: movedStart, title: '改到傍晚看展', scene: '新馆'});
    assert.equal(moved.id, original.id);
    assert.equal(moved.title, '改到傍晚看展');
    assert.equal(moved.startsAt, movedStart);
    const event = database.prepare("SELECT * FROM companion_life_events WHERE persona_id = ? AND type = 'schedule_rescheduled' ORDER BY occurred_at DESC LIMIT 1").get(persona.id);
    assert.equal(event.causation_id, original.id);
    const audit = JSON.parse(event.payload_json);
    assert.equal(audit.previous.id, original.id);
    assert.equal(audit.next.startsAt, movedStart);
    assert.throws(() => rescheduleScheduleItem(persona.id, original.id, {startsAt: new Date(Date.now() - 60_000).toISOString()}), /未来的明确时间/);
    assert.throws(() => rescheduleScheduleItem('not-this-persona', original.id, {startsAt: new Date(Date.now() + 9 * 3_600_000).toISOString()}), /人格不存在/);
});

test('explicit chat plans retain source-message provenance and reject ambiguous or invalid plans', () => {
    const persona = createPersona({name: '唐宁', role: '学生', foundation: '唐宁喜欢和朋友一起看展。'});
    const accepted = explicitPlanFromMessage('我们约好了明天下午3点去看展。');
    assert.ok(accepted);
    assert.equal(explicitPlanFromMessage('明天可能去看展吧。'), null);
    assert.equal(explicitPlanFromMessage('我们说好了明天去看展。'), null);
    assert.equal(explicitPlanFromMessage('我们约好了明天晚上25点去看展。'), null);

    const schedule = createScheduleItem(persona.id, {
        ...accepted,
        source: 'explicit_chat_plan',
        sourceMessageId: 'message_explicitly_accepted',
        scene: '当代艺术馆'
    });
    const persisted = database.prepare('SELECT * FROM companion_schedule_items WHERE id = ?').get(schedule.id);
    assert.equal(persisted.source, 'explicit_chat_plan');
    assert.deepEqual(JSON.parse(persisted.details_json), {scene: '当代艺术馆', sourceMessageId: 'message_explicitly_accepted'});

    assert.throws(() => createScheduleItem(persona.id, {
        title: '过去的约定', startsAt: new Date(Date.now() - 60_000).toISOString(), source: 'explicit_chat_plan'
    }), /未来的明确时间/);
    assert.throws(() => createScheduleItem(persona.id, {
        title: '结束时间无效', startsAt: new Date(Date.now() + 3_600_000).toISOString(), endsAt: new Date(Date.now() + 1_800_000).toISOString(), source: 'explicit_chat_plan'
    }), /结束时间无效/);
});

test('expired transient appearance is cleared when persona state recovers', () => {
    const persona = createPersona({name: '许棠', role: '学生', foundation: '许棠喜欢把日常打扮得舒适自然。'});
    const event = createEvent(requirePersona(persona.id), {
        type: 'shopping', situation: '刚买完雨天用的外套', mood: '开心', scene: '商场',
        appearance: {outerwear: '深绿色雨衣', accessory: '透明雨伞'},
        resolvesAt: new Date(Date.now() + 60_000).toISOString()
    }, {publish: false, simulated: true, source: 'test'});
    assert.deepEqual(stateShape(persona.id).appearance, {outerwear: '深绿色雨衣', accessory: '透明雨伞'});

    database.prepare('UPDATE companion_life_events SET resolves_at = ? WHERE id = ?').run(new Date(Date.now() - 60_000).toISOString(), event.eventId);
    database.prepare('UPDATE companion_persona_states SET checkpoint_at = ? WHERE persona_id = ?').run(new Date(Date.now() - 31 * 60 * 1000).toISOString(), persona.id);
    recoverPersona(persona.id);

    assert.deepEqual(stateShape(persona.id).appearance, {});
    assert.equal(stateShape(persona.id).source.kind, 'recovery');
});

test('media settlement only completes the leased placeholder once', () => {
    const persona = createPersona({name: '白露', role: '摄影爱好者', foundation: '白露喜欢拍城市里的光影。'});
    const request = createChatMediaRequest(persona.id, mediaCall('image', '午后街角的自然照片'));
    const job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const leaseOwner = 'lease_media_settlement';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ? WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), job.id);

    const rejected = completePolledMediaJob({...job, lease_owner: 'stale_lease'}, 'prompt_valid', [{filename: 'street.png', type: 'output'}]);
    assert.equal(rejected.completed, false);
    assert.equal(listMessages(persona.id, {markRead: false}).items.find(item => item.id === request.message.id).generation.status, 'queued');
    assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'leased');

    const completed = completePolledMediaJob({...job, lease_owner: leaseOwner}, 'prompt_valid', [{filename: 'street.png', type: 'output'}]);
    assert.equal(completed.completed, true);
    const settled = listMessages(persona.id, {markRead: false}).items.find(item => item.id === request.message.id);
    assert.equal(settled.generation.status, 'ready');
    assert.equal(settled.generation.promptId, 'prompt_valid');
    assert.equal(settled.attachments.length, 1);

    const duplicate = completePolledMediaJob({...job, lease_owner: leaseOwner}, 'prompt_valid', [{filename: 'street.png', type: 'output'}]);
    assert.equal(duplicate.completed, false);
    assert.equal(database.prepare('SELECT COUNT(*) AS count FROM companion_media_assets WHERE filename = ?').get('street.png').count, 1);
    assert.equal(database.prepare('SELECT status FROM companion_jobs WHERE id = ?').get(job.id).status, 'complete');
});

test('shared scene migration, policy detail route, and scene-event projection stay persona-scoped', () => {
    const migration = database.prepare('SELECT name FROM companion_schema_migrations WHERE version = 11').get();
    assert.equal(migration.name, 'shared-scene-and-image-generation-policy');
    const personaColumns = database.prepare('PRAGMA table_info(companion_personas)').all().map(column => column.name);
    const stateColumns = database.prepare('PRAGMA table_info(companion_persona_states)').all().map(column => column.name);
    assert.equal(personaColumns.includes('image_generation_policy'), true);
    assert.equal(stateColumns.includes('shared_scene_json'), true);

    const persona = createPersona({name: '共同场景测试', role: '陪伴者', foundation: '共同场景测试喜欢和用户自然聊天。'});
    const row = database.prepare('SELECT image_generation_policy FROM companion_personas WHERE id = ?').get(persona.id);
    assert.equal(row.image_generation_policy, 'autonomous');
    assert.equal(invokeRoute('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {params: {personaId: persona.id}, body: {policy: 'ask'}}).body.imageGenerationPolicy, 'ask');
    assert.equal(invokeRoute('/api/companion/personas/:personaId/image-generation-policy', 'PUT', {params: {personaId: persona.id}, body: {policy: 'bad'}}).statusCode, 400);
    const detail = invokeRoute('/api/companion/personas/:personaId', 'GET', {params: {personaId: persona.id}}).body;
    assert.equal(detail.persona.imageGenerationPolicy, 'ask');
    assert.equal(detail.imageGenerationPolicy, 'ask');
    assert.equal(persona.currentSituation, detail.persona.currentSituation);

    const source = appendMessage(persona.id, {role: 'user', text: '我们去湖边走走吧。'});
    const started = applySceneEvent(persona, {operation: 'start', location: '湖边公园', room: '', activity: '沿湖散步', situation: '和用户一起沿湖边散步', mood: '放松', objects: ['雨伞'], participants: ['user', 'persona']}, source.id);
    assert.equal(sharedSceneFor(persona.id).eventId, started.eventId);
    assert.equal(resolvedStateFor(persona.id).resolved_source, 'shared_scene');
    assert.equal(stateShape(persona.id).location, '湖边公园');
    assert.match(contextFor(persona.id).layers.lifeState, /湖边公园/);
    assert.match(contextFor(persona.id).layers.systemCapability, /人格生图频率.*始终询问/);

    const switched = applySceneEvent(persona, {operation: 'switch', location: '湖边咖啡馆', room: '靠窗座位', activity: '一起喝咖啡', situation: '和用户在湖边咖啡馆靠窗喝咖啡', mood: '安静'}, source.id);
    assert.equal(sharedSceneFor(persona.id).eventId, switched.eventId);
    const switchPayload = JSON.parse(database.prepare('SELECT payload_json FROM companion_life_events WHERE id = ?').get(switched.eventId).payload_json);
    assert.equal(switchPayload.previousScene.eventId, started.eventId);

    const ended = applySceneEvent(persona, {operation: 'end'}, source.id);
    assert.equal(ended.operation, 'end');
    assert.equal(sharedSceneFor(persona.id), null);
    assert.notEqual(resolvedStateFor(persona.id).resolved_source, 'shared_scene');
    assert.equal(stateShape(persona.id).sharedScene, null);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_activities WHERE persona_id = ? AND event_id IN (?, ?, ?)").get(persona.id, started.eventId, switched.eventId, ended.eventId).count, 0);

    assert.throws(() => normalizeSceneEventCall({operation: 'start', location: '湖边'}), /situation/);
    assert.throws(() => normalizeSceneEventCall({operation: 'switch', location: '湖边', situation: '散步', unsupported: true}), /不支持字段/);
});

test('scene tool contract accumulates streamed fragments and parenthesized text remains ordinary escaped message text', async () => {
    assert.equal(sceneEventTool.function.name, 'scene_event');
    assert.deepEqual(sceneEventTool.function.parameters.required, ['operation']);
    const chunks = [
        'data: ' + JSON.stringify({choices: [{delta: {tool_calls: [{index: 0, id: 'call_scene', type: 'function', function: {name: 'scene_event', arguments: '{"operation":"start","location":"湖边"'}}]}}]}) + '\n\n',
        'data: ' + JSON.stringify({choices: [{delta: {tool_calls: [{index: 0, function: {arguments: ',"situation":"一起散步"}'}}]}}]}) + '\n\n',
        'data: [DONE]\n\n'
    ];
    const response = {body: {getReader() {
        let index = 0;
        return {read: async () => index < chunks.length ? {value: new TextEncoder().encode(chunks[index++]), done: false} : {value: undefined, done: true}};
    }}};
    const completion = await consumeStreamedCompletion(response);
    assert.equal(completion.toolCalls[0].id, 'call_scene');
    assert.equal(JSON.parse(completion.toolCalls[0].function.arguments).situation, '一起散步');
    const source = readFileSync(new URL('../src/companion-main.js', import.meta.url), 'utf8');
    assert.match(source, /const content = esc\(message\.text\)/);
    assert.doesNotMatch(source, /scene-panel|quick-reply/);
});

test.after(() => rmSync(dataDir, {recursive: true, force: true}));
