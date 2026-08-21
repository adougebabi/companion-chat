import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

import {BACKEND_CONTRACT_BASELINE, assertCapabilityDispatcherPort, validatePorts} from './contracts/index.js';
import {createCapabilityHandoffStep, createFlowRegistry} from './application/flow-registry.js';
import {createFlowExecutor} from './application/flow-executor.js';
import {createRuntime, createCompanionRuntime} from './runtime/runtime.js';
export {createCompositionRoot as createProductionCompositionRoot} from './composition-root.js';
export {createCompanionRouteHandlers} from './application/companion-route-handlers.js';
export {createPendingEventFlow} from './application/pending-event-flow.js';
export {createSceneEventFlow} from './application/scene-event-flow.js';
export {createMediaFlow} from './application/media-flow.js';
export {createCompanionApplication} from './application/companion-application.js';
export {createBasicCompanionServices} from './application/basic-companion-services.js';
export {createIdentitySettingsService, publicSettings, redactSettings} from './application/identity-settings-service.js';
export {createMediaObservability} from './application/media-observability.js';
export {createMediaJobService} from './application/media-job-service.js';
export {createCompanionChatService} from './application/chat-service.js';
export {createLifeWorldReader} from './application/life-world-reader.js';
export {createConversationService} from './application/conversation-service.js';

export {createRuntime, createCompanionRuntime};

/** Start a fully assembled modular runtime supplied by the caller. */
export async function startModularRuntime(options = {}) {
    const {startOptions, ...runtimeOptions} = options;
    const runtime = createRuntime(runtimeOptions);
    await runtime.start(startOptions);
    return runtime;
}

/** Start the complete companion application assembled by the modular root. */
export async function startCompanionRuntime(options = {}) {
    const {startOptions, ...runtimeOptions} = options;
    const runtime = createCompanionRuntime(runtimeOptions);
    await runtime.start(startOptions);
    return runtime;
}

function cliRuntimeOptions({environment, runtimeOptions = {}} = {}) {
    if (!environment || typeof environment !== 'object' || Array.isArray(environment)) {
        throw new TypeError('CLI environment must be an object');
    }
    if (!runtimeOptions || typeof runtimeOptions !== 'object' || Array.isArray(runtimeOptions)) {
        throw new TypeError('CLI runtimeOptions must be an object');
    }
    return {
        ...runtimeOptions,
        environment: runtimeOptions.environment ?? environment,
        port: runtimeOptions.port ?? environment.PORT,
        host: runtimeOptions.host ?? environment.HOST,
        dataDir: runtimeOptions.dataDir ?? environment.DATA_DIR,
        databasePath: runtimeOptions.databasePath ?? environment.DATABASE_PATH
    };
}

function reportShutdownError(logger, error) {
    if (typeof logger?.error === 'function') logger.error('Companion runtime shutdown failed', error);
}

/**
 * Install signal handlers for a runtime and return one idempotent cleanup
 * function. The signal source is injectable so CLI lifecycle tests do not
 * need to install handlers on the test runner's process.
 */
export function installShutdownHandlers(runtime, {signalSource = process, logger = console} = {}) {
    if (!runtime || typeof runtime.stop !== 'function') {
        throw new TypeError('CLI runtime must provide stop()');
    }
    if (!signalSource || typeof signalSource.on !== 'function') {
        throw new TypeError('CLI signalSource must provide on()');
    }

    let cleanupPromise = null;
    const remove = typeof signalSource.removeListener === 'function'
        ? (signal, listener) => signalSource.removeListener(signal, listener)
        : typeof signalSource.off === 'function'
            ? (signal, listener) => signalSource.off(signal, listener)
            : () => {};
    const onSignal = () => {
        void cleanup().catch(error => reportShutdownError(logger, error));
    };
    const cleanup = () => {
        if (cleanupPromise) return cleanupPromise;
        cleanupPromise = Promise.resolve()
            .then(() => runtime.stop())
            .finally(() => {
                remove('SIGINT', onSignal);
                remove('SIGTERM', onSignal);
            });
        return cleanupPromise;
    };

    signalSource.on('SIGINT', onSignal);
    signalSource.on('SIGTERM', onSignal);
    return cleanup;
}

/**
 * Create and start the companion runtime for direct CLI use.
 *
 * `runtimeFactory` is intentionally injectable: tests can verify option
 * forwarding and signal cleanup without opening SQLite or binding a port.
 */
export async function runModularCli({
    environment = process.env,
    runtimeFactory = createCompanionRuntime,
    runtimeOptions = {},
    startOptions,
    signalSource = process,
    logger = console
} = {}) {
    if (typeof runtimeFactory !== 'function') throw new TypeError('CLI runtimeFactory must be a function');
    const options = cliRuntimeOptions({environment, runtimeOptions});
    const runtime = await runtimeFactory(options);
    if (!runtime || typeof runtime.start !== 'function' || typeof runtime.stop !== 'function') {
        throw new TypeError('CLI runtimeFactory must return a runtime with start() and stop()');
    }
    await runtime.start(startOptions);
    const cleanup = installShutdownHandlers(runtime, {signalSource, logger});
    return Object.freeze({runtime, cleanup});
}

export const runCli = runModularCli;

function isDirectExecution() {
    return typeof process?.argv?.[1] === 'string'
        && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
}

if (isDirectExecution()) {
    runModularCli().catch(error => {
        console.error('Unable to start companion runtime', error);
        process.exitCode = 1;
    });
}

/**
 * Modular composition exports.
 *
 * `createRuntime()` is the executable lifecycle factory. The contract
 * composition below remains side-effect free for flow tests, while direct
 * execution above owns only process startup and shutdown wiring.
 */
function resolveCommitAdapter(value) {
    if (value === undefined || value === null) return null;
    if (typeof value === 'function') return value;
    const commit = typeof value.commit === 'function'
        ? value.commit.bind(value)
        : typeof value.commitStepResult === 'function'
            ? value.commitStepResult.bind(value)
            : null;
    if (typeof commit !== 'function') throw new TypeError('Composition root commitAdapter must provide commit()');
    return commit;
}

export function createCompositionRoot({ports, capabilityDispatcher, flowExecutor, commit, commitBoundary, commitAdapter} = {}) {
    const validatedPorts = validatePorts(ports);
    const dispatcher = assertCapabilityDispatcherPort(capabilityDispatcher);
    const adapterCommit = resolveCommitAdapter(commitAdapter);
    const flows = createFlowRegistry();
    flows.register({
        id: 'chat-turn',
        version: 1,
        layer: 'application',
        dependencies: [{id: 'backend-contracts', layer: 'contracts'}],
        steps: [createCapabilityHandoffStep({dispatcher})]
    });
    const executor = flowExecutor ?? createFlowExecutor({registry: flows, commit, commitBoundary: commitBoundary ?? adapterCommit});
    if (!executor || typeof executor.run !== 'function') throw new TypeError('Composition root flowExecutor must provide run()');
    return Object.freeze({
        contractVersion: BACKEND_CONTRACT_BASELINE.version,
        contracts: BACKEND_CONTRACT_BASELINE,
        ports: validatedPorts,
        flows,
        capabilityDispatcher: dispatcher,
        flowExecutor: executor,
        commitAdapter: adapterCommit
    });
}
