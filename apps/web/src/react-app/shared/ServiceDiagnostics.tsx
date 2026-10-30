import {useEffect,useRef,useState} from 'react';
import {RefreshCw,MoreHorizontal} from 'lucide-react';
import WorkspaceInfo from './WorkspaceInfo';
import {Button} from '@/components/ui/button';
import {Surface} from '../controls';
import {useWorkspace} from '../page';
import useWorkspaceRead from './useWorkspaceRead';
import DiagnosticStatus from './DiagnosticStatus';
import {InlineFeedback} from './Presentation';
import JsonDetails from './JsonDetails';
import {configurationTestSummary} from '@/v3/configuration-feedback.mjs';

/** Probe only fresh keys owned by this invocation, including when storage access throws. */
export function probeBrowserStorage(host:any=globalThis){
 return ['localStorage','sessionStorage'].map(name=>{
  const key=`fluctlight:diagnostic-probe:${Date.now()}:${Math.random().toString(36).slice(2)}`;
  let storage:any,reason='';
  try{storage=host[name];storage.setItem(key,key);if(storage.getItem(key)!==key)throw Error('写入后未能读取相同内容');}
  catch(cause){reason=cause instanceof Error?cause.message:String(cause);}
  finally{if(storage)try{storage.removeItem(key);}catch(cause){reason=reason||String(cause);}}
  return {name,status:reason?'failed':'ok',...(reason?{reason}:{})};
 });
}
export default function ServiceDiagnostics({view}:any){
 const w=useWorkspace(),owner=w.state.session?.actorId;
 const savedVersion=JSON.stringify(w.state.runtimeCatalog?.versions||{});
 const read=useWorkspaceRead({identity:['diagnostic-service-snapshot',savedVersion],characterScoped:false,read:async()=>{
  const [configuration,connection]=await Promise.allSettled([w.services.configurationClient.read(),w.services.operationsClient.health()]);
  return {items:[],next_cursor:'',configuration:configuration.status==='fulfilled'?configuration.value:null,configurationError:configuration.status==='rejected'?String(configuration.reason?.message||configuration.reason):'',connection:connection.status==='fulfilled'?connection.value:null,connectionError:connection.status==='rejected'?String(connection.reason?.message||connection.reason):'',storage:probeBrowserStorage()};
 }});
 const extra=read.extra as Record<string,any>;
 const config=extra.configuration,version=JSON.stringify([owner,savedVersion,config||null]),current=useRef(version),active=useRef(true),epochs=useRef<Record<string,number>>({});
 current.current=version;
 const [tests,setTests]=useState<Record<string,any>>({}),[detailsOpen,setDetailsOpen]=useState(false);
 useEffect(()=>{active.current=true;setTests({});return()=>{active.current=false;};},[version]);
 async function test(kind:string,target:string){
  const captured=version,token=(epochs.current[kind]||0)+1;epochs.current[kind]=token;
  const versionKey=({vision:'vision.understanding',media:'media.comfyui',kev:'kev'} as Record<string,string>)[kind],expected=config?.versions?.[versionKey];
  setTests(old=>({...old,[kind]:{version:captured,busy:true}}));
  const result=await view.test(target);
  if(!active.current||current.current!==captured||w.state.session?.actorId!==owner||epochs.current[kind]!==token)return;
  const stale=typeof expected==='number'&&typeof result?.config_version==='number'&&expected!==result.config_version;
  setTests(old=>({...old,[kind]:{version:captured,result,busy:false,stale}}));
 }
 const receipt=(kind:string)=>tests[kind]?.version===version?tests[kind]:undefined;
 const testing=Object.values(tests).some(entry=>entry.version===version&&entry.busy);
 const endpoints=Array.isArray(config?.endpoints)?config.endpoints:[],bindings=(Array.isArray(config?.bindings)?config.bindings:[]).filter((binding:any)=>binding.model_id&&binding.endpoint_id&&endpoints.some((endpoint:any)=>endpoint.id===binding.endpoint_id)),media=config?.effective_values?.['media.comfyui'],mediaConfigured=!!media?.baseUrl;
 const storage=extra.storage||[],storageFailed=storage.some((s:any)=>s.status!=='ok');
 const description=(kind:string)=>{
  const entry=receipt(kind);if(entry?.busy)return '正在执行显式测试…';if(!entry?.result)return '';if(entry.stale)return '测试回执的配置版本与当前页面不同，请重新检查已保存配置。';
  const result=entry.result,summary=configurationTestSummary(result);
  if(summary.state==='failed')return '测试未通过：'+(result.reason||summary.failures.map((f:any)=>f.value).join('；'));
  if(kind==='vision')return result.image_input_verified===true?'已验证图片输入；业务结果以实际任务为准。':'已收到测试回执，尚未确认图片输入验证通过。';
  if(kind==='media')return summary.state==='success'?'已检查服务连接与工作流；尚未执行图片生成。':'已收到测试回执，连接结论尚未确认；未执行图片生成。';
  return summary.text;
 };
 const rowState=(kind:string,configured:boolean)=>receipt(kind)?.busy?{label:'测试中',tone:'blue' as const}:receipt(kind)?.stale?{label:'配置已变化',tone:'amber' as const}:receipt(kind)?.result?{label:configurationTestSummary(receipt(kind).result).state==='failed'?'需检查':'已测试',tone:configurationTestSummary(receipt(kind).result).state==='failed'?'amber' as const:'neutral' as const}:{label:read.busy?'检查中':!read.ready?'尚未检查':extra.configurationError?'读取失败':configured?'已配置 · 未测试':'未配置',tone:'neutral' as const};
 return <Surface className="replica-service-diagnostics">
  <div className="replica-service-heading"><div><h2>服务诊断</h2><p>检查存储、配置与服务状态。</p></div><div className="replica-service-actions"><Button variant="ghost" size="icon" aria-label="显式连接测试与检查记录" aria-expanded={detailsOpen} onClick={()=>setDetailsOpen(value=>!value)}><MoreHorizontal size={16}/><span className="sr-only">显式连接测试与检查记录</span></Button><Button variant="outline" onClick={read.load} disabled={read.busy}><RefreshCw size={14} className={read.busy?'animate-spin':''}/>重新检查</Button></div></div>
  {read.error&&<InlineFeedback kind="error">{read.error}<Button variant="outline" onClick={read.retry}>重试原读取</Button></InlineFeedback>}
  <div className="replica-service-row"><div><strong>浏览器存储</strong><small>{!read.ready?'尚未完成当前浏览器检查':storageFailed?storage.filter((s:any)=>s.status!=='ok').map((s:any)=>s.name+'：'+s.reason).join('；'):'当前浏览器的本次存储读写检查已通过。'}</small></div><DiagnosticStatus label={read.busy?'检查中':!read.ready?'尚未检查':storageFailed?'受限':'正常'} tone={!read.busy&&read.ready&&!storageFailed?'green':'neutral'}/></div>
  <div className="replica-service-row"><div><strong>模型服务配置</strong><small>{config?`已保存 ${endpoints.length} 个服务地址、${bindings.length} 个模型用途绑定；尚未验证模型可用性。`:extra.configurationError||'尚未读取已保存配置'}</small>{description('vision')&&<small>{description('vision')}</small>}</div><DiagnosticStatus {...rowState('vision',bindings.length>0)}/></div>
  <div className="replica-service-row"><div><strong>图片生成服务</strong><small>{config?(mediaConfigured?'已保存 ComfyUI 地址；尚未验证图片生成。':'尚未配置 ComfyUI 地址。'):'尚未读取图片配置'}</small>{description('media')&&<small>{description('media')}</small>}</div><DiagnosticStatus {...rowState('media',mediaConfigured)}/></div>
  {extra.configurationError&&<InlineFeedback kind="error">已保存配置读取失败：{extra.configurationError}<Button variant="outline" onClick={read.load}>重新读取配置</Button></InlineFeedback>}
  {config&&(!bindings.length||!mediaConfigured)&&<Button variant="link" onClick={()=>w.navigatePath('/settings?section='+encodeURIComponent(!bindings.length?'模型用途':'图片（ComfyUI）'))}>前往配置缺少的服务</Button>}
  <div className="replica-service-guidance"><WorkspaceInfo>这里读取已保存配置并检查当前浏览器。模型与图片服务测试只在你明确执行时运行，连接可达不代表业务生成完成。</WorkspaceInfo></div>{detailsOpen&&<section className="replica-service-technical"><p>以下操作会实际调用已保存的配置。重新检查仅执行读取与当前浏览器临时存储检查。</p><div className="replica-service-tests"><Button variant="outline" disabled={testing||!config} onClick={()=>test('vision','看图理解')}>验证视觉模型图片输入</Button><Button variant="outline" disabled={testing||!config} onClick={()=>test('media','图片（ComfyUI）')}>检查 ComfyUI 连接与工作流</Button><Button variant="outline" disabled={testing||!config} onClick={()=>test('kev','高级')}>测试辅助决策连接</Button></div>{description('kev')&&<p>{description('kev')}</p>}{['vision','media','kev'].map(kind=>receipt(kind)?.result&&<JsonDetails key={kind} value={receipt(kind).result} title={{vision:'视觉测试回执',media:'ComfyUI 检查回执',kev:'辅助决策测试回执'}[kind]}/>)}<p>工作区连接：{extra.connectionError||(!read.ready?'尚未读取':extra.connection?'已收到就绪检查回执；此项不证明模型可用':'尚无检查回执')}</p>{extra.connection&&<JsonDetails value={extra.connection} title="工作区连接原始回执"/>}<JsonDetails value={storage} title="浏览器临时存储检查记录"/></section>}
 </Surface>;
}
