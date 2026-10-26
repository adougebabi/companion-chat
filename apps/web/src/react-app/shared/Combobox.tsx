import {useId,useState} from 'react';
import {CheckIcon,ChevronsUpDownIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Popover,PopoverContent,PopoverTrigger} from '@/components/ui/popover';
import {Command,CommandInput,CommandList,CommandItem,CommandEmpty} from '@/components/ui/command';
export default function Combobox({id,label,value,options,onChange,disabled=false,allowCustom=false}:any){
 const [open,setOpen]=useState(false),[query,setQuery]=useState(''),generated=useId();
 const select=(next:string)=>{onChange(next);setOpen(false);setQuery('');};
 const entries=[...new Set([value,...options].filter(Boolean))] as string[];
 return <Popover open={open} onOpenChange={setOpen}><PopoverTrigger asChild><Button id={id||generated} type="button" variant="outline" role="combobox" aria-label={label} aria-expanded={open} disabled={disabled} className="w-full min-w-0 justify-between whitespace-normal"><span className="truncate">{value||'请选择'}</span><ChevronsUpDownIcon className="size-4 shrink-0"/></Button></PopoverTrigger><PopoverContent className="w-[var(--radix-popover-trigger-width)] min-w-64 p-0"><Command><CommandInput aria-label={'搜索'+label} placeholder={'搜索'+label} value={query} onValueChange={setQuery}/><CommandList><CommandEmpty>没有匹配项。</CommandEmpty>{allowCustom&&query.trim()&&!entries.includes(query.trim())&&<CommandItem value={'采用 '+query} onSelect={()=>select(query.trim())}>采用「{query.trim()}」</CommandItem>}{entries.map(entry=><CommandItem key={entry} value={entry} onSelect={()=>select(entry)}>{entry}<CheckIcon className={'ml-auto size-4 '+(entry===value?'opacity-100':'opacity-0')}/></CommandItem>)}</CommandList></Command></PopoverContent></Popover>;
}
