import {createProactiveJobService} from '../../server/application/proactive-job-service.js';
import {createJobDispatcher} from '../../server/runtime/job-dispatcher.js';

export const NOW = '2026-08-21T00:00:00.000Z';

function leasedJob() {
    return {
        id: 'job_proactive_fixture',
        job_type: 'proactive_message',
        status: 'leased',
        lease_owner: 'worker_fixture',
        lease_expires_at: '2026-08-21T00:01:00.000Z',
        attempt_count: 1,
        max_attempts: 3,
        persona_id: 'persona_fixture',
        payload_json: JSON.stringify({eventId: 'event_fixture', causationId: 'cause_fixture'}),
        result_json: '{}'
    };
}

export function createProactiveSettlementFixture() {
    const job = leasedJob();
    const calls = [];
    const repository = {
        findLeased(input) {
            calls.push(['findLeased', input]);
            return job.status === 'leased' && job.lease_owner === input.leaseOwner ? job : null;
        },
        settle(input) {
            calls.push(['settle', input]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            job.status = input.status;
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.result_json = JSON.stringify(input.result ?? {});
            return {changed: true, status: input.status, job};
        },
        retry(input) {
            calls.push(['retry', input]);
            if (job.status !== 'leased' || job.lease_owner !== input.leaseOwner) return {changed: false, status: null, job: null};
            job.status = 'queued';
            job.lease_owner = null;
            job.lease_expires_at = null;
            job.run_after = input.runAfter;
            return {changed: true, status: 'queued', job};
        }
    };
    let flowCalls = 0;
    const service = createProactiveJobService({
        flows: {
            proactive_message(command) {
                flowCalls += 1;
                return {skipped: 'decision_send_false', causationId: command.causationId};
            }
        },
        repositories: {conversation: {findMessage() {}}, activity: {publish() {}}, lifeEvent: {findById() {}}}
    });
    const dispatcher = createJobDispatcher({repository, clock: () => NOW});
    service.register(dispatcher);
    const context = {
        leaseOwner: 'worker_fixture',
        leaseMs: 60_000,
        now: NOW,
        signal: new AbortController().signal,
        correlationId: 'correlation_fixture'
    };
    return Object.freeze({job, calls, repository, service, dispatcher, context, flowCalls: () => flowCalls});
}
