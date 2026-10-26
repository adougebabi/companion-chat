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
  const primary = {
    identity: new Set([
      "name",
      "gender",
      "age",
      "occupation",
      "residence",
      "timezone",
      "background_story",
      "core_values",
    ]),
    behavioral_policy: new Set([
      "response_style",
      "message_length",
      "directness",
      "initiative",
      "conflict_style",
      "refusal_style",
    ]),
    life_profile: new Set([
      "social_background",
      "preferences",
      "life_habits",
      "character_constraints",
    ]),
  };
  const sections = selector(() =>
    [
      ["identity", "身份与背景", "更多身份资料"],
      ["behavioral_policy", "表达与相处", "更多表达设置"],
      ["life_profile", "生活设定", "更多生活与外貌设定"],
    ]
      .map(([key, title, moreTitle]) => {
        const fields = props.fields.filter((f) => f.path[0] === key);
        return {
          key,
          title,
          moreTitle,
          fields,
          primary: fields.filter((f) => primary[key].has(f.path[1])),
          more: fields.filter((f) => !primary[key].has(f.path[1])),
        };
      })
      .filter((g) => g.fields.length),
  );
  return (
    <fieldset
      className="character-fieldset character-stack"
      disabled={__props.disabled}
    >
      <Fragment>
        {listValues(sections.value).map((group) => {
          return (
            <section key={group.key} className="character-stack">
              <h3>{displayText(group.title)}</h3>
              {group.primary.length ? (
                <div key={0} className="character-field-grid">
                  <Fragment>
                    {listValues(group.primary).map((field) => {
                      return (
                        <ProfileField
                          key={JSON.stringify(field.path)}
                          field={field}
                        />
                      );
                    })}
                  </Fragment>
                </div>
              ) : null}
              {group.more.length ? (
                <Disclosure key={1} title={<>{displayText(group.moreTitle)}</>}>
                  <div className="character-field-grid approved-more-fields">
                    <Fragment>
                      {listValues(group.more).map((field) => {
                        return (
                          <ProfileField
                            key={JSON.stringify(field.path)}
                            field={field}
                          />
                        );
                      })}
                    </Fragment>
                  </div>
                </Disclosure>
              ) : null}
            </section>
          );
        })}
      </Fragment>
    </fieldset>
  );
}
