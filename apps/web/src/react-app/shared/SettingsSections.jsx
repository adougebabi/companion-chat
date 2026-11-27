import {Checkbox} from '@/components/ui/checkbox';
import {FieldError} from '@/components/ui/field';
import {useEffect,useId,useRef,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Surface,Disclosure} from '../controls';
import {isInternalId} from '@/v3/internal-id.mjs';
import {autonomyActionChoices} from '@/v3/proto/components/configuration-fields.js';
import JsonDetails from './JsonDetails';

const snakeCasePluginName = /^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$/i;

function pluginDisplayName(plugin) {
 if (plugin.id === 'image') return '图片';
 if (plugin.id === 'moment') return '动态';

 const {manifest} = plugin;
 const candidates = [plugin.display_name, plugin.title, plugin.name,
  manifest?.display_name, manifest?.title, manifest?.name];
 for (const candidate of candidates) {
  if (typeof candidate !== 'string') continue;
  const name = candidate.trim();
  if (name && !isInternalId(name) && !snakeCasePluginName.test(name)) return name;
 }
 return '插件';
}

function pluginHealthLabel(plugin){
 if(plugin.health==='unconfigured')return '图片服务未配置';
 if(plugin.health==='configuration_invalid')return '图片服务配置不完整';
 return '健康状态未检测';
}

