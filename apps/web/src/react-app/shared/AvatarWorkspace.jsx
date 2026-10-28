import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Surface, Disclosure, FieldInput, Textarea } from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import { MediaReader, MediaMore, SavedMedia, MediaById } from "./MediaParts";
import { requiredText } from "@/approved-ui/media-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
export default function AvatarWorkspace({ model: m }) {
  const fid = m.target,
    [search, setSearch] = useState(""),
    [query, setQuery] = useState(""),
    [epoch, setEpoch] = useState(0),
    [notice, setNotice] = useState(""),
    freshRef = useRef(null),
    formRoot = useRef(null);
  const avatar = useWorkspaceRead({
    identity: ["display-avatar", fid],
    characterScoped: false,
    read: async () => {
      const r = await m.service.avatar(fid);
      if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.revision))
        throw Error("头像范围或版本不匹配。");
      return { items: [r], next_cursor: "" };
    },
  });
  freshRef.current = avatar;
  const images = useWorkspaceRead({
    identity: ["avatar-images", fid, query],
    characterScoped: false,
    read: (cursor) => m.service.list({ kind: "image", q: query, cursor }),
  });
  const history = useWorkspaceRead({
      identity: ["avatar-history", fid],
      characterScoped: false,
      read: (cursor) => m.service.avatarHistory(fid, cursor),
    }),
    value = avatar.items[0];
  const linkedId = m.route.query?.get("asset");
  const link = useWorkspaceRead({
    identity: ["linked-avatar-asset", linkedId],
    characterScoped: false,
    enabled: !!linkedId,
    read: async () => ({
      items: [await m.readAsset(linkedId)],
      next_cursor: "",
    }),
  });
  const initial = value
    ? {
        mode: value.mode === "asset" ? "asset" : "canonical",
        selected: value.selection?.asset_id
          ? {
              id: value.selection.asset_id,
              asset_revision: value.selection.asset_revision,
              title: "原已选图片",
              saved: true,
              version: value.selection.version,
              sha256: value.selection.sha256,
            }
          : null,
        revision: value.revision,
        reason: "",
      }
    : null;
  const action = value
    ? {
        identity: "avatar:" + fid + ":" + epoch,
        presentation: "inline",
        scope: ["media-avatar", fid],
        storageKey: `fluctlight-media:${m.owner}:${fid}:replica-avatar`,
        title: "选择喜欢的显示方式",
        submitLabel: "保存显示头像",
        cancelLabel: "撤销更改",
        resetDraft: () => initial,
        disabled: m.readOnly,
        initial,
        validate: (d) => {
          m.assertWritable();
          requiredText(d.reason, "选择原因", 1000);
          if (d.mode === "asset" && !d.selected?.id)
            throw Error("请选择一张可读取的图片。");
        },
        build: (d) => ({
          mode: d.mode,
          expectedRevision: d.revision,
          reason: d.reason,
          ...(d.mode === "asset"
            ? {
                assetId: d.selected.id,
                expectedAssetRevision: d.selected.asset_revision,
              }
            : {}),
        }),
        execute: async (body, key) => {
          m.assertWritable();
          const r = await m.service.selectAvatar(fid, {
            ...body,
            idempotencyKey: key,
          });
          if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.revision))
            throw Error("显示头像回执尚未完整确认。");
          return r;
        },
        afterAcknowledged: (d, r) => ({ ...d, revision: r.revision }),
        readLatest: () => m.service.avatar(fid),
        adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
        onSaved: async (r, body) => {
          const row = await m.service.avatar(fid);
          if (
            row.fluctlight_id !== fid ||
            row.revision < r.revision ||
            (row.revision === r.revision &&
              (row.mode !== body.mode ||
                (body.mode === "asset" &&
                  (row.selection?.asset_id !== body.assetId ||
                    row.selection?.asset_revision !==
                      body.expectedAssetRevision))))
          )
            throw Error("显示头像的新选择尚未完整读回。");
          if (m.live()) {
            freshRef.current.adopt({ items: [row], next_cursor: "" });
            setNotice("显示头像已保存并读回。");
            await history.load();
            try {
              await m.service.refresh();
            } catch {
              if (m.live())
                setNotice("头像已保存，其他页面资料暂未刷新，可重新读取。");
            }
          }
        },
        fields: (d, edit) => {
          const actualSrc = value.avatar?.url
            ? value.avatar.url || m.service.url(value.avatar.asset_id)
            : null;
          return (
            <div className="r-avatar-workspace">
              <Surface className="panel r-avatar-preview">
                <span className="r-tiny-label">A FAMILIAR FACE</span>
                <div className="r-avatar-big">
                  {d.mode === "asset" &&
                  d.selected?.id &&
                  d.selected.saved === false ? (
                    <MediaById
                      id={d.selected.id}
                      model={m}
                      className="media-avatar-image"
                      expected={d.selected}
                    />
                  ) : actualSrc &&
                    d.mode === value.mode &&
                    (d.mode !== "asset" ||
                      d.selected?.id === value.selection?.asset_id) ? (
                    <img
                      src={actualSrc}
                      alt={(m.actor?.name || "角色") + "的当前显示头像"}
                    />
                  ) : (
                    <span>{[...(m.actor?.name || "角色")][0]}</span>
                  )}
                </div>
                <h2>{m.actor?.name || "角色"}</h2>
                <p>{d.mode === "asset" ? "明确选择的素材" : "跟随规范参考"}</p>
                <small>
                  显示头像独立保存。名称首字是没有可读图片时的自动回退。
                </small>
                {!value.avatar && (
                  <p className="r-fineprint">
                    当前保存的头像来源尚不可用，实际显示回退为名字首字。
                  </p>
                )}
              </Surface>
              <Surface className="panel r-padded r-form-stack">
                <div>
                  <div className="r-inline-between">
                    <h2 className="r-inline-title">选择喜欢的显示方式</h2>
                    <Button type="button" variant="ghost" onClick={avatar.load}>
                      重新读取头像
                    </Button>
                  </div>
                  <p className="r-muted">
                    角色库、聊天和资料会使用实际保存的头像。
                  </p>
                </div>
                {link.items[0] && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={
                      !link.items[0].readable || link.items[0].kind !== "image"
                    }
                    onClick={() =>
                      edit({
                        mode: "asset",
                        selected: {
                          id: link.items[0].id,
                          asset_revision: link.items[0].asset_revision,
                          title: link.items[0].title || "链接中的图片",
                          saved: false,
                          version: link.items[0].version,
                          sha256: link.items[0].sha256,
                        },
                      })
                    }
                  >
                    明确选择链接中的图片
                  </Button>
                )}
                <div className="r-avatar-modes">
                  {[
                    [
                      "canonical",
                      "跟随规范参考",
                      "使用当前已接受的视觉身份图片。",
                    ],
                    [
                      "asset",
                      "选择一张媒体图片",
                      "明确选择已有可读图片及其版本。",
                    ],
                  ].map(([id, title, desc]) => (
                    <Button
                      key={id}
                      variant="ghost"
                      type="button"
                      disabled={m.readOnly}
                      aria-pressed={d.mode === id}
                      className={d.mode === id ? "selected" : ""}
                      onClick={() => edit({ mode: id })}
                    >
                      <span className="r-radio-dot">
                        {d.mode === id && <i />}
                      </span>
                      <span>
                        <b>{title}</b>
                        <small>{desc}</small>
                      </span>
                    </Button>
                  ))}
                </div>
                {d.mode === "asset" && (
                  <>
                    <p className="r-muted">
                      当前选择：{d.selected?.title || "尚未选择图片"}
                    </p>
                    <div className="r-search">
                      <FieldInput
                        aria-label="搜索头像图片"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                      />
                      <Button
                        type="button"
                        variant="outline"
                        onClick={() => setQuery(search.trim())}
                      >
                        搜索
                      </Button>
                    </div>
                    <MediaReader source={images} label="可选头像图片" />
                    <div className="r-avatar-picker">
                      {images.items.map((a) => (
                        <button
                          type="button"
                          aria-label={
                            "选择头像 " +
                            (a.title || a.source_filename || "图片")
                          }
                          disabled={m.readOnly || !a.readable}
                          key={a.id}
                          className={d.selected?.id === a.id ? "selected" : ""}
                          onClick={() =>
                            edit({
                              selected: {
                                id: a.id,
                                asset_revision: a.asset_revision,
                                version: a.version,
                                sha256: a.sha256,
                                saved: false,
                                title: a.title || a.source_filename,
                              },
                            })
                          }
                        >
                          <SavedMedia
                            asset={a}
                            model={m}
                            thumbnail
                            className="media-avatar-choice"
                          />
                          <span>
                            {a.title || a.source_filename || "未命名图片"}
                          </span>
                        </button>
                      ))}
                    </div>
                    <MediaMore source={images} label="图片" />
                    {images.ready && !images.items.length && (
                      <p>还没有可选图片。</p>
                    )}
                  </>
                )}
                <label className="replica-field">
                  <span>选择原因</span>
                  <Textarea
                    aria-label="选择原因"
                    value={d.reason}
                    disabled={m.readOnly}
                    onChange={(e) => edit({ reason: e.target.value })}
                  />
                </label>
                <WorkspaceInfo>
                  只改变显示头像，不修改规范视觉身份、身体、穿着或模型参考授权。
                </WorkspaceInfo>
              </Surface>
            </div>
          );
        },
      }
    : null;
  return (
    <div className="r-form-stack" ref={formRoot}>
      <MediaReader source={avatar} label="显示头像" />

      {linkedId && (
        <>
          <MediaReader source={link} label="链接中的头像图片" />
          {link.items[0] && (
            <WorkspaceInfo>
              链接中的图片：{link.items[0].title || "已有图片"}
              。请在“选择一张媒体图片”中明确选择并保存，不会自动更换头像。
            </WorkspaceInfo>
          )}
        </>
      )}
      {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
      {action && (
        <WorkspaceCommandDialog
          action={action}
          returnFocus={() =>
            formRoot.current?.querySelector('button[type="submit"]')
          }
          onClose={() => setEpoch((e) => e + 1)}
        />
      )}
      <Surface className="panel r-padded">
        <h2 className="r-inline-title">显示头像选择历史</h2>
        <MediaReader source={history} label="头像历史" />
        {history.items.map((row, index) => (
          <div className="r-timeline-entry" key={row.id || index}>
            <b>选择版本 {row.revision}</b>
            <p>{row.reason}</p>
            <small>
              {row.created_at ? userInstant(row.created_at) : "时间未记录"}
            </small>
            <Disclosure title="选择来源与前后状态">
              <pre className="replica-source-record">
                {JSON.stringify(row, null, 2)}
              </pre>
            </Disclosure>
          </div>
        ))}
        {history.ready && !history.items.length && <p>尚未保存头像选择。</p>}
        <MediaMore source={history} label="头像历史" />
      </Surface>
    </div>
  );
}
