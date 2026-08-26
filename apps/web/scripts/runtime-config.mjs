export function runtimeConfigSource(bffOrigin) {
  const parsed = new URL(bffOrigin);
  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw new Error('FLUCTLIGHT_BFF_ORIGIN must use http or https');
  }
  return `window.__FLUCTLIGHT_RUNTIME_CONFIG__ = Object.freeze({ bffOrigin: ${JSON.stringify(parsed.toString())} });\n`;
}
