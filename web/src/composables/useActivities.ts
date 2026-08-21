import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue';
import type {Ref} from 'vue';
import {useActivitiesStore} from '../stores/activities';

type MaybeRef<T> = Ref<T> | (() => T) | T;

function resolve<T>(target: MaybeRef<T>): T {
  if (typeof target === 'function') return (target as () => T)();
  if (target && typeof target === 'object' && 'value' in target) return (target as Ref<T>).value;
  return target;
}

export function useActivities(
  personaId: MaybeRef<string | null | undefined> = null,
  options: {autoLoad?: boolean} = {}
) {
  const store = useActivitiesStore();
  const sentinel = ref<HTMLElement | null>(null);
  const currentId = () => resolve(personaId) || null;
  const state = computed(() => store.get(currentId()));
  let observer: IntersectionObserver | null = null;

  async function loadInitial(signal?: AbortSignal): Promise<void> {
    await store.loadInitial(currentId(), signal);
  }

  async function loadMore(signal?: AbortSignal): Promise<void> {
    await store.loadMore(currentId(), signal);
  }

  async function retry(signal?: AbortSignal): Promise<void> {
    const snapshot = state.value;
    if (snapshot.nextCursor && snapshot.hasMore) await loadMore(signal);
    else await loadInitial(signal);
  }

  function observe(): void {
    observer?.disconnect();
    observer = null;
    if (typeof IntersectionObserver === 'undefined' || !sentinel.value) return;
    observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) void loadMore().catch(() => {});
    }, {threshold: 0});
    observer.observe(sentinel.value);
  }

  onMounted(() => {
    observe();
    if (options.autoLoad !== false) void loadInitial().catch(() => {});
  });
  const stopWatch = watch(() => currentId(), () => {
    if (options.autoLoad !== false && typeof window !== 'undefined') void loadInitial().catch(() => {});
  });
  onBeforeUnmount(() => {
    observer?.disconnect();
    stopWatch();
  });

  return {store, state, sentinel, loadInitial, loadMore, retry, observe};
}

export default useActivities;
