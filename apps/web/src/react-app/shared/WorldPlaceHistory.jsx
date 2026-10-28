import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import WorldField, { WorldForm, WorldReader } from "./WorldFields";
import JsonDetails from "./JsonDetails";
import SemanticTree from "./SemanticTree";
import { userInstant } from "@/v3/presentation-time.mjs";
import { semanticLabel } from "@/v3/proto/components/semantic-fields.js";
export default function WorldPlaceHistory({ model: m, selection: s, onClose }) {
  const { fid, owner, w } = m,
    wid = s.world.id,
    pid = s.place.id;
  const [source, setSource] = useState("recorded"),
    [action, setAction] = useState(null),
    [notice, setNotice] = useState("");
  const trigger = useRef(null);
  const check = (row) => {
    if (row.world_id !== wid || row.place_id !== pid)
      throw Error("地点基准范围不匹配。");
    return row;
  };
  const readDefault = async (from = source) =>
    check(await w.client.placeDefaults(fid, wid, pid, from));
  const defaults = useWorkspaceRead({
    identity: ["place-default", wid, pid, source],
    read: async () => ({ items: [await readDefault()], next_cursor: "" }),
  });
  const history = useWorkspaceRead({
    identity: ["place-history", wid, pid],
    read: async (cursor) => {
      const page = await w.client.placeDefaultHistory(fid, wid, pid, cursor);
      if (
        page.items.some(
          (row) =>
            row.fluctlight_id !== fid ||
            row.world_id !== wid ||
            row.place_id !== pid,
        )
      )
        throw Error("地点历史范围不匹配。");
      return page;
    },
  });
  const current = defaults.items[0];
  function command(event) {
    m.assertWritable();
    trigger.current = event.currentTarget;
    const operation = source === "recorded" ? "restore" : "capture";
    setAction({
      identity: ["place-default", wid, pid, operation, source].join(":"),
      scope: ["place-default", fid, wid, pid, operation, source],
      storageKey: `fluctlight-world-framework:${owner}:${fid}:replica:default:${wid}:${pid}:${operation}:${source}`,
      title:
        operation === "restore" ? "恢复已核对的默认字段" : "明确保存恢复基准",
      description:
        operation === "restore"
          ? "恢复基准覆盖的地点字段，保留正式名称、空间关系与历史。"
          : "把已选择的真实保存状态或已应用定义冻结为恢复基准。",
      submitLabel:
        operation === "restore" ? "确认恢复默认字段" : "确认保存恢复基准",
      initial: {
        placeRevision: current.place_revision,
        defaultRevision: current.default_revision,
        reason: "",
      },
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason.trim()].length > 1000)
          throw Error("请填写操作说明，最多1000个字符。");
      },
      build: (d) => ({
        worldId: wid,
        expectedRevision: d.placeRevision,
        expectedDefaultRevision: d.defaultRevision,
        operation,
        source,
        reason: d.reason.trim(),
      }),
      execute: async (body, key) => {
        m.assertWritable();
        const r = await w.client.savePlaceDefaults(fid, pid, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== pid ||
          !Number.isSafeInteger(r.place_revision) ||
          !Number.isSafeInteger(r.default_revision)
        )
          throw Error("基准操作回执未完整确认。");
        return r;
      },
      readLatest: () => readDefault(source),
      adoptLatest: (d, row) => ({
        ...d,
        placeRevision: row.place_revision,
        defaultRevision: row.default_revision,
      }),
      onSaved: async (r) => {
        const fresh = await readDefault(source),
          page = await m.readCatalog(),
          place = page.snapshots[wid]?.places.find((p) => p.id === pid);
        if (
          fresh.place_revision < r.place_revision ||
          fresh.default_revision < r.default_revision ||
          !place ||
          place.revision < r.place_revision
        )
          throw Error("地点或基准的新版本尚未读回。");
        if (m.live()) {
          defaults.adopt({ items: [fresh], next_cursor: "" });
          m.catalog.adopt(page);
          await history.load();
          setNotice("实际地点与恢复基准已读回，历史记录继续保留。");
        }
      },
      fields: (d, edit) => (
        <WorldForm>
          <WorldField
            label="地点基准操作说明"
            multiline
            value={d.reason}
            onChange={(reason) => edit({ reason })}
          />
        </WorldForm>
      ),
    });
  }
  const labels = {
    initial_saved: "建立地点与初始字段基准",
    edit_fields: "更新地点字段",
    set_baseline: "明确保存恢复基准",
    restore_fields: "恢复默认字段",
  };
  return (
    <>
      <WorkspaceDialog
        className="replica-world"
        open={!action}
        wide
        title="地点修订记录与默认字段"
        description={"回看「" + s.place.name + "」的字段修改与恢复记录。"}
        onOpenChange={(v) => {
          if (!v) onClose();
        }}
        returnFocus={() => s.trigger}
      >
        <div className="r-form-stack">
          <div className="r-inline-actions">
            {[
              ["recorded", "已记录的恢复基准"],
              ["current_saved", "预览当前已保存字段"],
              ["current_definition", "预览已应用定义"],
            ].map(([value, label]) => (
              <Button
                key={value}
                variant={source === value ? "default" : "outline"}
                onClick={() => setSource(value)}
              >
                {label}
              </Button>
            ))}
          </div>
          {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
          <WorldReader source={defaults} label="地点基准" />
          {current && (
            <>
              <p className="r-muted">
                地点 v{current.place_revision} · 基准 v
                {current.default_revision}
              </p>
              {current.baseline ? (
                <>
                  <JsonDetails
                    value={current.baseline}
                    title="实际基准与完整来源"
                  />
                  <div className="r-timeline">
                    {current.diff.map((row) => (
                      <div key={row.field}>
                        <i />
                        <span>
                          <b>{semanticLabel(row.field)}</b>
                          <SemanticTree
                            value={{ 原值: row.from, 目标值: row.to }}
                          />
                        </span>
                      </div>
                    ))}
                  </div>
                  <Button
                    variant="outline"
                    disabled={m.readOnly}
                    onClick={command}
                  >
                    {source === "recorded"
                      ? "恢复默认字段"
                      : "明确保存此恢复基准"}
                  </Button>
                </>
              ) : (
                <WorkspaceInfo>
                  这个地点还没有可验证的恢复快照。可预览当前已保存字段，或实际已应用的定义，再明确保存恢复基准。
                </WorkspaceInfo>
              )}
            </>
          )}
          <h3>实际字段修订与恢复历史</h3>
          <p className="r-fineprint">
            这里记录地点字段与恢复基准的变化；空间层级和连通关系以完整地点资料为准。
          </p>
          <WorldReader source={history} label="地点修订" />
          <div className="r-timeline">
            {history.items.map((row, index) => (
              <div key={row.id || row.audit_id || index}>
                <i />
                <span>
                  <b>{labels[row.operation] || row.operation}</b>
                  <small>{userInstant(row.created_at)}</small>
                  {row.reason && <p>{row.reason}</p>}
                  <JsonDetails value={row} title="这次修改的字段与来源" />
                </span>
              </div>
            ))}
          </div>
          {history.ready && !history.items.length && !history.error && (
            <p className="r-muted">尚未读到地点修订记录。</p>
          )}
          {history.cursor && (
            <Button
              variant="outline"
              disabled={history.busy}
              onClick={history.more}
            >
              加载更多地点修订
            </Button>
          )}
        </div>
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => trigger.current}
        />
      )}
    </>
  );
}
