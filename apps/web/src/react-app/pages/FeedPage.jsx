import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { Button } from "@/components/ui/button";
import { ChoiceSelect, Textarea } from "../controls";
import { useWorkspace } from "../page";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import FeedPageView from "../views/FeedPage";
import FeedMediaPicker from "../shared/FeedMediaPicker";
import {
  feedFilters,
  feedHref,
  checkMoment,
  momentText,
} from "@/approved-ui/feed-presentation.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
/** @param {{route?: {query?: URLSearchParams,feature?:any},initialPostId?:string}} [input] */
export default function FeedPage({ route, initialPostId } = {}) {
  const w = useWorkspace();
  useSyncExternalStore(w.subscribe, w.getSnapshot, w.getSnapshot);
  const owner = w.state.session?.actorId,
    current = w.current.value,
    service = w.services.momentsClient;
  const filters = feedFilters(route?.query),
    filterKey = JSON.stringify(filters),
    scope = JSON.stringify([owner, filterKey]);
  const [search, setSearch] = useState(filters.q),
    [selected, setSelected] = useState(initialPostId || null),
    [action, setAction] = useState(null),
    [suggestion, setSuggestion] = useState(null),
    [feedback, setFeedback] = useState(""),
    [filterError, setFilterError] = useState("");
  const active = useRef(false),
    scopeRef = useRef(scope),
    trigger = useRef(null),
    commandTrigger = useRef(null),
    feedRef = useRef(null),
    suggestionsRef = useRef(null),
    lastCreated = useRef(null),
    lastSuggestion = useRef(null);
  scopeRef.current = scope;
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    setSearch(filters.q);
    setSelected(initialPostId || null);
    setAction(null);
    setSuggestion(null);
  }, [scope, initialPostId]);
  const live = () =>
    active.current &&
    scopeRef.current === scope &&
    w.state.session?.actorId === owner;
  const assertCurrent = () => {
    if (!live()) throw Error("账号或动态范围已经变化，请返回原提交核对。");
  };
  const assertTarget = (fid) => {
    assertCurrent();
    if (
      fid &&
      !w.state.characters.some(
        (c) =>
          c.id === fid &&
          !["retired", "archived", "已退休", "已归档"].includes(c.status),
      )
    )
      throw Error("请选择当前可写的真实角色。");
  };
  const feed = useWorkspaceRead({
    identity: ["feed", filters],
    characterScoped: false,
    read: (cursor) =>
      service.list({
        fluctlightId: filters.fid,
        status: filters.status,
        q: filters.q,
        cursor,
      }),
  });
  const suggestions = useWorkspaceRead({
    identity: ["feed-suggestions", filters.fid],
    characterScoped: false,
    read: (cursor) => service.suggestions(filters.fid, cursor),
  });
  feedRef.current = feed;
  suggestionsRef.current = suggestions;
  const lastKey = `fluctlight-studio:${owner}:moment:feed:last-suggestion`;
  const lastPostKey = `fluctlight-studio:${owner}:moment:feed:last-publish`;
  useEffect(() => {
    try {
      const value = JSON.parse(studioDraftStorage().getItem(lastKey) || "null");
      if (value?.id && value?.fid) lastSuggestion.current = value;
    } catch {}
  }, [lastKey]);
  function navigateFilters(next) {
    const q = String(next.q ?? filters.q).trim();
    if ([...q].length > 200) {
      setFilterError("搜索最多200个字符。");
      return;
    }
    setFilterError("");
    const query = new URLSearchParams({ ...filters, ...next, q });
    for (const [k, v] of [...query]) if (!v) query.delete(k);
    w.navigatePath("/feed?" + query);
  }
  async function readMoment(id) {
    return checkMoment(await service.record(id), id);
  }
  function adoptPost(row) {
    if (!live()) return;
    const f = feedRef.current;
    const previous = f.items.find((p) => p.id === row.id);
    if (
      previous &&
      (previous.revision > row.revision ||
        JSON.stringify(previous) === JSON.stringify(row))
    )
      return;
    const matches =
      (filters.status === "all" || row.status === filters.status) &&
      (!filters.fid || row.owner_fluctlight_id === filters.fid) &&
      (!filters.q || row.text.toLowerCase().includes(filters.q.toLowerCase()));
    f.adopt({
      ...f.extra,
      items: f.items.some((p) => p.id === row.id)
        ? f.items.flatMap((p) =>
            p.id === row.id ? (matches ? [row] : []) : [p],
          )
        : matches
          ? [row, ...f.items]
          : f.items,
      next_cursor: f.cursor,
    });
  }
  const returnHref = /^#\/feed(?:\?|$)/.test(
    route?.query?.get("returnTo") || "",
  )
    ? route.query.get("returnTo")
    : feedHref(filters);
  const model = {
    w,
    owner,
    current,
    service,
    filters,
    scope,
    live,
    assertCurrent,
    assertTarget,
    readMoment,
    adoptPost,
    feed,
    suggestions,
    search,
    setSearch,
    selected,
    setSelected,
    action,
    setAction,
    suggestion,
    setSuggestion,
    feedback,
    setFeedback,
    filterError,
    navigateFilters,
    trigger,
    commandTrigger,
    initialPostId,
    returnHref,
  };
  function base(identity) {
    return {
      identity,
      scope: ["feed", identity],
      storageKey: `fluctlight-studio:${owner}:moment:replica:${identity}`,
    };
  }
  function openPost(id, source) {
    trigger.current = source;
    setSelected(id);
  }
  function interact(kind, post, source, preset, onReadback) {
    assertCurrent();
    commandTrigger.current = source;
    setAction({
      ...base(post.id + ":" + kind),
      title:
        kind === "reaction"
          ? "回应这个片刻"
          : kind === "hide"
            ? "隐藏这条动态？"
            : "恢复展示这条动态？",
      description:
        kind === "reaction"
          ? "赞、关心和庆祝，每人保留一种回应。"
          : "只改变展示状态，正文、评论和来源仍保留。",
      initial: { revision: post.revision, kind: preset || "like" },
      submitLabel:
        "确认" + { reaction: "回应", hide: "隐藏", restore: "恢复展示" }[kind],
      build: (d) => ({
        expectedRevision: d.revision,
        reason:
          "所有者明确" +
          { reaction: "回应", hide: "隐藏", restore: "恢复展示" }[kind] +
          "动态",
        ...(kind === "reaction" ? { kind: d.kind } : {}),
      }),
      execute: async (body, key) => {
        assertCurrent();
        return checkMoment(
          await (kind === "reaction"
            ? service.reaction(post.id, { ...body, idempotencyKey: key })
            : service.status(post.id, kind, { ...body, idempotencyKey: key })),
          post.id,
        );
      },
      readLatest: () => readMoment(post.id),
      adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
      onSaved: async (r, body) => {
        const fresh = await readMoment(post.id);
        if (
          fresh.revision < r.revision ||
          (fresh.revision === r.revision &&
            (kind === "reaction"
              ? fresh.viewer_reaction !==
                (body.kind === "none" ? null : body.kind)
              : fresh.status !== (kind === "hide" ? "hidden" : "visible")))
        )
          throw Error("这次回应或展示状态的新版本尚未读回。");
        if (live()) {
          adoptPost(fresh);
          onReadback?.(fresh);
          setFeedback("已读回这次操作后的实际动态。");
        }
      },
      fields: (d, edit) =>
        kind === "reaction" ? (
          <label className="replica-field">
            <span>回应</span>
            <ChoiceSelect
              aria-label="回应"
              value={d.kind}
              onChange={(e) => edit({ kind: e.target.value })}
            >
              <option value="like">赞</option>
              <option value="care">关心</option>
              <option value="celebrate">庆祝</option>
              <option value="none">取消我的回应</option>
            </ChoiceSelect>
          </label>
        ) : (
          <p className="companion-body-text">
            {kind === "hide"
              ? "隐藏后可在「已隐藏」中恢复。"
              : "恢复后可以继续评论和回应。"}
          </p>
        ),
    });
  }
  function compose(mode, source, preset) {
    assertCurrent();
    commandTrigger.current = source;
    lastCreated.current = null;
    const initial = {
      text: "",
      fid: mode === "suggest" ? filters.fid || current.id || "" : filters.fid,
      mediaIds: [],
      ...preset,
    };
    setAction({
      ...base("compose:" + mode + (preset ? ":retry:" + preset.requestId : "")),
      title: mode === "publish" ? "留下一点今天的生活" : "邀请分享近况",
      description:
        mode === "publish"
          ? "这条动态将以「我」的身份发布。"
          : "你可以提出分享建议，由角色决定发不发、怎么写。",
      submitLabel: mode === "publish" ? "发布动态" : "提出分享建议",
      canSubmit: (d) => !!d?.text?.trim(),
      initial,
      validate: (d) => {
        momentText(d.text);
        if (
          d.mediaIds.length > 8 ||
          new Set(d.mediaIds).size !== d.mediaIds.length
        )
          throw Error("最多选择8项不同的附件。");
        if (mode === "suggest" && !d.fid) throw Error("请选择真实角色。");
        assertTarget(d.fid);
      },
      build: (d) => ({
        text: momentText(d.text),
        mediaIds: [...d.mediaIds],
        ...(mode === "suggest"
          ? { fid: d.fid }
          : d.fid
            ? { worldId: d.fid }
            : {}),
      }),
      execute: async (value, key) => {
        assertTarget(value.fid || value.worldId);
        const { fid, ...body } = value;
        if (mode === "suggest") {
          const r = await service.suggest(fid, {
            ...body,
            idempotencyKey: key,
          });
          if (
            !r.request?.id ||
            r.request.fluctlight_id !== fid ||
            r.request.kind !== "moment_suggestion"
          )
            throw Error("分享建议回执尚未完整确认。");
          return r;
        }
        const r = await service.publish({ ...body, idempotencyKey: key });
        if (!r.id || r.author_actor_id !== owner || r.author_type !== "user")
          throw Error("发布身份回执尚未完整确认。");
        return r;
      },
      afterAcknowledged: (d, r) => ({
        ...d,
        lastId: mode === "suggest" ? r.request.id : r.id,
        ...(mode === "suggest" ? { lastFid: r.request.fluctlight_id } : {}),
      }),
      readLatest: async (d) =>
        d.lastId
          ? mode === "suggest"
            ? w.services.lifeClient.request(d.lastFid, d.lastId)
            : readMoment(d.lastId)
          : {
              message:
                "当前草稿将形成一条新的" +
                (mode === "suggest" ? "建议" : "动态"),
            },
      adoptLatest: (d) => d,
      onSaved: async (r, value) => {
        if (mode === "suggest") {
          const request = await w.services.lifeClient.request(
            value.fid,
            r.request.id,
          );
          if (
            request.id !== r.request.id ||
            request.fluctlight_id !== value.fid ||
            request.kind !== "moment_suggestion"
          )
            throw Error("原分享建议尚未完整读回。");
          if (live()) {
            const pointer = { id: request.id, fid: value.fid };
            studioDraftStorage().setItem(lastKey, JSON.stringify(pointer));
            lastSuggestion.current = pointer;
            setSuggestion({ ...pointer, initial: request });
            await suggestionsRef.current.load();
            setFeedback("分享建议已受理，实际判断和发布结果分别显示。");
          }
        } else {
          const row = await readMoment(r.id);
          if (
            row.author_actor_id !== owner ||
            row.author_type !== "user" ||
            row.text !== value.text ||
            row.owner_fluctlight_id !== (value.worldId || null) ||
            JSON.stringify(row.media_asset_ids) !==
              JSON.stringify(value.mediaIds)
          )
            throw Error("已发布正文、附件或身份尚未完整读回。");
          if (live()) {
            adoptPost(row);
            lastCreated.current = row.id;
            studioDraftStorage().setItem(
              lastPostKey,
              JSON.stringify({ id: row.id }),
            );
            setFeedback("你的动态已保存并读回。");
          }
        }
      },
      fields: (d, edit) => (
        <div className="companion-form-stack">
          <label className="replica-field">
            <span>{mode === "suggest" ? "建议哪位角色" : "发布范围"}</span>
            <ChoiceSelect
              aria-label={mode === "suggest" ? "建议哪位角色" : "发布范围"}
              value={d.fid}
              onChange={(e) => edit({ fid: e.target.value })}
            >
              <option value="">
                {mode === "suggest" ? "请选择角色" : "我的全局动态"}
              </option>
              {w.state.characters.map((c) => (
                <option
                  key={c.id}
                  value={c.id}
                  disabled={[
                    "retired",
                    "archived",
                    "已退休",
                    "已归档",
                  ].includes(c.status)}
                >
                  {c.name}
                  {mode === "publish" ? "的动态范围" : ""}
                </option>
              ))}
            </ChoiceSelect>
          </label>
          <Textarea
            aria-label={mode === "publish" ? "动态正文" : "分享建议"}
            rows={6}
            value={d.text}
            placeholder={
              mode === "publish"
                ? "今天有什么想留住的片刻？"
                : "想邀请她分享什么近况？"
            }
            onChange={(e) => edit({ text: e.target.value })}
          />
          <FeedMediaPicker
            model={model}
            ids={d.mediaIds}
            onChange={(mediaIds) => edit({ mediaIds })}
          />
          {preset && (
            <p>原失败建议已带回，确认后会提出一笔新请求；旧记录保留。</p>
          )}
          {d.lastId && (
            <p>原提交已有回执，后续输入仍保留；明确核对后可发布新的内容。</p>
          )}
        </div>
      ),
    });
  }
  Object.assign(model, {
    openPost,
    interact,
    compose,
    closeAction: () => {
      setAction(null);
      if (lastCreated.current) {
        setSelected(lastCreated.current);
        lastCreated.current = null;
      }
    },
    resumeSuggestion: () => {
      if (lastSuggestion.current) setSuggestion(lastSuggestion.current);
    },
    hasLastSuggestion: () => !!lastSuggestion.current,
    composerRecovery: ["publish", "suggest"].filter((mode) => {
      try {
        const slot = `fluctlight-studio:${owner}:moment:replica:compose:${mode}`;
        return [":draft", ":command", ":command:outcome"].some((suffix) =>
          studioDraftStorage().getItem(slot + suffix),
        );
      } catch {
        return false;
      }
    }),
  });
  return <FeedPageView model={model} />;
}
