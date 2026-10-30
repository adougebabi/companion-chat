import {useEffect,useRef,useState,useSyncExternalStore} from 'react';
import {ArrowLeft,RefreshCw} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Surface} from '../controls';
import {useWorkspace} from '../page';
import {inspectCreationRecovery} from '@/approved-ui/creation-recovery-state.mjs';
import {readApprovedCreationProgress} from '@/approved-ui/creation-progress.mjs';
import {creationRecoveryChanged} from '@/v3/creation-draft.mjs';

// Reads existing recovery records only. Never migrate, repair or clear storage here.
export default function DiagnosticCreationContext({returnTo=''}:{returnTo?:string}){
 const w=useWorkspace();useSyncExternalStore(w.subscribe,w.getSnapshot,w.getSnapshot);
 const owner=w.state.session.actorId,characters=w.state.characters||[],characterScope=JSON.stringify(characters.map((row:any)=>[row.id,row.name]));
 const [result,setResult]=useState<any>(null),[revision,setRevision]=useState(0),epoch=useRef(0);
 useEffect(()=>{
  const token=++epoch.current;let active=true;const live=()=>active&&token===epoch.current&&w.state.session?.actorId===owner;
  setResult(null);
  void(async()=>{
   try{
    const storage=localStorage,records:any[]=[],seen=new Set();let problem='';
    const resume=returnTo.replace(/^#/,'').startsWith('/library/create')?new URLSearchParams(returnTo.split('?')[1]||'').get('resume'):'';
    for(const id of [resume||'',...characters.map((row:any)=>row.id)]){
     const record:any=inspectCreationRecovery({owner,storage,resume:id});
     if(record.blocked){problem ||=record.title+'。'+record.description;continue;}
     if(record.state==='preparation'&&record.draft?.activatedId&&!seen.has(record.draft.activatedId)){seen.add(record.draft.activatedId);records.push(record);}
    }
    for(const record of records){
     const draft=record.draft;
     const progress=draft.activationDate?await readApprovedCreationProgress(w.client,draft):null;
     if(!live())return;
     if(progress?.complete)continue;
     const name=characters.find((row:any)=>row.id===draft.activatedId)?.name||draft.name||'新相识';
     const stages=progress?.stages||[],unknown=stages.some((stage:any)=>stage.state==='unknown');
     setResult({owner,name,href:'#/library/create?resume='+encodeURIComponent(draft.activatedId),count:stages.length?stages.filter((stage:any)=>stage.state==='ready').length:null,total:stages.length,unknown});return;
    }
    if(problem&&live())setResult({owner,error:problem});
   }catch{if(live())setResult({owner,error:'初始化准备状态暂未读到，原创建资料和任务会保留。'});}
  })();
  const changed=(event:any)=>{if(!event.detail?.owner||event.detail.owner===owner)setRevision(value=>value+1);};
  const stored=(event:StorageEvent)=>{if(!event.key||event.key.startsWith(`fluctlight-creation:${owner}:`))setRevision(value=>value+1);};
  window.addEventListener(creationRecoveryChanged,changed);window.addEventListener('storage',stored);
  return()=>{active=false;epoch.current++;window.removeEventListener(creationRecoveryChanged,changed);window.removeEventListener('storage',stored);};
 },[w,owner,characterScope,returnTo,revision]);
 if(!result||result.owner!==owner)return null;
 return <Surface className="replica-diagnostic-context"><div><h3>{result.error?'初始化记录暂未读全':`${result.name} · 初始化流程${result.count===null?'':` ${result.count}/${result.total}`}`}</h3><p>{result.error|| (result.unknown?'部分准备状态未读到，可重新读取或返回原初始化流程核对。':'已有资料和原任务会保留，准备进度以实际记录为准。')}</p></div>{result.href&&<Button asChild variant="outline"><a href={result.href}><ArrowLeft size={14}/>查看并返回初始化</a></Button>}{(result.error||result.unknown)&&<Button variant="outline" onClick={()=>setRevision(value=>value+1)}><RefreshCw size={14}/>重新读取初始化状态</Button>}</Surface>;
}
