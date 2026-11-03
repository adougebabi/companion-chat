import { CircleCheck, CircleHelp, CircleX, Info } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
import { mediaReadState, mediaSource } from "@/approved-ui/media-presentation.mjs";

type Outcome = "pass" | "fail" | "pending" | "info";
type Result = { label: string; value: string; outcome: Outcome };
type RecordValue = Record<string, unknown>;
const record = (value: unknown): RecordValue =>
  value && typeof value === "object" && !Array.isArray(value)
    ? (value as RecordValue)
    : {};
const text = (value: unknown) =>
  typeof value === "string" && value.trim() ? value : "";
const indicators = {
  pass: { Icon: CircleCheck, label: "通过", tone: "text-primary" },
  fail: { Icon: CircleX, label: "失败", tone: "text-destructive" },
  pending: { Icon: CircleHelp, label: "待确认", tone: "text-muted-foreground" },
  info: { Icon: Info, label: "说明", tone: "text-muted-foreground" },
};
const assetStates: Record<string, [string, Outcome]> = {
  ready: ["素材已就绪", "pass"],
  pending: ["素材准备中", "pending"],
  unavailable: ["素材暂不可用", "fail"],
  tombstoned: ["删除处理中，已禁止读取", "fail"],
  deleted: ["素材已删除", "fail"],
  failed: ["素材准备失败", "fail"],
};
const generationStates: Record<string, [string, Outcome]> = {
  completed: ["生成已完成", "pass"],
  pending: ["等待生成", "pending"],
  queued: ["等待生成", "pending"],
  running: ["正在生成", "pending"],
  failed: ["生成失败", "fail"],
  cancelled: ["生成已取消", "fail"],
};
const purposes: Record<string, string> = {
  visual_identity: "角色视觉形象",
  scene: "场景画面",
  selfie: "自拍画面",
  portrait: "人物画面",
};
function stateResult(
  label: string,
  value: unknown,
  states: Record<string, [string, Outcome]>,
): Result {
  const [description, outcome] =
    typeof value === "string" && Object.hasOwn(states, value)
      ? states[value]
      : ["状态尚未确认", "pending" as const];
  return { label, value: description, outcome };
}

// Explicit presentation fields keep internal provenance and future wire fields
// out of visible content, accessibility labels and hover text.
function resultGroups(asset: RecordValue) {
  const snapshot = record(asset.source_snapshot);
  const studio = record(snapshot.studio_source);
  const normalized = record(snapshot.normalized_output);
  const availability: Result[] = [
    stateResult("资产状态", asset.status, assetStates),
    {
      label: "读取校验",
      value:
        typeof asset.readable === "boolean"
          ? text(mediaReadState(asset)) || "素材暂不可读取"
          : "读取结果尚未确认",
      outcome:
        asset.readable === true ? "pass" : asset.readable === false ? "fail" : "pending",
    },
  ];
  if (asset.origin !== "owner_upload") {
    availability.push(stateResult("生成状态", asset.generation_status, generationStates));
  }
  const source: Result[] = [
    { label: "来源渠道", value: text(mediaSource(asset)) || "已登记素材", outcome: "info" },
    {
      label: "来源记录",
      ...(asset.source_state === "owner_original"
        ? { value: "已记录原始上传来源", outcome: "pass" as const }
        : asset.source_state === "recorded" && Object.keys(snapshot).length > 0
          ? { value: "已保存生成时来源快照", outcome: "pass" as const }
          : {
              value: asset.source_state === "legacy_unstructured"
                ? "旧记录未提供结构化来源"
                : "来源记录尚未完整确认",
              outcome: "pending" as const,
            }),
    },
  ];
  if (text(snapshot.purpose)) {
    source.push({
      label: "生成用途",
      value: Object.hasOwn(purposes, String(snapshot.purpose))
        ? purposes[String(snapshot.purpose)]
        : "用途尚未确认",
      outcome: "info",
    });
  }
  const description = text(snapshot.description) || text(normalized.prompt);
  if (description) source.push({ label: "画面说明", value: description, outcome: "info" });
  if (text(studio.name)) source.push({ label: "创作名称", value: text(studio.name), outcome: "info" });
  if (text(record(snapshot.reference_asset).asset_id)) {
    source.push({ label: "参考素材", value: "已关联原始参考素材", outcome: "info" });
  }
  const library: Result[] = [
    {
      label: "库中展示",
      value: asset.hidden === true ? "已在素材库隐藏" : asset.hidden === false ? "在素材库显示" : "展示状态尚未确认",
      outcome: "info",
    },
    {
      label: "事实属性",
      value: asset.factual === false ? "不作为角色生活事实" : asset.factual === true ? "标记为角色生活事实" : "事实属性尚未记录",
      outcome: "info",
    },
  ];
  return [
    { title: "资产可用性", rows: availability },
    { title: "生成时来源", rows: source },
    { title: "库中状态", rows: library },
  ];
}

export default function MediaSourceResults({ asset }: { asset: RecordValue }) {
  return (
    <div className="grid min-w-0 gap-5" data-testid="media-source-results">
      <p className="text-sm text-muted-foreground">素材读取结果与生成时保存的来源记录。</p>
      {resultGroups(asset).map(({ title, rows }) => (
        <section key={title} aria-label={title} className="grid min-w-0 gap-2">
          <h3 className="text-sm font-medium">{title}</h3>
          <ItemGroup aria-label={title + "结果"}>
            {rows.map(({ label, value, outcome }) => {
              const { Icon, label: status, tone } = indicators[outcome];
              return (
                <Item key={label} role="listitem" variant="outline" size="sm" className="min-w-0 items-start" data-result={outcome}>
                  <ItemMedia variant="icon" className={tone}><Icon aria-hidden="true" /></ItemMedia>
                  <ItemContent className="min-w-0">
                    <ItemTitle className="line-clamp-none w-full flex-wrap">{label}<Badge variant={outcome === "fail" ? "destructive" : "secondary"} className={tone}>{status}</Badge></ItemTitle>
                    <ItemDescription className="line-clamp-none whitespace-pre-wrap [overflow-wrap:anywhere]">{value}</ItemDescription>
                  </ItemContent>
                </Item>
              );
            })}
          </ItemGroup>
        </section>
      ))}
    </div>
  );
}
