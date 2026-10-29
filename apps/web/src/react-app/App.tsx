import {workspaceReadFeedback} from '@/approved-ui/read-feedback.mjs';
import {InlineFeedback,Feedback} from '@/react-app/shared/Presentation';
import {ReadState} from "@/react-app/shared/Presentation";
import {Surface as CardSurface} from "@/react-app/controls";
import {configureToast} from '@/v3/proto/components/toast.js';
import OverlayHost from './shared/OverlayHost';
import WorkspaceBreadcrumb from './shared/WorkspaceBreadcrumb';
import {SidebarProvider,Sidebar,SidebarInset,SidebarTrigger,SidebarRail,useSidebar} from '@/components/ui/sidebar';
import WorkspaceNavigation,{workspaceNavigationGroup,workspacePageLabel} from './shared/WorkspaceNavigation';
import WorkspaceQuickJump from './shared/WorkspaceQuickJump';
import {Toaster} from '@/components/ui/sonner';
import {toast} from 'sonner';
import LoadingState from './shared/LoadingState';
import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  BellIcon,LogOutIcon,UserRoundIcon,
  PanelRightCloseIcon,
  PanelRightOpenIcon,
} from "lucide-react";
import { createWorkspace } from "../state/workspace.mjs";
import {
  approvedRoute,
  workspaceReadingKey,
  approvedHref,
  lifePages,
} from "../approved-ui/routing.mjs";
import { chatSummaryPreference } from "../v3/chat-summary-preference.mjs";
import { WorkspaceContext } from "./page";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
const ChatPage = lazy(() => import("./pages/ChatPage"));
const GoalsPage = lazy(() => import("./pages/GoalsPage"));
const DiaryPage = lazy(() => import("./pages/DiaryPage"));
const MemoryPage = lazy(() => import("./pages/MemoryPage"));
const SchedulePage = lazy(() => import("./pages/SchedulePage"));
const LifePage = lazy(() => import("./pages/LifePage"));
const WardrobePage = lazy(() => import("./pages/WardrobePage"));
const WorldPage = lazy(() => import("./pages/WorldPage"));
const PeoplePage = lazy(() => import("./pages/PeoplePage"));
const FeedPage = lazy(() => import("./pages/FeedPage"));
const BrowsePage = lazy(() => import("./pages/BrowsePage"));
const ManagementPage = lazy(() => import("./pages/ManagementPage"));
const LibraryPage = lazy(() => import("./pages/LibraryPage"));
const CharacterWizardPage = lazy(() => import("./pages/CharacterWizardPage"));
const CharacterDetailPage = lazy(() => import("./pages/CharacterDetailPage"));
const DomainPage = lazy(() => import("./pages/DomainPage"));
const MediaPage = lazy(() => import("./pages/MediaPage"));
const StudioPage = lazy(() => import("./pages/StudioPage"));
const AuthPage = lazy(() => import("./pages/AuthPage"));
type Workspace = ReturnType<typeof createWorkspace>;
type Route = {
  page?: string;
  fid?: string | null;
  removed?: boolean;
  notFound?: boolean;
  query?: URLSearchParams;
  diagnostics?: boolean;
  feature?: any;
  information?: {title:string;body:string;page:string;label:string};
  global?: boolean;
  path?: string;
  title?: string;
};

