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
  LeafIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  MessageCircleIcon,
  SproutIcon,
  EarthIcon,
  UsersIcon,
  ImagesIcon,
  InboxIcon,
  LibraryIcon,
  WandSparklesIcon,
  SettingsIcon,
  PanelRightCloseIcon,
  PanelRightOpenIcon,
  MenuIcon,
} from "lucide-react";
import { createWorkspace } from "../state/workspace.mjs";
import {
  approvedRoute,
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
import {
  Sheet,
  SheetTrigger,
  SheetContent,
  SheetTitle,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import CharacterAvatar from "./shared/CharacterAvatar";
import { cn } from "@/lib/utils";
const ChatPage = lazy(() => import("./pages/ChatPage"));
const LifePage = lazy(() => import("./pages/LifePage"));
const WardrobePage = lazy(() => import("./pages/WardrobePage"));
const WorldPage = lazy(() => import("./pages/WorldPage"));
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
const characterNavigation = [
  ["世界", EarthIcon],
  ["动态", SproutIcon],
  ["人物关系", UsersIcon],
  ["媒体", ImagesIcon],
] as const;
const globalNavigation = [
  ["通知与待处理", InboxIcon],
  ["角色库", LibraryIcon],
  ["创作中心", WandSparklesIcon],
  ["管理中心", SettingsIcon],
] as const;

export default function App() {
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  useEffect(() => {
    const current = createWorkspace();
    setWorkspace(current);
    return () => current.dispose();
  }, []);
  return workspace ? (
    <WorkspaceContext.Provider value={workspace}>
      <div className="react-workspace">
        <WorkspaceShell workspace={workspace} />
      </div>
    </WorkspaceContext.Provider>
  ) : (
    <p role="status">正在准备工作区…</p>
  );
}
function WorkspaceShell({ workspace }: { workspace: Workspace }) {
  const state: any = useSyncExternalStore(
    workspace.subscribe,
    workspace.getSnapshot,
    workspace.getSnapshot,
  );
  const [route, setRoute] = useState<Route>(() => approvedRoute(location.hash));
  const [lifeOpen, setLifeOpen] = useState(true),
    [mobileOpen, setMobileOpen] = useState(false),
    [preview, setPreview] = useState(() => !window.matchMedia("(max-width: 800px)").matches);
  const main = useRef<HTMLElement>(null),
    page = route.page || state.page;
  const current = workspace.current.value;
  const routeCharacterExists = !route.fid || state.characters.some((character: {id:string}) => character.id === route.fid);
  const character =
    page === "初始化角色"
      ? { ...current, name: "新的相识", initial: "+", color: "#dce9e2" }
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
    const narrow=window.matchMedia("(max-width: 800px)");
    const resize=()=>{setMobileOpen(false);if(narrow.matches)setPreview(false);};
    narrow.addEventListener("change",resize);return()=>narrow.removeEventListener("change",resize);
  },[]);
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
    workspace.state.feedScope = route.query?.get("fid") || null;
    setPreview(
      !window.matchMedia("(max-width: 800px)").matches && !chatSummaryPreference({
        owner: state.session.actorId,
        fid: workspace.state.selectedId,
        defaultFolded: false,
      }).read(),
    );
    void (
      route.feature || route.information || ["媒体", "创作中心", "登录"].includes(page) || (page==="世界"&&!workspace.state.selectedId)
        ? Promise.resolve()
        : workspace.load(page)
    ).then(() => {
      if (active) main.current?.scrollTo({ top: 0 });
    });
    return () => {
      active = false;
    };
  }, [route, state.ready, state.session?.actorId, workspace, navigate, page, routeCharacterExists]);
  const togglePreview = () => {
    const next = !preview;
    setPreview(next);
    if (state.session && state.selectedId)
      chatSummaryPreference({
        owner: state.session.actorId,
        fid: state.selectedId,
        defaultFolded: false,
      }).save(!next);
  };
  if (route.removed || route.notFound)
    return (
      <section className="approved-bootstrap">
        <h1>页面不存在</h1>
        <p>这个地址已停用。</p>
        <Button asChild>
          <a href="#/library">返回角色库</a>
        </Button>
      </section>
    );
  if (state.ready && state.session && !routeCharacterExists) return <section className="approved-bootstrap"><h1>角色不存在</h1><p>请回到角色库选择当前可用的角色。</p><Button asChild><a href="#/library">返回角色库</a></Button></section>;
  if (state.ready && !state.session)
    return (
      <Suspense fallback={<p role="status">正在读取登录状态…</p>}>
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
      <div className="approved-bootstrap" role="status">
        正在读取你的角色与工作区…
      </div>
    );
  if (state.errors.bootstrap)
    return (
      <section className="card approved-bootstrap" role="alert">
        <p>{state.errors.bootstrap}</p>
        <Button onClick={() => void workspace.bootstrap().catch(() => {})}>
          重新读取
        </Button>
      </section>
    );
  const navItem = (label: string, Icon?: typeof LeafIcon, scoped = true) => (
    <Button
      key={label}
      type="button"
      variant="ghost"
      data-testid={`nav-${label}`}
      disabled={scoped && !character.id}
      aria-current={page === label ? "page" : undefined}
      className={cn("nav-item justify-start", page === label && "active")}
      onClick={() => navigate(label)}
    >
      {Icon && <Icon size={17} />}
      <span>{label}</span>
    </Button>
  );
  const navigation = (
    <>
      <div className="workspace-label">
        角色工作区 <span>{character.name}</span>
      </div>
      <nav>
        {navItem("聊天", MessageCircleIcon)}
        <Button
          type="button"
          variant="ghost"
          className="nav-item justify-start"
          disabled={!character.id}
          aria-expanded={lifeOpen}
          onClick={() => setLifeOpen(!lifeOpen)}
        >
          <SproutIcon size={17} />
          <span>生活</span>
          <ChevronDownIcon size={13} />
        </Button>
        {lifeOpen && (
          <div className="life-nav">
            {lifePages.map((label) => navItem(label))}
          </div>
        )}
        {characterNavigation.map(([label, Icon]) => navItem(label, Icon, label === "人物关系"))}
      </nav>
      <Separator className="nav-divider" />
      <nav>
        {globalNavigation.map(([label, Icon]) => navItem(label, Icon, false))}
      </nav>
      <div className="sidebar-footer">
        每一个世界，都有回响 <small>Fluctlight · v6</small>
      </div>
    </>
  );
  const content = route.feature ? <DomainPage key={route.path+"?"+route.query} route={route}/>
    : route.information ? <section className="card"><h2>{route.information.title}</h2><p>{route.information.body}</p><Button variant="outline" onClick={()=>navigate(route.information!.page)}>{route.information.label}</Button></section>
    : page === "聊天" ? <ChatPage key={character.id} preview={preview} onGo={navigate}/>
    : page === "衣柜" ? <WardrobePage key={"wardrobe:"+character.id}/>
    : page === "世界" ? character.id ? <WorldPage key={"world:"+character.id}/> : <StudioPage key="global-worlds" kind="world" route={route}/>
    : page === "媒体" ? <MediaPage key={route.path+"?"+route.query} route={route}/>
    : page === "创作中心" ? <StudioPage key={route.path+"?"+route.query} route={route}/>
    : page === "角色详情" ? <CharacterDetailPage key={"profile:"+character.id}/>
    : lifePages.includes(page) ? <LifePage key={character.id} page={page} onGo={navigate}/>
    : page === "管理中心" ? <ManagementPage/>
    : page === "角色库" ? <LibraryPage/>
    : page === "初始化角色" ? <CharacterWizardPage/>
    : <BrowsePage key={page+":"+character.id} page={page}/>;
  const readError = Object.values(state.errors).filter(Boolean).at(-1) as
    string | undefined;
  return (
    <div className="shell">
      <header className="topbar">
        <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
          <SheetTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="mobile-nav-trigger"
              aria-label="打开导航"
            >
              <MenuIcon />
            </Button>
          </SheetTrigger>
          <SheetContent
            side="left"
            className="react-workspace mobile-nav-content overflow-y-auto"
          >
            <SheetTitle className="sr-only">工作区导航</SheetTitle>
            {navigation}
          </SheetContent>
        </Sheet>
        <Button
          variant="ghost"
          className="brand"
          onClick={() => navigate("聊天")}
        >
          <span className="brand-mark">
            <LeafIcon size={19} />
          </span>
          摇光<span className="brand-en">FLUCTLIGHT</span>
        </Button>
        <span className="top-divider" />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="character-switch">
              <CharacterAvatar
                className="small"
                name={character.name}
                initial={character.initial}
                color={character.color}
                src={character._directory?.avatar}
              />
              <b>{character.name}</b>
              <ChevronDownIcon size={14} />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {state.characters.map((c: any) => (
              <DropdownMenuItem
                key={c.id}
                onSelect={() => {
                  workspace.select(c.id);
                  navigate(page);
                }}
              >
                <CharacterAvatar
                  className="small"
                  name={c.name}
                  initial={c.initial}
                  color={c.color}
                  src={c._directory?.avatar}
                />
                {c.name}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        <span className="scope">角色工作区</span>
        <span className="spacer" />
        <span className="local">
          <i />
          相伴此刻
        </span>
        <span className="self-avatar">我</span>
      </header>
      <div className="layout">
        <aside className="sidebar">{navigation}</aside>
        <main
          ref={main}
          tabIndex={-1}
          className={page === "聊天" ? "chat-main" : ""}
        >
          <div className="page-heading">
            <div>
              <div className="breadcrumb">
                {character.id ? `${character.name} 的工作区 ` : "我的空间 "}
                <ChevronRightIcon size={12} />
                {page}
              </div>
              <h1>
                {page}
                {page === "聊天" && (
                  <span className="heading-caption">
                    日常的片刻，与你分享。
                  </span>
                )}
              </h1>
            </div>
            {page === "聊天" && (
              <Button variant="outline" onClick={togglePreview}>
                {preview ? (
                  <PanelRightCloseIcon size={15} />
                ) : (
                  <PanelRightOpenIcon size={15} />
                )}{" "}
                {preview ? "收起预览" : "展开预览"}
              </Button>
            )}
          </div>
          <Suspense key={state.session?.actorId} fallback={<p role="status">正在读取页面…</p>}>
            {page === "聊天" ? (
              content
            ) : (
              <div className="page-content">{content}</div>
            )}
          </Suspense>
        </main>
      </div>
      {state.toast && (
        <div className="toast" role="status" data-testid="toast">
          {state.toast.text}{" "}
          {state.toast.link && (
            <Button
              variant="link"
              onClick={() => {
                workspace.state.outfitTab = "方案库";
                navigate("衣柜");
              }}
            >
              去方案库 ↗
            </Button>
          )}
        </div>
      )}
      {readError && (
        <div className="approved-read-feedback" role="alert">
          {readError}
          <Button variant="link" onClick={() => workspace.load(page)}>
            重新读取
          </Button>
        </div>
      )}
    </div>
  );
}
