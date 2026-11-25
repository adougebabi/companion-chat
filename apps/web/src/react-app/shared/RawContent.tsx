import {useState,type ReactNode} from 'react';
import {Check,ChevronDown,Copy} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleContent,CollapsibleTrigger} from '@/components/ui/collapsible';

const asText=(value:unknown)=>typeof value==='string'?value:JSON.stringify(value,null,2);

/**
 * RawContent — collapsed by default; expands one recorded value (prompt,
 * response, error, protocol record) for reading and copying. Used by task
 * model-call rounds and diagnostics call details so long content never pushes
 * the primary actions out of reach.
 */
export default function RawContent({label,value,missing,children}:{label:string;value:unknown;missing:string;children?:ReactNode}){
 const [copy,setCopy]=useState<'idle'|'done'|'failed'>('idle');
 const absent=value===undefined||value===null||value==='';
 if(absent)return <p className="replica-raw-content-missing">{label}：{missing}</p>;
 const text=asText(value),lines=text.split('\n').length;
 async function copyValue(){
  try{await navigator.clipboard.writeText(text);setCopy('done');}catch{setCopy('failed');}
  setTimeout(()=>setCopy('idle'),2500);
 }
 return <Collapsible className="replica-raw-content">
  <div className="replica-raw-content-head">
   <CollapsibleTrigger asChild><Button type="button" variant="ghost" size="sm" className="replica-raw-content-trigger"><ChevronDown aria-hidden="true"/>{label}<span className="replica-raw-content-size">{lines>1?`${lines} 行`:`${text.length} 字`}</span></Button></CollapsibleTrigger>
   <Button type="button" variant="ghost" size="sm" onClick={()=>void copyValue()} aria-label={'复制'+label}>{copy==='done'?<Check aria-hidden="true"/>:<Copy aria-hidden="true"/>}{copy==='done'?'已复制':copy==='failed'?'复制失败':'复制'}</Button>
   <span className="sr-only" role="status">{copy==='done'?label+'已复制':copy==='failed'?label+'复制失败，请展开后手动选择':''}</span>
  </div>
  <CollapsibleContent className="replica-raw-content-body">{children??<pre className="replica-model-call-wire">{text}</pre>}</CollapsibleContent>
 </Collapsible>;
}
