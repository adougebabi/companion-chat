import {BACKEND_CONTRACT_BASELINE, assertCapabilityDispatcherPort, validatePorts} from './contracts/index.js';
import {createCapabilityHandoffStep, createFlowRegistry} from './application/flow-registry.js';
import {createFlowExecutor} from './application/flow-executor.js';

/**
 * Future composition root for the modular backend.
 *
 * It is intentionally inert: the current package still starts server.js.
 * Keeping construction separate lets contract and flow tests run without
 * opening SQLite, binding a port, or contacting a provider.
 */
export function createCompositionRoot({ports, capabilityDispatcher, flowExecutor, commit, commitBoundary} = {}) {
    const validatedPorts = validatePorts(ports);
    const dispatcher = assertCapabilityDispatcherPort(capabilityDispatcher);
    const flows = createFlowRegistry();
    flows.register({
        id: 'chat-turn',
        version: 1,
        layer: 'application',
        dependencies: [{id: 'backend-contracts', layer: 'contracts'}],
        steps: [createCapabilityHandoffStep({dispatcher})]
    });
    const executor = flowExecutor ?? createFlowExecutor({registry: flows, commit, commitBoundary});
    if (!executor || typeof executor.run !== 'function') throw new TypeError('Composition root flowExecutor must provide run()');
    return Object.freeze({
        contractVersion: BACKEND_CONTRACT_BASELINE.version,
        contracts: BACKEND_CONTRACT_BASELINE,
        ports: validatedPorts,
        flows,
        capabilityDispatcher: dispatcher,
        flowExecutor: executor
    });
}
