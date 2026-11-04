import { useRef, useState,useEffect } from "react";
import {ResultFeedback} from './Presentation';
import { Button } from "@/components/ui/button";
import {RadioGroup,RadioGroupItem} from "@/components/ui/radio-group";
import { Surface, Disclosure, FieldInput, Textarea, Badge } from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import useMediaFollow from "./useMediaFollow";
import { MediaReader, MediaMore, SavedMedia, MediaById } from "./MediaParts";
import { requiredText } from "@/approved-ui/media-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { avatarInitial } from "@/lib/avatar-presentation.mjs";
export default function AvatarWorkspace({ model: m }) {
  const fid = m.target,
    [search, setSearch] = useState(""),
    [query, setQuery] = useState(""),
    [epoch, setEpoch] = useState(0),
    [notice, setNotice] = useState(null),
    [generationAction, setGenerationAction] = useState(null),
    freshRef = useRef(null),
    formRoot = useRef(null),
    generationTrigger = useRef(null);
  const avatar = useMediaFollow({
    identity: ["display-avatar", fid],
    read: async () => {
      const r = await m.service.avatar(fid);
      if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.revision))
        throw Error("头像范围或版本不匹配。");
      return r;
    },
    pending: (r) => r.generation?.status === "generating",
  });
  useEffect(()=>setNotice(null),[fid,m.owner]);
  async function refreshRelated(){
    setNotice({kind:'progress',text:'正在重新读取其他页面的头像资料…'});
    try{await m.service.refresh();if(m.live())setNotice(null);}
    catch{if(m.live())setNotice({kind:'warning',text:'头像已保存，其他页面资料暂未刷新。',actions:<Button type="button" variant="outline" onClick={()=>void refreshRelated()}>重新读取其他页面资料</Button>});}
  }
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
  function generateAvatar(event) {
    if (!value?.reference_available || !Number.isSafeInteger(value.visual_revision)) return;
    m.assertWritable();
    generationTrigger.current = event.currentTarget;
    setGenerationAction({
      identity: "generated-avatar:" + fid,
      scope: ["generated-avatar", fid],
      storageKey: `fluctlight-media:${m.owner}:${fid}:generated-avatar`,
      title: "生成独立头像",
      description: "根据已确认的形象参考生成单独的头像；已手动选择的显示头像不会被覆盖。",
      submitLabel: "开始生成头像",
      initial: { visualRevision: value.visual_revision },
      build: (d) => ({ expectedVisualRevision: d.visualRevision }),
      execute: async (body, key) => {
        m.assertWritable();
        const result = await m.service.generateAvatar(fid, { ...body, idempotencyKey: key });
        if (result.fluctlight_id !== fid || result.visual_revision !== body.expectedVisualRevision || typeof result.id !== "string" || !result.id || typeof result.review_id !== "string" || !result.review_id)
          throw Error("头像生成回执尚未完整确认。");
        return result;
      },
      readLatest: () => m.service.avatar(fid),
      adoptLatest: (d, row) => {
        if (row.fluctlight_id !== fid || !row.reference_available || !Number.isSafeInteger(row.visual_revision))
          throw Error("形象参考尚未确认，请重新读取后生成头像。");
        return { ...d, visualRevision: row.visual_revision };
      },
      onSaved: async (result, body) => {
        const original = await m.w.client.generatedVisualAssetReview(fid, result.review_id);
        if (original.fluctlight_id !== fid || original.id !== result.review_id || original.kind !== "avatar" || original.media_intent_id !== result.id || original.visual_revision !== body.expectedVisualRevision)
          throw Error("原头像生成任务尚未完整读回。");
        const fresh = await m.service.avatar(fid);
        if (fresh.fluctlight_id !== fid || !Number.isSafeInteger(fresh.visual_revision)) throw Error("当前头像状态尚未完整读回。");
        if (m.live()) {
          freshRef.current.adopt({ items: [fresh], next_cursor: "" });
          const sameTask = fresh.visual_revision === body.expectedVisualRevision && fresh.generation?.media_intent_id === result.id;
          setNotice({ kind: "information", text: !sameTask ? "原头像任务已记录；当前形象已变化，请以当前状态为准。" : fresh.generation?.status === "awaiting_review" ? "头像已生成，等待审核。" : "头像生成已排队，完成后按实际审核状态显示。" });
        }
      },
      fields: () => <p>生成完成后会单独审核，头像与形象参考图分别保存。</p>,
    });
  }
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
            setNotice({kind:'success',text:"显示头像已保存并读回。"});
            await history.load();
            try {
              await m.service.refresh();
            } catch {
              if (m.live())
                setNotice({kind:'warning',text:"头像已保存，其他页面资料暂未刷新。",actions:<Button type="button" variant="outline" onClick={()=>void refreshRelated()}>重新读取其他页面资料</Button>});
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
                    <span>{avatarInitial(m.actor?.name, "", "角")}</span>
                  )}
                </div>
                <h2>{m.actor?.name || "角色"}</h2>
                <p>{m.actor?.job || (d.mode === "asset" ? "明确选择的素材" : "使用生成的头像")}</p>
                <Badge variant="outline" className="r-avatar-independent">显示头像独立保存</Badge>
                <small>
                  更换头像不会替换角色的规范参考图。
                </small>
                {!value.avatar && (
                  <p className="r-fineprint">
                    {value.read_state === "generation_needed"
                      ? "尚未生成独立头像，可以在右侧开始生成。"
                      : value.read_state === "generating"
                        ? "独立头像正在生成，完成后会进入审核。"
                        : value.read_state === "awaiting_review"
                          ? "独立头像已生成，等待人工审核。"
                          : value.read_state === "failed"
                            ? "独立头像生成未完成，可以重新生成。"
                            : "当前头像来源暂不可用，实际显示回退为名字首字。"}
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
                  {value.generation?.status === "awaiting_review" && (
                    <WorkspaceInfo>
                      独立头像已生成，等待审核。
                      <Button variant="outline" asChild><a href={`#/c/${encodeURIComponent(fid)}/media/identity`}>前往审核头像</a></Button>
                    </WorkspaceInfo>
                  )}
                  <Button type="button" variant="outline" disabled={m.readOnly || !value.reference_available || !Number.isSafeInteger(value.visual_revision)} onClick={generateAvatar}>
                    {value.generation?.status === "generating" ? "查看头像生成" : value.generation?.status === "failed" ? "重新生成头像" : value.generation?.status === "accepted" ? "生成新头像" : "生成头像"}
                  </Button>
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
                <RadioGroup className="r-avatar-modes" aria-label="显示头像方式" value={d.mode} onValueChange={mode=>edit({mode})} disabled={m.readOnly}>
                  {[
                    [
                      "canonical",
                      "使用生成的头像",
                      "使用根据已确认形象单独生成并通过审核的头像。",
                    ],
                    [
                      "asset",
                      "选择一张媒体图片",
                      "明确选择已有可读图片及其版本。",
                    ],
                  ].map(([id, title, desc]) => (
                    <label key={id} className={"r-avatar-option"+(d.mode===id?" selected":"")}>
                      <RadioGroupItem value={id} aria-label={title}/>
                      <span>
                        <b>{title}</b>
                        <small>{desc}</small>
                      </span>
                    </label>
                  ))}
                </RadioGroup>
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
      <ResultFeedback notice={notice} onDismiss={value=>setNotice(current=>current===value?null:current)}/>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          returnFocus={() =>
            formRoot.current?.querySelector('button[type="submit"]')
          }
          onClose={() => setEpoch((e) => e + 1)}
        />
      )}
      {generationAction && (
        <WorkspaceCommandDialog
          action={generationAction}
          returnFocus={() => generationTrigger.current}
          onClose={() => setGenerationAction(null)}
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
