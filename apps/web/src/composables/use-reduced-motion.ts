import { onBeforeUnmount, onMounted, ref } from "vue";

export function useReducedMotion() {
  const reduced = ref(false);
  let media: MediaQueryList | null = null;
  function sync() { reduced.value = Boolean(media?.matches); }
  onMounted(() => {
    media = window.matchMedia("(prefers-reduced-motion: reduce)");
    sync();
    media.addEventListener("change", sync);
  });
  onBeforeUnmount(() => media?.removeEventListener("change", sync));
  return reduced;
}
