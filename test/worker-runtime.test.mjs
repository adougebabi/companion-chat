import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

import createWorkerRuntime from '../server/runtime/worker-runtime.js';

function fakeTimers() {
    const intervals = new Map();
    const timeouts = new Map();
    let nextHandle = 0;
    return {
        intervals,
        timeouts,
        setInterval(callback, delay) {
            const handle = ++nextHandle;
            intervals.set(handle, {callback, delay});
            return handle;
        },
        clearInterval(handle) {
            intervals.delete(handle);
        },
        setTimeout(callback, delay) {
            const handle = ++nextHandle;
            timeouts.set(handle, {callback, delay});
            return handle;
        },
        clearTimeout(handle) {
            timeouts.delete(handle);
        },
        fireInterval(handle) {
            const timer = intervals.get(handle);
            if (timer) timer.callback();
        },
        fireAllIntervals() {
            for (const handle of [...intervals.keys()]) this.fireInterval(handle);
        },
        fireTimeout(handle) {
            const timer = timeouts.get(handle);
            if (timer) {
                timeouts.delete(handle);
                timer.callback();
            }
        }
    };
}

function deferred() {
    let resolve;
    const promise = new Promise(value => { resolve = value; });
    return {promise, resolve};
}

test('worker runtime module is inert when imported', () => {
    const source = readFileSync(new URL('../server/runtime/worker-runtime.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(|child_process|app\.listen/i);
});

test('start performs one lease recovery, schedules one timer, and prevents duplicate starts', async () => {
    const timers = fakeTimers();
    const calls = [];
    const runtime = createWorkerRuntime({
        leaseOwner: 'worker_test',
        pollIntervalMs: 25,
        timers,
        recoverLeases(context) {
            calls.push(['recover', context.leaseOwner, context.signal.aborted]);
        },
        jobTick(context) {
            calls.push(['tick', context.leaseOwner, context.signal.aborted]);
        },
        onError: error => { throw error; }
    });

    assert.equal(runtime.state, 'idle');
    assert.equal(await runtime.start(), true);
    assert.equal(await runtime.start(), false);
    assert.equal(runtime.state, 'running');
    assert.equal(timers.intervals.size, 1);
    assert.deepEqual(calls, [['recover', 'worker_test', false]]);
    assert.equal(timers.intervals.values().next().value.delay, 25);

    timers.fireAllIntervals();
    await runtime.tick();
    assert.deepEqual(calls, [
        ['recover', 'worker_test', false], ['recover', 'worker_test', false], ['tick', 'worker_test', false],
    ]);
    await runtime.stop();
});

test('default claim/run adapter preserves one lease owner and supports expired-lease recovery', async () => {
    const timers = fakeTimers();
    const calls = [];
    let available = true;
    const runtime = createWorkerRuntime({
        owner: 'owner_from_port',
        timers,
        clock: () => '2026-08-20T00:00:00.000Z',
        leaseMs: 90_000,
        claimJob(input) {
            calls.push(['claim', input.leaseOwner, input.now, input.leaseMs, input.signal.aborted]);
            if (!available) return null;
            available = false;
            return {id: 'expired_job', status: 'leased'};
        },
        runJob(job, context) {
            calls.push(['run', job.id, context.leaseOwner, context.signal.aborted]);
        },
        onError: error => { throw error; }
    });

    await runtime.start();
    assert.deepEqual(calls, []);
    timers.fireAllIntervals();
    await runtime.tick();
    assert.deepEqual(calls, [
        ['claim', 'owner_from_port', '2026-08-20T00:00:00.000Z', 90_000, false],
        ['run', 'expired_job', 'owner_from_port', false]
    ]);
    timers.fireAllIntervals();
    await runtime.tick();
    assert.equal(calls.filter(([kind]) => kind === 'claim').length, 2);
    assert.equal(runtime.stats.recoveryCount, 0);
    await runtime.stop();
});

test('stop clears injected timers, aborts the active signal, and is idempotent', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const aborted = deferred();
    const runtime = createWorkerRuntime({
        leaseOwner: 'worker_stop',
        timers,
        runOnStart: true,
        jobTick({signal}) {
            entered.resolve();
            signal.addEventListener('abort', () => aborted.resolve(), {once: true});
            return new Promise(() => {});
        },
        onError: () => {}
    });

    await runtime.start();
    await entered.promise;
    const firstStop = runtime.stop();
    const secondStop = runtime.stop();
    assert.strictEqual(firstStop, secondStop);
    await aborted.promise;
    await firstStop;
    assert.equal(runtime.state, 'stopped');
    assert.equal(runtime.signal, null);
    assert.equal(timers.intervals.size, 0);
    assert.equal(timers.timeouts.size, 0);
    assert.equal(await runtime.stop(), false);
});

test('overlapping timer callbacks do not create a second job tick', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const release = deferred();
    let ticks = 0;
    const runtime = createWorkerRuntime({
        timers,
        runOnStart: true,
        jobTick() {
            ticks += 1;
            if (ticks === 1) {
                entered.resolve();
                return release.promise;
            }
            return undefined;
        },
        onError: error => { throw error; }
    });

    await runtime.start();
    await entered.promise;
    timers.fireAllIntervals();
    timers.fireAllIntervals();
    assert.equal(ticks, 1);
    assert.equal(runtime.stats.skippedTickCount, 2);
    release.resolve();
    await release.promise;
    await Promise.resolve();
    await runtime.tick();
    await runtime.tick();
    assert.equal(ticks, 2);
    await runtime.stop();
});

