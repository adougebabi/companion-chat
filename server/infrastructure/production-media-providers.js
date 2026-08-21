import {mkdirSync, readFileSync, statSync} from 'node:fs';
import {spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import {join, resolve} from 'node:path';

import {createMtplxProvider} from './llm-provider.js';
import {createMediaProviders} from './media-providers.js';
import {createProviderRegistry} from './provider-ports.js';

const DEFAULT_H3_TIMEOUT_MS = 15 * 60_000;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function text(value) {
    return typeof value === 'string' ? value.trim() : '';
}

function settingValue(settings, key, fallback = '') {
    const value = settings?.[key];
    return value === undefined || value === null ? fallback : value;
}

function settingsReader(settings) {
    if (typeof settings === 'function') return settings;
    if (isRecord(settings) && typeof settings.read === 'function') return settings.read.bind(settings);
    if (isRecord(settings)) return () => settings;
    return () => ({});
}

function validComfyPromptId(value) {
    return typeof value === 'string'
        && value.length <= 160
        && /^[A-Za-z0-9][A-Za-z0-9._:-]*$/.test(value);
}

function comfyOutputFiles(history, promptId) {
    const outputs = history?.[promptId]?.outputs || {};
    return Object.values(outputs)
        .flatMap(output => [...(output?.images || []), ...(output?.gifs || [])])
        .filter(file => file?.filename)
        .slice(0, 3);
}

function safeH3Path(value, rootPath) {
    const candidate = resolve(String(value || ''));
    const root = text(rootPath) ? resolve(String(rootPath)) : '';
    return Boolean(root) && (candidate === root || candidate.startsWith(`${root}/`));
}

function h3OutputFile(payload, config, id) {
    const directory = text(config?.h3OutputDir);
    const allowedRoot = text(config?.h3AllowedRoot) || directory;
    if (!directory || !safeH3Path(directory, allowedRoot)) throw new Error('h3 output directory is outside the allowed root');
    const requested = text(payload?.outputPath);
    const file = requested || join(directory, `${id('h3')}.mp4`);
    if (!safeH3Path(file, allowedRoot) || !/\.mp4$/i.test(file)) throw new Error('h3 output file path is invalid');
    return file;
}

function h3Args(payload, config, outputPath) {
    const options = {
        ...(isRecord(config?.h3Defaults) ? config.h3Defaults : {}),
        ...(isRecord(payload?.h3) ? payload.h3 : {})
    };
    const args = [];
    const push = (flag, value) => {
        if (value !== undefined && value !== null && value !== '') args.push(flag, String(value));
    };
    push('-d', config?.h3ModelDir || options.profile);
    push('-p', payload?.prompt);
    for (const [key, flag] of [['width', '--width'], ['height', '--height'], ['frames', '--frames'], ['steps', '--steps'], ['layers', '--layers']]) {
        const value = Number(options[key]);
        if (Number.isFinite(value) && value > 0 && value <= 100_000) push(flag, Math.trunc(value));
    }
    if (options.reuse === true) args.push('--reuse');
    else if (Number.isFinite(Number(options.reuse)) && Number(options.reuse) >= 0) push('--reuse', Math.trunc(Number(options.reuse)));
    if (options.ssdStreaming === true || options['ssd-streaming'] === true) args.push('--ssd-streaming');
    push('-o', outputPath);
    return args;
}

function runH3(executable, args, timeoutMs, {spawnImpl = spawn, onOutput} = {}) {
    return new Promise((resolvePromise, rejectPromise) => {
        let child;
        try {
            child = spawnImpl(executable, args, {stdio: ['ignore', 'pipe', 'pipe'], shell: false});
        } catch {
            rejectPromise(new Error('h3 process failed to start'));
            return;
        }
        let settled = false;
        const flushers = [];
        const emitOutput = (stream, value) => {
            if (settled || !value) return;
            try { onOutput?.(stream, value); } catch { /* progress is best effort */ }
        };
        const capture = (stream, source) => {
            if (!stream || typeof stream.on !== 'function') return;
            let buffer = '';
            const flush = () => {
                const pending = buffer;
                buffer = '';
                emitOutput(source, pending);
            };
            flushers.push(flush);
            stream.on('data', chunk => {
                buffer += String(chunk || '');
                const pieces = buffer.split(/[\r\n]+/);
                buffer = pieces.pop() || '';
                for (const piece of pieces) emitOutput(source, piece);
            });
            stream.once?.('end', flush);
        };
        capture(child.stdout, 'stdout');
        capture(child.stderr, 'stderr');
        const finish = (settler, value) => {
            if (settled) return;
            for (const flush of flushers) flush();
            settled = true;
            clearTimeout(timer);
            settler(value);
        };
        const timer = setTimeout(() => {
            try { child.kill?.('SIGTERM'); } catch { /* process may already be gone */ }
            finish(rejectPromise, new Error('h3 process timed out'));
        }, Number(timeoutMs) > 0 ? Number(timeoutMs) : DEFAULT_H3_TIMEOUT_MS);
        child.once?.('error', () => finish(rejectPromise, new Error('h3 process failed to start')));
        child.once?.('close', (code, signal) => {
            if (code !== 0) return finish(rejectPromise, new Error(signal ? 'h3 process was terminated' : `h3 process exited with code ${code}`));
            finish(resolvePromise, {});
        });
    });
}

function defaultSettingsWithEnvironment(readSettings, environment = {}) {
    const current = readSettings();
    const envValue = (...names) => names.map(name => environment?.[name]).find(value => text(value)) || '';
    return {
        ...(isRecord(current) ? current : {}),
        lmStudioUrl: settingValue(current, 'lmStudioUrl', envValue('MTPLX_URL', 'LM_STUDIO_URL') || 'http://127.0.0.1:8000/v1'),
        lmStudioApiKey: settingValue(current, 'lmStudioApiKey', envValue('MTPLX_API_KEY', 'LM_STUDIO_API_KEY')),
        model: settingValue(current, 'model', envValue('MTPLX_MODEL', 'LM_STUDIO_MODEL')),
        comfyUrl: settingValue(current, 'comfyUrl', envValue('COMFYUI_URL') || 'http://127.0.0.1:8188'),
        imageWorkflow: settingValue(current, 'imageWorkflow', envValue('COMFYUI_IMAGE_WORKFLOW')),
        videoWorkflow: settingValue(current, 'videoWorkflow', envValue('COMFYUI_VIDEO_WORKFLOW')),
        h3Executable: settingValue(current, 'h3Executable', envValue('H3_EXECUTABLE') || 'h3.c'),
        h3ModelDir: settingValue(current, 'h3ModelDir', envValue('H3_MODEL_DIR')),
        h3OutputDir: settingValue(current, 'h3OutputDir', envValue('H3_OUTPUT_DIR')),
        h3AllowedRoot: settingValue(current, 'h3AllowedRoot', envValue('H3_ALLOWED_ROOT')),
        h3TimeoutMs: Number(settingValue(current, 'h3TimeoutMs', envValue('H3_TIMEOUT_MS') || DEFAULT_H3_TIMEOUT_MS))
    };
}

/**
 * Compose the production provider registry. Construction is side-effect free;
 * network and child-process work only happens when a registered adapter method
 * is invoked by an application flow or worker.
 */
export function createProductionProviderRegistry({settings, environment = {}, fetchImpl, spawnImpl, fs, id, providerAdapters, promptRuns} = {}) {
    if (providerAdapters !== undefined) return createProviderRegistry({providers: providerAdapters});
    const readSettings = settingsReader(settings);
    const settingsForProvider = () => defaultSettingsWithEnvironment(readSettings, environment);
    const generateId = typeof id === 'function' ? id : prefix => `${prefix}_${randomUUID()}`;
    const fileSystem = fs ?? {mkdirSync, statSync, readFileSync};
    const media = createMediaProviders({
        fetch: fetchImpl,
        fs: fileSystem,
        spawn: spawnImpl ?? spawn,
        h3Args,
        h3OutputFile: (payload, config) => h3OutputFile(payload, config, generateId),
        runH3: (executable, args, timeoutMs, options) => runH3(executable, args, timeoutMs, {...options, spawnImpl: spawnImpl ?? spawn}),
        safeH3Path,
        validComfyPromptId,
        comfyOutputFiles,
        id: generateId,
        settings: settingsForProvider
    });
    const mtplx = createMtplxProvider({settings: settingsForProvider, fetchImpl, promptRuns});
    return createProviderRegistry({providers: [mtplx, media.comfyui, media.h3]});
}

export {comfyOutputFiles, h3Args, h3OutputFile, runH3, safeH3Path, validComfyPromptId};

export default createProductionProviderRegistry;
