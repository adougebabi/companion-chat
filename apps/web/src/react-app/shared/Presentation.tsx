import type {HTMLAttributes,ReactNode} from 'react';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {Empty} from '@/components/ui/empty';
import LoadingState from './LoadingState';
import {cn} from '@/lib/utils';

export function InlineFeedback({children,className,variant='destructive',...props}:HTMLAttributes<HTMLDivElement>&{variant?:'default'|'destructive'}){
 return <Alert {...props} variant={variant} className={cn('min-w-0',className)}><AlertDescription>{children}</AlertDescription></Alert>;
}
export function EmptyState({children,className,...props}:HTMLAttributes<HTMLDivElement>){return <Empty {...props} className={cn('border bg-card',className)}>{children}</Empty>;}
export function ReadState({children,role,...props}:HTMLAttributes<HTMLDivElement>){return <div {...props}><LoadingState label={children||'正在读取…'}/></div>;}
