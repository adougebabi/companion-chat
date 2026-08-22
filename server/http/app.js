import express from 'express';
import {basename, dirname, join, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {brotliCompressSync, constants as zlibConstants, gzipSync} from 'node:zlib';

const DEFAULT_JSON_LIMIT = '12mb';
const DEFAULT_API_CACHE_CONTROL = 'no-store, max-age=0';
const DEFAULT_STATIC_CACHE_CONTROL = 'no-store, max-age=0';
const VERSIONED_STATIC_CACHE_CONTROL = 'public, max-age=31536000, immutable';
const DEFAULT_SSE_HEADERS = Object.freeze({
    'Content-Type': 'text/event-stream; charset=utf-8',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive'
});
const DEFAULT_HEALTH_RESPONSE = Object.freeze({ok: true, storage: 'companion-v2'});
const STATIC_NO_STORE_FILES = new Set(['index.html']);

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function resolveExpressFactory(options) {
    const configured = options.expressFactory ?? options.express;
    const candidate = configured?.default ?? configured ?? express;
    if (typeof candidate !== 'function') throw new TypeError('HTTP app requires an Express factory');
    if (typeof candidate.json !== 'function' || typeof candidate.static !== 'function') {
        throw new TypeError('HTTP app Express factory must provide json() and static()');
    }
    return candidate;
}

function resolveApp(options, expressFactory) {
    const app = options.app ?? expressFactory();
    if (!app || (typeof app !== 'object' && typeof app !== 'function')) throw new TypeError('HTTP app factory must return an app object');
    for (const method of ['use', 'get', 'post']) {
        if (typeof app[method] !== 'function') throw new TypeError(`HTTP app must provide ${method}()`);
    }
    return app;
}

function resolveStaticRoot(options) {
    const configured = options.staticRoot ?? options.staticDir;
    if (configured !== undefined) return resolve(String(configured));
    const root = options.root === undefined ? projectRoot : resolve(String(options.root));
    return join(root, 'dist');
}

function responseHeaders(res, headers) {
    if (typeof res?.set === 'function') {
        res.set(headers);
        return;
    }
    if (typeof res?.setHeader === 'function') {
        for (const [name, value] of Object.entries(headers)) res.setHeader(name, value);
    }
}

function hasResponseEnded(res) {
    return Boolean(res?.headersSent || res?.writableEnded || res?.writableFinished || res?.destroyed);
}

function errorStatus(error, fallbackStatus = 400) {
    const value = error?.status ?? error?.statusCode;
    return Number.isInteger(value) && value >= 400 && value <= 599 ? value : fallbackStatus;
}

function errorMessage(error, fallbackMessage = '请求无法处理') {
    if (typeof error === 'string' && error.trim()) return error;
    if (typeof error?.error === 'string' && error.error.trim()) return error.error;
    if (typeof error?.message === 'string' && error.message.trim()) return error.message;
    return fallbackMessage;
}

/**
 * Send the existing small HTTP error contract once. A route that has already
 * committed an SSE or other response must be left to Express' final handler.
 */
export function sendHttpError(res, error, {defaultStatus = 400, fallbackMessage = '请求无法处理'} = {}) {
    if (hasResponseEnded(res)) return false;
    const status = errorStatus(error, defaultStatus);
    const payload = {error: errorMessage(error, fallbackMessage)};
    if (typeof res?.status === 'function') res.status(status);
    if (typeof res?.json === 'function') {
        res.json(payload);
        return true;
    }
    if (typeof res?.end === 'function') {
        responseHeaders(res, {'Content-Type': 'application/json; charset=utf-8'});
        res.end(JSON.stringify(payload));
        return true;
    }
    return false;
}

/**
 * Wrap a route so both synchronous throws and rejected promises use the same
 * status and `{error}` response shape as the legacy HTTP entrypoint.
 */
export function wrapHttpRoute(handler, options = {}) {
    if (typeof handler !== 'function') throw new TypeError('HTTP route handler must be a function');
    return function wrappedHttpRoute(req, res, next) {
        const fail = error => {
            if (hasResponseEnded(res)) {
                if (typeof next === 'function') return next(error);
                return undefined;
            }
            return sendHttpError(res, error, options);
        };

        try {
            const output = handler(req, res, next);
            if (output && typeof output.then === 'function') return output.catch(fail);
            return output;
        } catch (error) {
            return fail(error);
        }
    };
}

function createErrorMiddleware(options) {
    return function httpErrorMiddleware(error, req, res, next) {
        if (hasResponseEnded(res)) {
            if (typeof next === 'function') return next(error);
            return undefined;
        }
        return sendHttpError(res, error, options);
    };
}

function staticHeaders(options) {
    const configured = options.staticSetHeaders ?? options.setStaticHeaders;
    const cacheControl = options.staticCacheControl ?? DEFAULT_STATIC_CACHE_CONTROL;
    return (res, filePath) => {
        const pathParts = String(filePath).split(/[\\/]/);
        if (STATIC_NO_STORE_FILES.has(basename(filePath))) {
            responseHeaders(res, {'Cache-Control': cacheControl});
        } else if (pathParts.includes('assets')) {
            responseHeaders(res, {'Cache-Control': VERSIONED_STATIC_CACHE_CONTROL});
        }
        if (options.staticCompression !== false) installStaticCompression(res, filePath);
        if (typeof configured === 'function') configured(res, filePath);
    };
}

function installStaticCompression(res, filePath) {
    const req = res?.req;
    const path = String(filePath).split('?')[0];
    const encoding = acceptedStaticEncoding(req);
    if (req?.method === 'HEAD' || req?.method !== 'GET' || !path.split(/[\\/]/).includes('assets') || !/\.(?:css|html|js|json|map|svg|wasm)$/i.test(path) || !encoding || res?.__companionStaticCompression) return;
    if (!res) return;
    res.__companionStaticCompression = true;
    const chunks = [];
    const originalEnd = res.end.bind(res);
    res.write = (chunk, chunkEncoding, callback) => {
        if (chunk !== undefined && chunk !== null) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk, chunkEncoding));
        if (typeof callback === 'function') process.nextTick(callback);
        return true;
    };
    res.end = (chunk, chunkEncoding, callback) => {
        if (typeof chunkEncoding === 'function') {
            callback = chunkEncoding;
            chunkEncoding = undefined;
        }
        if (chunk !== undefined && chunk !== null) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk, chunkEncoding));
        const body = chunks.length ? Buffer.concat(chunks) : Buffer.alloc(0);
        const existingEncoding = res.getHeader?.('Content-Encoding');
        if (body.length && !existingEncoding) {
            const compressed = compressStaticBody(body, encoding);
            res.setHeader?.('Content-Encoding', encoding);
            res.setHeader?.('Vary', 'Accept-Encoding');
            res.removeHeader?.('Content-Length');
            return originalEnd(compressed, callback);
        }
        return originalEnd(body.length ? body : undefined, callback);
    };
}

