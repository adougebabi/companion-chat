import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import {
  Badge,
  ChoiceSelect,
  Disclosure,
  FieldInput,
  Textarea,
} from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import {Feedback} from './Presentation';
import useWorkspaceRead from "./useWorkspaceRead";
import { PeopleFact, PeopleReader } from "./PeopleParts";
import SemanticTree from "./SemanticTree";
import JsonDetails from "./JsonDetails";
import DateTimeField from "./DateTimeField";
import PersonRecords from "./PersonRecords";
import { afterPaint } from "../page";
import {
  npcDraft,
  npcWrite,
  acknowledgeNpcDraft,
  rebaseNpcDraft,
  encounterWrite,
} from "@/approved-ui/people-commands.mjs";
import {
  relationshipFor,
  npcLocation,
} from "@/approved-ui/people-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import { errorMessage } from "@/v3/domain-presentation.mjs";
function field(label, child) {
  return (
    <label key={label} className="replica-field">
      <span>{label}</span>
      {child}
    </label>
  );
}
export default function PersonDetail({
  model: m,
  npcId,
  onClose,
  initialEdit = false,
}) {
  const { w, fid, owner } = m,
    places = m.catalog.extra.places || [],
    [action, setAction] = useState(null),
    [manage, setManage] = useState(false),
    [error, setError] = useState("");
  const editTrigger = useRef(null),
    managementEditTrigger = useRef(null),
    encounterTrigger = useRef(null),
    removeTrigger = useRef(null),
    requestedEdit = useRef(false),
    detailRef = useRef(null),
    deleted = useRef(false);
  const check = (row) => {
    if (
      !row?.id ||
      row.id !== npcId ||
      row.world_id !== m.catalog.extra.world?.id
    )
      throw Error("人物与当前世界范围不匹配。");
    return row;
  };
  const read = async () => check(await w.client.runtimeNPC(fid, npcId));
  const detail = useWorkspaceRead({
    identity: ["person-detail", npcId, m.catalog.extra.world?.id],
    enabled: !!m.catalog.extra.world,
    read: async () => ({ items: [await read()], next_cursor: "" }),
  });
  detailRef.current = detail;
  const npc = detail.items[0],
    profile = m.profile.items[0],
    relation = relationshipFor(
      npc?.relationship_context?.relationships || profile?.relationships,
      npc?.actor_id,
      profile?.personality_runtime?.active_profile_id,
      fid,
    ),
    location = npc ? npcLocation(npc, places) : null;
  function base(identity) {
    return {
      identity,
      scope: ["people", fid, npcId, identity],
      storageKey: `fluctlight-people:${owner}:${fid}:npc:${npcId}:${identity}`,
    };
  }
  function edit() {
    if (!npc) return;
    m.assertWritable();
    setAction({
      ...base("edit"),
      returnFocus: () =>
        manage ? managementEditTrigger.current : editTrigger.current,
      title: "编辑世界人物",
      description: "保存当前世界的局部人物资料，关系与实际接触分别保留。",
      submitLabel: "保存人物",
      initial: npcDraft(npc),
      validate: npcWrite,
      build: npcWrite,
      afterAcknowledged: acknowledgeNpcDraft,
      execute: async (body, key) => {
        m.assertWritable();
        const r = await w.client.updateRuntimeNPC(fid, npcId, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== npcId ||
          !Number.isSafeInteger(r.revision) ||
          r.actor_id !== npc.actor_id
        )
          throw Error("人物保存回执尚未完整确认。");
        return r;
      },
      readLatest: read,
      adoptLatest: rebaseNpcDraft,
      onSaved: async (r) => {
        const fresh = await read();
        if (fresh.revision < r.revision) throw Error("人物的新版本尚未读回。");
        if (m.live()) {
          detailRef.current.adopt({ items: [fresh], next_cursor: "" });
          await m.adoptNpc(fresh);
        }
      },
      fields: (d, change) => (
        <NpcFields draft={d} edit={change} places={places} />
      ),
    });
  }
  useEffect(() => {
    if (initialEdit && npc && !requestedEdit.current && !m.readOnly) {
      requestedEdit.current = true;
      edit();
    }
  }, [npc?.id, initialEdit]);
  async function record() {
    if (!npc) return;
    m.assertWritable();
    setError("");
    try {
      const authority = await m.readProfile();
      if (!m.live()) return;
      if (!authority.context?.context_revision)
        throw Error("生活上下文版本尚未完整读到。");
      setAction({
        ...base("encounter"),
        returnFocus: () => encounterTrigger.current,
        title: "记录已实际发生的接触",
        description:
          "仅记录你确认在角色生活中已经发生的接触，保存会产生事件、关系与记忆。",
        wide: true,
        submitLabel: "确认记录实际接触",
        initial: {
          placeId: "",
          summary: "",
          startAt: "",
          endAt: "",
          zone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
          exact: false,
          evidence: "owner_confirmation",
          confirmed: false,
          lifeRevision: authority.context.context_revision,
        },
        validate: encounterWrite,
        build: encounterWrite,
        execute: async (body, key) => {
          m.assertWritable();
          const r = await w.client.recordNPCEncounter(fid, npcId, {
            ...body,
            idempotencyKey: key,
          });
          if (
            r.status !== "recorded" ||
            !r.event_id ||
            !r.relationship_id ||
            !r.memory_id ||
            r.npc_id !== npcId ||
            !r.id
          )
            throw Error("实际接触回执尚未完整确认。");
          return r;
        },
        readLatest: m.readProfile,
        adoptLatest: (d, row) => ({
          ...d,
          lifeRevision: row.context?.context_revision,
        }),
        onSaved: async (r) => {
          const [fresh, authority, event, memory] = await Promise.all([
            read(),
            m.readProfile(),
            w.client.worldLifeEvent(fid, r.event_id),
            w.client.managedMemory(r.memory_id),
          ]);
          const related = [
            ...(fresh.relationship_context?.relationships || []),
            ...(authority.relationships || []),
          ].some(
            (row) =>
              row.id === r.relationship_id &&
              row.target_actor_id === fresh.actor_id,
          );
          if (
            event.id !== r.event_id ||
            event.fluctlight_id !== fid ||
            event.result?.npc_id !== npcId ||
            memory.id !== r.memory_id ||
            memory.owner_fluctlight_id !== fid ||
            !memory.event_refs?.includes(r.event_id) ||
            !related
          )
            throw Error("实际接触的事件、关系或记忆来源尚未完整读回。");
          if (m.live()) {
            detailRef.current.adopt({ items: [fresh], next_cursor: "" });
            m.profile.adopt({ items: [authority], next_cursor: "" });
            await m.adoptNpc(fresh);
          }
        },
        fields: (d, change) => (
          <div className="replica-profile replica-people companion-form-stack lp-wide">
            <p className="companion-caption">
              时间按 {d.zone}{" "}
              填写。本次实际确认会记录为来源，已有记录引用可在证据字段补充。
            </p>
            {field(
              "接触地点",
              <ChoiceSelect
                aria-label="接触地点"
                value={d.placeId}
                onChange={(e) => change({ placeId: e.target.value })}
              >
                <option value="">选择实际地点</option>
                {places
                  .filter((p) => p.status === "active")
                  .map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.alias || p.name}
                    </option>
                  ))}
              </ChoiceSelect>,
            )}
            <div className="companion-edit-grid">
              {["startAt", "endAt"].map((key) =>
                field(
                  key === "startAt" ? "开始时间" : "结束时间",
                  d.exact ? (
                    <FieldInput
                      aria-label={key === "startAt" ? "开始时间" : "结束时间"}
                      value={d[key]}
                      placeholder="2026-10-28T12:00:00+08:00"
                      onChange={(e) => change({ [key]: e.target.value })}
                    />
                  ) : (
                    <DateTimeField
                      aria-label={key === "startAt" ? "开始时间" : "结束时间"}
                      value={d[key]}
                      onChange={(e) => change({ [key]: e.target.value })}
                    />
                  ),
                ),
              )}
            </div>
            <label className="companion-checkbox-row">
              <Checkbox
                checked={d.exact}
                onCheckedChange={(value) =>
                  change({ exact: !!value, startAt: "", endAt: "" })
                }
              />
              使用含时区偏移的时间
            </label>
            {field(
              "实际经过",
              <Textarea
                aria-label="实际经过"
                value={d.summary}
                onChange={(e) => change({ summary: e.target.value })}
              />,
            )}
            {field(
              "证据引用（每行一条）",
              <Textarea
                aria-label="证据引用"
                value={d.evidence}
                onChange={(e) => change({ evidence: e.target.value })}
              />,
            )}
            <label className="companion-checkbox-row">
              <Checkbox
                aria-label="确认接触实际发生"
                checked={d.confirmed}
                onCheckedChange={(value) => change({ confirmed: !!value })}
              />
              我确认这次接触在角色生活中实际发生，并同意记录到生活历史
            </label>
          </div>
        ),
      });
    } catch (cause) {
      if (m.live()) setError(errorMessage(cause));
    }
  }
  function remove() {
    if (!npc) return;
    m.assertWritable();
    setAction({
      ...base("remove"),
      returnFocus: () => removeTrigger.current,
      title: "移除「" + npc.name + "」？",
      description:
        "仅移除没有历史引用的人物。存在关系、记忆或接触等记录时，请使用归档并保留历史。",
      submitLabel: "确认移除人物",
      initial: { revision: npc.revision, reason: "" },
      preAdmissionConflicts: ["revision_conflict", "resource_referenced"],
      validate: (d) => {
        if (!d.reason.trim() || [...d.reason.trim()].length > 1000)
          throw Error("请填写移除说明，最多1000个字符。");
      },
      build: (d) => ({ expectedRevision: d.revision, reason: d.reason.trim() }),
      execute: async (body, key) => {
        m.assertWritable();
        const r = await w.client.deleteRuntimeNPC(fid, npcId, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.id !== npcId ||
          r.status !== "deleted" ||
          (r.actor_id && r.actor_id !== npc.actor_id)
        )
          throw Error("人物移除回执未完整确认。");
        return r;
      },
      readLatest: async () => {
        const view = await w.client.catalogWorld(fid, npc.world_id),
          row = view.npcs?.find((p) => p.id === npcId);
        if (view.world?.fluctlight_id !== fid || !row)
          throw Error("原人物未读到，请核对原操作结果。");
        return row;
      },
      adoptLatest: (d, row) => ({ ...d, revision: row.revision }),
      onSaved: async () => {
        const view = await w.client.catalogWorld(fid, npc.world_id);
        if (
          view.world?.fluctlight_id !== fid ||
          view.npcs.some((p) => p.id === npcId)
        )
          throw Error("人物移除结果尚未读回。");
        if (m.live()) {
          const page = await m.readCatalog();
          if (m.live()) {
            m.catalog.adopt(page);
            deleted.current = true;
          }
        }
      },
      fields: (d, edit) =>
        field(
          "移除说明",
          <Textarea
            aria-label="人物移除说明"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
          />,
        ),
    });
  }
  return (
    <>
      <WorkspaceDialog
        className="replica-profile replica-people replica-person-detail"
        open={!action && !manage}
        title={npc?.name || "人物详情"}
        description={npc?.role || "读取实际人物、来源与接触记录。"}
        onOpenChange={(v) => {
          if (!v) onClose();
        }}
        returnFocus={() => m.trigger.current}
        footer={
          npc && (
            <>
              <Button
                ref={editTrigger}
                variant="outline"
                disabled={m.readOnly}
                onClick={edit}
              >
                编辑人物资料
              </Button>
              <Button
                variant="outline"
                disabled={m.readOnly}
                onClick={() => setManage(true)}
              >
                人物管理
              </Button>
              <Button
                ref={encounterTrigger}
                variant="outline"
                disabled={m.readOnly || npc.status !== "active" || !npc.enabled}
                onClick={record}
              >
                记录实际接触
              </Button>
              <Button
                onClick={() =>
                  w.navigatePath("/c/" + encodeURIComponent(fid) + "/worlds")
                }
              >
                查看所在世界
              </Button>
            </>
          )
        }
      >
        <div className="companion-form-stack">
          <PeopleReader source={detail} label="人物详情" />
          {error && <Feedback kind="error" title="接触资料暂未读全" actions={<Button variant="outline" disabled={m.readOnly} onClick={()=>void record()}>重新读取接触资料</Button>} details={<pre>{error}</pre>}>人物详情仍可查看，当前没有提交接触记录。</Feedback>}
          {npc && (
            <>
              <Badge variant="outline">
                {relation ? "已经结识" : "尚未读到当前人格的结识记录"}
              </Badge>
              <p className="companion-body-text">
                {npc.bio || "背景尚未提供。"}
              </p>
              <PeopleFact label={location.label}>{location.name}</PeopleFact>
              {npc.confirmed_at && (
                <PeopleFact label="上次实际接触">
                  {userInstant(npc.confirmed_at)} ·{" "}
                  {places.find((p) => p.id === npc.confirmed_place_id)?.name ||
                    "历史地点"}
                </PeopleFact>
              )}
              <PeopleFact label="目录来源">
                <a
                  href={
                    "#/studio/edit/" +
                    encodeURIComponent(npc.source_resource_id)
                  }
                >
                  人物定义
                </a>{" "}
                · {npc.source_version}
              </PeopleFact>
              {relation && <SemanticTree value={relation} />}
              <WorkspaceInfo>
                目录、预计位置、在场与实际结识分别记录，浏览不会建立关系。
              </WorkspaceInfo>
              <PersonRecords
                key={npc.id + ":" + npc.revision}
                model={m}
                npc={npc}
                onUpdated={async () => {
                  const [fresh, authority] = await Promise.all([
                    read(),
                    m.readProfile(),
                  ]);
                  if (m.live()) {
                    detailRef.current.adopt({
                      items: [fresh],
                      next_cursor: "",
                    });
                    m.profile.adopt({ items: [authority], next_cursor: "" });
                    await m.adoptNpc(fresh);
                  }
                }}
              />
              <JsonDetails value={npc} title="完整人物、生活规律与来源" />
            </>
          )}
        </div>
      </WorkspaceDialog>
      <WorkspaceDialog
        className="replica-profile replica-people"
        open={manage && !action}
        title="人物管理与治理"
        description={npc?.name || "当前世界人物"}
        onOpenChange={setManage}
        returnFocus={() => m.trigger.current}
      >
        <div className="companion-form-stack">
          <WorkspaceInfo>
            归档会保留已有关系、记忆与历史。移除须通过后端的完整引用检查。
          </WorkspaceInfo>
          <Button
            ref={managementEditTrigger}
            variant="outline"
            disabled={m.readOnly}
            onClick={edit}
          >
            编辑出现、位置规律与归档
          </Button>
          <Button
            ref={removeTrigger}
            variant="destructive"
            disabled={m.readOnly}
            onClick={remove}
          >
            移除无引用人物
          </Button>
        </div>
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          key={action.identity}
          action={action}
          onClose={() => {
            setAction(null);
            if (deleted.current) {
              deleted.current = false;
              onClose();
              return;
            }
            void afterPaint(() => {
              const source = action.returnFocus?.();
              if (source?.isConnected) source.focus({ preventScroll: true });
            });
          }}
          returnFocus={action.returnFocus}
        />
      )}
    </>
  );
}
function NpcFields({ draft: d, edit, places }) {
  const input = (label, key, long = false) =>
    field(
      label,
      long ? (
        <Textarea
          aria-label={label}
          value={d[key]}
          onChange={(e) => edit({ [key]: e.target.value })}
        />
      ) : (
        <FieldInput
          aria-label={label}
          value={d[key]}
          onChange={(e) => edit({ [key]: e.target.value })}
        />
      ),
    );
  const locationOptions = (
    <>
      <option value="">未设定</option>
      {places
        .filter((p) => p.status === "active" || p.id === d.homePlaceId)
        .map((p) => (
          <option key={p.id} value={p.id}>
            {p.alias || p.name}
            {p.status === "archived" ? "（已归档）" : ""}
          </option>
        ))}
    </>
  );
  return (
    <div className="replica-profile replica-people companion-form-stack lp-wide">
      {input("名字", "name")}
      {input("职业 / 身份", "role")}
      {field(
        "常在的地点",
        <ChoiceSelect
          aria-label="常在的地点"
          value={d.homePlaceId}
          onChange={(e) => edit({ homePlaceId: e.target.value })}
        >
          {locationOptions}
        </ChoiceSelect>,
      )}
      {input("背景与性格", "background", true)}
      <Disclosure title="生活规律、出现与归档">
        <div className="companion-edit-grid">
          {field(
            "位置模式",
            <ChoiceSelect
              aria-label="位置模式"
              value={d.positionMode}
              onChange={(e) => edit({ positionMode: e.target.value })}
            >
              <option value="unknown">未知</option>
              <option value="routine">按生活规律预计</option>
              <option value="fixed">固定地点预计</option>
            </ChoiceSelect>,
          )}
          {field(
            "人物状态",
            <ChoiceSelect
              aria-label="人物状态"
              value={d.status}
              onChange={(e) => edit({ status: e.target.value })}
            >
              <option value="active">启用</option>
              <option value="archived">归档并保留历史</option>
            </ChoiceSelect>,
          )}
        </div>
        <label className="companion-checkbox-row">
          <Switch
            aria-label="在世界里出现"
            checked={d.enabled}
            onCheckedChange={(enabled) => edit({ enabled })}
          />
          在世界里出现
        </label>
        <p className="companion-caption">
          规律用于预计位置，实际在场和接触以运行记录为准。
        </p>
        {d.routine.map((slot, index) => (
          <div className="people-routine-row" key={index}>
            <ChoiceSelect
              aria-label={"规律" + (index + 1) + "适用日"}
              value={slot.day}
              onChange={(e) =>
                edit({
                  routine: d.routine.map((s, i) =>
                    i === index ? { ...s, day: e.target.value } : s,
                  ),
                })
              }
            >
              <option value="daily">每天</option>
              <option value="weekday">工作日</option>
              <option value="weekend">周末</option>
            </ChoiceSelect>
            {["start", "end", "activity"].map((key) => (
              <FieldInput
                key={key}
                aria-label={
                  "规律" +
                  (index + 1) +
                  { start: "开始", end: "结束", activity: "活动" }[key]
                }
                placeholder={key === "activity" ? "活动" : "HH:mm"}
                value={slot[key]}
                onChange={(e) =>
                  edit({
                    routine: d.routine.map((s, i) =>
                      i === index ? { ...s, [key]: e.target.value } : s,
                    ),
                  })
                }
              />
            ))}
            <ChoiceSelect
              aria-label={"规律" + (index + 1) + "地点"}
              value={slot.place_id}
              onChange={(e) =>
                edit({
                  routine: d.routine.map((s, i) =>
                    i === index ? { ...s, place_id: e.target.value } : s,
                  ),
                })
              }
            >
              {locationOptions}
            </ChoiceSelect>
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                edit({ routine: d.routine.filter((_, i) => i !== index) })
              }
            >
              移除规律
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="outline"
          disabled={d.routine.length >= 42}
          onClick={() =>
            edit({
              routine: [
                ...d.routine,
                {
                  day: "daily",
                  start: "09:00",
                  end: "17:00",
                  place_id: d.homePlaceId || "",
                  activity: "",
                },
              ],
            })
          }
        >
          添加生活规律
        </Button>
        <JsonDetails value={d.routine} title="完整规律记录" />
      </Disclosure>
    </div>
  );
}
