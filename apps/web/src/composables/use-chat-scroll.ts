import { nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";

import { isNearBottom, scrollTopAfterPrepend } from "./chat-scroll";

export function useChatScroll(container: Ref<HTMLElement | null>, signature: Ref<string>) {
  const following = ref(true);
  let anchorHeight = 0;
  let anchorTop = 0;
  let anchored = false;
  let frame = 0;

  function onScroll() {
    const element = container.value;
    if (!element) return;
    following.value = isNearBottom(element);
  }

  function captureAnchor() {
    const element = container.value;
    if (!element) return;
    anchorHeight = element.scrollHeight;
    anchorTop = element.scrollTop;
    anchored = true;
  }

  function scrollToLatest(behavior: ScrollBehavior = "auto") {
    const element = container.value;
    if (!element) return;
    const motion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? "auto" : behavior;
    element.scrollTo({ top: element.scrollHeight, behavior: motion });
    following.value = true;
  }

  function restoreOrFollow() {
    const element = container.value;
    if (!element) return;
    if (anchored) {
      element.scrollTop = scrollTopAfterPrepend(anchorHeight, anchorTop, element.scrollHeight);
      anchored = false;
      following.value = isNearBottom(element);
      return;
    }
    if (following.value) element.scrollTop = element.scrollHeight;
  }

  watch(signature, () => {
    void nextTick().then(() => {
      if (frame) cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        frame = 0;
        restoreOrFollow();
      });
    });
  });

  onMounted(() => {
    container.value?.addEventListener("scroll", onScroll, { passive: true });
    scrollToLatest();
  });
  onBeforeUnmount(() => {
    container.value?.removeEventListener("scroll", onScroll);
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
  });

  return { following, captureAnchor, scrollToLatest };
}
