import { worldDefinitionError } from "@/v3/proto/components/world-definition.js";
import { stagedCommand } from "@/approved-ui/staged-command.mjs";
// Retains the existing real Studio expansion sequence and original staged request identities.
export function runWorldExpansion({
  w,
  fid,
  owner,
  target,
  instruction,
  onSaved,
}) {
  const source = target._source || target;
  const input = {
    wid: target.id,
    world: source,
    instruction:
      instruction.trim() ||
      "按当前世界主题补充合理的场地分类与具体地点，保留现有地点及历史，不改变角色位置。",
  };
  const record = w.command(
    "世界",
    "expand:" + target.id,
    input,
    async (value, key) => {
      const live = () => w.isCurrent(owner, fid),
        journal = stagedCommand({
          storage: sessionStorage,
          key: "fluctlight:approved-stages:" + key,
          live,
        });
      const body = {
        background: value.world.background || value.world.description || "",
        rules: value.world.rules || "",
        description: value.world.description || "",
        flavor: value.world.flavor || "",
      };
      const resource = await journal.stage(
        "source",
        () => ({
          kind: "world",
          name: value.world.name,
          body,
          reason: "为原世界准备扩写底稿",
        }),
        (payload, idempotencyKey) =>
          w.services.studioClient.save(null, {
            ...payload,
            idempotencyKey,
          }),
      );
      const accepted = await journal.stage(
        "job",
        () => ({
          resourceId: resource.id,
          resourceRevision: resource.revision,
          kind: "assistant",
          name: resource.name,
          body,
          instruction: value.instruction,
          reason: "扩写已有世界中的场地和地点",
        }),
        (payload, idempotencyKey) =>
          w.services.studioClient.createJob({
            ...payload,
            idempotencyKey,
          }),
      );
      let job = accepted;
      for (let attempt = 0; attempt < 100; attempt++) {
        if (!live()) throw Error("角色已变化，请回到原世界继续");
        if (document.hidden)
          throw Error("已暂停后台观察，返回后可重试读取原扩写任务");
        job = await w.services.studioClient.job(accepted.id);
        if (!live()) throw Error("角色已变化");
        if (!["queued", "running"].includes(job.status)) break;
        await new Promise((resolve) => setTimeout(resolve, 3000));
      }
      if (
        job.status !== "completed" ||
        job.result?.disposition !== "proposal" ||
        !job.result.body?.framework
      )
        throw Error(job.error_code || "扩写仍未完成，请查看原创作任务");
      const proposal = {
          ...body,
          framework: job.result.body.framework,
        },
        invalid = worldDefinitionError(proposal);
      if (invalid) throw Error(invalid);
      const saved = await journal.stage(
        "save",
        () => ({
          resourceId: resource.id,
          kind: "world",
          name: resource.name,
          body: proposal,
          expectedVersion: resource.draft_version,
          reason: "保存实际模型返回的世界扩写",
        }),
        ({ resourceId, ...payload }, idempotencyKey) =>
          w.services.studioClient.save(resourceId, {
            ...payload,
            idempotencyKey,
          }),
      );
      const published = await journal.stage(
        "publish",
        () => ({
          resourceId: saved.id,
          expectedRevision: saved.revision,
          reason: "发布实际世界扩写",
        }),
        ({ resourceId, ...payload }, idempotencyKey) =>
          w.client.publishStudioResource(resourceId, {
            ...payload,
            idempotencyKey,
          }),
      );
      const result = await journal.stage(
        "apply",
        async () => {
          const preview = await w.services.studioClient.preview(
            fid,
            published.id,
            published.published_version,
            value.wid,
          );
          return {
            resourceId: published.id,
            fluctlightId: fid,
            runtimeWorldId: value.wid,
            resourceVersion: published.published_version,
            expectedInstanceRevision: preview.instance_revision,
            expectedRuntimeRevision: preview.runtime_revision,
            choices: preview.diff.map((row) => ({
              field: row.field,
              decision: row.field === "framework" ? "override" : "keep",
            })),
            reason: "应用明确请求的世界扩写，保留已有地点与历史",
          };
        },
        ({ resourceId, ...payload }, idempotencyKey) =>
          w.services.studioClient.apply(resourceId, {
            ...payload,
            idempotencyKey,
          }),
      );
      journal.finish();
      return result;
    },
    {
      onSaved,
      success: "扩写已应用并读回实际地点",
      multiStage: true,
    },
  );
  return record;
}
