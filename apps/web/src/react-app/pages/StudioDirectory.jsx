import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useWorkspace } from "../page";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import StudioDirectoryView from "../views/StudioDirectory";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
/** @param {{kind?:string,route?:any}} input */
export default function StudioDirectory({ kind: fixedKind = "", route } = {}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const owner = w.state.session.actorId,
    preferenceKey=`fluctlight-studio:${owner}:${fixedKind?'directory-ui:'+fixedKind:'directory-ui'}`;
  const [preferences] = useState(() => {
    try {
      return JSON.parse(
        sessionStorage.getItem(preferenceKey) ||
          "{}",
      );
    } catch {
      return {};
    }
  });
  const [search, setSearch] = useState(
      route?.query?.get("q") ?? preferences.search ?? "",
    ),
    [query, setQuery] = useState(
      route?.query?.get("q") ?? preferences.query ?? "",
    ),
    [kind, setKind] = useState(
      fixedKind || route?.query?.get("kind") || preferences.kind || "",
    ),
    [archived, setArchived] = useState(
      route?.query?.has("archived")
        ? route.query.get("archived") === "true"
        : !!preferences.archived,
    ),
    [action, setAction] = useState(null),
    trigger = useRef(null),
    active = useRef(false),
    reader = useRef(null),
    actionTriggers = useRef(new Map());
  const scope = JSON.stringify([owner, query, kind, archived]),
    scopeRef = useRef(scope);
  scopeRef.current = scope;
  useEffect(() => {
    try {
      sessionStorage.setItem(
        preferenceKey,
        JSON.stringify({ search, query, kind, archived }),
      );
    } catch {}
  }, [owner, preferenceKey, search, query, kind, archived]);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  const live = () =>
    active.current &&
    scopeRef.current === scope &&
    w.state.session.actorId === owner;
  const assertCurrent = () => {
    if (!live()) throw Error("账号或资源目录范围已经变化。");
  };
  const list = useWorkspaceRead({
    identity: ["studio-directory", query, kind, archived],
    characterScoped: false,
    read: (cursor) =>
      w.client.listStudioResources({
        q: query || undefined,
        kind: kind || undefined,
        archived,
        cursor,
      }),
  });
  reader.current = list;
  const service = w.services.studioClient;
  const legacyKey = `approved-studio:${owner}:directory-command`;
  let legacy = null,
    legacyError = "";
  try {
    const raw = localStorage.getItem(legacyKey),
      outcomeRaw = localStorage.getItem(legacyKey + ":outcome");
    if (raw || outcomeRaw) {
      const record = raw ? JSON.parse(raw) : null,
        outcome = outcomeRaw ? JSON.parse(outcomeRaw) : null;
      const value =
        record?.value ||
        outcome?.accepted?.value ||
        JSON.parse(
          localStorage.getItem(legacyKey + ":recovery:draft") || "null",
        );
      if (
        !value ||
        !["archive", "restore"].includes(value.operation) ||
        typeof value.id !== "string" ||
        !Number.isSafeInteger(value.revision) ||
        typeof value.reason !== "string"
      )
        throw Error("旧版资源管理原提交无法读取，请保留原记录核对。");
      legacy = value;
    }
  } catch (cause) {
    legacyError = cause.message;
  }
  function recoverLegacy(event) {
    assertCurrent();
    trigger.current = event.currentTarget;
    const value = legacy;
    setAction({
      identity: "studio-directory-legacy:" + owner,
      className: "replica-studio",
      scope: ["studio-directory-legacy", value.id],
      storage: () => localStorage,
      storageKey: legacyKey + ":recovery",
      commandKey: legacyKey,
      title: "核对旧版资源" + (value.operation === "archive" ? "归档" : "恢复"),
      description:
        "继续原提交的对象、原因、版本和请求身份；核对完成后再进行新的资源管理操作。",
      initial: value,
      build: (d) => ({
        operation: d.operation,
        id: d.id,
        revision: d.revision,
        reason: d.reason,
      }),
      execute: async (original, key) => {
        if (w.state.session.actorId !== owner) throw Error("账号已切换。");
        const result = await service[original.operation](original.id, {
          expectedRevision: original.revision,
          reason: original.reason,
          idempotencyKey: key,
        });
        if (result.id !== original.id || !Number.isSafeInteger(result.revision))
          throw Error("原资源管理回执尚未完整确认。");
        return result;
      },
      readLatest: (d) => service.resource(d.id),
      adoptLatest: (d, r) => ({ ...d, revision: r.revision }),
      onSaved: async (result, original) => {
        const actual = await service.resource(original.id);
        if (
          actual.id !== original.id ||
          actual.revision < result.revision ||
          (actual.revision === result.revision &&
            actual.status !== result.status)
        )
          throw Error("原资源管理结果尚未读回。");
        if (live()) await reader.current.load();
      },
    });
  }
  function manage(row, event, originalOperation) {
    assertCurrent();
    if (legacy || legacyError) return;
    trigger.current = actionTriggers.current.get(row.id) || event.currentTarget;
    const operation =
      originalOperation || (row.status === "archived" ? "restore" : "archive");
    studioDraftStorage().setItem(
      `fluctlight-studio:${owner}:directory:${row.id}:${operation}:context`,
      JSON.stringify({ id: row.id, name: row.name, operation }),
    );
    setAction({
      identity: "studio-directory:" + row.id + ":" + operation,
      className: "replica-studio",
      scope: ["studio-directory", row.id, operation],
      storageKey: `fluctlight-studio:${owner}:directory:${row.id}:${operation}`,
      title:
        (operation === "restore" ? "恢复" : "归档") + "「" + row.name + "」",
      description:
        "保存内容、发布历史、任务与素材保留；仍被引用时由服务端拒绝归档。恢复不会自动应用。",
      submitLabel: "确认" + (operation === "restore" ? "恢复" : "归档"),
      initial: { revision: row.revision, reason: "" },
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason].length > 1000)
          throw Error("请填写操作原因，最多1000个字符。");
      },
      build: (d) => ({ expectedRevision: d.revision, reason: d.reason }),
      execute: async (body, key) => {
        assertCurrent();
        const r = await service[operation](row.id, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== row.id ||
          !Number.isSafeInteger(r.revision) ||
          r.status !==
            (operation === "archive"
              ? "archived"
              : r.published_version
                ? "published"
                : "draft")
        )
          throw Error("资源管理回执尚未完整确认。");
        return r;
      },
      readLatest: () => service.resource(row.id),
      adoptLatest: (d, r) => ({ ...d, revision: r.revision }),
      onSaved: async (r) => {
        const actual = await service.resource(row.id);
        if (
          actual.id !== row.id ||
          actual.revision < r.revision ||
          (actual.revision === r.revision && actual.status !== r.status)
        )
          throw Error("资源管理的新状态尚未读回。");
        if (live()) await reader.current.load();
      },
    });
  }
  let recovery = [];
  try {
    const s = studioDraftStorage(),
      prefix = `fluctlight-studio:${owner}:directory:`;
    recovery = Object.keys(s)
      .filter((key) => key.startsWith(prefix) && key.endsWith(":context"))
      .map((key) => ({ slot: key.slice(0, -8), ...JSON.parse(s.getItem(key)) }))
      .filter(
        (row) =>
          typeof row.id === "string" &&
          ["archive", "restore"].includes(row.operation) &&
          [":command", ":command:outcome"].some((suffix) =>
            s.getItem(row.slot + suffix),
          ),
      );
  } catch {}
  return (
    <StudioDirectoryView
      model={{
        w,
        owner,
        fixedKind,
        route,
        list,
        search,
        setSearch,
        query,
        setQuery,
        kind,
        setKind,
        archived,
        setArchived,
        action,
        setAction,
        manage,
        trigger,
        actionTriggers,
        recovery,
        legacy,
        legacyError,
        recoverLegacy,
        create: (type) =>
          w.navigatePath(
            "/studio/edit/new?kind=" +
              encodeURIComponent(type || kind || "world"),
          ),
        edit: (row) =>
          w.navigatePath(
            (row.kind === "npc" ? "/studio/npc/" : "/studio/edit/") +
              encodeURIComponent(row.id),
          ),
      }}
    />
  );
}
