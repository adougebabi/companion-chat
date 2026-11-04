import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { ChoiceSelect, Disclosure, FieldInput, Label, Textarea } from "../controls";
import { wardrobeCategories, wardrobeSlots, wardrobeOwnershipOptions, wardrobeCategoryLabel, wardrobeSlotLabel } from "@/lib/wardrobe-form";
import { creationPreviewLabel } from "./CreationPreviewValue";
import CreationScaleHint, { CreationScaleEnds, creationScaleHints } from "./CreationScaleHint";

// Suggested fields are added only after the user explicitly chooses one.
// Existing keys, including unknown extensions, always stay in the draft.
const suggestions = {
  appearance: [["description", ""], ["hair_style", ""], ["eyes", ""], ["body_type", ""], ["physical_features", {}], ["style_preferences", {}]],
  social_background: [["family", ""], ["education", ""], ["occupation", ""], ["background", ""]],
  preferences: [["food", ""], ["drinks", ""], ["music", ""], ["books", ""], ["movies", ""], ["hobbies", ""]],
  media_preferences: [["music", ""], ["books", ""], ["movies", ""], ["topics", ""]],
  initial_relationships: [["target_actor_id", ""], ["role", ""], ["summary", ""], ["metrics", {}], ["emotional_association", ""]],
  relationship_seeds: [["target_actor_id", "actor_user"], ["type", ""], ["relationship", ""], ["description", ""]],
  initial_goals: [["description", ""], ["importance", 0.5], ["urgency", 0.5], ["scope", ""]],
  initial_intentions: [["action", ""], ["confidence", 0.5], ["goal_index", 0]],
  wardrobe_items: [["description", ""], ["category", ""], ["slot", ""], ["ownership", "unknown"], ["available", true], ["currently_worn", false]],
  personality_system: [["profiles", []], ["switching", {}], ["influence", {}], ["core_relationship", ""], ["core_conflict", ""], ["takeover_rules", []]],
  profiles: [["id", ""], ["name", ""], ["identity", {}], ["personality", {}], ["behavioral_policy", {}], ["voice", {}], ["fears", []], ["desires", []]],
  metrics: [["familiarity", 0.5], ["trust", 0.5], ["affection", 0.5]],
};
const labels = {
  hair_style: "发型", body_type: "体态", physical_features: "外貌特征", style_preferences: "风格偏好",
  education: "教育经历", hobbies: "爱好", category: "分类", slot: "穿着部位", ownership: "归属",
  available: "是否可用", currently_worn: "是否正在穿着", id: "人格标识", name: "名称",
  identity: "身份", personality: "性格", behavioral_policy: "表达与相处", voice: "声音与说话方式",
  fears: "担忧", desires: "愿望", core_relationship: "人格间关系", core_conflict: "核心分歧",
};
const labelFor = (field, index = 0) => labels[field] || creationPreviewLabel(field, index);
const clone = (value) => JSON.parse(JSON.stringify(value));
const listTemplates = {
  wardrobe_items: ["衣物", { category: "", slot: "", description: "", ownership: "unknown", available: true, currently_worn: false }],
  initial_relationships: ["与你的关系", { target_actor_id: "actor_user", role: "", summary: "" }],
  initial_goals: ["目标", { description: "" }],
  initial_intentions: ["意图", { action: "" }],
  relationship_seeds: ["与你的关系线索", { target_actor_id: "actor_user", relationship: "" }],
  recurring_commitments: ["约定", { description: "" }],
  profiles: ["人格档案", { id: "", name: "" }],
  life_habits: ["习惯", ""],
  character_constraints: ["边界", ""],
};

