import {Disclosure} from '../controls';
import RecordView from './RecordView';
import { selector, readValue } from "@/state/view-model";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
import {
  semanticLabel,
  hasSemanticValue,
} from "@/v3/proto/components/semantic-fields.js";
import { enumLabel } from "@/lib/fluctlight-display.ts";
import {
  wardrobeCategoryLabel,
  wardrobeSlotLabel,
  wardrobeOwnershipLabel,
  wardrobeAvailabilityLabel,
} from "@/lib/wardrobe-form.ts";
import {
  semanticEntries,
  portraitPresentation,
} from "@/approved-ui/profile-presentation.mjs";
import { bodyFieldVisible } from "@/lib/body-field-presentation.mjs";
import {relationMetrics} from '@/approved-ui/people-presentation.mjs';
const technicalSemanticKey = k => /^(?:id|schema_version|provenance|extensions|evidence_refs|source|source_kind|source_ref|source_verified|revision|instant|update_policy|owner_switch_targets|owner_switch_status|hash|source_hash)$/.test(k) || /_(?:id|ids|revision|version|at)$/.test(k);
export function hasVisibleSemanticValue(value,field='',identity={}) {
 if(technicalSemanticKey(field)||!bodyFieldVisible(field,value,identity))return false;
 if(Array.isArray(value))return value.some(item=>hasVisibleSemanticValue(item,'',identity));
 if(value&&typeof value==='object'){if(value.status==='unknown'&&Object.entries(value).every(([key,item])=>key==='status'||technicalSemanticKey(key)||item==null||item===''))return false;}
 if(value&&typeof value==='object')return semanticEntries(value).some(([key,item])=>hasVisibleSemanticValue(item,key,identity));
 return hasSemanticValue(value);
}
/** @param {{value?: unknown, field?: string, depth?: number, identity?: object}} input */
export default function SemanticTree(input = {}) {
  const __props = {
    field: "",
    depth: 0,
    identity: (() => ({}))(),
    ...input,
  };
  const props = __props;
  const technical = technicalSemanticKey;
  function categoryLabel(value) {
    if(value==='upper')return '上装';
    if(value==='lower')return '下装';
    if(value==='full')return '连身服装';
    const category = wardrobeCategoryLabel(value);
    if (category !== value || !/^[a-z\d_.-]+$/i.test(category)) return category;
    const slot = wardrobeSlotLabel(value);
    return slot !== value ? slot : "自定义分类";
  }
  const entries = selector(() =>
    props.value &&
    typeof props.value === "object" &&
    !Array.isArray(props.value)
      ? semanticEntries(props.value).filter(
          ([key, value]) =>
            !technical(key) &&
            hasVisibleSemanticValue(value,key,props.identity) &&
            bodyFieldVisible(key, value, props.identity),
        )
      : [],
  );
  const dimensions = {
    openness: ["偏稳妥熟悉", "开放程度适中", "愿意探索新事物"],
    conscientiousness: ["偏随性", "条理程度适中", "重视计划与责任"],
    extraversion: ["偏内敛", "内外向适中", "偏外向"],
    agreeableness: ["更重视独立立场", "亲和程度适中", "偏温和合作"],
    neuroticism: ["情绪较稳定", "敏感程度适中", "感受较敏锐"],
    curiosity: ["偏熟悉事物", "好奇程度适中", "好奇心较强"],
    independence: ["重视共同决定", "独立程度适中", "偏独立自主"],
    patience: ["偏及时行动", "耐心程度适中", "比较耐心"],
    empathy: ["更重视事实判断", "共情程度适中", "比较细腻体贴"],
    assertiveness: ["偏含蓄表达", "主张程度适中", "倾向明确表达立场"],
    humor: ["表达较认真", "幽默程度适中", "表达较幽默"],
    sociability: ["偏少而深入的交往", "社交倾向适中", "比较乐于交往"],
    risk_tolerance: ["偏谨慎", "冒险倾向适中", "比较愿意尝试"],
  };
  const scalar = selector(() => {
    const value =
      dimensions[props.field] &&
      typeof props.value === "string" &&
      /^\d+(?:\.\d+)?$/.test(props.value)
        ? Number(props.value)
        : props.value;
    if(props.field==='seasons'&&typeof value==='string')return ({spring:'春',summer:'夏',autumn:'秋',winter:'冬'})[value]||value;
    if(props.field==='clothing_type'&&typeof value==='string')return wardrobeSlotLabel(value);
    if (typeof value === "boolean") return value ? "是" : "否";
    if (typeof value === "number") {
      if (dimensions[props.field] && value >= 0 && value <= 1)
        return dimensions[props.field][value < 0.35 ? 0 : value > 0.65 ? 2 : 1];
      const affect = {
        pleasure: ["有些低落", "情绪平稳", "心情愉悦"],
        arousal: ["平静放松", "精力适中", "比较活跃"],
        dominance: ["需要支持", "掌控感适中", "比较有把握"],
      };
      if (affect[props.field] && value >= -1 && value <= 1)
        return affect[props.field][value < -0.25 ? 0 : value > 0.25 ? 2 : 1];
      return String(value);
    }
    const aliases = {
      known: "已记录",
      unknown: "尚未确定",
      male: "男性",
      female: "女性",
      nonbinary: "非二元性别",
      ready: "已就绪",
      awake: "清醒",
      asleep: "休息中",
      assessment_required: "需要评估",
      single: "单一人格",
      multiple: "多重人格",
      stale: "等待更新",
      missing: "尚未整理",
    };
    const translated = aliases[value] || enumLabel(value);
    return translated === "未分类" ? String(value ?? "") : translated;
  });
  const _component_SemanticTree = SemanticTree;
  if(props.value&&typeof props.value==='object'&&!Array.isArray(props.value)&&Object.hasOwn(props.value,'value')&&props.value.value!=null&&Object.keys(props.value).every(key=>['value','status'].includes(key)||technical(key)))return <SemanticTree value={props.value.value} field={props.field} depth={props.depth+1} identity={props.identity}/>;
  if(props.field==='metrics'&&props.value&&typeof props.value==='object'&&!Array.isArray(props.value)){
    const known=new Map(relationMetrics({metrics:props.value}).map(row=>[row.key,row]));
    return <dl className="approved-facts creation-relation-metrics">{Object.entries(props.value).filter(([key,value])=>hasVisibleSemanticValue(value,key,props.identity)).map(([key,value])=>{const metric=known.get(key);return <div key={key}><dt>{metric?.label||(semanticLabel(key)==='自定义字段'?key:semanticLabel(key))}</dt><dd>{metric?<span>{new Intl.NumberFormat('zh-CN',{style:'percent',maximumFractionDigits:2}).format(metric.value)}</span>:<SemanticTree value={value} field={key} depth={props.depth+1} identity={props.identity}/>}</dd></div>;})}</dl>;
  }
  return __props.value == null || __props.value === "" ? (
    <span key={0} className="muted">
      {"尚未记录"}
    </span>
  ) : __props.field === "portrait_text" && typeof __props.value === "string" ? (
    <ul key={1} className="semantic-list">
      <Fragment>
        {listValues(readValue(portraitPresentation)(__props.value)).map(
          (line, index) => {
            return (
              <li key={index}>
                {line.field ? (
                  <Fragment key={0}>
                    <b>
                      {displayText(readValue(semanticLabel)(line.field)) + "："}
                    </b>
                    <_component_SemanticTree
                      value={line.value}
                      field={line.field}
                      depth={__props.depth + 1}
                    />
                  </Fragment>
                ) : (
                  <Fragment key={1}>{displayText(line.text)}</Fragment>
                )}
              </li>
            );
          },
        )}
      </Fragment>
    </ul>
  ) : typeof __props.value !== "object" ? (
    <span key={2}>{displayText(scalar.value)}</span>
  ) : __props.depth >= 7 ? (
    <p key={3} className="muted">
      {"更多层级可在技术详情中查看。"}
    </p>
  ) : Array.isArray(__props.value) &&
    ["wardrobe_items", "worn_items", "used_items"].includes(__props.field) ? (
    <ul key={4} className="semantic-list">
      <Fragment>
        {listValues(__props.value).map((item, index) => {
          const used=new Set(['slot','ownership',typeof item?.available==='boolean'?'available':'availability',item?.description?'description':'name',item?.category?'category':item?.clothing_type?'clothing_type':'type']);
          const extra=item&&typeof item==='object'?Object.fromEntries(Object.entries(item).filter(([key])=>!used.has(key))):{};
          return (
            <li key={index}>
              {item && typeof item === "object" ? (
                <Fragment key={0}>
                  <strong>
                    {displayText(
                      item.description || item.name || "物品描述尚未记录",
                    )}
                  </strong>
                  <p className="muted">
                    {displayText(
                      [
                        (item.category||item.clothing_type||item.type)&&categoryLabel(item.category||item.clothing_type||item.type),
                        item.slot && categoryLabel(item.category||item.clothing_type||item.type)!==readValue(wardrobeSlotLabel)(item.slot)&&!(String(item.category)==='shoes'&&item.slot==='feet')&&readValue(wardrobeSlotLabel)(item.slot),
                        readValue(wardrobeOwnershipLabel)(item.ownership),
                        typeof item.available === "boolean"
                          ? item.available
                            ? "可用"
                            : "不可用"
                          : readValue(wardrobeAvailabilityLabel)(
                              item.availability,
                            ),
                      ]
                        .filter(Boolean)
                        .join(" · "),
                    )}
                  </p>
                  {hasVisibleSemanticValue(extra,'',props.identity)&&<SemanticTree value={extra} depth={props.depth+1} identity={props.identity}/>}
                </Fragment>
              ) : (
                <Fragment key={1}>{displayText(item)}</Fragment>
              )}
            </li>
          );
        })}
      </Fragment>
    </ul>
  ) : Array.isArray(__props.value) ? (
    <ul key={5} className="semantic-list">
      <Fragment>
        {listValues(__props.value).map((item, index) => {
          return (
            <li key={index}>
              <_component_SemanticTree
                value={item}
                field={__props.field}
                depth={__props.depth + 1}
                identity={__props.identity}
              />
            </li>
          );
        })}
      </Fragment>
      {!__props.value.length ? (
        <li key={0} className="muted">
          {"尚未记录"}
        </li>
      ) : null}
    </ul>
  ) : (
    <dl key={6} className="approved-facts">
      <Fragment>
        {listValues(entries.value).filter(([key])=>!['strength','stability','trend'].includes(key)).map(([key, item], index) => {
          return (
            <div key={key}>
              {readValue(semanticLabel)(key)!=="自定义字段"&&<dt>{displayText(readValue(semanticLabel)(key))}</dt>}
              <dd>
                <_component_SemanticTree
                  value={item}
                  field={key}
                  depth={__props.depth + 1}
                  identity={__props.identity}
                />
              </dd>
            </div>
          );
        })}
      </Fragment>
      {entries.value.some(([key])=>['strength','stability','trend'].includes(key))&&<div><dd><Disclosure title="量表与评估记录"><p className="text-sm text-muted-foreground">这些数值是记录时的评估结果，不代表行为已经发生。</p><RecordView value={Object.fromEntries(entries.value.filter(([key])=>['strength','stability','trend'].includes(key)))} technical/></Disclosure></dd></div>}
      {!entries.value.length ? (
        <div key={0}>
          <dd className="muted">{"尚未记录"}</dd>
        </div>
      ) : null}
    </dl>
  );
}
