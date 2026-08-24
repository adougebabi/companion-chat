import {createHttpApp} from '../http/app.js';
import {registerCompanionRoutes} from '../http/route-registry.js';
import {createCompanionRouteHandlers} from '../application/companion-route-handlers.js';
import {createCompanionApplication} from '../application/companion-application.js';
import {createBasicCompanionServices} from '../application/basic-companion-services.js';
import {createIdentitySettingsService} from '../application/identity-settings-service.js';
import {createActivityService} from '../application/activity-service.js';
import {createProactiveJobService} from '../application/proactive-job-service.js';
import {createChatProductionPorts, createChatLlmStreamingPort} from '../application/chat-production-adapter.js';
import {createConversationCommitAdapter} from '../infrastructure/conversation-commit-adapter.js';
import {createActivityRepository} from '../infrastructure/activity-repository.js';
import {createConversationRepository} from '../infrastructure/conversation-repository.js';
import {createGroupRepository} from '../infrastructure/group-repository.js';
import {createJobRepository} from '../infrastructure/job-repository.js';
import {createLifeEventRepository} from '../infrastructure/life-event-repository.js';
import {createMemoryRepository} from '../infrastructure/memory-repository.js';
import {createAffectRepository} from '../infrastructure/affect-repository.js';
import {createInteractionFactRepository} from '../infrastructure/interaction-fact-repository.js';
import {createAppraisalRepository} from '../infrastructure/appraisal-repository.js';
import {createMemoryConsolidationRepository} from '../infrastructure/memory-consolidation-repository.js';
import {createSelfModelRepository} from '../infrastructure/self-model-repository.js';
import {createAgencyIntentionRepository} from '../infrastructure/agency-intention-repository.js';
import {createPendingEventRepository} from '../infrastructure/pending-event-repository.js';
import {createPersonaRepository} from '../infrastructure/persona-repository.js';
import {createRelationshipRepository} from '../infrastructure/relationship-repository.js';
import {createSettingsRepository} from '../infrastructure/settings-repository.js';
import {createFoundationRepository} from '../infrastructure/foundation-repository.js';
import {createScheduleRepository} from '../infrastructure/schedule-repository.js';
import {createInterviewRepository} from '../infrastructure/interview-repository.js';
import {createPersonaLifecycleRepository} from '../infrastructure/persona-lifecycle-repository.js';
import {createMediaAssetRepository} from '../infrastructure/media-asset-repository.js';
import {createPromptRunRepository} from '../infrastructure/prompt-run-repository.js';
import {createStateRepository} from '../infrastructure/state-repository.js';
import {createBlueprintRepository} from '../infrastructure/blueprint-repository.js';
import {createDailyPlanRepository} from '../infrastructure/daily-plan-repository.js';
import {createPresenceRepository} from '../infrastructure/presence-repository.js';
import {createSupportingCharacterRepository} from '../infrastructure/supporting-character-repository.js';
import {createTimelineRepository} from '../infrastructure/timeline-repository.js';
import {h3RuntimeHelpers} from '../infrastructure/h3-preflight.js';
import {createDebugService} from '../application/debug-service.js';
import {createMediaDebugService} from '../application/media-debug-service.js';
import {createSettingsPolicy} from '../application/settings-policy.js';
import {createLifeWorldReader} from '../application/life-world-reader.js';
import {createLifeStateResolver} from '../domain/life-state-resolver.js';
import {createLifeStateService} from '../application/life-state-service.js';
import {createLifeEventFlow} from '../application/life-event-flow.js';
import {createContextPipeline} from '../application/context-pipeline.js';
import {createTimelineFlow} from '../application/timeline-flow.js';
import {createRelationshipFlow} from '../application/relationship-flow.js';
import {createAffectFlow} from '../application/affect-flow.js';
import {createAppraisalFlow} from '../application/appraisal-flow.js';
import {createMemoryConsolidationFlow} from '../application/memory-consolidation-flow.js';
import {createSelfModelFlow} from '../application/self-model-flow.js';
import {createAgencyIntentionFlow} from '../application/agency-intention-flow.js';
import {createMemoryEventFlow} from '../application/memory-flow.js';
import {createMemoryService} from '../application/memory-service.js';
import {createDeferredChatPolicy} from '../application/deferred-chat-policy.js';
import {imageGenerationPolicyLabels, systemCapabilityPromptFor} from '../application/context-contracts.js';
import {createProviderRegistry} from '../infrastructure/provider-ports.js';
import {createProductionProviderRegistry} from '../infrastructure/production-media-providers.js';
import {createMtplxCompletionPort} from '../infrastructure/llm-provider.js';
import {createMtplxJsonCompletionPort} from '../infrastructure/llm-provider.js';
import {getAllCapabilityTools} from '../application/capability-catalog.js';
import {createInterviewAnalyzer} from '../application/interview-analyzer.js';
import {createMediaJobRepository} from '../infrastructure/media-job-repository.js';
import {createMediaPromptMaster, createSkippedMediaAcceptance} from '../infrastructure/media-prompt-master.js';
import {createProductionDeferredChatBatchRepository, createProductionProactiveFlows} from '../infrastructure/production-proactive-ports.js';
import {createMediaJobApplication} from '../application/media-job-composition.js';
import {createMediaFlow} from '../application/media-flow.js';
import {createPendingEventFlow} from '../application/pending-event-flow.js';
import {createSceneEventFlow} from '../application/scene-event-flow.js';
import {createAppearanceEventFlow} from '../application/appearance-event-flow.js';
import {isPlanDate, localDateFor, nextDate, timezoneFor, zonedInstant} from '../domain/daily-plan-defaults.js';
import {createFlowRegistry} from '../application/flow-registry.js';
import {registerFlowAdapter} from '../application/flow-effect-adapter.js';
import {createFlowEffectAdapter} from '../application/flow-effect-adapter.js';
import createJobDispatcher from './job-dispatcher.js';
import createStartupRuntime from './startup.js';
import createWorkerRuntime from './worker-runtime.js';

const DEFAULT_PORT = 4178;
const DEFAULT_HOST = '0.0.0.0';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function persistedDebugInspector(settings) {
    try {
        const value = typeof settings?.read === 'function' ? settings.read() : typeof settings === 'function' ? settings() : null;
        return isRecord(value) && value.debugInspector === true;
    } catch {
        return false;
    }
}

// All current capabilities are universal. Filtering can be introduced later
// without changing the transport or application dispatcher contract.
const COMPANION_CAPABILITY_TOOLS = getAllCapabilityTools();

function nonEmpty(value, fallback) {
    if (typeof value === 'string' && value.trim() !== '') return value.trim();
    return fallback;
}

function resolvePort(value, environment) {
    const candidate = value ?? environment?.PORT ?? DEFAULT_PORT;
    const port = Number(candidate);
    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        throw new RangeError('Runtime port must be an integer between 0 and 65535');
    }
    return port;
}

function resolveStartup(options) {
    const configured = options.startupRuntime ?? options.startup;
    if (configured !== undefined) {
        if (!isRecord(configured) || !isRecord(configured.database) || typeof configured.close !== 'function') {
            throw new TypeError('Runtime startup must provide database and close()');
        }
        return configured;
    }
    const startupOptions = isRecord(options.startupOptions) ? options.startupOptions : {};
    return createStartupRuntime({
        ...startupOptions,
        environment: startupOptions.environment ?? options.environment ?? process.env,
        Database: startupOptions.Database ?? options.Database,
        dataDir: startupOptions.dataDir ?? options.dataDir,
        databasePath: startupOptions.databasePath ?? options.databasePath,
        now: startupOptions.now ?? options.now,
        clock: startupOptions.clock ?? options.clock,
        id: startupOptions.id ?? options.id,
        idGenerator: startupOptions.idGenerator ?? options.idGenerator,
        settings: startupOptions.settings ?? options.settings,
        migrations: startupOptions.migrations
    });
}

function resolveApp(options, startup, worker) {
    const configured = options.app;
    if (configured !== undefined) return configured;
    const httpOptions = isRecord(options.httpOptions) ? options.httpOptions : {};
    const configuredRegistrar = httpOptions.routeRegistrar ?? options.routeRegistrar;
    const routeHandlers = httpOptions.routeHandlers ?? options.routeHandlers;
    const chatRoute = httpOptions.chatRoute ?? options.chatRoute ?? options.application?.chatRoute;
    const routeRegistrar = configuredRegistrar ?? (routeHandlers ? ({app: routeApp, wrapRoute, sendError}) => registerCompanionRoutes({
        app: routeApp,
        handlers: routeHandlers,
        wrapRoute,
        sendError,
        debugInspectorEnabled: options.debugInspectorEnabled === true || httpOptions.debugInspectorEnabled === true,
        missingHandler: httpOptions.missingHandler ?? options.missingHandler ?? 'error',
        skip: ['health', ...(chatRoute ? ['chat'] : [])]
    }) : undefined);
    return createHttpApp({
        ...httpOptions,
        root: httpOptions.root ?? options.root,
        staticRoot: httpOptions.staticRoot ?? options.staticRoot,
        routeRegistrar,
        chatRoute,
        chatSseAdapter: httpOptions.chatSseAdapter ?? options.chatSseAdapter,
        healthResponse: httpOptions.healthResponse ?? options.healthResponse,
        worker,
        database: startup.database
    });
}