function acceptedStaticEncoding(req) {
    const header = String(req?.headers?.['accept-encoding'] ?? req?.get?.('accept-encoding') ?? '').toLowerCase();
    if (/\bbr\b/.test(header)) return 'br';
    if (/\bgzip\b/.test(header)) return 'gzip';
    return null;
}

function compressStaticBody(body, encoding) {
    if (encoding === 'br') {
        return brotliCompressSync(body, {params: {[zlibConstants.BROTLI_PARAM_QUALITY]: 5}});
    }
    return gzipSync(body, {level: 6});
}

function apiCacheMiddleware(options) {
    const cacheControl = options.apiCacheControl ?? DEFAULT_API_CACHE_CONTROL;
    return (req, res, next) => {
        responseHeaders(res, {'Cache-Control': cacheControl});
        next();
    };
}

function routeHandler(value, label) {
    if (typeof value === 'function') return value;
    if (!isRecord(value)) throw new TypeError(`HTTP ${label} must be a function or handler object`);
    for (const method of ['handle', 'handleChatTurn', 'run', 'stream']) {
        if (typeof value[method] === 'function') return value[method].bind(value);
    }
    throw new TypeError(`HTTP ${label} must provide handle()`);
}

function invokeFactory(factory, first, second) {
    if (typeof factory !== 'function') return factory;
    return factory(first, second);
}

function sseHeaders(res) {
    if (!hasResponseEnded(res)) {
        if (typeof res?.status === 'function') res.status(200);
        responseHeaders(res, DEFAULT_SSE_HEADERS);
        res?.flushHeaders?.();
    }
}

function defaultChatContext(req, res) {
    return {req, request: req, res, response: res};
}

function defaultChatCommand(req) {
    return isRecord(req?.body) ? req.body : {};
}

