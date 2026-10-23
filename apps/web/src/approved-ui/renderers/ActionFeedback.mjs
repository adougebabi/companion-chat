import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, createTextVNode as Z, toDisplayString as v, createElementVNode as o, createCommentVNode as X } from 'vue';
const Dc = {
  key: 0,
  class: "inline-error",
  role: "alert"
};
function renderActionFeedbackBody(__view) {
  return (n, l) => {
    var i;
    return ((i = __view.s.value) == null ? void 0 : i.status) === "error" ? (p(), g("p", Dc, [Z(v(__view.s.value.reason) + " ", 1), approvedButtonVNode({
      class: "text-button",
      onClick: l[0] || (l[0] = (...r) => __view.s.value.run && __view.s.value.run(...r))
    }, v(__view.s.value.actionText||"重试"))])) : X("", !0);
  };
}

export const renderActionFeedback = unoptimizedRenderer(renderActionFeedbackBody);