test('startup delay is injectable and stop removes a pending startup timer', async () => {
    const timers = fakeTimers();
    let ticks = 0;
    const runtime = createWorkerRuntime({
        timers,
        startupDelayMs: 10,
        jobTick: () => { ticks += 1; },
        onError: error => { throw error; }
    });

    await runtime.start();
    assert.equal(timers.timeouts.size, 1);
    assert.equal(ticks, 0);
    await runtime.stop();
    assert.equal(timers.timeouts.size, 0);
    assert.equal(timers.intervals.size, 0);
    assert.equal(ticks, 0);
});

test('a stopped startup cannot clear or abort a later runtime generation', async () => {
    const timers = fakeTimers();
    const recoveryOne = deferred();
    const recoveryTwo = deferred();
    const controllers = [];
    let recoveryCalls = 0;
    const runtime = createWorkerRuntime({
        timers,
        abortControllerFactory() {
            const controller = new AbortController();
            controllers.push(controller);
            return controller;
        },
        recoverLeases() {
            recoveryCalls += 1;
            return recoveryCalls === 1 ? recoveryOne.promise : recoveryTwo.promise;
        },
        jobTick: () => {},
        onError: () => {}
    });

    const firstStart = runtime.start();
    const firstStop = runtime.stop();
    await firstStop;
    assert.equal(runtime.state, 'stopped');

    const secondStart = runtime.start();
    assert.notStrictEqual(firstStart, secondStart);
    recoveryTwo.resolve();
    assert.equal(await secondStart, true);
    assert.equal(runtime.state, 'running');
    assert.equal(controllers[0].signal.aborted, true);
    assert.equal(controllers[1].signal.aborted, false);
    assert.equal(timers.intervals.size, 1);

    recoveryOne.resolve();
    assert.equal(await firstStart, false);
    assert.equal(runtime.state, 'running');
    assert.equal(timers.intervals.size, 1);
    await runtime.stop();
});

test('waitForTasks drains an abort-aware tick before stop resolves', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const runtime = createWorkerRuntime({
        timers,
        runOnStart: true,
        jobTick({signal}) {
            entered.resolve();
            return new Promise(resolve => signal.addEventListener('abort', resolve, {once: true}));
        },
        onError: () => {}
    });
    await runtime.start();
    await entered.promise;
    await runtime.stop({waitForTasks: true, drainTimeoutMs: 100});
    assert.equal(runtime.state, 'stopped');
});