function createChatAdapterRoute(options, adapter) {
    const contextFactory = options.chatContext ?? options.chatContextFactory;
    const commandFactory = options.chatCommand ?? options.chatCommandFactory;
    return wrapHttpRoute((req, res) => {
        sseHeaders(res);
        const context = invokeFactory(contextFactory, req, res) ?? defaultChatContext(req, res);
        const command = invokeFactory(commandFactory, req, res) ?? defaultChatCommand(req);
        if (!isRecord(context)) throw new TypeError('Chat route context must be an object');
        if (!isRecord(command)) throw new TypeError('Chat route command must be an object');
        // chat-turn-sse-adapter accepts this envelope and uses the second
        // argument as its response sink. Keeping req/res in the envelope also
        // lets it observe disconnects without knowing about Express.
        return adapter({context, command, req, res, sink: res}, res);
    }, {defaultStatus: 400});
}

function registerChatRoute({app, options, adapter}) {
    const path = options.chatPath ?? '/api/companion/chat';
    const directRoute = options.chatRoute ?? options.chatSseRoute;
    const handler = directRoute === undefined
        ? createChatAdapterRoute(options, adapter)
        : wrapHttpRoute(routeHandler(directRoute, 'chat route'), {defaultStatus: 400});
    app.post(path, handler);
    return handler;
}

function resolveRouteRegistrar(options) {
    return options.routeRegistrar ?? options.registerRoutes ?? options.routes;
}

function registerRoutes({app, options, expressFactory, chatSseAdapter, registerChat}) {
    const registrar = resolveRouteRegistrar(options);
    if (registrar === undefined) return;
    if (typeof registrar !== 'function') throw new TypeError('HTTP route registrar must be a function');
    return registrar({
        app,
        express: expressFactory,
        options,
        chatSseAdapter,
        registerChatRoute: registerChat,
        wrapRoute: wrapHttpRoute,
        sendError: sendHttpError
    });
}

function registerHealthRoute({app, options}) {
    const healthPath = options.healthPath ?? '/api/health';
    const configured = options.healthResponse ?? options.health;
    const storage = options.storage ?? options.storageName ?? 'companion-v2';
    const handler = wrapHttpRoute((req, res) => {
        const payload = typeof configured === 'function'
            ? configured(req, res)
            : isRecord(configured)
                ? configured
                : {...DEFAULT_HEALTH_RESPONSE, storage};
        if (payload && typeof payload.then === 'function') return payload.then(value => res.json(value));
        return res.json(payload);
    });
    app.get(healthPath, handler);
    return handler;
}

/**
 * Construct the HTTP composition boundary without opening a listener.
 *
 * `routeRegistrar` owns application routes. `chatSseAdapter` is mounted only
 * when supplied, while `chatRoute` can replace its Express-facing wrapper for
 * callers that already own request validation and SSE framing.
 */
export function createHttpApp(options = {}) {
    if (!isRecord(options)) throw new TypeError('HTTP app options must be an object');
    const expressFactory = resolveExpressFactory(options);
    const app = resolveApp(options, expressFactory);

    app.set?.('etag', false);
    app.use(expressFactory.json({limit: options.jsonLimit ?? DEFAULT_JSON_LIMIT}));
    app.use('/api', apiCacheMiddleware(options));
    app.use(expressFactory.static(resolveStaticRoot(options), {
        setHeaders: options.staticCompression === false ? staticHeaders({...options, staticCompression: false}) : staticHeaders(options)
    }));

    registerHealthRoute({app, options});
    const configuredAdapter = options.chatSseAdapter ?? options.sseAdapter ?? options.chatAdapter;
    const chatSseAdapter = configuredAdapter === undefined ? undefined : routeHandler(configuredAdapter, 'SSE adapter');
    let chat;
    const registerChat = () => {
        if (chat) return chat;
        if (options.chatRoute === undefined && !chatSseAdapter) {
            throw new TypeError('HTTP chat route requires chatSseAdapter or chatRoute');
        }
        chat = registerChatRoute({app, options, adapter: chatSseAdapter});
        return chat;
    };

    if (options.chatRoute !== undefined || options.chatSseRoute !== undefined || chatSseAdapter) registerChat();
    registerRoutes({app, options, expressFactory, chatSseAdapter, registerChat});
    app.use(createErrorMiddleware({defaultStatus: options.errorStatus ?? 400}));

    return app;
}

export const createHttpApplication = createHttpApp;
export const route = wrapHttpRoute;
