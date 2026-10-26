import {ScrollArea} from '@/components/ui/scroll-area';
import {Disclosure} from '../controls';
export default function JsonDetails({value,title='技术详情'}:{value:any;title?:string}){return <Disclosure title={title}><ScrollArea className="h-72 w-full rounded-md border"><pre className="m-0 whitespace-pre-wrap break-words p-4 text-xs">{JSON.stringify(value,null,2)}</pre></ScrollArea></Disclosure>;}
