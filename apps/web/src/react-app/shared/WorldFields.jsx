import { FieldInput, Textarea, ChoiceSelect, Disclosure } from "../controls";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import LoadingState from "./LoadingState";
import { InlineFeedback } from "./Presentation";
import WorkspaceInfo from "./WorkspaceInfo";
import SemanticTree from './SemanticTree';
export default function WorldField({
  label,
  value,
  onChange,
  multiline = false,
  ...props
}) {
  const Input = multiline ? Textarea : FieldInput;
  return (
    <label className="replica-field">
      <span>{label}{props.required&&<span className="replica-required" aria-hidden="true"> *</span>}</span>
      <Input
        aria-label={label}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        {...props}
      />
    </label>
  );
}
function WorldCollectionField({label,value,onChange}){
  let entries=null,invalid=false;
  if(typeof value==='string'&&value.trim().startsWith('[')){
    try{const parsed=JSON.parse(value);if(Array.isArray(parsed))entries=parsed;else invalid=true;}
    catch{invalid=true;}
  }
  if(invalid)return <div className="r-form-stack"><p className="r-muted">{label}的列表格式尚未读懂，原内容已保留，请核对完整结构。</p><WorldField label={label+'完整结构'} value={value} onChange={onChange} multiline/></div>;
  if(!entries)return <WorldField label={label} value={value} onChange={onChange} multiline/>;
  if(entries.some(entry=>typeof entry!=='string'))return <section className="r-form-stack" aria-label={label}><h3 className="r-field-label">{label}</h3><SemanticTree value={entries}/><Disclosure title={'编辑'+label+'的完整结构'}><WorldField label={label+'完整结构'} value={value} onChange={onChange} multiline/></Disclosure></section>;
  const update=next=>onChange(JSON.stringify(next,null,2));
  return <section className="r-form-stack" aria-label={label}><h3 className="r-field-label">{label}</h3>{entries.map((entry,index)=><div className="r-inline-actions" key={index}><FieldInput aria-label={label+'第'+(index+1)+'项'} value={entry} onChange={event=>update(entries.map((before,i)=>i===index?event.target.value:before))}/><Button type="button" variant="ghost" aria-label={'移除'+label+'第'+(index+1)+'项'} onClick={()=>update(entries.filter((_,i)=>i!==index))}>移除</Button></div>)}{!entries.length&&<p className="r-muted">尚未登记。</p>}<Button type="button" variant="outline" disabled={entries.length>=200} onClick={()=>update([...entries,''])}>添加{label}</Button></section>;
}
export function WorldForm({ children }) {
  return <div className="replica-world r-form-stack lp-wide">{children}</div>;
}
export function WorldReader({ source, label }) {
  return (
    <>
      {source.busy && <LoadingState label={"正在读取" + label + "…"} />}{" "}
      {source.error && (
        <InlineFeedback role="alert">
          {source.error}
          <Button variant="outline" onClick={source.retry}>
            重新读取{label}
          </Button>
        </InlineFeedback>
      )}
    </>
  );
}
export function WorldEditFields({ draft: d, edit }) {
  return (
    <WorldForm>
      <WorldField
        label="世界名称"
        data-testid="world-name"
        required
        value={d.name}
        onChange={(name) => edit({ name })}
      />
      <WorldField
        label="世界类型"
        data-testid="world-genre"
        value={d.flavor}
        onChange={(flavor) => edit({ flavor })}
      />
      <WorldField
        label="世界背景"
        multiline
        rows={4}
        value={d.background}
        onChange={(background) => edit({ background })}
      />
      <WorldField
        label="基础规则"
        multiline
        rows={4}
        value={d.rules}
        onChange={(rules) => edit({ rules })}
      />
      <Disclosure title="其他已有世界字段">
        <WorldField
          label="补充描述"
          multiline
          value={d.description}
          onChange={(description) => edit({ description })}
        />
        <WorldCollectionField
          label="世界物品池"
          value={d.itemPool}
          onChange={(itemPool) => edit({ itemPool })}
        />
      </Disclosure>
    </WorldForm>
  );
}
export function PlaceEditFields({
  draft: d,
  edit,
  categories,
  places,
  onHistory,
}) {
  return (
    <WorldForm>
      {Object.keys(d.unappliedIdentity || {}).length > 0 && (
        <WorkspaceInfo>
          原创建已保存为「{d.source?.name || d.name}」。后续未应用的名称
          {d.unappliedIdentity.name
            ? "「" + d.unappliedIdentity.name + "」"
            : ""}
          和分类
          {d.unappliedIdentity.category
            ? "「" +
              (categories.find((c) => c.id === d.unappliedIdentity.category)
                ?.name || d.unappliedIdentity.category) +
              "」"
            : ""}
          保留在草稿中；已有地点的正式名称与分类固定，可在核对后用于另建地点。
        </WorkspaceInfo>
      )}
      <div className="r-form-grid">
        <WorldField
          label="正式名称"
          required
          readOnly={!!d.id}
          value={d.name}
          onChange={(name) => edit({ name })}
        />
        <WorldField
          label="别名"
          value={d.alias}
          onChange={(alias) => edit({ alias })}
        />
        <label className="replica-field">
          <span>所属场地</span>
          <ChoiceSelect
            aria-label="所属场地"
            disabled={!!d.source?.venue_category_id}
            value={d.category}
            onChange={(e) => edit({ category: e.target.value })}
          >
            <option value="">未分类</option>
            {categories.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </ChoiceSelect>
        </label>
        <WorldField
          label="楼层 / 位置"
          value={d.floor}
          onChange={(floor) => edit({ floor })}
        />
      </div>
      <WorldField
        label="场景描述"
        multiline
        rows={4}
        value={d.description}
        onChange={(description) => edit({ description })}
      />
      <div className="r-form-grid">
        <WorldField
          label="氛围"
          value={d.atmosphere}
          onChange={(atmosphere) => edit({ atmosphere })}
        />
        <WorldCollectionField
          label="固定陈设"
          value={d.furniture}
          onChange={(furniture) => edit({ furniture })}
        />
      </div>
      <WorldField
        label="空间布局"
        multiline
        value={d.layout}
        onChange={(layout) => edit({ layout })}
      />
      <Disclosure title="场景与空间连接">
        <WorldField
          label="场景素材引用"
          value={d.scene}
          onChange={(scene) => edit({ scene })}
        />
        <label className="replica-field">
          <span>父地点</span>
          <ChoiceSelect
            aria-label="父地点"
            value={d.parentPlaceId}
            onChange={(e) => edit({ parentPlaceId: e.target.value })}
          >
            <option value="">无父地点</option>
            {places
              .filter((p) => p.id !== d.id && p.status === "active")
              .map((p) => (
                <option key={p.id} value={p.id}>
                  {p.alias || p.name}
                </option>
              ))}
          </ChoiceSelect>
        </label>
        <p className="r-field-label">连通地点</p>
        <div className="r-check-list">
          {places
            .filter((p) => p.id !== d.id && p.status === "active")
            .map((p) => (
              <label key={p.id}>
                <Checkbox
                  checked={d.connectionPlaceIds.includes(p.id)}
                  onCheckedChange={(checked) =>
                    edit({
                      connectionPlaceIds: checked
                        ? [...d.connectionPlaceIds, p.id]
                        : d.connectionPlaceIds.filter((id) => id !== p.id),
                    })
                  }
                />
                {p.alias || p.name}
              </label>
            ))}
        </div>
        <WorldCollectionField
          label="其他连接说明"
          value={d.connections}
          onChange={(connections) => edit({ connections })}
        />
        <WorldCollectionField
          label="房内物品"
          value={d.roomItems}
          onChange={(roomItems) => edit({ roomItems })}
        />
      </Disclosure>
      {d.id && onHistory && (
        <div className="r-inline-actions">
          <Button type="button" variant="ghost" onClick={onHistory}>
            恢复默认字段
          </Button>
          <Button type="button" variant="ghost" onClick={onHistory}>
            修订历史
          </Button>
        </div>
      )}
    </WorldForm>
  );
}
