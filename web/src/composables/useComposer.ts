import {ref} from 'vue';
import type {Ref} from 'vue';
import type {Attachment} from '../types';

export interface ComposerSendInput {
  text: string;
  attachments?: Attachment[];
}

export interface ComposerSender<TResult = unknown> {
  send(input: ComposerSendInput): Promise<TResult>;
}

function textareaFrom(event: Event): HTMLTextAreaElement | null {
  return event.currentTarget instanceof HTMLTextAreaElement
    ? event.currentTarget
    : event.target instanceof HTMLTextAreaElement ? event.target : null;
}

export function useComposer<TResult = unknown>(sender: ComposerSender<TResult>, initialDraft = '') {
  const textarea = ref<HTMLTextAreaElement | null>(null);
  const draft = ref(initialDraft);
  const selectionStart = ref(initialDraft.length);
  const selectionEnd = ref(initialDraft.length);
  const isComposing = ref(false);
  const isSending = ref(false);
  const error = ref<string | null>(null);

  function updateSelection(element = textarea.value): void {
    if (!element) return;
    selectionStart.value = element.selectionStart ?? draft.value.length;
    selectionEnd.value = element.selectionEnd ?? selectionStart.value;
  }

  function onInput(event: Event): void {
    const element = textareaFrom(event);
    if (!element) return;
    draft.value = element.value;
    updateSelection(element);
  }

  function onSelect(event: Event): void {
    updateSelection(textareaFrom(event) ?? textarea.value);
  }

  function onCompositionStart(): void {
    isComposing.value = true;
  }

  function onCompositionEnd(event?: CompositionEvent): void {
    isComposing.value = false;
    if (event) onInput(event);
  }

  function setDraft(value: string): void {
    draft.value = value;
    selectionStart.value = Math.min(selectionStart.value, value.length);
    selectionEnd.value = Math.min(selectionEnd.value, value.length);
  }

  function setSelection(start: number, end = start): void {
    selectionStart.value = Math.max(0, Math.min(start, draft.value.length));
    selectionEnd.value = Math.max(selectionStart.value, Math.min(end, draft.value.length));
  }

  function clearError(): void {
    error.value = null;
  }

  function restoreSelection(): void {
    const element = textarea.value;
    if (!element) return;
    element.setSelectionRange(selectionStart.value, selectionEnd.value);
  }

  async function submit(event?: Event): Promise<TResult | null> {
    event?.preventDefault();
    if (isComposing.value || isSending.value) return null;
    const original = draft.value;
    const text = original.trim();
    if (!text) return null;
    error.value = null;
    isSending.value = true;
    try {
      const result = await sender.send({text});
      // A user may begin composing the next message during the stream. Do
      // not erase that newer draft when the previous send settles.
      if (draft.value === original) {
        draft.value = '';
        selectionStart.value = 0;
        selectionEnd.value = 0;
        if (textarea.value) textarea.value.value = '';
      }
      return result;
    } catch (caught) {
      // Keep the original draft and selection on every failed request.
      error.value = caught instanceof Error ? caught.message : '发送失败';
      throw caught;
    } finally {
      isSending.value = false;
    }
  }

  const bindings = {
    onInput,
    onSelect,
    onCompositionstart: onCompositionStart,
    onCompositionend: onCompositionEnd
  };

  return {
    textarea,
    draft,
    selectionStart,
    selectionEnd,
    isComposing,
    isSending,
    error,
    clearError,
    setDraft,
    setSelection,
    updateSelection,
    restoreSelection,
    onInput,
    onSelect,
    onCompositionStart,
    onCompositionEnd,
    submit,
    bindings
  };
}

export default useComposer;
