import {esc} from './controls.js';
import {JsonDetails} from './records.js';
import {domainName} from '../../domain-presentation.mjs';
export function RuntimeCatalog(catalog){
 const row=(title,meta,description,source)=>`<details class="catalog-entry"><summary><b>${esc(title)}</b>${meta&&meta!==title?` <span class="small muted">${esc(meta)}</span>`:''}</summary>${description?`<p>${esc(description)}</p>`:''}${JsonDetails(source,'完整定义与执行边界')}</details>`;
 return `<div class="settings-page stack"><p>这里只查看系统已注册的能力。实际可用范围仍由角色权限与运行配置决定。</p><section class="card stack"><h3>Agent · 任务处理</h3>${catalog.agents.map(a=>row(a.name||domainName(a.role,'已注册 Agent'),domainName(a.role,'角色尚未分类'),a.description,a)).join('')||'<p class="small muted">尚未读取到已注册的 Agent。</p>'}</section><section class="card stack"><h3>Tool · 可用工具</h3>${catalog.capabilities.map(c=>row((typeof c.description==='string'&&c.description.trim()?c.description.trim().split(/[。\n]/)[0].slice(0,80):c.name)||'名称未提供的能力',[({query:'查询',action:'行动',internal:'内部能力'})[c.type]||'类别待确认',c.version?`版本 ${c.version}`:''].filter(Boolean).join(' · '),c.description,c.sourceContract||c)).join('')||'<p class="small muted">尚未读取到已注册工具。</p>'}</section><p class="small muted">本目录只读。审核角色的能力请求，不会安装代码或扩大系统授权。</p></div>`;
}
