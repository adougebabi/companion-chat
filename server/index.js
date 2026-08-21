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
export {createMediaObservability} from './application/media-observability.js';
export {createMediaJobService} from './application/media-job-service.js';
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

/**
 * Modular composition exports.
 *
 * `createRuntime()` is the executable lifecycle factory. The small contract
 * composition below remains side-effect free for flow tests while the package
 * entrypoint is migrated in a later cutover once all business adapters are
 * supplied by the new root.
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
