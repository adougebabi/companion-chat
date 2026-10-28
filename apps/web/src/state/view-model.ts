import { proxy, ref as untracked, getVersion, snapshot } from "valtio/vanilla";

export const valueCell = Symbol("view-model-value");
export type Cell<T> = { value: T; [valueCell]: true };
export function cell<T>(value?: T): Cell<T | undefined> {
  const model = proxy({ value });
  Object.defineProperty(model, valueCell, { value: true });
  return model as Cell<T | undefined>;
}
export function selector<T>(
  read: (() => T) | { get(): T; set(value: T): void },
): Cell<T> {
  return {
    [valueCell]: true,
    get value() {
      return typeof read === "function" ? read() : read.get();
    },
    set value(value) {
      if (typeof read !== "function") read.set(value);
    },
  };
}
export function readValue<T>(value: T | Cell<T>): T {
  return value && typeof value === "object" && valueCell in value
    ? (value as Cell<T>).value
    : (value as T);
}
export function setElement(target: Cell<unknown>, element: Element | null) {
  target.value = element ? untracked(element) : null;
}
export function valueSnapshot(value: unknown): unknown {
  if (getVersion(value) !== undefined) return snapshot(value as object);
  if (Array.isArray(value)) return value.map(valueSnapshot);
  return value;
}
// JSON DTOs can contain nested Valtio proxies after entering a domain store.
// Detach their serializable values before holding an editable/frozen UI baseline.
export function jsonValueSnapshot<T>(value: T): T {
  return JSON.parse(JSON.stringify(value));
}
export { proxy as model };

const elementRefs = new WeakMap<object, (node: Element | null) => void>();
export function elementRef(target: Cell<unknown>) {
  let callback = elementRefs.get(target);
  if (!callback) {
    callback = (node) => setElement(target, node);
    elementRefs.set(target, callback);
  }
  return callback;
}
