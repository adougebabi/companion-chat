import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuItem} from "@/components/ui/dropdown-menu";
import {InlineFeedback, EmptyState} from "@/react-app/shared/Presentation";
import LoadingState from "@/react-app/shared/LoadingState";
import {ArrowRightIcon,CheckIcon,ChevronRightIcon,LayoutGridIcon,ListIcon,PlusIcon,SearchIcon,XIcon} from "lucide-react";
import {useController} from "@/react-app/page";
import {selector} from "@/state/view-model";
import {Surface} from "@/react-app/controls";
import CharacterAvatar from "@/react-app/shared/CharacterAvatar";
import {StoryMark,WorldArt,worldArtworkKind} from "@/react-app/shared/WorkspaceArtwork";
import {currentReadError} from "@/approved-ui/read-feedback.mjs";
import {relativeTime} from "@/approved-ui/presentation.mjs";
import {migrateCreationRecovery,creationHistoryKey} from "@/approved-ui/creation-recovery.mjs";

function createLibraryPageController(__props, context, lifecycle) {
  const w = lifecycle.workspace,
    search = lifecycle.cell(""),
    view = lifecycle.cell("grid"),
    filters = lifecycle.cell({
      tag: "",
    }),
    configured = lifecycle.cell(null),
    saved = lifecycle.cell(null),
    error = lifecycle.cell("");
  let active = true;
  const characters = selector(() => w.state.characters),
    groups = selector(() => [
      {
        key: "tag",
        label: "标签",
        options: [...new Set(characters.value.flatMap((c) => c.tags || []))],
      },
    ]),
    rows = selector(() =>
      characters.value.filter(
        (c) =>
          [c.name, c.job, c.bio, ...(c.tags || [])]
            .filter(Boolean)
            .join(" ")
            .toLowerCase()
            .includes(search.value.trim().toLowerCase()) &&
          (!filters.value.tag || c.tags?.includes(filters.value.tag)),
      ),
    );
  function progressHref(id) {
    return localStorage.getItem(creationHistoryKey(w.state.session.actorId, id))
      ? "#/library/create?resume=" + encodeURIComponent(id)
      : "";
  }
  function details(id) {
    w.select(id);
    w.navigate("角色详情");
  }
  function choose(id) {
    w.select(id);
    w.navigate("聊天");
  }
  const subtitle = (c) =>
    [c.job, c.detail?.world?.name || c.detail?.identity?.residence].filter(Boolean).join(" · ");
  const lastConversation = c => {
    const value=c._directory?.lastConversationAt;
    const label=relativeTime(value,c.timezone||'Asia/Shanghai');
    return label?'最近相处：'+label:value===null?'还没有会话记录':'最近相处时间尚未提供';
  };
  const owner = w.state.session.actorId;
  const uiKey = `fluctlight:library-view:${owner}`;
  const saveView = () => {
    try {
      if(w.state.session?.actorId!==owner){sessionStorage.removeItem(uiKey);return;}
      sessionStorage.setItem(uiKey, JSON.stringify({search: search.value, tag: filters.value.tag, view: view.value}));
    } catch { /* UI preferences are optional. */ }
  };
  lifecycle.onStart(() => {
    try {
      const stored = JSON.parse(sessionStorage.getItem(uiKey) || "null");
      if (stored) {
        search.value = typeof stored.search === "string" ? stored.search : "";
        filters.value = {tag: typeof stored.tag === "string" ? stored.tag : ""};
        view.value = stored.view === "list" ? "list" : "grid";
      }
    } catch { /* An invalid UI preference cannot block real data. */ }
    try {
      migrateCreationRecovery(
        w.state.session.actorId,
        localStorage,
        sessionStorage,
      );
      const raw = localStorage.getItem(
        `fluctlight-creation:${w.state.session.actorId}:draft`,
      );
      if (raw) saved.value = JSON.parse(raw);
    } catch {
      error.value = "上次的创建资料需要核对，可以从继续创建入口查看。";
    }
    void w.services.configurationClient
      .read()
      .then((value) => {
        if (active)
          configured.value = value.bindings.some(
            (b) => b.role === "generic_llm",
          );
      })
      .catch(() => {
        if (active) configured.value = null;
      });
  });
  lifecycle.observe(() => [search.value, filters.value.tag, view.value], saveView);
  lifecycle.onDispose(() => { active = false; saveView(); });
  return () => {
    const current = characters.value.find(character=>character.id===w.state.selectedId) || {};
    const createHref = saved.value?.activatedId ? "#/library/create?new=1" : "#/library/create";
    const readError = currentReadError(w.state, "library", JSON.stringify([w.state.session.actorId,w.state.selectedId]));
    const allTags = groups.value[0].options, tags = allTags.slice(0,6), extraTags = allTags.slice(6);
    return <div className="replica-library" data-testid="replica-library" aria-busy={!!w.state.loading?.library}>
      <section className="library-hero">
        <div className="hero-copy">
          <div className="eyebrow">A LITTLE WORLD, A MEANINGFUL CONNECTION</div>
          <h1>每一场相遇，<br/>都值得认真开始。</h1>
          <p>在自己的世界里生活，也在日常里慢慢靠近。<br/>这里，收藏着你们的每一个故事。</p>
          <Button asChild><a href={createHref}><PlusIcon/>{saved.value && !saved.value.activatedId ? "继续这段相识" : "开启新的相识"}<ArrowRightIcon/></a></Button>
        </div>
        <div className="hero-orbit" aria-hidden="true"/>
        <div className="contents" aria-hidden="true"><StoryMark className="hero-plant"/></div>
        {current.id && <div className="library-resume">
          <small>CONTINUE YOUR STORY</small>
          <div className="resume-character"><CharacterAvatar name={current.name} initial={current.initial} color={current.color} src={current._directory?.avatar}/><div><h3>{current.name}</h3><p>你的上一次相遇，还在这里。</p></div></div>
          <Button variant="outline" onClick={()=>choose(current.id)}>继续相处<ArrowRightIcon/></Button>
        </div>}
      </section>
      {saved.value && <Surface as="aside" className="library-draft-notice"><div><b>{saved.value.activatedId ? "继续查看准备进度" : "还有一段尚未开始的故事。"}</b><p>上次的资料和准备进度已经为你保留。</p></div><Button variant="outline" asChild><a href="#/library/create">继续创建<ArrowRightIcon/></a></Button></Surface>}
      {error.value && <InlineFeedback role="alert">{error.value}</InlineFeedback>}
      {readError && <InlineFeedback role="alert" className="library-read-error"><p>角色资料暂未完整读到，已保留当前角色列表。{readError}</p><Button variant="outline" disabled={!!w.state.loading?.library} onClick={()=>void w.load("角色库")}>重新读取角色资料</Button></InlineFeedback>}
      <div className="library-header"><h2>我的角色 <span>{characters.value.length} 位相识</span></h2><div className="library-actions">
        <div className="library-search"><SearchIcon size={15}/><Input type="search" aria-label="搜索名字、职业或标签" placeholder="搜索名字、职业或标签" value={search.value} onChange={event=>search.value=event.target.value}/>{search.value && <Button variant="ghost" size="icon" aria-label="清除搜索" onClick={()=>search.value=""}><XIcon/></Button>}</div>
        <div className="library-view" role="group" aria-label="角色列表视图"><Button variant={view.value==="grid"?"secondary":"ghost"} size="icon" aria-label="网格视图" aria-pressed={view.value==="grid"} onClick={()=>view.value="grid"}><LayoutGridIcon/></Button><Button variant={view.value==="list"?"secondary":"ghost"} size="icon" aria-label="列表视图" aria-pressed={view.value==="list"} onClick={()=>view.value="list"}><ListIcon/></Button></div>
      </div></div>
      <div className="tag-filter" aria-label="角色标签"><Button variant="ghost" aria-pressed={!filters.value.tag} className={!filters.value.tag?"active":""} onClick={()=>filters.value={tag:""}}>全部</Button>{tags.map(tag=><Button variant="ghost" key={tag} aria-pressed={filters.value.tag===tag} className={filters.value.tag===tag?"active":""} onClick={()=>filters.value={tag}}>{tag}</Button>)}{extraTags.length>0&&<DropdownMenu><DropdownMenuTrigger asChild><Button variant="ghost" className={extraTags.includes(filters.value.tag)?"active":""} aria-label="更多角色标签">{extraTags.includes(filters.value.tag)?filters.value.tag:"更多标签"}</Button></DropdownMenuTrigger><DropdownMenuContent>{extraTags.map(tag=><DropdownMenuItem key={tag} onSelect={()=>filters.value={tag}}>{tag}</DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu>}</div>
      {w.state.loading?.library && <LoadingState label="正在读取角色资料…"/>}
      <div className={view.value==="grid"?"character-grid":"character-list"} data-testid="library-grid" aria-label="角色卡片">
        {rows.value.map(character=><Card asChild key={character.id} className="library-card"><article data-testid={"library-card-"+character.id}>
          <div className="library-card-art" aria-hidden="true"><WorldArt kind={worldArtworkKind(character.detail?.world)}/></div>
          <span className={"library-status "+(w.state.selectedId===character.id?"status-current":"")}>{w.state.selectedId===character.id?"当前相处":({active:"生活中",正常:"生活中",paused:"已暂停",已暂停:"已暂停",retired:"已退休",已退休:"已退休",failed:"需要处理",异常:"需要处理"}[character.status] || "准备中")}</span>
          <div className="library-card-content"><div className="library-card-title"><CharacterAvatar className="library-avatar" name={character.name} initial={character.initial} color={character.color} src={character._directory?.avatar}/><div className="library-card-name"><h3>{character.name}</h3><p>{subtitle(character) || "生活资料待补充"}</p></div></div>
          <p>{character.detail?.identity?.tagline || character.bio || "生活细节会在相处中慢慢补充。"}</p>
          <div className="library-chips">{(character.tags||[]).map(tag=><span key={tag}>{tag}</span>)}</div>
          <div className="library-card-bottom"><span>{lastConversation(character)}</span><div className="library-actions"><Button variant="ghost" onClick={()=>details(character.id)}>角色档案</Button><Button variant="secondary" onClick={()=>choose(character.id)}>进入聊天<ArrowRightIcon/></Button></div></div>
          {progressHref(character.id)&&<a className="library-progress" href={progressHref(character.id)}>查看准备进度<ChevronRightIcon size={12}/></a>}
          </div>
        </article></Card>)}
      </div>
      {!rows.value.length && !w.state.loading?.library && (characters.value.length ? <EmptyState><h3>没有找到这位相识</h3><p>换一个关键词，或者从一个新的故事开始。</p><Button variant="outline" onClick={()=>{search.value="";filters.value={tag:""};}}>清除筛选</Button></EmptyState> : <Surface as="section" className="library-first-use"><h2>还没有一起生活的人</h2><p>从一个名字或一段介绍开始，为新的相识准备一个生活世界。</p>{configured.value===false&&<p>还没有连接通用模型。可以先配置模型，也可以选择空白创建。</p>}<div className="library-actions">{configured.value!==true&&<Button variant="outline" asChild><a href="#/settings?section=%E6%A8%A1%E5%9E%8B%E7%94%A8%E9%80%94">先连接模型</a></Button>}<Button asChild><a data-testid="library-create-first" href={createHref}>{saved.value&&!saved.value.activatedId?"继续创建第一个角色":"创建第一个角色"}</a></Button></div></Surface>)}
      <div className="library-footer"><span><CheckIcon size={12}/>角色资料来自当前工作区</span><Button variant="link" asChild><a href="#/studio?kind=character_template">去创作中心寻找灵感<ChevronRightIcon size={12}/></a></Button></div>
    </div>;
  };
}
export default function LibraryPage(input={}) { return useController(createLibraryPageController,input); }
