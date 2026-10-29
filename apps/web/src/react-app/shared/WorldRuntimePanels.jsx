import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRightIcon,
  BookOpenIcon,
  CalendarDaysIcon,
  PlusIcon,
  SearchIcon,
  SendIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Surface,
  Badge,
  FieldInput,
  ChoiceSelect,
  Disclosure,
} from "../controls";
import WorkspaceInfo from "./WorkspaceInfo";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import useWorkspaceRead from "./useWorkspaceRead";
import WorldField, { WorldForm, WorldReader } from "./WorldFields";
import SemanticTree from "./SemanticTree";
import JsonDetails from "./JsonDetails";
import WorkspaceGarmentArt from "./WorkspaceGarmentArt";
import { EmptyState, InlineFeedback } from "./Presentation";
import { WorldItemSuggestion } from "@/v3/proto/components/world-items.js";
import { userInstant } from "@/v3/presentation-time.mjs";
import { stateLabel, errorMessage } from "@/v3/domain-presentation.mjs";
const check = (row, m, id) => {
  if (
    !row?.id ||
    row.fluctlight_id !== m.fid ||
    row.world_id !== m.world.id ||
    (id && row.id !== id)
  )
    throw Error("记录不属于当前角色与世界。");
  return row;
};
const templateCheck = (row, m, id) => {
  if (!row?.id || row.world_id !== m.world.id || (id && row.id !== id))
    throw Error("事件模板不属于当前世界。");
  return row;
};
function actionBase(m, identity) {
  return {
    identity,
    scope: ["world", m.fid, identity],
    storageKey: `fluctlight-world-framework:${m.owner}:${m.fid}:replica:${identity}`,
  };
}
function Reason({ draft, edit }) {
  return (
    <WorldField
      label="世界事件操作说明"
      multiline
      value={draft.reason}
      onChange={(reason) => edit({ reason })}
    />
  );
}
function reasonValid(d) {
  if (!d.reason.trim() || [...d.reason.trim()].length > 1000)
    throw Error("请填写操作说明，最多1000个字符。");
}
export function WorldItemsPanel({ model: m }) {
  const { w, fid } = m;
  const [query, setQuery] = useState(""),
    [search, setSearch] = useState(""),
    [selected, setSelected] = useState(null),
    [action, setAction] = useState(null),
    [notice, setNotice] = useState("");
  const trigger = useRef(null),
    commandTrigger = useRef(null);
  const items = useWorkspaceRead({
    identity: ["world-items", m.world.id, query],
    read: async (cursor) => {
      const page = await w.client.worldItems(fid, { q: query, cursor });
      if (page.world_id !== m.world.id)
        throw Error("物品目录的当前世界已变化，请刷新世界目录。");
      return { ...page, items: page.items.map((row) => check(row, m)) };
    },
  });
  const detail = useWorkspaceRead({
    identity: ["world-item", selected],
    enabled: !!selected,
    read: async () => ({
      items: [check(await w.client.worldItem(fid, selected), m, selected)],
      next_cursor: "",
    }),
  });
  const current = detail.items[0];
  function configure(event) {
    if (!current) return;
    m.assertWritable();
    commandTrigger.current = event.currentTarget;
    const id = current.id;
    setAction({
      ...actionBase(m, "item:" + id),
      title: "配置取得范围",
      description:
        "配置可取得范围，不直接添加到角色库存。已经开始的行动继续使用原目录快照。",
      submitLabel: "保存取得范围",
      initial: {
        revision: current.revision,
        available: current.available,
        placeId: current.place_id || "",
      },
      build: (d) => ({
        expectedRevision: d.revision,
        available: d.available,
        placeId: d.placeId || null,
        reason: "所有者配置世界物品的取得范围",
      }),
      execute: async (body, key) => {
        m.assertWritable();
        const row = check(
          await w.client.configureWorldItem(fid, id, {
            ...body,
            idempotencyKey: key,
          }),
          m,
          id,
        );
        if (!Number.isSafeInteger(row.revision))
          throw Error("物品配置回执尚未完整确认。");
        return row;
      },
      readLatest: async () => check(await w.client.worldItem(fid, id), m, id),
      adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
      onSaved: async (result) => {
        const row = check(await w.client.worldItem(fid, id), m, id);
        if (row.revision < result.revision)
          throw Error("物品取得范围的新版本尚未读回。");
        if (m.live()) {
          detail.adopt({ items: [row], next_cursor: "" });
          items.adopt({
            items: items.items.map((old) => (old.id === row.id ? row : old)),
            next_cursor: items.cursor,
            ...items.extra,
          });
          await items.load();
          setNotice("取得范围已读回，实际拥有与穿着仍以库存记录为准。");
        }
      },
      fields: (d, edit) => (
        <WorldForm>
          <div className="r-switch-row">
            <div>
              <h3>允许提出取得请求</h3>
              <p>角色是否取得、何时行动，以实际处理为准。</p>
            </div>
            <Switch
              aria-label="允许提出取得请求"
              checked={d.available}
              onCheckedChange={(available) => edit({ available })}
            />
          </div>
          <label className="replica-field">
            <span>取得地点</span>
            <ChoiceSelect
              aria-label="取得地点"
              value={d.placeId}
              onChange={(e) => edit({ placeId: e.target.value })}
            >
              <option value="">不限地点</option>
              {m.snapshot.places
                .filter((p) => p.status === "active")
                .map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.alias || p.name}
                  </option>
                ))}
            </ChoiceSelect>
          </label>
        </WorldForm>
      ),
    });
  }
  function suggest() {
    if (!current || !current.available || !m.live()) return;
    try {
      const key = "fluctlight:approved-chat-draft:" + w.currentScope(),
        old = sessionStorage.getItem(key) || "",
        suggestion = WorldItemSuggestion(current);
      sessionStorage.setItem(
        key,
        old.trim() ? old + "\n\n" + suggestion : suggestion,
      );
      w.navigatePath("/c/" + encodeURIComponent(fid) + "/chat");
    } catch (cause) {
      setNotice("聊天草稿未能保存：" + errorMessage(cause));
    }
  }
  return (
    <>
      <form
        className="r-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          setQuery(search.trim());
        }}
      >
        <div className="r-search">
          <FieldInput
            type="search"
            aria-label="搜索目录物品"
            placeholder="搜索目录物品"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Button
            variant="ghost"
            size="icon"
            type="submit"
            aria-label="应用目录搜索"
          >
            <SearchIcon size={14} />
          </Button>
        </div>
        <Button
          variant="outline"
          type="button"
          onClick={() => w.navigatePath("/studio?kind=item&new=1")}
        >
          <PlusIcon size={14} />
          新建物品定义
        </Button>
        <Button
          variant="ghost"
          type="button"
          disabled={items.busy}
          onClick={items.load}
        >
          刷新物品目录
        </Button>
      </form>
      <WorkspaceInfo>
        这里是世界中的可取得物品。实际拥有的服装，保存在角色衣柜。
      </WorkspaceInfo>
      {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
      <WorldReader source={items} label="物品目录" />
      <div className="r-item-grid">
        {items.items.map((item) => (
          <Surface key={item.id} className="r-item-card">
            <div className="r-item-sketch" aria-hidden="true">
              {item.definition.item_kind === "wearable" ? (
                <WorkspaceGarmentArt
                  type={item.definition.slot || "outer"}
                  color="#c2bd9f"
                />
              ) : (
                <BookOpenIcon size={54} strokeWidth={1} />
              )}
            </div>
            <div className="r-card-body">
              <span className="r-tiny-label">
                {item.definition.item_kind === "wearable" ? "服饰" : "普通物品"}{" "}
                · {item.definition.category}
                {item.definition.slot ? " / " + item.definition.slot : ""}
              </span>
              <h3>{item.name}</h3>
              <p>{item.definition.description}</p>
              <div className="r-card-footer">
                <Badge variant="outline">
                  {item.available ? "可提出取得请求" : "暂不可取得"}
                </Badge>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={(e) => {
                    trigger.current = e.currentTarget;
                    setSelected(item.id);
                  }}
                >
                  详情
                  <ArrowUpRightIcon size={13} />
                </Button>
              </div>
            </div>
          </Surface>
        ))}
      </div>
      {items.ready && !items.items.length && !items.error && (
        <EmptyState>
          <h3>{query ? "没有找到匹配的目录物品" : "世界物品目录暂时留白"}</h3>
          <p>发布并应用物品定义后，可以在这里配置真实取得范围。</p>
        </EmptyState>
      )}
      {items.cursor && (
        <Button variant="outline" disabled={items.busy} onClick={items.more}>
          加载更多目录物品
        </Button>
      )}
      <WorkspaceDialog
        className="replica-world"
        open={!!selected && !action}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
        title={current?.name || "物品详情"}
        description={
          current?.definition.description || "读取目录来源与取得范围。"
        }
        returnFocus={() => trigger.current}
        footer={
          current && (
            <>
              <Button
                variant="outline"
                disabled={m.readOnly}
                onClick={configure}
              >
                配置取得范围
              </Button>
              <Button
                disabled={!current.available || m.readOnly}
                onClick={suggest}
              >
                <SendIcon size={14} />
                向角色提出请求
              </Button>
            </>
          )
        }
      >
        <div className="r-form-stack">
          <WorldReader source={detail} label="物品详情" />
          {current && (
            <>
              <p>
                来自{" "}
                <a
                  href={
                    "#/studio/edit/" +
                    encodeURIComponent(current.source_resource_id)
                  }
                >
                  物品定义
                </a>{" "}
                · 精确发布版本 {current.source_version}
              </p>
              <p className="r-prose">
                {current.available ? "允许提出取得请求" : "当前不允许取得请求"}{" "}
                ·{" "}
                {m.snapshot.places.find((p) => p.id === current.place_id)
                  ?.name || (current.place_id ? "指定地点待读取" : "不限地点")}
              </p>
              <WorkspaceInfo>
                提出请求会写入聊天草稿，确认发送后才交给角色处理。目录记录不证明已经拥有、穿上或使用。
              </WorkspaceInfo>
              <JsonDetails
                value={current}
                title="完整物品定义、版本与本地差异"
              />
            </>
          )}
        </div>
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => commandTrigger.current}
        />
      )}
    </>
  );
}
export function WorldEventsPanel({ model: m }) {
  const { w, fid } = m;
  const [selected, setSelected] = useState(null),
    [eventId, setEventId] = useState(null),
    [action, setAction] = useState(null),
    [notice, setNotice] = useState("");
  const trigger = useRef(null),
    commandTrigger = useRef(null),
    latest = useRef(null);
  const templates = useWorkspaceRead({
    identity: ["world-templates", m.world.id],
    read: async () => {
      const page = await w.client.worldTemplates(fid);
      if (page.world_id !== m.world.id)
        throw Error("事件列表的当前世界已变化，请刷新世界目录。");
      return { ...page, items: page.items.map((row) => templateCheck(row, m)) };
    },
  });
  const detail = useWorkspaceRead({
    identity: ["world-template", selected],
    enabled: !!selected,
    read: async () => ({
      items: [
        templateCheck(
          await w.client.worldEventTemplate(fid, selected),
          m,
          selected,
        ),
      ],
      next_cursor: "",
    }),
  });
  const runs = useWorkspaceRead({
    identity: ["world-template-runs", selected],
    enabled: !!selected,
    read: async (cursor) => {
      const page = await w.client.worldEventRuns(fid, selected, cursor);
      if (page.items.some((row) => row.template_id !== selected))
        throw Error("事件执行记录范围不匹配。");
      return { ...page, items: page.items.map((row) => check(row, m)) };
    },
  });
  const events = useWorkspaceRead({
    identity: ["world-life-events", m.world.id],
    read: async (cursor) => {
      const page = await w.client.worldLifeEvents(fid, cursor);
      if (
        page.world_id !== m.world.id ||
        page.items.some(
          (row) =>
            row.fluctlight_id !== fid ||
            (row.world_id && row.world_id !== m.world.id),
        )
      )
        throw Error("世界生活事件范围不匹配。");
      return page;
    },
  });
  const event = useWorkspaceRead({
    identity: ["world-life-event", eventId],
    enabled: !!eventId,
    read: async () => {
      const row = await w.client.worldLifeEvent(fid, eventId);
      if (
        row.id !== eventId ||
        row.fluctlight_id !== fid ||
        (row.world_id && row.world_id !== m.world.id)
      )
        throw Error("生活事件范围不匹配。");
      return { items: [row], next_cursor: "" };
    },
  });
  const current = detail.items[0],
    hasQueued = runs.items.some((row) => row.status === "queued");
  latest.current = { runs, detail, events };
  useEffect(() => {
    if (!selected || !hasQueued) return;
    let reading = false;
    const timer = setInterval(async () => {
      if (document.hidden || reading || !m.live()) return;
      reading = true;
      try {
        await Promise.all([
          latest.current.runs.load(),
          latest.current.detail.load(),
          latest.current.events.load(),
        ]);
      } finally {
        reading = false;
      }
    }, 3000);
    return () => clearInterval(timer);
  }, [selected, hasQueued, m.fid, m.world.id]);
  function command(row, operation, event, run) {
    m.assertWritable();
    commandTrigger.current = event.currentTarget;
    const id = row.id,
      enabled = !row.enabled,
      label =
        operation === "toggle"
          ? enabled
            ? "启用事件"
            : "暂停事件"
          : operation === "trigger"
            ? "请求触发一次"
            : "恢复这次失败执行";
    setAction({
      ...actionBase(
        m,
        "event:" + id + ":" + operation + (run ? ":" + run.id : ""),
      ),
      title: label,
      description:
        operation === "trigger"
          ? "提交真实事件执行。受理后可能排队、应用、过滤或失败，结果以实际执行记录为准。"
          : "核对当前模板与原执行版本后提交。",
      submitLabel: label,
      initial: {
        revision: row.revision,
        runRevision: run?.revision,
        reason: "",
      },
      validate: reasonValid,
      build: (d) => ({
        expectedRevision: d.revision,
        reason: d.reason.trim(),
        ...(operation === "toggle" ? { enabled } : {}),
        ...(operation === "retry"
          ? { expectedRunRevision: d.runRevision }
          : {}),
      }),
      execute: async (body, key) => {
        m.assertWritable();
        const value = { ...body, idempotencyKey: key };
        const r = await (operation === "toggle"
          ? w.client.setWorldTemplate(fid, id, value)
          : operation === "trigger"
            ? w.client.triggerWorldEvent(fid, id, value)
            : w.client.retryWorldEvent(fid, id, run.id, value));
        if ((!r?.id && !r?.template_id) || !Number.isSafeInteger(r.revision))
          throw Error("事件操作回执尚未完整确认。");
        if (operation === "retry") {
          // Retry returns a command receipt, not the full execution projection.
          // The original run is scoped by the request; onSaved verifies its GET.
          if (
            r.id !== run.id ||
            r.status !== "queued" ||
            r.revision !== body.expectedRunRevision + 1
          )
            throw Error("原事件执行回执范围不匹配。");
        } else if (operation === "trigger") {
          check(r, m);
          if (r.template_id !== id)
            throw Error("原事件执行回执范围不匹配。");
        }
        return r;
      },
      readLatest: async () => ({
        template: templateCheck(
          await w.client.worldEventTemplate(fid, id),
          m,
          id,
        ),
        ...(run
          ? {
              run: check(
                await w.client.worldEventRun(fid, id, run.id),
                m,
                run.id,
              ),
            }
          : {}),
      }),
      adoptLatest: (d, value) => {
        if (
          run &&
          (value.template.revision !== value.run.template_revision ||
            value.run.status !== "failed")
        )
          throw Error("原执行或模板状态已变化，请返回核对实际记录。");
        return {
          ...d,
          revision: value.template.revision,
          runRevision: value.run?.revision,
        };
      },
      onSaved: async (result, executed) => {
        const fresh = templateCheck(
          await w.client.worldEventTemplate(fid, id),
          m,
          id,
        );
        if (operation === "toggle") {
          if (
            fresh.revision < result.revision ||
            (fresh.revision === result.revision &&
              fresh.enabled !== executed.enabled)
          )
            throw Error("事件启用状态尚未读回。");
        } else {
          const actual = check(
            await w.client.worldEventRun(fid, id, result.id),
            m,
            result.id,
          );
          if (actual.template_id !== id || actual.revision < result.revision)
            throw Error("事件执行版本尚未读回。");
          if (m.live() && selected === id)
            runs.adopt({
              items: [
                actual,
                ...runs.items.filter((old) => old.id !== actual.id),
              ],
              next_cursor: runs.cursor,
            });
        }
        if (m.live()) {
          if (selected === id)
            detail.adopt({ items: [fresh], next_cursor: "" });
          templates.adopt({
            items: templates.items.map((old) => (old.id === id ? fresh : old)),
            next_cursor: "",
            ...templates.extra,
          });
          await templates.load();
          if (selected === id) await runs.load();
          await events.load();
          setNotice(
            operation === "toggle"
              ? "事件启用状态已读回。"
              : "事件执行请求已读回，请核对实际状态与生活记录。",
          );
        }
      },
      fields: (d, edit) => (
        <WorldForm>
          <Reason draft={d} edit={edit} />
        </WorldForm>
      ),
    });
  }
  const runLabels = {
    queued: "等待执行",
    applied: "已应用",
    filtered: "条件过滤",
    failed: "执行失败",
  };
  return (
    <>
      <Surface className="r-padded">
        <div className="section-heading">
          <div>
            <h2>让世界发生一点变化</h2>
            <p>启用事件后，由实际执行判断是否产生生活变化。</p>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => w.navigatePath("/studio?kind=event_template&new=1")}
          >
            <PlusIcon size={14} />
            创建事件底稿
          </Button>
        </div>
        {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
        <Button
          variant="ghost"
          disabled={templates.busy}
          onClick={() =>
            Promise.all([
              templates.load(),
              ...(selected ? [detail.load(), runs.load()] : []),
            ])
          }
        >
          刷新事件模板
        </Button>
        <WorldReader source={templates} label="事件模板" />
        {templates.items.map((row) => (
          <div className="r-event-row" key={row.id}>
            <span className="r-date-icon">
              <CalendarDaysIcon size={20} />
            </span>
            <div>
              <h3>{row.name}</h3>
              <p>
                {typeof row.definition.description === "string"
                  ? row.definition.description
                  : ""}
              </p>
              <small>
                {row.last_triggered_at
                  ? "最近触发 " + userInstant(row.last_triggered_at)
                  : "尚未读到触发时间"}{" "}
                · 模板 v{row.revision}
              </small>
            </div>
            <Switch
              aria-label={"启用" + row.name}
              disabled={m.readOnly}
              checked={row.enabled}
              onCheckedChange={() =>
                command(row, "toggle", {
                  currentTarget: document.activeElement,
                })
              }
            />
            <Button
              variant="outline"
              size="sm"
              onClick={(e) => {
                trigger.current = e.currentTarget;
                setSelected(row.id);
              }}
            >
              详情
            </Button>
          </div>
        ))}
        {templates.ready && !templates.items.length && !templates.error && (
          <EmptyState>
            <h3>这个世界还没有事件模板</h3>
          </EmptyState>
        )}
      </Surface>
      <Surface className="r-padded r-world-life-events">
        <div className="section-heading">
          <h2>实际生活事件</h2>
        </div>
        <WorldReader source={events} label="生活事件" />
        {events.items.map((row) => (
          <Button
            variant="ghost"
            className="r-event-record"
            key={row.id}
            onClick={(e) => {
              trigger.current = e.currentTarget;
              setEventId(row.id);
            }}
          >
            <span>
              <b>{row.title}</b>
              <small>
                {userInstant(row.start_at)} ·{" "}
                {row.world_id ? "当前世界" : "历史归属未记录"}
              </small>
            </span>
            <Badge variant="outline">{stateLabel(row.status)}</Badge>
          </Button>
        ))}
        {events.ready && !events.items.length && !events.error && (
          <p className="r-muted">尚未读到实际生活事件。</p>
        )}
        {events.cursor && (
          <Button
            variant="outline"
            disabled={events.busy}
            onClick={events.more}
          >
            加载更多生活事件
          </Button>
        )}
      </Surface>
      <WorkspaceDialog
        className="replica-world"
        wide
        open={!!selected && !action}
        title={current?.name || "世界事件"}
        description="查看事件定义、来源和实际执行记录。"
        returnFocus={() => trigger.current}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
        footer={
          current && (
            <Button
              disabled={m.readOnly || !current.enabled}
              onClick={(e) => command(current, "trigger", e)}
            >
              请求触发一次
            </Button>
          )
        }
      >
        <div className="r-form-stack">
          <WorldReader source={detail} label="事件详情" />
          {current && (
            <>
              <p className="r-muted">
                {current.enabled ? "已启用" : "已暂停"} · 模板 v
                {current.revision}
              </p>
              <SemanticTree value={current.definition} />
              <p>
                来自{" "}
                <a
                  href={
                    "#/studio/edit/" +
                    encodeURIComponent(current.source_resource_id)
                  }
                >
                  事件底稿
                </a>{" "}
                · {current.source_version}
              </p>
              <JsonDetails value={current} title="完整模板与运行状态" />
            </>
          )}
          <div className="r-inline-actions">
            <h3>实际执行记录</h3>
            <Button
              variant="ghost"
              disabled={runs.busy || detail.busy}
              onClick={() =>
                Promise.all([detail.load(), runs.load(), events.load()])
              }
            >
              刷新事件与执行
            </Button>
          </div>
          <WorldReader source={runs} label="事件执行" />
          <div className="r-timeline">
            {runs.items.map((row) => (
              <div key={row.id}>
                <i />
                <span>
                  <b>{runLabels[row.status] || row.status}</b>
                  <small>
                    {userInstant(row.due_at)} · 执行 v{row.revision}
                  </small>
                  {row.status === "queued" && <p>请求已受理，等待实际执行。</p>}
                  {row.status === "filtered" && (
                    <p>这次执行被条件过滤，没有据此确认生活变化。</p>
                  )}
                  {row.life_event_id && (
                    <Button
                      variant="link"
                      onClick={() => {
                        setSelected(null);
                        setEventId(row.life_event_id);
                      }}
                    >
                      查看实际生活事件
                    </Button>
                  )}
                  <JsonDetails value={row} title="执行结果、窗口与恢复记录" />
                  {row.status === "failed" && current && (
                    <Button
                      variant="outline"
                      disabled={
                        m.readOnly ||
                        !current.enabled ||
                        current.revision !== row.template_revision ||
                        !Number.isFinite(Date.parse(row.expires_at)) ||
                        Date.parse(row.expires_at) <= Date.now()
                      }
                      onClick={(e) => command(current, "retry", e, row)}
                    >
                      恢复这次失败执行
                    </Button>
                  )}
                </span>
              </div>
            ))}
          </div>
          {runs.cursor && (
            <Button variant="outline" disabled={runs.busy} onClick={runs.more}>
              加载更多执行记录
            </Button>
          )}
        </div>
      </WorkspaceDialog>
      <WorkspaceDialog
        className="replica-world"
        wide
        open={!!eventId}
        title={event.items[0]?.title || "实际生活事件"}
        description="查看已记录的结果、证据和直接关联。"
        returnFocus={() => trigger.current}
        onOpenChange={(v) => {
          if (!v) setEventId(null);
        }}
      >
        <WorldReader source={event} label="生活事件详情" />
        {event.items[0] && (
          <div className="r-form-stack">
            <p className="r-muted">
              {stateLabel(event.items[0].status)} ·{" "}
              {userInstant(event.items[0].start_at)}
              {event.items[0].end_at
                ? " 至 " + userInstant(event.items[0].end_at)
                : ""}
            </p>
            {event.items[0].result?.effects && (
              <SemanticTree value={event.items[0].result.effects} />
            )}
            <WorkspaceInfo>关联依据以已保存的直接记录为准。</WorkspaceInfo>
            <JsonDetails
              value={event.items[0]}
              title="完整结果、证据与影响记录"
            />
          </div>
        )}
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => commandTrigger.current}
        />
      )}
    </>
  );
}
