import {
  createContext,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { proxy, snapshot, subscribe } from "valtio/vanilla";
import { cell, selector, readValue, valueSnapshot } from "../state/view-model";

// Pages keep their long-lived domain controllers; React owns their render tree
// and starts/disposes effects. No request or timer starts during render.
export const WorkspaceContext = createContext<any>(null);
export const useWorkspace = () => useContext(WorkspaceContext);
const same = (a: any, b: any): boolean =>
  Object.is(a, b) ||
  (Array.isArray(a) &&
    Array.isArray(b) &&
    a.length === b.length &&
    a.every((v, i) => same(v, b[i])));
export const afterPaint = (run?: () => unknown): Promise<any> =>
  new Promise((resolve) => {
    const finish = () => resolve(run?.());
    if (typeof requestAnimationFrame === "function")
      requestAnimationFrame(() => requestAnimationFrame(finish));
    else queueMicrotask(finish);
  });

export function useController(
  factory: (
    props: any,
    events: { emit: (event: string, ...args: unknown[]) => unknown },
    controller: any,
  ) => () => ReactNode,
  input: any,
) {
  const workspace = useWorkspace();
  const props = useRef(input);
  props.current = input;
  const [scope] = useState(() => {
    const signal = proxy({ revision: 0 });
    let alive = false,
      queued = false;
    let cleanup: (() => void)[] = [],
      starts: (() => unknown)[] = [],
      watches: (() => void)[] = [];
    const changed = () => {
      if (!alive || queued) return;
      queued = true;
      queueMicrotask(() => {
        queued = false;
        if (!alive) return;
        signal.revision++;
      });
    };
    const track = <T extends object>(value: T) => {
      cleanup.push(subscribe(value, changed));
      return value;
    };
    const scope = {
      workspace,
      signal,
      view: null as null | (() => ReactNode),
      props: new Proxy(
        {},
        {
          get: (_target, key) => {
            if (key in props.current) return props.current[key];
            return undefined;
          },
        },
      ),
      emit: (event: string, ...args: unknown[]) => {
        const name =
          "on" +
          event[0].toUpperCase() +
          event.slice(1).replace(/:([a-z])/g, (_, s) => s.toUpperCase());
        return props.current[name]?.(...args);
      },
      cell: <T,>(value?: T) => track(cell(value)),
      model: <T extends object>(value: T) => track(proxy(value)),
      selector,
      observe(source: any, callback: any, options: any = {}) {
        const read = () =>
          typeof source === "function"
            ? source()
            : Array.isArray(source)
              ? source.map(readValue)
              : readValue(source);
        let previous: any,
          before: any,
          initialized = false,
          invalidate: undefined | (() => void);
        const run = () => {
          const value = read(),
            current = valueSnapshot(value);
          if (!initialized || !same(current, previous)) {
            const initial = !initialized,
              old = before;
            initialized = true;
            previous = current;
            before = value;
            if (!initial || options.immediate) {
              invalidate?.();
              callback(value, old, (fn: () => void) => (invalidate = fn));
            }
          }
        };
        watches.push(run);
        cleanup.push(() => invalidate?.());
        return () => {
          watches = watches.filter((w) => w !== run);
          invalidate?.();
        };
      },
      onStart: (run: () => unknown) => starts.push(run),
      onDispose: (run: () => void) => cleanup.push(run),
      commit: () => {
        if (alive) for (const run of [...watches]) run();
      },
      start() {
        alive = true;
        scope.workspace = workspace;
        if (workspace) cleanup.push(workspace.subscribe(changed));
        scope.view = factory(scope.props, { emit: scope.emit }, scope);
        for (const run of starts) void run();
        changed();
      },
      dispose() {
        alive = false;
        for (const stop of cleanup.reverse()) stop();
        cleanup = [];
        starts = [];
        watches = [];
        scope.view = null;
      },
    };
    return scope;
  });
  useSyncExternalStore(
    (listener) => subscribe(scope.signal, listener),
    () => snapshot(scope.signal),
    () => snapshot(scope.signal),
  );
  useLayoutEffect(() => {
    scope.start();
    return () => scope.dispose();
  }, [scope]);
  useLayoutEffect(() => scope.commit());
  return scope.view?.() ?? null;
}