function resolveWorker(options, startup, repositories) {
    if (options.workerRuntime === false || options.worker === false) return null;
    const configured = options.workerRuntime ?? (isRecord(options.worker) ? options.worker : undefined);
    if (configured !== undefined) {
        if (!isRecord(configured) || typeof configured.start !== 'function' || typeof configured.stop !== 'function') {
            throw new TypeError('Runtime worker must provide start() and stop()');
        }
        return configured;
    }
    const workerOptions = isRecord(options.workerOptions) ? options.workerOptions : {};
    const hasWork = ['jobTick', 'tick', 'processTick', 'claimJob', 'runJob'].some(name => typeof workerOptions[name] === 'function' || typeof options[name] === 'function')
        || typeof options.jobDispatcher?.jobTick === 'function'
        || typeof options.jobDispatcher?.runJob === 'function';
    if (!hasWork) return null;
    return createWorkerRuntime({
        ...workerOptions,
        jobRepository: workerOptions.jobRepository ?? options.jobRepository ?? repositories?.jobRepository ?? repositories?.job,
        jobTick: workerOptions.jobTick ?? options.jobTick ?? options.jobDispatcher?.jobTick,
        claimJob: workerOptions.claimJob ?? options.claimJob,
        runJob: workerOptions.runJob ?? options.runJob,
        recoverLeases: workerOptions.recoverLeases ?? options.recoverLeases,
        clock: workerOptions.clock ?? options.clock,
        timers: workerOptions.timers ?? options.timers,
        leaseOwner: workerOptions.leaseOwner ?? options.leaseOwner,
        onError: workerOptions.onError ?? options.onWorkerError
    });
}

function resolveProviders(options, environment, repositories) {
    const configured = options.providerRegistry ?? options.providers;
    if (configured && typeof configured.get === 'function' && typeof configured.register === 'function') return configured;
    const adapters = options.providerAdapters ?? options.mediaProviderAdapters ?? options.providers;
    if (adapters === undefined || adapters === null) return createProviderRegistry();
    return createProviderRegistry({providers: adapters, dryRunAdapters: options.dryRunAdapters});
}

function isOpenDatabase(database) {
    return isRecord(database) && typeof database.prepare === 'function' && typeof database.transaction === 'function';
}

function defaultProductionProviders(options, environment, repositories) {
    const settings = options.settings ?? repositories?.settings;
    return createProductionProviderRegistry({
        settings,
        environment,
        fetchImpl: options.fetchImpl ?? options.fetch,
        spawnImpl: options.spawnImpl ?? options.spawn,
        fs: options.providerFs ?? options.mediaProviderFs,
        id: runtimeId(options.idGenerator ?? options.id),
        promptRuns: repositories?.promptRun,
        providerAdapters: options.providerAdapters ?? options.mediaProviderAdapters
    });
}

function createDefaultCapabilityPort() {
    return Object.freeze({
        dispatch() {
            // Native capabilities remain fail-closed until their application
            // registry is explicitly composed. Ordinary chat still has one
            // valid dispatcher boundary and never falls back to marker SQL.
            return {results: [], effects: []};
        }
    });
}

function replyPostureFor(affect, drives = {}) {
    const pleasure = Number(affect?.pleasure ?? 0);
    const arousal = Number(affect?.arousal ?? 0);
    const dominance = Number(affect?.dominance ?? 0);
    const pressure = key => Math.max(0, Math.min(1, Number(drives?.[key] ?? 0.5)));
    return {
        warmth: pleasure <= -0.35 ? 'cool' : pleasure >= 0.25 ? 'warm' : 'neutral',
        energy: arousal <= -0.25 ? 'low' : arousal >= 0.3 ? 'high' : 'steady',
        directness: dominance >= 0.3 ? 'direct' : dominance <= -0.25 ? 'accommodating' : 'balanced',
        initiative: pressure('social') >= 0.7 || pressure('exploration') >= 0.7 ? 'available' : 'restraint',
        restPressure: Number(pressure('rest').toFixed(3)),
        source: affect ? 'affect_state' : 'default'
    };
}

