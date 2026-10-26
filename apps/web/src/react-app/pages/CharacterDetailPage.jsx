import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {InlineFeedback,ReadState} from "@/react-app/shared/Presentation";
import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import { useController } from "@/react-app/page";
import { selector, readValue } from "@/state/view-model";
import {
  ActionButton,
  Surface,
  FieldInput,
  ChoiceSelect,
  Textarea,
  Disclosure,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
import { ActionFeedback } from "@/react-app/components";
import CharacterAvatar from "@/react-app/shared/CharacterAvatar";
import SemanticTree from "@/react-app/shared/SemanticTree";
import ProfileForm from "@/react-app/shared/ProfileForm";
import {
  personaPresentation,
  workingPersonaPresentation,
} from "@/approved-ui/profile-presentation.mjs";
import { profileSnapshotFromDetail } from "@/v3/proto/data/profile.js";
import {
  semanticLabel,
  hasSemanticValue,
} from "@/v3/proto/components/semantic-fields.js";
import {
  coreFromDetail,
  profileFields,
  profileChanges,
  profileSaveCommand,
  rebaseProfileFields,
} from "@/approved-ui/profile-fields.mjs";
import { errorMessage, stateLabel } from "@/v3/domain-presentation.mjs";
export function createProfileModel(lifecycle) {

  const w = lifecycle.workspace,
    fid = w.state.selectedId,
    scope = w.currentScope(),
    detail = lifecycle.cell(null),
    world = lifecycle.cell(null),
    editing = lifecycle.cell(false),
    fields = lifecycle.cell([]),
    base = lifecycle.cell(null),
    baseline = lifecycle.cell(null),
    saving = lifecycle.cell(false),
    readbackPending = lifecycle.cell(false),
    submittedFields = lifecycle.cell(null),
    error = lifecycle.cell(""),
    conflict = lifecycle.cell(false),
    last = lifecycle.cell(null),
    switching = lifecycle.cell(false),
    target = lifecycle.cell(""),
    reason = lifecycle.cell(""),
    confirm = lifecycle.cell(false),
    personaRecord = lifecycle.cell(null),
    personaSubmitted = lifecycle.cell(null),
    personaReadbackPending = lifecycle.cell(false),
    personaFeedback = lifecycle.cell("");
  let active = true,
    epoch = 0,
    reading = false,
    timer;
  const live = () => active && w.currentScope() === scope;
  const snapshot = selector(() =>
      profileSnapshotFromDetail(detail.value || {}, w.current.value._directory),
    ),
    core = selector(() => coreFromDetail(detail.value || {})),
    personas = selector(() => snapshot.value.profiles),
    currentPersona = selector(() =>
      personas.value.find((p) => p.id === snapshot.value.activeProfileId),
    );
  const sections = selector(() =>
    [
      ["identity", "身份与背景", core.value.identity],
      [
        "personality",
        "共同性格",
        detail.value?.personality || detail.value?.core_persona?.personality,
      ],
      ["policy", "表达与相处", core.value.behavioral_policy],
      ["life", "生活设定", core.value.life_profile],
      [
        "working",
        "当前表达画像",
        workingPersonaPresentation(
          snapshot.value.groups.find((group) => group.key === "working")?.value,
        ),
      ],
      [
        "current",
        "此刻状态",
        detail.value?.current_state || detail.value?.inner_state,
      ],
      ["developing", "发展中的自我", detail.value?.developing_self],
    ].filter(([, , value]) => hasSemanticValue(value)),
  );
  const personaValue = (persona) =>
    personaPresentation(persona, {
      ...core.value,
      personality:
        detail.value?.personality || detail.value?.core_persona?.personality,
    });
  const title = selector(
      () => detail.value?.identity?.name || w.current.value.name,
    ),
    subtitle = selector(() =>
      [
        detail.value?.identity?.age != null
          ? detail.value.identity.age + " 岁"
          : "",
        detail.value?.identity?.occupation,
        stateLabel(detail.value?.status, ""),
      ]
        .filter(Boolean)
        .join(" · "),
    );
  const avatar = selector(
    () =>
      snapshot.value.avatar || w.current.value._directory?.avatar?.url || "",
  );
  const governance = (kind) =>
    "#/c/" +
    encodeURIComponent(fid) +
    "/governance" +
    (kind ? "?tab=" + encodeURIComponent(kind) : "");
  async function read({ preserve = false } = {}) {
    const token = ++epoch;
    try {
      const raw = await w.client.detail(fid);
      if (!live() || token !== epoch) return;
      detail.value = raw;
      w.current.value.detail = raw;
      if (preserve) {
        base.value = coreFromDetail(raw);
        baseline.value = raw.current_revision;
        fields.value = rebaseProfileFields(
          base.value,
          fields.value,
          semanticLabel,
          readbackPending.value ? submittedFields.value : null,
        );
      }
      error.value = "";
      readbackPending.value = false;
      submittedFields.value = null;
      return true;
    } catch (cause) {
      if (live() && token === epoch) error.value = errorMessage(cause);
      return false;
    }
  }
  function edit() {
    if (!detail.value || snapshot.value.readOnly) return;
    base.value = coreFromDetail(detail.value);
    baseline.value = detail.value.current_revision;
    fields.value = profileFields(base.value, semanticLabel);
    editing.value = true;
    error.value = "";
    conflict.value = false;
  }
  async function save() {
    if (saving.value || snapshot.value.readOnly) return;
    try {
      const changes = profileChanges(base.value, fields.value);
      if (!Object.keys(changes).length) {
        editing.value = false;
        return;
      }
      if (!Number.isSafeInteger(baseline.value))
        throw Error("资料版本尚未读取，请重新读取后保存");
      const submitted = JSON.parse(JSON.stringify(fields.value));
      saving.value = true;
      error.value = "";
      const record = w.command(
        "角色详情",
        "profile:" + fid,
        {
          changes,
          expectedRevision: baseline.value,
          reason: "所有者编辑角色设定资料",
        },
        profileSaveCommand({
          client: w.client,
          fid,
          live,
        }),
        {
          preAdmissionConflicts: [
            "revision_conflict",
            "foundation_proposal_stale",
          ],
          success: "角色资料已保存，并记录本次修订",
          onSaved: async () => {
            if (!live()) return;
            readbackPending.value = true;
            submittedFields.value = submitted;
            const readback = await read({
              preserve: true,
            });
            if (!readback) return;
            if (live()) {
              if (!Object.keys(profileChanges(base.value, fields.value)).length)
                editing.value = false;
              void w.refreshDirectory();
            }
          },
        },
      );
      last.value = record;
      await record.completion;
    } catch (cause) {
      if (live()) {
        error.value = errorMessage(cause);
        conflict.value = cause.status === 409;
      }
    } finally {
      if (live()) saving.value = false;
    }
  }
  async function rebase() {
    try {
      if (last.value) await last.value.release();
      if (!live()) return;
      await read({
        preserve: true,
      });
      conflict.value = false;
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    }
  }
  async function switchPersona() {
  if (
    !live() || snapshot.value.readOnly || personaRecord.value?.requestUnresolved || personaReadbackPending.value ||
    !target.value ||
    !reason.value.trim() ||
    !confirm.value ||
    switching.value
  )
    return;
  if (
    !personas.value.some((p) => p.id === target.value) ||
    target.value === snapshot.value.activeProfileId
  ) {
    error.value = "请选择当前角色可用的其他人格";
    return;
  }
  if (
    snapshot.value.switchTargetIds &&
    !snapshot.value.switchTargetIds.includes(target.value)
  ) {
    error.value = "该人格当前不能通过所有者指令切换";
    return;
  }
  switching.value = true;
  const submitted = { target: target.value, reason: reason.value };
  personaSubmitted.value=submitted;
  personaFeedback.value='';error.value='';
  try {
    const body = {
      targetPersonaId: target.value,
      expectedRevision: snapshot.value.foundationRevision,
      expectedRuntimeRevision: snapshot.value.runtimeRevision,
      reason: reason.value.trim(),
      confirm: true,
    };
    if (
      !Number.isSafeInteger(body.expectedRevision) ||
      !Number.isSafeInteger(body.expectedRuntimeRevision)
    )
      throw Error("人格或运行版本未读取完整，请重新读取");
    const record = w.command(
      "角色详情",
      "persona-switch",
      body,
      async (value, key) => {
        const result = await w.client.switchPersona(fid, {
          ...value,
          idempotencyKey: key,
        });
        return result.request_id
          ? { request: { id: result.request_id, status: "pending" } }
          : result;
      },
      {
        onSaved:readPersonaOutcome,
        success: "人格切换命令已提交，请核对当前状态",
      },
    );
    personaRecord.value=record;
    await record.completion;
  } catch (cause) {
    if (live()) error.value = errorMessage(cause);
  } finally {
    if (live()) switching.value = false;
  }
}
async function readPersonaOutcome(receipt){
  if(!live()||!personaSubmitted.value)return;
  personaReadbackPending.value=true;
  if(!await read()){
    if(live())error.value='命令已提交，但最新状态读取失败。原目标和原因已保留，请重新读取，确认前不要重复切换。';
    return;
  }
  if(!live())return;
  const submitted=personaSubmitted.value;
  const result=receipt?.result||receipt;
  const currentMatches=snapshot.value.activeProfileId===submitted.target;
  const recorded=result?.status==='completed'&&result.active_profile_id===submitted.target;
  if(currentMatches||recorded){
    personaReadbackPending.value=false;
    personaFeedback.value=currentMatches?'目标人格已生效，已读取最新状态。':'原切换已完成；当前人格已有后续变化，请以当前记录为准。';
    if(target.value===submitted.target&&reason.value===submitted.reason){target.value='';reason.value='';confirm.value=false;}
  }else error.value='命令已提交，当前人格尚未与目标一致。原目标与原因已保留，请查询原请求和切换记录。';
}

  async function pollPreparation() {
    if (
      !live() ||
      document.hidden ||
      reading ||
      editing.value ||
      saving.value ||
      switching.value ||
      !["pending", "running"].includes(
        detail.value?.persona_preparation?.status,
      )
    )
      return;
    reading = true;
    try {
      await read();
    } finally {
      reading = false;
    }
  }
  async function recoverPreparation() {
    if (
      !live() ||
      saving.value ||
      !detail.value?.persona_preparation?.can_recover
    )
      return;
    saving.value = true;
    try {
      const record = w.command(
        "角色详情",
        "persona-preparation:" + fid,
        {
          expectedRevision: detail.value.current_revision,
        },
        (value, idempotencyKey) =>
          w.client.recoverPersonaPreparation(fid, {
            ...value,
            idempotencyKey,
          }),
        {
          success: "已继续准备角色资料",
          onSaved: async () => {
            if (live()) await read();
          },
        },
      );
      await record.completion;
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    } finally {
      if (live()) saving.value = false;
    }
  }
  const visibility = () => {
    if (!document.hidden) void pollPreparation();
  };
  lifecycle.onStart(() => {
    void read();
    timer = setInterval(() => void pollPreparation(), 5000);
    document.addEventListener("visibilitychange", visibility);
    void w.client
      .world(fid)
      .then((value) => {
        if (live()) world.value = value.world;
      })
      .catch(() => {});
  });
  lifecycle.onDispose(() => {
    active = false;
    epoch++;
    clearInterval(timer);
    document.removeEventListener("visibilitychange", visibility);
  });

const render=(_ctx, _cache) => {
    return (
      <div className="character-detail character-workflow">
        <Surface as="section" >
          <header className="row character-profile-heading">
            <CharacterAvatar
              className="character-avatar"
              name={title.value}
              initial={readValue(w).current.value.initial}
              color={readValue(w).current.value.color}
              src={avatar.value}
            />
            <div>
              <h2>{displayText(title.value)}</h2>
              {subtitle.value ? (
                <small key={0}>{displayText(subtitle.value)}</small>
              ) : null}
            </div>
            <span className="spacer" />
            {detail.value && !editing.value ? (
              <ActionButton variant="outline"
                key={0}

                data-testid="detail-edit"
                disabled={snapshot.value.readOnly}
                onClick={edit}
              >
                {"编辑资料"}
              </ActionButton>
            ) : null}
          </header>
          {detail.value?.persona_preparation &&
          detail.value.persona_preparation.status !== "ready" ? (
            <OperationStatus variant="default" key={0} className="approved-guidance" role="status">
              {" 角色资料已保存。" +
                displayText(
                  detail.value.persona_preparation.status === "failed"
                    ? "表达画像准备遇到问题，可继续原角色的准备。"
                    : "正在后台整理表达画像，离开页面后仍会继续。",
                ) +
                " "}
              {detail.value.persona_preparation.can_recover ? (
                <ActionButton variant="ghost"
                  key={0}

                  disabled={saving.value || editing.value}
                  onClick={recoverPreparation}
                >
                  {"继续准备"}
                </ActionButton>
              ) : null}
            </OperationStatus>
          ) : null}
          {editing.value ? (
            <form
              key={1}
              className="character-form detail-form"
              noValidate=""
              onSubmit={eventModifiers(save, ["prevent"])}
            >
              <p className="muted">
                {
                  " 保存会生成并接受一条资料修订记录。这里编辑的是设定；实际穿着、位置和已发生的经历由生活记录决定。 "
                }
              </p>
              <ProfileForm fields={fields.value} />
              {error.value ? (
                <InlineFeedback key={0}  role="alert">
                  {displayText(error.value)}
                </InlineFeedback>
              ) : null}
              <ActionFeedback labels={["角色详情"]} />
              {readbackPending.value ? (
                <ActionButton variant="outline"
                  key={1}
                  type="button"

                  onClick={($event) =>
                    read({
                      preserve: true,
                    })
                  }
                >
                  {"重新读取已保存资料"}
                </ActionButton>
              ) : null}
              {conflict.value ? (
                <ActionButton variant="outline"
                  key={2}
                  type="button"

                  onClick={rebase}
                >
                  {"读取最新资料，保留当前编辑"}
                </ActionButton>
              ) : null}
              <footer className="wizard-footer">
                <ActionButton variant="outline"
                  type="button"

                  disabled={saving.value}
                  onClick={($event) => (editing.value = false)}
                >
                  {"取消"}
                </ActionButton>
                <span className="spacer" />
                <ActionButton variant="default"

                  data-testid="detail-save"
                  disabled={saving.value || readbackPending.value}
                >
                  {displayText(saving.value ? "保存中…" : "保存")}
                </ActionButton>
              </footer>
            </form>
          ) : (
            <div key={2} className="character-detail-sections">
              {!detail.value && !error.value ? (
                <ReadState key={0} role="status">
                  {"正在读取角色资料…"}
                </ReadState>
              ) : null}
              {error.value ? (
                <InlineFeedback key={1}  role="alert">
                  {displayText(error.value) + " "}
                  <ActionButton variant="ghost"

                    onClick={($event) => read()}
                  >
                    {"重新读取"}
                  </ActionButton>
                </InlineFeedback>
              ) : null}
              <Fragment>
                {listValues(sections.value).map(([key, label, value]) => {
                  return (
                    <section key={key}>
                      <h3>{displayText(label)}</h3>
                      <SemanticTree
                        value={value}
                        identity={core.value.identity}
                        field={key === "personality" ? "personality" : ""}
                      />
                    </section>
                  );
                })}
              </Fragment>
              {personas.value.length ? (
                <section key={2}>
                  <h3>{"人格档案"}</h3>
                  <p>
                    {" 当前人格：" +
                      displayText(
                        currentPersona.value?.name ||
                          snapshot.value.activeProfileId ||
                          "尚未确认",
                      )}
                  </p>
                  <Fragment>
                    {listValues(personas.value).map((persona) => {
                      return (
                        <Disclosure
                          key={persona.id}
                          title={
                            <>
                              {displayText(persona.name || persona.id) + " "}
                              {persona.id === snapshot.value.activeProfileId ? (
                                <small key={0}>{"当前"}</small>
                              ) : null}
                            </>
                          }
                        >
                          <p className="muted">
                            {"这里展示此人格相对共同设定的补充与差异。"}
                          </p>
                          <SemanticTree
                            value={personaValue(persona)}
                            identity={core.value.identity}
                          />
                        </Disclosure>
                      );
                    })}
                  </Fragment>
                  {personas.value.length > 1 && !snapshot.value.readOnly ? (
                    <Disclosure key={0} title={<>{"切换人格"}</>}>
                      <form
                        className="character-stack"
                        onSubmit={eventModifiers(switchPersona, ["prevent"])}
                      >
                        <label>
                          {"目标人格"}
                          <ChoiceSelect
                            aria-label="目标人格"
                            value={target.value}
                            onChange={(event) => {
                              (($event) => (target.value = $event))(
                                event.target.value,
                              );
                            }}
                          >
                            <option value="">{"请选择"}</option>
                            <Fragment>
                              {listValues(
                                personas.value.filter(
                                  (p) =>
                                    p.id !== snapshot.value.activeProfileId &&
                                    (!snapshot.value.switchTargetIds ||
                                      snapshot.value.switchTargetIds.includes(
                                        p.id,
                                      )),
                                ),
                              ).map((persona) => {
                                return (
                                  <option key={persona.id} value={persona.id}>
                                    {displayText(persona.name || persona.id)}
                                  </option>
                                );
                              })}
                            </Fragment>
                          </ChoiceSelect>
                        </label>
                        <label>
                          {"切换原因"}
                          <Textarea
                            maxLength="1000"
                            rows="2"
                            value={reason.value}
                            onChange={(event) => {
                              (($event) => (reason.value = $event))(
                                event.target.value,
                              );
                            }}
                          />
                        </label>
                        <label className="check-field">
                          <FieldInput
                            type="checkbox"
                            modelValue={confirm.value}
                            onModelChange={($event) => (confirm.value = $event)}
                          />
                          {"我确认向当前角色提出这次人格切换"}
                        </label>
                        <ActionButton variant="outline"

                          disabled={
                            switching.value || snapshot.value.readOnly || personaRecord.value?.requestUnresolved || personaReadbackPending.value ||
                            !target.value ||
                            !reason.value.trim() ||
                            !confirm.value
                          }
                        >
                          {displayText(
                            switching.value ? "处理中…" : "确认切换",
                          )}
                        </ActionButton>
{personaFeedback.value && <OperationStatus variant="default" role="status">{personaFeedback.value}</OperationStatus>}
{personaReadbackPending.value && <ActionButton variant="outline" type="button"  disabled={switching.value} onClick={() => readPersonaOutcome()}>重新读取人格切换结果</ActionButton>}
{personaRecord.value?.requestUnresolved && <small className="muted">切换请求已受理，等待原请求确认，当前输入已保留。</small>}

                      </form>
                    </Disclosure>
                  ) : null}
                </section>
              ) : null}
              {detail.value?.core_persona?.personality_system ? (
                <section key={3}>
                  <Disclosure title={<>{"人格之间的关系与协作"}</>}>
                    <SemanticTree
                      value={Object.fromEntries(
                        Object.entries(
                          detail.value.core_persona.personality_system,
                        ).filter(([key]) => key !== "profiles"),
                      )}
                    />
                  </Disclosure>
                </section>
              ) : null}
              {snapshot.value.history.length ? (
                <Disclosure key={4} title={<>{"人格切换记录"}</>}>
                  <ul className="semantic-list">
                    <Fragment>
                      {listValues(snapshot.value.history).map(
                        (entry, index) => {
                          return (
                            <li key={entry.id || index}>
                              {displayText(
                                personas.value.find(
                                  (persona) =>
                                    persona.id === entry.from_profile_id,
                                )?.name || "原人格",
                              ) +
                                " → " +
                                displayText(
                                  personas.value.find(
                                    (persona) =>
                                      persona.id === entry.to_profile_id,
                                  )?.name || "目标人格",
                                ) +
                                " "}
                              {entry.reason ? (
                                <p key={0} className="muted">
                                  {displayText(entry.reason)}
                                </p>
                              ) : null}
                            </li>
                          );
                        },
                      )}
                    </Fragment>
                  </ul>
                  {snapshot.value.historyHasMore ? (
                    <p key={0} className="muted">
                      {"这里展示最近的切换记录，完整记录可在治理入口查看。"}
                    </p>
                  ) : null}
                </Disclosure>
              ) : null}
              <section>
                <h3>{"头像与生活世界"}</h3>
                {world.value?.name ? (
                  <p key={0}>{displayText(world.value.name)}</p>
                ) : null}
                {detail.value?.identity?.residence ? (
                  <p key={1}>
                    {" 居住地 · " +
                      displayText(detail.value.identity.residence)}
                  </p>
                ) : null}
                <div className="row">
                  <ActionButton variant="ghost" asChild >
                    <a

                      href={
                        "#/c/" +
                        encodeURIComponent(readValue(fid)) +
                        "/media/avatar"
                      }
                    >
                      {"更换头像 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                    </a>
                  </ActionButton>
                  <ActionButton variant="ghost" asChild >
                    <a

                      href={
                        "#/c/" + encodeURIComponent(readValue(fid)) + "/world"
                      }
                    >
                      {"查看生活世界 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                    </a>
                  </ActionButton>
                  <ActionButton variant="ghost" asChild >
                    <a

                      href={
                        "#/c/" +
                        encodeURIComponent(readValue(fid)) +
                        "/life/wardrobe"
                      }
                    >
                      {"查看衣柜 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/>
                    </a>
                  </ActionButton>
                </div>
              </section>
              {detail.value ? (
                <Disclosure key={5} title={<>{"技术详情与来源"}</>}>
                  <p>
                    {" 身份核心版本 " +
                      displayText(
                        snapshot.value.foundationRevision ?? "未读取",
                      ) +
                      " · 运行版本 " +
                      displayText(snapshot.value.runtimeRevision ?? "未读取")}
                  </p>
                  <pre className="approved-json">
                    {displayText(
                      JSON.stringify(
                        {
                          core_persona: detail.value.core_persona,
                          provenance: detail.value.provenance,
                          initialization_source:
                            detail.value.initialization_source,
                          foundation_revisions:
                            detail.value.foundation_revisions,
                          persona_switch_history:
                            detail.value.persona_switch_history,
                          persona_compilation: detail.value.persona_compilation,
                          working_personas: detail.value.working_personas,
                        },
                        null,
                        2,
                      ),
                    )}
                  </pre>
                </Disclosure>
              ) : null}
            </div>
          )}
        </Surface>
        <p className="character-more">
          <small>{"更多："}</small>
          <ActionButton variant="ghost" asChild >
            <a  href={governance("capabilities")}>
              {"能力申请"}
            </a>
          </ActionButton>
          <span>{"·"}</span>
          <ActionButton variant="ghost" asChild >
            <a  href={governance("developing")}>
              {"发展中的自我"}
            </a>
          </ActionButton>
          <span>{"·"}</span>
          <ActionButton variant="ghost" asChild >
            <a  href={governance("foundation")}>
              {"身份核心与提议"}
            </a>
          </ActionButton>
        </p>
        {!editing.value ? (
          <ActionFeedback key={0} labels={["角色详情"]} />
        ) : null}
      </div>
    );
  };
return {w,fid,detail,world,editing,fields,saving,readbackPending,error,conflict,personaRecord,personaSubmitted,personaReadbackPending,personaFeedback,switching,target,reason,confirm,snapshot,core,personas,currentPersona,sections,personaValue,title,subtitle,avatar,governance,read,edit,save,rebase,switchPersona,readPersonaOutcome,recoverPreparation,render};
}
function createCharacterDetailPageController(props,context,lifecycle){return createProfileModel(lifecycle).render;}

export default function CharacterDetailPage(input = {}) {
  return useController(createCharacterDetailPageController, {
    ...input,
  });
}
