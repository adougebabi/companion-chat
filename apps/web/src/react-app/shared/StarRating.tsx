import {StarIcon} from 'lucide-react';
export default function StarRating({value,max=5}:{value:number;max?:number}){return <span className="inline-flex items-center gap-1" role="img" aria-label={'重要度 '+value+' / '+max}>{Array.from({length:max},(_,index)=><StarIcon key={index} aria-hidden="true" className={'size-4 '+(index<value?'fill-current text-primary':'text-muted-foreground')}/>)}</span>;}
