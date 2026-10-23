// Isolated approved-demo-fixture. Never included in a real production build.
import {h as vn,ref as G,computed as _e,defineComponent as nt,openBlock as p,createElementBlock as g,normalizeClass as ge,createElementVNode as o,Fragment as O,renderList as oe,unref as M,toDisplayString as v,nextTick as ls,watch as it,normalizeStyle as yt,createTextVNode as Z,createVNode as me,createCommentVNode as X,withDirectives as pe,withKeys as Gi,withModifiers as Ve,vModelText as $e,onMounted as wl,onUnmounted as Ks,createBlock as Ae,Teleport as ea,renderSlot as ha,vModelCheckbox as ss,Transition as ru,withCtx as Ht,vModelSelect as Et,resolveDynamicComponent as as,vModelDynamic as Vu,isRef as Re} from 'vue';
const Bu = (e) => e.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ var en = {
  xmlns: "http://www.w3.org/2000/svg",
  width: 24,
  height: 24,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  "stroke-width": 2,
  "stroke-linecap": "round",
  "stroke-linejoin": "round",
};
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Ku = (
  {
    size: e,
    strokeWidth: t = 2,
    absoluteStrokeWidth: s,
    color: n,
    iconNode: l,
    name: i,
    class: r,
    ...a
  },
  { slots: u },
) =>
  vn(
    "svg",
    {
      ...en,
      width: e || en.width,
      height: e || en.height,
      stroke: n || en.stroke,
      "stroke-width": s ? (Number(t) * 24) / Number(e) : t,
      class: ["lucide", `lucide-${Bu(i ?? "icon")}`],
      ...a,
    },
    [...l.map((f) => vn(...f)), ...(u.default ? [u.default()] : [])],
  );
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Oe =
  (e, t) =>
  (s, { slots: n }) =>
    vn(Ku, { ...s, iconNode: t, name: e }, n);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const wo = Oe("ChevronDownIcon", [
  ["path", { d: "m6 9 6 6 6-6", key: "qrunsl" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Ji = Oe("ChevronRightIcon", [
  ["path", { d: "m9 18 6-6-6-6", key: "mthhwq" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Wu = Oe("EarthIcon", [
  ["path", { d: "M21.54 15H17a2 2 0 0 0-2 2v4.54", key: "1djwo0" }],
  [
    "path",
    {
      d: "M7 3.34V5a3 3 0 0 0 3 3a2 2 0 0 1 2 2c0 1.1.9 2 2 2a2 2 0 0 0 2-2c0-1.1.9-2 2-2h3.17",
      key: "1tzkfa",
    },
  ],
  [
    "path",
    {
      d: "M11 21.95V18a2 2 0 0 0-2-2a2 2 0 0 1-2-2v-1a2 2 0 0 0-2-2H2.05",
      key: "14pb5j",
    },
  ],
  ["circle", { cx: "12", cy: "12", r: "10", key: "1mglay" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const $o = Oe("ImageIcon", [
  [
    "rect",
    {
      width: "18",
      height: "18",
      x: "3",
      y: "3",
      rx: "2",
      ry: "2",
      key: "1m3agn",
    },
  ],
  ["circle", { cx: "9", cy: "9", r: "2", key: "af1f0g" }],
  ["path", { d: "m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21", key: "1xmnt7" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const qu = Oe("ImagesIcon", [
  ["path", { d: "M18 22H4a2 2 0 0 1-2-2V6", key: "pblm9e" }],
  [
    "path",
    { d: "m22 13-1.296-1.296a2.41 2.41 0 0 0-3.408 0L11 18", key: "nf6bnh" },
  ],
  ["circle", { cx: "12", cy: "8", r: "2", key: "1822b1" }],
  [
    "rect",
    { width: "16", height: "16", x: "6", y: "2", rx: "2", key: "12espp" },
  ],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const zu = Oe("InboxIcon", [
  ["polyline", { points: "22 12 16 12 14 15 10 15 8 12 2 12", key: "o97t9d" }],
  [
    "path",
    {
      d: "M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z",
      key: "oot6mr",
    },
  ],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Tl = Oe("LeafIcon", [
  [
    "path",
    {
      d: "M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z",
      key: "nnexq3",
    },
  ],
  [
    "path",
    { d: "M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12", key: "mt58a7" },
  ],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Gu = Oe("LibraryIcon", [
  ["path", { d: "m16 6 4 14", key: "ji33uf" }],
  ["path", { d: "M12 6v14", key: "1n7gus" }],
  ["path", { d: "M8 8v12", key: "1gg7y9" }],
  ["path", { d: "M4 4v16", key: "6qkkli" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Ju = Oe("MessageCircleIcon", [
  ["path", { d: "M7.9 20A9 9 0 1 0 4 16.1L2 22Z", key: "vv11sd" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Yu = Oe("PanelRightCloseIcon", [
  [
    "rect",
    { width: "18", height: "18", x: "3", y: "3", rx: "2", key: "afitv7" },
  ],
  ["path", { d: "M15 3v18", key: "14nvp0" }],
  ["path", { d: "m8 9 3 3-3 3", key: "12hl5m" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Zu = Oe("PanelRightOpenIcon", [
  [
    "rect",
    { width: "18", height: "18", x: "3", y: "3", rx: "2", key: "afitv7" },
  ],
  ["path", { d: "M15 3v18", key: "14nvp0" }],
  ["path", { d: "m10 15-3-3 3-3", key: "1pgupc" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Qu = Oe("RotateCcwIcon", [
  [
    "path",
    { d: "M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8", key: "1357e3" },
  ],
  ["path", { d: "M3 3v5h5", key: "1xhq8a" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Xu = Oe("SendIcon", [
  [
    "path",
    {
      d: "M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z",
      key: "1ffxy3",
    },
  ],
  ["path", { d: "m21.854 2.147-10.94 10.939", key: "12cjpa" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const ec = Oe("SettingsIcon", [
  [
    "path",
    {
      d: "M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z",
      key: "1qme2f",
    },
  ],
  ["circle", { cx: "12", cy: "12", r: "3", key: "1v7zrd" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const tc = Oe("ShieldCheckIcon", [
  [
    "path",
    {
      d: "M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z",
      key: "oel41y",
    },
  ],
  ["path", { d: "m9 12 2 2 4-4", key: "dzmm74" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Co = Oe("SproutIcon", [
  ["path", { d: "M7 20h10", key: "e6iznv" }],
  ["path", { d: "M10 20c5.5-2.5.8-6.4 3-10", key: "161w41" }],
  [
    "path",
    {
      d: "M9.5 9.4c1.1.8 1.8 2.2 2.3 3.7-2 .4-3.5.4-4.8-.3-1.2-.6-2.3-1.9-3-4.2 2.8-.5 4.4 0 5.5.8z",
      key: "9gtqwd",
    },
  ],
  [
    "path",
    {
      d: "M14.1 6a7 7 0 0 0-1.1 4c1.9-.1 3.3-.6 4.3-1.4 1-1 1.6-2.3 1.7-4.6-2.7.1-4 1-4.9 2z",
      key: "bkxnd2",
    },
  ],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const sc = Oe("UsersIcon", [
  ["path", { d: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2", key: "1yyitq" }],
  ["circle", { cx: "9", cy: "7", r: "4", key: "nufk8" }],
  ["path", { d: "M22 21v-2a4 4 0 0 0-3-3.87", key: "kshegd" }],
  ["path", { d: "M16 3.13a4 4 0 0 1 0 7.75", key: "1da9ce" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const xo = Oe("VideoIcon", [
  [
    "path",
    {
      d: "m16 13 5.223 3.482a.5.5 0 0 0 .777-.416V7.87a.5.5 0 0 0-.752-.432L16 10.5",
      key: "ftymec",
    },
  ],
  [
    "rect",
    { x: "2", y: "6", width: "14", height: "12", rx: "2", key: "158x01" },
  ],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const So = Oe("Volume2Icon", [
  [
    "path",
    {
      d: "M11 4.702a.705.705 0 0 0-1.203-.498L6.413 7.587A1.4 1.4 0 0 1 5.416 8H3a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2.416a1.4 1.4 0 0 1 .997.413l3.383 3.384A.705.705 0 0 0 11 19.298z",
      key: "uqj9uw",
    },
  ],
  ["path", { d: "M16 9a5 5 0 0 1 0 6", key: "1q6k2b" }],
  ["path", { d: "M19.364 18.364a9 9 0 0 0 0-12.728", key: "ijwkga" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const nc = Oe("WandSparklesIcon", [
    [
      "path",
      {
        d: "m21.64 3.64-1.28-1.28a1.21 1.21 0 0 0-1.72 0L2.36 18.64a1.21 1.21 0 0 0 0 1.72l1.28 1.28a1.2 1.2 0 0 0 1.72 0L21.64 5.36a1.2 1.2 0 0 0 0-1.72",
        key: "ul74o6",
      },
    ],
    ["path", { d: "m14 7 3 3", key: "1r5n42" }],
    ["path", { d: "M5 6v4", key: "ilb8ba" }],
    ["path", { d: "M19 14v4", key: "blhpug" }],
    ["path", { d: "M10 2v2", key: "7u0qdc" }],
    ["path", { d: "M7 8H3", key: "zfb6yr" }],
    ["path", { d: "M21 16h-4", key: "1cnmox" }],
    ["path", { d: "M11 3H9", key: "1obp7u" }],
  ]),
  lc = [
    {
      name: "林晚晴",
      initial: "晴",
      subtitle: "把平凡的日子，过成喜欢的样子。",
      job: "独立插画师",
      mood: "平静 · 有一点期待",
      location: "家里的工作室",
      outfit: "奶油色针织衫",
      color: "#e4ede4",
      activity: "画一张秋日速写",
      world: "临海小城 · 秋天 · 晴，22°C",
      bio: "26 岁，独立插画师。喜欢植物、旧书和窗边的阳光，习惯用画笔收集生活里细小的温柔。",
      friend: "许知夏",
      hobby: "画画 / 逛书店 / 手冲咖啡",
    },
    {
      name: "陈屿",
      initial: "屿",
      subtitle: "慢一点，也能抵达想去的地方。",
      job: "建筑设计师",
      mood: "专注 · 轻松",
      location: "河岸咖啡馆",
      outfit: "深蓝色衬衫",
      color: "#e0e8ef",
      activity: "整理建筑手稿",
      world: "江边城市 · 秋天 · 多云，20°C",
      bio: "28 岁，建筑设计师。喜欢城市散步和胶片摄影，认真观察每一栋房子背后的故事。",
      friend: "周远",
      hobby: "建筑 / 摄影 / 徒步",
    },
    {
      name: "苏念安",
      initial: "安",
      subtitle: "今天也想发现一点新鲜的事。",
      job: "花店店主",
      mood: "愉快 · 充满灵感",
      location: "街角花店",
      outfit: "浅绿色连衣裙",
      color: "#eee5df",
      activity: "制作秋日花束",
      world: "南方小镇 · 秋天 · 晴，24°C",
      bio: "24 岁，经营一家小小的花店。热爱鲜花、烘焙和音乐，相信每一天都值得被好好记住。",
      friend: "叶小满",
      hobby: "花艺 / 烘焙 / 音乐",
    },
  ].map((e, t) => ({
    ...e,
    id: t,
    messages: [
      { text: "今天过得怎么样？", user: !0, time: "14:20" },
      {
        text: `刚刚在${e.location}${e.activity}，窗外的光正好落在桌上。突然觉得，日子这样慢慢过也很好。`,
        time: "14:21",
      },
      { text: "听起来很舒服，有什么想和我分享的吗？", user: !0, time: "14:22" },
      {
        text:
          t === 0
            ? `有呀。今天路过一家旧书店，发现了一本很喜欢的画集。里面夹着一片银杏叶，不知道是哪位读者留下的。

我想，等手上的画完成了，也去捡一片秋天吧。`
            : `今天${e.activity}的时候，有了一些新灵感。等忙完了，想和你一起出去走走。`,
        time: "14:23",
      },
    ],
    posts: [
      {
        text:
          t === 0
            ? `午后的光，和一杯刚刚好的咖啡。
今天也有在认真收集生活里的小美好。`
            : `${e.activity}的一天。愿每一份热爱，都能在日常里生根。`,
        time: "今天 13:30",
        likes: 12 + t,
        liked: !1,
      },
    ],
    schedule: [
      { time: "09:00", text: "晨间散步与早餐" },
      { time: "14:00", text: e.activity },
      { time: "16:00", text: "出门走走，买一束花" },
      { time: "18:30", text: "做一顿喜欢的晚餐" },
      { time: "21:00", text: "阅读与睡前整理" },
    ],
    diary: [
      {
        date: "10月22日 · 星期四",
        text: `今天在${e.location}度过了安静的午后。${e.activity}让我重新找到了专注的感觉。希望明天也能留一点时间给自己。`,
      },
    ],
    wardrobe: [e.outfit, "白色棉麻衬衫", "燕麦色风衣", "休闲牛仔外套"],
    memories: [
      "你说过，喜欢秋天和下雨的声音。",
      "我们约好，下次一起去看看海。",
    ],
    goals: [
      {
        text: ["完成秋日速写集", "完成河岸住宅手稿", "设计一组秋日花束"][t],
        done: !1,
        progress: [45, 65, 30][t],
      },
      { text: "每周留一天给自己", done: !0, progress: 100 },
    ],
    permissions: { post: !0, diary: !0, schedule: !1 },
  })),
  fe = (e, t, s = "text", n, l) => ({
    label: e,
    value: t,
    type: s,
    min: n,
    max: l,
  }),
  ks = (e, t, s) => ({ label: e, value: t, type: "select", options: s }),
  oc = ["思考与通用分析", "聊天回复", "看图理解", "记忆检索向量"],
  As = [
    "聊天理解",
    "聊天回复",
    "人格接管判断",
    "人格接管回复",
    "定时醒来",
    "反思",
    "每日回顾",
    "日程生成",
    "日程重排",
    "日记素材",
    "对话分段",
    "对话摘要",
    "每日对话记忆",
    "记忆检索判断",
    "目标评估",
    "目标规划",
    "虚拟活动结果",
    "图片提示词整理",
    "出图质量验收",
    "角色初始化",
    "人设编译",
    "看图理解",
    "记忆向量化",
    "日记生成",
    "记忆提取",
    "动态生成",
  ],
  To = [
    ...As.slice(0, 23).filter(
      (e) => !["聊天回复", "看图理解", "记忆向量化"].includes(e),
    ),
    "穿搭准备",
    "动态（朋友圈）",
    "世界 / NPC 相遇",
  ],
  hn = ["聊天照片", "动态配图", "物品照", "换装照", "形象定妆 / 设定图"],
  ic = [
    "超时",
    "被取消/中断",
    "服务连不上/报错",
    "没配好模型",
    "内容太长超预算",
    "返回格式不对",
    "结果不确定",
    "角色已暂停/停用而跳过",
    "工具执行失败",
  ],
  Mo = [
    { code: "provider_timeout", reason: "服务没有在等待上限内返回。" },
    { code: "request_cancelled", reason: "这次请求已被取消或中断。" },
    {
      code: "provider_unavailable",
      reason: "暂时无法连接模型服务，请检查服务地址。",
    },
    {
      code: "provider_role_unassigned",
      reason: "这个用途还没有绑定可用模型。",
    },
    {
      code: "prompt_budget_exceeded",
      reason: "发送的内容超过模型可用输入预算。",
    },
    {
      code: "structured_response_invalid",
      reason: "模型返回的内容不符合本次要求的格式。",
    },
    {
      code: "provider_result_ambiguous",
      reason: "服务返回的结果不明确，无法确认是否完成。",
    },
    {
      code: "provider_suppressed_fluctlight_paused",
      reason: "角色已暂停，这次调用已跳过。",
    },
    {
      code: "capability_prepare_failed",
      reason: "准备工具时失败，本次任务未能完成。",
    },
  ],
  Yi = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"],
  Eo = G([
    {
      id: "primary",
      name: "primary",
      protocol: "openai-compatible",
      url: "http://localhost:1234/v1",
      secret: "",
      clearSecret: !1,
      models: ["Qwen", "Cydonia", "Vision", "Embedding"],
      status: "就绪",
    },
  ]),
  Ao = G(
    oc.map((e, t) => ({
      name: e,
      role: [
        "generic_llm",
        "conversation_reply",
        "vision_understanding",
        "embedding",
      ][t],
      endpoint: "primary",
      model: ["Qwen", "Cydonia", "Vision", "Embedding"][t],
      tokens: 4096,
      timeout: 60,
      window: 32768,
    })),
  ),
  Io = G({
    enabled: !0,
    start: "23:00",
    end: "08:00",
    days: [...Yi],
    behavior: "暂停主动联系",
    character: "全部角色",
  }),
  Oo = G([
    {
      title: "图片（ComfyUI）",
      fields: [
        fe("服务地址", "http://localhost:8188", "url"),
        fe("图片生成等待上限（秒）", 1200, "number", 30, 1200),
        fe("图片工作流", '{"prompt":"{{prompt}}","seed":"{{seed}}"}', "json"),
        fe("视觉身份工作流", "{}", "json"),
        ...["人物", "场景", "动物", "物品", "肖像"].map((e) =>
          fe(e + "工作流", "{}", "json"),
        ),
      ],
    },
    {
      title: "聊天回复",
      fields: [
        ks("回复模式", "独立回复模型（Fact-only）", [
          "独立回复模型（Fact-only）",
          "旧模式",
        ]),
        ks("表达版本", "v5", ["v5", "v4"]),
        fe("温度", 0.8, "number", 0, 2),
        fe("Top P", 0.9, "number", 0.01, 1),
        fe("单条回复长度上限", 1024, "number", 1, 32768),
        fe("回复可读上下文", 8192, "number", 1024, 131072),
        fe("回复总超时（秒）", 300, "number", 1, 3600),
        fe("同时生成回复数", 1, "number", 1, 8),
        fe("出错自动重试次数", 1, "number", 0, 2),
        fe("媒体动作回交上限", 2, "number", 1, 2),
        ks("回复模型失败时", "直接报错", ["直接报错", "改用思考模型兜底"]),
      ],
    },
    { title: "看图理解", fields: [fe("启用看图理解", !1, "checkbox")] },
    {
      title: "角色自主与主动联系",
      fields: [
        ks("自主模式", "允许自主行动", ["允许自主行动", "暂停"]),
        ...["主动发消息", "发动态", "请求新能力", "请求生成图片"].map((e) =>
          fe("允许" + e, e !== "请求生成图片", "checkbox"),
        ),
        fe("剩余行动额度", 100, "number", 0),
        fe("重复话题冷却（秒）", 300, "number", 300, 604800),
        fe("冷却到", "", "datetime-local"),
        fe("同时进行的自主行动数", 1, "number", 1),
        fe("暂停主动联系", !1, "checkbox"),
        fe("需要我确认的行动", "请求生成图片"),
      ],
    },
    {
      title: "作息与后台节奏",
      fields: [
        fe("定时醒来（固定间隔 600 秒）", !0, "checkbox"),
        fe("对话摘要间隔（秒）", 300, "number", 300, 600),
        fe("摘要长度（字）", 2048, "number", 512, 4096),
        fe("目标重试次数", 5, "number", 1, 20),
        fe("重试起始间隔（秒）", 60, "number", 30),
        fe("重试间隔上限（秒）", 3600, "number", 60),
      ],
    },
    {
      title: "资源与预算",
      fields: [
        fe("思考模型同时请求数", 1, "number", 1, 8),
        fe("向量模型同时请求数", 1, "number", 1, 8),
      ],
    },
    {
      title: "通知",
      fields: ["新消息", "目标进展", "媒体完成"].map((e) =>
        fe(e, !0, "checkbox"),
      ),
    },
    {
      title: "高级",
      fields: [
        fe("启用 Kev", !1, "checkbox"),
        fe("Kev 地址", "http://127.0.0.1:8010/v1/systemone", "url"),
        fe("Kev 凭据", "", "password"),
        fe("清除 Kev 凭据", !1, "checkbox"),
        fe("请求超时（ms）", 1500, "number", 50, 6e4),
        fe("阶段总预算（ms）", 3e3, "number", 50, 12e4),
        fe("媒体方案补充", !0, "checkbox"),
        ...[
          "自动唤醒",
          "自动反思",
          "目标完成检查",
          "目标规划",
          "工具预披露",
          "人格切换",
          "可选上下文",
          "媒体入镜范围",
          "媒体拍摄方式",
          "媒体视图构图",
        ].map((e) => fe(e, !0, "checkbox")),
        ks("策略", "直接采用", ["直接采用", "加置信门槛"]),
        fe("最低概率", 0.7, "number", 0, 1),
        fe("前两名差距", 0.1, "number", 0, 1),
        fe("最多连续延后", 3, "number", 1, 100),
        fe("延后秒数", 60, "number", 1),
        fe("记录保留（天）", 7, "number", 1, 90),
      ],
    },
  ]),
  Vo = [
    {
      title: "逻辑运行",
      rows: [
        "思考 #a28 · 失败 · 工具阶段：能力准备失败",
        "思考 #a27 · 完成 · 包含 3 次模型调用",
      ],
    },
    {
      title: "后台任务",
      rows: [
        "目标规划 · 重试耗尽 · 10:04",
        "对话摘要 · 完成 · 10:03",
        "图片生成 · 完成 · 10:01",
      ],
    },
    {
      title: "生命周期",
      rows: [
        "10:04 因安静时段跳过主动联系",
        "10:02 反思排队",
        "10:00 唤醒完成，无需行动",
      ],
    },
    {
      title: "回合诊断",
      rows: ["回合 #42 · 已完成", "回合 #41 · 工具执行失败"],
    },
    {
      title: "Kev 决策",
      rows: ["自动唤醒 · 延后 60 秒 · 概率 0.82", "工具预披露 · 选择记忆检索"],
    },
    {
      title: "主动触达投递",
      rows: ["10:04 · 未投递：处于安静时段", "昨天 18:30 · 已送达"],
    },
    { title: "待处理事项 / 重试耗尽", rows: ["目标规划 · 等待手动重试"] },
    {
      title: "技术详情",
      rows: [
        "执行器关联 · workflow-demo-42",
        "系统事件 · 思考处理已结束",
        "诊断保留：30 天 / 每类 10000 条（固定）",
      ],
    },
    { title: "安全拦截", rows: ["今天未发生永久伤害拦截"] },
  ],
  rc = {
    聊天理解: {
      prompt:
        "分析用户消息：“明天十点一起去旧书店吧，记得带上次那本画集。”识别意图、日期、地点及待确认事项，只输出结构化事实。当前日期：2026-10-23。",
      response:
        "意图：发起约定；时间：10月24日 10:00；地点：旧书店；需携带：上次的画集。用户尚未指定碰面位置，需要在回复中确认。",
    },
    聊天回复: {
      prompt:
        "用户说：“今天改了三遍稿子，还是不太满意，有点累。”请以角色口吻先回应疲惫，再提出一个轻松的陪伴选择。不要替用户判断作品好坏，也不要连续追问。",
      response:
        "改了三遍，脑袋一定有点累了。先把稿子放一会儿吧，要不要泡杯热茶？我在这儿，等你想聊的时候再慢慢说。",
    },
    人格接管判断: {
      prompt:
        "判断当前对话是否需要切换至沉稳表达模式。用户：“先别开玩笑，我想认真说说这件事。”当前角色状态平静；只分析表达风格，不改变角色身份。输出判断与依据。",
      response:
        "切换表达模式：是；置信度：0.93。依据：用户明确要求认真倾听。下一条回复减少玩笑和感叹，保留原有角色身份与记忆。",
    },
    人格接管回复: {
      prompt:
        "已决定使用沉稳表达模式。用户正在解释与同事发生的误会，希望先被听完。生成一句表示认真倾听的回复，暂不提供解决方案。",
      response:
        "好，我认真听着。你可以从最想说的地方开始，我会等你说完，再一起想接下来怎么办。",
    },
    定时醒来: {
      prompt:
        "检查14:10的自动唤醒。下一项日程为14:30整理手稿，上次主动联系在13:55，用户没有未回复的问题，当前不在安静时段。判断是否需要联系或执行任务。",
      response:
        "本次无需主动联系；距上次联系仅15分钟，没有新事项。保留14:30的手稿整理安排，下次检查时间14:25。",
    },
    反思: {
      prompt:
        "回看午后的邀请：用户说今天想独处，角色已答应不再追问。请提炼相处中的改进点，区分一次性的情绪与长期偏好，不推断未说出的原因。",
      response:
        "今天应给用户留出独处空间，晚间不追加邀约。一次想独处不足以推断长期偏好；后续等用户主动分享，再自然回应。",
    },
    每日回顾: {
      prompt:
        "整理10月23日的生活记录：09:00去书店，14:30完成两页手稿，17:00沿河散步；用户确认了明天的旧书店约定。按完成事项、相处进展、明日待办总结。",
      response:
        "今天完成书店采购、两页手稿和河岸散步；相处进展是确认了明早见面。明日待办：09:40带画集出门，10:00在旧书店门口碰面。",
    },
    日程生成: {
      prompt:
        "为2026-10-24生成日程。固定约定：10:00旧书店见面、16:30河边散步。待办：整理手稿90分钟，采购食材30分钟；保留午餐、休息及20分钟通勤。输出不冲突的时间安排。",
      response:
        "09:40 出发去旧书店；10:00 与你见面；12:00 午餐与休息；14:00–15:30 整理手稿；15:40 采购食材；16:30 河边散步。固定约定均已保留，通勤时间充足。",
    },
    日程重排: {
      prompt:
        "17:00开始下雨，原计划17:10户外速写，18:30与朋友晚餐，20:00阅读。把速写改为室内活动，晚餐与阅读时间不可移动。说明调整的具体原因。",
      response:
        "将17:10户外速写调整为在家整理速写草稿，避免雨中外出；18:30晚餐和20:00阅读保持原定时间，没有新增时间冲突。",
    },
    日记素材: {
      prompt:
        "从今日事件中提取日记素材：旧书店买到散文集；书页夹着银杏叶；下午在窗边画了两页草稿；对话中约好明早十点见面。只列已发生的细节及角色明确表达的感受。",
      response:
        "可写素材：旧书纸张的气味、夹在书页里的银杏叶、窗边完成的两页草稿。已表达的感受：找到书时很开心，期待明早见面。约定本身已确认，见面尚未发生。",
    },
    对话分段: {
      prompt:
        "对消息1–8分段：1–3讨论修改手稿；4–5闲聊午餐；6–8确认明天10:00在旧书店门口见面。为每段给出消息范围、主题和是否已结束。",
      response:
        "消息1–3：创作遇到的困难，已结束；消息4–5：午餐闲聊，已结束；消息6–8：旧书店见面约定，已确认。共3段，保留原始消息顺序。",
    },
    对话摘要: {
      prompt:
        "压缩本轮对话以供后续聊天使用：用户今天在改稿，想先休息；双方确认明早十点旧书店门口见；角色答应带画集。保留具体承诺，省略问候与重复确认。",
      response:
        "用户今天改稿疲惫，准备先休息。双方约好10月24日10:00在旧书店门口见面；角色需带上次的画集。当前没有未回答的问题。",
    },
    每日对话记忆: {
      prompt:
        "合并今日3段对话中的可记忆信息。用户再次提到喜欢雨声；新增明早旧书店约定；“今天想独处”仅适用于当日。与已有“喜欢秋天”记录去重，区分长期偏好和短期事件。",
      response:
        "长期偏好补充：喜欢听雨声，来源为今日明确陈述。新增事件：10月24日10:00旧书店约定。今天想独处仅保留为当日上下文，不写成长期性格。",
    },
    记忆检索判断: {
      prompt:
        "用户问：“你还记得我上次想找的那本书吗？”判断是否需要检索记忆；当前窗口只含今天的问候，没有书名。给出检索主题和时间线索，不编造书名。",
      response:
        "需要检索；查询：用户上次提到的想找的书、旧书店、书名；时间范围：最近两周。检索到可核实的记录后再回答，目前不能确定书名。",
    },
    目标评估: {
      prompt:
        "评估“完成秋日短篇初稿”。计划共10页，已有6页可读草稿，本周新写2页；结尾仍是提纲。依据产物评估进度，不能因一次聊天就判定完成。",
      response:
        "目标进度60%，尚未完成。已有6页草稿可核对，剩余4页及结尾待写；建议下次检查以新增两页正文为依据。",
    },
    目标规划: {
      prompt:
        "把“周日前完成剩余4页短篇初稿”拆成行动。周六14:00–15:30可写作，周日09:00–10:30可写作；不占用已确认的书店见面和散步时间。",
      response:
        "周六14:00梳理结尾并写第7–8页；周日09:00写第9–10页；周日10:00通读并标记修改点。每次写作中途休息10分钟，不移动既有约定。",
    },
    虚拟活动结果: {
      prompt:
        "模拟角色在旧书店停留30分钟的活动结果。已知店员陈思远、文学书架及一册旧画集。输出有边界的生活事件，不替用户作出决定，不产生未授权的共同约定。",
      response:
        "角色在文学书架找到一本散文集，与陈思远聊了几句纸张装帧，随后买书离开。没有新增共同约定，回家后可把书中夹着银杏叶的小发现分享给用户。",
    },
    图片提示词整理: {
      prompt:
        "把“想看看你窗边画画的样子”整理成聊天照片提示词。沿用已保存的角色外观和米色针织衫，环境为午后书桌；描述镜头、动作与光线，不添加文字或水印。",
      response:
        "单人半身生活照，米色针织衫，坐在窗边木桌前画速写，手握铅笔，桌上摊开草稿本，左侧柔和自然光，平视镜头，背景整洁，保留角色参考外观。",
    },
    出图质量验收: {
      prompt:
        "验收生成图片comfy-demo-65。要求：单人、米色针织衫、窗边木桌、手持铅笔。检测结果：单人1、可见双手结构正常、无文字水印、角色参考一致性0.94。给出通过与否和依据。",
      response:
        "验收通过。人物数量、针织衫和窗边画画场景符合要求；双手结构正常，无可见文字水印；参考外观一致性0.94，无需重新生成。",
    },
    角色初始化: {
      prompt:
        "根据新角色资料建立初始状态：插画师，居住在河岸街区，喜欢旧书与散步，今天下午计划整理草稿。初始化位置、活动与心情，不生成与用户之间尚未发生的经历。",
      response:
        "初始位置：家中书桌；当前活动：整理下午的草稿；心情：平静而有期待。日程写入午后绘画与傍晚散步，共同经历保持为空。",
    },
    人设编译: {
      prompt:
        "将设定“说话温和、喜欢观察小事、先听完再建议、不会替对方决定”编译为回复约束。保留兴趣与表达习惯，输出可直接用于聊天的规则及一句示例。",
      response:
        "表达规则：用简洁温和的句子，先回应对方感受，再询问是否需要建议；分享生活细节时只使用已知事实。示例：“我听着呢，你想先说说今天发生的事吗？”",
    },
    看图理解: {
      prompt:
        "描述用户上传的本地图片：画面含木桌、白色陶杯、摊开的书与窗外树影。分别报告可见物体、光线和不确定细节，不猜测人物身份或不可辨认的书名。",
      response:
        "画面里是一张靠窗木桌，桌上有白色陶杯和摊开的书，右上方能看到树影。自然光偏暖；书页文字不清晰，无法判断书名，画面中没有可见人物。",
    },
    记忆向量化: {
      prompt:
        "为记忆“用户喜欢秋天和雨声，愿意在下雨时留在窗边读书”生成检索向量。记录ID memory-20261023-03，使用1536维向量，保留原文与日期元数据，返回写入结果。",
      response:
        "memory-20261023-03已写入本地模拟索引；向量维度1536，已归一化。元数据：日期2026-10-23，标签偏好、雨声；相似检索可关联窗边阅读主题。",
    },
    日记生成: {
      prompt:
        "用角色第一人称写10月23日日记，120字左右。事实素材：旧书店的散文集、书页里的银杏叶、窗边画完两页草稿、明早十点的见面约定。用自然的叙事，不写尚未发生的见面。",
      response:
        "今天在旧书店找到一本散文集，翻开时，一片银杏叶从书页间落下来。回家后在窗边画了两页草稿，光慢慢移到桌角，才发现下午已经过去了。和你约好了明早十点见面，睡前记得把画集装进包里。有一点期待，也想把今晚留给安静的阅读。",
    },
    记忆提取: {
      prompt:
        "从用户原话提取长期记忆：“我一直喜欢秋天，下雨的时候会在窗边读书。”每条标明内容、来源、标签与重要度；不要把角色自己的偏好混入用户记忆。",
      response:
        "提取2条：①用户一直喜欢秋天，标签“偏好”，重要度4/5；②用户下雨时会在窗边读书，标签“阅读、日常”，重要度3/5。来源均为10月23日聊天中的用户明确陈述。",
    },
    动态生成: {
      prompt:
        "为角色生成一条生活动态，60字左右。素材：傍晚沿河散步，看见长椅旁的银杏叶，拍了一张河面倒影。语气轻松，正文无需话题标签，不提用户或私密对话。",
      response:
        "傍晚沿着河岸多走了一段，长椅旁落了一圈银杏叶。风一吹，河面把天空揉成碎金。拍下来的那一刻，觉得今天可以慢慢收尾了。",
    },
  };
function ac(e, t) {
  const s = rc[e];
  return {
    input: `用途：${e}
角色：${t}
${s.prompt}`,
    response: s.response,
  };
}
const uc = [
    {
      workflow: "人物照片 / portrait-photo-v2",
      prompt:
        "单人半身聊天照片，坐在家中窗边画速写，米色针织衫，手握铅笔看向镜头，画面亲近自然，柔和侧光，保留角色参考面容。",
      negativePrompt: "多个人物，畸形手指，面容不一致，模糊，水印，文字",
      width: 768,
      height: 1024,
      summary: "聊天照片已生成：窗边画画的半身近照，角色面容和手部检查通过。",
    },
    {
      workflow: "动态配图 / daily-post",
      prompt:
        "用于河岸散步动态的横幅风景，傍晚水面倒影与长椅旁的银杏叶，无人物，暖金色天空，留出自然呼吸感，不添加配文。",
      negativePrompt: "人物，字幕，水印，地平线倾斜，过曝",
      width: 1024,
      height: 768,
      summary: "动态配图已生成：傍晚河岸与银杏倒影横幅，可用于今日散步动态。",
    },
    {
      workflow: "物品静物 / still-life",
      prompt:
        "只拍木桌上的一束秋花与白色陶杯，正方形静物构图，花瓣和杯沿清晰，浅景深，自然窗光，不出现人物、手或多余器具。",
      negativePrompt: "人脸，手臂，多余花瓶，杯口变形，模糊，文字，水印",
      width: 1024,
      height: 1024,
      summary: "物品照已生成：秋花与白色陶杯静物，杯沿和花瓣细节完整。",
    },
    {
      workflow: "换装 / outfit-try-on",
      prompt:
        "全身正面换装预览，沿用角色参考面容与体态，穿燕麦风衣、白色衬衫、直筒长裤和帆布鞋，双脚完整入镜，素色背景，重点展示衣服层次及版型。",
      negativePrompt: "裁切脚部，多余肢体，衣服穿模，错色，面容漂移，水印",
      width: 768,
      height: 1152,
      summary:
        "换装照已生成：燕麦风衣与帆布鞋的全身搭配，衣服层次及鞋履展示完整。",
    },
    {
      workflow: "视觉身份 / character-sheet",
      prompt:
        "角色形象设定图，同一角色的正面、侧面与背面三视图，面部轮廓、发型、身高比例一致，日常基础服装，中性背景，均匀棚拍光，不添加标注文字。",
      negativePrompt:
        "三视图比例不一致，不同人物，遮挡脸部，透视扭曲，标注文字，水印",
      width: 1536,
      height: 1024,
      summary:
        "形象设定图已生成：正侧背三视图，面部、发型与体态比例一致，可作后续出图参考。",
    },
  ],
  cc = [...As.slice(0, 23), ...hn, ...As.slice(0, 9), ...As.slice(23), hn[1]],
  Zi = G(
    cc.map((e, t) => {
      const s = hn.indexOf(e),
        n = s >= 0,
        l = ["林晚晴", "陈屿", "苏念安"][t % 3],
        i = t < 9 || t === 24 ? "error" : "success",
        r = n ? uc[s] : void 0,
        a = r
          ? {
              type: e,
              workflow: r.workflow,
              prompt: `用途：${e}
角色参考：${l}
${r.prompt}`,
              negativePrompt: r.negativePrompt,
              params: {
                width: r.width,
                height: r.height,
                steps: 24,
                cfg: 6.5,
                seed: 42810 + t,
                sampler: "dpmpp_2m",
              },
              output: `${r.summary}
本地模拟 · comfy-demo-${42 + t} · ${r.width} × ${r.height} PNG`,
            }
          : void 0,
        u = a ? { input: a.prompt, response: a.output } : ac(e, l);
      return {
        id: -100 - t,
        label: e,
        kind: n ? "媒体调用" : "模型调用",
        character: l,
        start: t * 4,
        duration: n ? 18 + (t % 12) : 2 + (t % 7),
        status: i,
        ...u,
        reason:
          i === "success"
            ? ""
            : n
              ? "图片服务暂时无法连接，请确认 ComfyUI 已启动。"
              : Mo[t % 9].reason,
        category: n ? "服务连不上/报错" : ic[t % 9],
        model: n ? "ComfyUI" : "Qwen",
        tokens: n ? void 0 : 846 + t * 17,
        code: n ? "media_connection_failed" : Mo[t % 9].code,
        media: a,
        run() {},
      };
    }),
  ),
  At = ["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"],
  dc = [
    [
      "陈思远",
      "书店店员",
      "总能从旧书堆里找出适合朋友的读物。",
      "09:30 整理新到的文学书",
    ],
    [
      "赵明轩",
      "自由撰稿人",
      "习惯带着笔记本记录街坊的小故事。",
      "10:00 采访街区老住户",
    ],
    [
      "沈禾",
      "邻居",
      "阳台种满香草，常把多余的薄荷送给邻居。",
      "08:00 给阳台植物浇水",
    ],
    [
      "陆川",
      "项目同事",
      "做事有条理，午休时喜欢聊最近看的电影。",
      "14:00 核对项目进度",
    ],
    [
      "吴晓彤",
      "咖啡师",
      "正在练习拉花，也认真记着熟客的口味。",
      "11:00 试做秋季特调",
    ],
    [
      "顾言",
      "旧同学",
      "毕业后仍保持联系，喜欢组织小范围聚餐。",
      "18:30 和同学吃晚饭",
    ],
    [
      "郑雨欣",
      "服装店主",
      "擅长用简单单品搭配出舒服的日常穿着。",
      "10:30 布置橱窗新品",
    ],
    [
      "王嘉宁",
      "烘焙师",
      "每天清晨烤第一炉面包，周末尝试新配方。",
      "07:00 烘烤全麦面包",
    ],
    [
      "梁文博",
      "语文老师",
      "爱读散文，课后会带学生观察校园里的树。",
      "15:30 准备阅读分享课",
    ],
    [
      "孙悦",
      "图书管理员",
      "熟悉每排书架的位置，正在整理地方志。",
      "13:00 修补旧书封面",
    ],
    [
      "何志成",
      "体育教练",
      "说话爽朗，愿意陪刚开始跑步的人慢慢练。",
      "17:00 带领操场慢跑",
    ],
    [
      "徐静",
      "社区医生",
      "休息时喜欢做家常菜，关心朋友的作息。",
      "19:00 准备一周的便当",
    ],
    [
      "高亦凡",
      "摄影师",
      "偏爱雨后街景，随身带着一台旧相机。",
      "16:00 拍摄商店街街景",
    ],
    [
      "唐敏",
      "陶艺工作者",
      "喜欢手作杯盘，也享受等待窑炉冷却的时光。",
      "14:30 修整新做的陶杯",
    ],
    [
      "冯子涵",
      "研究生",
      "研究城市植物，散步时总会停下来辨认叶片。",
      "09:00 整理校园植物标本",
    ],
    [
      "蒋宁",
      "音乐老师",
      "喜欢民谣，偶尔为朋友弹唱刚学会的曲子。",
      "16:30 排练合唱伴奏",
    ],
    [
      "韩雪",
      "花艺师",
      "善于搭配当季枝叶，最近在学习干花制作。",
      "11:30 挑选花束配材",
    ],
    [
      "程浩",
      "维修师傅",
      "熟悉街区里的老房子，修好东西后很有成就感。",
      "15:00 检查家中的旧台灯",
    ],
  ],
  fc = [
    [
      "卧室",
      "休息与睡前阅读的私人房间",
      "柔和暖光，安稳放松",
      "双人床、床头灯、衣柜",
      "床靠内墙，窗边放阅读椅",
    ],
    [
      "浴室",
      "洗漱与沐浴的干湿分离空间",
      "水汽轻盈，清爽洁净",
      "洗手台、镜柜、淋浴隔断",
      "洗漱区临门，淋浴间位于里侧",
    ],
    [
      "客厅",
      "招待朋友和看电影的起居空间",
      "午后明亮，亲切随意",
      "布艺沙发、矮茶几、投影幕",
      "沙发面向投影墙，中间留出活动区",
    ],
    [
      "服装店",
      "出售当季服饰的街角小店",
      "轻快音乐，整洁时髦",
      "落地试衣镜、挂衣架、收银台",
      "新品陈列在入口，两侧通往试衣间",
    ],
    [
      "小吃店",
      "供应热汤面和手工点心的老店",
      "烟火热闹，汤香四溢",
      "长条餐桌、蒸笼、菜单灯箱",
      "门口点单，靠墙设置双人座",
    ],
    [
      "书店",
      "收藏文学与艺术书籍的独立书店",
      "纸墨清香，低声交谈",
      "实木书架、阅读长桌、推荐展台",
      "书架围出回形走道，阅读区临窗",
    ],
    [
      "教室",
      "供学生上课与小组讨论的学习空间",
      "晨光通透，专注有序",
      "课桌椅、黑板、公告栏",
      "课桌分列排列，后墙展示学生作品",
    ],
    [
      "图书馆",
      "提供借阅与安静自习的校园建筑",
      "肃静沉稳，翻页声细微",
      "分类书架、自习桌、借还书机",
      "中央检索区连接书库与自习区",
    ],
    [
      "操场",
      "进行跑步和球类训练的户外场地",
      "开阔有风，充满活力",
      "塑胶跑道、足球门、看台",
      "跑道环绕草坪，看台位于西侧",
    ],
  ],
  et = G(
    lc.map((e) => ({
      ...e,
      tags:
        e.id === 0
          ? ["温柔", "创作"]
          : e.id === 1
            ? ["沉稳", "城市"]
            : ["温柔", "自然"],
      posts: e.posts.map((t) => ({
        ...t,
        comments: [
          { name: e.friend, text: "这样的午后真好。", time: "今天 13:42" },
        ],
      })),
      garments: At.flatMap((t, s) =>
        [0, 1].map((n) => ({
          id: s * 2 + n + 1,
          name: [
            ["燕麦风衣", "牛仔外套"],
            [e.outfit, "白色棉麻衬衫"],
            ["棉质内衣", "轻薄内衣"],
            ["棉质内裤", "舒适内裤"],
            ["直筒长裤", "休闲短裤"],
            ["白色棉袜", "灰色短袜"],
            ["帆布鞋", "皮质便鞋"],
            ["银色手表", "帆布包"],
          ][s][n],
          type: t,
          seasons: n ? ["春", "夏"] : ["春", "秋", "冬"],
        })),
      ),
      outfitTags: ["日常", "散步", "工作"],
      outfits: [
        {
          id: 1,
          name: "秋日散步",
          tag: "散步",
          items: Object.fromEntries(At.map((t, s) => [t, [s * 2 + 1]])),
        },
      ],
      wearing: Object.fromEntries(At.map((t, s) => [t, [s * 2 + 1]])),
      genre: "现代都市",
      sites: ["家", "商店街", "学校"].map((t, s) => ({
        name: t,
        places: fc
          .slice(s * 3, s * 3 + 3)
          .map(([n, l, i, r, a], u) => ({
            id: s * 10 + u,
            name: n,
            alias: "",
            floor: "1F",
            description: l,
            atmosphere: i,
            furniture: r,
            layout: a,
            custom: !1,
          })),
      })),
      people: dc.map(([t, s, n, l], i) => ({
        id: i,
        name: i === 0 ? e.friend : t,
        identity: s,
        relation: ["朋友", "熟悉", "初识"][i % 3],
        location: [
          "商店街",
          "商店街",
          "家",
          "商店街",
          "商店街",
          "家",
          "商店街",
          "商店街",
          "学校",
          "学校",
          "学校",
          "家",
          "商店街",
          "家",
          "学校",
          "学校",
          "商店街",
          "家",
        ][i],
        bio: n,
        schedule: l,
        tag: ["日常", "工作", "旧友"][Math.floor(i / 2) % 3],
        type: i === 1 ? "用户" : "世界中的人",
        status: i % 4 === 0 ? "暂未联系" : "活跃",
      })),
    })),
  ),
  cs = G(0),
  W = _e(() => et.value[cs.value]);
function Rt(e, t) {
  return Object.fromEntries(
    At.map((s) => [
      s,
      (t[s] || []).filter((n) =>
        e.garments.some((l) => l.id === n && l.type === s),
      ),
    ]),
  );
}
function pc(e, t) {
  ((e.garments = e.garments.filter((s) => s.id !== t)),
    (e.wearing = Rt(e, e.wearing)),
    e.outfits.forEach((s) => (s.items = Rt(e, s.items))));
}
const Is = G(null);
let Qi,
  il = 0;
function mc() {
  (il++, rl());
}
function rl() {
  (clearTimeout(Qi), (Is.value = null));
}
function Ml(e, t) {
  (rl(), (Is.value = { text: e, link: t }), (Qi = setTimeout(rl, 2500)));
}
const ns = G(!1),
  vs = G([]);
let vc = 0;
function Je(e, t, s, n = t, l) {
  const i = il,
    r = {
      id: ++vc,
      label: e,
      input: t,
      status: "pending",
      character: W.value.name,
      start: Math.round((Date.now() % 12e4) / 1e3),
      duration: 1,
      response: "请求已完成。",
      run: () => {},
    };
  vs.value.push(r);
  const a = vs.value.at(-1);
  let u = !1;
  return (
    (a.run = () => {
      u ||
        ((u = !0),
        (a.status = "pending"),
        (a.reason = ""),
        setTimeout(() => {
          (ns.value || t.includes("失败")
            ? ((a.status = "error"),
              (a.reason = t.includes("失败")
                ? "输入包含“失败”，本次操作未完成，请修改输入后重新提交。"
                : "模拟失败已开启，本次操作未完成，关闭后可重试。"))
            : (s(),
              (a.status = "success"),
              i === il &&
                Ml(
                  l || "已保存「" + n.slice(0, 32) + "」",
                  e === "衣柜" ? "方案库" : void 0,
                )),
            (u = !1));
        }, 800));
    }),
    a.run(),
    a
  );
}
function hc(e, t) {
  return (
    (e.wearing[t] || [])
      .map((s) => {
        var n;
        return (n = e.garments.find((l) => l.id === s)) == null
          ? void 0
          : n.name;
      })
      .filter(Boolean)
      .join("、") || "未穿戴"
  );
}
const gc = [
    [
      ["图片", "日常", "窗边的午后"],
      ["图片", "日常", "一杯热茶"],
      ["图片", "日常", "周末早餐"],
      ["图片", "风景", "河岸的银杏"],
      ["图片", "创作", "秋日速写"],
      ["图片", "创作", "新画集封面"],
      ["语音", "日常", "晚安小问候"],
      ["语音", "风景", "窗外的雨声"],
      ["语音", "创作", "短篇朗读片段"],
      ["视频", "日常", "周末生活片段"],
      ["视频", "风景", "晚风里的河面"],
      ["视频", "创作", "水彩上色过程"],
    ],
    [
      ["图片", "日常", "晨间咖啡"],
      ["图片", "风景", "城市的天际线"],
      ["图片", "风景", "雨后小路"],
      ["图片", "创作", "手作木架草图"],
      ["语音", "日常", "下班路上的问候"],
      ["语音", "日常", "今晚的厨房闲聊"],
      ["语音", "风景", "街巷雨声采样"],
      ["语音", "创作", "吉他练习随录"],
      ["语音", "创作", "给短片的旁白"],
      ["视频", "日常", "煮一碗热汤面"],
      ["视频", "风景", "黄昏城市漫步"],
      ["视频", "创作", "木架制作记录"],
    ],
    [
      ["图片", "日常", "书页之间"],
      ["图片", "风景", "晨光下的山坡"],
      ["图片", "创作", "植物观察手账"],
      ["语音", "日常", "午后的轻声问候"],
      ["语音", "风景", "林间鸟鸣"],
      ["语音", "创作", "一首新写的小诗"],
      ["视频", "日常", "阳台浇花日常"],
      ["视频", "日常", "周末烘焙时光"],
      ["视频", "风景", "沿着溪流散步"],
      ["视频", "创作", "压花书签制作"],
      ["视频", "创作", "手账拼贴过程"],
      ["视频", "创作", "秋叶拓印小实验"],
    ],
  ],
  Po = et.value.flatMap((e) =>
    gc[e.id].map(([t, s, n], l) => ({
      id: e.id * 100 + l,
      character: e.name,
      type: t,
      tag: s,
      title: n,
      time: "10月" + (23 - l) + "日",
    })),
  );
et.value.forEach((e) => {
  ((e.memories = [
    "偏好 · 从聊天里记住你喜欢秋天和雨声。",
    "相处变化 · 从日常问候到主动分享创作，越来越愿意表达自己的感受。",
  ]),
    e.posts.push(
      ...[
        "散步时发现街角的树叶变黄了，想把这份秋意分享给你。",
        "今天试着放慢脚步，认真做了一顿晚饭。",
      ].map((t, s) => ({
        text: t,
        time: s ? "昨天 18:30" : "今天 10:20",
        likes: s + 3,
        liked: !1,
        comments: [
          { name: e.friend, text: "下次一起吧。", time: "今天 13:50" },
        ],
      })),
    ));
});
const El = G("为她搭配"),
  ds = (e) =>
    _e(() =>
      vs.value.some((t) => e.includes(t.label) && t.status === "pending"),
    ),
  Xi = G("配置中心"),
  al = G(null),
  tn = G(!1),
  gn = "2026-10-23",
  yc = [
    {
      title: "给秋天留个位置",
      weather: "晴",
      mood: "轻快",
      text: "窗台上的绿萝又长出一片新叶。给它浇水时，忽然觉得日子也在悄悄往前走。",
    },
    {
      title: "一碗热汤的晚上",
      weather: "阴",
      mood: "满足",
      text: "回家的路上有一点冷。煮了番茄面，卧了一个鸡蛋。热气漫上来的时候，觉得今天已经很好了。",
    },
    {
      title: "雨声里的半页书",
      weather: "小雨",
      mood: "安静",
      text: "雨从午后一直下到天黑。原本想出门，最后在窗边读完了半本书。夹在书里的车票落到了地上。那趟旅行的风，好像又吹回来了一点。",
    },
    {
      title: "绕远路回家",
      weather: "多云",
      mood: "好奇",
      text: "今天走了一条没走过的小巷。墙角开着不知名的小花。路尽头是一家修表店，门口挂着很旧的钟。老板说它已经不准了，只是舍不得摘。买了一杯豆浆，慢慢走回家。",
    },
    {
      title: "不着急的一天",
      weather: "微风",
      mood: "释然",
      text: "早上醒得很早，却没有急着起床。听见楼下有人搬花盆。早餐吃了烤吐司，边角有一点焦。上午把桌上积着的纸张整理好了。有些没完成的念头，决定先放一放。傍晚的天空很淡，像一张还没画满的纸。",
    },
    {
      title: "街角传来的歌",
      weather: "阵雨",
      mood: "雀跃",
      text: "出门时忘了带伞。在屋檐下躲雨，听见旁边小店放着熟悉的歌。店主递来一张纸巾。我们聊起很久以前听过的演唱会。雨停得很快。地面映着灯光，像铺了一条小河。回家之后，找出那张旧唱片又听了一遍。",
    },
    {
      title: "把平凡认真收好",
      weather: "晴转多云",
      mood: "温柔",
      text: "上午去了市场。挑了三个番茄和一小把青菜。卖菜的阿姨多送了一根葱。回家路上碰见邻居的猫，它还是不肯让人摸。午后给朋友回了一封很长的信。写到最后，阳光已经离开了书桌。晚上做饭时想起那根葱。被人惦记的一点小事，也会让一整天变得柔软。",
    },
    {
      title: "晚一点也没关系",
      weather: "薄雾",
      mood: "期待",
      text: "今天的事情没有全做完，但也不想责怪自己。收工后沿着河岸走了一小段，风把疲倦吹散了。明天还有新的光，会慢慢来。",
    },
  ],
  er = G(
    et.value.map((e) => ({
      appointments: [
        { date: "2026-10-24", time: "10:00", text: "一起去旧书店" },
        { date: "2026-10-24", time: "16:30", text: "河边散步" },
        { date: "2026-10-26", time: "18:00", text: "分享一顿晚餐" },
      ],
      diaries: yc.map((t, s) => ({
        ...t,
        id: e.id * 100 + s,
        date: "2026-10-" + String(2 + s * 3).padStart(2, "0"),
        time: ["20:15", "21:30", "22:05"][s % 3],
        text: s === 0 ? `在${e.location}，${t.text}` : t.text,
      })),
      memories: Array.from({ length: 10 }, (t, s) => ({
        id: e.id * 100 + s,
        text: [
          "记得你喜欢秋天和雨声。",
          "和你在旧书店发现夹着银杏叶的画集。",
          "想找个晴天一起去看海。",
        ][s % 3],
        date: "2026-10-" + String(s + 3).padStart(2, "0"),
        source: ["聊天中记下", "共同经历", "日常反思"][s % 3],
        importance: (s % 5) + 1,
        category: ["长期记忆", "发生过的事情", "最近在想"][s % 3],
        tags: [["偏好", "日常", "约定"][s % 3]],
      })),
    })),
  );
function tr(e, t) {
  const s = er.value[e].diaries,
    n = s.find((l) => l.date === gn);
  if (n) {
    ((n.text = t), (n.time = "21:40"));
    return;
  }
  s.push({
    id: Date.now(),
    date: gn,
    time: "21:40",
    title: "今天的小小回响",
    weather: "晴",
    mood: "平静",
    text: t,
  });
}
const Ut = G(
    et.value.map((e) => ({
      id: e.id,
      name: e.world.split(" · ")[0],
      genre: e.genre,
      residents: [e.id],
      sites: JSON.parse(JSON.stringify(e.sites)),
    })),
  ),
  sr = nt({
    __name: "CurrentOutfit",
    props: { compact: { type: Boolean } },
    setup(e) {
      function t() {
        ((El.value = "为她搭配"), (location.hash = encodeURIComponent("衣柜")));
      }
      return (s, n) => (
        p(),
        g(
          "section",
          {
            class: ge(["outfit-block", { "outfit-compact": e.compact }]),
            "data-testid": "current-outfit",
          },
          [
            o("div", { class: "row" }, [
              n[0] || (n[0] = o("b", null, "今日穿着", -1)),
              n[1] || (n[1] = o("span", { class: "spacer" }, null, -1)),
              o("button", { class: "text-button", onClick: t }, "去编辑 ↗"),
            ]),
            o(
              "div",
              { class: ge(e.compact ? "outfit-tags" : "outfit-lines") },
              [
                (p(!0),
                g(
                  O,
                  null,
                  oe(
                    M(At),
                    (l) => (
                      p(),
                      g("div", { key: l }, [
                        o("small", null, v(l), 1),
                        o("span", null, v(M(hc)(M(W), l)), 1),
                      ])
                    ),
                  ),
                  128,
                )),
              ],
              2,
            ),
          ],
          2,
        )
      );
    },
  }),
  bc = { class: "chat-panel" },
  _c = { class: "chat-header" },
  kc = { class: "chat-private" },
  wc = { class: "message-body" },
  $c = { class: "message-meta" },
  Cc = { class: "bubble" },
  xc = { key: 0, class: "error", role: "alert" },
  Sc = ["onClick"],
  Tc = { key: 0, class: "typing" },
  Mc = { class: "composer" },
  Ec = { class: "composer-bottom" },
  Ac = ["disabled"],
  Ic = { key: 0, class: "preview" },
  Oc = { class: "preview-heading" },
  Vc = { class: "scene" },
  Pc = { class: "scene-caption" },
  Lc = { class: "compact-bio" },
  Uc = { class: "schedule-preview" },
  Nc = { class: "next-item" },
  Fc = { class: "preview-tip" },
  Rc = nt({
    __name: "ChatPage",
    props: { preview: { type: Boolean } },
    emits: ["go"],
    setup(e, { emit: t }) {
      const s = t,
        n = (c) => s("go", c),
        l = G(""),
        i = G();
      function r() {
        ls(() => {
          var c;
          return (c = i.value) == null
            ? void 0
            : c.scrollTo({ top: i.value.scrollHeight, behavior: "smooth" });
        });
      }
      function a(c, d) {
        if (/动态|发帖/.test(d))
          return (
            c.posts.unshift({
              text:
                d.replace(/.*?(发一条动态|发动态|发布动态)[：:，,]?/, "") ||
                "今天也在认真感受生活，记录此刻的小美好。",
              time: "刚刚",
              likes: 0,
              liked: !1,
              comments: [],
            }),
            "好呀，已经把这一刻写成动态了。去动态里看看吧。"
          );
        if (/换|穿|衣服/.test(d)) {
          const C = c.garments.find((w) => d.includes(w.name));
          return C
            ? ((c.wearing[C.type] = [C.id]),
              (c.outfit = C.name),
              `换上${c.outfit}了，你觉得怎么样？`)
            : "想换哪件衣服？可以去衣柜里挑选。";
        }
        return /世界|天气|下雨/.test(d)
          ? ((c.world = d),
            (c.location = d.includes("海边") ? "海边" : c.location),
            "好，世界设定已经调整了。新的风景也让人有一点期待。")
          : /日程|安排/.test(d)
            ? (c.schedule.push({ time: "20:00", text: d }),
              "已经把这件事加入日程，今晚会留出时间的。")
            : /日记|补写/.test(d)
              ? (tr(c.id, `${d}。今天的这些小事，都想认真记下来。`),
                "日记补写好了。有些小事，记下来才发现很珍贵。")
              : /累|烦|难过|压力/.test(d)
                ? "听起来今天有些辛苦。愿意和我说说发生了什么吗？我陪你慢慢聊。"
                : /你好|早安|晚安|晚上好/.test(d)
                  ? d.includes("晚安")
                    ? "晚安，愿你今晚睡得安稳。明天再聊。"
                    : "你好呀，见到你很开心。今天过得怎么样？"
                  : /吃|饭|饿/.test(d)
                    ? "说到吃饭，我也想来一碗热汤面。你现在想吃什么？"
                    : /书|读/.test(d)
                      ? "最近在读什么书？想听听你喜欢它的地方。"
                      : /开心|高兴|喜欢/.test(d)
                        ? "听起来是件让人开心的事。愿意多和我分享一点吗？"
                        : /[?？]|怎么|什么/.test(d)
                          ? "我们可以一起想想。你更在意哪一方面？告诉我一些细节吧。"
                          : "我在听，愿意再和我说说吗？也想知道你当时的感受。";
      }
      function u() {
        const c = l.value.trim();
        if (!c || f.value) return;
        const d = W.value;
        l.value = "";
        const C = { text: c, user: !0, time: "刚刚", action: void 0 };
        d.messages.push(C);
        const w = d.messages[d.messages.length - 1];
        ((w.action = Je("聊天回复", c, () =>
          d.messages.push({ text: a(d, c), time: "刚刚" }),
        )),
          r());
      }
      const f = _e(() =>
        W.value.messages.some((c) => {
          var d;
          return ((d = c.action) == null ? void 0 : d.status) === "pending";
        }),
      );
      return (
        it(() => W.value.messages.length, r),
        it(f, r),
        (c, d) => {
          var C, w;
          return (
            p(),
            g(
              "div",
              { class: ge(["chat-layout", { collapsed: !e.preview }]) },
              [
                o("section", bc, [
                  o("header", _c, [
                    o(
                      "span",
                      {
                        class: "avatar",
                        style: yt({ background: M(W).color }),
                      },
                      v(M(W).initial),
                      5,
                    ),
                    o("div", null, [
                      o("strong", null, v(M(W).name), 1),
                      o("small", null, [
                        d[3] ||
                          (d[3] = o("i", { class: "online-dot" }, null, -1)),
                        d[4] || (d[4] = Z(" 在线 ", -1)),
                        o("span", null, "· " + v(M(W).job), 1),
                      ]),
                    ]),
                    d[6] || (d[6] = o("span", { class: "spacer" }, null, -1)),
                    o("span", kc, [
                      me(M(tc), { size: 13 }),
                      d[5] || (d[5] = Z("只属于你们的对话", -1)),
                    ]),
                  ]),
                  o(
                    "div",
                    { ref_key: "timeline", ref: i, class: "timeline" },
                    [
                      d[9] ||
                        (d[9] = o(
                          "div",
                          { class: "date-divider" },
                          [o("span", null, "今天 · 10月23日 星期五")],
                          -1,
                        )),
                      d[10] ||
                        (d[10] = o(
                          "div",
                          { class: "conversation-note" },
                          "从一句问候开始，让陪伴慢慢发生。",
                          -1,
                        )),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(M(W).messages, (A, B) => {
                          var te;
                          return (
                            p(),
                            g(
                              "div",
                              {
                                key: B,
                                class: ge(["message", { mine: A.user }]),
                              },
                              [
                                o(
                                  "span",
                                  {
                                    class: "avatar message-avatar",
                                    style: yt({
                                      background: A.user
                                        ? "#e9eae6"
                                        : M(W).color,
                                    }),
                                  },
                                  v(A.user ? "我" : M(W).initial),
                                  5,
                                ),
                                o("div", wc, [
                                  o("div", $c, [
                                    Z(v(A.user ? "我" : M(W).name), 1),
                                    o("time", null, v(A.time), 1),
                                  ]),
                                  o("div", Cc, v(A.text), 1),
                                  ((te = A.action) == null
                                    ? void 0
                                    : te.status) === "error"
                                    ? (p(),
                                      g("div", xc, [
                                        Z(v(A.action.reason), 1),
                                        o(
                                          "button",
                                          { onClick: (se) => A.action.run() },
                                          [
                                            me(M(Qu), { size: 12 }),
                                            d[7] || (d[7] = Z("重试", -1)),
                                          ],
                                          8,
                                          Sc,
                                        ),
                                      ]))
                                    : X("", !0),
                                ]),
                              ],
                              2,
                            )
                          );
                        }),
                        128,
                      )),
                      f.value
                        ? (p(),
                          g("div", Tc, [
                            Z(v(M(W).name) + " 正在输入 ", 1),
                            d[8] || (d[8] = o("span", null, "•••", -1)),
                          ]))
                        : X("", !0),
                    ],
                    512,
                  ),
                  o("div", Mc, [
                    pe(
                      o(
                        "textarea",
                        {
                          "onUpdate:modelValue":
                            d[0] || (d[0] = (A) => (l.value = A)),
                          "aria-label": "聊天消息",
                          placeholder: "说点什么，分享你的今天…",
                          onKeydown:
                            d[1] ||
                            (d[1] = Gi(
                              Ve(
                                (A) => !A.isComposing && u(),
                                ["exact", "prevent"],
                              ),
                              ["enter"],
                            )),
                        },
                        null,
                        544,
                      ),
                      [[$e, l.value]],
                    ),
                    o("div", Ec, [
                      d[11] ||
                        (d[11] = o(
                          "small",
                          null,
                          "Enter 发送 · Shift + Enter 换行",
                          -1,
                        )),
                      o(
                        "button",
                        {
                          class: "button primary",
                          disabled: !l.value.trim() || f.value,
                          onClick: u,
                        },
                        [
                          Z(v(f.value ? "回应中" : "发送"), 1),
                          me(M(Xu), { size: 14 }),
                        ],
                        8,
                        Ac,
                      ),
                    ]),
                  ]),
                ]),
                e.preview
                  ? (p(),
                    g("aside", Ic, [
                      o("div", Oc, [
                        o(
                          "span",
                          null,
                          v(M(W).id === 1 ? "他的此刻" : "她的此刻"),
                          1,
                        ),
                        d[12] ||
                          (d[12] = o(
                            "span",
                            { class: "pill" },
                            "实时预览",
                            -1,
                          )),
                      ]),
                      o("div", Vc, [
                        d[14] ||
                          (d[14] = o(
                            "div",
                            { class: "scene-window" },
                            [o("i"), o("i")],
                            -1,
                          )),
                        d[15] ||
                          (d[15] = o("div", { class: "scene-sun" }, null, -1)),
                        d[16] ||
                          (d[16] = o("div", { class: "scene-vase" }, "♧", -1)),
                        d[17] ||
                          (d[17] = o("div", { class: "scene-desk" }, null, -1)),
                        o("div", Pc, [
                          d[13] ||
                            (d[13] = o(
                              "span",
                              { class: "scene-tag" },
                              "A LITTLE MOMENT",
                              -1,
                            )),
                          o("p", null, v(M(W).location) + "的午后", 1),
                        ]),
                      ]),
                      o("div", Lc, [
                        o("b", null, v(M(W).name) + " · " + v(M(W).job), 1),
                        o("p", null, v(M(W).bio), 1),
                        o(
                          "small",
                          null,
                          v(M(W).mood) + " · " + v(M(W).location),
                          1,
                        ),
                      ]),
                      me(sr),
                      o("div", Uc, [
                        o("div", null, [
                          d[19] ||
                            (d[19] = o("span", null, "接下来的安排", -1)),
                          o(
                            "button",
                            { onClick: d[2] || (d[2] = (A) => n("日程")) },
                            [
                              d[18] || (d[18] = Z("查看全部 ", -1)),
                              me(M(Ji), { size: 12 }),
                            ],
                          ),
                        ]),
                        o("div", Nc, [
                          d[21] ||
                            (d[21] = o(
                              "span",
                              { class: "schedule-dot" },
                              null,
                              -1,
                            )),
                          o("div", null, [
                            o(
                              "small",
                              null,
                              v(
                                (C = M(W).schedule[1]) == null
                                  ? void 0
                                  : C.time,
                              ) + " — 16:00",
                              1,
                            ),
                            o(
                              "p",
                              null,
                              v(
                                (w = M(W).schedule[1]) == null
                                  ? void 0
                                  : w.text,
                              ),
                              1,
                            ),
                            d[20] ||
                              (d[20] = o(
                                "span",
                                { class: "pill" },
                                "进行中",
                                -1,
                              )),
                          ]),
                        ]),
                      ]),
                      o("div", Fc, [
                        me(M(Tl), { size: 16 }),
                        o("p", null, [
                          d[22] ||
                            (d[22] = Z("生活在继续，故事也在慢慢生长。", -1)),
                          d[23] || (d[23] = o("br", null, null, -1)),
                          Z("这里是 " + v(M(W).name) + " 的小小世界。", 1),
                        ]),
                      ]),
                    ]))
                  : X("", !0),
              ],
              2,
            )
          );
        }
      );
    },
  }),
  Dc = { key: 0, class: "inline-error", role: "alert" },
  It = nt({
    __name: "ActionFeedback",
    props: { labels: {} },
    setup(e) {
      const t = e,
        s = _e(() =>
          vs.value
            .filter(
              (n) => t.labels.includes(n.label) && n.character === W.value.name,
            )
            .at(-1),
        );
      return (n, l) => {
        var i;
        return ((i = s.value) == null ? void 0 : i.status) === "error"
          ? (p(),
            g("p", Dc, [
              Z(v(s.value.reason) + " ", 1),
              o(
                "button",
                {
                  class: "text-button",
                  onClick:
                    l[0] || (l[0] = (...r) => s.value.run && s.value.run(...r)),
                },
                "重试",
              ),
            ]))
          : X("", !0);
      };
    },
  }),
  jc = ["data-testid"],
  Hc = { class: "filter-search-row" },
  Bc = ["value", "placeholder", "aria-label", "data-testid"],
  Kc = ["data-testid"],
  Wc = ["aria-label"],
  qc = ["aria-pressed", "data-testid", "onClick"],
  zc = ["aria-pressed", "data-testid", "onClick"],
  Os = nt({
    __name: "FilterChips",
    props: {
      search: {},
      filters: {},
      groups: {},
      placeholder: { default: "搜索…" },
      testId: {},
    },
    emits: ["update:search", "update:filters", "clear"],
    setup(e, { emit: t }) {
      const s = e,
        n = t,
        l = _e(
          () => !!s.search.trim() || Object.values(s.filters).some(Boolean),
        );
      function i(a, u) {
        n("update:filters", { ...s.filters, [a]: s.filters[a] === u ? "" : u });
      }
      function r() {
        (n("update:search", ""),
          n(
            "update:filters",
            Object.fromEntries(s.groups.map((a) => [a.key, ""])),
          ),
          n("clear"));
      }
      return (a, u) => (
        p(),
        g(
          "section",
          { class: "filter-chips", "data-testid": e.testId },
          [
            o("div", Hc, [
              o(
                "input",
                {
                  value: e.search,
                  type: "search",
                  placeholder: e.placeholder,
                  "aria-label": e.placeholder,
                  "data-testid": `${e.testId}-search`,
                  onInput:
                    u[0] || (u[0] = (f) => n("update:search", f.target.value)),
                },
                null,
                40,
                Bc,
              ),
              l.value
                ? (p(),
                  g(
                    "button",
                    {
                      key: 0,
                      type: "button",
                      class: "text-button",
                      "data-testid": `${e.testId}-clear`,
                      onClick: r,
                    },
                    "清除",
                    8,
                    Kc,
                  ))
                : X("", !0),
            ]),
            (p(!0),
            g(
              O,
              null,
              oe(
                e.groups,
                (f) => (
                  p(),
                  g(
                    "div",
                    {
                      key: f.key,
                      class: "filter-group",
                      role: "group",
                      "aria-label": f.label,
                    },
                    [
                      o("small", null, v(f.label), 1),
                      o(
                        "button",
                        {
                          type: "button",
                          class: ge([
                            "filter-chip",
                            { active: !e.filters[f.key] },
                          ]),
                          "aria-pressed": !e.filters[f.key],
                          "data-testid": `${e.testId}-${f.key}-all`,
                          onClick: (c) => i(f.key, ""),
                        },
                        "全部",
                        10,
                        qc,
                      ),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          f.options,
                          (c) => (
                            p(),
                            g(
                              "button",
                              {
                                key: c,
                                type: "button",
                                class: ge([
                                  "filter-chip",
                                  { active: e.filters[f.key] === c },
                                ]),
                                "aria-pressed": e.filters[f.key] === c,
                                "data-testid": `${e.testId}-${f.key}-${c}`,
                                onClick: (d) => i(f.key, c),
                              },
                              v(c),
                              11,
                              zc,
                            )
                          ),
                        ),
                        128,
                      )),
                    ],
                    8,
                    Wc,
                  )
                ),
              ),
              128,
            )),
          ],
          8,
          jc,
        )
      );
    },
  }),
  Gc = ["data-testid"],
  Jc = ["aria-label", "data-testid"],
  Yc = { class: "drawer-heading" },
  Zc = ["data-testid"],
  Vs = nt({
    __name: "DetailDrawer",
    props: { title: {}, testId: {} },
    emits: ["close"],
    setup(e, { emit: t }) {
      const s = t,
        n = G();
      let l = null;
      function i(r) {
        var c;
        if ((r.key === "Escape" && s("close"), r.key !== "Tab")) return;
        const a =
          (c = n.value) == null
            ? void 0
            : c.querySelectorAll(
                'button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),a[href],summary,[tabindex="0"]',
              );
        if (!(a != null && a.length)) return;
        const u = a[0],
          f = a[a.length - 1];
        r.shiftKey && document.activeElement === u
          ? (r.preventDefault(), f.focus())
          : !r.shiftKey &&
            document.activeElement === f &&
            (r.preventDefault(), u.focus());
      }
      return (
        wl(() => {
          ((l = document.activeElement),
            document.addEventListener("keydown", i),
            ls(() => {
              var r;
              return (r = n.value) == null ? void 0 : r.focus();
            }));
        }),
        Ks(() => {
          (document.removeEventListener("keydown", i),
            l == null || l.focus({ preventScroll: !0 }));
        }),
        (r, a) => (
          p(),
          Ae(ea, { to: "body" }, [
            o(
              "div",
              {
                class: "drawer-backdrop",
                "data-testid": `${e.testId}-overlay`,
                onClick: a[1] || (a[1] = Ve((u) => s("close"), ["self"])),
              },
              [
                o(
                  "aside",
                  {
                    ref_key: "panel",
                    ref: n,
                    class: "detail-drawer",
                    role: "dialog",
                    "aria-modal": "true",
                    "aria-label": e.title,
                    tabindex: "-1",
                    "data-testid": e.testId,
                  },
                  [
                    o("header", Yc, [
                      o("h2", null, v(e.title), 1),
                      o(
                        "button",
                        {
                          type: "button",
                          class: "text-button",
                          "data-testid": `${e.testId}-close`,
                          onClick: a[0] || (a[0] = (u) => s("close")),
                        },
                        "关闭",
                        8,
                        Zc,
                      ),
                    ]),
                    ha(r.$slots, "default"),
                  ],
                  8,
                  Jc,
                ),
              ],
              8,
              Gc,
            ),
          ])
        )
      );
    },
  }),
  Qc = { class: "card now-card" },
  Xc = { class: "grid now-grid" },
  ed = { class: "card" },
  td = { class: "card now-appointments" },
  sd = { class: "row" },
  nd = { key: 0, class: "muted" },
  ld = { class: "card schedule-list", "data-testid": "schedule-list" },
  od = ["data-testid"],
  id = ["disabled"],
  rd = ["disabled"],
  ad = { class: "schedule-text" },
  ud = { key: 0, class: "pill", "data-testid": "schedule-current" },
  cd = { key: 1, class: "actions-menu" },
  dd = ["data-testid"],
  fd = { class: "scroll-lock-list", "data-scroll-lock": "" },
  pd = ["disabled", "onClick"],
  md = { key: 0, class: "muted" },
  vd = { class: "small-card-grid", "data-testid": "goal-grid" },
  hd = { class: "row" },
  gd = { class: "pill" },
  yd = { class: "goal" },
  bd = ["onUpdate:modelValue", "data-testid"],
  _d = ["aria-label", "aria-valuenow"],
  kd = ["data-testid"],
  wd = { class: "small-card-grid", "data-testid": "appointment-grid" },
  $d = { class: "row" },
  Cd = ["disabled"],
  xd = { class: "diary-layout" },
  Sd = { class: "card calendar" },
  Td = { class: "calendar-grid" },
  Md = ["disabled", "aria-label", "aria-pressed", "data-testid", "onClick"],
  Ed = { key: 0 },
  Ad = {
    class: "diary-book",
    "data-testid": "diary-book",
    "aria-label": "日记簿",
  },
  Id = ["data-date"],
  Od = { class: "row diary-attributes" },
  Vd = { class: "diary-text" },
  Pd = { class: "diary-signature" },
  Ld = { key: 1, class: "diary-paper empty-state" },
  Ud = { class: "book-controls" },
  Nd = ["disabled"],
  Fd = { "aria-live": "polite", "data-testid": "diary-page-indicator" },
  Rd = ["disabled"],
  Dd = { class: "memory-list" },
  jd = ["data-testid"],
  Hd = ["aria-expanded", "aria-controls", "data-testid", "onClick"],
  Bd = { class: "memory-chevron" },
  Kd = { class: "memory-content" },
  Wd = { class: "pill" },
  qd = ["id"],
  zd = ["aria-label"],
  Gd = { key: 0, class: "muted" },
  Jd = { class: "row" },
  Yd = ["data-testid", "onClick"],
  Zd = ["data-testid", "onClick"],
  Qd = { key: 0, class: "card empty-state" },
  Xd = { key: 0, class: "inline-error" },
  ef = { class: "chip-field" },
  tf = { class: "star-control", role: "group", "aria-label": "重要度" },
  sf = ["aria-label", "aria-pressed", "data-testid", "onClick"],
  nf = { class: "chip-field" },
  lf = { class: "chip-options scroll-lock-list", "data-scroll-lock": "" },
  of = ["aria-pressed", "onClick"],
  rf = { class: "chip-field" },
  af = { class: "chip-options scroll-lock-list", "data-scroll-lock": "" },
  uf = ["aria-pressed", "data-testid", "onClick"],
  cf = { class: "input-row" },
  df = ["disabled"],
  ff = ["disabled"],
  pf = { key: 2, class: "modal-backdrop" },
  mf = {
    class: "card confirm-dialog",
    role: "alertdialog",
    "aria-modal": "true",
    "aria-label": "遗忘这段记忆？",
    "data-testid": "memory-forget-confirm",
  },
  vf = { class: "row" },
  hf = nt({
    __name: "LifePage",
    props: { page: {} },
    emits: ["go"],
    setup(e, { emit: t }) {
      var $;
      const s = e,
        n = t,
        l = _e(() => er.value[cs.value]),
        i = ds(["日记"]),
        r = ds(["日程"]),
        a = ds(["记忆"]),
        u = _e(() =>
          [...W.value.schedule].sort((h, b) => h.time.localeCompare(b.time)),
        ),
        f = _e(() => u.value.filter((h) => h.time <= "14:30").at(-1)),
        c = _e(() => u.value.filter((h) => h === f.value || h.time > "14:30")),
        d = G(null),
        C = G(""),
        w = G("");
      function A(h, b) {
        var k;
        ((k = b.target.closest("details")) == null || k.removeAttribute("open"),
          (d.value = W.value.schedule.indexOf(h)),
          (C.value = h.time),
          (w.value = h.text));
      }
      function B() {
        if (d.value === null || r.value || !C.value || !w.value.trim()) return;
        const h = W.value,
          b = d.value,
          k = C.value,
          ve = w.value.trim();
        Je("日程", k + " " + ve, () => {
          ((h.schedule[b] = { time: k, text: ve }), (d.value = null));
        });
      }
      const te = G(""),
        se = G({ category: "", tag: "" }),
        ne = _e(() => {
          var h;
          return [
            ...new Set(
              l.value.memories
                .flatMap((b) => b.tags)
                .concat(((h = K.value) == null ? void 0 : h.tags) || []),
            ),
          ];
        }),
        ue = _e(() => [
          {
            key: "category",
            label: "类型",
            options: ["长期记忆", "发生过的事情", "最近在想"],
          },
          { key: "tag", label: "标签", options: ne.value },
        ]),
        S = _e(() =>
          l.value.memories.filter(
            (h) =>
              [h.text, h.source, ...h.tags]
                .join(" ")
                .toLowerCase()
                .includes(te.value.trim().toLowerCase()) &&
              (!se.value.category || h.category === se.value.category) &&
              (!se.value.tag || h.tags.includes(se.value.tag)),
          ),
        ),
        I = G(null),
        K = G(null),
        le = G(null),
        H = G("");
      function q() {
        ((te.value = ""), (se.value = { category: "", tag: "" }));
      }
      function V(h) {
        K.value &&
          (K.value.tags = K.value.tags.includes(h)
            ? K.value.tags.filter((b) => b !== h)
            : [...K.value.tags, h]);
      }
      function U() {
        const h = H.value.trim().replace(/^#\s*/, "");
        !h ||
          !K.value ||
          (K.value.tags.includes(h) || K.value.tags.push(h), (H.value = ""));
      }
      function _() {
        var k;
        if (!((k = K.value) != null && k.text.trim()) || a.value) return;
        const h = l.value,
          b = { ...K.value, tags: [...K.value.tags] };
        Je("记忆", [b.text, b.date, b.category, ...b.tags].join(" "), () => {
          const ve = h.memories.findIndex((Ye) => Ye.id === b.id);
          (ve >= 0 && (h.memories[ve] = b), (K.value = null));
        });
      }
      const ee = _e(() =>
          [...l.value.diaries].sort((h, b) => h.date.localeCompare(b.date)),
        ),
        he = G((($ = ee.value.at(-1)) == null ? void 0 : $.date) || ""),
        j = G("page-next"),
        T = _e(() => ee.value.findIndex((h) => h.date === he.value)),
        x = _e(() => ee.value[T.value]);
      function R(h) {
        !ee.value.some((b) => b.date === h) ||
          h === he.value ||
          ((j.value = h > he.value ? "page-next" : "page-prev"),
          (he.value = h));
      }
      function ye(h) {
        const b = ee.value[T.value + h];
        b && R(b.date);
      }
      function F() {
        if (i.value) return;
        const h = W.value,
          b = l.value.diaries.some((k) => k.date === gn);
        Je(
          "日记",
          "生成" + h.name + "的日记",
          () => {
            (tr(
              h.id,
              `今天在${h.location}${h.activity}，窗外的光一点点移过桌面。忙完后给自己泡了一杯茶，也想起与你聊天的片刻。那些平常的小事，原来一直在心里留下回响。`,
            ),
              R(gn));
          },
          h.name + "的今日日记",
          b ? "今日日记已更新" : "今日日记已写好",
        );
      }
      return (
        it(
          () => s.page,
          async (h) => {
            var b;
            ((d.value = null),
              (K.value = null),
              (le.value = null),
              h === "目标" && tn.value
                ? (await ls(),
                  (b = document.getElementById("appointments")) == null ||
                    b.scrollIntoView({ behavior: "smooth", block: "center" }))
                : h !== "目标" && (tn.value = !1));
          },
          { immediate: !0 },
        ),
        (h, b) => (
          p(),
          g(
            O,
            null,
            [
              e.page === "此刻"
                ? (p(),
                  g(
                    O,
                    { key: 0 },
                    [
                      o("div", Qc, [
                        b[16] ||
                          (b[16] = o(
                            "span",
                            { class: "eyebrow" },
                            "FRIDAY, OCTOBER 23",
                            -1,
                          )),
                        o("h2", null, v(M(W).activity), 1),
                        o(
                          "p",
                          null,
                          v(M(W).location) + " · " + v(M(W).mood),
                          1,
                        ),
                        o("p", null, v(M(W).world), 1),
                      ]),
                      o("div", Xc, [
                        o("div", ed, [me(sr, { compact: "" })]),
                        o("section", td, [
                          o("div", sd, [
                            b[17] || (b[17] = o("h2", null, "约定", -1)),
                            b[18] ||
                              (b[18] = o(
                                "span",
                                { class: "spacer" },
                                null,
                                -1,
                              )),
                            o(
                              "button",
                              {
                                class: "text-button",
                                "data-testid": "now-appointments",
                                onClick:
                                  b[0] ||
                                  (b[0] = (k) => {
                                    ((tn.value = !0), n("go", "目标"));
                                  }),
                              },
                              "查看全部 ↗",
                            ),
                          ]),
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(
                              l.value.appointments.slice(0, 2),
                              (k) => (
                                p(),
                                g(
                                  "div",
                                  {
                                    key: k.date + k.time,
                                    class: "appointment-row",
                                  },
                                  [
                                    o("time", null, [
                                      Z(
                                        v(k.date.slice(5).replace("-", "月")) +
                                          "日",
                                        1,
                                      ),
                                      b[19] ||
                                        (b[19] = o("br", null, null, -1)),
                                      Z(v(k.time), 1),
                                    ]),
                                    o("span", null, v(k.text), 1),
                                  ],
                                )
                              ),
                            ),
                            128,
                          )),
                          l.value.appointments.length
                            ? X("", !0)
                            : (p(),
                              g("p", nd, "还没有约定，留一些期待给明天。")),
                        ]),
                      ]),
                    ],
                    64,
                  ))
                : X("", !0),
              e.page === "此刻" || e.page === "日程"
                ? (p(),
                  g(
                    O,
                    { key: 1 },
                    [
                      o(
                        "h2",
                        null,
                        v(e.page === "此刻" ? "此刻与接下来" : "今天的安排"),
                        1,
                      ),
                      o("section", ld, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            e.page === "此刻" ? c.value : u.value,
                            (k) => (
                              p(),
                              g(
                                "div",
                                {
                                  key: k.time + k.text,
                                  class: ge([
                                    "schedule-row",
                                    { "is-current": k === f.value },
                                  ]),
                                  "data-testid": "schedule-" + k.time,
                                },
                                [
                                  d.value === M(W).schedule.indexOf(k) &&
                                  e.page === "日程"
                                    ? (p(),
                                      g(
                                        "form",
                                        {
                                          key: 0,
                                          class: "schedule-editor",
                                          onSubmit: Ve(B, ["prevent"]),
                                        },
                                        [
                                          pe(
                                            o(
                                              "input",
                                              {
                                                "onUpdate:modelValue":
                                                  b[1] ||
                                                  (b[1] = (ve) =>
                                                    (C.value = ve)),
                                                type: "time",
                                                "aria-label": "时间",
                                              },
                                              null,
                                              512,
                                            ),
                                            [[$e, C.value]],
                                          ),
                                          pe(
                                            o(
                                              "input",
                                              {
                                                "onUpdate:modelValue":
                                                  b[2] ||
                                                  (b[2] = (ve) =>
                                                    (w.value = ve)),
                                                "aria-label": "日程内容",
                                              },
                                              null,
                                              512,
                                            ),
                                            [[$e, w.value]],
                                          ),
                                          o(
                                            "button",
                                            {
                                              class: "button",
                                              disabled:
                                                M(r) ||
                                                !w.value.trim() ||
                                                !C.value,
                                              "data-testid": "schedule-save",
                                            },
                                            v(M(r) ? "处理中…" : "保存"),
                                            9,
                                            id,
                                          ),
                                          o(
                                            "button",
                                            {
                                              type: "button",
                                              class: "text-button",
                                              disabled: M(r),
                                              onClick:
                                                b[3] ||
                                                (b[3] = (ve) =>
                                                  (d.value = null)),
                                            },
                                            "取消",
                                            8,
                                            rd,
                                          ),
                                        ],
                                        32,
                                      ))
                                    : (p(),
                                      g(
                                        O,
                                        { key: 1 },
                                        [
                                          o("time", null, v(k.time), 1),
                                          b[20] ||
                                            (b[20] = o(
                                              "span",
                                              { class: "schedule-dot" },
                                              null,
                                              -1,
                                            )),
                                          o("span", ad, v(k.text), 1),
                                          k === f.value
                                            ? (p(), g("span", ud, "进行中"))
                                            : X("", !0),
                                          e.page === "日程"
                                            ? (p(),
                                              g("details", cd, [
                                                o(
                                                  "summary",
                                                  {
                                                    "aria-label": "日程操作",
                                                    "data-testid":
                                                      "schedule-menu-" + k.time,
                                                  },
                                                  "⋯",
                                                  8,
                                                  dd,
                                                ),
                                                o("div", fd, [
                                                  o(
                                                    "button",
                                                    {
                                                      disabled: M(r),
                                                      onClick: (ve) => A(k, ve),
                                                    },
                                                    "编辑",
                                                    8,
                                                    pd,
                                                  ),
                                                ]),
                                              ]))
                                            : X("", !0),
                                        ],
                                        64,
                                      )),
                                ],
                                10,
                                od,
                              )
                            ),
                          ),
                          128,
                        )),
                        (e.page === "此刻" ? c.value : u.value).length
                          ? X("", !0)
                          : (p(), g("p", md, "暂时没有安排，留一点自由时间。")),
                      ]),
                    ],
                    64,
                  ))
                : X("", !0),
              e.page === "目标"
                ? (p(),
                  g(
                    O,
                    { key: 2 },
                    [
                      b[24] ||
                        (b[24] = o(
                          "p",
                          { class: "muted" },
                          "想做的事慢慢来，约好的事认真记住。",
                          -1,
                        )),
                      o("div", vd, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            M(W).goals,
                            (k, ve) => (
                              p(),
                              g(
                                "article",
                                { key: k.text, class: "card goal-card" },
                                [
                                  o("div", hd, [
                                    o("span", gd, v(ve ? "生活" : "创作"), 1),
                                    b[21] ||
                                      (b[21] = o(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    o(
                                      "small",
                                      null,
                                      v(k.done ? 100 : k.progress) + "%",
                                      1,
                                    ),
                                  ]),
                                  o("label", yd, [
                                    pe(
                                      o(
                                        "input",
                                        {
                                          "onUpdate:modelValue": (Ye) =>
                                            (k.done = Ye),
                                          type: "checkbox",
                                          "data-testid": "goal-toggle-" + ve,
                                        },
                                        null,
                                        8,
                                        bd,
                                      ),
                                      [[ss, k.done]],
                                    ),
                                    o(
                                      "span",
                                      { class: ge({ done: k.done }) },
                                      v(k.text),
                                      3,
                                    ),
                                  ]),
                                  o(
                                    "div",
                                    {
                                      class: "progress",
                                      role: "progressbar",
                                      "aria-label": k.text,
                                      "aria-valuenow": k.done
                                        ? 100
                                        : k.progress,
                                      "aria-valuemin": 0,
                                      "aria-valuemax": 100,
                                    },
                                    [
                                      o(
                                        "i",
                                        {
                                          style: yt({
                                            width:
                                              (k.done ? 100 : k.progress) + "%",
                                          }),
                                          "data-testid": "goal-progress-" + ve,
                                        },
                                        null,
                                        12,
                                        kd,
                                      ),
                                    ],
                                    8,
                                    _d,
                                  ),
                                ],
                              )
                            ),
                          ),
                          128,
                        )),
                      ]),
                      o(
                        "section",
                        { id: "appointments", class: ge({ highlight: M(tn) }) },
                        [
                          b[23] ||
                            (b[23] = o(
                              "h2",
                              { class: "section-heading" },
                              "约定",
                              -1,
                            )),
                          o("div", wd, [
                            (p(!0),
                            g(
                              O,
                              null,
                              oe(
                                [...l.value.appointments].sort((k, ve) =>
                                  (k.date + k.time).localeCompare(
                                    ve.date + ve.time,
                                  ),
                                ),
                                (k) => (
                                  p(),
                                  g(
                                    "article",
                                    {
                                      key: k.date + k.time,
                                      class: "card appointment-card",
                                    },
                                    [
                                      o(
                                        "small",
                                        null,
                                        v(k.date) + " · " + v(k.time),
                                        1,
                                      ),
                                      o("h3", null, v(k.text), 1),
                                      b[22] ||
                                        (b[22] = o(
                                          "span",
                                          { class: "muted" },
                                          "留一个位置，给共同的期待。",
                                          -1,
                                        )),
                                    ],
                                  )
                                ),
                              ),
                              128,
                            )),
                          ]),
                        ],
                        2,
                      ),
                    ],
                    64,
                  ))
                : X("", !0),
              e.page === "日记"
                ? (p(),
                  g(
                    O,
                    { key: 3 },
                    [
                      o("div", $d, [
                        b[25] ||
                          (b[25] = o(
                            "div",
                            null,
                            [
                              o("h2", null, "写给自己的时光"),
                              o(
                                "p",
                                { class: "muted" },
                                "有些心事，留在纸页之间。",
                              ),
                            ],
                            -1,
                          )),
                        b[26] ||
                          (b[26] = o("span", { class: "spacer" }, null, -1)),
                        o(
                          "button",
                          {
                            class: "button primary",
                            disabled: M(i),
                            "data-testid": "diary-generate",
                            onClick: F,
                          },
                          v(M(i) ? "处理中…" : "生成日记"),
                          9,
                          Cd,
                        ),
                      ]),
                      o("div", xd, [
                        o("section", Sd, [
                          b[27] || (b[27] = o("h3", null, "2026 年 10 月", -1)),
                          o("div", Td, [
                            (p(),
                            g(
                              O,
                              null,
                              oe(
                                ["一", "二", "三", "四", "五", "六", "日"],
                                (k) => o("small", { key: k }, v(k), 1),
                              ),
                              64,
                            )),
                            (p(),
                            g(
                              O,
                              null,
                              oe(3, (k) => o("span", { key: "blank" + k })),
                              64,
                            )),
                            (p(),
                            g(
                              O,
                              null,
                              oe(31, (k) =>
                                o(
                                  "button",
                                  {
                                    key: k,
                                    class: ge({
                                      marked: ee.value.some(
                                        (ve) => Number(ve.date.slice(-2)) === k,
                                      ),
                                      selected:
                                        he.value ===
                                        `2026-10-${String(k).padStart(2, "0")}`,
                                    }),
                                    disabled: !ee.value.some(
                                      (ve) => Number(ve.date.slice(-2)) === k,
                                    ),
                                    "aria-label": `2026年10月${k}日`,
                                    "aria-pressed":
                                      he.value ===
                                      `2026-10-${String(k).padStart(2, "0")}`,
                                    "data-testid": `calendar-day-2026-10-${String(k).padStart(2, "0")}`,
                                    onClick: (ve) =>
                                      R(
                                        `2026-10-${String(k).padStart(2, "0")}`,
                                      ),
                                  },
                                  [
                                    Z(v(k), 1),
                                    ee.value.some(
                                      (ve) => Number(ve.date.slice(-2)) === k,
                                    )
                                      ? (p(), g("i", Ed))
                                      : X("", !0),
                                  ],
                                  10,
                                  Md,
                                ),
                              ),
                              64,
                            )),
                          ]),
                          b[28] ||
                            (b[28] = o(
                              "p",
                              { class: "calendar-note muted" },
                              "有圆点的日子，藏着一页故事。",
                              -1,
                            )),
                        ]),
                        o("section", Ad, [
                          me(
                            ru,
                            { name: j.value, mode: "out-in" },
                            {
                              default: Ht(() => [
                                x.value
                                  ? (p(),
                                    g(
                                      "article",
                                      {
                                        key: x.value.date,
                                        class: "diary-paper",
                                        "data-testid": "diary-entry",
                                        "data-date": x.value.date,
                                      },
                                      [
                                        o(
                                          "small",
                                          null,
                                          v(x.value.date) +
                                            " · " +
                                            v(x.value.time),
                                          1,
                                        ),
                                        o("h2", null, v(x.value.title), 1),
                                        o("div", Od, [
                                          o(
                                            "span",
                                            null,
                                            "天气 · " + v(x.value.weather),
                                            1,
                                          ),
                                          o(
                                            "span",
                                            null,
                                            "心情 · " + v(x.value.mood),
                                            1,
                                          ),
                                        ]),
                                        o("p", Vd, v(x.value.text), 1),
                                        o("p", Pd, "— " + v(M(W).name), 1),
                                      ],
                                      8,
                                      Id,
                                    ))
                                  : (p(),
                                    g("div", Ld, [
                                      ...(b[29] ||
                                        (b[29] = [
                                          o(
                                            "p",
                                            null,
                                            "这一页，等着今天的故事。",
                                            -1,
                                          ),
                                        ])),
                                    ])),
                              ]),
                              _: 1,
                            },
                            8,
                            ["name"],
                          ),
                          o("footer", Ud, [
                            o(
                              "button",
                              {
                                class: "text-button",
                                disabled: T.value <= 0,
                                "data-testid": "diary-prev",
                                onClick: b[4] || (b[4] = (k) => ye(-1)),
                              },
                              "← 上一页",
                              8,
                              Nd,
                            ),
                            o(
                              "small",
                              Fd,
                              v(x.value ? T.value + 1 : 0) +
                                " / " +
                                v(ee.value.length),
                              1,
                            ),
                            o(
                              "button",
                              {
                                class: "text-button",
                                disabled:
                                  T.value < 0 || T.value >= ee.value.length - 1,
                                "data-testid": "diary-next",
                                onClick: b[5] || (b[5] = (k) => ye(1)),
                              },
                              "下一页 →",
                              8,
                              Rd,
                            ),
                          ]),
                        ]),
                      ]),
                    ],
                    64,
                  ))
                : X("", !0),
              e.page === "记忆"
                ? (p(),
                  g(
                    O,
                    { key: 4 },
                    [
                      b[41] ||
                        (b[41] = o(
                          "p",
                          { class: "muted" },
                          "把相处留下的细节，慢慢收进记忆。",
                          -1,
                        )),
                      me(
                        Os,
                        {
                          search: te.value,
                          "onUpdate:search":
                            b[6] || (b[6] = (k) => (te.value = k)),
                          filters: se.value,
                          "onUpdate:filters":
                            b[7] || (b[7] = (k) => (se.value = k)),
                          groups: ue.value,
                          "test-id": "memory-filters",
                          placeholder: "搜索记忆 / 标签",
                        },
                        null,
                        8,
                        ["search", "filters", "groups"],
                      ),
                      o("section", Dd, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            S.value,
                            (k) => (
                              p(),
                              g(
                                "article",
                                {
                                  key: k.id,
                                  class: ge([
                                    "memory-item",
                                    { expanded: I.value === k.id },
                                  ]),
                                  "data-testid": "memory-row-" + k.id,
                                },
                                [
                                  o(
                                    "button",
                                    {
                                      class: "memory-summary",
                                      "aria-expanded": I.value === k.id,
                                      "aria-controls": "memory-details-" + k.id,
                                      "data-testid": "memory-toggle-" + k.id,
                                      onClick: (ve) =>
                                        (I.value =
                                          I.value === k.id ? null : k.id),
                                    },
                                    [
                                      o(
                                        "span",
                                        Bd,
                                        v(I.value === k.id ? "−" : "+"),
                                        1,
                                      ),
                                      o("span", Kd, v(k.text), 1),
                                      o("time", null, v(k.date), 1),
                                      o("span", Wd, v(k.category), 1),
                                    ],
                                    8,
                                    Hd,
                                  ),
                                  I.value === k.id
                                    ? (p(),
                                      g(
                                        "div",
                                        {
                                          key: 0,
                                          id: "memory-details-" + k.id,
                                          class: "memory-details",
                                        },
                                        [
                                          o("dl", null, [
                                            o("div", null, [
                                              b[30] ||
                                                (b[30] = o(
                                                  "dt",
                                                  null,
                                                  "来源",
                                                  -1,
                                                )),
                                              o(
                                                "dd",
                                                null,
                                                v(k.source) + " · " + v(k.date),
                                                1,
                                              ),
                                            ]),
                                            o("div", null, [
                                              b[31] ||
                                                (b[31] = o(
                                                  "dt",
                                                  null,
                                                  "重要度",
                                                  -1,
                                                )),
                                              o(
                                                "dd",
                                                {
                                                  class: "memory-stars",
                                                  "aria-label":
                                                    k.importance + "星，共5星",
                                                },
                                                [
                                                  Z(
                                                    v("★".repeat(k.importance)),
                                                    1,
                                                  ),
                                                  o(
                                                    "span",
                                                    null,
                                                    v(
                                                      "☆".repeat(
                                                        5 - k.importance,
                                                      ),
                                                    ),
                                                    1,
                                                  ),
                                                ],
                                                8,
                                                zd,
                                              ),
                                            ]),
                                            o("div", null, [
                                              b[32] ||
                                                (b[32] = o(
                                                  "dt",
                                                  null,
                                                  "标签",
                                                  -1,
                                                )),
                                              o("dd", null, [
                                                (p(!0),
                                                g(
                                                  O,
                                                  null,
                                                  oe(
                                                    k.tags,
                                                    (ve) => (
                                                      p(),
                                                      g(
                                                        "span",
                                                        {
                                                          key: ve,
                                                          class: "pill",
                                                        },
                                                        "# " + v(ve),
                                                        1,
                                                      )
                                                    ),
                                                  ),
                                                  128,
                                                )),
                                                k.tags.length
                                                  ? X("", !0)
                                                  : (p(),
                                                    g("span", Gd, "暂无标签")),
                                              ]),
                                            ]),
                                          ]),
                                          o("div", Jd, [
                                            o(
                                              "button",
                                              {
                                                class: "text-button",
                                                "data-testid":
                                                  "memory-edit-" + k.id,
                                                onClick: (ve) => {
                                                  ((K.value = {
                                                    ...k,
                                                    tags: [...k.tags],
                                                  }),
                                                    (H.value = ""));
                                                },
                                              },
                                              "编辑",
                                              8,
                                              Yd,
                                            ),
                                            o(
                                              "button",
                                              {
                                                class: "text-button",
                                                "data-testid":
                                                  "memory-forget-" + k.id,
                                                onClick: (ve) => (le.value = k),
                                              },
                                              "遗忘",
                                              8,
                                              Zd,
                                            ),
                                          ]),
                                        ],
                                        8,
                                        qd,
                                      ))
                                    : X("", !0),
                                ],
                                10,
                                jd,
                              )
                            ),
                          ),
                          128,
                        )),
                      ]),
                      S.value.length
                        ? X("", !0)
                        : (p(),
                          g("div", Qd, [
                            b[33] ||
                              (b[33] = o(
                                "p",
                                null,
                                "没有符合条件的记忆。",
                                -1,
                              )),
                            o(
                              "button",
                              {
                                class: "button",
                                "data-testid": "memory-empty-clear",
                                onClick: q,
                              },
                              "清除筛选",
                            ),
                          ])),
                      K.value
                        ? (p(),
                          Ae(
                            Vs,
                            {
                              key: 1,
                              title: "编辑记忆",
                              "test-id": "memory-drawer",
                              onClose:
                                b[13] || (b[13] = (k) => (K.value = null)),
                            },
                            {
                              default: Ht(() => [
                                o(
                                  "form",
                                  {
                                    class: "form-grid",
                                    onSubmit: Ve(_, ["prevent"]),
                                  },
                                  [
                                    o("label", null, [
                                      b[34] || (b[34] = Z("内容", -1)),
                                      pe(
                                        o(
                                          "textarea",
                                          {
                                            "onUpdate:modelValue":
                                              b[8] ||
                                              (b[8] = (k) =>
                                                (K.value.text = k)),
                                            rows: "4",
                                            "data-testid":
                                              "memory-edit-content",
                                          },
                                          null,
                                          512,
                                        ),
                                        [[$e, K.value.text]],
                                      ),
                                      K.value.text.trim()
                                        ? X("", !0)
                                        : (p(),
                                          g("small", Xd, "请填写记忆内容")),
                                    ]),
                                    o("label", null, [
                                      b[35] || (b[35] = Z("来源", -1)),
                                      pe(
                                        o(
                                          "input",
                                          {
                                            "onUpdate:modelValue":
                                              b[9] ||
                                              (b[9] = (k) =>
                                                (K.value.source = k)),
                                          },
                                          null,
                                          512,
                                        ),
                                        [[$e, K.value.source]],
                                      ),
                                    ]),
                                    o("label", null, [
                                      b[36] || (b[36] = Z("来源日期", -1)),
                                      pe(
                                        o(
                                          "input",
                                          {
                                            "onUpdate:modelValue":
                                              b[10] ||
                                              (b[10] = (k) =>
                                                (K.value.date = k)),
                                            type: "date",
                                          },
                                          null,
                                          512,
                                        ),
                                        [[$e, K.value.date]],
                                      ),
                                    ]),
                                    o("fieldset", ef, [
                                      b[37] ||
                                        (b[37] = o(
                                          "legend",
                                          null,
                                          "重要度",
                                          -1,
                                        )),
                                      o("div", tf, [
                                        (p(),
                                        g(
                                          O,
                                          null,
                                          oe(5, (k) =>
                                            o(
                                              "button",
                                              {
                                                key: k,
                                                type: "button",
                                                "aria-label": k + "星",
                                                "aria-pressed":
                                                  K.value.importance === k,
                                                "data-testid":
                                                  "memory-star-" + k,
                                                class: ge({
                                                  filled:
                                                    k <= K.value.importance,
                                                }),
                                                onClick: (ve) =>
                                                  (K.value.importance = k),
                                              },
                                              "★",
                                              10,
                                              sf,
                                            ),
                                          ),
                                          64,
                                        )),
                                        o(
                                          "small",
                                          null,
                                          v(K.value.importance) + " / 5",
                                          1,
                                        ),
                                      ]),
                                    ]),
                                    o("fieldset", nf, [
                                      b[38] ||
                                        (b[38] = o("legend", null, "类型", -1)),
                                      o("div", lf, [
                                        (p(),
                                        g(
                                          O,
                                          null,
                                          oe(
                                            [
                                              "长期记忆",
                                              "发生过的事情",
                                              "最近在想",
                                            ],
                                            (k) =>
                                              o(
                                                "button",
                                                {
                                                  key: k,
                                                  type: "button",
                                                  class: ge([
                                                    "filter-chip",
                                                    {
                                                      active:
                                                        K.value.category === k,
                                                    },
                                                  ]),
                                                  "aria-pressed":
                                                    K.value.category === k,
                                                  onClick: (ve) =>
                                                    (K.value.category = k),
                                                },
                                                v(k),
                                                11,
                                                of,
                                              ),
                                          ),
                                          64,
                                        )),
                                      ]),
                                    ]),
                                    o("fieldset", rf, [
                                      b[39] ||
                                        (b[39] = o("legend", null, "标签", -1)),
                                      o("div", af, [
                                        (p(!0),
                                        g(
                                          O,
                                          null,
                                          oe(
                                            ne.value,
                                            (k) => (
                                              p(),
                                              g(
                                                "button",
                                                {
                                                  key: k,
                                                  type: "button",
                                                  class: ge([
                                                    "filter-chip",
                                                    {
                                                      active:
                                                        K.value.tags.includes(
                                                          k,
                                                        ),
                                                    },
                                                  ]),
                                                  "aria-pressed":
                                                    K.value.tags.includes(k),
                                                  "data-testid":
                                                    "memory-tag-" + k,
                                                  onClick: (ve) => V(k),
                                                },
                                                "# " + v(k),
                                                11,
                                                uf,
                                              )
                                            ),
                                          ),
                                          128,
                                        )),
                                      ]),
                                      o("div", cf, [
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                b[11] ||
                                                (b[11] = (k) => (H.value = k)),
                                              "aria-label": "新标签",
                                              placeholder: "添加一个新标签",
                                              "data-testid": "memory-new-tag",
                                              onKeydown:
                                                b[12] ||
                                                (b[12] = Gi(
                                                  Ve(
                                                    (k) =>
                                                      !k.isComposing && U(),
                                                    ["prevent"],
                                                  ),
                                                  ["enter"],
                                                )),
                                            },
                                            null,
                                            544,
                                          ),
                                          [[$e, H.value]],
                                        ),
                                        o(
                                          "button",
                                          {
                                            type: "button",
                                            class: "text-button",
                                            disabled: !H.value.trim(),
                                            "data-testid": "memory-add-tag",
                                            onClick: U,
                                          },
                                          "添加",
                                          8,
                                          df,
                                        ),
                                      ]),
                                    ]),
                                    me(It, { labels: ["记忆"] }),
                                    o(
                                      "button",
                                      {
                                        class: "button primary",
                                        disabled: M(a) || !K.value.text.trim(),
                                        "data-testid": "memory-save",
                                      },
                                      v(M(a) ? "处理中…" : "保存"),
                                      9,
                                      ff,
                                    ),
                                  ],
                                  32,
                                ),
                              ]),
                              _: 1,
                            },
                          ))
                        : X("", !0),
                      le.value
                        ? (p(),
                          g("div", pf, [
                            o("section", mf, [
                              b[40] ||
                                (b[40] = o("h2", null, "遗忘这段记忆？", -1)),
                              o("p", null, v(le.value.text), 1),
                              o("div", vf, [
                                o(
                                  "button",
                                  {
                                    class: "button",
                                    onClick:
                                      b[14] ||
                                      (b[14] = (k) => (le.value = null)),
                                  },
                                  "取消",
                                ),
                                o(
                                  "button",
                                  {
                                    class: "button primary",
                                    "data-testid": "memory-confirm-forget",
                                    onClick:
                                      b[15] ||
                                      (b[15] = (k) => {
                                        ((l.value.memories =
                                          l.value.memories.filter((ve) => {
                                            var Ye;
                                            return (
                                              ve.id !==
                                              ((Ye = le.value) == null
                                                ? void 0
                                                : Ye.id)
                                            );
                                          })),
                                          (le.value = null));
                                      }),
                                  },
                                  "确认遗忘",
                                ),
                              ]),
                            ]),
                          ]))
                        : X("", !0),
                    ],
                    64,
                  ))
                : X("", !0),
              K.value
                ? X("", !0)
                : (p(),
                  Ae(
                    It,
                    {
                      key: e.page,
                      labels: [e.page === "此刻" ? "日程" : e.page],
                    },
                    null,
                    8,
                    ["labels"],
                  )),
            ],
            64,
          )
        )
      );
    },
  }),
  gf = {
    viewBox: "0 0 100 100",
    width: "84",
    height: "84",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": "2.5",
    "stroke-linejoin": "round",
    "aria-hidden": "true",
  },
  yf = { key: 0, d: "M50 28v56M34 18l8 26 8-9 8 9 8-26" },
  bf = nt({
    __name: "GarmentArt",
    props: { type: {} },
    setup(e) {
      return (t, s) => (
        p(),
        g("svg", gf, [
          e.type === "鞋子"
            ? (p(),
              g(
                O,
                { key: 0 },
                [
                  s[0] ||
                    (s[0] = o(
                      "path",
                      {
                        d: "M18 55l15-25 16 11 8 17 25 8v13H16V62z",
                        fill: "currentColor",
                        opacity: ".3",
                      },
                      null,
                      -1,
                    )),
                  s[1] ||
                    (s[1] = o(
                      "path",
                      {
                        d: "M18 55l15-25 16 11 8 17 25 8v13H16V62zM17 72h65M39 47l12-4M43 54l12-4",
                      },
                      null,
                      -1,
                    )),
                ],
                64,
              ))
            : e.type === "下衣"
              ? (p(),
                g(
                  O,
                  { key: 1 },
                  [
                    s[2] ||
                      (s[2] = o(
                        "path",
                        {
                          d: "M28 15h44l7 70H55L50 47 45 85H21z",
                          fill: "currentColor",
                          opacity: ".25",
                        },
                        null,
                        -1,
                      )),
                    s[3] ||
                      (s[3] = o(
                        "path",
                        {
                          d: "M28 15h44l7 70H55L50 47 45 85H21zM28 25h44M50 25v22",
                        },
                        null,
                        -1,
                      )),
                  ],
                  64,
                ))
              : e.type === "配饰"
                ? (p(),
                  g(
                    O,
                    { key: 2 },
                    [
                      s[4] ||
                        (s[4] = o(
                          "circle",
                          { cx: "50", cy: "50", r: "21" },
                          null,
                          -1,
                        )),
                      s[5] ||
                        (s[5] = o(
                          "path",
                          { d: "M39 30V10h22v20M39 70v20h22V70M50 36v16l10 7" },
                          null,
                          -1,
                        )),
                    ],
                    64,
                  ))
                : e.type === "袜子"
                  ? (p(),
                    g(
                      O,
                      { key: 3 },
                      [
                        s[6] ||
                          (s[6] = o(
                            "path",
                            {
                              d: "M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13z",
                              fill: "currentColor",
                              opacity: ".25",
                            },
                            null,
                            -1,
                          )),
                        s[7] ||
                          (s[7] = o(
                            "path",
                            {
                              d: "M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13zM37 25h28M26 64l22 15",
                            },
                            null,
                            -1,
                          )),
                      ],
                      64,
                    ))
                  : e.type === "内裤"
                    ? (p(),
                      g(
                        O,
                        { key: 4 },
                        [
                          s[8] ||
                            (s[8] = o(
                              "path",
                              {
                                d: "M18 28h64l-15 40H33z",
                                fill: "currentColor",
                                opacity: ".25",
                              },
                              null,
                              -1,
                            )),
                          s[9] ||
                            (s[9] = o(
                              "path",
                              {
                                d: "M18 28h64l-15 40H33zM20 37h60M22 42l22 26M78 42L56 68",
                              },
                              null,
                              -1,
                            )),
                        ],
                        64,
                      ))
                    : (p(),
                      g(
                        O,
                        { key: 5 },
                        [
                          s[10] ||
                            (s[10] = o(
                              "path",
                              {
                                d: "M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z",
                                fill: "currentColor",
                                opacity: ".25",
                              },
                              null,
                              -1,
                            )),
                          s[11] ||
                            (s[11] = o(
                              "path",
                              {
                                d: "M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z",
                              },
                              null,
                              -1,
                            )),
                          e.type === "外套" ? (p(), g("path", yf)) : X("", !0),
                        ],
                        64,
                      )),
        ])
      );
    },
  }),
  _f = { class: "tabs" },
  kf = ["onClick"],
  wf = { class: "toolbar" },
  $f = { class: "card slot-picker scroll-lock-list", "data-scroll-lock": "" },
  Cf = { key: 0, class: "muted", role: "status" },
  xf = { class: "slot-accordion" },
  Sf = { class: "slot-row scroll-lock-list", "data-scroll-lock": "" },
  Tf = ["disabled", "title", "onClick"],
  Mf = { key: 0 },
  Ef = { class: "card" },
  Af = { class: "slot-summary" },
  If = { key: 0, class: "inline-error" },
  Of = { key: 1, class: "muted" },
  Vf = ["disabled"],
  Pf = { class: "card" },
  Lf = { class: "tag-list" },
  Uf = { class: "pill" },
  Nf = ["onClick"],
  Ff = ["onClick"],
  Rf = ["placeholder"],
  Df = ["disabled"],
  jf = { class: "row" },
  Hf = ["onSubmit"],
  Bf = ["disabled"],
  Kf = { key: 1 },
  Wf = { class: "pill" },
  qf = { class: "scroll-lock-list", "data-scroll-lock": "" },
  zf = ["onClick"],
  Gf = ["onClick"],
  Jf = ["onClick"],
  Yf = ["onClick"],
  Zf = ["disabled", "onClick"],
  Qf = { key: 0, class: "slot-summary" },
  Xf = { key: 0, class: "muted" },
  ep = { class: "toolbar" },
  tp = { class: "garment-grid" },
  sp = { class: "row" },
  np = { class: "scroll-lock-list", "data-scroll-lock": "" },
  lp = ["onClick"],
  op = ["onClick"],
  ip = { key: 0, class: "muted" },
  rp = { key: 0, class: "inline-error" },
  ap = { class: "row" },
  up = ["value"],
  cp = { class: "row" },
  dp = ["disabled"],
  fp = ["aria-label"],
  pp = { class: "row" },
  mp = nt({
    __name: "WardrobePage",
    setup(e) {
      const t = G(""),
        s = G(null),
        n = ds(["衣柜", "换装"]),
        l = _e(() =>
          W.value.outfits.some(
            (j) => j.name === a.value.trim() && j.id !== f.value,
          ),
        ),
        i = El,
        r = G(W.value.outfitTags[0] ?? ""),
        a = G(""),
        u = G(JSON.parse(JSON.stringify(W.value.wearing))),
        f = G(null),
        c = G(null),
        d = G(""),
        C = G(""),
        w = G(""),
        A = G(""),
        B = G(null),
        te = G(""),
        se = G(null);
      function ne(j, T, x) {
        se.value = { title: j, message: T, run: x };
      }
      function ue(j) {
        const T = W.value,
          x = te.value.trim();
        if (!(!x || n.value)) {
          if (T.outfits.some((R) => R.id !== j.id && R.name === x)) {
            Ml("方案名称已存在，请换一个名称");
            return;
          }
          ((t.value = "rename"),
            Je("衣柜", x, () => {
              ((j.name = x), (B.value = null));
            }));
        }
      }
      (it(
        () => W.value.garments.map((j) => j.id).join(","),
        () => {
          u.value = Rt(W.value, u.value);
        },
      ),
        it(
          () => W.value.id,
          () => {
            ((r.value = W.value.outfitTags[0] ?? ""),
              (u.value = {}),
              (a.value = ""),
              (f.value = null));
          },
        ));
      const S = G(null),
        I = _e(() =>
          W.value.garments.filter(
            (j) =>
              (!w.value || j.type === w.value) &&
              (!A.value || j.seasons.includes(A.value)),
          ),
        );
      function K() {
        if (n.value || l.value || !a.value.trim() || !r.value) return;
        const j = W.value,
          T = {
            id: f.value ?? Date.now(),
            name: a.value.trim(),
            tag: r.value,
            items: Rt(j, u.value),
          };
        ((t.value = "save"),
          Je("衣柜", T.name, () => {
            T.items = Rt(j, T.items);
            const x = j.outfits.findIndex((R) => R.id === T.id);
            (x < 0 ? j.outfits.push(T) : (j.outfits[x] = T),
              (a.value = ""),
              (u.value = {}),
              (f.value = null));
          }));
      }
      function le(j, T, x = !1) {
        if (!r.value) return;
        const R = u.value[j] || [];
        u.value[j] = x
          ? R.includes(T)
            ? R.filter((ye) => ye !== T)
            : [...R, T]
          : R.includes(T)
            ? []
            : [T];
      }
      function H(j) {
        ((f.value = j.id),
          (a.value = j.name),
          (r.value = j.tag),
          (u.value = JSON.parse(JSON.stringify(j.items))),
          (i.value = "为她搭配"));
      }
      function q(j) {
        return At.map(
          (T) =>
            T +
            "：" +
            ((j.items[T] || [])
              .map((x) => {
                var R;
                return (R = W.value.garments.find((ye) => ye.id === x)) == null
                  ? void 0
                  : R.name;
              })
              .filter(Boolean)
              .join("、") || "未选"),
        ).join(" / ");
      }
      function V(j) {
        if (n.value) return;
        const T = W.value;
        ((t.value = "wear"),
          (s.value = j.id),
          Je("换装", j.name, () => {
            var x;
            ((T.wearing = Rt(T, j.items)),
              (T.outfit =
                ((x = T.garments.find((R) => {
                  var ye;
                  return (ye = j.items.上衣) == null
                    ? void 0
                    : ye.includes(R.id);
                })) == null
                  ? void 0
                  : x.name) || j.name));
          }));
      }
      function U() {
        var R;
        if (n.value || !((R = S.value) != null && R.name.trim())) return;
        const j = W.value,
          T = JSON.parse(JSON.stringify(S.value)),
          x = j.garments.some((ye) => ye.id === T.id);
        ((t.value = "garment"),
          Je("衣柜", T.name, () => {
            const ye = j.garments.findIndex((F) => F.id === T.id);
            (ye < 0 ? x || j.garments.push(T) : (j.garments[ye] = T),
              (j.wearing = Rt(j, j.wearing)),
              j.outfits.forEach((F) => (F.items = Rt(j, F.items))),
              (S.value = null));
          }));
      }
      function _(j) {
        const T = W.value,
          x = T.outfits
            .filter((R) => Object.values(R.items).some((ye) => ye.includes(j)))
            .map((R) => "「" + R.name + "」");
        ne(
          "删除衣服",
          x.length
            ? "以下方案引用了这件衣服：" +
                x.join("、") +
                "。确认后将从这些方案和当前穿着中移除。"
            : "确定删除这件衣服吗？",
          () => pc(T, j),
        );
      }
      function ee() {
        const j = d.value.trim(),
          T = W.value;
        if (!(!j || T.outfitTags.includes(j))) {
          if (C.value) {
            const x = C.value;
            ((T.outfitTags = T.outfitTags.map((R) => (R === x ? j : R))),
              T.outfits.forEach((R) => {
                R.tag === x && (R.tag = j);
              }),
              r.value === x && (r.value = j));
          } else T.outfitTags.push(j);
          ((d.value = ""), (C.value = ""));
        }
      }
      function he(j) {
        ne("删除标签", "删除「" + j + "」？方案将保留。", () => {
          const T = W.value;
          ((T.outfitTags = T.outfitTags.filter((x) => x !== j)),
            T.outfits.forEach((x) => {
              x.tag === j && (x.tag = "");
            }),
            r.value === j && (r.value = ""));
        });
      }
      return (j, T) => (
        p(),
        g(
          O,
          null,
          [
            o("div", _f, [
              (p(),
              g(
                O,
                null,
                oe(["为她搭配", "方案库", "全部衣服"], (x) =>
                  o(
                    "button",
                    {
                      class: ge({ active: M(i) === x }),
                      onClick: (R) => (i.value = x),
                    },
                    v(x === "为她搭配" && M(W).id === 1 ? "为他搭配" : x),
                    11,
                    kf,
                  ),
                ),
                64,
              )),
            ]),
            M(i) === "为她搭配"
              ? (p(),
                g(
                  O,
                  { key: 0 },
                  [
                    o("div", wf, [
                      o("label", null, [
                        T[20] || (T[20] = Z("先选标签 ", -1)),
                        pe(
                          o(
                            "select",
                            {
                              "onUpdate:modelValue":
                                T[0] || (T[0] = (x) => (r.value = x)),
                            },
                            [
                              T[19] ||
                                (T[19] = o(
                                  "option",
                                  { value: "" },
                                  "请选择",
                                  -1,
                                )),
                              (p(!0),
                              g(
                                O,
                                null,
                                oe(
                                  M(W).outfitTags,
                                  (x) => (p(), g("option", null, v(x), 1)),
                                ),
                                256,
                              )),
                            ],
                            512,
                          ),
                          [[Et, r.value]],
                        ),
                      ]),
                      T[21] ||
                        (T[21] = o(
                          "small",
                          null,
                          "标签可在方案库维护 · 每个部位选一件，配饰可多选",
                          -1,
                        )),
                    ]),
                    o("div", $f, [
                      r.value ? X("", !0) : (p(), g("p", Cf, "请先选择标签")),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          M(At),
                          (x) => (
                            p(),
                            g("details", xf, [
                              o("summary", null, [
                                Z(v(x) + " ", 1),
                                o(
                                  "small",
                                  null,
                                  v(
                                    (u.value[x] || [])
                                      .map((R) => {
                                        var ye;
                                        return (ye = M(W).garments.find(
                                          (F) => F.id === R,
                                        )) == null
                                          ? void 0
                                          : ye.name;
                                      })
                                      .join("、") || "未选择",
                                  ),
                                  1,
                                ),
                              ]),
                              o("div", Sf, [
                                (p(!0),
                                g(
                                  O,
                                  null,
                                  oe(
                                    M(W).garments.filter((R) => R.type === x),
                                    (R) => {
                                      var ye;
                                      return (
                                        p(),
                                        g(
                                          "button",
                                          {
                                            disabled: !r.value,
                                            title: r.value
                                              ? void 0
                                              : "请先选择标签",
                                            class: ge([
                                              "choice",
                                              {
                                                chosen:
                                                  (ye = u.value[x]) == null
                                                    ? void 0
                                                    : ye.includes(R.id),
                                              },
                                            ]),
                                            onClick: (F) =>
                                              le(x, R.id, x === "配饰"),
                                          },
                                          v(R.name),
                                          11,
                                          Tf,
                                        )
                                      );
                                    },
                                  ),
                                  256,
                                )),
                                M(W).garments.some((R) => R.type === x)
                                  ? X("", !0)
                                  : (p(), g("small", Mf, "暂无衣服")),
                              ]),
                            ])
                          ),
                        ),
                        256,
                      )),
                    ]),
                    o("div", Ef, [
                      T[22] || (T[22] = o("h3", null, "当前搭配", -1)),
                      o(
                        "p",
                        Af,
                        v(q({ name: a.value, tag: r.value, items: u.value })),
                        1,
                      ),
                    ]),
                    l.value
                      ? (p(), g("p", If, "方案名称已存在，请换一个名称。"))
                      : X("", !0),
                    a.value.trim()
                      ? X("", !0)
                      : (p(), g("p", Of, "请填写方案名称后保存。")),
                    o(
                      "form",
                      { class: "input-row", onSubmit: Ve(K, ["prevent"]) },
                      [
                        pe(
                          o(
                            "input",
                            {
                              "onUpdate:modelValue":
                                T[1] || (T[1] = (x) => (a.value = x)),
                              placeholder: "方案名称",
                              "aria-label": "方案名称",
                            },
                            null,
                            512,
                          ),
                          [[$e, a.value]],
                        ),
                        o(
                          "button",
                          {
                            class: "button primary",
                            disabled:
                              M(n) || l.value || !r.value || !a.value.trim(),
                          },
                          v(
                            M(n) && t.value === "save"
                              ? "处理中…"
                              : f.value
                                ? "保存修改"
                                : "保存为方案",
                          ),
                          9,
                          Vf,
                        ),
                        o(
                          "button",
                          {
                            type: "button",
                            class: "button",
                            onClick:
                              T[2] ||
                              (T[2] = (x) => {
                                ((f.value = null),
                                  (u.value = {}),
                                  (a.value = ""));
                              }),
                          },
                          "重新搭配",
                        ),
                      ],
                      32,
                    ),
                  ],
                  64,
                ))
              : M(i) === "方案库"
                ? (p(),
                  g(
                    O,
                    { key: 1 },
                    [
                      o("div", Pf, [
                        T[23] ||
                          (T[23] = o(
                            "div",
                            { class: "row" },
                            [
                              o("b", null, "标签管理"),
                              o("small", null, "删除标签保留方案"),
                            ],
                            -1,
                          )),
                        o("div", Lf, [
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(
                              M(W).outfitTags,
                              (x) => (
                                p(),
                                g("span", Uf, [
                                  Z(v(x) + " ", 1),
                                  o(
                                    "button",
                                    {
                                      onClick: (R) => {
                                        ((C.value = x), (d.value = x));
                                      },
                                    },
                                    "改名",
                                    8,
                                    Nf,
                                  ),
                                  o(
                                    "button",
                                    { onClick: (R) => he(x) },
                                    "删除",
                                    8,
                                    Ff,
                                  ),
                                ])
                              ),
                            ),
                            256,
                          )),
                        ]),
                        o(
                          "form",
                          { class: "input-row", onSubmit: Ve(ee, ["prevent"]) },
                          [
                            pe(
                              o(
                                "input",
                                {
                                  "onUpdate:modelValue":
                                    T[3] || (T[3] = (x) => (d.value = x)),
                                  placeholder: C.value
                                    ? "新标签名"
                                    : "添加标签",
                                  "aria-label": "标签名称",
                                },
                                null,
                                8,
                                Rf,
                              ),
                              [[$e, d.value]],
                            ),
                            o(
                              "button",
                              { class: "button", disabled: !d.value.trim() },
                              v(C.value ? "保存标签" : "添加"),
                              9,
                              Df,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          M(W).outfits,
                          (x) => (
                            p(),
                            g("article", { key: x.id, class: "card compact" }, [
                              o("div", jf, [
                                B.value === x.id
                                  ? (p(),
                                    g(
                                      "form",
                                      {
                                        key: 0,
                                        class: "input-row",
                                        onSubmit: Ve((R) => ue(x), ["prevent"]),
                                      },
                                      [
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                T[4] ||
                                                (T[4] = (R) => (te.value = R)),
                                              "aria-label": "方案新名称",
                                              autofocus: "",
                                            },
                                            null,
                                            512,
                                          ),
                                          [[$e, te.value]],
                                        ),
                                        o(
                                          "button",
                                          {
                                            class: "button",
                                            disabled: M(n) || !te.value.trim(),
                                          },
                                          v(
                                            M(n) && t.value === "rename"
                                              ? "处理中…"
                                              : "保存",
                                          ),
                                          9,
                                          Bf,
                                        ),
                                        o(
                                          "button",
                                          {
                                            type: "button",
                                            class: "text-button",
                                            onClick:
                                              T[5] ||
                                              (T[5] = (R) => (B.value = null)),
                                          },
                                          "取消",
                                        ),
                                      ],
                                      40,
                                      Hf,
                                    ))
                                  : (p(), g("b", Kf, v(x.name), 1)),
                                o("span", Wf, v(x.tag || "未分类"), 1),
                                T[25] ||
                                  (T[25] = o(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                o(
                                  "details",
                                  {
                                    class: "actions-menu",
                                    onClick:
                                      T[6] ||
                                      (T[6] = (R) =>
                                        R.target.closest("button") &&
                                        R.currentTarget.removeAttribute(
                                          "open",
                                        )),
                                  },
                                  [
                                    T[24] ||
                                      (T[24] = o(
                                        "summary",
                                        { "aria-label": "方案操作" },
                                        "⋯",
                                        -1,
                                      )),
                                    o("div", qf, [
                                      o(
                                        "button",
                                        {
                                          onClick: (R) =>
                                            (c.value =
                                              c.value === x.id ? null : x.id),
                                        },
                                        "预览",
                                        8,
                                        zf,
                                      ),
                                      o(
                                        "button",
                                        { onClick: (R) => H(x) },
                                        "编辑搭配",
                                        8,
                                        Gf,
                                      ),
                                      o(
                                        "button",
                                        {
                                          onClick: (R) => {
                                            ((B.value = x.id),
                                              (te.value = x.name));
                                          },
                                        },
                                        "重命名",
                                        8,
                                        Jf,
                                      ),
                                      o(
                                        "button",
                                        {
                                          onClick: (R) =>
                                            ne(
                                              "删除方案",
                                              "确定删除「" + x.name + "」？",
                                              () =>
                                                (M(W).outfits = M(
                                                  W,
                                                ).outfits.filter(
                                                  (ye) => ye.id !== x.id,
                                                )),
                                            ),
                                        },
                                        "删除",
                                        8,
                                        Yf,
                                      ),
                                    ]),
                                  ],
                                ),
                                o(
                                  "button",
                                  {
                                    class: "button",
                                    disabled: M(n),
                                    onClick: (R) => V(x),
                                  },
                                  v(
                                    M(n) &&
                                      t.value === "wear" &&
                                      s.value === x.id
                                      ? "处理中…"
                                      : M(W).id === 1
                                        ? "请他换上"
                                        : "请她换上",
                                  ),
                                  9,
                                  Zf,
                                ),
                              ]),
                              c.value === x.id
                                ? (p(), g("p", Qf, v(q(x)), 1))
                                : X("", !0),
                            ])
                          ),
                        ),
                        128,
                      )),
                      M(W).outfits.length
                        ? X("", !0)
                        : (p(), g("p", Xf, "还没有方案，去搭配一套吧。")),
                    ],
                    64,
                  ))
                : (p(),
                  g(
                    O,
                    { key: 2 },
                    [
                      o("div", ep, [
                        pe(
                          o(
                            "select",
                            {
                              "onUpdate:modelValue":
                                T[7] || (T[7] = (x) => (w.value = x)),
                              "aria-label": "按类型筛选",
                            },
                            [
                              T[26] ||
                                (T[26] = o(
                                  "option",
                                  { value: "" },
                                  "全部类型",
                                  -1,
                                )),
                              (p(!0),
                              g(
                                O,
                                null,
                                oe(
                                  M(At),
                                  (x) => (p(), g("option", null, v(x), 1)),
                                ),
                                256,
                              )),
                            ],
                            512,
                          ),
                          [[Et, w.value]],
                        ),
                        pe(
                          o(
                            "select",
                            {
                              "onUpdate:modelValue":
                                T[8] || (T[8] = (x) => (A.value = x)),
                              "aria-label": "按季节筛选",
                            },
                            [
                              T[27] ||
                                (T[27] = o(
                                  "option",
                                  { value: "" },
                                  "全部季节",
                                  -1,
                                )),
                              (p(),
                              g(
                                O,
                                null,
                                oe(["春", "夏", "秋", "冬"], (x) =>
                                  o("option", null, v(x), 1),
                                ),
                                64,
                              )),
                            ],
                            512,
                          ),
                          [[Et, A.value]],
                        ),
                        T[28] ||
                          (T[28] = o("span", { class: "spacer" }, null, -1)),
                        o(
                          "button",
                          {
                            class: "button",
                            onClick:
                              T[9] ||
                              (T[9] = (x) =>
                                (S.value = {
                                  id: Date.now(),
                                  name: "",
                                  type: "上衣",
                                  seasons: ["秋"],
                                })),
                          },
                          "添加衣服",
                        ),
                      ]),
                      o("div", tp, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            I.value,
                            (x) => (
                              p(),
                              g(
                                "article",
                                { key: x.id, class: "card garment-card" },
                                [
                                  o(
                                    "div",
                                    {
                                      class: ge([
                                        "garment-thumb",
                                        "garment-type-" + x.type,
                                      ]),
                                    },
                                    [
                                      me(bf, { type: x.type }, null, 8, [
                                        "type",
                                      ]),
                                      o("small", null, v(x.type), 1),
                                    ],
                                    2,
                                  ),
                                  o("div", sp, [
                                    o("div", null, [
                                      o("h3", null, v(x.name), 1),
                                      o(
                                        "small",
                                        null,
                                        v(x.seasons.join(" / ")),
                                        1,
                                      ),
                                    ]),
                                    T[30] ||
                                      (T[30] = o(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    o(
                                      "details",
                                      {
                                        class: "actions-menu",
                                        onClick:
                                          T[10] ||
                                          (T[10] = (R) =>
                                            R.target.closest("button") &&
                                            R.currentTarget.removeAttribute(
                                              "open",
                                            )),
                                      },
                                      [
                                        T[29] ||
                                          (T[29] = o(
                                            "summary",
                                            { "aria-label": "衣服操作" },
                                            "⋯",
                                            -1,
                                          )),
                                        o("div", np, [
                                          o(
                                            "button",
                                            {
                                              onClick: (R) =>
                                                (S.value = JSON.parse(
                                                  JSON.stringify(x),
                                                )),
                                            },
                                            "编辑",
                                            8,
                                            lp,
                                          ),
                                          o(
                                            "button",
                                            { onClick: (R) => _(x.id) },
                                            "删除",
                                            8,
                                            op,
                                          ),
                                        ]),
                                      ],
                                    ),
                                  ]),
                                ],
                              )
                            ),
                          ),
                          128,
                        )),
                        I.value.length
                          ? X("", !0)
                          : (p(), g("p", ip, "没有符合条件的衣服。")),
                      ]),
                    ],
                    64,
                  )),
            S.value
              ? (p(),
                g(
                  "div",
                  {
                    key: 3,
                    class: "drawer-backdrop",
                    onClick:
                      T[15] || (T[15] = Ve((x) => (S.value = null), ["self"])),
                  },
                  [
                    o(
                      "form",
                      {
                        "data-testid": "garment-drawer",
                        class: "detail-drawer form-grid",
                        role: "dialog",
                        "aria-modal": "true",
                        "aria-label": "衣服",
                        onSubmit: Ve(U, ["prevent"]),
                      },
                      [
                        me(It, { labels: ["衣柜"] }),
                        o(
                          "h2",
                          null,
                          v(
                            M(W).garments.some((x) => {
                              var R;
                              return (
                                x.id === ((R = S.value) == null ? void 0 : R.id)
                              );
                            })
                              ? "编辑衣服"
                              : "添加衣服",
                          ),
                          1,
                        ),
                        o("label", null, [
                          T[31] || (T[31] = Z("名称", -1)),
                          pe(
                            o(
                              "input",
                              {
                                "onUpdate:modelValue":
                                  T[11] || (T[11] = (x) => (S.value.name = x)),
                              },
                              null,
                              512,
                            ),
                            [[$e, S.value.name]],
                          ),
                          S.value.name.trim()
                            ? X("", !0)
                            : (p(), g("small", rp, "请填写衣服名称。")),
                        ]),
                        o("label", null, [
                          T[32] || (T[32] = Z("类型", -1)),
                          pe(
                            o(
                              "select",
                              {
                                "onUpdate:modelValue":
                                  T[12] || (T[12] = (x) => (S.value.type = x)),
                              },
                              [
                                (p(!0),
                                g(
                                  O,
                                  null,
                                  oe(
                                    M(At),
                                    (x) => (p(), g("option", null, v(x), 1)),
                                  ),
                                  256,
                                )),
                              ],
                              512,
                            ),
                            [[Et, S.value.type]],
                          ),
                        ]),
                        o("div", ap, [
                          (p(),
                          g(
                            O,
                            null,
                            oe(["春", "夏", "秋", "冬"], (x) =>
                              o("label", null, [
                                pe(
                                  o(
                                    "input",
                                    {
                                      "onUpdate:modelValue":
                                        T[13] ||
                                        (T[13] = (R) => (S.value.seasons = R)),
                                      type: "checkbox",
                                      value: x,
                                    },
                                    null,
                                    8,
                                    up,
                                  ),
                                  [[ss, S.value.seasons]],
                                ),
                                Z(v(x), 1),
                              ]),
                            ),
                            64,
                          )),
                        ]),
                        o("div", cp, [
                          o(
                            "button",
                            {
                              class: "button primary",
                              disabled: M(n) || !S.value.name.trim(),
                            },
                            v(
                              M(n) && t.value === "garment"
                                ? "处理中…"
                                : "保存衣服",
                            ),
                            9,
                            dp,
                          ),
                          o(
                            "button",
                            {
                              type: "button",
                              class: "button",
                              onClick:
                                T[14] || (T[14] = (x) => (S.value = null)),
                            },
                            "取消",
                          ),
                        ]),
                      ],
                      32,
                    ),
                  ],
                ))
              : X("", !0),
            se.value
              ? (p(),
                g(
                  "div",
                  {
                    key: 4,
                    class: "modal-backdrop",
                    onClick:
                      T[18] || (T[18] = Ve((x) => (se.value = null), ["self"])),
                  },
                  [
                    o(
                      "section",
                      {
                        class: "card confirm-dialog",
                        role: "alertdialog",
                        "aria-modal": "true",
                        "aria-label": se.value.title,
                      },
                      [
                        o("h2", null, v(se.value.title), 1),
                        o("p", null, v(se.value.message), 1),
                        o("div", pp, [
                          o(
                            "button",
                            {
                              class: "button",
                              onClick:
                                T[16] || (T[16] = (x) => (se.value = null)),
                            },
                            "取消",
                          ),
                          o(
                            "button",
                            {
                              class: "button primary",
                              onClick:
                                T[17] ||
                                (T[17] = (x) => {
                                  (se.value.run(), (se.value = null));
                                }),
                            },
                            "确认删除",
                          ),
                        ]),
                      ],
                      8,
                      fp,
                    ),
                  ],
                ))
              : X("", !0),
            S.value
              ? X("", !0)
              : (p(), Ae(It, { key: 5, labels: ["衣柜", "换装"] })),
          ],
          64,
        )
      );
    },
  }),
  vp = { class: "world-list" },
  hp = { class: "row" },
  gp = { key: 0, class: "pill" },
  yp = { class: "muted" },
  bp = { class: "row" },
  _p = ["onClick"],
  kp = ["onClick"],
  wp = ["onClick"],
  $p = { class: "row" },
  Cp = { class: "card world-header" },
  xp = { class: "row" },
  Sp = { class: "toolbar" },
  Tp = ["disabled"],
  Mp = {
    key: 0,
    class: "card empty-state",
    "data-testid": "world-empty-sites",
  },
  Ep = { class: "row" },
  Ap = ["onClick"],
  Ip = { key: 0, class: "muted" },
  Op = { class: "place-grid" },
  Vp = { class: "row" },
  Pp = ["onClick"],
  Lp = { key: 0, class: "inline-error" },
  Up = ["onUpdate:modelValue"],
  Np = ["disabled"],
  Fp = { key: 0, class: "inline-error" },
  Rp = ["disabled"],
  Dp = { key: 0 },
  jp = ["disabled"],
  Hp = { key: 5, class: "modal-backdrop" },
  Bp = { class: "card confirm-dialog" },
  Kp = nt({
    __name: "WorldPage",
    setup(e) {
      const t = G(null),
        s = _e(() => Ut.value.find((S) => S.id === t.value)),
        n = G(!1),
        l = G(""),
        i = G(null),
        r = G(0),
        a = G(""),
        u = G(!1),
        f = ds(["世界"]),
        c = G(null),
        d = G(null),
        C = G("");
      function w() {
        c.value = { id: Date.now(), name: "", genre: "现代都市" };
      }
      const A = [
        ["alias", "别名"],
        ["floor", "楼层"],
        ["description", "描述"],
        ["atmosphere", "氛围"],
        ["furniture", "固定陈设"],
        ["layout", "布局描述"],
      ];
      function B() {
        var I;
        if (!((I = c.value) != null && I.name.trim()) || f.value) return;
        const S = { ...c.value };
        ((C.value = "world"),
          Je(
            "世界",
            S.name + " " + S.genre,
            () => {
              const K = Ut.value.find((le) => le.id === S.id);
              (K
                ? Object.assign(K, S)
                : Ut.value.push({ ...S, residents: [], sites: [] }),
                (t.value = S.id),
                (n.value = !0),
                (c.value = null));
            },
            S.name,
          ));
      }
      function te() {
        const S = s.value;
        !S ||
          f.value ||
          ((C.value = "expand"),
          Je("世界", l.value || "河岸茶馆", () => {
            (S.sites.length || S.sites.push({ name: "街区", places: [] }),
              S.sites[0].places.push({
                id: Date.now(),
                name: "河岸茶馆",
                alias: "",
                floor: "1F",
                description: "河岸边安静的休息处",
                atmosphere: "温暖、茶香",
                furniture: "茶桌、藤椅",
                layout: "临街座位",
                custom: !1,
              }));
          }));
      }
      function se(S, I) {
        ((r.value = I), (i.value = { ...S }));
      }
      function ne() {
        var K;
        if (!((K = i.value) != null && K.name.trim()) || f.value) return;
        const S = { ...i.value },
          I = s.value.sites[r.value];
        ((C.value = "place"),
          Je(
            "世界",
            Object.values(S).join(" "),
            () => {
              const le = I.places.findIndex((H) => H.id === S.id);
              (le < 0 ? I.places.push(S) : (I.places[le] = S),
                (i.value = null));
            },
            S.name,
          ));
      }
      function ue() {
        const S = s.value,
          I = a.value.trim();
        !S ||
          !I ||
          f.value ||
          ((C.value = "site"),
          Je("世界", I, () => {
            (S.sites.push({ name: I, places: [] }),
              (a.value = ""),
              (u.value = !1));
          }));
      }
      return (S, I) => {
        var K, le;
        return (
          p(),
          g(
            O,
            null,
            [
              s.value
                ? (p(),
                  g(
                    O,
                    { key: 1 },
                    [
                      o("div", $p, [
                        o(
                          "button",
                          {
                            class: "text-button",
                            onClick: I[0] || (I[0] = (H) => (t.value = null)),
                          },
                          "世界列表",
                        ),
                        o("span", null, " / " + v(s.value.name), 1),
                        I[16] ||
                          (I[16] = o("span", { class: "spacer" }, null, -1)),
                        o(
                          "button",
                          {
                            class: "button",
                            onClick: I[1] || (I[1] = (H) => (u.value = !0)),
                          },
                          "新增场地",
                        ),
                      ]),
                      o("div", Cp, [
                        o("div", xp, [
                          I[17] ||
                            (I[17] = o(
                              "span",
                              { class: "eyebrow" },
                              "A WORLD OF YOUR OWN",
                              -1,
                            )),
                          I[18] ||
                            (I[18] = o("span", { class: "spacer" }, null, -1)),
                          o(
                            "button",
                            {
                              class: "button",
                              onClick:
                                I[2] ||
                                (I[2] = (H) =>
                                  (c.value = {
                                    id: s.value.id,
                                    name: s.value.name,
                                    genre: s.value.genre,
                                  })),
                            },
                            "编辑世界信息",
                          ),
                          o(
                            "button",
                            {
                              class: "button",
                              onClick:
                                I[3] || (I[3] = (H) => (n.value = !n.value)),
                            },
                            v(n.value ? "结束编辑" : "编辑地点"),
                            1,
                          ),
                        ]),
                        o("h2", null, v(s.value.name), 1),
                        o("p", null, v(s.value.genre), 1),
                      ]),
                      o("div", Sp, [
                        pe(
                          o(
                            "input",
                            {
                              "onUpdate:modelValue":
                                I[4] || (I[4] = (H) => (l.value = H)),
                              placeholder: "补充扩展方向，例如：河岸书店",
                            },
                            null,
                            512,
                          ),
                          [[$e, l.value]],
                        ),
                        o(
                          "button",
                          { class: "button", disabled: M(f), onClick: te },
                          v(
                            M(f) && C.value === "expand"
                              ? "处理中…"
                              : "AI 扩展世界",
                          ),
                          9,
                          Tp,
                        ),
                      ]),
                      s.value.sites.length
                        ? X("", !0)
                        : (p(),
                          g("div", Mp, [
                            ...(I[19] ||
                              (I[19] = [o("p", null, "暂无场地", -1)])),
                          ])),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          s.value.sites,
                          (H, q) => (
                            p(),
                            g("section", { key: q, class: "card compact" }, [
                              o("div", Ep, [
                                o("h3", null, "场地 · " + v(H.name), 1),
                                I[20] ||
                                  (I[20] = o(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                n.value
                                  ? (p(),
                                    g(
                                      "button",
                                      {
                                        key: 0,
                                        class: "text-button",
                                        onClick: (V) => {
                                          ((r.value = q),
                                            (i.value = {
                                              id: Date.now(),
                                              name: "",
                                              alias: "",
                                              floor: "1F",
                                              description: "",
                                              atmosphere: "",
                                              furniture: "",
                                              layout: "",
                                              custom: !0,
                                            }));
                                        },
                                      },
                                      "添加地点",
                                      8,
                                      Ap,
                                    ))
                                  : X("", !0),
                              ]),
                              H.places.length
                                ? X("", !0)
                                : (p(), g("p", Ip, "暂无地点")),
                              o("div", Op, [
                                (p(!0),
                                g(
                                  O,
                                  null,
                                  oe(
                                    H.places,
                                    (V) => (
                                      p(),
                                      g(
                                        "article",
                                        { key: V.id, class: "place" },
                                        [
                                          o("div", Vp, [
                                            o(
                                              "b",
                                              null,
                                              v(V.alias || V.name),
                                              1,
                                            ),
                                            o("small", null, v(V.floor), 1),
                                          ]),
                                          o(
                                            "p",
                                            null,
                                            v(V.description) +
                                              " · " +
                                              v(V.atmosphere),
                                            1,
                                          ),
                                          o(
                                            "small",
                                            null,
                                            "固定陈设：" + v(V.furniture),
                                            1,
                                          ),
                                          o(
                                            "small",
                                            null,
                                            "布局：" + v(V.layout),
                                            1,
                                          ),
                                          n.value
                                            ? (p(),
                                              g(
                                                "button",
                                                {
                                                  key: 0,
                                                  class: "button edit-place",
                                                  onClick: (U) => se(V, q),
                                                },
                                                "编辑地点",
                                                8,
                                                Pp,
                                              ))
                                            : X("", !0),
                                        ],
                                      )
                                    ),
                                  ),
                                  128,
                                )),
                              ]),
                            ])
                          ),
                        ),
                        128,
                      )),
                    ],
                    64,
                  ))
                : (p(),
                  g(
                    O,
                    { key: 0 },
                    [
                      o("div", { class: "row" }, [
                        I[14] ||
                          (I[14] = o(
                            "div",
                            null,
                            [
                              o("h2", null, "世界列表"),
                              o(
                                "p",
                                { class: "muted" },
                                "每个人，都在自己的风景里生活。",
                              ),
                            ],
                            -1,
                          )),
                        I[15] ||
                          (I[15] = o("span", { class: "spacer" }, null, -1)),
                        o(
                          "button",
                          {
                            class: "button",
                            "data-testid": "world-add",
                            onClick: w,
                          },
                          "添加世界",
                        ),
                      ]),
                      o("div", vp, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            M(Ut),
                            (H) => (
                              p(),
                              g(
                                "article",
                                {
                                  key: H.id,
                                  class: ge([
                                    "card",
                                    { current: H.residents.includes(M(W).id) },
                                  ]),
                                },
                                [
                                  o("div", hp, [
                                    o("h2", null, v(H.name), 1),
                                    H.residents.includes(M(W).id)
                                      ? (p(), g("span", gp, "当前"))
                                      : X("", !0),
                                  ]),
                                  o(
                                    "p",
                                    null,
                                    v(H.genre) +
                                      " · " +
                                      v(H.sites.length) +
                                      " 处场地",
                                    1,
                                  ),
                                  o(
                                    "p",
                                    yp,
                                    "居住在这里：" +
                                      v(
                                        H.residents
                                          .map((q) => {
                                            var V;
                                            return (V = M(et)[q]) == null
                                              ? void 0
                                              : V.name;
                                          })
                                          .join("、") || "暂无角色",
                                      ),
                                    1,
                                  ),
                                  o("div", bp, [
                                    o(
                                      "button",
                                      {
                                        class: "button primary",
                                        onClick: (q) => {
                                          ((t.value = H.id), (n.value = !1));
                                        },
                                      },
                                      "进入",
                                      8,
                                      _p,
                                    ),
                                    o(
                                      "button",
                                      {
                                        class: "text-button",
                                        onClick: (q) => {
                                          ((t.value = H.id), (n.value = !0));
                                        },
                                      },
                                      "编辑",
                                      8,
                                      kp,
                                    ),
                                    o(
                                      "button",
                                      {
                                        class: "text-button",
                                        onClick: (q) => (d.value = H.id),
                                      },
                                      "删除",
                                      8,
                                      wp,
                                    ),
                                  ]),
                                ],
                                2,
                              )
                            ),
                          ),
                          128,
                        )),
                      ]),
                    ],
                    64,
                  )),
              i.value
                ? (p(),
                  Ae(
                    Vs,
                    {
                      key: 2,
                      title:
                        (K = s.value) != null &&
                        K.sites.some((H) =>
                          H.places.some((q) => {
                            var V;
                            return (
                              q.id === ((V = i.value) == null ? void 0 : V.id)
                            );
                          }),
                        )
                          ? "编辑地点"
                          : "添加地点",
                      "test-id": "place-drawer",
                      onClose: I[6] || (I[6] = (H) => (i.value = null)),
                    },
                    {
                      default: Ht(() => [
                        o(
                          "form",
                          { class: "form-grid", onSubmit: Ve(ne, ["prevent"]) },
                          [
                            me(It, { labels: ["世界"] }),
                            o("label", null, [
                              I[21] || (I[21] = Z("地点名称", -1)),
                              pe(
                                o(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      I[5] ||
                                      (I[5] = (H) => (i.value.name = H)),
                                  },
                                  null,
                                  512,
                                ),
                                [[$e, i.value.name]],
                              ),
                              i.value.name.trim()
                                ? X("", !0)
                                : (p(), g("small", Lp, "请填写地点名称。")),
                            ]),
                            (p(),
                            g(
                              O,
                              null,
                              oe(A, ([H, q]) =>
                                o("label", { key: H }, [
                                  Z(v(q), 1),
                                  pe(
                                    o(
                                      "input",
                                      {
                                        "onUpdate:modelValue": (V) =>
                                          (i.value[H] = V),
                                      },
                                      null,
                                      8,
                                      Up,
                                    ),
                                    [[$e, i.value[H]]],
                                  ),
                                ]),
                              ),
                              64,
                            )),
                            o(
                              "button",
                              {
                                class: "button primary",
                                disabled: M(f) || !i.value.name.trim(),
                              },
                              v(
                                M(f) && C.value === "place"
                                  ? "处理中…"
                                  : "保存地点",
                              ),
                              9,
                              Np,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      _: 1,
                    },
                    8,
                    ["title"],
                  ))
                : X("", !0),
              c.value
                ? (p(),
                  Ae(
                    Vs,
                    {
                      key: 3,
                      title: M(Ut).some((H) => {
                        var q;
                        return H.id === ((q = c.value) == null ? void 0 : q.id);
                      })
                        ? "编辑世界"
                        : "添加世界",
                      "test-id": "world-drawer",
                      onClose: I[9] || (I[9] = (H) => (c.value = null)),
                    },
                    {
                      default: Ht(() => [
                        o(
                          "form",
                          { class: "form-grid", onSubmit: Ve(B, ["prevent"]) },
                          [
                            me(It, { labels: ["世界"] }),
                            o("label", null, [
                              I[22] || (I[22] = Z("世界名称", -1)),
                              pe(
                                o(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      I[7] ||
                                      (I[7] = (H) => (c.value.name = H)),
                                    "data-testid": "world-name",
                                  },
                                  null,
                                  512,
                                ),
                                [[$e, c.value.name]],
                              ),
                              c.value.name.trim()
                                ? X("", !0)
                                : (p(), g("small", Fp, "请填写世界名称。")),
                            ]),
                            o("label", null, [
                              I[23] || (I[23] = Z("世界类型", -1)),
                              pe(
                                o(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      I[8] ||
                                      (I[8] = (H) => (c.value.genre = H)),
                                    "data-testid": "world-genre",
                                  },
                                  null,
                                  512,
                                ),
                                [[$e, c.value.genre]],
                              ),
                            ]),
                            o(
                              "button",
                              {
                                class: "button primary",
                                disabled: M(f) || !c.value.name.trim(),
                                "data-testid": "world-save",
                              },
                              v(
                                M(f) && C.value === "world"
                                  ? "处理中…"
                                  : "保存世界",
                              ),
                              9,
                              Rp,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      _: 1,
                    },
                    8,
                    ["title"],
                  ))
                : X("", !0),
              u.value
                ? (p(),
                  Ae(
                    Vs,
                    {
                      key: 4,
                      title: "添加场地",
                      "test-id": "site-drawer",
                      onClose: I[11] || (I[11] = (H) => (u.value = !1)),
                    },
                    {
                      default: Ht(() => [
                        o(
                          "form",
                          { class: "form-grid", onSubmit: Ve(ue, ["prevent"]) },
                          [
                            me(It, { labels: ["世界"] }),
                            o("label", null, [
                              I[24] || (I[24] = Z("名称", -1)),
                              pe(
                                o(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      I[10] || (I[10] = (H) => (a.value = H)),
                                  },
                                  null,
                                  512,
                                ),
                                [[$e, a.value]],
                              ),
                              a.value.trim()
                                ? X("", !0)
                                : (p(), g("small", Dp, "请填写场地名称。")),
                            ]),
                            o(
                              "button",
                              {
                                class: "button primary",
                                disabled: M(f) || !a.value.trim(),
                              },
                              v(
                                M(f) && C.value === "site"
                                  ? "处理中…"
                                  : "保存场地",
                              ),
                              9,
                              jp,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      _: 1,
                    },
                  ))
                : X("", !0),
              d.value !== null
                ? (p(),
                  g("div", Hp, [
                    o("section", Bp, [
                      I[25] || (I[25] = o("h2", null, "删除世界？", -1)),
                      o(
                        "p",
                        null,
                        "「" +
                          v(
                            (le = M(Ut).find((H) => H.id === d.value)) == null
                              ? void 0
                              : le.name,
                          ) +
                          "」及其中的场地将被移除。",
                        1,
                      ),
                      o(
                        "button",
                        {
                          class: "button",
                          onClick: I[12] || (I[12] = (H) => (d.value = null)),
                        },
                        "取消",
                      ),
                      o(
                        "button",
                        {
                          class: "button primary",
                          onClick:
                            I[13] ||
                            (I[13] = (H) => {
                              ((Ut.value = M(Ut).filter(
                                (q) => q.id !== d.value,
                              )),
                                (d.value = null));
                            }),
                        },
                        "确认删除",
                      ),
                    ]),
                  ]))
                : X("", !0),
              !i.value && !c.value && !u.value
                ? (p(), Ae(It, { key: 6, labels: ["世界"] }))
                : X("", !0),
            ],
            64,
          )
        );
      };
    },
  }),
  Wp = { class: "row" },
  qp = { class: "avatar small" },
  zp = { class: "post-art" },
  Gp = { class: "row" },
  Jp = ["onClick"],
  Yp = ["data-testid", "onClick"],
  Zp = { key: 0, class: "comments" },
  Qp = { class: "avatar small" },
  Xp = ["onSubmit"],
  e1 = ["onUpdate:modelValue", "data-testid", "onKeydown"],
  t1 = ["disabled"],
  s1 = { class: "muted" },
  n1 = { class: "people-grid" },
  l1 = ["onClick"],
  o1 = { class: "avatar" },
  i1 = { key: 0, class: "card empty-state" },
  r1 = ["aria-label"],
  a1 = { class: "muted" },
  u1 = { class: "media-grid" },
  c1 = { class: "card media-tile" },
  d1 = ["onClick"],
  f1 = { key: 0, class: "audio-control" },
  p1 = ["onClick"],
  m1 = ["value"],
  v1 = {
    "data-testid": "media-preview-drawer",
    class: "card media-modal",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": "媒体预览",
  },
  h1 = { class: "row" },
  g1 = { key: 1, class: "card empty-state" },
  y1 = { class: "row" },
  b1 = { class: "pill" },
  _1 = ["onClick"],
  k1 = { key: 0, class: "pill current-badge" },
  w1 = ["onClick"],
  $1 = { key: 0, class: "inline-profile" },
  C1 = { key: 0 },
  x1 = { key: 4, class: "card placeholder" },
  S1 = ["onClick"],
  T1 = { class: "pill" },
  M1 = ["data-testid", "onClick"],
  E1 = { key: 0, class: "muted" },
  A1 = nt({
    __name: "BrowsePage",
    props: { page: {} },
    setup(e) {
      const t = ds(["动态"]),
        s = G(null),
        n = G({}),
        l = setInterval(() => {
          if (s.value === null) return;
          const V = s.value;
          ((n.value[V] = (n.value[V] || 0) + 2),
            n.value[V] >= 100 && ((n.value[V] = 0), (s.value = null)));
        }, 500);
      Ks(() => clearInterval(l));
      const i = _e(() =>
        [...Zi.value, ...vs.value].filter((V) => V.status === "error"),
      );
      function r(V) {
        ((Xi.value = "运行诊断"),
          (al.value = V),
          (location.hash = encodeURIComponent("管理中心")));
      }
      function a(V) {
        ((cs.value = V), (location.hash = encodeURIComponent("聊天")));
      }
      const u = e,
        f = G(""),
        c = G({
          status: "",
          tag: "",
          scene: "",
          type: "",
          who: u.page === "媒体" ? W.value.name : "",
        }),
        d = G(null),
        C = G({}),
        w = G({}),
        A = _e(() => [
          { key: "status", label: "状态", options: ["活跃", "暂未联系"] },
          { key: "tag", label: "标签", options: ["日常", "工作", "旧友"] },
          {
            key: "scene",
            label: "场景",
            options: W.value.sites.map((V) => V.name),
          },
        ]),
        B = _e(() => [
          { key: "type", label: "类型", options: ["图片", "语音", "视频"] },
          { key: "tag", label: "标签", options: ["日常", "风景", "创作"] },
          { key: "who", label: "角色", options: et.value.map((V) => V.name) },
        ]),
        te = [
          {
            key: "tag",
            label: "标签",
            options: ["温柔", "创作", "沉稳", "城市", "自然"],
          },
        ];
      function se() {
        ((f.value = ""),
          (c.value = { status: "", tag: "", scene: "", type: "", who: "" }));
      }
      const ne = G(null);
      function ue(V, U) {
        V.key === "Enter" &&
          !V.shiftKey &&
          !V.isComposing &&
          (V.preventDefault(), q(U));
      }
      const S = (V) => V.toLowerCase().includes(f.value.trim().toLowerCase()),
        I = _e(() =>
          W.value.people.filter(
            (V) =>
              S(V.name + " " + V.identity) &&
              (!c.value.status || V.status === c.value.status) &&
              (!c.value.tag || V.tag === c.value.tag) &&
              (!c.value.scene || V.location === c.value.scene),
          ),
        ),
        K = _e(() => W.value.people.find((V) => V.id === d.value)),
        le = _e(() =>
          et.value.filter(
            (V) =>
              S(V.name + " " + V.job + " " + V.bio) &&
              (!c.value.tag || V.tags.includes(c.value.tag)),
          ),
        ),
        H = _e(() =>
          Po.filter(
            (V) =>
              S(V.title + " " + V.character) &&
              (!c.value.type || V.type === c.value.type) &&
              (!c.value.who || V.character === c.value.who) &&
              (!c.value.tag || V.tag === c.value.tag),
          ),
        );
      function q(V) {
        var ee;
        const U = (ee = C.value[V]) == null ? void 0 : ee.trim();
        if (!U || t.value) return;
        ne.value = V;
        const _ = W.value.posts[V];
        (Je("动态", U, () =>
          _.comments.push({ name: "我", text: U, time: "刚刚" }),
        ),
          (C.value[V] = ""));
      }
      return (V, U) =>
        e.page === "动态"
          ? (p(),
            g(
              O,
              { key: 0 },
              [
                U[13] ||
                  (U[13] = o(
                    "p",
                    { class: "muted" },
                    "生活的切片，留一点位置给彼此的回应。",
                    -1,
                  )),
                (p(!0),
                g(
                  O,
                  null,
                  oe(M(W).posts, (_, ee) => {
                    var he;
                    return (
                      p(),
                      g("article", { key: ee, class: "card post compact" }, [
                        o("div", Wp, [
                          o("span", qp, v(M(W).initial), 1),
                          o("b", null, v(M(W).name), 1),
                          o("small", null, v(_.time), 1),
                        ]),
                        o("p", null, v(_.text), 1),
                        o("div", zp, [
                          me(M(Tl), { size: 36 }),
                          U[10] ||
                            (U[10] = o("span", null, "把日子过成一首小诗", -1)),
                        ]),
                        o("div", Gp, [
                          o(
                            "button",
                            {
                              class: "text-button",
                              onClick: (j) => {
                                ((_.liked = !_.liked),
                                  (_.likes += _.liked ? 1 : -1));
                              },
                            },
                            v(_.liked ? "♥" : "♡") + " " + v(_.likes) + " 喜欢",
                            9,
                            Jp,
                          ),
                          o(
                            "button",
                            {
                              "data-testid": "comment-toggle-" + ee,
                              class: "text-button",
                              onClick: (j) => (w.value[ee] = !w.value[ee]),
                            },
                            "评论 " +
                              v(_.comments.length) +
                              " " +
                              v(w.value[ee] ? "收起" : "展开"),
                            9,
                            Yp,
                          ),
                        ]),
                        w.value[ee]
                          ? (p(),
                            g("div", Zp, [
                              (p(!0),
                              g(
                                O,
                                null,
                                oe(
                                  _.comments,
                                  (j, T) => (
                                    p(),
                                    g("p", { key: T }, [
                                      o("span", Qp, v(j.name.slice(0, 1)), 1),
                                      o("span", null, [
                                        o("b", null, v(j.name), 1),
                                        U[11] || (U[11] = Z()),
                                        o("small", null, v(j.time), 1),
                                        U[12] ||
                                          (U[12] = o("br", null, null, -1)),
                                        Z(v(j.text), 1),
                                      ]),
                                    ])
                                  ),
                                ),
                                128,
                              )),
                              o(
                                "form",
                                {
                                  class: "input-row",
                                  onSubmit: Ve((j) => q(ee), ["prevent"]),
                                },
                                [
                                  pe(
                                    o(
                                      "textarea",
                                      {
                                        "onUpdate:modelValue": (j) =>
                                          (C.value[ee] = j),
                                        placeholder: "留下一句回应…",
                                        "aria-label": "评论内容",
                                        rows: "2",
                                        "data-testid": "comment-input-" + ee,
                                        onKeydown: (j) => ue(j, ee),
                                      },
                                      null,
                                      40,
                                      e1,
                                    ),
                                    [[$e, C.value[ee]]],
                                  ),
                                  o(
                                    "button",
                                    {
                                      class: "button",
                                      disabled:
                                        M(t) ||
                                        !(
                                          (he = C.value[ee]) != null &&
                                          he.trim()
                                        ),
                                    },
                                    v(
                                      M(t) && ne.value === ee
                                        ? "处理中…"
                                        : "发表评论",
                                    ),
                                    9,
                                    t1,
                                  ),
                                ],
                                40,
                                Xp,
                              ),
                            ]))
                          : X("", !0),
                      ])
                    );
                  }),
                  128,
                )),
                me(It, { labels: ["动态"] }),
              ],
              64,
            ))
          : e.page === "人物关系"
            ? (p(),
              g(
                O,
                { key: 1 },
                [
                  me(
                    Os,
                    {
                      search: f.value,
                      "onUpdate:search": U[0] || (U[0] = (_) => (f.value = _)),
                      filters: c.value,
                      "onUpdate:filters": U[1] || (U[1] = (_) => (c.value = _)),
                      groups: A.value,
                      "test-id": "people-filters",
                      placeholder: "搜索姓名 / 身份",
                    },
                    null,
                    8,
                    ["search", "filters", "groups"],
                  ),
                  o("small", s1, v(I.value.length) + " 位相识", 1),
                  o("div", n1, [
                    (p(!0),
                    g(
                      O,
                      null,
                      oe(
                        I.value,
                        (_) => (
                          p(),
                          g(
                            "button",
                            {
                              key: _.id,
                              class: "card person-card",
                              onClick: (ee) => (d.value = _.id),
                            },
                            [
                              o("span", o1, v(_.name.slice(0, 1)), 1),
                              o("h3", null, v(_.name), 1),
                              o(
                                "p",
                                null,
                                v(_.identity) + " · " + v(_.relation),
                                1,
                              ),
                              o(
                                "small",
                                null,
                                v(_.location) + " · " + v(_.tag),
                                1,
                              ),
                            ],
                            8,
                            l1,
                          )
                        ),
                      ),
                      128,
                    )),
                  ]),
                  I.value.length
                    ? X("", !0)
                    : (p(),
                      g("div", i1, [
                        U[14] || (U[14] = o("p", null, "暂无匹配人物", -1)),
                        o(
                          "button",
                          { class: "button", onClick: se },
                          "清除筛选",
                        ),
                      ])),
                  K.value
                    ? (p(),
                      g(
                        "div",
                        {
                          key: 1,
                          "data-testid": "person-drawer-overlay",
                          class: "drawer-backdrop",
                          onClick:
                            U[3] ||
                            (U[3] = Ve((_) => (d.value = null), ["self"])),
                        },
                        [
                          o(
                            "aside",
                            {
                              "data-testid": "person-drawer",
                              class: "detail-drawer",
                              role: "dialog",
                              "aria-modal": "true",
                              "aria-label": K.value.name,
                            },
                            [
                              o(
                                "button",
                                {
                                  class: "button",
                                  onClick:
                                    U[2] || (U[2] = (_) => (d.value = null)),
                                },
                                "关闭",
                              ),
                              o("h2", null, v(K.value.name), 1),
                              o(
                                "p",
                                null,
                                v(K.value.identity) +
                                  " · " +
                                  v(K.value.type) +
                                  " · " +
                                  v(K.value.status),
                                1,
                              ),
                              o("p", null, v(K.value.bio), 1),
                              o("p", null, [
                                Z("关系：" + v(K.value.relation), 1),
                                U[15] || (U[15] = o("br", null, null, -1)),
                                Z("位置：" + v(K.value.location), 1),
                                U[16] || (U[16] = o("br", null, null, -1)),
                                Z("日程：" + v(K.value.schedule), 1),
                                U[17] || (U[17] = o("br", null, null, -1)),
                                Z("标签：" + v(K.value.tag), 1),
                              ]),
                            ],
                            8,
                            r1,
                          ),
                        ],
                      ))
                    : X("", !0),
                ],
                64,
              ))
            : e.page === "媒体"
              ? (p(),
                g(
                  O,
                  { key: 2 },
                  [
                    me(
                      Os,
                      {
                        search: f.value,
                        "onUpdate:search":
                          U[4] || (U[4] = (_) => (f.value = _)),
                        filters: c.value,
                        "onUpdate:filters":
                          U[5] || (U[5] = (_) => (c.value = _)),
                        groups: B.value,
                        "test-id": "media-filters",
                        placeholder: "搜索媒体 / 角色",
                      },
                      null,
                      8,
                      ["search", "filters", "groups"],
                    ),
                    o("small", a1, v(H.value.length) + " 项", 1),
                    o("div", u1, [
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          H.value,
                          (_) => (
                            p(),
                            g("article", c1, [
                              o(
                                "button",
                                {
                                  class: "media-open",
                                  onClick: (ee) => (d.value = _.id),
                                },
                                [
                                  o(
                                    "div",
                                    {
                                      class: ge([
                                        "media-preview",
                                        "media-art-" + (_.id % 6),
                                        "media-type-" + _.type,
                                        _.type === "图片" ? "photo" : "",
                                      ]),
                                    },
                                    [
                                      (p(),
                                      Ae(
                                        as(
                                          _.type === "图片"
                                            ? M($o)
                                            : _.type === "语音"
                                              ? M(So)
                                              : M(xo),
                                        ),
                                        { size: 30 },
                                      )),
                                    ],
                                    2,
                                  ),
                                  o("b", null, v(_.title), 1),
                                  o(
                                    "small",
                                    null,
                                    v(_.character) +
                                      " · " +
                                      v(_.type) +
                                      " · " +
                                      v(_.time),
                                    1,
                                  ),
                                ],
                                8,
                                d1,
                              ),
                              _.type === "语音"
                                ? (p(),
                                  g("div", f1, [
                                    o(
                                      "button",
                                      {
                                        class: "button",
                                        onClick: (ee) =>
                                          (s.value =
                                            s.value === _.id ? null : _.id),
                                      },
                                      v(s.value === _.id ? "暂停" : "播放"),
                                      9,
                                      p1,
                                    ),
                                    o(
                                      "progress",
                                      { value: n.value[_.id] || 0, max: "100" },
                                      null,
                                      8,
                                      m1,
                                    ),
                                    o(
                                      "small",
                                      null,
                                      v(
                                        Math.floor((n.value[_.id] || 0) * 0.3),
                                      ) + " / 30 秒",
                                      1,
                                    ),
                                  ]))
                                : X("", !0),
                            ])
                          ),
                        ),
                        256,
                      )),
                    ]),
                    d.value !== null
                      ? (p(),
                        g(
                          "div",
                          {
                            key: 0,
                            class: "modal-backdrop",
                            onClick:
                              U[7] ||
                              (U[7] = Ve((_) => (d.value = null), ["self"])),
                          },
                          [
                            o("section", v1, [
                              (p(!0),
                              g(
                                O,
                                null,
                                oe(
                                  M(Po).filter((_) => _.id === d.value),
                                  (_) => (
                                    p(),
                                    g(
                                      O,
                                      null,
                                      [
                                        o("div", h1, [
                                          o("h2", null, v(_.title), 1),
                                          U[18] ||
                                            (U[18] = o(
                                              "span",
                                              { class: "spacer" },
                                              null,
                                              -1,
                                            )),
                                          o(
                                            "button",
                                            {
                                              class: "button",
                                              onClick:
                                                U[6] ||
                                                (U[6] = (ee) =>
                                                  (d.value = null)),
                                            },
                                            "关闭预览",
                                          ),
                                        ]),
                                        o(
                                          "div",
                                          {
                                            class: ge([
                                              "media-preview",
                                              "large-media",
                                              "media-art-" + (_.id % 6),
                                              "media-type-" + _.type,
                                            ]),
                                          },
                                          [
                                            (p(),
                                            Ae(
                                              as(
                                                _.type === "图片"
                                                  ? M($o)
                                                  : _.type === "语音"
                                                    ? M(So)
                                                    : M(xo),
                                              ),
                                              { size: 72 },
                                            )),
                                            o(
                                              "span",
                                              null,
                                              v(
                                                _.type === "语音"
                                                  ? "▂ ▅ ▃ ▇ ▄ ▂ ▆"
                                                  : _.type === "视频"
                                                    ? "▶"
                                                    : _.title,
                                              ),
                                              1,
                                            ),
                                          ],
                                          2,
                                        ),
                                        o(
                                          "p",
                                          null,
                                          v(_.character) +
                                            " · " +
                                            v(_.time) +
                                            " · " +
                                            v(_.tag),
                                          1,
                                        ),
                                      ],
                                      64,
                                    )
                                  ),
                                ),
                                256,
                              )),
                            ]),
                          ],
                        ))
                      : X("", !0),
                    H.value.length
                      ? X("", !0)
                      : (p(),
                        g("div", g1, [
                          U[19] || (U[19] = o("p", null, "暂无匹配媒体。", -1)),
                          o(
                            "button",
                            {
                              class: "button",
                              "data-testid": "media-empty-clear",
                              onClick: se,
                            },
                            "清除筛选",
                          ),
                        ])),
                  ],
                  64,
                ))
              : e.page === "角色库"
                ? (p(),
                  g(
                    O,
                    { key: 3 },
                    [
                      me(
                        Os,
                        {
                          search: f.value,
                          "onUpdate:search":
                            U[8] || (U[8] = (_) => (f.value = _)),
                          filters: c.value,
                          "onUpdate:filters":
                            U[9] || (U[9] = (_) => (c.value = _)),
                          groups: te,
                          "test-id": "library-filters",
                          placeholder: "搜索角色 / 职业",
                        },
                        null,
                        8,
                        ["search", "filters"],
                      ),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          le.value,
                          (_) => (
                            p(),
                            g("article", { key: _.id, class: "card compact" }, [
                              o("div", y1, [
                                o(
                                  "span",
                                  {
                                    class: "avatar",
                                    style: yt({ background: _.color }),
                                  },
                                  v(_.initial),
                                  5,
                                ),
                                o("div", null, [
                                  o("b", null, v(_.name) + " · " + v(_.job), 1),
                                  o("p", null, v(_.subtitle), 1),
                                  (p(!0),
                                  g(
                                    O,
                                    null,
                                    oe(
                                      _.tags,
                                      (ee) => (p(), g("span", b1, v(ee), 1)),
                                    ),
                                    256,
                                  )),
                                ]),
                                U[20] ||
                                  (U[20] = o(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                o(
                                  "button",
                                  {
                                    class: "text-button",
                                    onClick: (ee) =>
                                      (d.value =
                                        d.value === _.id ? null : _.id),
                                  },
                                  v(d.value === _.id ? "收起详情" : "角色详情"),
                                  9,
                                  _1,
                                ),
                                M(cs) === _.id
                                  ? (p(), g("span", k1, "当前"))
                                  : (p(),
                                    g(
                                      "button",
                                      {
                                        key: 1,
                                        class: "button primary",
                                        onClick: (ee) => (cs.value = _.id),
                                      },
                                      "切换角色",
                                      8,
                                      w1,
                                    )),
                              ]),
                              d.value === _.id
                                ? (p(),
                                  g("div", $1, [
                                    o("h2", null, v(_.name), 1),
                                    o("p", null, v(_.bio), 1),
                                    o("p", null, [
                                      Z("职业：" + v(_.job), 1),
                                      U[21] ||
                                        (U[21] = o("br", null, null, -1)),
                                      Z("兴趣：" + v(_.hobby), 1),
                                      U[22] ||
                                        (U[22] = o("br", null, null, -1)),
                                      Z("心情：" + v(_.mood), 1),
                                      U[23] ||
                                        (U[23] = o("br", null, null, -1)),
                                      Z("位置：" + v(_.location), 1),
                                      U[24] ||
                                        (U[24] = o("br", null, null, -1)),
                                      Z("世界：" + v(_.world), 1),
                                    ]),
                                  ]))
                                : X("", !0),
                            ])
                          ),
                        ),
                        128,
                      )),
                      le.value.length
                        ? X("", !0)
                        : (p(), g("p", C1, "暂无匹配角色。")),
                    ],
                    64,
                  ))
                : e.page === "创作中心"
                  ? (p(), g("div", x1, "即将重新设计"))
                  : e.page === "通知与待处理"
                    ? (p(),
                      g(
                        O,
                        { key: 5 },
                        [
                          U[29] || (U[29] = o("h2", null, "未读消息", -1)),
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(M(et), (_, ee) => {
                              var he;
                              return (
                                p(),
                                g(
                                  "button",
                                  {
                                    class: "card row notification-card",
                                    onClick: (j) => a(_.id),
                                  },
                                  [
                                    o(
                                      "span",
                                      {
                                        class: "avatar",
                                        style: yt({ background: _.color }),
                                      },
                                      v(_.initial),
                                      5,
                                    ),
                                    o("div", null, [
                                      o("b", null, v(_.name) + " 在找你", 1),
                                      o(
                                        "p",
                                        null,
                                        v(
                                          (he = _.messages
                                            .filter((j) => !j.user)
                                            .at(-1)) == null
                                            ? void 0
                                            : he.text.slice(0, 60),
                                        ),
                                        1,
                                      ),
                                    ]),
                                    U[25] ||
                                      (U[25] = o(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    o("span", T1, v(ee + 1) + " 条未读", 1),
                                  ],
                                  8,
                                  S1,
                                )
                              );
                            }),
                            256,
                          )),
                          o("h2", null, [
                            U[26] || (U[26] = Z("异常检查 ", -1)),
                            o("small", null, v(i.value.length) + " 项", 1),
                          ]),
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(
                              i.value,
                              (_) => (
                                p(),
                                g(
                                  "button",
                                  {
                                    "data-testid":
                                      "notification-diagnostic-" + _.id,
                                    class: "card row notification-card",
                                    onClick: (ee) => r(_.id),
                                  },
                                  [
                                    o("b", null, v(_.label), 1),
                                    o("span", null, v(_.reason), 1),
                                    U[27] ||
                                      (U[27] = o(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    U[28] || (U[28] = Z("查看诊断 ↗", -1)),
                                  ],
                                  8,
                                  M1,
                                )
                              ),
                            ),
                            256,
                          )),
                          i.value.length
                            ? X("", !0)
                            : (p(), g("p", E1, "目前没有异常。")),
                        ],
                        64,
                      ))
                    : X("", !0);
    },
  }),
  I1 = { class: "tabs" },
  O1 = ["onClick"],
  V1 = { key: 0, class: "configuration-layout" },
  P1 = {
    class: "configuration-nav",
    "aria-label": "配置章节",
    "data-testid": "config-section-nav",
  },
  L1 = ["href", "data-testid", "onClick"],
  U1 = ["id"],
  N1 = { class: "row" },
  F1 = ["onClick"],
  R1 = { class: "form-grid" },
  D1 = ["onUpdate:modelValue"],
  j1 = ["onUpdate:modelValue"],
  H1 = ["onUpdate:modelValue"],
  B1 = ["onUpdate:modelValue"],
  K1 = { class: "check-field" },
  W1 = ["onUpdate:modelValue"],
  q1 = { class: "muted" },
  z1 = ["onClick"],
  G1 = ["id"],
  J1 = { class: "form-grid" },
  Y1 = ["onUpdate:modelValue"],
  Z1 = ["value"],
  Q1 = ["onUpdate:modelValue", "list"],
  X1 = ["id"],
  em = ["value"],
  tm = ["onUpdate:modelValue"],
  sm = ["onUpdate:modelValue"],
  nm = ["onUpdate:modelValue", "min"],
  lm = { class: "muted" },
  om = { class: "management-block" },
  im = { class: "shared-scenario" },
  rm = ["id"],
  am = { class: "form-grid" },
  um = { class: "check-field" },
  cm = ["disabled"],
  dm = { class: "form-grid" },
  fm = { class: "quiet-days" },
  pm = ["value"],
  mm = { key: 0 },
  vm = { key: 1 },
  hm = { key: 2, class: "muted" },
  gm = { class: "form-grid" },
  ym = ["onUpdate:modelValue"],
  bm = ["value", "onInput"],
  _m = ["onUpdate:modelValue", "type", "min", "max", "step", "placeholder"],
  km = ["onClick"],
  wm = { key: 0, role: "status", class: "muted" },
  $m = { class: "settings-save-bar", "data-testid": "config-save-bar" },
  Cm = { key: 0, role: "alert", class: "inline-error" },
  xm = { key: 1, class: "muted" },
  Sm = ["disabled"],
  Tm = { class: "tabs" },
  Mm = ["data-testid", "onClick"],
  Em = { class: "muted" },
  Am = { class: "diagnostic-feed", "data-testid": "diagnostic-feed" },
  Im = ["id", "data-testid"],
  Om = ["data-testid", "onClick"],
  Vm = { class: "diagnostic-meta" },
  Pm = { class: "pill" },
  Lm = { class: "diagnostic-summary" },
  Um = ["data-testid", "onClick"],
  Nm = { key: 0, class: "card empty-state" },
  Fm = { class: "detail-meta" },
  Rm = { class: "pill" },
  Dm = { key: 0, class: "inline-error" },
  jm = { class: "media-call-attributes", "data-testid": "media-call-details" },
  Hm = { class: "detail-section" },
  Bm = { class: "detail-section" },
  Km = { class: "detail-section" },
  Wm = { class: "media-call-attributes" },
  qm = { class: "detail-section", "data-testid": "media-call-output" },
  zm = { key: 0, class: "mock-image-output" },
  Gm = { key: 1, class: "muted" },
  Jm = { class: "muted" },
  Ym = { class: "detail-section" },
  Zm = { class: "detail-section" },
  Qm = { class: "detail-section" },
  Xm = { class: "secondary-diagnostics" },
  ev = { class: "quiet-days" },
  tv = ["onClick"],
  sv = { key: 0, class: "card" },
  nv = { class: "row" },
  lv = { role: "status" },
  ov = nt({
    __name: "ManagementPage",
    setup(e) {
      let t = !0;
      Ks(() => {
        t = !1;
      });
      const s = (F) => JSON.parse(JSON.stringify(F)),
        n = G(s(Oo.value)),
        l = G(s(Eo.value)),
        i = G(s(Ao.value)),
        r = G(s(Io.value)),
        a = Xi,
        u = G("模型调用"),
        f = G(""),
        c = G({ type: "", who: "", status: "" }),
        d = G(null),
        C = G(null),
        w = G(!1),
        A = G(""),
        B = G(""),
        te = G(""),
        se = {
          聊天回复: {
            prompt:
              "回应用户本次消息，使用角色已知事实与自然口吻，先回应感受，不编造共同经历。",
            summary: "已生成回应本次消息的聊天回复",
          },
          日程: {
            prompt:
              "按请求中的时间和内容更新角色日程，核对安排并保留其他约定。",
            summary: "已更新日程安排",
          },
          日记: {
            prompt: "根据角色今日经历整理第一人称日记，将内容写入当日条目。",
            summary: "已写入今日日记",
          },
          记忆: {
            prompt: "整理请求中的记忆事实，保留来源、日期、重要度与标签。",
            summary: "已保存记忆内容与属性",
          },
          动态: {
            prompt: "处理本次动态或评论内容，保留用户原意与生活语气。",
            summary: "已发布动态或评论内容",
          },
          衣柜: {
            prompt: "按请求保存衣服或搭配方案，检查名称与引用的衣服。",
            summary: "已保存衣柜资料",
          },
          换装: {
            prompt: "按请求更新角色穿着，使用衣柜中实际存在的单品。",
            summary: "已更新角色穿着",
          },
          世界: {
            prompt: "按请求整理世界或地点信息，保留已知场地与角色关系。",
            summary: "已保存世界与场地信息",
          },
        },
        ne = _e(() =>
          vs.value.map((F, $) => {
            const h = se[F.label];
            return {
              ...F,
              kind: "模型调用",
              start: F.start ?? $ * 4,
              duration: F.duration ?? 1,
              model: "通用模型",
              category: "工具执行失败",
              code: "capability_prepare_failed",
              response:
                F.response && F.response !== "请求已完成。"
                  ? F.response
                  : `${(h == null ? void 0 : h.summary) || F.label + "已完成"}：${F.input}`,
              reason: F.reason || "",
              character: F.character || "",
              input: `用途：${F.label}
角色：${F.character}
${(h == null ? void 0 : h.prompt) || ""}
本次请求：${F.input}`,
            };
          }),
        ),
        ue = _e(() => [...Zi.value, ...ne.value]),
        S = _e(() => ue.value.find((F) => F.id === d.value)),
        I = (F) =>
          F === "error" ? "失败" : F === "pending" ? "处理中" : "完成",
        K = _e(() =>
          ue.value
            .filter((F) => {
              var $, h, b;
              return (
                F.kind === u.value &&
                (!c.value.type || F.label === c.value.type) &&
                (!c.value.who || F.character === c.value.who) &&
                (!c.value.status || I(F.status) === c.value.status) &&
                [
                  F.label,
                  F.input,
                  F.character,
                  F.model,
                  F.reason,
                  ($ = F.media) == null ? void 0 : $.workflow,
                  (h = F.media) == null ? void 0 : h.negativePrompt,
                  (b = F.media) == null ? void 0 : b.output,
                ]
                  .join(" ")
                  .toLowerCase()
                  .includes(f.value.trim().toLowerCase())
              );
            })
            .sort((F, $) => $.start - F.start),
        ),
        le = _e(() => [
          {
            key: "type",
            label: "类型",
            options: [
              ...new Set([
                ...(u.value === "模型调用" ? As : hn),
                ...ue.value
                  .filter((F) => F.kind === u.value)
                  .map((F) => F.label),
              ]),
            ],
          },
          { key: "who", label: "角色", options: et.value.map((F) => F.name) },
          { key: "status", label: "状态", options: ["完成", "失败", "处理中"] },
        ]);
      function H() {
        ((f.value = ""), (c.value = { type: "", who: "", status: "" }));
      }
      function q(F) {
        ((u.value = F), H(), (d.value = null), (C.value = null));
      }
      function V(F) {
        ((d.value = F.id), (C.value = F.id));
      }
      (it(
        al,
        async (F) => {
          if (F === null) return;
          const $ = ue.value.find((h) => h.id === F);
          $ &&
            ((a.value = "运行诊断"),
            (u.value = $.kind),
            H(),
            V($),
            (al.value = null),
            await ls(),
            requestAnimationFrame(() => {
              var h;
              return (h = document.getElementById("diag-record-" + F)) == null
                ? void 0
                : h.scrollIntoView({ behavior: "smooth", block: "center" });
            }));
        },
        { immediate: !0 },
      ),
        it(a, (F) => {
          F === "配置中心" && (d.value = null);
        }));
      const U = _e(() => [
          "服务地址",
          "模型用途",
          "安静时段",
          ...n.value.map((F) => F.title),
        ]),
        _ = (F) => "config-" + F;
      function ee(F) {
        const $ = document.getElementById(_(F));
        (($ == null ? void 0 : $.tagName) === "DETAILS" &&
          $.setAttribute("open", ""),
          $ == null ||
            $.scrollIntoView({ behavior: "smooth", block: "start" }));
      }
      const he = (F) =>
          "14:" +
          String(Math.floor(F.start / 60)).padStart(2, "0") +
          ":" +
          String(F.start % 60).padStart(2, "0"),
        j = _e(() => Vo.find((F) => F.title === B.value));
      function T(F) {
        if (F.status !== "pending") {
          if (F.id > 0) {
            F.run();
            return;
          }
          ((F.status = "pending"),
            setTimeout(() => {
              var h;
              const $ = ns.value || F.input.includes("失败");
              ((F.status = $ ? "error" : "success"),
                $
                  ? (F.reason = F.input.includes("失败")
                      ? "输入包含“失败”，请修改输入后重新提交。"
                      : "模拟失败已开启，关闭后可重试。")
                  : ((F.reason = ""),
                    (F.response =
                      ((h = F.media) == null ? void 0 : h.output) ||
                      F.response)));
            }, 900));
        }
      }
      async function x(F) {
        ((te.value = F + "：测试中…"),
          await new Promise(($) => setTimeout($, 700)),
          (te.value =
            F +
            "：" +
            (ns.value
              ? "连接失败，请检查地址。"
              : "模拟测试通过（仅检查可达性）。")));
      }
      async function R() {
        if (w.value) return;
        A.value = "";
        for (const h of n.value)
          for (const b of h.fields) {
            if (b.type === "json")
              try {
                const k = JSON.parse(String(b.value));
                if (!k || typeof k != "object" || Array.isArray(k))
                  throw Error();
              } catch {
                A.value = b.label + "：请填写有效 JSON 对象。";
                return;
              }
            if (
              b.type === "number" &&
              (!Number.isFinite(+b.value) ||
                (b.min !== void 0 && +b.value < b.min) ||
                (b.max !== void 0 && +b.value > b.max))
            ) {
              A.value = b.label + "：数值超出范围。";
              return;
            }
          }
        for (const h of l.value)
          try {
            const b = new URL(h.url);
            if (
              !["http:", "https:"].includes(b.protocol) ||
              b.username ||
              b.password
            )
              throw Error();
          } catch {
            A.value = "服务地址必须是不带账号密码的 http(s) URL。";
            return;
          }
        if (i.value.some((h) => !l.value.some((b) => b.id === h.endpoint))) {
          A.value = "请为四个用途选择有效的服务地址。";
          return;
        }
        if (
          r.value.enabled &&
          (!r.value.days.length ||
            !r.value.start ||
            !r.value.end ||
            r.value.start === r.value.end)
        ) {
          A.value = "请完整设置安静时段，开始与结束时间需不同。";
          return;
        }
        const F = n.value.find((h) => h.title === "高级");
        if (
          Number(F.fields.find((h) => h.label === "阶段总预算（ms）").value) <
          Number(F.fields.find((h) => h.label === "请求超时（ms）").value)
        ) {
          A.value = "Kev 阶段总预算不能小于请求超时。";
          return;
        }
        const $ = [
          ...l.value.flatMap((h) => [h.name, h.protocol, h.url, h.secret]),
          ...i.value.map((h) => h.model),
          ...n.value.flatMap((h) => h.fields.map((b) => String(b.value))),
        ].some((h) => h.includes("失败"));
        if (
          ((w.value = !0),
          await new Promise((h) => setTimeout(h, 800)),
          (w.value = !1),
          $)
        ) {
          A.value = "输入包含“失败”，请修改后重新提交。";
          return;
        }
        if (ns.value) {
          A.value = "模拟保存失败，请关闭模拟失败后重试。";
          return;
        }
        ((Oo.value = s(
          n.value.map((h) => ({
            ...h,
            fields: h.fields.map((b) =>
              b.type === "password" ? { ...b, value: "" } : b,
            ),
          })),
        )),
          (Eo.value = s(l.value.map((h) => ({ ...h, secret: "" })))),
          (Ao.value = s(i.value)),
          (Io.value = s(r.value)),
          l.value.forEach((h) => (h.secret = "")),
          n.value.forEach((h) =>
            h.fields.forEach((b) => {
              b.type === "password" && (b.value = "");
            }),
          ),
          t && Ml("设置已保存（本地模拟）。"));
      }
      function ye() {
        l.value.push({
          id: "service-" + Date.now(),
          name: "新服务",
          protocol: "openai-compatible",
          url: "",
          secret: "",
          clearSecret: !1,
          models: [],
          status: "未配置",
        });
      }
      return (F, $) => (
        p(),
        g(
          O,
          null,
          [
            o("div", I1, [
              (p(),
              g(
                O,
                null,
                oe(["配置中心", "运行诊断"], (h) =>
                  o(
                    "button",
                    {
                      key: h,
                      class: ge({ active: M(a) === h }),
                      onClick: (b) => (a.value = h),
                    },
                    v(h),
                    11,
                    O1,
                  ),
                ),
                64,
              )),
            ]),
            M(a) === "配置中心"
              ? (p(),
                g("div", V1, [
                  o("nav", P1, [
                    (p(!0),
                    g(
                      O,
                      null,
                      oe(
                        U.value,
                        (h) => (
                          p(),
                          g(
                            "a",
                            {
                              key: h,
                              href: "#" + encodeURIComponent("管理中心"),
                              "data-testid": "config-anchor-" + h,
                              onClick: Ve((b) => ee(h), ["prevent"]),
                            },
                            v(h),
                            9,
                            L1,
                          )
                        ),
                      ),
                      128,
                    )),
                  ]),
                  o(
                    "form",
                    {
                      class: "settings-form management-form",
                      onSubmit: Ve(R, ["prevent"]),
                    },
                    [
                      o(
                        "section",
                        { id: _("服务地址"), class: "card settings-group" },
                        [
                          o("div", { class: "row" }, [
                            $[12] || ($[12] = o("h2", null, "服务地址", -1)),
                            $[13] ||
                              ($[13] = o(
                                "span",
                                { class: "spacer" },
                                null,
                                -1,
                              )),
                            o(
                              "button",
                              {
                                type: "button",
                                class: "text-button",
                                onClick: ye,
                              },
                              "添加服务",
                            ),
                          ]),
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(
                              l.value,
                              (h, b) => (
                                p(),
                                g(
                                  "div",
                                  { key: h.id, class: "management-block" },
                                  [
                                    o("div", N1, [
                                      o("b", null, v(h.name), 1),
                                      o(
                                        "small",
                                        null,
                                        v(h.status) + " · 模拟状态",
                                        1,
                                      ),
                                      $[14] ||
                                        ($[14] = o(
                                          "span",
                                          { class: "spacer" },
                                          null,
                                          -1,
                                        )),
                                      o(
                                        "button",
                                        {
                                          type: "button",
                                          class: "text-button",
                                          onClick: (k) => l.value.splice(b, 1),
                                        },
                                        "移除",
                                        8,
                                        F1,
                                      ),
                                    ]),
                                    o("div", R1, [
                                      o("label", null, [
                                        $[15] || ($[15] = Z("服务名称", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.name = k),
                                              required: "",
                                              maxlength: "128",
                                            },
                                            null,
                                            8,
                                            D1,
                                          ),
                                          [[$e, h.name]],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[16] || ($[16] = Z("接口协议", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.protocol = k),
                                              required: "",
                                            },
                                            null,
                                            8,
                                            j1,
                                          ),
                                          [[$e, h.protocol]],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[17] || ($[17] = Z("服务地址", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.url = k),
                                              required: "",
                                              type: "url",
                                            },
                                            null,
                                            8,
                                            H1,
                                          ),
                                          [[$e, h.url]],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[18] || ($[18] = Z("访问密钥", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.secret = k),
                                              type: "password",
                                              placeholder:
                                                "留空保留原值 · 只写不读",
                                              autocomplete: "new-password",
                                            },
                                            null,
                                            8,
                                            B1,
                                          ),
                                          [[$e, h.secret]],
                                        ),
                                      ]),
                                      o("label", K1, [
                                        $[19] ||
                                          ($[19] = Z("清除已存密钥", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.clearSecret = k),
                                              type: "checkbox",
                                            },
                                            null,
                                            8,
                                            W1,
                                          ),
                                          [[ss, h.clearSecret]],
                                        ),
                                      ]),
                                    ]),
                                    o(
                                      "p",
                                      q1,
                                      "可用模型：" +
                                        v(
                                          h.models.join("、") ||
                                            "暂未读取，可在用途配置中手填模型 ID",
                                        ),
                                      1,
                                    ),
                                    o(
                                      "button",
                                      {
                                        type: "button",
                                        class: "text-button",
                                        onClick: (k) => x(h.name),
                                      },
                                      "测试连接",
                                      8,
                                      z1,
                                    ),
                                  ],
                                )
                              ),
                            ),
                            128,
                          )),
                          $[20] ||
                            ($[20] = o(
                              "p",
                              { class: "muted" },
                              "服务地址修改并保存后，其用途绑定需要重新确认；本原型保留选择供检查。",
                              -1,
                            )),
                        ],
                        8,
                        U1,
                      ),
                      o(
                        "section",
                        { id: _("模型用途"), class: "card settings-group" },
                        [
                          $[27] ||
                            ($[27] = o("h2", null, "各用途对应的模型配置", -1)),
                          $[28] ||
                            ($[28] = o(
                              "p",
                              { class: "muted" },
                              "只有以下四个用途可独立选模型。默认安全余量 512 token，输出预留采用用途预算。",
                              -1,
                            )),
                          (p(!0),
                          g(
                            O,
                            null,
                            oe(i.value, (h) => {
                              var b;
                              return (
                                p(),
                                g(
                                  "details",
                                  {
                                    key: h.name,
                                    class: "management-block",
                                    open: "",
                                  },
                                  [
                                    o("summary", null, v(h.name), 1),
                                    o("div", J1, [
                                      o("label", null, [
                                        $[21] || ($[21] = Z("使用服务", -1)),
                                        pe(
                                          o(
                                            "select",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.endpoint = k),
                                              required: "",
                                            },
                                            [
                                              (p(!0),
                                              g(
                                                O,
                                                null,
                                                oe(
                                                  l.value,
                                                  (k) => (
                                                    p(),
                                                    g(
                                                      "option",
                                                      { value: k.id },
                                                      v(k.name),
                                                      9,
                                                      Z1,
                                                    )
                                                  ),
                                                ),
                                                256,
                                              )),
                                            ],
                                            8,
                                            Y1,
                                          ),
                                          [[Et, h.endpoint]],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[22] || ($[22] = Z("模型 ID", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.model = k),
                                              list: "models-" + h.role,
                                              required: "",
                                            },
                                            null,
                                            8,
                                            Q1,
                                          ),
                                          [[$e, h.model]],
                                        ),
                                        o(
                                          "datalist",
                                          { id: "models-" + h.role },
                                          [
                                            (p(!0),
                                            g(
                                              O,
                                              null,
                                              oe(
                                                (b = l.value.find(
                                                  (k) => k.id === h.endpoint,
                                                )) == null
                                                  ? void 0
                                                  : b.models,
                                                (k) => (
                                                  p(),
                                                  g(
                                                    "option",
                                                    { value: k },
                                                    null,
                                                    8,
                                                    em,
                                                  )
                                                ),
                                              ),
                                              256,
                                            )),
                                          ],
                                          8,
                                          X1,
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[23] || ($[23] = Z("Token 预算", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.tokens = k),
                                              type: "number",
                                              min: "1",
                                              required: "",
                                            },
                                            null,
                                            8,
                                            tm,
                                          ),
                                          [
                                            [
                                              $e,
                                              h.tokens,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[24] || ($[24] = Z("超时（秒）", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.timeout = k),
                                              type: "number",
                                              min: "1",
                                              required: "",
                                            },
                                            null,
                                            8,
                                            sm,
                                          ),
                                          [
                                            [
                                              $e,
                                              h.timeout,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      o("label", null, [
                                        $[25] ||
                                          ($[25] = Z("上下文窗口（可选）", -1)),
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (k) =>
                                                (h.window = k),
                                              type: "number",
                                              min: h.tokens + 512,
                                            },
                                            null,
                                            8,
                                            nm,
                                          ),
                                          [
                                            [
                                              $e,
                                              h.window,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      o(
                                        "p",
                                        lm,
                                        "可用输入预算：" +
                                          v(
                                            h.window
                                              ? Math.max(
                                                  0,
                                                  h.window - h.tokens - 512,
                                                )
                                              : "自动",
                                          ) +
                                          " token",
                                        1,
                                      ),
                                    ]),
                                  ],
                                )
                              );
                            }),
                            128,
                          )),
                          o("details", om, [
                            o(
                              "summary",
                              null,
                              "共用通用模型的场景 · " + v(M(To).length) + " 项",
                              1,
                            ),
                            (p(!0),
                            g(
                              O,
                              null,
                              oe(
                                M(To),
                                (h) => (
                                  p(),
                                  g("div", im, [
                                    o("span", null, v(h), 1),
                                    $[26] ||
                                      ($[26] = o(
                                        "small",
                                        null,
                                        "使用：通用模型",
                                        -1,
                                      )),
                                  ])
                                ),
                              ),
                              256,
                            )),
                          ]),
                        ],
                        8,
                        G1,
                      ),
                      o(
                        "section",
                        { id: _("安静时段"), class: "card settings-group" },
                        [
                          $[37] || ($[37] = o("h2", null, "安静时段", -1)),
                          o("div", am, [
                            o("label", um, [
                              $[29] || ($[29] = Z("启用安静时段", -1)),
                              pe(
                                o(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      $[0] ||
                                      ($[0] = (h) => (r.value.enabled = h)),
                                    class: "switch",
                                    type: "checkbox",
                                  },
                                  null,
                                  512,
                                ),
                                [[ss, r.value.enabled]],
                              ),
                            ]),
                            o("label", null, [
                              $[31] || ($[31] = Z("适用角色", -1)),
                              pe(
                                o(
                                  "select",
                                  {
                                    "onUpdate:modelValue":
                                      $[1] ||
                                      ($[1] = (h) => (r.value.character = h)),
                                  },
                                  [
                                    $[30] ||
                                      ($[30] = o(
                                        "option",
                                        null,
                                        "全部角色",
                                        -1,
                                      )),
                                    (p(!0),
                                    g(
                                      O,
                                      null,
                                      oe(
                                        M(et),
                                        (h) => (
                                          p(),
                                          g("option", null, v(h.name), 1)
                                        ),
                                      ),
                                      256,
                                    )),
                                  ],
                                  512,
                                ),
                                [[Et, r.value.character]],
                              ),
                            ]),
                          ]),
                          o(
                            "fieldset",
                            { disabled: !r.value.enabled },
                            [
                              o("div", dm, [
                                o("label", null, [
                                  $[32] || ($[32] = Z("开始时间", -1)),
                                  pe(
                                    o(
                                      "input",
                                      {
                                        "onUpdate:modelValue":
                                          $[2] ||
                                          ($[2] = (h) => (r.value.start = h)),
                                        type: "time",
                                      },
                                      null,
                                      512,
                                    ),
                                    [[$e, r.value.start]],
                                  ),
                                ]),
                                o("label", null, [
                                  $[33] || ($[33] = Z("结束时间", -1)),
                                  pe(
                                    o(
                                      "input",
                                      {
                                        "onUpdate:modelValue":
                                          $[3] ||
                                          ($[3] = (h) => (r.value.end = h)),
                                        type: "time",
                                      },
                                      null,
                                      512,
                                    ),
                                    [[$e, r.value.end]],
                                  ),
                                ]),
                                o("label", null, [
                                  $[35] || ($[35] = Z("行为", -1)),
                                  pe(
                                    o(
                                      "select",
                                      {
                                        "onUpdate:modelValue":
                                          $[4] ||
                                          ($[4] = (h) =>
                                            (r.value.behavior = h)),
                                      },
                                      [
                                        ...($[34] ||
                                          ($[34] = [
                                            o(
                                              "option",
                                              null,
                                              "暂停主动联系",
                                              -1,
                                            ),
                                            o(
                                              "option",
                                              null,
                                              "延后至安静时段结束",
                                              -1,
                                            ),
                                          ])),
                                      ],
                                      512,
                                    ),
                                    [[Et, r.value.behavior]],
                                  ),
                                ]),
                              ]),
                              $[36] ||
                                ($[36] = o(
                                  "p",
                                  null,
                                  "生效日期（Asia/Shanghai，支持跨夜）",
                                  -1,
                                )),
                              o("div", fm, [
                                (p(!0),
                                g(
                                  O,
                                  null,
                                  oe(
                                    M(Yi),
                                    (h) => (
                                      p(),
                                      g("label", null, [
                                        pe(
                                          o(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                $[5] ||
                                                ($[5] = (b) =>
                                                  (r.value.days = b)),
                                              type: "checkbox",
                                              value: h,
                                            },
                                            null,
                                            8,
                                            pm,
                                          ),
                                          [[ss, r.value.days]],
                                        ),
                                        Z(v(h), 1),
                                      ])
                                    ),
                                  ),
                                  256,
                                )),
                              ]),
                            ],
                            8,
                            cm,
                          ),
                        ],
                        8,
                        rm,
                      ),
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(
                          n.value,
                          (h) => (
                            p(),
                            Ae(
                              as(h.title === "高级" ? "details" : "section"),
                              {
                                key: h.title,
                                id: _(h.title),
                                class: "card settings-group",
                              },
                              {
                                default: Ht(() => [
                                  h.title === "高级"
                                    ? (p(),
                                      g("summary", mm, "高级 · Kev 决策服务"))
                                    : (p(), g("h2", vm, v(h.title), 1)),
                                  h.title === "图片（ComfyUI）"
                                    ? (p(),
                                      g(
                                        "p",
                                        hm,
                                        "图片已接入 ComfyUI · 语音：未接入 · 视频：未接入",
                                      ))
                                    : X("", !0),
                                  o("div", gm, [
                                    (p(!0),
                                    g(
                                      O,
                                      null,
                                      oe(
                                        h.fields,
                                        (b) => (
                                          p(),
                                          g(
                                            "label",
                                            {
                                              key: b.label,
                                              class: ge({
                                                "check-field":
                                                  b.type === "checkbox",
                                              }),
                                            },
                                            [
                                              o("span", null, v(b.label), 1),
                                              b.type === "select"
                                                ? pe(
                                                    (p(),
                                                    g(
                                                      "select",
                                                      {
                                                        key: 0,
                                                        "onUpdate:modelValue": (
                                                          k,
                                                        ) => (b.value = k),
                                                      },
                                                      [
                                                        (p(!0),
                                                        g(
                                                          O,
                                                          null,
                                                          oe(
                                                            b.options,
                                                            (k) => (
                                                              p(),
                                                              g(
                                                                "option",
                                                                null,
                                                                v(k),
                                                                1,
                                                              )
                                                            ),
                                                          ),
                                                          256,
                                                        )),
                                                      ],
                                                      8,
                                                      ym,
                                                    )),
                                                    [[Et, b.value]],
                                                  )
                                                : b.type === "json"
                                                  ? (p(),
                                                    g(
                                                      "textarea",
                                                      {
                                                        key: 1,
                                                        value: String(b.value),
                                                        onInput: (k) =>
                                                          (b.value =
                                                            k.target.value),
                                                        rows: "3",
                                                      },
                                                      null,
                                                      40,
                                                      bm,
                                                    ))
                                                  : pe(
                                                      (p(),
                                                      g(
                                                        "input",
                                                        {
                                                          key: 2,
                                                          "onUpdate:modelValue":
                                                            (k) =>
                                                              (b.value = k),
                                                          type: b.type,
                                                          min: b.min,
                                                          max: b.max,
                                                          step:
                                                            b.type === "number"
                                                              ? "any"
                                                              : void 0,
                                                          class: ge({
                                                            switch:
                                                              b.type ===
                                                              "checkbox",
                                                          }),
                                                          placeholder:
                                                            b.type ===
                                                            "password"
                                                              ? "留空保留原值 · 只写不读"
                                                              : void 0,
                                                        },
                                                        null,
                                                        10,
                                                        _m,
                                                      )),
                                                      [[Vu, b.value]],
                                                    ),
                                            ],
                                            2,
                                          )
                                        ),
                                      ),
                                      128,
                                    )),
                                  ]),
                                  [
                                    "图片（ComfyUI）",
                                    "高级",
                                    "看图理解",
                                  ].includes(h.title)
                                    ? (p(),
                                      g(
                                        "button",
                                        {
                                          key: 3,
                                          type: "button",
                                          class: "text-button",
                                          onClick: (b) => x(h.title),
                                        },
                                        v(
                                          h.title === "看图理解"
                                            ? "测试看图（使用已保存配置）"
                                            : "测试连接",
                                        ),
                                        9,
                                        km,
                                      ))
                                    : X("", !0),
                                ]),
                                _: 2,
                              },
                              1032,
                              ["id"],
                            )
                          ),
                        ),
                        128,
                      )),
                      te.value ? (p(), g("p", wm, v(te.value), 1)) : X("", !0),
                      o("div", $m, [
                        A.value
                          ? (p(), g("p", Cm, v(A.value), 1))
                          : (p(), g("span", xm, "让相处的节奏，更合心意。")),
                        o(
                          "button",
                          {
                            class: "button primary",
                            disabled: w.value,
                            "data-testid": "config-save",
                          },
                          v(w.value ? "保存中…" : "保存设置"),
                          9,
                          Sm,
                        ),
                      ]),
                    ],
                    32,
                  ),
                ]))
              : (p(),
                g(
                  O,
                  { key: 1 },
                  [
                    $[55] ||
                      ($[55] = o(
                        "p",
                        { class: "muted" },
                        "每次回应背后的记录。模型与媒体的运行情况，都在这里。",
                        -1,
                      )),
                    o("div", Tm, [
                      (p(),
                      g(
                        O,
                        null,
                        oe(["模型调用", "媒体调用"], (h) =>
                          o(
                            "button",
                            {
                              key: h,
                              class: ge({ active: u.value === h }),
                              "data-testid": "diag-tab-" + h,
                              onClick: (b) => q(h),
                            },
                            v(h),
                            11,
                            Mm,
                          ),
                        ),
                        64,
                      )),
                    ]),
                    me(
                      Os,
                      {
                        search: f.value,
                        "onUpdate:search":
                          $[6] || ($[6] = (h) => (f.value = h)),
                        filters: c.value,
                        "onUpdate:filters":
                          $[7] || ($[7] = (h) => (c.value = h)),
                        groups: le.value,
                        "test-id": "diag-filters",
                        placeholder: "搜索调用 / 提示词 / 工作流 / 角色",
                      },
                      null,
                      8,
                      ["search", "filters", "groups"],
                    ),
                    o(
                      "small",
                      Em,
                      v(K.value.length) + " 条记录 · 最近的记录在前",
                      1,
                    ),
                    o("section", Am, [
                      (p(!0),
                      g(
                        O,
                        null,
                        oe(K.value, (h) => {
                          var b;
                          return (
                            p(),
                            g(
                              "article",
                              {
                                id: "diag-record-" + h.id,
                                key: h.id,
                                class: ge([
                                  "diagnostic-record",
                                  { highlighted: C.value === h.id },
                                ]),
                                "data-testid": "diag-record-" + h.id,
                              },
                              [
                                o(
                                  "button",
                                  {
                                    class: "diagnostic-open",
                                    "data-testid": "diag-open-" + h.id,
                                    onClick: (k) => V(h),
                                  },
                                  [
                                    o("div", Vm, [
                                      o("time", null, v(he(h)), 1),
                                      o("span", Pm, v(h.label), 1),
                                      o("span", null, v(h.character), 1),
                                      $[38] ||
                                        ($[38] = o(
                                          "span",
                                          { class: "spacer" },
                                          null,
                                          -1,
                                        )),
                                      o(
                                        "span",
                                        {
                                          class: ge(["call-status", h.status]),
                                        },
                                        v(I(h.status)),
                                        3,
                                      ),
                                      o(
                                        "small",
                                        null,
                                        v(h.duration) + " 秒",
                                        1,
                                      ),
                                    ]),
                                    o(
                                      "p",
                                      Lm,
                                      v(
                                        h.status === "error"
                                          ? h.reason
                                          : h.status === "pending"
                                            ? "正在等待服务回应"
                                            : h.kind === "媒体调用"
                                              ? (b = h.media) == null
                                                ? void 0
                                                : b.output
                                              : h.response,
                                      ),
                                      1,
                                    ),
                                  ],
                                  8,
                                  Om,
                                ),
                                h.status === "error"
                                  ? (p(),
                                    g(
                                      "button",
                                      {
                                        key: 0,
                                        class: "text-button diagnostic-retry",
                                        "data-testid": "diag-retry-" + h.id,
                                        onClick: (k) => T(h),
                                      },
                                      "重试",
                                      8,
                                      Um,
                                    ))
                                  : X("", !0),
                              ],
                              10,
                              Im,
                            )
                          );
                        }),
                        128,
                      )),
                      K.value.length
                        ? X("", !0)
                        : (p(),
                          g("div", Nm, [
                            $[39] ||
                              ($[39] = o(
                                "p",
                                null,
                                "暂无符合条件的调用。",
                                -1,
                              )),
                            o(
                              "button",
                              {
                                class: "button",
                                "data-testid": "diag-empty-clear",
                                onClick: H,
                              },
                              "清除筛选",
                            ),
                          ])),
                    ]),
                    S.value
                      ? (p(),
                        Ae(
                          Vs,
                          {
                            key: 0,
                            title: S.value.label + " · 调用详情",
                            "test-id": "diagnostic-drawer",
                            onClose: $[9] || ($[9] = (h) => (d.value = null)),
                          },
                          {
                            default: Ht(() => [
                              o("div", Fm, [
                                o("span", Rm, v(S.value.kind), 1),
                                o(
                                  "p",
                                  null,
                                  v(S.value.character) +
                                    " · " +
                                    v(he(S.value)) +
                                    " · 用时 " +
                                    v(S.value.duration) +
                                    " 秒",
                                  1,
                                ),
                                o(
                                  "span",
                                  {
                                    class: ge(["call-status", S.value.status]),
                                  },
                                  v(I(S.value.status)),
                                  3,
                                ),
                              ]),
                              S.value.status === "error"
                                ? (p(),
                                  g("p", Dm, [
                                    Z(
                                      v(S.value.category) +
                                        "：" +
                                        v(S.value.reason) +
                                        " ",
                                      1,
                                    ),
                                    o(
                                      "button",
                                      {
                                        class: "text-button",
                                        "data-testid": "diag-drawer-retry",
                                        onClick:
                                          $[8] || ($[8] = (h) => T(S.value)),
                                      },
                                      "重试",
                                    ),
                                  ]))
                                : X("", !0),
                              S.value.kind === "媒体调用" && S.value.media
                                ? (p(),
                                  g(
                                    O,
                                    { key: 1 },
                                    [
                                      o("dl", jm, [
                                        o("div", null, [
                                          $[40] ||
                                            ($[40] = o("dt", null, "角色", -1)),
                                          o(
                                            "dd",
                                            null,
                                            v(S.value.character),
                                            1,
                                          ),
                                        ]),
                                        o("div", null, [
                                          $[41] ||
                                            ($[41] = o(
                                              "dt",
                                              null,
                                              "媒体类型",
                                              -1,
                                            )),
                                          o(
                                            "dd",
                                            null,
                                            v(S.value.media.type) + " · 图片",
                                            1,
                                          ),
                                        ]),
                                        o("div", null, [
                                          $[42] ||
                                            ($[42] = o(
                                              "dt",
                                              null,
                                              "工作流",
                                              -1,
                                            )),
                                          o(
                                            "dd",
                                            null,
                                            v(S.value.media.workflow),
                                            1,
                                          ),
                                        ]),
                                      ]),
                                      o("section", Hm, [
                                        $[43] ||
                                          ($[43] = o(
                                            "h3",
                                            null,
                                            "提示词 / Prompt",
                                            -1,
                                          )),
                                        o(
                                          "pre",
                                          null,
                                          v(S.value.media.prompt),
                                          1,
                                        ),
                                      ]),
                                      o("section", Bm, [
                                        $[44] ||
                                          ($[44] = o(
                                            "h3",
                                            null,
                                            "负面提示词 / Negative prompt",
                                            -1,
                                          )),
                                        o(
                                          "pre",
                                          null,
                                          v(S.value.media.negativePrompt),
                                          1,
                                        ),
                                      ]),
                                      o("section", Km, [
                                        $[45] ||
                                          ($[45] = o(
                                            "h3",
                                            null,
                                            "生成参数",
                                            -1,
                                          )),
                                        o("dl", Wm, [
                                          (p(!0),
                                          g(
                                            O,
                                            null,
                                            oe(
                                              S.value.media.params,
                                              (h, b) => (
                                                p(),
                                                g("div", { key: b }, [
                                                  o(
                                                    "dt",
                                                    null,
                                                    v(
                                                      {
                                                        width: "宽度",
                                                        height: "高度",
                                                        steps: "步数",
                                                        cfg: "CFG",
                                                        seed: "种子",
                                                        sampler: "采样器",
                                                      }[b],
                                                    ),
                                                    1,
                                                  ),
                                                  o("dd", null, v(h), 1),
                                                ])
                                              ),
                                            ),
                                            128,
                                          )),
                                        ]),
                                      ]),
                                      o("section", qm, [
                                        $[48] ||
                                          ($[48] = o("h3", null, "输出", -1)),
                                        S.value.status === "success"
                                          ? (p(),
                                            g("div", zm, [
                                              $[46] ||
                                                ($[46] = o(
                                                  "span",
                                                  null,
                                                  "◇",
                                                  -1,
                                                )),
                                              o(
                                                "p",
                                                null,
                                                v(S.value.media.output),
                                                1,
                                              ),
                                              $[47] ||
                                                ($[47] = o(
                                                  "small",
                                                  null,
                                                  "图片占位预览 · 本地模拟",
                                                  -1,
                                                )),
                                            ]))
                                          : (p(),
                                            g(
                                              "p",
                                              Gm,
                                              v(
                                                S.value.status === "pending"
                                                  ? "等待图片工作流完成"
                                                  : "未生成输出，请重试。",
                                              ),
                                              1,
                                            )),
                                      ]),
                                    ],
                                    64,
                                  ))
                                : (p(),
                                  g(
                                    O,
                                    { key: 2 },
                                    [
                                      o("p", Jm, [
                                        Z("模型 · " + v(S.value.model), 1),
                                        S.value.tokens
                                          ? (p(),
                                            g(
                                              O,
                                              { key: 0 },
                                              [
                                                Z(
                                                  " · " +
                                                    v(S.value.tokens) +
                                                    " token",
                                                  1,
                                                ),
                                              ],
                                              64,
                                            ))
                                          : X("", !0),
                                      ]),
                                      o("section", Ym, [
                                        $[49] ||
                                          ($[49] = o(
                                            "h3",
                                            null,
                                            "请求提示词 / Prompt",
                                            -1,
                                          )),
                                        o("pre", null, v(S.value.input), 1),
                                      ]),
                                      o("section", Zm, [
                                        $[50] ||
                                          ($[50] = o(
                                            "h3",
                                            null,
                                            "响应内容",
                                            -1,
                                          )),
                                        o(
                                          "pre",
                                          null,
                                          v(
                                            S.value.status === "success"
                                              ? S.value.response
                                              : "尚未收到响应",
                                          ),
                                          1,
                                        ),
                                      ]),
                                    ],
                                    64,
                                  )),
                              o("details", Qm, [
                                $[52] ||
                                  ($[52] = o("summary", null, "技术详情", -1)),
                                o("p", null, [
                                  Z(
                                    "失败码：" +
                                      v(
                                        S.value.status === "error"
                                          ? S.value.code
                                          : "无",
                                      ),
                                    1,
                                  ),
                                  $[51] || ($[51] = o("br", null, null, -1)),
                                  Z("关联 ID：demo" + v(S.value.id), 1),
                                ]),
                              ]),
                            ]),
                            _: 1,
                          },
                          8,
                          ["title"],
                        ))
                      : X("", !0),
                    o("div", Xm, [
                      $[54] || ($[54] = o("small", null, "其它诊断", -1)),
                      o("div", ev, [
                        (p(!0),
                        g(
                          O,
                          null,
                          oe(
                            M(Vo),
                            (h) => (
                              p(),
                              g(
                                "button",
                                {
                                  class: "text-button",
                                  onClick: (b) =>
                                    (B.value =
                                      B.value === h.title ? "" : h.title),
                                },
                                v(h.title),
                                9,
                                tv,
                              )
                            ),
                          ),
                          256,
                        )),
                      ]),
                      j.value
                        ? (p(),
                          g("div", sv, [
                            o("div", nv, [
                              o("b", null, v(j.value.title) + " · 模拟记录", 1),
                              $[53] ||
                                ($[53] = o(
                                  "span",
                                  { class: "spacer" },
                                  null,
                                  -1,
                                )),
                              o(
                                "button",
                                {
                                  onClick:
                                    $[10] || ($[10] = (h) => (B.value = "")),
                                },
                                "关闭",
                              ),
                            ]),
                            (p(!0),
                            g(
                              O,
                              null,
                              oe(
                                j.value.rows,
                                (h) => (p(), g("p", null, v(h), 1)),
                              ),
                              256,
                            )),
                            ["后台任务", "待处理事项 / 重试耗尽"].includes(
                              B.value,
                            )
                              ? (p(),
                                g(
                                  "button",
                                  {
                                    key: 0,
                                    class: "text-button",
                                    onClick:
                                      $[11] || ($[11] = (h) => x("任务重试")),
                                  },
                                  "重试",
                                ))
                              : X("", !0),
                            o("p", lv, v(te.value), 1),
                          ]))
                        : X("", !0),
                    ]),
                  ],
                  64,
                )),
          ],
          64,
        )
      );
    },
  }),
  iv = (e, t) => {
    const s = e.__vccOpts || e;
    for (const [n, l] of t) s[n] = l;
    return s;
  },
  rv = iv(ov, [["__scopeId", "data-v-5bb367ad"]]),
  av = { class: "shell" },
  uv = { class: "topbar" },
  cv = { class: "brand-mark" },
  dv = { class: "failure-toggle" },
  fv = {
    key: 0,
    class: "switch-menu scroll-lock-list",
    "data-scroll-lock": "",
  },
  pv = ["onClick"],
  mv = { class: "layout" },
  vv = { class: "sidebar" },
  hv = { class: "workspace-label" },
  gv = ["aria-current"],
  yv = ["aria-expanded"],
  bv = { key: 0, class: "life-nav" },
  _v = ["data-testid", "aria-current", "onClick"],
  kv = ["data-testid", "aria-current", "onClick"],
  wv = ["data-testid", "aria-current", "onClick"],
  $v = { class: "page-heading" },
  Cv = { class: "breadcrumb" },
  xv = { key: 0, class: "heading-caption" },
  Sv = { key: 1, class: "page-content" },
  Tv = { key: 0, "data-testid": "toast", class: "toast", role: "status" },
  Mv = nt({
    __name: "App",
    setup(e) {
      const t = ["此刻", "日程", "目标", "衣柜", "日记", "记忆"],
        s = [
          ["世界", Wu],
          ["动态", Co],
          ["人物关系", sc],
          ["媒体", qu],
        ],
        n = [
          ["通知与待处理", zu],
          ["角色库", Gu],
          ["创作中心", nc],
          ["管理中心", ec],
        ],
        l = ["聊天", ...t, ...s.map((C) => C[0]), ...n.map((C) => C[0])];
      function i() {
        try {
          const C = decodeURIComponent(location.hash.slice(1));
          return l.includes(C) ? C : "聊天";
        } catch {
          return "聊天";
        }
      }
      const r = G(i()),
        a = G(!0),
        u = G(!1),
        f = G(!0);
      window.addEventListener("hashchange", () => (r.value = i()));
      const c = G();
      it(r, () => {
        (mc(),
          ls(() => {
            var C;
            return (C = c.value) == null ? void 0 : C.scrollTo({ top: 0 });
          }));
      });
      function d(C) {
        ((r.value = C), (location.hash = encodeURIComponent(C)));
      }
      return (C, w) => (
        p(),
        g("div", av, [
          o("header", uv, [
            o(
              "button",
              { class: "brand", onClick: w[0] || (w[0] = (A) => d("聊天")) },
              [
                o("span", cv, [me(M(Tl), { size: 19 })]),
                w[7] || (w[7] = Z("摇光", -1)),
                w[8] ||
                  (w[8] = o("span", { class: "brand-en" }, "FLUCTLIGHT", -1)),
              ],
            ),
            w[10] || (w[10] = o("span", { class: "top-divider" }, null, -1)),
            o(
              "button",
              {
                class: "character-switch",
                onClick: w[1] || (w[1] = (A) => (u.value = !u.value)),
              },
              [
                o(
                  "span",
                  {
                    class: "avatar small",
                    style: yt({ background: M(W).color }),
                  },
                  v(M(W).initial),
                  5,
                ),
                o("b", null, v(M(W).name), 1),
                me(M(wo), { size: 14 }),
              ],
            ),
            w[11] || (w[11] = o("span", { class: "scope" }, "角色工作区", -1)),
            w[12] || (w[12] = o("span", { class: "spacer" }, null, -1)),
            w[13] ||
              (w[13] = o(
                "span",
                { class: "local" },
                [o("i"), Z("相伴此刻")],
                -1,
              )),
            o("label", dv, [
              pe(
                o(
                  "input",
                  {
                    "data-testid": "simulate-failure",
                    "onUpdate:modelValue":
                      w[2] || (w[2] = (A) => (Re(ns) ? (ns.value = A) : null)),
                    type: "checkbox",
                  },
                  null,
                  512,
                ),
                [[ss, M(ns)]],
              ),
              w[9] || (w[9] = Z("模拟失败", -1)),
            ]),
            w[14] || (w[14] = o("span", { class: "self-avatar" }, "我", -1)),
            u.value
              ? (p(),
                g("div", fv, [
                  (p(!0),
                  g(
                    O,
                    null,
                    oe(
                      M(et),
                      (A) => (
                        p(),
                        g(
                          "button",
                          {
                            key: A.id,
                            onClick: (B) => {
                              ((cs.value = A.id), (u.value = !1));
                            },
                          },
                          [
                            o(
                              "span",
                              {
                                class: "avatar small",
                                style: yt({ background: A.color }),
                              },
                              v(A.initial),
                              5,
                            ),
                            Z(v(A.name), 1),
                          ],
                          8,
                          pv,
                        )
                      ),
                    ),
                    128,
                  )),
                ]))
              : X("", !0),
          ]),
          o("div", mv, [
            o("aside", vv, [
              o("div", hv, [
                w[15] || (w[15] = Z("角色工作区 ", -1)),
                o("span", null, v(M(W).name), 1),
              ]),
              (p(),
              g("nav", { key: "workspace-" + r.value }, [
                o(
                  "button",
                  {
                    "data-testid": "nav-聊天",
                    "aria-current": r.value === "聊天" ? "page" : void 0,
                    class: ge(["nav-item", { active: r.value === "聊天" }]),
                    onClick: w[3] || (w[3] = (A) => d("聊天")),
                  },
                  [
                    me(M(Ju), { size: 17 }),
                    w[16] || (w[16] = o("span", null, "聊天", -1)),
                  ],
                  10,
                  gv,
                ),
                o(
                  "button",
                  {
                    class: "nav-item",
                    onClick: w[4] || (w[4] = (A) => (a.value = !a.value)),
                    "aria-expanded": a.value,
                  },
                  [
                    me(M(Co), { size: 17 }),
                    w[17] || (w[17] = o("span", null, "生活", -1)),
                    me(M(wo), { size: 13 }),
                  ],
                  8,
                  yv,
                ),
                a.value
                  ? (p(),
                    g("div", bv, [
                      (p(),
                      g(
                        O,
                        null,
                        oe(t, (A) =>
                          o(
                            "button",
                            {
                              key: A,
                              "data-testid": "nav-" + A,
                              "aria-current": r.value === A ? "page" : void 0,
                              class: ge([
                                "nav-item",
                                { active: r.value === A },
                              ]),
                              onClick: (B) => d(A),
                            },
                            v(A),
                            11,
                            _v,
                          ),
                        ),
                        64,
                      )),
                    ]))
                  : X("", !0),
                (p(),
                g(
                  O,
                  null,
                  oe(s, ([A, B]) =>
                    o(
                      "button",
                      {
                        key: A,
                        "data-testid": "nav-" + A,
                        "aria-current": r.value === A ? "page" : void 0,
                        class: ge(["nav-item", { active: r.value === A }]),
                        onClick: (te) => d(A),
                      },
                      [
                        (p(), Ae(as(B), { size: 17 })),
                        o("span", null, v(A), 1),
                      ],
                      10,
                      kv,
                    ),
                  ),
                  64,
                )),
              ])),
              w[18] || (w[18] = o("hr", { class: "nav-divider" }, null, -1)),
              (p(),
              g("nav", { key: "global-" + r.value }, [
                (p(),
                g(
                  O,
                  null,
                  oe(n, ([A, B]) =>
                    o(
                      "button",
                      {
                        key: A,
                        "data-testid": "nav-" + A,
                        "aria-current": r.value === A ? "page" : void 0,
                        class: ge(["nav-item", { active: r.value === A }]),
                        onClick: (te) => d(A),
                      },
                      [
                        (p(), Ae(as(B), { size: 17 })),
                        o("span", null, v(A), 1),
                      ],
                      10,
                      wv,
                    ),
                  ),
                  64,
                )),
              ])),
              w[19] ||
                (w[19] = o(
                  "div",
                  { class: "sidebar-footer" },
                  [
                    Z("每一个世界，都有回响 "),
                    o("small", null, "Fluctlight · v6"),
                  ],
                  -1,
                )),
            ]),
            o(
              "main",
              {
                ref_key: "main",
                ref: c,
                class: ge({ "chat-main": r.value === "聊天" }),
              },
              [
                o("div", $v, [
                  o("div", null, [
                    o("div", Cv, [
                      Z(v(M(W).name) + " 的工作区 ", 1),
                      me(M(Ji), { size: 12 }),
                      Z(v(r.value), 1),
                    ]),
                    o("h1", null, [
                      Z(v(r.value), 1),
                      r.value === "聊天"
                        ? (p(), g("span", xv, "日常的片刻，与你分享。"))
                        : X("", !0),
                    ]),
                  ]),
                  r.value === "聊天"
                    ? (p(),
                      g(
                        "button",
                        {
                          key: 0,
                          class: "button",
                          onClick: w[5] || (w[5] = (A) => (f.value = !f.value)),
                        },
                        [
                          (p(), Ae(as(f.value ? M(Yu) : M(Zu)), { size: 15 })),
                          Z(v(f.value ? "收起预览" : "展开预览"), 1),
                        ],
                      ))
                    : X("", !0),
                ]),
                r.value === "聊天"
                  ? (p(),
                    Ae(
                      Rc,
                      { key: M(W).id, preview: f.value, onGo: d },
                      null,
                      8,
                      ["preview"],
                    ))
                  : (p(),
                    g("div", Sv, [
                      r.value === "衣柜"
                        ? (p(), Ae(mp, { key: "w" + M(W).id }))
                        : r.value === "世界"
                          ? (p(), Ae(Kp, { key: "world" + M(W).id }))
                          : t.includes(r.value)
                            ? (p(),
                              Ae(
                                hf,
                                { page: r.value, key: M(W).id, onGo: d },
                                null,
                                8,
                                ["page"],
                              ))
                            : r.value === "管理中心"
                              ? (p(), Ae(rv, { key: 3 }))
                              : (p(),
                                Ae(
                                  A1,
                                  { page: r.value, key: r.value + M(W).id },
                                  null,
                                  8,
                                  ["page"],
                                )),
                    ])),
              ],
              2,
            ),
          ]),
          M(Is)
            ? (p(),
              g("div", Tv, [
                Z(v(M(Is).text) + " ", 1),
                M(Is).link
                  ? (p(),
                    g(
                      "a",
                      {
                        key: 0,
                        href: "#衣柜",
                        onClick:
                          w[6] ||
                          (w[6] = (A) => {
                            (d("衣柜"), (El.value = "方案库"));
                          }),
                      },
                      "去方案库 ↗",
                    ))
                  : X("", !0),
              ]))
            : X("", !0),
        ])
      );
    },
  });
Mv.__approvedFixture='approved-demo-fixture';
export const approvedReferenceData={characters:et,worlds:Ut,life:er,media:Po,endpoints:Eo,bindings:Ao,quiet:Io,settings:Oo};
export default Mv;
document.addEventListener(
  "wheel",
  (e) => {
    const t = e.target instanceof Element ? e.target : null;
    if (
      !t ||
      t.closest("select, input, textarea, option, datalist") ||
      e.ctrlKey ||
      !e.deltaY
    )
      return;
    const s = t.closest(".scroll-lock-list, [data-scroll-lock]");
    if (!s) return;
    const n = getComputedStyle(s),
      l =
        !!s.closest("details[open], .switch-menu") &&
        s.getClientRects().length > 0 &&
        n.visibility === "visible" &&
        s.clientHeight > 0 &&
        s.scrollHeight > s.clientHeight &&
        ["auto", "scroll"].includes(n.overflowY);
    if (((s.style.overscrollBehavior = l ? "contain" : "auto"), !l)) return;
    const i = s.scrollTop <= 0,
      r = s.scrollTop + s.clientHeight >= s.scrollHeight;
    ((e.deltaY < 0 && i) || (e.deltaY > 0 && r)) && e.preventDefault();
  },
  { capture: !0, passive: !1 },
);
document.addEventListener("click", (e) => {
  const t = e.target;
  document.querySelectorAll("details.actions-menu[open]").forEach((s) => {
    (!s.contains(t) || t.closest("button")) && (s.open = !1);
  });
});
