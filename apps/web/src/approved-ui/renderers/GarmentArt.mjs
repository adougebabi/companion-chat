import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import {openBlock as p,createElementBlock as g,Fragment as O,createElementVNode as o,createCommentVNode as X} from 'vue';
const gf={
  viewBox: "0 0 100 100",
  width: "84",
  height: "84",
  fill: "none",
  stroke: "currentColor",
  "stroke-width": "2.5",
  "stroke-linejoin": "round",
  "aria-hidden": "true"
};
const yf={
  key: 0,
  d: "M50 28v56M34 18l8 26 8-9 8 9 8-26"
};
function renderGarmentArtBody(__view){return (t, s) => (p(), g("svg", gf, [__view.props.type === "鞋子" ? (p(), g(O, {
  key: 0
}, [s[0] || (s[0] = o("path", {
  d: "M18 55l15-25 16 11 8 17 25 8v13H16V62z",
  fill: "currentColor",
  opacity: ".3"
}, null, -1)), s[1] || (s[1] = o("path", {
  d: "M18 55l15-25 16 11 8 17 25 8v13H16V62zM17 72h65M39 47l12-4M43 54l12-4"
}, null, -1))], 64)) : __view.props.type === "下衣" ? (p(), g(O, {
  key: 1
}, [s[2] || (s[2] = o("path", {
  d: "M28 15h44l7 70H55L50 47 45 85H21z",
  fill: "currentColor",
  opacity: ".25"
}, null, -1)), s[3] || (s[3] = o("path", {
  d: "M28 15h44l7 70H55L50 47 45 85H21zM28 25h44M50 25v22"
}, null, -1))], 64)) : __view.props.type === "配饰" ? (p(), g(O, {
  key: 2
}, [s[4] || (s[4] = o("circle", {
  cx: "50",
  cy: "50",
  r: "21"
}, null, -1)), s[5] || (s[5] = o("path", {
  d: "M39 30V10h22v20M39 70v20h22V70M50 36v16l10 7"
}, null, -1))], 64)) : __view.props.type === "袜子" ? (p(), g(O, {
  key: 3
}, [s[6] || (s[6] = o("path", {
  d: "M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13z",
  fill: "currentColor",
  opacity: ".25"
}, null, -1)), s[7] || (s[7] = o("path", {
  d: "M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13zM37 25h28M26 64l22 15"
}, null, -1))], 64)) : __view.props.type === "内裤" ? (p(), g(O, {
  key: 4
}, [s[8] || (s[8] = o("path", {
  d: "M18 28h64l-15 40H33z",
  fill: "currentColor",
  opacity: ".25"
}, null, -1)), s[9] || (s[9] = o("path", {
  d: "M18 28h64l-15 40H33zM20 37h60M22 42l22 26M78 42L56 68"
}, null, -1))], 64)) : (p(), g(O, {
  key: 5
}, [s[10] || (s[10] = o("path", {
  d: "M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z",
  fill: "currentColor",
  opacity: ".25"
}, null, -1)), s[11] || (s[11] = o("path", {
  d: "M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z"
}, null, -1)), __view.props.type === "外套" ? (p(), g("path", yf)) : X("", !0)], 64))]));}

export const renderGarmentArt = unoptimizedRenderer(renderGarmentArtBody);
