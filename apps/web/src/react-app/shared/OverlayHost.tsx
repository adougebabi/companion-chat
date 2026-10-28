import {useEffect,useLayoutEffect,useRef,useSyncExternalStore} from 'react';
import {Button} from '@/components/ui/button';
import {AlertDialog,AlertDialogContent,AlertDialogTitle,AlertDialogDescription,AlertDialogFooter,AlertDialogCancel,AlertDialogAction} from '@/components/ui/alert-dialog';
import {Sheet,SheetContent,SheetTitle,SheetDescription} from '@/components/ui/sheet';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/ui/dialog';
import {closeOverlay,closeAllOverlays,overlaySnapshot,subscribeOverlays,type OverlayRequest} from '@/state/overlays';
import {useWorkspace} from '../page';
import DomainPrimitives from './DomainPrimitives';
function RetainedContent({node}:{node:HTMLElement}){const host=useRef<HTMLDivElement>(null);useLayoutEffect(()=>{host.current?.append(node);return()=>node.remove();},[node]);return <><div ref={host} className="min-h-0 overflow-y-auto"/><DomainPrimitives root={node}/></>;}
function restore(request:OverlayRequest,event:Event){
 event.preventDefault();const target=request.trigger;if(!target?.isConnected)return;
 const focus=()=>{if(!target.isConnected||target.matches(':disabled')||target.closest('[inert]'))return false;target.focus({preventScroll:true});return true;};
 requestAnimationFrame(()=>{if(focus())return;const observer=new MutationObserver(()=>{if(focus()){observer.disconnect();clearTimeout(timer);}});observer.observe(target,{attributes:true,attributeFilter:['disabled','inert']});const timer=setTimeout(()=>observer.disconnect(),1000);});
}

export default function OverlayHost(){
 const w=useWorkspace();useSyncExternalStore(w?.subscribe||(()=>()=>{}),w?.getSnapshot||(()=>null),w?.getSnapshot||(()=>null));
 const owner=w?.state.session?.actorId,requests=useSyncExternalStore(subscribeOverlays,overlaySnapshot,overlaySnapshot),previousOwner=useRef(owner);
 useEffect(()=>{if(previousOwner.current!==owner){closeAllOverlays();previousOwner.current=owner;}},[owner]);
 useEffect(()=>()=>closeAllOverlays(),[]);
 return requests.filter(request=>!request.owner||!owner||request.owner===owner).map(request=>request.kind==='confirm'?<AlertDialog key={request.id} open onOpenChange={open=>!open&&closeOverlay(request.id)}><AlertDialogContent className="react-workspace replica-overlay" onCloseAutoFocus={event=>restore(request,event)}><AlertDialogTitle>{request.title}</AlertDialogTitle><AlertDialogDescription className="whitespace-pre-wrap break-words">{request.text}</AlertDialogDescription><AlertDialogFooter><AlertDialogCancel onClick={()=>closeOverlay(request.id)}>取消</AlertDialogCancel><AlertDialogAction asChild><Button variant={request.destructive?'destructive':'default'} data-test="confirm-ok" onClick={()=>closeOverlay(request.id,true)}>{request.okText||'确认'}</Button></AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>:request.kind==='sheet'?<Sheet key={request.id} open onOpenChange={open=>!open&&closeOverlay(request.id)}><SheetContent className={'react-workspace replica-overlay product-sheet flex min-w-0 flex-col p-6 '+(request.wide?'replica-overlay-wide':'')} onCloseAutoFocus={event=>restore(request,event)}><SheetTitle>{request.title}</SheetTitle><SheetDescription className="sr-only">查看资料并完成当前操作。</SheetDescription>{request.node&&<RetainedContent node={request.node}/>}</SheetContent></Sheet>:<Dialog key={request.id} open onOpenChange={open=>!open&&closeOverlay(request.id)}><DialogContent className="react-workspace replica-overlay flex max-h-[90dvh] flex-col" onCloseAutoFocus={event=>restore(request,event)}><DialogTitle>{request.title}</DialogTitle><DialogDescription className="sr-only">查看资料并完成当前操作。</DialogDescription>{request.node&&<RetainedContent node={request.node}/>}</DialogContent></Dialog>);
}
