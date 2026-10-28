import { useEffect, useRef, useState } from "react";
import useWorkspaceRead from "./useWorkspaceRead";
/** Follows only an existing domain record. A read limit pauses observation, never marks failure. */
export default function useMediaFollow({
  identity,
  read,
  pending,
  enabled = true,
  guard,
}) {
  const automatic = useRef(false),
    latest = useRef(null),
    reads = useRef(0),
    loader = useRef(null),
    [paused, setPaused] = useState(false);
  const source = useWorkspaceRead({
    identity,
    characterScoped: false,
    enabled,
    read: async () => {
      const auto = automatic.current,
        row = await read();
      if (auto && guard && latest.current && !guard(latest.current, row))
        throw Error("原会话已经变化，请明确重新读取新的状态。");
      return { items: row ? [row] : [], next_cursor: "" };
    },
  });
  latest.current = source.items[0];
  loader.current = source.load;
  const key = JSON.stringify(identity),
    isPending = !!source.items[0] && pending(source.items[0]);
  useEffect(() => {
    reads.current = 0;
    setPaused(false);
  }, [key]);
  useEffect(() => {
    let alive = true,
      timer;
    const query = () => {
      if (!alive || document.hidden || reads.current >= 120) return;
      reads.current++;
      automatic.current = true;
      void loader.current().finally(() => {
        automatic.current = false;
      });
      if (reads.current >= 120) setPaused(true);
    };
    const visible = () => {
      clearTimeout(timer);
      if (!document.hidden && isPending && !source.error) query();
    };
    if (
      enabled &&
      isPending &&
      !source.busy &&
      !source.error &&
      !document.hidden &&
      reads.current < 120
    )
      timer = setTimeout(query, 3000);
    document.addEventListener("visibilitychange", visible);
    return () => {
      alive = false;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
    };
  }, [key, enabled, isPending, source.items[0], source.busy, source.error]);
  return {
    ...source,
    paused,
    load: () => {
      reads.current = 0;
      setPaused(false);
      automatic.current = false;
      return source.load();
    },
  };
}
