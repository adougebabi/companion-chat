import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, Fragment as O, createElementVNode as o, renderList as oe, unref as M, toDisplayString as v, createVNode as me, createTextVNode as Z, withModifiers as Ve, withDirectives as pe, vModelText as $e, createCommentVNode as X, normalizeClass as ge, createBlock as Ae, resolveDynamicComponent as as, normalizeStyle as yt } from 'vue';
import { Tl, $o, So, xo } from '../shared/icons.mjs';
const Wp = {
  class: "row"
};
const qp = {
  class: "avatar small"
};
const zp = {
  class: "post-art"
};
const Gp = {
  class: "row"
};
const Jp = ["onClick"];
const Yp = ["data-testid", "onClick"];
const Zp = {
  key: 0,
  class: "comments"
};
const Qp = {
  class: "avatar small"
};
const e1 = ["onUpdate:modelValue", "data-testid", "onKeydown"];
const t1 = ["disabled"];
const Xp = ["onSubmit"];
const s1 = {
  class: "muted"
};
const n1 = {
  class: "people-grid"
};
const o1 = {
  class: "avatar"
};
const l1 = ["onClick"];
const i1 = {
  key: 0,
  class: "card empty-state"
};
const r1 = ["aria-label"];
const a1 = {
  class: "muted"
};
const u1 = {
  class: "media-grid"
};
const c1 = {
  class: "card media-tile"
};
const d1 = ["onClick"];
const f1 = {
  key: 0,
  class: "audio-control"
};
const p1 = ["onClick"];
const m1 = ["value"];
const v1 = {
  "data-testid": "media-preview-drawer",
  class: "card media-modal",
  role: "dialog",
  "aria-modal": "true",
  "aria-label": "媒体预览"
};
const h1 = {
  class: "row"
};
const g1 = {
  key: 1,
  class: "card empty-state"
};
const y1 = {
  class: "row"
};
const b1 = {
  class: "pill"
};
const _1 = ["onClick"];
const k1 = {
  key: 0,
  class: "pill current-badge"
};
const w1 = ["onClick"];
const $1 = {
  key: 0,
  class: "inline-profile"
};
const C1 = {
  key: 0
};
const x1 = {
  key: 4,
  class: "card placeholder"
};
const T1 = {
  class: "pill"
};
const S1 = ["onClick"];
const M1 = ["data-testid", "onClick"];
const E1 = {
  key: 0,
  class: "muted"
};
function renderBrowsePageBody(__view) {
  return (V, U) => __view.props.page === "动态" ? (p(), g(O, {
    key: 0
  }, [U[13] || (U[13] = o("p", {
    class: "muted"
  }, "生活的切片，留一点位置给彼此的回应。", -1)), (p(!0), g(O, null, oe(M(__view.character).posts, (_, ee) => {
    var he;
    return p(), approvedSurfaceBlock("article", {
      key: ee,
      class: "card post compact"
    }, [o("div", Wp, [o("span", qp, v(_.authorInitial), 1), o("b", null, v(_.authorName), 1), o("small", null, v(_.time), 1)]), o("p", null, v(_.text), 1), o("div", zp, [me(M(Tl), {
      size: 36
    }), U[10] || (U[10] = o("span", null, "把日子过成一首小诗", -1))]), o("div", Gp, [approvedButtonVNode({
      class: "text-button",
      onClick: () => __view.reactPost(_)
    }, v(_.liked ? "♥" : "♡") + " " + v(_.likes) + " 喜欢"), approvedButtonVNode({
      "data-testid": "comment-toggle-" + ee,
      class: "text-button",
      onClick: j => __view.w.value[ee] = !__view.w.value[ee]
    }, "评论 " + v(_.comments.length) + " " + v(__view.w.value[ee] ? "收起" : "展开"))]), __view.w.value[ee] ? (p(), g("div", Zp, [(p(!0), g(O, null, oe(_.comments, (j, T) => (p(), g("p", {
      key: T
    }, [o("span", Qp, v(j.name.slice(0, 1)), 1), o("span", null, [o("b", null, v(j.name), 1), U[11] || (U[11] = Z()), o("small", null, v(j.time), 1), U[12] || (U[12] = o("br", null, null, -1)), Z(v(j.text), 1)])]))), 128)), o("form", {
      class: "input-row",
      onSubmit: Ve(j => __view.q(ee), ["prevent"])
    }, [pe(o("textarea", {
      "onUpdate:modelValue": j => __view.C.value[ee] = j,
      placeholder: "留下一句回应…",
      "aria-label": "评论内容",
      rows: "2",
      "data-testid": "comment-input-" + ee,
      onKeydown: j => __view.ue(j, ee)
    }, null, 40, e1), [[$e, __view.C.value[ee]]]), approvedButtonVNode({
      class: "button",
      disabled: M(__view.t) || !((he = __view.C.value[ee]) != null && he.trim())
    }, v(M(__view.t) && __view.ne.value === ee ? "处理中…" : "发表评论"))], 40, Xp)])) : X("", !0)]);
  }), 128)), me(__view.ActionFeedback, {
    labels: ["动态"]
  })], 64)) : __view.props.page === "人物关系" ? (p(), g(O, {
    key: 1
  }, [me(__view.FilterChips, {
    search: __view.f.value,
    "onUpdate:search": U[0] || (U[0] = _ => __view.f.value = _),
    filters: __view.c.value,
    "onUpdate:filters": U[1] || (U[1] = _ => __view.c.value = _),
    groups: __view.A.value,
    "test-id": "people-filters",
    placeholder: "搜索姓名 / 身份"
  }, null, 8, ["search", "filters", "groups"]), o("small", s1, v(__view.I.value.length) + " 位相识", 1), o("div", n1, [(p(!0), g(O, null, oe(__view.I.value, _ => (p(), approvedButtonBlock({
    key: _.id,
    class: "card person-card",
    onClick: ee => __view.d.value = _.id
  }, [o("span", o1, v(_.name.slice(0, 1)), 1), o("h3", null, v(_.name), 1), o("p", null, v(_.identity) + " · " + v(_.relation), 1), o("small", null, v(_.location) + " · " + v(_.tag), 1)]))), 128))]), __view.I.value.length ? X("", !0) : (p(), approvedSurfaceBlock("div", i1, [U[14] || (U[14] = o("p", null, "暂无匹配人物", -1)), approvedButtonVNode({
    class: "button",
    onClick: __view.se
  }, "清除筛选")])), __view.K.value ? (p(), g("div", {
    key: 1,
    "data-testid": "person-drawer-overlay",
    class: "drawer-backdrop",
    onClick: U[3] || (U[3] = Ve(_ => __view.d.value = null, ["self"]))
  }, [o("aside", {
    "data-testid": "person-drawer",
    class: "detail-drawer",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": __view.K.value.name
  }, [approvedButtonVNode({
    class: "button",
    onClick: U[2] || (U[2] = _ => __view.d.value = null)
  }, "关闭"), o("h2", null, v(__view.K.value.name), 1), o("p", null, v(__view.K.value.identity) + " · " + v(__view.K.value.type) + " · " + v(__view.K.value.status), 1), o("p", null, v(__view.K.value.bio), 1), o("p", null, [Z("关系：" + v(__view.K.value.relation), 1), U[15] || (U[15] = o("br", null, null, -1)), Z("位置：" + v(__view.K.value.location), 1), U[16] || (U[16] = o("br", null, null, -1)), Z("日程：" + v(__view.K.value.schedule), 1), U[17] || (U[17] = o("br", null, null, -1)), Z("标签：" + v(__view.K.value.tag), 1)])], 8, r1)])) : X("", !0)], 64)) : __view.props.page === "媒体" ? (p(), g(O, {
    key: 2
  }, [me(__view.FilterChips, {
    search: __view.f.value,
    "onUpdate:search": U[4] || (U[4] = _ => __view.f.value = _),
    filters: __view.c.value,
    "onUpdate:filters": U[5] || (U[5] = _ => __view.c.value = _),
    groups: __view.B.value,
    "test-id": "media-filters",
    placeholder: "搜索媒体 / 角色"
  }, null, 8, ["search", "filters", "groups"]), o("small", a1, v(__view.H.value.length) + " 项", 1), o("div", u1, [(p(!0), g(O, null, oe(__view.H.value, _ => (p(), approvedSurfaceBlock("article", c1, [approvedButtonVNode({
    class: "media-open",
    onClick: ee => __view.d.value = _.id
  }, [o("div", {
    class: ge(["media-preview", "media-art-" + _.artIndex % 6, "media-type-" + _.type, _.type === "图片" ? "photo" : ""])
  }, [__view.mediaContent(_,false)], 2), o("b", null, v(_.title), 1), o("small", null, v(_.character) + " · " + v(_.type) + " · " + v(_.time), 1)]), _.type === "语音" ? (p(), g("div", f1, [approvedButtonVNode({
    class: "button",
    onClick: ee => __view.playMedia(_.id)
  }, v(__view.s.value === _.id ? "暂停" : "播放")), o("progress", {
    value: __view.n.value[_.id] || 0,
    max: "100"
  }, null, 8, m1), o("small", null, v(__view.mediaTime(_.id)), 1)])) : X("", !0)]))), 256))]), __view.d.value !== null ? (p(), g("div", {
    key: 0,
    class: "modal-backdrop",
    onClick: U[7] || (U[7] = Ve(_ => __view.d.value = null, ["self"]))
  }, [approvedSurfaceVNode("section", v1, [(p(!0), g(O, null, oe(M(__view.media).filter(_ => _.id === __view.d.value), _ => (p(), g(O, null, [o("div", h1, [o("h2", null, v(_.title), 1), U[18] || (U[18] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    class: "button",
    onClick: U[6] || (U[6] = ee => __view.d.value = null)
  }, "关闭预览")]), o("div", {
    class: ge(["media-preview", "large-media", "media-art-" + _.artIndex % 6, "media-type-" + _.type])
  }, [__view.mediaContent(_,true)], 2), o("p", null, v(_.character) + " · " + v(_.time) + " · " + v(_.tag), 1)], 64))), 256))])])) : X("", !0), __view.H.value.length ? X("", !0) : (p(), approvedSurfaceBlock("div", g1, [U[19] || (U[19] = o("p", null, "暂无匹配媒体。", -1)), approvedButtonVNode({
    class: "button",
    "data-testid": "media-empty-clear",
    onClick: __view.se
  }, "清除筛选")]))], 64)) : __view.props.page === "角色库" ? (p(), g(O, {
    key: 3
  }, [approvedButtonVNode({class:"button",onClick:__view.createCharacter},"认识新的人"),me(__view.FilterChips, {
    search: __view.f.value,
    "onUpdate:search": U[8] || (U[8] = _ => __view.f.value = _),
    filters: __view.c.value,
    "onUpdate:filters": U[9] || (U[9] = _ => __view.c.value = _),
    groups: __view.te.value,
    "test-id": "library-filters",
    placeholder: "搜索角色 / 职业"
  }, null, 8, ["search", "filters"]), (p(!0), g(O, null, oe(__view.le.value, _ => (p(), approvedSurfaceBlock("article", {
    key: _.id,
    class: "card compact"
  }, [o("div", y1, [o("span", {
    class: "avatar",
    style: yt({
      background: _.color
    })
  }, v(_.initial), 5), o("div", null, [o("b", null, v(_.name) + " · " + v(_.job), 1), o("p", null, v(_.subtitle), 1), (p(!0), g(O, null, oe(_.tags, ee => (p(), g("span", b1, v(ee), 1))), 256))]), U[20] || (U[20] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    class: "text-button",
    onClick: ee => __view.d.value = __view.d.value === _.id ? null : _.id
  }, v(__view.d.value === _.id ? "收起详情" : "角色详情")), M(__view.selectedId) === _.id ? (p(), g("span", k1, "当前")) : (p(), approvedButtonBlock({
    key: 1,
    class: "button primary",
    onClick: ee => __view.selectedId.value = _.id
  }, "切换角色"))]), __view.d.value === _.id ? (p(), g("div", $1, [o("h2", null, v(_.name), 1), o("p", null, v(_.bio), 1), o("p", null, [Z("职业：" + v(_.job), 1), U[21] || (U[21] = o("br", null, null, -1)), Z("兴趣：" + v(_.hobby), 1), U[22] || (U[22] = o("br", null, null, -1)), Z("心情：" + v(_.mood), 1), U[23] || (U[23] = o("br", null, null, -1)), Z("位置：" + v(_.location), 1), U[24] || (U[24] = o("br", null, null, -1)), Z("世界：" + v(_.world), 1)])])) : X("", !0)]))), 128)), __view.le.value.length ? X("", !0) : (p(), g("p", C1, "暂无匹配角色。"))], 64)) : __view.props.page === "创作中心" ? (p(), approvedSurfaceBlock("div", x1, [o("p",null,"即将重新设计"),approvedButtonVNode({class:"button",onClick:__view.openStudio},"打开创作工作台")])) : __view.props.page === "通知与待处理" ? (p(), g(O, {
    key: 5
  }, [U[29] || (U[29] = o("h2", null, "未读消息", -1)),M(__view.characters).length?X("",!0):o("p",{class:"muted"},"暂时没有未读消息。"), (p(!0), g(O, null, oe(M(__view.characters), (_, ee) => {
    var he;
    return p(), approvedButtonBlock({
      class: "card row notification-card",
      onClick: j => __view.a(_.id)
    }, [o("span", {
      class: "avatar",
      style: yt({
        background: _.color
      })
    }, v(_.initial), 5), o("div", null, [o("b", null, v(_.noticeTitle || _.name), 1), o("p", null, v((he = _.messages.filter(j => !j.user).at(-1)) == null ? void 0 : he.text.slice(0, 60)), 1)]), U[25] || (U[25] = o("span", {
      class: "spacer"
    }, null, -1)), o("span", T1, v(_.unreadCount) + " 条未读", 1)]);
  }), 256)), o("h2", null, [U[26] || (U[26] = Z("异常检查 ", -1)), o("small", null, v(__view.i.value.length) + " 项", 1)]), (p(!0), g(O, null, oe(__view.i.value, _ => (p(), approvedButtonBlock({
    "data-testid": "notification-diagnostic-" + _.id,
    class: "card row notification-card",
    onClick: ee => __view.r(_.id)
  }, [o("b", null, v(_.label), 1), o("span", null, v(_.reason), 1), U[27] || (U[27] = o("span", {
    class: "spacer"
  }, null, -1)), U[28] || (U[28] = Z("查看诊断 ↗", -1))]))), 256)), __view.i.value.length ? X("", !0) : (p(), g("p", E1, "目前没有异常。"))], 64)) : X("", !0);
}

export const renderBrowsePage = unoptimizedRenderer(renderBrowsePageBody);
