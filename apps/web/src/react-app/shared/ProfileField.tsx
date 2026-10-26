import { useId } from "react";
import { FieldInput, ChoiceSelect, Textarea, Label } from "../controls";
export default function ProfileField({ field }: any) {
  const id = useId(),
    long =
      field.kind === "lines" ||
      String(field.value).length > 80 ||
      /background|biography|notes|description/.test(field.path.at(-1));
  const set = (e: any) =>
    (field.value =
      field.kind === "number" && e.target.value !== ""
        ? Number(e.target.value)
        : e.target.value);
  return (
    <Label htmlFor={id} className={long ? "full-field" : ""}>
      {field.label}
      {field.kind === "boolean" || field.kind === "select" ? (
        <ChoiceSelect
          id={id}
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
          value={field.value ?? ""}
          rows={3}
          aria-label={field.label}
          placeholder={field.kind === "lines" ? "每行一项，可留空" : "尚未填写"}
          onChange={set}
        />
      ) : (
        <FieldInput
          id={id}
          value={field.value}
          type={field.kind === "number" ? "number" : "text"}
          min={field.min}
          max={field.max}
          step={field.kind === "number" ? "any" : undefined}
          aria-label={field.label}
          placeholder="尚未填写"
          onChange={set}
        />
      )}
      {field.max === 1 && (
        <small className="muted">0–1，数值越大表示这项倾向越明显。</small>
      )}
    </Label>
  );
}