export function PluginStatus({plugin,view}){
 const moment=plugin.id==='moment',scope=moment?view.momentHealthScope():view.mediaHealthScope(),key=JSON.stringify([scope.owner,scope.version,scope.reset,moment?plugin.enabled:null]),observed=useRef({key,epoch:0}),active=useRef(true),[probe,setProbe]=useState(null);
 if(observed.current.key!==key)observed.current={key,epoch:observed.current.epoch+1};
 useEffect(()=>{active.current=true;return()=>{active.current=false;};},[]);
 const current=probe?.key===key&&probe?.epoch===observed.current.epoch?probe:null;
 async function check(){
  const epoch=observed.current.epoch;setProbe({key,epoch,busy:true});
  const result=await view.test(moment?'动态（Moment）':'图片（ComfyUI）');
  if(!active.current||observed.current.epoch!==epoch||observed.current.key!==key)return;
  const stale=result?.state==='configuration_changed'||(typeof result?.config_version==='number'&&result.config_version!==scope.version)||(moment&&typeof result?.plugin_enabled==='boolean'&&result.plugin_enabled!==plugin.enabled);
  const ready=moment?result?.state==='ready'&&result.registry_checked===true&&result.data_read_tested===true&&result.publication_tested===false&&result.model_tested===false&&result.plugin_enabled===plugin.enabled&&result.config_version===scope.version:result?.state==='reachable'&&result.generation_tested===false&&result.config_version===scope.version;
  const state=!result?'superseded':stale?'stale':ready?'reachable':moment&&result?.state!=='unavailable'&&result?.status!=='failed'?'unknown':'failed';
  setProbe({key,epoch,state});
 }
 const label=current?.busy?(moment?'正在检查动态能力':'正在检查图片服务'):current?.state==='stale'?'配置已变化，请重新检查':current?.state==='superseded'?'检查结果已失效，请重新检查':current?.state==='reachable'?(moment?'动态列表可读取，未验证发布和模型':'图片服务可连接，未验证图片生成'):current?.state==='failed'?(moment?'动态能力检查未通过':'图片服务检查未通过'):current?.state==='unknown'?'检查回执未确认，请重新检查':pluginHealthLabel(plugin);
 return <div className="grid gap-2">
  <p className="muted">{pluginDisplayName(plugin)} · {plugin.enabled?'已启用':'已停用'} · {label}{plugin.required_dependencies?.includes('comfyui')?' · 生图需要 ComfyUI':''}{plugin.optional_dependencies?.includes('image')?' · 图片为可选依赖':''}</p>
  {plugin.id==='image'&&<Button className="w-fit" type="button" variant="outline" disabled={current?.busy||plugin.health==='unconfigured'||plugin.health==='configuration_invalid'} onClick={check}>检查图片服务连接</Button>}
  {moment&&<Button className="w-fit" type="button" variant="outline" disabled={current?.busy} onClick={check}>检查动态能力</Button>}
  <JsonDetails value={plugin} title="原始插件记录（诊断）" downloadOnly />
 </div>;
}

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
 else if(title==='能力插件')content=<><Surface className="settings-card"><div className="replica-settings-section-head"><div><h2>能力插件</h2><p>停用后不再受理新请求。当前进程中已开始的运行可以完成；已创建的媒体任务继续恢复，已发布内容仍可读取。尚未创建媒体任务的排队工作在恢复时会检查当前设置。动态不依赖图片能力。</p></div></div>{input(fields)}{view.domainPlugins.value.map(plugin=><PluginStatus key={plugin.id} plugin={plugin} view={view}/>)}</Surface></>;
 else if(title==='看图理解')content=card('图片理解',fields,'开启后使用独立视觉模型理解收到的图片。',test());
 else if(title==='图片（ComfyUI）')content=<>{card('图片服务',fields.filter(f=>['baseUrl','generation_timeout_seconds'].includes(f._key)),'连接图片服务，等待时间按真实工作流设置。',test())}{card('默认工作流',fields.filter(f=>f._key==='workflow'),'填写图片服务导出的 API 工作流，原结构会保留。')}{card('按用途覆盖',fields.filter(f=>!['baseUrl','generation_timeout_seconds','workflow'].includes(f._key)),'可选。留空时沿用默认工作流。')}</>;
 else if(title==='角色自主与主动联系'){
  const allowed=field('allowed_actions'),confirmed=field('confirm_action_types');
  content=<><Surface className="settings-card"><div className="replica-settings-section-head"><div><h2>自主行动</h2><p>在你的授权范围内，保留角色独立判断和实际执行的区分。</p></div></div>{allowed&&confirmed&&<ActionCards allowed={allowed} confirmed={confirmed}/>} {input(fields.filter(f=>!['allowed_actions','confirm_action_types'].includes(f._key)))}</Surface></>;
 }else if(title==='通知')content=card('想收到的通知',fields,'关闭某一类提醒后仍保留通知记录。');
 else if(title==='作息与后台节奏')content=<>{card('后台节奏',[...bySetting('product.wakeup'),...bySetting('product.summary')],'定时唤醒与对话摘要分别按已保存的设置运行。')}{card('目标失败后的重试',bySetting('product.goal_retry'),'调整重试次数和等待间隔，实际状态以原任务记录为准。')}</>;
 else if(title==='资源与预算')content=card('模型并发',fields,'生成与记忆检索使用各自的排队设置。');
 else if(title==='高级'){
  const source=group.valueSources?.kev,main=['endpoint','strategy','timeout_ms','budget_ms','concurrency','batch_size','retention_days','min_probability','min_margin','max_deferrals','deferral_seconds','model','model_revision'],toggles=['enabled','media_enabled'];
  content=<><Surface className="settings-card settings-kev-card"><div className="replica-settings-section-head"><div><h2>KEV 决策评分</h2><p>在候选行动之间加入独立评分，授权与实际执行分别记录。</p></div><span className="settings-value-source">{source==='runtime_default'?'系统默认 · 尚未保存':source==='stored'?'已保存配置':'配置来源未确认'}</span></div><div className="settings-kev-toggles">{toggles.map(key=>field(key)&&<view.SettingsField key={key} field={field(key)}/>)}{field('enabled')?.value===false&&field('media_enabled')?.value===true&&<p className="settings-field-hint settings-kev-hint">媒体决策的选择已保留，启用 KEV 后生效。</p>}</div><div className="form-grid settings-kev-fields">{main.map(key=>field(key)&&<view.SettingsField key={key} field={field(key)}/>)}</div></Surface><Surface className="settings-card"><Disclosure title="决策场景与技术参数">{input(fields.filter(f=>![...toggles,...main].includes(f._key)))}{test()}</Disclosure></Surface></>;

 }
 else content=card(view.sectionLabel(title),fields);
 return <section hidden={!visible||!view.catalogReady.value} data-section={title} id={view.sectionId(title)} className="settings-group replica-settings-section">{content}</section>;
}
