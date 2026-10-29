import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useWorkspace } from "../page";
import { jsonValueSnapshot } from "@/state/view-model";
import { errorMessage,feedbackDiagnostic } from "@/v3/domain-presentation.mjs";
const blank = () => ({
  items: [],
  cursor: "",
  ready: false,
  busy: false,
  error: "",
  failure: null,
  extra: {},
});
/** One scoped read source. Retry retains its original loader/cursor, never a write. */
export default function useWorkspaceRead({
  identity,
  read,
  enabled = true,
  characterScoped = true,
  keyOf = (row) => row.id ?? row.revision ?? row.source_ref,
}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const owner = w.state.session?.actorId,
    fid = w.current?.value?.id || w.state.selectedId || "",
    scope = JSON.stringify([owner, characterScoped ? fid : null, identity]);
  const [state, setState] = useState(blank),
    current = useRef(state),
    currentScope = useRef(scope),
    loader = useRef(read),
    keys = useRef(keyOf),
    active = useRef(false),
    epoch = useRef(0),
    failed = useRef(null),
    scopeOfState = useRef(scope);
  current.current = state;
  currentScope.current = scope;
  loader.current = read;
  keys.current = keyOf;
  const live = () =>
    active.current &&
    enabled &&
    !!owner &&
    (characterScoped
      ? !!fid && w.isCurrent(owner, fid)
      : w.state.session?.actorId === owner) &&
    currentScope.current === scope;
  async function load(append = false, retry = false) {
    if (!live()) return;
    const request = retry
      ? failed.current
      : {
          scope,
          append,
          cursor: append ? current.current.cursor : "",
          read: loader.current,
        };
    if (!request || request.scope !== scope) return;
    const token = ++epoch.current;
    setState((old) => ({ ...old, busy: true, error: "" }));
    try {
      const page = await request.read(request.cursor);
      if (!live() || token !== epoch.current) return;
      const { items, next_cursor, ...extra } = jsonValueSnapshot(page);
      if (!Array.isArray(items)) throw Error("记录目录尚未完整返回。");
      setState((old) => {
        const merged = request.append
          ? new Map(old.items.map((row) => [keys.current(row), row]))
          : new Map();
        for (const row of items) merged.set(keys.current(row), row);
        return {
          items: [...merged.values()],
          cursor: next_cursor || "",
          extra,
          ready: true,
          busy: false,
          error: "",
          failure:null,
        };
      });
      failed.current = null;
    } catch (cause) {
      if (live() && token === epoch.current) {
        failed.current = request;
        setState((old) => ({
          ...old,
          busy: false,
          error: errorMessage(cause),
          failure:feedbackDiagnostic(cause),
        }));
      }
    }
  }
  useEffect(() => {
    active.current = true;
    epoch.current++;
    if (scopeOfState.current !== scope) {
      scopeOfState.current = scope;
      current.current = blank();
      setState(blank());
      failed.current = null;
    }
    if (enabled && !current.current.ready) void load();
    else if (!enabled)
      setState((old) => (old.busy ? { ...old, busy: false } : old));
    return () => {
      active.current = false;
      epoch.current++;
    };
  }, [scope, enabled]);
  function adopt(page) {
    if (!live()) return;
    const { items, next_cursor, ...extra } = jsonValueSnapshot(page);
    if (!Array.isArray(items)) throw Error("记录目录尚未完整返回。");
    epoch.current++;
    failed.current = null;
    const next = {
      items,
      cursor: next_cursor || "",
      extra,
      ready: true,
      busy: false,
      error: "",
      failure:null,
    };
    current.current = next;
    setState(next);
  }
  return {
    ...(scopeOfState.current === scope ? state : blank()),
    load: () => load(false),
    more: () => load(true),
    retry: () => load(false, true),
    adopt,
  };
}
