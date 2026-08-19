import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-test-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '0';
const {companionApp, companionTestHooks} = await import(`../server.js?test=${Date.now()}`);
const {database, createPersona, createEvent, requirePersona, deletePersona, listActivities, listMessages, appendMessage, appendUserVisibleAssistantReply, splitUserVisibleAssistantReply, userVisibleChatPrompt, extractMediaIntent, mediaRequestFromText, mediaCommitmentFromText, normalizeMediaRequest, normalizeMediaIntent, systemCapabilityReplyForm, systemCapabilityMediaContract, imagePromptMasterContract, addActivityComment, setUserReaction, activeMemories, stateFor, resolvedStateFor, stateShape, scheduledState, contextFor, mediaIntentFor, compileMediaPrompt, normalizeMediaRefinement, refineMediaIntent, applyRelationshipEvolution, activeRelationshipPatch, explicitPlanFromMessage, createScheduleItem, rescheduleScheduleItem, createChatMediaRequest, completePolledMediaJob, completeProactiveMessageJob, proactiveEligibility, personaFocusTier, publicBlueprint, restoreFoundationRevision, recoverPersona, buildInitialBlueprint, normalizeLifeBlueprint, validateLifeBlueprint, finalizeLifeBlueprint, generateInitialLifeBlueprint, lifeModelSchemaVersion, zonedPlanInstant, localDayBounds, chooseTimelineTemplate, instantiateTimelineEvent, sleepAvailability, deferredBatchForMessage, createInterview, answerInterview, activateInterview, debugContextFor, redactDebugValue, debugSummary, debugInspectorEnabled, providerFor, providerSummaries, h3Args, h3OutputFile, leaseDurationForJob, saveSettings, publicSettings} = companionTestHooks;

const routePaths = app => (app.router?.stack || []).flatMap(layer => layer.route ? [layer.route.path] : []);

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
    assert.deepEqual(extractMediaIntent('我来找一张图。<media-intent>{"kind":"image","request":"咖啡馆窗边的自然照片"}</media-intent>'), {text: '我来找一张图。', media: {kind: 'image', prompt: '咖啡馆窗边的自然照片'}});
    assert.equal(extractMediaIntent('好的，我这就给你生成一张海边日落的图片。').media, null);
    assert.equal(extractMediaIntent('<media-intent>{"kind":"audio","request":"不支持"}</media-intent>').media, null);
    assert.equal(mediaCommitmentFromText('我待会拍完发你。'), null);
    assert.deepEqual(mediaRequestFromText('请给我生成一段在雨中散步的视频。'), {kind: 'video', prompt: '请给我生成一段在雨中散步的视频。'});
    assert.equal(mediaRequestFromText('不要生成图片，我们继续聊天。'), null);
    assert.equal(extractMediaIntent('今天聊聊天。').media, null);
});

test('media requests and refinements are server-validated while locked narrative facts remain authoritative', () => {
    const persona = createPersona({name: '契约', role: '学生', foundation: '契约会遵守当前生活状态。'});
    assert.equal(normalizeMediaRequest({kind: 'image', request: ''}), null);
    assert.deepEqual(normalizeMediaRequest({kind: 'video', request: '傍晚散步', count: 9}), {kind: 'video', prompt: '傍晚散步', count: 3});
    const intent = mediaIntentFor(requirePersona(persona.id), {kind: 'image', request: '手持手机自拍，举高45度角比心', event: {type: 'social', scene: '公园', situation: '和闺蜜散步', mood: '开心'}});
    const tampered = structuredClone(intent);
    tampered.locked.capture.view = 'unbounded_camera';
    assert.throws(() => normalizeMediaIntent(tampered), /取景契约无效/);
    assert.equal(normalizeMediaRefinement({photographyStyle: '自然纪实', forbidden: '篡改锁定事实'}), null);
    const lockedPatch = normalizeMediaIntent({...intent, enrichable: {...intent.enrichable, shotAngle: '不应保留', poseDetail: '不应保留'}});
    assert.equal(lockedPatch.enrichable.shotAngle, undefined);
    assert.equal(lockedPatch.enrichable.poseDetail, undefined);
    assert.throws(() => createChatMediaRequest(persona.id, {kind: 'audio', prompt: '无效'}), /媒体请求/);
});

