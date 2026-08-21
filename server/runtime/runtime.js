import {createHttpApp} from '../http/app.js';
import {registerCompanionRoutes} from '../http/route-registry.js';
import {createCompanionRouteHandlers} from '../application/companion-route-handlers.js';
import {createCompanionApplication} from '../application/companion-application.js';
import {createBasicCompanionServices} from '../application/basic-companion-services.js';
import {createIdentitySettingsService} from '../application/identity-settings-service.js';
import {createActivityService} from '../application/activity-service.js';
import {createProactiveJobService} from '../application/proactive-job-service.js';
import {createChatProductionPorts} from '../application/chat-production-adapter.js';
import {createConversationCommitAdapter} from '../infrastructure/conversation-commit-adapter.js';
import {createActivityRepository} from '../infrastructure/activity-repository.js';
import {createConversationRepository} from '../infrastructure/conversation-repository.js';
import {createGroupRepository} from '../infrastructure/group-repository.js';
import {createJobRepository} from '../infrastructure/job-repository.js';
import {createLifeEventRepository} from '../infrastructure/life-event-repository.js';
import {createMemoryRepository} from '../infrastructure/memory-repository.js';
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
import {h3RuntimeHelpers} from '../infrastructure/h3-preflight.js';
import {createDebugService} from '../application/debug-service.js';
import {createSettingsPolicy} from '../application/settings-policy.js';
import {createLifeWorldReader} from '../application/life-world-reader.js';
import {createLifeStateResolver} from '../domain/life-state-resolver.js';
import {createLifeStateService} from '../application/life-state-service.js';
import {createProviderRegistry} from '../infrastructure/provider-ports.js';
import {createProductionProviderRegistry} from '../infrastructure/production-media-providers.js';
import {createMtplxCompletionPort} from '../infrastructure/llm-provider.js';
import {createMediaJobRepository} from '../infrastructure/media-job-repository.js';
import {createMediaPromptMaster, createSkippedMediaAcceptance} from '../infrastructure/media-prompt-master.js';
import {createProductionProactiveFlows} from '../infrastructure/production-proactive-ports.js';
import {createMediaJobApplication} from '../application/media-job-composition.js';
import createJobDispatcher from './job-dispatcher.js';
import createStartupRuntime from './startup.js';
import createWorkerRuntime from './worker-runtime.js';

