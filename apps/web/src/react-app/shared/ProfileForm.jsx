import { useState } from "react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { selector } from "@/state/view-model";
import { Disclosure } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
import ProfileField from "@/react-app/shared/ProfileField";
export default function ProfileForm(input = {}) {
  const __props = {
    disabled: false,
    ...input,
  };
  const props = __props;
  const basic = new Set([
    "name",
    "gender",
    "age",
    "occupation",
    "residence",
    "timezone",
  ]);
  const expression = new Set([
    "core_values",
    "response_style",
    "message_length",
    "directness",
    "initiative",
    "conflict_style",
    "refusal_style",
  ]);
  const life = new Set([
    "background_story",
    "social_background",
    "preferences",
    "life_habits",
    "character_constraints",
  ]);
  const category = (field) =>
    props.replica
      ? field.path[0] === "identity" &&
        [
          "name",
          "nickname",
          "gender",
          "age",
          "occupation",
          "residence",
          "timezone",
        ].includes(field.path.at(-1))
        ? "basic"
        : expression.has(field.path.at(-1))
          ? "expression"
          : field.path[0] === "life_profile" ||
              ["background_story", "background", "biography"].includes(
                field.path.at(-1),
              )
            ? "life"
            : "more"
      : basic.has(field.path.at(-1)) && field.path[0] === "identity"
        ? "basic"
        : expression.has(field.path.at(-1))
          ? "expression"
          : life.has(field.path[1]) || life.has(field.path.at(-1))
            ? "life"
            : "more";
  const sections = {
    value: [
      ["basic", "基本资料", "名字、身份与生活所在地。"],
      [
        "expression",
        "性格与说话方式",
        "这些表达偏好会和性格设定一起影响相处方式。",
      ],
      ["life", "生活", "核对习惯、偏好和相处边界。"],
      ["more", "更多设定", "保留当前资料中的其他字段。"],
    ]
      .map(([key, title, note]) => ({
        key,
        title,
        note,
        fields: props.fields.filter((field) => category(field) === key),
      }))
      .filter(
        (group) =>
          group.fields.length &&
          (!props.primaryOnly || group.key !== "more") &&
          (!props.additionalOnly || group.key === "more"),
      ),
  };
  const [localSelected, setLocalSelected] = useState(
    sections.value[0]?.key || "basic",
  );
  const requested = props.group || localSelected;
  const selected = sections.value.some((group) => group.key === requested)
    ? requested
    : sections.value[0]?.key;
  const select = (value) => {
    setLocalSelected(value);
    props.onGroupChange?.(value);
  };
  return (
    <fieldset
      className="character-fieldset character-stack"
      disabled={__props.disabled}
    >
      <Tabs value={selected} onValueChange={select}>
        <TabsList
          variant={props.replica ? "line" : "default"}
          className={
            props.replica
              ? "replica-segment-tabs"
              : "h-auto group-data-horizontal/tabs:h-auto max-w-full flex-wrap"
          }
        >
          {sections.value.map((group) => (
            <TabsTrigger key={group.key} value={group.key}>
              {props.replica
                ? {
                    basic: "基本资料",
                    expression: "性格与表达",
                    life: "生活设定",
                    more: "更多设定",
                  }[group.key]
                : group.title}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <Fragment>
        {listValues(sections.value).map((group) => {
          return (
            <section
              key={group.key}
              hidden={selected !== group.key}
              className="character-stack"
            >
              {!props.replica && (
                <>
                  <h3>{displayText(group.title)}</h3>
                  <p className="muted">{group.note}</p>
                </>
              )}
              <div className="character-field-grid">
                {group.fields.map((field) => (
                  <ProfileField
                    key={JSON.stringify(field.path)}
                    field={field}
                    disabled={props.disabled}
                  />
                ))}
              </div>
            </section>
          );
        })}
      </Fragment>
    </fieldset>
  );
}