function createDefaultChatProductionPorts(options, repositories, providers) {
    if (!isRecord(repositories?.conversation)
        || typeof repositories.conversation.listMessages !== 'function'
        || typeof repositories.conversation.getOrCreateConversation !== 'function') return null;
    const mtplx = typeof providers?.find === 'function'
        ? providers.find('mtplx', {portType: 'llm-streaming'})
        : providers?.get?.('mtplx', {portType: 'llm-streaming'});
    if (!mtplx) return null;
    const clock = runtimeClock(options.clock);
    const idGenerator = runtimeId(options.idGenerator ?? options.id);
    const settings = options.settings ?? repositories.settings;
    const readSettings = typeof settings === 'function' ? settings : settings?.read?.bind(settings) ?? (() => ({}));
    const lifeWorldReady = isRecord(repositories.blueprint)
        && isRecord(repositories.schedule)
        && isRecord(repositories.dailyPlan)
        && isRecord(repositories.presence);
    const lifeWorldReader = lifeWorldReady
        ? createLifeWorldReader({repositories, blueprintReader: repositories.blueprint, clock})
        : null;
    const resolveLifeState = createLifeStateResolver();
    const contextPipeline = createContextPipeline({
        maxChars: options.contextBudget?.maxChars ?? 12_000,
        maxFragments: options.contextBudget?.maxFragments ?? 24
    });
    const deferredLifeWorld = lifeWorldReader
        ? {
            read({personaId, at} = {}) {
                return resolveLifeState(lifeWorldReader.readResolverInput({personaId, at}));
            }
        }
        : null;
    const resolveDeferredSleep = ({personaId, at, state} = {}) => {
        const life = repositories.blueprint?.read?.({personaId}) ?? {};
        const timezone = life.timezone || 'Asia/Shanghai';
        const sleeping = state?.sleeping === true
            || state?.isSleeping === true
            || state?.slotKind === 'baseline_sleep'
            || state?.slot_kind === 'baseline_sleep'
            || state?.slotKind === 'sleep'
            || state?.slot_kind === 'sleep';
        // Clock time alone is not evidence of sleep. A newly created persona
        // may be in a normal rest/baseline state at 23:00 without being asleep.
        if (!sleeping) return {sleeping: false, immediate: true, timezone};
        const relationship = repositories.relationship?.activePatch?.({personaId});
        const relationshipIntimacy = relationship?.intimacy
            ?? relationship?.intimacyScore
            ?? relationship?.intimacy_score;
        const intimacyNumber = Number(relationshipIntimacy);
        const intimacy = Number.isFinite(intimacyNumber) ? Math.max(0, Math.min(4, intimacyNumber)) : null;
        const affect = repositories.affect?.readSnapshot?.({personaId, at: at}) ?? null;
        const socialPressure = Math.max(0, Math.min(1, Number(affect?.drives?.social ?? 0.5)));
        const restPressure = Math.max(0, Math.min(1, Number(affect?.drives?.rest ?? 0.5)));
        const key = `${personaId}:${String(at).slice(0, 13)}`;
        const draw = Array.from(key).reduce((sum, char) => (sum * 33 + char.charCodeAt(0)) >>> 0, 17) % 100;
        return {
            sleeping: true,
            ...(intimacy === null ? {} : {intimacy}),
            draw,
            nextBoundaryAt: state?.nextBoundaryAt ?? state?.next_boundary_at ?? null,
            timezone,
            socialPressure,
            restPressure
        };
    };
    const sleepDecisionCompletion = createMtplxJsonCompletionPort({provider: mtplx, settings: readSettings, timeoutMs: 15_000});
    const sleepDecision = async ({personaId, at, state, availability, userMessage, text} = {}) => {
        const response = await sleepDecisionCompletion.complete({
            model: readSettings().model,
            messages: [
                {role: 'system', content: '你只负责判断睡眠中的聊天请求是否现在回复。只输出 JSON：{"immediate":true或false,"deliverAt":ISO时间或null,"reason":"简短原因"}。当前状态必须明确是睡眠；不要把普通休息当成睡眠。综合关系亲密度、情绪/驱动力、随机值和用户消息重要性做判断，不生成回复正文。'},
                {role: 'user', content: JSON.stringify({
                    at,
                    state: {
                        source: state?.source ?? null,
                        slotKind: state?.slotKind ?? state?.slot_kind ?? null,
                        sleeping: state?.sleeping === true || state?.isSleeping === true,
                        situation: state?.situation ?? '',
                        scene: state?.scene ?? '',
                        endsAt: state?.endsAt ?? state?.nextBoundaryAt ?? null
                    },
                    relationship: {intimacy: availability?.intimacy ?? null},
                    affect: {socialPressure: availability?.socialPressure ?? 0.5, restPressure: availability?.restPressure ?? 0.5},
                    randomDraw: availability?.draw ?? 0,
                    userMessage: String(userMessage?.text ?? text ?? '').slice(0, 1_000)
                })}
            ],
            trace: {personaId, messageId: userMessage?.id ?? null, operation: 'sleep-decision'}
        });
        const raw = response?.content ?? response?.text ?? '';
        const parsed = JSON.parse(String(raw).replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/i, '').trim());
        if (!isRecord(parsed) || typeof parsed.immediate !== 'boolean') throw new Error('Sleep decision schema is invalid');
        const deliverAt = typeof parsed.deliverAt === 'string' && Number.isFinite(Date.parse(parsed.deliverAt))
            && Date.parse(parsed.deliverAt) > Date.parse(at)
            && Date.parse(parsed.deliverAt) <= Date.parse(at) + 24 * 60 * 60_000
            ? new Date(parsed.deliverAt).toISOString()
            : null;
        const reason = typeof parsed.reason === 'string' && parsed.reason.trim()
            ? parsed.reason.trim().slice(0, 160)
            : 'sleep_decision';
        return {immediate: parsed.immediate, reason, ...(deliverAt ? {deliverAt} : {})};
    };
    const deferredChatPolicy = options.deferredChatPolicy ?? createDeferredChatPolicy({
        deferredBatch: repositories.deferredChatBatch,
        conversationRepository: repositories.conversation,
        lifeWorld: deferredLifeWorld,
        sleepAvailability: resolveDeferredSleep,
        sleepDecision,
        clock,
        idGenerator
    });
    const contextReader = {
        read({command = {}, messages = []} = {}) {
            const personaId = command.personaId ?? command?.command?.personaId;
            const persona = repositories.persona?.findActive?.(personaId);
            if (!persona) throw Object.assign(new Error('人格不存在'), {status: 404});
            const at = command.chatAt ?? clock();
            const worldInput = lifeWorldReader?.readResolverInput({personaId, at}) ?? null;
            const resolved = worldInput ? resolveLifeState(worldInput) : null;
            const state = resolved?.situation
                ? `${resolved.situation}（${resolved.scene || '日常场景'}；地点：${resolved.location || '未确认'}；房间：${resolved.room || '未确认'}；心情：${resolved.mood || '平静'}）`
                : '当前没有额外的已确认生活事件。';
            const memories = repositories.memory?.listActive?.({personaId, limit: 12}) ?? [];
            const selfModelClaims = repositories.selfModel?.listActive?.({personaId, limit: 12})
                ?? repositories.selfModelRepository?.listActive?.({personaId, limit: 12})
                ?? [];
            const boundedSelfModelClaims = (Array.isArray(selfModelClaims) ? selfModelClaims : [])
                .filter(claim => isRecord(claim) && typeof claim.summary === 'string' && claim.summary.trim())
                .slice(0, 12)
                .map(claim => ({
                    id: claim.id ?? null,
                    category: String(claim.category ?? '').slice(0, 80),
                    summary: claim.summary.trim().slice(0, 1_000),
                    status: claim.status ?? 'active',
                    revision: claim.revision ?? 1
                }));
            const relationship = repositories.relationship?.activePatch?.({personaId}) ?? null;
            let relationshipPatch = {};
            try { relationshipPatch = relationship?.next_patch ? JSON.parse(relationship.next_patch) : {}; } catch { relationshipPatch = {}; }
            const foundation = repositories.foundation?.getDraft?.({personaId}) ?? repositories.foundation?.draft?.({personaId}) ?? null;
            const imagePolicy = ['ask', 'always', 'important', 'user_only', 'autonomous'].includes(persona.image_generation_policy)
                ? persona.image_generation_policy : 'autonomous';
            const imagePolicyMeaning = {
                ask: '出现适合视觉记录的时刻时先自然询问用户。',
                always: '用户可见回复包含括号动作时必须调用 media_event；没有动作不强制生图。',
                important: '只在结合关系和上下文判断为重要的时刻调用 media_event。',
                user_only: '不要主动发起 media_event，只响应用户明确要求。',
                autonomous: '由人格结合上下文自然决定是否调用 media_event。'
            }[imagePolicy];
            const appearance = resolved?.appearance ?? worldInput?.state?.appearance ?? {};
            const affect = repositories.affect?.readSnapshot?.({personaId, at}) ?? null;
            const replyPosture = replyPostureFor(affect, affect?.drives);
            const timeFacts = {
                source: resolved?.source ?? 'unknown', startsAt: resolved?.startsAt ?? null,
                endsAt: resolved?.endsAt ?? null, timeFact: resolved?.timeFact ?? 'unknown',
                nextBoundaryAt: resolved?.nextBoundaryAt ?? null
            };
            const lifeLayer = [
                `生活状态：${state}`,
                `可信时间：source=${timeFacts.source}; startsAt=${timeFacts.startsAt || '无'}; endsAt=${timeFacts.endsAt || '无'}; nextBoundaryAt=${timeFacts.nextBoundaryAt || '无'}; timeFact=${timeFacts.timeFact}`,
                `外观变化：${JSON.stringify(appearance)}`,
                worldInput?.dailyPlan ? `当天计划：${JSON.stringify(worldInput.dailyPlan)}` : '当天没有已确认的日计划。',
                worldInput?.presence?.active ? `共同场景：${JSON.stringify(worldInput.presence)}` : '当前没有持久化共同场景。',
                `当前表达姿态：${JSON.stringify(replyPosture)}`
            ].join('\n');
            const identityLayer = `人格：${persona.name}；角色：${persona.role || '陪伴者'}；基础设定：${foundation?.foundation || '暂无'}。`;
            const relationshipLayer = `长期了解：${memories.map(memory => `${memory.memory_key ?? memory.memoryKey}:${memory.value}`).join('；') || '暂无'}。关系补丁：${JSON.stringify(relationshipPatch)}。`;
            const selfModelLayer = boundedSelfModelClaims.length
                ? `当前自我摘要：\n${boundedSelfModelClaims.map(claim => `${claim.category ? `${claim.category}：` : ''}${claim.summary}`).join('\n')}`
                : '当前没有已确认的自我模型摘要。';
            const capabilityLayer = [
                systemCapabilityPromptFor(),
                `【系统能力层：人格生图频率】当前人格偏好为“${imageGenerationPolicyLabels[imagePolicy]}”（${imagePolicy}）：${imagePolicyMeaning}。这是行为偏好，不是服务器关键词触发器。`,
                '工具调用由应用校验并提交；不要把工具参数、内部状态或系统标识写入用户可见回复。'
            ].join('\n');
            const contextFragments = contextPipeline.collect({fragments: [
                {source: 'identity', priority: 100, text: identityLayer},
                {source: 'life_state', priority: 90, value: lifeLayer},
                {source: 'memory', priority: 50, value: memories.slice(0, 12)},
                {source: 'relationship', priority: 40, value: relationshipPatch},
                {source: 'affect', priority: 45, value: replyPosture},
                {source: 'self_model', priority: 55, text: selfModelLayer}
            ]});
            const serializedContext = contextPipeline.serialize({budget: contextPipeline.budget(contextFragments)});
            return {
                persona: {id: persona.id, name: persona.name, role: persona.role, color: persona.color},
                prompt: `你是 ${persona.name}，角色是 ${persona.role || '陪伴者'}。请基于已确认事实与用户交流，不要编造当前状态。\n\n${serializedContext}\n\n${capabilityLayer}`,
                layers: {
                    lifeState: lifeLayer,
                    memory: JSON.stringify(memories.slice(0, 12)),
                    immutableIdentity: identityLayer,
                    relationship: relationshipLayer,
                    selfModel: selfModelLayer,
                    affect: JSON.stringify(replyPosture),
                    timeFacts,
                    systemCapability: capabilityLayer
                },
                capabilityPromptIncluded: true,
                settings: readSettings(),
                history: messages,
                state: resolved,
                lifeWorld: worldInput,
                memories,
                selfModelClaims: boundedSelfModelClaims,
                selfModelSummary: selfModelLayer,
                relationship: relationshipPatch,
                affect,
                drives: affect?.drives ?? {},
                replyPosture,
                imageGenerationPolicy: imagePolicy
            };
        },
        readContext(input = {}) { return this.read(input); }
    };
    const rawLlmStreamingPort = createMtplxCompletionPort({provider: mtplx, settings: readSettings, tools: COMPANION_CAPABILITY_TOOLS});
    const llmStreamingPort = createChatLlmStreamingPort({
        llmStreamingPort: rawLlmStreamingPort,
        stream: rawLlmStreamingPort,
        tools: COMPANION_CAPABILITY_TOOLS
    });
    const commitBoundary = createConversationCommitAdapter({
        repository: repositories.conversation,
        clock,
        idGenerator,
        transaction: options.transaction
    });
    const userMessageWriter = ({personaId, text: messageText, attachments = [], userMessageId} = {}) => {
        const userCreatedAt = new Date(Date.parse(clock()) - 1).toISOString();
        const conversation = repositories.conversation.getOrCreateConversation({
            personaId,
            id: idGenerator('conversation'),
            createdAt: userCreatedAt,
            updatedAt: userCreatedAt
        });
        const requestedId = typeof userMessageId === 'string' && userMessageId.trim() && userMessageId.trim().length <= 160
            ? userMessageId.trim()
            : null;
        if (requestedId && typeof repositories.conversation.findMessage === 'function') {
            const existing = repositories.conversation.findMessage({id: requestedId, personaId});
            if (existing) return existing;
        }
        return repositories.conversation.appendMessage({
            id: requestedId ?? idGenerator('message'),
            conversationId: conversation.id,
            role: 'user',
            text: String(messageText ?? ''),
            attachmentsJson: JSON.stringify(Array.isArray(attachments) ? attachments : []),
            generationJson: null,
            jobsJson: '[]',
            createdAt: userCreatedAt,
            readAt: userCreatedAt
        });
    };
    return {
        ...createChatProductionPorts({
            contextReader,
            llmStreamingPort,
            conversationRepository: repositories.conversation,
            clock,
            idGenerator
        }),
        conversationRepository: repositories.conversation,
        userMessageWriter,
        deferredChatPolicy,
        commitBoundary,
        sendSse(sink, event) {
            if (typeof sink?.write === 'function') sink.write(`data: ${JSON.stringify(event)}\n\n`);
        },
        end(sink) {
            if (typeof sink?.end === 'function') sink.end();
            else if (sink && typeof sink === 'object') sink.writableEnded = true;
        },
        enableContinuation: true
    };
}