test('camera-geometry prompt keeps high-angle and POV facts when malformed refinement falls back', async () => {
    const persona = createPersona({name: '几何', role: '摄影社成员', foundation: '几何会认真记录朋友。'});
    const intent = mediaIntentFor(requirePersona(persona.id), {
        kind: 'image',
        request: '第一人称 POV，举高45度从上往下拍摄闺蜜',
        event: {type: 'social', scene: '公园花墙前', situation: '正在给闺蜜拍照', mood: '开心'}
    });
    const prompt = compileMediaPrompt(intent);
    assert.equal(intent.locked.capture.view, 'operator_pov');
    assert.equal(intent.locked.capture.angleLocked, true);
    assert.match(prompt, /^这是一张真实生活摄影质感的照片/);
    assert.match(prompt, /镜头位于人格主角所在的摄影者位置，正对被摄主体，机位高度为与被摄主体视线大致同高，以从上往下的俯拍角度的方向拍摄，并保持摄影者第一人称透视（POV）/);
    assert.match(prompt, /最终必须严格保持镜头位于人格主角所在的摄影者位置/);
    assert.match(prompt, /不得改成其他视角/);
    assert.doesNotMatch(prompt, /(?:取景|拍摄者|设备)=/);

    const originalFetch = globalThis.fetch;
    globalThis.fetch = async url => ({
        ok: true,
        json: async () => String(url).endsWith('/models')
            ? {data: [{id: 'test-model'}]}
            : {choices: [{message: {content: '{"shotAngle":"改成仰拍","locked":{"capture":{"view":"external_observer"}}}'}}]}
    });
    try {
        const refined = await refineMediaIntent(intent);
        assert.equal(refined.status, 'deterministic_fallback');
        assert.equal(refined.intent.locked.capture.view, 'operator_pov');
        assert.equal(refined.intent.locked.capture.downwardAngle, '从上往下的俯拍角度');
        assert.match(compileMediaPrompt(refined.intent), /以从上往下的俯拍角度的方向拍摄/);
    } finally {
        globalThis.fetch = originalFetch;
    }
});

test('media providers validate capabilities, persist selection, and keep h3 paths private', () => {
    assert.deepEqual(providerSummaries().map(provider => provider.id), ['comfyui', 'h3']);
    assert.throws(() => providerFor('image', 'h3'), /不支持图片/);
    saveSettings({videoProvider: 'h3', h3Executable: '/private/bin/h3.c', h3ModelDir: '/private/models', h3OutputDir: '/private/output', h3AllowedRoot: '/private', h3Defaults: {width: 720, reuse: 2}});
    assert.equal(publicSettings().videoProvider, 'h3');
    assert.equal(Object.hasOwn(publicSettings(), 'h3Executable'), false);
    assert.equal(Object.hasOwn(publicSettings().h3Defaults, 'profile'), false);
    assert.deepEqual(h3Args({prompt: '已编译提示词'}, {h3ModelDir: '/private/models', h3Defaults: {width: 720, reuse: 2}}, '/private/output/video.mp4'), ['-d', '/private/models', '-p', '已编译提示词', '--width', '720', '--reuse', '2', '-o', '/private/output/video.mp4']);
    assert.equal(h3OutputFile({}, {h3OutputDir: '/private/output', h3AllowedRoot: '/private'}).startsWith('/private/output/'), true);
    assert.throws(() => h3OutputFile({outputPath: '/outside/video.mp4'}, {h3OutputDir: '/private/output', h3AllowedRoot: '/private'}), /路径无效/);
    assert.equal(leaseDurationForJob({payload_json: JSON.stringify({provider: 'h3'})}) > 90_000, true);
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
    const media = createChatMediaRequest(persona.id, {kind: 'image', prompt: '拍一张现在的自然照片'});
    const intent = JSON.parse(database.prepare('SELECT payload_json FROM companion_jobs WHERE id = ?').get(media.jobId).payload_json).mediaIntent;
    assert.equal(intent.location, '图书馆自习区');
    assert.equal(intent.action, '正在图书馆整理课程笔记');
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_daily_plans WHERE persona_id = ? AND status = 'queued'").get(persona.id).count, 1);
    assert.equal(database.prepare("SELECT COUNT(*) AS count FROM companion_jobs WHERE persona_id = ? AND job_type = 'daily_plan'").get(persona.id).count, 1);
});

test('model-authorized media intent preserves a bounded creative direction', () => {
    const parsed = extractMediaIntent('我晚点把这组街拍发给你。<media-intent>{"kind":"image","request":"傍晚街拍","creativeDirection":{"photographyStyle":"35mm 胶片感","wardrobeAccessories":"银色耳环","location":"不可覆盖"}}</media-intent>');
    assert.deepEqual(parsed.media, {kind: 'image', prompt: '傍晚街拍', creativeDirection: {photographyStyle: '35mm 胶片感', wardrobeAccessories: '银色耳环'}});
    assert.match(imagePromptMasterContract, /AI 生图提示词大师/);
    assert.match(imagePromptMasterContract, /不得改变任何锁定的镜头几何/);
});

