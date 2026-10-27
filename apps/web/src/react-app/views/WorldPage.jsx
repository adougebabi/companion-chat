import {AlertDialogCancel,AlertDialogAction} from "@/components/ui/alert-dialog";
import {CardHeader,CardTitle,CardContent,CardFooter} from "@/components/ui/card";
import EntityCardGrid from "@/react-app/shared/EntityCardGrid";
import WorldScenePreview from "@/react-app/shared/WorldScenePreview";
import ControlHint from "@/react-app/shared/ControlHint";
import {MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import {InlineFeedback,EmptyState} from "@/react-app/shared/Presentation";
import ConfirmationFrame from "@/react-app/shared/ConfirmationFrame";
import { readValue } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,Textarea,
  DialogFrame,
  Badge,
  MenuDisclosure,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
export function WorldPageView({ view: view }) {
  var K, le;
  return (
    <Fragment>
      {view.world.value ? (
        <Fragment key={1}>
          <div className="row">
            <ActionButton variant="ghost"

              onClick={(H) => (view.selected.value = null)}
            >
              {"返回世界列表"}
            </ActionButton>

            <span className="spacer" />
            <ActionButton variant="outline"

              onClick={(H) => (view.siteDialog.value = !0)}
            >
              {"新增场地分类"}
            </ActionButton>
          </div>
          <Surface as="div" className="world-header">
            <div className="row">
              <span className="eyebrow">{"生活世界"}</span>
              <span className="spacer" />
              <ActionButton variant="outline"

                onClick={(H) =>
                  (view.worldDraft.value = {
                    id: view.world.value.id,
                    name: view.world.value.name,
                    genre: view.world.value.genre,
                    _source: view.world.value._source,
                  })
                }
              >
                {"编辑世界信息"}
              </ActionButton>
              <ActionButton variant="outline"

                onClick={(H) => (view.expanded.value = !view.expanded.value)}
              >
                {displayText(view.expanded.value ? "完成地点管理" : "管理场地与地点")}
              </ActionButton>
            </div>
            <h2>{displayText(view.world.value.name)}</h2>
            <p>{displayText(view.world.value.genre)}</p>
          </Surface>
          <div className="toolbar">
            <FieldInput
              placeholder="补充扩展方向，例如：河岸书店"
              value={view.instruction.value}
              onChange={(event) => {
                ((H) => (view.instruction.value = H))(event.target.value);
              }}
            />
            <ActionButton variant="outline"

              disabled={readValue(view.busy)}
              onClick={view.expandWorld}
            >
              {displayText(
                readValue(view.busy) && view.operation.value === "expand"
                  ? "处理中…"
                  : "AI 扩展世界",
              )}
            </ActionButton>
          </div>
          {view.world.value.sites.length ? null : (
            <EmptyState
              as="div"
              key={0}

              data-testid="world-empty-sites"
            >
              {[<p key={0}>{"暂无场地"}</p>]}
            </EmptyState>
          )}
          <Fragment>
            {listValues(view.world.value.sites).map((H, q) => (
              <Surface as="section" key={q} className="compact">
                <div className="row">
                  <h3>{"场地 · " + displayText(H.name)}</h3>
                  <span className="spacer" />
                  {view.expanded.value ? (
                    <ActionButton variant="ghost"
                      key={0}

                      onClick={(V) => {
                        ((view.siteIndex.value = q),
                          (view.place.value = {
                            id: Date.now(),
                            name: "",
                            alias: "",
                            floor: "",
                            description: "",
                            atmosphere: "",
                            furniture: "",
                            layout: "",
                            custom: !0,
                          }));
                      }}
                    >
                      {"添加地点"}
                    </ActionButton>
                  ) : null}
                </div>
                {H.places.length ? null : (
                  <p key={0} className="muted">
                    {"暂无地点"}
                  </p>
                )}
                <div className="place-grid">
                  <Fragment>
                    {listValues(H.places).map((V) => (
                      <article key={V.id} className="place">
                        <div className="row">
                          <b>{displayText(V.alias || V.name)}</b>
                          {V.floor ? (
                            <small>{displayText(V.floor)}</small>
                          ) : null}
                        </div>
                        {V.description || V.atmosphere ? (
                          <p>
                            {[V.description, V.atmosphere]
                              .filter(Boolean)
                              .join(" · ")}
                          </p>
                        ) : null}
                        {V.furniture ? (
                          <small>
                            {"固定陈设：" + displayText(V.furniture)}
                          </small>
                        ) : null}
                        {V.layout ? (
                          <small>{"布局：" + displayText(V.layout)}</small>
                        ) : null}
                        {view.expanded.value ? (
                          <ActionButton variant="outline"
                            key={0}
                            className="edit-place"
                            onClick={(U) => view.editPlace(V, q)}
                          >
                            {"编辑地点"}
                          </ActionButton>
                        ) : null}
                      </article>
                    ))}
                  </Fragment>
                </div>
              </Surface>
            ))}
          </Fragment>
        </Fragment>
      ) : (
        <Fragment key={0}>
          <div className="row">
            <div>
              <h2>{"返回世界列表"}</h2>
              <p className="muted">{"每个人，都在自己的风景里生活。"}</p>
            </div>
            <span className="spacer" />
            <ActionButton variant="default"

              data-testid="world-add"
              onClick={view.newWorld}
            >
              {"添加世界"}
            </ActionButton>
          </div>
          <EntityCardGrid className="world-list" data-testid="world-grid" aria-label="世界卡片">
            {listValues(readValue(view.worlds)).map(H=><Surface as="article" key={H.id} className={cn('entity-card world-card h-full gap-0 overflow-hidden p-0',H.residents.includes(readValue(view.character).id)&&'ring-primary/30')} data-testid={'world-card-'+H.id}>
              <WorldScenePreview world={H}/>
              <CardHeader className="gap-3"><div className="flex min-w-0 items-start justify-between gap-2"><ControlHint text={H.name}><CardTitle className="line-clamp-2 min-w-0 [overflow-wrap:anywhere]">{H.name}</CardTitle></ControlHint><div className="flex shrink-0 items-center gap-1">{H.residents.includes(readValue(view.character).id)&&<Badge variant="secondary">当前</Badge>}
                    <MenuDisclosure title={<MoreActionsIcon className="size-4"/>} aria-label={H.name + "的世界操作"}>
                    <ActionButton variant="ghost"

                      onClick={(q) => {
                        ((view.selected.value = H.id),
                          (view.expanded.value = !0));
                      }}
                    >
                      {"编辑"}
                    </ActionButton>
                    <ActionButton variant="destructive"

                      onClick={(q) => (view.removeId.value = H.id)}
                    >
                      {"删除"}
                    </ActionButton>
                    </MenuDisclosure>
              </div></div></CardHeader>
              <CardContent className="flex flex-1 flex-col gap-3 pb-5"><p className="m-0 text-sm">{(H.genre?H.genre+' · ':'')+H.sites.length+' 处场地'}</p><p className="m-0 break-words text-sm text-muted-foreground">{'居住在这里：'+(H.residents.map(id=>readValue(view.characters).find(character=>character.id===id)?.name).filter(Boolean).join('、')||'暂无角色')}</p></CardContent>
              <CardFooter><ActionButton className="w-full" variant="default" onClick={()=>{view.selected.value=H.id;view.expanded.value=false;}}>进入</ActionButton></CardFooter>
            </Surface>)}
          </EntityCardGrid>
        </Fragment>
      )}
      {view.place.value ? (
        <view.DetailDrawer
          key={2}
          title={
            (K = view.world.value) != null &&
            K.sites.some((H) =>
              H.places.some((q) => {
                var V;
                return (
                  q.id === ((V = view.place.value) == null ? void 0 : V.id)
                );
              }),
            )
              ? "编辑地点"
              : "添加地点"
          }
          testId="place-drawer"
          footer={<><ActionButton variant="outline" type="button" onClick={()=>{view.place.value=null;}}>取消</ActionButton><ActionButton variant="default" type="submit" form="world-place-form" disabled={readValue(view.busy)||!view.place.value.name.trim()} data-testid="place-save">{readValue(view.busy)&&view.operation.value==='place'?'处理中…':'保存地点'}</ActionButton></>}
          onClose={(H) => (view.place.value = null)}
        >
          <form id="world-place-form"
            className="form-grid"
            onSubmit={eventModifiers(view.savePlace, ["prevent"])}
          >
            <view.ActionFeedback labels={["世界"]} />
            <label>
              {"地点名称"}<span aria-hidden="true" className="text-destructive"> *</span>
              <FieldInput
                required readOnly={!!view.place.value._source}
                title={
                  view.place.value._source
                    ? "正式名称固定，可修改别名"
                    : undefined
                }
                value={view.place.value.name}
                onChange={(event) => {
                  ((H) => (view.place.value.name = H))(event.target.value);
                }}
              />
              {view.place.value._source ? (
                <small className="muted">{"正式名称固定，可修改别名"}</small>
              ) : null}
              {!view.place.value.name.trim()&&<p className="text-sm text-muted-foreground">填写地点名称后可以保存。</p>}
            </label>
            <Fragment>
              {listValues(view.fields).map(([H, q]) => (
                <label key={H} className={['description','atmosphere','furniture','layout'].includes(H)?'full-field':undefined}>
                  {displayText(q)}
                  {['description','atmosphere','furniture','layout'].includes(H)?<Textarea rows={3} value={view.place.value[H]} onChange={event=>view.place.value[H]=event.target.value}/>:<FieldInput value={view.place.value[H]} onChange={event=>view.place.value[H]=event.target.value}/>}
                </label>
              ))}
            </Fragment>
          </form>
        </view.DetailDrawer>
      ) : null}
      {view.worldDraft.value ? (
        <view.DetailDrawer
          key={3}
          title={
            readValue(view.worlds).some((H) => {
              var q;
              return (
                H.id === ((q = view.worldDraft.value) == null ? void 0 : q.id)
              );
            })
              ? "编辑世界"
              : "添加世界"
          }
          testId="world-drawer"
          footer={<><ActionButton variant="outline" type="button" onClick={()=>{view.worldDraft.value=null;}}>取消</ActionButton><ActionButton variant="default" type="submit" form="world-info-form" disabled={readValue(view.busy)||!view.worldDraft.value.name.trim()} data-testid="world-save">{readValue(view.busy)&&view.operation.value==='world'?'处理中…':'保存世界'}</ActionButton></>}
          onClose={(H) => (view.worldDraft.value = null)}
        >
          <form id="world-info-form"
            className="form-grid"
            onSubmit={eventModifiers(view.saveWorld, ["prevent"])}
          >
            <view.ActionFeedback labels={["世界"]} />
            <label>
              {"世界名称"}<span aria-hidden="true" className="text-destructive"> *</span>
              <FieldInput
                required data-testid="world-name"
                value={view.worldDraft.value.name}
                onChange={(event) => {
                  ((H) => (view.worldDraft.value.name = H))(event.target.value);
                }}
              />
              {!view.worldDraft.value.name.trim()&&<p className="text-sm text-muted-foreground">填写世界名称后可以保存。</p>}
            </label>
            <label>
              {"世界类型"}
              <FieldInput
                data-testid="world-genre"
                value={view.worldDraft.value.genre}
                onChange={(event) => {
                  ((H) => (view.worldDraft.value.genre = H))(
                    event.target.value,
                  );
                }}
              />
            </label>
          </form>
        </view.DetailDrawer>
      ) : null}
      {view.siteDialog.value ? (
        <view.DetailDrawer
          key={4}
          title="添加场地分类"
          testId="site-drawer"
          footer={<><ActionButton variant="outline" type="button" onClick={()=>{view.siteDialog.value=false;}}>取消</ActionButton><ActionButton variant="default" type="submit" form="world-site-form" disabled={readValue(view.busy)||!view.siteName.value.trim()} data-testid="site-save">{readValue(view.busy)&&view.operation.value==='site'?'处理中…':'保存分类'}</ActionButton></>}
          onClose={(H) => (view.siteDialog.value = !1)}
        >
          <form id="world-site-form"
            className="form-grid"
            onSubmit={eventModifiers(view.addSite, ["prevent"])}
          >
            <p className="muted">
              {
                "分类用于组织地点，例如商店、公共设施。保存后可在分类中添加服装店等具体地点。"
              }
            </p>
            <view.ActionFeedback labels={["世界"]} />
            <label>
              {"分类名称"}<span aria-hidden="true" className="text-destructive"> *</span>
              <FieldInput
                required value={view.siteName.value}
                onChange={(event) => {
                  ((H) => (view.siteName.value = H))(event.target.value);
                }}
              />
              {view.siteName.value.trim() ? null : (
                <small key={0}>{"填写分类名称后可以保存。"}</small>
              )}
            </label>
          </form>
        </view.DetailDrawer>
      ) : null}
      {view.removeId.value!==null?<ConfirmationFrame title="删除世界？" description={'「'+(readValue(view.worlds).find(world=>world.id===view.removeId.value)?.name||'')+'」及其中的场地将被移除。'} onClose={()=>view.removeId.value=null}><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction variant="destructive" disabled={readValue(view.busy)} onClick={event=>{event.preventDefault();view.removeWorld(view.removeId.value);}}>确认删除</AlertDialogAction></ConfirmationFrame>:null}
      {!view.place.value && !view.worldDraft.value && !view.siteDialog.value ? (
        <view.ActionFeedback key={6} labels={["世界"]} />
      ) : null}
    </Fragment>
  );
}
