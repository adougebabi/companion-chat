import {Skeleton} from '@/components/ui/skeleton';
export default function LoadingState({label='正在读取…'}:{label?:React.ReactNode}){return <div className="grid gap-3 py-4" role="status"><span className="text-sm text-muted-foreground">{label}</span><Skeleton className="h-5 w-2/3"/><Skeleton className="h-20 w-full"/></div>;}