test('social-event media intent preserves people, location, action, and contextual pose', () => {
    const persona = createPersona({name: '画面', role: '学生', foundation: '画面喜欢与朋友分享校园生活。', supportingCast: [{name: '小柯', relationshipKind: '室友'}]});
    const friend = database.prepare('SELECT id FROM companion_supporting_characters WHERE persona_id = ?').get(persona.id);
    const intent = mediaIntentFor(requirePersona(persona.id), {kind: 'image', event: {type: 'social', scene: '校园咖啡馆窗边', situation: '和室友小柯喝咖啡聊天', mood: '轻松', participants: [friend.id]}});
    const prompt = compileMediaPrompt(intent);
    assert.match(intent.subject, /画面/);
    assert.match(intent.subject, /小柯/);
    assert.equal(intent.location, '校园咖啡馆窗边');
    assert.match(intent.pose, /互动真实放松/);
    assert.match(prompt, /场景位于校园咖啡馆窗边/);
    assert.match(prompt, /正在和室友小柯喝咖啡聊天/);
});

test('persona photographing her female friend uses photographer POV and excludes wrong photographers', () => {
    const persona = createPersona({name: '若岚', role: '摄影爱好者', foundation: '若岚是一位女性摄影爱好者，常带相机记录朋友。'});
    const intent = mediaIntentFor(requirePersona(persona.id), {kind: 'image', event: {type: 'social', scene: '公园花墙前', situation: '正在给闺蜜拍照', mood: '开心'}});
    const prompt = compileMediaPrompt(intent);
    assert.equal(intent.actor, '若岚');
    assert.equal(intent.locked.capture.view, 'operator_pov');
    assert.equal(intent.locked.capture.operator, 'off_camera_subject');
    assert.equal(intent.locked.capture.cameraVisibility, 'not_visible');
    assert.deepEqual(intent.people, ['闺蜜']);
    assert.equal(intent.subject.includes('闺蜜'), true);
    assert.equal(intent.mustNotAppear.includes('摄影者不入镜'), true);
    assert.equal(intent.mustNotAppear.includes('不要出现额外摄影者'), true);
    assert.match(prompt, /画面中只出现闺蜜，共1人/);
    assert.match(prompt, /不要生成外部旁观者视角/);
    assert.match(prompt, /设备本体物理上位于镜头正后方，完全处于画框之外/);
    assert.match(prompt, /不得出现手机、相机或任何手持设备/);
    assert.match(prompt, /不得出现屏幕、镜面或反射中的设备/);
    assert.match(prompt, /不得出现设备投下的阴影/);
    assert.match(prompt, /不得让设备或持有设备的手遮挡画面/);
});

test('chat media jobs persist a placeholder and replace that exact message on completion', () => {
    const persona = createPersona({name: '沈青', role: '在读大学生', foundation: '沈青是摄影社成员，喜欢记录普通日常。'});
    const request = createChatMediaRequest(persona.id, {kind: 'image', prompt: '今天在校园里的自然照片'});
    assert.equal(request.message.generation.status, 'queued');
    assert.equal(request.message.generation.kind, 'image');
    assert.equal(request.message.attachments.length, 0);

    const job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const payload = JSON.parse(job.payload_json);
    assert.equal(payload.mediaIntent.mediaKind, 'image');
    assert.match(payload.prompt, /^这是一张真实生活摄影质感的照片/);
    assert.match(payload.prompt, /画面中只出现/);
    assert.match(payload.prompt, /场景位于/);
    assert.match(payload.prompt, /画面必须避免/);
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

    const video = createChatMediaRequest(persona.id, {kind: 'video', prompt: '一段傍晚散步的视频'});
    assert.equal(video.message.generation.kind, 'video');
    const videoPayload = JSON.parse(database.prepare('SELECT payload_json FROM companion_jobs WHERE id = ?').get(video.jobId).payload_json);
    assert.equal(videoPayload.mediaIntent.mediaKind, 'video');
    assert.match(videoPayload.prompt, /使用.*色调保持/);
    assert.throws(() => createChatMediaRequest(persona.id, {kind: 'image,video'}), /媒体类型/);
});

