import {createHttpApp} from '../http/app.js';
import createStartupRuntime from './startup.js';
import createWorkerRuntime from './worker-runtime.js';

const DEFAULT_PORT = 4178;
const DEFAULT_HOST = '0.0.0.0';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function nonEmpty(value, fallback) {
    if (typeof value === 'string' && value.trim() !== '') return value.trim();
    return fallback;
}

function resolvePort(value, environment) {
    const candidate = value ?? environment?.PORT ?? DEFAULT_PORT;
    const port = Number(candidate);
    if (!Number.isInteger(port) || port < 0 || port > 65535) {
        throw new RangeError('Runtime port must be an integer between 0 and 65535');
    }
    return port;
}

function resolveStartup(options) {
    const configured = options.startupRuntime ?? options.startup;
    if (configured !== undefined) {
        if (!isRecord(configured) || !isRecord(configured.database) || typeof configured.close !== 'function') {
            throw new TypeError('Runtime startup must provide database and close()');
        }
        return configured;
    }
    const startupOptions = isRecord(options.startupOptions) ? options.startupOptions : {};
    return createStartupRuntime({
        ...startupOptions,
        environment: startupOptions.environment ?? options.environment ?? process.env,
        Database: startupOptions.Database ?? options.Database,
        dataDir: startupOptions.dataDir ?? options.dataDir,
        databasePath: startupOptions.databasePath ?? options.databasePath,
        now: startupOptions.now ?? options.now,
        clock: startupOptions.clock ?? options.clock,
        id: startupOptions.id ?? options.id,
        idGenerator: startupOptions.idGenerator ?? options.idGenerator,
        settings: startupOptions.settings ?? options.settings,
        migrations: startupOptions.migrations
    });
}

function resolveApp(options, startup, worker) {
    const configured = options.app;
    if (configured !== undefined) return configured;
    const httpOptions = isRecord(options.httpOptions) ? options.httpOptions : {};
    return createHttpApp({
        ...httpOptions,
        root: httpOptions.root ?? options.root,
        staticRoot: httpOptions.staticRoot ?? options.staticRoot,
        routeRegistrar: httpOptions.routeRegistrar ?? options.routeRegistrar,
        chatSseAdapter: httpOptions.chatSseAdapter ?? options.chatSseAdapter,
        healthResponse: httpOptions.healthResponse ?? options.healthResponse,
        worker,
        database: startup.database
    });
}

function resolveWorker(options, startup) {
    if (options.workerRuntime === false || options.worker === false) return null;
    const configured = options.workerRuntime ?? (isRecord(options.worker) ? options.worker : undefined);
    if (configured !== undefined) {
        if (!isRecord(configured) || typeof configured.start !== 'function' || typeof configured.stop !== 'function') {
            throw new TypeError('Runtime worker must provide start() and stop()');
        }
        return configured;
    }
    const workerOptions = isRecord(options.workerOptions) ? options.workerOptions : {};
    const hasWork = ['jobTick', 'tick', 'processTick', 'claimJob', 'runJob'].some(name => typeof workerOptions[name] === 'function' || typeof options[name] === 'function');
    if (!hasWork) return null;
    return createWorkerRuntime({
        ...workerOptions,
        jobRepository: workerOptions.jobRepository ?? options.jobRepository,
        jobTick: workerOptions.jobTick ?? options.jobTick,
        claimJob: workerOptions.claimJob ?? options.claimJob,
        runJob: workerOptions.runJob ?? options.runJob,
        recoverLeases: workerOptions.recoverLeases ?? options.recoverLeases,
        clock: workerOptions.clock ?? options.clock,
        timers: workerOptions.timers ?? options.timers,
        leaseOwner: workerOptions.leaseOwner ?? options.leaseOwner,
        onError: workerOptions.onError ?? options.onWorkerError
    });
}

