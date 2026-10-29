import {Field,FieldLabel,FieldSet,FieldLegend,FieldGroup,FieldError} from '@/components/ui/field';
import { useId } from "react";
import { FieldInput, ChoiceSelect, Textarea, Label } from "../controls";
import { autonomyActionChoices,configurationChoiceLabel,configurationPoints } from "@/v3/proto/components/configuration-fields.js";
export default function SettingsField({ field }: any) {
  const id = useId();
  const validationId=field.validationError?id+'-error':undefined,identity=JSON.stringify([field._settingKey,field._key]);
  const update=(value:any)=>{field.value=value;field.validationError='';};
  if(field.type==="boolean-map"){const values=field.value&&typeof field.value==='object'&&!Array.isArray(field.value)?field.value:{};return <FieldSet data-setting-field={identity} aria-invalid={!!field.validationError} aria-describedby={validationId} className="full-field"><FieldLegend>{field.label}</FieldLegend><p className="text-sm text-muted-foreground">选择允许辅助判断的场景；未覆盖的媒体决策沿用系统默认。</p><FieldGroup className="grid gap-3 sm:grid-cols-2">{[...new Set([...configurationPoints,...Object.keys(values)])].map((key,index)=><Field key={key} orientation="horizontal"><FieldInput id={id+index} type="checkbox" checked={values[key]??true} onChange={(event:any)=>update({...(typeof field.value==='object'?values:Object.fromEntries(configurationPoints.map(point=>[point,true]))),[key]:event.target.checked})}/><FieldLabel htmlFor={id+index}>{configurationChoiceLabel(key)}</FieldLabel></Field>)}</FieldGroup>{field.validationError&&<FieldError id={validationId} data-feedback-kind="error">{field.validationError}</FieldError>}</FieldSet>;}
  if (field.type === "action-types") {
    const choices = new Map<string, string>(
      autonomyActionChoices as [string, string][],
    );
    for (const value of Array.isArray(field.value) ? field.value : [])
      if (!choices.has(value)) choices.set(value, value);
    return (
      <FieldSet data-setting-field={identity} aria-invalid={!!field.validationError} aria-describedby={validationId} className="action-type-field">
        <FieldLegend>{field.label}</FieldLegend><FieldGroup>
        {[...choices].map(([value, label], index) => (
          <Label key={value} htmlFor={`${id}-${index}`} className="check-field">
            <FieldInput
              id={`${id}-${index}`}
              type="checkbox"
              checked={
                Array.isArray(field.value) && field.value.includes(value)
              }
              onChange={(event) => {
                const values = new Set(
                  Array.isArray(field.value) ? field.value : [],
                );
                if (event.target.checked) values.add(value);
                else values.delete(value);
                update([...values]);
              }}
            />
            {label}
          </Label>
        ))}
      </FieldGroup>{field.validationError&&<FieldError id={validationId} data-feedback-kind="error">{field.validationError}</FieldError>}</FieldSet>
    );
  }
  return (
    <Field data-setting-field={identity} data-invalid={!!field.validationError} orientation={field.type === "checkbox" ? "horizontal" : "vertical"} className={field.type==='json'?'min-w-0 full-field':'min-w-0'}>
      <FieldLabel htmlFor={id}>{field.label}</FieldLabel>
      {field.type === "select" ? (
        <ChoiceSelect
          id={id}
          aria-label={field.label}
          aria-invalid={!!field.validationError}
          aria-describedby={validationId}
          value={field.value}
          onChange={(e: any) => update(e.target.value)}
        >
          {field.options.map((option: string) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </ChoiceSelect>
      ) : field.type === "json" ? (
        <Textarea
          id={id}
          aria-label={field.label}
          aria-invalid={!!field.validationError}
          aria-describedby={validationId}
          value={String(field.value)}
          rows={6}
          className="min-h-36 font-mono text-sm"
          placeholder={field._definition?.type==='workflow'?'{"1":{"class_type":"CLIPTextEncode","inputs":{"text":"{{prompt}}"}}}':'例如 {}；每个分支填写完整工作流节点对象'}
          onChange={(e) => update(e.target.value)}
        />
      ) : (
        <FieldInput
          id={id}
          aria-label={field.label}
          aria-invalid={!!field.validationError}
          aria-describedby={validationId}
          type={field.type || "text"}
          className={field.type === "checkbox" ? "switch" : undefined}
          value={field.type === "checkbox" ? undefined : field.value}
          checked={!!field.value}
          min={field.min}
          max={field.max}
          step={field._definition?.step || "1"}
          onChange={(e: any) =>
            update(
              field.type === "checkbox"
                ? e.target.checked
                : field.type === "number" && e.target.value !== ""
                  ? Number(e.target.value)
                  : e.target.value)
          }
        />
      )}
      {field.type==='number'&&<p className="text-sm text-muted-foreground">{[field.min!=null?'最小 '+field.min:'',field.max!=null?'最大 '+field.max:''].filter(Boolean).join(' · ')}</p>}
      {field.type==='json'&&<p className="text-sm text-muted-foreground">填写 JSON 对象；保存前会校验格式和工作流节点。可选覆盖留空时沿用默认。</p>}
      {field.validationError&&<FieldError id={validationId} data-feedback-kind="error">{field.validationError}</FieldError>}
    </Field>
  );
}
