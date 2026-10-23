// Isolated v6.2 approved-demo-fixture; selected only at build time.
import {h as Il,ref as z,computed as ye,reactive as Hl,defineComponent as je,openBlock as c,createElementBlock as v,normalizeClass as ke,createElementVNode as n,Fragment as I,renderList as te,unref as h,toDisplayString as m,nextTick as Qt,watch as vt,normalizeStyle as at,createTextVNode as K,createVNode as ae,createCommentVNode as J,withDirectives as re,withKeys as pr,withModifiers as Ue,vModelText as he,onMounted as Wl,onUnmounted as ll,createBlock as Se,Teleport as Ma,renderSlot as Ka,vModelCheckbox as Ft,Transition as Pu,withCtx as Gt,vModelSelect as pt,resolveDynamicComponent as hs,vModelDynamic as rd,mergeProps as qa,useModel as ys,isRef as Fe} from 'vue';
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const gd = (e) => e.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ var pl = {
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
 */ const yd = (
  {
    size: e,
    strokeWidth: t = 2,
    absoluteStrokeWidth: s,
    color: l,
    iconNode: o,
    name: i,
    class: r,
    ...u
  },
  { slots: d },
) =>
  Il(
    "svg",
    {
      ...pl,
      width: e || pl.width,
      height: e || pl.height,
      stroke: l || pl.stroke,
      "stroke-width": s ? (Number(t) * 24) / Number(e) : t,
      class: ["lucide", `lucide-${gd(i ?? "icon")}`],
      ...u,
    },
    [...o.map((f) => Il(...f)), ...(d.default ? [d.default()] : [])],
  );
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Pe =
  (e, t) =>
  (s, { slots: l }) =>
    Il(yd, { ...s, iconNode: t, name: e }, l);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const bd = Pe("CheckIcon", [
  ["path", { d: "M20 6 9 17l-5-5", key: "1gmf2c" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Ko = Pe("ChevronDownIcon", [
  ["path", { d: "m6 9 6 6 6-6", key: "qrunsl" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const Jn = Pe("ChevronRightIcon", [
  ["path", { d: "m9 18 6-6-6-6", key: "mthhwq" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const kd = Pe("EarthIcon", [
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
 */ const Sn = Pe("ImageIcon", [
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
 */ const _d = Pe("ImagesIcon", [
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
 */ const wd = Pe("InboxIcon", [
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
 */ const Gn = Pe("LeafIcon", [
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
 */ const $d = Pe("LibraryIcon", [
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
 */ const Cd = Pe("MessageCircleIcon", [
  ["path", { d: "M7.9 20A9 9 0 1 0 4 16.1L2 22Z", key: "vv11sd" }],
]);
/**
 * @license lucide-vue-next v0.468.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */ const xd = Pe("PanelRightCloseIcon", [
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
 */ const Sd = Pe("PanelRightOpenIcon", [
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
 */ const Td = Pe("RotateCcwIcon", [
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
 */ const Md = Pe("SendIcon", [
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
 */ const Id = Pe("SettingsIcon", [
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
 */ const Ed = Pe("ShieldCheckIcon", [
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
 */ const Tn = Pe("SproutIcon", [
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
 */ const Ad = Pe("UsersIcon", [
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
 */ const zo = Pe("VideoIcon", [
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
 */ const qo = Pe("Volume2Icon", [
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
 */ const Vd = Pe("WandSparklesIcon", [
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
  Od = [
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
  ve = (e, t, s = "text", l, o) => ({
    label: e,
    value: t,
    type: s,
    min: l,
    max: o,
  }),
  As = (e, t, s) => ({ label: e, value: t, type: "select", options: s }),
  Ud = ["思考与通用分析", "聊天回复", "看图理解", "记忆检索向量"],
  Ds = [
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
  Wo = [
    ...Ds.slice(0, 23).filter(
      (e) => !["聊天回复", "看图理解", "记忆向量化"].includes(e),
    ),
    "穿搭准备",
    "动态（朋友圈）",
    "世界 / NPC 相遇",
  ],
  El = ["聊天照片", "动态配图", "物品照", "换装照", "形象定妆 / 设定图"],
  Pd = [
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
  Jo = [
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
  mr = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"],
  Go = z([
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
  Yo = z(
    Ud.map((e, t) => ({
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
  Zo = z({
    enabled: !0,
    start: "23:00",
    end: "08:00",
    days: [...mr],
    behavior: "暂停主动联系",
    character: "全部角色",
  }),
  Qo = z([
    {
      title: "图片（ComfyUI）",
      fields: [
        ve("服务地址", "http://localhost:8188", "url"),
        ve("图片生成等待上限（秒）", 1200, "number", 30, 1200),
        ve("图片工作流", '{"prompt":"{{prompt}}","seed":"{{seed}}"}', "json"),
        ve("视觉身份工作流", "{}", "json"),
        ...["人物", "场景", "动物", "物品", "肖像"].map((e) =>
          ve(e + "工作流", "{}", "json"),
        ),
      ],
    },
    {
      title: "聊天回复",
      fields: [
        As("回复模式", "独立回复模型（Fact-only）", [
          "独立回复模型（Fact-only）",
          "旧模式",
        ]),
        As("表达版本", "v5", ["v5", "v4"]),
        ve("温度", 0.8, "number", 0, 2),
        ve("Top P", 0.9, "number", 0.01, 1),
        ve("单条回复长度上限", 1024, "number", 1, 32768),
        ve("回复可读上下文", 8192, "number", 1024, 131072),
        ve("回复总超时（秒）", 300, "number", 1, 3600),
        ve("同时生成回复数", 1, "number", 1, 8),
        ve("出错自动重试次数", 1, "number", 0, 2),
        ve("媒体动作回交上限", 2, "number", 1, 2),
        As("回复模型失败时", "直接报错", ["直接报错", "改用思考模型兜底"]),
      ],
    },
    { title: "看图理解", fields: [ve("启用看图理解", !1, "checkbox")] },
    {
      title: "角色自主与主动联系",
      fields: [
        As("自主模式", "允许自主行动", ["允许自主行动", "暂停"]),
        ...["主动发消息", "发动态", "请求新能力", "请求生成图片"].map((e) =>
          ve("允许" + e, e !== "请求生成图片", "checkbox"),
        ),
        ve("剩余行动额度", 100, "number", 0),
        ve("重复话题冷却（秒）", 300, "number", 300, 604800),
        ve("冷却到", "", "datetime-local"),
        ve("同时进行的自主行动数", 1, "number", 1),
        ve("暂停主动联系", !1, "checkbox"),
        ve("需要我确认的行动", "请求生成图片"),
      ],
    },
    {
      title: "作息与后台节奏",
      fields: [
        ve("定时醒来（固定间隔 600 秒）", !0, "checkbox"),
        ve("对话摘要间隔（秒）", 300, "number", 300, 600),
        ve("摘要长度（字）", 2048, "number", 512, 4096),
        ve("目标重试次数", 5, "number", 1, 20),
        ve("重试起始间隔（秒）", 60, "number", 30),
        ve("重试间隔上限（秒）", 3600, "number", 60),
      ],
    },
    {
      title: "资源与预算",
      fields: [
        ve("思考模型同时请求数", 1, "number", 1, 8),
        ve("向量模型同时请求数", 1, "number", 1, 8),
      ],
    },
    {
      title: "通知",
      fields: ["新消息", "目标进展", "媒体完成"].map((e) =>
        ve(e, !0, "checkbox"),
      ),
    },
    {
      title: "高级",
      fields: [
        ve("启用 Kev", !1, "checkbox"),
        ve("Kev 地址", "http://127.0.0.1:8010/v1/systemone", "url"),
        ve("Kev 凭据", "", "password"),
        ve("清除 Kev 凭据", !1, "checkbox"),
        ve("请求超时（ms）", 1500, "number", 50, 6e4),
        ve("阶段总预算（ms）", 3e3, "number", 50, 12e4),
        ve("媒体方案补充", !0, "checkbox"),
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
        ].map((e) => ve(e, !0, "checkbox")),
        As("策略", "直接采用", ["直接采用", "加置信门槛"]),
        ve("最低概率", 0.7, "number", 0, 1),
        ve("前两名差距", 0.1, "number", 0, 1),
        ve("最多连续延后", 3, "number", 1, 100),
        ve("延后秒数", 60, "number", 1),
        ve("记录保留（天）", 7, "number", 1, 90),
      ],
    },
  ]),
  Xo = [
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
  Ld = {
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
function Nd(e, t) {
  const s = Ld[e];
  return {
    input: `用途：${e}
角色：${t}
${s.prompt}`,
    response: s.response,
  };
}
const Rd = [
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
  Fd = [...Ds.slice(0, 23), ...El, ...Ds.slice(0, 9), ...Ds.slice(23), El[1]],
  vr = z(
    Fd.map((e, t) => {
      const s = El.indexOf(e),
        l = s >= 0,
        o = ["林晚晴", "陈屿", "苏念安"][t % 3],
        i = t < 9 || t === 24 ? "error" : "success",
        r = l ? Rd[s] : void 0,
        u = r
          ? {
              type: e,
              workflow: r.workflow,
              prompt: `用途：${e}
角色参考：${o}
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
        d = u ? { input: u.prompt, response: u.output } : Nd(e, o);
      return {
        id: -100 - t,
        label: e,
        kind: l ? "媒体调用" : "模型调用",
        character: o,
        start: t * 4,
        duration: l ? 18 + (t % 12) : 2 + (t % 7),
        status: i,
        ...d,
        reason:
          i === "success"
            ? ""
            : l
              ? "图片服务暂时无法连接，请确认 ComfyUI 已启动。"
              : Jo[t % 9].reason,
        category: l ? "服务连不上/报错" : Pd[t % 9],
        model: l ? "ComfyUI" : "Qwen",
        tokens: l ? void 0 : 846 + t * 17,
        code: l ? "media_connection_failed" : Jo[t % 9].code,
        media: u,
        run() {},
      };
    }),
  ),
  xt = ["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"],
  jd = [
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
  Dd = [
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
  Ae = z(
    Od.map((e) => ({
      ...e,
      age: [26, 28, 24][e.id],
      traits: e.id === 1 ? ["沉稳", "细致"] : ["温柔", "好奇"],
      speakingStyle: "自然、温和，愿意分享日常的小事。",
      coreValues: "真诚相待，尊重彼此的节奏。",
      avatarMode: "initial",
      worldId: e.id,
      homeLocation: "家",
      wardrobeStyles: ["舒适日常"],
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
      garments: xt.flatMap((t, s) =>
        [0, 1].map((l) => ({
          id: s * 2 + l + 1,
          name: [
            ["燕麦风衣", "牛仔外套"],
            [e.outfit, "白色棉麻衬衫"],
            ["棉质内衣", "轻薄内衣"],
            ["棉质内裤", "舒适内裤"],
            ["直筒长裤", "休闲短裤"],
            ["白色棉袜", "灰色短袜"],
            ["帆布鞋", "皮质便鞋"],
            ["银色手表", "帆布包"],
          ][s][l],
          type: t,
          seasons: l ? ["春", "夏"] : ["春", "秋", "冬"],
        })),
      ),
      outfitTags: ["日常", "散步", "工作"],
      outfits: [
        {
          id: 1,
          name: "秋日散步",
          tag: "散步",
          items: Object.fromEntries(xt.map((t, s) => [t, [s * 2 + 1]])),
        },
      ],
      wearing: Object.fromEntries(xt.map((t, s) => [t, [s * 2 + 1]])),
      genre: "现代都市",
      sites: ["家", "商店街", "学校"].map((t, s) => ({
        name: t,
        places: Dd.slice(s * 3, s * 3 + 3).map(([l, o, i, r, u], d) => ({
          id: s * 10 + d,
          name: l,
          alias: "",
          floor: "1F",
          description: o,
          atmosphere: i,
          furniture: r,
          layout: u,
          custom: !1,
        })),
      })),
      people: jd.map(([t, s, l, o], i) => ({
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
        bio: l,
        schedule: o,
        tag: ["日常", "工作", "旧友"][Math.floor(i / 2) % 3],
        type: i === 1 ? "用户" : "世界中的人",
        status: i % 4 === 0 ? "暂未联系" : "活跃",
      })),
    })),
  ),
  lt = z(0),
  U = ye(() => Ae.value[lt.value]);
function qt(e, t) {
  return Object.fromEntries(
    xt.map((s) => [
      s,
      (t[s] || []).filter((l) =>
        e.garments.some((o) => o.id === l && o.type === s),
      ),
    ]),
  );
}
function Hd(e, t) {
  ((e.garments = e.garments.filter((s) => s.id !== t)),
    (e.wearing = qt(e, e.wearing)),
    e.outfits.forEach((s) => (s.items = qt(e, s.items))));
}
const Hs = z(null);
let hr,
  Mn = 0;
function Bd() {
  (Mn++, Xs());
}
function Xs() {
  (clearTimeout(hr), (Hs.value = null));
}
function el(e, t) {
  (Xs(), (Hs.value = { text: e, link: t }), (hr = setTimeout(Xs, 2500)));
}
const Yt = z(!1),
  $s = z([]);
let Kd = 0;
function Xe(e, t, s, l = t, o) {
  var f;
  const i = Mn,
    r = {
      id: ++Kd,
      label: e,
      input: t,
      status: "pending",
      character: ((f = U.value) == null ? void 0 : f.name) || "角色工作区",
      start: Math.round((Date.now() % 12e4) / 1e3),
      duration: 1,
      response: "请求已完成。",
      run: () => {},
    };
  $s.value.push(r);
  const u = $s.value.at(-1);
  let d = !1;
  return (
    (u.run = () => {
      d ||
        ((d = !0),
        (u.status = "pending"),
        (u.reason = ""),
        setTimeout(() => {
          (Yt.value || t.includes("失败")
            ? ((u.status = "error"),
              (u.reason = t.includes("失败")
                ? "输入包含“失败”，本次操作未完成，请修改输入后重新提交。"
                : "模拟失败已开启，本次操作未完成，关闭后可重试。"))
            : (s(),
              (u.status = "success"),
              i === Mn &&
                el(
                  o || "已保存「" + l.slice(0, 32) + "」",
                  e === "衣柜" ? "方案库" : void 0,
                )),
            (d = !1));
        }, 800));
    }),
    u.run(),
    u
  );
}
function zd(e, t) {
  return (
    (e.wearing[t] || [])
      .map((s) => {
        var l;
        return (l = e.garments.find((o) => o.id === s)) == null
          ? void 0
          : l.name;
      })
      .filter(Boolean)
      .join("、") || "未穿戴"
  );
}
const qd = [
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
  ei = Ae.value.flatMap((e) =>
    qd[e.id].map(([t, s, l], o) => ({
      id: e.id * 100 + o,
      character: e.name,
      type: t,
      tag: s,
      title: l,
      time: "10月" + (23 - o) + "日",
    })),
  );
Ae.value.forEach((e) => {
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
const Yn = z("为她搭配"),
  fs = (e) =>
    ye(() =>
      $s.value.some((t) => e.includes(t.label) && t.status === "pending"),
    ),
  Zn = z("配置中心"),
  In = z(null),
  ml = z(!1),
  Al = "2026-10-23",
  Wd = [
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
  Cs = z(
    Ae.value.map((e) => ({
      appointments: [
        { date: "2026-10-24", time: "10:00", text: "一起去旧书店" },
        { date: "2026-10-24", time: "16:30", text: "河边散步" },
        { date: "2026-10-26", time: "18:00", text: "分享一顿晚餐" },
      ],
      diaries: Wd.map((t, s) => ({
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
function gr(e, t) {
  const s = Cs.value[e].diaries,
    l = s.find((o) => o.date === Al);
  if (l) {
    ((l.text = t), (l.time = "21:40"));
    return;
  }
  s.push({
    id: Date.now(),
    date: Al,
    time: "21:40",
    title: "今天的小小回响",
    weather: "晴",
    mood: "平静",
    text: t,
  });
}
const Oe = z(
    Ae.value.map((e) => ({
      id: e.id,
      name: e.world.split(" · ")[0],
      genre: e.genre,
      residents: [e.id],
      sites: JSON.parse(JSON.stringify(e.sites)),
    })),
  ),
  ft = (e) => JSON.parse(JSON.stringify(e)),
  yr = ft(Ae.value),
  Jd = ft(Cs.value),
  Gd = ft(Oe.value),
  br = ["#e4ede4", "#e0e8ef", "#eee5df", "#eee8d5", "#e9e2ed"],
  Yd = ["温柔", "沉稳", "好奇", "坦率", "细致", "独立", "幽默", "创作"],
  Zd = ["舒适日常", "简约", "自然系", "复古", "轻运动"],
  kr = [
    { id: 1, name: "燕麦色风衣", type: "外套", seasons: ["春", "秋"] },
    { id: 2, name: "白色棉麻衬衫", type: "上衣", seasons: ["春", "夏", "秋"] },
    {
      id: 3,
      name: "棉质内衣",
      type: "内衣",
      seasons: ["春", "夏", "秋", "冬"],
    },
    {
      id: 4,
      name: "舒适内裤",
      type: "内裤",
      seasons: ["春", "夏", "秋", "冬"],
    },
    { id: 5, name: "直筒长裤", type: "下衣", seasons: ["春", "秋", "冬"] },
    {
      id: 6,
      name: "白色棉袜",
      type: "袜子",
      seasons: ["春", "夏", "秋", "冬"],
    },
    { id: 7, name: "帆布鞋", type: "鞋子", seasons: ["春", "夏", "秋"] },
    { id: 8, name: "帆布包", type: "配饰", seasons: ["春", "夏", "秋", "冬"] },
  ];
function _r() {
  var e;
  return {
    name: "",
    age: 26,
    intro: "",
    initial: "",
    color: br[0],
    avatarMode: "initial",
    traits: ["温柔"],
    speakingStyle: "自然、温和，像日常聊天一样。",
    coreValues: "真诚相待，尊重彼此。",
    worldId: ((e = Oe.value[0]) == null ? void 0 : e.id) ?? "new",
    worldName: "",
    genre: "现代都市",
    homeLocation: "家",
    wardrobeStyles: ["舒适日常"],
    garmentIds: [2, 3, 4, 5, 6, 7],
  };
}
const fe = z(_r()),
  ts = z(!1),
  Ne = z(1),
  Ke = z("editing"),
  Bs = z(!0),
  wr = z(null),
  xs = Hl([
    {
      key: "profile",
      name: "人物资料",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
    {
      key: "personality",
      name: "人格与关系设定",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
    {
      key: "world",
      name: "世界与地点",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
    {
      key: "wardrobe",
      name: "初始衣柜",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
    {
      key: "reference",
      name: "规范参考图",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
    {
      key: "avatar",
      name: "显示头像",
      state: "等待中",
      reason: "",
      attempts: 0,
    },
  ]),
  Qd = ye(() => xs.some((e) => e.state === "进行中"));
let En = 0,
  $r;
function Qn() {
  (En++,
    clearTimeout($r),
    (ts.value = !1),
    (Ne.value = 1),
    (Ke.value = "editing"),
    (fe.value = _r()),
    (Bs.value = !0),
    (wr.value = null),
    xs.forEach((e) =>
      Object.assign(e, { state: "等待中", reason: "", attempts: 0 }),
    ));
}
function Cr() {
  (Qn(),
    (Ae.value = []),
    (Cs.value = []),
    (lt.value = 0),
    Oe.value.forEach((e) => (e.residents = [])),
    Xs());
}
function vl() {
  if (
    Ae.value.some((e) => !yr.some((o) => o.name === e.name)) &&
    !confirm(
      "恢复示例角色会替换当前角色列表，你新建的角色将被移除。确定继续吗？",
    )
  )
    return;
  ((Ae.value = ft(yr)),
    (Cs.value = ft(Jd)),
    (Oe.value = ft(Gd)),
    (lt.value = 0),
    Qn(),
    Xs(),
    (location.hash = encodeURIComponent("角色库")));
}
function Ps() {
  ((!ts.value || Ke.value === "complete") && Qn(),
    (ts.value = !0),
    (location.hash = encodeURIComponent("创建角色")));
}
function Vl(e) {
  const t = fe.value;
  if (e === 1) {
    if (!t.name.trim()) return "请先给角色起一个名字。";
    if (!Number.isInteger(t.age) || t.age < 1 || t.age > 150)
      return "请填写 1–150 岁之间的整数年龄。";
    if (!t.intro.trim()) return "写一句简短的介绍，让相识有个开始。";
  }
  if (
    e === 2 &&
    (!t.traits.length || !t.speakingStyle.trim() || !t.coreValues.trim())
  )
    return "请选择至少一个性格特质，并补充说话方式和核心价值观。";
  if (e === 3) {
    if (t.worldId === "new" && !t.worldName.trim())
      return "请为新世界起一个名字。";
    if (t.worldId !== "new" && !Oe.value.some((s) => s.id === t.worldId))
      return "请选择一个可用世界，或创建新世界。";
    if (!t.homeLocation.trim()) return "请填写角色居住的地点。";
  }
  return e === 4 && (!t.wardrobeStyles.length || !t.garmentIds.length)
    ? "请选择穿衣风格和至少一件初始衣物。"
    : "";
}
function Xd() {
  var r;
  if (Ke.value === "complete") return;
  const e = ft(fe.value),
    t = Ae.value.length;
  let s = Oe.value.find((u) => u.id === e.worldId);
  if (e.worldId === "new") {
    const u = {
      id: Date.now(),
      name: e.homeLocation.trim(),
      alias: "",
      floor: "1F",
      description: "日常生活与休息的地方。",
      atmosphere: "安静、舒适",
      furniture: "基础家具",
      layout: "等待一起慢慢布置",
      custom: !0,
    };
    ((s = {
      id: Math.max(-1, ...Oe.value.map((d) => d.id)) + 1,
      name: e.worldName.trim(),
      genre: e.genre.trim() || "现代都市",
      residents: [],
      sites: [{ name: "家", places: [u] }],
    }),
      Oe.value.push(s));
  }
  if (!s) {
    Object.assign(xs[2], {
      state: "失败",
      reason: "所选世界已被移除，请返回世界与地点重新选择。",
    });
    return;
  }
  const l = ft(kr.filter((u) => e.garmentIds.includes(u.id))),
    o = Object.fromEntries(
      xt.map((u) => [
        u,
        l
          .filter((d) => d.type === u)
          .slice(0, 1)
          .map((d) => d.id),
      ]),
    ),
    i = {
      ...ft(yr[0]),
      id: t,
      name: e.name.trim(),
      age: e.age,
      bio: e.intro.trim(),
      subtitle: e.intro.trim(),
      job: "新来的生活伙伴",
      initial: e.initial.trim() || Array.from(e.name.trim()).at(-1),
      color: e.color,
      avatarMode: e.avatarMode,
      traits: e.traits,
      tags: [...e.traits],
      speakingStyle: e.speakingStyle.trim(),
      coreValues: e.coreValues.trim(),
      worldId: s.id,
      world: s.name,
      genre: s.genre,
      sites: ft(s.sites),
      homeLocation: e.homeLocation.trim(),
      location: e.homeLocation.trim(),
      mood: "平静 · 期待新的相识",
      activity: "熟悉新的生活",
      outfit:
        ((r = l.find((u) => u.type === "上衣")) == null ? void 0 : r.name) ||
        l[0].name,
      wardrobeStyles: e.wardrobeStyles,
      wardrobe: l.map((u) => u.name),
      garments: l,
      wearing: o,
      outfits: [{ id: 1, name: "初次见面", tag: "日常", items: ft(o) }],
      outfitTags: ["日常"],
      friend: "",
      hobby: "慢慢发现生活里的喜欢",
      people: [],
      posts: [],
      memories: [],
      goals: [],
      diary: [],
      messages: [
        {
          text: `你好，我是${e.name.trim()}。很高兴从今天开始，和你分享日常。`,
          time: "刚刚",
        },
      ],
      schedule: [
        { time: "09:00", text: "早餐与整理" },
        { time: "14:00", text: "熟悉新的生活" },
        { time: "18:30", text: "晚餐与休息" },
      ],
    };
  ((Cs.value[t] = { appointments: [], diaries: [], memories: [] }),
    Ae.value.push(i),
    s.residents.push(t),
    (lt.value = t),
    (wr.value = t),
    (Ke.value = "complete"));
}
function Ol(e) {
  const t = xs[e];
  if (!t) {
    Xd();
    return;
  }
  if (t.state === "已完成") {
    Ol(e + 1);
    return;
  }
  const s = En;
  ((t.state = "进行中"),
    (t.reason = ""),
    t.attempts++,
    ($r = setTimeout(() => {
      if (s !== En) return;
      const l = t.key === "world" ? Vl(3) : "",
        o = Yt.value
          ? "模拟失败已开启，请关闭顶栏的“模拟失败”后重试。"
          : t.key === "reference" && Bs.value && t.attempts === 1
            ? "图像模型未配置"
            : l;
      if (o) {
        ((t.state = "失败"), (t.reason = o));
        return;
      }
      ((t.state = "已完成"), Ol(e + 1));
    }, 700)));
}
function ec() {
  if (Ke.value === "editing") {
    for (let e = 1; e <= 4; e++)
      if (Vl(e)) {
        Ne.value = e;
        return;
      }
    ((ts.value = !0), (Ke.value = "preparing"), Ol(0));
  }
}
function tc(e) {
  if (Qd.value || Ke.value !== "preparing") return;
  const t = xs.findIndex((s) => s.key === e && s.state === "失败");
  t >= 0 && Ol(t);
}
const xr = je({
    __name: "CurrentOutfit",
    props: { compact: { type: Boolean } },
    setup(e) {
      function t() {
        ((Yn.value = "为她搭配"), (location.hash = encodeURIComponent("衣柜")));
      }
      return (s, l) => (
        c(),
        v(
          "section",
          {
            class: ke(["outfit-block", { "outfit-compact": e.compact }]),
            "data-testid": "current-outfit",
          },
          [
            n("div", { class: "row" }, [
              l[0] || (l[0] = n("b", null, "今日穿着", -1)),
              l[1] || (l[1] = n("span", { class: "spacer" }, null, -1)),
              n("button", { class: "text-button", onClick: t }, "去编辑 ↗"),
            ]),
            n(
              "div",
              { class: ke(e.compact ? "outfit-tags" : "outfit-lines") },
              [
                (c(!0),
                v(
                  I,
                  null,
                  te(
                    h(xt),
                    (o) => (
                      c(),
                      v("div", { key: o }, [
                        n("small", null, m(o), 1),
                        n("span", null, m(h(zd)(h(U), o)), 1),
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
  sc = { class: "chat-panel" },
  lc = { class: "chat-header" },
  nc = { class: "chat-private" },
  oc = { class: "message-body" },
  ic = { class: "message-meta" },
  rc = { class: "bubble" },
  ac = { key: 0, class: "error", role: "alert" },
  uc = ["onClick"],
  dc = { key: 0, class: "typing" },
  cc = { class: "composer" },
  fc = { class: "composer-bottom" },
  pc = ["disabled"],
  mc = { key: 0, class: "preview" },
  vc = { class: "preview-heading" },
  hc = { class: "scene" },
  gc = { class: "scene-caption" },
  yc = { class: "compact-bio" },
  bc = { class: "schedule-preview" },
  kc = { class: "next-item" },
  _c = { class: "preview-tip" },
  wc = je({
    __name: "ChatPage",
    props: { preview: { type: Boolean } },
    emits: ["go"],
    setup(e, { emit: t }) {
      const s = t,
        l = (a) => s("go", a),
        o = z(""),
        i = z();
      function r() {
        Qt(() => {
          var a;
          return (a = i.value) == null
            ? void 0
            : a.scrollTo({ top: i.value.scrollHeight, behavior: "smooth" });
        });
      }
      function u(a, p) {
        if (/动态|发帖/.test(p))
          return (
            a.posts.unshift({
              text:
                p.replace(/.*?(发一条动态|发动态|发布动态)[：:，,]?/, "") ||
                "今天也在认真感受生活，记录此刻的小美好。",
              time: "刚刚",
              likes: 0,
              liked: !1,
              comments: [],
            }),
            "好呀，已经把这一刻写成动态了。去动态里看看吧。"
          );
        if (/换|穿|衣服/.test(p)) {
          const _ = a.garments.find((b) => p.includes(b.name));
          return _
            ? ((a.wearing[_.type] = [_.id]),
              (a.outfit = _.name),
              `换上${a.outfit}了，你觉得怎么样？`)
            : "想换哪件衣服？可以去衣柜里挑选。";
        }
        return /世界|天气|下雨/.test(p)
          ? ((a.world = p),
            (a.location = p.includes("海边") ? "海边" : a.location),
            "好，世界设定已经调整了。新的风景也让人有一点期待。")
          : /日程|安排/.test(p)
            ? (a.schedule.push({ time: "20:00", text: p }),
              "已经把这件事加入日程，今晚会留出时间的。")
            : /日记|补写/.test(p)
              ? (gr(a.id, `${p}。今天的这些小事，都想认真记下来。`),
                "日记补写好了。有些小事，记下来才发现很珍贵。")
              : /累|烦|难过|压力/.test(p)
                ? "听起来今天有些辛苦。愿意和我说说发生了什么吗？我陪你慢慢聊。"
                : /你好|早安|晚安|晚上好/.test(p)
                  ? p.includes("晚安")
                    ? "晚安，愿你今晚睡得安稳。明天再聊。"
                    : "你好呀，见到你很开心。今天过得怎么样？"
                  : /吃|饭|饿/.test(p)
                    ? "说到吃饭，我也想来一碗热汤面。你现在想吃什么？"
                    : /书|读/.test(p)
                      ? "最近在读什么书？想听听你喜欢它的地方。"
                      : /开心|高兴|喜欢/.test(p)
                        ? "听起来是件让人开心的事。愿意多和我分享一点吗？"
                        : /[?？]|怎么|什么/.test(p)
                          ? "我们可以一起想想。你更在意哪一方面？告诉我一些细节吧。"
                          : "我在听，愿意再和我说说吗？也想知道你当时的感受。";
      }
      function d() {
        const a = o.value.trim();
        if (!a || f.value) return;
        const p = U.value;
        o.value = "";
        const _ = { text: a, user: !0, time: "刚刚", action: void 0 };
        p.messages.push(_);
        const b = p.messages[p.messages.length - 1];
        ((b.action = Xe("聊天回复", a, () =>
          p.messages.push({ text: u(p, a), time: "刚刚" }),
        )),
          r());
      }
      const f = ye(() =>
        U.value.messages.some((a) => {
          var p;
          return ((p = a.action) == null ? void 0 : p.status) === "pending";
        }),
      );
      return (
        vt(() => U.value.messages.length, r),
        vt(f, r),
        (a, p) => {
          var _, b;
          return (
            c(),
            v(
              "div",
              { class: ke(["chat-layout", { collapsed: !e.preview }]) },
              [
                n("section", sc, [
                  n("header", lc, [
                    n(
                      "span",
                      {
                        class: "avatar",
                        style: at({ background: h(U).color }),
                      },
                      m(h(U).initial),
                      5,
                    ),
                    n("div", null, [
                      n("strong", null, m(h(U).name), 1),
                      n("small", null, [
                        p[3] ||
                          (p[3] = n("i", { class: "online-dot" }, null, -1)),
                        p[4] || (p[4] = K(" 在线 ", -1)),
                        n("span", null, "· " + m(h(U).job), 1),
                      ]),
                    ]),
                    p[6] || (p[6] = n("span", { class: "spacer" }, null, -1)),
                    n("span", nc, [
                      ae(h(Ed), { size: 13 }),
                      p[5] || (p[5] = K("只属于你们的对话", -1)),
                    ]),
                  ]),
                  n(
                    "div",
                    { ref_key: "timeline", ref: i, class: "timeline" },
                    [
                      p[9] ||
                        (p[9] = n(
                          "div",
                          { class: "date-divider" },
                          [n("span", null, "今天 · 10月23日 星期五")],
                          -1,
                        )),
                      p[10] ||
                        (p[10] = n(
                          "div",
                          { class: "conversation-note" },
                          "从一句问候开始，让陪伴慢慢发生。",
                          -1,
                        )),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(h(U).messages, (A, P) => {
                          var V;
                          return (
                            c(),
                            v(
                              "div",
                              {
                                key: P,
                                class: ke(["message", { mine: A.user }]),
                              },
                              [
                                n(
                                  "span",
                                  {
                                    class: "avatar message-avatar",
                                    style: at({
                                      background: A.user
                                        ? "#e9eae6"
                                        : h(U).color,
                                    }),
                                  },
                                  m(A.user ? "我" : h(U).initial),
                                  5,
                                ),
                                n("div", oc, [
                                  n("div", ic, [
                                    K(m(A.user ? "我" : h(U).name), 1),
                                    n("time", null, m(A.time), 1),
                                  ]),
                                  n("div", rc, m(A.text), 1),
                                  ((V = A.action) == null
                                    ? void 0
                                    : V.status) === "error"
                                    ? (c(),
                                      v("div", ac, [
                                        K(m(A.action.reason), 1),
                                        n(
                                          "button",
                                          { onClick: (le) => A.action.run() },
                                          [
                                            ae(h(Td), { size: 12 }),
                                            p[7] || (p[7] = K("重试", -1)),
                                          ],
                                          8,
                                          uc,
                                        ),
                                      ]))
                                    : J("", !0),
                                ]),
                              ],
                              2,
                            )
                          );
                        }),
                        128,
                      )),
                      f.value
                        ? (c(),
                          v("div", dc, [
                            K(m(h(U).name) + " 正在输入 ", 1),
                            p[8] || (p[8] = n("span", null, "•••", -1)),
                          ]))
                        : J("", !0),
                    ],
                    512,
                  ),
                  n("div", cc, [
                    re(
                      n(
                        "textarea",
                        {
                          "onUpdate:modelValue":
                            p[0] || (p[0] = (A) => (o.value = A)),
                          "aria-label": "聊天消息",
                          placeholder: "说点什么，分享你的今天…",
                          onKeydown:
                            p[1] ||
                            (p[1] = pr(
                              Ue(
                                (A) => !A.isComposing && d(),
                                ["exact", "prevent"],
                              ),
                              ["enter"],
                            )),
                        },
                        null,
                        544,
                      ),
                      [[he, o.value]],
                    ),
                    n("div", fc, [
                      p[11] ||
                        (p[11] = n(
                          "small",
                          null,
                          "Enter 发送 · Shift + Enter 换行",
                          -1,
                        )),
                      n(
                        "button",
                        {
                          class: "button primary",
                          disabled: !o.value.trim() || f.value,
                          onClick: d,
                        },
                        [
                          K(m(f.value ? "回应中" : "发送"), 1),
                          ae(h(Md), { size: 14 }),
                        ],
                        8,
                        pc,
                      ),
                    ]),
                  ]),
                ]),
                e.preview
                  ? (c(),
                    v("aside", mc, [
                      n("div", vc, [
                        n(
                          "span",
                          null,
                          m(h(U).id === 1 ? "他的此刻" : "她的此刻"),
                          1,
                        ),
                        p[12] ||
                          (p[12] = n(
                            "span",
                            { class: "pill" },
                            "实时预览",
                            -1,
                          )),
                      ]),
                      n("div", hc, [
                        p[14] ||
                          (p[14] = n(
                            "div",
                            { class: "scene-window" },
                            [n("i"), n("i")],
                            -1,
                          )),
                        p[15] ||
                          (p[15] = n("div", { class: "scene-sun" }, null, -1)),
                        p[16] ||
                          (p[16] = n("div", { class: "scene-vase" }, "♧", -1)),
                        p[17] ||
                          (p[17] = n("div", { class: "scene-desk" }, null, -1)),
                        n("div", gc, [
                          p[13] ||
                            (p[13] = n(
                              "span",
                              { class: "scene-tag" },
                              "A LITTLE MOMENT",
                              -1,
                            )),
                          n("p", null, m(h(U).location) + "的午后", 1),
                        ]),
                      ]),
                      n("div", yc, [
                        n("b", null, m(h(U).name) + " · " + m(h(U).job), 1),
                        n("p", null, m(h(U).bio), 1),
                        n(
                          "small",
                          null,
                          m(h(U).mood) + " · " + m(h(U).location),
                          1,
                        ),
                      ]),
                      ae(xr),
                      n("div", bc, [
                        n("div", null, [
                          p[19] ||
                            (p[19] = n("span", null, "接下来的安排", -1)),
                          n(
                            "button",
                            { onClick: p[2] || (p[2] = (A) => l("日程")) },
                            [
                              p[18] || (p[18] = K("查看全部 ", -1)),
                              ae(h(Jn), { size: 12 }),
                            ],
                          ),
                        ]),
                        n("div", kc, [
                          p[21] ||
                            (p[21] = n(
                              "span",
                              { class: "schedule-dot" },
                              null,
                              -1,
                            )),
                          n("div", null, [
                            n(
                              "small",
                              null,
                              m(
                                (_ = h(U).schedule[1]) == null
                                  ? void 0
                                  : _.time,
                              ) + " — 16:00",
                              1,
                            ),
                            n(
                              "p",
                              null,
                              m(
                                (b = h(U).schedule[1]) == null
                                  ? void 0
                                  : b.text,
                              ),
                              1,
                            ),
                            p[20] ||
                              (p[20] = n(
                                "span",
                                { class: "pill" },
                                "进行中",
                                -1,
                              )),
                          ]),
                        ]),
                      ]),
                      n("div", _c, [
                        ae(h(Gn), { size: 16 }),
                        n("p", null, [
                          p[22] ||
                            (p[22] = K("生活在继续，故事也在慢慢生长。", -1)),
                          p[23] || (p[23] = n("br", null, null, -1)),
                          K("这里是 " + m(h(U).name) + " 的小小世界。", 1),
                        ]),
                      ]),
                    ]))
                  : J("", !0),
              ],
              2,
            )
          );
        }
      );
    },
  }),
  $c = { key: 0, class: "inline-error", role: "alert" },
  St = je({
    __name: "ActionFeedback",
    props: { labels: {} },
    setup(e) {
      const t = e,
        s = ye(() =>
          $s.value
            .filter(
              (l) => t.labels.includes(l.label) && l.character === U.value.name,
            )
            .at(-1),
        );
      return (l, o) => {
        var i;
        return ((i = s.value) == null ? void 0 : i.status) === "error"
          ? (c(),
            v("p", $c, [
              K(m(s.value.reason) + " ", 1),
              n(
                "button",
                {
                  class: "text-button",
                  onClick:
                    o[0] || (o[0] = (...r) => s.value.run && s.value.run(...r)),
                },
                "重试",
              ),
            ]))
          : J("", !0);
      };
    },
  }),
  Cc = ["data-testid"],
  xc = { class: "filter-search-row" },
  Sc = ["value", "placeholder", "aria-label", "data-testid"],
  Tc = ["data-testid"],
  Mc = ["aria-label"],
  Ic = ["aria-pressed", "data-testid", "onClick"],
  Ec = ["aria-pressed", "data-testid", "onClick"],
  bs = je({
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
        l = t,
        o = ye(
          () => !!s.search.trim() || Object.values(s.filters).some(Boolean),
        );
      function i(u, d) {
        l("update:filters", { ...s.filters, [u]: s.filters[u] === d ? "" : d });
      }
      function r() {
        (l("update:search", ""),
          l(
            "update:filters",
            Object.fromEntries(s.groups.map((u) => [u.key, ""])),
          ),
          l("clear"));
      }
      return (u, d) => (
        c(),
        v(
          "section",
          { class: "filter-chips", "data-testid": e.testId },
          [
            n("div", xc, [
              n(
                "input",
                {
                  value: e.search,
                  type: "search",
                  placeholder: e.placeholder,
                  "aria-label": e.placeholder,
                  "data-testid": `${e.testId}-search`,
                  onInput:
                    d[0] || (d[0] = (f) => l("update:search", f.target.value)),
                },
                null,
                40,
                Sc,
              ),
              o.value
                ? (c(),
                  v(
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
                    Tc,
                  ))
                : J("", !0),
            ]),
            (c(!0),
            v(
              I,
              null,
              te(
                e.groups,
                (f) => (
                  c(),
                  v(
                    "div",
                    {
                      key: f.key,
                      class: "filter-group",
                      role: "group",
                      "aria-label": f.label,
                    },
                    [
                      n("small", null, m(f.label), 1),
                      n(
                        "button",
                        {
                          type: "button",
                          class: ke([
                            "filter-chip",
                            { active: !e.filters[f.key] },
                          ]),
                          "aria-pressed": !e.filters[f.key],
                          "data-testid": `${e.testId}-${f.key}-all`,
                          onClick: (a) => i(f.key, ""),
                        },
                        "全部",
                        10,
                        Ic,
                      ),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          f.options,
                          (a) => (
                            c(),
                            v(
                              "button",
                              {
                                key: a,
                                type: "button",
                                class: ke([
                                  "filter-chip",
                                  { active: e.filters[f.key] === a },
                                ]),
                                "aria-pressed": e.filters[f.key] === a,
                                "data-testid": `${e.testId}-${f.key}-${a}`,
                                onClick: (p) => i(f.key, a),
                              },
                              m(a),
                              11,
                              Ec,
                            )
                          ),
                        ),
                        128,
                      )),
                    ],
                    8,
                    Mc,
                  )
                ),
              ),
              128,
            )),
          ],
          8,
          Cc,
        )
      );
    },
  }),
  Ac = ["data-testid"],
  Vc = ["aria-label", "data-testid"],
  Oc = { class: "drawer-heading" },
  Uc = ["data-testid"],
  Ks = je({
    __name: "DetailDrawer",
    props: { title: {}, testId: {} },
    emits: ["close"],
    setup(e, { emit: t }) {
      const s = t,
        l = z();
      let o = null;
      function i(r) {
        var a;
        if ((r.key === "Escape" && s("close"), r.key !== "Tab")) return;
        const u =
          (a = l.value) == null
            ? void 0
            : a.querySelectorAll(
                'button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),a[href],summary,[tabindex="0"]',
              );
        if (!(u != null && u.length)) return;
        const d = u[0],
          f = u[u.length - 1];
        r.shiftKey && document.activeElement === d
          ? (r.preventDefault(), f.focus())
          : !r.shiftKey &&
            document.activeElement === f &&
            (r.preventDefault(), d.focus());
      }
      return (
        Wl(() => {
          ((o = document.activeElement),
            document.addEventListener("keydown", i),
            Qt(() => {
              var r;
              return (r = l.value) == null ? void 0 : r.focus();
            }));
        }),
        ll(() => {
          (document.removeEventListener("keydown", i),
            o == null || o.focus({ preventScroll: !0 }));
        }),
        (r, u) => (
          c(),
          Se(Ma, { to: "body" }, [
            n(
              "div",
              {
                class: "drawer-backdrop",
                "data-testid": `${e.testId}-overlay`,
                onClick: u[1] || (u[1] = Ue((d) => s("close"), ["self"])),
              },
              [
                n(
                  "aside",
                  {
                    ref_key: "panel",
                    ref: l,
                    class: "detail-drawer",
                    role: "dialog",
                    "aria-modal": "true",
                    "aria-label": e.title,
                    tabindex: "-1",
                    "data-testid": e.testId,
                  },
                  [
                    n("header", Oc, [
                      n("h2", null, m(e.title), 1),
                      n(
                        "button",
                        {
                          type: "button",
                          class: "text-button",
                          "data-testid": `${e.testId}-close`,
                          onClick: u[0] || (u[0] = (d) => s("close")),
                        },
                        "关闭",
                        8,
                        Uc,
                      ),
                    ]),
                    Ka(r.$slots, "default"),
                  ],
                  8,
                  Vc,
                ),
              ],
              8,
              Ac,
            ),
          ])
        )
      );
    },
  }),
  Pc = { class: "card now-card" },
  Lc = { class: "grid now-grid" },
  Nc = { class: "card" },
  Rc = { class: "card now-appointments" },
  Fc = { class: "row" },
  jc = { key: 0, class: "muted" },
  Dc = { class: "card schedule-list", "data-testid": "schedule-list" },
  Hc = ["data-testid"],
  Bc = ["disabled"],
  Kc = ["disabled"],
  zc = { class: "schedule-text" },
  qc = { key: 0, class: "pill", "data-testid": "schedule-current" },
  Wc = { key: 1, class: "actions-menu" },
  Jc = ["data-testid"],
  Gc = { class: "scroll-lock-list", "data-scroll-lock": "" },
  Yc = ["disabled", "onClick"],
  Zc = { key: 0, class: "muted" },
  Qc = { class: "small-card-grid", "data-testid": "goal-grid" },
  Xc = { class: "row" },
  ef = { class: "pill" },
  tf = { class: "goal" },
  sf = ["onUpdate:modelValue", "data-testid"],
  lf = ["aria-label", "aria-valuenow"],
  nf = ["data-testid"],
  of = { class: "small-card-grid", "data-testid": "appointment-grid" },
  rf = { class: "row" },
  af = ["disabled"],
  uf = { class: "diary-layout" },
  df = { class: "card calendar" },
  cf = { class: "calendar-grid" },
  ff = ["disabled", "aria-label", "aria-pressed", "data-testid", "onClick"],
  pf = { key: 0 },
  mf = {
    class: "diary-book",
    "data-testid": "diary-book",
    "aria-label": "日记簿",
  },
  vf = ["data-date"],
  hf = { class: "row diary-attributes" },
  gf = { class: "diary-text" },
  yf = { class: "diary-signature" },
  bf = { key: 1, class: "diary-paper empty-state" },
  kf = { class: "book-controls" },
  _f = ["disabled"],
  wf = { "aria-live": "polite", "data-testid": "diary-page-indicator" },
  $f = ["disabled"],
  Cf = { class: "memory-list" },
  xf = ["data-testid"],
  Sf = ["aria-expanded", "aria-controls", "data-testid", "onClick"],
  Tf = { class: "memory-chevron" },
  Mf = { class: "memory-content" },
  If = { class: "pill" },
  Ef = ["id"],
  Af = ["aria-label"],
  Vf = { key: 0, class: "muted" },
  Of = { class: "row" },
  Uf = ["data-testid", "onClick"],
  Pf = ["data-testid", "onClick"],
  Lf = { key: 0, class: "card empty-state" },
  Nf = { key: 0, class: "inline-error" },
  Rf = { class: "chip-field" },
  Ff = { class: "star-control", role: "group", "aria-label": "重要度" },
  jf = ["aria-label", "aria-pressed", "data-testid", "onClick"],
  Df = { class: "chip-field" },
  Hf = { class: "chip-options scroll-lock-list", "data-scroll-lock": "" },
  Bf = ["aria-pressed", "onClick"],
  Kf = { class: "chip-field" },
  zf = { class: "chip-options scroll-lock-list", "data-scroll-lock": "" },
  qf = ["aria-pressed", "data-testid", "onClick"],
  Wf = { class: "input-row" },
  Jf = ["disabled"],
  Gf = ["disabled"],
  Yf = { key: 2, class: "modal-backdrop" },
  Zf = {
    class: "card confirm-dialog",
    role: "alertdialog",
    "aria-modal": "true",
    "aria-label": "遗忘这段记忆？",
    "data-testid": "memory-forget-confirm",
  },
  Qf = { class: "row" },
  Xf = je({
    __name: "LifePage",
    props: { page: {} },
    emits: ["go"],
    setup(e, { emit: t }) {
      var S;
      const s = e,
        l = t,
        o = ye(() => Cs.value[lt.value]),
        i = fs(["日记"]),
        r = fs(["日程"]),
        u = fs(["记忆"]),
        d = ye(() =>
          [...U.value.schedule].sort((y, $) => y.time.localeCompare($.time)),
        ),
        f = ye(() => d.value.filter((y) => y.time <= "14:30").at(-1)),
        a = ye(() => d.value.filter((y) => y === f.value || y.time > "14:30")),
        p = z(null),
        _ = z(""),
        b = z("");
      function A(y, $) {
        var x;
        ((x = $.target.closest("details")) == null || x.removeAttribute("open"),
          (p.value = U.value.schedule.indexOf(y)),
          (_.value = y.time),
          (b.value = y.text));
      }
      function P() {
        if (p.value === null || r.value || !_.value || !b.value.trim()) return;
        const y = U.value,
          $ = p.value,
          x = _.value,
          ge = b.value.trim();
        Xe("日程", x + " " + ge, () => {
          ((y.schedule[$] = { time: x, text: ge }), (p.value = null));
        });
      }
      const V = z(""),
        le = z({ category: "", tag: "" }),
        ne = ye(() => {
          var y;
          return [
            ...new Set(
              o.value.memories
                .flatMap(($) => $.tags)
                .concat(((y = G.value) == null ? void 0 : y.tags) || []),
            ),
          ];
        }),
        ce = ye(() => [
          {
            key: "category",
            label: "类型",
            options: ["长期记忆", "发生过的事情", "最近在想"],
          },
          { key: "tag", label: "标签", options: ne.value },
        ]),
        w = ye(() =>
          o.value.memories.filter(
            (y) =>
              [y.text, y.source, ...y.tags]
                .join(" ")
                .toLowerCase()
                .includes(V.value.trim().toLowerCase()) &&
              (!le.value.category || y.category === le.value.category) &&
              (!le.value.tag || y.tags.includes(le.value.tag)),
          ),
        ),
        O = z(null),
        G = z(null),
        oe = z(null),
        W = z("");
      function Y() {
        ((V.value = ""), (le.value = { category: "", tag: "" }));
      }
      function L(y) {
        G.value &&
          (G.value.tags = G.value.tags.includes(y)
            ? G.value.tags.filter(($) => $ !== y)
            : [...G.value.tags, y]);
      }
      function F() {
        const y = W.value.trim().replace(/^#\s*/, "");
        !y ||
          !G.value ||
          (G.value.tags.includes(y) || G.value.tags.push(y), (W.value = ""));
      }
      function C() {
        var x;
        if (!((x = G.value) != null && x.text.trim()) || u.value) return;
        const y = o.value,
          $ = { ...G.value, tags: [...G.value.tags] };
        Xe("记忆", [$.text, $.date, $.category, ...$.tags].join(" "), () => {
          const ge = y.memories.findIndex((nt) => nt.id === $.id);
          (ge >= 0 && (y.memories[ge] = $), (G.value = null));
        });
      }
      const se = ye(() =>
          [...o.value.diaries].sort((y, $) => y.date.localeCompare($.date)),
        ),
        be = z(((S = se.value.at(-1)) == null ? void 0 : S.date) || ""),
        q = z("page-next"),
        M = ye(() => se.value.findIndex((y) => y.date === be.value)),
        T = ye(() => se.value[M.value]);
      function H(y) {
        !se.value.some(($) => $.date === y) ||
          y === be.value ||
          ((q.value = y > be.value ? "page-next" : "page-prev"),
          (be.value = y));
      }
      function _e(y) {
        const $ = se.value[M.value + y];
        $ && H($.date);
      }
      function D() {
        if (i.value) return;
        const y = U.value,
          $ = o.value.diaries.some((x) => x.date === Al);
        Xe(
          "日记",
          "生成" + y.name + "的日记",
          () => {
            (gr(
              y.id,
              `今天在${y.location}${y.activity}，窗外的光一点点移过桌面。忙完后给自己泡了一杯茶，也想起与你聊天的片刻。那些平常的小事，原来一直在心里留下回响。`,
            ),
              H(Al));
          },
          y.name + "的今日日记",
          $ ? "今日日记已更新" : "今日日记已写好",
        );
      }
      return (
        vt(
          () => s.page,
          async (y) => {
            var $;
            ((p.value = null),
              (G.value = null),
              (oe.value = null),
              y === "目标" && ml.value
                ? (await Qt(),
                  ($ = document.getElementById("appointments")) == null ||
                    $.scrollIntoView({ behavior: "smooth", block: "center" }))
                : y !== "目标" && (ml.value = !1));
          },
          { immediate: !0 },
        ),
        (y, $) => (
          c(),
          v(
            I,
            null,
            [
              e.page === "此刻"
                ? (c(),
                  v(
                    I,
                    { key: 0 },
                    [
                      n("div", Pc, [
                        $[16] ||
                          ($[16] = n(
                            "span",
                            { class: "eyebrow" },
                            "FRIDAY, OCTOBER 23",
                            -1,
                          )),
                        n("h2", null, m(h(U).activity), 1),
                        n(
                          "p",
                          null,
                          m(h(U).location) + " · " + m(h(U).mood),
                          1,
                        ),
                        n("p", null, m(h(U).world), 1),
                      ]),
                      n("div", Lc, [
                        n("div", Nc, [ae(xr, { compact: "" })]),
                        n("section", Rc, [
                          n("div", Fc, [
                            $[17] || ($[17] = n("h2", null, "约定", -1)),
                            $[18] ||
                              ($[18] = n(
                                "span",
                                { class: "spacer" },
                                null,
                                -1,
                              )),
                            n(
                              "button",
                              {
                                class: "text-button",
                                "data-testid": "now-appointments",
                                onClick:
                                  $[0] ||
                                  ($[0] = (x) => {
                                    ((ml.value = !0), l("go", "目标"));
                                  }),
                              },
                              "查看全部 ↗",
                            ),
                          ]),
                          (c(!0),
                          v(
                            I,
                            null,
                            te(
                              o.value.appointments.slice(0, 2),
                              (x) => (
                                c(),
                                v(
                                  "div",
                                  {
                                    key: x.date + x.time,
                                    class: "appointment-row",
                                  },
                                  [
                                    n("time", null, [
                                      K(
                                        m(x.date.slice(5).replace("-", "月")) +
                                          "日",
                                        1,
                                      ),
                                      $[19] ||
                                        ($[19] = n("br", null, null, -1)),
                                      K(m(x.time), 1),
                                    ]),
                                    n("span", null, m(x.text), 1),
                                  ],
                                )
                              ),
                            ),
                            128,
                          )),
                          o.value.appointments.length
                            ? J("", !0)
                            : (c(),
                              v("p", jc, "还没有约定，留一些期待给明天。")),
                        ]),
                      ]),
                    ],
                    64,
                  ))
                : J("", !0),
              e.page === "此刻" || e.page === "日程"
                ? (c(),
                  v(
                    I,
                    { key: 1 },
                    [
                      n(
                        "h2",
                        null,
                        m(e.page === "此刻" ? "此刻与接下来" : "今天的安排"),
                        1,
                      ),
                      n("section", Dc, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            e.page === "此刻" ? a.value : d.value,
                            (x) => (
                              c(),
                              v(
                                "div",
                                {
                                  key: x.time + x.text,
                                  class: ke([
                                    "schedule-row",
                                    { "is-current": x === f.value },
                                  ]),
                                  "data-testid": "schedule-" + x.time,
                                },
                                [
                                  p.value === h(U).schedule.indexOf(x) &&
                                  e.page === "日程"
                                    ? (c(),
                                      v(
                                        "form",
                                        {
                                          key: 0,
                                          class: "schedule-editor",
                                          onSubmit: Ue(P, ["prevent"]),
                                        },
                                        [
                                          re(
                                            n(
                                              "input",
                                              {
                                                "onUpdate:modelValue":
                                                  $[1] ||
                                                  ($[1] = (ge) =>
                                                    (_.value = ge)),
                                                type: "time",
                                                "aria-label": "时间",
                                              },
                                              null,
                                              512,
                                            ),
                                            [[he, _.value]],
                                          ),
                                          re(
                                            n(
                                              "input",
                                              {
                                                "onUpdate:modelValue":
                                                  $[2] ||
                                                  ($[2] = (ge) =>
                                                    (b.value = ge)),
                                                "aria-label": "日程内容",
                                              },
                                              null,
                                              512,
                                            ),
                                            [[he, b.value]],
                                          ),
                                          n(
                                            "button",
                                            {
                                              class: "button",
                                              disabled:
                                                h(r) ||
                                                !b.value.trim() ||
                                                !_.value,
                                              "data-testid": "schedule-save",
                                            },
                                            m(h(r) ? "处理中…" : "保存"),
                                            9,
                                            Bc,
                                          ),
                                          n(
                                            "button",
                                            {
                                              type: "button",
                                              class: "text-button",
                                              disabled: h(r),
                                              onClick:
                                                $[3] ||
                                                ($[3] = (ge) =>
                                                  (p.value = null)),
                                            },
                                            "取消",
                                            8,
                                            Kc,
                                          ),
                                        ],
                                        32,
                                      ))
                                    : (c(),
                                      v(
                                        I,
                                        { key: 1 },
                                        [
                                          n("time", null, m(x.time), 1),
                                          $[20] ||
                                            ($[20] = n(
                                              "span",
                                              { class: "schedule-dot" },
                                              null,
                                              -1,
                                            )),
                                          n("span", zc, m(x.text), 1),
                                          x === f.value
                                            ? (c(), v("span", qc, "进行中"))
                                            : J("", !0),
                                          e.page === "日程"
                                            ? (c(),
                                              v("details", Wc, [
                                                n(
                                                  "summary",
                                                  {
                                                    "aria-label": "日程操作",
                                                    "data-testid":
                                                      "schedule-menu-" + x.time,
                                                  },
                                                  "⋯",
                                                  8,
                                                  Jc,
                                                ),
                                                n("div", Gc, [
                                                  n(
                                                    "button",
                                                    {
                                                      disabled: h(r),
                                                      onClick: (ge) => A(x, ge),
                                                    },
                                                    "编辑",
                                                    8,
                                                    Yc,
                                                  ),
                                                ]),
                                              ]))
                                            : J("", !0),
                                        ],
                                        64,
                                      )),
                                ],
                                10,
                                Hc,
                              )
                            ),
                          ),
                          128,
                        )),
                        (e.page === "此刻" ? a.value : d.value).length
                          ? J("", !0)
                          : (c(), v("p", Zc, "暂时没有安排，留一点自由时间。")),
                      ]),
                    ],
                    64,
                  ))
                : J("", !0),
              e.page === "目标"
                ? (c(),
                  v(
                    I,
                    { key: 2 },
                    [
                      $[24] ||
                        ($[24] = n(
                          "p",
                          { class: "muted" },
                          "想做的事慢慢来，约好的事认真记住。",
                          -1,
                        )),
                      n("div", Qc, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            h(U).goals,
                            (x, ge) => (
                              c(),
                              v(
                                "article",
                                { key: x.text, class: "card goal-card" },
                                [
                                  n("div", Xc, [
                                    n("span", ef, m(ge ? "生活" : "创作"), 1),
                                    $[21] ||
                                      ($[21] = n(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    n(
                                      "small",
                                      null,
                                      m(x.done ? 100 : x.progress) + "%",
                                      1,
                                    ),
                                  ]),
                                  n("label", tf, [
                                    re(
                                      n(
                                        "input",
                                        {
                                          "onUpdate:modelValue": (nt) =>
                                            (x.done = nt),
                                          type: "checkbox",
                                          "data-testid": "goal-toggle-" + ge,
                                        },
                                        null,
                                        8,
                                        sf,
                                      ),
                                      [[Ft, x.done]],
                                    ),
                                    n(
                                      "span",
                                      { class: ke({ done: x.done }) },
                                      m(x.text),
                                      3,
                                    ),
                                  ]),
                                  n(
                                    "div",
                                    {
                                      class: "progress",
                                      role: "progressbar",
                                      "aria-label": x.text,
                                      "aria-valuenow": x.done
                                        ? 100
                                        : x.progress,
                                      "aria-valuemin": 0,
                                      "aria-valuemax": 100,
                                    },
                                    [
                                      n(
                                        "i",
                                        {
                                          style: at({
                                            width:
                                              (x.done ? 100 : x.progress) + "%",
                                          }),
                                          "data-testid": "goal-progress-" + ge,
                                        },
                                        null,
                                        12,
                                        nf,
                                      ),
                                    ],
                                    8,
                                    lf,
                                  ),
                                ],
                              )
                            ),
                          ),
                          128,
                        )),
                      ]),
                      n(
                        "section",
                        { id: "appointments", class: ke({ highlight: h(ml) }) },
                        [
                          $[23] ||
                            ($[23] = n(
                              "h2",
                              { class: "section-heading" },
                              "约定",
                              -1,
                            )),
                          n("div", of, [
                            (c(!0),
                            v(
                              I,
                              null,
                              te(
                                [...o.value.appointments].sort((x, ge) =>
                                  (x.date + x.time).localeCompare(
                                    ge.date + ge.time,
                                  ),
                                ),
                                (x) => (
                                  c(),
                                  v(
                                    "article",
                                    {
                                      key: x.date + x.time,
                                      class: "card appointment-card",
                                    },
                                    [
                                      n(
                                        "small",
                                        null,
                                        m(x.date) + " · " + m(x.time),
                                        1,
                                      ),
                                      n("h3", null, m(x.text), 1),
                                      $[22] ||
                                        ($[22] = n(
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
                : J("", !0),
              e.page === "日记"
                ? (c(),
                  v(
                    I,
                    { key: 3 },
                    [
                      n("div", rf, [
                        $[25] ||
                          ($[25] = n(
                            "div",
                            null,
                            [
                              n("h2", null, "写给自己的时光"),
                              n(
                                "p",
                                { class: "muted" },
                                "有些心事，留在纸页之间。",
                              ),
                            ],
                            -1,
                          )),
                        $[26] ||
                          ($[26] = n("span", { class: "spacer" }, null, -1)),
                        n(
                          "button",
                          {
                            class: "button primary",
                            disabled: h(i),
                            "data-testid": "diary-generate",
                            onClick: D,
                          },
                          m(h(i) ? "处理中…" : "生成日记"),
                          9,
                          af,
                        ),
                      ]),
                      n("div", uf, [
                        n("section", df, [
                          $[27] || ($[27] = n("h3", null, "2026 年 10 月", -1)),
                          n("div", cf, [
                            (c(),
                            v(
                              I,
                              null,
                              te(
                                ["一", "二", "三", "四", "五", "六", "日"],
                                (x) => n("small", { key: x }, m(x), 1),
                              ),
                              64,
                            )),
                            (c(),
                            v(
                              I,
                              null,
                              te(3, (x) => n("span", { key: "blank" + x })),
                              64,
                            )),
                            (c(),
                            v(
                              I,
                              null,
                              te(31, (x) =>
                                n(
                                  "button",
                                  {
                                    key: x,
                                    class: ke({
                                      marked: se.value.some(
                                        (ge) => Number(ge.date.slice(-2)) === x,
                                      ),
                                      selected:
                                        be.value ===
                                        `2026-10-${String(x).padStart(2, "0")}`,
                                    }),
                                    disabled: !se.value.some(
                                      (ge) => Number(ge.date.slice(-2)) === x,
                                    ),
                                    "aria-label": `2026年10月${x}日`,
                                    "aria-pressed":
                                      be.value ===
                                      `2026-10-${String(x).padStart(2, "0")}`,
                                    "data-testid": `calendar-day-2026-10-${String(x).padStart(2, "0")}`,
                                    onClick: (ge) =>
                                      H(
                                        `2026-10-${String(x).padStart(2, "0")}`,
                                      ),
                                  },
                                  [
                                    K(m(x), 1),
                                    se.value.some(
                                      (ge) => Number(ge.date.slice(-2)) === x,
                                    )
                                      ? (c(), v("i", pf))
                                      : J("", !0),
                                  ],
                                  10,
                                  ff,
                                ),
                              ),
                              64,
                            )),
                          ]),
                          $[28] ||
                            ($[28] = n(
                              "p",
                              { class: "calendar-note muted" },
                              "有圆点的日子，藏着一页故事。",
                              -1,
                            )),
                        ]),
                        n("section", mf, [
                          ae(
                            Pu,
                            { name: q.value, mode: "out-in" },
                            {
                              default: Gt(() => [
                                T.value
                                  ? (c(),
                                    v(
                                      "article",
                                      {
                                        key: T.value.date,
                                        class: "diary-paper",
                                        "data-testid": "diary-entry",
                                        "data-date": T.value.date,
                                      },
                                      [
                                        n(
                                          "small",
                                          null,
                                          m(T.value.date) +
                                            " · " +
                                            m(T.value.time),
                                          1,
                                        ),
                                        n("h2", null, m(T.value.title), 1),
                                        n("div", hf, [
                                          n(
                                            "span",
                                            null,
                                            "天气 · " + m(T.value.weather),
                                            1,
                                          ),
                                          n(
                                            "span",
                                            null,
                                            "心情 · " + m(T.value.mood),
                                            1,
                                          ),
                                        ]),
                                        n("p", gf, m(T.value.text), 1),
                                        n("p", yf, "— " + m(h(U).name), 1),
                                      ],
                                      8,
                                      vf,
                                    ))
                                  : (c(),
                                    v("div", bf, [
                                      ...($[29] ||
                                        ($[29] = [
                                          n(
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
                          n("footer", kf, [
                            n(
                              "button",
                              {
                                class: "text-button",
                                disabled: M.value <= 0,
                                "data-testid": "diary-prev",
                                onClick: $[4] || ($[4] = (x) => _e(-1)),
                              },
                              "← 上一页",
                              8,
                              _f,
                            ),
                            n(
                              "small",
                              wf,
                              m(T.value ? M.value + 1 : 0) +
                                " / " +
                                m(se.value.length),
                              1,
                            ),
                            n(
                              "button",
                              {
                                class: "text-button",
                                disabled:
                                  M.value < 0 || M.value >= se.value.length - 1,
                                "data-testid": "diary-next",
                                onClick: $[5] || ($[5] = (x) => _e(1)),
                              },
                              "下一页 →",
                              8,
                              $f,
                            ),
                          ]),
                        ]),
                      ]),
                    ],
                    64,
                  ))
                : J("", !0),
              e.page === "记忆"
                ? (c(),
                  v(
                    I,
                    { key: 4 },
                    [
                      $[41] ||
                        ($[41] = n(
                          "p",
                          { class: "muted" },
                          "把相处留下的细节，慢慢收进记忆。",
                          -1,
                        )),
                      ae(
                        bs,
                        {
                          search: V.value,
                          "onUpdate:search":
                            $[6] || ($[6] = (x) => (V.value = x)),
                          filters: le.value,
                          "onUpdate:filters":
                            $[7] || ($[7] = (x) => (le.value = x)),
                          groups: ce.value,
                          "test-id": "memory-filters",
                          placeholder: "搜索记忆 / 标签",
                        },
                        null,
                        8,
                        ["search", "filters", "groups"],
                      ),
                      n("section", Cf, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            w.value,
                            (x) => (
                              c(),
                              v(
                                "article",
                                {
                                  key: x.id,
                                  class: ke([
                                    "memory-item",
                                    { expanded: O.value === x.id },
                                  ]),
                                  "data-testid": "memory-row-" + x.id,
                                },
                                [
                                  n(
                                    "button",
                                    {
                                      class: "memory-summary",
                                      "aria-expanded": O.value === x.id,
                                      "aria-controls": "memory-details-" + x.id,
                                      "data-testid": "memory-toggle-" + x.id,
                                      onClick: (ge) =>
                                        (O.value =
                                          O.value === x.id ? null : x.id),
                                    },
                                    [
                                      n(
                                        "span",
                                        Tf,
                                        m(O.value === x.id ? "−" : "+"),
                                        1,
                                      ),
                                      n("span", Mf, m(x.text), 1),
                                      n("time", null, m(x.date), 1),
                                      n("span", If, m(x.category), 1),
                                    ],
                                    8,
                                    Sf,
                                  ),
                                  O.value === x.id
                                    ? (c(),
                                      v(
                                        "div",
                                        {
                                          key: 0,
                                          id: "memory-details-" + x.id,
                                          class: "memory-details",
                                        },
                                        [
                                          n("dl", null, [
                                            n("div", null, [
                                              $[30] ||
                                                ($[30] = n(
                                                  "dt",
                                                  null,
                                                  "来源",
                                                  -1,
                                                )),
                                              n(
                                                "dd",
                                                null,
                                                m(x.source) + " · " + m(x.date),
                                                1,
                                              ),
                                            ]),
                                            n("div", null, [
                                              $[31] ||
                                                ($[31] = n(
                                                  "dt",
                                                  null,
                                                  "重要度",
                                                  -1,
                                                )),
                                              n(
                                                "dd",
                                                {
                                                  class: "memory-stars",
                                                  "aria-label":
                                                    x.importance + "星，共5星",
                                                },
                                                [
                                                  K(
                                                    m("★".repeat(x.importance)),
                                                    1,
                                                  ),
                                                  n(
                                                    "span",
                                                    null,
                                                    m(
                                                      "☆".repeat(
                                                        5 - x.importance,
                                                      ),
                                                    ),
                                                    1,
                                                  ),
                                                ],
                                                8,
                                                Af,
                                              ),
                                            ]),
                                            n("div", null, [
                                              $[32] ||
                                                ($[32] = n(
                                                  "dt",
                                                  null,
                                                  "标签",
                                                  -1,
                                                )),
                                              n("dd", null, [
                                                (c(!0),
                                                v(
                                                  I,
                                                  null,
                                                  te(
                                                    x.tags,
                                                    (ge) => (
                                                      c(),
                                                      v(
                                                        "span",
                                                        {
                                                          key: ge,
                                                          class: "pill",
                                                        },
                                                        "# " + m(ge),
                                                        1,
                                                      )
                                                    ),
                                                  ),
                                                  128,
                                                )),
                                                x.tags.length
                                                  ? J("", !0)
                                                  : (c(),
                                                    v("span", Vf, "暂无标签")),
                                              ]),
                                            ]),
                                          ]),
                                          n("div", Of, [
                                            n(
                                              "button",
                                              {
                                                class: "text-button",
                                                "data-testid":
                                                  "memory-edit-" + x.id,
                                                onClick: (ge) => {
                                                  ((G.value = {
                                                    ...x,
                                                    tags: [...x.tags],
                                                  }),
                                                    (W.value = ""));
                                                },
                                              },
                                              "编辑",
                                              8,
                                              Uf,
                                            ),
                                            n(
                                              "button",
                                              {
                                                class: "text-button",
                                                "data-testid":
                                                  "memory-forget-" + x.id,
                                                onClick: (ge) => (oe.value = x),
                                              },
                                              "遗忘",
                                              8,
                                              Pf,
                                            ),
                                          ]),
                                        ],
                                        8,
                                        Ef,
                                      ))
                                    : J("", !0),
                                ],
                                10,
                                xf,
                              )
                            ),
                          ),
                          128,
                        )),
                      ]),
                      w.value.length
                        ? J("", !0)
                        : (c(),
                          v("div", Lf, [
                            $[33] ||
                              ($[33] = n(
                                "p",
                                null,
                                "没有符合条件的记忆。",
                                -1,
                              )),
                            n(
                              "button",
                              {
                                class: "button",
                                "data-testid": "memory-empty-clear",
                                onClick: Y,
                              },
                              "清除筛选",
                            ),
                          ])),
                      G.value
                        ? (c(),
                          Se(
                            Ks,
                            {
                              key: 1,
                              title: "编辑记忆",
                              "test-id": "memory-drawer",
                              onClose:
                                $[13] || ($[13] = (x) => (G.value = null)),
                            },
                            {
                              default: Gt(() => [
                                n(
                                  "form",
                                  {
                                    class: "form-grid",
                                    onSubmit: Ue(C, ["prevent"]),
                                  },
                                  [
                                    n("label", null, [
                                      $[34] || ($[34] = K("内容", -1)),
                                      re(
                                        n(
                                          "textarea",
                                          {
                                            "onUpdate:modelValue":
                                              $[8] ||
                                              ($[8] = (x) =>
                                                (G.value.text = x)),
                                            rows: "4",
                                            "data-testid":
                                              "memory-edit-content",
                                          },
                                          null,
                                          512,
                                        ),
                                        [[he, G.value.text]],
                                      ),
                                      G.value.text.trim()
                                        ? J("", !0)
                                        : (c(),
                                          v("small", Nf, "请填写记忆内容")),
                                    ]),
                                    n("label", null, [
                                      $[35] || ($[35] = K("来源", -1)),
                                      re(
                                        n(
                                          "input",
                                          {
                                            "onUpdate:modelValue":
                                              $[9] ||
                                              ($[9] = (x) =>
                                                (G.value.source = x)),
                                          },
                                          null,
                                          512,
                                        ),
                                        [[he, G.value.source]],
                                      ),
                                    ]),
                                    n("label", null, [
                                      $[36] || ($[36] = K("来源日期", -1)),
                                      re(
                                        n(
                                          "input",
                                          {
                                            "onUpdate:modelValue":
                                              $[10] ||
                                              ($[10] = (x) =>
                                                (G.value.date = x)),
                                            type: "date",
                                          },
                                          null,
                                          512,
                                        ),
                                        [[he, G.value.date]],
                                      ),
                                    ]),
                                    n("fieldset", Rf, [
                                      $[37] ||
                                        ($[37] = n(
                                          "legend",
                                          null,
                                          "重要度",
                                          -1,
                                        )),
                                      n("div", Ff, [
                                        (c(),
                                        v(
                                          I,
                                          null,
                                          te(5, (x) =>
                                            n(
                                              "button",
                                              {
                                                key: x,
                                                type: "button",
                                                "aria-label": x + "星",
                                                "aria-pressed":
                                                  G.value.importance === x,
                                                "data-testid":
                                                  "memory-star-" + x,
                                                class: ke({
                                                  filled:
                                                    x <= G.value.importance,
                                                }),
                                                onClick: (ge) =>
                                                  (G.value.importance = x),
                                              },
                                              "★",
                                              10,
                                              jf,
                                            ),
                                          ),
                                          64,
                                        )),
                                        n(
                                          "small",
                                          null,
                                          m(G.value.importance) + " / 5",
                                          1,
                                        ),
                                      ]),
                                    ]),
                                    n("fieldset", Df, [
                                      $[38] ||
                                        ($[38] = n("legend", null, "类型", -1)),
                                      n("div", Hf, [
                                        (c(),
                                        v(
                                          I,
                                          null,
                                          te(
                                            [
                                              "长期记忆",
                                              "发生过的事情",
                                              "最近在想",
                                            ],
                                            (x) =>
                                              n(
                                                "button",
                                                {
                                                  key: x,
                                                  type: "button",
                                                  class: ke([
                                                    "filter-chip",
                                                    {
                                                      active:
                                                        G.value.category === x,
                                                    },
                                                  ]),
                                                  "aria-pressed":
                                                    G.value.category === x,
                                                  onClick: (ge) =>
                                                    (G.value.category = x),
                                                },
                                                m(x),
                                                11,
                                                Bf,
                                              ),
                                          ),
                                          64,
                                        )),
                                      ]),
                                    ]),
                                    n("fieldset", Kf, [
                                      $[39] ||
                                        ($[39] = n("legend", null, "标签", -1)),
                                      n("div", zf, [
                                        (c(!0),
                                        v(
                                          I,
                                          null,
                                          te(
                                            ne.value,
                                            (x) => (
                                              c(),
                                              v(
                                                "button",
                                                {
                                                  key: x,
                                                  type: "button",
                                                  class: ke([
                                                    "filter-chip",
                                                    {
                                                      active:
                                                        G.value.tags.includes(
                                                          x,
                                                        ),
                                                    },
                                                  ]),
                                                  "aria-pressed":
                                                    G.value.tags.includes(x),
                                                  "data-testid":
                                                    "memory-tag-" + x,
                                                  onClick: (ge) => L(x),
                                                },
                                                "# " + m(x),
                                                11,
                                                qf,
                                              )
                                            ),
                                          ),
                                          128,
                                        )),
                                      ]),
                                      n("div", Wf, [
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                $[11] ||
                                                ($[11] = (x) => (W.value = x)),
                                              "aria-label": "新标签",
                                              placeholder: "添加一个新标签",
                                              "data-testid": "memory-new-tag",
                                              onKeydown:
                                                $[12] ||
                                                ($[12] = pr(
                                                  Ue(
                                                    (x) =>
                                                      !x.isComposing && F(),
                                                    ["prevent"],
                                                  ),
                                                  ["enter"],
                                                )),
                                            },
                                            null,
                                            544,
                                          ),
                                          [[he, W.value]],
                                        ),
                                        n(
                                          "button",
                                          {
                                            type: "button",
                                            class: "text-button",
                                            disabled: !W.value.trim(),
                                            "data-testid": "memory-add-tag",
                                            onClick: F,
                                          },
                                          "添加",
                                          8,
                                          Jf,
                                        ),
                                      ]),
                                    ]),
                                    ae(St, { labels: ["记忆"] }),
                                    n(
                                      "button",
                                      {
                                        class: "button primary",
                                        disabled: h(u) || !G.value.text.trim(),
                                        "data-testid": "memory-save",
                                      },
                                      m(h(u) ? "处理中…" : "保存"),
                                      9,
                                      Gf,
                                    ),
                                  ],
                                  32,
                                ),
                              ]),
                              _: 1,
                            },
                          ))
                        : J("", !0),
                      oe.value
                        ? (c(),
                          v("div", Yf, [
                            n("section", Zf, [
                              $[40] ||
                                ($[40] = n("h2", null, "遗忘这段记忆？", -1)),
                              n("p", null, m(oe.value.text), 1),
                              n("div", Qf, [
                                n(
                                  "button",
                                  {
                                    class: "button",
                                    onClick:
                                      $[14] ||
                                      ($[14] = (x) => (oe.value = null)),
                                  },
                                  "取消",
                                ),
                                n(
                                  "button",
                                  {
                                    class: "button primary",
                                    "data-testid": "memory-confirm-forget",
                                    onClick:
                                      $[15] ||
                                      ($[15] = (x) => {
                                        ((o.value.memories =
                                          o.value.memories.filter((ge) => {
                                            var nt;
                                            return (
                                              ge.id !==
                                              ((nt = oe.value) == null
                                                ? void 0
                                                : nt.id)
                                            );
                                          })),
                                          (oe.value = null));
                                      }),
                                  },
                                  "确认遗忘",
                                ),
                              ]),
                            ]),
                          ]))
                        : J("", !0),
                    ],
                    64,
                  ))
                : J("", !0),
              G.value
                ? J("", !0)
                : (c(),
                  Se(
                    St,
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
  ep = {
    viewBox: "0 0 100 100",
    width: "84",
    height: "84",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": "2.5",
    "stroke-linejoin": "round",
    "aria-hidden": "true",
  },
  tp = { key: 0, d: "M50 28v56M34 18l8 26 8-9 8 9 8-26" },
  sp = je({
    __name: "GarmentArt",
    props: { type: {} },
    setup(e) {
      return (t, s) => (
        c(),
        v("svg", ep, [
          e.type === "鞋子"
            ? (c(),
              v(
                I,
                { key: 0 },
                [
                  s[0] ||
                    (s[0] = n(
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
                    (s[1] = n(
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
              ? (c(),
                v(
                  I,
                  { key: 1 },
                  [
                    s[2] ||
                      (s[2] = n(
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
                      (s[3] = n(
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
                ? (c(),
                  v(
                    I,
                    { key: 2 },
                    [
                      s[4] ||
                        (s[4] = n(
                          "circle",
                          { cx: "50", cy: "50", r: "21" },
                          null,
                          -1,
                        )),
                      s[5] ||
                        (s[5] = n(
                          "path",
                          { d: "M39 30V10h22v20M39 70v20h22V70M50 36v16l10 7" },
                          null,
                          -1,
                        )),
                    ],
                    64,
                  ))
                : e.type === "袜子"
                  ? (c(),
                    v(
                      I,
                      { key: 3 },
                      [
                        s[6] ||
                          (s[6] = n(
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
                          (s[7] = n(
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
                    ? (c(),
                      v(
                        I,
                        { key: 4 },
                        [
                          s[8] ||
                            (s[8] = n(
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
                            (s[9] = n(
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
                    : (c(),
                      v(
                        I,
                        { key: 5 },
                        [
                          s[10] ||
                            (s[10] = n(
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
                            (s[11] = n(
                              "path",
                              {
                                d: "M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z",
                              },
                              null,
                              -1,
                            )),
                          e.type === "外套" ? (c(), v("path", tp)) : J("", !0),
                        ],
                        64,
                      )),
        ])
      );
    },
  }),
  lp = { class: "tabs" },
  np = ["onClick"],
  op = { class: "toolbar" },
  ip = { class: "card slot-picker scroll-lock-list", "data-scroll-lock": "" },
  rp = { key: 0, class: "muted", role: "status" },
  ap = { class: "slot-accordion" },
  up = { class: "slot-row scroll-lock-list", "data-scroll-lock": "" },
  dp = ["disabled", "title", "onClick"],
  cp = { key: 0 },
  fp = { class: "card" },
  pp = { class: "slot-summary" },
  mp = { key: 0, class: "inline-error" },
  vp = { key: 1, class: "muted" },
  hp = ["disabled"],
  gp = { class: "card" },
  yp = { class: "tag-list" },
  bp = { class: "pill" },
  kp = ["onClick"],
  _p = ["onClick"],
  wp = ["placeholder"],
  $p = ["disabled"],
  Cp = { class: "row" },
  xp = ["onSubmit"],
  Sp = ["disabled"],
  Tp = { key: 1 },
  Mp = { class: "pill" },
  Ip = { class: "scroll-lock-list", "data-scroll-lock": "" },
  Ep = ["onClick"],
  Ap = ["onClick"],
  Vp = ["onClick"],
  Op = ["onClick"],
  Up = ["disabled", "onClick"],
  Pp = { key: 0, class: "slot-summary" },
  Lp = { key: 0, class: "muted" },
  Np = { class: "toolbar" },
  Rp = { class: "garment-grid" },
  Fp = { class: "row" },
  jp = { class: "scroll-lock-list", "data-scroll-lock": "" },
  Dp = ["onClick"],
  Hp = ["onClick"],
  Bp = { key: 0, class: "muted" },
  Kp = { key: 0, class: "inline-error" },
  zp = { class: "row" },
  qp = ["value"],
  Wp = { class: "row" },
  Jp = ["disabled"],
  Gp = ["aria-label"],
  Yp = { class: "row" },
  Zp = je({
    __name: "WardrobePage",
    setup(e) {
      const t = z(""),
        s = z(null),
        l = fs(["衣柜", "换装"]),
        o = ye(() =>
          U.value.outfits.some(
            (q) => q.name === u.value.trim() && q.id !== f.value,
          ),
        ),
        i = Yn,
        r = z(U.value.outfitTags[0] ?? ""),
        u = z(""),
        d = z(JSON.parse(JSON.stringify(U.value.wearing))),
        f = z(null),
        a = z(null),
        p = z(""),
        _ = z(""),
        b = z(""),
        A = z(""),
        P = z(null),
        V = z(""),
        le = z(null);
      function ne(q, M, T) {
        le.value = { title: q, message: M, run: T };
      }
      function ce(q) {
        const M = U.value,
          T = V.value.trim();
        if (!(!T || l.value)) {
          if (M.outfits.some((H) => H.id !== q.id && H.name === T)) {
            el("方案名称已存在，请换一个名称");
            return;
          }
          ((t.value = "rename"),
            Xe("衣柜", T, () => {
              ((q.name = T), (P.value = null));
            }));
        }
      }
      (vt(
        () => U.value.garments.map((q) => q.id).join(","),
        () => {
          d.value = qt(U.value, d.value);
        },
      ),
        vt(
          () => U.value.id,
          () => {
            ((r.value = U.value.outfitTags[0] ?? ""),
              (d.value = {}),
              (u.value = ""),
              (f.value = null));
          },
        ));
      const w = z(null),
        O = ye(() =>
          U.value.garments.filter(
            (q) =>
              (!b.value || q.type === b.value) &&
              (!A.value || q.seasons.includes(A.value)),
          ),
        );
      function G() {
        if (l.value || o.value || !u.value.trim() || !r.value) return;
        const q = U.value,
          M = {
            id: f.value ?? Date.now(),
            name: u.value.trim(),
            tag: r.value,
            items: qt(q, d.value),
          };
        ((t.value = "save"),
          Xe("衣柜", M.name, () => {
            M.items = qt(q, M.items);
            const T = q.outfits.findIndex((H) => H.id === M.id);
            (T < 0 ? q.outfits.push(M) : (q.outfits[T] = M),
              (u.value = ""),
              (d.value = {}),
              (f.value = null));
          }));
      }
      function oe(q, M, T = !1) {
        if (!r.value) return;
        const H = d.value[q] || [];
        d.value[q] = T
          ? H.includes(M)
            ? H.filter((_e) => _e !== M)
            : [...H, M]
          : H.includes(M)
            ? []
            : [M];
      }
      function W(q) {
        ((f.value = q.id),
          (u.value = q.name),
          (r.value = q.tag),
          (d.value = JSON.parse(JSON.stringify(q.items))),
          (i.value = "为她搭配"));
      }
      function Y(q) {
        return xt
          .map(
            (M) =>
              M +
              "：" +
              ((q.items[M] || [])
                .map((T) => {
                  var H;
                  return (H = U.value.garments.find((_e) => _e.id === T)) ==
                    null
                    ? void 0
                    : H.name;
                })
                .filter(Boolean)
                .join("、") || "未选"),
          )
          .join(" / ");
      }
      function L(q) {
        if (l.value) return;
        const M = U.value;
        ((t.value = "wear"),
          (s.value = q.id),
          Xe("换装", q.name, () => {
            var T;
            ((M.wearing = qt(M, q.items)),
              (M.outfit =
                ((T = M.garments.find((H) => {
                  var _e;
                  return (_e = q.items.上衣) == null
                    ? void 0
                    : _e.includes(H.id);
                })) == null
                  ? void 0
                  : T.name) || q.name));
          }));
      }
      function F() {
        var H;
        if (l.value || !((H = w.value) != null && H.name.trim())) return;
        const q = U.value,
          M = JSON.parse(JSON.stringify(w.value)),
          T = q.garments.some((_e) => _e.id === M.id);
        ((t.value = "garment"),
          Xe("衣柜", M.name, () => {
            const _e = q.garments.findIndex((D) => D.id === M.id);
            (_e < 0 ? T || q.garments.push(M) : (q.garments[_e] = M),
              (q.wearing = qt(q, q.wearing)),
              q.outfits.forEach((D) => (D.items = qt(q, D.items))),
              (w.value = null));
          }));
      }
      function C(q) {
        const M = U.value,
          T = M.outfits
            .filter((H) => Object.values(H.items).some((_e) => _e.includes(q)))
            .map((H) => "「" + H.name + "」");
        ne(
          "删除衣服",
          T.length
            ? "以下方案引用了这件衣服：" +
                T.join("、") +
                "。确认后将从这些方案和当前穿着中移除。"
            : "确定删除这件衣服吗？",
          () => Hd(M, q),
        );
      }
      function se() {
        const q = p.value.trim(),
          M = U.value;
        if (!(!q || M.outfitTags.includes(q))) {
          if (_.value) {
            const T = _.value;
            ((M.outfitTags = M.outfitTags.map((H) => (H === T ? q : H))),
              M.outfits.forEach((H) => {
                H.tag === T && (H.tag = q);
              }),
              r.value === T && (r.value = q));
          } else M.outfitTags.push(q);
          ((p.value = ""), (_.value = ""));
        }
      }
      function be(q) {
        ne("删除标签", "删除「" + q + "」？方案将保留。", () => {
          const M = U.value;
          ((M.outfitTags = M.outfitTags.filter((T) => T !== q)),
            M.outfits.forEach((T) => {
              T.tag === q && (T.tag = "");
            }),
            r.value === q && (r.value = ""));
        });
      }
      return (q, M) => (
        c(),
        v(
          I,
          null,
          [
            n("div", lp, [
              (c(),
              v(
                I,
                null,
                te(["为她搭配", "方案库", "全部衣服"], (T) =>
                  n(
                    "button",
                    {
                      class: ke({ active: h(i) === T }),
                      onClick: (H) => (i.value = T),
                    },
                    m(T === "为她搭配" && h(U).id === 1 ? "为他搭配" : T),
                    11,
                    np,
                  ),
                ),
                64,
              )),
            ]),
            h(i) === "为她搭配"
              ? (c(),
                v(
                  I,
                  { key: 0 },
                  [
                    n("div", op, [
                      n("label", null, [
                        M[20] || (M[20] = K("先选标签 ", -1)),
                        re(
                          n(
                            "select",
                            {
                              "onUpdate:modelValue":
                                M[0] || (M[0] = (T) => (r.value = T)),
                            },
                            [
                              M[19] ||
                                (M[19] = n(
                                  "option",
                                  { value: "" },
                                  "请选择",
                                  -1,
                                )),
                              (c(!0),
                              v(
                                I,
                                null,
                                te(
                                  h(U).outfitTags,
                                  (T) => (c(), v("option", null, m(T), 1)),
                                ),
                                256,
                              )),
                            ],
                            512,
                          ),
                          [[pt, r.value]],
                        ),
                      ]),
                      M[21] ||
                        (M[21] = n(
                          "small",
                          null,
                          "标签可在方案库维护 · 每个部位选一件，配饰可多选",
                          -1,
                        )),
                    ]),
                    n("div", ip, [
                      r.value ? J("", !0) : (c(), v("p", rp, "请先选择标签")),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          h(xt),
                          (T) => (
                            c(),
                            v("details", ap, [
                              n("summary", null, [
                                K(m(T) + " ", 1),
                                n(
                                  "small",
                                  null,
                                  m(
                                    (d.value[T] || [])
                                      .map((H) => {
                                        var _e;
                                        return (_e = h(U).garments.find(
                                          (D) => D.id === H,
                                        )) == null
                                          ? void 0
                                          : _e.name;
                                      })
                                      .join("、") || "未选择",
                                  ),
                                  1,
                                ),
                              ]),
                              n("div", up, [
                                (c(!0),
                                v(
                                  I,
                                  null,
                                  te(
                                    h(U).garments.filter((H) => H.type === T),
                                    (H) => {
                                      var _e;
                                      return (
                                        c(),
                                        v(
                                          "button",
                                          {
                                            disabled: !r.value,
                                            title: r.value
                                              ? void 0
                                              : "请先选择标签",
                                            class: ke([
                                              "choice",
                                              {
                                                chosen:
                                                  (_e = d.value[T]) == null
                                                    ? void 0
                                                    : _e.includes(H.id),
                                              },
                                            ]),
                                            onClick: (D) =>
                                              oe(T, H.id, T === "配饰"),
                                          },
                                          m(H.name),
                                          11,
                                          dp,
                                        )
                                      );
                                    },
                                  ),
                                  256,
                                )),
                                h(U).garments.some((H) => H.type === T)
                                  ? J("", !0)
                                  : (c(), v("small", cp, "暂无衣服")),
                              ]),
                            ])
                          ),
                        ),
                        256,
                      )),
                    ]),
                    n("div", fp, [
                      M[22] || (M[22] = n("h3", null, "当前搭配", -1)),
                      n(
                        "p",
                        pp,
                        m(Y({ name: u.value, tag: r.value, items: d.value })),
                        1,
                      ),
                    ]),
                    o.value
                      ? (c(), v("p", mp, "方案名称已存在，请换一个名称。"))
                      : J("", !0),
                    u.value.trim()
                      ? J("", !0)
                      : (c(), v("p", vp, "请填写方案名称后保存。")),
                    n(
                      "form",
                      { class: "input-row", onSubmit: Ue(G, ["prevent"]) },
                      [
                        re(
                          n(
                            "input",
                            {
                              "onUpdate:modelValue":
                                M[1] || (M[1] = (T) => (u.value = T)),
                              placeholder: "方案名称",
                              "aria-label": "方案名称",
                            },
                            null,
                            512,
                          ),
                          [[he, u.value]],
                        ),
                        n(
                          "button",
                          {
                            class: "button primary",
                            disabled:
                              h(l) || o.value || !r.value || !u.value.trim(),
                          },
                          m(
                            h(l) && t.value === "save"
                              ? "处理中…"
                              : f.value
                                ? "保存修改"
                                : "保存为方案",
                          ),
                          9,
                          hp,
                        ),
                        n(
                          "button",
                          {
                            type: "button",
                            class: "button",
                            onClick:
                              M[2] ||
                              (M[2] = (T) => {
                                ((f.value = null),
                                  (d.value = {}),
                                  (u.value = ""));
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
              : h(i) === "方案库"
                ? (c(),
                  v(
                    I,
                    { key: 1 },
                    [
                      n("div", gp, [
                        M[23] ||
                          (M[23] = n(
                            "div",
                            { class: "row" },
                            [
                              n("b", null, "标签管理"),
                              n("small", null, "删除标签保留方案"),
                            ],
                            -1,
                          )),
                        n("div", yp, [
                          (c(!0),
                          v(
                            I,
                            null,
                            te(
                              h(U).outfitTags,
                              (T) => (
                                c(),
                                v("span", bp, [
                                  K(m(T) + " ", 1),
                                  n(
                                    "button",
                                    {
                                      onClick: (H) => {
                                        ((_.value = T), (p.value = T));
                                      },
                                    },
                                    "改名",
                                    8,
                                    kp,
                                  ),
                                  n(
                                    "button",
                                    { onClick: (H) => be(T) },
                                    "删除",
                                    8,
                                    _p,
                                  ),
                                ])
                              ),
                            ),
                            256,
                          )),
                        ]),
                        n(
                          "form",
                          { class: "input-row", onSubmit: Ue(se, ["prevent"]) },
                          [
                            re(
                              n(
                                "input",
                                {
                                  "onUpdate:modelValue":
                                    M[3] || (M[3] = (T) => (p.value = T)),
                                  placeholder: _.value
                                    ? "新标签名"
                                    : "添加标签",
                                  "aria-label": "标签名称",
                                },
                                null,
                                8,
                                wp,
                              ),
                              [[he, p.value]],
                            ),
                            n(
                              "button",
                              { class: "button", disabled: !p.value.trim() },
                              m(_.value ? "保存标签" : "添加"),
                              9,
                              $p,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          h(U).outfits,
                          (T) => (
                            c(),
                            v("article", { key: T.id, class: "card compact" }, [
                              n("div", Cp, [
                                P.value === T.id
                                  ? (c(),
                                    v(
                                      "form",
                                      {
                                        key: 0,
                                        class: "input-row",
                                        onSubmit: Ue((H) => ce(T), ["prevent"]),
                                      },
                                      [
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                M[4] ||
                                                (M[4] = (H) => (V.value = H)),
                                              "aria-label": "方案新名称",
                                              autofocus: "",
                                            },
                                            null,
                                            512,
                                          ),
                                          [[he, V.value]],
                                        ),
                                        n(
                                          "button",
                                          {
                                            class: "button",
                                            disabled: h(l) || !V.value.trim(),
                                          },
                                          m(
                                            h(l) && t.value === "rename"
                                              ? "处理中…"
                                              : "保存",
                                          ),
                                          9,
                                          Sp,
                                        ),
                                        n(
                                          "button",
                                          {
                                            type: "button",
                                            class: "text-button",
                                            onClick:
                                              M[5] ||
                                              (M[5] = (H) => (P.value = null)),
                                          },
                                          "取消",
                                        ),
                                      ],
                                      40,
                                      xp,
                                    ))
                                  : (c(), v("b", Tp, m(T.name), 1)),
                                n("span", Mp, m(T.tag || "未分类"), 1),
                                M[25] ||
                                  (M[25] = n(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                n(
                                  "details",
                                  {
                                    class: "actions-menu",
                                    onClick:
                                      M[6] ||
                                      (M[6] = (H) =>
                                        H.target.closest("button") &&
                                        H.currentTarget.removeAttribute(
                                          "open",
                                        )),
                                  },
                                  [
                                    M[24] ||
                                      (M[24] = n(
                                        "summary",
                                        { "aria-label": "方案操作" },
                                        "⋯",
                                        -1,
                                      )),
                                    n("div", Ip, [
                                      n(
                                        "button",
                                        {
                                          onClick: (H) =>
                                            (a.value =
                                              a.value === T.id ? null : T.id),
                                        },
                                        "预览",
                                        8,
                                        Ep,
                                      ),
                                      n(
                                        "button",
                                        { onClick: (H) => W(T) },
                                        "编辑搭配",
                                        8,
                                        Ap,
                                      ),
                                      n(
                                        "button",
                                        {
                                          onClick: (H) => {
                                            ((P.value = T.id),
                                              (V.value = T.name));
                                          },
                                        },
                                        "重命名",
                                        8,
                                        Vp,
                                      ),
                                      n(
                                        "button",
                                        {
                                          onClick: (H) =>
                                            ne(
                                              "删除方案",
                                              "确定删除「" + T.name + "」？",
                                              () =>
                                                (h(U).outfits = h(
                                                  U,
                                                ).outfits.filter(
                                                  (_e) => _e.id !== T.id,
                                                )),
                                            ),
                                        },
                                        "删除",
                                        8,
                                        Op,
                                      ),
                                    ]),
                                  ],
                                ),
                                n(
                                  "button",
                                  {
                                    class: "button",
                                    disabled: h(l),
                                    onClick: (H) => L(T),
                                  },
                                  m(
                                    h(l) &&
                                      t.value === "wear" &&
                                      s.value === T.id
                                      ? "处理中…"
                                      : h(U).id === 1
                                        ? "请他换上"
                                        : "请她换上",
                                  ),
                                  9,
                                  Up,
                                ),
                              ]),
                              a.value === T.id
                                ? (c(), v("p", Pp, m(Y(T)), 1))
                                : J("", !0),
                            ])
                          ),
                        ),
                        128,
                      )),
                      h(U).outfits.length
                        ? J("", !0)
                        : (c(), v("p", Lp, "还没有方案，去搭配一套吧。")),
                    ],
                    64,
                  ))
                : (c(),
                  v(
                    I,
                    { key: 2 },
                    [
                      n("div", Np, [
                        re(
                          n(
                            "select",
                            {
                              "onUpdate:modelValue":
                                M[7] || (M[7] = (T) => (b.value = T)),
                              "aria-label": "按类型筛选",
                            },
                            [
                              M[26] ||
                                (M[26] = n(
                                  "option",
                                  { value: "" },
                                  "全部类型",
                                  -1,
                                )),
                              (c(!0),
                              v(
                                I,
                                null,
                                te(
                                  h(xt),
                                  (T) => (c(), v("option", null, m(T), 1)),
                                ),
                                256,
                              )),
                            ],
                            512,
                          ),
                          [[pt, b.value]],
                        ),
                        re(
                          n(
                            "select",
                            {
                              "onUpdate:modelValue":
                                M[8] || (M[8] = (T) => (A.value = T)),
                              "aria-label": "按季节筛选",
                            },
                            [
                              M[27] ||
                                (M[27] = n(
                                  "option",
                                  { value: "" },
                                  "全部季节",
                                  -1,
                                )),
                              (c(),
                              v(
                                I,
                                null,
                                te(["春", "夏", "秋", "冬"], (T) =>
                                  n("option", null, m(T), 1),
                                ),
                                64,
                              )),
                            ],
                            512,
                          ),
                          [[pt, A.value]],
                        ),
                        M[28] ||
                          (M[28] = n("span", { class: "spacer" }, null, -1)),
                        n(
                          "button",
                          {
                            class: "button",
                            onClick:
                              M[9] ||
                              (M[9] = (T) =>
                                (w.value = {
                                  id: Date.now(),
                                  name: "",
                                  type: "上衣",
                                  seasons: ["秋"],
                                })),
                          },
                          "添加衣服",
                        ),
                      ]),
                      n("div", Rp, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            O.value,
                            (T) => (
                              c(),
                              v(
                                "article",
                                { key: T.id, class: "card garment-card" },
                                [
                                  n(
                                    "div",
                                    {
                                      class: ke([
                                        "garment-thumb",
                                        "garment-type-" + T.type,
                                      ]),
                                    },
                                    [
                                      ae(sp, { type: T.type }, null, 8, [
                                        "type",
                                      ]),
                                      n("small", null, m(T.type), 1),
                                    ],
                                    2,
                                  ),
                                  n("div", Fp, [
                                    n("div", null, [
                                      n("h3", null, m(T.name), 1),
                                      n(
                                        "small",
                                        null,
                                        m(T.seasons.join(" / ")),
                                        1,
                                      ),
                                    ]),
                                    M[30] ||
                                      (M[30] = n(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    n(
                                      "details",
                                      {
                                        class: "actions-menu",
                                        onClick:
                                          M[10] ||
                                          (M[10] = (H) =>
                                            H.target.closest("button") &&
                                            H.currentTarget.removeAttribute(
                                              "open",
                                            )),
                                      },
                                      [
                                        M[29] ||
                                          (M[29] = n(
                                            "summary",
                                            { "aria-label": "衣服操作" },
                                            "⋯",
                                            -1,
                                          )),
                                        n("div", jp, [
                                          n(
                                            "button",
                                            {
                                              onClick: (H) =>
                                                (w.value = JSON.parse(
                                                  JSON.stringify(T),
                                                )),
                                            },
                                            "编辑",
                                            8,
                                            Dp,
                                          ),
                                          n(
                                            "button",
                                            { onClick: (H) => C(T.id) },
                                            "删除",
                                            8,
                                            Hp,
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
                        O.value.length
                          ? J("", !0)
                          : (c(), v("p", Bp, "没有符合条件的衣服。")),
                      ]),
                    ],
                    64,
                  )),
            w.value
              ? (c(),
                v(
                  "div",
                  {
                    key: 3,
                    class: "drawer-backdrop",
                    onClick:
                      M[15] || (M[15] = Ue((T) => (w.value = null), ["self"])),
                  },
                  [
                    n(
                      "form",
                      {
                        "data-testid": "garment-drawer",
                        class: "detail-drawer form-grid",
                        role: "dialog",
                        "aria-modal": "true",
                        "aria-label": "衣服",
                        onSubmit: Ue(F, ["prevent"]),
                      },
                      [
                        ae(St, { labels: ["衣柜"] }),
                        n(
                          "h2",
                          null,
                          m(
                            h(U).garments.some((T) => {
                              var H;
                              return (
                                T.id === ((H = w.value) == null ? void 0 : H.id)
                              );
                            })
                              ? "编辑衣服"
                              : "添加衣服",
                          ),
                          1,
                        ),
                        n("label", null, [
                          M[31] || (M[31] = K("名称", -1)),
                          re(
                            n(
                              "input",
                              {
                                "onUpdate:modelValue":
                                  M[11] || (M[11] = (T) => (w.value.name = T)),
                              },
                              null,
                              512,
                            ),
                            [[he, w.value.name]],
                          ),
                          w.value.name.trim()
                            ? J("", !0)
                            : (c(), v("small", Kp, "请填写衣服名称。")),
                        ]),
                        n("label", null, [
                          M[32] || (M[32] = K("类型", -1)),
                          re(
                            n(
                              "select",
                              {
                                "onUpdate:modelValue":
                                  M[12] || (M[12] = (T) => (w.value.type = T)),
                              },
                              [
                                (c(!0),
                                v(
                                  I,
                                  null,
                                  te(
                                    h(xt),
                                    (T) => (c(), v("option", null, m(T), 1)),
                                  ),
                                  256,
                                )),
                              ],
                              512,
                            ),
                            [[pt, w.value.type]],
                          ),
                        ]),
                        n("div", zp, [
                          (c(),
                          v(
                            I,
                            null,
                            te(["春", "夏", "秋", "冬"], (T) =>
                              n("label", null, [
                                re(
                                  n(
                                    "input",
                                    {
                                      "onUpdate:modelValue":
                                        M[13] ||
                                        (M[13] = (H) => (w.value.seasons = H)),
                                      type: "checkbox",
                                      value: T,
                                    },
                                    null,
                                    8,
                                    qp,
                                  ),
                                  [[Ft, w.value.seasons]],
                                ),
                                K(m(T), 1),
                              ]),
                            ),
                            64,
                          )),
                        ]),
                        n("div", Wp, [
                          n(
                            "button",
                            {
                              class: "button primary",
                              disabled: h(l) || !w.value.name.trim(),
                            },
                            m(
                              h(l) && t.value === "garment"
                                ? "处理中…"
                                : "保存衣服",
                            ),
                            9,
                            Jp,
                          ),
                          n(
                            "button",
                            {
                              type: "button",
                              class: "button",
                              onClick:
                                M[14] || (M[14] = (T) => (w.value = null)),
                            },
                            "取消",
                          ),
                        ]),
                      ],
                      32,
                    ),
                  ],
                ))
              : J("", !0),
            le.value
              ? (c(),
                v(
                  "div",
                  {
                    key: 4,
                    class: "modal-backdrop",
                    onClick:
                      M[18] || (M[18] = Ue((T) => (le.value = null), ["self"])),
                  },
                  [
                    n(
                      "section",
                      {
                        class: "card confirm-dialog",
                        role: "alertdialog",
                        "aria-modal": "true",
                        "aria-label": le.value.title,
                      },
                      [
                        n("h2", null, m(le.value.title), 1),
                        n("p", null, m(le.value.message), 1),
                        n("div", Yp, [
                          n(
                            "button",
                            {
                              class: "button",
                              onClick:
                                M[16] || (M[16] = (T) => (le.value = null)),
                            },
                            "取消",
                          ),
                          n(
                            "button",
                            {
                              class: "button primary",
                              onClick:
                                M[17] ||
                                (M[17] = (T) => {
                                  (le.value.run(), (le.value = null));
                                }),
                            },
                            "确认删除",
                          ),
                        ]),
                      ],
                      8,
                      Gp,
                    ),
                  ],
                ))
              : J("", !0),
            w.value
              ? J("", !0)
              : (c(), Se(St, { key: 5, labels: ["衣柜", "换装"] })),
          ],
          64,
        )
      );
    },
  }),
  Qp = { class: "world-list" },
  Xp = { class: "row" },
  em = { key: 0, class: "pill" },
  tm = { class: "muted" },
  sm = { class: "row" },
  lm = ["onClick"],
  nm = ["onClick"],
  om = ["onClick"],
  im = { class: "row" },
  rm = { class: "card world-header" },
  am = { class: "row" },
  um = { class: "toolbar" },
  dm = ["disabled"],
  cm = {
    key: 0,
    class: "card empty-state",
    "data-testid": "world-empty-sites",
  },
  fm = { class: "row" },
  pm = ["onClick"],
  mm = { key: 0, class: "muted" },
  vm = { class: "place-grid" },
  hm = { class: "row" },
  gm = ["onClick"],
  ym = { key: 0, class: "inline-error" },
  bm = ["onUpdate:modelValue"],
  km = ["disabled"],
  _m = { key: 0, class: "inline-error" },
  wm = ["disabled"],
  $m = { key: 0 },
  Cm = ["disabled"],
  xm = { key: 5, class: "modal-backdrop" },
  Sm = { class: "card confirm-dialog" },
  Tm = je({
    __name: "WorldPage",
    setup(e) {
      const t = z(null),
        s = ye(() => Oe.value.find((w) => w.id === t.value)),
        l = z(!1),
        o = z(""),
        i = z(null),
        r = z(0),
        u = z(""),
        d = z(!1),
        f = fs(["世界"]),
        a = z(null),
        p = z(null),
        _ = z("");
      function b() {
        a.value = { id: Date.now(), name: "", genre: "现代都市" };
      }
      const A = [
        ["alias", "别名"],
        ["floor", "楼层"],
        ["description", "描述"],
        ["atmosphere", "氛围"],
        ["furniture", "固定陈设"],
        ["layout", "布局描述"],
      ];
      function P() {
        var O;
        if (!((O = a.value) != null && O.name.trim()) || f.value) return;
        const w = { ...a.value };
        ((_.value = "world"),
          Xe(
            "世界",
            w.name + " " + w.genre,
            () => {
              const G = Oe.value.find((oe) => oe.id === w.id);
              (G
                ? Object.assign(G, w)
                : Oe.value.push({ ...w, residents: [], sites: [] }),
                (t.value = w.id),
                (l.value = !0),
                (a.value = null));
            },
            w.name,
          ));
      }
      function V() {
        const w = s.value;
        !w ||
          f.value ||
          ((_.value = "expand"),
          Xe("世界", o.value || "河岸茶馆", () => {
            (w.sites.length || w.sites.push({ name: "街区", places: [] }),
              w.sites[0].places.push({
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
      function le(w, O) {
        ((r.value = O), (i.value = { ...w }));
      }
      function ne() {
        var G;
        if (!((G = i.value) != null && G.name.trim()) || f.value) return;
        const w = { ...i.value },
          O = s.value.sites[r.value];
        ((_.value = "place"),
          Xe(
            "世界",
            Object.values(w).join(" "),
            () => {
              const oe = O.places.findIndex((W) => W.id === w.id);
              (oe < 0 ? O.places.push(w) : (O.places[oe] = w),
                (i.value = null));
            },
            w.name,
          ));
      }
      function ce() {
        const w = s.value,
          O = u.value.trim();
        !w ||
          !O ||
          f.value ||
          ((_.value = "site"),
          Xe("世界", O, () => {
            (w.sites.push({ name: O, places: [] }),
              (u.value = ""),
              (d.value = !1));
          }));
      }
      return (w, O) => {
        var G, oe;
        return (
          c(),
          v(
            I,
            null,
            [
              s.value
                ? (c(),
                  v(
                    I,
                    { key: 1 },
                    [
                      n("div", im, [
                        n(
                          "button",
                          {
                            class: "text-button",
                            onClick: O[0] || (O[0] = (W) => (t.value = null)),
                          },
                          "世界列表",
                        ),
                        n("span", null, " / " + m(s.value.name), 1),
                        O[16] ||
                          (O[16] = n("span", { class: "spacer" }, null, -1)),
                        n(
                          "button",
                          {
                            class: "button",
                            onClick: O[1] || (O[1] = (W) => (d.value = !0)),
                          },
                          "新增场地",
                        ),
                      ]),
                      n("div", rm, [
                        n("div", am, [
                          O[17] ||
                            (O[17] = n(
                              "span",
                              { class: "eyebrow" },
                              "A WORLD OF YOUR OWN",
                              -1,
                            )),
                          O[18] ||
                            (O[18] = n("span", { class: "spacer" }, null, -1)),
                          n(
                            "button",
                            {
                              class: "button",
                              onClick:
                                O[2] ||
                                (O[2] = (W) =>
                                  (a.value = {
                                    id: s.value.id,
                                    name: s.value.name,
                                    genre: s.value.genre,
                                  })),
                            },
                            "编辑世界信息",
                          ),
                          n(
                            "button",
                            {
                              class: "button",
                              onClick:
                                O[3] || (O[3] = (W) => (l.value = !l.value)),
                            },
                            m(l.value ? "结束编辑" : "编辑地点"),
                            1,
                          ),
                        ]),
                        n("h2", null, m(s.value.name), 1),
                        n("p", null, m(s.value.genre), 1),
                      ]),
                      n("div", um, [
                        re(
                          n(
                            "input",
                            {
                              "onUpdate:modelValue":
                                O[4] || (O[4] = (W) => (o.value = W)),
                              placeholder: "补充扩展方向，例如：河岸书店",
                            },
                            null,
                            512,
                          ),
                          [[he, o.value]],
                        ),
                        n(
                          "button",
                          { class: "button", disabled: h(f), onClick: V },
                          m(
                            h(f) && _.value === "expand"
                              ? "处理中…"
                              : "AI 扩展世界",
                          ),
                          9,
                          dm,
                        ),
                      ]),
                      s.value.sites.length
                        ? J("", !0)
                        : (c(),
                          v("div", cm, [
                            ...(O[19] ||
                              (O[19] = [n("p", null, "暂无场地", -1)])),
                          ])),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          s.value.sites,
                          (W, Y) => (
                            c(),
                            v("section", { key: Y, class: "card compact" }, [
                              n("div", fm, [
                                n("h3", null, "场地 · " + m(W.name), 1),
                                O[20] ||
                                  (O[20] = n(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                l.value
                                  ? (c(),
                                    v(
                                      "button",
                                      {
                                        key: 0,
                                        class: "text-button",
                                        onClick: (L) => {
                                          ((r.value = Y),
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
                                      pm,
                                    ))
                                  : J("", !0),
                              ]),
                              W.places.length
                                ? J("", !0)
                                : (c(), v("p", mm, "暂无地点")),
                              n("div", vm, [
                                (c(!0),
                                v(
                                  I,
                                  null,
                                  te(
                                    W.places,
                                    (L) => (
                                      c(),
                                      v(
                                        "article",
                                        { key: L.id, class: "place" },
                                        [
                                          n("div", hm, [
                                            n(
                                              "b",
                                              null,
                                              m(L.alias || L.name),
                                              1,
                                            ),
                                            n("small", null, m(L.floor), 1),
                                          ]),
                                          n(
                                            "p",
                                            null,
                                            m(L.description) +
                                              " · " +
                                              m(L.atmosphere),
                                            1,
                                          ),
                                          n(
                                            "small",
                                            null,
                                            "固定陈设：" + m(L.furniture),
                                            1,
                                          ),
                                          n(
                                            "small",
                                            null,
                                            "布局：" + m(L.layout),
                                            1,
                                          ),
                                          l.value
                                            ? (c(),
                                              v(
                                                "button",
                                                {
                                                  key: 0,
                                                  class: "button edit-place",
                                                  onClick: (F) => le(L, Y),
                                                },
                                                "编辑地点",
                                                8,
                                                gm,
                                              ))
                                            : J("", !0),
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
                : (c(),
                  v(
                    I,
                    { key: 0 },
                    [
                      n("div", { class: "row" }, [
                        O[14] ||
                          (O[14] = n(
                            "div",
                            null,
                            [
                              n("h2", null, "世界列表"),
                              n(
                                "p",
                                { class: "muted" },
                                "每个人，都在自己的风景里生活。",
                              ),
                            ],
                            -1,
                          )),
                        O[15] ||
                          (O[15] = n("span", { class: "spacer" }, null, -1)),
                        n(
                          "button",
                          {
                            class: "button",
                            "data-testid": "world-add",
                            onClick: b,
                          },
                          "添加世界",
                        ),
                      ]),
                      n("div", Qp, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            h(Oe),
                            (W) => (
                              c(),
                              v(
                                "article",
                                {
                                  key: W.id,
                                  class: ke([
                                    "card",
                                    { current: W.residents.includes(h(U).id) },
                                  ]),
                                },
                                [
                                  n("div", Xp, [
                                    n("h2", null, m(W.name), 1),
                                    W.residents.includes(h(U).id)
                                      ? (c(), v("span", em, "当前"))
                                      : J("", !0),
                                  ]),
                                  n(
                                    "p",
                                    null,
                                    m(W.genre) +
                                      " · " +
                                      m(W.sites.length) +
                                      " 处场地",
                                    1,
                                  ),
                                  n(
                                    "p",
                                    tm,
                                    "居住在这里：" +
                                      m(
                                        W.residents
                                          .map((Y) => {
                                            var L;
                                            return (L = h(Ae)[Y]) == null
                                              ? void 0
                                              : L.name;
                                          })
                                          .join("、") || "暂无角色",
                                      ),
                                    1,
                                  ),
                                  n("div", sm, [
                                    n(
                                      "button",
                                      {
                                        class: "button primary",
                                        onClick: (Y) => {
                                          ((t.value = W.id), (l.value = !1));
                                        },
                                      },
                                      "进入",
                                      8,
                                      lm,
                                    ),
                                    n(
                                      "button",
                                      {
                                        class: "text-button",
                                        onClick: (Y) => {
                                          ((t.value = W.id), (l.value = !0));
                                        },
                                      },
                                      "编辑",
                                      8,
                                      nm,
                                    ),
                                    n(
                                      "button",
                                      {
                                        class: "text-button",
                                        onClick: (Y) => (p.value = W.id),
                                      },
                                      "删除",
                                      8,
                                      om,
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
                ? (c(),
                  Se(
                    Ks,
                    {
                      key: 2,
                      title:
                        (G = s.value) != null &&
                        G.sites.some((W) =>
                          W.places.some((Y) => {
                            var L;
                            return (
                              Y.id === ((L = i.value) == null ? void 0 : L.id)
                            );
                          }),
                        )
                          ? "编辑地点"
                          : "添加地点",
                      "test-id": "place-drawer",
                      onClose: O[6] || (O[6] = (W) => (i.value = null)),
                    },
                    {
                      default: Gt(() => [
                        n(
                          "form",
                          { class: "form-grid", onSubmit: Ue(ne, ["prevent"]) },
                          [
                            ae(St, { labels: ["世界"] }),
                            n("label", null, [
                              O[21] || (O[21] = K("地点名称", -1)),
                              re(
                                n(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      O[5] ||
                                      (O[5] = (W) => (i.value.name = W)),
                                  },
                                  null,
                                  512,
                                ),
                                [[he, i.value.name]],
                              ),
                              i.value.name.trim()
                                ? J("", !0)
                                : (c(), v("small", ym, "请填写地点名称。")),
                            ]),
                            (c(),
                            v(
                              I,
                              null,
                              te(A, ([W, Y]) =>
                                n("label", { key: W }, [
                                  K(m(Y), 1),
                                  re(
                                    n(
                                      "input",
                                      {
                                        "onUpdate:modelValue": (L) =>
                                          (i.value[W] = L),
                                      },
                                      null,
                                      8,
                                      bm,
                                    ),
                                    [[he, i.value[W]]],
                                  ),
                                ]),
                              ),
                              64,
                            )),
                            n(
                              "button",
                              {
                                class: "button primary",
                                disabled: h(f) || !i.value.name.trim(),
                              },
                              m(
                                h(f) && _.value === "place"
                                  ? "处理中…"
                                  : "保存地点",
                              ),
                              9,
                              km,
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
                : J("", !0),
              a.value
                ? (c(),
                  Se(
                    Ks,
                    {
                      key: 3,
                      title: h(Oe).some((W) => {
                        var Y;
                        return W.id === ((Y = a.value) == null ? void 0 : Y.id);
                      })
                        ? "编辑世界"
                        : "添加世界",
                      "test-id": "world-drawer",
                      onClose: O[9] || (O[9] = (W) => (a.value = null)),
                    },
                    {
                      default: Gt(() => [
                        n(
                          "form",
                          { class: "form-grid", onSubmit: Ue(P, ["prevent"]) },
                          [
                            ae(St, { labels: ["世界"] }),
                            n("label", null, [
                              O[22] || (O[22] = K("世界名称", -1)),
                              re(
                                n(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      O[7] ||
                                      (O[7] = (W) => (a.value.name = W)),
                                    "data-testid": "world-name",
                                  },
                                  null,
                                  512,
                                ),
                                [[he, a.value.name]],
                              ),
                              a.value.name.trim()
                                ? J("", !0)
                                : (c(), v("small", _m, "请填写世界名称。")),
                            ]),
                            n("label", null, [
                              O[23] || (O[23] = K("世界类型", -1)),
                              re(
                                n(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      O[8] ||
                                      (O[8] = (W) => (a.value.genre = W)),
                                    "data-testid": "world-genre",
                                  },
                                  null,
                                  512,
                                ),
                                [[he, a.value.genre]],
                              ),
                            ]),
                            n(
                              "button",
                              {
                                class: "button primary",
                                disabled: h(f) || !a.value.name.trim(),
                                "data-testid": "world-save",
                              },
                              m(
                                h(f) && _.value === "world"
                                  ? "处理中…"
                                  : "保存世界",
                              ),
                              9,
                              wm,
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
                : J("", !0),
              d.value
                ? (c(),
                  Se(
                    Ks,
                    {
                      key: 4,
                      title: "添加场地",
                      "test-id": "site-drawer",
                      onClose: O[11] || (O[11] = (W) => (d.value = !1)),
                    },
                    {
                      default: Gt(() => [
                        n(
                          "form",
                          { class: "form-grid", onSubmit: Ue(ce, ["prevent"]) },
                          [
                            ae(St, { labels: ["世界"] }),
                            n("label", null, [
                              O[24] || (O[24] = K("名称", -1)),
                              re(
                                n(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      O[10] || (O[10] = (W) => (u.value = W)),
                                  },
                                  null,
                                  512,
                                ),
                                [[he, u.value]],
                              ),
                              u.value.trim()
                                ? J("", !0)
                                : (c(), v("small", $m, "请填写场地名称。")),
                            ]),
                            n(
                              "button",
                              {
                                class: "button primary",
                                disabled: h(f) || !u.value.trim(),
                              },
                              m(
                                h(f) && _.value === "site"
                                  ? "处理中…"
                                  : "保存场地",
                              ),
                              9,
                              Cm,
                            ),
                          ],
                          32,
                        ),
                      ]),
                      _: 1,
                    },
                  ))
                : J("", !0),
              p.value !== null
                ? (c(),
                  v("div", xm, [
                    n("section", Sm, [
                      O[25] || (O[25] = n("h2", null, "删除世界？", -1)),
                      n(
                        "p",
                        null,
                        "「" +
                          m(
                            (oe = h(Oe).find((W) => W.id === p.value)) == null
                              ? void 0
                              : oe.name,
                          ) +
                          "」及其中的场地将被移除。",
                        1,
                      ),
                      n(
                        "button",
                        {
                          class: "button",
                          onClick: O[12] || (O[12] = (W) => (p.value = null)),
                        },
                        "取消",
                      ),
                      n(
                        "button",
                        {
                          class: "button primary",
                          onClick:
                            O[13] ||
                            (O[13] = (W) => {
                              ((Oe.value = h(Oe).filter(
                                (Y) => Y.id !== p.value,
                              )),
                                (p.value = null));
                            }),
                        },
                        "确认删除",
                      ),
                    ]),
                  ]))
                : J("", !0),
              !i.value && !a.value && !d.value
                ? (c(), Se(St, { key: 6, labels: ["世界"] }))
                : J("", !0),
            ],
            64,
          )
        );
      };
    },
  }),
  Mm = { class: "row" },
  Im = { class: "avatar small" },
  Em = { class: "post-art" },
  Am = { class: "row" },
  Vm = ["onClick"],
  Om = ["data-testid", "onClick"],
  Um = { key: 0, class: "comments" },
  Pm = { class: "avatar small" },
  Lm = ["onSubmit"],
  Nm = ["onUpdate:modelValue", "data-testid", "onKeydown"],
  Rm = ["disabled"],
  Fm = { class: "muted" },
  jm = { class: "people-grid" },
  Dm = ["onClick"],
  Hm = { class: "avatar" },
  Bm = { key: 0, class: "card empty-state" },
  Km = ["aria-label"],
  zm = { class: "muted" },
  qm = { class: "media-grid" },
  Wm = { class: "card media-tile" },
  Jm = ["onClick"],
  Gm = { key: 0, class: "audio-control" },
  Ym = ["onClick"],
  Zm = ["value"],
  Qm = {
    "data-testid": "media-preview-drawer",
    class: "card media-modal",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": "媒体预览",
  },
  Xm = { class: "row" },
  e1 = { key: 1, class: "card empty-state" },
  t1 = { class: "row" },
  s1 = { class: "pill" },
  l1 = ["onClick"],
  n1 = { key: 0, class: "pill current-badge" },
  o1 = ["onClick"],
  i1 = { key: 0, class: "inline-profile" },
  r1 = { key: 0 },
  a1 = { key: 4, class: "card placeholder" },
  u1 = ["onClick"],
  d1 = { class: "pill" },
  c1 = ["data-testid", "onClick"],
  f1 = { key: 0, class: "muted" },
  p1 = je({
    __name: "BrowsePage",
    props: { page: {} },
    setup(e) {
      const t = fs(["动态"]),
        s = z(null),
        l = z({}),
        o = setInterval(() => {
          if (s.value === null) return;
          const L = s.value;
          ((l.value[L] = (l.value[L] || 0) + 2),
            l.value[L] >= 100 && ((l.value[L] = 0), (s.value = null)));
        }, 500);
      ll(() => clearInterval(o));
      const i = ye(() =>
        [...vr.value, ...$s.value].filter((L) => L.status === "error"),
      );
      function r(L) {
        ((Zn.value = "运行诊断"),
          (In.value = L),
          (location.hash = encodeURIComponent("管理中心")));
      }
      function u(L) {
        ((lt.value = L), (location.hash = encodeURIComponent("聊天")));
      }
      const d = e,
        f = z(""),
        a = z({
          status: "",
          tag: "",
          scene: "",
          type: "",
          who: d.page === "媒体" ? U.value.name : "",
        }),
        p = z(null),
        _ = z({}),
        b = z({}),
        A = ye(() => [
          { key: "status", label: "状态", options: ["活跃", "暂未联系"] },
          { key: "tag", label: "标签", options: ["日常", "工作", "旧友"] },
          {
            key: "scene",
            label: "场景",
            options: U.value.sites.map((L) => L.name),
          },
        ]),
        P = ye(() => [
          { key: "type", label: "类型", options: ["图片", "语音", "视频"] },
          { key: "tag", label: "标签", options: ["日常", "风景", "创作"] },
          { key: "who", label: "角色", options: Ae.value.map((L) => L.name) },
        ]),
        V = [
          {
            key: "tag",
            label: "标签",
            options: ["温柔", "创作", "沉稳", "城市", "自然"],
          },
        ];
      function le() {
        ((f.value = ""),
          (a.value = { status: "", tag: "", scene: "", type: "", who: "" }));
      }
      const ne = z(null);
      function ce(L, F) {
        L.key === "Enter" &&
          !L.shiftKey &&
          !L.isComposing &&
          (L.preventDefault(), Y(F));
      }
      const w = (L) => L.toLowerCase().includes(f.value.trim().toLowerCase()),
        O = ye(() =>
          U.value.people.filter(
            (L) =>
              w(L.name + " " + L.identity) &&
              (!a.value.status || L.status === a.value.status) &&
              (!a.value.tag || L.tag === a.value.tag) &&
              (!a.value.scene || L.location === a.value.scene),
          ),
        ),
        G = ye(() => U.value.people.find((L) => L.id === p.value)),
        oe = ye(() =>
          Ae.value.filter(
            (L) =>
              w(L.name + " " + L.job + " " + L.bio) &&
              (!a.value.tag || L.tags.includes(a.value.tag)),
          ),
        ),
        W = ye(() =>
          ei.filter(
            (L) =>
              w(L.title + " " + L.character) &&
              (!a.value.type || L.type === a.value.type) &&
              (!a.value.who || L.character === a.value.who) &&
              (!a.value.tag || L.tag === a.value.tag),
          ),
        );
      function Y(L) {
        var se;
        const F = (se = _.value[L]) == null ? void 0 : se.trim();
        if (!F || t.value) return;
        ne.value = L;
        const C = U.value.posts[L];
        (Xe("动态", F, () =>
          C.comments.push({ name: "我", text: F, time: "刚刚" }),
        ),
          (_.value[L] = ""));
      }
      return (L, F) =>
        e.page === "动态"
          ? (c(),
            v(
              I,
              { key: 0 },
              [
                F[13] ||
                  (F[13] = n(
                    "p",
                    { class: "muted" },
                    "生活的切片，留一点位置给彼此的回应。",
                    -1,
                  )),
                (c(!0),
                v(
                  I,
                  null,
                  te(h(U).posts, (C, se) => {
                    var be;
                    return (
                      c(),
                      v("article", { key: se, class: "card post compact" }, [
                        n("div", Mm, [
                          n("span", Im, m(h(U).initial), 1),
                          n("b", null, m(h(U).name), 1),
                          n("small", null, m(C.time), 1),
                        ]),
                        n("p", null, m(C.text), 1),
                        n("div", Em, [
                          ae(h(Gn), { size: 36 }),
                          F[10] ||
                            (F[10] = n("span", null, "把日子过成一首小诗", -1)),
                        ]),
                        n("div", Am, [
                          n(
                            "button",
                            {
                              class: "text-button",
                              onClick: (q) => {
                                ((C.liked = !C.liked),
                                  (C.likes += C.liked ? 1 : -1));
                              },
                            },
                            m(C.liked ? "♥" : "♡") + " " + m(C.likes) + " 喜欢",
                            9,
                            Vm,
                          ),
                          n(
                            "button",
                            {
                              "data-testid": "comment-toggle-" + se,
                              class: "text-button",
                              onClick: (q) => (b.value[se] = !b.value[se]),
                            },
                            "评论 " +
                              m(C.comments.length) +
                              " " +
                              m(b.value[se] ? "收起" : "展开"),
                            9,
                            Om,
                          ),
                        ]),
                        b.value[se]
                          ? (c(),
                            v("div", Um, [
                              (c(!0),
                              v(
                                I,
                                null,
                                te(
                                  C.comments,
                                  (q, M) => (
                                    c(),
                                    v("p", { key: M }, [
                                      n("span", Pm, m(q.name.slice(0, 1)), 1),
                                      n("span", null, [
                                        n("b", null, m(q.name), 1),
                                        F[11] || (F[11] = K()),
                                        n("small", null, m(q.time), 1),
                                        F[12] ||
                                          (F[12] = n("br", null, null, -1)),
                                        K(m(q.text), 1),
                                      ]),
                                    ])
                                  ),
                                ),
                                128,
                              )),
                              n(
                                "form",
                                {
                                  class: "input-row",
                                  onSubmit: Ue((q) => Y(se), ["prevent"]),
                                },
                                [
                                  re(
                                    n(
                                      "textarea",
                                      {
                                        "onUpdate:modelValue": (q) =>
                                          (_.value[se] = q),
                                        placeholder: "留下一句回应…",
                                        "aria-label": "评论内容",
                                        rows: "2",
                                        "data-testid": "comment-input-" + se,
                                        onKeydown: (q) => ce(q, se),
                                      },
                                      null,
                                      40,
                                      Nm,
                                    ),
                                    [[he, _.value[se]]],
                                  ),
                                  n(
                                    "button",
                                    {
                                      class: "button",
                                      disabled:
                                        h(t) ||
                                        !(
                                          (be = _.value[se]) != null &&
                                          be.trim()
                                        ),
                                    },
                                    m(
                                      h(t) && ne.value === se
                                        ? "处理中…"
                                        : "发表评论",
                                    ),
                                    9,
                                    Rm,
                                  ),
                                ],
                                40,
                                Lm,
                              ),
                            ]))
                          : J("", !0),
                      ])
                    );
                  }),
                  128,
                )),
                ae(St, { labels: ["动态"] }),
              ],
              64,
            ))
          : e.page === "人物关系"
            ? (c(),
              v(
                I,
                { key: 1 },
                [
                  ae(
                    bs,
                    {
                      search: f.value,
                      "onUpdate:search": F[0] || (F[0] = (C) => (f.value = C)),
                      filters: a.value,
                      "onUpdate:filters": F[1] || (F[1] = (C) => (a.value = C)),
                      groups: A.value,
                      "test-id": "people-filters",
                      placeholder: "搜索姓名 / 身份",
                    },
                    null,
                    8,
                    ["search", "filters", "groups"],
                  ),
                  n("small", Fm, m(O.value.length) + " 位相识", 1),
                  n("div", jm, [
                    (c(!0),
                    v(
                      I,
                      null,
                      te(
                        O.value,
                        (C) => (
                          c(),
                          v(
                            "button",
                            {
                              key: C.id,
                              class: "card person-card",
                              onClick: (se) => (p.value = C.id),
                            },
                            [
                              n("span", Hm, m(C.name.slice(0, 1)), 1),
                              n("h3", null, m(C.name), 1),
                              n(
                                "p",
                                null,
                                m(C.identity) + " · " + m(C.relation),
                                1,
                              ),
                              n(
                                "small",
                                null,
                                m(C.location) + " · " + m(C.tag),
                                1,
                              ),
                            ],
                            8,
                            Dm,
                          )
                        ),
                      ),
                      128,
                    )),
                  ]),
                  O.value.length
                    ? J("", !0)
                    : (c(),
                      v("div", Bm, [
                        F[14] || (F[14] = n("p", null, "暂无匹配人物", -1)),
                        n(
                          "button",
                          { class: "button", onClick: le },
                          "清除筛选",
                        ),
                      ])),
                  G.value
                    ? (c(),
                      v(
                        "div",
                        {
                          key: 1,
                          "data-testid": "person-drawer-overlay",
                          class: "drawer-backdrop",
                          onClick:
                            F[3] ||
                            (F[3] = Ue((C) => (p.value = null), ["self"])),
                        },
                        [
                          n(
                            "aside",
                            {
                              "data-testid": "person-drawer",
                              class: "detail-drawer",
                              role: "dialog",
                              "aria-modal": "true",
                              "aria-label": G.value.name,
                            },
                            [
                              n(
                                "button",
                                {
                                  class: "button",
                                  onClick:
                                    F[2] || (F[2] = (C) => (p.value = null)),
                                },
                                "关闭",
                              ),
                              n("h2", null, m(G.value.name), 1),
                              n(
                                "p",
                                null,
                                m(G.value.identity) +
                                  " · " +
                                  m(G.value.type) +
                                  " · " +
                                  m(G.value.status),
                                1,
                              ),
                              n("p", null, m(G.value.bio), 1),
                              n("p", null, [
                                K("关系：" + m(G.value.relation), 1),
                                F[15] || (F[15] = n("br", null, null, -1)),
                                K("位置：" + m(G.value.location), 1),
                                F[16] || (F[16] = n("br", null, null, -1)),
                                K("日程：" + m(G.value.schedule), 1),
                                F[17] || (F[17] = n("br", null, null, -1)),
                                K("标签：" + m(G.value.tag), 1),
                              ]),
                            ],
                            8,
                            Km,
                          ),
                        ],
                      ))
                    : J("", !0),
                ],
                64,
              ))
            : e.page === "媒体"
              ? (c(),
                v(
                  I,
                  { key: 2 },
                  [
                    ae(
                      bs,
                      {
                        search: f.value,
                        "onUpdate:search":
                          F[4] || (F[4] = (C) => (f.value = C)),
                        filters: a.value,
                        "onUpdate:filters":
                          F[5] || (F[5] = (C) => (a.value = C)),
                        groups: P.value,
                        "test-id": "media-filters",
                        placeholder: "搜索媒体 / 角色",
                      },
                      null,
                      8,
                      ["search", "filters", "groups"],
                    ),
                    n("small", zm, m(W.value.length) + " 项", 1),
                    n("div", qm, [
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          W.value,
                          (C) => (
                            c(),
                            v("article", Wm, [
                              n(
                                "button",
                                {
                                  class: "media-open",
                                  onClick: (se) => (p.value = C.id),
                                },
                                [
                                  n(
                                    "div",
                                    {
                                      class: ke([
                                        "media-preview",
                                        "media-art-" + (C.id % 6),
                                        "media-type-" + C.type,
                                        C.type === "图片" ? "photo" : "",
                                      ]),
                                    },
                                    [
                                      (c(),
                                      Se(
                                        hs(
                                          C.type === "图片"
                                            ? h(Sn)
                                            : C.type === "语音"
                                              ? h(qo)
                                              : h(zo),
                                        ),
                                        { size: 30 },
                                      )),
                                    ],
                                    2,
                                  ),
                                  n("b", null, m(C.title), 1),
                                  n(
                                    "small",
                                    null,
                                    m(C.character) +
                                      " · " +
                                      m(C.type) +
                                      " · " +
                                      m(C.time),
                                    1,
                                  ),
                                ],
                                8,
                                Jm,
                              ),
                              C.type === "语音"
                                ? (c(),
                                  v("div", Gm, [
                                    n(
                                      "button",
                                      {
                                        class: "button",
                                        onClick: (se) =>
                                          (s.value =
                                            s.value === C.id ? null : C.id),
                                      },
                                      m(s.value === C.id ? "暂停" : "播放"),
                                      9,
                                      Ym,
                                    ),
                                    n(
                                      "progress",
                                      { value: l.value[C.id] || 0, max: "100" },
                                      null,
                                      8,
                                      Zm,
                                    ),
                                    n(
                                      "small",
                                      null,
                                      m(
                                        Math.floor((l.value[C.id] || 0) * 0.3),
                                      ) + " / 30 秒",
                                      1,
                                    ),
                                  ]))
                                : J("", !0),
                            ])
                          ),
                        ),
                        256,
                      )),
                    ]),
                    p.value !== null
                      ? (c(),
                        v(
                          "div",
                          {
                            key: 0,
                            class: "modal-backdrop",
                            onClick:
                              F[7] ||
                              (F[7] = Ue((C) => (p.value = null), ["self"])),
                          },
                          [
                            n("section", Qm, [
                              (c(!0),
                              v(
                                I,
                                null,
                                te(
                                  h(ei).filter((C) => C.id === p.value),
                                  (C) => (
                                    c(),
                                    v(
                                      I,
                                      null,
                                      [
                                        n("div", Xm, [
                                          n("h2", null, m(C.title), 1),
                                          F[18] ||
                                            (F[18] = n(
                                              "span",
                                              { class: "spacer" },
                                              null,
                                              -1,
                                            )),
                                          n(
                                            "button",
                                            {
                                              class: "button",
                                              onClick:
                                                F[6] ||
                                                (F[6] = (se) =>
                                                  (p.value = null)),
                                            },
                                            "关闭预览",
                                          ),
                                        ]),
                                        n(
                                          "div",
                                          {
                                            class: ke([
                                              "media-preview",
                                              "large-media",
                                              "media-art-" + (C.id % 6),
                                              "media-type-" + C.type,
                                            ]),
                                          },
                                          [
                                            (c(),
                                            Se(
                                              hs(
                                                C.type === "图片"
                                                  ? h(Sn)
                                                  : C.type === "语音"
                                                    ? h(qo)
                                                    : h(zo),
                                              ),
                                              { size: 72 },
                                            )),
                                            n(
                                              "span",
                                              null,
                                              m(
                                                C.type === "语音"
                                                  ? "▂ ▅ ▃ ▇ ▄ ▂ ▆"
                                                  : C.type === "视频"
                                                    ? "▶"
                                                    : C.title,
                                              ),
                                              1,
                                            ),
                                          ],
                                          2,
                                        ),
                                        n(
                                          "p",
                                          null,
                                          m(C.character) +
                                            " · " +
                                            m(C.time) +
                                            " · " +
                                            m(C.tag),
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
                      : J("", !0),
                    W.value.length
                      ? J("", !0)
                      : (c(),
                        v("div", e1, [
                          F[19] || (F[19] = n("p", null, "暂无匹配媒体。", -1)),
                          n(
                            "button",
                            {
                              class: "button",
                              "data-testid": "media-empty-clear",
                              onClick: le,
                            },
                            "清除筛选",
                          ),
                        ])),
                  ],
                  64,
                ))
              : e.page === "角色库"
                ? (c(),
                  v(
                    I,
                    { key: 3 },
                    [
                      ae(
                        bs,
                        {
                          search: f.value,
                          "onUpdate:search":
                            F[8] || (F[8] = (C) => (f.value = C)),
                          filters: a.value,
                          "onUpdate:filters":
                            F[9] || (F[9] = (C) => (a.value = C)),
                          groups: V,
                          "test-id": "library-filters",
                          placeholder: "搜索角色 / 职业",
                        },
                        null,
                        8,
                        ["search", "filters"],
                      ),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          oe.value,
                          (C) => (
                            c(),
                            v("article", { key: C.id, class: "card compact" }, [
                              n("div", t1, [
                                n(
                                  "span",
                                  {
                                    class: "avatar",
                                    style: at({ background: C.color }),
                                  },
                                  m(C.initial),
                                  5,
                                ),
                                n("div", null, [
                                  n("b", null, m(C.name) + " · " + m(C.job), 1),
                                  n("p", null, m(C.subtitle), 1),
                                  (c(!0),
                                  v(
                                    I,
                                    null,
                                    te(
                                      C.tags,
                                      (se) => (c(), v("span", s1, m(se), 1)),
                                    ),
                                    256,
                                  )),
                                ]),
                                F[20] ||
                                  (F[20] = n(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                n(
                                  "button",
                                  {
                                    class: "text-button",
                                    onClick: (se) =>
                                      (p.value =
                                        p.value === C.id ? null : C.id),
                                  },
                                  m(p.value === C.id ? "收起详情" : "角色详情"),
                                  9,
                                  l1,
                                ),
                                h(lt) === C.id
                                  ? (c(), v("span", n1, "当前"))
                                  : (c(),
                                    v(
                                      "button",
                                      {
                                        key: 1,
                                        class: "button primary",
                                        onClick: (se) => (lt.value = C.id),
                                      },
                                      "切换角色",
                                      8,
                                      o1,
                                    )),
                              ]),
                              p.value === C.id
                                ? (c(),
                                  v("div", i1, [
                                    n("h2", null, m(C.name), 1),
                                    n("p", null, m(C.bio), 1),
                                    n("p", null, [
                                      K("职业：" + m(C.job), 1),
                                      F[21] ||
                                        (F[21] = n("br", null, null, -1)),
                                      K("兴趣：" + m(C.hobby), 1),
                                      F[22] ||
                                        (F[22] = n("br", null, null, -1)),
                                      K("心情：" + m(C.mood), 1),
                                      F[23] ||
                                        (F[23] = n("br", null, null, -1)),
                                      K("位置：" + m(C.location), 1),
                                      F[24] ||
                                        (F[24] = n("br", null, null, -1)),
                                      K("世界：" + m(C.world), 1),
                                    ]),
                                  ]))
                                : J("", !0),
                            ])
                          ),
                        ),
                        128,
                      )),
                      oe.value.length
                        ? J("", !0)
                        : (c(), v("p", r1, "暂无匹配角色。")),
                    ],
                    64,
                  ))
                : e.page === "创作中心"
                  ? (c(), v("div", a1, "即将重新设计"))
                  : e.page === "通知与待处理"
                    ? (c(),
                      v(
                        I,
                        { key: 5 },
                        [
                          F[29] || (F[29] = n("h2", null, "未读消息", -1)),
                          (c(!0),
                          v(
                            I,
                            null,
                            te(h(Ae), (C, se) => {
                              var be;
                              return (
                                c(),
                                v(
                                  "button",
                                  {
                                    class: "card row notification-card",
                                    onClick: (q) => u(C.id),
                                  },
                                  [
                                    n(
                                      "span",
                                      {
                                        class: "avatar",
                                        style: at({ background: C.color }),
                                      },
                                      m(C.initial),
                                      5,
                                    ),
                                    n("div", null, [
                                      n("b", null, m(C.name) + " 在找你", 1),
                                      n(
                                        "p",
                                        null,
                                        m(
                                          (be = C.messages
                                            .filter((q) => !q.user)
                                            .at(-1)) == null
                                            ? void 0
                                            : be.text.slice(0, 60),
                                        ),
                                        1,
                                      ),
                                    ]),
                                    F[25] ||
                                      (F[25] = n(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    n("span", d1, m(se + 1) + " 条未读", 1),
                                  ],
                                  8,
                                  u1,
                                )
                              );
                            }),
                            256,
                          )),
                          n("h2", null, [
                            F[26] || (F[26] = K("异常检查 ", -1)),
                            n("small", null, m(i.value.length) + " 项", 1),
                          ]),
                          (c(!0),
                          v(
                            I,
                            null,
                            te(
                              i.value,
                              (C) => (
                                c(),
                                v(
                                  "button",
                                  {
                                    "data-testid":
                                      "notification-diagnostic-" + C.id,
                                    class: "card row notification-card",
                                    onClick: (se) => r(C.id),
                                  },
                                  [
                                    n("b", null, m(C.label), 1),
                                    n("span", null, m(C.reason), 1),
                                    F[27] ||
                                      (F[27] = n(
                                        "span",
                                        { class: "spacer" },
                                        null,
                                        -1,
                                      )),
                                    F[28] || (F[28] = K("查看诊断 ↗", -1)),
                                  ],
                                  8,
                                  c1,
                                )
                              ),
                            ),
                            256,
                          )),
                          i.value.length
                            ? J("", !0)
                            : (c(), v("p", f1, "目前没有异常。")),
                        ],
                        64,
                      ))
                    : J("", !0);
    },
  }),
  m1 = { class: "tabs" },
  v1 = ["onClick"],
  h1 = { key: 0, class: "configuration-layout" },
  g1 = {
    class: "configuration-nav",
    "aria-label": "配置章节",
    "data-testid": "config-section-nav",
  },
  y1 = ["href", "data-testid", "onClick"],
  b1 = ["id"],
  k1 = { class: "row" },
  _1 = ["onClick"],
  w1 = { class: "form-grid" },
  $1 = ["onUpdate:modelValue"],
  C1 = ["onUpdate:modelValue"],
  x1 = ["onUpdate:modelValue"],
  S1 = ["onUpdate:modelValue"],
  T1 = { class: "check-field" },
  M1 = ["onUpdate:modelValue"],
  I1 = { class: "muted" },
  E1 = ["onClick"],
  A1 = ["id"],
  V1 = { class: "form-grid" },
  O1 = ["onUpdate:modelValue"],
  U1 = ["value"],
  P1 = ["onUpdate:modelValue", "list"],
  L1 = ["id"],
  N1 = ["value"],
  R1 = ["onUpdate:modelValue"],
  F1 = ["onUpdate:modelValue"],
  j1 = ["onUpdate:modelValue", "min"],
  D1 = { class: "muted" },
  H1 = { class: "management-block" },
  B1 = { class: "shared-scenario" },
  K1 = ["id"],
  z1 = { class: "form-grid" },
  q1 = { class: "check-field" },
  W1 = ["disabled"],
  J1 = { class: "form-grid" },
  G1 = { class: "quiet-days" },
  Y1 = ["value"],
  Z1 = { key: 0 },
  Q1 = { key: 1 },
  X1 = { key: 2, class: "muted" },
  ev = { class: "form-grid" },
  tv = ["onUpdate:modelValue"],
  sv = ["value", "onInput"],
  lv = ["onUpdate:modelValue", "type", "min", "max", "step", "placeholder"],
  nv = ["onClick"],
  ov = { key: 0, role: "status", class: "muted" },
  iv = { class: "settings-save-bar", "data-testid": "config-save-bar" },
  rv = { key: 0, role: "alert", class: "inline-error" },
  av = { key: 1, class: "muted" },
  uv = ["disabled"],
  dv = { class: "tabs" },
  cv = ["data-testid", "onClick"],
  fv = { class: "muted" },
  pv = { class: "diagnostic-feed", "data-testid": "diagnostic-feed" },
  mv = ["id", "data-testid"],
  vv = ["data-testid", "onClick"],
  hv = { class: "diagnostic-meta" },
  gv = { class: "pill" },
  yv = { class: "diagnostic-summary" },
  bv = ["data-testid", "onClick"],
  kv = { key: 0, class: "card empty-state" },
  _v = { class: "detail-meta" },
  wv = { class: "pill" },
  $v = { key: 0, class: "inline-error" },
  Cv = { class: "media-call-attributes", "data-testid": "media-call-details" },
  xv = { class: "detail-section" },
  Sv = { class: "detail-section" },
  Tv = { class: "detail-section" },
  Mv = { class: "media-call-attributes" },
  Iv = { class: "detail-section", "data-testid": "media-call-output" },
  Ev = { key: 0, class: "mock-image-output" },
  Av = { key: 1, class: "muted" },
  Vv = { class: "muted" },
  Ov = { class: "detail-section" },
  Uv = { class: "detail-section" },
  Pv = { class: "detail-section" },
  Lv = { class: "secondary-diagnostics" },
  Nv = { class: "quiet-days" },
  Rv = ["onClick"],
  Fv = { key: 0, class: "card" },
  jv = { class: "row" },
  Dv = { role: "status" },
  Hv = je({
    __name: "ManagementPage",
    setup(e) {
      let t = !0;
      ll(() => {
        t = !1;
      });
      const s = (D) => JSON.parse(JSON.stringify(D)),
        l = z(s(Qo.value)),
        o = z(s(Go.value)),
        i = z(s(Yo.value)),
        r = z(s(Zo.value)),
        u = Zn,
        d = z("模型调用"),
        f = z(""),
        a = z({ type: "", who: "", status: "" }),
        p = z(null),
        _ = z(null),
        b = z(!1),
        A = z(""),
        P = z(""),
        V = z(""),
        le = {
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
        ne = ye(() =>
          $s.value.map((D, S) => {
            const y = le[D.label];
            return {
              ...D,
              kind: "模型调用",
              start: D.start ?? S * 4,
              duration: D.duration ?? 1,
              model: "通用模型",
              category: "工具执行失败",
              code: "capability_prepare_failed",
              response:
                D.response && D.response !== "请求已完成。"
                  ? D.response
                  : `${(y == null ? void 0 : y.summary) || D.label + "已完成"}：${D.input}`,
              reason: D.reason || "",
              character: D.character || "",
              input: `用途：${D.label}
角色：${D.character}
${(y == null ? void 0 : y.prompt) || ""}
本次请求：${D.input}`,
            };
          }),
        ),
        ce = ye(() => [...vr.value, ...ne.value]),
        w = ye(() => ce.value.find((D) => D.id === p.value)),
        O = (D) =>
          D === "error" ? "失败" : D === "pending" ? "处理中" : "完成",
        G = ye(() =>
          ce.value
            .filter((D) => {
              var S, y, $;
              return (
                D.kind === d.value &&
                (!a.value.type || D.label === a.value.type) &&
                (!a.value.who || D.character === a.value.who) &&
                (!a.value.status || O(D.status) === a.value.status) &&
                [
                  D.label,
                  D.input,
                  D.character,
                  D.model,
                  D.reason,
                  (S = D.media) == null ? void 0 : S.workflow,
                  (y = D.media) == null ? void 0 : y.negativePrompt,
                  ($ = D.media) == null ? void 0 : $.output,
                ]
                  .join(" ")
                  .toLowerCase()
                  .includes(f.value.trim().toLowerCase())
              );
            })
            .sort((D, S) => S.start - D.start),
        ),
        oe = ye(() => [
          {
            key: "type",
            label: "类型",
            options: [
              ...new Set([
                ...(d.value === "模型调用" ? Ds : El),
                ...ce.value
                  .filter((D) => D.kind === d.value)
                  .map((D) => D.label),
              ]),
            ],
          },
          { key: "who", label: "角色", options: Ae.value.map((D) => D.name) },
          { key: "status", label: "状态", options: ["完成", "失败", "处理中"] },
        ]);
      function W() {
        ((f.value = ""), (a.value = { type: "", who: "", status: "" }));
      }
      function Y(D) {
        ((d.value = D), W(), (p.value = null), (_.value = null));
      }
      function L(D) {
        ((p.value = D.id), (_.value = D.id));
      }
      (vt(
        In,
        async (D) => {
          if (D === null) return;
          const S = ce.value.find((y) => y.id === D);
          S &&
            ((u.value = "运行诊断"),
            (d.value = S.kind),
            W(),
            L(S),
            (In.value = null),
            await Qt(),
            requestAnimationFrame(() => {
              var y;
              return (y = document.getElementById("diag-record-" + D)) == null
                ? void 0
                : y.scrollIntoView({ behavior: "smooth", block: "center" });
            }));
        },
        { immediate: !0 },
      ),
        vt(u, (D) => {
          D === "配置中心" && (p.value = null);
        }));
      const F = ye(() => [
          "服务地址",
          "模型用途",
          "安静时段",
          ...l.value.map((D) => D.title),
        ]),
        C = (D) => "config-" + D;
      function se(D) {
        const S = document.getElementById(C(D));
        ((S == null ? void 0 : S.tagName) === "DETAILS" &&
          S.setAttribute("open", ""),
          S == null ||
            S.scrollIntoView({ behavior: "smooth", block: "start" }));
      }
      const be = (D) =>
          "14:" +
          String(Math.floor(D.start / 60)).padStart(2, "0") +
          ":" +
          String(D.start % 60).padStart(2, "0"),
        q = ye(() => Xo.find((D) => D.title === P.value));
      function M(D) {
        if (D.status !== "pending") {
          if (D.id > 0) {
            D.run();
            return;
          }
          ((D.status = "pending"),
            setTimeout(() => {
              var y;
              const S = Yt.value || D.input.includes("失败");
              ((D.status = S ? "error" : "success"),
                S
                  ? (D.reason = D.input.includes("失败")
                      ? "输入包含“失败”，请修改输入后重新提交。"
                      : "模拟失败已开启，关闭后可重试。")
                  : ((D.reason = ""),
                    (D.response =
                      ((y = D.media) == null ? void 0 : y.output) ||
                      D.response)));
            }, 900));
        }
      }
      async function T(D) {
        ((V.value = D + "：测试中…"),
          await new Promise((S) => setTimeout(S, 700)),
          (V.value =
            D +
            "：" +
            (Yt.value
              ? "连接失败，请检查地址。"
              : "模拟测试通过（仅检查可达性）。")));
      }
      async function H() {
        if (b.value) return;
        A.value = "";
        for (const y of l.value)
          for (const $ of y.fields) {
            if ($.type === "json")
              try {
                const x = JSON.parse(String($.value));
                if (!x || typeof x != "object" || Array.isArray(x))
                  throw Error();
              } catch {
                A.value = $.label + "：请填写有效 JSON 对象。";
                return;
              }
            if (
              $.type === "number" &&
              (!Number.isFinite(+$.value) ||
                ($.min !== void 0 && +$.value < $.min) ||
                ($.max !== void 0 && +$.value > $.max))
            ) {
              A.value = $.label + "：数值超出范围。";
              return;
            }
          }
        for (const y of o.value)
          try {
            const $ = new URL(y.url);
            if (
              !["http:", "https:"].includes($.protocol) ||
              $.username ||
              $.password
            )
              throw Error();
          } catch {
            A.value = "服务地址必须是不带账号密码的 http(s) URL。";
            return;
          }
        if (i.value.some((y) => !o.value.some(($) => $.id === y.endpoint))) {
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
        const D = l.value.find((y) => y.title === "高级");
        if (
          Number(D.fields.find((y) => y.label === "阶段总预算（ms）").value) <
          Number(D.fields.find((y) => y.label === "请求超时（ms）").value)
        ) {
          A.value = "Kev 阶段总预算不能小于请求超时。";
          return;
        }
        const S = [
          ...o.value.flatMap((y) => [y.name, y.protocol, y.url, y.secret]),
          ...i.value.map((y) => y.model),
          ...l.value.flatMap((y) => y.fields.map(($) => String($.value))),
        ].some((y) => y.includes("失败"));
        if (
          ((b.value = !0),
          await new Promise((y) => setTimeout(y, 800)),
          (b.value = !1),
          S)
        ) {
          A.value = "输入包含“失败”，请修改后重新提交。";
          return;
        }
        if (Yt.value) {
          A.value = "模拟保存失败，请关闭模拟失败后重试。";
          return;
        }
        ((Qo.value = s(
          l.value.map((y) => ({
            ...y,
            fields: y.fields.map(($) =>
              $.type === "password" ? { ...$, value: "" } : $,
            ),
          })),
        )),
          (Go.value = s(o.value.map((y) => ({ ...y, secret: "" })))),
          (Yo.value = s(i.value)),
          (Zo.value = s(r.value)),
          o.value.forEach((y) => (y.secret = "")),
          l.value.forEach((y) =>
            y.fields.forEach(($) => {
              $.type === "password" && ($.value = "");
            }),
          ),
          t && el("设置已保存（本地模拟）。"));
      }
      function _e() {
        o.value.push({
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
      return (D, S) => (
        c(),
        v(
          I,
          null,
          [
            n("div", m1, [
              (c(),
              v(
                I,
                null,
                te(["配置中心", "运行诊断"], (y) =>
                  n(
                    "button",
                    {
                      key: y,
                      class: ke({ active: h(u) === y }),
                      onClick: ($) => (u.value = y),
                    },
                    m(y),
                    11,
                    v1,
                  ),
                ),
                64,
              )),
            ]),
            h(u) === "配置中心"
              ? (c(),
                v("div", h1, [
                  n("nav", g1, [
                    (c(!0),
                    v(
                      I,
                      null,
                      te(
                        F.value,
                        (y) => (
                          c(),
                          v(
                            "a",
                            {
                              key: y,
                              href: "#" + encodeURIComponent("管理中心"),
                              "data-testid": "config-anchor-" + y,
                              onClick: Ue(($) => se(y), ["prevent"]),
                            },
                            m(y),
                            9,
                            y1,
                          )
                        ),
                      ),
                      128,
                    )),
                  ]),
                  n(
                    "form",
                    {
                      class: "settings-form management-form",
                      onSubmit: Ue(H, ["prevent"]),
                    },
                    [
                      n(
                        "section",
                        { id: C("服务地址"), class: "card settings-group" },
                        [
                          n("div", { class: "row" }, [
                            S[12] || (S[12] = n("h2", null, "服务地址", -1)),
                            S[13] ||
                              (S[13] = n(
                                "span",
                                { class: "spacer" },
                                null,
                                -1,
                              )),
                            n(
                              "button",
                              {
                                type: "button",
                                class: "text-button",
                                onClick: _e,
                              },
                              "添加服务",
                            ),
                          ]),
                          (c(!0),
                          v(
                            I,
                            null,
                            te(
                              o.value,
                              (y, $) => (
                                c(),
                                v(
                                  "div",
                                  { key: y.id, class: "management-block" },
                                  [
                                    n("div", k1, [
                                      n("b", null, m(y.name), 1),
                                      n(
                                        "small",
                                        null,
                                        m(y.status) + " · 模拟状态",
                                        1,
                                      ),
                                      S[14] ||
                                        (S[14] = n(
                                          "span",
                                          { class: "spacer" },
                                          null,
                                          -1,
                                        )),
                                      n(
                                        "button",
                                        {
                                          type: "button",
                                          class: "text-button",
                                          onClick: (x) => o.value.splice($, 1),
                                        },
                                        "移除",
                                        8,
                                        _1,
                                      ),
                                    ]),
                                    n("div", w1, [
                                      n("label", null, [
                                        S[15] || (S[15] = K("服务名称", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.name = x),
                                              required: "",
                                              maxlength: "128",
                                            },
                                            null,
                                            8,
                                            $1,
                                          ),
                                          [[he, y.name]],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[16] || (S[16] = K("接口协议", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.protocol = x),
                                              required: "",
                                            },
                                            null,
                                            8,
                                            C1,
                                          ),
                                          [[he, y.protocol]],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[17] || (S[17] = K("服务地址", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.url = x),
                                              required: "",
                                              type: "url",
                                            },
                                            null,
                                            8,
                                            x1,
                                          ),
                                          [[he, y.url]],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[18] || (S[18] = K("访问密钥", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.secret = x),
                                              type: "password",
                                              placeholder:
                                                "留空保留原值 · 只写不读",
                                              autocomplete: "new-password",
                                            },
                                            null,
                                            8,
                                            S1,
                                          ),
                                          [[he, y.secret]],
                                        ),
                                      ]),
                                      n("label", T1, [
                                        S[19] ||
                                          (S[19] = K("清除已存密钥", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.clearSecret = x),
                                              type: "checkbox",
                                            },
                                            null,
                                            8,
                                            M1,
                                          ),
                                          [[Ft, y.clearSecret]],
                                        ),
                                      ]),
                                    ]),
                                    n(
                                      "p",
                                      I1,
                                      "可用模型：" +
                                        m(
                                          y.models.join("、") ||
                                            "暂未读取，可在用途配置中手填模型 ID",
                                        ),
                                      1,
                                    ),
                                    n(
                                      "button",
                                      {
                                        type: "button",
                                        class: "text-button",
                                        onClick: (x) => T(y.name),
                                      },
                                      "测试连接",
                                      8,
                                      E1,
                                    ),
                                  ],
                                )
                              ),
                            ),
                            128,
                          )),
                          S[20] ||
                            (S[20] = n(
                              "p",
                              { class: "muted" },
                              "服务地址修改并保存后，其用途绑定需要重新确认；本原型保留选择供检查。",
                              -1,
                            )),
                        ],
                        8,
                        b1,
                      ),
                      n(
                        "section",
                        { id: C("模型用途"), class: "card settings-group" },
                        [
                          S[27] ||
                            (S[27] = n("h2", null, "各用途对应的模型配置", -1)),
                          S[28] ||
                            (S[28] = n(
                              "p",
                              { class: "muted" },
                              "只有以下四个用途可独立选模型。默认安全余量 512 token，输出预留采用用途预算。",
                              -1,
                            )),
                          (c(!0),
                          v(
                            I,
                            null,
                            te(i.value, (y) => {
                              var $;
                              return (
                                c(),
                                v(
                                  "details",
                                  {
                                    key: y.name,
                                    class: "management-block",
                                    open: "",
                                  },
                                  [
                                    n("summary", null, m(y.name), 1),
                                    n("div", V1, [
                                      n("label", null, [
                                        S[21] || (S[21] = K("使用服务", -1)),
                                        re(
                                          n(
                                            "select",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.endpoint = x),
                                              required: "",
                                            },
                                            [
                                              (c(!0),
                                              v(
                                                I,
                                                null,
                                                te(
                                                  o.value,
                                                  (x) => (
                                                    c(),
                                                    v(
                                                      "option",
                                                      { value: x.id },
                                                      m(x.name),
                                                      9,
                                                      U1,
                                                    )
                                                  ),
                                                ),
                                                256,
                                              )),
                                            ],
                                            8,
                                            O1,
                                          ),
                                          [[pt, y.endpoint]],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[22] || (S[22] = K("模型 ID", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.model = x),
                                              list: "models-" + y.role,
                                              required: "",
                                            },
                                            null,
                                            8,
                                            P1,
                                          ),
                                          [[he, y.model]],
                                        ),
                                        n(
                                          "datalist",
                                          { id: "models-" + y.role },
                                          [
                                            (c(!0),
                                            v(
                                              I,
                                              null,
                                              te(
                                                ($ = o.value.find(
                                                  (x) => x.id === y.endpoint,
                                                )) == null
                                                  ? void 0
                                                  : $.models,
                                                (x) => (
                                                  c(),
                                                  v(
                                                    "option",
                                                    { value: x },
                                                    null,
                                                    8,
                                                    N1,
                                                  )
                                                ),
                                              ),
                                              256,
                                            )),
                                          ],
                                          8,
                                          L1,
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[23] || (S[23] = K("Token 预算", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.tokens = x),
                                              type: "number",
                                              min: "1",
                                              required: "",
                                            },
                                            null,
                                            8,
                                            R1,
                                          ),
                                          [
                                            [
                                              he,
                                              y.tokens,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[24] || (S[24] = K("超时（秒）", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.timeout = x),
                                              type: "number",
                                              min: "1",
                                              required: "",
                                            },
                                            null,
                                            8,
                                            F1,
                                          ),
                                          [
                                            [
                                              he,
                                              y.timeout,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      n("label", null, [
                                        S[25] ||
                                          (S[25] = K("上下文窗口（可选）", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue": (x) =>
                                                (y.window = x),
                                              type: "number",
                                              min: y.tokens + 512,
                                            },
                                            null,
                                            8,
                                            j1,
                                          ),
                                          [
                                            [
                                              he,
                                              y.window,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                      n(
                                        "p",
                                        D1,
                                        "可用输入预算：" +
                                          m(
                                            y.window
                                              ? Math.max(
                                                  0,
                                                  y.window - y.tokens - 512,
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
                          n("details", H1, [
                            n(
                              "summary",
                              null,
                              "共用通用模型的场景 · " + m(h(Wo).length) + " 项",
                              1,
                            ),
                            (c(!0),
                            v(
                              I,
                              null,
                              te(
                                h(Wo),
                                (y) => (
                                  c(),
                                  v("div", B1, [
                                    n("span", null, m(y), 1),
                                    S[26] ||
                                      (S[26] = n(
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
                        A1,
                      ),
                      n(
                        "section",
                        { id: C("安静时段"), class: "card settings-group" },
                        [
                          S[37] || (S[37] = n("h2", null, "安静时段", -1)),
                          n("div", z1, [
                            n("label", q1, [
                              S[29] || (S[29] = K("启用安静时段", -1)),
                              re(
                                n(
                                  "input",
                                  {
                                    "onUpdate:modelValue":
                                      S[0] ||
                                      (S[0] = (y) => (r.value.enabled = y)),
                                    class: "switch",
                                    type: "checkbox",
                                  },
                                  null,
                                  512,
                                ),
                                [[Ft, r.value.enabled]],
                              ),
                            ]),
                            n("label", null, [
                              S[31] || (S[31] = K("适用角色", -1)),
                              re(
                                n(
                                  "select",
                                  {
                                    "onUpdate:modelValue":
                                      S[1] ||
                                      (S[1] = (y) => (r.value.character = y)),
                                  },
                                  [
                                    S[30] ||
                                      (S[30] = n(
                                        "option",
                                        null,
                                        "全部角色",
                                        -1,
                                      )),
                                    (c(!0),
                                    v(
                                      I,
                                      null,
                                      te(
                                        h(Ae),
                                        (y) => (
                                          c(),
                                          v("option", null, m(y.name), 1)
                                        ),
                                      ),
                                      256,
                                    )),
                                  ],
                                  512,
                                ),
                                [[pt, r.value.character]],
                              ),
                            ]),
                          ]),
                          n(
                            "fieldset",
                            { disabled: !r.value.enabled },
                            [
                              n("div", J1, [
                                n("label", null, [
                                  S[32] || (S[32] = K("开始时间", -1)),
                                  re(
                                    n(
                                      "input",
                                      {
                                        "onUpdate:modelValue":
                                          S[2] ||
                                          (S[2] = (y) => (r.value.start = y)),
                                        type: "time",
                                      },
                                      null,
                                      512,
                                    ),
                                    [[he, r.value.start]],
                                  ),
                                ]),
                                n("label", null, [
                                  S[33] || (S[33] = K("结束时间", -1)),
                                  re(
                                    n(
                                      "input",
                                      {
                                        "onUpdate:modelValue":
                                          S[3] ||
                                          (S[3] = (y) => (r.value.end = y)),
                                        type: "time",
                                      },
                                      null,
                                      512,
                                    ),
                                    [[he, r.value.end]],
                                  ),
                                ]),
                                n("label", null, [
                                  S[35] || (S[35] = K("行为", -1)),
                                  re(
                                    n(
                                      "select",
                                      {
                                        "onUpdate:modelValue":
                                          S[4] ||
                                          (S[4] = (y) =>
                                            (r.value.behavior = y)),
                                      },
                                      [
                                        ...(S[34] ||
                                          (S[34] = [
                                            n(
                                              "option",
                                              null,
                                              "暂停主动联系",
                                              -1,
                                            ),
                                            n(
                                              "option",
                                              null,
                                              "延后至安静时段结束",
                                              -1,
                                            ),
                                          ])),
                                      ],
                                      512,
                                    ),
                                    [[pt, r.value.behavior]],
                                  ),
                                ]),
                              ]),
                              S[36] ||
                                (S[36] = n(
                                  "p",
                                  null,
                                  "生效日期（Asia/Shanghai，支持跨夜）",
                                  -1,
                                )),
                              n("div", G1, [
                                (c(!0),
                                v(
                                  I,
                                  null,
                                  te(
                                    h(mr),
                                    (y) => (
                                      c(),
                                      v("label", null, [
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                S[5] ||
                                                (S[5] = ($) =>
                                                  (r.value.days = $)),
                                              type: "checkbox",
                                              value: y,
                                            },
                                            null,
                                            8,
                                            Y1,
                                          ),
                                          [[Ft, r.value.days]],
                                        ),
                                        K(m(y), 1),
                                      ])
                                    ),
                                  ),
                                  256,
                                )),
                              ]),
                            ],
                            8,
                            W1,
                          ),
                        ],
                        8,
                        K1,
                      ),
                      (c(!0),
                      v(
                        I,
                        null,
                        te(
                          l.value,
                          (y) => (
                            c(),
                            Se(
                              hs(y.title === "高级" ? "details" : "section"),
                              {
                                key: y.title,
                                id: C(y.title),
                                class: "card settings-group",
                              },
                              {
                                default: Gt(() => [
                                  y.title === "高级"
                                    ? (c(),
                                      v("summary", Z1, "高级 · Kev 决策服务"))
                                    : (c(), v("h2", Q1, m(y.title), 1)),
                                  y.title === "图片（ComfyUI）"
                                    ? (c(),
                                      v(
                                        "p",
                                        X1,
                                        "图片已接入 ComfyUI · 语音：未接入 · 视频：未接入",
                                      ))
                                    : J("", !0),
                                  n("div", ev, [
                                    (c(!0),
                                    v(
                                      I,
                                      null,
                                      te(
                                        y.fields,
                                        ($) => (
                                          c(),
                                          v(
                                            "label",
                                            {
                                              key: $.label,
                                              class: ke({
                                                "check-field":
                                                  $.type === "checkbox",
                                              }),
                                            },
                                            [
                                              n("span", null, m($.label), 1),
                                              $.type === "select"
                                                ? re(
                                                    (c(),
                                                    v(
                                                      "select",
                                                      {
                                                        key: 0,
                                                        "onUpdate:modelValue": (
                                                          x,
                                                        ) => ($.value = x),
                                                      },
                                                      [
                                                        (c(!0),
                                                        v(
                                                          I,
                                                          null,
                                                          te(
                                                            $.options,
                                                            (x) => (
                                                              c(),
                                                              v(
                                                                "option",
                                                                null,
                                                                m(x),
                                                                1,
                                                              )
                                                            ),
                                                          ),
                                                          256,
                                                        )),
                                                      ],
                                                      8,
                                                      tv,
                                                    )),
                                                    [[pt, $.value]],
                                                  )
                                                : $.type === "json"
                                                  ? (c(),
                                                    v(
                                                      "textarea",
                                                      {
                                                        key: 1,
                                                        value: String($.value),
                                                        onInput: (x) =>
                                                          ($.value =
                                                            x.target.value),
                                                        rows: "3",
                                                      },
                                                      null,
                                                      40,
                                                      sv,
                                                    ))
                                                  : re(
                                                      (c(),
                                                      v(
                                                        "input",
                                                        {
                                                          key: 2,
                                                          "onUpdate:modelValue":
                                                            (x) =>
                                                              ($.value = x),
                                                          type: $.type,
                                                          min: $.min,
                                                          max: $.max,
                                                          step:
                                                            $.type === "number"
                                                              ? "any"
                                                              : void 0,
                                                          class: ke({
                                                            switch:
                                                              $.type ===
                                                              "checkbox",
                                                          }),
                                                          placeholder:
                                                            $.type ===
                                                            "password"
                                                              ? "留空保留原值 · 只写不读"
                                                              : void 0,
                                                        },
                                                        null,
                                                        10,
                                                        lv,
                                                      )),
                                                      [[rd, $.value]],
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
                                  ].includes(y.title)
                                    ? (c(),
                                      v(
                                        "button",
                                        {
                                          key: 3,
                                          type: "button",
                                          class: "text-button",
                                          onClick: ($) => T(y.title),
                                        },
                                        m(
                                          y.title === "看图理解"
                                            ? "测试看图（使用已保存配置）"
                                            : "测试连接",
                                        ),
                                        9,
                                        nv,
                                      ))
                                    : J("", !0),
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
                      V.value ? (c(), v("p", ov, m(V.value), 1)) : J("", !0),
                      n("div", iv, [
                        A.value
                          ? (c(), v("p", rv, m(A.value), 1))
                          : (c(), v("span", av, "让相处的节奏，更合心意。")),
                        n(
                          "button",
                          {
                            class: "button primary",
                            disabled: b.value,
                            "data-testid": "config-save",
                          },
                          m(b.value ? "保存中…" : "保存设置"),
                          9,
                          uv,
                        ),
                      ]),
                    ],
                    32,
                  ),
                ]))
              : (c(),
                v(
                  I,
                  { key: 1 },
                  [
                    S[55] ||
                      (S[55] = n(
                        "p",
                        { class: "muted" },
                        "每次回应背后的记录。模型与媒体的运行情况，都在这里。",
                        -1,
                      )),
                    n("div", dv, [
                      (c(),
                      v(
                        I,
                        null,
                        te(["模型调用", "媒体调用"], (y) =>
                          n(
                            "button",
                            {
                              key: y,
                              class: ke({ active: d.value === y }),
                              "data-testid": "diag-tab-" + y,
                              onClick: ($) => Y(y),
                            },
                            m(y),
                            11,
                            cv,
                          ),
                        ),
                        64,
                      )),
                    ]),
                    ae(
                      bs,
                      {
                        search: f.value,
                        "onUpdate:search":
                          S[6] || (S[6] = (y) => (f.value = y)),
                        filters: a.value,
                        "onUpdate:filters":
                          S[7] || (S[7] = (y) => (a.value = y)),
                        groups: oe.value,
                        "test-id": "diag-filters",
                        placeholder: "搜索调用 / 提示词 / 工作流 / 角色",
                      },
                      null,
                      8,
                      ["search", "filters", "groups"],
                    ),
                    n(
                      "small",
                      fv,
                      m(G.value.length) + " 条记录 · 最近的记录在前",
                      1,
                    ),
                    n("section", pv, [
                      (c(!0),
                      v(
                        I,
                        null,
                        te(G.value, (y) => {
                          var $;
                          return (
                            c(),
                            v(
                              "article",
                              {
                                id: "diag-record-" + y.id,
                                key: y.id,
                                class: ke([
                                  "diagnostic-record",
                                  { highlighted: _.value === y.id },
                                ]),
                                "data-testid": "diag-record-" + y.id,
                              },
                              [
                                n(
                                  "button",
                                  {
                                    class: "diagnostic-open",
                                    "data-testid": "diag-open-" + y.id,
                                    onClick: (x) => L(y),
                                  },
                                  [
                                    n("div", hv, [
                                      n("time", null, m(be(y)), 1),
                                      n("span", gv, m(y.label), 1),
                                      n("span", null, m(y.character), 1),
                                      S[38] ||
                                        (S[38] = n(
                                          "span",
                                          { class: "spacer" },
                                          null,
                                          -1,
                                        )),
                                      n(
                                        "span",
                                        {
                                          class: ke(["call-status", y.status]),
                                        },
                                        m(O(y.status)),
                                        3,
                                      ),
                                      n(
                                        "small",
                                        null,
                                        m(y.duration) + " 秒",
                                        1,
                                      ),
                                    ]),
                                    n(
                                      "p",
                                      yv,
                                      m(
                                        y.status === "error"
                                          ? y.reason
                                          : y.status === "pending"
                                            ? "正在等待服务回应"
                                            : y.kind === "媒体调用"
                                              ? ($ = y.media) == null
                                                ? void 0
                                                : $.output
                                              : y.response,
                                      ),
                                      1,
                                    ),
                                  ],
                                  8,
                                  vv,
                                ),
                                y.status === "error"
                                  ? (c(),
                                    v(
                                      "button",
                                      {
                                        key: 0,
                                        class: "text-button diagnostic-retry",
                                        "data-testid": "diag-retry-" + y.id,
                                        onClick: (x) => M(y),
                                      },
                                      "重试",
                                      8,
                                      bv,
                                    ))
                                  : J("", !0),
                              ],
                              10,
                              mv,
                            )
                          );
                        }),
                        128,
                      )),
                      G.value.length
                        ? J("", !0)
                        : (c(),
                          v("div", kv, [
                            S[39] ||
                              (S[39] = n(
                                "p",
                                null,
                                "暂无符合条件的调用。",
                                -1,
                              )),
                            n(
                              "button",
                              {
                                class: "button",
                                "data-testid": "diag-empty-clear",
                                onClick: W,
                              },
                              "清除筛选",
                            ),
                          ])),
                    ]),
                    w.value
                      ? (c(),
                        Se(
                          Ks,
                          {
                            key: 0,
                            title: w.value.label + " · 调用详情",
                            "test-id": "diagnostic-drawer",
                            onClose: S[9] || (S[9] = (y) => (p.value = null)),
                          },
                          {
                            default: Gt(() => [
                              n("div", _v, [
                                n("span", wv, m(w.value.kind), 1),
                                n(
                                  "p",
                                  null,
                                  m(w.value.character) +
                                    " · " +
                                    m(be(w.value)) +
                                    " · 用时 " +
                                    m(w.value.duration) +
                                    " 秒",
                                  1,
                                ),
                                n(
                                  "span",
                                  {
                                    class: ke(["call-status", w.value.status]),
                                  },
                                  m(O(w.value.status)),
                                  3,
                                ),
                              ]),
                              w.value.status === "error"
                                ? (c(),
                                  v("p", $v, [
                                    K(
                                      m(w.value.category) +
                                        "：" +
                                        m(w.value.reason) +
                                        " ",
                                      1,
                                    ),
                                    n(
                                      "button",
                                      {
                                        class: "text-button",
                                        "data-testid": "diag-drawer-retry",
                                        onClick:
                                          S[8] || (S[8] = (y) => M(w.value)),
                                      },
                                      "重试",
                                    ),
                                  ]))
                                : J("", !0),
                              w.value.kind === "媒体调用" && w.value.media
                                ? (c(),
                                  v(
                                    I,
                                    { key: 1 },
                                    [
                                      n("dl", Cv, [
                                        n("div", null, [
                                          S[40] ||
                                            (S[40] = n("dt", null, "角色", -1)),
                                          n(
                                            "dd",
                                            null,
                                            m(w.value.character),
                                            1,
                                          ),
                                        ]),
                                        n("div", null, [
                                          S[41] ||
                                            (S[41] = n(
                                              "dt",
                                              null,
                                              "媒体类型",
                                              -1,
                                            )),
                                          n(
                                            "dd",
                                            null,
                                            m(w.value.media.type) + " · 图片",
                                            1,
                                          ),
                                        ]),
                                        n("div", null, [
                                          S[42] ||
                                            (S[42] = n(
                                              "dt",
                                              null,
                                              "工作流",
                                              -1,
                                            )),
                                          n(
                                            "dd",
                                            null,
                                            m(w.value.media.workflow),
                                            1,
                                          ),
                                        ]),
                                      ]),
                                      n("section", xv, [
                                        S[43] ||
                                          (S[43] = n(
                                            "h3",
                                            null,
                                            "提示词 / Prompt",
                                            -1,
                                          )),
                                        n(
                                          "pre",
                                          null,
                                          m(w.value.media.prompt),
                                          1,
                                        ),
                                      ]),
                                      n("section", Sv, [
                                        S[44] ||
                                          (S[44] = n(
                                            "h3",
                                            null,
                                            "负面提示词 / Negative prompt",
                                            -1,
                                          )),
                                        n(
                                          "pre",
                                          null,
                                          m(w.value.media.negativePrompt),
                                          1,
                                        ),
                                      ]),
                                      n("section", Tv, [
                                        S[45] ||
                                          (S[45] = n(
                                            "h3",
                                            null,
                                            "生成参数",
                                            -1,
                                          )),
                                        n("dl", Mv, [
                                          (c(!0),
                                          v(
                                            I,
                                            null,
                                            te(
                                              w.value.media.params,
                                              (y, $) => (
                                                c(),
                                                v("div", { key: $ }, [
                                                  n(
                                                    "dt",
                                                    null,
                                                    m(
                                                      {
                                                        width: "宽度",
                                                        height: "高度",
                                                        steps: "步数",
                                                        cfg: "CFG",
                                                        seed: "种子",
                                                        sampler: "采样器",
                                                      }[$],
                                                    ),
                                                    1,
                                                  ),
                                                  n("dd", null, m(y), 1),
                                                ])
                                              ),
                                            ),
                                            128,
                                          )),
                                        ]),
                                      ]),
                                      n("section", Iv, [
                                        S[48] ||
                                          (S[48] = n("h3", null, "输出", -1)),
                                        w.value.status === "success"
                                          ? (c(),
                                            v("div", Ev, [
                                              S[46] ||
                                                (S[46] = n(
                                                  "span",
                                                  null,
                                                  "◇",
                                                  -1,
                                                )),
                                              n(
                                                "p",
                                                null,
                                                m(w.value.media.output),
                                                1,
                                              ),
                                              S[47] ||
                                                (S[47] = n(
                                                  "small",
                                                  null,
                                                  "图片占位预览 · 本地模拟",
                                                  -1,
                                                )),
                                            ]))
                                          : (c(),
                                            v(
                                              "p",
                                              Av,
                                              m(
                                                w.value.status === "pending"
                                                  ? "等待图片工作流完成"
                                                  : "未生成输出，请重试。",
                                              ),
                                              1,
                                            )),
                                      ]),
                                    ],
                                    64,
                                  ))
                                : (c(),
                                  v(
                                    I,
                                    { key: 2 },
                                    [
                                      n("p", Vv, [
                                        K("模型 · " + m(w.value.model), 1),
                                        w.value.tokens
                                          ? (c(),
                                            v(
                                              I,
                                              { key: 0 },
                                              [
                                                K(
                                                  " · " +
                                                    m(w.value.tokens) +
                                                    " token",
                                                  1,
                                                ),
                                              ],
                                              64,
                                            ))
                                          : J("", !0),
                                      ]),
                                      n("section", Ov, [
                                        S[49] ||
                                          (S[49] = n(
                                            "h3",
                                            null,
                                            "请求提示词 / Prompt",
                                            -1,
                                          )),
                                        n("pre", null, m(w.value.input), 1),
                                      ]),
                                      n("section", Uv, [
                                        S[50] ||
                                          (S[50] = n(
                                            "h3",
                                            null,
                                            "响应内容",
                                            -1,
                                          )),
                                        n(
                                          "pre",
                                          null,
                                          m(
                                            w.value.status === "success"
                                              ? w.value.response
                                              : "尚未收到响应",
                                          ),
                                          1,
                                        ),
                                      ]),
                                    ],
                                    64,
                                  )),
                              n("details", Pv, [
                                S[52] ||
                                  (S[52] = n("summary", null, "技术详情", -1)),
                                n("p", null, [
                                  K(
                                    "失败码：" +
                                      m(
                                        w.value.status === "error"
                                          ? w.value.code
                                          : "无",
                                      ),
                                    1,
                                  ),
                                  S[51] || (S[51] = n("br", null, null, -1)),
                                  K("关联 ID：demo" + m(w.value.id), 1),
                                ]),
                              ]),
                            ]),
                            _: 1,
                          },
                          8,
                          ["title"],
                        ))
                      : J("", !0),
                    n("div", Lv, [
                      S[54] || (S[54] = n("small", null, "其它诊断", -1)),
                      n("div", Nv, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            h(Xo),
                            (y) => (
                              c(),
                              v(
                                "button",
                                {
                                  class: "text-button",
                                  onClick: ($) =>
                                    (P.value =
                                      P.value === y.title ? "" : y.title),
                                },
                                m(y.title),
                                9,
                                Rv,
                              )
                            ),
                          ),
                          256,
                        )),
                      ]),
                      q.value
                        ? (c(),
                          v("div", Fv, [
                            n("div", jv, [
                              n("b", null, m(q.value.title) + " · 模拟记录", 1),
                              S[53] ||
                                (S[53] = n(
                                  "span",
                                  { class: "spacer" },
                                  null,
                                  -1,
                                )),
                              n(
                                "button",
                                {
                                  onClick:
                                    S[10] || (S[10] = (y) => (P.value = "")),
                                },
                                "关闭",
                              ),
                            ]),
                            (c(!0),
                            v(
                              I,
                              null,
                              te(
                                q.value.rows,
                                (y) => (c(), v("p", null, m(y), 1)),
                              ),
                              256,
                            )),
                            ["后台任务", "待处理事项 / 重试耗尽"].includes(
                              P.value,
                            )
                              ? (c(),
                                v(
                                  "button",
                                  {
                                    key: 0,
                                    class: "text-button",
                                    onClick:
                                      S[11] || (S[11] = (y) => T("任务重试")),
                                  },
                                  "重试",
                                ))
                              : J("", !0),
                            n("p", Dv, m(V.value), 1),
                          ]))
                        : J("", !0),
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
  Bv = (e, t) => {
    const s = e.__vccOpts || e;
    for (const [l, o] of t) s[l] = o;
    return s;
  },
  Kv = Bv(Hv, [["__scopeId", "data-v-5bb367ad"]]),
  zv = ["aria-label"],
  ss = je({
    __name: "CharacterAvatar",
    props: {
      name: {},
      initial: {},
      color: {},
      avatarMode: {},
      small: { type: Boolean },
    },
    setup(e) {
      return (t, s) => (
        c(),
        v(
          "span",
          {
            class: ke(["avatar", { small: e.small }]),
            style: at({ background: e.color }),
            "aria-label":
              e.avatarMode === "image" ? "图片头像占位" : e.name + "的头像",
          },
          [
            e.avatarMode === "image"
              ? (c(),
                Se(h(Sn), { key: 0, size: e.small ? 16 : 24 }, null, 8, [
                  "size",
                ]))
              : (c(),
                v(
                  I,
                  { key: 1 },
                  [K(m(e.initial || Array.from(e.name).at(-1) || "你"), 1)],
                  64,
                )),
          ],
          14,
          zv,
        )
      );
    },
  }),
  qv = { class: "row library-toolbar" },
  Wv = { class: "muted" },
  Jv = { class: "actions-menu library-demo-menu" },
  Gv = {
    key: 0,
    class: "card library-first-use",
    "data-testid": "library-empty-guide",
  },
  Yv = { key: 0, class: "muted" },
  Zv = { class: "row" },
  Qv = { class: "library-character-copy" },
  Xv = { key: 0, class: "pill current-badge" },
  eh = { class: "actions-menu" },
  th = ["aria-label"],
  sh = ["onClick"],
  lh = ["onClick"],
  nh = { key: 0, class: "card empty-state" },
  oh = je({
    __name: "LibraryPage",
    setup(e) {
      const t = z(""),
        s = z({ tag: "" }),
        l = ye(() => [
          {
            key: "tag",
            label: "标签",
            options: [
              ...new Set([
                "温柔",
                "创作",
                "沉稳",
                "城市",
                "自然",
                ...Ae.value.flatMap((u) => u.tags),
              ]),
            ],
          },
        ]),
        o = ye(() =>
          Ae.value.filter(
            (u) =>
              (u.name + " " + u.job + " " + u.bio)
                .toLowerCase()
                .includes(t.value.trim().toLowerCase()) &&
              (!s.value.tag || u.tags.includes(s.value.tag)),
          ),
        );
      function i(u) {
        ((lt.value = u), (location.hash = encodeURIComponent("角色详情")));
      }
      function r() {
        (Cr(),
          (t.value = ""),
          (s.value = { tag: "" }),
          (location.hash = encodeURIComponent("角色库")));
      }
      return (u, d) => (
        c(),
        v(
          I,
          null,
          [
            n("div", qv, [
              n(
                "p",
                Wv,
                m(
                  h(Ae).length
                    ? "在这里，遇见和你一起生活的人。"
                    : "相识，可以从一句简单的介绍开始。",
                ),
                1,
              ),
              d[8] || (d[8] = n("span", { class: "spacer" }, null, -1)),
              h(Ae).length
                ? (c(),
                  v(
                    "button",
                    {
                      key: 0,
                      class: "button primary",
                      onClick: d[0] || (d[0] = (...f) => h(Ps) && h(Ps)(...f)),
                    },
                    m(h(ts) && h(Ke) !== "complete" ? "继续创建" : "创建角色"),
                    1,
                  ))
                : J("", !0),
              n("details", Jv, [
                d[7] ||
                  (d[7] = n(
                    "summary",
                    { "aria-label": "角色库演示选项" },
                    "⋯",
                    -1,
                  )),
                n("div", null, [
                  n(
                    "button",
                    { "data-testid": "library-demo-empty", onClick: r },
                    "演示：清空角色（首次使用）",
                  ),
                  h(Ae).length
                    ? (c(),
                      v(
                        "button",
                        {
                          key: 0,
                          "data-testid": "library-restore",
                          onClick:
                            d[1] || (d[1] = (...f) => h(vl) && h(vl)(...f)),
                        },
                        "恢复示例角色",
                      ))
                    : J("", !0),
                ]),
              ]),
            ]),
            h(Ae).length
              ? (c(),
                v(
                  I,
                  { key: 1 },
                  [
                    ae(
                      bs,
                      {
                        search: t.value,
                        "onUpdate:search":
                          d[4] || (d[4] = (f) => (t.value = f)),
                        filters: s.value,
                        "onUpdate:filters":
                          d[5] || (d[5] = (f) => (s.value = f)),
                        groups: l.value,
                        "test-id": "library-filters",
                        placeholder: "搜索角色 / 职业",
                      },
                      null,
                      8,
                      ["search", "filters", "groups"],
                    ),
                    (c(!0),
                    v(
                      I,
                      null,
                      te(
                        o.value,
                        (f) => (
                          c(),
                          v(
                            "article",
                            {
                              key: f.id,
                              class: "card compact library-character",
                            },
                            [
                              n("div", Zv, [
                                ae(
                                  ss,
                                  {
                                    name: f.name,
                                    initial: f.initial,
                                    color: f.color,
                                    "avatar-mode": f.avatarMode,
                                  },
                                  null,
                                  8,
                                  ["name", "initial", "color", "avatar-mode"],
                                ),
                                n("div", Qv, [
                                  n("b", null, m(f.name) + " · " + m(f.job), 1),
                                  n("p", null, m(f.subtitle), 1),
                                  (c(!0),
                                  v(
                                    I,
                                    null,
                                    te(
                                      f.tags,
                                      (a) => (
                                        c(),
                                        v(
                                          "span",
                                          { key: a, class: "pill" },
                                          m(a),
                                          1,
                                        )
                                      ),
                                    ),
                                    128,
                                  )),
                                ]),
                                d[11] ||
                                  (d[11] = n(
                                    "span",
                                    { class: "spacer" },
                                    null,
                                    -1,
                                  )),
                                h(lt) === f.id
                                  ? (c(), v("span", Xv, "当前"))
                                  : J("", !0),
                                n("details", eh, [
                                  n(
                                    "summary",
                                    { "aria-label": f.name + "的操作" },
                                    "⋯",
                                    8,
                                    th,
                                  ),
                                  n("div", null, [
                                    n(
                                      "button",
                                      { onClick: (a) => i(f.id) },
                                      "角色详情",
                                      8,
                                      sh,
                                    ),
                                    h(lt) !== f.id
                                      ? (c(),
                                        v(
                                          "button",
                                          {
                                            key: 0,
                                            onClick: (a) => (lt.value = f.id),
                                          },
                                          "切换角色",
                                          8,
                                          lh,
                                        ))
                                      : J("", !0),
                                  ]),
                                ]),
                              ]),
                            ],
                          )
                        ),
                      ),
                      128,
                    )),
                    o.value.length
                      ? J("", !0)
                      : (c(),
                        v("div", nh, [
                          d[12] || (d[12] = n("p", null, "暂无匹配角色。", -1)),
                          n(
                            "button",
                            {
                              class: "button",
                              onClick:
                                d[6] ||
                                (d[6] = (f) => {
                                  ((t.value = ""), (s.value = { tag: "" }));
                                }),
                            },
                            "清除筛选",
                          ),
                        ])),
                  ],
                  64,
                ))
              : (c(),
                v("section", Gv, [
                  ae(h(Tn), { size: 30, "aria-hidden": "true" }),
                  d[9] || (d[9] = n("h2", null, "还没有一起生活的人", -1)),
                  d[10] ||
                    (d[10] = n(
                      "p",
                      null,
                      [
                        K(
                          "给一个名字，聊聊性格，再为对方准备一个世界和几件日常衣物。",
                        ),
                        n("br"),
                        K("我们会一步步陪你准备，慢慢开启新的生活。"),
                      ],
                      -1,
                    )),
                  n(
                    "button",
                    {
                      class: "button primary",
                      "data-testid": "library-create-first",
                      onClick: d[2] || (d[2] = (...f) => h(Ps) && h(Ps)(...f)),
                    },
                    "创建第一个角色",
                  ),
                  h(ts) && h(Ke) !== "complete"
                    ? (c(), v("p", Yv, "上次的资料和准备进度已经为你保留。"))
                    : J("", !0),
                  n(
                    "button",
                    {
                      class: "text-button library-restore",
                      "data-testid": "library-restore",
                      onClick: d[3] || (d[3] = (...f) => h(vl) && h(vl)(...f)),
                    },
                    "恢复示例角色",
                  ),
                ])),
          ],
          64,
        )
      );
    },
  }),
  ih = { class: "character-fieldset" },
  rh = { class: "row avatar-settings" },
  ah = { class: "character-stack" },
  uh = { class: "chip-options", "aria-label": "头像形式" },
  dh = ["aria-pressed"],
  ch = ["aria-pressed"],
  fh = { key: 0 },
  ph = { key: 1, class: "character-inline-label" },
  mh = ["placeholder"],
  vh = { class: "avatar-colors", "aria-label": "头像底色" },
  hh = ["aria-label", "aria-pressed", "onClick"],
  Sr = je({
    __name: "AvatarSettings",
    props: qa(
      { name: {} },
      {
        color: { required: !0 },
        colorModifiers: {},
        initial: { required: !0 },
        initialModifiers: {},
        mode: { required: !0 },
        modeModifiers: {},
      },
    ),
    emits: ["update:color", "update:initial", "update:mode"],
    setup(e) {
      const t = ys(e, "color"),
        s = ys(e, "initial"),
        l = ys(e, "mode");
      return (o, i) => (
        c(),
        v("fieldset", ih, [
          i[4] || (i[4] = n("legend", null, "头像", -1)),
          n("div", rh, [
            ae(
              ss,
              {
                name: e.name,
                initial: s.value,
                color: t.value,
                "avatar-mode": l.value,
              },
              null,
              8,
              ["name", "initial", "color", "avatar-mode"],
            ),
            n("div", ah, [
              n("div", uh, [
                n(
                  "button",
                  {
                    type: "button",
                    class: "filter-chip",
                    "aria-pressed": l.value === "initial",
                    onClick: i[0] || (i[0] = (r) => (l.value = "initial")),
                  },
                  "颜色与文字",
                  8,
                  dh,
                ),
                n(
                  "button",
                  {
                    type: "button",
                    class: "filter-chip",
                    "aria-pressed": l.value === "image",
                    onClick: i[1] || (i[1] = (r) => (l.value = "image")),
                  },
                  "图片占位",
                  8,
                  ch,
                ),
              ]),
              l.value === "image"
                ? (c(), v("small", fh, "头像上传占位 · 本原型使用本地占位图。"))
                : (c(),
                  v("label", ph, [
                    i[3] || (i[3] = K("头像文字", -1)),
                    re(
                      n(
                        "input",
                        {
                          "onUpdate:modelValue":
                            i[2] || (i[2] = (r) => (s.value = r)),
                          maxlength: "2",
                          placeholder:
                            Array.from(e.name).at(-1) || "自动取名字末字",
                        },
                        null,
                        8,
                        mh,
                      ),
                      [[he, s.value]],
                    ),
                  ])),
            ]),
          ]),
          n("div", vh, [
            (c(!0),
            v(
              I,
              null,
              te(
                h(br),
                (r, u) => (
                  c(),
                  v(
                    "button",
                    {
                      key: r,
                      type: "button",
                      style: at({ background: r }),
                      "aria-label": [
                        "鼠尾草绿",
                        "雾蓝",
                        "暖杏",
                        "米黄",
                        "淡紫",
                      ][u],
                      "aria-pressed": t.value === r,
                      onClick: (d) => (t.value = r),
                    },
                    m(t.value === r ? "✓" : ""),
                    13,
                    hh,
                  )
                ),
              ),
              128,
            )),
          ]),
        ])
      );
    },
  }),
  gh = { class: "character-fieldset" },
  yh = { class: "chip-options" },
  bh = ["aria-pressed", "onClick"],
  Tr = je({
    __name: "PersonalityFields",
    props: {
      traits: { required: !0 },
      traitsModifiers: {},
      style: { required: !0 },
      styleModifiers: {},
      values: { required: !0 },
      valuesModifiers: {},
    },
    emits: ["update:traits", "update:style", "update:values"],
    setup(e) {
      const t = ys(e, "traits"),
        s = ys(e, "style"),
        l = ys(e, "values");
      function o(i) {
        t.value = t.value.includes(i)
          ? t.value.filter((r) => r !== i)
          : [...t.value, i];
      }
      return (i, r) => (
        c(),
        v(
          I,
          null,
          [
            n("fieldset", gh, [
              r[2] || (r[2] = n("legend", null, "性格特质", -1)),
              n("div", yh, [
                (c(!0),
                v(
                  I,
                  null,
                  te(
                    [...new Set([...h(Yd), ...t.value])],
                    (u) => (
                      c(),
                      v(
                        "button",
                        {
                          key: u,
                          type: "button",
                          class: "filter-chip",
                          "aria-pressed": t.value.includes(u),
                          onClick: (d) => o(u),
                        },
                        m(u),
                        9,
                        bh,
                      )
                    ),
                  ),
                  128,
                )),
              ]),
            ]),
            n("label", null, [
              r[3] || (r[3] = K("说话方式", -1)),
              re(
                n(
                  "textarea",
                  {
                    "onUpdate:modelValue":
                      r[0] || (r[0] = (u) => (s.value = u)),
                    rows: "2",
                    maxlength: "500",
                    required: "",
                    placeholder: "例如：温和直接，喜欢分享生活里的小发现。",
                  },
                  null,
                  512,
                ),
                [[he, s.value]],
              ),
            ]),
            n("label", null, [
              r[4] || (r[4] = K("核心价值观", -1)),
              re(
                n(
                  "textarea",
                  {
                    "onUpdate:modelValue":
                      r[1] || (r[1] = (u) => (l.value = u)),
                    rows: "2",
                    maxlength: "500",
                    required: "",
                    placeholder: "例如：真诚、尊重边界，认真对待每一次相处。",
                  },
                  null,
                  512,
                ),
                [[he, l.value]],
              ),
            ]),
          ],
          64,
        )
      );
    },
  }),
  kh = { class: "character-workflow" },
  _h = { class: "row" },
  wh = { class: "wizard-steps", "aria-label": "创建角色步骤" },
  $h = ["aria-current"],
  Ch = ["data-testid"],
  xh = { class: "muted" },
  Sh = { class: "form-grid" },
  Th = ["value"],
  Mh = { key: 0, class: "form-grid" },
  Ih = { id: "wizard-home-locations" },
  Eh = ["value"],
  Ah = { class: "character-fieldset" },
  Vh = { class: "chip-options" },
  Oh = ["aria-pressed", "onClick"],
  Uh = { class: "character-fieldset" },
  Ph = { class: "starter-garments" },
  Lh = ["value"],
  Nh = { class: "wizard-review" },
  Rh = { key: 0, role: "alert", class: "inline-error" },
  Fh = { class: "wizard-footer" },
  jh = ["disabled"],
  Dh = { key: 0, class: "button primary", "data-testid": "wizard-next" },
  Hh = { key: 1, class: "button primary", "data-testid": "wizard-submit" },
  Bh = { key: 1, class: "card character-preparation" },
  Kh = { class: "muted" },
  zh = { class: "preparation-list", "aria-label": "角色准备进度" },
  qh = ["data-testid", "data-state"],
  Wh = { class: "preparation-number" },
  Jh = { class: "preparation-copy" },
  Gh = { class: "row" },
  Yh = { key: 0, class: "preparation-error" },
  Zh = { role: "alert" },
  Qh = { class: "row" },
  Xh = ["href"],
  eg = ["data-testid", "onClick"],
  tg = { key: 0, class: "row preparation-done" },
  sg = { key: 2, class: "wizard-demo-toggle" },
  lg = je({
    __name: "CharacterWizardPage",
    setup(e) {
      ts.value = !0;
      const t = ["基本资料", "人格", "世界与地点", "衣柜初始化"],
        s = z(""),
        l = ye(() => Oe.value.find((f) => f.id === fe.value.worldId));
      function o() {
        ((s.value = Vl(Ne.value)), s.value || Ne.value++);
      }
      function i() {
        ((s.value = Vl(4)), s.value || ec());
      }
      function r(f) {
        const a = fe.value.wardrobeStyles;
        fe.value.wardrobeStyles = a.includes(f)
          ? a.filter((p) => p !== f)
          : [...a, f];
      }
      function u() {
        location.hash = encodeURIComponent("角色库");
      }
      function d() {
        ((Zn.value = "配置中心"),
          (location.hash =
            encodeURIComponent("管理中心") +
            "?section=" +
            encodeURIComponent("图片（ComfyUI）")));
      }
      return (f, a) => {
        var p, _;
        return (
          c(),
          v("div", kh, [
            n("div", _h, [
              n("button", { class: "text-button", onClick: u }, "角色库"),
              ae(h(Jn), { size: 13 }),
              n(
                "small",
                null,
                m(h(Ke) === "editing" ? "新的相识" : "为新的生活做准备"),
                1,
              ),
            ]),
            h(Ke) === "editing"
              ? (c(),
                v(
                  I,
                  { key: 0 },
                  [
                    n("ol", wh, [
                      (c(),
                      v(
                        I,
                        null,
                        te(t, (b, A) =>
                          n(
                            "li",
                            {
                              key: b,
                              "aria-current": h(Ne) === A + 1 ? "step" : void 0,
                              class: ke({
                                active: h(Ne) === A + 1,
                                complete: h(Ne) > A + 1,
                              }),
                            },
                            [n("span", null, m(A + 1), 1), K(m(b), 1)],
                            10,
                            $h,
                          ),
                        ),
                        64,
                      )),
                    ]),
                    n(
                      "form",
                      {
                        class: "card character-form",
                        onSubmit:
                          a[15] ||
                          (a[15] = Ue(
                            (b) => (h(Ne) < 4 ? o() : i()),
                            ["prevent"],
                          )),
                      },
                      [
                        (c(),
                        v(
                          "section",
                          {
                            key: h(Ne),
                            "data-testid": "wizard-step-" + h(Ne),
                            class: "character-stack",
                          },
                          [
                            n("header", null, [
                              n("small", null, "第 " + m(h(Ne)) + " / 4 步", 1),
                              n("h2", null, m(t[h(Ne) - 1]), 1),
                              n(
                                "p",
                                xh,
                                m(
                                  [
                                    "先从一个名字和一点日常开始。",
                                    "每一种性格，都有自己的相处节奏。",
                                    "给日常安放一个温柔的起点。",
                                    "挑几件日常衣物，以后再慢慢添置。",
                                  ][h(Ne) - 1],
                                ),
                                1,
                              ),
                            ]),
                            h(Ne) === 1
                              ? (c(),
                                v(
                                  I,
                                  { key: 0 },
                                  [
                                    n("div", Sh, [
                                      n("label", null, [
                                        a[18] || (a[18] = K("名字", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                a[0] ||
                                                (a[0] = (b) =>
                                                  (h(fe).name = b)),
                                              "data-testid": "wizard-name",
                                              required: "",
                                              maxlength: "30",
                                              placeholder: "你想怎么称呼对方？",
                                            },
                                            null,
                                            512,
                                          ),
                                          [[he, h(fe).name]],
                                        ),
                                      ]),
                                      n("label", null, [
                                        a[19] || (a[19] = K("年龄", -1)),
                                        re(
                                          n(
                                            "input",
                                            {
                                              "onUpdate:modelValue":
                                                a[1] ||
                                                (a[1] = (b) => (h(fe).age = b)),
                                              "data-testid": "wizard-age",
                                              type: "number",
                                              min: "1",
                                              max: "150",
                                              step: "1",
                                              required: "",
                                            },
                                            null,
                                            512,
                                          ),
                                          [
                                            [
                                              he,
                                              h(fe).age,
                                              void 0,
                                              { number: !0 },
                                            ],
                                          ],
                                        ),
                                      ]),
                                    ]),
                                    n("label", null, [
                                      a[20] || (a[20] = K("简短介绍", -1)),
                                      re(
                                        n(
                                          "textarea",
                                          {
                                            "onUpdate:modelValue":
                                              a[2] ||
                                              (a[2] = (b) => (h(fe).intro = b)),
                                            "data-testid": "wizard-intro",
                                            rows: "3",
                                            maxlength: "500",
                                            required: "",
                                            placeholder:
                                              "喜欢什么、怎样生活，或是你期待的初次相识。",
                                          },
                                          null,
                                          512,
                                        ),
                                        [[he, h(fe).intro]],
                                      ),
                                    ]),
                                    ae(
                                      Sr,
                                      {
                                        color: h(fe).color,
                                        "onUpdate:color":
                                          a[3] ||
                                          (a[3] = (b) => (h(fe).color = b)),
                                        initial: h(fe).initial,
                                        "onUpdate:initial":
                                          a[4] ||
                                          (a[4] = (b) => (h(fe).initial = b)),
                                        mode: h(fe).avatarMode,
                                        "onUpdate:mode":
                                          a[5] ||
                                          (a[5] = (b) =>
                                            (h(fe).avatarMode = b)),
                                        name: h(fe).name,
                                      },
                                      null,
                                      8,
                                      ["color", "initial", "mode", "name"],
                                    ),
                                  ],
                                  64,
                                ))
                              : h(Ne) === 2
                                ? (c(),
                                  Se(
                                    Tr,
                                    {
                                      key: 1,
                                      traits: h(fe).traits,
                                      "onUpdate:traits":
                                        a[6] ||
                                        (a[6] = (b) => (h(fe).traits = b)),
                                      style: at(h(fe).speakingStyle),
                                      "onUpdate:style":
                                        a[7] ||
                                        (a[7] = (b) =>
                                          (h(fe).speakingStyle = b)),
                                      values: h(fe).coreValues,
                                      "onUpdate:values":
                                        a[8] ||
                                        (a[8] = (b) => (h(fe).coreValues = b)),
                                    },
                                    null,
                                    8,
                                    ["traits", "style", "values"],
                                  ))
                                : h(Ne) === 3
                                  ? (c(),
                                    v(
                                      I,
                                      { key: 2 },
                                      [
                                        n("label", null, [
                                          a[22] || (a[22] = K("所在世界", -1)),
                                          re(
                                            n(
                                              "select",
                                              {
                                                "onUpdate:modelValue":
                                                  a[9] ||
                                                  (a[9] = (b) =>
                                                    (h(fe).worldId = b)),
                                                "data-testid": "wizard-world",
                                              },
                                              [
                                                (c(!0),
                                                v(
                                                  I,
                                                  null,
                                                  te(
                                                    h(Oe),
                                                    (b) => (
                                                      c(),
                                                      v(
                                                        "option",
                                                        {
                                                          key: b.id,
                                                          value: b.id,
                                                        },
                                                        m(b.name),
                                                        9,
                                                        Th,
                                                      )
                                                    ),
                                                  ),
                                                  128,
                                                )),
                                                a[21] ||
                                                  (a[21] = n(
                                                    "option",
                                                    { value: "new" },
                                                    "创建一个新世界…",
                                                    -1,
                                                  )),
                                              ],
                                              512,
                                            ),
                                            [[pt, h(fe).worldId]],
                                          ),
                                        ]),
                                        h(fe).worldId === "new"
                                          ? (c(),
                                            v("div", Mh, [
                                              n("label", null, [
                                                a[23] ||
                                                  (a[23] = K("世界名称", -1)),
                                                re(
                                                  n(
                                                    "input",
                                                    {
                                                      "onUpdate:modelValue":
                                                        a[10] ||
                                                        (a[10] = (b) =>
                                                          (h(fe).worldName =
                                                            b)),
                                                      required: "",
                                                      maxlength: "60",
                                                      placeholder:
                                                        "例如：有风的小城",
                                                    },
                                                    null,
                                                    512,
                                                  ),
                                                  [[he, h(fe).worldName]],
                                                ),
                                              ]),
                                              n("label", null, [
                                                a[24] ||
                                                  (a[24] = K("世界类型", -1)),
                                                re(
                                                  n(
                                                    "input",
                                                    {
                                                      "onUpdate:modelValue":
                                                        a[11] ||
                                                        (a[11] = (b) =>
                                                          (h(fe).genre = b)),
                                                      maxlength: "40",
                                                      placeholder: "现代都市",
                                                    },
                                                    null,
                                                    512,
                                                  ),
                                                  [[he, h(fe).genre]],
                                                ),
                                              ]),
                                            ]))
                                          : J("", !0),
                                        n("label", null, [
                                          a[25] || (a[25] = K("居住地点", -1)),
                                          re(
                                            n(
                                              "input",
                                              {
                                                "onUpdate:modelValue":
                                                  a[12] ||
                                                  (a[12] = (b) =>
                                                    (h(fe).homeLocation = b)),
                                                list: "wizard-home-locations",
                                                required: "",
                                                maxlength: "100",
                                                placeholder:
                                                  "例如：河岸公寓 · 卧室",
                                              },
                                              null,
                                              512,
                                            ),
                                            [[he, h(fe).homeLocation]],
                                          ),
                                          n("datalist", Ih, [
                                            (c(!0),
                                            v(
                                              I,
                                              null,
                                              te(
                                                ((p = l.value) == null
                                                  ? void 0
                                                  : p.sites.flatMap(
                                                      (b) => b.places,
                                                    )) || [],
                                                (b) => (
                                                  c(),
                                                  v(
                                                    "option",
                                                    {
                                                      key: b.id,
                                                      value: b.name,
                                                    },
                                                    null,
                                                    8,
                                                    Eh,
                                                  )
                                                ),
                                              ),
                                              128,
                                            )),
                                          ]),
                                        ]),
                                      ],
                                      64,
                                    ))
                                  : (c(),
                                    v(
                                      I,
                                      { key: 3 },
                                      [
                                        n("fieldset", Ah, [
                                          a[26] ||
                                            (a[26] = n(
                                              "legend",
                                              null,
                                              "穿衣风格",
                                              -1,
                                            )),
                                          n("div", Vh, [
                                            (c(!0),
                                            v(
                                              I,
                                              null,
                                              te(
                                                h(Zd),
                                                (b) => (
                                                  c(),
                                                  v(
                                                    "button",
                                                    {
                                                      key: b,
                                                      type: "button",
                                                      class: "filter-chip",
                                                      "aria-pressed":
                                                        h(
                                                          fe,
                                                        ).wardrobeStyles.includes(
                                                          b,
                                                        ),
                                                      onClick: (A) => r(b),
                                                    },
                                                    m(b),
                                                    9,
                                                    Oh,
                                                  )
                                                ),
                                              ),
                                              128,
                                            )),
                                          ]),
                                        ]),
                                        n("fieldset", Uh, [
                                          a[27] ||
                                            (a[27] = n(
                                              "legend",
                                              null,
                                              "初始衣物 · 可多选",
                                              -1,
                                            )),
                                          n("div", Ph, [
                                            (c(!0),
                                            v(
                                              I,
                                              null,
                                              te(
                                                h(kr),
                                                (b) => (
                                                  c(),
                                                  v("label", { key: b.id }, [
                                                    re(
                                                      n(
                                                        "input",
                                                        {
                                                          "onUpdate:modelValue":
                                                            a[13] ||
                                                            (a[13] = (A) =>
                                                              (h(
                                                                fe,
                                                              ).garmentIds =
                                                                A)),
                                                          type: "checkbox",
                                                          value: b.id,
                                                        },
                                                        null,
                                                        8,
                                                        Lh,
                                                      ),
                                                      [[Ft, h(fe).garmentIds]],
                                                    ),
                                                    n(
                                                      "span",
                                                      null,
                                                      m(b.name),
                                                      1,
                                                    ),
                                                    n(
                                                      "small",
                                                      null,
                                                      m(b.type),
                                                      1,
                                                    ),
                                                  ])
                                                ),
                                              ),
                                              128,
                                            )),
                                          ]),
                                        ]),
                                        n("div", Nh, [
                                          ae(
                                            ss,
                                            {
                                              name: h(fe).name,
                                              initial: h(fe).initial,
                                              color: h(fe).color,
                                              "avatar-mode": h(fe).avatarMode,
                                            },
                                            null,
                                            8,
                                            [
                                              "name",
                                              "initial",
                                              "color",
                                              "avatar-mode",
                                            ],
                                          ),
                                          n("div", null, [
                                            n(
                                              "b",
                                              null,
                                              m(h(fe).name) +
                                                " · " +
                                                m(h(fe).age) +
                                                " 岁",
                                              1,
                                            ),
                                            n(
                                              "p",
                                              null,
                                              m(h(fe).traits.join("、")) +
                                                " · " +
                                                m(
                                                  h(fe).worldId === "new"
                                                    ? h(fe).worldName
                                                    : (_ = l.value) == null
                                                      ? void 0
                                                      : _.name,
                                                ) +
                                                " · " +
                                                m(h(fe).homeLocation),
                                              1,
                                            ),
                                            a[28] ||
                                              (a[28] = n(
                                                "small",
                                                null,
                                                "确认后，将按下面六个环节逐步准备。",
                                                -1,
                                              )),
                                          ]),
                                        ]),
                                      ],
                                      64,
                                    )),
                          ],
                          8,
                          Ch,
                        )),
                        s.value ? (c(), v("p", Rh, m(s.value), 1)) : J("", !0),
                        n("footer", Fh, [
                          n(
                            "button",
                            {
                              type: "button",
                              class: "button",
                              "data-testid": "wizard-prev",
                              disabled: h(Ne) === 1,
                              onClick:
                                a[14] ||
                                (a[14] = (b) => {
                                  (Ne.value--, (s.value = ""));
                                }),
                            },
                            "上一步",
                            8,
                            jh,
                          ),
                          a[29] ||
                            (a[29] = n("span", { class: "spacer" }, null, -1)),
                          h(Ne) < 4
                            ? (c(), v("button", Dh, "下一步"))
                            : (c(), v("button", Hh, "确认并开始准备")),
                        ]),
                      ],
                      32,
                    ),
                  ],
                  64,
                ))
              : (c(),
                v("section", Bh, [
                  n("header", null, [
                    n("small", null, m(h(fe).name) + "的新生活", 1),
                    n(
                      "h2",
                      null,
                      m(
                        h(Ke) === "complete"
                          ? "准备好了，很高兴遇见你"
                          : "一点点，准备好相遇",
                      ),
                      1,
                    ),
                    n(
                      "p",
                      Kh,
                      m(
                        h(Ke) === "complete"
                          ? "资料和初始衣柜已保存，接下来一起走进日常。"
                          : "进度会为你保留。遇到问题时，可以单独重试未完成的环节。",
                      ),
                      1,
                    ),
                  ]),
                  n("ol", zh, [
                    (c(!0),
                    v(
                      I,
                      null,
                      te(
                        h(xs),
                        (b, A) => (
                          c(),
                          v(
                            "li",
                            {
                              key: b.key,
                              "data-testid": "prep-step-" + b.key,
                              "data-state": b.state,
                            },
                            [
                              n("span", Wh, [
                                b.state === "已完成"
                                  ? (c(), Se(h(bd), { key: 0, size: 15 }))
                                  : (c(),
                                    v(I, { key: 1 }, [K(m(A + 1), 1)], 64)),
                              ]),
                              n("div", Jh, [
                                n("div", Gh, [
                                  n("b", null, m(b.name), 1),
                                  a[30] ||
                                    (a[30] = n(
                                      "span",
                                      { class: "spacer" },
                                      null,
                                      -1,
                                    )),
                                  n(
                                    "span",
                                    {
                                      class: ke([
                                        "preparation-state",
                                        {
                                          "is-failed": b.state === "失败",
                                          "is-current": b.state === "进行中",
                                        },
                                      ]),
                                      role: "status",
                                    },
                                    m(b.state),
                                    3,
                                  ),
                                ]),
                                b.state === "失败"
                                  ? (c(),
                                    v("div", Yh, [
                                      n("p", Zh, m(b.reason), 1),
                                      n("div", Qh, [
                                        b.key === "reference"
                                          ? (c(),
                                            v(
                                              "a",
                                              {
                                                key: 0,
                                                class: "text-button",
                                                "data-testid":
                                                  "prep-goto-config",
                                                href:
                                                  "#" +
                                                  encodeURIComponent(
                                                    "管理中心",
                                                  ) +
                                                  "?section=" +
                                                  encodeURIComponent(
                                                    "图片（ComfyUI）",
                                                  ),
                                                onClick: Ue(d, ["prevent"]),
                                              },
                                              "去配置中心 ↗",
                                              8,
                                              Xh,
                                            ))
                                          : J("", !0),
                                        n(
                                          "button",
                                          {
                                            class: "button",
                                            "data-testid":
                                              "prep-retry-" + b.key,
                                            onClick: (P) => h(tc)(b.key),
                                          },
                                          "重试",
                                          8,
                                          eg,
                                        ),
                                      ]),
                                      b.key === "world"
                                        ? (c(),
                                          v(
                                            "button",
                                            {
                                              key: 0,
                                              class: "text-button",
                                              onClick:
                                                a[16] ||
                                                (a[16] = (P) => {
                                                  ((Ke.value = "editing"),
                                                    (Ne.value = 3));
                                                }),
                                            },
                                            "返回世界与地点",
                                          ))
                                        : J("", !0),
                                    ]))
                                  : J("", !0),
                              ]),
                            ],
                            8,
                            qh,
                          )
                        ),
                      ),
                      128,
                    )),
                  ]),
                  h(Ke) === "complete"
                    ? (c(),
                      v("div", tg, [
                        ae(
                          ss,
                          {
                            name: h(fe).name,
                            initial: h(fe).initial,
                            color: h(fe).color,
                            "avatar-mode": h(fe).avatarMode,
                          },
                          null,
                          8,
                          ["name", "initial", "color", "avatar-mode"],
                        ),
                        a[31] ||
                          (a[31] = n("span", { class: "spacer" }, null, -1)),
                        a[32] ||
                          (a[32] = n(
                            "a",
                            {
                              class: "button primary",
                              "data-testid": "prep-done-now",
                              href: "#此刻",
                            },
                            "去此刻",
                            -1,
                          )),
                        a[33] ||
                          (a[33] = n(
                            "a",
                            {
                              class: "button",
                              "data-testid": "prep-done-detail",
                              href: "#角色详情",
                            },
                            "角色详情",
                            -1,
                          )),
                      ]))
                    : J("", !0),
                ])),
            h(Ke) !== "complete"
              ? (c(),
                v("label", sg, [
                  re(
                    n(
                      "input",
                      {
                        "onUpdate:modelValue":
                          a[17] ||
                          (a[17] = (b) => (Fe(Bs) ? (Bs.value = b) : null)),
                        type: "checkbox",
                        "data-testid": "wizard-fail-toggle",
                      },
                      null,
                      512,
                    ),
                    [[Ft, h(Bs)]],
                  ),
                  a[34] || (a[34] = K("演示：某一步失败 ", -1)),
                  a[35] ||
                    (a[35] = n(
                      "small",
                      null,
                      "首次准备规范参考图时提示未配置；再次重试可完成。",
                      -1,
                    )),
                ]))
              : J("", !0),
          ])
        );
      };
    },
  }),
  ng = { class: "character-detail character-workflow" },
  og = { class: "card" },
  ig = { class: "row character-profile-heading" },
  rg = ["disabled"],
  ag = ["disabled"],
  ug = ["value"],
  dg = ["value"],
  cg = { key: 0, class: "inline-error", role: "alert" },
  fg = { class: "wizard-footer" },
  pg = ["disabled"],
  mg = ["disabled"],
  vg = { key: 1, class: "character-detail-sections" },
  hg = { "data-testid": "detail-intro" },
  gg = { class: "chip-options" },
  yg = { class: "row" },
  bg = { "data-testid": "detail-world" },
  kg = { class: "character-more" },
  _g = je({
    __name: "CharacterDetailPage",
    setup(e) {
      const t = z(!1),
        s = z(""),
        l = fs(["角色详情"]);
      function o() {
        const f = U.value;
        return {
          intro: f.bio,
          traits: [...f.traits],
          style: f.speakingStyle,
          values: f.coreValues,
          color: f.color,
          initial: f.initial,
          avatarMode: f.avatarMode,
          worldId: f.worldId,
        };
      }
      const i = z(o());
      function r() {
        ((i.value = o()), (s.value = ""), (t.value = !0));
      }
      function u() {
        ((i.value = o()), (s.value = ""), (t.value = !1));
      }
      function d() {
        if (l.value) return;
        s.value = "";
        const f = i.value;
        if (!f.intro.trim()) {
          s.value = "请填写角色简介。";
          return;
        }
        if (!f.traits.length || !f.style.trim() || !f.values.trim()) {
          s.value = "请选择性格特质，并补充说话方式和核心价值观。";
          return;
        }
        if (!Oe.value.some((_) => _.id === f.worldId)) {
          s.value = "请选择一个可用世界。";
          return;
        }
        const a = JSON.parse(JSON.stringify(f)),
          p = U.value;
        Xe(
          "角色详情",
          [a.intro, a.style, a.values, a.initial, ...a.traits].join(" "),
          () => {
            var b, A;
            const _ = Oe.value.find((P) => P.id === a.worldId);
            _ &&
              (p.worldId !== _.id &&
                (Oe.value.forEach(
                  (P) => (P.residents = P.residents.filter((V) => V !== p.id)),
                ),
                _.residents.push(p.id),
                (p.world = _.name),
                (p.worldId = _.id),
                (p.genre = _.genre),
                (p.sites = JSON.parse(JSON.stringify(_.sites))),
                (p.homeLocation =
                  ((A = (b = _.sites[0]) == null ? void 0 : b.places[0]) == null
                    ? void 0
                    : A.name) || _.name),
                (p.location = p.homeLocation)),
              Object.assign(p, {
                bio: a.intro.trim(),
                subtitle: a.intro.trim(),
                traits: a.traits,
                tags: [...a.traits],
                speakingStyle: a.style.trim(),
                coreValues: a.values.trim(),
                color: a.color,
                initial: a.initial.trim() || Array.from(p.name).at(-1),
                avatarMode: a.avatarMode,
              }),
              (t.value = !1));
          },
          p.name,
          "角色资料已保存。",
        );
      }
      return (f, a) => {
        var p;
        return (
          c(),
          v("div", ng, [
            n("section", og, [
              n("header", ig, [
                ae(
                  ss,
                  {
                    name: h(U).name,
                    initial: h(U).initial,
                    color: h(U).color,
                    "avatar-mode": h(U).avatarMode,
                  },
                  null,
                  8,
                  ["name", "initial", "color", "avatar-mode"],
                ),
                n("div", null, [
                  n("h2", null, m(h(U).name), 1),
                  n("small", null, m(h(U).age) + " 岁 · " + m(h(U).job), 1),
                ]),
                a[10] || (a[10] = n("span", { class: "spacer" }, null, -1)),
                t.value
                  ? J("", !0)
                  : (c(),
                    v(
                      "button",
                      {
                        key: 0,
                        class: "button",
                        "data-testid": "detail-edit",
                        disabled: h(l),
                        onClick: r,
                      },
                      "编辑资料",
                      8,
                      rg,
                    )),
              ]),
              t.value
                ? (c(),
                  v(
                    "form",
                    {
                      key: 0,
                      class: "character-form detail-form",
                      onSubmit: Ue(d, ["prevent"]),
                    },
                    [
                      n(
                        "fieldset",
                        {
                          disabled: h(l),
                          class: "character-fieldset character-stack",
                        },
                        [
                          n("label", null, [
                            a[11] || (a[11] = K("简介", -1)),
                            re(
                              n(
                                "textarea",
                                {
                                  "onUpdate:modelValue":
                                    a[0] || (a[0] = (_) => (i.value.intro = _)),
                                  "data-testid": "detail-intro",
                                  rows: "4",
                                  required: "",
                                  maxlength: "500",
                                },
                                null,
                                512,
                              ),
                              [[he, i.value.intro]],
                            ),
                          ]),
                          a[13] || (a[13] = n("h3", null, "人格", -1)),
                          ae(
                            Tr,
                            {
                              traits: i.value.traits,
                              "onUpdate:traits":
                                a[1] || (a[1] = (_) => (i.value.traits = _)),
                              style: at(i.value.style),
                              "onUpdate:style":
                                a[2] || (a[2] = (_) => (i.value.style = _)),
                              values: i.value.values,
                              "onUpdate:values":
                                a[3] || (a[3] = (_) => (i.value.values = _)),
                            },
                            null,
                            8,
                            ["traits", "style", "values"],
                          ),
                          ae(
                            Sr,
                            {
                              color: i.value.color,
                              "onUpdate:color":
                                a[4] || (a[4] = (_) => (i.value.color = _)),
                              initial: i.value.initial,
                              "onUpdate:initial":
                                a[5] || (a[5] = (_) => (i.value.initial = _)),
                              mode: i.value.avatarMode,
                              "onUpdate:mode":
                                a[6] ||
                                (a[6] = (_) => (i.value.avatarMode = _)),
                              name: h(U).name,
                            },
                            null,
                            8,
                            ["color", "initial", "mode", "name"],
                          ),
                          n("label", null, [
                            a[12] || (a[12] = K("所在世界", -1)),
                            re(
                              n(
                                "select",
                                {
                                  "onUpdate:modelValue":
                                    a[7] ||
                                    (a[7] = (_) => (i.value.worldId = _)),
                                  "data-testid": "detail-world",
                                  required: "",
                                },
                                [
                                  h(Oe).some((_) => _.id === i.value.worldId)
                                    ? J("", !0)
                                    : (c(),
                                      v(
                                        "option",
                                        {
                                          key: 0,
                                          value: i.value.worldId,
                                          disabled: "",
                                        },
                                        "原世界已移除，请重新选择",
                                        8,
                                        ug,
                                      )),
                                  (c(!0),
                                  v(
                                    I,
                                    null,
                                    te(
                                      h(Oe),
                                      (_) => (
                                        c(),
                                        v(
                                          "option",
                                          { key: _.id, value: _.id },
                                          m(_.name),
                                          9,
                                          dg,
                                        )
                                      ),
                                    ),
                                    128,
                                  )),
                                ],
                                512,
                              ),
                              [[pt, i.value.worldId]],
                            ),
                          ]),
                        ],
                        8,
                        ag,
                      ),
                      s.value ? (c(), v("p", cg, m(s.value), 1)) : J("", !0),
                      ae(St, { labels: ["角色详情"] }),
                      n("footer", fg, [
                        n(
                          "button",
                          {
                            type: "button",
                            class: "button",
                            "data-testid": "detail-cancel",
                            disabled: h(l),
                            onClick: u,
                          },
                          "取消",
                          8,
                          pg,
                        ),
                        a[14] ||
                          (a[14] = n("span", { class: "spacer" }, null, -1)),
                        n(
                          "button",
                          {
                            class: "button primary",
                            "data-testid": "detail-save",
                            disabled: h(l),
                          },
                          m(h(l) ? "保存中…" : "保存"),
                          9,
                          mg,
                        ),
                      ]),
                    ],
                    32,
                  ))
                : (c(),
                  v("div", vg, [
                    n("section", null, [
                      a[15] || (a[15] = n("h3", null, "简介", -1)),
                      n("p", hg, m(h(U).bio), 1),
                    ]),
                    n("section", null, [
                      a[18] || (a[18] = n("h3", null, "人格", -1)),
                      n("div", gg, [
                        (c(!0),
                        v(
                          I,
                          null,
                          te(
                            h(U).traits,
                            (_) => (
                              c(),
                              v("span", { key: _, class: "pill" }, m(_), 1)
                            ),
                          ),
                          128,
                        )),
                      ]),
                      n("dl", null, [
                        n("div", null, [
                          a[16] || (a[16] = n("dt", null, "说话方式", -1)),
                          n("dd", null, m(h(U).speakingStyle), 1),
                        ]),
                        n("div", null, [
                          a[17] || (a[17] = n("dt", null, "核心价值观", -1)),
                          n("dd", null, m(h(U).coreValues), 1),
                        ]),
                      ]),
                    ]),
                    n("section", null, [
                      a[19] || (a[19] = n("h3", null, "头像", -1)),
                      n("div", yg, [
                        ae(
                          ss,
                          {
                            name: h(U).name,
                            initial: h(U).initial,
                            color: h(U).color,
                            "avatar-mode": h(U).avatarMode,
                          },
                          null,
                          8,
                          ["name", "initial", "color", "avatar-mode"],
                        ),
                        n(
                          "small",
                          null,
                          m(
                            h(U).avatarMode === "image"
                              ? "图片头像 · 本地占位"
                              : "文字头像 · " + h(U).initial,
                          ),
                          1,
                        ),
                      ]),
                    ]),
                    n("section", null, [
                      a[20] || (a[20] = n("h3", null, "所在世界", -1)),
                      n(
                        "p",
                        bg,
                        m(
                          ((p = h(Oe).find((_) => _.id === h(U).worldId)) ==
                          null
                            ? void 0
                            : p.name) || "原世界已移除，可编辑后重新选择",
                        ),
                        1,
                      ),
                      n("small", null, "居住地点 · " + m(h(U).homeLocation), 1),
                    ]),
                  ])),
            ]),
            n("p", kg, [
              a[21] || (a[21] = n("small", null, "更多：", -1)),
              n(
                "button",
                {
                  class: "text-button",
                  onClick:
                    a[8] ||
                    (a[8] = (_) =>
                      h(el)(
                        "能力申请：之后可以在这里查看与确认，目前为演示占位。",
                      )),
                },
                "能力申请",
              ),
              a[22] || (a[22] = n("span", null, "·", -1)),
              n(
                "button",
                {
                  class: "text-button",
                  onClick:
                    a[9] ||
                    (a[9] = (_) =>
                      h(el)(
                        "发展中的自我：记录相处中慢慢发生的变化，目前为演示占位。",
                      )),
                },
                "发展中的自我",
              ),
            ]),
          ])
        );
      };
    },
  }),
  wg = { class: "shell" },
  $g = { class: "topbar" },
  Cg = { class: "brand-mark" },
  xg = ["aria-expanded"],
  Sg = { key: 1, class: "avatar small" },
  Tg = { class: "failure-toggle" },
  Mg = {
    key: 0,
    class: "switch-menu scroll-lock-list",
    "data-scroll-lock": "",
  },
  Ig = ["onClick"],
  Eg = { class: "layout" },
  Ag = { class: "sidebar" },
  Vg = { class: "workspace-label" },
  Og = ["aria-current"],
  Ug = ["aria-expanded"],
  Pg = { key: 0, class: "life-nav" },
  Lg = ["data-testid", "aria-current", "onClick"],
  Ng = ["data-testid", "aria-current", "onClick"],
  Rg = ["data-testid", "aria-current", "onClick"],
  Fg = { class: "page-heading" },
  jg = { class: "breadcrumb" },
  Dg = { key: 0, class: "heading-caption" },
  Hg = { key: 1, class: "page-content" },
  Bg = { key: 0, "data-testid": "toast", class: "toast", role: "status" },
  Kg = je({
    __name: "App",
    setup(e) {
      const t = ["此刻", "角色详情", "日程", "目标", "衣柜", "日记", "记忆"],
        s = [
          ["世界", kd],
          ["动态", Tn],
          ["人物关系", Ad],
          ["媒体", _d],
        ],
        l = [
          ["通知与待处理", wd],
          ["角色库", $d],
          ["创作中心", Vd],
          ["管理中心", Id],
        ],
        o = [
          "聊天",
          ...t,
          ...s.map((P) => P[0]),
          ...l.map((P) => P[0]),
          "创建角色",
        ];
      function i() {
        try {
          const [P, V = ""] = decodeURIComponent(location.hash.slice(1)).split(
            "?",
          );
          return (
            P === "角色库" &&
              new URLSearchParams(V).get("empty") === "1" &&
              Cr(),
            o.includes(P) ? P : "聊天"
          );
        } catch {
          return "聊天";
        }
      }
      const r = z(i()),
        u = z(!0),
        d = z(!1),
        f = z(!0),
        a = z(),
        p = ye(
          () =>
            r.value === "聊天" ||
            t.includes(r.value) ||
            s.some(([P]) => P === r.value),
        ),
        _ = ye(() => !Ae.value.length && p.value);
      async function b() {
        if ((await Qt(), r.value !== "管理中心")) return;
        const P = location.hash.split("?")[1] || "",
          V = new URLSearchParams(P).get("section");
        V &&
          requestAnimationFrame(() => {
            var le;
            return (le = document.getElementById("config-" + V)) == null
              ? void 0
              : le.scrollIntoView({ block: "start" });
          });
      }
      (window.addEventListener("hashchange", () => {
        ((r.value = i()), b());
      }),
        Wl(b),
        vt(r, () => {
          (Bd(),
            (d.value = !1),
            Qt(() => {
              var P;
              return (P = a.value) == null ? void 0 : P.scrollTo({ top: 0 });
            }));
        }));
      function A(P) {
        ((d.value = !1),
          (r.value = P),
          (location.hash = encodeURIComponent(P)));
      }
      return (P, V) => {
        var le, ne, ce;
        return (
          c(),
          v("div", wg, [
            n("header", $g, [
              n(
                "button",
                { class: "brand", onClick: V[0] || (V[0] = (w) => A("聊天")) },
                [
                  n("span", Cg, [ae(h(Gn), { size: 19 })]),
                  V[10] || (V[10] = K("摇光", -1)),
                  V[11] ||
                    (V[11] = n(
                      "span",
                      { class: "brand-en" },
                      "FLUCTLIGHT",
                      -1,
                    )),
                ],
              ),
              V[14] || (V[14] = n("span", { class: "top-divider" }, null, -1)),
              n(
                "button",
                {
                  class: "character-switch",
                  "aria-expanded": d.value,
                  "aria-label": "切换角色",
                  onClick: V[1] || (V[1] = (w) => (d.value = !d.value)),
                },
                [
                  h(U)
                    ? (c(),
                      Se(
                        ss,
                        {
                          key: 0,
                          name: h(U).name,
                          initial: h(U).initial,
                          color: h(U).color,
                          "avatar-mode": h(U).avatarMode,
                          small: "",
                        },
                        null,
                        8,
                        ["name", "initial", "color", "avatar-mode"],
                      ))
                    : (c(), v("span", Sg, "·")),
                  n(
                    "b",
                    null,
                    m(
                      ((le = h(U)) == null ? void 0 : le.name) ||
                        "等待初次相识",
                    ),
                    1,
                  ),
                  ae(h(Ko), { size: 14 }),
                ],
                8,
                xg,
              ),
              V[15] ||
                (V[15] = n("span", { class: "scope" }, "角色工作区", -1)),
              V[16] || (V[16] = n("span", { class: "spacer" }, null, -1)),
              V[17] ||
                (V[17] = n(
                  "span",
                  { class: "local" },
                  [n("i"), K("相伴此刻")],
                  -1,
                )),
              n("label", Tg, [
                re(
                  n(
                    "input",
                    {
                      "data-testid": "simulate-failure",
                      "onUpdate:modelValue":
                        V[2] ||
                        (V[2] = (w) => (Fe(Yt) ? (Yt.value = w) : null)),
                      type: "checkbox",
                    },
                    null,
                    512,
                  ),
                  [[Ft, h(Yt)]],
                ),
                V[12] || (V[12] = K("模拟失败", -1)),
              ]),
              V[18] || (V[18] = n("span", { class: "self-avatar" }, "我", -1)),
              d.value
                ? (c(),
                  v("div", Mg, [
                    h(Ae).length
                      ? (c(!0),
                        v(
                          I,
                          { key: 0 },
                          te(
                            h(Ae),
                            (w) => (
                              c(),
                              v(
                                "button",
                                {
                                  key: w.id,
                                  onClick: (O) => {
                                    ((lt.value = w.id), (d.value = !1));
                                  },
                                },
                                [
                                  ae(
                                    ss,
                                    {
                                      name: w.name,
                                      initial: w.initial,
                                      color: w.color,
                                      "avatar-mode": w.avatarMode,
                                      small: "",
                                    },
                                    null,
                                    8,
                                    ["name", "initial", "color", "avatar-mode"],
                                  ),
                                  K(m(w.name), 1),
                                ],
                                8,
                                Ig,
                              )
                            ),
                          ),
                          128,
                        ))
                      : (c(),
                        v(
                          I,
                          { key: 1 },
                          [
                            V[13] ||
                              (V[13] = n(
                                "p",
                                { class: "muted switch-empty" },
                                "还没有一起生活的人",
                                -1,
                              )),
                            n(
                              "button",
                              {
                                onClick:
                                  V[3] ||
                                  (V[3] = (w) => {
                                    ((d.value = !1), h(Ps)());
                                  }),
                              },
                              "创建第一个角色",
                            ),
                            n(
                              "button",
                              { onClick: V[4] || (V[4] = (w) => A("角色库")) },
                              "前往角色库",
                            ),
                          ],
                          64,
                        )),
                  ]))
                : J("", !0),
            ]),
            n("div", Eg, [
              n("aside", Ag, [
                n("div", Vg, [
                  V[19] || (V[19] = K("角色工作区 ", -1)),
                  n(
                    "span",
                    null,
                    m(((ne = h(U)) == null ? void 0 : ne.name) || "初次使用"),
                    1,
                  ),
                ]),
                (c(),
                v("nav", { key: "workspace-" + r.value }, [
                  n(
                    "button",
                    {
                      "data-testid": "nav-聊天",
                      "aria-current": r.value === "聊天" ? "page" : void 0,
                      class: ke(["nav-item", { active: r.value === "聊天" }]),
                      onClick: V[5] || (V[5] = (w) => A("聊天")),
                    },
                    [
                      ae(h(Cd), { size: 17 }),
                      V[20] || (V[20] = n("span", null, "聊天", -1)),
                    ],
                    10,
                    Og,
                  ),
                  n(
                    "button",
                    {
                      class: "nav-item",
                      onClick: V[6] || (V[6] = (w) => (u.value = !u.value)),
                      "aria-expanded": u.value,
                    },
                    [
                      ae(h(Tn), { size: 17 }),
                      V[21] || (V[21] = n("span", null, "生活", -1)),
                      ae(h(Ko), { size: 13 }),
                    ],
                    8,
                    Ug,
                  ),
                  u.value
                    ? (c(),
                      v("div", Pg, [
                        (c(),
                        v(
                          I,
                          null,
                          te(t, (w) =>
                            n(
                              "button",
                              {
                                key: w,
                                "data-testid": "nav-" + w,
                                "aria-current": r.value === w ? "page" : void 0,
                                class: ke([
                                  "nav-item",
                                  { active: r.value === w },
                                ]),
                                onClick: (O) => A(w),
                              },
                              m(w),
                              11,
                              Lg,
                            ),
                          ),
                          64,
                        )),
                      ]))
                    : J("", !0),
                  (c(),
                  v(
                    I,
                    null,
                    te(s, ([w, O]) =>
                      n(
                        "button",
                        {
                          key: w,
                          "data-testid": "nav-" + w,
                          "aria-current": r.value === w ? "page" : void 0,
                          class: ke(["nav-item", { active: r.value === w }]),
                          onClick: (G) => A(w),
                        },
                        [
                          (c(), Se(hs(O), { size: 17 })),
                          n("span", null, m(w), 1),
                        ],
                        10,
                        Ng,
                      ),
                    ),
                    64,
                  )),
                ])),
                V[22] || (V[22] = n("hr", { class: "nav-divider" }, null, -1)),
                (c(),
                v("nav", { key: "global-" + r.value }, [
                  (c(),
                  v(
                    I,
                    null,
                    te(l, ([w, O]) =>
                      n(
                        "button",
                        {
                          key: w,
                          "data-testid": "nav-" + w,
                          "aria-current": r.value === w ? "page" : void 0,
                          class: ke(["nav-item", { active: r.value === w }]),
                          onClick: (G) => A(w),
                        },
                        [
                          (c(), Se(hs(O), { size: 17 })),
                          n("span", null, m(w), 1),
                        ],
                        10,
                        Rg,
                      ),
                    ),
                    64,
                  )),
                ])),
                V[23] ||
                  (V[23] = n(
                    "div",
                    { class: "sidebar-footer" },
                    [
                      K("每一个世界，都有回响 "),
                      n("small", null, "Fluctlight · v6.2"),
                    ],
                    -1,
                  )),
              ]),
              n(
                "main",
                {
                  ref_key: "main",
                  ref: a,
                  class: ke({ "chat-main": r.value === "聊天" && !_.value }),
                },
                [
                  n("div", Fg, [
                    n("div", null, [
                      n("div", jg, [
                        K(
                          m(h(U) ? h(U).name + " 的工作区" : "初次相识") + " ",
                          1,
                        ),
                        ae(h(Jn), { size: 12 }),
                        K(m(_.value ? "角色库" : r.value), 1),
                      ]),
                      n("h1", null, [
                        K(m(_.value ? "角色库" : r.value), 1),
                        r.value === "聊天" && !_.value
                          ? (c(), v("span", Dg, "日常的片刻，与你分享。"))
                          : J("", !0),
                      ]),
                    ]),
                    r.value === "聊天" && !_.value
                      ? (c(),
                        v(
                          "button",
                          {
                            key: 0,
                            class: "button",
                            onClick:
                              V[7] || (V[7] = (w) => (f.value = !f.value)),
                          },
                          [
                            (c(),
                            Se(hs(f.value ? h(xd) : h(Sd)), { size: 15 })),
                            K(m(f.value ? "收起预览" : "展开预览"), 1),
                          ],
                        ))
                      : J("", !0),
                    r.value === "管理中心" && h(ts) && h(Ke) !== "complete"
                      ? (c(),
                        v(
                          "button",
                          {
                            key: 1,
                            class: "text-button",
                            onClick: V[8] || (V[8] = (w) => A("创建角色")),
                          },
                          "返回角色准备 ↗",
                        ))
                      : J("", !0),
                  ]),
                  r.value === "聊天" && h(U)
                    ? (c(),
                      Se(
                        wc,
                        { key: h(U).id, preview: f.value, onGo: A },
                        null,
                        8,
                        ["preview"],
                      ))
                    : (c(),
                      v("div", Hg, [
                        r.value === "角色库" || _.value
                          ? (c(), Se(oh, { key: 0 }))
                          : r.value === "创建角色"
                            ? (c(), Se(lg, { key: 1 }))
                            : r.value === "角色详情" && h(U)
                              ? (c(), Se(_g, { key: "detail" + h(U).id }))
                              : r.value === "衣柜" && h(U)
                                ? (c(), Se(Zp, { key: "w" + h(U).id }))
                                : r.value === "世界" && h(U)
                                  ? (c(), Se(Tm, { key: "world" + h(U).id }))
                                  : t.includes(r.value) && h(U)
                                    ? (c(),
                                      Se(
                                        Xf,
                                        {
                                          page: r.value,
                                          key: h(U).id,
                                          onGo: A,
                                        },
                                        null,
                                        8,
                                        ["page"],
                                      ))
                                    : r.value === "管理中心"
                                      ? (c(), Se(Kv, { key: 6 }))
                                      : (c(),
                                        Se(
                                          p1,
                                          {
                                            page: r.value,
                                            key:
                                              r.value +
                                              (((ce = h(U)) == null
                                                ? void 0
                                                : ce.id) ?? "empty"),
                                          },
                                          null,
                                          8,
                                          ["page"],
                                        )),
                      ])),
                ],
                2,
              ),
            ]),
            h(Hs)
              ? (c(),
                v("div", Bg, [
                  K(m(h(Hs).text) + " ", 1),
                  h(Hs).link
                    ? (c(),
                      v(
                        "a",
                        {
                          key: 0,
                          href: "#衣柜",
                          onClick:
                            V[9] ||
                            (V[9] = (w) => {
                              (A("衣柜"), (Yn.value = "方案库"));
                            }),
                        },
                        "去方案库 ↗",
                      ))
                    : J("", !0),
                ]))
              : J("", !0),
          ])
        );
      };
    },
  });
Kg.__approvedFixture='approved-demo-fixture';
export const approvedReferenceData={characters:Ae.value,worlds:Oe.value,life:ml.value,media:ei.value};
export default Kg;
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
    const l = getComputedStyle(s),
      o =
        !!s.closest("details[open], .switch-menu") &&
        s.getClientRects().length > 0 &&
        l.visibility === "visible" &&
        s.clientHeight > 0 &&
        s.scrollHeight > s.clientHeight &&
        ["auto", "scroll"].includes(l.overflowY);
    if (((s.style.overscrollBehavior = o ? "contain" : "auto"), !o)) return;
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
