/**
 * Registration seam for the proactive and deferred worker jobs.
 *
 * This module deliberately does not know SQLite, a provider, or the durable
 * job repository. The generic job dispatcher owns lease validation, retry,
 * and settlement. A registered flow owns the feature-specific work and gets
 * only the already-claimed job plus application ports.
 */

export const PROACTIVE_JOB_SERVICE_VERSION = 1;

export const PROACTIVE_JOB_TYPES = Object.freeze([
    'proactive_message',
    'pending_event',
    'activity_decision',
    'deferred_chat_reply'
]);

// Timeline effects written before the canonical activity job name was wired
// into the runtime may still be queued. Keep that persisted spelling as an
// alias while exposing one application flow and one settlement path.
export const PROACTIVE_JOB_ALIASES = Object.freeze({
    'timeline.activity_decision': 'activity_decision'
});

const FLOW_ALIASES = Object.freeze({
    proactive_message: Object.freeze(['proactive_message', 'proactiveMessage', 'proactive', 'proactiveFlow', 'proactiveMessageFlow']),
    pending_event: Object.freeze(['pending_event', 'pendingEvent', 'pending', 'pendingEventFlow']),
    activity_decision: Object.freeze(['activity_decision', 'activityDecision', 'activity', 'activityDecisionFlow']),
    deferred_chat_reply: Object.freeze(['deferred_chat_reply', 'deferredChatReply', 'deferred', 'deferredChatReplyFlow'])
});

const FLOW_IDS = Object.freeze({
    proactive_message: Object.freeze(['proactive-message', 'proactive_message']),
    pending_event: Object.freeze(['pending-event', 'pending_event']),
    activity_decision: Object.freeze(['activity-decision', 'activity_decision']),
    deferred_chat_reply: Object.freeze(['deferred-chat-reply', 'deferred_chat_reply'])
});

const BLOCKER_DETAILS = Object.freeze({
    proactive_message: Object.freeze({
        code: 'missing_proactive_flow',
        owner: 'activity/proactive application flow',
        missing: Object.freeze(['eligibility policy', 'structured decision call', 'frozen decision persistence', 'user-visible assistant reply projection']),
        reason: 'The former handler combined life-event lookup, eligibility, model evaluation, decision freezing, and assistant-message creation in one transport-bound branch.'
    }),
    pending_event: Object.freeze({
        code: 'missing_pending_event_worker_flow',
        owner: 'pending-event application flow',
        missing: Object.freeze(['due/expiry transition', 'triggered-state idempotency', 'frozen decision reuse', 'pending-event message projection']),
        reason: 'pending-event-flow currently registers a pending fact and its job; it does not execute a due worker job.'
    }),
    activity_decision: Object.freeze({
        code: 'missing_activity_decision_flow',
        owner: 'activity application flow',
        missing: Object.freeze(['life-event read', 'structured publish decision', 'activity projection', 'media effect intent']),
        reason: 'The former handler performed model parsing, activity persistence, supporting comments, and media enqueue in one branch.'
    }),
    deferred_chat_reply: Object.freeze({
        code: 'missing_deferred_chat_reply_flow',
        owner: 'conversation/deferred-reply application flow',
        missing: Object.freeze(['deferred-batch read', 'life-world-aware reply context', 'single reply projection', 'batch completion projection']),
        reason: 'The former handler read and updated deferred batches, called the model, and persisted the reply in one transaction.'
    })
});

