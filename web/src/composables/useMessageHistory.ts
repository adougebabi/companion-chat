import {nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue';
import type {Ref} from 'vue';
import {useConversationsStore} from '../stores/conversations';

type MaybeRef<T> = Ref<T> | (() => T) | T;

function resolve<T>(target: MaybeRef<T>): T {
  if (typeof target === 'function') return (target as () => T)();
  if (target && typeof target === 'object' && 'value' in target) return (target as Ref<T>).value;
  return target;
}

interface ScrollAnchor {
  id: string;
  offset: number;
}

function messageElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>('[data-message-id]'));
}

function captureAnchor(container: HTMLElement | null): ScrollAnchor | null {
  if (!container) return null;
  const bounds = container.getBoundingClientRect();
  const candidate = messageElements(container).find(element => element.getBoundingClientRect().bottom > bounds.top);
  if (!candidate?.dataset.messageId) return null;
  return {id: candidate.dataset.messageId, offset: candidate.getBoundingClientRect().top - bounds.top};
}

function restoreAnchor(container: HTMLElement | null, anchor: ScrollAnchor | null): void {
  if (!container || !anchor) return;
  const target = messageElements(container).find(element => element.dataset.messageId === anchor.id);
  if (!target) return;
  const bounds = container.getBoundingClientRect();
  const currentOffset = target.getBoundingClientRect().top - bounds.top;
  container.scrollTop += currentOffset - anchor.offset;
}

export function useMessageHistory(
  personaId: MaybeRef<string | null | undefined>,
  containerRef: Ref<HTMLElement | null>,
  options: {threshold?: number; autoLoad?: boolean} = {}
) {
  const conversations = useConversationsStore();
  const topSentinel = ref<HTMLElement | null>(null);
  const loading = ref(false);
  let observer: IntersectionObserver | null = null;
  let stopPersonaWatch: (() => void) | null = null;

  function currentPersonaId(): string | null {
    return resolve(personaId) || null;
  }

  async function loadInitial(signal?: AbortSignal): Promise<void> {
    const id = currentPersonaId();
    if (!id) return;
    loading.value = true;
    try {
      await conversations.loadInitial(id, {signal});
    } finally {
      loading.value = false;
    }
  }

  async function loadOlder(signal?: AbortSignal): Promise<void> {
    const id = currentPersonaId();
    if (!id) return;
    const state = conversations.ensure(id);
    if (state.loadingOlder || !state.hasMore || !state.nextCursor) return;
    const anchor = captureAnchor(containerRef.value);
    loading.value = true;
    try {
      await conversations.loadOlder(id, {signal});
      await nextTick();
      restoreAnchor(containerRef.value, anchor);
    } finally {
      loading.value = false;
    }
  }

  async function retry(signal?: AbortSignal): Promise<void> {
    const id = currentPersonaId();
    if (!id) return;
    const state = conversations.ensure(id);
    if (state.nextCursor && state.hasMore) await loadOlder(signal);
    else await loadInitial(signal);
  }

  function onScroll(): void {
    const container = containerRef.value;
    if (!container || container.scrollTop > (options.threshold ?? 64)) return;
    void loadOlder().catch(() => {});
  }

  function observeSentinel(): void {
    observer?.disconnect();
    observer = null;
    if (!options.autoLoad || typeof IntersectionObserver === 'undefined' || !topSentinel.value) return;
    observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) void loadOlder().catch(() => {});
    }, {root: containerRef.value, threshold: 0});
    observer.observe(topSentinel.value);
  }

  onMounted(() => {
    observeSentinel();
    if (options.autoLoad !== false) void loadInitial().catch(() => {});
  });

  stopPersonaWatch = watch(() => resolve(personaId), () => {
    if (typeof window !== 'undefined') void loadInitial().catch(() => {});
  });

  onBeforeUnmount(() => {
    observer?.disconnect();
    observer = null;
    stopPersonaWatch?.();
  });

  return {
    conversations,
    topSentinel,
    loading,
    loadInitial,
    loadOlder,
    retry,
    onScroll,
    observeSentinel,
    captureAnchor: () => captureAnchor(containerRef.value),
    restoreAnchor: (anchor: ScrollAnchor | null) => restoreAnchor(containerRef.value, anchor)
  };
}

export default useMessageHistory;

