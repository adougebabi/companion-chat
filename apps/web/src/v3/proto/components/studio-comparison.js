import {esc} from '../ui.js';
import {DiffRows} from './forms.js';
import {semanticLabel} from './semantic-fields.js';
const encoded=value=>value===undefined?'未设置':typeof value==='string'?value:JSON.stringify(value,null,2);
const object=value=>value!==null&&typeof value==='object'&&!Array.isArray(value);
export function studioChanges(draft,proposal){
 const changed=[],same=[];
 function visit(before,after,path,label){
  if(JSON.stringify(before)===JSON.stringify(after)){same.push({path,label,value:after});return;}
  if(object(before)&&object(after)){
   for(const key of new Set([...Object.keys(before),...Object.keys(after)]))visit(before[key],after[key],path+'.'+key,label+' / '+(semanticLabel(key)==='自定义字段'?key:semanticLabel(key)));
  }else changed.push({field:label,path,before:encoded(before),after:encoded(after)});
 }
 visit(draft.name,proposal.name,'name','名称');visit(draft.body,proposal.body,'body','定义');
 return {changed,same};
}
export function StudioComparison(draft,proposal){
 const {changed,same}=studioChanges(draft,proposal);
 return `<section class="studio-comparison" data-comparison><div class="row"><h3 class="grow">本次改变 · ${changed.length} 项</h3><button data-domain-ui="button" class="btn accent" data-adopt ${changed.length?'':'disabled'}>采用完整建议到草稿</button></div><p class="small muted">与此刻工作草稿比较，包含生成期间的手动修改。采用后仍需单独保存。</p>${DiffRows(changed,'建议与当前草稿一致')}<details data-domain-ui="disclosure" data-compare-detail="unchanged"><summary>未改内容 · ${same.length} 项</summary>${same.map(row=>`<div class="comparison-unchanged"><b>${esc(row.label)}</b><pre>${esc(encoded(row.value))}</pre></div>`).join('')}</details><details data-domain-ui="disclosure" data-compare-detail="full"><summary>完整名称与定义</summary><div class="grid g2"><div><h4>当前工作草稿</h4><pre>${esc(JSON.stringify({name:draft.name,body:draft.body},null,2))}</pre></div><div><h4>完整生成建议</h4><pre>${esc(JSON.stringify({name:proposal.name,body:proposal.body},null,2))}</pre></div></div></details></section>`;
}
