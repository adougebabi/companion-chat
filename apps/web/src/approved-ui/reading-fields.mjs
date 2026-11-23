import { isInternalId } from '../v3/internal-id.mjs';

// Open objects are allowed only inside a selected business field.
export const narrativeFields = Symbol('narrativeFields');

export const readingFields = keys => Object.fromEntries(keys.split(' ').map(key => [key, narrativeFields]));

const technicalKey = /^(?:id|ids|version|revision|schema|protocol|provenance|source|extensions|metadata|metrics|telemetry|hash|digest|implementation|registry|update_policy|source_window|seed)$|(?:^|_)(?:id|ids|ref|refs|revision|version|idempotency|correlation|protocol|schema|digest|hash)(?:_|$)|^(?:implementation|registry|provider|runtime|source|internal|technical|execution|scheduling)_(?:key|name|kind|metrics|metadata|trace|token|debug)$|^(?:run_latency|latency_ms|api_key)$/i;
const technicalValue = /^(?:v\d+(?:\.\d+)*|[a-z][\w.-]*[.-]v\d+(?:\.\d+)*|(?:actor|profile|req|task|audit|npc|goal|memory|event|ctx)_[\w-]+)$/i;
const normalizedKey = key => key.replace(/([a-z\d])([A-Z])/g, '$1_$2').replace(/[ -]+/g, '_').toLowerCase();

export function readingText(value) {
  const text = typeof value === 'string' ? value.trim() : '';
  return text && !isInternalId(text) && !technicalValue.test(text) ? text : '';
}

export function selectReadingFields(value, shape = narrativeFields, { showClears = false, text = readingText } = {}) {
  if (Array.isArray(value)) {
    const items = value.map(item => selectReadingFields(item, shape, { text })).filter(item => item !== undefined);
    return items.length ? items : undefined;
  }
  if (value === '' && showClears) return '清空该设定';
  if (typeof value === 'string') return text(value) || undefined;
  if (typeof value === 'boolean' || Number.isFinite(value)) return value;
  if (!value || typeof value !== 'object') return undefined;
  if (shape === true) return selectReadingFields(value.value, true, { text });

  const entries = shape === narrativeFields
    ? Object.keys(value).filter(key => !technicalKey.test(normalizedKey(key))).map(key => [key, narrativeFields])
    : Object.entries(shape);
  const selected = entries.flatMap(([key, child]) => {
    const item = selectReadingFields(value[key], child, { showClears, text });
    return item === undefined ? [] : [[key, item]];
  });
  return selected.length ? Object.fromEntries(selected) : undefined;
}
