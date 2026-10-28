export const mediaSource = (a) =>
  ({
    owner_upload: "我上传的素材",
    studio: "创作素材",
    fluctlight: "角色生成",
  })[a?.origin] || "已登记素材";
export const mediaKind = (k) =>
  ({ image: "图片", audio: "音频", video: "视频" })[k] || "媒体素材";
export const briefFields = [
  ["appearance", "外貌要点"],
  ["clothing", "服装风格"],
  ["constraints", "一致性约束"],
  ["references", "参考图说明"],
];
export const referencePurposes = {
  scene: "场景参考",
  animal: "动物参考",
  object: "物品参考",
  person: "人物画面参考",
};
export const mediaBytes = (v) =>
  Number.isFinite(Number(v))
    ? Number(v) >= 1048576
      ? (Number(v) / 1048576).toFixed(1) + " MB"
      : (Number(v) / 1024).toFixed(1) + " KB"
    : "大小未记录";
export function checkMediaAsset(row, id) {
  if (
    !row?.id ||
    (id && row.id !== id) ||
    !Number.isSafeInteger(row.asset_revision) ||
    !Number.isSafeInteger(row.metadata_revision)
  )
    throw Error("素材身份或双版本尚未完整确认。");
  return row;
}
export function mediaReference(g, fid) {
  if (
    g.status !== "active" ||
    g.target_scope !== "fluctlight" ||
    g.target_id !== fid ||
    !referencePurposes[g.purpose]
  )
    throw Error("这份参考授权当前不可用于该角色。");
  return {
    assetId: g.asset_id,
    purpose: g.purpose,
    grantId: g.id,
    grantRevision: g.revision,
  };
}
export function requiredText(text, label, max) {
  if (typeof text !== "string" || !text.trim() || [...text].length > max)
    throw Error("请填写" + label + "，最多" + max + "个字符。");
  return text;
}
export function mediaReferencePath(r, assetId) {
  const f = encodeURIComponent(r.fluctlight_id || ""),
    id = encodeURIComponent(r.target_id || "");
  switch (r.target_type) {
    case "studio_material":
      return "#/studio/material/" + id;
    case "display_avatar":
      return "#/c/" + f + "/media/avatar";
    case "visual_identity":
    case "visual_attempt":
    case "visual_session":
      return "#/c/" + f + "/media/identity";
    case "item":
      return "#/c/" + f + "/life/wardrobe/item/" + id;
    case "model_reference_grant":
      return "#/media/" + encodeURIComponent(assetId) + "?authorizeReference=1";
    case "conversation":
      return "#/c/" + f + "/chat?conversation=" + id;
    case "conversation_message":
      return (
        "#/c/" +
        f +
        "/chat" +
        (r.conversation_id
          ? "?conversation=" +
            encodeURIComponent(r.conversation_id) +
            "&message=" +
            id
          : "")
      );
    default:
      return "";
  }
}
export const mediaReadState = (asset) =>
  asset?.readable === true
    ? "可读取"
    : {
        source_unavailable: "来源暂不可用",
        upload_unsettled: "原上传尚未就绪",
        storage_scope_changed: "存储范围已变化",
        studio_result_unavailable: "创作结果尚未就绪",
        storage_not_configured: "存储尚未配置",
        tombstoned: "删除处理中，已禁止读取",
        deleted: "对象已删除",
        unavailable: "暂不可读取",
      }[asset?.read_state] || "素材暂不可读取";
export const visualSessionState = (value) =>
  ({
    queued: "等待准备",
    running: "正在准备候选",
    awaiting_review: "待人工评审",
    character_sheet_pending: "规范已接受，设定图准备中",
    completed: "本次准备已完成",
    failed: "本次准备失败",
    cancelled: "本次准备已取消",
    renderer_pending: "等待渲染配置",
  })[value] || "状态待确认";
