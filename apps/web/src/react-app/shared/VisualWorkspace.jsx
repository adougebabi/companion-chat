import { useEffect, useRef, useState } from "react";
import { StoryMark } from "./WorkspaceArtwork";
import { Pencil, Wand2, ImageIcon, History } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Surface,
  Badge,
  Disclosure,
  Textarea,
  FieldInput,
} from "../controls";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import useWorkspaceRead from "./useWorkspaceRead";
import useMediaFollow from "./useMediaFollow";
import { MediaReader, MediaMore, MediaById } from "./MediaParts";
import {
  briefFields,
  visualSessionState,
  requiredText,
} from "@/approved-ui/media-presentation.mjs";
import { studioDraftStorage } from "@/v3/proto/components/studio-draft-storage.js";
import { stateLabel, errorMessage } from "@/v3/domain-presentation.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
export default function VisualWorkspace({ model: m }) {
  const fid = m.target,
    [action, setAction] = useState(null),
    [historyOpen, setHistoryOpen] = useState(false),
    [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [receipt, setReceipt] = useState(null),
    trigger = useRef(null),
    identityRef = useRef(null),
    briefRef = useRef(null),
    startedEdit = useRef(false);
  const prefix = `fluctlight-media:${m.owner}:${fid}:replica-visual`,
    client = m.w.client;
  const readIdentity = async () => {
    const r = await client.visualIdentityManagement(fid);
    if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.current_revision))
      throw Error("视觉身份范围或版本不匹配。");
    return r;
  };
  const readBrief = async () => {
    const r = await client.visualBrief(fid);
    if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.revision))
      throw Error("视觉定义范围或版本不匹配。");
    return r;
  };
  const identity = useMediaFollow({
      identity: ["visual-identity", fid],
      read: readIdentity,
      pending: (r) => {
        const sheet = r.generated_reviews?.character_sheet;
        const visualPending = ["queued", "running"].includes(r.session?.status) ||
          (r.session?.status === "character_sheet_pending" && !["awaiting_review", "failed"].includes(sheet?.status));
        return visualPending || Object.values(r.generated_reviews || {}).some((row) => row?.status === "generating");
      },
      guard: (old, row) => old.session?.id === row.session?.id,
    }),
    brief = useWorkspaceRead({
      identity: ["visual-brief", fid],
      characterScoped: false,
      read: async () => ({ items: [await readBrief()], next_cursor: "" }),
    }),
    timeline = useWorkspaceRead({
      identity: ["visual-timeline", fid],
      characterScoped: false,
      read: (cursor) => client.visualIdentityTimeline(fid, cursor),
    }),
    history = useWorkspaceRead({
      identity: ["visual-brief-history", fid],
      characterScoped: false,
      enabled: historyOpen,
      read: (cursor) => client.visualBriefHistory(fid, cursor),
    });
  identityRef.current = identity;
  briefRef.current = brief;
  const visual = identity.items[0],
    definition = brief.items[0],
    session = visual?.session,
    current = session?.attempts?.find(
      (a) => a.attempt_number === session.current_attempt,
    ),
    candidate = current?.candidate_asset_id,
    canReview =
      session?.status === "awaiting_review" && candidate && !current.decision;
  useEffect(() => {
    try {
      const r = JSON.parse(
        studioDraftStorage().getItem(prefix + ":last") || "null",
      );
      if (r?.requestId) setReceipt(r);
    } catch {}
  }, [prefix]);
  useEffect(() => {
    if (m.initialEdit && definition && !startedEdit.current && !m.readOnly) {
      startedEdit.current = true;
      editBrief();
    }
  }, [definition, m.initialEdit]);
  function base(key) {
    return {
      identity: "visual:" + fid + ":" + key,
      className: "replica-media",
      scope: ["visual", fid, key],
      storageKey: prefix + ":" + key,
    };
  }
  function editBrief(event) {
    if (!definition) return;
    m.assertWritable();
    trigger.current = event?.currentTarget;
    const values = Object.fromEntries(
      briefFields.map(([k]) => [k, definition.brief?.[k] ?? ""]),
    );
    setAction({
      ...base("brief"),
      title: "视觉定义",
      description: "参考约束用于下一次生成，当前规范参考保持原样。",
      submitLabel: "保存视觉定义",
      preAdmissionConflicts: ["visual_brief_conflict", "revision_conflict"],
      initial: {
        ...values,
        reason: "",
        revision: definition.revision,
        baseline: values,
      },
      validate: (d) => {
        requiredText(d.reason, "保存原因", 1024);
        for (const [k, label] of briefFields)
          if (typeof d[k] !== "string" || [...d[k]].length > 1000)
            throw Error(label + "最多1000个字符。");
      },
      build: (d) => ({
        ...Object.fromEntries(briefFields.map(([k]) => [k, d[k]])),
        expectedRevision: d.revision,
        reason: d.reason,
      }),
      execute: async (body, key) => {
        m.assertWritable();
        const r = await client.saveVisualBrief(fid, {
          ...body,
          idempotencyKey: key,
        });
        if (r.fluctlight_id !== fid || !Number.isSafeInteger(r.revision))
          throw Error("视觉定义回执尚未完整确认。");
        return r;
      },
      readLatest: readBrief,
      adoptLatest: (d, row) => {
        const latest = Object.fromEntries(
          briefFields.map(([k]) => [k, row.brief?.[k] ?? ""]),
        );
        return {
          ...d,
          ...Object.fromEntries(
            briefFields.map(([k]) => [
              k,
              d[k] === d.baseline[k] ? latest[k] : d[k],
            ]),
          ),
          baseline: latest,
          revision: row.revision,
        };
      },
      onSaved: async (r, body) => {
        const fresh = await readBrief();
        if (
          fresh.revision < r.revision ||
          (fresh.revision === r.revision &&
            briefFields.some(([k]) => fresh.brief?.[k] !== body[k]))
        )
          throw Error("视觉定义的新版本尚未读回。");
        if (m.live()) {
          briefRef.current.adopt({ items: [fresh], next_cursor: "" });
          setNotice("视觉定义已保存并读回；规范参考需单独生成和接受。");
          if (historyOpen) await history.load();
        }
      },
      fields: (d, edit) => (
        <div className="r-form-stack">
          {briefFields.map(([k, label]) => (
            <label className="replica-field" key={k}>
              <span>{label}</span>
              <Textarea
                aria-label={label}
                value={d[k]}
                onChange={(e) => edit({ [k]: e.target.value })}
              />
            </label>
          ))}
          <label className="replica-field">
            <span>保存原因</span>
            <FieldInput
              aria-label="保存原因"
              value={d.reason}
              onChange={(e) => edit({ reason: e.target.value })}
            />
          </label>
        </div>
      ),
    });
  }
  async function generate(event) {
    m.assertWritable();
    trigger.current = event.currentTarget;
    setError("");
    try {
      const [v, b] = await Promise.all([readIdentity(), readBrief()]);
      if (!m.live()) return;
      setAction({
        ...base("generate"),
        title: "生成新的视觉候选",
        description:
          "使用已保存的视觉定义。已有规范参考保留；已有会话占用时，新请求可能先排队。",
        submitLabel: "开始准备候选",
        preAdmissionConflicts: [
          "visual_generation_conflict",
          "revision_conflict",
        ],
        initial: {
          visualRevision: v.current_revision,
          briefRevision: b.revision,
          reason: "",
        },
        validate: (d) => requiredText(d.reason, "生成原因", 1024),
        build: (d) => ({
          expectedVisualRevision: d.visualRevision,
          expectedBriefRevision: d.briefRevision,
          reason: d.reason,
        }),
        execute: async (body, key) => {
          m.assertWritable();
          const r = await client.generateVisualIdentity(fid, {
            ...body,
            idempotencyKey: key,
          });
          if (
            r.fluctlight_id !== fid ||
            !r.request_id ||
            r.request?.id !== r.request_id ||
            !["pending", "queued"].includes(r.status)
          )
            throw Error("视觉准备请求尚未返回完整受理凭据。");
          return r;
        },
        readLatest: async () => ({
          visual: await readIdentity(),
          brief: await readBrief(),
        }),
        adoptLatest: (d, row) => ({
          ...d,
          visualRevision: row.visual.current_revision,
          briefRevision: row.brief.revision,
        }),
        onSaved: async (r) => {
          const fresh = await readIdentity();
          if (m.live()) {
            const pointer = {
              requestId: r.request_id,
              sessionId: r.session_id || null,
              taskId: r.task_id || null,
              acceptedStatus: r.status,
            };
            studioDraftStorage().setItem(
              prefix + ":last",
              JSON.stringify(pointer),
            );
            setReceipt(pointer);
            identityRef.current.adopt({ items: [fresh], next_cursor: "" });
            setNotice(
              r.status === "pending"
                ? "视觉准备请求已受理，受理时正在等待可用会话。"
                : "视觉准备请求已受理，后续候选和评审以实际状态显示。",
            );
            await timeline.load();
          }
        },
        fields: (d, edit) => (
          <div className="r-form-stack">
            <p>
              已保存定义 v{d.briefRevision} · 当前规范版本 {d.visualRevision}
            </p>
            <label className="replica-field">
              <span>生成原因</span>
              <Textarea
                aria-label="生成原因"
                value={d.reason}
                onChange={(e) => edit({ reason: e.target.value })}
              />
            </label>
          </div>
        ),
      });
    } catch (cause) {
      if (m.live()) setError(errorMessage(cause));
    }
  }
  function review(event, restore = null) {
    if (!restore && !canReview) return;
    if (restore) m.assertCurrent();
    else m.assertWritable();
    trigger.current = event.currentTarget;
    const attempt = restore?.attempt ?? session.current_attempt,
      sessionId = restore?.sessionId || session.id,
      visualRevision = restore?.visualRevision ?? visual.current_revision,
      candidate = restore?.candidate || current.candidate_asset_id,
      attemptId = restore?.attemptId || current?.id;
    studioDraftStorage().setItem(
      prefix + ":review-context:" + sessionId + ":" + attempt,
      JSON.stringify({
        attempt,
        sessionId,
        visualRevision,
        candidate,
        attemptId,
      }),
    );
    setAction({
      ...base("review:" + sessionId + ":" + attempt),
      title: "评审这次视觉候选",
      description:
        "查看候选后选择通过或不通过，结果会记录为人工评审。",
      wide: true,
      submitLabel: "提交人工评审",
      disabled: m.readOnly,
      preAdmissionConflicts: ["visual_review_conflict", "revision_conflict"],
      initial: {
        decision: "",
        identityMatch: null,
        confidence: null,
        observations: "",
        missingSections: "",
        summary: "",
        feedback: "",
        reason: "",
        visualRevision,
        attempt,
        reviewed: false,
      },
      canSubmit: (d) => !d?.reviewed,
      validate: (d) => {
        if (d.reviewed) throw Error("原候选已经评审，请重新读取下一阶段。");
        if (!["accepted", "regenerate"].includes(d.decision))
          throw Error("请选择通过或不通过。");
        if (d.decision === "accepted") return;
        if (!d.feedback?.trim())
          throw Error("请说明图片哪里不符合，以及希望怎样修改。");
        for (const k of ["identityMatch", "confidence"])
          if (d[k] != null && (typeof d[k] !== "number" || !Number.isFinite(d[k]) || d[k] < 0 || d[k] > 1))
            throw Error("评审程度须在0—100%之间。");
        const lines = (v) =>
            (v || "")
              .split("\n")
              .map((s) => s.trim())
              .filter(Boolean),
          obs = lines(d.observations),
          missing = lines(d.missingSections);
        if (
          obs.length > 24 ||
          obs.some((s) => [...s].length > 500)
        )
          throw Error("实际观察最多24项，每项最多500个字符。");
        if (missing.length > 16 || missing.some((s) => [...s].length > 200))
          throw Error("缺失分区最多16项，每项200个字符。");
        if ([...(d.summary || "")].length > 1000)
          throw Error("评审摘要最多1000个字符。");
        if ([...(d.reason || "")].length > 1024)
          throw Error("操作理由最多1024个字符。");
        if ([...d.feedback].length > 2000)
          throw Error("不通过的原因最多2000个字符。");
      },
      build: (d) => {
        const body = {
          decision: d.decision,
          expectedAttempt: d.attempt,
          expectedVisualRevision: d.visualRevision,
        };
        if (d.decision === "regenerate") {
          body.feedback = d.feedback;
          for (const key of ["identityMatch", "confidence"])
            if (d[key] != null) body[key] = d[key];
          for (const key of ["observations", "missingSections"]) {
            const lines = (d[key] || "").split("\n").map((s) => s.trim()).filter(Boolean);
            if (lines.length) body[key] = lines;
          }
          for (const key of ["summary", "reason"])
            if (d[key]?.trim()) body[key] = d[key];
        }
        return body;
      },
      execute: async (body, key) => {
        m.assertWritable();
        const r = await client.reviewVisualIdentity(fid, sessionId, {
          ...body,
          idempotencyKey: key,
        });
        if (
          r.fluctlight_id !== fid ||
          r.session_id !== sessionId ||
          !r.operation_id ||
          r.result?.session_id !== sessionId ||
          r.result?.attempt !== body.expectedAttempt ||
          r.result?.decision !== body.decision ||
          (body.decision === "accepted" &&
            r.result.canonical_asset_id !== candidate)
        )
          throw Error("原候选人工评审回执尚未完整确认。");
        return r;
      },
      afterAcknowledged: (d) => ({ ...d, reviewed: true }),
      readLatest: readIdentity,
      adoptLatest: (d, row) => {
        if (
          row.session?.id !== sessionId ||
          row.session.current_attempt !== attempt ||
          row.session.attempts?.find((a) => a.attempt_number === attempt)
            ?.decision
        )
          throw Error("原候选已变化或已评审，请保留输入并重新读取下一阶段。");
        return { ...d, visualRevision: row.current_revision };
      },
      onSaved: async (r, body) => {
        const fresh = await readIdentity();
        if (body.decision === "accepted") {
          if (
            fresh.current_revision < body.expectedVisualRevision + 1 ||
            (fresh.current_revision === body.expectedVisualRevision + 1 &&
              fresh.canonical_asset_id !== candidate)
          )
            throw Error("接受后的规范参考尚未读回。");
        } else {
          const prior = fresh.session?.attempts?.find(
            (a) => a.attempt_number === attempt,
          );
          if (
            fresh.session?.id !== sessionId ||
            prior?.decision !== "regenerate"
          ) {
            let cursor = "",
              found = false;
            const seen = new Set();
            do {
              if (seen.has(cursor))
                throw Error("视觉记录分页位置重复，请核对原记录。");
              seen.add(cursor);
              const page = await client.visualIdentityTimeline(fid, cursor);
              found = page.items.some(
                (row) =>
                  row.session_id === sessionId &&
                  row.attempt_id === attemptId &&
                  row.stage === "regenerate",
              );
              cursor = page.next_cursor || "";
            } while (!found && cursor);
            if (!found) throw Error("原重新生成决定尚未读回。");
          }
        }
        if (m.live()) {
          identityRef.current.adopt({ items: [fresh], next_cursor: "" });
          setNotice(
            body.decision === "accepted"
              ? fresh.current_revision > body.expectedVisualRevision + 1
                ? "原候选的接受已核对，当前规范已到更晚版本；设定图与头像分别显示。"
                : "候选已接受为规范参考；设定图准备和显示头像分别核对。"
              : r.result.next_attempt
                ? "重新生成建议已记录，下一次实际候选单独显示。"
                : "重新生成建议已记录，本次回执没有启动下一次尝试。",
          );
          await timeline.load();
          if (body.decision === "accepted" && m.live())
            try {
              await m.service.refresh();
            } catch {
              if (m.live())
                setNotice(
                  "规范参考已接受并读回；其他页面显示暂未刷新，可重新读取头像。",
                );
            }
        }
      },
      fields: (d, edit) => (
        <div className="r-review-layout">
          <div>
            <MediaById id={candidate} model={m} className="r-review-art" />
            <p className="r-fineprint">第 {attempt} 次尝试 · 人工评审</p>
          </div>
          <div className="r-form-stack">
            <div className="replica-field">
              <span>评审结果</span>
              <RadioGroup
                aria-label="评审结果"
                value={d.decision || ""}
                onValueChange={(decision) => edit({ decision })}
                className="r-review-choices"
              >
                {[
                  ["accepted", "通过", "接受这张候选图作为形象参考"],
                  ["regenerate", "不通过", "说明需要修改的地方，重新生成"],
                ].map(([value, label, description]) => (
                  <label className="r-review-choice" key={value}>
                    <RadioGroupItem value={value} aria-label={label} />
                    <span><b>{label}</b><small>{description}</small></span>
                  </label>
                ))}
              </RadioGroup>
            </div>
            {d.decision === "regenerate" && (
              <>
                <label className="replica-field">
                  <span>不通过的原因 · 必填</span>
                  <Textarea
                    aria-label="不通过的原因"
                    aria-required="true"
                    placeholder="例如：侧面视图缺失，服装颜色与参考不一致"
                    value={d.feedback || ""}
                    onChange={(e) => edit({ feedback: e.target.value })}
                  />
                  <span>这些修改要求会用于下一次生成。</span>
                </label>
                <Disclosure title="补充评审细节（选填）">
                  <div className="r-form-stack">
                    {[["identityMatch", "身份符合程度"], ["confidence", "评审置信程度"]].map(([key, label]) => (
                      <div className="replica-field" key={key}>
                        <div className="r-review-score-heading">
                          <span>{label} · {d[key] == null ? "未设置" : Math.round(d[key] * 100) + "%"}</span>
                          {d[key] == null && (
                            <Button type="button" variant="outline" size="sm"
                              aria-label={`将${label}设为0%`}
                              onClick={() => edit({ [key]: 0 })}>设为 0%</Button>
                          )}
                        </div>
                        <Slider aria-label={label} min={0} max={100} step={1}
                          value={[d[key] == null ? 0 : Math.round(d[key] * 100)]}
                          onValueChange={(v) => edit({ [key]: v[0] / 100 })} />
                      </div>
                    ))}
                    {[
                      ["observations", "实际观察（一行一项）"],
                      ["missingSections", "图片缺少的内容（一行一项）"],
                      ["summary", "评审摘要"],
                      ["reason", "操作理由"],
                    ].map(([key, label]) => (
                      <label className="replica-field" key={key}>
                        <span>{label} · 选填</span>
                        <Textarea aria-label={label} value={d[key] || ""}
                          onChange={(e) => edit({ [key]: e.target.value })} />
                      </label>
                    ))}
                  </div>
                </Disclosure>
              </>
            )}
            {d.reviewed && (
              <WorkspaceInfo>
                原候选已经评审，后续输入只作保留；请关闭后重新读取下一阶段。
              </WorkspaceInfo>
            )}
          </div>
        </div>
      ),
    });
  }
  function reviewGenerated(event, row) {
    if (!row?.id || !row?.candidate_asset_id || !row?.reference_asset_id) return;
    m.assertCurrent();
    trigger.current = event.currentTarget;
    const reviewId = row.id,
      kind = row.kind,
      candidate = row.candidate_asset_id,
      reference = row.reference_asset_id,
      title = kind === "avatar" ? "头像" : "角色设定图";
    studioDraftStorage().setItem(prefix + ":generated-review-context:" + reviewId, JSON.stringify(row));
    setAction({
      ...base("generated-review:" + reviewId),
      title: `评审${title}`,
      description: "查看这张图片后选择通过或不通过，结果会记录为人工评审。",
      wide: true,
      submitLabel: "提交人工评审",
      disabled: m.readOnly,
      preAdmissionConflicts: ["visual_review_conflict", "revision_conflict"],
      initial: { decision: "", feedback: "", visualRevision: row.visual_revision, reviewRevision: row.revision, reviewed: false },
      canSubmit: (d) => !d?.reviewed,
      validate: (d) => {
        if (d.reviewed) throw Error("这张图片已经评审，请重新读取状态。");
        if (!["accepted", "regenerate"].includes(d.decision)) throw Error("请选择通过或不通过。");
        if (d.decision === "regenerate" && !d.feedback?.trim()) throw Error("请说明图片哪里不符合，以及希望怎样修改。");
        if ([...(d.feedback || "")].length > 2000) throw Error("不通过的原因最多2000个字符。");
      },
      build: (d) => ({
        decision: d.decision,
        expectedVisualRevision: d.visualRevision,
        expectedReviewRevision: d.reviewRevision,
        ...(d.decision === "regenerate" ? { feedback: d.feedback } : {}),
      }),
      execute: async (body, key) => {
        m.assertWritable();
        const r = await client.reviewGeneratedVisualAsset(fid, reviewId, { ...body, idempotencyKey: key });
        if (r.fluctlight_id !== fid || r.id !== reviewId || r.kind !== kind || r.decision !== body.decision || r.candidate_asset_id !== candidate)
          throw Error("原图片的人工评审回执尚未完整确认。");
        return r;
      },
      afterAcknowledged: (d) => ({ ...d, reviewed: true }),
      readLatest: readIdentity,
      adoptLatest: (d, fresh) => {
        const currentReview = fresh.generated_reviews?.[kind];
        if (fresh.current_revision !== row.visual_revision || currentReview?.id !== reviewId || currentReview?.status !== "awaiting_review")
          throw Error("原图片已变化或已评审，请保留输入并重新读取状态。");
        return { ...d, reviewRevision: currentReview.revision };
      },
      onSaved: async (_result, body) => {
        const original = await client.generatedVisualAssetReview(fid, reviewId);
        const expectedStatus = body.decision === "accepted" ? "accepted" : "rejected";
        if (original.fluctlight_id !== fid || original.id !== reviewId || original.kind !== kind || original.candidate_asset_id !== candidate || original.reference_asset_id !== reference || original.status !== expectedStatus || original.review_source !== "manual")
          throw Error("原图片的人工评审尚未完整读回。");
        const fresh = await readIdentity();
        if (m.live()) {
          identityRef.current.adopt({ items: [fresh], next_cursor: "" });
          setNotice(body.decision === "accepted" ? `${title}已通过人工评审。` : `${title}的修改要求已记录，新的候选正在准备。`);
          await timeline.load();
          if (kind === "avatar" && body.decision === "accepted" && m.live()) {
            try {
              await m.service.refresh();
            } catch {
              if (m.live()) setNotice("头像已通过审核；其他页面暂未刷新，可重新读取头像。");
            }
          }
        }
      },
      fields: (d, edit) => (
        <div className="r-review-layout">
          <div className="r-derived-comparison">
            <div><p>已确认的形象参考</p><MediaById id={reference} model={m} className="r-review-art" /></div>
            <div><p>待审核{title}</p><MediaById id={candidate} model={m} className="r-review-art" /></div>
          </div>
          <div className="r-form-stack">
            <div className="replica-field">
              <span>评审结果</span>
              <RadioGroup aria-label="评审结果" value={d.decision || ""} onValueChange={(decision) => edit({ decision })} className="r-review-choices">
                {[["accepted", "通过", "接受这张图片"], ["regenerate", "不通过", "说明需要修改的地方，重新生成"]].map(([value, label, description]) => (
                  <label className="r-review-choice" key={value}>
                    <RadioGroupItem value={value} aria-label={label} />
                    <span><b>{label}</b><small>{description}</small></span>
                  </label>
                ))}
              </RadioGroup>
            </div>
            {d.decision === "regenerate" && <label className="replica-field"><span>不通过的原因 · 必填</span><Textarea aria-label="不通过的原因" aria-required="true" value={d.feedback || ""} onChange={(e) => edit({ feedback: e.target.value })} /><span>这些修改要求会用于下一次生成。</span></label>}
            {d.reviewed && <WorkspaceInfo>这张图片已经评审，请重新读取下一阶段。</WorkspaceInfo>}
          </div>
        </div>
      ),
    });
  }
  let reviewRecovery = [];
  let generatedReviewRecovery = [];
  try {
    const storage = studioDraftStorage();
    reviewRecovery = Object.keys(storage)
      .filter((key) => key.startsWith(prefix + ":review-context:"))
      .map((key) => JSON.parse(storage.getItem(key)))
      .filter(
        (row) =>
          typeof row?.sessionId === "string" &&
          typeof row.candidate === "string" &&
          Number.isSafeInteger(row.attempt) &&
          row.attempt > 0 &&
          Number.isSafeInteger(row.visualRevision),
      )
      .filter((row) =>
        [":draft", ":command", ":command:outcome"].some((suffix) =>
          storage.getItem(
            prefix + ":review:" + row.sessionId + ":" + row.attempt + suffix,
          ),
        ),
      );
    generatedReviewRecovery = Object.keys(storage)
      .filter((key) => key.startsWith(prefix + ":generated-review-context:"))
      .map((key) => JSON.parse(storage.getItem(key)))
      .filter((row) => typeof row?.id === "string" && typeof row.candidate_asset_id === "string" && typeof row.reference_asset_id === "string" && ["avatar", "character_sheet"].includes(row.kind) && Number.isSafeInteger(row.visual_revision) && Number.isSafeInteger(row.revision))
      .filter((row) => [":draft", ":command", ":command:outcome"].some((suffix) => storage.getItem(prefix + ":generated-review:" + row.id + suffix)));
  } catch {}
  return (
    <div className="r-form-stack">
      <MediaReader source={identity} label="视觉身份" />
      <MediaReader source={brief} label="视觉定义" />
      {error && <p role="alert">{error}</p>}
      {notice && <WorkspaceInfo>{notice}</WorkspaceInfo>}
      {reviewRecovery.map((row) => (
        <WorkspaceInfo key={row.sessionId + ":" + row.attempt}>
          原人工评审的输入或回执仍保留。
          <Button variant="outline" onClick={(event) => review(event, row)}>
            继续核对第{row.attempt}次人工评审
          </Button>
        </WorkspaceInfo>
      ))}
      {generatedReviewRecovery.map((row) => (
        <WorkspaceInfo key={row.id}>
          原图片的人工评审输入或回执仍保留。
          <Button variant="outline" onClick={(event) => reviewGenerated(event, row)}>继续核对{row.kind === "avatar" ? "头像" : "角色设定图"}评审</Button>
        </WorkspaceInfo>
      ))}
      {Object.values(visual?.generated_reviews || {}).filter((row) => row?.status === "awaiting_review" && row?.candidate_asset_id && row?.reference_asset_id).map((row) => (
        <Surface className="panel r-padded r-form-stack" key={row.id}>
          <h2 className="r-inline-title">{row.kind === "avatar" ? "头像" : "角色设定图"}等待人工审核</h2>
          <MediaById id={row.candidate_asset_id} model={m} className="r-identity-image" />
          <p>请查看这张实际图片，再选择通过或不通过。</p>
          {row.review_data?.reason && <p>{errorMessage({ code: row.review_data.reason }, "视觉模型暂时无法审核，请人工查看。")}</p>}
          {Array.isArray(row.review_data?.violations) && row.review_data.violations.length > 0 && (
            <ul>{row.review_data.violations.map((item, index) => <li key={index}>{item.detail || "视觉检查发现问题"}</li>)}</ul>
          )}
          {row.review_data?.retry_guidance && <p>模型建议：{row.review_data.retry_guidance}</p>}
          <Button disabled={m.readOnly} onClick={(event) => reviewGenerated(event, row)}>查看并审核{row.kind === "avatar" ? "头像" : "角色设定图"}</Button>
        </Surface>
      ))}
      <div className="r-toolbar">
        <div className="r-grow">
          <h2 className="r-inline-title">同一个人，越来越清晰</h2>
          <p className="r-muted">规范参考、候选与评审，各自留下真实记录。</p>
        </div>
        <Button
          variant="outline"
          disabled={!definition || m.readOnly}
          onClick={editBrief}
        >
          <Pencil size={14} />
          编辑视觉定义
        </Button>
        <Button
          disabled={!visual || !definition || m.readOnly}
          onClick={generate}
        >
          <Wand2 size={14} />
          生成新候选
        </Button>
        <Button
          variant="ghost"
          onClick={() => {
            void identity.load();
            void brief.load();
            void timeline.load();
          }}
        >
          重新读取
        </Button>
      </div>
      <div className="r-identity-grid">
        <Surface className="panel r-identity-card">
          <div className="r-inline-between">
            <h3>已接受的规范参考</h3>
            <Badge variant="outline">
              {visual?.canonical_asset_id
                ? "当前生效"
                : visual
                  ? "尚未建立"
                  : "待读取"}
            </Badge>
          </div>
          {visual?.canonical_asset_id ? (
            <MediaById
              id={visual.canonical_asset_id}
              model={m}
              className="r-identity-image"
            />
          ) : (
            <div className="r-identity-placeholder">
              <StoryMark />
              <h3>{visual ? "从第一个视觉候选开始" : "规范参考待读取"}</h3>
              <p>
                {visual
                  ? "保存视觉定义、生成候选并评审后，规范参考会出现在这里。"
                  : "读取后展示已确认的规范参考，失败时可以重新读取。"}
              </p>
            </div>
          )}
          <div className="r-card-body">
            <b>{m.actor?.name || "角色"}的视觉身份</b>
            <p>新的尝试不会覆盖这张图，直到明确接受候选。</p>
            <Button
              variant="ghost"
              disabled={!visual?.canonical_asset_id}
              onClick={(event) =>
                m.openAsset(visual.canonical_asset_id, event.currentTarget)
              }
            >
              查看参考图
            </Button>
          </div>
        </Surface>
        <Surface className="panel r-identity-card">
          <div className="r-inline-between">
            <h3>当前候选</h3>
            <Badge variant="outline">
              {visual
                ? session
                  ? visualSessionState(session.status)
                  : "等待新尝试"
                : "待读取"}
            </Badge>
          </div>
          {candidate ? (
            <MediaById id={candidate} model={m} className="r-identity-image" />
          ) : (
            <div className="r-identity-placeholder">
              <StoryMark />
              <h3>
                {!visual
                  ? "候选状态待读取"
                  : ["queued", "running"].includes(session?.status)
                    ? "正在准备实际候选"
                    : "给形象一点新的可能"}
              </h3>
              <p>
                {!visual
                  ? "请重新读取当前会话与候选状态。"
                  : session
                    ? "第 " +
                      session.current_attempt +
                      " 次尝试 · " +
                      visualSessionState(session.status)
                    : "生成候选后，可以与规范参考并排比较。"}
              </p>
            </div>
          )}
          <div className="r-card-body">
            <b>
              {current
                ? "第 " + current.attempt_number + " 次尝试"
                : "准备下一次尝试"}
            </b>
            <p>
              {canReview
                ? "候选已到达待评审状态，请查看图片并选择通过或不通过。"
                : "已接受的规范与后续设定图保持独立状态。"}
            </p>
            <Button disabled={!canReview || m.readOnly} onClick={review}>
              查看候选并评审
            </Button>
          </div>
        </Surface>
      </div>
      {receipt && (
        <WorkspaceInfo>
          上次视觉准备回执：
          {receipt.acceptedStatus === "pending"
            ? "受理时等待可用会话"
            : "受理时已排入准备"}
          。当前状态以本页重新读取的会话为准。
          {receipt.taskId && (
            <a href={"#/diag/task/" + encodeURIComponent(receipt.taskId)}>
              查看实际任务
            </a>
          )}
        </WorkspaceInfo>
      )}
      {identity.paused && <p>自动跟进已暂停，原任务保留，可重新读取。</p>}
      <div className="r-two-column">
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">
            视觉定义 · {definition ? "v" + definition.revision : "待读取"}
          </h2>
          {briefFields.map(([key, label]) => (
            <div className="profile-key-value" key={key}>
              <span>{label}</span>
              <div>
                {definition ? definition.brief?.[key] || "尚未填写" : "待读取"}
              </div>
            </div>
          ))}
          {definition?.brief?.source_resource_id && (
            <a
              href={
                "#/studio/edit/" +
                encodeURIComponent(definition.brief.source_resource_id)
              }
            >
              查看定义来源 · {definition.brief.source_version}
            </a>
          )}
          <Button variant="ghost" onClick={() => setHistoryOpen(true)}>
            <History size={13} />
            定义修订
          </Button>
        </Surface>
        <Surface className="panel r-padded">
          <h2 className="r-inline-title">候选与评审记录</h2>
          <MediaReader source={timeline} label="视觉记录" />
          {timeline.items.map((row, index) => (
            <div className="r-timeline-entry" key={row.id || index}>
              <b>{typeof row.summary === "string" ? row.summary.replace(/Visual Identity/g,"形象准备") : stateLabel(row.stage) || "视觉记录"}</b>
              <small>
                {row.occurred_at ? userInstant(row.occurred_at) : "时间未记录"}
              </small>
              <p>
                {row.review?.summary || ""}
                {row.review?.source
                  ? " · " +
                    (["manual", "owner_manual"].includes(row.review.source)
                      ? "人工评审"
                      : "记录来源：" + row.review.source)
                  : ""}
              </p>
              {(row.asset_ids || []).map((id) => (
                <Button
                  key={id}
                  variant="ghost"
                  onClick={(e) => m.openAsset(id, e.currentTarget)}
                >
                  查看当时的图片
                </Button>
              ))}
              <Disclosure title="完整视觉记录">
                <pre className="replica-source-record">
                  {JSON.stringify(row, null, 2)}
                </pre>
              </Disclosure>
            </div>
          ))}
          {timeline.ready && !timeline.items.length && (
            <p>尚无视觉生成与评审记录。</p>
          )}
          <MediaMore source={timeline} label="视觉记录" />
        </Surface>
      </div>
      <Surface className="panel r-padded">
        <h2 className="r-inline-title">角色设定图</h2>
        {visual?.character_sheet_asset_id ? (
          <MediaById
            id={visual.character_sheet_asset_id}
            model={m}
            className="r-sheet-image"
            onOpen={m.openAsset}
          />
        ) : (
          <p className="r-muted">
            {session?.status === "character_sheet_pending"
              ? "规范参考已接受，设定图仍在准备。"
              : "尚未读到完成的角色设定图。"}
          </p>
        )}
        <Disclosure title="规范来源、渲染约束与完整会话">
          <pre className="replica-source-record">
            {JSON.stringify(visual || {}, null, 2)}
          </pre>
        </Disclosure>
      </Surface>
      {action && (
        <WorkspaceCommandDialog
          action={action}
          onClose={() => setAction(null)}
          returnFocus={() => trigger.current}
        />
      )}
      <WorkspaceDialog
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        title="视觉定义修订"
        description="每次实际保存的参考说明"
      >
        <MediaReader source={history} label="定义修订" />
        {history.items.map((row, index) => (
          <div className="r-timeline-entry" key={row.id || index}>
            <b>视觉定义 v{row.revision}</b>
            <p>{row.reason}</p>
            <Disclosure title="此版本与前后变化">
              <pre className="replica-source-record">
                {JSON.stringify(row, null, 2)}
              </pre>
            </Disclosure>
          </div>
        ))}
        <MediaMore source={history} label="定义修订" />
      </WorkspaceDialog>
    </div>
  );
}
