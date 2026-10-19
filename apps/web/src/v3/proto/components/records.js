import { esc, badge } from './controls.js';
import { attributes } from './forms.js';

export function RecordList({ rows, fields, action, attrs = {} }) {
  return `<div class="card" ${attributes(attrs)}><div class="lrow small muted">${fields.map(field => `<span class="grow">${esc(field.label)}</span>`).join('')}<span>操作</span></div>${rows.map(row => `<div class="lrow">${fields.map(field => `<span class="grow">${field.render ? field.render(row) : esc(row[field.key])}</span>`).join('')}${action ? action(row) : ''}</div>`).join('') || '<p class="small muted">当前筛选没有记录。</p>'}</div>`;
}
export function JsonDetails(value, label = '查看完整内容') {
  return `<details class="mgmt small muted"><summary>${esc(label)}</summary><pre style="white-space:pre-wrap;overflow-wrap:anywhere;max-height:360px;overflow:auto">${esc(JSON.stringify(value, null, 2))}</pre></details>`;
}

export function RecordDetails(rows,{empty='尚无记录'}={}) {
  return rows.length?rows.map(row=>`<details class="lrow" style="display:block"><summary>${esc(row.reason || row.title || row.status || row.id || row.operation || '来源记录')}</summary><pre style="white-space:pre-wrap;overflow-wrap:anywhere">${esc(JSON.stringify(row,null,2))}</pre></details>`).join(''):`<p class="small muted">${esc(empty)}</p>`;
}
