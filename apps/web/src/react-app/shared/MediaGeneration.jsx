import { useEffect, useRef, useState } from "react";
import { WorldArt } from "./WorkspaceArtwork";
import { Sparkles, Plus, ImageIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Surface,
  Disclosure,
  FieldInput,
  Textarea,
  ChoiceSelect,
} from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import useMediaFollow from "./useMediaFollow";
import { MediaReader, MediaMore, SavedMedia } from "./MediaParts";
import {
  mediaReference,
  referencePurposes,
  requiredText,
} from "@/approved-ui/media-presentation.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
import { stateLabel, errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
function GenerationFields({ model: m, draft: d, edit }) {
  const fid = m.target,
    [query, setQuery] = useState(""),
    [search, setSearch] = useState("");
  const locations = useWorkspaceRead({
      identity: ["media-places", fid],
      characterScoped: false,
      read: async () => {
        const row = await m.service.locations(fid);
        if (!Array.isArray(row.places))
          throw Error("当前世界地点尚未完整返回。");
        return {
          items: row.places.filter((p) => p.status === "active"),
          next_cursor: "",
        };
      },
    }),
    items = useWorkspaceRead({
      identity: ["media-items", fid, query],
      characterScoped: false,
      read: (cursor) =>
        m.w.services.inventoryClient.list(fid, { q: query, cursor }),
    }),
    grants = useWorkspaceRead({
      identity: ["media-authorized", fid],
      characterScoped: false,
      read: (cursor) =>
        m.service.grants({
          target_scope: "fluctlight",
          target_id: fid,
          status: "active",
          cursor,
        }),
    });
  const linked = m.route.query?.get("grant"),
    link = useWorkspaceRead({
      identity: ["media-linked-grant", fid, linked],
      characterScoped: false,
      enabled: !!linked,
      read: async () => {
        const g = await m.service.grant(linked);
        mediaReference(g, fid);
        return { items: [g], next_cursor: "" };
      },
    });
  const places = locations.items,
    selectedPlace =
      d.place && !places.some((p) => p.id === d.place.id) ? [d.place] : [];
  return (
    <div className="r-form-stack">
      <label className="replica-field">
        <span>画面描述</span>
        <Textarea
          aria-label="画面描述"
          placeholder="例如：想看看你在书店窗边看书的样子，午后的光线轻轻落在书页上。"
          rows={5}
          value={d.prompt}
          onChange={(e) => edit({ prompt: e.target.value })}
        />
      </label>
      <div className="r-prompt-chips">
        {["窗边阅读的午后", "河岸边的秋日散步", "当前真实穿搭的全身插画"].map(
          (text) => (
            <Button
              type="button"
              variant="ghost"
              key={text}
              onClick={() => edit({ prompt: "请画一张" + text + "。" })}
            >
              {text}
              <Plus size={11} />
            </Button>
          ),
        )}
      </div>
      <label className="replica-field">
        <span>画面地点</span>
        <ChoiceSelect
          aria-label="画面地点"
          value={d.place?.id || ""}
          onChange={(e) => {
            const p = places.find((p) => p.id === e.target.value);
            edit({
              place: p
                ? { id: p.id, name: p.name, revision: p.revision }
                : null,
            });
          }}
        >
          <option value="">不指定地点</option>
          {[...places, ...selectedPlace].map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
              {selectedPlace.includes(p) ? "（原选择，请核对）" : ""}
            </option>
          ))}
        </ChoiceSelect>
      </label>
      <MediaReader source={locations} label="画面地点" />
      <Disclosure
        title={"关联登记物品 · " + (d.item?.description || "不关联物品")}
      >
        <div className="r-form-stack">
          <div className="r-search">
            <FieldInput
              aria-label="搜索登记物品"
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
          <Button
            type="button"
            variant="ghost"
            onClick={() => edit({ item: null })}
          >
            不关联物品
          </Button>
          <MediaReader source={items} label="登记物品" />
          {items.items.map((item) => (
            <div className="media-picker-row" key={item.id}>
              <span>
                {item.description} · v{item.revision}
              </span>
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  edit({
                    item: {
                      id: item.id,
                      revision: item.revision,
                      description: item.description,
                    },
                  })
                }
              >
                关联画面
              </Button>
            </div>
          ))}
          <MediaMore source={items} label="物品" />
        </div>
      </Disclosure>
      <Disclosure
        title={
          "参考图片 · " +
          (d.reference
            ? referencePurposes[d.reference.purpose]
            : "不使用参考图片")
        }
      >
        <div className="r-form-stack">
          <WorkspaceInfo>
            仅使用明确授权给当前角色的用途。人物画面参考不证明角色身份。
          </WorkspaceInfo>
          <Button
            type="button"
            variant="ghost"
            onClick={() => edit({ reference: null })}
          >
            不使用参考图片
          </Button>
          <MediaReader source={grants} label="已授权图片" />
          {grants.items.map((g) => (
            <div className="media-picker-row" key={g.id}>
              <span>
                {referencePurposes[g.purpose] || "参考"} · 授权 v{g.revision}
              </span>
              <Button
                type="button"
                variant="outline"
                onClick={() => edit({ reference: mediaReference(g, fid) })}
              >
                选择参考
              </Button>
            </div>
          ))}
          <MediaMore source={grants} label="图片授权" />
          <a href="#/media">管理素材与参考授权</a>
        </div>
      </Disclosure>
      {linked && (
        <>
          <MediaReader source={link} label="链接中的参考授权" />
          {link.items[0] && (
            <WorkspaceInfo>
              链接中的参考保持原用途和版本。
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  edit({ reference: mediaReference(link.items[0], fid) })
                }
              >
                明确改用这份参考
              </Button>
            </WorkspaceInfo>
          )}
        </>
      )}
      <label className="replica-field">
        <span>请求原因</span>
        <Textarea
          aria-label="请求原因"
          value={d.reason}
          onChange={(e) => edit({ reason: e.target.value })}
        />
      </label>
      <WorkspaceInfo>
        生成图·非事实。角色可以拒绝或暂缓；画面不会移动角色、取得物品或改变实际穿着。
      </WorkspaceInfo>
    </div>
  );
}
export default function MediaGeneration({ model: m }) {
  const fid = m.target,
    slot = `fluctlight-media:${m.owner}:${fid}:replica-generation`,
    [epoch, setEpoch] = useState(0),
    [selected, setSelected] = useState(m.route.query?.get("request") || ""),
    [notice, setNotice] = useState(""),
    requestRef = useRef(null),
    historyRef = useRef(null);
  useEffect(() => {
    if (m.route.query?.get("request")) return;
    try {
      const v = JSON.parse(
        studioDraftStorage().getItem(slot + ":result") || "null",
      );
      if (v?.id) setSelected(v.id);
    } catch {}
  }, [slot]);
  const history = useWorkspaceRead({
    identity: ["media-request-history", fid],
    characterScoped: false,
    read: (cursor) => m.service.generations(fid, cursor),
  });
  historyRef.current = history;
  async function read(id) {
    const r = await m.service.generation(fid, id);
    if (
      r.fluctlight_id !== fid ||
      r.request?.id !== id ||
      r.request?.fluctlight_id !== fid
    )
      throw Error("图片请求范围或身份不匹配。");
    return r;
  }
  const result = useMediaFollow({
    identity: ["media-generation-result", fid, selected],
    enabled: !!selected,
    read: () => read(selected),
    pending: (r) =>
      r.media
        ? ["pending", "running"].includes(r.media.status)
        : [
            "pending",
            "processing",
            "accepted",
            "deferred",
            "awaiting_goal",
          ].includes(r.request.status),
  });
  requestRef.current = result;
  const record = result.items[0],
    ownerRequest = record?.request,
    media = record?.media,
    asset = record?.asset,
    failed =
      media?.status === "failed" ||
      ["failed", "rejected", "cancelled"].includes(ownerRequest?.status) ||
      !!ownerRequest?.result?.error_code,
    complete = media?.status === "completed" && asset?.readable;
  const action = {
    identity: "media-generation:" + fid + ":" + epoch,
    presentation: "inline",
    scope: ["media-generation", fid],
    storageKey: slot,
    title: "想留下一幅怎样的画面？",
    description: "描述想法，再由角色决定如何表达。",
    submitLabel: "提交图片请求",
    cancelLabel: "重置当前草稿",
    resetDraft: () => ({
      prompt: m.route.query?.get("prompt") || "",
      reason: "",
      item: null,
      place: null,
      reference: null,
    }),
    disabled: m.readOnly,
    canSubmit: (d) => !!d?.prompt?.trim() && !!d?.reason?.trim(),
    preAdmissionConflicts: [
      "revision_conflict",
      "media_reference_authorization_changed",
      "media_reference_source_changed",
    ],
    initial: {
      prompt: m.route.query?.get("prompt") || "",
      reason: "",
      item: null,
      place: null,
      reference: null,
    },
    validate: (d) => {
      m.assertWritable();
      requiredText(d.prompt, "画面描述", 2000);
      requiredText(d.reason, "请求原因", 500);
      if (d.item && !Number.isSafeInteger(d.item.revision))
        throw Error("所选物品版本尚未确认。");
      if (d.place && !Number.isSafeInteger(d.place.revision))
        throw Error("所选地点版本尚未确认。");
    },
    build: (d) => ({
      prompt: d.prompt,
      reason: d.reason,
      ...(d.item
        ? { itemId: d.item.id, expectedItemRevision: d.item.revision }
        : {}),
      ...(d.place
        ? { placeId: d.place.id, expectedPlaceRevision: d.place.revision }
        : {}),
      ...(d.reference ? { referenceAsset: { ...d.reference } } : {}),
    }),
    execute: async (body, key) => {
      m.assertWritable();
      const r = await m.service.generate(fid, { ...body, idempotencyKey: key });
      if (!r.request?.id || r.request.fluctlight_id !== fid)
        throw Error("图片请求未返回真实受理身份。");
      return r;
    },
    afterAcknowledged: (d, r) => ({ ...d, lastRequestId: r.request.id }),
    readLatest: async (d) => {
      const fingerprint = JSON.stringify({
        item: d.item,
        place: d.place,
        reference: d.reference,
      });
      const item = d.item
          ? await m.w.services.inventoryClient.read(fid, d.item.id)
          : null,
        world = await m.service.locations(fid);
      if (d.reference) {
        const g = await m.service.grant(d.reference.grantId);
        if (
          JSON.stringify(mediaReference(g, fid)) !== JSON.stringify(d.reference)
        )
          throw Error("原参考授权已变化，请明确选择新授权或不使用参考。");
      }
      const place = d.place
        ? world.places?.find(
            (p) => p.id === d.place.id && p.status === "active",
          )
        : null;
      if (d.place && !place)
        throw Error("原地点不在当前运行世界，请明确重新选择或取消限制。");
      return { fingerprint, item, place };
    },
    adoptLatest: (d, row) => {
      if (
        JSON.stringify({
          item: d.item,
          place: d.place,
          reference: d.reference,
        }) !== row.fingerprint
      )
        throw Error("来源选择已经变化，请保留新选择并重新核对。");
      return {
        ...d,
        item: row.item
          ? {
              id: row.item.id,
              revision: row.item.revision,
              description: row.item.description,
            }
          : null,
        place: row.place
          ? {
              id: row.place.id,
              name: row.place.name,
              revision: row.place.revision,
            }
          : null,
      };
    },
    onSaved: async (r) => {
      const fresh = await read(r.request.id);
      if (m.live()) {
        studioDraftStorage().setItem(
          slot + ":result",
          JSON.stringify({ id: r.request.id }),
        );
        setSelected(r.request.id);
        setNotice("图片请求已受理。角色判断、生成任务和实际图片分别显示。");
        await historyRef.current.load();
      }
    },
    fields: (d, edit) => <GenerationFields model={m} draft={d} edit={edit} />,
  };
  return (
    <div className="r-generation-layout">
      <div className="r-form-stack">
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">想留下一幅怎样的画面？</h2>
          <p className="r-muted">描述你的想法，再交给角色决定如何表达。</p>
          {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
          <WorkspaceCommandDialog
            action={action}
            onClose={() => setEpoch((e) => e + 1)}
          />
        </Surface>
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">图片请求记录</h2>
          <MediaReader source={history} label="图片请求记录" />
          {history.items.map((r) => (
            <button
              key={r.id}
              className={"r-job-row" + (selected === r.id ? " active" : "")}
              onClick={() => setSelected(r.id)}
            >
              <span>
                <b>{r.payload?.prompt || "原图片请求"}</b>
                <small>
                  {r.created_at ? userInstant(r.created_at) : "时间未记录"}
                </small>
              </span>
              <span>{stateLabel(r.status)}</span>
            </button>
          ))}
          {history.ready && !history.items.length && (
            <p className="r-muted">新的画面，会从你的第一个想法开始。</p>
          )}
          <MediaMore source={history} label="图片请求" />
        </Surface>
      </div>
      <Surface className="panel r-padded r-image-result">
        <h2 className="r-inline-title">画面正在发生</h2>
        <MediaReader source={result} label="实际图片结果" />
        {!selected && (
          <div className="r-result-empty">
            <WorldArt kind="city" />
            <h3>把一个喜欢的瞬间，留在这里</h3>
            <p>选择原请求，或提出第一个想法。</p>
          </div>
        )}
        {record && (
          <div className="r-form-stack">
            <div className="r-request-stages">
              {[
                [
                  "角色处理",
                  ["accepted", "completed"].includes(ownerRequest.status),
                ],
                ["生成画面", media?.status === "completed"],
                ["收进媒体库", complete],
              ].map(([label, done], index) => (
                <div key={label} className={done ? "done" : ""}>
                  <span>{done ? "✓" : index + 1}</span>
                  {label}
                </div>
              ))}
            </div>
            <p>
              {stateLabel(ownerRequest.status)} ·{" "}
              {media ? stateLabel(media.status) : "尚无已受理的生成任务"}
            </p>
            {failed ? (
              <p role="alert">
                {errorMessage(
                  {
                    message:
                      media?.error_code ||
                      ownerRequest.decision_reason ||
                      ownerRequest.result?.error_code,
                  },
                  "本次图片请求未完成，请查看原请求和生成记录。",
                )}
              </p>
            ) : complete ? (
              <WorkspaceInfo>图片已生成，实际资产已读回。</WorkspaceInfo>
            ) : media?.status === "completed" ? (
              <p role="alert">生成记录已完成，图片资产尚未确认可读取。</p>
            ) : (
              <WorkspaceInfo>
                正在核对原请求。关闭页面只停止观察，不会重新提交图片请求。
              </WorkspaceInfo>
            )}
            {asset && (
              <>
                <SavedMedia
                  model={m}
                  asset={asset}
                  className="r-generated-result"
                />
                <Button
                  onClick={(event) =>
                    m.openAsset(asset.id, event.currentTarget)
                  }
                >
                  查看实际素材与来源
                </Button>
              </>
            )}
            <a href={"#/diag/request/" + encodeURIComponent(ownerRequest.id)}>
              查看原请求与判断
            </a>
            {(ownerRequest.tasks || []).map((t) => (
              <a
                key={t.task_id}
                href={"#/diag/task/" + encodeURIComponent(t.task_id)}
              >
                查看后台任务 ·{" "}
                {t.status === "completed"
                  ? "执行已结束，图片另行核对"
                  : stateLabel(t.status)}
              </a>
            ))}
            <Button
              variant="outline"
              disabled={result.busy}
              onClick={result.load}
            >
              继续查询实际结果
            </Button>
            {result.paused && (
              <p>自动查询已暂停，后台任务仍保留，可手动继续查询。</p>
            )}
            <Disclosure title="冻结来源、实际任务与图片结果">
              <pre className="replica-source-record">
                {JSON.stringify(record, null, 2)}
              </pre>
            </Disclosure>
          </div>
        )}
      </Surface>
    </div>
  );
}
