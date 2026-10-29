import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Textarea, FieldInput, Disclosure } from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import { MediaReader, SavedMedia } from "./MediaParts";
import MediaGrants from "./MediaGrants";
import MediaLifecycle from "./MediaLifecycle";
import {
  mediaSource,
  mediaKind,
  mediaBytes,
  requiredText,
  checkMediaAsset,
  mediaReadState,
} from "@/approved-ui/media-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { stateLabel } from "@/v3/domain-presentation.mjs";
export default function MediaDetail({ model: m, id, onClose }) {
  const detail = useWorkspaceRead({
      identity: ["media-detail", id, m.route.fid || ""],
      characterScoped: false,
      read: async () => ({
        items: [await m.readAsset(id, m.route.fid || "")],
        next_cursor: "",
      }),
    }),
    ref = useRef(detail),
    source = useRef(null),
    [action, setAction] = useState(null),
    [advanced, setAdvanced] = useState(
      m.route.query?.get("authorizeReference") === "1",
    );
  ref.current = detail;
  const asset = detail.items[0];
  useEffect(() => {
    if (asset && m.live()) m.adoptAsset(asset);
  }, [asset]);
  function adopt(row) {
    if (m.live()) {
      ref.current.adopt({ items: [row], next_cursor: "" });
      m.adoptAsset(row);
    }
  }
  function edit(event) {
    m.assertCurrent();
    source.current = event.currentTarget;
    setAction({
      identity: "metadata:" + id,
      scope: ["media", id, "metadata"],
      storageKey: `fluctlight-media:${m.owner}:${id}:replica-metadata`,
      title: "素材库信息",
      className: "replica-media",
      description:
        "名称和隐藏状态只影响库中的展示，字节、来源和实际引用分别保留。",
      submitLabel: "保存库信息",
      disabled: asset.status === "deleted",
      initial: {
        title: asset.title || "",
        hidden: asset.hidden,
        reason: "",
        assetRevision: asset.asset_revision,
        metadataRevision: asset.metadata_revision,
        baseline: { title: asset.title || "", hidden: asset.hidden },
      },
      validate: (d) => {
        requiredText(d.reason, "修改原因", 1000);
        if ([...d.title].length > 200) throw Error("素材名称最多200个字符。");
      },
      build: (d) => ({
        title: d.title,
        hidden: d.hidden,
        reason: d.reason,
        expectedAssetRevision: d.assetRevision,
        expectedMetadataRevision: d.metadataRevision,
      }),
      execute: async (body, key) => {
        m.assertCurrent();
        const r = checkMediaAsset(
          await m.service.save(id, { ...body, idempotencyKey: key }),
          id,
        );
        if (
          r.asset_revision < body.expectedAssetRevision ||
          r.metadata_revision <= body.expectedMetadataRevision ||
          r.title !== body.title ||
          r.hidden !== body.hidden
        )
          throw Error("素材保存回执尚未完整确认。");
        return r;
      },
      readLatest: () => m.readAsset(id),
      adoptLatest: (d, row) => ({
        ...d,
        title: d.title === d.baseline.title ? row.title || "" : d.title,
        hidden: d.hidden === d.baseline.hidden ? row.hidden : d.hidden,
        assetRevision: row.asset_revision,
        metadataRevision: row.metadata_revision,
        baseline: { title: row.title || "", hidden: row.hidden },
      }),
      onSaved: async (r, body) => {
        const row = await m.readAsset(id);
        if (
          r.id !== id ||
          row.asset_revision < r.asset_revision ||
          row.metadata_revision < r.metadata_revision ||
          (row.metadata_revision === r.metadata_revision &&
            (row.title !== body.title || row.hidden !== body.hidden))
        )
          throw Error("素材库信息的新版本尚未读回。");
        adopt(row);
      },
      fields: (d, change) => (
        <div className="r-form-stack">
          <label className="replica-field">
            <span>素材名称</span>
            <FieldInput
              aria-label="素材名称"
              value={d.title}
              onChange={(e) => change({ title: e.target.value })}
            />
          </label>
          <label className="companion-checkbox-row">
            <Switch
              aria-label="在素材库中隐藏"
              checked={d.hidden}
              onCheckedChange={(hidden) => change({ hidden })}
            />
            在素材库中隐藏
          </label>
          <label className="replica-field">
            <span>修改原因</span>
            <Textarea
              aria-label="修改原因"
              value={d.reason}
              onChange={(e) => change({ reason: e.target.value })}
            />
          </label>
        </div>
      ),
    });
  }
  return (
    <>
      <WorkspaceDialog
        open={!action}
        title={asset?.title || "画面详情"}
        description="保存的画面、实际来源与库中管理"
        wide
        returnFocus={() => m.trigger.current}
        onOpenChange={(v) => {
          if (!v && !action) onClose();
        }}
        footer={
          asset && (
            <>
              <Button
                variant="outline"
                disabled={asset.status === "deleted"}
                ref={source}
                onClick={edit}
              >
                编辑库信息
              </Button>
              <Button variant="outline" onClick={() => setAdvanced((v) => !v)}>
                引用授权与删除管理
              </Button>
              <Button
                disabled={
                  !m.target || asset.kind !== "image" || !asset.readable
                }
                onClick={() => {
                  m.w.navigatePath(
                    "/c/" +
                      encodeURIComponent(m.target) +
                      "/media/avatar?asset=" +
                      encodeURIComponent(id),
                  );
                }}
              >
                选择为显示头像
              </Button>
            </>
          )
        }
      >
        <div className="replica-media r-form-stack">
          <MediaReader source={detail} label="素材详情" />
          <Button
            variant="outline"
            disabled={detail.busy}
            onClick={detail.load}
          >
            刷新素材
          </Button>
          {asset && (
            <>
              <SavedMedia
                model={m}
                asset={asset}
                className="r-media-detail-art"
              />
              <div className="r-form-grid">
                {[
                  ["来源", mediaSource(asset)],
                  [
                    "创建时间",
                    asset.created_at ? userInstant(asset.created_at) : "未记录",
                  ],
                  [
                    "类型与大小",
                    mediaKind(asset.kind) + " · " + mediaBytes(asset.byte_size),
                  ],
                  ["读取状态", mediaReadState(asset)],
                ].map(([label, value]) => (
                  <div className="profile-key-value" key={label}>
                    <span>{label}</span>
                    <div>{value}</div>
                  </div>
                ))}
              </div>
              <WorkspaceInfo>
                这是保存的素材，不会据此改变角色的穿着、位置或拥有关系。来源快照只使用生成时的实际记录。
              </WorkspaceInfo>
              {asset.source_snapshot?.reference_asset?.asset_id && (
                <a
                  href={
                    "#/media/" +
                    encodeURIComponent(
                      asset.source_snapshot.reference_asset.asset_id,
                    )
                  }
                >
                  查看原始参考素材
                </a>
              )}
              <Disclosure title="生成时来源与资产校验信息">
                <pre className="replica-source-record">
                  {JSON.stringify(asset, null, 2)}
                </pre>
              </Disclosure>
              {advanced && (
                <>
                  <MediaGrants key={"grants:" + id} model={m} asset={asset} />
                  <MediaLifecycle
                    key={"lifecycle:" + id}
                    model={m}
                    asset={asset}
                    onAsset={adopt}
                  />
                </>
              )}
            </>
          )}
        </div>
      </WorkspaceDialog>
      {action&&<WorkspaceCommandDialog action={action} onClose={()=>setAction(null)} returnFocus={()=>source.current}/>}

    </>
  );
}
