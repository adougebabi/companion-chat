export const studioKindNames = {
  world: "世界底稿",
  character_template: "角色模板",
  place: "地点",
  item: "物品",
  outfit: "服装方案",
  npc: "世界人物",
  event_template: "事件",
  persona_fragment: "人格资料",
  visual_style: "视觉参考",
};
export const studioJobNames = {
  assistant: "助手建议",
  text_preview: "文本预览",
  image_preview: "图片预览",
};
export function studioVersion(value, empty = "尚未保存") {
  return value
    ? /^\d+$/.test(String(value))
      ? "v" + value
      : String(value)
    : empty;
}
export function studioCompactTime(value) {
  return String(value || "").replace(
    /^\d{4}-(\d{2})-(\d{2}) (\d{2}:\d{2}).*$/,
    "$1/$2 $3",
  );
}
export function studioSummary(row) {
  return (
    ["background", "description", "appearance", "role", "constraints"]
      .map((key) => row.body?.[key])
      .find((value) => typeof value === "string" && value.trim()) ||
    "打开后查看完整定义和来源。"
  );
}
export function studioTargetHref(kind, fid, result = {}) {
  const root = "#/c/" + encodeURIComponent(fid);
  if (result.target_kind === "foundation" && result.proposal_id)
    return (
      root + "/governance/foundation/" + encodeURIComponent(result.proposal_id)
    );
  return (
    root +
    {
      world: "/worlds",
      place: "/worlds",
      item: "/world/items",
      outfit: "/life/wardrobe",
      npc: "/people",
      event_template: "/world/events",
      visual_style: "/media/identity",
      character_template: "/profile",
      persona_fragment: "/profile",
    }[kind]
  );
}
export function studioFieldLabel(field) {
  return (
    {
      name: "名称",
      background: "世界背景",
      rules: "基础规则",
      framework: "场地框架",
      description: "描述",
      flavor: "风格",
      item_pool: "物品池",
      appearance: "外貌要点",
      clothing: "服装风格",
      constraints: "一致性约束",
      references: "参考说明",
      role: "身份 / 职业",
      members: "服饰成员",
      definition: "事件定义",
      changes: "身份资料修改",
      initialization: "初始化定义",
      core_persona: "身份核心",
      alias: "别名",
      floor: "楼层",
      scene: "场景",
      atmosphere: "氛围",
      furniture: "固定陈设",
      layout: "布局",
      connections: "地点连接",
      room_items: "室内物品",
    }[field] || field
  );
}
