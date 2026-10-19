import { onBeforeUnmount, ref, watch, type Ref } from 'vue';
import { BrowserApiError, BrowserClient, type RuntimeNPC, type StudioResource, type WorldView } from '@fluctlight/browser-client';
import { apiOrigin } from '../runtime-config';
import { randomId } from '../random-id';
import { userError } from '../lib/user-feedback';

export function useWorldWorkspace(fluctlightId: Ref<string>) {
  const client = new BrowserClient(apiOrigin);
  const snapshot = ref<WorldView | null>(null), definitions = ref<StudioResource[]>([]), selectedNPC = ref<RuntimeNPC | null>(null);
  const loading = ref(false), busy = ref(false), error = ref(''), definitionError = ref(''), notice = ref('');
  let epoch = 0, detailEpoch = 0;
  const commands = new Map<string, { fingerprint: string; key: string }>();
  async function load() {
    const e = ++epoch, fid = fluctlightId.value; loading.value = true; error.value = '';
    const results = await Promise.allSettled([client.world(fid), client.listStudioResources()]);
    if (e !== epoch || fid !== fluctlightId.value) return false;
    const [world, resources] = results;
    if (world.status === 'fulfilled') snapshot.value = world.value;
    else error.value = userError(world.reason, '运行世界读取失败，请重试。');
    if (resources.status === 'fulfilled') { definitions.value = resources.value.items; definitionError.value = ''; }
    else definitionError.value = userError(resources.reason, '定义目录读取失败；运行世界仍可独立查看。');
    loading.value = false; return world.status === 'fulfilled';
  }
  async function selectNPC(id: string) {
    const e = ++detailEpoch, fid = fluctlightId.value; selectedNPC.value = null;
    if (!id) return;
    try { const value = await client.runtimeNPC(fid, id); if (e === detailEpoch && fid === fluctlightId.value) selectedNPC.value = value; }
    catch (cause) { if (e === detailEpoch) error.value = userError(cause, '人物详情读取失败，请重试。'); }
  }
  async function mutate<T>(operation: string, body: unknown, action: (key: string) => Promise<T>): Promise<T | null> {
    if (busy.value) return null;
    const fid = fluctlightId.value, e = epoch, fingerprint = JSON.stringify({ fid, body });
    if (commands.get(operation)?.fingerprint !== fingerprint) commands.set(operation, { fingerprint, key: randomId() });
    busy.value = true; error.value = ''; notice.value = '';
    try {
      const result = await action(commands.get(operation)!.key);
      if (fid !== fluctlightId.value || e !== epoch) return null;
      if (!await load()) { error.value = '服务端已处理，重新查询失败。请保留当前内容并重新读取，勿重复提交。'; return null; }
      commands.delete(operation);
      notice.value = '已保存并重新查询。'; return result;
    } catch (cause) {
      if (fid === fluctlightId.value) error.value = cause instanceof BrowserApiError && cause.status === 409
        ? `版本或引用冲突，草稿已保留。请重新读取后核对再提交。(${cause.code})`
        : userError(cause, '操作未完成，草稿已保留。');
      return null;
    } finally { if (fid === fluctlightId.value) busy.value = false; }
  }
  watch(fluctlightId, () => { epoch++; detailEpoch++; snapshot.value = null; selectedNPC.value = null; definitions.value = []; commands.clear(); busy.value = false; notice.value = ''; void load(); }, { immediate: true });
  onBeforeUnmount(() => { epoch++; detailEpoch++; });
  return { client, snapshot, definitions, selectedNPC, loading, busy, error, definitionError, notice, load, selectNPC, mutate };
}
