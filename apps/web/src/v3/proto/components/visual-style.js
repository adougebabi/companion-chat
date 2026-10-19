import {Field} from './forms.js';
import {visualBriefFields} from './visual-identity-records.js';
export const newVisualStyle=()=>({schema:'visual-style.v1',...Object.fromEntries(visualBriefFields.map(([key])=>[key,'']))});
export function VisualStyleFields(body){return `<p class="small muted">视觉风格是未来生成会话的参考。应用只保存目标角色的参考版本，不修改当前身体、穿着、规范图或全局工作流。</p>${visualBriefFields.map(([name,label,hint])=>Field({name,label,hint,value:body[name]??'',type:'textarea',attrs:{maxlength:1000}})).join('')}<details><summary>完整定义 JSON / 旧草稿迁移</summary>${Field({label:'完整视觉风格定义',name:'visualStyleJson',value:JSON.stringify(body,null,2),type:'textarea',rows:10,hint:'发布要求 visual-style.v1 与四个显式文本字段。旧字段由你明确修订，不会自动删除。'})}</details>`;}
export function bindVisualStyle(box,body,{changed}){
 const raw=box.querySelector('[name=visualStyleJson]');
 for(const [name]of visualBriefFields)box.querySelector(`[name=${name}]`).oninput=e=>{body.schema='visual-style.v1';body[name]=e.target.value;raw.value=JSON.stringify(body,null,2);raw.setCustomValidity('');changed();};
 raw.oninput=e=>{try{const value=JSON.parse(e.target.value);if(!value||typeof value!=='object'||Array.isArray(value))throw new Error('视觉风格定义必须是JSON对象');for(const key of Object.keys(body))delete body[key];Object.assign(body,value);for(const [name]of visualBriefFields)box.querySelector(`[name=${name}]`).value=typeof body[name]==='string'?body[name]:'';e.target.setCustomValidity('');changed();}catch(error){e.target.setCustomValidity(error.message);}};
}
