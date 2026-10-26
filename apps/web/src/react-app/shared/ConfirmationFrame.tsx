import {useState,Children,isValidElement} from 'react';
import {AlertDialog,AlertDialogContent,AlertDialogTitle,AlertDialogDescription,AlertDialogCancel} from '@/components/ui/alert-dialog';
import {Button} from '@/components/ui/button';
import {XIcon} from 'lucide-react';
import type {ReactNode,ReactElement} from 'react';
function heading(nodes:ReactNode):ReactNode{for(const node of Children.toArray(nodes)){if(!isValidElement(node))continue;const element=node as ReactElement<{children?:ReactNode}>;if(typeof element.type==='string'&&/^h[123]$/.test(element.type))return element.props.children;const nested=heading(element.props.children);if(nested)return nested;}return null;}
export default function ConfirmationFrame({children,onClose,...props}:{children:ReactNode;onClose:()=>void;[key:string]:any}){
 const [trigger]=useState(()=>document.activeElement as HTMLElement|null);
 return <AlertDialog open onOpenChange={open=>!open&&onClose()}><AlertDialogContent {...props} className="react-workspace max-h-[90dvh] overflow-y-auto" onCloseAutoFocus={event=>{event.preventDefault();(trigger?.isConnected?trigger:document.querySelector<HTMLElement>('main'))?.focus({preventScroll:true});}}><AlertDialogTitle className="sr-only">{heading(children)||'确认当前操作'}</AlertDialogTitle><AlertDialogDescription className="sr-only">请核对操作说明。取消会保留原有内容。</AlertDialogDescription><AlertDialogCancel asChild><Button type="button" variant="ghost" size="icon" className="absolute right-2 top-2" aria-label="关闭确认"><XIcon/></Button></AlertDialogCancel>{children}</AlertDialogContent></AlertDialog>;
}