function createDefaultMediaCapabilityOptions(options, repositories) {
    const settings = options.settings ?? repositories.settings;
    const readSettings = typeof settings === 'function' ? settings : settings?.read?.bind(settings) ?? (() => ({}));
    const normalizeMediaCapabilityCall = (value = {}) => {
        if (!isRecord(value)) throw new TypeError('media_event 参数必须是 JSON 对象');
        const kind = value.kind;
        if (kind !== 'image' && kind !== 'video') throw new TypeError('media_event.kind 无效');
        const count = value.count === undefined ? 1 : Number(value.count);
        if (!Number.isInteger(count) || count < 1 || count > 3) throw new RangeError('media_event.count 必须在 1 到 3 之间');
        const concept = isRecord(value.personaMediaConcept) ? value.personaMediaConcept : {
            schemaVersion: 1,
            mediaKind: kind,
            scene: String(value.request || '').slice(0, 800),
            action: String(value.request || '').slice(0, 800),
            mood: '', narrative: '', humanSubjects: [], nonHumanObjects: [],
            capture: {mode: 'other', operator: '', deviceVisibility: 'unspecified', framingIntent: ''},
            compositionIntent: ''
        };
        if (concept.mediaKind !== kind) throw new TypeError('media_event.personaMediaConcept.mediaKind 必须与 kind 一致');
        return {
            kind,
            count,
            request: typeof value.request === 'string' ? value.request.trim().slice(0, 500) : '',
            personaMediaConcept: concept,
            ...(value.currentEvent === null || isRecord(value.currentEvent) ? {currentEvent: value.currentEvent ?? null} : {}),
            ...(isRecord(value.temporaryAppearance) ? {temporaryAppearance: value.temporaryAppearance} : {})
        };
    };
    return {
        normalizeMediaCapabilityCall,
        mediaConceptEnvelopeFor(persona, input = {}) {
            const blueprint = repositories.blueprint?.read?.({personaId: persona?.id}) ?? {};
            return {
                schemaVersion: 1,
                mediaKind: input.kind,
                personaId: persona?.id ?? null,
                personaName: persona?.name ?? '',
                personaRole: persona?.role ?? '',
                request: input.request ?? '',
                currentEvent: input.currentEvent ?? null,
                temporaryAppearance: input.temporaryAppearance ?? {},
                immutableIdentity: {
                    foundation: blueprint.foundation ?? '',
                    identityProfile: blueprint.identityProfile ?? blueprint.identity ?? {},
                    appearance: blueprint.appearance ?? blueprint.visualBaseline ?? {},
                    style: blueprint.style ?? {}
                }
            };
        },
        providerFor(kind) {
            const config = readSettings();
            return config?.[`${kind}Provider`] || 'comfyui';
        }
    };
}

function isMediaJobService(value) {
    return isRecord(value) && (
        typeof value.register === 'function'
        || typeof value.submit === 'function'
        || isRecord(value.handlers)
        || isRecord(value.handlerMap)
    );
}

function hasMediaJobConfiguration(options) {
    return options.defaultProductionComposition === true
        || options.mediaJobServiceOptions !== undefined
        || options.mediaJobOptions !== undefined
        || (options.mediaJobService !== undefined && !isMediaJobService(options.mediaJobService))
        || [
            'mediaObservability',
            'observability',
            'observabilityPorts',
            'observabilityOptions',
            'mediaFlow',
            'mediaRepositories',
            'mediaProviderAdapters',
            'mediaPromptMaster',
            'mediaAcceptance',
            'promptMaster',
            'acceptance'
        ].some(name => options[name] !== undefined);
}

function resolveMediaComposition(options, repositories, providers, startup) {
    const configured = options.mediaJobService;
    // An explicit null disables the media worker. This is useful for a
    // composition that intentionally exposes no provider/job service; it must
    // not be replaced by the default production composition below.
    if (configured === null) return {observability: null, mediaJobService: null};
    if (isMediaJobService(configured)) return {
        observability: configured.observability ?? options.mediaObservability ?? options.observability ?? null,
        mediaJobService: configured
    };
    if (!hasMediaJobConfiguration(options)) return {observability: null, mediaJobService: null};

    const configuredService = isRecord(options.mediaJobService) && !isMediaJobService(options.mediaJobService)
        ? options.mediaJobService
        : {};
    const productionDefaults = options.defaultProductionComposition === true;
    const productionMediaFlow = productionDefaults && isOpenDatabase(startup?.database)
        && typeof (repositories.jobRepository ?? repositories.job)?.enqueue === 'function'
        ? createMediaJobRepository({
            database: startup.database,
            jobRepository: repositories.jobRepository ?? repositories.job,
            activityRepository: repositories.activity,
            conversationRepository: repositories.conversation,
            clock: options.clock,
            id: options.idGenerator ?? options.id
        })
        : null;
    const mtplx = typeof providers?.find === 'function'
        ? providers.find('mtplx', {portType: 'llm-streaming'})
        : providers?.get?.('mtplx', {portType: 'llm-streaming'});
    const defaultPromptMaster = productionDefaults && mtplx
        ? createMediaPromptMaster({provider: mtplx, settings: options.settings ?? repositories.settings})
        : null;
    const defaultAcceptance = productionDefaults ? createSkippedMediaAcceptance({clock: runtimeClock(options.clock)}) : null;
    const nested = {
        ...configuredService,
        ...(isRecord(options.mediaJobServiceOptions)
            ? options.mediaJobServiceOptions
            : isRecord(options.mediaJobOptions) ? options.mediaJobOptions : {})
    };
    const mediaJobService = createMediaJobApplication({
        ...nested,
        ...options,
        providers: nested.providers ?? options.providers ?? providers,
        repositories: nested.repositories ?? options.mediaRepositories ?? repositories,
        observability: nested.observability ?? options.mediaObservability ?? options.observability,
        providerAdapters: nested.providerAdapters ?? options.mediaProviderAdapters,
        mediaFlow: nested.mediaFlow ?? options.mediaFlow ?? productionMediaFlow,
        promptMaster: nested.promptMaster ?? options.mediaPromptMaster ?? options.promptMaster ?? defaultPromptMaster,
        acceptance: nested.acceptance ?? options.mediaAcceptance ?? options.acceptance ?? defaultAcceptance,
        clock: nested.clock ?? options.clock
    });
    return {
        observability: mediaJobService.observability ?? options.mediaObservability ?? options.observability ?? null,
        mediaJobService
    };
}

function resolveProactiveJobService(options, repositories, startup, providers) {
    if (options.proactiveJobService !== undefined) return options.proactiveJobService;
    const configured = options.proactiveJobServiceOptions ?? options.proactiveJobOptions;
    const hasFlows = options.proactiveFlows !== undefined
        || options.jobFlows !== undefined
        || options.applicationFlows !== undefined
        || options.proactiveFlowRegistry !== undefined
        || options.flowRegistry !== undefined;
    if (configured === undefined && !hasFlows && options.defaultProductionComposition !== true) return null;
    const nested = isRecord(configured) ? configured : {};
    const lifeWorldReady = isRecord(repositories.blueprint)
        && isRecord(repositories.schedule)
        && isRecord(repositories.dailyPlan)
        && isRecord(repositories.presence);
    const lifeWorldReader = lifeWorldReady
        ? createLifeWorldReader({repositories, blueprintReader: repositories.blueprint, clock: options.clock})
        : null;
    const resolveLifeState = createLifeStateResolver();
    const lifeWorld = lifeWorldReader
        ? {read({personaId, at} = {}) { return resolveLifeState(lifeWorldReader.readResolverInput({personaId, at})); }}
        : null;
    const proactiveMediaFlow = options.mediaFlow
        ?? (options.defaultProductionComposition === true
            && typeof options.normalizeMediaCapabilityCall === 'function'
            && repositories.conversation
            && repositories.job
            ? createMediaFlow({
                repositories,
                clock: options.clock,
                idGenerator: options.idGenerator ?? options.id,
                normalizeMediaCapabilityCall: options.normalizeMediaCapabilityCall,
                mediaConceptEnvelopeFor: options.mediaConceptEnvelopeFor,
                providerFor: options.providerFor,
                transaction: runtimeTransaction(options, startup),
                effectAdapter: options.effectAdapter
            })
            : null);
    const canComposeDefaultFlows = options.defaultProductionComposition === true
        && isOpenDatabase(startup?.database)
        && repositories.conversation
        && repositories.lifeEvent
        && repositories.activity
        && repositories.job
        && typeof repositories.job.enqueue === 'function';
    const defaultFlows = canComposeDefaultFlows
        ? createProductionProactiveFlows({
            database: startup.database,
            repositories,
            provider: typeof providers?.find === 'function'
                ? providers.find('mtplx', {portType: 'llm-streaming'})
                : providers?.get?.('mtplx', {portType: 'llm-streaming'}),
            settings: options.settings ?? repositories.settings,
            clock: options.clock,
            id: options.idGenerator ?? options.id,
            lifeWorld,
            contextReader: options.contextReader,
            mediaFlow: proactiveMediaFlow,
            transaction: runtimeTransaction(options, startup)
        })
        : undefined;
    const flowRegistry = options.flowRegistry;
    const flows = nested.flows ?? options.proactiveFlows ?? options.jobFlows ?? options.applicationFlows ?? defaultFlows;
    if (flowRegistry && isRecord(flows)) {
        const ids = {
            proactive_message: 'proactive-message',
            pending_event: 'pending-event',
            activity_decision: 'activity-decision',
            deferred_chat_reply: 'deferred-chat-reply'
        };
        for (const [type, flow] of Object.entries(flows)) {
            registerFlowAdapter(flowRegistry, {id: ids[type] ?? type.replaceAll('_', '-'), flow, version: flow?.version ?? 1});
        }
    }
    return createProactiveJobService({
        ...nested,
        ...options,
        repositories: nested.repositories ?? repositories,
        flows,
        flowRegistry: nested.flowRegistry ?? options.proactiveFlowRegistry ?? flowRegistry,
        ports: nested.ports ?? options.proactivePorts ?? options.applicationPorts
    });
}