test('explicit chat visual direction overrides generic current-state defaults in the media intent', () => {
    const persona = createPersona({name: '方向', role: '学生', foundation: '方向正在图书馆整理笔记。'});
    createEvent(requirePersona(persona.id), {type: 'study', situation: '正在图书馆整理笔记', mood: '专注', scene: '图书馆'}, {publish: false, source: 'test'});
    const request = createChatMediaRequest(persona.id, {kind: 'image', prompt: '生成一张海边日落的无人风景照片'});
    const job = database.prepare('SELECT payload_json FROM companion_jobs WHERE id = ?').get(request.jobId);
    const payload = JSON.parse(job.payload_json);
    assert.equal(payload.mediaIntent.location, '图书馆');
    assert.deepEqual(payload.mediaIntent.people, []);
    assert.match(payload.mediaIntent.subject, /海边日落/);
    assert.match(payload.prompt, /场景位于图书馆/);
    assert.doesNotMatch(payload.prompt, /user visual direction/);

    const selfie = mediaIntentFor(requirePersona(persona.id), {kind: 'image', request: '请生成一张我在河边自拍的照片，表情开心'});
    assert.equal(selfie.location, '河边');
    assert.equal(selfie.locked.capture.view, 'self_capture');
    assert.match(selfie.subject, /我在河边自拍/);
    assert.match(selfie.action, /表情开心/);
});

test('selfie directions lock people, front-camera composition, pose, expression, and left-side light', () => {
    const persona = createPersona({name: '林晚', role: '学生', foundation: '林晚是摄影社成员，喜欢记录朋友。'});
    const intent = mediaIntentFor(requirePersona(persona.id), {
        kind: 'image',
        request: '再补两张呗，手持手机，用内摄像头来张自拍合照，再和小周一起比个心，手机举高45度角，自然光从左侧，两人自然笑'
    });
    const prompt = compileMediaPrompt(intent);
    assert.deepEqual(intent.people, ['林晚', '小周']);
    assert.equal(intent.locked.capture.view, 'self_capture');
    assert.equal(intent.locked.capture.operator, 'visible_subject');
    assert.equal(intent.locked.capture.cameraVisibility, 'not_visible');
    assert.match(intent.camera, /前置摄像头/);
    assert.match(intent.framing, /双人自拍合照/);
    assert.match(intent.pose, /一起朝镜头比心/);
    assert.match(intent.pose, /45°斜拍/);
    assert.equal(intent.locked.capture.angle, '从上往下约45°斜拍');
    assert.match(intent.expression, /自然放松地微笑/);
    assert.match(intent.lighting, /左侧/);
    assert.match(prompt, /画面中只出现林晚、小周，共2人/);
    assert.match(prompt, /手机前置摄像头是拍摄方式，但设备本体物理上位于镜头正后方，完全处于画框之外/);
    assert.match(prompt, /画面必须避免.*不要生成外部旁观者视角/);
    assert.match(prompt, /不得出现手机、相机或任何手持设备/);
    assert.match(prompt, /不得出现屏幕、镜面或反射中的设备/);
    assert.match(prompt, /不得出现设备投下的阴影/);
    assert.match(prompt, /不得让设备或持有设备的手遮挡画面/);
    assert.doesNotMatch(prompt, /=/);
    assert.deepEqual(mediaRequestFromText('再补两张自拍合照发动态'), {kind: 'image', prompt: '再补两张自拍合照发动态', count: 2});
    assert.equal(extractMediaIntent('我待会拍一张，拍完发你。').media, null);
});

test('an explicitly visible capture device remains allowed and does not receive hidden-device prohibitions', () => {
    const persona = createPersona({name: '可见设备', role: '学生', foundation: '可见设备喜欢记录日常。'});
    const intent = mediaIntentFor(requirePersona(persona.id), {
        kind: 'image',
        request: '镜面自拍合照，手机入镜可见，和小周自然微笑'
    });
    const prompt = compileMediaPrompt(intent);
    assert.equal(intent.locked.capture.cameraVisibility, 'visible');
    assert.match(prompt, /用户明确允许该设备作为画面的一部分可见/);
    assert.doesNotMatch(prompt, /设备本体物理上位于镜头正后方/);
    assert.doesNotMatch(prompt, /不得出现手机、相机或任何手持设备/);
    assert.doesNotMatch(prompt, /不得出现屏幕、镜面或反射中的设备/);
    assert.doesNotMatch(prompt, /不得出现设备投下的阴影/);
    assert.doesNotMatch(prompt, /不得让设备或持有设备的手遮挡画面/);
});

