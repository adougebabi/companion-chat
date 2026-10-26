import {Button} from '@/components/ui/button';
import {Alert,AlertDescription} from '@/components/ui/alert';
import {useWorkspace} from '../page';
import LoadingState from './LoadingState';
export default function RecordShell({title,back,backLabel='返回',read,children}:any){const w=useWorkspace();return <section className="grid min-w-0 gap-5"><header className="flex flex-wrap items-center gap-3">{back&&<Button type="button" variant="outline" onClick={()=>w.navigatePath(String(back).replace(/^#/,''))}>{backLabel}</Button>}<h2 className="text-xl font-semibold">{title}</h2>{read&&<Button type="button" variant="outline" disabled={read.loading} onClick={()=>void read.reload().catch(()=>{})}>重新读取</Button>}</header>{read?.error&&<Alert variant="destructive"><AlertDescription>{read.error}{read.data?'，此前读到的资料仍保留。':''}</AlertDescription></Alert>}{read?.loading&&!read.data&&<LoadingState/>}{children}</section>;}
