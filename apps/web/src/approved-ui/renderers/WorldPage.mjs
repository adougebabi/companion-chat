import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, Fragment as O, createElementVNode as o, toDisplayString as v, withDirectives as pe, vModelText as $e, unref as M, createCommentVNode as X, renderList as oe, normalizeClass as ge, createBlock as Ae, withCtx as Ht, withModifiers as Ve, createVNode as me, createTextVNode as Z } from 'vue';
const $p = {
  class: "row"
};
const Cp = {
  class: "card world-header"
};
const xp = {
  class: "row"
};
const Sp = {
  class: "toolbar"
};
const Tp = ["disabled"];
const Mp = {
  key: 0,
  class: "card empty-state",
  "data-testid": "world-empty-sites"
};
const Ep = {
  class: "row"
};
const Ap = ["onClick"];
const Ip = {
  key: 0,
  class: "muted"
};
const Op = {
  class: "place-grid"
};
const Vp = {
  class: "row"
};
const Pp = ["onClick"];
const vp = {
  class: "world-list"
};
const hp = {
  class: "row"
};
const gp = {
  key: 0,
  class: "pill"
};
const yp = {
  class: "muted"
};
const bp = {
  class: "row"
};
const _p = ["onClick"];
const kp = ["onClick"];
const wp = ["onClick"];
const Lp = {
  key: 0,
  class: "inline-error"
};
const Up = ["onUpdate:modelValue"];
const Np = ["disabled"];
const Fp = {
  key: 0,
  class: "inline-error"
};
const Rp = ["disabled"];
const Dp = {
  key: 0
};
const jp = ["disabled"];
const Hp = {
  key: 5,
  class: "modal-backdrop"
};
const Bp = {
  class: "card confirm-dialog"
};
function renderWorldPageBody(__view) {
  return (S, I) => {
    var K, le;
    return p(), g(O, null, [__view.s.value ? (p(), g(O, {
      key: 1
    }, [o("div", $p, [approvedButtonVNode({
      class: "text-button",
      onClick: I[0] || (I[0] = H => __view.t.value = null)
    }, "世界列表"), o("span", null, " / " + v(__view.s.value.name), 1), I[16] || (I[16] = o("span", {
      class: "spacer"
    }, null, -1)), approvedButtonVNode({
      class: "button",
      onClick: I[1] || (I[1] = H => __view.u.value = !0)
    }, "新增场地分类")]), approvedSurfaceVNode("div", Cp, [o("div", xp, [I[17] || (I[17] = o("span", {
      class: "eyebrow"
    }, "A WORLD OF YOUR OWN", -1)), I[18] || (I[18] = o("span", {
      class: "spacer"
    }, null, -1)), approvedButtonVNode({
      class: "button",
      onClick: I[2] || (I[2] = H => __view.c.value = {
        id: __view.s.value.id,
        name: __view.s.value.name,
        genre: __view.s.value.genre,
        _source: __view.s.value._source
      })
    }, "编辑世界信息"), approvedButtonVNode({
      class: "button",
      onClick: I[3] || (I[3] = H => __view.n.value = !__view.n.value)
    }, v(__view.n.value ? "结束编辑" : "编辑地点"))]), o("h2", null, v(__view.s.value.name), 1), o("p", null, v(__view.s.value.genre), 1)]), o("div", Sp, [pe(o("input", {
      "onUpdate:modelValue": I[4] || (I[4] = H => __view.l.value = H),
      placeholder: "补充扩展方向，例如：河岸书店"
    }, null, 512), [[$e, __view.l.value]]), approvedButtonVNode({
      class: "button",
      disabled: M(__view.f),
      onClick: __view.te
    }, v(M(__view.f) && __view.C.value === "expand" ? "处理中…" : "AI 扩展世界"))]), __view.s.value.sites.length ? X("", !0) : (p(), approvedSurfaceBlock("div", Mp, [...(I[19] || (I[19] = [o("p", null, "暂无场地", -1)]))])), (p(!0), g(O, null, oe(__view.s.value.sites, (H, q) => (p(), approvedSurfaceBlock("section", {
      key: q,
      class: "card compact"
    }, [o("div", Ep, [o("h3", null, "场地 · " + v(H.name), 1), I[20] || (I[20] = o("span", {
      class: "spacer"
    }, null, -1)), __view.n.value ? (p(), approvedButtonBlock({
      key: 0,
      class: "text-button",
      onClick: V => {
        __view.r.value = q, __view.i.value = {
          id: Date.now(),
          name: "",
          alias: "",
          floor: "",
          description: "",
          atmosphere: "",
          furniture: "",
          layout: "",
          custom: !0
        };
      }
    }, "添加地点")) : X("", !0)]), H.places.length ? X("", !0) : (p(), g("p", Ip, "暂无地点")), o("div", Op, [(p(!0), g(O, null, oe(H.places, V => (p(), g("article", {
      key: V.id,
      class: "place"
    }, [o("div", Vp, [o("b", null, v(V.alias || V.name), 1), V.floor ? o("small", null, v(V.floor), 1) : X("", !0)]), V.description || V.atmosphere ? o("p", null, [V.description,V.atmosphere].filter(Boolean).join(" · "), 1) : X("", !0), V.furniture ? o("small", null, "固定陈设：" + v(V.furniture), 1) : X("", !0), V.layout ? o("small", null, "布局：" + v(V.layout), 1) : X("", !0), __view.n.value ? (p(), approvedButtonBlock({
      key: 0,
      class: "button edit-place",
      onClick: U => __view.se(V, q)
    }, "编辑地点")) : X("", !0)]))), 128))])]))), 128))], 64)) : (p(), g(O, {
      key: 0
    }, [o("div", {
      class: "row"
    }, [I[14] || (I[14] = o("div", null, [o("h2", null, "世界列表"), o("p", {
      class: "muted"
    }, "每个人，都在自己的风景里生活。")], -1)), I[15] || (I[15] = o("span", {
      class: "spacer"
    }, null, -1)), approvedButtonVNode({
      class: "button",
      "data-testid": "world-add",
      onClick: __view.w
    }, "添加世界")]), o("div", vp, [(p(!0), g(O, null, oe(M(__view.worlds), H => (p(), approvedSurfaceBlock("article", {
      key: H.id,
      class: ge(["card", {
        current: H.residents.includes(M(__view.character).id)
      }])
    }, [o("div", hp, [o("h2", null, v(H.name), 1), H.residents.includes(M(__view.character).id) ? (p(), g("span", gp, "当前")) : X("", !0)]), o("p", null, (H.genre?v(H.genre)+" · ":"") + v(H.sites.length) + " 处场地", 1), o("p", yp, "居住在这里：" + v(H.residents.map(q => {
      var V;
      return (V = M(__view.characters).find(c=>c.id===q)) == null ? void 0 : V.name;
    }).join("、") || "暂无角色"), 1), o("div", bp, [approvedButtonVNode({
      class: "button primary",
      onClick: q => {
        __view.t.value = H.id, __view.n.value = !1;
      }
    }, "进入"), o("details", {class:"actions-menu",onClick:event=>event.target.closest("button")&&event.currentTarget.removeAttribute("open")}, [o("summary", {"aria-label":H.name+"的世界操作"}, "⋯"), o("div", null, [approvedButtonVNode({
      class: "text-button",
      onClick: q => {
        __view.t.value = H.id, __view.n.value = !0;
      }
    }, "编辑"), approvedButtonVNode({
      class: "text-button",
      onClick: q => __view.d.value = H.id
    }, "删除")])])])]))), 128))])], 64)), __view.i.value ? (p(), Ae(__view.DetailDrawer, {
      key: 2,
      title: (K = __view.s.value) != null && K.sites.some(H => H.places.some(q => {
        var V;
        return q.id === ((V = __view.i.value) == null ? void 0 : V.id);
      })) ? "编辑地点" : "添加地点",
      "test-id": "place-drawer",
      onClose: I[6] || (I[6] = H => __view.i.value = null)
    }, {
      default: Ht(() => [o("form", {
        class: "form-grid",
        onSubmit: Ve(__view.ne, ["prevent"])
      }, [me(__view.ActionFeedback, {
        labels: ["世界"]
      }), o("label", null, [I[21] || (I[21] = Z("地点名称", -1)), pe(o("input", {
        "onUpdate:modelValue": I[5] || (I[5] = H => __view.i.value.name = H),readonly:!!__view.i.value._source, title:__view.i.value._source?"正式名称固定，可修改别名":undefined
      }, null, 520,["readonly","title"]), [[$e, __view.i.value.name]]),__view.i.value._source?o("small",{class:"muted"},"正式名称固定，可修改别名"):X("",!0), __view.i.value.name.trim() ? X("", !0) : (p(), g("small", Lp, "请填写地点名称。"))]), (p(), g(O, null, oe(__view.A, ([H, q]) => o("label", {
        key: H
      }, [Z(v(q), 1), pe(o("input", {
        "onUpdate:modelValue": V => __view.i.value[H] = V
      }, null, 8, Up), [[$e, __view.i.value[H]]])])), 64)), approvedButtonVNode({
        class: "button primary",
        disabled: M(__view.f) || !__view.i.value.name.trim()
      }, v(M(__view.f) && __view.C.value === "place" ? "处理中…" : "保存地点"))], 32)]),
      _: 1
    }, 8, ["title"])) : X("", !0), __view.c.value ? (p(), Ae(__view.DetailDrawer, {
      key: 3,
      title: M(__view.worlds).some(H => {
        var q;
        return H.id === ((q = __view.c.value) == null ? void 0 : q.id);
      }) ? "编辑世界" : "添加世界",
      "test-id": "world-drawer",
      onClose: I[9] || (I[9] = H => __view.c.value = null)
    }, {
      default: Ht(() => [o("form", {
        class: "form-grid",
        onSubmit: Ve(__view.B, ["prevent"])
      }, [me(__view.ActionFeedback, {
        labels: ["世界"]
      }), o("label", null, [I[22] || (I[22] = Z("世界名称", -1)), pe(o("input", {
        "onUpdate:modelValue": I[7] || (I[7] = H => __view.c.value.name = H),
        "data-testid": "world-name"
      }, null, 512), [[$e, __view.c.value.name]]), __view.c.value.name.trim() ? X("", !0) : (p(), g("small", Fp, "请填写世界名称。"))]), o("label", null, [I[23] || (I[23] = Z("世界类型", -1)), pe(o("input", {
        "onUpdate:modelValue": I[8] || (I[8] = H => __view.c.value.genre = H),
        "data-testid": "world-genre"
      }, null, 512), [[$e, __view.c.value.genre]])]), approvedButtonVNode({
        class: "button primary",
        disabled: M(__view.f) || !__view.c.value.name.trim(),
        "data-testid": "world-save"
      }, v(M(__view.f) && __view.C.value === "world" ? "处理中…" : "保存世界"))], 32)]),
      _: 1
    }, 8, ["title"])) : X("", !0), __view.u.value ? (p(), Ae(__view.DetailDrawer, {
      key: 4,
      title: "添加场地分类",
      "test-id": "site-drawer",
      onClose: I[11] || (I[11] = H => __view.u.value = !1)
    }, {
      default: Ht(() => [o("form", {
        class: "form-grid",
        onSubmit: Ve(__view.ue, ["prevent"])
      }, [o("p", {class:"muted"}, "分类用于组织地点，例如商店、公共设施。保存后可在分类中添加服装店等具体地点。"), me(__view.ActionFeedback, {
        labels: ["世界"]
      }), o("label", null, [I[24] || (I[24] = Z("分类名称", -1)), pe(o("input", {
        "onUpdate:modelValue": I[10] || (I[10] = H => __view.a.value = H)
      }, null, 512), [[$e, __view.a.value]]), __view.a.value.trim() ? X("", !0) : (p(), g("small", Dp, "请填写场地名称。"))]), approvedButtonVNode({
        class: "button primary",
        disabled: M(__view.f) || !__view.a.value.trim()
      }, v(M(__view.f) && __view.C.value === "site" ? "处理中…" : "保存场地"))], 32)]),
      _: 1
    })) : X("", !0), __view.d.value !== null ? (p(), g("div", Hp, [approvedSurfaceVNode("section", Bp, [I[25] || (I[25] = o("h2", null, "删除世界？", -1)), o("p", null, "「" + v((le = M(__view.worlds).find(H => H.id === __view.d.value)) == null ? void 0 : le.name) + "」及其中的场地将被移除。", 1), approvedButtonVNode({
      class: "button",
      onClick: I[12] || (I[12] = H => __view.d.value = null)
    }, "取消"), approvedButtonVNode({
      class: "button primary",
      onClick: () => __view.removeWorld(__view.d.value)
    }, "确认删除")])])) : X("", !0), !__view.i.value && !__view.c.value && !__view.u.value ? (p(), Ae(__view.ActionFeedback, {
      key: 6,
      labels: ["世界"]
    })) : X("", !0)], 64);
  };
}

export const renderWorldPage = unoptimizedRenderer(renderWorldPageBody);
