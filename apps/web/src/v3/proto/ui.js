// Compatibility facade: pages compose shared components; state stays in the Mock store.
import * as store from './store.js';
import { confirm } from './components/overlay.js';
export * from './components/controls.js';
export * from './components/overlay.js';
export * from './components/toast.js';
export * from './components/states.js';
export * from './components/sources.js';
export const $ = (selector, root = document) => root.querySelector(selector);
export const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
/** 所有者操作前确保处于“管理”身份 */
export async function requireOwner() {
  if (store.get('ui').identity === '管理') return true;
  const ok = await confirm({ title: '切换到管理身份', text: '当前为“交流”身份。所有者修改需要切换到“管理”身份后执行。', okText: '切换并继续', identity: '所有者修改' });
  if (ok) store.setUi({ identity: '管理' });
  return ok;
}
