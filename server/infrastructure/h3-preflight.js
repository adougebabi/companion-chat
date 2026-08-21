import {accessSync, constants, mkdirSync, statSync, writeFileSync, unlinkSync} from 'node:fs';
import {dirname, isAbsolute, relative, resolve} from 'node:path';
import {spawn} from 'node:child_process';

const MAX_OUTPUT = 4;
const MAX_OUTPUT_LENGTH = 480;
const SAFE_VALUE = /Bearer\s+[^\s,;]+|(?:api[_-]?key|token|secret|password)=[^&\s]+/gi;

function bounded(value) {
    const text = String(value ?? '').replace(/\p{Cc}/gu, ' ').replace(SAFE_VALUE, '[redacted]').replace(/\s+/g, ' ').trim();
    return text.length <= MAX_OUTPUT_LENGTH ? text : `${text.slice(0, MAX_OUTPUT_LENGTH - 3)}...`;
}

function checkPath(path, kind) {
    if (typeof path !== 'string' || !path.trim()) return {configured: false, valid: false, error: `${kind}未配置`};
    const value = path.trim();
    if (!isAbsolute(value)) return {configured: true, valid: false, displayName: `…/${value.split(/[\\/]/).at(-1)}`, error: `${kind}必须是绝对路径`};
    try {
        const stat = statSync(value);
        if (kind === 'h3 可执行文件' && (!stat.isFile())) return {configured: true, valid: false, displayName: `…/${value.split(/[\\/]/).at(-1)}`, error: `${kind}不是文件`};
        if (kind !== 'h3 可执行文件' && !stat.isDirectory()) return {configured: true, valid: false, displayName: `…/${value.split(/[\\/]/).at(-1)}`, error: `${kind}不是目录`};
        if (kind === 'h3 可执行文件') accessSync(value, constants.X_OK);
        return {configured: true, valid: true, displayName: `…/${value.split(/[\\/]/).at(-1)}`};
    } catch {
        return {configured: true, valid: false, displayName: `…/${value.split(/[\\/]/).at(-1)}`, error: `${kind}不可用`};
    }
}

function insideRoot(path, root) {
    const target = resolve(path);
    const allowed = resolve(root || path);
    const rel = relative(allowed, target);
    return rel === '' || (!rel.startsWith('..') && !isAbsolute(rel));
}

export function inspectH3Configuration(config = {}) {
    const executable = checkPath(config.h3Executable, 'h3 可执行文件');
    const modelDir = checkPath(config.h3ModelDir, 'h3 模型目录');
    const outputDir = checkPath(config.h3OutputDir, 'h3 输出目录');
    const root = config.h3AllowedRoot || config.h3OutputDir;
    if (outputDir.configured && (!root || !insideRoot(config.h3OutputDir, root))) {
        outputDir.valid = false;
        outputDir.error = 'h3 输出目录不在允许范围内';
    }
    return {ok: executable.valid && modelDir.valid && outputDir.valid, checks: {executable, modelDir, outputDir}};
}

function runCommand(executable, args = ['--help'], timeoutMs = 8_000, {onOutput} = {}) {
    return new Promise((resolvePromise, rejectPromise) => {
        let child;
        try { child = spawn(executable, args, {stdio: ['ignore', 'pipe', 'pipe'], shell: false}); }
        catch { rejectPromise(new Error('h3 启动失败')); return; }
        let settled = false;
        const output = [];
        const push = (stream, chunk) => {
            const text = bounded(chunk);
            if (!text) return;
            output.push({stream, text});
            while (output.length > MAX_OUTPUT) output.shift();
            try { onOutput?.(stream, text); } catch {}
        };
        child.stdout?.on('data', chunk => push('stdout', chunk));
        child.stderr?.on('data', chunk => push('stderr', chunk));
        const finish = (fn, value) => {
            if (settled) return;
            settled = true;
            clearTimeout(timer);
            fn(value);
        };
        const timer = setTimeout(() => { child.kill?.('SIGTERM'); finish(rejectPromise, new Error('h3 进程超时')); }, timeoutMs);
        child.once('error', () => finish(rejectPromise, new Error('h3 启动失败')));
        child.once('close', code => code === 0 ? finish(resolvePromise, output) : finish(rejectPromise, new Error('h3 进程无法在当前运行环境启动')));
    });
}

export function createH3Preflight({run = runCommand} = {}) {
    if (typeof run !== 'function') throw new TypeError('h3 preflight run must be a function');
    return async function h3Preflight(config = {}) {
        const filesystem = inspectH3Configuration(config);
        if (!filesystem.ok) return {ok: false, stage: 'filesystem', checks: filesystem.checks};
        const output = [];
        try {
            await run(config.h3Executable, ['--help'], Number(config.h3PreflightTimeoutMs) || 8_000, {
                onOutput(stream, text) {
                    output.push({stream, text: bounded(text)});
                    while (output.length > MAX_OUTPUT) output.shift();
                }
            });
            return {ok: true, stage: 'process', checks: filesystem.checks, process: {started: true, output}};
        } catch {
            return {ok: false, stage: 'process', checks: filesystem.checks, process: {
                started: false,
                error: 'h3 进程无法在当前运行环境启动；请确认二进制与服务运行环境兼容。',
                output
            }};
        }
    };
}

export function safeH3Path(path, allowedRoot) {
    if (typeof path !== 'string' || !path.trim() || !allowedRoot || !isAbsolute(path) || !isAbsolute(allowedRoot)) return false;
    return insideRoot(path, allowedRoot);
}

export function h3RuntimeHelpers({id = prefix => `${prefix}_${Date.now()}`} = {}) {
    return {
        inspectH3Configuration,
        h3Preflight: createH3Preflight(),
        safeH3Path,
        runH3: runCommand,
        h3OutputFile(payload, config) {
            const directory = config.h3OutputDir;
            const output = payload?.outputPath || `${directory}/${id('h3')}.mp4`;
            if (!safeH3Path(output, config.h3AllowedRoot || directory) || !/\.mp4$/i.test(output)) throw new Error('h3 输出文件路径无效');
            return output;
        },
        h3Args(payload, config, outputPath) {
            const defaults = {...(config.h3Defaults || {}), ...(payload?.h3 || {})};
            const args = [];
            const push = (flag, value) => { if (value !== undefined && value !== null && value !== '') args.push(flag, String(value)); };
            push('-d', config.h3ModelDir || defaults.profile);
            push('-p', payload?.prompt);
            for (const [key, flag] of [['width', '--width'], ['height', '--height'], ['frames', '--frames'], ['steps', '--steps'], ['layers', '--layers']]) {
                const value = Number(defaults[key]);
                if (Number.isFinite(value) && value > 0 && value <= 100000) push(flag, Math.trunc(value));
            }
            if (defaults.reuse === true) args.push('--reuse');
            else if (Number.isFinite(Number(defaults.reuse)) && Number(defaults.reuse) >= 0) push('--reuse', Math.trunc(Number(defaults.reuse)));
            if (defaults.ssdStreaming === true || defaults['ssd-streaming'] === true) args.push('--ssd-streaming');
            push('-o', outputPath);
            return args;
        }
    };
}

export default createH3Preflight;
