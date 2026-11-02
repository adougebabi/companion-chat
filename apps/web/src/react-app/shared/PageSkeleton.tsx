import {Skeleton} from '@/components/ui/skeleton';

export default function PageSkeleton({label='正在读取页面…',contentOnly=false}:{label?:string;contentOnly?:boolean}) {
 return <div role="status" aria-label={label} aria-live="polite" data-testid="page-skeleton" className="grid w-full min-w-0 gap-6">
  <span className="sr-only">{label}</span>
  {!contentOnly&&<div aria-hidden="true" className="grid gap-4 py-2"><Skeleton className="h-3 w-40 bg-border/60"/><Skeleton className="h-8 w-64 max-w-full bg-border/60"/><Skeleton className="h-4 w-80 max-w-full bg-border/60"/></div>}
  <div aria-hidden="true" className="flex gap-4 border-b pb-4"><Skeleton className="h-8 w-24 bg-border/60"/><Skeleton className="h-8 w-24 bg-border/60"/><Skeleton className="h-8 w-24 bg-border/60"/></div>
  <div aria-hidden="true" className="grid gap-5 md:grid-cols-3">{[0,1,2].map(index=><div key={index} className="grid min-h-60 content-start gap-5 rounded-xl border bg-card p-6"><Skeleton className="h-5 w-3/5"/><Skeleton className="h-4 w-full"/><Skeleton className="h-4 w-4/5"/><Skeleton className="mt-5 h-20 w-full"/></div>)}</div>
 </div>;
}
