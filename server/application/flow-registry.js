import {
    assertCapabilityDispatcherPort,
    emptyStepResult,
    normalizeCapabilityCall,
    normalizeCapabilityDispatch,
    normalizeStepResult,
    validateLayerDependencies
} from '../contracts/index.js';

function normalizeDependency(value) {
    if (typeof value === 'string') return {id: value, layer: 'contracts'};
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new TypeError('Flow dependency must be an object');
    if (typeof value.id !== 'string' || !value.id.trim()) throw new TypeError('Flow dependency id must be a non-empty string');
    if (typeof value.layer !== 'string') throw new TypeError('Flow dependency layer must be a string');
    return {id: value.id, layer: value.layer};
}

function normalizeStep(step) {
    if (!step || typeof step !== 'object' || Array.isArray(step)) throw new TypeError('Flow step must be an object');
    if (typeof step.id !== 'string' || !step.id.trim()) throw new TypeError('Flow step id must be a non-empty string');
    if (typeof step.run !== 'function') throw new TypeError(`Flow step ${step.id} must provide run()`);
    const dependencies = Array.isArray(step.dependencies) ? step.dependencies.map(normalizeDependency) : [];
    validateLayerDependencies(step.layer || 'application', dependencies);
    return Object.freeze({
        id: step.id,
        layer: step.layer || 'application',
        dependencies,
        run: step.run
    });
}

function normalizeFlow(flow) {
    if (!flow || typeof flow !== 'object' || Array.isArray(flow)) throw new TypeError('Flow definition must be an object');
    if (typeof flow.id !== 'string' || !flow.id.trim()) throw new TypeError('Flow id must be a non-empty string');
    if (!Number.isInteger(flow.version) || flow.version < 1) throw new TypeError(`Flow ${flow.id} version must be a positive integer`);
    const layer = flow.layer || 'application';
    const dependencies = Array.isArray(flow.dependencies) ? flow.dependencies.map(normalizeDependency) : [];
    validateLayerDependencies(layer, dependencies);
    const steps = Array.isArray(flow.steps) ? flow.steps.map(normalizeStep) : [];
    const stepIds = new Set();
    for (const step of steps) {
        if (stepIds.has(step.id)) throw new Error(`Flow ${flow.id} contains duplicate step ${step.id}`);
        stepIds.add(step.id);
    }
    return Object.freeze({id: flow.id, version: flow.version, layer, dependencies, steps});
}

function mergeStepResults(target, next) {
    return {
        facts: target.facts.concat(next.facts),
        projections: target.projections.concat(next.projections),
        effects: target.effects.concat(next.effects),
        presentation: target.presentation.concat(next.presentation)
    };
}

export function createFlowRegistry() {
    const flows = new Map();
    const registry = {
        register(definition) {
            const flow = normalizeFlow(definition);
            if (flows.has(flow.id)) throw new Error(`Flow already registered: ${flow.id}`);
            flows.set(flow.id, flow);
            return registry;
        },
        has(id) {
            return flows.has(id);
        },
        get(id) {
            return flows.get(id) || null;
        },
        list() {
            return [...flows.values()].map(flow => ({id: flow.id, version: flow.version, layer: flow.layer, stepIds: flow.steps.map(step => step.id)}));
        },
        async run(id, context = {}, command = {}) {
            const flow = flows.get(id);
            if (!flow) throw new Error(`Unknown flow: ${id}`);
            let result = emptyStepResult();
            for (const step of flow.steps) {
                const next = normalizeStepResult(await step.run(context, command, result));
                result = mergeStepResults(result, next);
            }
            return result;
        }
    };
    return Object.freeze(registry);
}

/**
 * The backend flow consumes the native task's already-normalized calls. It
 * does not parse provider chunks or implement marker fallback a second time.
 */
export function createCapabilityHandoffStep({dispatcher}) {
    assertCapabilityDispatcherPort(dispatcher);
    return {
        id: 'capability-handoff',
        layer: 'application',
        dependencies: [{id: 'capability-contract', layer: 'contracts'}],
        async run(context = {}, command = {}) {
            const calls = Array.isArray(command.capabilityCalls) ? command.capabilityCalls.map(normalizeCapabilityCall) : [];
            const outcome = normalizeCapabilityDispatch(await dispatcher.dispatch({
                calls,
                context: {
                    personaId: context.personaId || command.personaId || null,
                    causationId: context.causationId || command.causationId || null,
                    correlationId: context.correlationId || command.correlationId || null
                }
            }));
            return {
                facts: [],
                projections: [],
                effects: outcome.effects,
                presentation: outcome.results.map(result => ({type: 'capability-result', result}))
            };
        }
    };
}

export {normalizeFlow, normalizeStep};
