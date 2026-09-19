export function runtimeConfigSource(apiOrigin = "") {
  if (!apiOrigin.trim()) {
    return 'window.__FLUCTLIGHT_RUNTIME_CONFIG__ = Object.freeze({ apiOrigin: "" });\n';
  }
  const parsed = new URL(apiOrigin);
  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw new Error('FLUCTLIGHT_API_ORIGIN must use http or https');
  }
  return `window.__FLUCTLIGHT_RUNTIME_CONFIG__ = Object.freeze({ apiOrigin: ${JSON.stringify(parsed.toString())} });\n`;
}
