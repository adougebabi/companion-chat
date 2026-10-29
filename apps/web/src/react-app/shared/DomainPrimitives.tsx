import {useEffect,useLayoutEffect,useRef,useState,useCallback} from 'react';
import {createPortal} from 'react-dom';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {Sheet,SheetContent,SheetHeader,SheetTitle,SheetDescription,SheetTrigger} from '@/components/ui/sheet';
import {Collapsible,CollapsibleContent,CollapsibleTrigger} from '@/components/ui/collapsible';
import {ChevronDownIcon} from 'lucide-react';
import {Tabs,TabsList,TabsTrigger} from '@/components/ui/tabs';
import {Calendar,CalendarDayButton} from '@/components/ui/calendar';
import {Progress} from '@/components/ui/progress';
import {zhCN} from 'react-day-picker/locale';
import {civilDate,civilDateString} from './DateField';
import LoadingState from './LoadingState';
import {FieldInput,Textarea,ChoiceSelect} from '../controls';
import ControlHint from './ControlHint';
import {forwardRef,useImperativeHandle} from 'react';
import {Card} from '@/components/ui/card';
import {Alert} from '@/components/ui/alert';
import {Item} from '@/components/ui/item';
import {Empty} from '@/components/ui/empty';
import {Avatar,AvatarImage,AvatarFallback} from '@/components/ui/avatar';
import {ToggleGroup,ToggleGroupItem} from '@/components/ui/toggle-group';
import {Pagination,PaginationContent,PaginationItem} from '@/components/ui/pagination';
import {ChevronLeftIcon,ChevronRightIcon} from 'lucide-react';
import {cn} from '@/lib/utils';

/**
 * A bounded port for the remaining imperative domain controllers. The original
 * element is a nonvisual form/event carrier, never a second visible control.
 * FormData, named form members, exact controller listeners and CAS stay owned by
 * the domain. All visible controls are portals in the ONE application tree.
 * Only explicitly marked primitives are accepted; this does not interpret page
 * HTML, execute templates or infer React components from arbitrary elements.
 */
