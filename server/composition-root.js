import {
    BACKEND_CONTRACT_BASELINE,
    assertCapabilityDispatcherPort
} from './contracts/index.js';
import {createChatTurnFlow} from './application/chat-turn-flow.js';
import {createFlowExecutor} from './application/flow-executor.js';
import {createFlowRegistry} from './application/flow-registry.js';
import {createChatTurnSseAdapter} from './http/chat-turn-sse-adapter.js';
import {createActivityRepository} from './infrastructure/activity-repository.js';
import {createConversationRepository} from './infrastructure/conversation-repository.js';
import {createJobRepository} from './infrastructure/job-repository.js';
import {createPendingEventRepository} from './infrastructure/pending-event-repository.js';

const REPOSITORY_NAMES = Object.freeze({
    conversation: ['conversationRepository', 'conversation'],
    activity: ['activityRepository', 'activity'],
    job: ['jobRepository', 'job'],
    pending: ['pendingEventRepository', 'pendingRepository', 'pendingEvent', 'pending']
});

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function resolveAliases({options, nested, names, field}) {
    const values = [];
    for (const source of [options, nested]) {
        if (!isRecord(source)) continue;
        for (const name of names) {
            if (Object.hasOwn(source, name) && source[name] !== undefined) values.push(source[name]);
        }
    }
    const first = values[0];
    if (values.some(value => value !== first)) {
        throw new TypeError(`Composition root received conflicting ${field} dependencies`);
    }
    return first;
}

function assertOpenDatabase(database) {
    if (!isRecord(database) || typeof database.prepare !== 'function' || typeof database.transaction !== 'function') {
        throw new TypeError('Composition root requires an already-open database');
    }
    return database;
}

function assertClock(clock) {
    if (typeof clock === 'function') return clock;
    if (isRecord(clock) && typeof clock.now === 'function') return clock;
    throw new TypeError('Composition root clock must be a function or provide now()');
}

function assertIdGenerator(idGenerator) {
    if (typeof idGenerator === 'function') return idGenerator;
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') return idGenerator;
    throw new TypeError('Composition root id helper must be a function or provide next()');
}

function assertContextReader(contextReader) {
    if (typeof contextReader === 'function') return contextReader;
    if (isRecord(contextReader) && (typeof contextReader.read === 'function' || typeof contextReader.readContext === 'function')) {
        return contextReader;
    }
    throw new TypeError('Composition root contextReader must provide read() or readContext()');
}

function assertLlmStreamPort(llmStreamingPort) {
    if (typeof llmStreamingPort === 'function') return llmStreamingPort;
    if (isRecord(llmStreamingPort) && ['stream', 'streamCompletion', 'streamChat'].some(method => typeof llmStreamingPort[method] === 'function')) {
        return llmStreamingPort;
    }
    throw new TypeError('Composition root llmStreamingPort must provide stream()');
}

function assertRepository(repository, name) {
    if (!isRecord(repository)) throw new TypeError(`Composition root requires ${name}`);
    return repository;
}

function resolveCommitAdapter(value) {
    if (typeof value === 'function') return value;
    if (!isRecord(value)) throw new TypeError('Composition root requires a SQLite commit adapter');
    if (typeof value.commit === 'function') return value.commit.bind(value);
    if (typeof value.commitStepResult === 'function') return value.commitStepResult.bind(value);
    throw new TypeError('Composition root SQLite commit adapter must provide commit()');
}

function isSseAdapter(value) {
    return typeof value === 'function'
        || (isRecord(value) && ['handleChatTurn', 'handle', 'run', 'stream'].some(method => typeof value[method] === 'function'));
}

function resolveSseAdapter({options, chatTurnFlow}) {
    const nested = options.sse ?? options.transport;
    const configured = resolveAliases({
        options,
        nested,
        names: ['chatSseAdapter', 'sseAdapter'],
        field: 'SSE adapter'
    });
    const factory = options.sseAdapterFactory ?? options.createSseAdapter;
    if (factory !== undefined && typeof factory !== 'function') {
        throw new TypeError('Composition root sseAdapterFactory must be a function');
    }
    if (configured !== undefined) {
        if (!isSseAdapter(configured)) throw new TypeError('Composition root SSE adapter must provide handle()');
        return configured;
    }
    if (factory) {
        const adapter = factory({chatTurnFlow});
        if (!isSseAdapter(adapter)) throw new TypeError('Composition root SSE adapter factory returned an invalid adapter');
        return adapter;
    }

    const sendSse = options.sendSse ?? nested?.sendSse;
    const end = options.end ?? nested?.end;
    if (sendSse === undefined && end === undefined) {
        throw new TypeError('Composition root requires an SSE adapter');
    }
    if (typeof sendSse !== 'function' || typeof end !== 'function') {
        throw new TypeError('Composition root SSE adapter requires sendSse() and end()');
    }
    const adapter = createChatTurnSseAdapter({
        chatTurnFlow,
        sendSse,
        end,
        errorMapper: options.errorMapper ?? nested?.errorMapper,
        isRequestAborted: options.isRequestAborted ?? nested?.isRequestAborted,
        requestAborted: options.requestAborted ?? nested?.requestAborted
    });
    return adapter;
}

function repositoryFromOptions({options, repositories, name, create, requireConfigured}) {
    const configured = resolveAliases({
        options,
        nested: repositories,
        names: REPOSITORY_NAMES[name],
        field: `${name} repository`
    });
    if (configured === undefined && requireConfigured) {
        const label = name === 'pending' ? 'pendingEventRepository' : `${name}Repository`;
        throw new TypeError(`Composition root requires ${label}`);
    }
    return configured ?? create();
}

