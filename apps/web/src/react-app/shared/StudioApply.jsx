import { useEffect } from "react";
import { ArrowUpRight, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ChoiceSelect, Label, FieldInput, Disclosure } from "../controls";
import WorkspaceDialog from "./WorkspaceDialog";
import WorkspaceInfo from "./WorkspaceInfo";
import SemanticTree from "./SemanticTree";
import JsonDetails from "./JsonDetails";
import {
  studioVersion,
  studioFieldLabel,
  studioTargetHref,
} from "@/approved-ui/studio-presentation.mjs";
export default function StudioApply({ model: m }) {
  const { controller: c, state: s, w, d, r, readOnly } = m,
    p = s.preview,
    result = s.lastApplication;
  const publication = p?.publication,
    kind = publication?.kind || r?.kind;
  const freshCreate =
    !!p?.create_required && !p.runtime_id && p.target_kind !== "foundation";
  useEffect(() => {
    if (!p || !freshCreate) return;
    for (const row of p.diff || [])
      if (!p.choices[row.field])
        c.chooseDifference(row.field, "override", p.viewEpoch);
  }, [p?.viewEpoch, freshCreate]);
  const targets = w.state.characters.filter(
    (actor) =>
      !["retired", "archived", "已退休", "已归档"].includes(actor.status),
  );
  return (
    <WorkspaceDialog
      open={m.applyOpen}
      onOpenChange={m.setApplyOpen}
      title={"应用「" + (publication?.name || r?.name || "资源") + "」"}
      description={
        "使用精确发布版本 " +
        studioVersion(p?.version || r?.published_version, "待读取") +
        "，逐项核对后再确认。"
      }
      wide
      className="replica-studio"
      returnFocus={() => m.dialogTriggers.current.apply}
      footer={
        result ? (
          <Button onClick={() => m.setApplyOpen(false)}>完成</Button>
        ) : (
          <>
            <Button variant="outline" onClick={() => m.setApplyOpen(false)}>
              取消
            </Button>
            {p && !p.preparation_required && (
              <Button
                disabled={
                  readOnly ||
                  s.busy ||
                  !!s.pending ||
                  s.readbackPending ||
                  (!p.create_required && !p.diff?.length)
                }
                onClick={() => void c.apply()}
              >
                确认应用所选字段
              </Button>
            )}
          </>
        )
      }
    >
      {result ? (
        <div className="r-result-success">
          <CheckCircle2 size={38} />
          <h3>
            {result.result.target_kind === "foundation"
              ? "身份修订提议已创建"
              : "原应用回执已核对"}
          </h3>
          <p>
            {studioVersion(result.version)} ·{" "}
            {w.state.characters.find((actor) => actor.id === result.fid)
              ?.name || "原目标角色"}
          </p>
          <Button
            variant="outline"
            onClick={() =>
              w.navigatePath(
                studioTargetHref(result.kind, result.fid, result.result).slice(
                  1,
                ),
              )
            }
          >
            查看角色中的结果
            <ArrowUpRight size={14} />
          </Button>
          <JsonDetails value={result.result} title="原应用回执与结果" />
        </div>
      ) : (
        <div className="r-form-stack">
          <Label htmlFor="studio-apply-target">
            目标角色
            <ChoiceSelect
              id="studio-apply-target"
              aria-label="目标角色"
              value={d.target || ""}
              disabled={readOnly || s.busy}
              onChange={(event) => c.setTarget(event.target.value)}
            >
              <option value="">请选择角色</option>
              {targets.map((actor) => (
                <option key={actor.id} value={actor.id}>
                  {actor.name}
                </option>
              ))}
            </ChoiceSelect>
          </Label>
          <Button
            variant="outline"
            disabled={
              readOnly ||
              s.busy ||
              !r?.published_version ||
              !d.target ||
              !!s.pending
            }
            onClick={() => void c.readPreview()}
          >
            读取应用差异
          </Button>
          {s.previewLoading && (
            <p role="status">正在读取精确发布版本与目标差异…</p>
          )}
          {s.error && <p role="alert">{s.error}</p>}
          {p && (
            <>
              {p.preparation_required ? (
                <>
                  <WorkspaceInfo>
                    {p.semantics ||
                      "这份已发布文本需要先解析为完整结构化定义。"}
                  </WorkspaceInfo>
                  <Button
                    disabled={s.busy || !!s.pending}
                    onClick={() => void c.prepare()}
                  >
                    解析已发布文本为结构化定义
                  </Button>
                  <p className="r-muted">
                    审阅建议、采用到同一份工作草稿、保存并发布后，再重新读取应用差异。
                  </p>
                  {s.jobId && (
                    <Button
                      variant="outline"
                      onClick={() => {
                        m.setApplyOpen(false);
                        m.changeMode();
                      }}
                    >
                      前往沙盒审阅原解析任务
                    </Button>
                  )}
                </>
              ) : (
                <>
                  {freshCreate && (
                    <WorkspaceInfo>
                      这是新的运行定义，将采用已发布内容。加入目录不代表取得、穿着、启用事件或已经相识。
                    </WorkspaceInfo>
                  )}
                  <div className="r-apply-table">
                    {(p.diff || []).map((row) => {
                      const locked = p.lockedFields?.[row.field];
                      return (
                        <div key={row.field}>
                          <b>{studioFieldLabel(row.field)}</b>
                          <div className="studio-diff-values">
                            <span>当前</span>
                            <SemanticTree value={row.from} />
                            <span>发布值</span>
                            <SemanticTree value={row.to} />
                          </div>
                          {freshCreate ? (
                            <span className="r-muted">采用发布内容</span>
                          ) : (
                            <ChoiceSelect
                              aria-label={
                                studioFieldLabel(row.field) + "的处理"
                              }
                              value={p.choices[row.field] || ""}
                              onChange={(event) =>
                                c.chooseDifference(
                                  row.field,
                                  event.target.value,
                                  p.viewEpoch,
                                )
                              }
                            >
                              <option value="">选择处理方式</option>
                              <option value="keep">保留当前值</option>
                              <option value="override" disabled={!!locked}>
                                采用发布值
                              </option>
                            </ChoiceSelect>
                          )}
                          {locked && <p className="r-fineprint">{locked}</p>}
                        </div>
                      );
                    })}
                  </div>
                  {!freshCreate && p.diff?.length > 0 && (
                    <Button
                      variant="ghost"
                      onClick={() => {
                        for (const row of p.diff)
                          c.chooseDifference(
                            row.field,
                            p.lockedFields?.[row.field] ? "keep" : "override",
                            p.viewEpoch,
                          );
                      }}
                    >
                      全部采用允许的发布值
                    </Button>
                  )}
                  {kind === "npc" && freshCreate && (
                    <Label>
                      初始地点
                      <ChoiceSelect
                        aria-label="新人物初始地点"
                        value={p.homePlaceId || ""}
                        onChange={(event) =>
                          c.setNPCHome(event.target.value, p.viewEpoch)
                        }
                      >
                        <option value="">未设定</option>
                        {(p.targetWorld?.places || [])
                          .filter((place) => place.status === "active")
                          .map((place) => (
                            <option key={place.id} value={place.id}>
                              {place.alias || place.name}
                            </option>
                          ))}
                      </ChoiceSelect>
                    </Label>
                  )}
                  {p.target_kind === "wardrobe_outfit" &&
                    (p.members || []).map((member, index) => (
                      <div
                        className="r-form-stack"
                        key={
                          member.resource_id +
                          ":" +
                          member.resource_version +
                          ":" +
                          index
                        }
                      >
                        <b>
                          {member.name || "服饰定义"} ·{" "}
                          {studioVersion(member.resource_version)}
                        </b>
                        <ChoiceSelect
                          aria-label={"成员 " + (index + 1) + " 的实际服饰"}
                          value={p.memberItems[index]}
                          onChange={(event) =>
                            c.chooseMember(
                              index,
                              event.target.value,
                              p.viewEpoch,
                            )
                          }
                        >
                          <option value="">请选择实际库存服饰</option>
                          {(member.candidates || []).map((item) => (
                            <option
                              key={item.item_id || item.id}
                              value={item.item_id || item.id}
                            >
                              {item.description || item.name || "已登记服饰"}
                            </option>
                          ))}
                        </ChoiceSelect>
                        {member.more_candidates && (
                          <p className="r-fineprint">
                            已读候选不是完整库存，可核对其他实际服饰。
                          </p>
                        )}
                        <Disclosure title="补充明确的其他实际库存引用">
                          <FieldInput
                            aria-label={
                              "成员 " + (index + 1) + " 的其他实际库存 ID"
                            }
                            value={p.memberItems[index]}
                            onChange={(event) =>
                              c.chooseMember(
                                index,
                                event.target.value,
                                p.viewEpoch,
                              )
                            }
                          />
                        </Disclosure>
                      </div>
                    ))}
                  {p.target_kind === "foundation" && (
                    <WorkspaceInfo>
                      应用会形成身份修订提议。接受、编译与提交后才生效，不能直接覆盖人格或成长记录。
                    </WorkspaceInfo>
                  )}
                  {p.semantics && <p className="r-muted">{p.semantics}</p>}
                  <JsonDetails value={p.conflicts} title="冲突与来源" />
                  <JsonDetails
                    value={{
                      publication: p.publication,
                      instance_revision: p.instance_revision,
                      runtime_revision: p.runtime_revision,
                      target_world: p.targetWorld?.world?.id,
                    }}
                    title="精确发布版与目标基线"
                  />
                </>
              )}
            </>
          )}
        </div>
      )}
    </WorkspaceDialog>
  );
}