test('media context preserves the previous outfit while allowing explicit user changes', () => {
    const persona = createPersona({name: '连续', role: '学生', foundation: '连续会保持连续的视觉设定。', visualBaseline: '白色针织衫，银色耳环'});
    const first = createChatMediaRequest(persona.id, {kind: 'image', prompt: '和小周合照，穿着蓝色衬衫，银色耳环'});
    database.prepare("UPDATE companion_jobs SET status = 'complete', completed_at = ? WHERE id = ?").run(new Date().toISOString(), first.jobId);
    const next = mediaIntentFor(requirePersona(persona.id), {kind: 'image', request: '再拍一张和小周的合照'});
    assert.match(next.wardrobe, /白色针织衫/);
    assert.doesNotMatch(next.wardrobe, /蓝色衬衫/);
    const changed = mediaIntentFor(requirePersona(persona.id), {kind: 'image', request: '再拍一张，穿着红色外套的合照'});
    assert.match(changed.wardrobe, /红色外套/);
});

test('debug context is redacted, bounded, and persona-scoped', () => {
    assert.equal(debugInspectorEnabled, false);
    const first = createPersona({name: '调试一号', role: '学生', foundation: '调试一号喜欢记录日常。'});
    const second = createPersona({name: '调试二号', role: '编辑', foundation: '调试二号喜欢收集旧书。'});
    appendMessage(first.id, {role: 'user', text: `Bearer secret-token-${'x'.repeat(80)} ${'a'.repeat(2600)}`});
    appendMessage(second.id, {role: 'user', text: '这条内容不能泄露到另一人格。'});
    const firstMedia = createChatMediaRequest(first.id, {kind: 'image', prompt: `apiKey=super-secret ${'p'.repeat(2600)}`});
    const secondMedia = createChatMediaRequest(second.id, {kind: 'video', prompt: '只属于第二人格的媒体意图'});

    const context = debugContextFor(first.id);
    assert.equal(context.recentRequests.length <= 10, true);
    assert.equal(context.mediaJobs.length <= 10, true);
    assert.equal(context.recentRequests.some(item => item.promptSummary.includes('super-secret')), false);
    assert.equal(context.recentRequests.some(item => item.promptSummary.includes('另一人格')), false);
    assert.equal(context.mediaJobs.some(item => item.id === secondMedia.jobId), false);
    assert.equal(context.mediaJobs.some(item => item.id === firstMedia.jobId), true);
    assert.equal(context.mediaJobs[0].promptSummary.length <= 2000, true);
    assert.deepEqual(redactDebugValue({authorization: 'Bearer abc', nested: {apiKey: 'secret'}}), {authorization: '[redacted]', nested: {apiKey: '[redacted]'}});
    assert.equal(debugSummary(`Bearer abcdefghijklmnopqrstuvwxyz ${'z'.repeat(2100)}`).includes('abcdefghijklmnopqrstuvwxyz'), false);
});

test('debug routes are not registered unless the explicit local flag is enabled', async () => {
    assert.equal(routePaths(companionApp).some(path => String(path).includes('debug-context')), false);
    assert.equal(routePaths(companionApp).some(path => String(path).includes('debug-media')), false);

    const debugDataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-debug-test-'));
    const originalDataDir = process.env.DATA_DIR;
    const originalFlag = process.env.COMPANION_DEBUG_INSPECTOR;
    process.env.DATA_DIR = debugDataDir;
    process.env.COMPANION_DEBUG_INSPECTOR = '1';
    try {
        const debugModule = await import(`../server.js?debug=${Date.now()}`);
        const persona = debugModule.companionTestHooks.createPersona({name: '本地检查', role: '开发测试', foundation: '只在本地检查器中查看。'});
        assert.equal(debugModule.companionTestHooks.debugInspectorEnabled, true);
        assert.equal(routePaths(debugModule.companionApp).includes('/api/companion/personas/:personaId/debug-context'), true);
        assert.equal(routePaths(debugModule.companionApp).includes('/api/companion/personas/:personaId/debug-media'), true);
        const context = debugModule.companionTestHooks.debugContextFor(persona.id);
        assert.equal(context.layers.identity.includes('本地检查'), true);
        const dispatch = debugModule.companionTestHooks.createChatMediaRequest(persona.id, {kind: 'image', prompt: '本地测试图片'});
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
    const firstMedia = createChatMediaRequest(first.id, {kind: 'image', prompt: '图书馆里的自然照片'});
    const secondMedia = createChatMediaRequest(second.id, {kind: 'video', prompt: '旧书店里翻书的短视频'});

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
    const media = createChatMediaRequest(removable.id, {kind: 'image', prompt: '删除测试图片'});
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
    appendMessage(visible.id, {role: 'user', text: '今天还好吗？'});

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
    const request = createChatMediaRequest(persona.id, {kind: 'image', prompt: '午后街角的自然照片'});
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

test.after(() => rmSync(dataDir, {recursive: true, force: true}));
