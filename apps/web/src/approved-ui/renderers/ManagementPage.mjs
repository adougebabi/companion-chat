import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, Fragment as O, createElementVNode as o, renderList as oe, normalizeClass as ge, unref as M, toDisplayString as v, withModifiers as Ve, createTextVNode as Z, withDirectives as pe, vModelText as $e, vModelCheckbox as ss, vModelSelect as Et, createBlock as Ae, resolveDynamicComponent as as, withCtx as Ht, createCommentVNode as X, vModelDynamic as Vu, createVNode as me } from 'vue';
const I1 = {
  class: "tabs"
};
const O1 = ["onClick"];
const V1 = {
  key: 0,
  class: "configuration-layout"
};
const P1 = {
  class: "configuration-nav",
  "aria-label": "配置章节",
  "data-testid": "config-section-nav"
};
const L1 = ["href", "data-testid", "onClick"];
const N1 = {
  class: "row"
};
const F1 = ["onClick"];
const R1 = {
  class: "form-grid"
};
const D1 = ["onUpdate:modelValue"];
const j1 = ["onUpdate:modelValue"];
const H1 = ["onUpdate:modelValue"];
const B1 = ["onUpdate:modelValue"];
const K1 = {
  class: "check-field"
};
const W1 = ["onUpdate:modelValue"];
const q1 = {
  class: "muted"
};
const z1 = ["onClick"];
const U1 = ["id"];
const J1 = {
  class: "form-grid"
};
const Z1 = ["value"];
const Y1 = ["onUpdate:modelValue"];
const Q1 = ["onUpdate:modelValue", "list"];
const em = ["value"];
const X1 = ["id"];
const tm = ["onUpdate:modelValue"];
const sm = ["onUpdate:modelValue"];
const nm = ["onUpdate:modelValue", "min"];
const lm = {
  class: "muted"
};
const om = {
  class: "management-block"
};
const im = {
  class: "shared-scenario"
};
const G1 = ["id"];
const am = {
  class: "form-grid"
};
const um = {
  class: "check-field"
};
const dm = {
  class: "form-grid"
};
const fm = {
  class: "quiet-days"
};
const Yi = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"];
const pm = ["value"];
const cm = ["disabled"];
const rm = ["id"];
const mm = {
  key: 0
};
const vm = {
  key: 1
};
const hm = {
  key: 2,
  class: "muted"
};
const gm = {
  class: "form-grid"
};
const ym = ["onUpdate:modelValue"];
const bm = ["value", "onInput"];
const _m = ["onUpdate:modelValue", "type", "min", "max", "step", "placeholder"];
const km = ["onClick"];
const wm = {
  key: 0,
  role: "status",
  class: "muted"
};
const $m = {
  class: "settings-save-bar",
  "data-testid": "config-save-bar"
};
const Cm = {
  key: 0,
  role: "alert",
  class: "inline-error"
};
const xm = {
  key: 1,
  class: "muted"
};
const Sm = ["disabled"];
const Tm = {
  class: "tabs"
};
const Mm = ["data-testid", "onClick"];
const Em = {
  class: "muted"
};
const Am = {
  class: "diagnostic-feed",
  "data-testid": "diagnostic-feed"
};
const Vm = {
  class: "diagnostic-meta"
};
const Pm = {
  class: "pill"
};
const Lm = {
  class: "diagnostic-summary"
};
const Om = ["data-testid", "onClick"];
const Um = ["data-testid", "onClick"];
const Im = ["id", "data-testid"];
const Nm = {
  key: 0,
  class: "card empty-state"
};
const Fm = {
  class: "detail-meta"
};
const Rm = {
  class: "pill"
};
const Dm = {
  key: 0,
  class: "inline-error"
};
const jm = {
  class: "media-call-attributes",
  "data-testid": "media-call-details"
};
const Hm = {
  class: "detail-section"
};
const Bm = {
  class: "detail-section"
};
const Km = {
  class: "detail-section"
};
const Wm = {
  class: "media-call-attributes"
};
const qm = {
  class: "detail-section",
  "data-testid": "media-call-output"
};
const zm = {
  key: 0,
  class: "mock-image-output"
};
const Gm = {
  key: 1,
  class: "muted"
};
const Jm = {
  class: "muted"
};
const Ym = {
  class: "detail-section"
};
const Zm = {
  class: "detail-section"
};
const Qm = {
  class: "detail-section"
};
const Xm = {
  class: "secondary-diagnostics"
};
const ev = {
  class: "quiet-days"
};
const tv = ["onClick"];
const sv = {
  key: 0,
  class: "card"
};
const nv = {
  class: "row"
};
const lv = {
  role: "status"
};
function renderManagementPageBody(__view) {
  return (F, $) => (p(), g(O, null, [o("div", I1, [(p(), g(O, null, oe(["配置中心", "运行诊断"], h => approvedButtonVNode({
    key: h,
    class: ge({
      active: M(__view.a) === h
    }),
    onClick: b => __view.a.value = h
  }, v(h))), 64))]), M(__view.a) === "配置中心" ? (p(), g("div", V1, [o("nav", P1, [(p(!0), g(O, null, oe(__view.U.value, h => (p(), g("a", {
    key: h,
    href: "#" + encodeURIComponent("管理中心"),
    "data-testid": "config-anchor-" + h,
    onClick: Ve(b => __view.ee(h), ["prevent"])
  }, v(h), 9, L1))), 128))]), o("form", {
    ref: __view.configForm,
    class: "settings-form management-form",
    onSubmit: Ve(__view.R, ["prevent"])
  }, [approvedSurfaceVNode("section", {
    id: __view._("服务地址"),
    class: "card settings-group"
  }, [o("div", {
    class: "row"
  }, [$[12] || ($[12] = o("h2", null, "服务地址", -1)), $[13] || ($[13] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    type: "button",
    class: "text-button",
    onClick: __view.ye
  }, "添加服务")]), (p(!0), g(O, null, oe(__view.l.value, (h, b) => (p(), g("div", {
    key: h.id,
    class: "management-block"
  }, [o("div", N1, [o("b", null, v(h.name), 1), o("small", null, v(h.status), 1), $[14] || ($[14] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    type: "button",
    class: "text-button",
    onClick: () => __view.removeEndpoint(b)
  }, "移除")]), o("div", R1, [o("label", null, [$[15] || ($[15] = Z("服务名称", -1)), pe(o("input", {
    "onUpdate:modelValue": k => h.name = k,
    readonly: !!h._source,
    required: "",
    maxlength: "128"
  }, null, 8, D1), [[$e, h.name]])]), o("label", null, [$[16] || ($[16] = Z("接口协议", -1)), pe(o("select", {
    "onUpdate:modelValue": k => h.protocol = k,
    required: ""
  }, [o("option",{value:"openai-compatible"},"OpenAI 兼容接口"),h.protocol&&h.protocol!=="openai-compatible"?o("option",{value:h.protocol},h.protocol):X("",!0)], 8, j1), [[Et, h.protocol]])]), o("label", null, [$[17] || ($[17] = Z("服务地址", -1)), pe(o("input", {
    "onUpdate:modelValue": k => h.url = k,
    required: "",
    type: "url"
  }, null, 8, H1), [[$e, h.url]])]), o("label", null, [$[18] || ($[18] = Z("访问密钥", -1)), pe(o("input", {
    "onUpdate:modelValue": k => h.secret = k,
    type: "password",
    placeholder: h._source?.secret_configured ? "留空保留已保存的密钥" : "可选：填写访问密钥",
    autocomplete: "new-password"
  }, null, 8, B1), [[$e, h.secret]])]), h._source?.secret_configured ? o("label", K1, [$[19] || ($[19] = Z("清除已存密钥", -1)), pe(o("input", {
    "onUpdate:modelValue": k => h.clearSecret = k,
    type: "checkbox"
  }, null, 8, W1), [[ss, h.clearSecret]])]) : X("",!0)]), me(__view.ModelDirectory,{models:h.models},null,8,["models"]), approvedButtonVNode({
    type: "button",
    class: "text-button",
    onClick: k => __view.x(h)
  }, "读取模型目录（已保存连接）"),h.feedback?o("p",{class:"muted",role:"status"},h.feedback):X("",!0)]))), 128)), $[20] || ($[20] = o("p", {
    class: "muted"
  }, "服务地址修改并保存后，其用途绑定需要重新确认，已保存绑定与草稿分别显示。", -1))]), approvedSurfaceVNode("section", {
    id: __view._("模型用途"),
    class: "card settings-group"
  }, [$[27] || ($[27] = o("h2", null, "各用途对应的模型配置", -1)), $[28] || ($[28] = o("p", {
    class: "muted"
  }, "可以先配置思考与通用分析、聊天回复；看图与记忆检索向量可在使用时再配置。应用不设置输入、输出 Token 上限。", -1)), (p(!0), g(O, null, oe(__view.i.value, h => {
    var b;
    const required=!!h._source||!!h.endpoint||!!h.model||(h.timeout!=null&&h.timeout!=="");
    return o("details", { key: h.name, class: "management-block", open: !!h._source||['generic_llm','conversation_reply'].includes(h.role) }, [
      o("summary", null, h.name), o("div", J1, [
        o("label", null, ["使用服务", pe(o("select", {"onUpdate:modelValue": value => h.endpoint = value, required, "aria-label":"使用服务"},
          [o("option",{value:""},"尚未配置"),...__view.l.value.map(endpoint => o("option", {value: endpoint.id}, endpoint.name))]), [[Et, h.endpoint]])]),
        o("label", null, ["模型 ID", pe(o("input", {"onUpdate:modelValue": value => h.model = value, list: "models-" + h.role, required, "aria-label":"模型 ID"}), [[$e, h.model]]),
          o("datalist", {id: "models-" + h.role}, (__view.l.value.find(endpoint => endpoint.id === h.endpoint)?.models || []).map(model => o("option", {value: model})))]),
        o("label", null, ["超时（秒，可选）", pe(o("input", {"onUpdate:modelValue": value => h.timeout = value, type: "number", min: "1", placeholder: "默认 120 秒", "aria-label":"超时（秒，可选）"}), [[$e, h.timeout, void 0, {number: true}]] ),o("small",{class:"muted"},"留空使用 120 秒；未配置用途可以全部留空。")])
      ])
    ]);
  }), 128)), o("details", om, [o("summary", null, "共用通用模型的场景 · " + v(M(__view.sharedScenarios).length) + " 项", 1), (p(!0), g(O, null, oe(M(__view.sharedScenarios), h => (p(), g("div", im, [o("span", null, v(h), 1), $[26] || ($[26] = o("small", null, "使用：通用模型", -1))]))), 256))])]), approvedSurfaceVNode("section", {
    id: __view._("安静时段"),
    class: "card settings-group"
  }, [$[37] || ($[37] = o("h2", null, "安静时段", -1)), o("div", am, [o("label", um, [$[29] || ($[29] = Z("启用安静时段", -1)), pe(o("input", {
    "onUpdate:modelValue": $[0] || ($[0] = h => __view.r.value.enabled = h),
    class: "switch",
    type: "checkbox"
  }, null, 512), [[ss, __view.r.value.enabled]])]), o("label", null, [$[31] || ($[31] = Z("适用角色", -1)), pe(o("select", {
    "onUpdate:modelValue": $[1] || ($[1] = h => __view.r.value.character = h)
  }, [$[30] || ($[30] = o("option", null, "全部角色", -1)), (p(!0), g(O, null, oe(M(__view.characters), h => (p(), g("option", null, v(h.name), 1))), 256))], 512), [[Et, __view.r.value.character]])])]), o("fieldset", {
    disabled: !__view.r.value.enabled
  }, [o("div", dm, [o("label", null, [$[32] || ($[32] = Z("开始时间", -1)), pe(o("input", {
    "onUpdate:modelValue": $[2] || ($[2] = h => __view.r.value.start = h),
    type: "time"
  }, null, 512), [[$e, __view.r.value.start]])]), o("label", null, [$[33] || ($[33] = Z("结束时间", -1)), pe(o("input", {
    "onUpdate:modelValue": $[3] || ($[3] = h => __view.r.value.end = h),
    type: "time"
  }, null, 512), [[$e, __view.r.value.end]])]), o("label", null, [$[35] || ($[35] = Z("行为", -1)), pe(o("select", {
    "onUpdate:modelValue": $[4] || ($[4] = h => __view.r.value.behavior = h)
  }, [...($[34] || ($[34] = [o("option", null, "暂停主动联系", -1), o("option", {
    disabled: true,
    title: "\u5F53\u524D\u7B56\u7565\u53EA\u6682\u505C\u65B0\u4E3B\u52A8\u884C\u52A8\uFF1B\u5DF2\u53D7\u7406\u4EFB\u52A1\u4ECD\u6309\u539F\u671F\u9650\u5904\u7406"
  }, "延后至安静时段结束", -1)]))], 512), [[Et, __view.r.value.behavior]])])]), $[36] || ($[36] = o("p", null, "生效日期（Asia/Shanghai，支持跨夜）", -1)), o("div", fm, [(p(!0), g(O, null, oe(M(Yi), h => (p(), g("label", null, [pe(o("input", {
    "onUpdate:modelValue": $[5] || ($[5] = b => __view.r.value.days = b),
    type: "checkbox",
    value: h
  }, null, 8, pm), [[ss, __view.r.value.days]]), Z(v(h), 1)]))), 256))])], 8, cm)]), (p(!0), g(O, null, oe(__view.n.value, h => (p(), Ae(as("details"), {
    key: h.title,
    id: __view._(h.title),
    class: "card settings-group"
  }, {
    default: Ht(() => [o("summary",mm,h.title === "高级" ? "高级 · Kev 决策服务" : h.title), h.title === "图片（ComfyUI）" ? (p(), g("p", hm, "图片使用 ComfyUI；需要图片功能时再展开配置。")) : X("", !0), o("div", gm, [(p(!0), g(O, null, oe(h.fields, b => me(__view.SettingsField,{key:b.label,field:b})), 128))]), ["图片（ComfyUI）", "高级", "看图理解"].includes(h.title) ? (p(), approvedButtonBlock({
      key: 3,
      type: "button",
      class: "text-button",
      onClick: b => __view.x(h.title)
    }, v(h.title === "看图理解" ? "测试看图（使用已保存配置）" : "测试连接"))) : X("", !0)]),
    _: 2
  }, 1032, ["id"]))), 128)), __view.te.value ? (p(), g("p", wm, v(__view.te.value), 1)) : X("", !0), o("div", $m, [__view.A.value ? (p(), g("p", Cm, v(__view.A.value), 1)) : o("p",{class:"muted",role:"status","aria-live":"polite","data-testid":"config-save-status"},__view.saveStatus.value || "让相处的节奏，更合心意。"), approvedButtonVNode({
    class: "button primary",
    disabled: __view.w.value,
    "data-testid": "config-save"
  }, v(__view.w.value ? "保存中…" : "保存设置"))])], 32)])) : (p(), g(O, {
    key: 1
  }, [$[55] || ($[55] = o("p", {
    class: "muted"
  }, "每次回应背后的记录。模型与媒体的运行情况，都在这里。", -1)), o("div", Tm, [(p(), g(O, null, oe(["模型调用", "媒体调用"], h => approvedButtonVNode({
    key: h,
    class: ge({
      active: __view.u.value === h
    }),
    "data-testid": "diag-tab-" + h,
    onClick: b => __view.q(h)
  }, v(h))), 64))]), me(__view.FilterChips, {
    search: __view.f.value,
    "onUpdate:search": $[6] || ($[6] = h => __view.f.value = h),
    filters: __view.c.value,
    "onUpdate:filters": $[7] || ($[7] = h => __view.c.value = h),
    groups: __view.le.value,
    "test-id": "diag-filters",
    placeholder: "搜索调用 / 提示词 / 工作流 / 角色"
  }, null, 8, ["search", "filters", "groups"]), o("small", Em, v(__view.K.value.length) + " 条记录 · 最近的记录在前", 1), o("section", Am, [(p(!0), g(O, null, oe(__view.K.value, h => {
    var b;
    return p(), g("article", {
      id: "diag-record-" + h.id,
      key: h.id,
      class: ge(["diagnostic-record", {
        highlighted: __view.C.value === h.id
      }]),
      "data-testid": "diag-record-" + h.id
    }, [approvedButtonVNode({
      class: "diagnostic-open",
      "data-testid": "diag-open-" + h.id,
      onClick: k => __view.V(h)
    }, [o("div", Vm, [o("time", null, v(__view.he(h)), 1), o("span", Pm, v(h.label), 1), o("span", null, v(h.character), 1), $[38] || ($[38] = o("span", {
      class: "spacer"
    }, null, -1)), o("span", {
      class: ge(["call-status", h.status])
    }, v(__view.I(h.status)), 3), o("small", null, v(h.durationLabel), 1)]), o("p", Lm, v(h.summary), 1)]), h.status === "error" ? (p(), approvedButtonBlock({
      key: 0,
      class: "text-button diagnostic-retry",
      "data-testid": "diag-retry-" + h.id,
      onClick: k => __view.T(h)
    }, "重试")) : X("", !0)], 10, Im);
  }), 128)), __view.K.value.length ? X("", !0) : (p(), approvedSurfaceBlock("div", Nm, [$[39] || ($[39] = o("p", null, "暂无符合条件的调用。", -1)), approvedButtonVNode({
    class: "button",
    "data-testid": "diag-empty-clear",
    onClick: __view.H
  }, "清除筛选")]))]), __view.S.value ? (p(), Ae(__view.DetailDrawer, {
    key: 0,
    title: __view.S.value.label + " · 调用详情",
    "test-id": "diagnostic-drawer",
    onClose: $[9] || ($[9] = h => __view.d.value = null)
  }, {
    default: Ht(() => [o("div", Fm, [o("span", Rm, v(__view.S.value.kind), 1), o("p", null, v(__view.S.value.character) + " · " + v(__view.he(__view.S.value)) + " · " + v(__view.S.value.durationLabel), 1), o("span", {
      class: ge(["call-status", __view.S.value.status])
    }, v(__view.I(__view.S.value.status)), 3)]), __view.S.value.status === "error" ? (p(), g("p", Dm, [Z(v(__view.S.value.category) + "：" + v(__view.S.value.reason) + " ", 1), approvedButtonVNode({
      class: "text-button",
      "data-testid": "diag-drawer-retry",
      onClick: $[8] || ($[8] = h => __view.T(__view.S.value))
    }, "重试")])) : X("", !0), __view.S.value.kind === "媒体调用" && __view.S.value.media ? (p(), g(O, {
      key: 1
    }, [o("dl", jm, [o("div", null, [$[40] || ($[40] = o("dt", null, "角色", -1)), o("dd", null, v(__view.S.value.character), 1)]), o("div", null, [$[41] || ($[41] = o("dt", null, "媒体类型", -1)), o("dd", null, v(__view.S.value.media.type) + " · 图片", 1)]), o("div", null, [$[42] || ($[42] = o("dt", null, "工作流", -1)), o("dd", null, v(__view.S.value.media.workflow), 1)])]), o("section", Hm, [$[43] || ($[43] = o("h3", null, "提示词 / Prompt", -1)), o("pre", null, v(__view.S.value.media.prompt), 1)]), o("section", Bm, [$[44] || ($[44] = o("h3", null, "负面提示词 / Negative prompt", -1)), o("pre", null, v(__view.S.value.media.negativePrompt), 1)]), o("section", Km, [$[45] || ($[45] = o("h3", null, "生成参数", -1)), o("dl", Wm, [(p(!0), g(O, null, oe(__view.S.value.media.params, (h, b) => (p(), g("div", {
      key: b
    }, [o("dt", null, v({
      width: "宽度",
      height: "高度",
      steps: "步数",
      cfg: "CFG",
      seed: "种子",
      sampler: "采样器"
    }[b]), 1), o("dd", null, v(h), 1)]))), 128))])]), o("section", qm, [$[48] || ($[48] = o("h3", null, "输出", -1)), __view.S.value.status === "success" ? (p(), g("div", zm, [$[46] || ($[46] = o("span", null, "◇", -1)), o("p", null, v(__view.S.value.media.output), 1), $[47] || ($[47] = o("small", null, "实际图片输出", -1))])) : (p(), g("p", Gm, v(__view.S.value.status === "pending" ? "等待图片工作流完成" : "未生成输出，请重试。"), 1))])], 64)) : (p(), g(O, {
      key: 2
    }, [o("p", Jm, [Z("模型 · " + v(__view.S.value.model), 1), __view.S.value.tokens ? (p(), g(O, {
      key: 0
    }, [Z(" · " + v(__view.S.value.tokens) + " token", 1)], 64)) : X("", !0)]), o("section", Ym, [$[49] || ($[49] = o("h3", null, "请求提示词 / Prompt", -1)), o("pre", null, v(__view.S.value.input), 1)]), o("section", Zm, [$[50] || ($[50] = o("h3", null, "响应内容", -1)), o("pre", null, v(__view.S.value.status === "success" ? __view.S.value.response : "尚未收到响应"), 1)])], 64)), o("details", Qm, [$[52] || ($[52] = o("summary", null, "技术详情", -1)), o("p", null, [Z("失败码：" + v(__view.S.value.status === "error" ? __view.S.value.code : "无"), 1), $[51] || ($[51] = o("br", null, null, -1)), Z("关联记录：" + v(__view.S.value.correlationId), 1)])])]),
    _: 1
  }, 8, ["title"])) : X("", !0), o("div", Xm, [$[54] || ($[54] = o("small", null, "其它诊断", -1)), o("div", ev, [(p(!0), g(O, null, oe(M(__view.referenceSections), h => (p(), approvedButtonBlock({
    class: "text-button",
    onClick: b => __view.B.value = __view.B.value === h.title ? "" : h.title
  }, v(h.title)))), 256))]), __view.j.value ? (p(), approvedSurfaceBlock("div", sv, [o("div", nv, [o("b", null, v(__view.j.value.title) + " · 实际记录", 1), $[53] || ($[53] = o("span", {
    class: "spacer"
  }, null, -1)), approvedButtonVNode({
    onClick: $[10] || ($[10] = h => __view.B.value = "")
  }, "关闭")]), (p(!0), g(O, null, oe(__view.j.value.rows, h => (p(), g("p", null, v(h), 1))), 256)), ["后台任务", "待处理事项 / 重试耗尽"].includes(__view.B.value) ? (p(), approvedButtonBlock({
    key: 0,
    class: "text-button",
    onClick: __view.openTaskDiagnostics
  }, "查看原任务与恢复")) : X("", !0), o("p", lv, v(__view.te.value), 1)])) : X("", !0)])], 64))], 64));
}

export const renderManagementPage = unoptimizedRenderer(renderManagementPageBody);
