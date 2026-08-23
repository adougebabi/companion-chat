/** Short model-facing guidance; schemas and application flows own details. */
export const systemCapabilityReplyForm = '用户可见回复每条是一句完整的话并以标点结束；需要多句时拆成多条消息，不要输出工具参数或内部状态。';

export const systemCapabilityMediaContract = 'media_event：仅在用户明确要求媒体，或你自然决定交付媒体时调用；必须提供完整媒体概念，不要让服务器猜画面。';

export const systemCapabilityPendingEventContract = 'pending_event：仅登记明确且未来需要跟进的事项；普通闲聊和已解决内容不要登记。';

export const systemCapabilityMemoryContract = 'memory_event：仅记录用户当前明确表达、稳定且未来有用的事实；不要从猜测或系统状态创建记忆。';

export const systemCapabilityMemoryConsolidationContract = 'memory consolidation（结构化候选通道）：只为有证据、可回溯且可能长期有用的经历提出候选；保留不确定性和来源引用，不要把单次猜测直接升级为长期记忆，也不要把候选 JSON 写进用户可见回复。';

export const systemCapabilityStateContract = 'affect_event/drive_signal：仅报告已确认的情绪或需求压力变化；数值幅度由服务器决定，工具信息不写入用户回复。';

export const systemCapabilityAppraisalContract = 'appraisal（结构化控制通道）：基于本回合已确认的 interaction facts 提出主观评价、证据引用、不确定性，以及需要交给 affect/drive reducer 的候选；不要从关键词猜测，不要提交 PAD 数值或任意 delta，不要把 appraisal JSON 写进用户可见回复。';

export const systemCapabilitySelfModelContract = 'self-model（结构化控制通道）：仅基于可回溯证据提出版本化自我模型 claim，必须同时提供 category、claim、summary、confidence、evidenceRefs、source 和 uncertainty；不要从关键词猜测，不要把 claim JSON 写进用户可见回复。';

export const systemCapabilityAgencyContract = 'agency_intention（结构化候选通道）：由你根据上下文提出想询问、继续话题、暂缓、拒绝或主动联系等意愿，并给出自然解释、证据和不确定性；不要让意愿单独触发投递，资格、冻结、租约和安全边界由应用治理。';

export const systemCapabilityTimeFact = '时间：只能引用应用提供的可信时间事实；只有 timeFact=known 才能说具体结束时间，不得猜测课程或时长。';

export const systemCapabilitySceneContract = 'scene_event：仅在共同地点或活动真正开始、切换或结束时调用；普通动作不改变生活事实。';

export const systemCapabilityAppearanceContract = 'appearance_event：服装实际变化时调用；场景开始或切换且服装随之改变时，可与 scene_event 在同一回合分别调用；当前场景内聊天明确换装时也可单独调用。普通动作描写不改变服装状态。';

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
    systemCapabilityMemoryConsolidationContract,
    systemCapabilityStateContract,
    systemCapabilityAppraisalContract,
    systemCapabilitySelfModelContract,
    systemCapabilityAgencyContract,
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
            layers.selfModel,
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
