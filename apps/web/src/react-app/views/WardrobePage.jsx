import Combobox from '@/react-app/shared/Combobox';
import {Field,FieldLabel} from '@/components/ui/field';
import {AlertDialogCancel,AlertDialogAction} from "@/components/ui/alert-dialog";
import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {InlineFeedback} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import ConfirmationFrame from "@/react-app/shared/ConfirmationFrame";
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
      <TabsList className="h-auto group-data-horizontal/tabs:h-auto max-w-full flex-wrap">
        <Fragment>
          {listValues(["为她搭配", "方案库", "全部衣服"]).map((x, rowIndex) => (
            <TabsTrigger className="h-auto min-h-10 max-w-full flex-none whitespace-normal break-words"
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
              <Field className="w-full min-w-0 sm:max-w-72">
                <FieldLabel htmlFor="wardrobe-tag">先选标签</FieldLabel>
                <Combobox id="wardrobe-tag" label="搭配标签" value={view.tag.value} options={[{value:'',label:'请选择标签'},...readValue(view.character).outfitTags.map(tag=>({value:tag,label:view.tagLabel(tag)}))]} onChange={value=>view.tag.value=value}/>
              </Field>
              <small>{"标签可在方案库维护 · 每个部位选一件，配饰可多选"}</small>
            </div>
            {view.tag.value?<Surface as="div" className="slot-picker">
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
                      className="slot-row"
                    >
                      <Fragment>
                        {listValues(
                          readValue(view.character).garments.filter(
                            (R) => R.type === x,
                          ),
                        ).map((R, rowIndex) => {
                          var ye;
                          return (
                            <ActionButton variant="ghost"
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
            </Surface>:<p className="py-4 text-sm text-muted-foreground" role="status">先选择一个搭配标签，再挑选各部位的衣服。</p>}
            <Surface as="div" >
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
              <InlineFeedback key={0} >
                {"方案名称已存在，请换一个名称。"}
              </InlineFeedback>
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
              <ActionButton variant="default"

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
              <ActionButton variant="outline"
                type="button"

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
            <Surface as="div" >
              <div className="row">
                <b>{"标签管理"}</b>
                <small>{"删除标签保留方案"}</small>
              </div>
              <div className="tag-list">
                <Fragment>
                  {listValues(readValue(view.character).outfitTags).map(
                    (x, rowIndex) => (
                      <div className="wardrobe-tag"

                        key={typeof x === "object" ? (x?.id ?? rowIndex) : x}
                      >
                        <Badge className="h-auto min-h-5 max-w-full whitespace-normal break-all">{displayText(x)}</Badge>
                        <ActionButton variant="ghost"
                          onClick={(R) => {
                            ((view.renameTag.value = x),
                              (view.newTag.value = x));
                          }}
                        >
                          {"改名"}
                        </ActionButton>
                        <ActionButton variant="ghost" onClick={(R) => view.removeTag(x)}>
                          {"删除"}
                        </ActionButton>
                      </div>
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
                <ActionButton variant="outline"

                  disabled={!view.newTag.value.trim()}
                >
                  {displayText(view.renameTag.value ? "保存标签" : "添加")}
                </ActionButton>
              </form>
            </Surface>
            <Fragment>
              {listValues(readValue(view.character).outfits).map((x) => (
                <Surface as="article" key={x.id} className="compact">
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
                        <ActionButton variant="outline"

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
                        <ActionButton variant="ghost"
                          type="button"

                          onClick={(R) => (view.renameOutfit.value = null)}
                        >
                          {"取消"}
                        </ActionButton>
                      </form>
                    ) : (
                      <b key={1}>{displayText(x.name)}</b>
                    )}
                    <Badge >
                      {displayText(view.tagLabel(x.tag) || "未分类")}
                    </Badge>
                    <span className="spacer" />
                    <MenuDisclosure
                      className="actions-menu"
                      onClick={(R) =>
                        R.target.closest("button") &&
                        R.currentTarget.removeAttribute("open")
                      }
                      title={<><MoreActionsIcon className="size-4" aria-hidden="true"/></>}
                      aria-label={x.name + "的方案操作"}
                    >
                      <div className="scroll-lock-list" data-scroll-lock="">
                        <ActionButton variant="ghost"
                          onClick={(R) =>
                            (view.preview.value =
                              view.preview.value === x.id ? null : x.id)
                          }
                        >
                          {"预览"}
                        </ActionButton>
                        <ActionButton variant="ghost" onClick={(R) => view.edit(x)}>
                          {"编辑搭配"}
                        </ActionButton>
                        <ActionButton variant="ghost"
                          onClick={(R) => {
                            ((view.renameOutfit.value = x.id),
                              (view.renameValue.value = x.name));
                          }}
                        >
                          {"重命名"}
                        </ActionButton>
                        <ActionButton variant="ghost"
                          disabled={true}
                          title="服务端保留方案及其历史；可编辑或重命名"
                        >
                          {"删除"}
                        </ActionButton>
                      </div>
                    </MenuDisclosure>
                    <ActionButton variant="outline"

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
              <ActionButton variant="outline"

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
                    className="garment-card"
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
                        title={<><MoreActionsIcon className="size-4" aria-hidden="true"/></>}
                        aria-label={x.name + "的衣服操作"}
                      >
                        <div className="scroll-lock-list" data-scroll-lock="">
                          <ActionButton variant="ghost"
                            onClick={(R) =>
                              (view.garment.value = JSON.parse(
                                JSON.stringify(x),
                              ))
                            }
                          >
                            {"编辑"}
                          </ActionButton>
                          <ActionButton variant="ghost"
                            onClick={(R) => view.removeGarment(x.id)}
                          >
                            {"归档"}
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
            title={readValue(view.character).garments.some(item=>item.id===view.garment.value.id)?"编辑衣服":"添加衣服"}
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

              <label>
                {"名称"}
                <FieldInput
                  required
                  aria-label="衣服名称"
                  value={view.garment.value.name}
                  onChange={(event) => {
                    ((x) => (view.garment.value.name = x))(event.target.value);
                  }}
                />
                <p className="text-sm text-muted-foreground">填写便于辨认的衣服名称。</p>
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
                <ActionButton variant="default"

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
                <ActionButton variant="outline"
                  type="button"

                  onClick={(x) => (view.garment.value = null)}
                >
                  {"取消"}
                </ActionButton>
              </div>
            </form>
          </DrawerFrame>
        ) : null}
        {view.confirmation.value?<ConfirmationFrame title={view.confirmation.value.title} description={view.confirmation.value.message} onClose={()=>view.confirmation.value=null}><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={()=>{view.confirmation.value.run();view.confirmation.value=null;}}>{view.confirmation.value.confirmLabel}</AlertDialogAction></ConfirmationFrame>:null}
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
