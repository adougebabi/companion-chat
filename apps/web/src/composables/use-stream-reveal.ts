import { onBeforeUnmount, ref, watch, type Ref } from "vue";

import { revealStep } from "./stream-reveal";
import { useReducedMotion } from "./use-reduced-motion";

export function useStreamReveal(target: Ref<string>, animate: Ref<boolean>) {
  const shown = ref(animate.value ? "" : target.value);
  const reduced = useReducedMotion();
  let frame = 0;

  function stop() {
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
  }

  function tick() {
    frame = 0;
    const next = revealStep(target.value, shown.value, { animate: animate.value, reducedMotion: reduced.value });
    shown.value = next;
    if (next !== target.value) frame = requestAnimationFrame(tick);
  }

  function start() {
    stop();
    if (!animate.value || reduced.value || typeof requestAnimationFrame !== "function") {
      shown.value = target.value;
      return;
    }
    if (!target.value.startsWith(shown.value)) {
      shown.value = target.value;
      return;
    }
    frame = requestAnimationFrame(tick);
  }

  watch([target, animate, reduced], start, { immediate: true });
  onBeforeUnmount(stop);
  return shown;
}
