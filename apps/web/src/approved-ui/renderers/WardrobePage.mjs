import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, Fragment as O, createElementVNode as o, renderList as oe, normalizeClass as ge, unref as M, toDisplayString as v, createTextVNode as Z, withDirectives as pe, vModelSelect as Et, createCommentVNode as X, withModifiers as Ve, vModelText as $e, createVNode as me, vModelCheckbox as ss, createBlock as Ae } from 'vue';
const _f = {
  class: "tabs"
};
const kf = ["onClick"];
const wf = {
  class: "toolbar"
};
const $f = {
  class: "card slot-picker scroll-lock-list",
  "data-scroll-lock": ""
};
const Cf = {
  key: 0,
  class: "muted",
  role: "status"
};
const At = ["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"];
const xf = {
  class: "slot-accordion"
};
const Sf = {
  class: "slot-row scroll-lock-list",
  "data-scroll-lock": ""
};
const Tf = ["disabled", "title", "onClick"];
const Mf = {
  key: 0
};
const Ef = {
  class: "card"
};
const Af = {
  class: "slot-summary"
};
const If = {
  key: 0,
  class: "inline-error"
};
const Of = {
  key: 1,
  class: "muted"
};
const Vf = ["disabled"];
const Pf = {
  class: "card"
};
const Lf = {
  class: "tag-list"
};
const Uf = {
  class: "pill"
};
const Nf = ["onClick"];
const Ff = ["onClick"];
const Rf = ["placeholder"];
const Df = ["disabled"];
const jf = {
  class: "row"
};
const Bf = ["disabled"];
const Hf = ["onSubmit"];
const Kf = {
  key: 1
};
const Wf = {
  class: "pill"
};
const qf = {
  class: "scroll-lock-list",
  "data-scroll-lock": ""
};
const zf = ["onClick"];
const Gf = ["onClick"];
const Jf = ["onClick"];
const Yf = ["onClick"];
const Zf = ["disabled", "onClick"];
const Qf = {
  key: 0,
  class: "slot-summary"
};
const Xf = {
  key: 0,
  class: "muted"
};
const ep = {
  class: "toolbar"
};
const tp = {
  class: "garment-grid"
};
const sp = {
  class: "row"
};
const np = {
  class: "scroll-lock-list",
  "data-scroll-lock": ""
};
const lp = ["onClick"];
const op = ["onClick"];
const ip = {
  key: 0,
  class: "muted"
};
const rp = {
  key: 0,
  class: "inline-error"
};
const ap = {
  class: "row"
};
const up = ["value"];
const cp = {
  class: "row"
};
const dp = ["disabled"];
const pp = {
  class: "row"
};
const fp = ["aria-label"];
function renderWardrobePageBody(__view) {
  return (j, T) => (p(), g(O, null, [o("div", _f, [(p(), g(O, null, oe(["为她搭配", "方案库", "全部衣服"], x => approvedButtonVNode({
    class: ge({
      active: M(__view.i) === x
    }),
    onClick: R => __view.i.value = x
  }, v(x === "为她搭配" && M(__view.character).id === 1 ? "为他搭配" : x))), 64))]), M(__view.i) === "为她搭配" ? (p(), g(O, {
    key: 0
  }, [o("div", wf, [o("label", null, [T[20] || (T[20] = Z("先选标签 ", -1)), pe(o("select", {
    "onUpdate:modelValue": T[0] || (T[0] = x => __view.r.value = x)
  }, [T[19] || (T[19] = o("option", {
    value: ""
  }, "请选择", -1)), (p(!0), g(O, null, oe(M(__view.character).outfitTags, x => (p(), g("option", null, v(x), 1))), 256))], 512), [[Et, __view.r.value]])]), T[21] || (T[21] = o("small", null, "标签可在方案库维护 · 每个部位选一件，配饰可多选", -1))]), approvedSurfaceVNode("div", $f, [__view.r.value ? X("", !0) : (p(), g("p", Cf, "请先选择标签")), (p(!0), g(O, null, oe(M(At), x => (p(), g("details", xf, [o("summary", null, [Z(v(x) + " ", 1), o("small", null, v((__view.u.value[x] || []).map(R => {
    var ye;
    return (ye = M(__view.character).garments.find(F => F.id === R)) == null ? void 0 : ye.name;
  }).join("、") || "未选择"), 1)]), o("div", Sf, [(p(!0), g(O, null, oe(M(__view.character).garments.filter(R => R.type === x), R => {
    var ye;
    return p(), approvedButtonBlock({
      disabled: !__view.r.value,
      title: __view.r.value ? void 0 : "请先选择标签",
      class: ge(["choice", {
        chosen: (ye = __view.u.value[x]) == null ? void 0 : ye.includes(R.id)
      }]),
      onClick: F => __view.le(x, R.id, x === "配饰")
    }, v(R.name));
  }), 256)), M(__view.character).garments.some(R => R.type === x) ? X("", !0) : (p(), g("small", Mf, "暂无衣服"))])]))), 256))]), approvedSurfaceVNode("div", Ef, [T[22] || (T[22] = o("h3", null, "当前搭配", -1)), o("p", Af, v(__view.q({
    name: __view.a.value,
    tag: __view.r.value,
    items: __view.u.value
  })), 1)]), __view.l.value ? (p(), g("p", If, "方案名称已存在，请换一个名称。")) : X("", !0), __view.a.value.trim() ? X("", !0) : (p(), g("p", Of, "请填写方案名称后保存。")), o("form", {
    class: "input-row",
    onSubmit: Ve(__view.K, ["prevent"])
  }, [pe(o("input", {
    "onUpdate:modelValue": T[1] || (T[1] = x => __view.a.value = x),
    placeholder: "方案名称",
    "aria-label": "方案名称"
  }, null, 512), [[$e, __view.a.value]]), approvedButtonVNode({
    class: "button primary",
    disabled: M(__view.n) || __view.l.value || !__view.r.value || !__view.a.value.trim()
  }, v(M(__view.n) && __view.t.value === "save" ? "处理中…" : __view.f.value ? "保存修改" : "保存为方案")), approvedButtonVNode({
    type: "button",
    class: "button",
    onClick: T[2] || (T[2] = x => {
      __view.f.value = null, __view.u.value = {}, __view.a.value = "";
    })
  }, "重新搭配")], 32)], 64)) : M(__view.i) === "方案库" ? (p(), g(O, {
    key: 1
  }, [approvedSurfaceVNode("div", Pf, [T[23] || (T[23] = o("div", {
    class: "row"
  }, [o("b", null, "标签管理"), o("small", null, "删除标签保留方案")], -1)), o("div", Lf, [(p(!0), g(O, null, oe(M(__view.character).outfitTags, x => (p(), g("span", Uf, [Z(v(x) + " ", 1), approvedButtonVNode({
    onClick: R => {
      __view.C.value = x, __view.d.value = x;
    }
  }, "改名"), approvedButtonVNode({
    onClick: R => __view.he(x)
  }, "删除")]))), 256))]), o("form", {
    class: "input-row",
    onSubmit: Ve(__view.ee, ["prevent"])
  }, [pe(o("input", {
    "onUpdate:modelValue": T[3] || (T[3] = x => __view.d.value = x),
    placeholder: __view.C.value ? "新标签名" : "添加标签",
    "aria-label": "标签名称"
  }, null, 8, Rf), [[$e, __view.d.value]]), approvedButtonVNode({
    class: "button",
    disabled: !__view.d.value.trim()
  }, v(__view.C.value ? "保存标签" : "添加"))], 32)]), (p(!0), g(O, null, oe(M(__view.character).outfits, x => (p(), approvedSurfaceBlock("article", {
    key: x.id,
    class: "card compact"
  }, [o("div", jf, [__view.B.value === x.id ? (p(), g("form", {
    key: 0,
    class: "input-row",
    onSubmit: Ve(R => __view.ue(x), ["prevent"])
  }, [pe(o("input", {
    "onUpdate:modelValue": T[4] || (T[4] = R => __view.te.value = R),
    "aria-label": "方案新名称",
    autofocus: ""
  }, null, 512), [[$e, __view.te.value]]), approvedButtonVNode({
    class: "button",
    disabled: M(__view.n) || !__view.te.value.trim()
  }, v(M(__view.n) && __view.t.value === "rename" ? "处理中…" : "保存")), approvedButtonVNode({
    type: "button",
    class: "text-button",
    onClick: T[5] || (T[5] = R => __view.B.value = null)
  }, "取消")], 40, Hf)) : (p(), g("b", Kf, v(x.name), 1)), o("span", Wf, v(x.tag || "未分类"), 1), T[25] || (T[25] = o("span", {
    class: "spacer"
  }, null, -1)), o("details", {
    class: "actions-menu",
    onClick: T[6] || (T[6] = R => R.target.closest("button") && R.currentTarget.removeAttribute("open"))
  }, [T[24] || (T[24] = o("summary", {
    "aria-label": "方案操作"
  }, "⋯", -1)), o("div", qf, [approvedButtonVNode({
    onClick: R => __view.c.value = __view.c.value === x.id ? null : x.id
  }, "预览"), approvedButtonVNode({
    onClick: R => __view.H(x)
  }, "编辑搭配"), approvedButtonVNode({
    onClick: R => {
      __view.B.value = x.id, __view.te.value = x.name;
    }
  }, "重命名"), approvedButtonVNode({
    disabled:true, title:"服务端保留方案及其历史；可编辑或重命名"
  }, "删除")])]), approvedButtonVNode({
    class: "button",
    disabled: M(__view.n),
    onClick: R => __view.V(x)
  }, v(M(__view.n) && __view.t.value === "wear" && __view.s.value === x.id ? "处理中…" : "请" + M(__view.character).name + "换上"))]), __view.c.value === x.id ? (p(), g("p", Qf, v(__view.q(x)), 1)) : X("", !0)]))), 128)), M(__view.character).outfits.length ? X("", !0) : (p(), g("p", Xf, "还没有方案，去搭配一套吧。"))], 64)) : (p(), g(O, {
    key: 2
  }, [o("div", ep, [pe(o("select", {
    "onUpdate:modelValue": T[7] || (T[7] = x => __view.w.value = x),
    "aria-label": "按类型筛选"
  }, [T[26] || (T[26] = o("option", {
    value: ""
  }, "全部类型", -1)), (p(!0), g(O, null, oe(M(At), x => (p(), g("option", null, v(x), 1))), 256))], 512), [[Et, __view.w.value]]), pe(o("select", {
    "onUpdate:modelValue": T[8] || (T[8] = x => __view.A.value = x),
    "aria-label": "按季节筛选"
  }, [T[27] || (T[27] = o("option", {
    value: ""
  }, "全部季节", -1)), (p(), g(O, null, oe(["春", "夏", "秋", "冬"], x => o("option", null, v(x), 1)), 64))], 512), [[Et, __view.A.value]]), T[28] || (T[28] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    class: "button",
    onClick: T[9] || (T[9] = x => __view.S.value = {
      id: Date.now(),
      name: "",
      type: "上衣",
      seasons: ["秋"]
    })
  }, "添加衣服")]), o("div", tp, [(p(!0), g(O, null, oe(__view.I.value, x => (p(), approvedSurfaceBlock("article", {
    key: x.id,
    class: "card garment-card"
  }, [o("div", {
    class: ge(["garment-thumb", "garment-type-" + x.type])
  }, [me(__view.GarmentArt, {
    type: x.type
  }, null, 8, ["type"]), o("small", null, v(x.type), 1)], 2), o("div", sp, [o("div", null, [o("h3", null, v(x.name), 1), o("small", null, v(x.seasons.join(" / ")), 1)]), T[30] || (T[30] = o("span", {
    class: "spacer"
  }, null, -1)), o("details", {
    class: "actions-menu",
    onClick: T[10] || (T[10] = R => R.target.closest("button") && R.currentTarget.removeAttribute("open"))
  }, [T[29] || (T[29] = o("summary", {
    "aria-label": "衣服操作"
  }, "⋯", -1)), o("div", np, [approvedButtonVNode({
    onClick: R => __view.S.value = JSON.parse(JSON.stringify(x))
  }, "编辑"), approvedButtonVNode({
    onClick: R => __view._(x.id)
  }, "删除")])])])]))), 128)), __view.I.value.length ? X("", !0) : (p(), g("p", ip, "没有符合条件的衣服。"))])], 64)), __view.S.value ? (p(), g("div", {
    key: 3,
    class: "drawer-backdrop",
    onClick: T[15] || (T[15] = Ve(x => __view.S.value = null, ["self"]))
  }, [o("form", {
    "data-testid": "garment-drawer",
    class: "detail-drawer form-grid",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": "衣服",
    onSubmit: Ve(__view.U, ["prevent"])
  }, [me(__view.ActionFeedback, {
    labels: ["衣柜"]
  }), o("h2", null, v(M(__view.character).garments.some(x => {
    var R;
    return x.id === ((R = __view.S.value) == null ? void 0 : R.id);
  }) ? "编辑衣服" : "添加衣服"), 1), o("label", null, [T[31] || (T[31] = Z("名称", -1)), pe(o("input", {
    "onUpdate:modelValue": T[11] || (T[11] = x => __view.S.value.name = x)
  }, null, 512), [[$e, __view.S.value.name]]), __view.S.value.name.trim() ? X("", !0) : (p(), g("small", rp, "请填写衣服名称。"))]), o("label", null, [T[32] || (T[32] = Z("类型", -1)), pe(o("select", {
    "onUpdate:modelValue": T[12] || (T[12] = x => __view.S.value.type = x)
  }, [(p(!0), g(O, null, oe(M(At), x => (p(), g("option", null, v(x), 1))), 256))], 512), [[Et, __view.S.value.type]])]), o("div", ap, [(p(), g(O, null, oe(["春", "夏", "秋", "冬"], x => o("label", null, [pe(o("input", {
    "onUpdate:modelValue": T[13] || (T[13] = R => __view.S.value.seasons = R),
    type: "checkbox",
    value: x
  }, null, 8, up), [[ss, __view.S.value.seasons]]), Z(v(x), 1)])), 64))]), o("div", cp, [approvedButtonVNode({
    class: "button primary",
    disabled: M(__view.n) || !__view.S.value.name.trim()
  }, v(M(__view.n) && __view.t.value === "garment" ? "处理中…" : "保存衣服")), approvedButtonVNode({
    type: "button",
    class: "button",
    onClick: T[14] || (T[14] = x => __view.S.value = null)
  }, "取消")])], 32)])) : X("", !0), __view.se.value ? (p(), g("div", {
    key: 4,
    class: "modal-backdrop",
    onClick: T[18] || (T[18] = Ve(x => __view.se.value = null, ["self"]))
  }, [approvedSurfaceVNode("section", {
    class: "card confirm-dialog",
    role: "alertdialog",
    "aria-modal": "true",
    "aria-label": __view.se.value.title
  }, [o("h2", null, v(__view.se.value.title), 1), o("p", null, v(__view.se.value.message), 1), o("div", pp, [approvedButtonVNode({
    class: "button",
    onClick: T[16] || (T[16] = x => __view.se.value = null)
  }, "取消"), approvedButtonVNode({
    class: "button primary",
    onClick: T[17] || (T[17] = x => {
      __view.se.value.run(), __view.se.value = null;
    })
  }, "确认删除")])])])) : X("", !0), __view.S.value ? X("", !0) : (p(), Ae(__view.ActionFeedback, {
    key: 5,
    labels: ["衣柜", "换装"]
  }))], 64));
}

export const renderWardrobePage = unoptimizedRenderer(renderWardrobePageBody);
