import {onBeforeUnmount, ref} from 'vue';
import {useAppStore} from '../stores/app';

export interface PollingGuardState {
  isSending?: boolean;
  isComposing?: boolean;
  draft?: string;
}

export interface BootstrapOptions {
  intervalMs?: number;
  guard?: () => PollingGuardState;
}

function pageIsHidden(): boolean {
  return typeof document !== 'undefined' && document.hidden;
}

function pollingAllowed(guard?: () => PollingGuardState): boolean {
  if (pageIsHidden()) return false;
  const state = guard?.() ?? {};
  return state.isSending !== true && state.isComposing !== true && !state.draft?.length;
}

/** Contacts are the only boot request. Conversation data is loaded by selection. */
export function useBootstrap(options: BootstrapOptions = {}) {
  const app = useAppStore();
  const polling = ref(false);
  let timer: ReturnType<typeof setInterval> | null = null;
  let visibilityListener: (() => void) | null = null;

  async function start(signal?: AbortSignal) {
    app.setView('contacts');
    return app.bootstrap({signal});
  }

  async function refreshQuietly(signal?: AbortSignal) {
    if (!pollingAllowed(options.guard)) return null;
    return app.bootstrap({signal, force: true});
  }

  function stopPolling(): void {
    if (timer !== null) clearInterval(timer);
    timer = null;
    polling.value = false;
    if (visibilityListener && typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', visibilityListener);
      visibilityListener = null;
    }
  }

  function startPolling(): void {
    stopPolling();
    const intervalMs = Math.max(5_000, options.intervalMs ?? 30_000);
    polling.value = true;
    timer = setInterval(() => {
      void refreshQuietly().catch(() => {
        // The next tick is the retry. Polling must not surface an intrusive
        // error or replace a composer while the user is interacting.
      });
    }, intervalMs);
    if (typeof document !== 'undefined') {
      visibilityListener = () => {
        if (!document.hidden) void refreshQuietly().catch(() => {});
      };
      document.addEventListener('visibilitychange', visibilityListener);
    }
  }

  onBeforeUnmount(stopPolling);

  return {app, polling, start, refreshQuietly, startPolling, stopPolling, pollingAllowed: () => pollingAllowed(options.guard)};
}

export default useBootstrap;

