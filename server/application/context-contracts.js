/** Short model-facing guidance; schemas and application flows own details. */
export const systemCapabilityReplyForm = '用户可见回复每条是一句完整的话并以标点结束；需要多句时拆成多条消息，不要输出工具参数或内部状态。';

export const systemCapabilityMediaContract = 'media_event：仅在用户明确要求媒体，或你自然决定交付媒体时调用；必须提供完整媒体概念，不要让服务器猜画面。';

export const systemCapabilityPendingEventContract = 'pending_event：仅登记明确且未来需要跟进的事项；普通闲聊和已解决内容不要登记。';

export const systemCapabilityMemoryContract = 'memory_event：仅记录用户当前明确表达、稳定且未来有用的事实；不要从猜测或系统状态创建记忆。';

export const systemCapabilityStateContract = 'affect_event/drive_signal：仅报告已确认的情绪或需求压力变化；数值幅度由服务器决定，工具信息不写入用户回复。';

export const systemCapabilityTimeFact = '时间：只能引用应用提供的可信时间事实；只有 timeFact=known 才能说具体结束时间，不得猜测课程或时长。';

export const systemCapabilitySceneContract = 'scene_event：仅在共同地点或活动真正开始、切换或结束时调用；普通动作不改变生活事实。';

export const systemCapabilityAppearanceContract = 'appearance_event：仅在服装或外观实际变化时调用；普通动作描述不改变当前服装状态。';

export const imageGenerationPolicyLabels = Object.freeze({
    ask: '始终询问',
    always: '始终生成',
    important: '重要时刻自动生成',
    user_only: '只有我要求才生成',
    autonomous: '人格自行决定'
});

export const systemCapabilityContracts = Object.freeze([
    systemCapabilityMediaContract,
    systemCapabilityPendingEventContract,
    systemCapabilityMemoryContract,
    systemCapabilityStateContract,
    systemCapabilityTimeFact,
    systemCapabilitySceneContract,
    systemCapabilityAppearanceContract,
    systemCapabilityReplyForm
]);

/**
 * Return the final system-capability layer carried by a context read. The
 * context reader is the owner of capability policy; consumers must not append
 * `layers.systemCapability` after already using `context.prompt`, because the
 * latter includes the final layer in the production runtime.
 */
export function systemCapabilityPromptFor(context = {}) {
    const supplied = context?.layers?.systemCapability ?? context?.systemCapability;
    return typeof supplied === 'string' && supplied.trim()
        ? supplied.trim()
        : systemCapabilityContracts.join('\n');
}

/**
 * Resolve one complete model system prompt from a context DTO. This keeps
 * workers and chat on the same context-reader contract and guarantees that a
 * context implementation which only returns structured layers still receives
 * the capability contract at the final prompt boundary.
 */
export function contextPromptFor(context = {}) {
    const layers = context?.layers ?? {};
    const explicitPrompt = typeof context?.prompt === 'string' && context.prompt.trim();
    const base = explicitPrompt
        ? context.prompt.trim()
        : [
            layers.immutableIdentity ?? layers.identity,
            layers.lifeState,
            layers.memory,
            layers.relationship,
            layers.timeFacts
        ].filter(value => typeof value === 'string' && value.trim()).join('\n\n');
    const capability = systemCapabilityPromptFor(context);
    // A context reader may already serialize the capability layer into prompt;
    // custom readers used by composition tests often return it separately.
    // Append only when the explicit prompt does not already carry it.
    if (explicitPrompt) {
        if (context?.capabilityPromptIncluded === true || !capability || base.includes(capability)) return base;
        return `${base}\n\n${capability}`;
    }
    return base ? `${base}\n\n${capability}` : capability;
}

export default systemCapabilityContracts;
