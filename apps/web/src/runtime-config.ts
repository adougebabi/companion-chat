declare global {
  interface Window {
    __FLUCTLIGHT_RUNTIME_CONFIG__?: {
      bffOrigin?: string;
    };
  }
}

function runtimeBffOrigin(): string {
  const origin = window.__FLUCTLIGHT_RUNTIME_CONFIG__?.bffOrigin?.trim() ?? "";
  return origin || window.location.origin;
}

export const bffOrigin = runtimeBffOrigin();
