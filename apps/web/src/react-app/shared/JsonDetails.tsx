import {Disclosure} from '../controls';
import {Button} from '@/components/ui/button';
import RecordView from './RecordView';

export default function JsonDetails({value,title='技术详情',downloadOnly=false,filename='fluctlight-record.json'}:{value:any;title?:string;downloadOnly?:boolean;filename?:string}){
 const empty=value==null||typeof value==='object'&&!Object.keys(value).length;
 return <Disclosure title={title}>
  {downloadOnly?<p className="text-sm text-muted-foreground">{empty?'尚未记录相关详情':'原始记录仅供下载，任务说明请查看上方业务摘要。'}</p>:<RecordView value={value} emptyText="尚未记录相关详情" technical/>}
  {!empty&&<Button type="button" variant="outline" onClick={()=>{
   const url=URL.createObjectURL(new Blob([JSON.stringify(value,null,2)],{type:'application/json'})),link=document.createElement('a');
   link.href=url;link.download=filename;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
  }}>下载原始记录</Button>}
 </Disclosure>;
}
