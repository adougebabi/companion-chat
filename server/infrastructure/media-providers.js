/**
 * Side-effect-free factories for the legacy ComfyUI and h3 media adapters.
 *
 * The factory owns provider protocol translation only. Runtime resources and
 * policy remain outside this module and are injected by the composition root:
 * fetch, filesystem/process helpers, URL/path validation, IDs, and settings.
 */

const DEFAULT_CLEAN_URL = value => {
    const raw = String(value ?? '').trim();
    if (!raw) return '';
    if (/^[a-z][a-z\d+.-]*:\/\/$/i.test(raw)) return raw;
    return raw.replace(/\/+$/, '');
};

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredFunction(value, name) {
    if (typeof value !== 'function') throw new TypeError(`Media provider requires ${name}()`);
    return value;
}

function resolveFetch(options) {
    const fetcher = options.fetch ?? options.fetchImpl ?? globalThis.fetch;
    return requiredFunction(fetcher, 'fetch');
}

function resolveFileHelper(options, name) {
    const helper = options[name] ?? options.fs?.[name];
    return requiredFunction(helper, name);
}

function resolveProviderDependencies(options = {}) {
    if (!isRecord(options)) throw new TypeError('Media provider factory options must be an object');
    return {
        fetch: resolveFetch(options),
        cleanUrl: options.cleanUrl === undefined ? DEFAULT_CLEAN_URL : requiredFunction(options.cleanUrl, 'cleanUrl'),
        mkdirSync: options.mkdirSync ?? options.fs?.mkdirSync,
        statSync: options.statSync ?? options.fs?.statSync,
        readFileSync: options.readFileSync ?? options.fs?.readFileSync,
        spawn: options.spawn,
        h3Args: options.h3Args,
        h3OutputFile: options.h3OutputFile,
        runH3: options.runH3,
        safeH3Path: options.safeH3Path,
        validComfyPromptId: options.validComfyPromptId,
        comfyOutputFiles: options.comfyOutputFiles,
        id: options.id,
        settings: options.settings
    };
}

function requireComfyDependency(dependencies, name) {
    return requiredFunction(dependencies[name], name);
}

function requireH3Dependency(dependencies, name) {
    return requiredFunction(dependencies[name], name);
}

function createComfyUiAdapter(dependencies) {
    return Object.freeze({
        id: 'comfyui',
        label: 'ComfyUI',
        portType: 'media',
        capabilities: ['image', 'video'],
        async submit({kind, prompt, settings: config}) {
            const workflowSource = kind === 'video' ? config.videoWorkflow : config.imageWorkflow;
            if (!workflowSource) throw new Error(`尚未配置${kind === 'video' ? '视频' : '图片'}工作流`);
            const workflow = JSON.parse(workflowSource);
            let found = false;
            for (const node of Object.values(workflow)) {
                if (!node?.inputs || typeof node.inputs !== 'object') continue;
                for (const [key, value] of Object.entries(node.inputs)) {
                    if (typeof value !== 'string' || !value.includes('{{prompt}}')) continue;
                    node.inputs[key] = value.replaceAll('{{prompt}}', prompt);
                    found = true;
                }
            }
            if (!found) throw new Error('工作流未包含 {{prompt}} 占位符');
            const fetcher = dependencies.fetch;
            const url = dependencies.cleanUrl(config.comfyUrl);
            const response = await fetcher(`${url}/prompt`, {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({prompt: workflow})
            });
            if (!response.ok) throw new Error(`ComfyUI HTTP ${response.status}`);
            const body = await response.json();
            const validPromptId = requireComfyDependency(dependencies, 'validComfyPromptId');
            if (!validPromptId(body?.prompt_id)) throw new Error('ComfyUI 未返回有效 prompt ID');
            return {externalId: body.prompt_id, pending: true};
        },
        async poll({externalId, settings: config}) {
            const validPromptId = requireComfyDependency(dependencies, 'validComfyPromptId');
            if (!validPromptId(externalId)) return {status: 'failed', error: '缺少有效的 ComfyUI prompt ID'};
            const response = await dependencies.fetch(`${dependencies.cleanUrl(config.comfyUrl)}/history/${encodeURIComponent(externalId)}`);
            if (!response.ok) throw new Error(`ComfyUI HTTP ${response.status}`);
            const history = await response.json();
            const filesForPrompt = requireComfyDependency(dependencies, 'comfyOutputFiles');
            const files = filesForPrompt(history, externalId);
            return files.length ? {status: 'complete', files} : {status: 'pending'};
        },
        async readAsset({asset, res, settings: config}) {
            const params = new URLSearchParams({filename: asset.filename, subfolder: asset.subfolder || '', type: asset.file_type || 'output'});
            const response = await dependencies.fetch(`${dependencies.cleanUrl(config.comfyUrl)}/view?${params}`);
            if (!response.ok) throw new Error(`ComfyUI HTTP ${response.status}`);
            res.set('Content-Type', response.headers.get('content-type') || 'application/octet-stream');
            res.send(Buffer.from(await response.arrayBuffer()));
        },
        async readCandidate({file, settings: config}) {
            const params = new URLSearchParams({filename: file.filename, subfolder: file.subfolder || '', type: file.type || 'output'});
            const response = await dependencies.fetch(`${dependencies.cleanUrl(config.comfyUrl)}/view?${params}`);
            if (!response.ok) throw new Error(`ComfyUI HTTP ${response.status}`);
            return {bytes: Buffer.from(await response.arrayBuffer()), mimeType: response.headers.get('content-type') || 'application/octet-stream'};
        }
    });
}

