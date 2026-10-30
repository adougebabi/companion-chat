import type {HTMLAttributes,ReactNode} from 'react';
import {useEffect,useRef} from 'react';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {Empty,EmptyMedia} from '@/components/ui/empty';
import {Button} from '@/components/ui/button';
import {Collapsible,CollapsibleContent,CollapsibleTrigger} from '@/components/ui/collapsible';
import {CircleAlert,Info,CheckCircle2,Clock3,ListTodo,TriangleAlert,ChevronDown,X,Leaf} from 'lucide-react';
import LoadingState from './LoadingState';
import {cn} from '@/lib/utils';

export type FeedbackKind='information'|'action-required'|'warning'|'error'|'success'|'progress';
export type ResultNotice=null|{kind:FeedbackKind;text:string;actions?:ReactNode};
const icons={information:Info,'action-required':ListTodo,warning:TriangleAlert,error:CircleAlert,success:CheckCircle2,progress:Clock3};
export type FeedbackProps=Omit<HTMLAttributes<HTMLDivElement>,'title'> & {
 kind:FeedbackKind;title?:ReactNode;actions?:ReactNode;details?:ReactNode;detailsLabel?:string;onDismiss?:()=>void;
};
export function Feedback({kind,title,children,actions,details,detailsLabel='查看技术详情',onDismiss,className,role,...props}:FeedbackProps){
 const Icon=icons[kind],liveRole=role||(kind==='error'?'alert':['progress','success','warning'].includes(kind)?'status':'note');
 return <Alert {...props} variant={kind==='error'?'destructive':'default'} role={liveRole} aria-live={liveRole==='alert'?'assertive':liveRole==='status'?'polite':undefined} data-feedback-kind={kind} className={cn('replica-feedback min-w-0',onDismiss&&'replica-feedback-dismissible',className)}>
  <Icon aria-hidden="true"/>
  <div className="replica-feedback-content">
   {title&&<div className="replica-feedback-title">{title}</div>}
   {children&&<AlertDescription>{children}</AlertDescription>}
   {actions&&<div className="replica-feedback-actions">{actions}</div>}
   {details&&<Collapsible className="replica-feedback-details" aria-live="off"><CollapsibleTrigger asChild><Button type="button" variant="ghost" size="sm">{detailsLabel}<ChevronDown size={13}/></Button></CollapsibleTrigger><CollapsibleContent>{details}</CollapsibleContent></Collapsible>}
  </div>
  {onDismiss&&<Button type="button" variant="ghost" size="icon" className="replica-feedback-dismiss" aria-label="关闭本次提示" onClick={onDismiss}><X size={13}/></Button>}
 </Alert>;
}
export function ResultFeedback({notice,onDismiss}:{notice:ResultNotice;onDismiss:(notice:NonNullable<ResultNotice>)=>void}){
 const dismiss=useRef(onDismiss);dismiss.current=onDismiss;
 useEffect(()=>{if(!notice||!['success','information'].includes(notice.kind))return;const timer=setTimeout(()=>dismiss.current(notice),6000);return()=>clearTimeout(timer);},[notice]);
 return notice?<Feedback kind={notice.kind} actions={notice.actions} onDismiss={['success','information'].includes(notice.kind)?()=>onDismiss(notice):undefined}>{notice.text}</Feedback>:null;
}
// Existing callers retain destructive error emphasis until their business
// owner explicitly classifies them. A neutral variant alone never means success.
export function InlineFeedback({children,className,variant='destructive',kind,...props}:HTMLAttributes<HTMLDivElement>&{variant?:'default'|'destructive';kind?:FeedbackKind}){
 return <Feedback {...props} kind={kind||(variant==='destructive'?'error':'information')} className={className}>{children}</Feedback>;
}
export function EmptyState({children,className,variant='default',...props}:HTMLAttributes<HTMLDivElement>&{variant?:'default'|'page'}){return <Empty {...props} data-empty-variant={variant} className={cn(variant==='page'?'replica-page-empty':'border bg-card',className)}>{variant==='page'&&<EmptyMedia><Leaf size={25} aria-hidden="true"/></EmptyMedia>}{children}</Empty>;}
export function ReadState({children,role,...props}:HTMLAttributes<HTMLDivElement>){return <div {...props}><LoadingState label={children||'正在读取…'}/></div>;}
export function ReaderFeedback({source,label,retryLabel}:{source:any;label:string;retryLabel?:string}){
 if(source.busy)return source.ready?<Feedback kind="progress">正在重新读取{label}，上次确认的内容仍保留。</Feedback>:null;
 if(!source.error)return null;
 const retained=source.ready||source.items?.length>0;
 return <Feedback kind={retained?'warning':'error'} title={label+'暂未读全'} details={source.failure&&<pre>{JSON.stringify(source.failure,null,2)}</pre>} actions={typeof source.retry==='function'&&<Button type="button" variant="outline" onClick={source.retry}>{retryLabel||'重新读取'+label}</Button>}>
  <p>{source.error}</p>{retained&&<p>上次确认的内容仍保留。</p>}
 </Feedback>;
}