/**
 * Assemble the modular backend from already-created runtime resources.
 *
 * This module is deliberately not the package entrypoint. It only validates
 * and wires ports/adapters; opening SQLite, migrations, providers, HTTP, and
 * workers remain the caller's responsibility.
 */
export function createCompositionRoot(options = {}) {
    if (!isRecord(options)) throw new TypeError('Composition root options must be an object');
    if (Object.hasOwn(options, 'dispatcher') || Object.hasOwn(options, 'capabilityDispatchers')) {
        throw new TypeError('Composition root accepts exactly one capabilityDispatcher port');
    }

    const database = assertOpenDatabase(options.database ?? options.db);
    const clock = assertClock(options.clock);
    const idGenerator = assertIdGenerator(options.idGenerator ?? options.id);
    const capabilityDispatcher = assertCapabilityDispatcherPort(options.capabilityDispatcher);
    const contextReader = assertContextReader(options.contextReader);
    const llmStreamingPort = assertLlmStreamPort(options.llmStreamingPort ?? options.llmStreamPort ?? options.llm);

    const repositoriesInput = options.repositories;
    if (repositoriesInput !== undefined && !isRecord(repositoriesInput)) {
        throw new TypeError('Composition root repositories must be an object');
    }
    const hasConfiguredRepositories = repositoriesInput !== undefined
        || Object.values(REPOSITORY_NAMES).flat().some(name => Object.hasOwn(options, name));
    const conversationRepository = repositoryFromOptions({
        options,
        repositories: repositoriesInput,
        name: 'conversation',
        create: () => createConversationRepository({database}),
        requireConfigured: hasConfiguredRepositories
    });
    const activityRepository = repositoryFromOptions({
        options,
        repositories: repositoriesInput,
        name: 'activity',
        create: () => createActivityRepository({database}),
        requireConfigured: hasConfiguredRepositories
    });
    const jobRepository = repositoryFromOptions({
        options,
        repositories: repositoriesInput,
        name: 'job',
        create: () => createJobRepository({database, clock, id: idGenerator}),
        requireConfigured: hasConfiguredRepositories
    });
    assertRepository(conversationRepository, 'conversationRepository');
    assertRepository(activityRepository, 'activityRepository');
    assertRepository(jobRepository, 'jobRepository');

    const pendingEventRepository = repositoryFromOptions({
        options,
        repositories: repositoriesInput,
        name: 'pending',
        create: () => {
            if (typeof jobRepository.enqueue !== 'function') {
                throw new TypeError('Composition root jobRepository must provide enqueue() for pending events');
            }
            return createPendingEventRepository({
                database,
                enqueueJob: jobRepository.enqueue.bind(jobRepository)
            });
        },
        requireConfigured: hasConfiguredRepositories
    });
    assertRepository(pendingEventRepository, 'pendingEventRepository');

    const configuredCommit = resolveAliases({
        options,
        nested: null,
        names: ['sqliteCommitAdapter', 'commitAdapter'],
        field: 'SQLite commit adapter'
    });
    const configuredBoundary = options.commitBoundary;
    if (configuredCommit === undefined && configuredBoundary === undefined) {
        throw new TypeError('Composition root requires a SQLite commit adapter');
    }
    if (configuredCommit !== undefined && configuredBoundary !== undefined && configuredCommit !== configuredBoundary) {
        throw new TypeError('Composition root received conflicting commit boundaries');
    }
    const commitAdapter = configuredCommit === undefined ? configuredBoundary : configuredCommit;
    const commitBoundary = resolveCommitAdapter(commitAdapter);

    const flowRegistry = options.flowRegistry ?? options.flows ?? createFlowRegistry();
    if (!isRecord(flowRegistry) || typeof flowRegistry.register !== 'function' || typeof flowRegistry.get !== 'function') {
        throw new TypeError('Composition root flowRegistry must provide register() and get()');
    }
    const flowExecutor = options.flowExecutor ?? options.executor ?? createFlowExecutor({
        registry: flowRegistry,
        commitBoundary
    });
    if (!isRecord(flowExecutor) || typeof flowExecutor.run !== 'function') {
        throw new TypeError('Composition root flowExecutor must provide run()');
    }

    const chatTurnFlow = createChatTurnFlow({
        registry: flowRegistry,
        executor: flowExecutor,
        contextReader,
        llmStreamingPort,
        capabilityDispatcher,
        conversationRepository,
        presentationMapper: options.presentationMapper
    });
    const chatSseAdapter = resolveSseAdapter({options, chatTurnFlow});

    const repositories = Object.freeze({
        conversation: conversationRepository,
        activity: activityRepository,
        job: jobRepository,
        pending: pendingEventRepository,
        pendingEvent: pendingEventRepository,
        conversationRepository,
        activityRepository,
        jobRepository,
        pendingEventRepository
    });
    return Object.freeze({
        contractVersion: BACKEND_CONTRACT_BASELINE.version,
        contracts: BACKEND_CONTRACT_BASELINE,
        database,
        clock,
        idGenerator,
        capabilityDispatcher,
        repositories,
        conversationRepository,
        activityRepository,
        jobRepository,
        pendingEventRepository,
        sqliteCommitAdapter: configuredCommit ?? commitAdapter,
        commitAdapter: configuredCommit ?? commitAdapter,
        commitBoundary,
        flowRegistry,
        flows: flowRegistry,
        flowExecutor,
        executor: flowExecutor,
        chatTurnFlow,
        chatSseAdapter,
        sseAdapter: chatSseAdapter
    });
}

export const createBackendComposition = createCompositionRoot;
export const createBackendCompositionRoot = createCompositionRoot;
