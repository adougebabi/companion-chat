import {useId,useState} from 'react';
import {CheckIcon,ChevronsUpDownIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {Popover,PopoverContent,PopoverTrigger} from '@/components/ui/popover';
import {Command,CommandInput,CommandList,CommandItem,CommandEmpty} from '@/components/ui/command';
type Option=string|{value:string;label:string};
type Props={id?:string;label:string;value:string;options:Option[];onChange:(value:string)=>void;disabled?:boolean;allowCustom?:boolean;required?:boolean;placeholder?:string;emptyText?:string};
export default function Combobox({id,label,value,options,onChange,disabled=false,allowCustom=false,required=false,placeholder='请选择',emptyText='没有匹配项。'}:Props){
 const [open,setOpen]=useState(false),[query,setQuery]=useState(''),generated=useId();
 const select=(next:string)=>{onChange(next);setOpen(false);setQuery('');};
 const entries=Array.from(new Map(options.map(option=>{const entry=typeof option==='string'?{value:option,label:option}:option;return [entry.value,entry];})).values());
 if(value&&!entries.some(entry=>entry.value===value))entries.unshift({value,label:value});
 const selected=entries.find(entry=>entry.value===value),custom=query.trim();
 return <Popover open={open} onOpenChange={next=>{setOpen(next);if(!next)setQuery('');}}><PopoverTrigger asChild><Button id={id||generated} type="button" variant="outline" role="combobox" aria-label={label} aria-expanded={open} aria-required={required||undefined} aria-controls={generated+'-options'} disabled={disabled} className="w-full min-w-0 justify-between whitespace-normal bg-background"><span className="min-w-0 truncate">{selected?.label||placeholder}</span><ChevronsUpDownIcon className="size-4 shrink-0"/></Button></PopoverTrigger><PopoverContent className="w-[var(--radix-popover-trigger-width)] max-w-[calc(100vw-2rem)] p-0"><Command><CommandInput aria-label={'搜索'+label} placeholder={allowCustom?'搜索或输入'+label:'搜索'+label} value={query} onValueChange={setQuery}/><CommandList id={generated+'-options'}><CommandEmpty>{emptyText}</CommandEmpty>{allowCustom&&custom&&!entries.some(entry=>entry.value===custom)&&<CommandItem value={'采用 '+custom} onSelect={()=>select(custom)}>采用「{custom}」</CommandItem>}{entries.map(entry=><CommandItem key={entry.value} value={entry.value+' '+entry.label} onSelect={()=>select(entry.value)} className="min-w-0"><span className="min-w-0 break-all">{entry.label}</span><CheckIcon aria-hidden="true" className={'ml-auto size-4 shrink-0 '+(entry.value===value?'opacity-100':'opacity-0')}/></CommandItem>)}</CommandList></Command></PopoverContent></Popover>;
}
