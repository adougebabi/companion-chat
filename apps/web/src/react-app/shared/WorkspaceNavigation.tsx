import {ChevronDownIcon,EarthIcon,ImagesIcon,InboxIcon,LibraryIcon,MessageCircleIcon,SettingsIcon,SproutIcon,UsersIcon,WandSparklesIcon,XIcon} from 'lucide-react';
import {SidebarHeader,SidebarContent,SidebarFooter,SidebarGroup,SidebarGroupLabel,SidebarGroupContent,SidebarMenu,SidebarMenuItem,SidebarMenuButton,SidebarMenuSub,SidebarMenuSubItem,SidebarMenuSubButton,SidebarSeparator,useSidebar} from '@/components/ui/sidebar';
import {Collapsible,CollapsibleTrigger,CollapsibleContent} from '@/components/ui/collapsible';
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuItem} from '@/components/ui/dropdown-menu';
import {TooltipProvider} from '@/components/ui/tooltip';
import {Button} from '@/components/ui/button';
import {lifePages} from '@/approved-ui/routing.mjs';
import {useEffect,useRef} from 'react';

type Props={page:string;character:{id?:string;name:string};lifeOpen:boolean;onLifeOpenChange:(open:boolean)=>void;onNavigate:(page:string)=>void};
const worldPages=[['世界',EarthIcon],['动态',SproutIcon],['人物关系',UsersIcon],['媒体',ImagesIcon]] as const;
const managementPages=[['通知与待处理',InboxIcon],['角色库',LibraryIcon],['创作中心',WandSparklesIcon],['管理中心',SettingsIcon]] as const;

export default function WorkspaceNavigation({page,character,lifeOpen,onLifeOpenChange,onNavigate}:Props){
 const {setOpenMobile,state,isMobile}=useSidebar();
 const iconOnly=state==='collapsed'&&!isMobile;
 const content=useRef<HTMLDivElement>(null);
 useEffect(()=>{const scroller=content.current,active=scroller?.querySelector<HTMLElement>('[data-active="true"]');if(!scroller||!active)return;const bounds=scroller.getBoundingClientRect(),row=active.getBoundingClientRect();if(row.top<bounds.top)scroller.scrollTop+=row.top-bounds.top;else if(row.bottom>bounds.bottom)scroller.scrollTop+=row.bottom-bounds.bottom;},[page,character.name]);
 const navigate=(target:string)=>{onNavigate(target);setOpenMobile(false);};
 const item=(label:string,Icon:typeof MessageCircleIcon,scoped=false)=><SidebarMenuItem key={label}><SidebarMenuButton type="button" className="h-10 min-h-[44px] text-base min-[801px]:min-h-0" tooltip={label} isActive={page===label} aria-current={page===label?'page':undefined} data-testid={'nav-'+label} disabled={scoped&&!character.id} onClick={()=>navigate(label)}><Icon aria-hidden="true"/><span>{label}</span></SidebarMenuButton></SidebarMenuItem>;
 return <TooltipProvider delayDuration={200}>
  <SidebarHeader className="gap-1 border-b border-sidebar-border px-4 py-4 group-data-[collapsible=icon]:items-center group-data-[collapsible=icon]:px-0">{iconOnly?<span className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground" aria-label={character.name}><SproutIcon className="size-4"/></span>:<><div className="flex items-center justify-between gap-2"><span className="text-sm text-muted-foreground">当前工作区</span><Button type="button" variant="ghost" size="icon" className="touch-target size-11 md:hidden" aria-label="关闭导航" onClick={()=>setOpenMobile(false)}><XIcon/></Button></div><p className="m-0 line-clamp-2 break-words text-base font-medium">{character.name}</p></>}</SidebarHeader>
  <SidebarContent ref={content} role="navigation" aria-label="工作区页面" className="gap-4 px-2 py-3">
   <SidebarGroup className="p-0"><SidebarGroupLabel className="text-sm group-data-[collapsible=icon]:hidden">日常</SidebarGroupLabel><SidebarGroupContent><SidebarMenu>
    {item('聊天',MessageCircleIcon,true)}
    {iconOnly?<SidebarMenuItem><DropdownMenu><DropdownMenuTrigger asChild><SidebarMenuButton type="button" tooltip="生活" aria-label="生活" disabled={!character.id} isActive={lifePages.includes(page)}><SproutIcon/><span>生活</span></SidebarMenuButton></DropdownMenuTrigger><DropdownMenuContent side="right" align="start" sideOffset={8}>{lifePages.map(label=><DropdownMenuItem key={label} data-testid={'nav-'+label} aria-current={page===label?'page':undefined} onSelect={()=>navigate(label)}>{label}</DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu></SidebarMenuItem>:<Collapsible asChild open={lifeOpen} onOpenChange={onLifeOpenChange}><SidebarMenuItem className="group/life"><CollapsibleTrigger asChild><SidebarMenuButton type="button" className="h-10 min-h-[44px] text-base min-[801px]:min-h-0" disabled={!character.id} aria-label="生活" isActive={lifePages.includes(page)&&!lifeOpen}><SproutIcon aria-hidden="true"/><span>生活</span><ChevronDownIcon className="ml-auto transition-transform group-data-[state=open]/life:rotate-180" aria-hidden="true"/></SidebarMenuButton></CollapsibleTrigger><CollapsibleContent><SidebarMenuSub>{lifePages.map(label=><SidebarMenuSubItem key={label}><SidebarMenuSubButton asChild className="w-full h-10 min-h-[44px] data-[size=md]:text-base min-[801px]:min-h-0" isActive={page===label} aria-current={page===label?'page':undefined}><button type="button" data-testid={'nav-'+label} disabled={!character.id} onClick={()=>navigate(label)}><span>{label}</span></button></SidebarMenuSubButton></SidebarMenuSubItem>)}</SidebarMenuSub></CollapsibleContent></SidebarMenuItem></Collapsible>}
   </SidebarMenu></SidebarGroupContent></SidebarGroup>
   <SidebarGroup className="p-0"><SidebarGroupLabel className="text-sm group-data-[collapsible=icon]:hidden">世界与联系</SidebarGroupLabel><SidebarGroupContent><SidebarMenu>{worldPages.map(([label,Icon])=>item(label,Icon,label==='人物关系'))}</SidebarMenu></SidebarGroupContent></SidebarGroup>
   <SidebarSeparator className="mx-0"/>
   <SidebarGroup className="p-0"><SidebarGroupLabel className="text-sm group-data-[collapsible=icon]:hidden">资源与管理</SidebarGroupLabel><SidebarGroupContent><SidebarMenu>{managementPages.map(([label,Icon])=>item(label,Icon))}</SidebarMenu></SidebarGroupContent></SidebarGroup>
  </SidebarContent>
  <SidebarFooter className="border-t border-sidebar-border px-4 py-3 text-sm text-muted-foreground group-data-[collapsible=icon]:hidden"><span>每一个世界，都有回响</span><span>Fluctlight · v6</span></SidebarFooter>
 </TooltipProvider>;
}
