import { FieldInput, Textarea, ChoiceSelect, Disclosure } from "../controls";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import LoadingState from "./LoadingState";
import { InlineFeedback } from "./Presentation";
import WorkspaceInfo from "./WorkspaceInfo";
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
      <span>{label}</span>
      <Input
        aria-label={label}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        {...props}
      />
    </label>
  );
}
export function WorldForm({ children }) {
  return <div className="replica-world r-form-stack">{children}</div>;
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
        <WorldField
          label="世界物品池（文本或JSON数组）"
          multiline
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
        <WorldField
          label="固定陈设（文本或JSON数组）"
          multiline
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
        <WorldField
          label="其他连接说明（文本或JSON数组）"
          multiline
          value={d.connections}
          onChange={(connections) => edit({ connections })}
        />
        <WorldField
          label="房内物品（文本或JSON数组）"
          multiline
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
