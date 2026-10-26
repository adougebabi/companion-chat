import {Field} from './forms.js';
export const newPlaceDefinition=()=>({description:'',alias:'',floor:'',atmosphere:'',furniture:'',layout:''});
const fields=[['description','地点描述',8000],['alias','别名',256],['group','场地分组',256],['floor','楼层',128],['atmosphere','氛围',2000],['furniture','固定陈设',16000],['layout','布局描述',8000]];
export function PlaceDefinitionFields(body){return `<p class="small muted">描述一个具体地点。保存和发布后，读取应用差异并确认，才会在目标角色的运行世界中创建地点。楼层未填写时保持未知。</p>${fields.map(([name,label,maxlength])=>Field({name,label,value:typeof body[name]==='string'?body[name]:'',type:['description','layout','furniture'].includes(name)?'textarea':'text',attrs:{maxlength}})).join('')}<details data-domain-ui="disclosure"><summary>高级定义 JSON</summary>${Field({label:'完整地点定义',name:'placeDefinitionJson',type:'textarea',rows:10,value:JSON.stringify(body,null,2)})}</details>`;}
export function bindPlaceDefinition(box,body,{changed,repaint}){
 const raw=box.querySelector('[name=placeDefinitionJson]');
 for(const [key]of fields)box.querySelector(`[name=${key}]`).oninput=e=>{body[key]=e.target.value;raw.value=JSON.stringify(body,null,2);raw.setCustomValidity('');changed();};
 raw.onchange=e=>{try{const parsed=JSON.parse(e.target.value);if(!parsed||typeof parsed!=='object'||Array.isArray(parsed))throw Error('地点定义须为 JSON 对象');for(const key of Object.keys(body))delete body[key];Object.assign(body,parsed);e.target.setCustomValidity('');changed();repaint();}catch(error){e.target.setCustomValidity(error.message);}};
}