function ObjectFields({ value, field, disabled, onChange }) {
  const [adding, setAdding] = useState(false), [extraKey, setExtraKey] = useState("");
  const entries = Object.entries(value), available = (suggestions[field] || []).filter(([key]) => !Object.hasOwn(value, key));
  const addKey = () => {
    const key = extraKey.trim();
    if (!key || Object.hasOwn(value, key)) return;
    onChange({ ...value, [key]: "" });
    setExtraKey("");
    setAdding(false);
  };
  return <div className="creation-structure-fields">
    {entries.map(([key, item], index) => <ValueEditor key={key} value={item} field={key} parentField={field}
      label={labelFor(key, index)} disabled={disabled} onChange={(next) => onChange({ ...value, [key]: next })}/>)}
    {!entries.length && <p className="muted">尚未填写，可从下方补充。</p>}
    <div className="creation-structure-add">
      {available.length > 0 && <ChoiceSelect aria-label="补充常用字段" contentPosition="popper" contentClassName="creation-select-content" value="" disabled={disabled} onChange={(event) => {
        const item = available.find(([key]) => key === event.target.value);
        if (item) onChange({ ...value, [item[0]]: clone(item[1]) });
      }}>
        <option value="">补充常用字段</option>
        {available.map(([key]) => <option key={key} value={key}>{labelFor(key)}</option>)}
      </ChoiceSelect>}
      {!adding ? <Button type="button" variant="outline" disabled={disabled} onClick={() => setAdding(true)}>添加自定义字段</Button>
        : <div className="creation-structure-custom">
          <Label>字段名称<FieldInput aria-label="自定义字段名称" value={extraKey} disabled={disabled}
            placeholder="例如：喜欢的季节" onChange={(event) => setExtraKey(event.target.value)}/></Label>
          <Button type="button" variant="outline" disabled={disabled || !extraKey.trim() || Object.hasOwn(value, extraKey.trim())} onClick={addKey}>添加字段</Button>
          <Button type="button" variant="ghost" onClick={() => { setAdding(false); setExtraKey(""); }}>取消</Button>
        </div>}
    </div>
  </div>;
}

function ArrayFields({ value, field, disabled, onChange }) {
  const rows = useRef({ keys: value.map((_, index) => index), next: value.length });
  while (rows.current.keys.length < value.length) rows.current.keys.push(rows.current.next++);
  const add = (item) => { rows.current.keys.push(rows.current.next++); onChange([...value, clone(item)]); };
  const template = listTemplates[field];
  return <div className="creation-structure-list">
    {value.map((item, index) => <fieldset className="creation-structure-item" key={rows.current.keys[index]}>
      <legend>第 {index + 1} 项</legend>
      <ValueEditor value={item} field={field} label={`第 ${index + 1} 项`} disabled={disabled} root
        onChange={(next) => onChange(value.map((before, i) => i === index ? next : before))}/>
      <Button type="button" variant="ghost" disabled={disabled} aria-label={`移除第 ${index + 1} 项`} onClick={() => {
        rows.current.keys.splice(index, 1); onChange(value.filter((_, i) => i !== index));
      }}>移除此项</Button>
    </fieldset>)}
    {!value.length && <p className="muted">还没有内容，可以添加。</p>}
    {template ? <Button type="button" variant="outline" disabled={disabled} onClick={() => add(template[1])}>添加{template[0]}</Button>
      : <div className="creation-structure-add">
        <Button type="button" variant="outline" disabled={disabled} onClick={() => add("")}>添加文字</Button>
        <Button type="button" variant="outline" disabled={disabled} onClick={() => add({})}>添加一组内容</Button>
        {value.some((item) => typeof item === "number") && <Button type="button" variant="outline" disabled={disabled} onClick={() => add(0)}>添加数值</Button>}
        {value.some((item) => typeof item === "boolean") && <Button type="button" variant="outline" disabled={disabled} onClick={() => add(false)}>添加是 / 否</Button>}
      </div>}
  </div>;
}