function runtimeClock(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.now === 'function') return value.now.bind(value);
    return () => new Date().toISOString();
}

function runtimeId(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.next === 'function') return value.next.bind(value);
    return prefix => `${prefix}_${crypto.randomUUID()}`;
}

function runtimeTransaction(options, startup) {
    if (typeof options.transaction === 'function' || typeof options.transaction?.run === 'function' || typeof options.transaction?.transaction === 'function') return options.transaction;
    const database = startup?.database;
    if (!database || typeof database.transaction !== 'function') return undefined;
    return work => database.inTransaction ? work() : database.transaction(work)();
}

function resolveRepositories(options, startup) {
    if (options.repositories !== undefined) {
        if (!isRecord(options.repositories)) throw new TypeError('Runtime repositories must be an object');
        const job = options.repositories.jobRepository ?? options.repositories.job ?? options.repositories.effectRepository ?? options.jobRepository;
        if (!job || (options.repositories.job === job && options.repositories.jobRepository === job)) return options.repositories;
        return Object.freeze({...options.repositories, job, jobRepository: job});
    }
    const database = startup.database;
    const clock = runtimeClock(options.clock);
    const id = runtimeId(options.idGenerator ?? options.id);
    const job = createJobRepository({database, clock, id});
    const pending = createPendingEventRepository({database, enqueueJob: job.enqueue.bind(job)});
    const deferredChatBatch = createProductionDeferredChatBatchRepository({database, jobRepository: job, clock, id});
    const settings = createSettingsRepository({database, defaults: () => ({}), clock});
    const personaLifecycle = createPersonaLifecycleRepository({database, clock, id, jobRepository: job});
    const interview = createInterviewRepository({database, clock, id, personaLifecycle});
    const supportingCharacter = createSupportingCharacterRepository({database});
    const timeline = createTimelineRepository({database, clock, id});
    const blueprint = createBlueprintRepository({database});
    const memory = createMemoryRepository({database, clock});
    const interactionFact = createInteractionFactRepository({database, clock, id});
    const appraisal = createAppraisalRepository({database, clock, id});
    const memoryConsolidation = createMemoryConsolidationRepository({database, clock, id});
    const selfModel = createSelfModelRepository({database, clock, id});
    const agencyIntention = createAgencyIntentionRepository({database, clock, id});
    return Object.freeze({
        conversation: createConversationRepository({database}),
        activity: createActivityRepository({database}),
        supportingCharacter,
        supportingCharacterRepository: supportingCharacter,
        timeline,
        eventDecision: timeline,
        eventDecisionRepository: timeline,
        timelineSlot: timeline,
        timelineSlotRepository: timeline,
        job,
        jobRepository: job,
        deferredChatBatch,
        pending,
        pendingEvent: pending,
        lifeEvent: createLifeEventRepository({database, clock, id}),
        persona: createPersonaRepository({database, clock, id}),
        group: createGroupRepository({database, clock, id}),
        memory,
        affect: createAffectRepository({database, clock, id, blueprintRepository: blueprint}),
        interactionFact,
        interactionFactRepository: interactionFact,
        appraisal,
        appraisalRepository: appraisal,
        memoryConsolidation,
        memoryConsolidationRepository: memoryConsolidation,
        selfModel,
        selfModelRepository: selfModel,
        agencyIntention,
        agencyIntentionRepository: agencyIntention,
        relationship: createRelationshipRepository({database, clock, id}),
        settings,
        personaLifecycle,
        foundation: createFoundationRepository({database, clock, id}),
        schedule: createScheduleRepository({database, clock, id}),
        interview,
        mediaAsset: createMediaAssetRepository({database}),
        promptRun: createPromptRunRepository({database, clock}),
        state: createStateRepository({database, clock}),
        blueprint,
        dailyPlan: createDailyPlanRepository({database, blueprintRepository: blueprint, jobRepository: job, clock, id}),
        presence: createPresenceRepository({database})
    });
}

function descriptorHandler(value, fallbackReceiver) {
    if (typeof value === 'function') return {handler: value, receiver: fallbackReceiver};
    if (!isRecord(value)) return {handler: null, receiver: undefined};
    const handler = value.handler ?? value.run ?? value.handle;
    const receiver = value.receiver
        ?? (typeof value.run === 'function'
            || typeof value.handle === 'function'
            || (typeof value.handler === 'function' && fallbackReceiver === undefined)
            ? value
            : fallbackReceiver);
    return {handler: typeof handler === 'function' ? handler : null, receiver};
}

function addJobHandler(target, type, value, source) {
    if (typeof type !== 'string' || type.trim() === '') throw new TypeError(`${source} job type must be a non-empty string`);
    const {handler, receiver} = descriptorHandler(value);
    if (!handler) throw new TypeError(`${source} handler for ${type} must be a function`);
    const normalizedType = type.trim();
    const existing = target.get(normalizedType);
    if (existing && (existing.handler !== handler || existing.receiver !== receiver)) {
        throw new Error(`Job type already registered: ${normalizedType}`);
    }
    target.set(normalizedType, Object.freeze({handler, receiver}));
}

function collectJobHandlers(target, input, source) {
    if (input === undefined || input === null) return;
    if (input instanceof Map) {
        for (const [type, value] of input) addJobHandler(target, type, value, source);
        return;
    }
    if (Array.isArray(input)) {
        for (const value of input) {
            if (!isRecord(value)) throw new TypeError(`${source} handlers must contain registration objects`);
            addJobHandler(target, value.type ?? value.jobType ?? value.job_type, value, source);
        }
        return;
    }
    if (!isRecord(input)) throw new TypeError(`${source} handlers must be a map, array, or object`);
    for (const [type, value] of Object.entries(input)) addJobHandler(target, type, value, source);
}

function mediaJobRegistrations(service) {
    if (!isRecord(service)) throw new TypeError('Runtime mediaJobService must be an object');
    if (typeof service.registrations === 'function') return service.registrations();
    const map = service.handlers ?? service.handlerMap;
    if (map instanceof Map) return [...map].map(([type, value]) => ({type, value, receiver: map}));
    if (isRecord(map)) return Object.entries(map).map(([type, value]) => ({type, value, receiver: map}));
    if (typeof service.list === 'function' && typeof service.get === 'function') {
        return service.list().map(type => ({type, handler: service.get(type), receiver: service}));
    }
    if (typeof service.register === 'function') {
        const registrations = [];
        service.register({register(type, handler, receiver) { registrations.push({type, handler, receiver}); }});
        return registrations;
    }
    throw new TypeError('Runtime mediaJobService must expose handlers or register()');
}

function mediaJobHandlers(service) {
    const handlers = new Map();
    for (const registration of mediaJobRegistrations(service)) {
        const {handler, receiver} = registration.value !== undefined
            ? descriptorHandler(registration.value, registration.receiver)
            : descriptorHandler(registration, service);
        if (!handler) throw new TypeError(`Runtime mediaJobService handler for ${registration?.type} must be a function`);
        // Media application handlers perform guarded projections themselves.
        // The generic dispatcher owns the one durable job transition when they
        // run inside a worker, so defer the service's repository settlement.
        const delegated = async (job, context = {}) => handler.call(receiver ?? service, job, {...context, deferSettlement: true});
        addJobHandler(handlers, registration.type, delegated, 'Runtime mediaJobService');
    }
    return handlers;
}

function resolveJobHandlers(options) {
    const handlers = new Map();
    if (options.mediaJobService !== undefined && options.mediaJobService !== null) {
        for (const [type, value] of mediaJobHandlers(options.mediaJobService)) addJobHandler(handlers, type, value, 'Runtime mediaJobService');
    }
    if (options.proactiveJobService !== undefined && options.proactiveJobService !== null) {
        const service = options.proactiveJobService;
        const handlerMap = service.handlers ?? service.handlerMap;
        const registrations = typeof service.registrations === 'function'
            ? service.registrations()
            : Object.entries(handlerMap ?? {}).map(([type, handler]) => ({type, handler, receiver: handlerMap}));
        if (!Array.isArray(registrations)) throw new TypeError('Runtime proactiveJobService registrations must be an array');
        for (const registration of registrations) {
            if (registration?.available === false) continue;
            addJobHandler(handlers, registration?.type, registration, 'Runtime proactiveJobService');
        }
    }
    // Keep registration sources separate so a duplicate type cannot be
    // silently overwritten by object spread before addJobHandler() sees it.
    collectJobHandlers(handlers, options.maintenanceHandlers, 'Runtime maintenance');
    collectJobHandlers(handlers, options.jobHandlers, 'Runtime jobHandlers');
    collectJobHandlers(handlers, options.jobRegistry, 'Runtime jobRegistry');
    collectJobHandlers(handlers, options.handlers, 'Runtime handlers');
    return handlers.size ? handlers : undefined;
}

