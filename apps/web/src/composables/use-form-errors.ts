import { nextTick, ref } from "vue";
import { validateFields, type FieldRule } from "../lib/form-validation";
export async function focusField(id: string) {
  await nextTick();
  if (typeof document !== "undefined") document.getElementById(id)?.focus();
}
export function useFormErrors() {
  const errors = ref<Record<string, string>>({});
  async function validate(fields: FieldRule[]) {
    errors.value = validateFields(fields);
    const first = Object.keys(errors.value)[0];
    if (!first) return true;
    await focusField(first);
    return false;
  }
  function clear() { errors.value = {}; }
  return { errors, validate, clear, focus: focusField };
}
