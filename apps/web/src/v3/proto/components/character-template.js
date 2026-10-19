import {Field} from './forms.js';

export const newCharacterTemplate=()=>({schema:'character-template.v1',description:''});
export function CharacterTemplateFields(body){
 const mode=body.initialization?'initialization':'description';
 return `<p class="small muted">角色模板保存可复用定义，可用于创建角色，或审阅后向已有角色提出Foundation变更。不会复制分析凭证、对话、记忆或运行状态。</p>${Field({label:'模板输入类型',name:'templateMode',type:'select',value:mode,options:[{value:'description',label:'完整角色描述'},{value:'initialization',label:'结构化初始化定义'}]})}${mode==='description'?Field({label:'完整角色描述',name:'templateDescription',value:body.description||'',type:'textarea',rows:12,attrs:{required:true}}):Field({label:'初始化 JSON（包含 core_persona）',name:'templateInitialization',value:JSON.stringify(body.initialization,null,2),type:'textarea',rows:16,attrs:{required:true},hint:'不包含 analysis_id、Actor 背景、当前状态或运行存档；发布时校验。'})}`;
}
export function bindCharacterTemplate(box,body,{changed,repaint}){
 box.querySelector('[name=templateMode]').onchange=e=>{
  body.schema='character-template.v1';
  if(e.target.value==='description'){delete body.initialization;body.description='';}
  else {delete body.description;body.initialization={core_persona:{}};}
  changed();repaint();
 };
 box.querySelector('[name=templateDescription]')?.addEventListener('input',e=>{body.schema='character-template.v1';body.description=e.target.value;changed();});
 box.querySelector('[name=templateInitialization]')?.addEventListener('input',e=>{
  try{const value=JSON.parse(e.target.value);if(!value||typeof value!=='object'||Array.isArray(value))throw new Error('初始化定义必须是 JSON 对象');body.schema='character-template.v1';body.initialization=value;e.target.setCustomValidity('');changed();}
  catch(error){e.target.setCustomValidity(error.message);}
 });
}
