import { DetailSheet } from '@/components/patterns';
import { useIsMobile } from '../hooks/use-mobile';
import {ResultFeedback} from '../shared/Presentation';
import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRightIcon,
  CheckCircle2Icon,
  ChevronRightIcon,
  Edit3Icon,
  LeafIcon,
  RefreshCwIcon,
  SearchIcon,
  StarIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Surface,
  ChoiceSelect,
  FieldInput,
  Textarea,
  Badge,
  Disclosure,
} from "../controls";
import { useWorkspace } from "../page";
import { jsonValueSnapshot } from "@/state/view-model";
import {
  labelFor, memoryFacts, memoryRevisionFacts, memorySourceFacts, memoryStatuses,
  memoryTypes, recordText, recordTime, scoreText, unitScore as number,
} from "@/approved-ui/memory-record-summary.mjs";
import {
  ClaimSummary, claimDownload, claimStatuses, memoryDownload, RecordDownload,
  ReflectionChanges, ReflectionFeedbackSummary, ReflectionObservations,
  reflectionDownload, reflectionStatus, reflectionTitle, SummaryFacts,
} from "./MemoryRecordSummary";
import {
  canRestoreSelfRevision,
  selfCategoryLabels,
} from "@/v3/proto/components/self-claims.js";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceCommandDialog from "../shared/WorkspaceCommandDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import RequestStatus from "../shared/RequestStatus";
import LoadingState from "../shared/LoadingState";
import useWorkspaceRead from "../shared/useWorkspaceRead";
import { EmptyState, InlineFeedback,ReaderFeedback } from "../shared/Presentation";
function routeState(query = new URLSearchParams()) {
  const tab = ["memory", "reflection", "self"].includes(query.get("tab"))
      ? query.get("tab")
      : "memory",
    status = query.get("status"),
    q = query.get("q") || "";
  return {
    tab,
    q,
    filters: {
      q,
      type: Object.hasOwn(memoryTypes, query.get("type"))
        ? query.get("type")
        : "",
      status: Object.hasOwn(memoryStatuses, status)
        ? status
        : status === "all"
          ? ""
          : "active",
      aboutOwner: query.get("aboutOwner") === "true",
    },
    claimStatus: [
      "active",
      "uncertain",
      "forgotten",
      "superseded",
      "all",
    ].includes(status)
      ? status
      : "active",
  };
}
const field = (label, control) => (
  <label className="replica-field lp-wide">
    <span>{label}</span>
    {control}
  </label>
);
function Rating({ value }) {
  const score = number(value);
  return score === null ? null : (
    <span
      className="lp-stars"
      role="img"
      aria-label={"重要度 " + Math.round(score * 100) + "%"}
      title={"重要度 " + score + "（0–1）"}
    >
      {[1, 2, 3, 4, 5].map((index) => (
        <StarIcon
          size={11}
          key={index}
          fill={index <= Math.round(score * 5) ? "currentColor" : "none"}
        />
      ))}
    </span>
  );
}
function ReadStatus({source,label}){return <>{source.busy&&!source.ready&&<LoadingState label={'正在读取'+label+'…'}/>}<ReaderFeedback source={source} label={label}/></>;}
function More({ source, label }) {
  return source.cursor ? (
    <Button variant="outline" disabled={source.busy} onClick={source.more}>
      加载更多{label}
    </Button>
  ) : null;
}
export default function MemoryPage({ route }) {
  const mobile = useIsMobile(), [filterOpen,setFilterOpen] = useState(false);
  const w = useWorkspace(),
    c = w.current.value,
    fid = c.id,
    owner = w.state.session.actorId,
    scope = JSON.stringify([owner, fid]);
  const initial = routeState(route?.query);
  const [tab, setTab] = useState(initial.tab),
    [query, setQuery] = useState(initial.q),
    [filters, setFilters] = useState(initial.filters),
    [filterError, setFilterError] = useState(""),
    [claimStatus, setClaimStatus] = useState(initial.claimStatus),
    [selection, setSelection] = useState(null),
    [detailTab, setDetailTab] = useState("content"),
    [sourceRevision, setSourceRevision] = useState(null),
    [action, setAction] = useState(null),
    [notice, setNotice] = useState(null),
    [feedbackPointer, setFeedbackPointer] = useState(null);
  const routeKey = route?.query?.toString() || "";
  useEffect(() => {
    const next = routeState(route?.query);
    setTab(next.tab);
    setQuery(next.q);
    setFilters(next.filters);
    setClaimStatus(next.claimStatus);
    setSelection(null);
    setAction(null);
  }, [routeKey, scope]);
  const trigger = useRef(null),
    commandTrigger = useRef(null),
    mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, [scope]);
  const live = () => mounted.current && !!fid && w.isCurrent(owner, fid),
    readonly = () =>
      ["retired", "archived", "已退休", "已归档"].includes(
        w.current.value.status,
      );
  function memoryCheck(row, id) {
    if (!row?.id || row.owner_fluctlight_id !== fid || (id && row.id !== id))
      throw Error("记忆范围不匹配，未采用这次结果。");
    return row;
  }
  function growthCheck(row, id) {
    if (!row?.id || row.fluctlight_id !== fid || (id && row.id !== id))
      throw Error("成长记录范围不匹配，未采用这次结果。");
    return row;
  }
  const readMemory = async (id) =>
      memoryCheck(await w.client.managedMemory(id), id),
    readReflection = async (id) =>
      growthCheck(await w.client.reflection(fid, id), id),
    readClaim = async (id) =>
      growthCheck(await w.client.selfClaim(fid, id), id);
  const memories = useWorkspaceRead({
    identity: ["memories", filters],
    read: async (cursor) => {
      const page = await w.client.managedMemories(fid, {
        ...filters,
        cursor,
        limit: 30,
      });
      return { ...page, items: page.items.map((row) => memoryCheck(row)) };
    },
  });
  const reflections = useWorkspaceRead({
    identity: ["reflections"],
    read: async (cursor) => {
      const page = await w.client.listReflections(fid, { cursor, limit: 30 });
      return { ...page, items: page.items.map((row) => growthCheck(row)) };
    },
  });
  const claims = useWorkspaceRead({
    identity: ["self", claimStatus],
    read: async (cursor) => {
      const page = await w.client.selfClaims(fid, {
        status: claimStatus,
        cursor,
        limit: 30,
      });
      return { ...page, items: page.items.map((row) => growthCheck(row)) };
    },
  });
  const detail = useWorkspaceRead({
    identity: ["growth-detail", selection?.kind, selection?.id],
    enabled: !!selection,
    read: async () => ({
      items: [
        await (selection.kind === "memory"
          ? readMemory(selection.id)
          : selection.kind === "reflection"
            ? readReflection(selection.id)
            : readClaim(selection.id)),
      ],
      next_cursor: "",
    }),
  });
  const sources = useWorkspaceRead({
    identity: ["memory-sources", selection?.id, sourceRevision],
    enabled:
      selection?.kind === "memory" && Number.isSafeInteger(sourceRevision),
    keyOf: (row) => row.source_ref,
    read: async (cursor) => {
      await readMemory(selection.id);
      return w.client.memorySources(selection.id, {
        cursor,
        revision: sourceRevision,
        limit: 20,
      });
    },
  });
  const revisions = useWorkspaceRead({
    identity: ["growth-revisions", selection?.kind, selection?.id],
    enabled: !!selection && selection.kind !== "reflection",
    keyOf: (row) => row.revision,
    read: async (cursor) => {
      if (selection.kind === "memory") {
        await readMemory(selection.id);
        return w.client.memoryRevisions(selection.id, { cursor, limit: 20 });
      }
      return w.client.selfClaimHistory(fid, selection.id, "revisions", cursor);
    },
  });
  const reviews = useWorkspaceRead({
    identity: ["self-reviews", selection?.id],
    enabled: selection?.kind === "self",
    read: (cursor) =>
      w.client.selfClaimHistory(fid, selection.id, "reviews", cursor),
  });
  const feedback = useWorkspaceRead({
    identity: ["reflection-feedback", selection?.id],
    enabled: selection?.kind === "reflection",
    read: async (cursor) => {
      const page = await w.client.reflectionFeedback(fid, selection.id, {
        cursor,
        limit: 20,
      });
      if (
        page.items.some(
          (row) =>
            row.fluctlight_id !== fid ||
            row.payload?.reflection_id !== selection.id,
        )
      )
        throw Error("反馈记录范围不匹配。");
      return page;
    },
  });
  const selected = detail.items[0] || selection?.seed;
  function open(kind, row, event) {
    trigger.current = event.currentTarget;
    setDetailTab("content");
    setSourceRevision(kind === "memory" ? row.revision : null);
    setSelection({ kind, id: row.id, seed: jsonValueSnapshot(row) });
  }
  function close() {
    setSelection(null);
    setFeedbackPointer(null);
  }
  function apply(patch = {}) {
    if (query.trim().length > 500) {
      setFilterError("搜索内容最多500个字符。");
      return;
    }
    setFilterError("");
    setFilters((old) => ({ ...old, q: query.trim(), ...patch }));
  }
  async function refreshSelection(confirmed = null) {
    if (confirmed) detail.adopt({ items: [confirmed], next_cursor: "" });
    else await detail.load();
    if (selection?.kind === "memory") {
      await memories.load();
      await revisions.load();
      await sources.load();
    } else if (selection?.kind === "reflection") {
      await reflections.load();
      await feedback.load();
      if (claims.ready) await claims.load();
    } else {
      await claims.load();
      await revisions.load();
      await reviews.load();
    }
  }
  function memoryCommand(operation, row, event, target = null) {
    if (!live() || readonly()) return;
    commandTrigger.current = event.currentTarget;
    const id = row.id,
      labels = {
        revise: "纠正这段记忆",
        forget: "不再把这段记忆作为当前认识？",
        rollback: "补偿恢复这段历史记忆",
      };
    setAction({
      identity:
        "memory:" + id + ":" + operation + ":" + (target?.revision ?? ""),
      scope: ["memory", fid, id, operation, target?.revision],
      storageKey:
        "fluctlight-memory:" +
        owner +
        ":" +
        fid +
        ":" +
        id +
        ":" +
        operation +
        ":" +
        (target?.revision ?? ""),
      casOnly: true,
      wide: true,
      title: labels[operation],
      submitLabel:
        operation === "revise"
          ? "保存纠正"
          : operation === "forget"
            ? "确认遗忘"
            : "确认补偿恢复",
      description:
        operation === "revise"
          ? "更正正文并追加修订；旧版及原消息仍可查阅。"
          : "追加治理修订，保留历史；相关领域的历史效果以实际记录为准。",
      pendingSummary: (value) => <SummaryFacts facts={[
        ["修改后的正文", value.content],
        ["修订说明", value.evidenceRefs?.[0]?.slice(("owner:" + owner + ":").length)],
        ["恢复内容", target?.content],
      ]} />,
      reviewSummary: (current) => <><p>{current.content}</p><SummaryFacts facts={memoryFacts(current)} /></>,
      initial: {
        content: row.content,
        baseline: row.content,
        revision: row.revision,
        status: row.status,
        reason: "",
      },
      validate: (d) => {
        if (!d.reason.trim() || d.reason.trim().length > 150)
          throw Error("请填写操作说明，最多150个字符。");
        const ref = "owner:" + owner + ":" + d.reason.trim();
        if ([...ref].length > 256)
          throw Error("说明与身份组合超过来源引用长度，请缩短说明。");
        if (operation !== "rollback" && d.status !== "active")
          throw Error("当前记忆已不是有效状态，请核对历史。");
        if (
          operation === "revise" &&
          (!d.content.trim() || d.content.trim().length > 4096)
        )
          throw Error("正文不能为空，最多4096个字符。");
        if (operation === "revise" && d.content.trim() === d.baseline)
          throw Error("当前版本已经是这段正文，请关闭编辑或继续核对。");
      },
      build: (d) => ({
        id,
        expectedRevision: d.revision,
        evidenceRefs: ["owner:" + owner + ":" + d.reason.trim()],
        ...(operation === "revise" ? { content: d.content.trim() } : {}),
        ...(operation === "rollback"
          ? { targetRevision: target.revision }
          : {}),
      }),
      execute: async (value) => {
        await readMemory(id);
        if (!live() || readonly()) throw Error("角色范围或可写状态已经变化。");
        const { id: _, ...body } = value;
        const result = await (operation === "revise"
          ? w.client.reviseMemory(id, body)
          : operation === "forget"
            ? w.client.forgetMemory(id, body)
            : w.client.rollbackMemory(id, body));
        if (
          result.id !== id ||
          result.operation !== operation ||
          !Number.isSafeInteger(result.revision)
        )
          throw Error("记忆回执尚未完整读到，请核对当前版本。");
        return result;
      },
      readLatest: () => readMemory(id),
      adoptLatest: (d, current) => {
        if (live()) {
          setSourceRevision(current.revision);
          void refreshSelection(current);
        }
        return {
          ...d,
          revision: current.revision,
          status: current.status,
          baseline: current.content,
        };
      },
      onSaved: async (result, value) => {
        const current = await readMemory(id);
        if (
          current.revision < result.revision ||
          (current.revision === result.revision &&
            ((operation === "revise" && current.content !== value.content) ||
              (operation === "forget" && current.status !== "forgotten") ||
              (operation === "rollback" && current.content !== target.content)))
        )
          throw Error("操作已有回执，当前记忆尚未完整读回。");
        if (live()) {
          setSourceRevision(current.revision);
          setNotice({kind:'success',text:"记忆治理结果已读回，旧版和来源仍可查阅。"});
          setDetailTab("history");
          await refreshSelection(current);
        }
      },
      fields: (d, edit) => (
        <>
          {operation === "rollback" && <p>{target.content}</p>}
          {operation === "revise" &&
            field(
              "修改后的正文",
              <Textarea
                aria-label="记忆修正正文"
                value={d.content}
                onChange={(e) => edit({ content: e.target.value })}
                rows={5}
              />,
            )}
          {field(
            "修订说明",
            <FieldInput
              aria-label="记忆操作说明"
              value={d.reason}
              onChange={(e) => edit({ reason: e.target.value })}
              placeholder="哪里需要调整，为什么？"
            />,
          )}
          <WorkspaceInfo>
            本次说明会作为更正依据。原始来源可在修订记录中查阅。
          </WorkspaceInfo>
        </>
      ),
    });
  }
  function selfCommand(operation, row, event, target = null) {
    if (!live() || readonly()) return;
    commandTrigger.current = event.currentTarget;
    const id = row.id;
    setAction({
      identity: "self:" + id + ":" + operation + ":" + (target?.revision ?? ""),
      scope: ["self", fid, id, operation, target?.revision],
      storageKey:
        "fluctlight-memory:" +
        owner +
        ":" +
        fid +
        ":self:" +
        id +
        ":" +
        operation +
        ":" +
        (target?.revision ?? ""),
      pendingSummary: (value) => <SummaryFacts facts={[
        ["操作说明", value.reason],
        ["恢复内容", (target?.after_value?.category ? target.after_value : target?.candidate)?.claim],
      ]} />,
      reviewSummary: (current) => <ClaimSummary record={current} />,
      initial: { revision: row.revision, reason: "" },
      title: {
        review: "确认已核对这条认识",
        forget: "不再采用这条认识",
        restore: "恢复历史版本内容",
      }[operation],
      description:
        operation === "review"
          ? "记录你已阅读当前认识，证据、置信度和人格照常保留。"
          : "新增治理修订。恢复仍需校验历史来源、有效期限和谱系。",
      submitLabel: {
        review: "确认已核对",
        forget: "确认不再采用",
        restore: "确认历史恢复",
      }[operation],
      preAdmissionConflicts: [
        "self_claim_revision_conflict",
        "self_snapshot_source_changed",
      ],
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason.trim()].length > 1024)
          throw Error("请填写操作说明，最多1024个字符。");
      },
      build: (d) => ({
        expectedRevision: d.revision,
        reason: d.reason.trim(),
        ...(operation === "restore" ? { targetRevision: target.revision } : {}),
      }),
      execute: async (value, key) => {
        if (!live() || readonly()) throw Error("当前角色不可写入。");
        const result = await w.client.manageSelfClaim(fid, id, operation, {
          ...value,
          idempotencyKey: key,
        });
        if (
          result.id !== id ||
          result.fluctlight_id !== fid ||
          result.operation !== operation ||
          !Number.isSafeInteger(result.revision)
        )
          throw Error("认识回执范围尚未完整确认，原提交仍保留。");
        return result;
      },
      readLatest: () => readClaim(id),
      adoptLatest: (d, current) => {
        if (live()) void refreshSelection(current);
        return { ...d, revision: current.revision };
      },
      onSaved: async (result) => {
        const current = await readClaim(id);
        if (
          current.revision < result.revision ||
          (operation !== "review" &&
            current.revision === result.revision &&
            current.status !== result.status) ||
          (operation === "review" &&
            current.revision === result.revision &&
            (!current.owner_review ||
              current.owner_review.claim_revision !== result.revision ||
              current.owner_review.fluctlight_id !== fid ||
              current.owner_review.claim_id !== id ||
              current.owner_review.owner_actor_id !== owner))
        )
          throw Error("操作已有回执，当前认识或复核记录尚未读回。");
        if (live()) {
          setNotice({kind:'success',text:"认识治理结果已读回，实际状态和来源如下。"});
          await refreshSelection(current);
        }
      },
      fields: (d, edit) =>
        field(
          "操作说明",
          <Textarea
            aria-label="认识操作说明"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
          />,
        ),
    });
  }
  function reflectionFeedback(stance, row, event) {
    if (!live() || readonly()) return;
    commandTrigger.current = event.currentTarget;
    const id = row.id;
    setAction({
      identity: "reflection:" + id + ":" + stance,
      scope: ["reflection-feedback", fid, id, stance],
      storageKey:
        "fluctlight-memory:" +
        owner +
        ":" +
        fid +
        ":reflection:" +
        id +
        ":" +
        stance,
      pendingSummary: (value) => <ReflectionFeedbackSummary record={{payload:value}} />,
      reviewSummary: (current) => <ReflectionObservations record={current} />,
      initial: { text: "" },
      title: stance === "agree" ? "赞同这次观察" : "说说不同的看法",
      description: "提交真实反馈请求，是否产生改变以之后的候选和应用结果为准。",
      submitLabel: "提交反馈",
      validate: (d) => {
        if (!d.text.trim() || [...d.text.trim()].length > 2000)
          throw Error("请填写反馈，最多2000个字符。");
      },
      build: (d) => ({ text: d.text.trim(), stance }),
      execute: async (value, key) => {
        await readReflection(id);
        if (!live() || readonly()) throw Error("当前角色不可提交反馈。");
        const result = await w.client.submitReflectionFeedback(fid, id, {
          ...value,
          idempotencyKey: key,
        });
        if (
          !result.request?.id ||
          result.request.fluctlight_id !== fid ||
          result.request.kind !== "reflection_feedback" ||
          result.request.payload?.reflection_id !== id
        )
          throw Error("反馈回执范围尚未确认，原身份已保留。");
        return result;
      },
      readLatest: () => readReflection(id),
      adoptLatest: (d) => d,
      onSaved: async (result) => {
        if (live()) {
          setFeedbackPointer({ reflectionId: id, request: result.request });
          setNotice({kind:'information',text:"反馈请求已受理，实际改变以处理结果为准。"});
          await feedback.load();
        }
      },
      fields: (d, edit) =>
        field(
          "你的反馈",
          <Textarea
            aria-label="反思反馈内容"
            value={d.text}
            onChange={(e) => edit({ text: e.target.value })}
          />,
        ),
    });
  }
  const filterControls = (            <div className="lp-row">
              <ChoiceSelect
                aria-label="记忆类型"
                value={filters.type}
                onChange={(e) => apply({ type: e.target.value })}
              >
                <option value="">所有类型</option>
                {Object.entries(memoryTypes).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </ChoiceSelect>
              <ChoiceSelect
                aria-label="记忆状态"
                value={filters.status}
                onChange={(e) => apply({ status: e.target.value })}
              >
                <option value="">全部状态</option>
                {Object.entries(memoryStatuses).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </ChoiceSelect>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label="刷新记忆"
                disabled={memories.busy}
                onClick={memories.load}
              >
                <RefreshCwIcon size={15} />
              </Button>
            </div>);
  return (
    <div className="replica-life replica-memory">
      <WorkspacePageHeader
        eyebrow="GROWTH / WHAT STAYS WITH US"
        title="记忆与成长"
        description="相处留下的细节，逐渐成为她理解世界的方式。"
        actions={
          <Button
            variant="outline"
            onClick={() =>
              w.navigatePath("/c/" + encodeURIComponent(fid) + "/people/owner")
            }
          >
            关于你
            <ArrowUpRightIcon size={14} />
          </Button>
        }
      />
      <Tabs value={tab} onValueChange={setTab} className="lp-stack">
        <TabsList variant="line" className="replica-segment-tabs">
          <TabsTrigger value="memory">
            记忆{" "}
            <span>
              {memories.ready
                ? memories.items.length + (memories.cursor ? "+" : "")
                : "…"}
            </span>
          </TabsTrigger>
          <TabsTrigger value="reflection">反思记录</TabsTrigger>
          <TabsTrigger value="self">自我认知</TabsTrigger>
        </TabsList>
        <ResultFeedback notice={notice} onDismiss={value=>setNotice(current=>current===value?null:current)}/>
        <TabsContent value="memory">
          <form
            className="lp-toolbar"
            onSubmit={(e) => {
              e.preventDefault();
              apply();
            }}
          >
            <div className="replica-memory-search">
              <Button type="submit" variant="ghost" size="icon" aria-label="应用记忆搜索"><SearchIcon size={15}/></Button>
              <Input
                aria-label="搜索记忆正文"
                placeholder="搜索记忆正文…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            {!mobile && filterControls}
            {mobile && <Button type="button" variant="outline" onClick={() => setFilterOpen(true)}>筛选</Button>}
            <DetailSheet mobileMode="sheet" open={mobile && filterOpen} onOpenChange={setFilterOpen} title="筛选记忆" footer={<Button onClick={() => setFilterOpen(false)}>完成</Button>}>{filterControls}</DetailSheet>
          </form>
          {filterError && (
            <InlineFeedback role="alert">{filterError}</InlineFeedback>
          )}
          {filters.aboutOwner && (
            <WorkspaceInfo>
              当前读取关于你的有效记忆。
              <Button
                variant="link"
                onClick={() =>
                  setFilters((old) => ({ ...old, aboutOwner: false }))
                }
              >
                返回全部角色记忆
              </Button>
            </WorkspaceInfo>
          )}
          <ReadStatus source={memories} label="记忆" />
          <div className="lp-grid">
            <div className="lp-stack">
              {memories.items.map((row) => (
                <Button
                  variant="ghost"
                  key={row.id}
                  className="lp-memory-card"
                  aria-label={"查看记忆 " + row.content}
                  onClick={(e) => open("memory", row, e)}
                >
                  <div className="lp-row lp-between">
                    <span className="lp-chip">
                      {labelFor(memoryTypes, row.type)}
                    </span>
                    {recordTime(row.created_at) && <time className="lp-muted" title={recordTime(row.created_at)}>{recordTime(row.created_at).split(" ")[0]}</time>}
                  </div>
                  <p>{row.content}</p>
                  <div className="lp-row lp-between">
                    <div className="lp-tag-row" title={labelFor(memoryStatuses, row.status) || undefined}>
                      {(row.tags||[]).map(recordText).filter(Boolean).map(tag=><span className="lp-muted" key={tag}>#{tag}</span>)}
                      {row.status!=='active'&&<span className="lp-muted">{labelFor(memoryStatuses, row.status)}</span>}
                      {["invalid", "pending"].includes(
                        row.provenance_status,
                      ) && <span className="lp-muted">来源待核对</span>}
                    </div>
                    <Rating value={row.importance} />
                  </div>
                </Button>
              ))}
              {memories.ready &&
                !memories.items.length &&
                !memories.error &&
                !memories.busy && (
                  <EmptyState variant="page">
                    <h3>{filters.q||filters.type||filters.aboutOwner||filters.status!=='active'?'这里没有找到匹配的记忆':'还没有记录下记忆'}</h3>
                    <p>{filters.q||filters.type||filters.aboutOwner||filters.status!=='active'?'试试别的关键词，或清除筛选。':'从一次聊天开始，值得记住的经历会慢慢积累在这里。'}</p>
                    {filters.q||filters.type||filters.aboutOwner||filters.status!=='active'?<Button
                      variant="outline"
                      onClick={() => {
                        setQuery("");
                        setFilters({
                          q: "",
                          type: "",
                          status: "active",
                          aboutOwner: false,
                        });
                      }}
                    >
                      清除筛选
                    </Button>:<Button asChild variant="outline"><a href={'#/c/'+encodeURIComponent(fid)+'/chat'}>去聊聊天</a></Button>}
                  </EmptyState>
                )}
              <More source={memories} label="记忆" />
            </div>
            <div className="lp-stack">
              <Surface className="lp-pad">
                <div className="replica-section-heading">
                  <h2>记忆小册</h2>
                </div>
                <div className="lp-two">
                  <div>
                    <strong className="replica-memory-number">
                      {memories.ready
                        ? memories.items.length + (memories.cursor ? "+" : "")
                        : "…"}
                    </strong>
                    <p className="lp-muted">当前筛选已读</p>
                  </div>
                  <div>
                    <strong className="replica-memory-number secondary">
                      {memories.ready
                        ? memories.items.filter((row) =>
                            row.actor_refs?.includes(owner),
                          ).length
                        : "…"}
                    </strong>
                    <p className="lp-muted">含你的引用</p>
                  </div>
                </div>
                <div className="lp-line" />
                <p className="lp-small">
                  记忆可以被纠正，也可以选择遗忘。原来的来源与修订会认真保留。数量仅包含已读取记录。
                </p>
                <div className="lp-action-row">
                  <Button
                    variant="link"
                    onClick={() => {
                      setQuery("");
                      setFilters({
                        q: "",
                        type: "",
                        status: "active",
                        aboutOwner: true,
                      });
                    }}
                  >
                    看看关于你的记忆
                  </Button>
                </div>
              </Surface>
              <div className="lp-soft-box">
                <div className="lp-quote-symbol">“</div>
                <p className="lp-copy">
                  不是记住所有的事情，
                  <br />
                  而是让重要的细节，
                  <br />
                  有地方慢慢安放。
                </p>
              </div>
            </div>
          </div>
        </TabsContent>
        <TabsContent value="reflection">
          <div className="lp-action-row">
            <Button
              variant="ghost"
              disabled={reflections.busy}
              onClick={reflections.load}
            >
              <RefreshCwIcon size={14} />
              刷新反思记录
            </Button>
          </div>
          <ReadStatus source={reflections} label="反思记录" />
          <div className="lp-two">
            {reflections.items.map((row) => (
              <Surface className="lp-reflection" key={row.id}>
                <div className="lp-row lp-between">
                  {(recordTime(row.at) || recordTime(row.created_at)) && <span className="lp-day-stamp">
                    {recordTime(row.at) || recordTime(row.created_at)}
                  </span>}
                  {reflectionStatus(row.status || row.state) && <Badge variant="outline">
                    {reflectionStatus(row.status || row.state)}
                  </Badge>}
                </div>
                {reflectionTitle(row.title) && <h3>{reflectionTitle(row.title)}</h3>}
                {recordText(row.summary) && <p>{recordText(row.summary)}</p>}
                <div className="lp-action-row">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => open("reflection", row, e)}
                  >
                    展开这次反思
                    <ChevronRightIcon size={13} />
                  </Button>
                </div>
              </Surface>
            ))}
          </div>
          {reflections.ready &&
            !reflections.items.length &&
            !reflections.error &&
            !reflections.busy && (
              <EmptyState variant="page">
                <h3>还没有形成反思</h3>
                <p>经历积累后，新的观察与认识会出现在这里。</p>
              </EmptyState>
            )}
          <More source={reflections} label="反思记录" />
          {reflections.extra.latest_task && Object.keys(reflections.extra.latest_task).length > 0 && (
            <Disclosure title="最近反思任务">
              <SummaryFacts facts={[
                ["处理状态", reflectionStatus(reflections.extra.latest_task.status)],
                ["开始时间", recordTime(reflections.extra.latest_task.started_at)],
                ["完成时间", recordTime(reflections.extra.latest_task.completed_at)],
                ["下次尝试", recordTime(reflections.extra.latest_task.next_attempt_at)],
              ]} />
              <RecordDownload value={reflections.extra.latest_task} title={reflectionDownload} />
            </Disclosure>
          )}
        </TabsContent>
        <TabsContent value="self">
          <div className="lp-toolbar">
            <div>
              <p className="lp-title">正在形成的认识</p>
              <p className="lp-muted">
                这些认识来自经历与反思，会随着生活继续变化。
              </p>
            </div>
            <div className="lp-row">
              <ChoiceSelect
                aria-label="认识状态"
                value={claimStatus}
                onChange={(e) => setClaimStatus(e.target.value)}
              >
                <option value="active">已记录的认识</option>
                <option value="uncertain">仍不确定</option>
                <option value="forgotten">已遗忘</option>
                <option value="superseded">已有替代</option>
                <option value="all">全部状态</option>
              </ChoiceSelect>
              <Button
                variant="outline"
                onClick={() =>
                  w.navigatePath(
                    "/c/" +
                      encodeURIComponent(fid) +
                      "/growth/self?view=snapshot",
                  )
                }
              >
                人格快照
                <ArrowUpRightIcon size={14} />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label="刷新自我认知"
                disabled={claims.busy}
                onClick={claims.load}
              >
                <RefreshCwIcon size={15} />
              </Button>
            </div>
          </div>
          <ReadStatus source={claims} label="自我认知" />
          <div className="lp-three">
            {claims.items.map((row) => (
              <article className="lp-claim" key={row.id}>
                <div className="lp-row lp-between">
                  <span className="lp-chip">
                    {labelFor(selfCategoryLabels, row.category)}
                  </span>
                  <LeafIcon size={15} />
                </div>
                <blockquote>“{row.claim}”</blockquote>
                <p className="lp-muted">
                  {labelFor(claimStatuses, row.status)}
                </p>
                {(row.source_valid === false || row.expired) && (
                  <p className="lp-muted">
                    {row.source_valid === false ? "所依赖来源需要重新核对。" : ""}
                    {row.expired ? "已超过有效期限。" : ""}
                  </p>
                )}
                {number(row.confidence) !== null && <Progress
                  value={row.confidence * 100}
                  className="lp-progress"
                  aria-label="认识置信度"
                />}
                <div className="lp-row lp-between">
                  {scoreText(row.confidence) && <span className="lp-muted">
                    置信度 {scoreText(row.confidence)}
                  </span>}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => open("self", row, e)}
                  >
                    查看依据
                    <ChevronRightIcon size={12} />
                  </Button>
                </div>
              </article>
            ))}
          </div>
          {claims.ready &&
            !claims.items.length &&
            !claims.error &&
            !claims.busy && (
              <EmptyState variant="page">
                <h3>认识自己，也需要时间</h3>
                <p>当前筛选还没有自我认知记录。</p>
              </EmptyState>
            )}
          <More source={claims} label="自我认知" />
        </TabsContent>
      </Tabs>
      <WorkspaceDialog
        open={!!selection && !action}
        onOpenChange={(value) => {
          if (!value) close();
        }}
        wide
        returnFocus={() => trigger.current}
        title={
          selection?.kind === "memory"
            ? "记忆详情"
            : selection?.kind === "reflection"
              ? reflectionTitle(selected?.title) || "反思记录"
              : "这条认识从哪里来"
        }
        description={
          selection?.kind === "memory"
            ? labelFor(memoryTypes, selected?.type) || undefined
            : selection?.kind === "self"
              ? labelFor(selfCategoryLabels, selected?.category) || undefined
              : "反思、来源和实际处置记录"
        }
        footer={
          selected ? (
            <>
              <Button variant="outline" onClick={close}>
                返回列表
              </Button>
              {selection?.kind === "memory" && selected.status === "active" && (
                <>
                  <Button
                    variant="ghost"
                    disabled={readonly()}
                    onClick={(e) => memoryCommand("forget", selected, e)}
                  >
                    遗忘这段记忆
                  </Button>
                  <Button
                    disabled={readonly()}
                    onClick={(e) => memoryCommand("revise", selected, e)}
                  >
                    <Edit3Icon size={14} />
                    记错了，纠正一下
                  </Button>
                </>
              )}
              {selection?.kind === "reflection" && (
                <>
                  <Button
                    variant="outline"
                    disabled={readonly()}
                    onClick={(e) => reflectionFeedback("disagree", selected, e)}
                  >
                    说说不同看法
                  </Button>
                  <Button
                    disabled={readonly()}
                    onClick={(e) => reflectionFeedback("agree", selected, e)}
                  >
                    赞同这次观察
                  </Button>
                </>
              )}
              {selection?.kind === "self" && (
                <>
                  <Button
                    variant="outline"
                    disabled={readonly() || selected.status === "forgotten"}
                    onClick={(e) => selfCommand("forget", selected, e)}
                  >
                    不再采用
                  </Button>
                  <Button
                    disabled={readonly()}
                    onClick={(e) => selfCommand("review", selected, e)}
                  >
                    <CheckCircle2Icon size={14} />
                    确认已核对
                  </Button>
                </>
              )}
            </>
          ) : undefined
        }
      >
        <ReadStatus source={detail} label="详情" />
        {selected && (
          <div className="lp-stack">
            <Button
              variant="link"
              disabled={detail.busy || (selection?.kind === "self" && (revisions.busy || reviews.busy))}
              onClick={selection?.kind === "self" ? () => void refreshSelection() : detail.load}
            >
              刷新当前详情
            </Button>
            {selection?.kind === "memory" ? (
              <Tabs
                value={detailTab}
                onValueChange={setDetailTab}
                className="replica-memory-detail-tabs"
              >
                <TabsList variant="line" className="replica-segment-tabs">
                  <TabsTrigger value="content">这段记忆</TabsTrigger>
                  <TabsTrigger value="source">原始来源</TabsTrigger>
                  <TabsTrigger value="history">修订记录</TabsTrigger>
                </TabsList>
                <TabsContent value="content">
                  <blockquote className="lp-memory-quote">
                    {selected.content}
                  </blockquote>
                  <SummaryFacts facts={[
                    ...memoryFacts(selected),
                    ["重要度", number(selected.importance) === null ? null : <Rating value={selected.importance} />],
                  ]} />
                  {["invalid", "pending"].includes(
                    selected.provenance_status,
                  ) && (
                    <WorkspaceInfo>
                      来源尚未有效，不能把这条记忆当作当前已确认事实。
                    </WorkspaceInfo>
                  )}
                  <RecordDownload value={selected} title={memoryDownload} />
                </TabsContent>
                <TabsContent value="source">
                  <p className="lp-muted">
                    {sourceRevision === selected.revision ? "当前记忆的来源" : "这次历史修订的来源"}，有效性以当前来源状态为准。
                  </p>
                  <ReadStatus source={sources} label="来源" />
                  {sources.items.map((row) => (
                    <div className="lp-source-card" key={row.source_ref}>
                      <SummaryFacts facts={memorySourceFacts(row)} />
                      {row.source_kind === "message" &&
                        row.conversation_id &&
                        row.source_id && (
                          <Button
                            variant="outline"
                            onClick={() =>
                              w.navigatePath(
                                "/c/" +
                                  encodeURIComponent(fid) +
                                  "/chat?" +
                                  new URLSearchParams({
                                    conversation: row.conversation_id,
                                    message: row.source_id,
                                  }),
                              )
                            }
                          >
                            查看原消息
                            <ArrowUpRightIcon size={14} />
                          </Button>
                        )}
                      <RecordDownload value={row} title={memoryDownload} />
                    </div>
                  ))}
                  <More source={sources} label="来源" />
                </TabsContent>
                <TabsContent value="history">
                  <ReadStatus source={revisions} label="修订" />
                  {revisions.items.map((row) => (
                    <div className="lp-history-item" key={row.revision}>
                      <h4>{row.revision === selected.revision ? "当前记录" : "历史修订"}</h4>
                      <p>{row.content}</p>
                      <SummaryFacts facts={memoryRevisionFacts(row)} />
                      <div className="lp-action-row">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setSourceRevision(row.revision);
                            setDetailTab("source");
                          }}
                        >
                          查看此版来源
                        </Button>
                        {row.status === "active" &&
                          row.revision < selected.revision && (
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={readonly()}
                              onClick={(e) =>
                                memoryCommand("rollback", selected, e, row)
                              }
                            >
                              补偿恢复至此版
                            </Button>
                          )}
                      </div>
                      <RecordDownload value={row} title={memoryDownload} />
                    </div>
                  ))}
                  <More source={revisions} label="修订" />
                </TabsContent>
              </Tabs>
            ) : selection?.kind === "reflection" ? (
              <>
                <ReflectionObservations record={selected} />
                {reflectionStatus(selected.status || selected.state) && <WorkspaceInfo>
                  实际状态：{reflectionStatus(selected.status || selected.state)}。反馈不会直接采纳候选或改写人格。
                </WorkspaceInfo>}
                <Disclosure title="提议与实际改变">
                  <ReflectionChanges record={selected} fid={fid} />
                </Disclosure>
                {feedbackPointer?.reflectionId === selected.id && (
                  <RequestStatus
                    key={feedbackPointer.request.id}
                    fid={fid}
                    requestId={feedbackPointer.request.id}
                    initial={feedbackPointer.request}
                    onTerminal={() => void refreshSelection()}
                    renderRecord={(record) => <>
                      <ReflectionFeedbackSummary record={record} />
                      <RecordDownload value={record} title={reflectionDownload} />
                    </>}
                  />
                )}
                <Disclosure title="所有者反馈与处理记录">
                  <ReadStatus source={feedback} label="反馈记录" />
                  {feedback.items.map((row) => (
                    <div className="lp-source-card" key={row.id}>
                      <ReflectionFeedbackSummary record={row} />
                      <Button
                        variant="link"
                        onClick={() =>
                          setFeedbackPointer({
                            reflectionId: selected.id,
                            request: row,
                          })
                        }
                      >
                        跟进原反馈请求
                      </Button>
                      <RecordDownload value={row} title={reflectionDownload} />
                    </div>
                  ))}
                  <More source={feedback} label="反馈记录" />
                </Disclosure>
                <RecordDownload value={selected} title={reflectionDownload} />
              </>
            ) : (
              <>
                <ClaimSummary record={selected} />
                <WorkspaceInfo>
                  复核记录你已阅读当前认识，证据和置信度仍以实际反思为准。认识正文由实际反思产生。
                </WorkspaceInfo>
                <RecordDownload value={selected} title={claimDownload} />
                <Disclosure title="之前的认识与历史恢复">
                  <ReadStatus source={revisions} label="认识修订" />
                  {revisions.items.map((row, index) => (
                    <div
                      className="lp-history-item"
                      key={row.revision ?? index}
                    >
                      <h4>{row.revision === selected.revision ? "当前认识" : "历史认识"}</h4>
                      <SummaryFacts facts={[
                        ["修订时间", recordTime(row.created_at)],
                        ["修订状态", labelFor({accepted:"已采纳", applied:"已应用", rejected:"未采纳", superseded:"已有替代"}, row.status)],
                      ]} />
                      <ClaimSummary record={row.after_value?.category ? row.after_value : row.candidate || row.after_value || {}} />
                      {canRestoreSelfRevision(row) &&
                        ["active", "uncertain"].includes(
                          (row.after_value?.category
                            ? row.after_value
                            : row.candidate
                          )?.status,
                        ) &&
                        row.revision < selected.revision && (
                          <Button
                            variant="outline"
                            disabled={readonly()}
                            onClick={(e) =>
                              selfCommand("restore", selected, e, row)
                            }
                          >
                            恢复此版内容
                          </Button>
                        )}
                      <RecordDownload value={row} title={claimDownload} />
                    </div>
                  ))}
                  <More source={revisions} label="认识修订" />
                </Disclosure>
                <Disclosure title="所有者复核记录">
                  <ReadStatus source={reviews} label="复核记录" />
                  {reviews.items.map((row) => (
                    <div className="lp-source-card" key={row.id}>
                      <SummaryFacts facts={[
                        ["复核说明", recordText(row.reason)],
                        ["复核时间", recordTime(row.created_at)],
                        ["复核对象", row.claim_revision === selected.revision ? "当前认识" : "历史认识"],
                      ]} />
                      <RecordDownload value={row} title={claimDownload} />
                    </div>
                  ))}
                  <More source={reviews} label="复核记录" />
                </Disclosure>
              </>
            )}
          </div>
        )}
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          key={action.identity}
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => commandTrigger.current}
        />
      )}
    </div>
  );
}
