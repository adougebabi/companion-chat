import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRightIcon,
  Edit3Icon,
  RefreshCwIcon,
  SearchIcon,
  SparklesIcon,
  StarIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuCheckboxItem} from '@/components/ui/dropdown-menu';
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Surface, FieldInput, Textarea, Disclosure, Badge } from "../controls";
import { useWorkspace } from "../page";
import { jsonValueSnapshot } from "@/state/view-model";
import { day } from "@/state/workspace.mjs";
import { previousDiaryDate } from "@/approved-ui/diary-generation.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import RequestStatus from "../shared/RequestStatus";
import JsonDetails from "../shared/JsonDetails";
import LoadingState from "../shared/LoadingState";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import { EmptyState, InlineFeedback } from "../shared/Presentation";
const statuses = {
  pending: "等待整理",
  running: "正在回顾",
  ready: "已经写下",
  no_content: "这次未形成正文",
  failed: "整理未完成",
  inactive: "尚未运行",
};
const empty = () => ({ items: [], cursor: "", ready: false });
const field = (label, control) => (
  <label className="replica-field">
    <span>{label}</span>
    {control}
  </label>
);
export default function DiaryPage() {
  const w = useWorkspace(),
    c = w.current.value,
    fid = c.id,
    owner = w.state.session.actorId,
    scope = JSON.stringify([owner, fid]);
  const [tab, setTab] = useState("diary"),
    [favorite, setFavorite] = useState(false),
    [query, setQuery] = useState(""),
    [lists, setLists] = useState({
      all: empty(),
      favorite: empty(),
      shared: empty(),
    }),
    [loading, setLoading] = useState({}),
    [errors, setErrors] = useState({}),
    [selectedId, setSelectedId] = useState(""),
    [detail, setDetail] = useState(null),
    [detailBusy, setDetailBusy] = useState(false),
    [detailError, setDetailError] = useState(""),
    [replies, setReplies] = useState(empty()),
    [replyBusy, setReplyBusy] = useState(false),
    [replyError, setReplyError] = useState(""),
    [action, setAction] = useState(null),
    [generationId, setGenerationId] = useState(""),
    [watchNote, setWatchNote] = useState(""),
    [watchRefresh, setWatchRefresh] = useState(0),
    [replyPointer, setReplyPointer] = useState(null),
    [notice, setNotice] = useState("");
  const mounted = useRef(false),
    epochs = useRef({}),
    detailEpoch = useRef(0),
    replyEpoch = useRef(0),
    selectedRef = useRef(""),
    cache = useRef(new Map()),
    retry = useRef({}),
    trigger = useRef(null),
    pinned = useRef(""),
    sequence = useRef(0),
    stamps = useRef(new Map());
  const live = () => mounted.current && !!fid && w.isCurrent(owner, fid),
    readonly = ["retired", "archived", "已退休", "已归档"].includes(c.status),
    kind = favorite ? "favorite" : "all",
    page = lists[kind],
    selected =
      detail?.id === selectedId ? detail : cache.current.get(selectedId),
    visible = page.items
      .map((row) => cache.current.get(row.id) || row)
      .filter((row) =>
        [row.date, row.body]
          .filter(Boolean)
          .join(" ")
          .toLowerCase()
          .includes(query.trim().toLowerCase()),
      );
  function check(row, id) {
    if (
      !row ||
      row.fluctlight_id !== fid ||
      row.source !== "agent" ||
      !row.id ||
      (id && row.id !== id)
    )
      throw Error("日记身份或来源不匹配，已保留确认内容。");
    return row;
  }
  function adopt(row, stamp = ++sequence.current) {
    check(row);
    const old = cache.current.get(row.id);
    if (
      old &&
      (row.revision < old.revision ||
        (row.revision === old.revision &&
          stamp < (stamps.current.get(row.id) || 0)))
    )
      return old;
    const next = { ...old, ...jsonValueSnapshot(row) };
    if (old?.annotation?.revision > row.annotation?.revision)
      next.annotation = old.annotation;
    cache.current.set(row.id, next);
    stamps.current.set(row.id, stamp);
    return next;
  }
  async function loadList(
    source,
    append = false,
    cursor = append ? lists[source].cursor : "",
  ) {
    if (!live()) return;
    const stamp = ++sequence.current,
      token = (epochs.current[source] || 0) + 1;
    epochs.current[source] = token;
    setLoading((old) => ({ ...old, [source]: true }));
    setErrors((old) => ({ ...old, [source]: "" }));
    try {
      const result = await (source === "shared"
        ? w.client.sharedExperiences(fid, cursor)
        : w.client.listDiary(fid, cursor, {
            favorite: source === "favorite",
            limit: 20,
          }));
      if (!live() || epochs.current[source] !== token) return;
      const rows =
        source === "shared"
          ? jsonValueSnapshot(result.items)
          : result.items.map((row) => adopt(row, stamp));
      setLists((old) => ({
        ...old,
        [source]: {
          items: append
            ? [
                ...old[source].items,
                ...rows.filter(
                  (row) => !old[source].items.some((old) => old.id === row.id),
                ),
              ]
            : rows,
          cursor: result.next_cursor || "",
          ready: true,
        },
      }));
    } catch (cause) {
      if (live() && epochs.current[source] === token) {
        setErrors((old) => ({ ...old, [source]: errorMessage(cause) }));
        retry.current[source] = () => void loadList(source, append, cursor);
      }
    } finally {
      if (live() && epochs.current[source] === token)
        setLoading((old) => ({ ...old, [source]: false }));
    }
  }
  async function readEntry(id = selectedId) {
    if (!live() || !id) return;
    const stamp = ++sequence.current,
      token = ++detailEpoch.current;
    setDetailBusy(true);
    setDetailError("");
    try {
      const row = check(await w.client.diary(fid, id), id);
      if (!live() || token !== detailEpoch.current) return;
      const value = adopt(row, stamp);
      if (selectedRef.current === id) setDetail(value);
      return value;
    } catch (cause) {
      if (live() && token === detailEpoch.current)
        setDetailError(errorMessage(cause));
      throw cause;
    } finally {
      if (live() && token === detailEpoch.current) setDetailBusy(false);
    }
  }
  async function readReplies(
    id = selectedId,
    append = false,
    cursor = append ? replies.cursor : "",
  ) {
    if (!live() || !id) return;
    const token = ++replyEpoch.current;
    setReplyBusy(true);
    setReplyError("");
    try {
      const result = await w.client.diaryReplies(fid, id, cursor);
      if (!live() || selectedRef.current !== id || token !== replyEpoch.current)
        return;
      if (result.items.some((row) => row.fluctlight_id !== fid))
        throw Error("留言请求范围不匹配。");
      setReplies((old) => ({
        items: append
          ? [
              ...old.items,
              ...result.items.filter(
                (row) => !old.items.some((value) => value.id === row.id),
              ),
            ]
          : result.items,
        cursor: result.next_cursor || "",
        ready: true,
      }));
    } catch (cause) {
      if (
        live() &&
        selectedRef.current === id &&
        token === replyEpoch.current
      ) {
        setReplyError(errorMessage(cause));
        retry.current.replies = () => void readReplies(id, append, cursor);
      }
    } finally {
      if (live() && token === replyEpoch.current) setReplyBusy(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    try {
      const saved = JSON.parse(
        sessionStorage.getItem(
          "fluctlight-diary:" + owner + ":" + fid + ":generation",
        ) || "null",
      );
      if (saved?.id) setGenerationId(saved.id);
    } catch {}
    return () => {
      mounted.current = false;
      detailEpoch.current++;
      replyEpoch.current++;
    };
  }, [scope]);
  useEffect(() => {
    if (
      live() &&
      !lists[tab === "shared" ? "shared" : kind].ready &&
      !loading[tab === "shared" ? "shared" : kind]
    )
      void loadList(tab === "shared" ? "shared" : kind);
  }, [tab, kind]);
  useEffect(() => {
    if (
      page.ready &&
      (!pinned.current || pinned.current !== selectedId) &&
      !page.items.some((row) => row.id === selectedId)
    )
      setSelectedId(page.items[0]?.id || "");
  }, [page.items, page.ready, kind]);
  useEffect(() => {
    selectedRef.current = selectedId;
    setDetail(cache.current.get(selectedId) || null);
    setDetailError("");
    setReplies(empty());
    setReplyError("");
    replyEpoch.current++;
    if (selectedId) {
      void readEntry(selectedId).catch(() => {});
      void readReplies(selectedId);
    }
  }, [selectedId]);
  useEffect(() => {
    if (action?.presentation === "inline" && action.diaryId !== selectedId)
      setAction(null);
  }, [selectedId]);
  const observedId =
    generationId ||
    (["pending", "running"].includes(selected?.status) ? selected?.id : "");
  useEffect(() => {
    if (!observedId) return;
    let active = true,
      reading = false,
      timer,
      count = 0,
      done = false;
    async function read() {
      if (!active || !live() || document.hidden || reading) return;
      reading = true;
      const stamp = ++sequence.current;
      try {
        const value = check(await w.client.diary(fid, observedId), observedId);
        if (!active || !live()) return;
        const row = adopt(value, stamp);
        if (selectedRef.current === row.id) setDetail(row);
        if (!["pending", "running"].includes(row.status)) {
          done = true;
          setWatchNote(
            row.status === "ready"
              ? "日记正文已读回。"
              : row.status === "no_content"
                ? "这次没有形成正文，原记录和素材仍保留。"
                : "本次整理未完成，可以查看尝试与原任务。",
          );
          if (generationId === row.id) {
            sessionStorage.removeItem(
              "fluctlight-diary:" + owner + ":" + fid + ":generation",
            );
            setGenerationId("");
          }
          void loadList("all");
          if (favorite) void loadList("favorite");
        } else setWatchNote("正在跟进原日记的实际整理状态。");
      } catch (cause) {
        if (active && live())
          setWatchNote(
            "状态读取未完成，原日记身份仍保留。" + errorMessage(cause),
          );
      } finally {
        reading = false;
        if (active && live() && !done && !document.hidden && ++count < 100)
          timer = setTimeout(read, 3000);
        else if (active && live() && !done && count >= 100)
          setWatchNote("自动更新已暂停，可重新读取继续跟进。");
      }
    }
    const visible = () => {
      clearTimeout(timer);
      if (!document.hidden) void read();
    };
    document.addEventListener("visibilitychange", visible);
    void read();
    return () => {
      active = false;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
    };
  }, [observedId, scope, watchRefresh]);
  function noteEditor(event) {
    if (!selected || readonly) return;
    trigger.current = event.currentTarget;
    const entry = jsonValueSnapshot(selected),
      id = entry.id;
    setAction({
      identity: "diary-note:" + id,
      scope: ["diary-note", fid, id],
      storageKey: "fluctlight-diary:" + owner + ":" + fid + ":note:" + id,
      initial: {
        note: entry.annotation.note,
        favorite: entry.annotation.favorite,
        revision: entry.annotation.revision,
      },
      title: "写一点批注",
      presentation: "inline",
      diaryId: id,
      description: "批注和收藏属于你，保留日记原文。",
      submitLabel: "保存批注",
      preAdmissionConflicts: ["diary_annotation_conflict"],
      validate: (d) => {
        if ([...d.note].length > 5000) throw Error("批注最多5000个字符。");
      },
      build: (d) => ({
        note: d.note,
        favorite: d.favorite,
        expectedRevision: d.revision,
      }),
      execute: async (value, key) => {
        if (!w.isCurrent(owner, fid)) throw Error("日记范围已切换。");
        const result = await w.client.annotateDiary(fid, id, {
          ...value,
          idempotencyKey: key,
        });
        if (result.diary_id !== id || !Number.isSafeInteger(result.revision))
          throw Error("批注回执尚未完整读到，原提交仍保留。");
        return result;
      },
      readLatest: async () => check(await w.client.diary(fid, id), id),
      adoptLatest: (d, row) => ({ ...d, revision: row.annotation.revision }),
      onSaved: async (result, value) => {
        const row = check(await w.client.diary(fid, id), id);
        if (
          row.annotation.revision < result.revision ||
          (row.annotation.revision === result.revision &&
            (row.annotation.note !== value.note ||
              row.annotation.favorite !== value.favorite))
        )
          throw Error("批注已有回执，保存内容尚未完整读回。");
        if (live()) {
          const saved = adopt(row);
          if (selectedRef.current === id) setDetail(saved);
          setNotice("批注已保存，日记原文保留。");
          await Promise.all([loadList("all"), loadList("favorite")]);
        }
      },
      fields: (d, edit) => (
        <>
          <label className="lp-row">
            <Checkbox
              checked={d.favorite}
              onCheckedChange={(value) => edit({ favorite: !!value })}
            />
            收藏这篇日记
          </label>
          {field(
            "你的批注",
            <Textarea
              aria-label="日记批注"
              value={d.note}
              onChange={(e) => edit({ note: e.target.value })}
              placeholder="留下一点感受。日记原文会被保留。"
            />,
          )}
        </>
      ),
    });
  }
  function replyEditor(event) {
    if (!selected || readonly || !selected.active_attempt_id) return;
    trigger.current = event.currentTarget;
    const entry = jsonValueSnapshot(selected),
      id = entry.id;
    setAction({
      identity: "diary-reply:" + id,
      scope: ["diary-reply", fid, id],
      storageKey: "fluctlight-diary:" + owner + ":" + fid + ":reply:" + id,
      initial: { text: "", revision: entry.revision },
      title: "读完后，想对" + c.name + "说",
      description: "留言会进入真实处理流程，是否有回应以实际交流记录为准。",
      submitLabel: "发送留言",
      preAdmissionConflicts: ["diary_revision_conflict"],
      validate: (d) => {
        if (!d.text.trim() || [...d.text.trim()].length > 2000)
          throw Error("请填写留言，最多2000个字符。");
      },
      build: (d) => ({ text: d.text.trim(), expectedRevision: d.revision }),
      execute: async (value, key) => {
        if (!w.isCurrent(owner, fid)) throw Error("日记范围已经切换。");
        const result = await w.client.replyToDiary(fid, id, {
          ...value,
          idempotencyKey: key,
        });
        if (!result?.request?.id || result.request.fluctlight_id !== fid)
          throw Error("留言回执尚未完整读到，原身份仍保留。");
        return result;
      },
      readLatest: async () => check(await w.client.diary(fid, id), id),
      adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
      onSaved: async (result) => {
        if (!live()) return;
        setReplyPointer({ diaryId: id, request: result.request });
        await readReplies(id);
      },
      fields: (d, edit) =>
        field(
          "留言内容",
          <Textarea
            aria-label="日记留言"
            value={d.text}
            onChange={(e) => edit({ text: e.target.value })}
          />,
        ),
    });
  }
  async function findDay(date, zone) {
    let cursor = "",
      seen = new Set();
    do {
      const result = await w.client.listDiary(fid, cursor, { limit: 100 });
      if (!live()) throw Error("日记范围已切换。");
      const row = result.items.find(
        (row) => row.date === date && row.timezone === zone,
      );
      if (row) return check(await w.client.diary(fid, row.id), row.id);
      cursor = result.next_cursor || "";
      if (cursor && seen.has(cursor))
        throw Error("日记分页重复，请先核对已有目录。");
      seen.add(cursor);
    } while (cursor);
    throw Error("还没有读到该日期的原日记，请重新读取目录核对。");
  }
  function generate(event, entry = null) {
    if (!live() || readonly || ["paused", "已暂停"].includes(c.status)) return;
    trigger.current = event.currentTarget;
    const zone = entry?.timezone || c.timezone || "Asia/Shanghai";
    setAction({
      identity: "diary-generation:" + fid + ":" + (entry?.id || "new"),
      scope: ["diary-generation", fid, entry?.id || "new"],
      storageKey:
        "fluctlight-diary:" +
        owner +
        ":" +
        fid +
        ":generate:" +
        (entry?.id || "new"),
      initial: {
        date: entry?.date || previousDiaryDate(day(new Date(), zone)),
        zone,
        revision: entry?.revision ?? null,
      },
      title: "整理过去一天的日记",
      description: "只回顾已经结束的一天；已有正文会保留。",
      submitLabel: "请求整理日记",
      preAdmissionConflicts: ["diary_revision_conflict"],
      validate: (d) => {
        if (
          !/^\d{4}-\d{2}-\d{2}$/.test(d.date) ||
          d.date > previousDiaryDate(day(new Date(), d.zone))
        )
          throw Error("请选择该时区已经结束的一天。");
      },
      build: (d) => ({
        localDate: d.date,
        timezone: d.zone,
        reason: "回顾所选已结束日期的真实经历",
        ...(Number.isSafeInteger(d.revision)
          ? { expectedRevision: d.revision }
          : {}),
      }),
      execute: async (value, key) => {
        if (
          !w.isCurrent(owner, fid) ||
          [
            "paused",
            "已暂停",
            "retired",
            "archived",
            "已退休",
            "已归档",
          ].includes(w.current.value.status)
        )
          throw Error("当前角色不能开始日记整理。");
        const result = await w.client.generateDiary(fid, {
          ...value,
          idempotencyKey: key,
        });
        if (
          !result?.diary_id ||
          result.fluctlight_id !== fid ||
          !statuses[result.status]
        )
          throw Error("日记回执尚未完整读到，原提交仍保留。");
        return result;
      },
      readLatest: (d) => findDay(d.date, d.zone),
      adoptLatest: (d, row) => {
        if (row.date !== d.date || row.timezone !== d.zone)
          throw Error("读回的日期或时区不匹配。");
        return { ...d, revision: row.revision };
      },
      onSaved: async (result, value) => {
        const row = check(
          await w.client.diary(fid, result.diary_id),
          result.diary_id,
        );
        if (row.date !== value.localDate || row.timezone !== value.timezone)
          throw Error("日记日期或时区尚未完整读回。");
        if (!live()) return;
        adopt(row);
        pinned.current = row.id;
        setGenerationId(row.id);
        sessionStorage.setItem(
          "fluctlight-diary:" + owner + ":" + fid + ":generation",
          JSON.stringify({ id: row.id }),
        );
        await loadList("all");
        setFavorite(false);
        setTab("diary");
        setSelectedId(row.id);
        setDetail(row);
        setNotice("整理请求已读回，正文以实际结果为准。");
      },
      fields: (d, edit) => (
        <>
          {field(
            "选择过去的日期",
            <FieldInput
              type="date"
              aria-label="日记整理日期"
              value={d.date}
              onChange={(e) => edit({ date: e.target.value, revision: null })}
            />,
          )}
          {field(
            "日期所属时区",
            <FieldInput
              aria-label="日记整理时区"
              value={d.zone}
              onChange={(e) => edit({ zone: e.target.value, revision: null })}
            />,
          )}
          <WorkspaceInfo>
            已经写下或未形成正文的原记录会保留；失败或未运行的日记需要核对原版本。
          </WorkspaceInfo>
        </>
      ),
    });
  }
  return (
    <div className="replica-life replica-diary">
      <WorkspacePageHeader
        eyebrow="LIFE / LITTLE PAGES"
        title="日记"
        description="那些没有说出口的日常，被认真地写在这里。"
        actions={
          <>
            <Button
              variant="outline"
              disabled={readonly || ["paused", "已暂停"].includes(c.status)}
              onClick={(e) => generate(e)}
            >
              <SparklesIcon size={15} />
              整理一篇日记
            </Button>
          </>
        }
      />
      <Tabs value={tab} onValueChange={setTab} className="lp-stack">
        <div className="lp-toolbar">
          <TabsList variant="line" className="replica-segment-tabs">
            <TabsTrigger value="diary">
              {c.name}的日记{" "}
              <span>
                {lists.all.ready
                  ? lists.all.items.length + (lists.all.cursor ? "+" : "")
                  : "…"}
              </span>
            </TabsTrigger>
            <TabsTrigger value="shared">共同经历</TabsTrigger>
          </TabsList>
          {tab === "shared" && (
            <>
              <Button
                variant="ghost"
                size="icon"
                aria-label="刷新日记"
                disabled={loading[tab === "shared" ? "shared" : kind]}
                onClick={() => {
                  void loadList(tab === "shared" ? "shared" : kind);
                  if (selectedId) void readEntry().catch(() => {});
                  setWatchRefresh((v) => v + 1);
                }}
              >
                <RefreshCwIcon size={15} />
              </Button>
            </>
          )}
          {tab === "diary" && (
            <div className="replica-diary-controls">
              <div className="replica-diary-options">
                <label className="lp-row">
                  <Checkbox
                    checked={favorite}
                    onCheckedChange={(v) => {
                      pinned.current = "";
                      setFavorite(!!v);
                    }}
                  />
                  <StarIcon size={13} />
                  只看收藏
                </label>
              </div>
              <div className="replica-diary-search">
                <SearchIcon size={15} />
                <Input
                  aria-label="搜索日记"
                  placeholder="在日记中寻找…"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />{" "}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon" aria-label="日记筛选" className="replica-diary-filter-menu">
                      <StarIcon size={14} fill={favorite?'currentColor':'none'}/>
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuCheckboxItem checked={favorite} onCheckedChange={value=>{pinned.current='';setFavorite(!!value);}}>
                      只看收藏
                    </DropdownMenuCheckboxItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="刷新日记"
                  disabled={loading[tab === "shared" ? "shared" : kind]}
                  onClick={() => {
                    void loadList(tab === "shared" ? "shared" : kind);
                    if (selectedId) void readEntry().catch(() => {});
                    setWatchRefresh((v) => v + 1);
                  }}
                >
                  <RefreshCwIcon size={15} />
                </Button>
              </div>
            </div>
          )}
        </div>
        {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
        {watchNote && <WorkspaceInfo>{watchNote}</WorkspaceInfo>}
        <TabsContent value="diary">
          {loading[kind] && <LoadingState label="正在读取日记…" />}
          {errors[kind] && (
            <InlineFeedback role="alert">
              {errors[kind]}
              <Button variant="outline" onClick={() => retry.current[kind]?.()}>
                重新读取日记目录
              </Button>
            </InlineFeedback>
          )}
          <div className="lp-diary-layout">
            <aside className="lp-diary-index" aria-label="日记目录">
              {visible.map((row) => (
                <Button
                  variant="ghost"
                  key={row.id}
                  className={
                    "lp-diary-date" + (row.id === selectedId ? " active" : "")
                  }
                  onClick={() => {
                    pinned.current = "";
                    setSelectedId(row.id);
                  }}
                >
                  <strong>{row.date}</strong>
                  <span>
                    {row.body?.trim().split("\n")[0] || statuses[row.status]}
                  </span>
                </Button>
              ))}
              {page.ready && !visible.length && !errors[kind] && (
                <p className="lp-empty-inline">没有找到这段回忆。</p>
              )}
              {page.cursor && (
                <Button
                  variant="outline"
                  disabled={loading[kind]}
                  onClick={() => void loadList(kind, true)}
                >
                  加载更早日记
                </Button>
              )}
            </aside>
            {selected ? (
              <article className="lp-paper" aria-label="日记正文">
                {detailBusy && <LoadingState label="正在读取这篇日记…" />}
                {detailError && (
                  <InlineFeedback role="alert">
                    {detailError}
                    <Button
                      variant="outline"
                      onClick={() => void readEntry().catch(() => {})}
                    >
                      重新读取这篇日记
                    </Button>
                  </InlineFeedback>
                )}
                <div className="lp-row lp-between">
                  <span className="lp-day-stamp">
                    {selected.date.replaceAll("-", " / ")}
                  </span>
                  <Badge variant="outline">{statuses[selected.status]}</Badge>
                </div>
                <h2>{selected.date}的日记</h2>
                {typeof selected.body === "string" && selected.body.trim() ? (
                  <div className="lp-paper-body">{selected.body}</div>
                ) : (
                  <WorkspaceInfo>
                    {selected.status === "no_content"
                      ? "这次没有足够或适合的素材形成日记。"
                      : "尚无已提交正文，受理不代表已经生成。"}
                  </WorkspaceInfo>
                )}
                <div className="lp-paper-footer">
                  <div className="lp-row lp-between">
                    <span className="lp-small">{c.name}的主观记录</span>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={readonly}
                      onClick={noteEditor}
                    >
                      <Edit3Icon size={13} />
                      {selected.annotation.note ? "修改批注" : "写一点批注"}
                    </Button>
                  </div>
                  {selected.annotation.favorite && (
                    <p className="lp-muted">已收藏</p>
                  )}
                  {selected.annotation.note &&
                    action?.presentation !== "inline" && (
                      <div className="lp-annotation">
                        <span className="lp-label">你的批注</span>
                        <p>{selected.annotation.note}</p>
                      </div>
                    )}
                  {action?.presentation === "inline" &&
                    action.diaryId === selected.id && (
                      <WorkspaceCommandDialog
                        key={action.identity}
                        action={action}
                        onClose={() => setAction(null)}
                        returnFocus={() => trigger.current}
                      />
                    )}
                  <div className="lp-action-row">
                    <Button
                      disabled={readonly || !selected.active_attempt_id}
                      variant="outline"
                      onClick={replyEditor}
                    >
                      读完后，想对{c.name}说
                    </Button>
                    {["failed", "inactive"].includes(selected.status) && (
                      <Button
                        variant="outline"
                        disabled={readonly}
                        onClick={(e) => generate(e, selected)}
                      >
                        核对版本后重新整理
                      </Button>
                    )}
                  </div>
                  {replyPointer?.diaryId === selected.id && (
                    <RequestStatus
                      key={replyPointer.request.id}
                      fid={fid}
                      requestId={replyPointer.request.id}
                      initial={replyPointer.request}
                      onTerminal={() => void readReplies(selected.id)}
                    />
                  )}
                  <Disclosure title="留言与实际处理记录">
                    <Button
                      variant="link"
                      disabled={replyBusy}
                      onClick={() => void readReplies(selected.id)}
                    >
                      刷新留言记录
                    </Button>
                    {replyBusy && <LoadingState label="正在读取留言请求…" />}
                    {replyError && (
                      <InlineFeedback role="alert">
                        {replyError}
                        <Button
                          variant="outline"
                          onClick={() => retry.current.replies?.()}
                        >
                          重新读取留言记录
                        </Button>
                      </InlineFeedback>
                    )}
                    {replies.items.map((row) => (
                      <div className="lp-source-card" key={row.id}>
                        <span className="lp-muted">
                          你 · {userInstant(row.created_at)}
                        </span>
                        <p>{row.payload?.text || "留言原文待读取"}</p>
                        <JsonDetails value={row} title="原留言请求与处理结果" />
                        <Button
                          variant="link"
                          onClick={() =>
                            setReplyPointer({
                              diaryId: selected.id,
                              request: row,
                            })
                          }
                        >
                          跟进这条留言
                        </Button>
                      </div>
                    ))}
                    {replies.ready && !replies.items.length && !replyError && (
                      <p className="lp-muted">还没有读到这篇日记的留言请求。</p>
                    )}
                    {replies.cursor && (
                      <Button
                        variant="outline"
                        disabled={replyBusy}
                        onClick={() => void readReplies(selected.id, true)}
                      >
                        加载更多留言
                      </Button>
                    )}
                    <p className="lp-muted">
                      处理请求不代表已经产生回复，交流内容以实际会话记录为准。
                    </p>
                  </Disclosure>
                  <Disclosure title="生成尝试、素材与来源">
                    <JsonDetails value={selected} title="完整日记与来源" />
                    {selected.attempts?.map((attempt) => (
                      <div className="lp-source-card" key={attempt.id}>
                        <p>
                          尝试 {attempt.attempt} ·{" "}
                          {statuses[attempt.status] || attempt.status}
                        </p>
                        {attempt.task_id && (
                          <Button variant="link" asChild>
                            <a
                              href={
                                "#/diag/task/" +
                                encodeURIComponent(attempt.task_id)
                              }
                            >
                              查看原任务
                            </a>
                          </Button>
                        )}
                      </div>
                    ))}
                    {selected.has_more_attempts && (
                      <p className="lp-muted">
                        当前显示最近20次尝试，更早记录保留在实际任务诊断。
                      </p>
                    )}
                  </Disclosure>
                </div>
              </article>
            ) : page.ready && !loading[kind] && !errors[kind] ? (
              <EmptyState>
                <h3>日记本还是空的</h3>
                <p>真实经历会在一天结束后被回顾。</p>
              </EmptyState>
            ) : null}
          </div>
        </TabsContent>
        <TabsContent value="shared">
          {loading.shared && <LoadingState label="正在读取共同经历…" />}
          {errors.shared && (
            <InlineFeedback role="alert">
              {errors.shared}
              <Button
                variant="outline"
                onClick={() => retry.current.shared?.()}
              >
                重新读取共同经历
              </Button>
            </InlineFeedback>
          )}
          <div className="lp-two">
            {lists.shared.items.map((row) => (
              <Surface className="lp-pad" key={row.id}>
                <span className="lp-day-stamp">
                  {userInstant(row.occurred_at)}
                </span>
                <h3 className="lp-title">{row.title}</h3>
                <p className="lp-muted">远程交流记录 · 不代表已经同场经历</p>
                <Button
                  variant="ghost"
                  onClick={() =>
                    w.navigatePath(
                      "/c/" +
                        encodeURIComponent(fid) +
                        "/chat?" +
                        new URLSearchParams({
                          conversation: row.conversation_id,
                          message: row.id,
                        }),
                    )
                  }
                >
                  查看原交流
                  <ArrowUpRightIcon size={13} />
                </Button>
              </Surface>
            ))}
          </div>
          {lists.shared.ready &&
            !lists.shared.items.length &&
            !errors.shared && (
              <EmptyState>
                <h3>共同的经历，慢慢积累</h3>
                <p>新的相处会留下有来源的交流记录。</p>
              </EmptyState>
            )}
          {lists.shared.cursor && (
            <Button
              variant="outline"
              disabled={loading.shared}
              onClick={() => void loadList("shared", true)}
            >
              加载更多共同经历
            </Button>
          )}
        </TabsContent>
      </Tabs>
      {action && action.presentation !== "inline" && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => trigger.current}
        />
      )}
    </div>
  );
}