export default function App() {
  useEffect(()=>configureToast((text:string,type:string)=>{if(type==='err')toast.error(text);else if(['ok','success'].includes(type))toast.success(text);else toast(text);}),[]);
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  useEffect(() => {
    const current = createWorkspace();
    setWorkspace(current);
    return () => current.dispose();
  }, []);
  return workspace ? (
    <WorkspaceContext.Provider value={workspace}>
      <div className="react-workspace replica-workspace">
        <SidebarProvider mobileBreakpoint={701} className="replica-shell h-dvh min-h-0 w-full overflow-hidden" style={{"--sidebar-width":"var(--workspace-sidebar-width)","--sidebar-width-icon":"51px"} as React.CSSProperties}><WorkspaceShell workspace={workspace}/><OverlayHost/><Toaster richColors closeButton/></SidebarProvider>
      </div>
    </WorkspaceContext.Provider>
  ) : (
    <LoadingState label="正在准备工作区…"/>
  );
}
function WorkspaceShell({ workspace }: { workspace: Workspace }) {
  const state: any = useSyncExternalStore(
    workspace.subscribe,
    workspace.getSnapshot,
    workspace.getSnapshot,
  );
  const [route, setRoute] = useState<Route>(() => approvedRoute(location.hash));
  const {setOpenMobile:setMobileOpen,open:sidebarOpen,isMobile}=useSidebar();
  const [accountBusy,setAccountBusy]=useState(false);
  const navigationTrigger=useRef<HTMLButtonElement>(null);
  const [preview, setPreview] = useState(true);
  const scrollPositions = useRef(new Map<string,number>());
  const main = useRef<HTMLDivElement>(null),
    page = route.removed||route.notFound?"页面暂不可用":route.page || state.page;
  const current = workspace.current.value;
  const routeCharacterExists = !route.fid || state.characters.some((character: {id:string}) => character.id === route.fid);
  const character =
    page === "初始化角色"
      ? { id:"", name: "创建角色", initial: "+", color: "#dce9e2", _directory:null }
      : current.id ? current : {...current,name:"我的空间",initial:"我",color:"#dce9e2"};
  const navigate = useCallback(
    (page: string) => {
      location.hash = approvedHref(page, workspace.state.selectedId);
      setMobileOpen(false);
    },
    [workspace],
  );
  useEffect(() => {
    workspace.navigate = navigate;
    const parse = () => {setMobileOpen(false);setRoute(approvedRoute(location.hash));};
    window.addEventListener("hashchange", parse);
    void workspace.bootstrap().catch(() => {});
    return () => window.removeEventListener("hashchange", parse);
  }, [workspace, navigate]);
  useEffect(() => {
    if (!state.ready || !state.session || route.removed || route.notFound || !routeCharacterExists)
      return;
    let active = true;
    setMobileOpen(false);
    Object.assign(workspace.state,{route});
    if(page==="登录"){navigate("角色库");return;}
    if (route.fid && state.characters.some((c: any) => c.id === route.fid))
      workspace.select(route.fid);
    if (
      !workspace.state.selectedId &&
      ["聊天", ...lifePages, "人物关系"].includes(page)
    ) {
      navigate("角色库");
      return;
    }
    workspace.state.page = page;
    workspace.state.managementSection = route.query?.get("section") || "";
    if (route.diagnostics) workspace.state.managementTab = "运行诊断";
    else if(page==="管理中心"&&route.query?.get("section"))workspace.state.managementTab="配置中心";
    workspace.state.feedScope = route.query?.get("fid") || null;
    setPreview(
      !chatSummaryPreference({
        owner: state.session.actorId,
        fid: workspace.state.selectedId,
        defaultFolded: false,
      }).read(),
    );
    void (
      route.feature&&route.feature.native!=='chat-history' || route.information || ["媒体", "创作中心", "登录", "日程", "目标", "日记", "记忆", "角色详情", "世界", "人物关系", "动态", "通知与待处理"].includes(page)
        ? Promise.resolve()
        : workspace.load(page)
    ).then(() => {
      if (active) main.current?.scrollTo({ top: scrollPositions.current.get(workspaceReadingKey(state.session?.actorId,route)) || 0 });
    });
    return () => {
      active = false;
    };
  }, [route, state.ready, state.session?.actorId, workspace, navigate, page, routeCharacterExists]);
  useEffect(()=>{if(!state.toast)return;const item=state.toast;toast(item.text,{action:item.link?{label:'去方案库',onClick:()=>{workspace.state.outfitTab='方案库';navigate('衣柜');}}:undefined});},[state.toast,navigate,workspace]);
  const changePreview = (next:boolean) => {
    setPreview(next);
    if (state.session && state.selectedId)
      chatSummaryPreference({
        owner: state.session.actorId,
        fid: state.selectedId,
        defaultFolded: false,
      }).save(!next);
  };
  const togglePreview = () => changePreview(!preview);
  if (state.ready && state.errors.bootstrap)
    return (
      <CardSurface as="section" className="approved-bootstrap self-start" role="alert">
        <p>{state.errors.bootstrap}</p>
        <Button onClick={() => void workspace.bootstrap().catch(() => {})}>
          重新读取
        </Button>
      </CardSurface>
    );
  if (state.ready && state.session && !routeCharacterExists) return <section className="approved-bootstrap"><h1>角色不存在</h1><p>请回到角色库选择当前可用的角色。</p><Button asChild><a href="#/library">返回角色库</a></Button></section>;
  if (state.ready && !state.session)
    return (
      <Suspense fallback={<LoadingState label="正在读取登录状态…"/>}>
        <AuthPage onAuthenticated={() => void workspace.bootstrap()} />
      </Suspense>
    );
  if (route.page === "登录" && state.session)
    return (
      <section className="approved-bootstrap">
        <p>你已登录。</p>
        <Button asChild>
          <a href="#/library">进入角色库</a>
        </Button>
      </section>
    );
  if (!state.ready)
    return (
      <ReadState className="approved-bootstrap" role="status">
        正在读取你的角色与工作区…
      </ReadState>
    );
  const unavailable=route.removed||route.notFound;
  const scopedHeading=state.pageHeading&&(route.feature?state.pageHeading.route===route.path&&state.pageHeading.owner===state.session?.actorId:state.pageHeading.page===page&&state.pageHeading.fid===state.selectedId)?state.pageHeading.title:"";
  const displayTitle=unavailable?"页面暂不可用":scopedHeading||route.feature?.title||route.title||(page==="管理中心"&&state.managementTab==="运行诊断"?"运行诊断":page);
  const content = unavailable?<CardSurface as="section"><h2>这个页面暂时无法访问</h2><p>可以返回刚才的页面，或从导航继续浏览。</p><Button variant="outline" onClick={()=>history.back()}>返回上一页</Button><Button asChild><a href="#/library">打开角色库</a></Button></CardSurface>:route.feature?.native==='people'?<PeoplePage key={'people:'+character.id} route={route} initialTab={route.feature.initialTab} initialNpcId={route.feature.params.npcId} initialEdit={route.feature.initialEdit} initialCreate={route.feature.initialCreate}/>:route.feature?.native==='media'?<MediaPage key={route.path+'?'+route.query} route={route} initialTab={route.feature.initialTab} initialAssetId={route.feature.params.mdId} initialEdit={route.feature.initialEdit}/>:route.feature?.native==='feed'?<FeedPage key={'feed:'+character.id} route={route} initialPostId={route.feature.params.postId}/>:route.feature?.native==='chat-history'?<ChatPage key={character.id} preview={preview} onPreviewChange={changePreview} onGo={navigate} route={route} historyRoute={route}/>:route.feature ? <DomainPage key={route.path+"?"+route.query} route={route}/>
    : route.information ? <CardSurface as="section" ><h2>{route.information.title}</h2><p>{route.information.body}</p><Button variant="outline" onClick={()=>navigate(route.information!.page)}>{route.information.label}</Button></CardSurface>
    : page === "聊天" ? <ChatPage key={character.id} preview={preview} onPreviewChange={changePreview} onGo={navigate} route={route} historyRoute={route.query?.has('conversation')||route.query?.has('message')?route:undefined}/>
    : page === "衣柜" ? <WardrobePage key={"wardrobe:"+character.id} route={route}/>
    : page === "世界" ? character.id ? <WorldPage key={"world:"+character.id}/> : <StudioPage key="global-worlds" kind="world" route={route}/>
    : page === "动态" ? <FeedPage key={"feed:"+character.id} route={route}/>
    : page === "人物关系" ? <PeoplePage key={'people:'+character.id} route={route}/>
    : page === "媒体" ? <MediaPage key={route.path+"?"+route.query} route={route}/>
    : page === "创作中心" ? <StudioPage key={route.path+"?"+route.query} route={route}/>
    : page === "角色详情" ? <CharacterDetailPage key={"profile:"+character.id}/>
    : page === "目标" ? <GoalsPage key={"goals:"+character.id} route={route} onGo={navigate}/>
    : page === "日记" ? <DiaryPage key={"diary:"+character.id}/>
    : page === "记忆" ? <MemoryPage key={"memory:"+character.id} route={route}/>
    : page === "日程" ? <SchedulePage key={"schedule:"+character.id} route={route}/>
    : lifePages.includes(page) ? <LifePage key={character.id} page={page} onGo={navigate}/>
    : page === "管理中心" ? <ManagementPage/>
    : page === "角色库" ? <LibraryPage/>
    : page === "初始化角色" ? <CharacterWizardPage key={route.path+"?"+route.query}/>
    : <BrowsePage key={page+":"+character.id} page={page}/>;
  const reading=workspaceReadFeedback(state,(route.feature||['通知与待处理','日程','目标','日记','记忆','角色详情','世界','人物关系','动态','媒体'].includes(page))?'':page,workspace.currentScope());
  const pageReadErrors=page==='此刻'?reading.errors.filter(error=>error.key!=='schedule'):reading.errors;
  const readError=state.errors.directory||state.errors.session||'';
  const isLifeRoute=lifePages.includes(page)||route.path?.includes('/life/');
  const ancestors=isLifeRoute&&page!=='此刻'?[{label:'生活',href:approvedHref('此刻',state.selectedId)}]:page==='管理中心'&&state.managementTab==='运行诊断'&&!route.feature?[{label:'管理中心',href:'#/settings?section='+encodeURIComponent('服务地址')}]:[];
  return (
    <>
      <Sidebar collapsible="icon" variant="inset" className="replica-sidebar" returnFocus={()=>navigationTrigger.current}><WorkspaceNavigation page={page} character={current.id?current:character} characters={state.characters} onCharacterChange={(id)=>{workspace.select(id);navigate(page==="初始化角色"?"聊天":page);}} onNavigate={navigate} unread={state.notificationCount?.count||0} accountMenu={<DropdownMenu><DropdownMenuTrigger asChild><Button type="button" variant="ghost" size="icon" aria-label="账户菜单"><LogOutIcon size={14}/></Button></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={()=>navigate('管理中心')}><UserRoundIcon/>账户与设置</DropdownMenuItem><DropdownMenuItem disabled={accountBusy} className="text-destructive focus:text-destructive" onSelect={async()=>{setAccountBusy(true);try{await workspace.logout();}catch(cause){toast.error(cause instanceof Error?cause.message:'退出未能确认，请重试');}finally{setAccountBusy(false);}}}><LogOutIcon/>{accountBusy?'正在退出…':'退出登录'}</DropdownMenuItem></DropdownMenuContent></DropdownMenu>}/><SidebarRail aria-label="切换导航宽度"/></Sidebar>
      <SidebarInset tabIndex={-1} className="replica-main min-w-0 min-h-0 overflow-hidden">
      <header className="replica-topbar">
        <SidebarTrigger ref={navigationTrigger} className="replica-topbar-icon" aria-label={!isMobile&&sidebarOpen?"收起导航":"打开导航"}/>
        <WorkspaceBreadcrumb parentLabel={workspaceNavigationGroup(page)} parentHref="#/library" ancestors={route.feature?ancestors:[]} currentLabel={workspacePageLabel(displayTitle)}/>
        <div className="replica-topbar-actions"><WorkspaceQuickJump key={state.session?.actorId} characters={state.characters} hasCharacter={!!current.id} onNavigate={navigate} onCharacter={id=>{workspace.select(id);navigate('聊天');}}/><Button type="button" variant="ghost" size="icon" className="replica-topbar-icon" title={state.notificationCountError?"未读计数暂未更新："+state.notificationCountError:state.notificationCount?state.notificationCount.count+" 条未读通知":"未读计数尚未读取"} aria-label="查看通知" onClick={()=>navigate('通知与待处理')}><BellIcon size={16}/>{(state.notificationCount?.alert_count||0)>0&&<span className="replica-notification-dot" aria-label={state.notificationCount.count+' 条未读通知'}/>}</Button></div>
      </header>
        <div
          ref={main}
          tabIndex={-1}
          onScroll={event=>scrollPositions.current.set(workspaceReadingKey(state.session?.actorId,route),event.currentTarget.scrollTop)}
          className={"workspace-page replica-page" + (page === "聊天" ? " replica-chat-main" : "")}
        >
          {!(page==='世界'&&!route.feature)&&!(page==='人物关系'&&(!route.feature||route.feature.native==='people'))&&!(page==="管理中心"&&(!route.feature||route.feature.native==="task"||route.path==="/data"))&&!(page==="通知与待处理"&&!route.feature)&&!["角色库","聊天","此刻","日程","目标","衣柜","日记","记忆","角色详情","动态","媒体","创作中心","初始化角色"].includes(page)&&<div className="page-heading">
            <div>

              <h1>
                {displayTitle}
                {page === "聊天" && (
                  <span className="heading-caption">
                    日常的片刻，与你分享。
                  </span>
                )}
              </h1>
            </div>
            {page === "聊天" && (
              <Button data-testid="chat-preview-toggle" className="touch-target" variant="outline" onClick={togglePreview}>
                {preview ? (
                  <PanelRightCloseIcon size={15} />
                ) : (
                  <PanelRightOpenIcon size={15} />
                )}{" "}
                {preview ? "收起预览" : "展开预览"}
              </Button>
            )}
            {page==='管理中心'&&!route.feature&&route.query?.get('returnTo')?.startsWith('#/')&&<Button variant="outline" onClick={()=>{const target=route.query?.get('returnTo');if(target?.startsWith('#/'))workspace.navigatePath(target.slice(1));}}>{route.query?.get('returnTo')==='#/inbox'?'返回通知中心':'返回来源页面'}</Button>}
          </div>}
          <Suspense key={state.session?.actorId} fallback={<LoadingState label="正在读取页面…"/>}>
            {page === "聊天" ? (
              content
            ) : (
              <div className="page-content">{pageReadErrors.length>0&&<Feedback kind={reading.initialFailure?"error":"warning"} className="mb-4" title={reading.initialFailure?"当前内容尚未读到":"部分内容暂未读全"} actions={<Button type="button" variant="outline" disabled={reading.loading} onClick={()=>void workspace.load(page)}>重新读取当前页面</Button>}>{reading.initialFailure?"需要重新读取后继续。":"上次确认的内容仍保留。"} {pageReadErrors.map(error=><p key={error.key}>{error.label}：{error.message}</p>)}</Feedback>}<div hidden={reading.initialFailure}>{content}</div></div>
            )}
          </Suspense>
        </div>
      {readError && (
        <div className="approved-read-feedback" role="alert">
          {readError}
          <Button variant="link" onClick={() => workspace.load(page)}>
            重新读取
          </Button>
        </div>
      )}
      </SidebarInset>
    </>
  );
}
