import {spawn} from 'node:child_process';

import {createMediaJobApplication, createMediaObservabilityApplication} from '../../server/application/media-job-composition.js';
import {createJobDispatcher} from '../../server/runtime/job-dispatcher.js';

export const NOW = '2026-08-21T00:00:00.000Z';

function parseJson(value, fallback = {}) {
    if (value && typeof value === 'object') return value;
    if (typeof value !== 'string' || !value.trim()) return fallback;
    try {
        const parsed = JSON.parse(value);
        return parsed && typeof parsed === 'object' ? parsed : fallback;
    } catch {
        return fallback;
    }
}

function clone(value) {
    return structuredClone(value);
}

function safeProgressText(value) {
    return String(value ?? '')
        .replace(/[\u001b\u009b][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[-a-zA-Z\d/#&.:=?%@~_]+)*)?\u0007)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[a-zA-Z\d]))/g, '')
        .replace(/\b(?:Bearer|Key)\s+[A-Za-z0-9._-]+/gi, '[redacted]')
        .replace(/(?:[A-Za-z]:)?\/[^\s]+/g, '[path]')
        .replace(/[\r\n]+/g, ' ')
        .replace(/\s+/g, ' ')
        .trim()
        .slice(0, 480);
}

function parseProgress(value) {
    const output = safeProgressText(value);
    const match = output.match(/(\d+(?:\.\d+)?)%/);
    const percent = match ? Math.max(0, Math.min(100, Number(match[1]))) : null;
    return {output, ...(Number.isFinite(percent) ? {percent} : {})};
}

function active(job, owner, now) {
    return job?.status === 'leased'
        && job.lease_owner === owner
        && (!job.lease_expires_at || Date.parse(job.lease_expires_at) > Date.parse(now));
}

function initialProgress(job, stage, now) {
    return {
        schemaVersion: 1,
        attempt: Math.max(1, Number(job.attempt_count) || 1),
        stage: stage || 'preparing',
        percent: null,
        startedAt: now,
        updatedAt: now,
        elapsedMs: 0,
        latestOutput: '',
        latestStream: null,
        outputSeen: false,
        outputLineCount: 0
    };
}

function progressFor(job, patch, now) {
    const result = parseJson(job.result_json, {});
    const current = result.progress && typeof result.progress === 'object' ? result.progress : null;
    const attempt = Math.max(1, Number(job.attempt_count) || 1);
    const base = current?.attempt === attempt ? current : initialProgress(job, patch.stage, now);
    const parsed = patch.output === undefined ? null : parseProgress(patch.output);
    const latestOutput = parsed?.output ?? (patch.latestOutput === undefined ? base.latestOutput : safeProgressText(patch.latestOutput));
    const percent = patch.percent === undefined || patch.percent === null
        ? (parsed?.percent ?? base.percent)
        : Math.max(0, Math.min(100, Number(patch.percent)));
    return {
        ...base,
        schemaVersion: 1,
        attempt,
        ...(patch.stage === undefined ? {} : {stage: String(patch.stage).slice(0, 80)}),
        percent: Number.isFinite(percent) ? percent : null,
        startedAt: base.startedAt || now,
        updatedAt: now,
        latestOutput,
        ...(patch.latestStream === 'stdout' || patch.latestStream === 'stderr' ? {latestStream: patch.latestStream} : {}),
        outputSeen: Boolean(base.outputSeen || latestOutput),
        outputLineCount: Math.min(1_000_000, (Number(base.outputLineCount) || 0) + Math.max(0, Number(patch.outputLineCountDelta) || 0))
    };
}

function runProcess(executable, args, timeoutMs, {onOutput} = {}) {
    return new Promise((resolve, reject) => {
        const child = spawn(executable, args, {stdio: ['ignore', 'pipe', 'pipe'], shell: false});
        let settled = false;
        const finish = (settler, value) => {
            if (settled) return;
            settled = true;
            clearTimeout(timer);
            settler(value);
        };
        const capture = (stream, name) => {
            let pending = '';
            stream.on('data', chunk => {
                pending += String(chunk ?? '');
                const parts = pending.split(/[\r\n]+/);
                pending = parts.pop() || '';
                for (const part of parts) if (part) onOutput?.(name, part);
            });
            stream.once('end', () => {
                if (pending) onOutput?.(name, pending);
            });
        };
        capture(child.stdout, 'stdout');
        capture(child.stderr, 'stderr');
        const timer = setTimeout(() => {
            child.kill('SIGTERM');
            finish(reject, new Error('fixture process timed out'));
        }, timeoutMs);
        child.once('error', () => finish(reject, new Error('fixture process failed to start')));
        child.once('close', (code, signal) => {
            if (code === 0) finish(resolve, {});
            else finish(reject, new Error(signal ? 'fixture process terminated' : `fixture process exited ${code}`));
        });
    });
}

function createRepository({calls, jobs}) {
    const repository = {
        findLeased(input) {
            const job = jobs.get(input.id);
            return active(job, input.leaseOwner, input.now) ? job : null;
        },
        patchResult(job, input) {
            const stored = jobs.get(job.id);
            if (!active(stored, input.leaseOwner ?? stored?.lease_owner, input.now ?? NOW)) return {changed: false, result: null, job: null};
            const result = {...parseJson(stored.result_json, {}), ...clone(input.patch || {})};
            stored.result_json = JSON.stringify(result);
            stored.updated_at = input.now ?? NOW;
            calls.push(['patch', stored.id, input.patch]);
            return {changed: true, result, job: stored};
        },
        settle(input) {
            const job = jobs.get(input.id);
            if (!active(job, input.leaseOwner, input.now)) return {changed: false, status: null, job: null};
            if (input.result !== undefined) job.result_json = JSON.stringify({...parseJson(job.result_json, {}), ...clone(input.result)});
            if (input.error) job.error = input.error;
            job.status = input.status || 'complete';
            if (job.status === 'complete' && parseJson(job.result_json, {}).progress) {
                const progress = progressFor(job, {stage: 'complete', percent: 100}, input.now ?? NOW);
                job.result_json = JSON.stringify({...parseJson(job.result_json, {}), progress});
            }
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.completed_at = input.now ?? NOW;
            calls.push(['settle', job.status]);
            return {changed: true, status: job.status, job};
        },
        retry(input) {
            const job = jobs.get(input.id);
            if (!active(job, input.leaseOwner, input.now)) return {changed: false, status: null, job: null};
            if (input.result !== undefined) job.result_json = JSON.stringify({...parseJson(job.result_json, {}), ...clone(input.result)});
            if (input.error) job.error = input.error;
            job.status = 'queued';
            job.run_after = input.runAfter;
            job.lease_owner = null;
            job.lease_expires_at = null;
            calls.push(['retry', job.id]);
            return {changed: true, status: 'queued', job};
        },
        find(input) {
            return jobs.get(input.id) || null;
        },
        enqueue(input) {
            const job = {
                id: input.id || `poll_${jobs.size + 1}`,
                job_type: input.jobType,
                status: 'queued',
                lease_owner: null,
                lease_expires_at: null,
                attempt_count: 0,
                max_attempts: input.maxAttempts || 3,
                persona_id: input.personaId,
                payload_json: JSON.stringify(input.payload || {}),
                result_json: '{}'
            };
            jobs.set(job.id, job);
            return job;
        }
    };
    return repository;
}

export function createMediaObservabilityFixture({provider = {}} = {}) {
    const calls = [];
    const jobs = new Map();
    const repository = createRepository({calls, jobs});
    const observabilityApplication = createMediaObservabilityApplication({
        progressParser: parseProgress,
        progressWriter(job, patch) {
            const progress = progressFor(job, patch, NOW);
            const result = repository.patchResult(job, {patch: {progress}, now: NOW, leaseOwner: job.lease_owner});
            return {...result, progress};
        },
        leaseGuard(job) {
            return repository.findLeased({id: job.id, leaseOwner: job.lease_owner, now: NOW});
        },
        settleJob(job, input) {
            const status = input.status ?? (input.error ? 'failed' : 'complete');
            const progressPatch = input.progressStage ? {stage: input.progressStage} : {};
            if (Object.keys(progressPatch).length) {
                const progress = progressFor(job, progressPatch, NOW);
                repository.patchResult(job, {patch: {progress}, now: NOW, leaseOwner: job.lease_owner});
            }
            return status === 'retry'
                ? repository.retry({...input, id: job.id, personaId: job.persona_id, status, leaseOwner: job.lease_owner, now: NOW})
                : repository.settle({...input, id: job.id, personaId: job.persona_id, status, leaseOwner: job.lease_owner, now: NOW});
        },
        debugProjector(value) {
            return value;
        },
        runProcess,
        reporterFactory: ({report}) => {
            const withParsedPercent = (patch, options) => {
                const parsed = patch.output === undefined ? {} : parseProgress(patch.output);
                return report({
                    ...patch,
                    ...(parsed.percent === undefined ? {} : {percent: parsed.percent})
                }, options);
            };
            withParsedPercent.stage = stage => report.stage(stage);
            withParsedPercent.output = (stream, output) => {
                const result = report.output(stream, output);
                const parsed = parseProgress(output);
                if (parsed.percent === undefined) return result;
                report({percent: parsed.percent});
                return result;
            };
            withParsedPercent.flush = () => report.flush();
            return withParsedPercent;
        },
        now: () => NOW
    });
    const mediaProvider = {
        id: 'h3',
        portType: 'media',
        capabilities: ['image', 'video'],
        async submit() {
            calls.push(['provider', 'submit']);
            return provider.submit ? provider.submit() : {
                externalId: 'h3_fixture_result',
                pending: false,
                files: [{filename: 'fixture.mp4', type: 'h3', format: 'video'}]
            };
        },
        async poll() {
            calls.push(['provider', 'poll']);
            return provider.poll ? provider.poll() : {status: 'complete', files: [{filename: 'fixture.mp4', type: 'h3', format: 'video'}]};
        }
    };
    const mediaJobService = createMediaJobApplication({
        providers: {h3: mediaProvider},
        repositories: {job: repository},
        observability: observabilityApplication.observability,
        promptMaster: {fill: () => ({finalPrompt: '最终送往 h3 的安全提示词'})},
        acceptance: {accept: () => ({verdict: 'pass'})},
        clock: () => NOW
    });
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    for (const type of mediaJobService.list()) {
        dispatcher.register(type, (job, context) => mediaJobService.get(type)(job, {...context, deferSettlement: true}));
    }

    function mediaJob(id, {kind = 'video', status = 'leased', leaseOwner = 'media_fixture_worker', attempt = 1, ...overrides} = {}) {
        const job = {
            id,
            job_type: kind === 'video' ? 'chat_video' : 'chat_image',
            status,
            lease_owner: status === 'leased' ? leaseOwner : null,
            lease_expires_at: status === 'leased' ? '2026-08-21T00:01:00.000Z' : null,
            attempt_count: attempt,
            max_attempts: 3,
            persona_id: 'persona_fixture',
            payload_json: JSON.stringify({
                kind,
                provider: 'h3',
                envelope: {schemaVersion: 1, mediaKind: kind},
                personaMediaConcept: {schemaVersion: 1, mediaKind: kind}
            }),
            result_json: '{}',
            ...overrides
        };
        jobs.set(job.id, job);
        return job;
    }

    function lease(job, owner, attempt = job.attempt_count || 1) {
        job.status = 'leased';
        job.lease_owner = owner;
        job.lease_expires_at = '2026-08-21T00:01:00.000Z';
        job.attempt_count = attempt;
        return job;
    }

    return Object.freeze({
        NOW,
        calls,
        jobs,
        repository,
        observability: observabilityApplication.observability,
        mediaJobService,
        dispatcher,
        mediaJob,
        lease,
        runProcess
    });
}
