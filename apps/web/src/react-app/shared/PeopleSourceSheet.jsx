import { DetailDrawer } from "../components";
import { Button } from "@/components/ui/button";
import { PeopleFact } from "./PeopleParts";
import { aboutFields, extraAboutFields } from "@/approved-ui/people-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";

// Source objects remain intact in the download; ordinary reading never traverses
// nested context/fact/history trees or mistakes technical keys for field labels.
export function SourceValue({ value }) {
  if (value === undefined) return "尚未记录";
  if (value === null) return "未填写（空值）";
  if (value === "") return "空文本";
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value !== "object") return String(value);
  return "结构化资料（完整值见原始记录下载）";
}
export function SourceDownload({ value, label = "下载完整原始记录" }) {
  return <Button variant="outline" size="sm" asChild><a
    href={"data:application/json;charset=utf-8," + encodeURIComponent(JSON.stringify(value, null, 2))}
    download="people-source.json">{label}</a></Button>;
}
export function SourceFact({ row }) {
  const label = [...aboutFields, ...extraAboutFields].find(([key]) => key === row.attribute)?.[1] || "其他资料";
  const transitions={correct: "纠正了自填资料", change: "更新了目前的情况", assert: "首次填写资料"};
  return <article className="people-source-record">
    <h3>{label}</h3>
    <PeopleFact label="记录内容"><SourceValue value={row.value} /></PeopleFact>
    <PeopleFact label="资料来源">{row.epistemic_kind === "owner_defined" ? "主动填写" : row.epistemic_kind === "user_statement" ? "交流中表达的事实" : "其他已记录来源"}</PeopleFact>
    <PeopleFact label="修改含义">{Object.hasOwn(transitions,row.transition_kind)?transitions[row.transition_kind]:"资料记录"}</PeopleFact>
    <PeopleFact label="记录时间">{row.effective_at || row.created_at ? userInstant(row.effective_at || row.created_at) : "尚未记录时间"}</PeopleFact>
    {row.revision != null && <PeopleFact label="修订版本">{String(row.revision)}</PeopleFact>}
    {row.reason && <PeopleFact label="修改说明"><SourceValue value={row.reason} /></PeopleFact>}
    <SourceDownload value={row} />
  </article>;
}
export default function PeopleSourceSheet({ open, onOpenChange, returnFocus, title, description, children, source, scope }) {
  if (!open) return null;
  return <DetailDrawer title={title} contentClassName="replica-overlay people-source-overlay" onClose={() => onOpenChange(false)} returnFocus={returnFocus}
    footer={source && <SourceDownload value={source} />}>
    <div className="replica-profile replica-people people-source-sheet">
      <p className="companion-body-text">{description}</p>
      <p className="people-source-scope">{scope}</p>
      <div className="companion-stack">{children}</div>
    </div>
  </DetailDrawer>;
}
