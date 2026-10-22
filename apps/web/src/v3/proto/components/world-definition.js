import {Field} from './forms.js';
const count=value=>[...String(value||'')].length;
const object=value=>value!==null&&typeof value==='object'&&!Array.isArray(value);
export const newWorldDefinition=()=>({background:'',rules:''});
// Matches Core validateWorldFramework; absence remains compatible with legacy shells.
export function worldDefinitionError(body){
 if(!object(body))return '世界定义须为对象';
 if(!Object.entries(body).some(([key,value])=>key!=='framework'&&((typeof value==='string'&&value.trim())||(object(value)&&Object.keys(value).length)))&&!body.framework)return '请填写世界背景或规则，不能保存或发布空世界底稿';
 const frame=body.framework;if(frame===undefined||frame===null)return '';
 if(!object(frame)||frame.schema!=='world-framework.v1')return '场地框架须使用 world-framework.v1';
 if(!Array.isArray(frame.venues)||!frame.venues.length||frame.venues.length>32)return '场地框架需要 1–32 个分类';
 const categories=new Set(),places=new Set();let total=0;
 for(const venue of frame.venues){
  const key=String(venue?.key||'').trim(),name=String(venue?.name||'').trim();
  if(typeof venue?.key!=='string'||typeof venue?.name!=='string'||!key||!name||count(key)>80||count(name)>128||categories.has(key))return '每个场地分类需要唯一的局部键和名称（分别最多80、128字）';categories.add(key);
  if(!Array.isArray(venue.places))return '分类的具体地点须为列表';
  for(const place of venue.places){
   const point=String(place?.key||'').trim(),label=String(place?.name||'').trim(),identity=JSON.stringify([key,point]);
   if(typeof place?.key!=='string'||typeof place?.name!=='string'||!point||!label||count(point)>80||count(label)>256||places.has(identity))return '每个具体地点需要分类内唯一的局部键和名称（分别最多80、256字）';places.add(identity);total++;
   for(const [field,limit] of Object.entries({description:8000,alias:256,floor:128,atmosphere:2000,layout:8000}))if(place[field]!=null&&(typeof place[field]!=='string'||count(place[field].trim())>limit))return `地点的${{description:'描述',alias:'别名',floor:'楼层',atmosphere:'氛围',layout:'布局'}[field]}格式或长度不符合要求`;
   if(place.furniture!=null&&!(typeof place.furniture==='string'?count(place.furniture)<=16000:Array.isArray(place.furniture)&&place.furniture.length<=200&&new TextEncoder().encode(JSON.stringify(place.furniture)).length<=32000))return '固定陈设须为不超过16000字的文字，或不超过200项、32000字节的列表';
   if(['id','place_id','world_id','parent_place_id','connection_place_ids','npc_id'].some(field=>place[field]!=null))return '底稿地点使用局部键，请移除运行世界ID';
  }
 }
 return total<1||total>200?'场地框架需要 1–200 个具体地点':'';
}
export function WorldDefinitionFields(body){
 return `${Field({label:'世界背景',name:'worldBackground',value:body.background||'',type:'textarea',hint:'描述你希望人物生活的世界；不会从已有角色的运行世界复制。',attrs:{rows:4,'data-world-text':'background'}})}${Field({label:'世界规则',name:'worldRules',value:body.rules||'',type:'textarea',attrs:{rows:3,'data-world-text':'rules'}})}<p data-world-validation role="alert" class="banner err" hidden></p><section class="stack"><h3>场地与具体地点</h3><p class="small muted">可先保存背景与规则。添加框架时需填写真实的场地分类与至少一个具体地点；局部键属于底稿，不是运行世界ID。</p>${(Array.isArray(body.framework?.venues)?body.framework.venues:[]).map((venue,i)=>`<fieldset class="card stack"><legend>场地分类 ${i+1}</legend>${['key','name'].map(field=>Field({label:field==='key'?'分类局部键':'分类名称',value:venue[field]||'',attrs:{'data-venue':i,'data-world-field':field,maxlength:field==='key'?80:128}})).join('')}${(Array.isArray(venue?.places)?venue.places:[]).map((place,j)=>`<fieldset class="stack"><legend>具体地点 ${j+1}</legend>${['key','name','description','floor','atmosphere','layout'].map(field=>Field({label:{key:'地点局部键',name:'地点名称',description:'地点描述',floor:'楼层',atmosphere:'氛围',layout:'布局'}[field],value:place[field]||'',attrs:{'data-venue':i,'data-place':j,'data-world-field':field}})).join('')}<button type="button" class="btn" data-remove-place="${i}:${j}">移除此地点</button></fieldset>`).join('')}<button type="button" class="btn" data-add-place="${i}">添加具体地点</button><button type="button" class="btn" data-remove-venue="${i}">移除此分类</button></fieldset>`).join('')}<button type="button" class="btn" data-add-venue>添加场地分类</button></section>`;
}
export function bindWorldDefinition(box,body,{changed,repaint}){
 box.querySelectorAll('[data-world-text]').forEach(input=>input.oninput=()=>{body[input.dataset.worldText]=input.value;const notice=box.querySelector('[data-world-validation]');notice.hidden=true;notice.textContent='';changed();});
 box.querySelectorAll('[data-world-field]').forEach(input=>input.oninput=()=>{const venue=body.framework.venues[Number(input.dataset.venue)],target=input.dataset.place===undefined?venue:venue.places[Number(input.dataset.place)];target[input.dataset.worldField]=input.value;changed();});
 box.querySelector('[data-add-venue]').onclick=()=>{body.framework??={schema:'world-framework.v1',venues:[]};if(!object(body.framework)||!Array.isArray(body.framework.venues))return;body.framework.venues.push({key:'',name:'',places:[]});changed();repaint();};
 box.querySelectorAll('[data-add-place]').forEach(button=>button.onclick=()=>{body.framework.venues[Number(button.dataset.addPlace)].places.push({key:'',name:''});changed();repaint();});
 box.querySelectorAll('[data-remove-venue]').forEach(button=>button.onclick=()=>{body.framework.venues.splice(Number(button.dataset.removeVenue),1);changed();repaint();});
 box.querySelectorAll('[data-remove-place]').forEach(button=>button.onclick=()=>{const [i,j]=button.dataset.removePlace.split(':').map(Number);body.framework.venues[i].places.splice(j,1);changed();repaint();});
}
