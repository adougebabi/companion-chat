import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createBlock as Ae, Teleport as ea, createElementVNode as o, withModifiers as Ve, toDisplayString as v, renderSlot as ha } from 'vue';
const Yc = {
  class: "drawer-heading"
};
const Zc = ["data-testid"];
const Jc = ["aria-label", "data-testid"];
const Gc = ["data-testid"];
function renderDetailDrawerBody(__view) {
  return (r, a) => (p(), Ae(ea, {
    to: "body"
  }, [o("div", {
    class: "drawer-backdrop",
    "data-testid": `${__view.props.testId}-overlay`,
    onClick: a[1] || (a[1] = Ve(u => __view.s("close"), ["self"]))
  }, [o("aside", {
    ref_key: "panel",
    ref: __view.n,
    class: "detail-drawer",
    role: "dialog",
    "aria-modal": "true",
    "aria-label": __view.props.title,
    tabindex: "-1",
    "data-testid": __view.props.testId
  }, [o("header", Yc, [o("h2", null, v(__view.props.title), 1), approvedButtonVNode({
    type: "button",
    class: "text-button",
    "data-testid": `${__view.props.testId}-close`,
    onClick: a[0] || (a[0] = u => __view.s("close"))
  }, "关闭")]), ha(r.$slots, "default")], 8, Jc)], 8, Gc)]));
}

export const renderDetailDrawer = unoptimizedRenderer(renderDetailDrawerBody);
