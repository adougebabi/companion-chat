import { useId, useState } from "react";
import Combobox from "./Combobox";
import { Slider } from "@/components/ui/slider";
import { FieldInput, ChoiceSelect, Textarea, Label } from "../controls";

const styleOptions: Record<string, string[]> = {
  response_style: ["自然随和", "简洁直接", "温和细腻"],
  conflict_style: ["先倾听再讨论", "直接表达分歧", "温和协商"],
  refusal_style: ["温和而坚定", "清楚说明原因", "简洁直接"],
};
const genderOptions = [
  { value: "female", label: "女", aliases: ["女", "女性"] },
  { value: "male", label: "男", aliases: ["男", "男性"] },
  { value: "non_binary", label: "非二元", aliases: ["nonbinary", "非二元"] },
  { value: "unspecified", label: "不透露", aliases: ["不透露"] },
];
type Choice = { value: string; label: string; aliases?: string[] };

// The manual choice is tied to the value being edited, so a later external
// preview/draft update can select a known option without stale local state.
function CustomChoice({ id, label, value, options, disabled, onChange, creation }: {
  id: string; label: string; value: unknown; options: Choice[];
  disabled: boolean; onChange: (value: string) => void; creation: boolean;
}) {
  const [manualValue, setManualValue] = useState<unknown>(undefined);
  const current = String(value ?? "");
  const normalized = current.trim().toLowerCase();
  const known = options.find((option) =>
    [option.value, ...(option.aliases || [])].some((alias) => alias.toLowerCase() === normalized),
  );
  const custom = (current !== "" && !known) ||
    (manualValue !== undefined && Object.is(manualValue, value));
  return <>
    <ChoiceSelect id={id} aria-label={label} disabled={disabled}
      contentPosition={creation ? "popper" : undefined} contentClassName={creation ? "creation-select-content" : undefined}
      value={custom ? "__custom_choice" : current}
      onChange={(event: any) => {
        const next = event.target.value;
        if (next === "__custom_choice") setManualValue(value);
        else { setManualValue(undefined); onChange(next); }
      }}>
      <option value="">尚未填写</option>
      {options.map((option) => <option key={option.value}
        value={known === option ? current : option.value}>{option.label}</option>)}
      <option value="__custom_choice">自定义</option>
    </ChoiceSelect>
    {custom && <FieldInput value={current} disabled={disabled}
      aria-label={`自定义${label}`} placeholder={`填写${label}`}
      onChange={(event: any) => { setManualValue(event.target.value); onChange(event.target.value); }}/>}
  </>;
}

export default function ProfileField({ field, disabled = false, creation = false }: any) {
  const id = useId(),
    long = field.kind === "lines" || /background|biography|notes|description/.test(field.path.at(-1)),
    key = field.path.at(-1);
  const set = (e: any) => (field.value =
    field.kind === "number" && e.target.value !== "" ? Number(e.target.value) : e.target.value);
  return (
    <Label htmlFor={id} className={long ? "full-field" : ""}>
      {field.label}
      {key === "gender" ? <CustomChoice id={id} label={field.label} value={field.value}
        options={genderOptions} disabled={disabled} creation={creation} onChange={(value) => set({ target: { value } })}/>
      : key === "timezone" ? <Combobox id={id} label={field.label} value={field.value} disabled={disabled} allowCustom
        options={["Asia/Shanghai", "Asia/Hong_Kong", "Asia/Tokyo", "Asia/Singapore", "Europe/London", "Europe/Paris", "America/New_York", "America/Los_Angeles", "Australia/Sydney", "UTC", ...(Intl.supportedValuesOf?.("timeZone") || [])]}
        onChange={(value: string) => set({ target: { value } })}/>
      : styleOptions[key] ? <CustomChoice id={id} label={field.label} value={field.value}
        options={styleOptions[key].map((value) => ({ value, label: value }))} disabled={disabled} creation={creation}
        onChange={(value) => set({ target: { value } })}/>
      : ["directness", "initiative"].includes(key) && field.max === 1 ? <div className="grid gap-3">
        <Slider min={0} max={1} step={0.01} disabled={disabled} aria-label={field.label}
          value={[field.value === "" ? 0.5 : Number(field.value)]} onValueChange={([value]) => set({ target: { value } })}/>
        <span className="text-sm text-muted-foreground">{field.value === "" ? "尚未填写" : `${Number(field.value) < 0.35 ? "低" : Number(field.value) > 0.65 ? "高" : "中"} · ${field.value}`}</span>
      </div>
      : field.kind === "boolean" || field.kind === "select" ? <ChoiceSelect id={id} disabled={disabled}
        contentPosition={creation ? "popper" : undefined} contentClassName={creation ? "creation-select-content" : undefined}
        aria-label={field.label} value={field.value} onChange={set}>
        {(field.kind === "boolean" ? [{ value: true, label: "是" }, { value: false, label: "否" }] : field.options)
          .map((option: any) => <option key={String(option.value)} value={option.value}>{option.label}</option>)}
      </ChoiceSelect>
      : long ? <Textarea id={id} disabled={disabled} value={field.value ?? ""} rows={3} aria-label={field.label}
        placeholder={field.kind === "lines" ? "每行一项，可留空" : field.label + "，可留空"} onChange={set}/>
      : <FieldInput id={id} disabled={disabled} value={field.value} type={field.kind === "number" ? "number" : "text"}
        min={field.min} max={field.max} step={field.kind === "number" ? "any" : undefined}
        aria-label={field.label} placeholder={
          key === "punctuation_style" ? "例如：少用感叹号、常用省略号" :
          key === "humor_style" ? "例如：轻松、自嘲或冷幽默" :
          key === "response_delay" && field.kind === "number" ? "填写延迟秒数，可留空" : field.label + "，可留空"
        } onChange={set}/>}
      {key === "response_delay" && field.kind === "number" && <small className="muted">单位：秒。0 表示不额外等待；数值越大，等待时间越长。</small>}
      {field.max === 1 && <small className="muted">0–1，数值越大表示这项倾向越明显。</small>}
    </Label>
  );
}
