import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, createElementVNode as o, createCommentVNode as X, Fragment as O, renderList as oe, toDisplayString as v, normalizeClass as ge } from 'vue';
const Hc = {
  class: "filter-search-row"
};
const Bc = ["value", "placeholder", "aria-label", "data-testid"];
const Kc = ["data-testid"];
const qc = ["aria-pressed", "data-testid", "onClick"];
const zc = ["aria-pressed", "data-testid", "onClick"];
const Wc = ["aria-label"];
const jc = ["data-testid"];
function renderFilterChipsBody(__view) {
  return (a, u) => (p(), g("section", {
    class: "filter-chips",
    "data-testid": __view.props.testId
  }, [o("div", Hc, [o("input", {
    value: __view.props.search,
    type: "search",
    placeholder: __view.props.placeholder,
    "aria-label": __view.props.placeholder,
    "data-testid": `${__view.props.testId}-search`,
    onInput: u[0] || (u[0] = f => __view.n("update:search", f.target.value))
  }, null, 40, Bc), __view.l.value ? (p(), approvedButtonBlock({
    key: 0,
    type: "button",
    class: "text-button",
    "data-testid": `${__view.props.testId}-clear`,
    onClick: __view.r
  }, "清除")) : X("", !0)]), (p(!0), g(O, null, oe(__view.props.groups, f => (p(), g("div", {
    key: f.key,
    class: "filter-group",
    role: "group",
    "aria-label": f.label
  }, [o("small", null, v(f.label), 1), approvedButtonVNode({
    type: "button",
    class: ge(["filter-chip", {
      active: !__view.props.filters[f.key]
    }]),
    "aria-pressed": !__view.props.filters[f.key],
    "data-testid": `${__view.props.testId}-${f.key}-all`,
    onClick: c => __view.i(f.key, "")
  }, "全部"), (p(!0), g(O, null, oe(f.options, c => (p(), approvedButtonBlock({
    key: c,
    type: "button",
    class: ge(["filter-chip", {
      active: __view.props.filters[f.key] === c
    }]),
    "aria-pressed": __view.props.filters[f.key] === c,
    "data-testid": `${__view.props.testId}-${f.key}-${c}`,
    onClick: d => __view.i(f.key, c)
  }, v(c)))), 128))], 8, Wc))), 128))], 8, jc));
}

export const renderFilterChips = unoptimizedRenderer(renderFilterChipsBody);
