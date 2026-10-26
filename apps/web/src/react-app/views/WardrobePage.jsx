const tabKeys = {
  为她搭配: "compose",
  方案库: "outfits",
  全部衣服: "garments",
};
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { DrawerFrame } from "@/react-app/controls";
import { readValue } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,
  ChoiceSelect,
  Disclosure,
  MenuDisclosure,
  DialogFrame,
  Badge,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
const At = ["外套", "上衣", "内衣", "内裤", "下衣", "袜子", "鞋子", "配饰"];
export function WardrobePageView({ view: view }) {
  return (
    <Tabs
      value={tabKeys[view.tab.value]}
      onValueChange={(value) => {
        const label = Object.keys(tabKeys).find(
          (key) => tabKeys[key] === value,
        );
        if (label) view.tab.value = label;
      }}
    >
      <TabsList className="tabs">
        <Fragment>
          {listValues(["为她搭配", "方案库", "全部衣服"]).map((x, rowIndex) => (
            <TabsTrigger
              className={cn({
                active: readValue(view.tab) === x,
              })}
              key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
              value={tabKeys[x]}
            >
              {displayText(
                x === "为她搭配"
                  ? "为" + readValue(view.character).name + "搭配"
                  : x,
              )}
            </TabsTrigger>
          ))}
        </Fragment>
      </TabsList>
      <TabsContent value={tabKeys[view.tab.value]} className="min-w-0">
        {readValue(view.tab) === "为她搭配" ? (
          <Fragment key={0}>
            <div className="toolbar">
              <label>
                {"先选标签 "}
                <ChoiceSelect
                  value={view.tag.value}
                  onChange={(event) => {
                    ((x) => (view.tag.value = x))(event.target.value);
                  }}
                >
                  <option value="">{"请选择"}</option>
                  <Fragment>
                    {listValues(readValue(view.character).outfitTags).map(
                      (x, rowIndex) => (
                        <option
                          key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                          value={x}
                        >
                          {displayText(view.tagLabel(x))}
                        </option>
                      ),
                    )}
                  </Fragment>
                </ChoiceSelect>
              </label>
              <small>{"标签可在方案库维护 · 每个部位选一件，配饰可多选"}</small>
            </div>
            <Surface
              as="div"
              className="card slot-picker scroll-lock-list"
              data-scroll-lock=""
            >
              {view.tag.value ? null : (
                <p key={0} className="muted" role="status">
                  {"请先选择标签"}
                </p>
              )}
              <Fragment>
                {listValues(readValue(At)).map((x, rowIndex) => (
                  <Disclosure
                    className="slot-accordion"
                    title={
                      <>
                        {displayText(x) + " "}
                        <small>
                          {displayText(
                            (view.items.value[x] || [])
                              .map((R) => {
                                var ye;
                                return (ye = readValue(
                                  view.character,
                                ).garments.find((F) => F.id === R)) == null
                                  ? void 0
                                  : ye.name;
                              })
                              .join("、") || "未选择",
                          )}
                        </small>
                      </>
                    }
                    key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                  >
                    <div
                      className="slot-row scroll-lock-list"
                      data-scroll-lock=""
                    >
                      <Fragment>
                        {listValues(
                          readValue(view.character).garments.filter(
                            (R) => R.type === x,
                          ),
                        ).map((R, rowIndex) => {
                          var ye;
                          return (
                            <ActionButton
                              aria-label={R.name}
                              disabled={!view.tag.value}
                              title={view.tag.value ? void 0 : "请先选择标签"}
                              className={cn([
                                "choice",
                                {
                                  chosen:
                                    (ye = view.items.value[x]) == null
                                      ? void 0
                                      : ye.includes(R.id),
                                },
                              ])}
                              onClick={(F) =>
                                view.choose(x, R.id, x === "配饰")
                              }
                              key={
                                typeof R === "object" ? (R?.id ?? rowIndex) : R
                              }
                            >
                              {displayText(R.name)}
                            </ActionButton>
                          );
                        })}
                      </Fragment>
                      {readValue(view.character).garments.some(
                        (R) => R.type === x,
                      ) ? null : (
                        <small key={0}>{"暂无衣服"}</small>
                      )}
                    </div>
                  </Disclosure>
                ))}
              </Fragment>
            </Surface>
            <Surface as="div" className="card">
              <h3>{"当前搭配"}</h3>
              <p className="slot-summary">
                {displayText(
                  view.summarize({
                    name: view.name.value,
                    tag: view.tag.value,
                    items: view.items.value,
                  }),
                )}
              </p>
            </Surface>
            {view.duplicate.value ? (
              <p key={0} className="inline-error">
                {"方案名称已存在，请换一个名称。"}
              </p>
            ) : null}
            {view.name.value.trim() ? null : (
              <p key={1} className="muted">
                {"请填写方案名称后保存。"}
              </p>
            )}
            <form
              className="input-row"
              onSubmit={eventModifiers(view.saveOutfit, ["prevent"])}
            >
              <FieldInput
                placeholder="方案名称"
                aria-label="方案名称"
                value={view.name.value}
                onChange={(event) => {
                  ((x) => (view.name.value = x))(event.target.value);
                }}
              />
              <ActionButton
                className="button primary"
                disabled={
                  readValue(view.busy) ||
                  view.duplicate.value ||
                  !view.tag.value ||
                  !view.name.value.trim()
                }
              >
                {displayText(
                  readValue(view.busy) && view.action.value === "save"
                    ? "处理中…"
                    : view.editingId.value
                      ? "保存修改"
                      : "保存为方案",
                )}
              </ActionButton>
              <ActionButton
                type="button"
                className="button"
                onClick={(x) => {
                  ((view.editingId.value = null),
                    (view.items.value = {}),
                    (view.name.value = ""));
                }}
              >
                {"重新搭配"}
              </ActionButton>
            </form>
          </Fragment>
        ) : readValue(view.tab) === "方案库" ? (
          <Fragment key={1}>
            <Surface as="div" className="card">
              <div className="row">
                <b>{"标签管理"}</b>
                <small>{"删除标签保留方案"}</small>
              </div>
              <div className="tag-list">
                <Fragment>
                  {listValues(readValue(view.character).outfitTags).map(
                    (x, rowIndex) => (
                      <Badge
                        className="pill"
                        key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                      >
                        {displayText(x) + " "}
                        <ActionButton
                          onClick={(R) => {
                            ((view.renameTag.value = x),
                              (view.newTag.value = x));
                          }}
                        >
                          {"改名"}
                        </ActionButton>
                        <ActionButton onClick={(R) => view.removeTag(x)}>
                          {"删除"}
                        </ActionButton>
                      </Badge>
                    ),
                  )}
                </Fragment>
              </div>
              <form
                className="input-row"
                onSubmit={eventModifiers(view.saveTags, ["prevent"])}
              >
                <FieldInput
                  placeholder={view.renameTag.value ? "新标签名" : "添加标签"}
                  aria-label="标签名称"
                  value={view.newTag.value}
                  onChange={(event) => {
                    ((x) => (view.newTag.value = x))(event.target.value);
                  }}
                />
                <ActionButton
                  className="button"
                  disabled={!view.newTag.value.trim()}
                >
                  {displayText(view.renameTag.value ? "保存标签" : "添加")}
                </ActionButton>
              </form>
            </Surface>
            <Fragment>
              {listValues(readValue(view.character).outfits).map((x) => (
                <Surface as="article" key={x.id} className="card compact">
                  <div className="row">
                    {view.renameOutfit.value === x.id ? (
                      <form
                        key={0}
                        className="input-row"
                        onSubmit={eventModifiers(
                          (R) => view.rename(x),
                          ["prevent"],
                        )}
                      >
                        <FieldInput
                          aria-label="方案新名称"
                          autoFocus=""
                          value={view.renameValue.value}
                          onChange={(event) => {
                            ((R) => (view.renameValue.value = R))(
                              event.target.value,
                            );
                          }}
                        />
                        <ActionButton
                          className="button"
                          disabled={
                            readValue(view.busy) ||
                            !view.renameValue.value.trim()
                          }
                        >
                          {displayText(
                            readValue(view.busy) &&
                              view.action.value === "rename"
                              ? "处理中…"
                              : "保存",
                          )}
                        </ActionButton>
                        <ActionButton
                          type="button"
                          className="text-button"
                          onClick={(R) => (view.renameOutfit.value = null)}
                        >
                          {"取消"}
                        </ActionButton>
                      </form>
                    ) : (
                      <b key={1}>{displayText(x.name)}</b>
                    )}
                    <Badge className="pill">
                      {displayText(view.tagLabel(x.tag) || "未分类")}
                    </Badge>
                    <span className="spacer" />
                    <MenuDisclosure
                      className="actions-menu"
                      onClick={(R) =>
                        R.target.closest("button") &&
                        R.currentTarget.removeAttribute("open")
                      }
                      title={<>{"⋯"}</>}
                      aria-label={x.name + "的方案操作"}
                    >
                      <div className="scroll-lock-list" data-scroll-lock="">
                        <ActionButton
                          onClick={(R) =>
                            (view.preview.value =
                              view.preview.value === x.id ? null : x.id)
                          }
                        >
                          {"预览"}
                        </ActionButton>
                        <ActionButton onClick={(R) => view.edit(x)}>
                          {"编辑搭配"}
                        </ActionButton>
                        <ActionButton
                          onClick={(R) => {
                            ((view.renameOutfit.value = x.id),
                              (view.renameValue.value = x.name));
                          }}
                        >
                          {"重命名"}
                        </ActionButton>
                        <ActionButton
                          disabled={true}
                          title="服务端保留方案及其历史；可编辑或重命名"
                        >
                          {"删除"}
                        </ActionButton>
                      </div>
                    </MenuDisclosure>
                    <ActionButton
                      className="button"
                      aria-label={
                        "请" +
                        readValue(view.character).name +
                        "换上「" +
                        x.name +
                        "」"
                      }
                      disabled={readValue(view.busy)}
                      onClick={(R) => view.wear(x)}
                    >
                      {displayText(
                        readValue(view.busy) &&
                          view.action.value === "wear" &&
                          view.selectedOutfit.value === x.id
                          ? "处理中…"
                          : "请" + readValue(view.character).name + "换上",
                      )}
                    </ActionButton>
                  </div>
                  <view.ActionFeedback
                    labels={["换装"]}
                    operation={"wear-outfit:" + x.id}
                    showStatus={true}
                  />
                  {view.preview.value === x.id ? (
                    <p key={0} className="slot-summary">
                      {displayText(view.summarize(x))}
                    </p>
                  ) : null}
                </Surface>
              ))}
            </Fragment>
            {readValue(view.character).outfits.length ? null : (
              <p key={0} className="muted">
                {"还没有方案，去搭配一套吧。"}
              </p>
            )}
          </Fragment>
        ) : (
          <Fragment key={2}>
            <div className="toolbar">
              <ChoiceSelect
                aria-label="按类型筛选"
                value={view.type.value}
                onChange={(event) => {
                  ((x) => (view.type.value = x))(event.target.value);
                }}
              >
                <option value="">{"全部类型"}</option>
                <Fragment>
                  {listValues(readValue(At)).map((x, rowIndex) => (
                    <option
                      key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                    >
                      {displayText(x)}
                    </option>
                  ))}
                </Fragment>
              </ChoiceSelect>
              <ChoiceSelect
                aria-label="按季节筛选"
                value={view.season.value}
                onChange={(event) => {
                  ((x) => (view.season.value = x))(event.target.value);
                }}
              >
                <option value="">{"全部季节"}</option>
                <Fragment>
                  {listValues(["春", "夏", "秋", "冬"]).map((x, rowIndex) => (
                    <option
                      key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                    >
                      {displayText(x)}
                    </option>
                  ))}
                </Fragment>
              </ChoiceSelect>
              <span className="spacer" />
              <ActionButton
                className="button"
                onClick={(x) =>
                  (view.garment.value = {
                    id: Date.now(),
                    name: "",
                    type: "上衣",
                    seasons: ["秋"],
                  })
                }
              >
                {"添加衣服"}
              </ActionButton>
            </div>
            <div className="garment-grid">
              <Fragment>
                {listValues(view.filteredGarments.value).map((x) => (
                  <Surface
                    as="article"
                    key={x.id}
                    className="card garment-card"
                  >
                    <div
                      className={cn([
                        "garment-thumb",
                        "garment-type-" + x.type,
                      ])}
                    >
                      <view.GarmentArt type={x.type} />
                      <small>{displayText(x.type)}</small>
                    </div>
                    <div className="row">
                      <div>
                        <h3>{displayText(x.name)}</h3>
                        <small>{displayText(x.seasons.join(" / "))}</small>
                      </div>
                      <span className="spacer" />
                      <MenuDisclosure
                        className="actions-menu"
                        onClick={(R) =>
                          R.target.closest("button") &&
                          R.currentTarget.removeAttribute("open")
                        }
                        title={<>{"⋯"}</>}
                        aria-label={x.name + "的衣服操作"}
                      >
                        <div className="scroll-lock-list" data-scroll-lock="">
                          <ActionButton
                            onClick={(R) =>
                              (view.garment.value = JSON.parse(
                                JSON.stringify(x),
                              ))
                            }
                          >
                            {"编辑"}
                          </ActionButton>
                          <ActionButton
                            onClick={(R) => view.removeGarment(x.id)}
                          >
                            {"删除"}
                          </ActionButton>
                        </div>
                      </MenuDisclosure>
                    </div>
                  </Surface>
                ))}
              </Fragment>
              {view.filteredGarments.value.length ? null : (
                <p key={0} className="muted">
                  {"没有符合条件的衣服。"}
                </p>
              )}
            </div>
          </Fragment>
        )}
        {view.garment.value ? (
          <DrawerFrame
            key={3}
            className="drawer-backdrop"
            onClose={(x) => (view.garment.value = null)}
          >
            <form
              data-testid="garment-drawer"
              className="drawer-form form-grid"
              aria-label="衣服"
              onSubmit={eventModifiers(view.saveGarment, ["prevent"])}
            >
              <view.ActionFeedback labels={["衣柜"]} />
              <h2>
                {displayText(
                  readValue(view.character).garments.some((x) => {
                    var R;
                    return (
                      x.id ===
                      ((R = view.garment.value) == null ? void 0 : R.id)
                    );
                  })
                    ? "编辑衣服"
                    : "添加衣服",
                )}
              </h2>
              <label>
                {"名称"}
                <FieldInput
                  value={view.garment.value.name}
                  onChange={(event) => {
                    ((x) => (view.garment.value.name = x))(event.target.value);
                  }}
                />
                {view.garment.value.name.trim() ? null : (
                  <small key={0} className="inline-error">
                    {"请填写衣服名称。"}
                  </small>
                )}
              </label>
              <label>
                {"类型"}
                <ChoiceSelect
                  value={view.garment.value.type}
                  onChange={(event) => {
                    ((x) => (view.garment.value.type = x))(event.target.value);
                  }}
                >
                  <Fragment>
                    {listValues(readValue(At)).map((x, rowIndex) => (
                      <option
                        key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                      >
                        {displayText(x)}
                      </option>
                    ))}
                  </Fragment>
                </ChoiceSelect>
              </label>
              <div className="row">
                <Fragment>
                  {listValues(["春", "夏", "秋", "冬"]).map((x, rowIndex) => (
                    <label
                      key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                    >
                      <FieldInput
                        type="checkbox"
                        value={x}
                        modelValue={view.garment.value.seasons}
                        onModelChange={(R) => (view.garment.value.seasons = R)}
                      />
                      {displayText(x)}
                    </label>
                  ))}
                </Fragment>
              </div>
              <div className="row">
                <ActionButton
                  className="button primary"
                  disabled={
                    readValue(view.busy) || !view.garment.value.name.trim()
                  }
                >
                  {displayText(
                    readValue(view.busy) && view.action.value === "garment"
                      ? "处理中…"
                      : "保存衣服",
                  )}
                </ActionButton>
                <ActionButton
                  type="button"
                  className="button"
                  onClick={(x) => (view.garment.value = null)}
                >
                  {"取消"}
                </ActionButton>
              </div>
            </form>
          </DrawerFrame>
        ) : null}
        {view.confirmation.value ? (
          <DialogFrame
            key={4}
            className="modal-backdrop"
            onClose={(x) => (view.confirmation.value = null)}
          >
            <Surface
              as="section"
              className="card confirm-dialog"
              role="alertdialog"
              aria-modal="true"
              aria-label={view.confirmation.value.title}
            >
              <h2>{displayText(view.confirmation.value.title)}</h2>
              <p>{displayText(view.confirmation.value.message)}</p>
              <div className="row">
                <ActionButton
                  className="button"
                  onClick={(x) => (view.confirmation.value = null)}
                >
                  {"取消"}
                </ActionButton>
                <ActionButton
                  className="button primary"
                  onClick={(x) => {
                    (view.confirmation.value.run(),
                      (view.confirmation.value = null));
                  }}
                >
                  {"确认删除"}
                </ActionButton>
              </div>
            </Surface>
          </DialogFrame>
        ) : null}
        {view.garment.value ? null : (
          <view.ActionFeedback
            key={5}
            labels={["衣柜", "换装"]}
            excludeOperationPrefix={
              readValue(view.tab) === "方案库" ? "wear-outfit:" : ""
            }
          />
        )}
      </TabsContent>
    </Tabs>
  );
}
