import {useId,useRef,useState} from 'react';
import {Button} from '@/components/ui/button';
import {Label,FieldInput,Textarea,ChoiceSelect,Disclosure} from '../controls';
import {semanticLabel} from '@/v3/proto/components/semantic-fields.js';
const valueKind=(value:any)=>value===null?'null':Array.isArray(value)?'array':typeof value;
const textField=(field:string)=>/(?:text|content|description|background|biography|notes|instruction|reason|rules|summary|guidance|prompt|layout|changes|json|policy|dependencies|order|criteria|motivation|observations|feedback|note|recoveryPolicy|sourceReviews|reviewedGoalIds)$/.test(field);
const labels:Record<string,string>={title:'标题',content:'内容',text:'正文',summary:'摘要',constraints:'约束条件',category:'分类',slot:'穿着部位',ownership:'归属',availability:'可用状态',identity:'身份资料',personality:'性格',behavioral_policy:'表达与相处',life_profile:'生活设定',preferences:'偏好',traits:'性格倾向',notes:'补充说明',schema:'格式',key:'局部引用键',name:'名称',description:'描述',background:'背景',rules:'规则',flavor:'风格',floor:'楼层',layout:'布局',members:'搭配成员',resource_id:'来源资源',resource_version:'来源版本',atmosphere:'氛围',furniture:'固定陈设',venues:'场地分类',places:'具体地点'};
export default function RecordEditor({value,onChange,disabled=false,fieldLabels={},exclude=[]}:any){
 const entries=Object.entries(value||{}).filter(([key])=>!exclude.includes(key));
 const name=(key:string)=>fieldLabels[key]||labels[key]||semanticLabel(key);
 const render=([key,item]:[string,unknown])=><ValueEditor key={key} value={item} field={key} label={name(key)==='自定义字段'?key:name(key)} disabled={disabled} onChange={(next:any)=>onChange(Object.fromEntries(Object.entries(value).map(([name,before])=>[name,name===key?next:before])))}/>;
 const extra=entries.filter(([key])=>name(key)==='自定义字段');
 return <div className="grid min-w-0 gap-4">{entries.filter(([key])=>name(key)!=='自定义字段').map(render)}{extra.length>0&&<Disclosure title="其他扩展字段"><p className="text-sm text-muted-foreground">保留提供方原字段名与内容，按需展开核对。</p><div className="grid gap-4">{extra.map(render)}</div></Disclosure>}</div>;
}

function ValueEditor({value,field,label,disabled,onChange}:any){
 const id=useId(),[kind,setKind]=useState(()=>valueKind(value)),rows=useRef({keys:Array.from({length:Array.isArray(value)?value.length:0},(_,index)=>index),next:Array.isArray(value)?value.length:0});
 if(kind==='object')return <Disclosure title={label}><div className="border-l pl-4"><RecordEditor value={value} onChange={onChange} disabled={disabled}/></div></Disclosure>;
 if(kind==='array'){
  const items=Array.isArray(value)?value:[];
  while(rows.current.keys.length<items.length)rows.current.keys.push(rows.current.next++);
  return <Disclosure title={label+'（'+items.length+' 项）'}><div className="grid gap-4">{items.map((item:any,index:number)=><div key={rows.current.keys[index]} className="grid gap-3 rounded-lg border p-3"><ValueEditor value={item} field={field} label={'第 '+(index+1)+' 项'} disabled={disabled} onChange={(next:any)=>onChange(items.map((before:any,i:number)=>i===index?next:before))}/><Button type="button" variant="outline" disabled={disabled} onClick={()=>{rows.current.keys.splice(index,1);onChange(items.filter((_item:any,i:number)=>i!==index));}}>移除此项</Button></div>)}<div className="flex flex-wrap gap-2">{[['文字',''],['数值',0],['对象',{}]].map(([name,initial])=><Button key={String(name)} type="button" variant="outline" disabled={disabled} onClick={()=>{rows.current.keys.push(rows.current.next++);onChange([...items,initial]);}}>添加{String(name)}项</Button>)}</div></div></Disclosure>;
 }
 if(kind==='null'||kind==='undefined')return <div className="grid gap-2"><Label htmlFor={id}>{label}</Label><ChoiceSelect id={id} value="" disabled={disabled} aria-label={label+'的补充类型'} onChange={(event:any)=>{const next=event.target.value;setKind(next);onChange(next==='string'?'':next==='number'?0:next==='boolean'?false:next==='array'?[]:{});}}><option value="">尚未填写</option>{[['string','补充文字'],['number','补充数值'],['boolean','选择是或否'],['array','补充列表'],['object','补充结构化字段']].map(([key,text])=><option key={key} value={key}>{text}</option>)}</ChoiceSelect></div>;
 return <div className="grid min-w-0 gap-2"><Label htmlFor={id}>{label}</Label>{kind==='boolean'?<ChoiceSelect id={id} value={value} disabled={disabled} onChange={(event:any)=>onChange(event.target.value)}><option value={true as any}>是</option><option value={false as any}>否</option></ChoiceSelect>:kind==='number'?<FieldInput id={id} aria-label={label} type="number" step="any" value={value??''} disabled={disabled} onChange={(event:any)=>onChange(event.target.value===''?null:Number(event.target.value))}/>:textField(field)?<Textarea id={id} value={value??''} disabled={disabled} onChange={event=>onChange(event.target.value)}/>:<FieldInput id={id} value={value??''} disabled={disabled} onChange={(event:any)=>onChange(event.target.value)}/>}</div>;
}
