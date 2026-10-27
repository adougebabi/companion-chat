import {useEffect,useRef,useState} from 'react';
import DateField from './DateField';
import {Input} from '@/components/ui/input';
import {InputGroup,InputGroupAddon} from '@/components/ui/input-group';
import {ClockIcon} from 'lucide-react';
export default function DateTimeField({value='',onChange,onInput,name,id,disabled,readOnly,min,max,...props}:any){
 const [parts,setParts]=useState(()=>String(value||'').split('T')),sent=useRef(String(value||''));
 useEffect(()=>{const next=String(value||'');if(next!==sent.current){sent.current=next;setParts(next.split('T'));}},[value]);
 const label=props['aria-label']||'北京时间';
 const accessibility={'aria-invalid':props['aria-invalid'],'aria-describedby':props['aria-describedby'],'aria-required':props['aria-required']||props.required};
 const change=(index:number,text:string)=>{const next=[parts[0]||'',parts[1]||''];next[index]=text;setParts(next);const combined=next[0]&&next[1]?next.join('T'):'';sent.current=combined;const event={target:{value:combined},currentTarget:null};onChange?.(event);onInput?.(event);};
 return <span className="grid w-full min-w-0 gap-2 sm:grid-cols-2"><DateField {...accessibility} id={id} aria-label={label+'日期'} value={parts[0]||''} min={min?.slice(0,10)} max={max?.slice(0,10)} disabled={disabled||readOnly} onChange={(e:any)=>change(0,e.target.value)}/><InputGroup className="h-auto min-h-11"><InputGroupAddon><ClockIcon/></InputGroupAddon><Input {...accessibility} type="text" inputMode="numeric" data-slot="input-group-control" placeholder="HH:mm" className="min-w-0 border-0 shadow-none focus-visible:ring-0" pattern="(?:[01][0-9]|2[0-3]):[0-5][0-9](?::[0-5][0-9](?:[.][0-9]+)?)?" aria-label={label+'时间'} value={parts[1]||''} disabled={disabled||readOnly} onChange={(e:any)=>change(1,e.target.value)}/></InputGroup>{name&&<input type="hidden" name={name} value={value||''}/>}</span>;
}
