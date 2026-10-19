import { icon } from './controls.js';

/** Exact v3 structural shell. Controllers attach events without owning markup. */
export function ApplicationShell() {
  return `
  <header class="topbar">
    <a class="brand" href="#/library" title="角色库"><span class="brand-dot"></span><span class="hide-n">摇光</span></a>
    <button class="charsw" data-test="charsw" id="charsw"></button>
    <div class="scopebar" id="scopebar"></div>
    <span class="spacer"></span>
    <button class="btn ghost sm hide-n" id="demoBtn" data-test="demo-menu">${icon('route')} 演示流程</button>
    <label class="toggle hide-n" title="显示每页 page key / BP / 接口状态"><input type="checkbox" id="notesT" data-test="notes-toggle"> 原型说明</label>
    <a class="btn ghost icon-only bell" href="#/inbox" aria-label="通知" data-test="bell">${icon('bell')}<span class="count" id="bellc"></span></a>
    <button class="btn ghost icon-only" id="moreBtn" aria-label="更多">${icon('more')}</button>
  </header>
  <div class="layout"><nav class="sidenav" id="sidenav" aria-label="角色工作区导航"></nav><main class="main" id="main"></main></div>
  <nav class="bottombar" id="bottombar" aria-label="底部导航"></nav>`;
}
