import { readValue } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,
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
            <ActionButton
              className="text-button"
              onClick={(H) => (view.selected.value = null)}
            >
              {"世界列表"}
            </ActionButton>
            <span>{" / " + displayText(view.world.value.name)}</span>
            <span className="spacer" />
            <ActionButton
              className="button"
              onClick={(H) => (view.siteDialog.value = !0)}
            >
              {"新增场地分类"}
            </ActionButton>
          </div>
          <Surface as="div" className="card world-header">
            <div className="row">
              <span className="eyebrow">{"A WORLD OF YOUR OWN"}</span>
              <span className="spacer" />
              <ActionButton
                className="button"
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
              <ActionButton
                className="button"
                onClick={(H) => (view.expanded.value = !view.expanded.value)}
              >
                {displayText(view.expanded.value ? "结束编辑" : "编辑地点")}
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
            <ActionButton
              className="button"
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
            <Surface
              as="div"
              key={0}
              className="card empty-state"
              data-testid="world-empty-sites"
            >
              {[<p key={0}>{"暂无场地"}</p>]}
            </Surface>
          )}
          <Fragment>
            {listValues(view.world.value.sites).map((H, q) => (
              <Surface as="section" key={q} className="card compact">
                <div className="row">
                  <h3>{"场地 · " + displayText(H.name)}</h3>
                  <span className="spacer" />
                  {view.expanded.value ? (
                    <ActionButton
                      key={0}
                      className="text-button"
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
                          <ActionButton
                            key={0}
                            className="button edit-place"
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
              <h2>{"世界列表"}</h2>
              <p className="muted">{"每个人，都在自己的风景里生活。"}</p>
            </div>
            <span className="spacer" />
            <ActionButton
              className="button"
              data-testid="world-add"
              onClick={view.newWorld}
            >
              {"添加世界"}
            </ActionButton>
          </div>
          <div className="world-list">
            <Fragment>
              {listValues(readValue(view.worlds)).map((H) => (
                <Surface
                  as="article"
                  key={H.id}
                  className={cn([
                    "card",
                    {
                      current: H.residents.includes(
                        readValue(view.character).id,
                      ),
                    },
                  ])}
                >
                  <div className="row">
                    <h2>{displayText(H.name)}</h2>
                    {H.residents.includes(readValue(view.character).id) ? (
                      <Badge key={0} className="pill">
                        {"当前"}
                      </Badge>
                    ) : null}
                  </div>
                  <p>
                    {(H.genre ? displayText(H.genre) + " · " : "") +
                      displayText(H.sites.length) +
                      " 处场地"}
                  </p>
                  <p className="muted">
                    {"居住在这里：" +
                      displayText(
                        H.residents
                          .map((q) => {
                            var V;
                            return (V = readValue(view.characters).find(
                              (c) => c.id === q,
                            )) == null
                              ? void 0
                              : V.name;
                          })
                          .join("、") || "暂无角色",
                      )}
                  </p>
                  <div className="row">
                    <ActionButton
                      className="button primary"
                      onClick={(q) => {
                        ((view.selected.value = H.id),
                          (view.expanded.value = !1));
                      }}
                    >
                      {"进入"}
                    </ActionButton>
                    <MenuDisclosure title="⋯" aria-label={H.name + "的世界操作"}>
                    <ActionButton
                      className="text-button"
                      onClick={(q) => {
                        ((view.selected.value = H.id),
                          (view.expanded.value = !0));
                      }}
                    >
                      {"编辑"}
                    </ActionButton>
                    <ActionButton
                      className="text-button"
                      onClick={(q) => (view.removeId.value = H.id)}
                    >
                      {"删除"}
                    </ActionButton>
                    </MenuDisclosure>
                  </div>
                </Surface>
              ))}
            </Fragment>
          </div>
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
          onClose={(H) => (view.place.value = null)}
        >
          <form
            className="form-grid"
            onSubmit={eventModifiers(view.savePlace, ["prevent"])}
          >
            <view.ActionFeedback labels={["世界"]} />
            <label>
              {"地点名称"}
              <FieldInput
                readOnly={!!view.place.value._source}
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
              {view.place.value.name.trim() ? null : (
                <small key={0} className="inline-error">
                  {"请填写地点名称。"}
                </small>
              )}
            </label>
            <Fragment>
              {listValues(view.fields).map(([H, q]) => (
                <label key={H}>
                  {displayText(q)}
                  <FieldInput
                    value={view.place.value[H]}
                    onChange={(event) => {
                      ((V) => (view.place.value[H] = V))(event.target.value);
                    }}
                  />
                </label>
              ))}
            </Fragment>
            <ActionButton
              className="button primary"
              disabled={readValue(view.busy) || !view.place.value.name.trim()}
            >
              {displayText(
                readValue(view.busy) && view.operation.value === "place"
                  ? "处理中…"
                  : "保存地点",
              )}
            </ActionButton>
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
          onClose={(H) => (view.worldDraft.value = null)}
        >
          <form
            className="form-grid"
            onSubmit={eventModifiers(view.saveWorld, ["prevent"])}
          >
            <view.ActionFeedback labels={["世界"]} />
            <label>
              {"世界名称"}
              <FieldInput
                data-testid="world-name"
                value={view.worldDraft.value.name}
                onChange={(event) => {
                  ((H) => (view.worldDraft.value.name = H))(event.target.value);
                }}
              />
              {view.worldDraft.value.name.trim() ? null : (
                <small key={0} className="inline-error">
                  {"请填写世界名称。"}
                </small>
              )}
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
            <ActionButton
              className="button primary"
              disabled={
                readValue(view.busy) || !view.worldDraft.value.name.trim()
              }
              data-testid="world-save"
            >
              {displayText(
                readValue(view.busy) && view.operation.value === "world"
                  ? "处理中…"
                  : "保存世界",
              )}
            </ActionButton>
          </form>
        </view.DetailDrawer>
      ) : null}
      {view.siteDialog.value ? (
        <view.DetailDrawer
          key={4}
          title="添加场地分类"
          testId="site-drawer"
          onClose={(H) => (view.siteDialog.value = !1)}
        >
          <form
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
              {"分类名称"}
              <FieldInput
                value={view.siteName.value}
                onChange={(event) => {
                  ((H) => (view.siteName.value = H))(event.target.value);
                }}
              />
              {view.siteName.value.trim() ? null : (
                <small key={0}>{"请填写场地名称。"}</small>
              )}
            </label>
            <ActionButton
              className="button primary"
              disabled={readValue(view.busy) || !view.siteName.value.trim()}
            >
              {displayText(
                readValue(view.busy) && view.operation.value === "site"
                  ? "处理中…"
                  : "保存场地",
              )}
            </ActionButton>
          </form>
        </view.DetailDrawer>
      ) : null}
      {view.removeId.value !== null ? (
        <DialogFrame
          key={5}
          className="modal-backdrop"
          onClose={() => (view.removeId.value = null)}
        >
          <Surface as="section" className="card confirm-dialog">
            <h2>{"删除世界？"}</h2>
            <p>
              {"「" +
                displayText(
                  (le = readValue(view.worlds).find(
                    (H) => H.id === view.removeId.value,
                  )) == null
                    ? void 0
                    : le.name,
                ) +
                "」及其中的场地将被移除。"}
            </p>
            <ActionButton
              className="button"
              onClick={(H) => (view.removeId.value = null)}
            >
              {"取消"}
            </ActionButton>
            <ActionButton
              className="button primary"
              onClick={() => view.removeWorld(view.removeId.value)}
            >
              {"确认删除"}
            </ActionButton>
          </Surface>
        </DialogFrame>
      ) : null}
      {!view.place.value && !view.worldDraft.value && !view.siteDialog.value ? (
        <view.ActionFeedback key={6} labels={["世界"]} />
      ) : null}
    </Fragment>
  );
}
