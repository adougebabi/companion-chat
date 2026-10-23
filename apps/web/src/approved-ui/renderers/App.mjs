import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, createElementVNode as o, createVNode as me, unref as M, createTextVNode as Z, normalizeStyle as yt, toDisplayString as v, withDirectives as pe, isRef as Re, vModelCheckbox as ss, Fragment as O, renderList as oe, createCommentVNode as X, normalizeClass as ge, createBlock as Ae, resolveDynamicComponent as as } from 'vue';
import { Tl, wo, Ju, Co, Ji, Yu, Zu } from '../shared/icons.mjs';
const av = {
  class: "shell"
};
const uv = {
  class: "topbar"
};
const cv = {
  class: "brand-mark"
};
const dv = {
  class: "failure-toggle"
};
const fv = {
  key: 0,
  class: "switch-menu scroll-lock-list",
  "data-scroll-lock": ""
};
const pv = ["onClick"];
const mv = {
  class: "layout"
};
const vv = {
  class: "sidebar"
};
const hv = {
  class: "workspace-label"
};
const gv = ["aria-current"];
const yv = ["aria-expanded"];
const bv = {
  key: 0,
  class: "life-nav"
};
const _v = ["data-testid", "aria-current", "onClick"];
const kv = ["data-testid", "aria-current", "onClick"];
const wv = ["data-testid", "aria-current", "onClick"];
const $v = {
  class: "page-heading"
};
const Cv = {
  class: "breadcrumb"
};
const xv = {
  key: 0,
  class: "heading-caption"
};
const Sv = {
  key: 1,
  class: "page-content"
};
const Tv = {
  key: 0,
  "data-testid": "toast",
  class: "toast",
  role: "status"
};
function renderAppBody(__view) {
  return (C, w) => (p(), g("div", av, [o("header", uv, [approvedButtonVNode({
    class: "brand",
    onClick: w[0] || (w[0] = A => __view.d("聊天"))
  }, [o("span", cv, [me(M(Tl), {
    size: 19
  })]), w[7] || (w[7] = Z("摇光", -1)), w[8] || (w[8] = o("span", {
    class: "brand-en"
  }, "FLUCTLIGHT", -1))]), w[10] || (w[10] = o("span", {
    class: "top-divider"
  }, null, -1)), approvedButtonVNode({
    class: "character-switch",
    onClick: w[1] || (w[1] = A => __view.u.value = !__view.u.value)
  }, [me(__view.CharacterAvatar,{class:"small",name:M(__view.character).name,initial:M(__view.character).initial,color:M(__view.character).color,src:M(__view.character)._directory?.avatar},null,8,["name","initial","color","src"]), o("b", null, v(M(__view.character).name), 1), me(M(wo), {
    size: 14
  })]), w[11] || (w[11] = o("span", {
    class: "scope"
  }, "角色工作区", -1)), w[12] || (w[12] = o("span", {
    class: "spacer"
  }, null, -1)), w[13] || (w[13] = o("span", {
    class: "local"
  }, [o("i"), Z("相伴此刻")], -1)), __view.isDemo ? o("label", dv, [pe(o("input", {
    "data-testid": "simulate-failure",
    "onUpdate:modelValue": w[2] || (w[2] = A => Re(__view.simulateFailure) ? __view.simulateFailure.value = A : null),
    type: "checkbox"
  }, null, 512), [[ss, M(__view.simulateFailure)]]), w[9] || (w[9] = Z("模拟失败", -1))]) : null, w[14] || (w[14] = o("span", {
    class: "self-avatar"
  }, "我", -1)), __view.u.value ? (p(), g("div", fv, [(p(!0), g(O, null, oe(M(__view.characters), A => (p(), approvedButtonBlock({
    key: A.id,
    onClick: B => {
      __view.selectedId.value = A.id, __view.u.value = !1;
    }
  }, [o("span", {
    class: "avatar small",
    style: yt({
      background: A.color
    })
  }, v(A.initial), 5), Z(v(A.name), 1)]))), 128))])) : X("", !0)]), o("div", mv, [o("aside", vv, [o("div", hv, [w[15] || (w[15] = Z("角色工作区 ", -1)), o("span", null, v(M(__view.character).name), 1)]), (p(), g("nav", {
    key: "workspace-" + __view.r.value
  }, [approvedButtonVNode({
    "data-testid": "nav-聊天",
    disabled: !M(__view.character).id,
    "aria-current": __view.r.value === "聊天" ? "page" : void 0,
    class: ge(["nav-item", {
      active: __view.r.value === "聊天"
    }]),
    onClick: w[3] || (w[3] = A => __view.d("聊天"))
  }, [me(M(Ju), {
    size: 17
  }), w[16] || (w[16] = o("span", null, "聊天", -1))]), approvedButtonVNode({
    class: "nav-item",disabled:!M(__view.character).id,
    onClick: w[4] || (w[4] = A => __view.a.value = !__view.a.value),
    "aria-expanded": __view.a.value
  }, [me(M(Co), {
    size: 17
  }), w[17] || (w[17] = o("span", null, "生活", -1)), me(M(wo), {
    size: 13
  })]), __view.a.value ? (p(), g("div", bv, [(p(), g(O, null, oe(__view.t, A => approvedButtonVNode({
    key: A,
    "data-testid": "nav-" + A,
    disabled:!M(__view.character).id,
    "aria-current": __view.r.value === A ? "page" : void 0,
    class: ge(["nav-item", {
      active: __view.r.value === A
    }]),
    onClick: B => __view.d(A)
  }, v(A))), 64))])) : X("", !0), (p(), g(O, null, oe(__view.s, ([A, B]) => approvedButtonVNode({
    key: A,
    "data-testid": "nav-" + A,
    disabled:!M(__view.character).id,
    "aria-current": __view.r.value === A ? "page" : void 0,
    class: ge(["nav-item", {
      active: __view.r.value === A
    }]),
    onClick: te => __view.d(A)
  }, [(p(), Ae(as(B), {
    size: 17
  })), o("span", null, v(A), 1)])), 64))])), w[18] || (w[18] = o("hr", {
    class: "nav-divider"
  }, null, -1)), (p(), g("nav", {
    key: "global-" + __view.r.value
  }, [(p(), g(O, null, oe(__view.n, ([A, B]) => approvedButtonVNode({
    key: A,
    "data-testid": "nav-" + A,
    "aria-current": __view.r.value === A ? "page" : void 0,
    class: ge(["nav-item", {
      active: __view.r.value === A
    }]),
    onClick: te => __view.d(A)
  }, [(p(), Ae(as(B), {
    size: 17
  })), o("span", null, v(A), 1)])), 64))])), w[19] || (w[19] = o("div", {
    class: "sidebar-footer"
  }, [Z("每一个世界，都有回响 "), o("small", null, "Fluctlight · v6")], -1))]), o("main", {
    ref_key: "main",
    ref: __view.c,
    class: ge({
      "chat-main": __view.r.value === "聊天"
    })
  }, [o("div", $v, [o("div", null, [o("div", Cv, [Z(v(M(__view.character).id?M(__view.character).name+" 的工作区 ":"新的相识 "), 1), me(M(Ji), {
    size: 12
  }), Z(v(__view.r.value), 1)]), o("h1", null, [Z(v(__view.r.value), 1), __view.r.value === "聊天" ? (p(), g("span", xv, "日常的片刻，与你分享。")) : X("", !0)])]), __view.r.value === "聊天" ? (p(), approvedButtonBlock({
    key: 0,
    class: "button",
    onClick: w[5] || (w[5] = A => __view.f.value = !__view.f.value)
  }, [(p(), Ae(as(__view.f.value ? M(Yu) : M(Zu)), {
    size: 15
  })), Z(v(__view.f.value ? "收起预览" : "展开预览"), 1)])) : X("", !0)]), __view.r.value === "聊天" ? (p(), Ae(__view.ChatPage, {
    key: M(__view.character).id,
    preview: __view.f.value,
    onGo: __view.d
  }, null, 8, ["preview"])) : (p(), g("div", Sv, [__view.r.value === "衣柜" ? (p(), Ae(__view.WardrobePage, {
    key: "w" + M(__view.character).id
  })) : __view.r.value === "世界" ? (p(), Ae(__view.WorldPage, {
    key: "world" + M(__view.character).id
  })) : __view.r.value === "角色详情" ? (p(), Ae(__view.CharacterDetailPage,{key:"profile"+M(__view.character).id})) : __view.t.includes(__view.r.value) ? (p(), Ae(__view.LifePage, {
    page: __view.r.value,
    key: M(__view.character).id,
    onGo: __view.d
  }, null, 8, ["page"])) : __view.r.value === "管理中心" ? (p(), Ae(__view.ManagementPage, {
    key: 3
  })) : __view.r.value === "角色库" ? (p(), Ae(__view.LibraryPage,{key:"library"})) : __view.r.value === "初始化角色" ? (p(), Ae(__view.CharacterWizardPage,{key:"creation"})) : (p(), Ae(__view.BrowsePage, {
    page: __view.r.value,
    key: __view.r.value + M(__view.character).id
  }, null, 8, ["page"]))]))], 2)]), M(__view.toast) ? (p(), g("div", Tv, [Z(v(M(__view.toast).text) + " ", 1), M(__view.toast).link ? (p(), g("a", {
    key: 0,
    href: "#衣柜",
    onClick: w[6] || (w[6] = A => {
      __view.d("衣柜"), __view.outfitTab.value = "方案库";
    })
  }, "去方案库 ↗")) : X("", !0)])) : X("", !0)]));
}

export const renderApp = unoptimizedRenderer(renderAppBody);
