import type {HTMLAttributes} from 'react';
import {cn} from '@/lib/utils';
export default function EntityCardGrid({className,...props}:HTMLAttributes<HTMLDivElement>){return <div {...props} className={cn('entity-card-grid',className)}/>;}
