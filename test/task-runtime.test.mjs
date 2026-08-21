import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

import createTaskRuntime, {MAX_TASK_ERROR_LENGTH, TaskRuntimeError} from '../server/runtime/task-runtime.js';

function fakeTimers() {
    const timeouts = new Map();
    const intervals = new Map();
    let nextHandle = 0;
    return {
        timeouts,
        intervals,
        setTimeout(callback, delay) {
            const handle = ++nextHandle;
            timeouts.set(handle, {callback, delay});
            return handle;
        },
        clearTimeout(handle) {
            timeouts.delete(handle);
        },
        setInterval(callback, delay) {
            const handle = ++nextHandle;
            intervals.set(handle, {callback, delay});
            return handle;
        },
        clearInterval(handle) {
            intervals.delete(handle);
        },
        fireTimeout(handle) {
            const timer = timeouts.get(handle);
            if (!timer) return;
            timeouts.delete(handle);
            timer.callback();
        },
        fireAllIntervals() {
            for (const timer of intervals.values()) timer.callback();
        }
    };
}

function deferred() {
    let resolve;
    let reject;
    const promise = new Promise((resolvePromise, rejectPromise) => {
        resolve = resolvePromise;
        reject = rejectPromise;
    });
    return {promise, resolve, reject};
}

async function flushMicrotasks() {
    for (let index = 0; index < 4; index += 1) await Promise.resolve();
}

test('task runtime has no import or startup side effects', () => {
    const source = readFileSync(new URL('../server/runtime/task-runtime.js', import.meta.url), 'utf8');
    assert.doesNotMatch(source, /^\s*import\s/m);
    assert.doesNotMatch(source, /server\.js|express|better-sqlite3|fetch\s*\(|child_process|app\.listen/i);
});

test('start runs the task after startup delay, keeps one owner, and arms the interval', async () => {
    const timers = fakeTimers();
    const calls = [];
    const runtime = createTaskRuntime({
        owner: 'presence-task',
        startupDelayMs: 50,
        intervalMs: 2_000,
        timers,
        task(context) {
            calls.push({owner: context.owner, signal: context.signal, generation: context.generation});
        }
    });

    assert.equal(runtime.state, 'idle');
    assert.equal(await runtime.start(), true);
    assert.equal(await runtime.start(), false);
    assert.equal(runtime.owner, 'presence-task');
    assert.equal(timers.timeouts.size, 1);
    assert.equal(timers.intervals.size, 0);

    timers.fireTimeout(timers.timeouts.keys().next().value);
    await flushMicrotasks();
    assert.equal(calls.length, 1);
    assert.equal(calls[0].owner, 'presence-task');
    assert.equal(calls[0].signal.aborted, false);
    assert.equal(timers.intervals.size, 1);
    assert.equal(timers.intervals.values().next().value.delay, 2_000);

    timers.fireAllIntervals();
    await flushMicrotasks();
    await runtime.runNow();
    assert.equal(calls.length, 3);
    assert.equal(new Set(calls.map(call => call.owner)).size, 1);
    await runtime.stop();
});

test('zero startup delay schedules the first task and prevents overlapping executions', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const release = deferred();
    let count = 0;
    const runtime = createTaskRuntime({
        intervalMs: 10,
        timers,
        task() {
            count += 1;
            if (count === 1) {
                entered.resolve();
                return release.promise;
            }
            return undefined;
        }
    });

    await runtime.start();
    await entered.promise;
    timers.fireAllIntervals();
    timers.fireAllIntervals();
    assert.equal(count, 1);
    assert.equal(runtime.stats.skippedTaskCount, 2);
    release.resolve();
    await release.promise;
    await flushMicrotasks();
    await runtime.runNow();
    assert.equal(count, 2);
    await runtime.stop();
});

test('stop aborts the current task, clears timers, and is idempotent', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const aborted = deferred();
    const runtime = createTaskRuntime({
        startupDelayMs: 10,
        intervalMs: 10,
        timers,
        task({signal}) {
            entered.resolve();
            signal.addEventListener('abort', () => aborted.resolve(), {once: true});
            return new Promise(() => {});
        }
    });

    await runtime.start();
    const startupHandle = timers.timeouts.keys().next().value;
    timers.fireTimeout(startupHandle);
    await entered.promise;
    const firstStop = runtime.stop();
    const secondStop = runtime.stop();
    assert.strictEqual(firstStop, secondStop);
    await aborted.promise;
    assert.equal(await firstStop, true);
    assert.equal(runtime.state, 'stopped');
    assert.equal(runtime.signal, null);
    assert.equal(timers.timeouts.size, 0);
    assert.equal(timers.intervals.size, 0);
    assert.equal(await runtime.stop(), false);
});

test('task failures are bounded and delivered to onError without unhandled rejection', async () => {
    const timers = fakeTimers();
    const errors = [];
    const secret = `task-secret-${'x'.repeat(MAX_TASK_ERROR_LENGTH + 50)}`;
    const runtime = createTaskRuntime({
        timers,
        onError(error, context) {
            errors.push({error, context});
        },
        task() {
            throw new Error(secret);
        }
    });

    await runtime.start();
    await Promise.resolve();
    assert.equal(errors.length, 1);
    assert.equal(errors[0].error instanceof TaskRuntimeError, true);
    assert.equal(errors[0].error.message.length <= MAX_TASK_ERROR_LENGTH, true);
    assert.equal(errors[0].error.message.includes(secret), false);
    assert.equal('cause' in errors[0].error, false);
    assert.equal(errors[0].context.owner, runtime.owner);
    assert.equal(runtime.stats.failureCount, 1);
    await runtime.stop();
});

test('an old generation cannot clear or interrupt a restarted runtime', async () => {
    const timers = fakeTimers();
    const firstTask = deferred();
    const secondEntered = deferred();
    const controllers = [];
    const errors = [];
    let taskCount = 0;
    const runtime = createTaskRuntime({
        timers,
        abortControllerFactory() {
            const controller = new AbortController();
            controllers.push(controller);
            return controller;
        },
        task() {
            taskCount += 1;
            if (taskCount === 1) return firstTask.promise;
            secondEntered.resolve();
            return undefined;
        },
        onError(error) {
            errors.push(error);
        }
    });

    await runtime.start();
    await Promise.resolve();
    const firstStop = runtime.stop();
    await firstStop;
    const secondStart = runtime.start();
    await secondEntered.promise;
    assert.equal(await secondStart, true);
    assert.equal(controllers[0].signal.aborted, true);
    assert.equal(controllers[1].signal.aborted, false);
    assert.equal(runtime.state, 'running');

    firstTask.reject(new Error('stale generation failure'));
    await flushMicrotasks();
    assert.equal(errors.length, 0);
    assert.equal(runtime.state, 'running');
    assert.equal(timers.intervals.size, 1);
    await runtime.stop();
});

test('waitForTasks drains an abort-aware task before stop resolves', async () => {
    const timers = fakeTimers();
    const entered = deferred();
    const runtime = createTaskRuntime({
        timers,
        startupDelayMs: 0,
        task({signal}) {
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
