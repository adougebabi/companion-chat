import {labelFor,enumLabel} from '../../../lib/fluctlight-display.ts';
import {esc} from './controls.js';

// Domain descriptions supplement the shared dictionary; technical identities
// remain in the original source disclosure, never as the primary field label.
const labels={identity:'身份与背景',personality:'性格',behavioral_policy:'表达与相处',life_profile:'生活设定',voice:'声音与说话方式',tone:'语气',speech_patterns:'说话习惯',body_language:'肢体习惯',behavior_loops:'反复出现的习惯',loops:'习惯描述',scenario_behavior:'不同场合的表现',secrets:'内心与秘密',fears:'担忧',desires:'愿望',output_preferences:'表达偏好',intimacy_progression:'亲密关系中的变化',behavior_state_machine:'行为与情境',emotional_state:'情绪设定',core_relationship:'人格之间的关系',core_conflict:'人格之间的分歧',integration:'协作与融合',shared_memory_policy:'记忆共享方式',stage:'阶段',milestones:'阶段条件',world:'世界背景',current_task:'当前活动',doing:'此刻正在做',activity:'活动',mood:'心情',energy:'精力',label:'描述',text:'内容',action:'行动',claim:'自我认知',value:'认识内容',worn_items:'当前穿着',used_items:'当前使用',wearing_state:'穿着记录',physical_features:'外貌特征',daily_outfit_preferences:'日常穿衣偏好',style_preferences:'风格偏好',nickname:'昵称',height:'身高',empathy:'共情',assertiveness:'主张性',patience:'耐心',humor:'幽默感'};
const technical=key=>/^(id|schema_version|extensions|provenance|evidence_refs|update_policy|source_hash|rules_version|owner_switch_targets)$/.test(key)||/(_id|_ids|_revision|_version|_at)$/.test(key);
const units=new Set(['openness','conscientiousness','extraversion','agreeableness','neuroticism','curiosity','independence','patience','empathy','assertiveness','humor','sociability','risk_tolerance','emoji_frequency','sarcasm_tendency','directness','initiative','topic_initiation','silence_tolerance','emotional_expression','confidence']);
const aliases={male:'男性',female:'女性',nonbinary:'非二元性别',natural:'自然',known:'已记录',unknown:'尚未确定',clear:'清晰明确',gentle:'温和',direct:'直接',black:'黑色',brown:'棕色',blonde:'金色',long:'长发',shoulder_length:'齐肩',short_hair:'短发'};
export const hasSemanticValue=value=>value!=null&&value!==''&&(!Array.isArray(value)||value.some(hasSemanticValue))&&(typeof value!=='object'||Array.isArray(value)||Object.values(value).some(hasSemanticValue));
export function semanticLabel(key){return labels[key]||labelFor(key);}
export function SemanticValue(value,key='',depth=0){
 if(!hasSemanticValue(value))return '';
 if(typeof value==='boolean')return value?'是':'否';
 if(typeof value==='number')return Number.isFinite(value)?esc(units.has(key)&&value>=0&&value<=1?`${Math.round(value*100)}%`:key==='response_delay'?`${value} 秒`:value):'';
 if(typeof value==='string'){const known=aliases[value]||enumLabel(value);return esc(known==='未分类'?value:known);}
 if(depth>=5)return '<span class="small muted">更多细节见完整设定记录</span>';
 if(Array.isArray(value)){const items=value.map(v=>SemanticValue(v,key,depth+1)).filter(Boolean);return items.length?`<ul class="semantic-list">${items.map(v=>`<li>${v}</li>`).join('')}</ul>`:'';}
 if(typeof value==='object'){
  const rows=Object.entries(value).filter(([k,v])=>!technical(k)&&hasSemanticValue(v)).map(([k,v])=>{const content=SemanticValue(v,k,depth+1);if(!content)return '';const label=semanticLabel(k);return `<div class="semantic-entry">${label==='自定义字段'?'':`<span class="small muted">${esc(label)}：</span>`}${content}</div>`;}).filter(Boolean);
  return rows.join('');
 }
 return '';
}
export function SemanticFields(value,{exclude=[],empty='尚未记录，可在身份与能力治理中继续补充。'}={}){
 const rows=value&&typeof value==='object'&&!Array.isArray(value)?Object.entries(value).filter(([key,v])=>!technical(key)&&!exclude.includes(key)&&hasSemanticValue(v)).map(([key,v])=>{const label=semanticLabel(key),content=SemanticValue(v,key);return content?`<div><dt>${esc(label==='自定义字段'?'补充设定':label)}</dt><dd>${content}</dd></div>`:'';}).filter(Boolean):[];
 return rows.length?`<dl class="profile-facts">${rows.join('')}</dl>`:`<p class="small muted">${esc(empty)}</p>`;
}
