import { useMemo } from "react";
import { Textarea } from "../controls";
import { useWorkspace } from "../page";
import WorkspaceCommandDialog from "./WorkspaceCommandDialog";
export default function MaterialRetention({
  material,
  open,
  onClose,
  onSaved,
}: any) {
  const w = useWorkspace(),
    owner = w.state.session.actorId,
    service = w.services.studioClient;
  const action = useMemo(
    () => ({
      identity: `studio-material:${owner}:${material.id}`,
      scope: ["studio-material-retention", material.id],
      storageKey: `fluctlight-studio:${owner}:material:${material.id}:retention`,
      className: "replica-studio",
      title: "创作素材保留状态",
      description: "取消保留只解除资源素材关联，原任务、字节和来源仍保留。",
      initial: {
        operation: material.released_at ? "retain" : "release",
        expectedRevision: material.revision,
        reason: "",
      },
      validate: (d: any) => {
        if (!d.reason.trim() || [...d.reason].length > 1000)
          throw Error("请填写修改原因，最多1000个字符。");
      },
      build: (d: any) => ({ ...d }),
      execute: async (value: any, key: string) => {
        const { operation, ...body } = value;
        if (!["retain", "release"].includes(operation))
          throw Error("未知素材保留操作。");
        const result = await service.materialRetention(material.id, operation, {
          ...body,
          idempotencyKey: key,
        });
        if (
          result.id !== material.id ||
          result.resource_id !== material.resource_id ||
          result.job_id !== material.job_id ||
          !Number.isSafeInteger(result.revision)
        )
          throw Error("素材保留状态回执尚未完整确认。");
        return result;
      },
      readLatest: () => service.material(material.id),
      adoptLatest: (d: any, r: any) => ({ ...d, expectedRevision: r.revision }),
      onSaved: async (result: any) => {
        const actual = await service.material(material.id);
        if (
          actual.id !== material.id ||
          actual.revision < result.revision ||
          (actual.revision === result.revision &&
            actual.released_at !== result.released_at)
        )
          throw Error("素材保留状态尚未读回。");
        await onSaved(actual);
      },
      fields: (d: any, edit: any) => (
        <>
          <p>
            本次意图：{d.operation === "retain" ? "恢复保留" : "取消保留"} ·
            基线版本 {d.expectedRevision}
          </p>
          <label className="replica-field">
            <span>修改原因</span>
            <Textarea
              aria-label="修改原因"
              required
              value={d.reason}
              onChange={(event) => edit({ reason: event.target.value })}
            />
          </label>
        </>
      ),
    }),
    [owner, material.id],
  );
  return open ? (
    <WorkspaceCommandDialog
      action={action}
      onClose={onClose}
      returnFocus={() => null}
    />
  ) : null;
}
