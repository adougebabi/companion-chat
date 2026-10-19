import {Field,Toggle} from './forms.js';
import {esc} from './controls.js';
import {parseCreationAnalysis} from '../../creation-json.mjs';

export function CreationPreview(draft){
 if(!draft.analysis)return '<p>空白初始化，无虚构经历与初始目标。</p>';
 const goals=draft.analysis.initial_goals;
 return `<div class="stack"><h3>初始目标审阅</h3>${goals.map((goal,i)=>`<div class="card"><b>${i+1}. ${esc(goal.description||goal.title||'目标')}</b>${goal.profile_id?`<p class="small muted">人格范围：${esc(goal.profile_id)}</p>`:''}<pre style="white-space:pre-wrap;overflow-wrap:anywhere">${esc(JSON.stringify(goal,null,2))}</pre></div>`).join('')||'<p class="small muted">分析来源没有初始目标。激活不会补造目标。</p>'}${Toggle({label:'应用预览中的初始目标与意向（取消后，两者都不导入）',name:'includeGoals',checked:draft.includeGoals})}<details><summary>检查或编辑完整初始化预览</summary>${Field({label:'分析预览 JSON',name:'analysisText',value:draft.analysisText,type:'textarea',rows:14,hint:'保留分析来源与版本。可修改初始资料、目标、意向及关系；正式激活由 Core 校验。'})}</details><button class="btn" type="button" data-export-analysis>导出当前分析预览</button></div>`;
}
export function bindCreationPreview(form,draft,{changed,error}){
 if(!draft.analysis)return;
 form.elements.includeGoals.onchange=e=>{draft.includeGoals=e.target.checked;changed();};
 form.elements.analysisText.oninput=e=>{
  draft.analysisText=e.target.value;
  try{draft.analysis=parseCreationAnalysis(draft.analysisText);e.target.setCustomValidity('');error('');}
  catch(cause){e.target.setCustomValidity(cause.message);error(cause.message);}
  changed();
 };
 form.querySelector('[data-export-analysis]').onclick=()=>{
  try{parseCreationAnalysis(draft.analysisText);}catch(cause){error(cause.message);return;}
  const url=URL.createObjectURL(new Blob([draft.analysisText],{type:'application/json'})),link=document.createElement('a');
  link.href=url;link.download='fluctlight-initialization-preview.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
 };
}