const DEFAULT_PORT = 4178;
const DEFAULT_HOST = '0.0.0.0';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

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
    const lifeWorldReader = createLifeWorldReader({
        repositories,
        blueprintReader: repositories.blueprint,
        clock
    });
    const resolveLifeState = createLifeStateResolver();
    const contextReader = {
        read({command = {}, messages = []} = {}) {
            const personaId = command.personaId ?? command?.command?.personaId;
            const persona = repositories.persona?.findActive?.(personaId);
            if (!persona) throw Object.assign(new Error('人格不存在'), {status: 404});
            const at = command.chatAt ?? clock();
            const resolverInput = lifeWorldReader.readResolverInput({personaId, at});
            const resolved = resolveLifeState(resolverInput);
            const state = resolved.situation ? `${resolved.situation}（${resolved.scene || '日常场景'}）` : '当前没有额外的已确认生活事件。';
            return {
                persona: {id: persona.id, name: persona.name, role: persona.role, color: persona.color},
                prompt: `你是 ${persona.name}，角色是 ${persona.role || '陪伴者'}。请基于已确认事实与用户交流，不要编造当前状态。`,
                layers: {
                    lifeState: state,
                    systemCapability: '只输出用户可见的自然回复；能力调用必须通过应用提供的 capability port。'
                },
                settings: readSettings(),
                history: messages
            };
        }
    };
    const llmStreamingPort = createMtplxCompletionPort({provider: mtplx, settings: readSettings});
    const commitBoundary = createConversationCommitAdapter({
        repository: repositories.conversation,
        clock,
        idGenerator,
        transaction: options.transaction
    });
    const userMessageWriter = ({personaId, text: messageText, attachments = {}} = {}) => {
        const conversation = repositories.conversation.getOrCreateConversation({
            personaId,
            id: idGenerator('conversation'),
            createdAt: clock(),
            updatedAt: clock()
        });
        return repositories.conversation.appendMessage({
            id: idGenerator('message'),
            conversationId: conversation.id,
            role: 'user',
            text: String(messageText ?? ''),
            attachmentsJson: JSON.stringify(Array.isArray(attachments) ? attachments : []),
            generationJson: null,
            jobsJson: '[]',
            createdAt: clock(),
            readAt: clock()
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
        commitBoundary,
        sendSse(sink, event) {
            if (typeof sink?.write === 'function') sink.write(`data: ${JSON.stringify(event)}\n\n`);
        },
        end(sink) {
            if (typeof sink?.end === 'function') sink.end();
            else if (sink && typeof sink === 'object') sink.writableEnded = true;
        }
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
        return {kind, count, request: typeof value.request === 'string' ? value.request.trim().slice(0, 500) : '', personaMediaConcept: concept};
    };
    return {
        normalizeMediaCapabilityCall,
        mediaConceptEnvelopeFor(persona, input = {}) {
            return {
                schemaVersion: 1,
                mediaKind: input.kind,
                personaId: persona?.id ?? null,
                personaName: persona?.name ?? '',
                personaRole: persona?.role ?? '',
                request: input.request ?? '',
                currentEvent: null,
                temporaryAppearance: {}
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
            id: options.idGenerator ?? options.id
        })
        : undefined;
    return createProactiveJobService({
        ...nested,
        ...options,
        repositories: nested.repositories ?? repositories,
        flows: nested.flows ?? options.proactiveFlows ?? options.jobFlows ?? options.applicationFlows ?? defaultFlows,
        flowRegistry: nested.flowRegistry ?? options.proactiveFlowRegistry ?? options.flowRegistry,
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
    const settings = createSettingsRepository({database, defaults: () => ({}), clock});
    const personaLifecycle = createPersonaLifecycleRepository({database, clock, id});
    const interview = createInterviewRepository({database, clock, id, personaLifecycle});
    return Object.freeze({
        conversation: createConversationRepository({database}),
        activity: createActivityRepository({database}),
        job,
        jobRepository: job,
        pending,
        pendingEvent: pending,
        lifeEvent: createLifeEventRepository({database, clock, id}),
        persona: createPersonaRepository({database, clock, id}),
        group: createGroupRepository({database, clock, id}),
        memory: createMemoryRepository({database, clock}),
        relationship: createRelationshipRepository({database, clock, id}),
        settings,
        personaLifecycle,
        foundation: createFoundationRepository({database, clock, id}),
        schedule: createScheduleRepository({database, clock, id}),
        interview,
        mediaAsset: createMediaAssetRepository({database}),
        promptRun: createPromptRunRepository({database, clock}),
        state: createStateRepository({database, clock}),
        blueprint: createBlueprintRepository({database}),
        dailyPlan: createDailyPlanRepository({database}),
        presence: createPresenceRepository({database})
    });
}

function descriptorHandler(value) {
    if (typeof value === 'function') return {handler: value, receiver: undefined};
    if (!isRecord(value)) return {handler: null, receiver: undefined};
    const handler = value.handler ?? value.run ?? value.handle;
    const receiver = value.receiver ?? (typeof handler === 'function' ? value : undefined);
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
    if (map instanceof Map) return [...map].map(([type, value]) => ({type, ...descriptorHandler(value)}));
    if (isRecord(map)) return Object.entries(map).map(([type, value]) => ({type, ...descriptorHandler(value)}));
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
        const {handler, receiver} = descriptorHandler(registration);
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
        const registrations = typeof service.registrations === 'function'
            ? service.registrations()
            : Object.entries(service.handlers ?? service.handlerMap ?? {}).map(([type, handler]) => ({type, handler}));
        if (!Array.isArray(registrations)) throw new TypeError('Runtime proactiveJobService registrations must be an array');
        for (const registration of registrations) {
            if (registration?.available === false) continue;
            addJobHandler(handlers, registration?.type, registration?.handler, 'Runtime proactiveJobService');
        }
    }
    collectJobHandlers(handlers, options.jobHandlers ?? options.jobRegistry ?? options.handlers, 'Runtime');
    return handlers.size ? handlers : undefined;
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
    const startup = resolveStartup({...options, environment});
    const repositories = resolveRepositories(options, startup);
    const hasExplicitProviders = options.providerRegistry !== undefined
        || options.providerAdapters !== undefined
        || options.mediaProviderAdapters !== undefined
        || options.providers !== undefined;
    const providers = options.defaultProductionComposition === true && !hasExplicitProviders
        ? defaultProductionProviders(options, environment, repositories)
        : resolveProviders(options, environment, repositories);
    const jobRepository = options.jobRepository ?? repositories.jobRepository ?? repositories.job;
    const mediaComposition = resolveMediaComposition(options, repositories, providers, startup);
    const mediaJobService = mediaComposition.mediaJobService;
    const mediaObservability = mediaComposition.observability;
    const proactiveJobService = resolveProactiveJobService(options, repositories, startup, providers);
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
    const h3Helpers = h3RuntimeHelpers({id: runtimeId(options.idGenerator ?? options.id)});
    const settingsPolicy = options.settingsPolicy ?? createSettingsPolicy({
        providers,
        h3Inspector: h3Helpers.inspectH3Configuration
    });
    const jobDispatcher = resolveJobDispatcher({...options, jobRepository, mediaJobService, proactiveJobService}, startup, repositories);
    const application = options.application ?? options.applicationFactory?.({
        ...options,
        repositories,
        jobRepository,
        providers,
        jobDispatcher,
        mediaJobService,
        mediaObservability,
        proactiveJobService,
        chatProductionPorts,
        settingsPolicy,
        ...mediaCapabilityOptions
    });
    const worker = resolveWorker({...options, jobRepository, jobDispatcher}, startup, repositories);
    const app = resolveApp({...options, application, routeHandlers: options.routeHandlers ?? application?.routeHandlers}, startup, worker);
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
                debugInspector: options.debugInspectorEnabled === true
            });
            const activityService = options.activityService ?? createActivityService({
                repositories: resolved.repositories,
                clock: resolved.clock,
                idGenerator: resolved.idGenerator ?? resolved.id
            });
            const h3 = h3RuntimeHelpers({id: resolved.idGenerator ?? resolved.id});
            const debugService = options.debugService ?? createDebugService({
                repositories: resolved.repositories,
                promptRuns: resolved.repositories.promptRun,
                h3Preflight: options.h3Preflight ?? h3.h3Preflight,
                contextReader: resolved.chatProductionPorts?.contextReader,
                mediaJobService: resolved.mediaJobService,
                clock: resolved.clock
            });
            const lifeReader = createLifeWorldReader({repositories: resolved.repositories, blueprintReader: resolved.repositories.blueprint, clock: resolved.clock});
            const lifeStateService = options.lifeStateService ?? createLifeStateService({
                reader: lifeReader,
                resolver: createLifeStateResolver(),
                stateRepository: resolved.repositories.state,
                clock: resolved.clock
            });
            return createCompanionApplication({
                ...resolved,
                services: options.services ?? createBasicCompanionServices({
                    repositories: resolved.repositories,
                    settings: options.settings ?? resolved.repositories.settings,
                    providers: resolved.providers,
                    clock: resolved.clock,
                    debugInspector: options.debugInspectorEnabled === true,
                    identitySettingsService: identitySettings,
                    activityService,
                    adapters: options.adapters,
                    personaLifecycle: options.personaLifecycle,
                    personaLifecycleService: options.personaLifecycleService,
                    interviewService: options.interviewService,
                    foundationService: options.foundationService,
                    scheduleService: options.scheduleService,
                    memoryService: options.memoryService,
                    debugService,
                    mediaService: options.mediaService,
                    lifeStateService
                })
            });
        };
    return createRuntime({
        ...options,
        defaultProductionComposition: options.defaultProductionComposition !== false,
        debugInspectorEnabled: options.debugInspectorEnabled
            ?? options.environment?.COMPANION_DEBUG_INSPECTOR === '1'
            ?? process.env.COMPANION_DEBUG_INSPECTOR === '1',
        applicationFactory,
        missingHandler: options.missingHandler ?? 'error'
    });
}
export default createRuntime;
