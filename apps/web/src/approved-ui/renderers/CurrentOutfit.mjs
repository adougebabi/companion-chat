import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, normalizeClass as ge, createElementVNode as o, Fragment as O, renderList as oe, unref as M, toDisplayString as v } from 'vue';
const At = ["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"];
function renderCurrentOutfitBody(__view) {
  return (s, n) => (p(), g("section", {
    class: ge(["outfit-block", {
      "outfit-compact": __view.props.compact
    }]),
    "data-testid": "current-outfit"
  }, [o("div", {
    class: "row"
  }, [n[0] || (n[0] = o("b", null, "今日穿着", -1)), n[1] || (n[1] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    class: "text-button",
    onClick: __view.t
  }, "去编辑 ↗")]), __view.readStatus?.value ? o("p",{class:"muted",role:"status"},__view.readStatus.value) : null, o("div", {
    class: ge(__view.props.compact ? "outfit-tags" : "outfit-lines")
  }, [(p(!0), g(O, null, oe(M(At), l => (p(), g("div", {
    key: l
  }, [o("small", null, v(l), 1), o("span", null, v(M(__view.outfitNames)(M(__view.character), l)), 1)]))), 128))], 2)], 2));
}

export const renderCurrentOutfit = unoptimizedRenderer(renderCurrentOutfitBody);