function ValueEditor({ value, field, parentField, label, disabled, onChange, root = false }) {
  const id = useId(), [customWardrobe, setCustomWardrobe] = useState(false);
  if (value == null) return <div className="creation-structure-field"><Label htmlFor={id}>{label}</Label>
    <ChoiceSelect id={id} aria-label={`${label}的补充类型`} contentPosition="popper" contentClassName="creation-select-content" value="" disabled={disabled} onChange={(event) => {
      const choices = { text: "", number: 0, boolean: false, list: [], record: {} };
      if (Object.hasOwn(choices, event.target.value)) onChange(clone(choices[event.target.value]));
    }}><option value="">尚未填写</option><option value="text">补充文字</option><option value="number">补充数值</option>
      <option value="boolean">选择是或否</option><option value="list">补充列表</option><option value="record">补充一组内容</option></ChoiceSelect>
  </div>;
  if (Array.isArray(value)) {
    const content = <ArrayFields value={value} field={field} disabled={disabled} onChange={onChange}/>;
    return root ? content : <Disclosure title={`${label}（${value.length} 项）`} open>{content}</Disclosure>;
  }
  if (typeof value === "object") {
    const content = <ObjectFields value={value} field={field} disabled={disabled} onChange={onChange}/>;
    return root ? content : <Disclosure title={label} open>{content}</Disclosure>;
  }
  if (typeof value === "boolean") return <div className="creation-structure-field"><Label htmlFor={id}>{label}</Label>
    <ChoiceSelect id={id} aria-label={label} contentPosition="popper" contentClassName="creation-select-content" value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
      <option value={true}>是</option><option value={false}>否</option>
    </ChoiceSelect></div>;
  if (["initial_relationships", "relationship_seeds"].includes(parentField) && field === "target_actor_id" && typeof value === "string") {
    const isUser = ["owner", "actor_user"].includes(value);
    return <div className="creation-structure-field"><Label htmlFor={id}>关系对象</Label>
      <ChoiceSelect id={id} aria-label="关系对象" value={value} disabled={disabled} contentPosition="popper" contentClassName="creation-select-content"
        onChange={(event) => onChange(event.target.value)}>
        <option value={isUser ? value : "actor_user"}>你</option>
        {!isUser && <option value={value}>{value ? `已记录对象（${value}）` : "尚未填写"}</option>}
      </ChoiceSelect>
      <small className="muted">初始化时的关系对象为你。</small>
    </div>;
  }
  if (parentField === "personality_system" && field === "mode" && typeof value === "string") return <div className="creation-structure-field">
    <Label htmlFor={id}>{label}</Label><ChoiceSelect id={id} aria-label={label} value={value} disabled={disabled}
      contentPosition="popper" contentClassName="creation-select-content" onChange={(event) => onChange(event.target.value)}>
      <option value="single">单一人格</option><option value="multiple">多重人格</option>
      {!["single", "multiple"].includes(value) && <option value={value}>{value || "尚未填写"}</option>}
    </ChoiceSelect>
  </div>;
  if (parentField === "wardrobe_items" && typeof value === "string" && ["category", "slot", "ownership"].includes(field)) {
    const options = field === "category" ? wardrobeCategories.filter((item) => item.kind === "wearable") :
      field === "slot" ? wardrobeSlots : wardrobeOwnershipOptions;
    const extraLabel = field === "category" ? ({ upper: "上装", lower: "下装", full: "连身服装" }[value] || wardrobeCategoryLabel(value)) :
      field === "slot" ? wardrobeSlotLabel(value) : value;
    return <div className="creation-structure-field"><Label htmlFor={id}>{label}</Label>
      <ChoiceSelect id={id} aria-label={label} value={customWardrobe ? "__custom_wardrobe" : value} disabled={disabled} contentPosition="popper" contentClassName="creation-select-content"
        onChange={(event) => {
          if (event.target.value === "__custom_wardrobe") setCustomWardrobe(true);
          else { setCustomWardrobe(false); onChange(event.target.value); }
        }}>
        <option value="">尚未填写</option>
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        {value && !options.some((option) => option.value === value) && <option value={value}>{extraLabel}</option>}
        <option value="__custom_wardrobe">自定义</option>
      </ChoiceSelect>
      {customWardrobe && <FieldInput aria-label={`自定义${label}`} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}/>}
    </div>;
  }
  if (typeof value === "number" && creationScaleHints[field] && value >= 0 && value <= 1) return <div className="creation-trait">
    <div><strong>{label} <CreationScaleHint field={field} label={label}/></strong><span>{value.toFixed(2)}</span></div>
    <Slider aria-label={label} min={0} max={1} step={0.01} value={[value]} disabled={disabled} onValueChange={([next]) => onChange(next)}/>
    <CreationScaleEnds field={field}/>
  </div>;
  return <div className="creation-structure-field"><Label htmlFor={id}>{label}</Label>
    {typeof value === "number" ? <FieldInput id={id} aria-label={label} type="number" step="any" value={value} disabled={disabled}
      onChange={(event) => onChange(event.target.value === "" ? null : Number(event.target.value))}/>
      : <Textarea id={id} aria-label={label} rows={2} value={String(value)} disabled={disabled}
        placeholder="可留空" onChange={(event) => onChange(event.target.value)}/>}
  </div>;
}

export default function CreationStructureEditor({ value, field, disabled, onChange }) {
  return <div className="creation-structure-editor"><ValueEditor value={value} field={field} label={labelFor(field)}
    root disabled={disabled} onChange={onChange}/></div>;
}
