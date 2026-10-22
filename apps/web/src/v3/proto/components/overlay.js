import { esc, icon, badge } from './controls.js';
let layer = [];
let overlaySequence = 0, previousOverflow;
const backgroundInert = new Map();
const focusableSelector = 'a[href],button,input,select,textarea,summary,[tabindex]';
function focusable(box) {
  return [...box.querySelectorAll(focusableSelector)].filter(node => !node.disabled && node.tabIndex >= 0 && !node.closest('[hidden],[inert]') && node.getClientRects().length);
}
function syncLayers() {
  const top = layer.at(-1);
  if (!top) {
    for (const [node, inert] of backgroundInert) node.inert = inert;
    backgroundInert.clear();
    document.body.style.overflow = previousOverflow ?? '';
    return;
  }
  for (const node of document.body.children) {
    if (!backgroundInert.has(node)) backgroundInert.set(node, node.inert);
    node.inert = node !== top.el && node !== top.scrim;
  }
  layer.forEach((item, index) => { item.scrim.style.zIndex = 50 + index * 2; item.el.style.zIndex = 51 + index * 2; });
}
function closeTop() { const t = layer.pop(); if (t) t.close(true); }
document.addEventListener('keydown', e => {
  if (!layer.length) return;
  if (e.key === 'Escape') { e.preventDefault(); closeTop(); }
  else if (e.key === 'Tab') {
    const top = layer.at(-1), controls = focusable(top.el), first = controls[0], last = controls.at(-1);
    if (!first) { e.preventDefault(); top.el.focus(); }
    else if (!top.el.contains(document.activeElement) || (e.shiftKey && document.activeElement === first) || (!e.shiftKey && document.activeElement === last)) {
      e.preventDefault(); (e.shiftKey ? last : first).focus();
    }
  }
});

function overlay(kind, { title, body = '', footer = '', onClose, wide } = {}) {
  const trigger = document.activeElement; let closed = false;
  const scrim = document.createElement('div'); scrim.className = 'scrim';
  const box = document.createElement('div'); box.className = kind; box.setAttribute('role', 'dialog'); box.setAttribute('aria-modal', 'true'); box.tabIndex = -1;
  const titleId = `overlay-title-${++overlaySequence}`; box.setAttribute('aria-labelledby', titleId);
  if (wide) box.style.width = 'min(720px,calc(100vw - 24px))';
  box.innerHTML = `<header><h2 id="${titleId}">${title}</h2><button class="btn ghost icon-only" type="button" data-x aria-label="关闭">${icon('close')}</button></header><div class="body">${body}</div>${footer ? `<footer>${footer}</footer>` : ''}`;
  if (!layer.length) { previousOverflow = document.body.style.overflow; document.body.style.overflow = 'hidden'; }
  document.body.append(scrim, box);
  const api = {
    el: box, scrim, body: box.querySelector('.body'),
    close(fromEsc) { if (closed) return; closed = true; scrim.remove(); box.remove(); layer = layer.filter(x => x !== api); syncLayers();
      const remainingTop=layer.at(-1);
      const restoreFocus=()=>{if(trigger?.isConnected&&!trigger.disabled&&!trigger.closest('[inert]')&&layer.at(-1)===remainingTop&&(document.activeElement===document.body||document.activeElement===trigger))trigger.focus({preventScroll:true});};
      restoreFocus();onClose?.(fromEsc);
      // Async owners re-enable their trigger after the confirmation resolves.
      requestAnimationFrame(restoreFocus); },
    setBody(html) { api.body.innerHTML = html; },
    setFooter(html) { let f = box.querySelector('footer'); if (!f) { f = document.createElement('footer'); box.append(f); } f.innerHTML = html; },
  };
  scrim.onclick = () => api.close(); box.querySelector('[data-x]').onclick = () => api.close();
  layer.push(api);
  syncLayers();
  setTimeout(() => { if (!closed && layer.at(-1) === api) box.querySelector('[data-x]')?.focus({preventScroll:true}); }, 0);
  return api;
}
/** 右侧抽屉：返回 {el, body, close, setBody, setFooter}，Esc/遮罩/× 可关 */
export const drawer = opts => overlay('drawer', opts);
/** 居中弹窗 */
export const modal = opts => overlay('modal', opts);
/** 二次确认：返回 Promise<boolean> */
export function confirm({ title = '确认', text, okText = '确认', identity = '所有者修改', type = 'default' }) {
  return new Promise(res => {
    let done = false;
    const m = modal({ title: `${badge(identity)} ${esc(title)}`, body: `<p>${text}</p><p class="small muted">${identity === '所有者修改' ? '此操作将记入运行诊断中的操作审计。' : ''}</p>`,
      footer: `<button class="btn" data-no>取消</button><button class="btn ${type==='danger'?'danger':'accent'}" data-ok data-test="confirm-ok">${esc(okText)}</button>`, onClose: () => { if (!done) res(false); } });
    m.el.querySelector('[data-no]').onclick = () => m.close();
    m.el.querySelector('[data-ok]').onclick = () => { done = true; m.close(); res(true); };
  });
}

export function closeOverlays() { for (const item of [...layer].reverse()) item.close(); }
