import { useRef, useState } from "react";
import {
  ArrowRightIcon,
  ChevronRightIcon,
  FileClockIcon,
  HeartIcon,
  RefreshCwIcon,
  PencilIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import {
  Surface,
  Badge,
  Disclosure,
  FieldInput,
  Textarea,
  ChoiceSelect,
} from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import PeopleSourceSheet, { SourceValue, SourceFact, SourceDownload } from "./PeopleSourceSheet";
import WorkspaceInfo from "./WorkspaceInfo";
import CharacterAvatar from "./CharacterAvatar";
import { EmptyState } from "./Presentation";
import useWorkspaceRead from "./useWorkspaceRead";
import { PeopleReader, PeopleSection, PeopleFact } from "./PeopleParts";
import { readBackground, saveBackground } from "@/v3/providers/real.ts";
import {
  aboutFields,
  extraAboutFields,
  authoredDraft,
  backgroundPatch,
  validateAuthoredDraft,
  authoredWrite,
  acknowledgeAuthoredDraft,
  rebaseAuthoredDraft,
  backgroundReadbackProven,
  relationshipFor,
  relationMetrics,
} from "@/approved-ui/people-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const meaningful = (value) =>
  value !== null && value !== undefined && value !== "";
export default function AboutYouPanel({ model: m, section = "you" }) {
  const { w, fid, owner, actorId } = m,
    isOwner = actorId === owner,
    subject = actorId === owner ? "你" : "该参与者";
  const [editing, setEditing] = useState(false),
    [knowledge, setKnowledge] = useState(null),
    [historyOpen, setHistoryOpen] = useState(false),
    [notice, setNotice] = useState("");
  const editTrigger = useRef(null),
    readTrigger = useRef(null),
    sourceRef = useRef(null);
  const storageKey = `fluctlight-people:${owner}:${fid}:about:${actorId}`;
  function recovery() {
    try {
      return [":draft", ":command", ":command:outcome"].some((part) =>
        sessionStorage.getItem(storageKey + part),
      );
    } catch {
      return false;
    }
  }
  const [hasRecovery, setRecovery] = useState(recovery);
  async function read() {
    const value = await readBackground(fid, actorId);
    if (
      value.actor_id !== actorId ||
      value.actor_type !== "human" ||
      value.owner !== isOwner ||
      !value.context_version ||
      !value.background ||
      typeof value.background !== "object" ||
      [...(value.facts || []), ...(value.history || [])].some(
        (row) => row.actor_id !== actorId,
      )
    )
      throw Error("人物资料或交流身份范围不匹配。");
    if (isOwner && !value.factsRevision)
      throw Error("当前事实版本尚未完整返回。");
    return value;
  }
  const source = useWorkspaceRead({
    identity: ["about-you", actorId],
    read: async () => ({ items: [await read()], next_cursor: "" }),
  });
  sourceRef.current = source;
  const data = source.items[0],
    profile = m.profile.items[0],
    activeProfile = profile?.personality_runtime?.active_profile_id,
    relation = relationshipFor(
      data?.relationships,
      actorId,
      activeProfile,
      fid,
    ),
    metrics = relationMetrics(relation),
    authoredHistory = (data?.history || []).filter(
      (row) => row.epistemic_kind === "owner_defined",
    ),
    historicalRelation = (data?.relationship_history || []).filter(
      (row) => row.relationship_id === relation?.id,
    ),
    facts = (data?.facts || []).filter(
      (row) => row.epistemic_kind === "user_statement",
    );
  const memories = useWorkspaceRead({
    identity: ["about-you-memories", actorId],
    enabled: knowledge === "facts" && isOwner,
    read: async (cursor) => {
      const page = await w.client.managedMemories(fid, {
        aboutOwner: true,
        status: "active",
        cursor,
        limit: 20,
      });
      if (page.items.some((row) => row.owner_fluctlight_id !== fid))
        throw Error("了解记录的角色范围不匹配。");
      return page;
    },
  });
  const openEdit = (event) => {
    editTrigger.current = event.currentTarget;
    setEditing(true);
  };
  const closeEdit = () => {
    setEditing(false);
    setRecovery(recovery());
  };
  const action = data
    ? {
        identity: "about-you:" + actorId,
        scope: ["about-you", fid, actorId],
        storageKey,
        presentation: "inline",
        title: "编辑关于你的资料",
        submitLabel: "保存资料",
        cancelLabel: "取消编辑",
        confirmDiscard: true,
        disabled: m.readOnly,
        isDirty: (d) =>
          !!Object.keys(backgroundPatch(d)).length || !!d.reason.trim(),
        initial: authoredDraft(data),
        validate: (d) => validateAuthoredDraft(d, isOwner),
        build: authoredWrite,
        preAdmissionConflicts: [
          "actor_user_background_conflict",
          "revision_conflict",
          "actor_context_conflict",
        ],
        execute: async (value, key) => {
          m.assertWritable();
          const result = await saveBackground(
            fid,
            actorId,
            { ...value.input, idempotencyKey: key },
            value.factsRevision,
          );
          if (
            !result?.background ||
            !Array.isArray(result.facts) ||
            (isOwner
              ? !result.current_facts_revision
              : result.actor_id !== actorId || !result.context_version)
          )
            throw Error("资料保存回执尚未完整确认，原提交仍保留。");
          return result;
        },
        afterAcknowledged: acknowledgeAuthoredDraft,
        readLatest: read,
        adoptLatest: rebaseAuthoredDraft,
        onSaved: async (result, value) => {
          const latest = await read();
          if (!backgroundReadbackProven(result, latest, value, isOwner))
            throw Error(
              "保存已受理，已提交的事实版本尚未完整读回，请只读恢复结果。",
            );
          if (m.live()) {
            sourceRef.current.adopt({ items: [latest], next_cursor: "" });
            setNotice("你的资料已保存并读回，自填来源与已有认识分别保留。");
          }
        },
        fields: (draft, edit) => (
          <fieldset className="people-editor-fields" disabled={m.readOnly}>
            <AboutFields draft={draft} edit={edit} owner={isOwner} />
          </fieldset>
        ),
      }
    : null;
  const record = (row, index) => <SourceFact key={row.fact_id || index} row={row} />;
  const sourceScope = m.c.name + " · " + (isOwner ? "你的资料" : "当前交流参与者") + " · 当前角色范围";
  const relationView = () =>
    relation ? (
      <>
        <Badge variant="outline">
          {relation.profile_id ? "当前人格视角" : "共同关系记录"}
        </Badge>
        <p className="companion-body-text">
          {relation.summary || "尚未提供额外关系说明。"}
        </p>
        <PeopleFact label="关系称呼">
          <SourceValue value={relation.role?.label ?? relation.role} />
        </PeopleFact>
        <SourceDownload value={relation} label="下载完整关系与来源" />
      </>
    ) : (
      <EmptyState>
        <h3>这里还没有读到当前范围的关系记录</h3>
        <p>自填关系描述与角色视角下的关系分别保存。</p>
      </EmptyState>
    );
  if (section === "relationship")
    return (
      <>
        <PeopleReader source={source} label="关系依据" />
        <div className="companion-two-columns">
          <Surface className="companion-relationship-card">
            <div className="companion-relationship-avatars">
              <span className="companion-owner-circle">
                {isOwner ? "你" : "对方"}
              </span>
              <span className="companion-relationship-line">
                <HeartIcon size={18} />
              </span>
              <CharacterAvatar
                className="avatar-lg"
                name={m.c.name}
                initial={m.c.initial}
                src={m.c.avatar}
              />
            </div>
            <h2>
              {relation?.role?.label ||
                (isOwner ? "你们的关系" : "与该参与者的关系")}，从认真倾听开始。
            </h2>
            <p>
              {data
                ? relation?.summary || "当前范围尚未读到关系说明。"
                : "关系资料待读取。"}
            </p>
            <div className="companion-relationship-metrics">
              {metrics.map((row) => (
                <div key={row.key}>
                  <span>{row.label}</span>
                  <Progress value={row.value * 100} aria-label={row.label} />
                  <small>{Math.round(row.value * 100)}%</small>
                </div>
              ))}
            </div>
            {data && !metrics.length && (
              <p className="companion-body-text">尚未读到可展示的关系指标。</p>
            )}
            <WorkspaceInfo>
              这是角色视角下的关系记录，自填资料不会直接改写这些指标。
            </WorkspaceInfo>
          </Surface>
          <PeopleSection title="一些拉近距离的片刻">
            <p className="companion-body-text">当前资料尚未提供可展示的共同经历。你们可以从接下来的交流中慢慢了解彼此。</p>
            <Button
              variant="outline"
              onClick={() =>
                w.navigatePath("/c/" + encodeURIComponent(fid) + "/chat")
              }
            >
              继续聊聊
              <ArrowRightIcon size={14} />
            </Button>
            {relation && (
              <Button variant="link" asChild>
                <a
                  href={
                    "#/c/" +
                    encodeURIComponent(fid) +
                    "/people/relation/" +
                    encodeURIComponent(relation.id) +
                    "?actor=" +
                    encodeURIComponent(actorId)
                  }
                >
                  关系修订与历史恢复
                </a>
              </Button>
            )}
          </PeopleSection>
        </div>
      </>
    );
  return (
    <>
      <PeopleReader source={source} label="关于你的资料" />
      {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
      {hasRecovery && !editing && (
        <WorkspaceInfo>
          上次资料修改的输入或回执仍待核对。
          <Button variant="outline" disabled={!data} onClick={openEdit}>
            继续核对资料修改
          </Button>
        </WorkspaceInfo>
      )}
      <div className="companion-two-columns">
        <PeopleSection
          title={isOwner ? "关于你" : "交流参与者资料"}
          description={
            m.c.name + "会将这些作为你主动填写的资料。只填你愿意分享的。"
          }
          action={
            !editing && (
              <div className="companion-action-row">
                <Button
                  variant="outline"
                  size="sm"
                  ref={editTrigger}
                  disabled={!data || m.readOnly}
                  onClick={openEdit}
                >
                  <PencilIcon size={13} />
                  编辑资料
                </Button>
              </div>
            )
          }
        >
          {editing && action ? (
            <WorkspaceCommandDialog
              action={action}
              onClose={closeEdit}
              returnFocus={() => (m.live() ? editTrigger.current : null)}
            />
          ) : data ? (
            <>
              <div className="companion-fact-grid">
                {aboutFields.slice(0, 6).map(([key, label]) => (
                  <PeopleFact key={key} label={label}>
                    {meaningful(data.background[key])
                      ? String(data.background[key])
                      : undefined}
                  </PeopleFact>
                ))}
              </div>
              {aboutFields.slice(6).map(([key, label]) => (
                <div className="companion-reading-block" key={key}>
                  <h3>{label}</h3>
                  <p>
                    {meaningful(data.background[key])
                      ? String(data.background[key])
                      : "还没有填写"}
                  </p>
                </div>
              ))}

            </>
          ) : (
            <p className="companion-body-text">你的资料待读取。</p>
          )}
        </PeopleSection>
        <aside className="companion-stack">
          <Surface className="companion-portrait-note">
            <span className="companion-eyebrow">IN HER EYES</span>
            <h3>
              {m.c.name}对{subject}的了解
            </h3>
            <p>你填写的资料，与{m.c.name}在相处中形成的认识，分别保存。</p>
            <Button
              variant="outline"
              disabled={!data}
              onClick={(e) => {
                readTrigger.current = e.currentTarget;
                setKnowledge("facts");
              }}
            >
              查看了解记录
              <ChevronRightIcon size={13} />
            </Button>
          </Surface>
          <PeopleSection title={m.c.name + "视角下的关系"}>
            <p className="companion-body-text">
              你填写的关系描述，不会直接改变{m.c.name}对你们关系的判断。
            </p>
            <Button
              variant="outline"
              disabled={!data}
              onClick={(e) => {
                readTrigger.current = e.currentTarget;
                setKnowledge("relation");
              }}
            >
              关系与交往依据
              <ArrowRightIcon size={13} />
            </Button>
          </PeopleSection>
          <PeopleSection title="来源与修订历史">
            <p className="companion-body-text">
              {data
                ? authoredHistory.length
                  ? "已读 " + authoredHistory.length + " 条自填资料记录。"
                  : "还没有已读自填资料修订记录。"
                : "修订记录待读取。"}
            </p>
            <Button
              variant="ghost"
              disabled={!data}
              onClick={(e) => {
                readTrigger.current = e.currentTarget;
                setHistoryOpen(true);
              }}
            >
              <FileClockIcon size={14} />
              查看修订记录
            </Button>
          </PeopleSection>
        </aside>
      </div>
      <PeopleSourceSheet
        scope={sourceScope}
        source={data}
        open={!!knowledge}
        onOpenChange={(v) => {
          if (!v) setKnowledge(null);
        }}
        returnFocus={() => readTrigger.current}
        title={
          m.c.name +
          (knowledge === "facts"
            ? "对" + subject + "的了解"
            : "视角下的关系与依据")
        }
        description="交流事实、真实记忆与自填资料保留各自来源。"
      >
        <div className="companion-stack">
          {knowledge === "relation" ? (
            relationView()
          ) : (
            <>
              <WorkspaceInfo>
                这里展示交流中表达的事实和实际记忆；Owner自填来源不会直接作为独立了解记录。
              </WorkspaceInfo>
              {facts.map(record)}
              {isOwner && <PeopleReader source={memories} label="了解记忆" />}
              {memories.items.map((row) => (
                <div className="companion-reading-block" key={row.id}>
                  <h3>{row.content}</h3>
                  <p>记忆 · {userInstant(row.created_at)}</p>
                  <Button
                    variant="link"
                    onClick={() =>
                      w.navigatePath(
                        "/c/" +
                          encodeURIComponent(fid) +
                          "/growth/memory/" +
                          encodeURIComponent(row.id),
                      )
                    }
                  >
                    查看记忆与来源
                  </Button>
                  <SourceDownload value={row} label="下载这条记忆的原记录" />
                </div>
              ))}
              {data &&
                !facts.length &&
                (!isOwner ||
                  (memories.ready &&
                    !memories.items.length &&
                    !memories.error)) && (
                  <EmptyState>
                    <h3>还在慢慢了解彼此</h3>
                    <p>
                      当前没有已读交流事实或有效记忆，自填资料继续独立保存。
                    </p>
                  </EmptyState>
                )}
              {memories.cursor && (
                <Button
                  variant="outline"
                  disabled={memories.busy}
                  onClick={memories.more}
                >
                  加载更多了解记忆
                </Button>
              )}
            </>
          )}
        </div>
      </PeopleSourceSheet>
      <PeopleSourceSheet
        scope={sourceScope}
        source={data}
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        returnFocus={() => readTrigger.current}
        title="你的资料修订历史"
        description="按实际事实记录展示自填来源，不把字段条数当作保存次数。"
      >
        <Button variant="outline" disabled={source.busy} onClick={source.load}><RefreshCwIcon size={14} />刷新你的资料</Button>
        <PeopleSection title="其他自填资料">
          {extraAboutFields.map(([key, label]) => Object.hasOwn(data?.background || {}, key) && <PeopleFact key={key} label={label}><SourceValue value={data.background[key]} /></PeopleFact>)}
          <p className="companion-body-text">完整原始记录下载保留全部来源、未知字段和原始值。</p>
        </PeopleSection>
        <PeopleSection title="自填资料修订">{authoredHistory.map(record)}</PeopleSection>
        {!!historicalRelation.length && <PeopleSection title="关系修订记录">{historicalRelation.map((row, index) => <article className="people-source-record" key={row.id || index}>
          <PeopleFact label="修订时间">{row.created_at ? userInstant(row.created_at) : "尚未记录时间"}</PeopleFact>
          <PeopleFact label="关系说明"><SourceValue value={row.summary} /></PeopleFact>
          <PeopleFact label="关系称呼"><SourceValue value={row.role?.label ?? row.role} /></PeopleFact>
          <SourceDownload value={row} />
        </article>)}</PeopleSection>}
        {data && !authoredHistory.length && (
          <EmptyState>
            <h3>尚无已读自填修订记录</h3>
          </EmptyState>
        )}
      </PeopleSourceSheet>
    </>
  );
}
function AboutFields({ draft: d, edit, owner }) {
  const field = ([key, label]) => {
    const Input = [
      "wants_you_to_know",
      "rules",
      "background",
      "shared_information",
    ].includes(key)
      ? Textarea
      : FieldInput;
    return (
      <label
        className={
          "replica-field " +
          (["wants_you_to_know", "rules"].includes(key)
            ? "companion-field-wide"
            : "")
        }
        key={key}
      >
        <span>{label}</span>
        {key === "meeting_confirmed" ? (
          <ChoiceSelect
            aria-label={label}
            value={d.fields[key]}
            onChange={(e) =>
              edit({ fields: { ...d.fields, [key]: e.target.value } })
            }
          >
            <option value="unknown">尚未确认</option>
            <option value="true">是</option>
            <option value="false">否</option>
          </ChoiceSelect>
        ) : (
          <Input
            aria-label={label}
            value={d.fields[key] ?? ""}
            onChange={(e) =>
              edit({ fields: { ...d.fields, [key]: e.target.value } })
            }
            maxLength={1024}
          />
        )}
      </label>
    );
  };
  return (
    <div className="companion-form-stack">
      <div className="companion-edit-grid">{aboutFields.map(field)}</div>
      <Disclosure title="其他自填字段">
        <div className="companion-edit-grid">{extraAboutFields.map(field)}</div>
      </Disclosure>
      <div className="companion-revision-box">
        <h3>本次修订</h3>
        <label className="replica-field">
          <span>修改含义</span>
          <ChoiceSelect
            aria-label="修改含义"
            value={d.operation}
            onChange={(e) => edit({ operation: e.target.value })}
          >
            <option value="change">现在情况变了</option>
            <option value="correct">之前写错了</option>
          </ChoiceSelect>
        </label>
        <label className="replica-field">
          <span>修改说明</span>
          <FieldInput
            aria-label="修改说明"
            value={d.reason}
            onChange={(e) => edit({ reason: e.target.value })}
            maxLength={owner ? 500 : 1000}
            placeholder="简单说明变化，保留修订的来由"
          />
        </label>
      </div>
    </div>
  );
}
