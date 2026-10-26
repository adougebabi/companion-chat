import { JSDOM } from "jsdom";
import React, { act } from "react";
const properties = [
  "window",
  "document",
  "localStorage",
  "sessionStorage",
  "navigator",
  "location",
  "history",
  "HTMLElement",
  "HTMLInputElement",
  "HTMLTextAreaElement",
  "HTMLSelectElement",
  "HTMLButtonElement",
  "HTMLFormElement",
  "Element",
  "Node",
  "NodeFilter",
  "DocumentFragment",
  "MutationObserver",
  "CustomEvent",
  "Event",
  "MouseEvent",
  "KeyboardEvent",
  "getComputedStyle",
  "requestAnimationFrame",
  "cancelAnimationFrame",
  "ResizeObserver",
  "IS_REACT_ACT_ENVIRONMENT",
];
export function installDOM() {
  const dom = new JSDOM(
    '<!doctype html><html><body><div id="react-test-root"></div></body></html>',
    { url: "http://react.test/", pretendToBeVisual: true },
  );
  const previous = new Map(
    properties.map((key) => [
      key,
      Object.getOwnPropertyDescriptor(globalThis, key),
    ]),
  );
  for (const key of properties) {
    let value = ["localStorage", "sessionStorage"].includes(key)
      ? (globalThis[key] ?? dom.window[key])
      : dom.window[key];
    if (key === "IS_REACT_ACT_ENVIRONMENT") value = true;
    if (key === "ResizeObserver")
      value = class {
        observe() {}
        unobserve() {}
        disconnect() {}
      };
    if (
      [
        "getComputedStyle",
        "requestAnimationFrame",
        "cancelAnimationFrame",
      ].includes(key)
    )
      value = value?.bind(dom.window);
    if (value !== undefined)
      Object.defineProperty(globalThis, key, {
        value,
        configurable: true,
        writable: true,
      });
  }
  dom.window.HTMLElement.prototype.scrollIntoView = function () {};
  dom.window.HTMLElement.prototype.scrollTo = function () {};
  dom.window.HTMLElement.prototype.hasPointerCapture = () => false;
  dom.window.HTMLElement.prototype.setPointerCapture = () => {};
  dom.window.HTMLElement.prototype.releasePointerCapture = () => {};
  return {
    dom,
    restore() {
      dom.window.close();
      for (const [key, descriptor] of previous)
        if (descriptor) Object.defineProperty(globalThis, key, descriptor);
        else delete globalThis[key];
    },
  };
}
export async function mountReact(
  vite,
  Component,
  { props = {}, workspace } = {},
) {
  const environment = installDOM(),
    { createRoot } = await import("react-dom/client");
  const host = document.getElementById("react-test-root"),
    root = createRoot(host);
  const { WorkspaceContext } = await vite.ssrLoadModule(
    "/src/react-app/page.tsx",
  );
  const element = () =>
    workspace
      ? React.createElement(
          WorkspaceContext.Provider,
          { value: workspace },
          React.createElement(Component, props),
        )
      : React.createElement(Component, props);
  await act(async () => root.render(element()));
  const flush = async () => {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 25));
    });
  };
  await flush();
  return {
    host,
    flush,
    async click(node) {
      await act(async () => node.click());
      await flush();
    },
    async input(node, value) {
      await act(async () => {
        const prototype =
          node.tagName === "TEXTAREA"
            ? window.HTMLTextAreaElement.prototype
            : window.HTMLInputElement.prototype;
        Object.getOwnPropertyDescriptor(prototype, "value").set.call(
          node,
          value,
        );
        node.dispatchEvent(new window.Event("input", { bubbles: true }));
        node.dispatchEvent(new window.Event("change", { bubbles: true }));
      });
      await flush();
    },
    async update(next) {
      props = next;
      await act(async () => root.render(element()));
      await flush();
    },
    async dispose() {
      await act(async () => root.unmount());
      environment.restore();
    },
  };
}
export async function renderReact(vite, Component, workspace, props = {}) {
  const view = await mountReact(vite, Component, { workspace, props });
  try {
    return view.host.innerHTML;
  } finally {
    await view.dispose();
  }
}
export { React, act };
