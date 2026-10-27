import {Dialog,DialogContent,DialogHeader,DialogTitle,DialogDescription} from '@/components/ui/dialog';
import {Badge} from '@/components/ui/badge';
import {CardFooter} from '@/components/ui/card';
import {userInstant} from '@/v3/presentation-time.mjs';
import Combobox from '../shared/Combobox';
import CursorControls from "../shared/CursorControls";
import {EmptyState,ReadState} from "@/react-app/shared/Presentation";
import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { useWorkspace } from '../page';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card,CardHeader,CardTitle,CardContent } from '@/components/ui/card';
import {Collapsible,CollapsibleTrigger,CollapsibleContent} from '@/components/ui/collapsible';
import {AspectRatio} from '@/components/ui/aspect-ratio';
import { ChoiceSelect, FieldInput, Label } from '../controls';
import MediaUpload from '../shared/MediaUpload';
import { errorMessage } from '@/v3/domain-presentation.mjs';
import type { BrowserMediaAsset } from '@fluctlight/browser-client';
export default function MediaPage({route}:{route:any}){
 const w=useWorkspace();useSyncExternalStore(w.subscribe,w.getSnapshot,w.getSnapshot);
 const [rows,setRows]=useState<BrowserMediaAsset[]>([]),[cursor,setCursor]=useState(''),[query,setQuery]=useState(''),[kind,setKind]=useState(''),[fid,setFid]=useState(()=>route.query.get('fid')||''),[origin,setOrigin]=useState('all'),[hidden,setHidden]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[uploadOpen,setUploadOpen]=useState(false);
 const previewTrigger=useRef<HTMLButtonElement|null>(null);
 const [previewAsset,setPreviewAsset]=useState<BrowserMediaAsset|null>(null);
 const failedRead=useRef<any>(null);
 const active=useRef(false),epoch=useRef(0),applied=useRef<any>(null),owner=w.state.session.actorId;
 const live=()=>active.current&&w.state.session?.actorId===owner;
 async function load(more=false,retry=false){
  const request=retry&&failedRead.current?failedRead.current:{more,options:more?applied.current:{fluctlightId:fid||undefined,scope:origin,kind:kind||undefined,q:query||undefined,hidden},cursor:more?cursor:undefined};
  const token=++epoch.current;setBusy(true);setError('');if(!request.more)applied.current=request.options;
  try{const page=await w.services.mediaClient.list({...request.options,cursor:request.cursor});if(!live()||token!==epoch.current)return;setRows(previous=>request.more?[...previous,...page.items]:page.items);setCursor(page.next_cursor||'');failedRead.current=null;}
  catch(cause){if(live()&&token===epoch.current){failedRead.current=request;setError(errorMessage(cause));}}finally{if(live()&&token===epoch.current)setBusy(false);}
 }
 const loadRef=useRef(load);loadRef.current=load;
 useEffect(()=>{active.current=true;void load();return()=>{active.current=false;epoch.current++;};},[w,owner]);

 const source=(asset:BrowserMediaAsset)=>({owner_upload:'我上传的素材',studio:'创作素材',fluctlight:'角色生成'})[asset.origin]||'已登记素材';
 const bytes=(value:any)=>{const size=Number(value);return Number.isFinite(size)?size>=1048576?(size/1048576).toFixed(1)+' MB':(size/1024).toFixed(1)+' KB':'大小未记录';};
 return <section className="media-library grid min-w-0 gap-5">
  <Collapsible open={uploadOpen} onOpenChange={setUploadOpen}><div className="row"><p className="muted">浏览你上传和角色生成的素材。画面内容不代表生活事件。</p><span className="spacer"/><CollapsibleTrigger asChild><Button>{uploadOpen?'收起上传':'上传文件'}</Button></CollapsibleTrigger></div><CollapsibleContent className="pt-4">{uploadOpen&&<MediaUpload onSaved={()=>{if(live())void loadRef.current();}}/>}</CollapsibleContent></Collapsible>
  <form className="media-filters grid min-w-0 items-end gap-4 sm:grid-cols-2 xl:grid-cols-4" role="search" onSubmit={event=>{event.preventDefault();void load();}}>
   <Label htmlFor="media-query">搜索素材<Input id="media-query" value={query} onChange={e=>setQuery(e.target.value)} placeholder="素材名称"/></Label>
   <Label htmlFor="media-role">角色<Combobox id="media-role" label="角色" value={fid} options={[{value:'',label:'全部角色'},...w.state.characters.map((character:any)=>({value:character.id,label:character.name}))]} onChange={setFid}/></Label>
   <Label htmlFor="media-kind">类型<ChoiceSelect id="media-kind" value={kind} onChange={(e:any)=>setKind(e.target.value)}><option value="">全部类型</option><option value="image">图片</option><option value="video">视频</option><option value="audio">音频</option></ChoiceSelect></Label>
   <Label htmlFor="media-origin">来源<ChoiceSelect id="media-origin" value={origin} onChange={(e:any)=>setOrigin(e.target.value)}><option value="all">全部来源</option><option value="owner_upload">我的上传</option><option value="studio">创作素材</option><option value="fluctlight">角色生成</option></ChoiceSelect></Label>
   <div className="flex min-h-11 flex-wrap items-center gap-4 sm:col-span-2 xl:col-span-4"><Label className="check-field" htmlFor="media-hidden"><FieldInput id="media-hidden" type="checkbox" checked={hidden} onChange={(e:any)=>setHidden(e.target.checked)}/>只看已隐藏</Label><Button type="submit" variant="outline" disabled={busy}>查询</Button></div>
  </form>
  <p className="muted">已读取 {rows.length} 项{cursor?'，还有更多':''}</p>
  <div className="media-grid">{rows.map(asset=><Card key={asset.id} className="min-w-0 overflow-hidden gap-4 py-0"><CardHeader className="pt-4"><CardTitle className="line-clamp-2 break-words">{asset.title||asset.source_filename||'未命名素材'}</CardTitle><p className="text-sm text-muted-foreground">{source(asset)}</p></CardHeader><CardContent className="px-4"><AspectRatio ratio={4/3} className="overflow-hidden rounded-lg bg-muted">{asset.kind==='image'&&asset.readable!==false?<button type="button" className="block h-full w-full" aria-label={'预览'+(asset.title||'图片')} onClick={event=>{previewTrigger.current=event.currentTarget;setPreviewAsset(asset);}}><img src={w.services.mediaClient.url(asset.id)} alt={asset.title||'图片素材'} className="h-full w-full object-cover" loading="lazy"/></button>:asset.kind==='video'?<video src={w.services.mediaClient.url(asset.id)} className="h-full w-full object-cover" controls preload="metadata"/>:<span className="flex h-full items-center justify-center text-sm text-muted-foreground">{asset.kind==='audio'?'音频素材':'素材暂不可预览'}</span>}</AspectRatio>{asset.kind==='audio'&&<audio className="mt-3 w-full" src={w.services.mediaClient.url(asset.id)} controls preload="metadata" aria-label={asset.title||'音频素材'}/>}<div className="mt-3 flex flex-wrap gap-2 text-sm text-muted-foreground"><Badge variant="outline">{({image:'图片',video:'视频',audio:'音频'})[asset.kind]||'素材'}</Badge><span>{bytes((asset as any).byte_size)}</span>{(asset as any).created_at&&<time title={userInstant((asset as any).created_at)}>{userInstant((asset as any).created_at)}</time>}</div></CardContent><CardFooter className="py-3"><Button type="button" variant="outline" className="w-full" aria-label={'查看'+(asset.title||'未命名素材')} onClick={()=>w.navigatePath('/media/'+encodeURIComponent(asset.id))}>素材详情与管理</Button></CardFooter></Card>)}</div>
  {busy?<ReadState role="status">正在读取素材…</ReadState>:!rows.length&&<EmptyState>还没有符合条件的素材，可以上传图片、音频或视频。</EmptyState>}
  <CursorControls label="素材" append={{loading:busy,hasMore:!!cursor,loadMore:()=>load(true),error:error,retry:()=>load(false,true)}}/>
  <Dialog open={!!previewAsset} onOpenChange={open=>{if(!open)setPreviewAsset(null);}}><DialogContent onCloseAutoFocus={event=>{if(previewTrigger.current?.isConnected){event.preventDefault();previewTrigger.current.focus({preventScroll:true});}}} className="react-workspace flex max-h-[90dvh] max-w-[min(90vw,1000px)] flex-col"><DialogHeader><DialogTitle>{previewAsset?.title||'图片预览'}</DialogTitle><DialogDescription>阅读素材原图，来源与管理信息保留在素材详情。</DialogDescription></DialogHeader>{previewAsset&&<img src={w.services.mediaClient.url(previewAsset.id)} alt={previewAsset.title||'图片'} className="min-h-0 max-h-[65dvh] w-full object-contain"/>}</DialogContent></Dialog>
 </section>;
}