function closeServer(server) {
    if (!server || typeof server.close !== 'function') return Promise.resolve();
    if (server.listening === false) return Promise.resolve();
    return new Promise((resolve, reject) => {
        server.close(error => error ? reject(error) : resolve());
    });
}

function listen(app, port, host) {
    if (!app || typeof app.listen !== 'function') throw new TypeError('Runtime app must provide listen()');
    return new Promise((resolve, reject) => {
        let settled = false;
        const onError = error => {
            if (settled) return;
            settled = true;
            reject(error);
        };
        const onListening = () => {
            // Express invokes the callback from the underlying `listen`
            // call, but a restricted host can emit `error` immediately after
            // that callback. Defer resolution until the handle confirms it is
            // actually listening so startup cannot report a false positive.
            const finish = () => {
                if (settled) return;
                if (server?.listening === false) return;
                settled = true;
                resolve(server ?? app);
            };
            if (server?.listening === false) {
                if (typeof setImmediate === 'function') setImmediate(finish);
                else queueMicrotask(finish);
                return;
            }
            finish();
        };
        app.once?.('error', onError);
        let server;
        try {
            server = app.listen(port, host, onListening);
        } catch (error) {
            onError(error);
            return;
        }
        if (server && typeof server.once === 'function') server.once('error', onError);
        if (server && typeof server.then === 'function') server.then(onListening, onError);
    });
}

/**
 * Compose the modular runtime without importing the legacy application root.
 * Startup opens SQLite eagerly, while HTTP binding and worker ownership begin
 * only when start() is called. All route and job behavior remains injected.
 */
export function createRuntime(options = {}) {
    if (!isRecord(options)) throw new TypeError('Runtime options must be an object');
    const environment = options.environment ?? options.env ?? process.env;
    if (!isRecord(environment)) throw new TypeError('Runtime environment must be an object');
    const startup = resolveStartup({...options, environment});
    const worker = resolveWorker(options, startup);
    const app = resolveApp(options, startup, worker);
    const port = resolvePort(options.port, environment);
    const host = nonEmpty(options.host ?? environment.HOST, DEFAULT_HOST);
    const repositories = options.repositories ?? Object.freeze({});
    const providers = options.providers ?? Object.freeze({});

    let phase = 'created';
    let server = null;
    let startPromise = null;
    let stopPromise = null;

    async function start({listen: shouldListen = true, worker: shouldStartWorker = true} = {}) {
        if (phase === 'running') return server;
        if (phase === 'starting') return startPromise;
        if (phase === 'stopping') return stopPromise.then(() => start({listen: shouldListen, worker: shouldStartWorker}));
        phase = 'starting';
        let pendingStart;
        pendingStart = (async () => {
            try {
                if (shouldListen) server = await listen(app, port, host);
                if (shouldStartWorker && worker) await worker.start();
                phase = 'running';
                return server;
            } catch (error) {
                await closeServer(server).catch(() => {});
                server = null;
                phase = 'created';
                throw error;
            } finally {
                if (startPromise === pendingStart) startPromise = null;
            }
        })();
        startPromise = pendingStart;
        return pendingStart;
    }

    async function stop() {
        if (phase === 'stopped' || phase === 'created') return false;
        if (phase === 'stopping') return stopPromise;
        phase = 'stopping';
        let pendingStop;
        pendingStop = (async () => {
            try {
                if (worker) await worker.stop();
                await closeServer(server);
                server = null;
                startup.close();
                phase = 'stopped';
                return true;
            } finally {
                if (stopPromise === pendingStop) stopPromise = null;
            }
        })();
        stopPromise = pendingStop;
        return pendingStop;
    }

    return Object.freeze({
        startup,
        database: startup.database,
        databaseConfig: startup.databaseConfig,
        repositories,
        providers,
        app,
        worker,
        port,
        host,
        start,
        stop,
        get server() {
            return server;
        },
        get state() {
            return phase;
        }
    });
}

export const createCompanionRuntime = createRuntime;
export default createRuntime;
