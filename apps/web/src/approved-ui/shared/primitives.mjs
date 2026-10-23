import { defineComponent, h, createVNode, createBlock } from "vue";
/** Shared native button. Existing demo classes and native form semantics pass through unchanged. */
export const ActionButton = defineComponent({
  name: "ActionButton",
  inheritAttrs: false,
  setup(_props, { attrs, slots }) {
    return () => h("button", attrs, slots.default?.());
  },
});
/** Shared surface with a semantic native root and no extra layout wrapper. */
export const Surface = defineComponent({
  name: "Surface",
  inheritAttrs: false,
  props: { as: { type: String, default: "div" } },
  setup(props, { attrs, slots }) {
    return () => h(props.as, attrs, slots.default?.());
  },
});
const dynamic = 16 | 1024;
export const approvedButtonVNode = (props, children) =>
  createVNode(ActionButton, props, { default: () => children }, dynamic);
export const approvedButtonBlock = (props, children) =>
  createBlock(ActionButton, props, { default: () => children }, dynamic);
export const approvedSurfaceVNode = (tag, props, children) =>
  createVNode(
    Surface,
    { ...props, as: tag },
    { default: () => children },
    dynamic,
  );
export const approvedSurfaceBlock = (tag, props, children) =>
  createBlock(
    Surface,
    { ...props, as: tag },
    { default: () => children },
    dynamic,
  );