const JOB_REPOSITORY_KEYS = new Set([
    'job', 'jobRepository', 'effect', 'effectRepository', 'queue', 'jobQueue', 'jobs'
]);

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${field} must be a non-empty string`);
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function requiredRecord(value, field) {
    if (!isRecord(value)) throw new TypeError(`${field} must be an object`);
    return value;
}

function valueFor(value, camel, snake) {
    return value?.[camel] === undefined ? value?.[snake] : value[camel];
}

function parsePayload(job) {
    const raw = valueFor(job, 'payload', 'payload_json');
    if (raw === undefined || raw === null || raw === '') return {};
    if (isRecord(raw)) return raw;
    if (typeof raw !== 'string') throw terminalError('Job payload must be a JSON object');
    let parsed;
    try {
        parsed = JSON.parse(raw);
    } catch {
        throw terminalError('Job payload JSON is invalid');
    }
    if (!isRecord(parsed)) throw terminalError('Job payload JSON must be an object');
    return parsed;
}

function terminalError(message) {
    return Object.assign(new Error(message), {retryable: false, terminal: true, code: 'PROACTIVE_JOB_INPUT_INVALID'});
}

function jobTypeFor(job) {
    return requiredText(valueFor(job, 'jobType', 'job_type'), 'Proactive job type', 80);
}

function personaIdFor(job) {
    const value = valueFor(job, 'personaId', 'persona_id');
    return value === null || value === undefined ? null : requiredText(value, 'Proactive job personaId', 160);
}

function jobIdFor(job) {
    return requiredText(job?.id, 'Proactive job id', 160);
}

function canonicalJobType(type) {
    return PROACTIVE_JOB_ALIASES[type] ?? type;
}

function repositoryPorts(repositories) {
    if (repositories === undefined || repositories === null) return Object.freeze({});
    if (!isRecord(repositories)) throw new TypeError('Proactive job service repositories must be an object');
    const safe = Object.fromEntries(Object.entries(repositories).filter(([key]) => !JOB_REPOSITORY_KEYS.has(key)));
    return Object.freeze(safe);
}

function applicationPorts(options, repositories) {
    const configured = options.ports ?? options.applicationPorts ?? {};
    if (!isRecord(configured)) throw new TypeError('Proactive job service ports must be an object');
    return Object.freeze({
        ...configured,
        repositories: repositoryPorts(configured.repositories ?? repositories),
        lifeWorld: configured.lifeWorld ?? options.lifeWorld ?? options.lifeWorldReader ?? null,
        contextReader: configured.contextReader ?? options.contextReader ?? null,
        llm: configured.llm ?? options.llm ?? options.llmPort ?? options.llmStreamingPort ?? null
    });
}

function directFlowValue(source, aliases) {
    if (!isRecord(source)) return null;
    for (const alias of aliases) {
        if (source[alias] !== undefined) return source[alias];
    }
    return null;
}

function flowMethod(flow) {
    if (typeof flow === 'function') return flow;
    if (!isRecord(flow)) return null;
    for (const method of ['handle', 'run', 'execute', 'process']) {
        if (flow[method] !== undefined) {
            if (typeof flow[method] !== 'function') throw new TypeError(`Proactive job flow.${method} must be a function`);
            return flow[method].bind(flow);
        }
    }
    return null;
}

function registryFlow(flowRegistry, type) {
    if (!flowRegistry || typeof flowRegistry.get !== 'function') return null;
    for (const id of FLOW_IDS[type]) {
        const found = flowRegistry.get(id);
        if (found) return {id, registry: flowRegistry};
    }
    return null;
}

function resolveFlow(options, type) {
    const aliases = FLOW_ALIASES[type];
    const source = options.flows ?? options.jobFlows ?? options.applicationFlows;
    const direct = directFlowValue(source, aliases)
        ?? directFlowValue(options.handlers, aliases)
        ?? directFlowValue(options, aliases);
    const method = flowMethod(direct);
    if (method) return Object.freeze({kind: 'flow', value: direct, invoke: method});

    const registry = registryFlow(options.flowRegistry ?? options.registry, type);
    if (registry) {
        return Object.freeze({
            kind: 'registry',
            value: registry.registry,
            flowId: registry.id,
            invoke: (command, context) => registry.registry.run(registry.id, context, command)
        });
    }
    return null;
}

function normalizeOutcome(value) {
    if (!isRecord(value)) return {status: 'complete', result: value};
    const status = value.status ?? value.outcome;
    if (status === 'retry' || value.retry === true) {
        return {
            status: 'retry',
            result: value.result,
            error: value.error,
            retryAt: value.retryAt ?? value.retry_at ?? value.runAfter ?? value.run_after
        };
    }
    if (status === 'failed' || status === 'terminal' || value.terminal === true) {
        return {
            status: 'failed',
            result: value.result,
            error: value.error || 'Proactive job flow reported a terminal failure'
        };
    }
    if (status === 'complete') return {status: 'complete', result: value.result};
    return {status: 'complete', result: value.result === undefined ? value : value.result};
}

function normalizeFlowError(error) {
    if (error?.retryable === false || error?.terminal === true) throw error;
    throw error;
}

function blockerFor(type, flow) {
    if (flow) return null;
    const details = BLOCKER_DETAILS[type];
    return Object.freeze({
        type,
        code: details.code,
        owner: details.owner,
        missing: details.missing,
        reason: details.reason,
        action: `Register ${type} through an application flow before enabling this worker job.`
    });
}

function commandFor(type, job, context, ports) {
    const payload = parsePayload(job);
    const personaId = personaIdFor(job);
    const jobId = jobIdFor(job);
    return Object.freeze({
        type,
        version: PROACTIVE_JOB_SERVICE_VERSION,
        job,
        jobId,
        personaId,
        payload,
        context,
        now: context?.now ?? null,
        causationId: payload.causationId ?? payload.causation_id ?? jobId,
        correlationId: context?.correlationId ?? jobId,
        ports
    });
}

/**
 * Create application-side handlers for the proactive job types.
 *
 * A flow is intentionally optional during the migration. Missing flows are
 * reported by audit() and fail closed as terminal input rather than silently
 * routing a job back to a removed compatibility implementation or retrying it forever.
 */
export function createProactiveJobService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Proactive job service options must be an object');
    const ports = applicationPorts(options, options.repositories ?? options.repositoryPorts);
    const resolved = Object.fromEntries(PROACTIVE_JOB_TYPES.map(type => [type, resolveFlow(options, type)]));
    const blockers = Object.freeze(PROACTIVE_JOB_TYPES.map(type => blockerFor(type, resolved[type])).filter(Boolean));

    async function run(type, job, context = {}) {
        const requested = requiredText(type, 'Proactive job type', 120);
        const expected = canonicalJobType(requested);
        if (!PROACTIVE_JOB_TYPES.includes(expected)) throw terminalError(`Unsupported proactive job type: ${expected}`);
        requiredRecord(job, 'Proactive job');
        requiredRecord(context, 'Proactive job context');
        const actual = jobTypeFor(job);
        if (canonicalJobType(actual) !== expected) throw terminalError(`Proactive job type mismatch: expected ${expected}, got ${actual}`);
        const flow = resolved[expected];
        if (!flow) {
            const blocker = blockers.find(item => item.type === expected);
            throw Object.assign(new Error(blocker.reason), {
                retryable: false,
                terminal: true,
                code: blocker.code,
                blocker
            });
        }
        const command = commandFor(expected, job, context, ports);
        try {
            const result = flow.invoke(command, context);
            return normalizeOutcome(await result);
        } catch (error) {
            return normalizeFlowError(error);
        }
    }

    const canonicalHandlers = Object.fromEntries(
        PROACTIVE_JOB_TYPES.map(type => [type, (job, context = {}) => run(type, job, context)])
    );
    const handlerMap = Object.freeze({
        ...canonicalHandlers,
        ...Object.fromEntries(Object.entries(PROACTIVE_JOB_ALIASES).map(([alias, type]) => [alias, canonicalHandlers[type]]))
    });

    const service = {
        version: PROACTIVE_JOB_SERVICE_VERSION,
        types: PROACTIVE_JOB_TYPES,
        handlers: handlerMap,
        handlerMap,
        ports,
        blockers,
        run,
        register(target, {receiver} = {}) {
            if (!target || typeof target.register !== 'function') {
                throw new TypeError('Proactive job service register target must provide register()');
            }
            const boundReceiver = receiver ?? service;
            for (const [type, handler] of Object.entries(handlerMap)) target.register(type, handler, boundReceiver);
            return target;
        },
        list() {
            return [...PROACTIVE_JOB_TYPES];
        },
        has(type) {
            return PROACTIVE_JOB_TYPES.includes(type) || Object.hasOwn(PROACTIVE_JOB_ALIASES, type);
        },
        get(type) {
            return handlerMap[type] ?? null;
        },
        audit() {
            return Object.freeze({
                version: PROACTIVE_JOB_SERVICE_VERSION,
                ready: blockers.length === 0,
                registeredTypes: [...PROACTIVE_JOB_TYPES],
                availableTypes: PROACTIVE_JOB_TYPES.filter(type => Boolean(resolved[type])),
                blockers
            });
        },
        registrations() {
            return Object.freeze([...PROACTIVE_JOB_TYPES, ...Object.keys(PROACTIVE_JOB_ALIASES)].map(type => {
                const canonical = canonicalJobType(type);
                return Object.freeze({
                type,
                available: Boolean(resolved[canonical]),
                handler: handlerMap[type],
                blocker: blockerFor(canonical, resolved[canonical])
                });
            }));
        }
    };
    return Object.freeze(service);
}

export const createProactiveJobApplicationService = createProactiveJobService;
export default createProactiveJobService;
