export type FieldRule = { id: string; label: string; value: unknown; required?: boolean; trim?: boolean; kind?: "url" | "number" | "timezone" | "json"; min?: number; max?: number; minLength?: number; maxLength?: number; maxBytes?: number; integer?: boolean };
export function validateFields(fields: FieldRule[]): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const field of fields) {
    const value = typeof field.value === "string" && field.trim !== false ? field.value.trim() : field.value;
    if (value === "" || value === null || value === undefined) {
      if (field.required) errors[field.id] = `请填写${field.label}。`;
      continue;
    }
    if (typeof value === "string" && field.maxLength !== undefined && [...value].length > field.maxLength) {
      errors[field.id] = `请控制在 ${field.maxLength} 个字符以内。`;
      continue;
    }
    if (typeof value === "string" && field.maxBytes !== undefined && new TextEncoder().encode(value).length > field.maxBytes) {
      errors[field.id] = `${field.label}过长，请精简后再保存。`;
      continue;
    }
    if (typeof value === "string" && field.minLength !== undefined && [...value].length < field.minLength) {
      errors[field.id] = `至少需要 ${field.minLength} 个字符。`;
      continue;
    }
    if (field.kind === "number") {
      const number = Number(value);
      if (!['number','string'].includes(typeof value) || !Number.isFinite(number) || (field.integer && !Number.isInteger(number))) errors[field.id] = `请填写${field.integer ? '整数' : '有效数字'}。`;
      else if (field.min !== undefined && number < field.min) errors[field.id] = `不能小于 ${field.min}。`;
      else if (field.max !== undefined && number > field.max) errors[field.id] = `不能大于 ${field.max}。`;
    } else if (field.kind === "url") {
      try { const url = new URL(String(value)); if (!['http:', 'https:'].includes(url.protocol)) throw new Error(); }
      catch { errors[field.id] = "请填写完整的 HTTP 或 HTTPS 地址。"; }
    } else if (field.kind === "timezone") {
      try { if (value === "Local") throw new Error(); new Intl.DateTimeFormat('zh-CN', { timeZone: String(value) }).format(); }
      catch { errors[field.id] = "请填写有效时区，例如 Asia/Shanghai；也可以留空。"; }
    } else if (field.kind === "json") {
      try { const parsed: unknown = JSON.parse(String(value)); if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(); }
      catch { errors[field.id] = "请填写有效的 JSON 对象。"; }
    }
  }
  return errors;
}
