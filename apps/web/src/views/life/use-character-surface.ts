import { computed, inject, watch, type InjectionKey } from "vue";

import type { BrowserDiagnosticMediaPrompt, BrowserWardrobeItem } from "@fluctlight/browser-client";
import { useControlCenterStore } from "../../stores/control-center";
import { useConversationStore } from "../../stores/conversations";
import type { LifeRecord } from "../../lib/life-surface";

export type LifePreview = {
  detail: LifeRecord | null;
  wardrobe: BrowserWardrobeItem[];
  prompts: BrowserDiagnosticMediaPrompt[];
};

export const lifePreviewKey: InjectionKey<LifePreview> = Symbol("life-preview");

export function useCharacterSurface() {
  const store = useConversationStore();
  const control = useControlCenterStore();
  const preview = inject(lifePreviewKey, null);
  watch(() => store.fluctlightId, (id) => {
    if (preview || !id) return;
    void control.loadFluctlightDetail(id);
  }, { immediate: true });
  const detail = computed(() => preview?.detail ?? (control.fluctlightDetail as LifeRecord | null));
  return { store, control, preview, detail, readOnlyPreview: computed(() => Boolean(preview)) };
}