function resolveApplicationFlows(options, repositories, startup) {
    const clock = options.clock;
    const idGenerator = options.idGenerator ?? options.id;
    const transaction = runtimeTransaction(options, startup);
    const effectAdapter = options.effectAdapter
        ?? ((repositories.job ?? repositories.jobRepository)?.enqueue
            ? createFlowEffectAdapter({jobRepository: repositories.job ?? repositories.jobRepository, clock, idGenerator})
            : null);
    const lifeEventFlow = options.lifeEventFlow ?? (repositories.lifeEvent || repositories.life
        ? createLifeEventFlow({repositories, clock, idGenerator, transaction, effectAdapter})
        : null);
    const timelineReady = repositories.eventDecisionRepository
        || repositories.timelineDecisionRepository
        || repositories.decisionRepository
        || repositories.eventDecision
        || repositories.decisions;
    const timelineFlow = options.timelineFlow ?? (timelineReady && lifeEventFlow
        ? createTimelineFlow({repositories, lifeEventFlow, clock, idGenerator, transaction, effectAdapter})
        : null);
    const relationshipFlow = options.relationshipFlow ?? (repositories.relationship || repositories.relationshipRepository
        ? createRelationshipFlow({repositories, clock, idGenerator, transaction, evaluator: options.relationshipEvaluator, effectAdapter})
        : null);
    const pendingReady = repositories.pending || repositories.pendingEvent;
    const pendingEventFlow = options.pendingEventFlow ?? (pendingReady && effectAdapter
        ? createPendingEventFlow({repositories, clock, idGenerator, transaction, effectAdapter, normalizeCall: options.normalizePendingEventCall})
        : null);
    const sceneEventFlow = options.sceneEventFlow ?? (repositories.state
        ? createSceneEventFlow({repositories, clock, idGenerator, transaction, normalizeCall: options.normalizeSceneEventCall, scheduledState: options.scheduledState})
        : null);
    const appearanceEventFlow = options.appearanceEventFlow ?? (repositories.state
        ? createAppearanceEventFlow({repositories, clock, idGenerator, transaction, normalizeCall: options.normalizeAppearanceEventCall})
        : null);
    const memoryService = options.memoryService ?? (repositories.memory
        ? createMemoryService({repositories, clock})
        : null);
    const memoryEventFlow = options.memoryEventFlow ?? (repositories.memory
        ? createMemoryEventFlow({repositories, clock, idGenerator, transaction, memoryService})
        : null);
    const affectFlow = options.affectFlow ?? (repositories.affect
        ? createAffectFlow({repositories, clock, idGenerator, transaction})
        : null);
    const appraisalFlow = options.appraisalFlow ?? (repositories.interactionFact && repositories.appraisal
        ? createAppraisalFlow({repositories, affectFlow, clock, idGenerator, transaction})
        : null);
    const memoryConsolidationFlow = options.memoryConsolidationFlow
        ?? (repositories.memoryConsolidation || repositories.memoryConsolidationRepository
            ? createMemoryConsolidationFlow({repositories, clock, idGenerator, transaction})
            : null);
    const selfModelFlow = options.selfModelFlow
        ?? (repositories.selfModel || repositories.selfModelRepository || repositories.selfModelClaims
            ? createSelfModelFlow({repositories, clock, idGenerator, transaction})
            : null);
    const agencyIntentionFlow = options.agencyIntentionFlow
        ?? (repositories.agencyIntention || repositories.agencyIntentionRepository
            ? createAgencyIntentionFlow({repositories, clock, idGenerator, transaction})
            : null);
    return {
        lifeEventFlow,
        timelineFlow,
        relationshipFlow,
        pendingEventFlow,
        sceneEventFlow,
        appearanceEventFlow,
        memoryService,
        memoryEventFlow,
        affectFlow,
        appraisalFlow,
        memoryConsolidationFlow,
        selfModelFlow,
        agencyIntentionFlow,
        effectAdapter,
        flowRegistry: options.flowRegistry
    };
}

function resolveMaintenanceHandlers(options, repositories, flows = {}) {
    const hasExplicitFlow = Boolean(options.timelineFlow || options.relationshipFlow || options.relationshipEvolution || options.relationshipEvolutionFlow || flows.timelineFlow || flows.relationshipFlow);
    if (options.defaultProductionComposition !== true || (options.repositories !== undefined && !hasExplicitFlow)) return {};
    const timelineFlow = flows.timelineFlow ?? options.timelineFlow;
    const relationshipFlow = flows.relationshipFlow ?? options.relationshipFlow;
    return {
        daily_plan: async (job, context = {}) => {
            const payload = typeof job.payload_json === 'string' ? JSON.parse(job.payload_json || '{}') : (job.payload ?? {});
            const at = context.now ?? runtimeClock(options.clock)();
            const personaId = job.persona_id ?? job.personaId ?? payload.personaId ?? payload.persona_id;
            const dailyPlan = repositories.dailyPlan;
            const ensure = dailyPlan?.ensure ?? dailyPlan?.ensureDailyPlan;
            const payloadPlanId = payload.dailyPlanId ?? payload.daily_plan_id ?? payload.id;

            // External compositions written before ensure() existed still own
            // their repository and plan lifecycle. Keep that adapter path, but
            // only report success after a real ready row is returned.
            if (typeof ensure !== 'function') {
                const stored = payloadPlanId
                    ? (dailyPlan?.readById?.({personaId, dailyPlanId: payloadPlanId})
                        ?? dailyPlan?.findById?.({personaId, id: payloadPlanId})
                        ?? dailyPlan?.read?.({personaId, at})
                        ?? null)
                    : dailyPlan?.read?.({personaId, at}) ?? null;
                const resolvedDailyPlanId = payloadPlanId ?? stored?.id;
                const plan = resolvedDailyPlanId ? dailyPlan?.markReady?.({personaId, dailyPlanId: resolvedDailyPlanId, updatedAt: at}) : stored;
                if (!plan || plan.status !== 'ready') throw new Error('Daily plan is not ready');
                const rawPlanInput = isRecord(plan.plan)
                    ? plan.plan
                    : isRecord(payload.plan) ? payload.plan : isRecord(stored?.plan) ? stored.plan : undefined;
                const planInput = isRecord(rawPlanInput) ? {...rawPlanInput, status: 'ready'} : rawPlanInput;
                const slots = timelineFlow?.syncDailyPlanSlots
                    ? await timelineFlow.syncDailyPlanSlots({personaId, planDate: payload.planDate ?? payload.plan_date ?? plan.planDate ?? plan.plan_date, plan: planInput, at})
                    : null;
                if (slots && (slots.skipped || !Array.isArray(slots.slots))) throw new Error('Daily plan slots were not synced');
                return {status: 'complete', result: {dailyPlanId: resolvedDailyPlanId, status: plan.status, slots: slots?.slots ?? []}};
            }

            const blueprint = repositories.blueprint?.read?.({personaId});
            if (!isRecord(blueprint) || typeof blueprint.timezone !== 'string' || !blueprint.timezone.trim()) throw new Error('Daily plan blueprint is missing');
            const timezone = timezoneFor(blueprint);
            const localToday = localDateFor(at, timezone);
            const requestedDate = payload.planDate ?? payload.plan_date;
            const sourcePlan = payloadPlanId
                ? (dailyPlan.readById?.({personaId, dailyPlanId: payloadPlanId}) ?? dailyPlan.findById?.({personaId, id: payloadPlanId}))
                : null;
            const startDate = requestedDate ?? sourcePlan?.planDate ?? sourcePlan?.plan_date ?? localToday;
            if (!isPlanDate(startDate)) throw new TypeError('Daily plan job requires a valid planDate');
            const endDate = startDate > localToday ? startDate : localToday;
            const distance = Math.floor((Date.parse(`${endDate}T00:00:00.000Z`) - Date.parse(`${startDate}T00:00:00.000Z`)) / 86_400_000);
            const maxCatchUpDays = 366;
            if (!Number.isFinite(distance) || distance < 0) throw new TypeError('Daily plan job date range is invalid');
            if (distance > maxCatchUpDays) return {status: 'retry', error: 'Daily plan catch-up span exceeds limit', result: {startDate, endDate, maxCatchUpDays}};

            const ensured = [];
            const syncedSlots = [];
            let date = startDate;
            for (let index = 0; index <= distance; index += 1) {
                const plan = await ensure.call(dailyPlan, {
                    personaId,
                    planDate: date,
                    at,
                    runAfter: zonedInstant(date, '00:00', timezone) ?? at,
                    blueprint
                });
                const planOwner = plan?.persona_id ?? plan?.personaId;
                const planDate = plan?.plan_date ?? plan?.planDate;
                if (!plan || plan.status !== 'ready' || planOwner !== personaId || planDate !== date) throw new Error(`Daily plan is not ready for ${date}`);
                if (typeof timelineFlow?.syncDailyPlanSlots !== 'function') throw new Error('Daily plan timeline sync is unavailable');
                const slots = await timelineFlow.syncDailyPlanSlots({personaId, planDate: date, plan: plan.plan ?? plan.dailyPlan ?? plan, at});
                if (!slots || slots.skipped || !Array.isArray(slots.slots)) throw new Error(`Daily plan slots were not synced for ${date}`);
                const restored = await ensure.call(dailyPlan, {
                    personaId,
                    planDate: date,
                    at,
                    runAfter: zonedInstant(date, '00:00', timezone) ?? at,
                    blueprint
                });
                const restoredOwner = restored?.persona_id ?? restored?.personaId;
                const restoredDate = restored?.plan_date ?? restored?.planDate;
                if (!restored || restored.status !== 'ready' || restoredOwner !== personaId || restoredDate !== date) throw new Error(`Daily plan baseline is not ready for ${date}`);
                ensured.push({dailyPlanId: plan.id ?? plan.planId, planDate: date});
                syncedSlots.push(...slots.slots);
                date = nextDate(date);
            }

            const nextPlan = await ensure.call(dailyPlan, {
                personaId,
                planDate: date,
                at,
                runAfter: zonedInstant(date, '00:00', timezone) ?? at,
                blueprint
            });
            const nextPlanOwner = nextPlan?.persona_id ?? nextPlan?.personaId;
            const nextPlanDate = nextPlan?.plan_date ?? nextPlan?.planDate;
            if (!nextPlan || nextPlan.status !== 'ready' || nextPlanOwner !== personaId || nextPlanDate !== date) throw new Error(`Next daily plan is not ready for ${date}`);
            return {
                status: 'complete',
                result: {
                    dailyPlanId: ensured[ensured.length - 1]?.dailyPlanId ?? nextPlan.id ?? nextPlan.planId,
                    planDate: endDate,
                    status: 'ready',
                    caughtUpDates: ensured.map(item => item.planDate),
                    nextPlanDate: date,
                    slots: syncedSlots
                }
            };
        },
        timeline_candidate: async (job, context) => {
            if (typeof timelineFlow?.handleJob === 'function') return timelineFlow.handleJob(job, context);
            return {status: 'complete', result: {skipped: 'timeline flow not configured'}};
        },
        timeline_reconcile: async (job, context) => {
            if (typeof timelineFlow?.handleJob === 'function') return timelineFlow.handleJob(job, context);
            return {status: 'complete', result: {skipped: 'timeline flow not configured'}};
        },
        relationship_evolution: async job => {
            const handler = options.relationshipEvolution ?? options.relationshipEvolutionFlow;
            if (typeof handler === 'function') return handler(job);
            if (typeof relationshipFlow?.handleJob === 'function') return relationshipFlow.handleJob(job, {now: typeof options.clock === 'function' ? options.clock() : options.clock?.now?.() ?? new Date().toISOString()});
            return {status: 'complete', result: {skipped: 'relationship evolution flow not configured'}};
        }
    };
}

