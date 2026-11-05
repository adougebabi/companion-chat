import { Disclosure } from "../controls";
import { initializationWarnings } from "@/v3/creation-feedback.mjs";
import { creationPreviewLabel } from "./CreationPreviewValue";
import WorkspaceInfo from "./WorkspaceInfo";

const labels = {
  relationship_seeds: "初始关系线索", initial_relationships: "初始关系",
  initial_goals: "初始目标", initial_intentions: "初始意图",
  wardrobe_items: "初始衣物", core_values: "核心价值观", life_habits: "生活习惯",
  recurring_commitments: "长期约定", character_constraints: "边界与约束",
  profiles: "人格档案", active_profile_id: "当前人格", takeover_rules: "接管规则",
  speech_patterns: "说话习惯", channels: "表达渠道", claims: "自我认知",
};

function fieldLabel(path) {
  const last = String(path || "").split(".").at(-1) || "";
  const key = last.replace(/\[\d+\]/g, "");
  const index = last.match(/\[(\d+)\]$/);
  const title = labels[key] || creationPreviewLabel(key);
  return index ? `${title} · 第 ${Number(index[1]) + 1} 项` : title;
}

// This is an evidence view: do not translate values, hide keys, or parse a
// string that happens to look like JSON. Those changes would obscure what the
// model actually returned.
function OriginalValue({ value }) {
  if (value === null) return <span>空值（null）</span>;
  if (typeof value === "string") return <span>文字：<span className="whitespace-pre-wrap break-all">{value === "" ? "（空文本）" : value}</span></span>;
  if (typeof value === "boolean") return <span>布尔值：{String(value)}</span>;
  if (typeof value === "number") return <span>数值：{String(value)}</span>;
  if (Array.isArray(value)) return value.length ? <ol className="grid min-w-0 gap-2">{value.map((item, index) => <li key={index}><span>第 {index + 1} 项：</span><OriginalValue value={item}/></li>)}</ol> : <span>空列表</span>;
  if (value && typeof value === "object") return Object.keys(value).length ? <dl className="grid min-w-0 gap-2">{Object.entries(value).map(([key, item]) => <div key={key} className="min-w-0"><dt className="break-all">{key}</dt><dd className="ml-3"><OriginalValue value={item}/></dd></div>)}</dl> : <span>空对象</span>;
  return <span>尚未提供</span>;
}

export default function InitializationRecoveryNotice({ extensions }) {
  const warnings = initializationWarnings(extensions?.initialization_warnings);
  if (!warnings.length) return null;
  const recovered = extensions?.recovered_fields && typeof extensions.recovered_fields === "object" && !Array.isArray(extensions.recovered_fields)
    ? extensions.recovered_fields : {};
  return <WorkspaceInfo kind="warning">
    <p>部分资料的格式需要核对。可以继续编辑；未采用的原始内容已保留。</p>
    <Disclosure title={`查看需要核对的内容（${warnings.length}）`}>
      <ul className="grid min-w-0 gap-4">
        {warnings.map((warning, index) => {
          const values = Object.entries(recovered).filter(([key]) => key === warning.path || key.startsWith(`${warning.path}#`));
          return <li key={`${warning.path}-${index}`} className="grid min-w-0 gap-1">
            <strong>{fieldLabel(warning.path)}</strong>
            <span>{warning.message}</span>
            {values.map(([key, value]) => <div key={key} className="min-w-0 break-words">
              <span>原始内容：</span><OriginalValue value={value}/>
            </div>)}
          </li>;
        })}
      </ul>
    </Disclosure>
  </WorkspaceInfo>;
}
