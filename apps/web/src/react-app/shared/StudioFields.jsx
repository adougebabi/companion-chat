import { useState } from "react";
import { Plus, X, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  FieldInput,
  Textarea,
  ChoiceSelect,
  Label,
  Disclosure,
} from "../controls";
import RecordEditor from "./RecordEditor";
import WorkspaceInfo from "./WorkspaceInfo";
import { useWorkspace } from "../page";
import useWorkspaceRead from "./useWorkspaceRead";
import { MediaReader, MediaMore } from "./MediaParts";
import {
  wardrobeCategoryOptions,
  wardrobeSlotOptions,
} from "@/lib/wardrobe-form";
import { randomId } from "@/random-id.mjs";
import { eventEffectLabels } from "@/v3/proto/components/event-definition.js";
const clone = (value) => JSON.parse(JSON.stringify(value));
const texts = {
  world: [
    ["background", "世界背景", true],
    ["rules", "基础规则", true],
  ],
  place: [
    ["description", "场景描述", true],
    ["alias", "别名"],
    ["floor", "楼层"],
    ["atmosphere", "氛围"],
    ["furniture", "固定陈设", true],
    ["layout", "布局描述", true],
  ],
  npc: [
    ["role", "身份 / 职业"],
    ["background", "人物背景", true],
  ],
  visual_style: [
    ["appearance", "外貌要点", true],
    ["clothing", "服装风格", true],
    ["constraints", "一致性约束", true],
    ["references", "参考图说明", true],
  ],
  outfit: [["description", "搭配说明", true]],
};
export default function StudioFields({
  kind,
  value,
  onChange,
  disabled = false,
}) {
  const body =
      value && typeof value === "object" && !Array.isArray(value) ? value : {},
    set = (key, next) => onChange({ ...body, [key]: next });
  const field = ([key, label, multiline = false]) =>
    typeof body[key] !== "string" && body[key] !== undefined ? (
      <div key={key}>
        <WorkspaceInfo>
          {label}仍保留原结构，可在完整JSON中明确修订。
        </WorkspaceInfo>
        <RecordEditor
          value={{ [key]: body[key] }}
          onChange={(next) => set(key, next[key])}
          disabled={disabled}
        />
      </div>
    ) : (
      <Label key={key}>
        {label}
        {multiline ? (
          <Textarea
            aria-label={label}
            value={body[key] ?? ""}
            disabled={disabled}
            onChange={(e) => set(key, e.target.value)}
          />
        ) : (
          <FieldInput
            aria-label={label}
            value={body[key] ?? ""}
            disabled={disabled}
            onChange={(e) => set(key, e.target.value)}
          />
        )}
      </Label>
    );
  const known = [
    "schema",
    ...(texts[kind] || []).map(([key]) => key),
    ...(kind === "world"
      ? ["framework"]
      : kind === "outfit"
        ? ["members"]
        : kind === "character_template"
          ? ["description", "initialization"]
          : kind === "persona_fragment"
            ? ["changes"]
            : kind === "item"
              ? ["item_kind", "category", "slot", "description"]
              : kind === "event_template"
                ? Object.keys(body)
                : []),
  ];
  return (
    <div className="r-form-stack">
      {(texts[kind] || []).map(field)}
      {kind === "world" && (
        <WorldFramework body={body} onChange={onChange} disabled={disabled} />
      )}{" "}
      {kind === "character_template" && (
        <>
          {!body.initialization && field(["description", "完整角色描述", true])}
          <WorkspaceInfo>
            模板只保留可复用设定，对话、记忆和运行状态不包含在内。
          </WorkspaceInfo>
          {body.initialization && (
            <Disclosure title="结构化初始化定义">
              <RecordEditor
                value={body.initialization}
                disabled={disabled}
                onChange={(initialization) =>
                  set("initialization", initialization)
                }
              />
            </Disclosure>
          )}
        </>
      )}
      {kind === "persona_fragment" && (
        <>
          <WorkspaceInfo>
            人格资料以changes结构保存；可从完整JSON建立需要的字段，已有字段在下方编辑。应用会先形成身份提议。
          </WorkspaceInfo>
          <RecordEditor
            value={body.changes || {}}
            disabled={disabled}
            onChange={(changes) => set("changes", changes)}
          />
        </>
      )}
      {kind === "item" && (
        <>
          <div className="r-form-grid">
            <Label>
              物品类型
              <ChoiceSelect
                aria-label="物品类型"
                value={body.item_kind || ""}
                disabled={disabled}
                onChange={(e) =>
                  onChange({
                    ...body,
                    item_kind: e.target.value,
                    category: "",
                    slot: "",
                  })
                }
              >
                <option value="object">普通物品</option>
                <option value="wearable">服饰</option>
              </ChoiceSelect>
            </Label>
            <Label>
              分类
              <ChoiceSelect
                aria-label="物品分类"
                value={body.category || ""}
                disabled={disabled}
                onChange={(e) =>
                  onChange({ ...body, category: e.target.value, slot: "" })
                }
              >
                <option value="">请选择</option>
                {wardrobeCategoryOptions(body.item_kind).map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
                {body.category &&
                  !wardrobeCategoryOptions(body.item_kind).some(
                    (item) => item.value === body.category,
                  ) && (
                    <option value={body.category}>
                      {body.category}（现有自定义值）
                    </option>
                  )}
              </ChoiceSelect>
            </Label>
          </div>
          {body.item_kind === "wearable" && (
            <Label>
              穿着部位
              <ChoiceSelect
                aria-label="穿着部位"
                value={body.slot || ""}
                disabled={disabled}
                onChange={(e) => set("slot", e.target.value)}
              >
                <option value="">请选择</option>
                {wardrobeSlotOptions(body.item_kind, body.category).map(
                  (item) => (
                    <option key={item.value} value={item.value}>
                      {item.label}
                    </option>
                  ),
                )}
                {body.slot &&
                  !wardrobeSlotOptions(body.item_kind, body.category).some(
                    (item) => item.value === body.slot,
                  ) && (
                    <option value={body.slot}>
                      {body.slot}（现有自定义值）
                    </option>
                  )}
              </ChoiceSelect>
            </Label>
          )}
          {field(["description", "物品描述", true])}
          <WorkspaceInfo>
            发布后可注册到世界目录。取得或穿用仍由实际生活行动结算。
          </WorkspaceInfo>
        </>
      )}
      {kind === "outfit" && (
        <OutfitFields body={body} onChange={onChange} disabled={disabled} />
      )}{" "}
      {kind === "event_template" && (
        <EventFields body={body} onChange={onChange} disabled={disabled} />
      )}
      {kind === "npc" && (
        <WorkspaceInfo>
          定义、应用到运行世界和实际结识分别进行。初始地点在应用时明确选择，生活规律在运行人物资料中保存。
        </WorkspaceInfo>
      )}
      {kind === "visual_style" && (
        <WorkspaceInfo>
          这些是未来视觉会话参考，不改变当前身体、穿着或规范图。
        </WorkspaceInfo>
      )}
      {Object.keys(body).some((key) => !known.includes(key)) && (
        <Disclosure title="其他扩展字段">
          <RecordEditor
            value={body}
            exclude={known}
            disabled={disabled}
            onChange={onChange}
          />
        </Disclosure>
      )}
    </div>
  );
}
function WorldFramework({ body, onChange, disabled }) {
  const framework = body.framework,
    valid =
      framework === undefined ||
      (framework &&
        typeof framework === "object" &&
        !Array.isArray(framework) &&
        Array.isArray(framework.venues) &&
        framework.venues.every(
          (v) =>
            v &&
            typeof v === "object" &&
            !Array.isArray(v) &&
            (v.places === undefined ||
              (Array.isArray(v.places) &&
                v.places.every(
                  (p) => p && typeof p === "object" && !Array.isArray(p),
                ))),
        ));
  const venues = valid ? framework?.venues || [] : [],
    update = (values) =>
      onChange({
        ...body,
        framework: {
          ...(framework || { schema: "world-framework.v1" }),
          venues: values,
        },
      }),
    changeVenue = (index, patch) =>
      update(venues.map((v, i) => (i === index ? { ...v, ...patch } : v)));
  return (
    <>
      {!valid && (
        <p role="alert">场地结构无法读取，原定义仍保留，请从完整 JSON 核对。</p>
      )}
      <Disclosure
        title={
          "场地与具体地点 · " + (valid ? venues.length + " 类" : "结构待核对")
        }
      >
        <div className="r-form-stack">
          {venues.map((venue, i) => (
            <div className="r-venue-edit" key={venue.key || i}>
              <div className="r-tag-edit">
                <FieldInput
                  aria-label={"分类名称" + (i + 1)}
                  value={venue.name || ""}
                  disabled={disabled}
                  onChange={(e) => changeVenue(i, { name: e.target.value })}
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={"移除场地分类" + (i + 1)}
                  disabled={disabled}
                  onClick={() => update(venues.filter((_, j) => j !== i))}
                >
                  <X size={14} />
                </Button>
              </div>
              {(venue.places || []).map((place, j) => (
                <div className="r-resource-place" key={place.key || j}>
                  <FieldInput
                    aria-label={"地点名称" + (i + 1) + "-" + (j + 1)}
                    placeholder="地点名称"
                    value={place.name || ""}
                    disabled={disabled}
                    onChange={(e) =>
                      changeVenue(i, {
                        places: venue.places.map((p, k) =>
                          j === k ? { ...p, name: e.target.value } : p,
                        ),
                      })
                    }
                  />
                  <FieldInput
                    aria-label={"地点描述" + (i + 1) + "-" + (j + 1)}
                    placeholder="地点描述"
                    value={
                      typeof place.description === "string"
                        ? place.description
                        : ""
                    }
                    disabled={
                      disabled ||
                      (place.description !== undefined &&
                        typeof place.description !== "string")
                    }
                    onChange={(e) =>
                      changeVenue(i, {
                        places: venue.places.map((p, k) =>
                          j === k ? { ...p, description: e.target.value } : p,
                        ),
                      })
                    }
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={"移除地点" + (i + 1) + "-" + (j + 1)}
                    disabled={disabled}
                    onClick={() =>
                      changeVenue(i, {
                        places: venue.places.filter((_, k) => k !== j),
                      })
                    }
                  >
                    <Trash2 size={12} />
                  </Button>
                  <Disclosure title="地点完整字段">
                    <RecordEditor
                      value={place}
                      disabled={disabled}
                      onChange={(next) =>
                        changeVenue(i, {
                          places: venue.places.map((p, k) =>
                            j === k ? next : p,
                          ),
                        })
                      }
                    />
                  </Disclosure>
                </div>
              ))}
              <Button
                type="button"
                variant="ghost"
                disabled={disabled}
                onClick={() =>
                  changeVenue(i, {
                    places: [
                      ...(venue.places || []),
                      {
                        key: "place_" + randomId().slice(0, 8),
                        name: "",
                        description: "",
                        floor: "",
                        atmosphere: "",
                        layout: "",
                      },
                    ],
                  })
                }
              >
                <Plus size={12} />
                添加具体地点
              </Button>
              <Disclosure title="分类局部键与扩展">
                <RecordEditor
                  value={venue}
                  exclude={["places", "name"]}
                  disabled={disabled}
                  onChange={(next) => changeVenue(i, next)}
                />
              </Disclosure>
            </div>
          ))}
          <Button
            type="button"
            variant="outline"
            disabled={disabled || !valid}
            onClick={() =>
              update([
                ...venues,
                {
                  key: "venue_" + randomId().slice(0, 8),
                  name: "",
                  places: [],
                },
              ])
            }
          >
            <Plus size={14} />
            添加场地分类
          </Button>
        </div>
      </Disclosure>
    </>
  );
}
function OutfitFields({ body, onChange, disabled }) {
  const w = useWorkspace(),
    [selected, setSelected] = useState("");
  const source = useWorkspaceRead({
    identity: ["studio-published-wearables"],
    characterScoped: false,
    read: (cursor) =>
      w.services.studioClient.list({ kind: "item", published: true, cursor }),
  });
  const members = Array.isArray(body.members) ? body.members : [],
    eligible = source.items.filter((r) => r.body?.item_kind === "wearable");
  return (
    <div className="r-form-stack">
      <p className="r-muted">
        引用精确发布版本的服饰定义；应用时还需明确映射实际库存。
      </p>
      {!Array.isArray(body.members) && (
        <p role="alert">原成员结构仍保留，请从完整JSON核对。</p>
      )}
      {members.map((member, i) => (
        <div
          className="r-resource-member"
          key={member.resource_id + ":" + member.resource_version + ":" + i}
        >
          <span>
            {source.items.find((r) => r.id === member.resource_id)?.name ||
              "已选择定义"}{" "}
            · {member.resource_version}
          </span>
          <Button
            variant="ghost"
            type="button"
            disabled={disabled}
            onClick={() =>
              onChange({ ...body, members: members.filter((_, j) => j !== i) })
            }
          >
            移除此成员
          </Button>
        </div>
      ))}
      <MediaReader source={source} label="服饰定义" />
      <ChoiceSelect
        aria-label="服饰定义"
        value={selected}
        disabled={disabled}
        onChange={(e) => setSelected(e.target.value)}
      >
        <option value="">请选择已发布服饰定义</option>
        {eligible.map((r) => (
          <option key={r.id} value={r.id}>
            {r.name} · {r.published_version}
          </option>
        ))}
      </ChoiceSelect>
      <Button
        variant="outline"
        type="button"
        disabled={disabled || !selected || !Array.isArray(body.members)}
        onClick={() => {
          const r = eligible.find((r) => r.id === selected);
          if (
            r &&
            !members.some(
              (m) =>
                m.resource_id === r.id &&
                m.resource_version === r.published_version,
            )
          )
            onChange({
              ...body,
              members: [
                ...members,
                { resource_id: r.id, resource_version: r.published_version },
              ],
            });
        }}
      >
        加入搭配成员
      </Button>
      <MediaMore source={source} label="服饰定义" />
    </div>
  );
}
function EventFields({ body, onChange, disabled }) {
  const update = (run) => {
    const next = clone(body);
    run(next);
    onChange(next);
  };
  return (
    <div className="r-form-stack">
      <Label>
        事件描述
        <Textarea
          aria-label="事件描述"
          value={typeof body.description === "string" ? body.description : ""}
          disabled={disabled}
          onChange={(e) =>
            update((next) => (next.description = e.target.value))
          }
        />
      </Label>
      <div className="r-form-grid">
        <Label>
          类别
          <ChoiceSelect
            aria-label="事件类别"
            value={body.category || ""}
            disabled={disabled}
            onChange={(e) => update((next) => (next.category = e.target.value))}
          >
            {Object.entries({
              environment: "环境变化",
              visitor: "人物到访",
              shop: "商店变化",
              encounter: "偶遇",
              temporary_state: "临时状态",
            }).map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </ChoiceSelect>
        </Label>
        <Label>
          强度
          <ChoiceSelect
            aria-label="事件强度"
            value={body.intensity || ""}
            disabled={disabled}
            onChange={(e) =>
              update((next) => (next.intensity = e.target.value))
            }
          >
            <option value="low">低</option>
            <option value="medium">中</option>
          </ChoiceSelect>
        </Label>
      </div>
      <RecordEditor
        value={body}
        exclude={[
          "schema_version",
          "description",
          "category",
          "intensity",
          "effects",
        ]}
        fieldLabels={{
          duration_minutes: "持续分钟",
          trigger: "触发与间隔",
          condition: "适用条件",
        }}
        disabled={disabled}
        onChange={onChange}
      />
      <Disclosure title="可能结果与范围">
        <div className="r-form-stack">
          {Array.isArray(body.effects) ? (
            body.effects.map((effect, i) => (
              <div className="r-venue-edit r-form-stack" key={i}>
                <ChoiceSelect
                  aria-label={"效果" + (i + 1)}
                  value={effect.kind}
                  disabled={disabled}
                  onChange={(e) =>
                    update((next) => {
                      const current = next.effects[i];
                      current.kind = e.target.value;
                      delete current.npc_id;
                      delete current.body_effect;
                      if (
                        ["temporary_state", "temporary_injury"].includes(
                          current.kind,
                        )
                      ) {
                        current.scope = "character";
                        next.condition = {
                          ...next.condition,
                          require_presence: true,
                        };
                      }
                      if (current.kind === "temporary_injury")
                        current.body_effect = {
                          body_part: "",
                          description: "",
                          severity: "minor",
                          impact: "none",
                        };
                      if (
                        ["npc_visit", "place_availability"].includes(
                          current.kind,
                        )
                      )
                        current.scope = "place";
                      if (current.scope !== "place") delete current.place_id;
                    })
                  }
                >
                  {Object.entries(eventEffectLabels).map(([key, label]) => (
                    <option key={key} value={key}>
                      {label}
                    </option>
                  ))}
                </ChoiceSelect>
                <ChoiceSelect
                  aria-label={"效果范围" + (i + 1)}
                  value={effect.scope}
                  disabled={disabled}
                  onChange={(e) =>
                    update((next) => {
                      next.effects[i].scope = e.target.value;
                      if (e.target.value !== "place")
                        delete next.effects[i].place_id;
                    })
                  }
                >
                  <option value="world">整个应用世界</option>
                  <option value="place">指定地点</option>
                  <option value="character">当前角色</option>
                </ChoiceSelect>
                <RecordEditor
                  value={effect}
                  exclude={["kind", "scope"]}
                  fieldLabels={{
                    value: "结果描述",
                    place_id: "明确地点引用",
                    npc_id: "明确人物引用",
                    body_effect: "临时身体影响",
                  }}
                  disabled={disabled}
                  onChange={(value) =>
                    update((next) => (next.effects[i] = value))
                  }
                />
                <Button
                  variant="outline"
                  type="button"
                  disabled={disabled}
                  onClick={() => update((next) => next.effects.splice(i, 1))}
                >
                  移除此效果
                </Button>
              </div>
            ))
          ) : (
            <p role="alert">原效果结构无法读取，请从完整JSON核对。</p>
          )}
          <Button
            variant="outline"
            type="button"
            disabled={
              disabled ||
              !Array.isArray(body.effects) ||
              body.effects.length >= 8
            }
            onClick={() =>
              update((next) =>
                next.effects.push({
                  kind: "weather",
                  scope: "world",
                  value: "",
                }),
              )
            }
          >
            添加效果
          </Button>
        </div>
      </Disclosure>
      <WorkspaceInfo>
        应用后事件启用与实际触发分别进行，人物到访不自动建立关系。
      </WorkspaceInfo>
    </div>
  );
}
