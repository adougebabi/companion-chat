import {emptyStepResult, normalizeStepResult} from '../contracts/index.js';

const MAX_CONTEXT_ID_LENGTH = 240;
const MAX_ERROR_LENGTH = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function boundedText(value, field, maxLength = MAX_CONTEXT_ID_LENGTH) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!text) throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function boundedErrorMessage(error) {
    const message = error instanceof Error ? error.message : String(error);
    const compact = message.replace(/\s+/g, ' ').trim() || 'Unknown flow execution failure';
    if (compact.length <= MAX_ERROR_LENGTH) return compact;
    return `${compact.slice(0, MAX_ERROR_LENGTH - 3)}...`;
}

function mergeStepResults(target, next) {
    return {
        facts: target.facts.concat(next.facts),
        projections: target.projections.concat(next.projections),
        effects: target.effects.concat(next.effects),
        presentation: target.presentation.concat(next.presentation)
    };
}

function normalizeContext(value) {
    if (value === undefined || value === null) return {};
    if (!isRecord(value)) throw new TypeError('Flow context must be an object');

    const context = {...value};
    for (const field of ['requestId', 'correlationId', 'causationId', 'subjectId', 'personaId']) {
        if (context[field] !== undefined && context[field] !== null) {
            context[field] = boundedText(context[field], `Flow context ${field}`);
        }
    }
    return context;
}

/**
 * A bounded error for failures at the flow or commit boundary. The original
 * error is intentionally not copied onto the public object so callers cannot
 * accidentally expose provider or persistence details in a transport error.
 */
export class FlowExecutionError extends Error {
    constructor({flowId, stepId = null, phase, error}) {
        const location = stepId ? ` step ${stepId}` : '';
        const message = `${phase} failed for flow ${flowId}${location}: ${boundedErrorMessage(error)}`;
        super(boundedErrorMessage(message));
        this.name = 'FlowExecutionError';
        this.code = phase === 'commit' ? 'FLOW_COMMIT_FAILED' : 'FLOW_EXECUTION_FAILED';
        this.flowId = flowId;
        this.stepId = stepId;
        this.phase = phase;
    }
}

function assertRegistry(registry) {
    if (!isRecord(registry) || typeof registry.get !== 'function') {
        throw new TypeError('Flow executor requires a flow registry');
    }
    return registry;
}

function resolveInvocation(flowIdOrOptions, contextArg, commandArg, commitArg, configuredCommit) {
    if (isRecord(flowIdOrOptions)) {
        const options = flowIdOrOptions;
        return {
            flowId: options.flowId,
            context: options.context,
            command: options.command,
            commit: options.commitBoundary === undefined ? options.commit ?? configuredCommit : options.commitBoundary
        };
    }
    return {
        flowId: flowIdOrOptions,
        context: contextArg,
        command: commandArg,
        commit: commitArg === undefined ? configuredCommit : commitArg
    };
}

/**
 * Create the application flow runner for an existing registry. The runner is
 * pure with respect to infrastructure: it only invokes registered steps and
 * hands one normalized aggregate to the optional commit boundary.
 */
export function createFlowExecutor({registry, commit, commitBoundary} = {}) {
    const flowRegistry = assertRegistry(registry);
    if (commit !== undefined && commitBoundary !== undefined && commit !== commitBoundary) {
        throw new TypeError('Flow executor received conflicting commit callbacks');
    }
    const configuredCommit = commitBoundary ?? commit ?? null;
    if (configuredCommit !== null && typeof configuredCommit !== 'function') {
        throw new TypeError('Flow executor commit boundary must be a function');
    }

    async function run(flowIdOrOptions, contextArg = {}, commandArg = {}, commitArg) {
        const invocation = resolveInvocation(flowIdOrOptions, contextArg, commandArg, commitArg, configuredCommit);
        const flowId = boundedText(invocation.flowId, 'Flow id');
        const flow = flowRegistry.get(flowId);
        if (!flow) {
            throw new FlowExecutionError({
                flowId,
                phase: 'lookup',
                error: new Error('Unknown flow')
            });
        }

        const context = normalizeContext(invocation.context);
        const command = invocation.command === undefined || invocation.command === null ? {} : invocation.command;
        if (!isRecord(command)) throw new TypeError('Flow command must be an object');
        if (invocation.commit !== null && invocation.commit !== undefined && typeof invocation.commit !== 'function') {
            throw new TypeError('Flow commit boundary must be a function');
        }

        let result = emptyStepResult();
        for (const step of flow.steps) {
            try {
                const next = normalizeStepResult(await step.run(context, command, result));
                result = mergeStepResults(result, next);
            } catch (error) {
                throw new FlowExecutionError({flowId, stepId: step.id, phase: 'step', error});
            }
        }

        const normalized = normalizeStepResult(result);
        if (invocation.commit) {
            try {
                await invocation.commit(normalized);
            } catch (error) {
                throw new FlowExecutionError({flowId, phase: 'commit', error});
            }
        }
        return normalized;
    }

    return Object.freeze({run, execute: run, runFlow: run});
}

