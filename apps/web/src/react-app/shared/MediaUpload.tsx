import {Attachment,AttachmentMedia,AttachmentContent,AttachmentTitle,AttachmentDescription} from '@/components/ui/attachment';
import {Spinner} from '@/components/ui/spinner';
import {Feedback,ResultFeedback,type ResultNotice} from "@/react-app/shared/Presentation";
import {useEffect,useMemo,useRef,useState} from 'react';
import {UploadIcon,FileImageIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Card,CardHeader,CardTitle,CardContent} from '@/components/ui/card';
import {mediaUploadController} from '@/v3/providers/media-upload.mjs';
import {studioDraftStorage} from '@/v3/proto/components/studio-draft-storage.js';
import {errorMessage,stateLabel,feedbackDiagnostic} from '@/v3/domain-presentation.mjs';
import {useWorkspace} from '../page';

export default function MediaUpload({onSaved=()=>{}}:{onSaved?:(asset:any)=>unknown}){
 const w=useWorkspace(),owner=w.state.session.actorId,input=useRef<HTMLInputElement>(null),alive=useRef(false),generation=useRef(0),saved=useRef(onSaved);
 saved.current=onSaved;
 const controller=useMemo(()=>mediaUploadController({slot:`fluctlight-media:${owner}:upload`,storage:studioDraftStorage,client:w.services.mediaClient,isOwner:()=>w.state.session?.actorId===owner}),[w,owner]);
 const [file,setFile]=useState<File|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[fileError,setFileError]=useState(''),[diagnostic,setDiagnostic]=useState<any>(null),[status,setStatus]=useState<ResultNotice>(null),[,redraw]=useState(0);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;generation.current++;};},[controller]);
 const live=()=>alive.current&&w.state.session?.actorId===owner;
 let pending:any=null,last:string|null=null,recordFailure:any=null,resultFailure:any=null;
 try{pending=controller.pending();if(pending&&(!pending.value?.filename||!Number.isSafeInteger(pending.value.size)||pending.value.size<=0||!/^[a-f0-9]{64}$/.test(pending.value.sha256||'')))throw Error('原上传校验记录不完整，当前不能安全重试。');}catch(cause){recordFailure=cause;}
 try{last=controller.lastAsset();if(last!==null&&!last)throw Error('上次上传结果的标识不可读取。');}catch(cause){resultFailure=cause;}
 function select(next:File|null){generation.current++;setFile(next);controller.select(next);setStatus(null);setFileError(next&&next.size>64*1024*1024?'文件超过 64MiB，请选择更小的素材。':'');if(!pending){setError('');setDiagnostic(null);}}
 async function refreshList(asset:any){if(!live())return;const token=++generation.current;setStatus({kind:'progress',text:'正在重新读取素材列表…'});try{await saved.current(asset);if(live()&&token===generation.current)setStatus(null);}catch(cause){if(live()&&token===generation.current)setStatus({kind:'warning',text:'素材已经保存，列表仍未刷新。',actions:<Button type="button" variant="outline" onClick={()=>void refreshList(asset)}>重新读取素材列表</Button>});}}
 async function run(retry=false){if(busy||!live()||recordFailure)return;const token=++generation.current;setBusy(true);setError('');setDiagnostic(null);setStatus({kind:'progress',text:retry?'正在核对原上传…':'正在上传…'});
  try{const asset=await(retry?controller.retry():controller.submit());if(!live()||token!==generation.current)return;setStatus({kind:asset.readable?'success':'warning',text:asset.readable?'上传已保存，可以查看素材详情。':`上传已保存，当前读取状态：${stateLabel(asset.read_state)}`,actions:!asset.readable&&<Button type="button" variant="outline" onClick={()=>w.navigatePath('/media/'+encodeURIComponent(asset.id))}>核对素材读取状态</Button>});try{await saved.current(asset);}catch(cause){if(live()&&token===generation.current)setStatus({kind:'warning',text:'素材已经保存，列表暂未刷新。',actions:<Button type="button" variant="outline" onClick={()=>void refreshList(asset)}>重新读取素材列表</Button>});}}
  catch(cause){if(live()&&token===generation.current){setError(errorMessage(cause));setDiagnostic(feedbackDiagnostic(cause));setStatus(null);}}finally{if(live()&&token===generation.current){setBusy(false);redraw(v=>v+1);}}}
 return <Card aria-label="上传素材"><CardHeader><CardTitle>上传素材</CardTitle></CardHeader><CardContent className="grid gap-4">
  <p className="text-sm text-muted-foreground">支持图片、视频和音频，单个文件不超过 64MiB。素材仅保存到你的媒体库。</p>
  <div className="grid justify-items-center gap-3 rounded-lg border border-dashed p-6 text-center" onDragOver={e=>e.preventDefault()} onDrop={e=>{e.preventDefault();if(!busy)select(e.dataTransfer.files[0]||null);}}><FileImageIcon className="size-7 text-muted-foreground" aria-hidden="true"/><p>{file?.name||'拖放文件到这里，或选择一个文件'}</p><Button type="button" variant="outline" aria-label="选择上传的媒体文件" disabled={busy} onClick={()=>input.current?.click()}><UploadIcon/>选择文件</Button><input ref={input} type="file" hidden tabIndex={-1} aria-hidden="true" aria-label="选择上传的媒体文件" accept="image/png,image/jpeg,image/gif,image/webp,video/mp4,video/webm,audio/mpeg,audio/wav,audio/ogg" disabled={busy} onChange={e=>select(e.target.files?.[0]||null)}/></div>
  {file&&<Attachment className="w-full" state={busy?'uploading':fileError?'error':pending?'processing':'idle'}><AttachmentMedia>{busy?<Spinner/>:<FileImageIcon/>}</AttachmentMedia><AttachmentContent><AttachmentTitle title={file.name}>{file.name}</AttachmentTitle><AttachmentDescription>{(file.size/1024/1024).toFixed(2)} MiB · {file.type||'媒体文件'}</AttachmentDescription></AttachmentContent></Attachment>}
  {fileError&&<Feedback kind="error">{fileError}</Feedback>}
  {recordFailure?<Feedback kind="error" title="原上传记录暂时无法读取" details={<pre>{JSON.stringify(feedbackDiagnostic(recordFailure),null,2)}</pre>} actions={<Button type="button" variant="outline" onClick={()=>redraw(v=>v+1)}>重新读取上传记录</Button>}>当前不会发起新的上传。原记录保留，可在记录可读后重新读取；关闭后仍可查看素材库。</Feedback>:pending&&!busy?<Feedback kind="action-required" title="上次上传还待核对" actions={<Button type="button" variant="outline" onClick={()=>void run(true)}>查询或重试原上传</Button>} details={diagnostic&&<pre>{JSON.stringify(diagnostic,null,2)}</pre>}><p>待确认：{pending.value.filename}。如需补传，请重新选择同一个文件。</p>{error&&<p>{error}</p>}</Feedback>:error&&!busy?<Feedback kind="error" details={diagnostic&&<pre>{JSON.stringify(diagnostic,null,2)}</pre>}>{error}</Feedback>:null}
  {!pending&&!recordFailure&&resultFailure&&<Feedback kind="warning" details={<pre>{JSON.stringify(feedbackDiagnostic(resultFailure),null,2)}</pre>}>上次上传的结果入口暂未读到，素材库里的既有内容不会被删除。</Feedback>}
  <ResultFeedback notice={status} onDismiss={value=>setStatus(current=>current===value?null:current)}/>
  <div className="flex flex-wrap gap-3"><Button type="button" disabled={busy||!!pending||!!recordFailure||!file||!!fileError} onClick={()=>void run()}>上传素材</Button>{last&&!resultFailure&&<Button type="button" variant="link" onClick={()=>w.navigatePath('/media/'+encodeURIComponent(last))}>查看上次上传</Button>}</div>
 </CardContent></Card>;
}
