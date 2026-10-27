import {useState} from 'react';
import type {ReactNode,HTMLAttributes} from 'react';
import {AlertDialog,AlertDialogContent,AlertDialogHeader,AlertDialogTitle,AlertDialogDescription,AlertDialogFooter,AlertDialogCancel} from '@/components/ui/alert-dialog';
import {XIcon} from 'lucide-react';
export default function ConfirmationFrame({title,description,children,onClose,...props}:Omit<HTMLAttributes<HTMLDivElement>,'title'> & {title:ReactNode;description:ReactNode;onClose:()=>void}){
 const [trigger]=useState(()=>document.activeElement as HTMLElement|null);
 return <AlertDialog open onOpenChange={open=>!open&&onClose()}><AlertDialogContent {...props} className="react-workspace max-h-[90dvh] overflow-y-auto" onCloseAutoFocus={event=>{event.preventDefault();(trigger?.isConnected?trigger:document.querySelector<HTMLElement>('main'))?.focus({preventScroll:true});}}>
  <AlertDialogCancel variant="ghost" size="icon" className="absolute right-2 top-2" aria-label="关闭确认"><XIcon/></AlertDialogCancel>
  <AlertDialogHeader className="pr-8"><AlertDialogTitle>{title}</AlertDialogTitle><AlertDialogDescription>{description}</AlertDialogDescription></AlertDialogHeader>
  <AlertDialogFooter>{children}</AlertDialogFooter>
 </AlertDialogContent></AlertDialog>;
}
