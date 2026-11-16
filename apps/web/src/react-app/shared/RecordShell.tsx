import {Feedback} from './Presentation';
import LoadingState from './LoadingState';
import RecordPageHeader from './RecordPageHeader';
export default function RecordShell({title,description,eyebrow,back,backLabel='返回',read,children}:any){return <section className="grid min-w-0 gap-5"><RecordPageHeader title={title} description={description} eyebrow={eyebrow} back={back} backLabel={backLabel} read={read}/>{read?.error&&!read.loading&&<Feedback kind={read.data?'warning':'error'} title={title+'暂未读全'}>{read.data?'此前读到的资料仍保留，可以重新读取。':'当前资料尚未完整读到，可以重新读取。'}</Feedback>}{read?.loading&&!read.data&&<LoadingState/>}{read?.loading&&read.data&&<Feedback kind="progress">正在重新读取{title}，此前确认的内容仍保留。</Feedback>}{children}</section>;}
