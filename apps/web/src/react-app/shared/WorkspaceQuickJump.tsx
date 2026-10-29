import {useEffect,useState} from 'react';
import {ChevronRightIcon,SearchIcon,PlusIcon} from 'lucide-react';
import {Dialog,DialogTrigger,DialogContent,DialogHeader,DialogTitle,DialogDescription} from '@/components/ui/dialog';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {characterScopedPages,workspaceNavigationGroups} from './WorkspaceNavigation';

export default function WorkspaceQuickJump({characters,hasCharacter,onNavigate,onCharacter}:{characters:{id:string;name:string}[];hasCharacter:boolean;onNavigate:(page:string)=>void;onCharacter:(id:string)=>void}){
 const [open,setOpen]=useState(false),[query,setQuery]=useState('');
 useEffect(()=>{const key=(event:KeyboardEvent)=>{if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='k'){event.preventDefault();setOpen(value=>!value);}};window.addEventListener('keydown',key);return()=>window.removeEventListener('keydown',key);},[]);
 const pages=[...workspaceNavigationGroups.flatMap(group=>group.items),['初始化角色','新的相识',PlusIcon] as const].filter(([value,label])=>(label.includes(query.trim())||value.includes(query.trim()))&&(hasCharacter||!characterScopedPages.includes(value)));
 const matches=characters.filter(character=>character.name.toLowerCase().includes(query.trim().toLowerCase()));
 const close=()=>{setOpen(false);setQuery('');};
 return <Dialog open={open} onOpenChange={next=>{setOpen(next);if(!next)setQuery('');}}>
  <DialogTrigger asChild><Button variant="ghost" className="replica-quick-trigger"><span><SearchIcon size={12}/>快速前往</span><kbd>⌘ K</kbd></Button></DialogTrigger>
  <DialogContent className="react-workspace replica-quick-dialog"><DialogHeader><DialogTitle>你想去哪里？</DialogTitle><DialogDescription>搜索页面，快速回到正在发生的日常。</DialogDescription></DialogHeader>
   <div className="replica-quick-search"><SearchIcon size={15} aria-hidden="true"/><Input autoFocus aria-label="搜索页面或角色名字" placeholder="搜索页面或角色名字" value={query} onChange={event=>setQuery(event.target.value)}/></div>
   <div className="replica-command-results">{pages.map(([value,label])=><Button variant="ghost" key={value} onClick={()=>{onNavigate(value);close();}}><span>{label}</span><ChevronRightIcon size={14}/></Button>)}{matches.map(character=><Button variant="ghost" key={character.id} onClick={()=>{onCharacter(character.id);close();}}><span>{character.name}</span><small>进入聊天</small></Button>)}{!pages.length&&!matches.length&&<p role="status">没有找到匹配的页面或角色。</p>}</div>
  </DialogContent>
 </Dialog>;
}
