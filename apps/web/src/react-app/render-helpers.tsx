import * as React from "react";
import { valueCell } from "../state/view-model";
export const Fragment = React.Fragment;
export const displayText = (value: unknown) =>
  value == null
    ? ""
    : typeof value === "object"
      ? JSON.stringify(value, null, 2)
      : String(value);
export const isCell = (value: any) =>
  Boolean(value && typeof value === "object" && valueCell in value);
export const slotContent = (render: any) => render;
export const numberInput = (value: any) =>
  value === "" ? "" : Number.isNaN(Number(value)) ? value : Number(value);
export function listValues(source: any): any {
  if (source == null) return [];
  if (Array.isArray(source) || typeof source === "string")
    return Array.from(source);
  if (typeof source === "number")
    return Array.from({ length: Math.max(0, source) }, (_, i) => i + 1);
  return {
    map: (render: any) =>
      Object.entries(source).map(([key, value], index) =>
        render(value, key, index),
      ),
  };
}
export function eventModifiers(handler: any, modifiers: string[]) {
  return (event: any, ...args: any[]) => {
    if (modifiers.includes("self") && event.target !== event.currentTarget)
      return;
    if (
      modifiers.includes("exact") &&
      (event.shiftKey || event.ctrlKey || event.altKey || event.metaKey)
    )
      return;
    if (modifiers.includes("stop")) event.stopPropagation();
    if (modifiers.includes("prevent")) event.preventDefault();
    return handler(event, ...args);
  };
}
export function keyModifiers(handler: any, keys: string[]) {
  return (event: any) => {
    if (
      event.isComposing ||
      event.nativeEvent?.isComposing ||
      event.keyCode === 229
    )
      return;
    const names: Record<string, string> = {
      enter: "Enter",
      esc: "Escape",
      space: " ",
      tab: "Tab",
      up: "ArrowUp",
      down: "ArrowDown",
    };
    if (keys.some((key) => event.key === (names[key] || key)))
      return handler(event);
  };
}
export function Dynamic({ as: Component, ...props }: any) {
  return <Component {...props} />;
}

export const isComposingKey = (event: any): boolean =>
  Boolean(
    event.isComposing ||
    event.nativeEvent?.isComposing ||
    event.keyCode === 229,
  );