type Port={source:HTMLElement;target:HTMLElement;id:number;kind:string;content?:HTMLElement;summary?:HTMLElement;focused?:boolean;business?:string;generated?:Set<string>;variant?:'default'|'destructive';dispose:()=>void};
const presentationKinds=new Set(['card','alert','item','empty']);
const oldShellTokens=new Set(['card','banner','lrow','statebox','chip']);
// Pure presentation primitives apply their actual props to the ORIGINAL host.
// Do not move form children, clone listeners or alter querySelector ancestry.
const RetainedShell=forwardRef<HTMLElement,any>(function RetainedShell({source,port,className,...props},ref){
 useImperativeHandle(ref,()=>source,[source]);
 useLayoutEffect(()=>{
  const next=cn(className,port.business);port.generated=new Set(String(className||'').split(/\s+/).filter(token=>!port.business?.split(/\s+/).includes(token)));
  if(source.className!==next)source.className=next;
  for(const name of ['data-slot','data-size','data-variant','role'])if(props[name]!==undefined&&source.getAttribute(name)!==String(props[name]))source.setAttribute(name,String(props[name]));
 });
 return null;
});
function PresentationPort({port}:{port:Port}){
 const [,refresh]=useState(0),source=port.source;
 useLayoutEffect(()=>{const observer=new MutationObserver(()=>refresh(value=>value+1));observer.observe(source,{attributes:true,childList:true,subtree:true,characterData:true});return()=>observer.disconnect();},[source]);
 const classes=source.className.split(/\s+/);
 if(!port.variant)port.variant=source.getAttribute('role')==='alert'?'destructive':'default';
 if(classes.includes('banner'))port.variant=classes.includes('err')?'destructive':'default';
 port.business=classes.filter(token=>!oldShellTokens.has(token)&&!(port.kind==='alert'&&['err','info','warn','ok'].includes(token))&&!port.generated?.has(token)).join(' ');
 const host=<RetainedShell source={source} port={port}/>;
 if(port.kind==='card')return <Card asChild className="min-w-0 gap-4 p-5">{host}</Card>;
 if(port.kind==='alert')return <Alert asChild variant={port.variant||'default'} role={source.getAttribute('role')||((port.variant==='destructive')?'alert':'status')}>{host}</Alert>;
 if(port.kind==='empty')return <Empty asChild className="border bg-card">{host}</Empty>;
 return <Item asChild variant="outline" className="min-w-0">{host}</Item>;
}
let sequence=0;
const radioScopes=new WeakMap<object,number>();
function radioScope(source:HTMLInputElement){const owner=source.form||source.ownerDocument;if(!radioScopes.has(owner))radioScopes.set(owner,++sequence);return radioScopes.get(owner);}
function propertyPort(element:any,name:string,changed:()=>void){
 const own=Object.getOwnPropertyDescriptor(element,name);let proto=element,descriptor=own;
 while(!descriptor&&(proto=Object.getPrototypeOf(proto)))descriptor=Object.getOwnPropertyDescriptor(proto,name);
 if(!descriptor?.get||!descriptor?.set||own&&!own.configurable)return()=>{};
 Object.defineProperty(element,name,{configurable:true,get(){return descriptor!.get!.call(this);},set(value){descriptor!.set!.call(this,value);changed();}});
 return()=>{if(own)Object.defineProperty(element,name,own);else delete element[name];};
}
function useCarrier(port:Port){
 const [,update]=useState(0);const changed=()=>update(value=>value+1);
 useLayoutEffect(()=>{
  const source:any=port.source,restore=['value','checked','disabled','readOnly','required','hidden','open'].map(name=>propertyPort(source,name,changed));
  const observer=new MutationObserver(changed);observer.observe(source,{attributes:true,childList:true,subtree:true,characterData:true});
  source.addEventListener('input',changed);source.addEventListener('change',changed);source.addEventListener('invalid',changed);source.addEventListener('domain-sync',changed);
  const originalFocus=source.focus,originalSelection=source.setSelectionRange;source.focus=(options:any)=>(port.target.querySelector<HTMLElement>('[data-slot="input"],[data-slot="input-group-control"],textarea')||port.target.querySelector<HTMLElement>('button,a'))?.focus(options);
  if(originalSelection)source.setSelectionRange=(...args:any[])=>{originalSelection.apply(source,args);(port.target.querySelector('input,textarea') as any)?.setSelectionRange(...args);};
  if(port.focused){const start=source.selectionStart,end=source.selectionEnd;source.focus({preventScroll:true});if(start!==null&&start!==undefined)source.setSelectionRange?.(start,end);}
  return()=>{observer.disconnect();restore.forEach(fn=>fn());source.removeEventListener('input',changed);source.removeEventListener('change',changed);source.removeEventListener('invalid',changed);source.removeEventListener('domain-sync',changed);source.focus=originalFocus;if(originalSelection)source.setSelectionRange=originalSelection;};
 },[port]);
 return port.source as any;
}
function OpaqueContent({source}:{source:HTMLElement}){
 const target=useRef<HTMLSpanElement>(null);
 // Retain already escaped domain nodes. No HTML parsing, evaluation or second
 // renderer is involved; React owns only the surrounding interaction primitive.
 const signature=source.innerHTML;
 useLayoutEffect(()=>{target.current?.replaceChildren(...Array.from(source.childNodes).filter(node=>!(node instanceof HTMLElement&&node.getAttribute('aria-hidden')==='true'&&['⌄','⌃','›'].includes(node.textContent?.trim()||''))).map(node=>node.cloneNode(true)));},[source,signature]);
 return <span ref={target} className="domain-rich-label inline-flex min-w-0 flex-wrap items-center gap-2 [&>.spacer]:flex-1 [&_.badge]:shrink-0"/>;
}
function copiedAttributes(source:HTMLElement){return Object.fromEntries(Array.from(source.attributes).filter(({name})=>name.startsWith('aria-')&&!['aria-hidden','aria-pressed'].includes(name)).map(({name,value})=>[name,value]));}
function FieldPort({port}:{port:Port}){
 const source:any=port.source,type=source.tagName==='TEXTAREA'?'textarea':source.tagName==='SELECT'?'select':source.type;
 const changesImmediately=['checkbox','radio','select','date','datetime-local'].includes(type);
 const id=(source.id||'domain-field-'+port.id)+'--ui';
 useLayoutEffect(()=>{const label=source.closest('label');if(label)label.htmlFor=id;},[source,id]);
 const syncSelection=(event:any,carrierOnly=false)=>{try{if(event.target.selectionStart!==null){if(carrierOnly){const prototype=source.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;prototype.setSelectionRange.call(source,event.target.selectionStart,event.target.selectionEnd);}else source.setSelectionRange?.(event.target.selectionStart,event.target.selectionEnd);}}catch{}};
 // WebKit may focus a visible control when setSelectionRange is called. Blur
 // records selection on the hidden carrier only; explicit controller recovery
 // still uses the forwarding method installed by useCarrier.
 // Immediate-change controls already notified their controller. Repeating change
 // here can reenter a synchronous redraw when it removes the focused control.
 const blur=(event:any)=>{syncSelection(event,true);if(!changesImmediately)source.dispatchEvent(new Event('change',{bubbles:true}));source.dispatchEvent(new source.ownerDocument.defaultView.FocusEvent('focusout',{bubbles:true,relatedTarget:event.relatedTarget}));};
 const props:any={...copiedAttributes(source),'aria-label':source.getAttribute('aria-label')||source.closest('label')?.querySelector('.field-label')?.textContent?.replace(/\*$/,'').trim(),id,...(type==='radio'?{name:'domain-ui:'+(source.name||port.id)+':'+radioScope(source),form:'domain-visual-radios-'+radioScope(source),className:'size-4 rounded-full'}:{}),disabled:source.matches(':disabled')||!!source.readOnly&&['select','date','file','checkbox','radio'].includes(type),readOnly:source.readOnly,'aria-required':source.required&&!source.readOnly||undefined,'aria-invalid':source.getAttribute('aria-invalid')||undefined,'aria-describedby':source.getAttribute('aria-describedby')||undefined,placeholder:source.placeholder,min:source.min||undefined,max:source.max||undefined,minLength:source.minLength<0?undefined:source.minLength,maxLength:source.maxLength<0?undefined:source.maxLength,step:source.step||undefined,pattern:source.pattern||undefined,rows:source.rows||undefined};
 const commit=(value:any,checkbox=false)=>{
  if(!source.isConnected||source.matches(':disabled')||source.readOnly)return;
  if(checkbox)source.checked=value;else source.value=value;
  if(type==='radio')for(const peer of source.ownerDocument.querySelectorAll('input[data-domain-ui="field"][type="radio"]'))if(peer.name===source.name&&peer.form===source.form)peer.dispatchEvent(new Event('domain-sync'));
  source.dispatchEvent(new Event('input',{bubbles:true}));
  if(changesImmediately)source.dispatchEvent(new Event('change',{bubbles:true}));
 };
 if(type==='file')return <FieldInput {...props} type="file" accept={source.accept} multiple={source.multiple} onChange={(event:any)=>{if(!source.isConnected||source.disabled)return;source.files=event.target.files;source.dispatchEvent(new Event('change',{bubbles:true}));}}/>;
 const checkable=type==='checkbox'||type==='radio';
 const control=type==='select'?<ChoiceSelect {...props} value={source.value} onChange={(event:any)=>commit(event.target.value)}>{Array.from(source.options,(option:any)=><option key={option.index} value={option.value} disabled={option.disabled}>{option.textContent}</option>)}</ChoiceSelect>:type==='textarea'?<Textarea {...props} value={source.value} onChange={(event:any)=>{commit(event.target.value);syncSelection(event);}} onBlur={blur}/>:<FieldInput {...props} type={type} value={source.value} checked={source.checked} onChange={(event:any)=>{commit(checkable?event.target.checked:event.target.value,checkable);syncSelection(event);}} onBlur={blur}/>;
 return <ControlHint text={source.title}>{control}</ControlHint>;
}
function ButtonPort({port}:{port:Port}){
 const source:any=port.source,anchor=source.tagName==='A',classes=source.classList;
 const variant=classes.contains('on')||source.getAttribute('aria-pressed')==='true'?'secondary':classes.contains('danger')?'destructive':classes.contains('primary')||classes.contains('accent')?'default':classes.contains('ghost')||classes.contains('chip')?'ghost':'outline';
 const activate=(event:any)=>{event.preventDefault();event.stopPropagation();if(source.isConnected&&!source.disabled)source.click();};
 const props:any={...copiedAttributes(source),...(source.hasAttribute('aria-pressed')?{'aria-pressed':source.getAttribute('aria-pressed')==='true'}:{}),variant,disabled:source.disabled,className:'max-w-full whitespace-normal text-left',onClick:activate};
 return <ControlHint text={source.title}><Button {...props} type={anchor?undefined:'button'} asChild={anchor}>{anchor?<a href={source.getAttribute('href')||undefined} target={source.target||undefined} download={source.getAttribute('download')||undefined}><OpaqueContent source={source}/></a>:<OpaqueContent source={source}/>}</Button></ControlHint>;
}
function DisclosurePort({port}:{port:Port}){
 const source:any=port.source,target=useRef<HTMLDivElement>(null);
 const mountContent=useCallback((node:HTMLDivElement|null)=>{target.current=node;if(node&&port.content)node.append(port.content);},[port]);
 const sheet=source.dataset.domainSurface==='sheet';
 useLayoutEffect(()=>{if(port.content)target.current?.append(port.content);return()=>port.content?.remove();},[port]);
 useEffect(()=>{const reveal=()=>{source.open=true;};port.target.addEventListener('domain-reveal',reveal);return()=>port.target.removeEventListener('domain-reveal',reveal);},[port,source]);
 if(sheet)return <Sheet open={source.open} onOpenChange={open=>{source.open=open;}}><SheetTrigger asChild><Button type="button" variant="outline">{port.summary?<OpaqueContent source={port.summary}/>:"设置"}</Button></SheetTrigger><SheetContent className="react-workspace replica-overlay product-sheet gap-0 overflow-hidden"><SheetHeader className="shrink-0 border-b border-border pr-14"><SheetTitle>{source.dataset.surfaceTitle||port.summary?.textContent||'设置'}</SheetTitle><SheetDescription>调整当前日程与约定设置</SheetDescription></SheetHeader><div ref={mountContent} className="min-h-0 flex-1 overflow-y-auto p-5"/></SheetContent></Sheet>;
 return <Collapsible open={source.open} onOpenChange={open=>{source.open=open;}} data-domain-disclosure className="min-w-0"><CollapsibleTrigger asChild><Button type="button" variant="ghost" className="w-full justify-start whitespace-normal text-left"><span className="min-w-0 flex-1">{port.summary?<OpaqueContent source={port.summary}/>:"更多内容"}</span><ChevronDownIcon className={'size-4 shrink-0 transition-transform '+(source.open?'rotate-180':'')}/></Button></CollapsibleTrigger><CollapsibleContent forceMount hidden={!source.open} className="min-w-0 pt-3"><div ref={mountContent} className="min-w-0 break-words [&_pre]:max-h-96 [&_pre]:overflow-auto [&_pre]:whitespace-pre-wrap [&_pre]:break-all"/></CollapsibleContent></Collapsible>;
}
function InteractivePort({port}:{port:Port}){
 const source=useCarrier(port);port.target.hidden=source.hidden;
 if(port.kind==='avatar')return <Avatar className={source.classList.contains('lg')?'size-14':'size-8'} role="img" aria-label={source.getAttribute('aria-label')||'人物头像'}><AvatarImage src={source.querySelector('img')?.getAttribute('src')||undefined} alt=""/><AvatarFallback>{source.querySelector('.avatar-fallback')?.textContent||'人'}</AvatarFallback></Avatar>;
 if(port.kind==='toggle-group'){const items=[...source.querySelectorAll('button')] as HTMLButtonElement[],selected=items.findIndex(item=>item.classList.contains('on')||item.getAttribute('aria-pressed')==='true');return <ToggleGroup type="single" variant="outline" value={selected<0?'':String(selected)} onValueChange={key=>{if(key!=='')items[Number(key)]?.click();}} aria-label={source.getAttribute('aria-label')||'筛选记录'} className="max-w-full flex-wrap justify-start">{items.map((item,index)=><ToggleGroupItem key={index} value={String(index)} disabled={item.disabled}><OpaqueContent source={item}/></ToggleGroupItem>)}</ToggleGroup>;}
 if(port.kind==='pagination')return <Pagination aria-label="记录分页" className="justify-start"><PaginationContent className="flex-wrap">{[...(source as HTMLElement).querySelectorAll<HTMLButtonElement>('button')].map((item,index)=><PaginationItem key={index}><Button type="button" variant="outline" disabled={item.disabled} onClick={()=>item.isConnected&&item.click()}>{item.hasAttribute('data-page-previous')?<ChevronLeftIcon/>:<ChevronRightIcon/>}{item.textContent}</Button></PaginationItem>)}</PaginationContent></Pagination>;
 if(port.kind==='calendar')return <Calendar locale={zhCN} mode="single" selected={civilDate(source.dataset.selected)} month={civilDate(source.dataset.monthState+'-01')} onMonthChange={date=>source.domainMonthChange?.(civilDateString(date).slice(0,7))} onSelect={date=>date&&source.domainDateSelect?.(civilDateString(date))} modifiers={{recorded:JSON.parse(source.dataset.recordDates||'[]').map(civilDate).filter(Boolean)}} components={{DayButton:props=><CalendarDayButton {...props} className={props.modifiers.recorded?'relative after:absolute after:bottom-0 after:left-1/2 after:size-1 after:rounded-full after:bg-primary':''}/>}} className="w-full"/>;
 if(port.kind==='tabs'){const items=[...source.children] as HTMLElement[],selected=items.findIndex(item=>item.classList.contains('on')||item.getAttribute('aria-current')==='page');return <Tabs value={selected<0?'':String(selected)} onValueChange={key=>items[Number(key)]?.click()}><TabsList className="h-auto group-data-horizontal/tabs:h-auto max-w-full flex-wrap">{items.map((item,index)=><TabsTrigger key={index} value={String(index)} disabled={(item as any).disabled}><OpaqueContent source={item}/></TabsTrigger>)}</TabsList></Tabs>;}
 if(port.kind==='progress')return <Progress value={Number(source.dataset.value)} aria-label={source.getAttribute('aria-label')||'处理阶段'}/>;
 if(port.kind==='skeleton')return <LoadingState label={source.textContent||undefined}/>;
 return port.kind==='field'?<FieldPort port={port}/>:port.kind==='disclosure'?<DisclosurePort port={port}/>:port.kind==='badge'?<Badge variant="secondary">{source.textContent}</Badge>:<ButtonPort port={port}/>;
}
export default function DomainPrimitives({root}:{root:HTMLElement|null}){
 const [ports,setPorts]=useState<Port[]>([]);
 useLayoutEffect(()=>{
  if(!root)return;const known=new Map<HTMLElement,Port>();let active=true,scheduled=false;
  function scan(){
   scheduled=false;if(!active)return;let changed=false;
   for(const [node,port] of known)if(!root!.contains(node)){port.dispose();known.delete(node);changed=true;}
   for(const source of root!.querySelectorAll<HTMLElement>('[data-domain-ui]')){
    // Ancestor disclosures move their original children into detached content.
    // Admit descendants only after that content has mounted back under root.
    if(!root!.contains(source))continue;
    if(known.has(source)||source.matches('input[type="hidden"]')||source.parentElement?.closest('[data-domain-ui="tabs"],[data-domain-ui="calendar"],[data-domain-ui="toggle-group"],[data-domain-ui="pagination"]')||source.closest('[data-domain-react-control="field"],[data-domain-react-control="button"],[data-domain-react-control="badge"],[data-domain-react-control="tabs"],[data-domain-react-control="calendar"],[data-domain-react-control="toggle-group"],[data-domain-react-control="pagination"],[data-domain-react-control="avatar"]'))continue;
    const kind=source.dataset.domainUi!;
    if(!['field','button','disclosure','badge','tabs','calendar','progress','skeleton','avatar','toggle-group','pagination',...presentationKinds].includes(kind))continue;
    const target=document.createElement('span');target.dataset.domainReactControl=kind;target.className=kind==='field'?'contents':kind==='disclosure'?'block min-w-0':'inline-flex min-w-0';
    const originalClass=source.className,originalRole=source.getAttribute('role'),originalSlot=source.getAttribute('data-slot'),originalAria=source.getAttribute('aria-hidden'),originalTab=source.getAttribute('tabindex');
    Object.defineProperty(target,'domainSource',{get(){const active=source.ownerDocument.activeElement as HTMLInputElement|null;if(active&&target.contains(active)&&active.selectionStart!==undefined&&active.selectionStart!==null){(source as any).selectionStart=active.selectionStart;(source as any).selectionEnd=active.selectionEnd;}return source;}});
    const port:Port={source,target,id:++sequence,kind,focused:document.activeElement===source,dispose:()=>{source.removeAttribute('data-domain-carrier');if(presentationKinds.has(kind)){source.className=originalClass;for(const [name,value]of [['role',originalRole],['data-slot',originalSlot]])if(value===null)source.removeAttribute(name!);else source.setAttribute(name!,value!);}if(originalAria===null)source.removeAttribute('aria-hidden');else source.setAttribute('aria-hidden',originalAria);if(originalTab===null)source.removeAttribute('tabindex');else source.setAttribute('tabindex',originalTab);target.remove();}};
    if(kind==='disclosure'){
     port.summary=source.querySelector(':scope > summary') as HTMLElement;
     port.content=document.createElement('div');port.content.className='grid min-w-0 gap-3';
     for(const node of [...source.childNodes])if(node!==port.summary)port.content.append(node);
    }
    if(!presentationKinds.has(kind)){source.setAttribute('data-domain-carrier','');source.setAttribute('aria-hidden','true');source.tabIndex=-1;}else target.className='contents';source.after(target);known.set(source,port);changed=true;
   }
   if(changed)setPorts([...known.values()]);
  }
  const observer=new MutationObserver(()=>{if(!scheduled){scheduled=true;queueMicrotask(scan);}});
  observer.observe(root,{childList:true,subtree:true});scan();
  return()=>{active=false;observer.disconnect();for(const port of known.values())port.dispose();};
 },[root]);
 return ports.map(port=>createPortal(presentationKinds.has(port.kind)?<PresentationPort port={port}/>:<InteractivePort port={port}/>,port.target,port.id));
}
