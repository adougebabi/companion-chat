import {stateLabel} from '../../domain-presentation.mjs';
import { esc, badge } from './controls.js';
import { attributes } from './forms.js';

export function RecordList({ rows, fields, action, attrs = {} }) {
  return `<div data-domain-ui="card" class="card" ${attributes(attrs)}><div data-domain-ui="item" class="lrow small muted">${fields.map(field => `<span class="grow">${esc(field.label)}</span>`).join('')}<span>操作</span></div>${rows.map(row => `<div data-domain-ui="item" class="lrow">${fields.map(field => `<span class="grow">${field.render ? field.render(row) : esc(row[field.key])}</span>`).join('')}${action ? action(row) : ''}</div>`).join('') || '<p class="small muted">当前筛选没有记录。</p>'}</div>`;
}
function readableRecord(value,field=""){
 if(value==null||value===''||typeof value==='object'&&!Object.keys(value).length)return '<p class="small muted">尚未记录相关信息。</p>';
 if(typeof value!=='object')return `<p class="record-value">${esc(typeof value==='boolean'?(value?'是':'否'):field==='status'?stateLabel(value,'状态待确认'):String(value))}</p>`;
 if(Array.isArray(value))return `<ul class="stack">${value.map(item=>`<li>${readableRecord(item)}</li>`).join('')}</ul>`;
 const labels={status:'状态',id:'标识',name:'名称',description:'描述',source:'来源',revision:'版本',created_at:'记录时间',reason:'原因',summary:'摘要',text:'正文',content:'内容',sha256:'完整性校验值',asset_id:'素材标识',source_snapshot:'来源快照'};
 return `<dl class="record-facts">${Object.entries(value).map(([key,item])=>`<div><dt>${esc(labels[key]||key.replaceAll('_',' '))}</dt><dd>${readableRecord(item,key)}</dd></div>`).join('')}</dl>`;
}
export function JsonDetails(value,label='查看完整内容'){return `<details data-domain-ui="disclosure" class="mgmt small muted"><summary>${esc(label)}</summary>${readableRecord(value)}</details>`;}

export function RecordDetails(rows,{empty='尚无记录'}={}) {
  return rows.length?rows.map(row=>`<details data-domain-ui="disclosure" class="lrow block"><summary>${esc(row.reason || row.title || row.status || row.id || row.operation || '来源记录')}</summary><pre class="[white-space:pre-wrap] [overflow-wrap:anywhere]">${esc(JSON.stringify(row,null,2))}</pre></details>`).join(''):`<p class="small muted">${esc(empty)}</p>`;
}
