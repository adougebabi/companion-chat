import {Checkbox} from '@/components/ui/checkbox';
import {FieldError} from '@/components/ui/field';
import {useId} from 'react';
import {Button} from '@/components/ui/button';
import {Surface} from '../controls';
import {autonomyActionChoices} from '@/v3/proto/components/configuration-fields.js';

function ActionCards({allowed,confirmed}){
 const id=useId(),allowedError=allowed.validationError?id+'-allowed':undefined,confirmedError=confirmed.validationError?id+'-confirmed':undefined;
 const names=new Map(autonomyActionChoices);
 for(const key of [...(allowed.value||[]),...(confirmed.value||[])])if(!names.has(key))names.set(key,key);
 const change=(field,key,value)=>{const next=new Set(field.value||[]);if(value)next.add(key);else next.delete(key);field.value=[...next];field.validationError='';};
 return <><div className="settings-action-cards">{[...names].map(([key,label])=><div className="settings-action-card" key={key}><label data-setting-field={JSON.stringify([allowed._settingKey,allowed._key])}><Checkbox aria-label={label} aria-invalid={!!allowedError} aria-describedby={allowedError} checked={(allowed.value||[]).includes(key)} onCheckedChange={value=>change(allowed,key,!!value)}/><strong>{label}</strong></label><p>{({proactive_message:'允许角色主动发起联系。',moment:'允许角色独立决定分享近况。',capability:'允许在已有授权内提出外部行动或生成需求。'})[key]||'保留当前服务支持的行动类型。'}</p><label data-setting-field={JSON.stringify([confirmed._settingKey,confirmed._key])}><Checkbox aria-label={label+'需要明确指令'} aria-invalid={!!confirmedError} aria-describedby={confirmedError} checked={(confirmed.value||[]).includes(key)} onCheckedChange={value=>change(confirmed,key,!!value)}/>需要明确用户指令</label></div>)}</div>{allowedError&&<FieldError id={allowedError}>{allowed.validationError}</FieldError>}{confirmedError&&<FieldError id={confirmedError}>{confirmed.validationError}</FieldError>}</>;
}
export default function SettingsSections({group,view,visible=true}){
 const fields=group.fields,field=key=>fields.find(f=>f._key===key),bySetting=key=>fields.filter(f=>f._settingKey===key),title=group.title;
 const input=rows=><div className="form-grid">{rows.map(f=><view.SettingsField key={JSON.stringify([f._settingKey,f._key])} field={f}/>)}</div>;
 const card=(heading,rows,description,action)=>!rows.length?null:<Surface className="settings-card" key={heading}><div className="replica-settings-section-head"><div><h2>{heading}</h2>{description&&<p>{description}</p>}</div></div>{input(rows)}{action}</Surface>;
 const test=()=> <Button type="button" variant="outline" onClick={()=>view.test(title)}>{title==='看图理解'?'测试看图（使用已保存配置）':'测试连接'}</Button>;
 let content;
 if(title==='聊天回复')content=card('回复方式',fields,'选择回复方式，调整表达的变化程度和采样范围。');
 else if(title==='看图理解')content=card('图片理解',fields,'开启后使用独立视觉模型理解收到的图片。',test());
 else if(title==='图片（ComfyUI）')content=<>{card('图片服务',fields.filter(f=>['baseUrl','generation_timeout_seconds'].includes(f._key)),'连接图片服务，等待时间按真实工作流设置。',test())}{card('默认工作流',fields.filter(f=>f._key==='workflow'),'填写图片服务导出的 API 工作流，原结构会保留。')}{card('按用途覆盖',fields.filter(f=>!['baseUrl','generation_timeout_seconds','workflow'].includes(f._key)),'可选。留空时沿用默认工作流。')}</>;
 else if(title==='角色自主与主动联系'){
  const allowed=field('allowed_actions'),confirmed=field('confirm_action_types');
  content=<><Surface className="settings-card"><div className="replica-settings-section-head"><div><h2>自主行动</h2><p>在你的授权范围内，保留角色独立判断和实际执行的区分。</p></div></div>{allowed&&confirmed&&<ActionCards allowed={allowed} confirmed={confirmed}/>} {input(fields.filter(f=>!['allowed_actions','confirm_action_types'].includes(f._key)))}</Surface></>;
 }else if(title==='通知')content=card('想收到的通知',fields,'关闭某一类提醒后仍保留通知记录。');
 else if(title==='作息与后台节奏')content=<>{card('后台节奏',[...bySetting('product.wakeup'),...bySetting('product.summary')],'定时唤醒与对话摘要分别按已保存的设置运行。')}{card('目标失败后的重试',bySetting('product.goal_retry'),'调整重试次数和等待间隔，实际状态以原任务记录为准。')}</>;
 else if(title==='资源与预算')content=card('模型并发',fields,'生成与记忆检索使用各自的排队设置。');
 else if(title==='高级')content=<>{card('辅助决策服务',fields.filter(f=>['enabled','media_enabled','endpoint','model','model_revision'].includes(f._key)),'按场景补充判断，实际授权和执行结果保持独立。',test())}{card('判定与等待',fields.filter(f=>['timeout_ms','budget_ms','concurrency','batch_size','strategy','min_probability','min_margin','max_deferrals','deferral_seconds'].includes(f._key)),'保留当前服务支持的范围和精度。')}{card('场景与记录',fields.filter(f=>!['enabled','media_enabled','endpoint','model','model_revision','timeout_ms','budget_ms','concurrency','batch_size','strategy','min_probability','min_margin','max_deferrals','deferral_seconds'].includes(f._key)))}</>;
 else content=card(view.sectionLabel(title),fields);
 return <section hidden={!visible||!view.catalogReady.value} data-section={title} id={view.sectionId(title)} className="settings-group replica-settings-section">{content}</section>;
}
