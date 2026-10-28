import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { ChoiceSelect, FieldInput, Textarea, Disclosure } from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import { PeopleReader, PeopleFact } from "./PeopleParts";
import JsonDetails from "./JsonDetails";
import SemanticTree from "./SemanticTree";
const clone = (value) => JSON.parse(JSON.stringify(value));
export default function PersonDefinition({ model: m, onClose }) {
  const { w, fid, owner } = m,
    key = `fluctlight-people:${owner}:${fid}:definition`,
    trigger = useRef(null),
    saved = useRef(null),
    applied = useRef(false);
  function restore() {
    try {
      return JSON.parse(sessionStorage.getItem(key + ":pointer") || "null");
    } catch {
      return null;
    }
  }
  const [pointer, setPointer] = useState(restore),
    [phase, setPhase] = useState(pointer ? "saved" : "create"),
    [action, setAction] = useState(null),
    [previewOpen, setPreviewOpen] = useState(false),
    [choices, setChoices] = useState({}),
    [appliedResult, setAppliedResult] = useState(null),
    [placeId, setPlaceId] = useState(pointer?.placeId || "");
  const resource = useWorkspaceRead({
    identity: ["npc-definition-result", pointer?.id],
    enabled: !!pointer?.id,
    read: async () => ({
      items: [await readResource(pointer.id)],
      next_cursor: "",
    }),
  });
  const r = resource.items[0],
    version = pointer?.version || r?.published_version;
  const preview = useWorkspaceRead({
    identity: ["npc-definition-preview", pointer?.id, version],
    enabled: !!r && !!version && previewOpen,
    read: async () => ({
      items: [await readApplyPreview(r.id, version)],
      next_cursor: "",
    }),
  });
  function remember(value) {
    if (!m.live()) return;
    sessionStorage.setItem(key + ":pointer", JSON.stringify(value));
    setPointer(value);
  }
  async function readResource(id) {
    const row = await w.client.studioResource(id);
    if (
      row.id !== id ||
      row.kind !== "npc" ||
      !Number.isSafeInteger(row.revision)
    )
      throw Error("人物定义来源不匹配。");
    return row;
  }
  const base = (identity) => ({
    identity,
    scope: ["npc-definition", fid, identity],
    storageKey: key + ":" + identity,
  });
  async function readApplyPreview(id, publication) {
    const worlds = await w.client.worldCatalog(fid),
      targetWorld = worlds.active_binding?.world_id;
    if (!targetWorld) throw Error("请先建立并选择运行世界，再明确应用人物。");
    const [p, snapshot] = await Promise.all([
      w.client.previewStudioApply(id, {
        fluctlightId: fid,
        resourceVersion: publication,
      }),
      w.client.catalogWorld(fid, targetWorld),
    ]);
    const existing = snapshot.npcs?.find((n) => n.id === p.runtime_id),
      matching = snapshot.npcs?.find((n) => n.source_resource_id === id);
    if (
      snapshot.world?.id !== targetWorld ||
      snapshot.world?.fluctlight_id !== fid ||
      p.available === false ||
      !Array.isArray(p.diff) ||
      !Number.isSafeInteger(p.instance_revision) ||
      (p.runtime_id &&
        (!existing || existing.revision !== p.runtime_revision)) ||
      (!p.runtime_id && matching)
    )
      throw Error(p.semantics || "实际应用差异或世界范围已变化，请重新预览。");
    return {
      ...p,
      id,
      targetWorld,
      localOverrides: existing?.local_overrides || {},
    };
  }
  function assertScope() {
    if (!m.live()) throw Error("账号或角色范围已变化，请回到原范围核对。");
  }
  function outstanding(identity) {
    try {
      return !!(
        sessionStorage.getItem(key + ":" + identity + ":command") ||
        sessionStorage.getItem(key + ":" + identity + ":command:outcome")
      );
    } catch {
      return true;
    }
  }
  function beginAnother() {
    if (
      outstanding("save") ||
      (r &&
        (outstanding("publish:" + r.id) ||
          (version && outstanding("apply:" + r.id + ":" + version))))
    )
      return;
    sessionStorage.removeItem(key + ":pointer");
    setPointer(null);
    setPhase("create");
    setPreviewOpen(false);
    setChoices({});
    setPlaceId("");
    setAppliedResult(null);
  }
  const createAction = {
    ...base("save"),
    title: "新增世界人物",
    description: "先保存人物定义；发布并明确应用后才进入世界目录。",
    submitLabel: "保存人物定义",
    initial: {
      id: null,
      name: "",
      role: "",
      background: "",
      placeId: "",
      body: {},
      baseName: "",
      baseBody: {},
      expectedVersion: undefined,
    },
    validate: (d) => {
      if (
        !d.name.trim() ||
        d.name.trim().length > 200 ||
        d.role.length > 512 ||
        d.background.length > 8000
      )
        throw Error("请填写人物名字，并核对身份与背景长度。");
    },
    build: (d) => ({
      id: d.id,
      placeId: d.placeId,
      payload: {
        kind: "npc",
        name: d.name.trim(),
        body: { ...d.body, role: d.role, background: d.background },
        ...(d.id ? { expectedVersion: d.expectedVersion } : {}),
        reason: "所有者保存世界人物定义",
      },
    }),
    execute: async (value, idempotencyKey) => {
      assertScope();
      const row = await (value.id
        ? w.client.updateStudioResource(value.id, {
            ...value.payload,
            idempotencyKey,
          })
        : w.client.createStudioResource({ ...value.payload, idempotencyKey }));
      if (
        !row.id ||
        row.kind !== "npc" ||
        !Number.isSafeInteger(row.revision) ||
        !row.draft_version
      )
        throw Error("人物定义保存回执尚未完整确认。");
      return row;
    },
    afterAcknowledged: (d, result) => ({
      ...d,
      id: result.id,
      expectedVersion: result.draft_version,
      body: clone(result.body),
      baseBody: clone(result.body),
      baseName: result.name,
    }),
    readLatest: async (d) => (d.id ? readResource(d.id) : null),
    adoptLatest: (d, row) =>
      row
        ? {
            ...d,
            id: row.id,
            expectedVersion: row.draft_version,
            body: clone(row.body),
            baseBody: clone(row.body),
            baseName: row.name,
            name: d.name === d.baseName ? row.name : d.name,
            role:
              d.role === (d.baseBody.role || "") ? row.body.role || "" : d.role,
            background:
              d.background === (d.baseBody.background || "")
                ? row.body.background || ""
                : d.background,
          }
        : d,
    onSaved: async (result, value) => {
      const current = await readResource(result.id);
      if (current.revision < result.revision)
        throw Error("人物定义的新版本尚未读回。");
      if (m.live())
        saved.current = {
          resource: current,
          pointer: {
            id: current.id,
            version: current.published_version || "",
            placeId: value.placeId,
          },
        };
    },
    fields: (d, edit) => (
      <div className="replica-profile replica-people companion-form-stack lp-wide">
        {[
          ["name", "名字"],
          ["role", "职业 / 身份"],
          ["background", "背景与性格"],
        ].map(([field, label]) => {
          const Input = field === "background" ? Textarea : FieldInput;
          return (
            <label className="replica-field" key={field}>
              <span>{label}</span>
              <Input
                aria-label={label}
                value={d[field]}
                onChange={(e) => edit({ [field]: e.target.value })}
              />
            </label>
          );
        })}
        <label className="replica-field">
          <span>常在的地点</span>
          <ChoiceSelect
            aria-label="常在的地点"
            value={d.placeId}
            onChange={(e) => edit({ placeId: e.target.value })}
          >
            <option value="">应用时再设定</option>
            {(m.catalog.extra.places || [])
              .filter((p) => p.status === "active")
              .map((p) => (
                <option key={p.id} value={p.id}>
                  {p.alias || p.name}
                </option>
              ))}
          </ChoiceSelect>
        </label>
        <WorkspaceInfo>
          保存定义不会自动发布、应用、生成关系或建立实际接触。
        </WorkspaceInfo>
      </div>
    ),
  };
  function finishCreate() {
    if (saved.current) {
      const result = saved.current;
      saved.current = null;
      remember(result.pointer);
      resource.adopt({ items: [result.resource], next_cursor: "" });
      setPlaceId(result.pointer.placeId);
      setPhase("saved");
    } else onClose();
  }
  function publish(event) {
    if (!r) return;
    trigger.current = event.currentTarget;
    setAction({
      ...base("publish:" + r.id),
      title: "发布人物保存版本",
      description: "发布已保存的明确版本，运行世界不会自动改变。",
      submitLabel: "确认发布人物定义",
      initial: { revision: r.revision, body: clone(r.body) },
      build: (d) => ({
        expectedRevision: d.revision,
        reason: "所有者发布人物定义保存版本",
      }),
      execute: async (body, idempotencyKey) => {
        assertScope();
        const result = await w.client.publishStudioResource(r.id, {
          ...body,
          idempotencyKey,
        });
        if (
          result.id !== r.id ||
          result.status !== "published" ||
          !result.published_version ||
          !Number.isSafeInteger(result.revision)
        )
          throw Error("人物发布回执尚未完整确认。");
        return result;
      },
      readLatest: () => readResource(r.id),
      adoptLatest: (d, row) => ({
        ...d,
        revision: row.revision,
        body: clone(row.body),
      }),
      onSaved: async (result) => {
        const row = await readResource(r.id);
        if (
          row.revision < result.revision ||
          (row.published_version !== result.published_version &&
            !row.versions?.some((v) => v.version === result.published_version))
        )
          throw Error("已发布的精确版本尚未读回。");
        if (m.live()) {
          resource.adopt({ items: [row], next_cursor: "" });
          remember({ id: row.id, version: result.published_version, placeId });
        }
      },
      fields: (d) => <SemanticTree value={d.body} />,
    });
  }
  function apply(event) {
    if (!r || !preview.items[0]) return;
    m.assertWritable();
    trigger.current = event.currentTarget;
    const p = preview.items[0];
    setAction({
      ...base("apply:" + r.id + ":" + version),
      title: "明确应用人物到世界",
      description:
        "使用已核对的发布版本与差异，关系和接触记录不会因应用而产生。",
      submitLabel: "确认应用已发布定义",
      preAdmissionConflicts: ["revision_conflict", "preview_target_changed"],
      initial: { preview: clone(p), choices: clone(choices), placeId },
      build: (d) => ({
        targetWorld: d.preview.targetWorld,
        payload: {
          fluctlightId: fid,
          resourceVersion: version,
          expectedInstanceRevision: d.preview.instance_revision,
          expectedRuntimeRevision: d.preview.runtime_revision ?? 0,
          homePlaceId: d.placeId || null,
          choices: d.preview.diff.map((row) => ({
            field: row.field,
            decision: Object.hasOwn(d.preview.localOverrides || {}, row.field)
              ? "keep"
              : !d.preview.runtime_id
                ? "override"
                : d.choices[row.field] || "keep",
          })),
          reason: "所有者明确应用已发布人物定义",
        },
      }),
      execute: async (value, idempotencyKey) => {
        m.assertWritable();
        const worlds = await w.client.worldCatalog(fid);
        if (worlds.active_binding?.world_id !== value.targetWorld)
          throw Object.assign(
            Error("当前世界已变化，请回到原世界核对本次应用。"),
            { status: 409, code: "preview_target_changed" },
          );
        const result = await w.client.applyStudioResource(r.id, {
          ...value.payload,
          idempotencyKey,
        });
        if (
          result.applied !== true ||
          result.status !== "applied" ||
          !result.id ||
          result.resource_id !== r.id ||
          result.resource_version !== value.payload.resourceVersion ||
          !Number.isSafeInteger(result.runtime_revision) ||
          (result.fluctlight_id && result.fluctlight_id !== fid)
        )
          throw Error("人物尚未实际应用，原提交仍保留。");
        return result;
      },
      readLatest: () => readApplyPreview(r.id, version),
      adoptLatest: (d, next) => {
        if (next.targetWorld !== d.preview.targetWorld)
          throw Error("预览时的世界已变化，请先返回原世界核对。");
        return { ...d, preview: next };
      },
      onSaved: async (result, value) => {
        const snapshot = await w.client.catalogWorld(fid, value.targetWorld),
          npc = snapshot.npcs?.find((row) => row.id === result.id);
        if (
          snapshot.world?.fluctlight_id !== fid ||
          !npc ||
          npc.world_id !== value.targetWorld ||
          npc.source_resource_id !== r.id ||
          (npc.revision === result.runtime_revision &&
            npc.source_version !== value.payload.resourceVersion) ||
          (result.runtime_revision && npc.revision < result.runtime_revision)
        )
          throw Error("实际人物和应用版本尚未读回。");
        if (m.live()) {
          if (m.catalog.extra.world?.id === value.targetWorld)
            await m.adoptNpc(npc);
          else await m.refresh();
          remember({ id: r.id, version, placeId });
          const superseded =
            npc.source_version !== value.payload.resourceVersion;
          setAppliedResult({
            expected: value.payload.resourceVersion,
            actual: npc.source_version,
            npcId: npc.id,
            superseded,
          });
          applied.current = superseded ? "show" : "close";
        }
      },
      fields: (d) => (
        <div className="replica-profile replica-people companion-form-stack lp-wide">
          <p>
            发布版本 {version} · 目标{" "}
            {m.catalog.extra.world?.name || "原预览世界"}
          </p>
          <JsonDetails value={d.preview} title="已核对的差异与版本" />
        </div>
      ),
    });
  }
  if (phase === "create")
    return (
      <WorkspaceCommandDialog
        action={createAction}
        onClose={finishCreate}
        returnFocus={() => m.trigger.current}
      />
    );
  return (
    <>
      <WorkspaceDialog
        className="replica-profile replica-people"
        wide
        open={!action}
        title={r?.name || "人物定义结果"}
        description="草稿、发布与运行世界的应用分别确认。"
        returnFocus={() => m.trigger.current}
        onOpenChange={(v) => {
          if (!v) onClose();
        }}
        footer={
          <>
            <Button variant="outline" onClick={onClose}>
              返回人物目录
            </Button>
            {r && (
              <Button
                variant="outline"
                disabled={
                  !outstanding("publish:" + r.id) &&
                  (!r.draft_version || r.draft_version === r.published_version)
                }
                onClick={publish}
              >
                {outstanding("publish:" + r.id)
                  ? "核对原发布"
                  : "发布此保存版本"}
              </Button>
            )}
            {r && version && (
              <Button
                disabled={m.readOnly || !preview.items[0] || preview.busy}
                onClick={apply}
              >
                明确应用到世界
              </Button>
            )}
          </>
        }
      >
        <div className="companion-form-stack">
          <PeopleReader source={resource} label="人物定义" />
          {appliedResult?.superseded && (
            <WorkspaceInfo>
              原发布版本 {appliedResult.expected}{" "}
              的应用回执已确认；当前人物已有后续应用，运行来源是{" "}
              {appliedResult.actual}。请以当前人物资料为准。
            </WorkspaceInfo>
          )}
          {r && (
            <>
              <PeopleFact label="已保存草稿">
                {r.draft_version || "无"}
              </PeopleFact>
              <PeopleFact label="选定发布版本">
                {version || "尚未发布"}
              </PeopleFact>
              <SemanticTree
                value={
                  r.versions?.find((row) => row.version === version)?.body ||
                  r.body
                }
              />
              <WorkspaceInfo>
                人物定义已经保存。发布不会自动应用，应用也不代表已经结识。
              </WorkspaceInfo>
              <Button variant="link" asChild>
                <a href={"#/studio/npc/" + encodeURIComponent(r.id)}>
                  在创作中心继续完整编辑与版本管理
                </a>
              </Button>
              <Button
                variant="outline"
                disabled={
                  outstanding("save") ||
                  outstanding("publish:" + r.id) ||
                  !!(version && outstanding("apply:" + r.id + ":" + version))
                }
                onClick={beginAnother}
              >
                创建另一份人物定义
              </Button>
              {version && (
                <>
                  <label className="replica-field">
                    <span>应用时的初始地点</span>
                    <ChoiceSelect
                      aria-label="应用时的初始地点"
                      value={placeId}
                      onChange={(e) => {
                        setPlaceId(e.target.value);
                        remember({ ...pointer, placeId: e.target.value });
                      }}
                    >
                      <option value="">未设定</option>
                      {(m.catalog.extra.places || [])
                        .filter((p) => p.status === "active")
                        .map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.alias || p.name}
                          </option>
                        ))}
                    </ChoiceSelect>
                  </label>
                  <Button
                    variant="outline"
                    disabled={preview.busy}
                    onClick={() => {
                      setPreviewOpen(true);
                      if (previewOpen) void preview.load();
                    }}
                  >
                    读取实际应用差异
                  </Button>
                  <PeopleReader source={preview} label="人物应用预览" />
                  {preview.items[0] && (
                    <>
                      <JsonDetails
                        value={preview.items[0]}
                        title="精确发布版本的当前差异"
                      />
                      {preview.items[0].diff.map((row) => (
                        <label className="replica-field" key={row.field}>
                          <span>
                            {{
                              name: "名字",
                              role: "职业 / 身份",
                              background: "背景与性格",
                            }[row.field] || row.field}
                          </span>
                          <SemanticTree
                            value={{ 当前值: row.from, 定义值: row.to }}
                          />
                          {preview.items[0].runtime_id ? (
                            <ChoiceSelect
                              aria-label={
                                ({
                                  name: "名字",
                                  role: "职业 / 身份",
                                  background: "背景与性格",
                                }[row.field] || row.field) + "应用决定"
                              }
                              disabled={Object.hasOwn(
                                preview.items[0].localOverrides || {},
                                row.field,
                              )}
                              value={
                                Object.hasOwn(
                                  preview.items[0].localOverrides || {},
                                  row.field,
                                )
                                  ? "keep"
                                  : choices[row.field] || "keep"
                              }
                              onChange={(e) =>
                                setChoices({
                                  ...choices,
                                  [row.field]: e.target.value,
                                })
                              }
                            >
                              <option value="keep">保留当前值</option>
                              <option value="override">采用发布定义</option>
                            </ChoiceSelect>
                          ) : (
                            <p className="companion-caption">
                              新人物将采用这个发布定义值。
                            </p>
                          )}
                          {Object.hasOwn(
                            preview.items[0].localOverrides || {},
                            row.field,
                          ) && (
                            <p className="companion-caption">
                              这是运行人物保留的局部修正，可在人物资料中明确编辑。
                            </p>
                          )}
                        </label>
                      ))}
                    </>
                  )}
                </>
              )}
              <Disclosure title="完整人物资源与不可变版本">
                <JsonDetails value={r} title="资源原记录" />
              </Disclosure>
            </>
          )}
        </div>
      </WorkspaceDialog>
      {action && (
        <WorkspaceCommandDialog
          key={action.identity}
          action={action}
          onClose={() => {
            setAction(null);
            if (applied.current === "close") {
              applied.current = false;
              onClose();
            } else if (applied.current === "show") {
              applied.current = false;
              setPreviewOpen(false);
            }
          }}
          returnFocus={() => trigger.current}
        />
      )}
    </>
  );
}
