import { useController } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { ActionButton, Disclosure } from "@/react-app/controls";
import { displayText } from "@/react-app/render-helpers";
import { errorMessage } from "@/v3/domain-presentation.mjs";
import PreparationSteps from "@/react-app/shared/PreparationSteps";
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
  const stages = selector(() => [
    wardrobePreparationStage(preparation.value, readError.value),
  ]);
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
    return (
      <Disclosure
        className="card approved-preparation"
        open={
          !!readError.value ||
          preparation.value?.status === "failed" ||
          preparation.value?.can_recover
        }
        title={
          <>
            {" 初始穿搭准备 "}
            <small>
              {displayText(
                preparation.value
                  ? {
                      ready: "已完成",
                      failed: "执行失败",
                      running: "进行中",
                      pending: "等待中",
                      unknown: "状态待确认",
                    }[stages.value[0].state]
                  : "读取中",
              )}
            </small>
          </>
        }
      >
        <PreparationSteps
          stages={stages.value}
          busy={busy.value}
          onRecover={recover}
        />
        {readError.value ? (
          <p key={0} className="inline-error" role="alert">
            {displayText(readError.value)}
          </p>
        ) : null}
        <ActionFeedback labels={["衣柜"]} />
        <ActionButton className="text-button" type="button" onClick={load}>
          {" 重新读取准备状态 "}
        </ActionButton>
      </Disclosure>
    );
  };
}
export default function WardrobePreparation(input = {}) {
  return useController(createWardrobePreparationController, {
    ...input,
  });
}
