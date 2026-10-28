import { Wand2, BookOpen, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Surface,
  Badge,
  Label,
  ChoiceSelect,
  FieldInput,
  Textarea,
  Disclosure,
} from "../controls";
import WorkspaceInfo from "../shared/WorkspaceInfo";
import WorkspaceDialog from "../shared/WorkspaceDialog";
import LoadingState from "../shared/LoadingState";
import { InlineFeedback } from "../shared/Presentation";
import JsonDetails from "../shared/JsonDetails";
import MediaPreview from "../shared/MediaPreview";
import CursorControls from "../shared/CursorControls";
import MaterialRetention from "../shared/MaterialRetention";
import StudioMaterialPage from "../pages/StudioMaterialPage";
import SemanticTree from "../shared/SemanticTree";
import { StoryMark } from "../shared/WorkspaceArtwork";
import { studioJobNames } from "@/approved-ui/studio-presentation.mjs";
import {
  referenceFromGrant,
  mediaReferencePurposes,
} from "@/v3/proto/components/media-reference-grants.js";
import { stateLabel, errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
const jobStatusLabel = (status) => status === "discarded" ? "已丢弃" : stateLabel(status);
export default function StudioJobsView({ model: m }) {
  const {
    ui,
    job,
    pending,
    busy,
    reading,
    error,
    detailError,
    notice,
    jobs,
    materials,
    grants,
    service,
    resource,
    out,
    comparison,
    proposal,
  } = m;
  const complete = job?.status === "completed",
    running = ["queued", "running"].includes(job?.status);
  return (
    <div className="r-sandbox-layout">
      <div className="r-form-stack">
        <Surface className="panel r-padded r-form-stack">
          <div>
            <h2 className="r-inline-title">试着丰富这个想法</h2>
            <p className="r-muted">
              当前工作草稿 · {m.readDraft()?.name || "未命名资源"}
            </p>
          </div>
          {[error, detailError].filter(Boolean).map((text) => (
            <InlineFeedback key={text} role="alert">
              {text}
            </InlineFeedback>
          ))}
          {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
          {ui ? (
            <form
              className="r-form-stack"
              onSubmit={(event) => {
                event.preventDefault();
                void m.create();
              }}
            >
              <Label htmlFor="studio-job-kind">
                创作方式
                <ChoiceSelect
                  id="studio-job-kind"
                  aria-label="创作方式"
                  value={ui.kind}
                  onChange={(event) => m.edit({ kind: event.target.value })}
                >
                  {Object.entries(studioJobNames).map(([kind, label]) => (
                    <option key={kind} value={kind}>
                      {label}
                    </option>
                  ))}
                </ChoiceSelect>
              </Label>
              <Label htmlFor="studio-job-instruction">
                创作请求
                <Textarea
                  id="studio-job-instruction"
                  aria-label="创作请求"
                  rows={5}
                  value={ui.instruction}
                  onChange={(event) =>
                    m.edit({ instruction: event.target.value })
                  }
                />
              </Label>
              {ui.kind === "image_preview" && (
                <>
                  <Label htmlFor="studio-job-reference">
                    图片参考
                    <ChoiceSelect
                      id="studio-job-reference"
                      aria-label="图片参考"
                      value={m.selectedReference}
                      onChange={(event) => {
                        const value = event.target.value,
                          grant = value.startsWith("grant:")
                            ? grants.items.find(
                                (entry) => entry.id === value.slice(6),
                              )
                            : null;
                        try {
                          m.edit({
                            referenceGrant: grant
                              ? referenceFromGrant(grant)
                              : null,
                            referenceAssetId: grant ? grant.asset_id : value,
                            referencePurpose: grant
                              ? grant.purpose
                              : ui.referencePurpose,
                          });
                        } catch (cause) {
                          m.setError(errorMessage(cause));
                        }
                      }}
                    >
                      {m.references.map((entry) => (
                        <option key={entry.value} value={entry.value}>
                          {entry.label}
                        </option>
                      ))}
                    </ChoiceSelect>
                  </Label>
                  <Label>
                    参考用途
                    <ChoiceSelect
                      aria-label="参考用途"
                      value={ui.referenceGrant?.purpose || ui.referencePurpose}
                      disabled={!!ui.referenceGrant}
                      onChange={(event) =>
                        m.edit({ referencePurpose: event.target.value })
                      }
                    >
                      <option value="">请选择</option>
                      {Object.entries(mediaReferencePurposes)
                        .filter(
                          ([key]) => !!ui.referenceGrant || key !== "person",
                        )
                        .map(([key, label]) => (
                          <option key={key} value={key}>
                            {label}
                          </option>
                        ))}
                    </ChoiceSelect>
                  </Label>
                  <CursorControls page={grants} label="参考授权" />
                </>
              )}
              <Button
                className="r-full"
                type="submit"
                disabled={
                  busy ||
                  !resource ||
                  resource.status === "archived" ||
                  !!pending ||
                  !!m.receipt ||
                  !ui.instruction.trim()
                }
              >
                <Wand2 size={15} />
                开始创作
              </Button>
              {!resource && (
                <WorkspaceInfo>
                  先保存这份资源，再开始一次创作尝试。
                </WorkspaceInfo>
              )}
            </form>
          ) : (
            <InlineFeedback role="alert">
              创作面板草稿无法读取，原记录仍保留。
            </InlineFeedback>
          )}
          {pending && (
            <WorkspaceInfo>
              原任务提交尚未确认，当前新输入单独保留。
              <JsonDetails value={pending.value} title="原任务提交" />
              {pending.rejection ? (
                <Button
                  variant="outline"
                  disabled={busy}
                  onClick={m.releaseRejected}
                >
                  核对后解除已拒绝的原提交
                </Button>
              ) : (
                <Button
                  variant="outline"
                  disabled={busy}
                  onClick={() => void m.submit(pending.value, true)}
                >
                  重试原任务提交
                </Button>
              )}
            </WorkspaceInfo>
          )}
          {m.receipt && (
            <WorkspaceInfo>
              原任务操作已有回执，继续读取实际结果。
              <Button variant="outline" disabled={busy} onClick={m.readReceipt}>
                恢复原任务结果读取
              </Button>
            </WorkspaceInfo>
          )}
          <JsonDetails value={m.readDraft()} title="当前工作草稿完整输入" />
        </Surface>
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">创作记录</h2>
          {jobs.loading && <LoadingState label="正在读取创作记录…" />}
          {jobs.items.map((entry) => (
            <button
              key={entry.id}
              className={
                "r-job-row" + (ui?.selected === entry.id ? " active" : "")
              }
              onClick={() => m.select(entry.id)}
            >
              <span>
                <b>{studioJobNames[entry.kind] || "创作任务"}</b>
                <small>
                  {entry.created_at
                    ? userInstant(entry.created_at)
                    : "时间未记录"}
                </small>
              </span>
              <Badge variant="outline" data-status={entry.status}>
                {entry.result?.disposition === "needs_input"
                  ? "需要补充输入"
                  : jobStatusLabel(entry.status)}
              </Badge>
            </button>
          ))}
          {!jobs.items.length && !jobs.loading && !jobs.error && (
            <p className="r-muted">第一次尝试会从这里开始。</p>
          )}
          <CursorControls page={jobs} label="创作任务" />
        </Surface>
      </div>
      <div className="r-form-stack">
        <Surface className="panel r-padded r-sandbox-result">
          <div className="r-section-heading">
            <h2>预览与建议</h2>
            {job && (
              <Badge variant="outline" data-status={job.status}>
                {out.disposition === "needs_input"
                  ? "需要补充输入"
                  : jobStatusLabel(job.status)}
              </Badge>
            )}
            <Button
              variant="ghost"
              disabled={busy || reading}
              onClick={() => void m.refresh()}
            >
              刷新任务状态
            </Button>
          </div>
          {!job ? (
            <div className="studio-result-empty">
              {reading ? (
                <LoadingState label="正在读取原创作任务…" />
              ) : (
                <>
                  <StoryMark />
                  <h3>想象正在等待一个开始</h3>
                  <p>写下方向，看看这份设定还能长出什么。</p>
                </>
              )}
            </div>
          ) : running ? (
            <div className="r-generation-loading">
              <StoryMark />
              <Sparkles size={24} />
              <h3>
                {job.status === "queued"
                  ? "正在等待实际执行"
                  : "正在展开这段想象"}
              </h3>
              <p>当前草稿完整保留，按原任务读取实际结果。</p>
              <Button
                variant="outline"
                disabled={reading}
                onClick={() => void m.observe()}
              >
                读取原任务进度
              </Button>
            </div>
          ) : job.status === "discarded" ? (
            <div className="studio-result-empty">
              <h3>这次结果已丢弃</h3>
              <p>已保存的素材和来源仍保留，可以换一个方向再试。</p>
            </div>
          ) : job.status === "failed" ? (
            <div className="studio-result-empty">
              <h3>这次尝试没有完成</h3>
              <p>
                {m.jobError[job.error_code] ||
                  errorMessage(job.error_code, "请查看原执行状态后重试。")}
              </p>
              <Button
                variant="outline"
                disabled={busy || !!pending || !!m.receipt}
                onClick={() => void m.operate("retry")}
              >
                重试这个任务
              </Button>
            </div>
          ) : (
            <div className="r-form-stack">
              {out.reply && <p className="r-prose">{out.reply}</p>}
              {out.disposition === "needs_input" && (
                <WorkspaceInfo>
                  需要补充输入。当前工作草稿保持，补充后可以明确发起新任务。
                </WorkspaceInfo>
              )}
              {job.kind === "text_preview" && out.text && (
                <div className="r-story-preview">
                  <span className="r-tiny-label">FICTIONAL TEXT PREVIEW</span>
                  <p>{out.text}</p>
                </div>
              )}
              {job.kind === "image_preview" && out.asset_id && (
                <MediaPreview
                  src={service.mediaUrl(out.asset_id)}
                  alt="实际创作图片预览，非生活事实"
                  className="r-sandbox-art"
                />
              )}
              {job.kind === "image_preview" && !out.asset_id && (
                <p className="r-muted">
                  尚未读到已完成的图片，任务结束与实际资产分别核对。
                </p>
              )}
              {comparison && (
                <>
                  <WorkspaceInfo>
                    已生成可供审阅的建议，尚未改动工作草稿。
                  </WorkspaceInfo>
                  <div className="r-proposal-preview">
                    <span className="r-tiny-label">建议补充</span>
                    <SemanticTree value={proposal.body} />
                  </div>
                  <Button
                    disabled={m.adopting || !comparison.changed.length}
                    onClick={() => void m.adoptProposal()}
                  >
                    核对并采用到草稿
                  </Button>
                  <Disclosure
                    title={"完整改变 · " + comparison.changed.length + " 项"}
                  >
                    <SemanticTree value={comparison.changed} />
                  </Disclosure>
                </>
              )}
              <div className="r-inline-actions">
                {complete &&
                  job.kind !== "assistant" &&
                  (job.kind === "text_preview" || out.asset_id) && (
                    <Button
                      variant="outline"
                      disabled={busy || !!pending || !!m.receipt}
                      onClick={() => void m.operate("save-material")}
                    >
                      存为资源素材
                    </Button>
                  )}
                <Button
                  variant="ghost"
                  disabled={busy || !!pending || !!m.receipt}
                  onClick={() => void m.operate("discard")}
                >
                  丢弃结果
                </Button>
              </div>
              <JsonDetails value={job.snapshot} title="这次创作的冻结输入" />
            </div>
          )}
          {job && (
            <>
              <JsonDetails
                value={job.attempts || []}
                title="实际执行尝试与工作流状态"
              />
              <JsonDetails value={job} title="任务永久标识与完整记录" />
            </>
          )}
        </Surface>
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">已保存的创作素材</h2>
          {ui && (
            <Label className="r-inline-check">
              <FieldInput
                type="checkbox"
                checked={!!ui.materialReleased}
                onChange={(event) =>
                  m.edit({ materialReleased: event.target.checked })
                }
              />
              查看已取消保留
            </Label>
          )}
          <div className="r-material-grid">
            {materials.items.map((item) => (
              <div className="r-material-card" key={item.id}>
                <button
                  aria-label={"查看创作素材 " + (item.label || "已有素材")}
                  onClick={() => m.setMaterialId(item.id)}
                >
                  {item.asset_id ? (
                    <MediaPreview
                      src={service.mediaUrl(item.asset_id)}
                      alt="已保存创作素材"
                      className="studio-material-thumb"
                    />
                  ) : (
                    <div className="r-material-text">
                      <BookOpen size={24} />
                      <p>{item.text?.slice(0, 60) || "文字内容待读取"}</p>
                    </div>
                  )}
                  <b>{item.label || "创作素材"}</b>
                  <small>
                    {item.released_at ? "已取消保留" : "保留中"} ·{" "}
                    {item.created_at
                      ? userInstant(item.created_at)
                      : "时间未记录"}
                  </small>
                </button>
                <div className="r-inline-actions">
                  <Button variant="ghost" onClick={() => m.select(item.job_id)}>
                    原任务
                  </Button>
                  <Button variant="ghost" onClick={() => m.setRetention(item)}>
                    {item.released_at ? "恢复保留" : "取消保留"}
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {!materials.items.length &&
            !materials.loading &&
            !materials.error && (
              <p className="r-muted">喜欢的文字或画面，可以单独留下来。</p>
            )}
          <CursorControls page={materials} label="创作素材" />
        </Surface>
      </div>
      {m.retention && (
        <MaterialRetention
          material={m.retention}
          open
          onClose={() => m.setRetention(null)}
          onSaved={() => materials.reload()}
        />
      )}
      <WorkspaceDialog
        open={!!m.materialId}
        onOpenChange={(open) => {
          if (!open) m.setMaterialId("");
        }}
        title="创作素材详情"
        description="保留原任务、来源和状态修订"
        className="replica-studio"
      >
        <StudioMaterialPage id={m.materialId} embedded />
      </WorkspaceDialog>
    </div>
  );
}
