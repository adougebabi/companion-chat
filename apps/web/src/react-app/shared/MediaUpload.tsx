import {useEffect,useMemo,useRef,useState} from 'react';
import {UploadIcon,FileImageIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Card,CardHeader,CardTitle,CardContent} from '@/components/ui/card';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {mediaUploadController} from '@/v3/providers/media-upload.mjs';
import {studioDraftStorage} from '@/v3/proto/components/studio-draft-storage.js';
import {errorMessage,stateLabel} from '@/v3/domain-presentation.mjs';
import {useWorkspace} from '../page';

export default function MediaUpload({onSaved=()=>{}}:{onSaved?:(asset:any)=>unknown}){
 const w=useWorkspace(),owner=w.state.session.actorId,input=useRef<HTMLInputElement>(null),alive=useRef(false),saved=useRef(onSaved);
 saved.current=onSaved;
 const controller=useMemo(()=>mediaUploadController({slot:`fluctlight-media:${owner}:upload`,storage:studioDraftStorage,client:w.services.mediaClient,isOwner:()=>w.state.session?.actorId===owner}),[w,owner]);
 const [file,setFile]=useState<File|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[status,setStatus]=useState(''),[,redraw]=useState(0);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[controller]);
 const live=()=>alive.current&&w.state.session?.actorId===owner;
 const pending=controller.pending(),last=controller.lastAsset();
 function select(next:File|null){setFile(next);controller.select(next);setError(next&&next.size>64*1024*1024?'文件超过 64MiB，请选择更小的素材。':'');}
 async function run(retry=false){if(busy||!live())return;setBusy(true);setError('');setStatus(retry?'正在核对原上传…':'正在上传…');
  try{const asset=await(retry?controller.retry():controller.submit());if(!live())return;setStatus(asset.readable?'上传已保存，可以在素材库查看。':`上传已保存，当前读取状态：${stateLabel(asset.read_state)}`);try{await saved.current(asset);}catch(cause){if(live())setError('素材已经保存；列表刷新失败：'+errorMessage(cause));}}
  catch(cause){if(live()){setError(errorMessage(cause));setStatus('');}}finally{if(live()){setBusy(false);redraw(v=>v+1);}}}
 return <Card aria-label="上传素材"><CardHeader><CardTitle>上传素材</CardTitle></CardHeader><CardContent className="grid gap-4">
  <p className="text-sm text-muted-foreground">支持图片、视频和音频，单个文件不超过 64MiB。素材仅保存到你的媒体库。</p>
  <div className="grid justify-items-center gap-3 rounded-lg border border-dashed p-6 text-center" onDragOver={e=>e.preventDefault()} onDrop={e=>{e.preventDefault();if(!busy)select(e.dataTransfer.files[0]||null);}}><FileImageIcon className="size-7 text-muted-foreground" aria-hidden="true"/><p>{file?.name||'拖放文件到这里，或选择一个文件'}</p><Button type="button" variant="outline" disabled={busy} onClick={()=>input.current?.click()}><UploadIcon/>选择文件</Button><input ref={input} type="file" className="sr-only" aria-label="选择上传的媒体文件" accept="image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm,audio/mpeg,audio/wav,audio/ogg" disabled={busy} onChange={e=>select(e.target.files?.[0]||null)}/></div>
  {pending&&<p className="text-sm">待确认：{pending.value.filename}。刷新后可查询原结果；尚未上传时，请重新选择同一个文件再重试。</p>}
  {error&&<Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}{status&&<p role="status">{status}</p>}
  <div className="flex flex-wrap gap-3"><Button type="button" disabled={busy||!!pending||!file||file.size>64*1024*1024} onClick={()=>void run()}>上传素材</Button>{pending&&<Button type="button" variant="outline" disabled={busy} onClick={()=>void run(true)}>查询或重试原上传</Button>}{last&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/media/'+encodeURIComponent(last))}>查看上次上传</Button>}</div>
 </CardContent></Card>;
}
