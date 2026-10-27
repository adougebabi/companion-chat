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
 return <section className="media-library grid min-w-0 gap-4"><Collapsible open={uploadOpen} onOpenChange={setUploadOpen}><div className="row"><p className="muted">上传与角色生成的素材都在这里。画面和素材来源分别保留，不作为已发生的生活事实。</p><span className="spacer"/><CollapsibleTrigger asChild><Button>{uploadOpen?'收起上传':'上传素材'}</Button></CollapsibleTrigger></div><CollapsibleContent className="pt-4">{uploadOpen&&<MediaUpload onSaved={()=>{if(live())void loadRef.current();}}/>}</CollapsibleContent></Collapsible><form className="domain-filter row" role="search" onSubmit={event=>{event.preventDefault();void load();}}><Label htmlFor="media-query">搜索素材<Input id="media-query" value={query} onChange={e=>setQuery(e.target.value)} placeholder="素材名称"/></Label><Label htmlFor="media-role">角色<ChoiceSelect id="media-role" value={fid} onChange={(e:any)=>setFid(e.target.value)}><option value="">全部角色</option>{w.state.characters.map((character:any)=><option key={character.id} value={character.id}>{character.name}</option>)}</ChoiceSelect></Label><Label htmlFor="media-kind">类型<ChoiceSelect id="media-kind" value={kind} onChange={(e:any)=>setKind(e.target.value)}><option value="">全部类型</option><option value="image">图片</option><option value="video">视频</option><option value="audio">音频</option></ChoiceSelect></Label><Label htmlFor="media-origin">来源<ChoiceSelect id="media-origin" value={origin} onChange={(e:any)=>setOrigin(e.target.value)}><option value="all">全部来源</option><option value="owner_upload">我的上传</option><option value="studio">创作素材</option><option value="fluctlight">角色生成</option></ChoiceSelect></Label><Label className="check-field" htmlFor="media-hidden"><FieldInput id="media-hidden" type="checkbox" checked={hidden} onChange={(e:any)=>setHidden(e.target.checked)}/>只看已隐藏</Label><Button type="submit" variant="outline" disabled={busy}>查询</Button></form><p className="muted">已读取 {rows.length} 项{cursor?'，还有更多':''}</p><div className="media-grid">{rows.map(asset=><Card key={asset.id} className="min-w-0 overflow-hidden"><CardContent className="px-4"><AspectRatio ratio={4/3} className="overflow-hidden rounded-lg bg-muted">{asset.kind==='image'&&asset.readable!==false?<img src={w.services.mediaClient.url(asset.id)} alt={asset.title||'图片素材'} className="h-full w-full object-contain" loading="lazy"/>:asset.kind==='video'?<video src={w.services.mediaClient.url(asset.id)} className="h-full w-full object-contain" preload="metadata"/>:<span className="flex h-full items-center justify-center text-sm text-muted-foreground">{asset.kind==='audio'?'音频素材':'素材暂不可预览'}</span>}</AspectRatio></CardContent><CardHeader><CardTitle><Button variant="link" className="h-auto min-w-0 justify-start whitespace-normal p-0 text-left" aria-label={'查看'+(asset.title||'未命名素材')} onClick={()=>w.navigatePath('/media/'+encodeURIComponent(asset.id))}>{asset.title||asset.source_filename||'未命名素材'}</Button></CardTitle><p className="text-sm text-muted-foreground">{source(asset)}</p></CardHeader>{asset.kind==='audio'&&<CardContent><audio className="w-full" src={w.services.mediaClient.url(asset.id)} controls preload="metadata" aria-label={asset.title||'音频素材'}/></CardContent>}</Card>)}</div>{busy?<ReadState role="status">正在读取素材…</ReadState>:!rows.length&&<EmptyState >还没有符合条件的素材，可以上传图片、音频或视频。</EmptyState>}<CursorControls label="素材" append={{loading:busy,hasMore:!!cursor,loadMore:()=>load(true),error:error,retry:()=>load(false,true)}}/></section>;
}