function registerJobHandlers(target, handlers) {
    if (!handlers?.size) return target;
    if (typeof target.register !== 'function') throw new TypeError('Runtime jobDispatcher must provide register() when mediaJobService is injected');
    for (const [type, value] of handlers) target.register(type, value.handler, value.receiver);
    return target;
}

function resolveJobDispatcher(options, startup, repositories) {
    const configured = options.jobDispatcher;
    const handlers = resolveJobHandlers(options);
    if (configured !== undefined) {
        if (!isRecord(configured) || typeof configured.runJob !== 'function' || typeof configured.jobTick !== 'function') {
            throw new TypeError('Runtime jobDispatcher must provide runJob() and jobTick()');
        }
        return registerJobHandlers(configured, handlers);
    }
    if (handlers === undefined) return null;
    return createJobDispatcher({
        jobRepository: options.jobRepository ?? repositories?.jobRepository ?? repositories?.job,
        handlers,
        clock: options.clock,
        receiver: options.jobHandlerReceiver,
        onRetry: options.onJobRetry,
        onTerminal: options.onJobTerminal,
        onSettled: options.onJobSettled
    });
}

function resolveAuxiliaryRuntimes(options) {
    const configured = options.auxiliaryRuntimes ?? options.auxiliaryRuntime ?? [];
    const values = Array.isArray(configured) ? configured : [configured];
    if (values.length === 1 && values[0] === undefined) return Object.freeze([]);
    for (const runtime of values) {
        if (!isRecord(runtime) || typeof runtime.start !== 'function' || typeof runtime.stop !== 'function') {
            throw new TypeError('Runtime auxiliaryRuntimes must provide start() and stop()');
        }
    }
    return Object.freeze(values.slice());
}

function closeServer(server) {
    if (!server || typeof server.close !== 'function') return Promise.resolve();
    if (server.listening === false) return Promise.resolve();
    return new Promise((resolve, reject) => {
        server.close(error => error ? reject(error) : resolve());
    });
}

function listen(app, port, host) {
    if (!app || typeof app.listen !== 'function') throw new TypeError('Runtime app must provide listen()');
    return new Promise((resolve, reject) => {
        let settled = false;
        const onError = error => {
            if (settled) return;
            settled = true;
            reject(error);
        };
        const onListening = () => {
            // Express invokes the callback from the underlying `listen`
            // call, but a restricted host can emit `error` immediately after
            // that callback. Defer resolution until the handle confirms it is
            // actually listening so startup cannot report a false positive.
            const finish = () => {
                if (settled) return;
                if (server?.listening === false) return;
                settled = true;
                resolve(server ?? app);
            };
            if (server?.listening === false) {
                if (typeof setImmediate === 'function') setImmediate(finish);
                else queueMicrotask(finish);
                return;
            }
            finish();
        };
        app.once?.('error', onError);
        let server;
        try {
            server = app.listen(port, host, onListening);
        } catch (error) {
            onError(error);
            return;
        }
        if (server && typeof server.once === 'function') server.once('error', onError);
        if (server && typeof server.then === 'function') server.then(onListening, onError);
    });
}

/**
 * Compose the modular runtime without importing the legacy application root.
 * Startup opens SQLite eagerly, while HTTP binding and worker ownership begin
 * only when start() is called. All route and job behavior remains injected.
 */
