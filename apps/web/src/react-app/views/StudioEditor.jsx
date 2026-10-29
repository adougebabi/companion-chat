import {
  ArrowLeft,
  ChevronRight,
  Check,
  Wand2,
  Pencil,
  History,
  ArrowUpRight,
  ArrowRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Surface,
  Badge,
  Label,
  FieldInput,
  Textarea,
  ChoiceSelect,
  Disclosure,
} from "../controls";
import WorkspacePageHeader from "../shared/WorkspacePageHeader";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import LoadingState from "../shared/LoadingState";
import { InlineFeedback } from "../shared/Presentation";
import StudioDefinitionForm from "../shared/StudioDefinitionForm";
import StudioLifecycle from "../shared/StudioLifecycle";
import StudioApply from "../shared/StudioApply";
import JsonDetails from "../shared/JsonDetails";
import {
  studioKindNames,
  studioVersion,
  studioTargetHref,
} from "@/approved-ui/studio-presentation.mjs";
export default function StudioEditorView({ model: m }) {
  const { w, controller: c, state: s, d, r, readOnly, changes, jobs } = m;
  const creating = m.resourceId === "new";
  return (
    <div className="replica-studio resource-page" data-testid="studio-editor">
      <div className="r-breadcrumb">
        <Button variant="ghost" onClick={() => w.navigatePath("/studio")}>
          <ArrowLeft size={14} />
          创作中心
        </Button>
        <ChevronRight size={12} />
        <span>
          {creating
            ? "新建" + (studioKindNames[d?.kind || m.kind] || "资源")
            : r?.name || "资源"}
        </span>
      </div>
      <WorkspacePageHeader
        eyebrow={
          m.sandbox
            ? "STUDIO / A SMALL EXPERIMENT"
            : "STUDIO / MAKE IT YOUR OWN"
        }
        title={
          m.sandbox
            ? "创作沙盒"
            : creating
              ? "新的" + (studioKindNames[d?.kind || m.kind] || "资源")
              : d?.name || r?.name || "编辑资源"
        }
        description={
          m.sandbox
            ? "给想象一点空间。预览与角色生活分别保存。"
            : "把想法慢慢补全，保存，再选择何时使用。"
        }
        actions={
          <>
            <Button variant="outline" onClick={m.changeMode}>
              {m.sandbox ? <Pencil size={14} /> : <Wand2 size={14} />}{" "}
              {m.sandbox ? "返回编辑" : "创作沙盒"}
            </Button>
            {!m.sandbox && (
              <Button
                disabled={
                  !d || readOnly || s.busy || !!s.pending || s.readbackPending
                }
                onClick={m.save}
              >
                <Check size={15} />
                保存草稿
              </Button>
            )}
          </>
        }
      />
      {s.error && (
        <InlineFeedback role="alert">
          {s.error}
          <Button
            variant="outline"
            disabled={s.loading || s.busy}
            onClick={() => void c.load()}
          >
            重新读取资源并保留草稿
          </Button>
        </InlineFeedback>
      )}
      {s.notice && <WorkspaceInfo>{s.notice}</WorkspaceInfo>}
      {s.loading && !d && <LoadingState label="正在读取创作资源…" />}
      {d && (
        <>
          {s.pending && (
            <WorkspaceInfo>
              原提交结果尚未确认，当前工作草稿单独保留。
              {!s.pending.rejection && (
                <Button
                  disabled={s.busy}
                  variant="outline"
                  onClick={() => void c.retry()}
                >
                  重试原提交
                </Button>
              )}
              <JsonDetails value={s.pending.value} title="原提交内容" />
            </WorkspaceInfo>
          )}
          {s.readbackPending && (
            <WorkspaceInfo>
              原操作已有回执，继续读取实际结果。
              <Button
                variant="outline"
                disabled={s.busy}
                onClick={() => void c.load()}
              >
                恢复资源结果读取
              </Button>
            </WorkspaceInfo>
          )}
          {m.sandbox ? (
            <>{jobs}</>
          ) : (
            <div className="r-studio-editor">
              <div className="r-form-stack">
                <Surface className="panel r-padded">
                  <div className="r-section-heading">
                    <h2>资源设定</h2>
                    <Badge
                      variant="outline"
                      data-status={
                        readOnly
                          ? "archived"
                          : c.dirty()
                            ? "pending"
                            : "completed"
                      }
                    >
                      {readOnly
                        ? "已归档"
                        : c.dirty()
                          ? "有未保存修改"
                          : "已保存"}
                    </Badge>
                  </div>
                  <form
                    ref={m.formRef}
                    onSubmit={(event) => {
                      event.preventDefault();
                      void c.save();
                    }}
                  >
                    <fieldset disabled={readOnly} className="r-form-stack">
                      <Label htmlFor="resource-name">
                        资源名称 <span className="replica-required" aria-hidden="true">*</span>
                        <FieldInput
                          id="resource-name"
                          required
                          aria-label="资源名称"
                          value={d.name}
                          onChange={(event) =>
                            c.edit({ name: event.target.value })
                          }
                        />
                      </Label>
                      {creating && (
                        <Label htmlFor="resource-kind">
                          资源类型
                          <ChoiceSelect
                            id="resource-kind"
                            aria-label="资源类型"
                            value={d.kind}
                            disabled={s.busy || !!s.pending}
                            onChange={(event) => c.setKind(event.target.value)}
                          >
                            {Object.entries(studioKindNames).map(
                              ([kind, label]) => (
                                <option key={kind} value={kind}>
                                  {label}
                                </option>
                              ),
                            )}
                          </ChoiceSelect>
                        </Label>
                      )}
                      <StudioDefinitionForm
                        key={d.kind + ":" + s.definitionEpoch}
                        kind={d.kind}
                        value={d.body}
                        disabled={readOnly || d.bodyText !== undefined}
                        onChange={(body) => c.edit({ body })}
                      />
                      <Label htmlFor="resource-reason">
                        保存原因 <span className="replica-required" aria-hidden="true">*</span>
                        <FieldInput
                          id="resource-reason"
                          aria-label="保存原因"
                          required
                          value={d.reason}
                          onChange={(event) =>
                            c.edit({ reason: event.target.value })
                          }
                        />
                      </Label>
                      <Disclosure title="完整 JSON 定义">
                        <p className="r-fineprint">
                          完整保留已有字段。尚未核对的输入独立于此前保存、发布和应用。
                        </p>
                        <Textarea
                          aria-label="完整 JSON 定义"
                          className="r-code-input"
                          value={d.bodyText ?? JSON.stringify(d.body, null, 2)}
                          onChange={(event) =>
                            c.edit({ bodyText: event.target.value })
                          }
                        />
                        {d.bodyText !== undefined && (
                          <div className="r-inline-actions">
                            <Button
                              variant="outline"
                              onClick={() => c.parseBodyText()}
                            >
                              核对并采用 JSON 输入
                            </Button>
                            <Button
                              variant="ghost"
                              onClick={() => c.edit({ bodyText: undefined })}
                            >
                              撤销 JSON 输入
                            </Button>
                          </div>
                        )}
                      </Disclosure>
                    </fieldset>
                  </form>
                </Surface>
                {r && (
                  <Surface className="panel r-padded">
                    <h2 className="r-inline-title">最近的使用</h2>
                    {(r.referenced_by || []).map((entry, index) => (
                      <div className="r-history-line" key={entry.id || index}>
                        <Check size={15} />
                        <span>
                          <b>
                            {w.state.characters.find(
                              (actor) => actor.id === entry.fluctlight_id,
                            )?.name || "目标记录"}{" "}
                            ·{" "}
                            {entry.target_kind === "foundation"
                              ? entry.currently_effective
                                ? "当前身份来源"
                                : entry.status === "proposed"
                                  ? "身份提议待审阅"
                                  : "身份来源记录"
                              : "已有关联来源"}
                          </b>
                          <small>
                            {studioVersion(entry.running_version, "版本待读取")}
                          </small>
                          {entry.fluctlight_id && (
                            <a
                              href={studioTargetHref(
                                r.kind,
                                entry.fluctlight_id,
                                {
                                  target_kind: entry.target_kind,
                                  proposal_id:
                                    entry.target_kind === "foundation"
                                      ? entry.runtime_id
                                      : undefined,
                                },
                              )}
                            >
                              查看实际记录
                            </a>
                          )}
                        </span>
                      </div>
                    ))}
                    {!r.referenced_by?.length && (
                      <p className="r-muted">
                        尚未读到运行引用。发布、应用和实际使用分别记录。
                      </p>
                    )}
                    <JsonDetails
                      value={changes}
                      title="保存版本与当前工作草稿的完整差异"
                    />
                  </Surface>
                )}
              </div>
              <div className="r-form-stack">
                <Surface className="panel r-padded r-sticky-panel">
                  <h2 className="r-inline-title">保存与发布</h2>
                  <div className="studio-key-value">
                    <span>保存版本</span>
                    <b>{studioVersion(r?.draft_version)}</b>
                  </div>
                  <div className="studio-key-value">
                    <span>当前可用版本</span>
                    <b>{studioVersion(r?.published_version, "尚未发布")}</b>
                  </div>
                  <div className="r-publish-steps">
                    <span className={r ? "done" : ""}>
                      <Check size={11} />
                      保存草稿
                    </span>
                    <i />
                    <span className={r?.published_version ? "done" : ""}>
                      <Check size={11} />
                      发布版本
                    </span>
                    <i />
                    <span>用于生活</span>
                  </div>
                  <p className="r-muted">
                    {c.dirty()
                      ? "先保存当前修改，再发布明确的保存版本。"
                      : r?.draft_version === r?.published_version
                        ? "当前保存版本已经发布，可供独立应用。"
                        : "这个草稿已保存，可以发布为可用版本。"}
                  </p>
                  <Button
                    className="r-full"
                    disabled={
                      !r ||
                      readOnly ||
                      s.busy ||
                      !!s.pending ||
                      s.readbackPending ||
                      c.dirty() ||
                      r.draft_version === r.published_version
                    }
                    onClick={(event) => {
                      m.dialogTriggers.current.publish = event.currentTarget;
                      c.reviewPublish();
                    }}
                  >
                    发布已保存版本
                    <ArrowUpRight size={14} />
                  </Button>
                  {r && (
                    <Button
                      className="r-full"
                      variant="outline"
                      onClick={(event) => {
                        m.dialogTriggers.current.history = event.currentTarget;
                        m.setHistoryOpen(true);
                      }}
                    >
                      <History size={14} />
                      版本历史
                    </Button>
                  )}
                  {r?.published_version && !readOnly && (
                    <>
                      <div className="r-divider" />
                      <Button
                        className="r-full"
                        variant="outline"
                        onClick={(event) => {
                          m.dialogTriggers.current.apply = event.currentTarget;
                          m.setApplyOpen(true);
                        }}
                      >
                        应用到角色
                        <ArrowRight size={14} />
                      </Button>
                      {["world", "character_template"].includes(r.kind) && (
                        <Button
                          className="r-full"
                          onClick={() =>
                            w.navigatePath(
                              "/library/create?" +
                                (r.kind === "world"
                                  ? "worldResourceId=" +
                                    encodeURIComponent(r.id) +
                                    "&worldVersion="
                                  : "template=" +
                                    encodeURIComponent(r.id) +
                                    "&version=") +
                                encodeURIComponent(r.published_version),
                            )
                          }
                        >
                          用于创建新角色
                          <ArrowUpRight size={14} />
                        </Button>
                      )}
                    </>
                  )}
                  {r && (
                    <Button
                      className="r-full"
                      variant="ghost"
                      disabled={s.busy || (!!s.pending && !s.pending.rejection)}
                      onClick={(event) => {
                        m.dialogTriggers.current.rebase = event.currentTarget;
                        void c.reviewRebase();
                      }}
                    >
                      核对最新版本基线
                    </Button>
                  )}
                </Surface>
                {r && (
                  <StudioLifecycle
                    resource={r}
                    onChanged={m.resourceChanged}
                    historyOpen={m.historyOpen}
                    onHistoryOpenChange={m.setHistoryOpen}
                    historyReturnFocus={() => m.dialogTriggers.current.history}
                  />
                )}
              </div>
            </div>
          )}
        </>
      )}
      <WorkspaceDialog
        open={!!s.rebaseReview}
        onOpenChange={(open) => {
          if (!open) c.state.rebaseReview = null;
        }}
        title="核对最新版本基线"
        returnFocus={() => m.dialogTriggers.current.rebase}
        className="replica-studio"
        description={
          "保留工作草稿，明确采用已读取的保存版本 " +
          studioVersion(
            s.rebaseReview?.resource.draft_version ||
              s.rebaseReview?.resource.published_version,
          )
        }
        footer={
          <>
            <Button
              variant="outline"
              onClick={() => (c.state.rebaseReview = null)}
            >
              取消
            </Button>
            <Button onClick={() => c.confirmRebase()}>
              已核对，采用最新基线
            </Button>
          </>
        }
      >
        <JsonDetails value={s.rebaseReview?.resource} title="最新资源与版本" />
      </WorkspaceDialog>
      <WorkspaceDialog
        open={!!s.publishReview}
        onOpenChange={(open) => {
          if (!open) c.state.publishReview = null;
        }}
        title="发布已保存版本"
        returnFocus={() => m.dialogTriggers.current.publish}
        className="replica-studio"
        description={
          "发布「" +
          (s.publishReview?.resource.name || "") +
          "」" +
          studioVersion(s.publishReview?.resource.draft_version) +
          "。之后应用仍需单独核对。"
        }
        footer={
          <>
            <Button
              variant="outline"
              onClick={() => (c.state.publishReview = null)}
            >
              取消
            </Button>
            <Button disabled={s.busy} onClick={() => void c.confirmPublish()}>
              确认发布保存版本
            </Button>
          </>
        }
      >
        <p className="r-muted">
          只发布已经保存并明确核对的版本，当前工作草稿不能绕过保存。
        </p>
      </WorkspaceDialog>
      {d && <StudioApply model={m} />}
    </div>
  );
}
