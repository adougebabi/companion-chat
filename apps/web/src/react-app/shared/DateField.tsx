import {useEffect,useState} from 'react';
import {CalendarIcon} from 'lucide-react';
import {zhCN} from 'react-day-picker/locale';
import {Button} from '@/components/ui/button';
import {Calendar,CalendarDayButton} from '@/components/ui/calendar';
import {Popover,PopoverContent,PopoverTrigger} from '@/components/ui/popover';
export function civilDate(value:string){if(!/^\d{4}-\d{2}-\d{2}$/.test(value||''))return undefined;const [y,m,d]=value.split('-').map(Number),date=new Date(y,m-1,d,12);return date.getFullYear()===y&&date.getMonth()===m-1&&date.getDate()===d?date:undefined;}
export const civilDateString=(date:Date)=>[date.getFullYear(),String(date.getMonth()+1).padStart(2,'0'),String(date.getDate()).padStart(2,'0')].join('-');
function DiaryDayButton(props:any){return <CalendarDayButton {...props} data-testid={'calendar-day-'+civilDateString(props.day.date)} className={props.modifiers.diary?'relative after:absolute after:bottom-0 after:left-1/2 after:size-1 after:rounded-full after:bg-primary':''}/>;}
export function DiaryCalendar({value,max,dates,onChange}:any){
 const selected=civilDate(value),[month,setMonth]=useState(selected||new Date());
 useEffect(()=>{const date=civilDate(value);if(date)setMonth(date);},[value]);
 return <div className="grid gap-3"><Calendar showOutsideDays={false} fixedWeeks locale={zhCN} mode="single" selected={selected} month={month} onMonthChange={setMonth} onSelect={date=>date&&onChange(civilDateString(date))} disabled={max?{after:civilDate(max)!}:undefined} modifiers={{diary:(dates||[]).map(civilDate).filter(Boolean)}} components={{DayButton:DiaryDayButton}} className="w-full [--cell-size:clamp(28px,5vw,36px)]"/><p className="text-xs text-muted-foreground">{dates?.length?'圆点标记已读取的日记；选择日期查看。':'尚未读到日记，暂时没有日期标记。'} 今天尚未结束，不能补写今天或未来的日记。</p></div>;
}
export default function DateField({value='',onChange,onInput,min,max,disabled,id,name,required,...props}:any){
 const [open,setOpen]=useState(false);
 return <span className="relative inline-flex w-full min-w-0"><Popover open={open} onOpenChange={setOpen}><PopoverTrigger asChild><Button {...props} id={id} type="button" variant="outline" role="combobox" aria-required={required} disabled={disabled} className="w-full min-w-0 justify-between gap-2"><span className="min-w-0 truncate" title={value||'选择日期'}>{value||'选择日期'}</span><CalendarIcon className="size-4 shrink-0"/></Button></PopoverTrigger><PopoverContent className="w-auto p-0"><Calendar locale={zhCN} mode="single" selected={civilDate(value)} defaultMonth={civilDate(value)} disabled={date=>!!(max&&civilDateString(date)>max||min&&civilDateString(date)<min)} onSelect={date=>{if(!date)return;const event={target:{value:civilDateString(date)},currentTarget:null};onChange?.(event);onInput?.(event);setOpen(false);}}/></PopoverContent></Popover>{name&&<input type="hidden" name={name} value={value}/>}</span>;
}