export function createRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Runtime options must be an object');
    const environment = options.environment ?? options.env ?? process.env;
    if (!isRecord(environment)) throw new TypeError('Runtime environment must be an object');
    const flowRegistry = options.flowRegistry ?? createFlowRegistry();
    const startup = resolveStartup({...options, environment});
    const repositories = resolveRepositories(options, startup);
    const debugInspectorEnabled = options.debugInspectorEnabled === true
        || options.httpOptions?.debugInspectorEnabled === true
        || persistedDebugInspector(options.settings ?? repositories.settings);
    const hasExplicitProviders = options.providerRegistry !== undefined
        || options.providerAdapters !== undefined
        || options.mediaProviderAdapters !== undefined
        || options.providers !== undefined;
    const providers = options.defaultProductionComposition === true && !hasExplicitProviders
        ? defaultProductionProviders(options, environment, repositories)
        : resolveProviders(options, environment, repositories);
    const interviewProvider = typeof providers?.find === 'function'
        ? providers.find('mtplx', {portType: 'llm-streaming'})
        : providers?.get?.('mtplx', {portType: 'llm-streaming'});
    const interviewJsonCompletion = options.interviewJsonCompletion
        ?? (interviewProvider ? createMtplxJsonCompletionPort({
            provider: interviewProvider,
            settings: options.settings ?? repositories.settings,
            timeoutMs: options.interviewAnalyzerTimeoutMs
        }) : null);
    const interviewAnalyzer = options.personaAnalyzer
        ?? options.interviewAnalyzer
        ?? (interviewJsonCompletion ? createInterviewAnalyzer({jsonCompletion: interviewJsonCompletion}) : null);
    const jobRepository = options.jobRepository ?? repositories.jobRepository ?? repositories.job;
    const mediaComposition = resolveMediaComposition(options, repositories, providers, startup);
    const mediaJobService = mediaComposition.mediaJobService;
    const mediaObservability = mediaComposition.observability;
    const transaction = runtimeTransaction(options, startup);
    const applicationFlows = resolveApplicationFlows({...options, flowRegistry}, repositories, startup);
    const explicitChatPorts = options.chatProductionPorts
        ?? options.productionChatPorts
        ?? options.chatOptions
        ?? options.chatPorts
        ?? ['contextReader', 'llmStreamingPort', 'llmStreamPort', 'llm', 'conversationRepository', 'conversation', 'capabilityDispatcher', 'commitBoundary', 'commit', 'sendSse', 'end']
            .some(name => options[name] !== undefined);
    const chatProductionPorts = options.chatProductionPorts
        ?? options.productionChatPorts
        ?? (options.defaultProductionComposition === true && !explicitChatPorts
            ? createDefaultChatProductionPorts(options, repositories, providers)
            : null);
    const mediaCapabilityOptions = options.defaultProductionComposition === true
        ? createDefaultMediaCapabilityOptions(options, repositories)
        : {};
    const proactiveJobService = resolveProactiveJobService({
        ...options,
        flowRegistry,
        ...mediaCapabilityOptions,
        contextReader: options.contextReader ?? chatProductionPorts?.contextReader
    }, repositories, startup, providers);
    const h3Helpers = h3RuntimeHelpers({id: runtimeId(options.idGenerator ?? options.id)});
    const settingsPolicy = options.settingsPolicy ?? createSettingsPolicy({
        providers,
        h3Inspector: h3Helpers.inspectH3Configuration
    });
    const maintenanceHandlers = resolveMaintenanceHandlers(options, repositories, applicationFlows);
    const jobDispatcher = resolveJobDispatcher({
        ...options,
        jobRepository,
        mediaJobService,
        proactiveJobService,
        maintenanceHandlers
    }, startup, repositories);
    const application = options.application ?? options.applicationFactory?.({
        ...options,
        debugInspectorEnabled,
        flowRegistry,
        transaction,
        repositories,
        ...applicationFlows,
        jobRepository,
        providers,
        jobDispatcher,
        mediaJobService,
        mediaObservability,
        proactiveJobService,
        chatProductionPorts,
        settingsPolicy,
        ...mediaCapabilityOptions,
        interviewAnalyzer
    });
    const worker = resolveWorker({...options, jobRepository, jobDispatcher}, startup, repositories);
    const app = resolveApp({...options, debugInspectorEnabled, application, routeHandlers: options.routeHandlers ?? application?.routeHandlers}, startup, worker);
    const auxiliaryRuntimes = resolveAuxiliaryRuntimes(options);
    const port = resolvePort(options.port, environment);
    const host = nonEmpty(options.host ?? environment.HOST, DEFAULT_HOST);

    let phase = 'created';
    let server = null;
    let startPromise = null;
    let stopPromise = null;

    async function start({listen: shouldListen = true, worker: shouldStartWorker = true} = {}) {
        if (phase === 'running') return server;
        if (phase === 'starting') return startPromise;
        if (phase === 'stopping') return stopPromise.then(() => start({listen: shouldListen, worker: shouldStartWorker}));
        phase = 'starting';
        let pendingStart;
        pendingStart = (async () => {
            try {
                if (shouldListen) server = await listen(app, port, host);
                if (shouldStartWorker && worker) await worker.start();
                for (const auxiliary of auxiliaryRuntimes) await auxiliary.start();
                phase = 'running';
                return server;
            } catch (error) {
                for (const auxiliary of [...auxiliaryRuntimes].reverse()) await auxiliary.stop().catch(() => {});
                if (worker) await worker.stop().catch(() => {});
                await closeServer(server).catch(() => {});
                server = null;
                phase = 'created';
                throw error;
            } finally {
                if (startPromise === pendingStart) startPromise = null;
            }
        })();
        startPromise = pendingStart;
        return pendingStart;
    }

    async function stop() {
        if (phase === 'stopped') return false;
        if (phase === 'created') {
            startup.close();
            phase = 'stopped';
            return true;
        }
        if (phase === 'stopping') return stopPromise;
        phase = 'stopping';
        let pendingStop;
        pendingStop = (async () => {
            try {
                for (const auxiliary of [...auxiliaryRuntimes].reverse()) await auxiliary.stop({waitForTasks: true});
                if (worker) await worker.stop({waitForTasks: true});
                await closeServer(server);
                server = null;
                startup.close();
                phase = 'stopped';
                return true;
            } finally {
                if (stopPromise === pendingStop) stopPromise = null;
            }
        })();
        stopPromise = pendingStop;
        return pendingStop;
    }

    return Object.freeze({
        startup,
        database: startup.database,
        databaseConfig: startup.databaseConfig,
        repositories,
        jobRepository,
        providers,
        debugInspectorEnabled,
        flowRegistry,
        jobDispatcher,
        mediaObservability,
        mediaJobService,
        proactiveJobService,
        application,
        app,
        worker,
        auxiliaryRuntimes,
        port,
        host,
        start,
        stop,
        get server() {
            return server;
        },
        get state() {
            return phase;
        }
    });
}

export function createCompanionRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Companion runtime options must be an object');
    const applicationFactory = options.application
        ? undefined
        : resolved => {
            const identitySettings = options.identitySettingsService ?? createIdentitySettingsService({
                repositories: resolved.repositories,
                settings: options.settings ?? resolved.repositories.settings,
                providers: resolved.providers,
                settingsPolicy: resolved.settingsPolicy ?? options.settingsPolicy,
                defaultTimezone: options.defaultTimezone,
                debugInspector: resolved.debugInspectorEnabled === true
            });
            const activityService = options.activityService ?? createActivityService({
                repositories: resolved.repositories,
                clock: resolved.clock,
                idGenerator: resolved.idGenerator ?? resolved.id
            });
            const h3 = h3RuntimeHelpers({id: resolved.idGenerator ?? resolved.id});
            const lifeEventFlow = options.lifeEventFlow
                ?? (resolved.repositories.lifeEvent?.createEvent || resolved.repositories.lifeEvent?.insertEvent
                    ? createLifeEventFlow({repositories: resolved.repositories, clock: resolved.clock, idGenerator: resolved.idGenerator ?? resolved.id})
                    : null);
            const mediaFlow = resolved.mediaFlow
                ?? (typeof resolved.normalizeMediaCapabilityCall === 'function'
                    ? createMediaFlow({
                        repositories: resolved.repositories,
                        clock: resolved.clock,
                        idGenerator: resolved.idGenerator ?? resolved.id,
                        normalizeMediaCapabilityCall: resolved.normalizeMediaCapabilityCall,
                        mediaConceptEnvelopeFor: resolved.mediaConceptEnvelopeFor,
                        providerFor: resolved.providerFor,
                        transaction: resolved.transaction
                    })
                    : null);
            const debugService = options.debugService ?? createDebugService({
                repositories: resolved.repositories,
                settings: options.settings ?? resolved.repositories.settings,
                promptRuns: resolved.repositories.promptRun,
                h3Preflight: options.h3Preflight ?? h3.h3Preflight,
                contextReader: resolved.chatProductionPorts?.contextReader,
                lifeEventFlow,
                mediaJobService: resolved.mediaJobService,
                clock: resolved.clock
            });
            const mediaDebugService = options.mediaDebugService ?? createMediaDebugService({
                repositories: resolved.repositories,
                settings: options.settings ?? resolved.repositories.settings,
                providers: resolved.providers,
                mediaFlow,
                mediaJobService: resolved.mediaJobService,
                contextReader: resolved.chatProductionPorts?.contextReader,
                observability: resolved.mediaObservability,
                clock: resolved.clock,
                enabled: resolved.debugInspectorEnabled === true
            });
            const debugApplication = Object.freeze({
                ...debugService,
                ...mediaDebugService,
                debug: Object.freeze({...(debugService.debug ?? {}), ...(mediaDebugService.debug ?? {})})
            });
            const lifeWorldReady = isRecord(resolved.repositories.blueprint)
                && isRecord(resolved.repositories.schedule)
                && isRecord(resolved.repositories.dailyPlan)
                && isRecord(resolved.repositories.presence);
            const lifeReader = lifeWorldReady
                ? createLifeWorldReader({repositories: resolved.repositories, blueprintReader: resolved.repositories.blueprint, clock: resolved.clock})
                : null;
            const lifeStateService = options.lifeStateService ?? (lifeReader ? createLifeStateService({
                reader: lifeReader,
                resolver: createLifeStateResolver(),
                stateRepository: resolved.repositories.state,
                lifeEventFlow: resolved.lifeEventFlow ?? lifeEventFlow,
                clock: resolved.clock
            }) : null);
            return createCompanionApplication({
                ...resolved,
                services: options.services ?? createBasicCompanionServices({
                    repositories: resolved.repositories,
                    settings: options.settings ?? resolved.repositories.settings,
                    providers: resolved.providers,
                    clock: resolved.clock,
                    idGenerator: resolved.idGenerator ?? resolved.id,
                    debugInspector: resolved.debugInspectorEnabled === true,
                    identitySettingsService: identitySettings,
                    activityService,
                    adapters: options.adapters,
                    personaLifecycle: options.personaLifecycle,
                    personaLifecycleService: options.personaLifecycleService,
                    interviewService: options.interviewService,
                    personaAnalyzer: options.personaAnalyzer ?? resolved.interviewAnalyzer,
                    foundationService: options.foundationService,
                    scheduleService: options.scheduleService,
                    memoryService: options.memoryService ?? resolved.memoryService,
                    debugService: debugApplication,
                    mediaDebugService,
                    mediaService: options.mediaService ?? mediaDebugService,
                    lifeStateService,
                    lifeEventFlow,
                    timelineFlow: resolved.timelineFlow,
                    relationshipFlow: resolved.relationshipFlow
                }),
                mediaFlow,
                lifeEventFlow: resolved.lifeEventFlow ?? lifeEventFlow,
                timelineFlow: resolved.timelineFlow,
                relationshipFlow: resolved.relationshipFlow
            });
        };
    return createRuntime({
        ...options,
        defaultProductionComposition: options.defaultProductionComposition !== false,
        debugInspectorEnabled: options.debugInspectorEnabled
            ?? (options.environment ?? process.env).COMPANION_DEBUG_INSPECTOR === '1',
        applicationFactory,
        missingHandler: options.missingHandler ?? 'error'
    });
}
export default createRuntime;
