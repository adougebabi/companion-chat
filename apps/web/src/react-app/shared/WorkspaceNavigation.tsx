import type {LucideIcon} from 'lucide-react';
import {useEffect,useRef,type ReactNode} from 'react';
import {BellIcon,BookOpenIcon,BrainIcon,CalendarDaysIcon,ChevronDownIcon,EarthIcon,HeartIcon,ImageIcon,LeafIcon,LibraryIcon,MessageCircleIcon,Settings2Icon,ShirtIcon,SproutIcon,TargetIcon,UsersIcon,WandSparklesIcon,XIcon} from 'lucide-react';
import {SidebarHeader,SidebarContent,SidebarFooter,SidebarGroup,SidebarGroupLabel,SidebarGroupContent,SidebarMenu,SidebarMenuItem,SidebarMenuButton,useSidebar} from '@/components/ui/sidebar';
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuItem} from '@/components/ui/dropdown-menu';
import {TooltipProvider} from '@/components/ui/tooltip';
import {Button} from '@/components/ui/button';
import CharacterAvatar from './CharacterAvatar';
import {StoryMark} from './WorkspaceArtwork';

export const workspaceNavigationGroups:readonly {label:string;items:readonly (readonly [string,string,LucideIcon])[]}[]=[
 {label:'日常相处',items:[['聊天','聊天',MessageCircleIcon],['此刻','此刻',SproutIcon],['日程','日程',CalendarDaysIcon],['目标','目标',TargetIcon],['衣柜','衣柜',ShirtIcon],['日记','日记',BookOpenIcon],['记忆','记忆',BrainIcon],['角色详情','角色档案',HeartIcon]]},
 {label:'世界与联系',items:[['世界','世界',EarthIcon],['动态','动态',LeafIcon],['人物关系','人物关系',UsersIcon],['媒体','媒体',ImageIcon]]},
 {label:'创作与管理',items:[['角色库','角色库',LibraryIcon],['创作中心','创作中心',WandSparklesIcon],['通知与待处理','通知与待处理',BellIcon],['管理中心','管理中心',Settings2Icon]]},
] as const;
export const workspacePageLabel=(page:string)=>page==='角色详情'?'角色档案':page==='初始化角色'?'新的相识':page;
export const workspaceNavigationGroup=(page:string)=>workspaceNavigationGroups.find(group=>group.items.some(([value])=>value===page))?.label||'角色库';
export const characterScopedPages=['聊天','此刻','日程','目标','衣柜','日记','记忆','角色详情','人物关系'];
type Character={id?:string;name:string;initial?:string;color?:string;job?:string;_directory?:any};
type Props={page:string;character:Character;characters?:Character[];onCharacterChange?:(id:string)=>void;onNavigate:(page:string)=>void;accountMenu?:ReactNode;unread?:number;lifeOpen?:boolean;onLifeOpenChange?:(open:boolean)=>void};

export default function WorkspaceNavigation({page,character,characters=[],onCharacterChange,onNavigate,accountMenu,unread=0}:Props){
 const {setOpenMobile,state,isMobile}=useSidebar();
 const iconOnly=state==='collapsed'&&!isMobile;
 const content=useRef<HTMLDivElement>(null);
 useEffect(()=>{const scroller=content.current,active=scroller?.querySelector<HTMLElement>('[data-active="true"]');if(!scroller||!active)return;const bounds=scroller.getBoundingClientRect(),row=active.getBoundingClientRect();if(row.top<bounds.top)scroller.scrollTop+=row.top-bounds.top;else if(row.bottom>bounds.bottom)scroller.scrollTop+=row.bottom-bounds.bottom;},[page,character.name]);
 const navigate=(target:string)=>{onNavigate(target);setOpenMobile(false);};
 return <TooltipProvider delayDuration={200}>
  <SidebarHeader className="replica-nav-header">
   <Button type="button" variant="ghost" className="replica-logo" onClick={()=>navigate('角色库')} aria-label="Fluctlight 角色库"><span className="replica-logo-symbol"><StoryMark/></span>{!iconOnly&&<span className="replica-logo-copy"><strong>Fluctlight</strong><small>A LIFE OF THEIR OWN</small></span>}</Button>
   {!iconOnly&&<div className="replica-workspace-switch"><CharacterAvatar name={character.name} initial={character.initial} color={character.color} src={character._directory?.avatar}/><div><small>正在相处</small><DropdownMenu><DropdownMenuTrigger asChild><Button variant="ghost" className="replica-character-switch" aria-label="切换当前角色" disabled={!characters.length}><span title={character.name+(character.job?' · '+character.job:'')}>{character.name}{character.job&&' · '+character.job}</span><ChevronDownIcon size={13}/></Button></DropdownMenuTrigger><DropdownMenuContent align="start">{characters.map(c=><DropdownMenuItem key={c.id} onSelect={()=>c.id&&onCharacterChange?.(c.id)}><CharacterAvatar className="small" name={c.name} initial={c.initial} color={c.color} src={c._directory?.avatar}/><span className="min-w-0 max-w-[65vw] whitespace-normal break-words">{c.name}{c.job&&' · '+c.job}</span>{c.id===character.id&&<span className="sr-only">当前角色</span>}</DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu></div>{isMobile&&<Button variant="ghost" size="icon" aria-label="关闭导航" onClick={()=>setOpenMobile(false)}><XIcon/></Button>}</div>}
  </SidebarHeader>
  <SidebarContent ref={content} role="navigation" aria-label="工作区页面" className="replica-nav-content">
   {workspaceNavigationGroups.map(group=><SidebarGroup key={group.label} className="replica-nav-section"><SidebarGroupLabel className={iconOnly?'hidden':'replica-nav-label'}>{group.label}</SidebarGroupLabel><SidebarGroupContent><SidebarMenu>{group.items.map(([value,label,Icon])=><SidebarMenuItem key={value}><SidebarMenuButton type="button" className="replica-nav-item" tooltip={label} aria-label={label} isActive={page===value||(page==='初始化角色'&&value==='角色库')} aria-current={page===value?'page':undefined} data-testid={'nav-'+value} disabled={characterScopedPages.includes(value)&&!character.id} onClick={()=>navigate(value)}><Icon aria-hidden="true"/><span>{label}</span>{value==='通知与待处理'&&unread>0&&<span className="replica-nav-count">{unread}</span>}</SidebarMenuButton></SidebarMenuItem>)}</SidebarMenu></SidebarGroupContent></SidebarGroup>)}
  </SidebarContent>
  <SidebarFooter className="replica-nav-footer"><span className="replica-owner-avatar">我</span>{!iconOnly&&<><div>我的空间<small>MY WORKSPACE</small></div>{accountMenu}</>}</SidebarFooter>
 </TooltipProvider>;
}
