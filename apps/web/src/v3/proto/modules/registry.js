// 模块注册表：各分区 js/modules/<分区>.js 调用 register() 挂页面
import {workspaceForPath} from '../../workspace-navigation.mjs';
const routes = [];
/**
 * register({route, key, title, bp, zone, coverage, iface, source, scope, render(el, ctx), renderState?(el, ctx), manualRefresh?})
 *  route    '/c/:charId/life/now' 形式，:param 进 ctx.params
 *  coverage 可操作｜仅展示｜占位｜未完成
 */
export function register(def) {
  if (!def.route || !def.key || typeof def.render !== 'function') throw new Error('register: 缺 route/key/render');
  const names = []; const re = new RegExp('^' + def.route.replace(/:(\w+)/g, (_, n) => { names.push(n); return '([^/]+)'; }) + '/?$');
  const i = routes.findIndex(r => r.route === def.route);
  const entry = { coverage: '占位', zone: '', bp: '', scope: '角色范围', workspace:workspaceForPath(def.route), ...def, re, names };
  if (i >= 0) routes[i] = entry; else routes.push(entry);
  return entry;
}
export function match(path) {
  for (const r of routes) { const m = path.match(r.re); if (m) { const params = {}; r.names.forEach((n, i) => params[n] = decodeURIComponent(m[i + 1])); return { def: r, params }; } }
  return null;
}
export const allRoutes = () => routes.slice();
/** 占位页工厂：命名归属分区；可选 extra(el, ctx) 追加少量只读内容 */
export function placeholder({ route, key, title, bp, zone, coverage = '占位', note = '', scope, extra }) {
  return register({ route, key, title, bp, zone, coverage, scope, iface: [['页面', '待核验']], source: note || '主稿 v1.0 附录',
    render(el, ctx) {
      el.innerHTML = `<div class="placeholder"><div class="card"><div style="font-size:42px">🚧</div><h2>${coverage === '未完成' ? '未完成' : '占位页面'}：${title}</h2>
      <p class="muted">page key <code>${key}</code> · ${bp} · 归属分区 <b>${zone}</b></p><p class="small muted">${note || '该页由对应分区在 js/modules/' + zone + '.js 中实现。'}</p><div data-extra></div>
      <div class="row" style="justify-content:center;margin-top:12px"><button class="btn" data-back>返回</button><a class="btn primary" href="#/c/${ctx.charId}/chat">回到对话</a></div></div></div>`;
      if (extra) extra(el.querySelector('[data-extra]'), ctx);
    } });
}
