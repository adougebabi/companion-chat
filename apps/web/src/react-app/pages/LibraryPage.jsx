import {Card,CardHeader,CardTitle,CardDescription,CardContent,CardFooter} from "@/components/ui/card";
import EntityCardGrid from "@/react-app/shared/EntityCardGrid";
import ControlHint from "@/react-app/shared/ControlHint";
import {InlineFeedback,EmptyState} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import { useController } from "@/react-app/page";
import { selector, readValue } from "@/state/view-model";
import {
  ActionButton,
  Surface,
  MenuDisclosure,
  Badge,
} from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
import { FilterChips } from "@/react-app/components";
import CharacterAvatar from "@/react-app/shared/CharacterAvatar";
import {
  migrateCreationRecovery,
  creationHistoryKey,
} from "@/approved-ui/creation-recovery.mjs";
import { LibraryIcon } from "lucide-react";
function createLibraryPageController(__props, context, lifecycle) {
  const w = lifecycle.workspace,
    search = lifecycle.cell(""),
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
          [c.name, c.job, c.bio]
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
    [c.job, c.detail?.identity?.residence].filter(Boolean).join(" · ");
  lifecycle.onStart(() => {
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
  lifecycle.onDispose(() => (active = false));
  return (_ctx, _cache) => {
    return (
      <div className="library">
        <div className="row library-toolbar">
          <p className="muted">
            {displayText(
              characters.value.length
                ? "在这里，遇见和你一起生活的人。"
                : "相识，可以从一句简单的介绍开始。",
            )}
          </p>
          <span className="spacer" />
          {characters.value.length ? (
            <ActionButton variant="default" asChild  key={0}>
              <a
                key={0}

                href={
                  saved.value?.activatedId
                    ? "#/library/create?new=1"
                    : "#/library/create"
                }
              >
                {displayText(
                  saved.value && !saved.value.activatedId
                    ? "继续创建"
                    : "创建角色",
                )}
              </a>
            </ActionButton>
          ) : null}
        </div>
        {saved.value ? (
          <Surface key={0} as="aside" className="approved-guidance">
            <b>
              {displayText(
                saved.value.activatedId ? "继续查看准备进度" : "继续上次的相识",
              )}
            </b>
            <p>{"上次的资料和准备进度已经为你保留。"}</p>
            <ActionButton variant="link" className="w-fit self-start" asChild>
              <a href="#/library/create">
                {"继续创建与查看进度 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
              </a>
            </ActionButton>
          </Surface>
        ) : null}
        {error.value ? (
          <InlineFeedback key={1}  role="alert">
            {displayText(error.value)}
          </InlineFeedback>
        ) : null}
        {characters.value.length ? (
          <Fragment key={2}>
            <FilterChips
              search={search.value}
              onUpdateSearch={($event) => (search.value = $event)}
              filters={filters.value}
              onUpdateFilters={($event) => (filters.value = $event)}
              groups={groups.value}
              testId="library-filters"
              placeholder="搜索角色 / 职业"
            />
            <EntityCardGrid className="library-grid" data-testid="library-grid" aria-label="角色卡片">
              {listValues(rows.value).map((character)=><Card key={character.id} asChild className="entity-card h-full gap-0 overflow-hidden py-0"><article className="library-character" data-testid={'library-card-'+character.id}>
                <CardHeader className="gap-3 pb-4"><div className="flex min-w-0 items-start justify-between gap-3"><CharacterAvatar className="size-12" name={character.name} initial={character.initial} color={character.color} src={character._directory?.avatar}/><div className="min-w-0 flex-1"><CardTitle className="line-clamp-2 break-words">{character.name}</CardTitle>{subtitle(character)&&<CardDescription>{subtitle(character)}</CardDescription>}</div><div className="flex shrink-0 items-center gap-1">{readValue(w).state.selectedId===character.id&&<Badge variant="secondary">当前</Badge>}
                      <MenuDisclosure
                        className="actions-menu"
                        title={<><MoreActionsIcon className="size-4" aria-hidden="true"/></>}
                        aria-label={character.name + "的操作"}
                      >
                        <div>
                          <ActionButton variant="ghost"
                            onClick={($event) => details(character.id)}
                          >
                            {"角色详情"}
                          </ActionButton>
                          {progressHref(character.id) ? (
                            <ActionButton variant="ghost"
                              asChild

                              key={0}
                            >
                              <a
                                key={0}

                                href={progressHref(character.id)}
                              >
                                {"查看准备进度"}
                              </a>
                            </ActionButton>
                          ) : null}
                          {readValue(w).state.selectedId !== character.id ? (
                            <ActionButton variant="ghost"
                              key={1}
                              onClick={($event) => choose(character.id)}
                            >
                              {"切换角色"}
                            </ActionButton>
                          ) : null}
                        </div>
                      </MenuDisclosure>
                </div></div></CardHeader>
                <CardContent className="flex flex-1 flex-col gap-3 pb-4">{!character.bio&&!(character.tags||[]).length&&<p className="text-sm text-muted-foreground">生活细节会在相处中慢慢补充。</p>}{character.bio&&<p className="m-0 line-clamp-3 break-words text-sm text-muted-foreground">{character.bio}</p>}<div className="flex flex-wrap gap-1.5">{(character.tags||[]).map(tag=><Badge key={tag} variant="outline">{tag}</Badge>)}</div></CardContent>
                <CardFooter className="grid grid-cols-2 gap-3 [&>button]:w-full"><ActionButton variant="outline" onClick={()=>details(character.id)}>角色详情</ActionButton><ActionButton variant="default" onClick={()=>choose(character.id)}>进入聊天</ActionButton></CardFooter>
              </article></Card>)}
            </EntityCardGrid>
            {!rows.value.length ? (
              <EmptyState key={0} as="div" >
                <p>{"暂无匹配角色。"}</p>
                <ActionButton variant="outline"

                  onClick={($event) => {
                    search.value = "";
                    filters.value = {
                      tag: "",
                    };
                  }}
                >
                  {"清除筛选"}
                </ActionButton>
              </EmptyState>
            ) : null}
          </Fragment>
        ) : (
          <Surface key={3} as="section" className="library-first-use">
            <LibraryIcon size={30} aria-hidden="true" />
            <h2>{"还没有一起生活的人"}</h2>
            <p>
              {" 从一个名字或一段介绍开始，为新的相识准备一个生活世界。"}
              <br />
              {"我们会一步步陪你整理资料，再由你确认开启生活。 "}
            </p>
            {configured.value === false ? (
              <p key={0} className="approved-guidance">
                {
                  " 还没有连接通用模型。描述分析需要先配置模型，也可以选择不调用模型的空白创建。 "
                }
              </p>
            ) : null}
            <div className="row first-use-actions">
              {configured.value !== true ? (
                <ActionButton variant="outline" asChild  key={0}>
                  <a
                    key={0}

                    href="#/settings?section=%E6%A8%A1%E5%9E%8B%E7%94%A8%E9%80%94"
                  >
                    {"先连接模型"}
                  </a>
                </ActionButton>
              ) : null}
              <ActionButton variant="default" asChild >
                <a

                  data-testid="library-create-first"
                  href="#/library/create"
                >
                  {displayText(
                    saved.value ? "继续创建第一个角色" : "创建第一个角色",
                  )}
                </a>
              </ActionButton>
            </div>
          </Surface>
        )}
      </div>
    );
  };
}
export default function LibraryPage(input = {}) {
  return useController(createLibraryPageController, {
    ...input,
  });
}
