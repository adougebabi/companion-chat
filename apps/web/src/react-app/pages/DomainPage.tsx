import { useEffect, useRef, useState } from 'react';
import { useWorkspace } from '../page';
import { Button } from '@/components/ui/button';
import { approvedHref } from '@/approved-ui/routing.mjs';
import { errorMessage } from '@/v3/domain-presentation.mjs';
import { closeOverlays } from '@/v3/proto/components/overlay.js';
import { installFieldValidation } from '@/v3/proto/components/forms.js';
import { refreshProduct } from '@/v3/providers/real';

// Existing unique domain tools share this React workspace. They own no app,
// router or stylesheet; their remaining custom controls are tracked separately.
export default function DomainPage({route}: {route:any}) {
  const w=useWorkspace(),host=useRef<HTMLDivElement>(null),[failure,setFailure]=useState(''),[revision,refresh]=useState(0);
  const owner=w.state.session?.actorId;
  useEffect(()=>{
    if(!host.current)return;
    let active=true,dispose:void|(()=>void),validation:void|(()=>void);
    const target=document.createElement('div');host.current.replaceChildren(target);setFailure('');
    const live=()=>active&&owner===w.state.session?.actorId;
    async function open(){
      try{
        const feature=route.feature;
        const render=await feature.load();
        if(route.fid)await refreshProduct();
        if(!live())return;
        const context={charId:route.fid||w.state.selectedId,params:{...feature.params},query:Object.fromEntries(route.query),path:route.path,raw:route.path+(route.query.size?'?'+route.query:''),def:{title:feature.title,mockSection:feature.params.section},navigate:(path:string)=>w.navigatePath(String(path).replace(/^#/,'')),refresh:()=>{if(live())refresh(value=>value+1);}};
        const cleanup=feature.render==='renderRealGovernance'&&context.query.tab==='developing'
          ? (await import('@/v3-domain/self.js')).renderRealSelfGovernance(target,context)
          : await render(target,context);
        if(!live()){cleanup?.();return;}
        dispose=cleanup;validation=installFieldValidation(target);
      }catch(cause){if(live())setFailure(errorMessage(cause,'这个功能暂时无法读取，请重试。'));}
    }
    void open();
    return()=>{active=false;dispose?.();validation?.();closeOverlays();};
  },[w,owner,route,revision]);
  return <section className="approved-domain" data-testid="domain-workspace"><header className="domain-heading"><Button variant="outline" onClick={()=>w.navigatePath(approvedHref(route.page,route.fid).slice(1))}>返回{route.page}</Button><h2>{route.feature.title}</h2></header>{failure&&<div className="inline-error" role="alert">{failure}<Button variant="outline" onClick={()=>refresh(value=>value+1)}>重新读取</Button></div>}<div className="domain-content" ref={host}/></section>;
}
