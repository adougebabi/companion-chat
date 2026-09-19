declare global {
  interface Window {
    __FLUCTLIGHT_RUNTIME_CONFIG__?: {
      apiOrigin?: string;
    };
  }
}

function runtimeApiOrigin(): string {
  const origin = window.__FLUCTLIGHT_RUNTIME_CONFIG__?.apiOrigin?.trim() ?? "";
  return origin || window.location.origin;
}

export const apiOrigin = runtimeApiOrigin();
