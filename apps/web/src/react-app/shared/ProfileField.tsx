import Combobox from './Combobox';
import {Slider} from '@/components/ui/slider';
import { useId, useState } from "react";
import { FieldInput, ChoiceSelect, Textarea, Label } from "../controls";
export default function ProfileField({ field, disabled=false }: any) {
  const id = useId(),
    long =
      field.kind === "lines" ||
      /background|biography|notes|description/.test(field.path.at(-1));
  const [customGender,setCustomGender]=useState(!['','female','male','non_binary','unspecified','女','女性','男','男性','非二元','不透露'].includes(field.value));
  const key=field.path.at(-1);
  const styles:Record<string,string[]>={response_style:['自然随和','简洁直接','温和细腻'],conflict_style:['先倾听再讨论','直接表达分歧','温和协商'],refusal_style:['温和而坚定','清楚说明原因','简洁直接']};
  const set = (e: any) =>
    (field.value =
      field.kind === "number" && e.target.value !== ""
        ? Number(e.target.value)
        : e.target.value);
  return (
    <Label htmlFor={id} className={long ? "full-field" : ""}>
      {field.label}
      {key==='gender'?<><ChoiceSelect id={id} aria-label={field.label} disabled={disabled} value={customGender?'__custom_gender':field.value} onChange={(event:any)=>{const value=event.target.value;setCustomGender(value==='__custom_gender');if(value!=='__custom_gender')set(event);}}>{[['','尚未填写'],['female','女'],['male','男'],['non_binary','非二元'],['unspecified','不透露'],['__custom_gender','自定义']].map(([value,label])=>{const aliases={female:['女','女性'],male:['男','男性'],non_binary:['非二元'],unspecified:['不透露']} as Record<string,string[]>;return <option key={value} value={aliases[value]?.includes(field.value)?field.value:value}>{label}</option>;})}</ChoiceSelect>{customGender&&<FieldInput value={field.value} disabled={disabled} aria-label="自定义性别" onChange={set}/>}</>:key==='timezone'?<Combobox id={id} label={field.label} value={field.value} disabled={disabled} allowCustom options={['Asia/Shanghai','Asia/Hong_Kong','Asia/Tokyo','Asia/Singapore','Europe/London','Europe/Paris','America/New_York','America/Los_Angeles','Australia/Sydney','UTC',...(Intl.supportedValuesOf?.('timeZone')||[])]} onChange={(value:string)=>set({target:{value}})}/>:styles[key]?<Combobox id={id} label={field.label} value={field.value} disabled={disabled} allowCustom options={styles[key]} onChange={(value:string)=>set({target:{value}})}/>:['directness','initiative'].includes(key)&&field.max===1?<div className="grid gap-3"><Slider min={0} max={1} step={0.01} disabled={disabled} aria-label={field.label} value={[field.value===''?0.5:Number(field.value)]} onValueChange={([value])=>set({target:{value}})}/><span className="text-sm text-muted-foreground">{field.value===''?'尚未填写':`${Number(field.value)<0.35?'低':Number(field.value)>0.65?'高':'中'} · ${field.value}`}</span></div>:field.kind === "boolean" || field.kind === "select" ? (
        <ChoiceSelect
          id={id}
          disabled={disabled}
          aria-label={field.label}
          value={field.value}
          onChange={set}
        >
          {(field.kind === "boolean"
            ? [
                { value: true, label: "是" },
                { value: false, label: "否" },
              ]
            : field.options
          ).map((option: any) => (
            <option key={String(option.value)} value={option.value}>
              {option.label}
            </option>
          ))}
        </ChoiceSelect>
      ) : long ? (
        <Textarea
          id={id}
          disabled={disabled}
          value={field.value ?? ""}
          rows={3}
          aria-label={field.label}
          placeholder={field.kind === "lines" ? "每行一项，可留空" : field.label+'，可留空'}
          onChange={set}
        />
      ) : (
        <FieldInput
          id={id}
          disabled={disabled}
          value={field.value}
          type={field.kind === "number" ? "number" : "text"}
          min={field.min}
          max={field.max}
          step={field.kind === "number" ? "any" : undefined}
          aria-label={field.label}
          placeholder={field.label+'，可留空'}
          onChange={set}
        />
      )}
      {field.max === 1 && (
        <small className="muted">0–1，数值越大表示这项倾向越明显。</small>
      )}
    </Label>
  );
}
