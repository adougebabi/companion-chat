import assert from 'node:assert/strict';
import test from 'node:test';

import {createMediaObservability} from '../server/application/media-observability.js';

function fixture(overrides = {}) {
    let time = 0;
    const calls = [];
    const options = {
        progressParser(value) {
            calls.push(['parse', value]);
            return {output: String(value).replace(/secret/gi, '[redacted]'), percent: String(value).match(/(\d+(?:\.\d+)?)%/)?.[1] ?? null};
        },
        progressWriter(job, patch) {
            calls.push(['write', job.id, patch]);
            return {changed: true, progress: patch};
        },
        leaseGuard(job) {
            calls.push(['guard', job.id]);
            return job.lease !== 'stale';
        },
        reporterFactory({report}) {
            return {report, stage: report.stage, output: report.output, flush: report.flush};
        },
        settleJob(job, options) {
            calls.push(['settle', job.id, options]);
            return {changed: true, status: options.error ? (options.terminal ? 'failed' : 'queued') : 'complete'};
        },
        debugProjector(value) {
            calls.push(['debug', value]);
            return value;
        },
        runProcess(...args) {
            calls.push(['process', ...args]);
            return {ok: true};
        },
        now() {
            return time;
        },
        progressIntervalMs: 1_000,
        ...overrides
    };
    return {observability: createMediaObservability(options), calls, setTime(value) { time = value; }};
}

test('media observability delegates parsing, guards leases, and settles terminal failures', () => {
    const fixtureValue = fixture();
    const job = {id: 'job_valid', lease: 'valid'};

    assert.deepEqual(fixtureValue.observability.parseProgress('12.5% secret'), {output: '12.5% [redacted]', percent: 12.5});
    assert.deepEqual(fixtureValue.observability.recordProgress(job, {stage: 'generating', output: '12.5% secret'}), {
        changed: true,
        progress: {stage: 'generating', output: '12.5% [redacted]', percent: 12.5}
    });
    assert.deepEqual(fixtureValue.observability.settle(job, {error: 'x'.repeat(500), terminal: true}), {changed: true, status: 'failed'});
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'write'), true);
    assert.equal(fixtureValue.calls.find(([kind]) => kind === 'settle')[2].error.length, 240);
});

test('stale leases reject progress and settlement without invoking writers', () => {
    const fixtureValue = fixture();
    const stale = {id: 'job_stale', lease: 'stale'};

    assert.deepEqual(fixtureValue.observability.recordProgress(stale, {output: '99% stale'}), {changed: false, reason: 'lease_rejected'});
    assert.deepEqual(fixtureValue.observability.settle(stale, {terminal: true, error: 'stale'}), {changed: false, reason: 'lease_rejected'});
    assert.equal(fixtureValue.calls.some(([kind]) => kind === 'write' || kind === 'settle'), false);
});

test('reporters throttle ordinary output and force stage/flush writes', () => {
    const fixtureValue = fixture();
    const reporter = fixtureValue.observability.createReporter({id: 'job_throttle', lease: 'valid'});

    assert.equal(reporter.output('stdout', '1% preparing').changed, true);
    assert.equal(reporter.output('stderr', '2% sampling').throttled, true);
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'write').length, 1);
    fixtureValue.setTime(1_000);
    assert.equal(reporter.flush().changed, true);
    assert.equal(fixtureValue.calls.filter(([kind]) => kind === 'write').length, 2);
    assert.equal(fixtureValue.calls.at(-1)[2].output, '2% sampling');
    fixtureValue.setTime(1_001);
    assert.equal(reporter.stage('complete').changed, true);
    assert.equal(fixtureValue.calls.at(-1)[2].stage, 'complete');
});

test('debug projection and process execution stay bounded and injected', () => {
    const fixtureValue = fixture();
    const projected = fixtureValue.observability.observeDebug({
        records: Array.from({length: 20}, (_, index) => ({id: index, text: 'x'.repeat(3_000)}))
    });
    assert.equal(projected.records.length, 10);
    assert.equal(projected.records[0].text.length, 2_000);
    assert.deepEqual(fixtureValue.observability.runProcess('demo', ['--help']), {ok: true});
    assert.deepEqual(fixtureValue.calls.at(-1), ['process', 'demo', ['--help']]);
});
