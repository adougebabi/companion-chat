import { useId } from "react";
import { FieldInput, ChoiceSelect, Textarea, Label } from "../controls";
import { autonomyActionChoices } from "@/v3/proto/components/configuration-fields.js";
export default function SettingsField({ field }: any) {
  const id = useId();
  if (field.type === "action-types") {
    const choices = new Map<string, string>(
      autonomyActionChoices as [string, string][],
    );
    for (const value of Array.isArray(field.value) ? field.value : [])
      if (!choices.has(value)) choices.set(value, value);
    return (
      <fieldset className="action-type-field">
        <legend>{field.label}</legend>
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
                field.value = [...values];
              }}
            />
            {label}
          </Label>
        ))}
      </fieldset>
    );
  }
  return (
    <Label htmlFor={id} className="settings-field">
      {field.label}
      {field.type === "select" ? (
        <ChoiceSelect
          id={id}
          aria-label={field.label}
          value={field.value}
          onChange={(e: any) => (field.value = e.target.value)}
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
          value={String(field.value)}
          rows={3}
          onChange={(e) => (field.value = e.target.value)}
        />
      ) : (
        <FieldInput
          id={id}
          aria-label={field.label}
          type={field.type || "text"}
          className={field.type === "checkbox" ? "switch" : undefined}
          value={field.type === "checkbox" ? undefined : field.value}
          checked={!!field.value}
          min={field.min}
          max={field.max}
          step={field._definition?.step || "any"}
          onChange={(e: any) =>
            (field.value =
              field.type === "checkbox"
                ? e.target.checked
                : field.type === "number" && e.target.value !== ""
                  ? Number(e.target.value)
                  : e.target.value)
          }
        />
      )}
    </Label>
  );
}
