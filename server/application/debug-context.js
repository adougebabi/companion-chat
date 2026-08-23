function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

const MAX_STATE_TEXT = 2_000;

function boundStateText(value) {
    const text = String(value ?? '').trim();
    return text.length <= MAX_STATE_TEXT ? text : `${text.slice(0, MAX_STATE_TEXT - 3)}...`;
}

export function debugText(value, fallback = '') {
    if (typeof value === 'string') return boundStateText(value);
    if (value === null || value === undefined) return fallback;
    if (typeof value === 'number' || typeof value === 'boolean') return String(value);
    if (isRecord(value)) {
        for (const key of ['label', 'name', 'title', 'description', 'rationale', 'kind', 'type']) {
            const candidate = debugText(value[key]);
            if (candidate) return candidate;
        }
        try { return boundStateText(JSON.stringify(value)); } catch { return fallback; }
    }
    return fallback;
}

export function debugStateFor(context) {
    const state = isRecord(context?.state) ? context.state : {};
    const appearanceValue = state.appearance;
    const appearance = isRecord(appearanceValue) ? appearanceValue : {};
    const source = state.source;
    return {
        situation: debugText(state.situation ?? state.resolved_situation),
        scene: debugText(state.scene ?? state.resolved_scene),
        outfit: debugText(isRecord(appearanceValue)
            ? appearance.outfit ?? appearance.clothing ?? appearance.wardrobe ?? appearance.description
                ?? appearance.dress ?? appearance.coat ?? appearance.shirt ?? appearance.top
                ?? appearance.bottom ?? appearance.shoes ?? appearance.accessories ?? appearance.衣服
            : appearanceValue),
        special: debugText(state.special ?? state.specialState ?? state.special_status ?? state.affect?.special ?? state.mood ?? (isRecord(source) ? source.rationale ?? source.label ?? source.kind : '')),
        mood: debugText(state.mood ?? state.resolved_mood)
    };
}