function createH3Adapter(dependencies) {
    return Object.freeze({
        id: 'h3',
        label: 'h3.c',
        portType: 'media',
        capabilities: ['video'],
        async submit({prompt, payload, settings: config, progress}) {
            const outputFile = requireH3Dependency(dependencies, 'h3OutputFile');
            const makeArgs = requireH3Dependency(dependencies, 'h3Args');
            const runH3 = requireH3Dependency(dependencies, 'runH3');
            const spawn = dependencies.spawn === undefined ? undefined : requireH3Dependency(dependencies, 'spawn');
            const outputPath = outputFile(payload, config);
            resolveFileHelper(dependencies, 'mkdirSync')(requirePathDirname(outputPath), {recursive: true});
            const args = makeArgs({...payload, prompt}, {...config, h3Defaults: config.h3Defaults}, outputPath);
            const preparing = progress?.stage('preparing');
            if (preparing && !preparing.changed) throw new Error('h3 作业租约已失效');
            const generating = progress?.stage('generating');
            if (generating && !generating.changed) throw new Error('h3 作业租约已失效');
            await runH3(config.h3Executable, args, Number(config.h3TimeoutMs) || 15 * 60_000, {
                spawn,
                onOutput: (stream, text) => progress?.output(stream, text)
            });
            progress?.flush();
            const validating = progress?.stage('validating_output');
            if (validating && !validating.changed) throw new Error('h3 作业租约已失效');
            let stat;
            try {
                stat = resolveFileHelper(dependencies, 'statSync')(outputPath);
            } catch {
                throw new Error('h3 未生成输出文件');
            }
            if (!stat.isFile() || stat.size <= 0) throw new Error('h3 输出文件为空');
            const makeId = requireH3Dependency(dependencies, 'id');
            return {externalId: makeId('h3_result'), pending: false, files: [{filename: outputPath, type: 'h3', format: 'video', path: outputPath}]};
        },
        async poll({externalId}) {
            if (typeof externalId !== 'string' || !/\.mp4$/i.test(externalId)) return {status: 'failed', error: 'h3 外部任务标识无效'};
            try {
                const stat = resolveFileHelper(dependencies, 'statSync')(externalId);
                return stat.isFile() && stat.size > 0
                    ? {status: 'complete', files: [{filename: externalId, type: 'h3', format: 'video', path: externalId}]}
                    : {status: 'pending'};
            } catch {
                return {status: 'pending'};
            }
        },
        async readAsset({asset, res}) {
            const readSettings = requireH3Dependency(dependencies, 'settings');
            const safePath = requireH3Dependency(dependencies, 'safeH3Path');
            const config = readSettings();
            const path = asset.locator?.path || asset.filename;
            if (!path || !safePath(path, config.h3AllowedRoot || config.h3OutputDir)) throw new Error('h3 资产路径无效');
            res.sendFile(path);
        },
        async readCandidate({file, settings: config}) {
            const safePath = requireH3Dependency(dependencies, 'safeH3Path');
            const path = file?.path || file?.filename;
            if (!path || !safePath(path, config.h3AllowedRoot || config.h3OutputDir)) throw new Error('h3 候选资产路径无效');
            const stat = resolveFileHelper(dependencies, 'statSync')(path);
            if (!stat.isFile() || stat.size <= 0 || stat.size > 96 * 1024 * 1024) throw new Error('h3 候选资产大小无效');
            return {bytes: resolveFileHelper(dependencies, 'readFileSync')(path), mimeType: 'video/mp4', path};
        }
    });
}

function requirePathDirname(path) {
    const separator = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
    return separator < 0 ? '.' : path.slice(0, separator) || path[0];
}

/**
 * Create both built-in media adapters without contacting a provider or
 * touching the filesystem. The returned object is intentionally explicit so
 * composition code can register one or both adapters independently.
 */
export function createMediaProviders(options = {}) {
    const dependencies = resolveProviderDependencies(options);
    const comfyui = createComfyUiAdapter(dependencies);
    const h3 = createH3Adapter(dependencies);
    const adapters = {comfyui, h3};
    Object.defineProperty(adapters, Symbol.iterator, {
        enumerable: false,
        value: function* iterateAdapters() {
            yield this.comfyui;
            yield this.h3;
        }
    });
    return Object.freeze(adapters);
}

export function createComfyUiProvider(options = {}) {
    return createComfyUiAdapter(resolveProviderDependencies(options));
}

export function createH3Provider(options = {}) {
    return createH3Adapter(resolveProviderDependencies(options));
}

export const createComfyUIProvider = createComfyUiProvider;
export const createMediaProviderAdapters = createMediaProviders;

export default createMediaProviders;
