const PROVIDER_KEYS = ['imageProvider', 'videoProvider'];

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function number(value) {
    const result = Number(value);
    return Number.isFinite(result) ? result : null;
}

export function createSettingsPolicy({providers, h3Inspector} = {}) {
    return Object.freeze({
        validate(candidate = {}) {
            if (!isRecord(candidate)) throw Object.assign(new TypeError('Settings must be an object'), {status: 400});
            const next = {...candidate};
            if (next.h3Profile !== undefined && next.h3ModelDir === undefined) next.h3ModelDir = next.h3Profile;
            delete next.h3Profile;
            if (next.simplifiedMediaMode !== undefined) {
                next.simplifiedMediaMode = next.simplifiedMediaMode === true
                    || ['true', '1', 'on'].includes(String(next.simplifiedMediaMode).toLowerCase());
            }
            for (const key of PROVIDER_KEYS) {
                if (next[key] === undefined) continue;
                const provider = providers?.find?.(next[key], {portType: 'media', capability: key === 'videoProvider' ? 'video' : 'image'});
                if (!provider) throw Object.assign(new Error(`${key} provider is unavailable`), {status: 400});
            }
            if (next.h3TimeoutMs !== undefined) {
                const timeout = number(next.h3TimeoutMs);
                if (timeout === null || timeout < 1_000 || timeout > 86_400_000) throw Object.assign(new Error('h3TimeoutMs 无效'), {status: 400});
                next.h3TimeoutMs = timeout;
            }
            const h3Changed = ['h3Executable', 'h3ModelDir', 'h3OutputDir', 'h3AllowedRoot'].some(key => Object.hasOwn(candidate, key));
            if (h3Changed && typeof h3Inspector === 'function') {
                const result = h3Inspector(next);
                if (!result?.ok) throw Object.assign(new Error('h3 配置不可用'), {status: 400});
            }
            return next;
        }
    });
}

export default createSettingsPolicy;
