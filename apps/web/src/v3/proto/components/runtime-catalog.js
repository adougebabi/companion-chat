import {esc} from './controls.js';
import {JsonDetails} from './records.js';
import {domainName} from '../../domain-presentation.mjs';
import {catalogLabel} from '../../runtime-catalog.mjs';
export function RuntimeCatalog(catalog){
 const row=(title,meta,description,source,name)=>`<details data-domain-ui="disclosure" class="catalog-entry" data-catalog-index="${esc([title,meta,description,name].join(' ').toLowerCase())}"><summary><b>${esc(title)}</b>${meta&&meta!==title?` <span class="small muted">${esc(meta)}</span>`:''}</summary>${description?`<p>${esc(description)}</p>`:''}${JsonDetails(source,'完整定义与执行边界')}</details>`;
 return `<div class="settings-page stack"><p>这里只查看系统已注册的能力。实际可用范围仍由角色权限与运行配置决定。</p><input data-domain-ui="field" class="input" type="search" data-catalog-search aria-label="搜索能力目录" placeholder="搜索能力名称、标识或说明…"><p class="small muted" data-catalog-count role="status">${catalog.agents.length} 项任务处理 · ${catalog.capabilities.length} 项工具</p><section data-domain-ui="card" class="card" data-catalog-section><h3>Agent · 任务处理</h3>${catalog.agents.map(a=>row(catalogLabel('agent',a.name,a.description),domainName(a.role,'其他已注册任务'),a.description,a,a.name)).join('')||'<p class="small muted">尚未读取到已注册的 Agent。</p>'}</section><section data-domain-ui="card" class="card" data-catalog-section><h3>Tool · 可用工具</h3>${catalog.capabilities.map(c=>row(catalogLabel('tool',c.name,c.description),[({query:'查询',action:'行动',internal:'内部能力'})[c.type]||'类别待确认',c.version?`版本 ${c.version}`:''].filter(Boolean).join(' · '),c.description,c.sourceContract||c,c.name)).join('')||'<p class="small muted">尚未读取到已注册工具。</p>'}</section><p class="small muted">本目录只读。审核角色的能力请求，不会安装代码或扩大系统授权。</p></div>`;
}
export function bindRuntimeCatalog(host){
 const search=host.querySelector('[data-catalog-search]');
 search.oninput=()=>{const query=search.value.trim().toLowerCase();let count=0;host.querySelectorAll('[data-catalog-index]').forEach(row=>{row.hidden=!row.dataset.catalogIndex.includes(query);if(!row.hidden)count++;});host.querySelectorAll('[data-catalog-section]').forEach(section=>{section.hidden=!!query&&![...section.querySelectorAll('[data-catalog-index]')].some(row=>!row.hidden);});host.querySelector('[data-catalog-count]').textContent=query?count?`找到 ${count} 项匹配能力`:'没有匹配的能力，可以尝试任务名称或工具标识。':`共 ${count} 项已注册能力`;};
}
