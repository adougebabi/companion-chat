import { esc, icon, badge } from './controls.js';
let layer = [];
function closeTop() { const t = layer.pop(); if (t) t.close(true); }
document.addEventListener('keydown', e => { if (e.key === 'Escape' && layer.length) { e.preventDefault(); closeTop(); } });

function overlay(kind, { title, body = '', footer = '', onClose, wide } = {}) {
  const trigger = document.activeElement; let closed = false;
  const scrim = document.createElement('div'); scrim.className = 'scrim';
  const box = document.createElement('div'); box.className = kind; box.setAttribute('role', 'dialog'); box.setAttribute('aria-modal', 'true');
  if (wide) box.style.width = 'min(720px,calc(100vw - 24px))';
  box.innerHTML = `<header><h2>${title}</h2><button class="btn ghost icon-only" data-x aria-label="关闭">${icon('close')}</button></header><div class="body">${body}</div>${footer ? `<footer>${footer}</footer>` : ''}`;
  document.body.append(scrim, box);
  const api = {
    el: box, body: box.querySelector('.body'),
    close(fromEsc) { if (closed) return; closed = true; scrim.remove(); box.remove(); layer = layer.filter(x => x !== api); if (trigger?.isConnected) trigger.focus(); onClose?.(fromEsc); },
    setBody(html) { api.body.innerHTML = html; },
    setFooter(html) { let f = box.querySelector('footer'); if (!f) { f = document.createElement('footer'); box.append(f); } f.innerHTML = html; },
  };
  scrim.onclick = () => api.close(); box.querySelector('[data-x]').onclick = () => api.close();
  layer.push(api);
  setTimeout(() => box.querySelector('[data-x]')?.focus(), 0);
  return api;
}
/** 右侧抽屉：返回 {el, body, close, setBody, setFooter}，Esc/遮罩/× 可关 */
export const drawer = opts => overlay('drawer', opts);
/** 居中弹窗 */
export const modal = opts => overlay('modal', opts);
/** 二次确认：返回 Promise<boolean> */
export function confirm({ title = '确认', text, okText = '确认', identity = '所有者修改' }) {
  return new Promise(res => {
    let done = false;
    const m = modal({ title: `${badge(identity)} ${esc(title)}`, body: `<p>${text}</p><p class="small muted">${identity === '所有者修改' ? '此操作将记入运行诊断 · 操作审计（diag.audit）。' : ''}</p>`,
      footer: `<button class="btn" data-no>取消</button><button class="btn accent" data-ok data-test="confirm-ok">${esc(okText)}</button>`, onClose: () => { if (!done) res(false); } });
    m.el.querySelector('[data-no]').onclick = () => m.close();
    m.el.querySelector('[data-ok]').onclick = () => { done = true; m.close(); res(true); };
  });
}

export function closeOverlays() { for (const item of [...layer].reverse()) item.close(); }
