import StudioJobsView from "../views/StudioJobs";
import { InlineFeedback as OperationStatus } from "@/react-app/shared/Presentation";
import MediaPreview from "./MediaPreview";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Label,
  Textarea,
  ChoiceSelect,
  FieldInput,
  Disclosure,
} from "../controls";
import { useWorkspace } from "../page";
import { useCursorPage } from "../hooks/use-cursor-page";
import CursorControls from "./CursorControls";
import MaterialRetention from "./MaterialRetention";
import JsonDetails from "./JsonDetails";
import { persistentCommand } from "@/v3/providers/persistent-command.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
import { studioChanges } from "@/v3/proto/components/studio-comparison.js";
import {
  referenceFromGrant,
  mediaReferencePurposes,
} from "@/v3/proto/components/media-reference-grants.js";
import { confirm } from "@/v3/proto/components/overlay.js";
import { errorMessage, stateLabel } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const copy = (value: any) => JSON.parse(JSON.stringify(value));
const kinds: Record<string, string> = {
  assistant: "助手建议",
  text_preview: "虚构文本预览",
  image_preview: "虚构图片预览",
};
const jobError: Record<string, string> = {
  studio_media_not_configured: "图片生成尚未配置，请配置后重试。",
  studio_media_generation_failed: "图片生成失败，请核对原执行状态后重试。",
  studio_media_execution_active: "原图片任务仍在执行或取消中，请读取原任务。",
  workflow_status_unavailable: "暂时无法查询工作流状态，请重新读取。",
};
export default function StudioJobs({
  resource,
  readDraft,
  adopt,
  jobId = "",
  onSelect = () => {},
  initialKind = "assistant",
  onResourceChanged = () => {},
}: any) {
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    service = w.services.studioClient,
    slot = `fluctlight-studio:${owner}:${resource?.id || "new"}:jobs`,
    active = useRef(false),
    epoch = useRef(0),
    draftReader = useRef(readDraft),
    selectCallback = useRef(onSelect),
    resourceChange = useRef(onResourceChanged);
  draftReader.current = readDraft;
  selectCallback.current = onSelect;
  resourceChange.current = onResourceChanged;
  const [ui, setUi] = useState<any>(() => {
      try {
        const value = {
          instruction: "",
          kind: initialKind,
          selected: jobId,
          referenceAssetId: "",
          referencePurpose: "",
          referenceGrant: null,
          materialReleased: false,
          ...JSON.parse(studioDraftStorage().getItem(slot + ":draft") || "{}"),
        };
        if (jobId) value.selected = jobId;
        if (initialKind === "text_preview" && !jobId) value.kind = initialKind;
        return value;
      } catch {
        return null;
      }
    }),
    [job, setJob] = useState<any>(null),
    [pending, setPending] = useState<any>(null),
    [receipt, setReceipt] = useState<any>(null),
    [busy, setBusy] = useState(false),
    [reading, setReading] = useState(false),
    [error, setError] = useState(""),
    [detailError, setDetailError] = useState(""),
    [notice, setNotice] = useState(""),
    [retention, setRetention] = useState<any>(null),
    [adopting, setAdopting] = useState(false);
  const [materialId, setMaterialId] = useState("");
  const current = useRef(ui),
    currentJob = useRef(job),
    currentScope = useRef(slot);
  current.current = ui;
  currentJob.current = job;
  currentScope.current = slot;
  const live = () =>
    active.current &&
    currentScope.current === slot &&
    w.state.session?.actorId === owner;
  const jobs = useCursorPage(
    (cursor) =>
      resource
        ? service.jobs({ resource_id: resource.id, cursor })
        : Promise.resolve({ items: [] }),
    JSON.stringify([owner, resource?.id, "studio-jobs"]),
  );
  const materials = useCursorPage(
    async (cursor) => {
      if (!resource) return { items: [] };
      const page = await service.materials({
        resource_id: resource.id,
        cursor,
        released: !!ui?.materialReleased,
      });
      return {
        ...page,
        items: page.items.filter(
          (item: any) =>
            item.owner_actor_id === owner && item.resource_id === resource.id,
        ),
      };
    },
    JSON.stringify([owner, resource?.id, "materials", ui?.materialReleased]),
  );
  const grants = useCursorPage(
    (cursor) =>
      resource
        ? service.referenceGrants({
            target_id: resource.id,
            target_scope: "studio_resource",
            cursor,
          })
        : Promise.resolve({ items: [] }),
    JSON.stringify([owner, resource?.id, "reference-grants"]),
  );
  const command = useMemo(
    () =>
      persistentCommand({
        key: slot + ":command",
        storage: studioDraftStorage,
        execute: async (value: any, key: string) => {
          if (!live()) throw Error("会话或资源范围已切换");
          let result: any;
          if (value.op === "create")
            result = await service.createJob({
              ...value.body,
              idempotencyKey: key,
            });
          else {
            const body = {
              expectedRevision: value.revision,
              reason: value.reason,
              idempotencyKey: key,
            };
            if (value.op === "retry")
              result = await service.retryJob(value.id, body);
            else if (value.op === "discard")
              result = await service.discardJob(value.id, body);
            else if (value.op === "save-material")
              result = await service.saveMaterial(value.id, body);
            else throw Error("未知创作任务操作");
          }
          if (
            typeof result?.id !== "string" ||
            !result.id ||
            result.resource_id !== resource.id ||
            (result.owner_actor_id && result.owner_actor_id !== owner) ||
            (value.op !== "create" &&
              value.op !== "save-material" &&
              result.id !== value.id) ||
            (value.op === "save-material" && result.job_id !== value.id)
          )
            throw Error(
              "创作操作回执的资源或任务标识尚未完整确认，请核对原提交。",
            );
          // Persist acceptance before persistentCommand releases the frozen request.
          // Unmount does not invalidate acceptance; an Owner purge does.
          const raw = studioDraftStorage().getItem(slot + ":command");
          if (
            w.state.session?.actorId === owner &&
            raw &&
            JSON.parse(raw).idempotencyKey === key
          )
            studioDraftStorage().setItem(
              slot + ":receipt",
              JSON.stringify({
                sourceId: resource.id,
                operation: value.op,
                id: result.id,
                jobId: value.op === "save-material" ? value.id : result.id,
                revision: result.revision,
              }),
            );
          return result;
        },
      }),
    [slot, service],
  );
  function persist(next: any) {
    if (!live()) return;
    try {
      studioDraftStorage().setItem(slot + ":draft", JSON.stringify(next));
      current.current = next;
      setUi(next);
    } catch (cause) {
      setError(errorMessage(cause));
    }
  }
  function edit(values: any) {
    if (!current.current) return;
    persist({ ...current.current, ...values });
  }
  function select(id: string) {
    if (!current.current) {
      setError("创作面板草稿无法读取，请保留原记录核对");
      return;
    }
    edit({ selected: id });
    setJob(null);
    epoch.current++;
    selectCallback.current(id);
    if (resource) {
      const value = new URL(location.href),
        [path, search = ""] = value.hash.replace(/^#/, "").split("?"),
        query = new URLSearchParams(search);
      query.set("job", id);
      history.replaceState(null, "", "#" + path + "?" + query);
    }
  }
  async function observe() {
    if (!resource || !live() || !current.current?.selected || document.hidden)
      return;
    const id = current.current.selected,
      token = ++epoch.current;
    setReading(true);
    try {
      const value = await service.job(id);
      if (!live() || token !== epoch.current || id !== current.current.selected)
        return;
      if (
        value.resource_id !== resource.id ||
        (value.owner_actor_id && value.owner_actor_id !== owner)
      )
        throw Error("该任务属于其他资源，请从原资源查看");
      setJob(value);
      setDetailError("");
    } catch (cause) {
      if (live() && token === epoch.current)
        setDetailError(errorMessage(cause));
    } finally {
      if (live() && token === epoch.current) setReading(false);
    }
  }
  const observeRef = useRef(observe);
  observeRef.current = observe;
  async function readReceipt() {
    if (!live() || !resource) return;
    const raw = studioDraftStorage().getItem(slot + ":receipt");
    if (!raw) return;
    const token = ++epoch.current;
    try {
      const pointer = JSON.parse(raw);
      setReceipt(pointer);
      if (
        pointer.sourceId !== resource.id ||
        typeof pointer.id !== "string" ||
        !["create", "retry", "discard", "save-material"].includes(
          pointer.operation,
        )
      )
        throw Error("原创作操作的回执无法读取，请保留原记录核对。");
      const actual = await (pointer.operation === "save-material"
        ? service.material(pointer.id)
        : service.job(pointer.id));
      if (
        !live() ||
        token !== epoch.current ||
        studioDraftStorage().getItem(slot + ":receipt") !== raw
      )
        return;
      if (
        actual?.id !== pointer.id ||
        actual.resource_id !== resource.id ||
        (actual.owner_actor_id && actual.owner_actor_id !== owner) ||
        (pointer.operation === "save-material" &&
          actual.job_id !== pointer.jobId) ||
        (Number.isSafeInteger(pointer.revision) &&
          actual.revision < pointer.revision)
      )
        throw Error("原操作对应的实际任务或素材尚未完整读回。");
      if (pointer.operation === "save-material") {
        setNotice(
          actual.released_at
            ? "原保存请求已记录；该素材目前已取消保留。"
            : "结果已读回，当前保留为资源的定义素材。",
        );
        await materials.reload();
      } else {
        select(actual.id);
        setJob(actual);
        setNotice(
          pointer.operation === "create"
            ? "创作任务已受理，已读回原任务；当前输入仍保留。"
            : "原任务操作已受理，已读回最新状态。",
        );
        await jobs.reload();
      }
      if (!live() || studioDraftStorage().getItem(slot + ":receipt") !== raw)
        return;
      studioDraftStorage().removeItem(slot + ":receipt");
      setReceipt(null);
      setError("");
    } catch (cause) {
      if (live())
        setError("原操作已有回执；结果读取尚未完成：" + errorMessage(cause));
    }
  }
  useEffect(() => {
    active.current = true;
    try {
      setPending(command.pending());
      void readReceipt();
    } catch (cause) {
      setError(errorMessage(cause));
    }
    return () => {
      active.current = false;
      epoch.current++;
    };
  }, [command]);
  useEffect(() => {
    if (ui?.selected) void observeRef.current();
  }, [ui?.selected]);
  useEffect(() => {
    if (jobId && jobId !== current.current?.selected) select(jobId);
  }, [jobId]);
  useEffect(() => {
    if (ui && !ui.selected) {
      const running = jobs.items.find((entry: any) =>
        ["queued", "running"].includes(entry.status),
      );
      if (running) select(running.id);
    }
  }, [jobs.items]);
  useEffect(() => {
    if (!job || !["queued", "running"].includes(job.status) || busy || adopting)
      return;
    let timer: ReturnType<typeof setTimeout>;
    const tick = () => {
      clearTimeout(timer);
      if (!document.hidden)
        timer = setTimeout(() => void observeRef.current(), 3000);
    };
    const visibility = () => {
      clearTimeout(timer);
      if (!document.hidden) void observeRef.current();
    };
    tick();
    document.addEventListener("visibilitychange", visibility);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [job, busy, adopting]);
  async function refresh() {
    await Promise.all([
      observeRef.current(),
      jobs.reload(),
      materials.reload(),
      grants.reload(),
    ]);
  }
  async function submit(value: any, retry = false) {
    if (
      !resource ||
      !live() ||
      busy ||
      receipt ||
      studioDraftStorage().getItem(slot + ":receipt")
    )
      return;
    setBusy(true);
    setError("");
    setNotice("");
    epoch.current++;
    try {
      await (retry ? command.retry() : command.submit(value));
      if (!live()) return;
      await readReceipt();
    } catch (cause) {
      if (live()) setError(errorMessage(cause));
    } finally {
      if (live()) {
        setBusy(false);
        setPending(command.pending());
      }
    }
  }
  function create() {
    if (resource?.status === "archived") {
      setError("资源已归档，恢复后才能提交创作请求。");
      return;
    }
    if (draftReader.current()?.bodyText !== undefined) {
      setError("请先核对完整 JSON 输入，再提交创作请求。");
      return;
    }
    if (!resource || !ui?.instruction?.trim()) {
      setError("请先保存资源，并填写创作指令。");
      return;
    }
    if (
      ui.kind === "image_preview" &&
      ui.referenceAssetId &&
      !ui.referenceGrant &&
      !["scene", "animal", "object"].includes(ui.referencePurpose)
    ) {
      setError("请选择参考用途，人物画面需明确的参考授权。");
      return;
    }
    const referenceAsset =
      ui.kind === "image_preview" && ui.referenceAssetId
        ? ui.referenceGrant
          ? copy(ui.referenceGrant)
          : { assetId: ui.referenceAssetId, purpose: ui.referencePurpose }
        : undefined;
    const draft = draftReader.current();
    return submit({
      op: "create",
      body: {
        resourceId: resource.id,
        resourceRevision: resource.revision,
        kind: ui.kind,
        ...(referenceAsset ? { referenceAsset } : {}),
        name: draft.name,
        body: copy(draft.body),
        instruction: ui.instruction,
        reason: "所有者提交隔离创作任务",
      },
    });
  }
  async function operate(op: string) {
    if (!job || busy) return;
    const selected = copy(job),
      reason =
        op === "retry"
          ? "所有者显式重试失败创作任务"
          : op === "discard"
            ? "所有者丢弃创作任务"
            : "所有者保存虚构创作结果为定义素材";
    if (
      op === "discard" &&
      !(await confirm({
        title: "丢弃创作结果",
        text: "该任务迟到的结果不能重新发布，已明确保存的素材仍保留。",
        type: "danger",
      }))
    )
      return;
    if (!live() || selected.id !== job.id) return;
    return submit({ op, id: selected.id, revision: selected.revision, reason });
  }
  async function adoptProposal() {
    if (!job || !adopt || adopting || job.result?.disposition !== "proposal")
      return;
    if (draftReader.current()?.bodyText !== undefined) {
      setError("请先核对完整 JSON 输入，再采用创作建议。");
      return;
    }
    const selected = copy(job),
      before = copy({
        name: draftReader.current().name,
        body: draftReader.current().body,
        bodyText: draftReader.current().bodyText,
      }),
      proposal = copy({ name: job.result.name, body: job.result.body });
    const changes = studioChanges(before, proposal);
    if (!changes.changed.length) return;
    setAdopting(true);
    try {
      if (
        !(await confirm({
          title: "采用完整建议到工作草稿",
          identity: "工作草稿",
          okText: "采用到草稿",
          text: `替换名称和完整定义，共 ${changes.changed.length} 项改变。采用后仍需保存、发布及应用。`,
        })) ||
        !live()
      )
        return;
      const latest = draftReader.current();
      if (
        current.current.selected !== selected.id ||
        JSON.stringify(before) !==
          JSON.stringify({
            name: latest.name,
            body: latest.body,
            bodyText: latest.bodyText,
          }) ||
        JSON.stringify(proposal) !==
          JSON.stringify({
            name: currentJob.current?.result?.name,
            body: currentJob.current?.result?.body,
          })
      ) {
        setError("草稿或建议已经变化，请核对新的比较后再采用。");
        return;
      }
      adopt(proposal);
    } finally {
      if (live()) setAdopting(false);
    }
  }
  const out = job?.result || {},
    proposal =
      job?.kind === "assistant" &&
      job?.status === "completed" &&
      out.disposition === "proposal"
        ? { name: out.name, body: out.body }
        : null,
    comparison =
      proposal &&
      studioChanges(
        { name: readDraft().name, body: readDraft().body },
        proposal,
      );
  const imageMaterials = materials.items.filter(
      (item: any) => item.asset_id && !item.released_at,
    ),
    references = [
      { value: "", label: "不使用参考图片" },
      ...imageMaterials.map((item: any) => ({
        value: item.asset_id,
        label: item.label || "已保存素材",
      })),
      ...grants.items.map((grant: any) => ({
        value: "grant:" + grant.id,
        label:
          "已授权 · " +
          (mediaReferencePurposes as Record<string, string>)[grant.purpose] +
          " · " +
          grant.asset_id,
      })),
    ];
  const selectedReference = ui?.referenceGrant
    ? "grant:" + ui.referenceGrant.grantId
    : ui?.referenceAssetId || "";
  if (
    selectedReference &&
    !references.some((entry) => entry.value === selectedReference)
  )
    references.push({
      value: selectedReference,
      label: "已恢复的原参考，请刷新核对状态",
    });
  return (
    <StudioJobsView
      model={{
        w,
        service,
        resource,
        ui,
        job,
        pending,
        busy,
        reading,
        error,
        detailError,
        notice,
        retention,
        setRetention,
        materialId,
        setMaterialId,
        adopting,
        jobs,
        materials,
        grants,
        edit,
        select,
        observe,
        refresh,
        submit,
        create,
        operate,
        adoptProposal,
        out,
        proposal,
        comparison,
        references,
        selectedReference,
        readDraft,
        setError,
        jobError,
        receipt,
        readReceipt,
        releaseRejected: async () => {
          if (
            !(await confirm({
              title: "核对后解除已拒绝的原提交",
              text: "先读取最新资源与任务状态，当前工作草稿保留。",
            })) ||
            !live()
          )
            return;
          try {
            const latest = await service.resource(resource.id);
            if (!live()) return;
            command.releaseRejected();
            await resourceChange.current(latest);
            setPending(command.pending());
            await refresh();
          } catch (cause) {
            if (live()) setError(errorMessage(cause));
          }
        },
      }}
    />
  );
}
