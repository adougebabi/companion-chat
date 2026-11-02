import {useEffect,useRef,useState} from 'react';
import {Download,Plus,Trash2} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Surface} from '../controls';
import {EmptyState,Feedback} from './Presentation';
import {formatBeijingInstant as userInstant} from '@/lib/instant';
import {randomId} from '@/random-id.mjs';
import ConfirmationFrame from './ConfirmationFrame';

type Backup={id:string;name:string;createdAt:string;pack:Record<string,unknown>};
type Props={owner:string;exportPack:()=>Promise<{pack?:Record<string,unknown>}>;onImport:(pack:Record<string,unknown>)=>void;storage?:Storage};

/** Explicit browser copies of exported settings, never a second live snapshot. */
export default function LocalBackupsPanel({owner,exportPack,onImport,storage}:Props){
 const slot=`fluctlight-local-backups:${owner}`,scope=useRef(0),[backups,setBackups]=useState<Backup[]>([]),[loaded,setLoaded]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const [deleting,setDeleting]=useState<Backup|null>(null);
 useEffect(()=>{
  scope.current++;setBackups([]);setLoaded(false);setBusy(false);setError('');setNotice('');setDeleting(null);
  try{const value=JSON.parse((storage??globalThis.localStorage).getItem(slot)||'[]');if(!Array.isArray(value)||value.some(row=>!row||typeof row.id!=='string'||typeof row.name!=='string'||typeof row.createdAt!=='string'||!row.pack||typeof row.pack!=='object'))throw Error();setBackups(value);setLoaded(true);}
  catch{setError('本地备份未能读取，浏览器中的原记录已保留。');}
  return()=>{scope.current++;};
 },[slot,storage]);
 function save(next:Backup[]){(storage??globalThis.localStorage).setItem(slot,JSON.stringify(next));setBackups(next);}
 async function create(){
  if(busy||!loaded)return;
  const submitted=scope.current;setBusy(true);setError('');setNotice('');
  try{const result=await exportPack();if(scope.current!==submitted)return;if(!result?.pack||typeof result.pack!=='object')throw Error();const createdAt=new Date().toISOString();save([...backups,{id:randomId(),name:'设定备份 · '+userInstant(createdAt),createdAt,pack:result.pack}]);setNotice('本地备份已保存到当前浏览器。');}
  catch{if(scope.current===submitted)setError('创建备份失败，请检查连接或浏览器存储空间后重试。已有备份保持原样。');}
  finally{if(scope.current===submitted)setBusy(false);}
 }
 function download(backup:Backup){
  let url='';try{url=URL.createObjectURL(new Blob([JSON.stringify(backup.pack,null,2)],{type:'application/json'}));const link=document.createElement('a');link.href=url;link.download='摇光设定备份.json';link.click();setNotice('设定文件已准备下载。');}
  catch{setError('下载未能开始，请保留本地备份后重试。');}
  finally{if(url)setTimeout(()=>URL.revokeObjectURL(url),1000);}
 }
 function remove(backup:Backup){try{save(backups.filter(row=>row.id!==backup.id));setNotice('本地副本已删除。');setError('');}catch{setError('未能删除本地副本，原备份已保留。');}finally{setDeleting(null);}}
 return <Surface as="section" className="local-backups-panel" aria-label="本地备份">
  <div className="replica-settings-section-head"><div><h3>本地备份</h3><p>保存已保存设定的浏览器副本；导入时先校验，再创建新的创作草稿。</p></div><Button type="button" variant="outline" disabled={!loaded||busy} onClick={()=>void create()}><Plus size={14}/>{busy?'正在创建…':'创建备份'}</Button></div>
  <p>{loaded?`${backups.length} 份本地备份`:'本地备份数量未读到'}</p>
  <p className="text-sm text-muted-foreground">包含作品设定、依赖、分组名称与通知偏好。不包含聊天、记忆、媒体和服务密钥；完整业务数据请使用下方的业务数据备份。</p>
  {error&&<Feedback kind="error">{error}</Feedback>}{notice&&<Feedback kind="success">{notice}</Feedback>}
  {loaded&&!backups.length&&<EmptyState variant="page"><h3>还没有本地备份</h3><p>为已经保存的设定留一份副本。</p></EmptyState>}
  <div className="grid gap-3">{backups.map(backup=><div className="flex flex-wrap items-center justify-between gap-3 border-b py-3" key={backup.id}><div><strong>{backup.name}</strong><p className="text-sm text-muted-foreground">{userInstant(backup.createdAt)}</p></div><div className="flex flex-wrap gap-2"><Button type="button" variant="outline" onClick={()=>download(backup)}><Download size={14}/>下载</Button><Button type="button" variant="outline" onClick={()=>{onImport(backup.pack);setNotice('已填入导入草稿，请先校验再确认导入。');}}>填入导入草稿</Button><Button type="button" variant="ghost" size="icon" disabled={busy} aria-label={'删除'+backup.name} onClick={()=>setDeleting(backup)}><Trash2 size={14}/></Button></div></div>)}</div>
  {deleting&&<ConfirmationFrame title="删除这份本地备份？" description="当前设定与工作区保持原样，只删除所选的浏览器副本。" onClose={()=>setDeleting(null)}><Button type="button" variant="outline" onClick={()=>setDeleting(null)}>取消</Button><Button type="button" variant="destructive" onClick={()=>remove(deleting)}>删除备份</Button></ConfirmationFrame>}
 </Surface>;
}
