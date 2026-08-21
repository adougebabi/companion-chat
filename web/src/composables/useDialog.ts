import {nextTick, onBeforeUnmount, ref} from 'vue';
import type {Ref} from 'vue';

function focusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(
    'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  ));
}

export function useDialog() {
  const dialog = ref<HTMLDialogElement | HTMLElement | null>(null);
  const isOpen = ref(false);
  let returnFocus: HTMLElement | null = null;

  async function open(trigger?: HTMLElement | null): Promise<void> {
    returnFocus = trigger ?? (typeof document !== 'undefined' && document.activeElement instanceof HTMLElement ? document.activeElement : null);
    isOpen.value = true;
    await nextTick();
    const element = dialog.value;
    if (!element) return;
    const nativeDialog = element instanceof HTMLDialogElement ? element : null;
    if (nativeDialog && !nativeDialog.open) nativeDialog.showModal();
    (element.querySelector<HTMLElement>('[data-initial-focus]') ?? focusableElements(element)[0] ?? element).focus();
  }

  function close(): void {
    const element = dialog.value;
    if (element instanceof HTMLDialogElement && element.open) element.close();
    isOpen.value = false;
    returnFocus?.focus();
    returnFocus = null;
  }

  function onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
      return;
    }
    if (event.key !== 'Tab' || !dialog.value) return;
    const elements = focusableElements(dialog.value);
    if (!elements.length) return;
    const first = elements[0];
    const last = elements[elements.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  onBeforeUnmount(() => {
    if (dialog.value instanceof HTMLDialogElement && dialog.value.open) dialog.value.close();
  });

  return {dialog, isOpen, open, close, onKeydown};
}

export default useDialog;

