import {InlineFeedback} from "@/react-app/shared/Presentation";
import {Disclosure} from "@/react-app/controls";
import { useController } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { ActionButton } from "@/react-app/controls";
import { displayText } from "@/react-app/render-helpers";
import { errorMessage } from "@/v3/domain-presentation.mjs";

import { ActionFeedback } from "@/react-app/components";
import { wardrobePreparationStage } from "@/approved-ui/wardrobe-preparation.mjs";
function createWardrobePreparationController(__props, context, lifecycle) {
  const w = lifecycle.workspace,
    fid = w.state.selectedId,
    scope = w.currentScope(),
    preparation = lifecycle.cell(null),
    readError = lifecycle.cell(""),
    busy = lifecycle.cell(false);
  let active = true,
    reading = false,
    timer;
  const live = () => active && w.currentScope() === scope;
  const stages = selector(() => {
    const stage=wardrobePreparationStage(preparation.value, readError.value);
    return [{...stage,note:readError.value?'':stage.note}];
  });
  async function load() {
    if (!live() || reading) return;
    reading = true;
    try {
      const value = await w.services.inventoryClient.preparation(fid);
      if (live()) {
        preparation.value = value;
        readError.value = "";
      }
    } catch (cause) {
      if (live()) readError.value = errorMessage(cause);
    } finally {
      reading = false;
    }
  }
  async function recover() {
    if (busy.value) return;
    busy.value = true;
    try {
      const latest = await w.services.inventoryClient.current(fid);
      if (!live()) return;
      const record = w.command(
        "衣柜",
        "wardrobe-preparation",
        {
          expectedRevision: latest.revision,
          reason: "恢复当前角色的初始穿搭准备",
        },
        (body, key) =>
          w.client.prepareInventory(fid, {
            ...body,
            idempotencyKey: key,
          }),
        {
          onSaved: async () => {
            if (live()) {
              await load();
              void w.loadWardrobe();
            }
          },
          success: "衣柜准备请求已受理，实际结果将继续更新",
        },
      );
      await record.completion;
    } catch (cause) {
      if (live()) readError.value = errorMessage(cause);
    } finally {
      if (live()) busy.value = false;
    }
  }
  const visibility = () => {
    if (!document.hidden) void load();
  };
  lifecycle.onStart(() => {
    void load();
    timer = setInterval(() => {
      if (
        !document.hidden &&
        preparation.value &&
        ["queued", "generating", "awaiting_wear"].includes(
          preparation.value.status,
        ) &&
        !preparation.value.can_recover
      )
        void load();
    }, 5000);
    document.addEventListener("visibilitychange", visibility);
  });
  lifecycle.onDispose(() => {
    active = false;
    clearInterval(timer);
    document.removeEventListener("visibilitychange", visibility);
  });
  return (_ctx, _cache) => {
    if(stages.value[0]?.state==='ready'&&!readError.value)return null;
    const stage = stages.value[0];
    const labels = {unknown:'初始衣柜状态待确认',pending:'初始衣柜正在准备',running:'初始衣柜正在准备',failed:'初始衣柜准备遇到问题'};
    return (
      <div className="approved-preparation wardrobe-preparation-inline">
        <p role="status">{labels[stage.state] || '初始衣柜正在准备'}{stage.state !== 'failed' && stage.note ? ' · ' + stage.note : ''}</p>
        {stage.state === 'failed' && stage.note && <InlineFeedback kind="error">{displayText(stage.note)}</InlineFeedback>}
        {readError.value && <InlineFeedback role="alert">{displayText(readError.value)}</InlineFeedback>}
        <div className="wardrobe-preparation-actions">
          {stage.retryable && <ActionButton type="button" variant={stage.state === 'failed' ? 'default' : 'outline'} disabled={busy.value} onClick={recover}>{busy.value ? '处理中…' : stage.state === 'failed' ? '重试此环节' : '恢复原任务'}</ActionButton>}
          {stage.state === 'failed' && stage.configSection && <ActionButton variant="ghost" asChild><a href={'#/settings?section='+encodeURIComponent(stage.configSection)}>去配置中心</a></ActionButton>}
          <Disclosure title="准备记录与读取">
            {stage.href && <ActionButton variant="ghost" asChild><a href={'#/diag?view=tasks&taskId='+encodeURIComponent(preparation.value.task_id)+'&returnTo='+encodeURIComponent(globalThis.location?.hash || '#/library')}>查看任务记录</a></ActionButton>}
            <ActionButton variant="ghost" type="button" onClick={load}>重新读取准备状态</ActionButton>
          </Disclosure>
        </div>
        <ActionFeedback labels={["衣柜"]} />
      </div>
    );
  };
}
export default function WardrobePreparation(input = {}) {
  return useController(createWardrobePreparationController, {
    ...input,
  });
}
