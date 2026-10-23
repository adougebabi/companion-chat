import {unoptimizedRenderer} from '../shared/render-runtime.mjs';
import { approvedButtonVNode, approvedButtonBlock, approvedSurfaceVNode, approvedSurfaceBlock } from "../shared/primitives.mjs";
// Render structure extracted from the approved index.html; business behavior is supplied by the page component.
import { openBlock as p, createElementBlock as g, normalizeClass as ge, createElementVNode as o, normalizeStyle as yt, unref as M, toDisplayString as v, createTextVNode as Z, createVNode as me, Fragment as O, renderList as oe, createCommentVNode as X, withDirectives as pe, withKeys as Gi, withModifiers as Ve, vModelText as $e } from 'vue';
import { tc, Qu, Xu, Ji, Tl } from '../shared/icons.mjs';
const bc = {
  class: "chat-panel"
};
const _c = {
  class: "chat-header"
};
const kc = {
  class: "chat-private"
};
const wc = {
  class: "message-body"
};
const $c = {
  class: "message-meta"
};
const Cc = {
  class: "bubble"
};
const xc = {
  key: 0,
  class: "error",
  role: "alert"
};
const Sc = ["onClick"];
const Tc = {
  key: 0,
  class: "typing"
};
const Mc = {
  class: "composer"
};
const Ec = {
  class: "composer-bottom"
};
const Ac = ["disabled"];
const Ic = {
  key: 0,
  class: "preview"
};
const Oc = {
  class: "preview-heading"
};
const Vc = {
  class: "scene"
};
const Pc = {
  class: "scene-caption"
};
const Lc = {
  class: "compact-bio"
};
const Uc = {
  class: "schedule-preview"
};
const Nc = {
  class: "next-item"
};
const Fc = {
  class: "preview-tip"
};
function renderChatPageBody(__view) {
  return (c, d) => {
    var C, w;
    return p(), g("div", {
      class: ge(["chat-layout", {
        collapsed: !__view.props.preview
      }])
    }, [o("section", bc, [o("header", _c, [o("span", {
      class: "avatar",
      style: yt({
        background: M(__view.character).color
      })
    }, v(M(__view.character).initial), 5), o("div", null, [o("strong", null, v(M(__view.character).name), 1), o("small", null, [d[3] || (d[3] = o("i", {
      class: "online-dot"
    }, null, -1)), Z(M(__view.character).status==='正常'?" 运行中 ":M(__view.character).status||" 状态未读取 ",1), o("span", null, "· " + v(M(__view.character).job), 1)])]), d[6] || (d[6] = o("span", {
      class: "spacer"
    }, null, -1)), o("span", kc, [me(M(tc), {
      size: 13
    }), d[5] || (d[5] = Z("只属于你们的对话", -1))])]), o("div", {
      ref_key: "timeline",
      ref: __view.i,
      class: "timeline"
    }, [o("div", {class:"date-divider"}, [o("span", null, v(M(__view.dateLabel)),1)]), d[10] || (d[10] = o("div", {
      class: "conversation-note"
    }, "从一句问候开始，让陪伴慢慢发生。", -1)), (p(!0), g(O, null, oe(M(__view.character).messages, (A, B) => {
      var te;
      return p(), g("div", {
        key: A.id,
        class: ge(["message", {
          mine: A.user
        }])
      }, [o("span", {
        class: "avatar message-avatar",
        style: yt({
          background: A.user ? "#e9eae6" : M(__view.character).color
        })
      }, v(A.user ? "我" : M(__view.character).initial), 5), o("div", wc, [o("div", $c, [Z(v(A.user ? "我" : M(__view.character).name), 1), o("time", {datetime:A.instant,title:A.fullTime,"aria-label":A.fullTime}, v(A.time), 0)]), o("div", Cc, [o("span",null,v(A.text),1),...(A.attachments||[]).map(id=>o("img",{src:__view.mediaUrl(id),alt:M(__view.character).name+" 的附件",loading:"lazy",class:"approved-chat-attachment"}))]), ((te = A.action) == null ? void 0 : te.status) === "error" ? (p(), g("div", xc, [Z(v(A.action.reason), 1), approvedButtonVNode({
        onClick: se => A.action.run()
      }, [me(M(Qu), {
        size: 12
      }), d[7] || (d[7] = Z("重试", -1))])])) : X("", !0)])], 2);
    }), 128)), __view.f.value ? (p(), g("div", Tc, [Z(v(M(__view.character).name) + " 正在输入 ", 1), d[8] || (d[8] = o("span", null, "•••", -1))])) : X("", !0)], 512), o("div", Mc, [pe(o("textarea", {
      "onUpdate:modelValue": d[0] || (d[0] = A => __view.l.value = A),
      "aria-label": "聊天消息",onCompositionstart:()=>__view.composition(true),onCompositionend:()=>__view.composition(false),
      placeholder: "说点什么，分享你的今天…",
      onKeydown: d[1] || (d[1] = Gi(Ve(A => !A.isComposing && __view.u(), ["exact", "prevent"]), ["enter"]))
    }, null, 544), [[$e, __view.l.value]]), o("div", Ec, [d[11] || (d[11] = o("small", null, "Enter 发送 · Shift + Enter 换行", -1)), approvedButtonVNode({
      class: "button primary",
      disabled: !__view.l.value.trim() || __view.f.value,
      onClick: __view.u
    }, [Z(v(__view.f.value ? "回应中" : "发送"), 1), me(M(Xu), {
      size: 14
    })])])])]), __view.props.preview ? (p(), g("aside", Ic, [o("div", Oc, [o("span", null, v(M(__view.character).name + "的此刻"), 1), d[12] || (d[12] = o("span", {
      class: "pill"
    }, "实时预览", -1))]), o("div", Vc, [d[14] || (d[14] = o("div", {
      class: "scene-window"
    }, [o("i"), o("i")], -1)), d[15] || (d[15] = o("div", {
      class: "scene-sun"
    }, null, -1)), d[16] || (d[16] = o("div", {
      class: "scene-vase"
    }, "♧", -1)), d[17] || (d[17] = o("div", {
      class: "scene-desk"
    }, null, -1)), o("div", Pc, [d[13] || (d[13] = o("span", {
      class: "scene-tag"
    }, "A LITTLE MOMENT", -1)), o("p", null, v(M(__view.character).location || M(__view.character).activity || "此刻"), 1)])]), o("div", Lc, [o("b", null, v(M(__view.character).name) + " · " + v(M(__view.character).job), 1), o("p", null, v(M(__view.character).bio), 1), o("small", null, v(M(__view.character).mood) + " · " + v(M(__view.character).location), 1)]), me(__view.CurrentOutfit), o("div", Uc, [o("div", null, [d[19] || (d[19] = o("span", null, "接下来的安排", -1)), approvedButtonVNode({
      onClick: d[2] || (d[2] = A => __view.n("go", "日程"))
    }, [d[18] || (d[18] = Z("查看全部 ", -1)), me(M(Ji), {
      size: 12
    })])]), M(__view.currentSchedule) ? o("div", Nc, [d[21] || (d[21] = o("span", {
      class: "schedule-dot"
    }, null, -1)), o("div", null, [o("small", null, v((C = M(__view.currentSchedule)) == null ? void 0 : C.time) + " — " + v(M(__view.currentSchedule)?.endTime||''), 1), o("p", null, v((w = M(__view.currentSchedule)) == null ? void 0 : w.text), 1), o("span", {class:"pill"},v(M(__view.currentSchedule)?.stateLabel||''),1)])]) : X("",!0)]), o("div", Fc, [me(M(Tl), {
      size: 16
    }), o("p", null, [d[22] || (d[22] = Z("生活在继续，故事也在慢慢生长。", -1)), d[23] || (d[23] = o("br", null, null, -1)), Z("这里是 " + v(M(__view.character).name) + " 的小小世界。", 1)])])])) : X("", !0)], 2);
  };
}

export const renderChatPage = unoptimizedRenderer(renderChatPageBody);
