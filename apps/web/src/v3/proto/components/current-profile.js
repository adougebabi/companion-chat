import {esc,badge} from './controls.js';
import {SemanticFields,SemanticValue,semanticLabel} from './semantic-fields.js';
import {ClothingRecords} from './clothing-records.js';
import {displayTimezone,userInstant} from '../../presentation-time.mjs';
import {stateLabel} from '../../domain-presentation.mjs';
export function CurrentProfile(current,{zone}={}){
 const context=current.context||{},appearance=current.appearance,chosen=displayTimezone(context.timezone,zone),pending=context.source==='pending'||context.authority_status==='pending';
 const mood=current.inner_state?.mood||current.mood;
 const moodText=typeof mood==='string'?mood:typeof mood?.label==='string'?mood.label:null;
 const facts={...(!pending?{scene:context.scene,activity:context.activity||current.activity,location:context.location||current.location}:{}),...(moodText?{mood:moodText}:{})};
 let body=SemanticFields(facts,{empty:pending?'当前活动尚待确认，可以先查看其他资料。':'当前活动与心情尚未记录。'});
 if(mood?.source==='server_default')body+='<p class="small muted">心情采用初始化记录，尚无后续更新。</p>';
 if(appearance===null)return body+'<p class="small muted">有效外观：未记录 / 已清除。不会使用旧初始外观代替。</p>';
 if(appearance&&typeof appearance==='object'){
  const fields=['hair_length','hair_color','hair_style','injuries','chest_cup'].filter(key=>Object.hasOwn(appearance.body_fields||{},key)).map(key=>[key,appearance.body_fields[key]]);
  if(fields.length)body+=`<h4>当前外观</h4><dl class="profile-facts">${fields.map(([key,record])=>`<div><dt>${esc(semanticLabel(key))}</dt><dd>${record?.status==='known'&&record.value!=null?SemanticValue(record.value,key):record?.status==='cleared'?'已清除，当前未记录':'尚未确定'}</dd></div>`).join('')}</dl>`;
  const worn=appearance.worn_items,known=appearance.wearing_state==='known'&&Array.isArray(worn)&&worn.every(i=>i.source_verified===true&&i.availability==='available');
  body+='<h4>当前穿着</h4>'+(known?(worn.length?ClothingRecords(worn,{mode:'current'}):'<p class="small muted">当前没有已登记的穿着。</p>'):'<p class="small muted">当前穿着尚未确认，不使用初始穿衣设定代替。</p>');
  if(Array.isArray(appearance.used_items)&&appearance.used_items.length)body+='<h4>当前使用</h4>'+ClothingRecords(appearance.used_items,{mode:'used'});
 }
 const activities=Array.isArray(current.active_activities)?current.active_activities:[];
 if(activities.length)body+=`<h4>活动记录</h4>${activities.map(a=>`<div class="lrow"><span class="grow">${esc([a.description,a.activity].find(v=>typeof v==='string'&&v.trim())||'活动内容见详情')}</span>${badge(stateLabel(a.status),'plain')}</div>`).join('')}`;
 if(context.instant)body+=`<p class="small muted">当前时间：${esc(userInstant(context.instant,chosen))}</p>`;
 else if(appearance?.captured_at)body+=`<p class="small muted">状态读取时间：${esc(userInstant(appearance.captured_at,chosen))}</p>`;
 return `<div class="stack current-profile">${body}</div>`;
}
