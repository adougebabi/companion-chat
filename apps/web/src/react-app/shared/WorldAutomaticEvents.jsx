import { Button } from '@/components/ui/button';
import { Badge, Surface } from '../controls';
import { EmptyState } from './Presentation';
import { WorldReader } from './WorldFields';
import useWorkspaceRead from './useWorkspaceRead';
import { taskStatusLabel, taskStatusPresentation } from '@/approved-ui/task-presentation.mjs';
import { userInstant } from '@/v3/presentation-time.mjs';

export default function WorldAutomaticEvents({ model: m }) {
  const { w } = m, worldId = m.world.id;
  const read = async (kind, cursor) => {
    const page = await w.client.ownerDiagnosticRecords(kind, {
      worldId, limit: 10, ...(cursor ? { cursor } : {}),
      ...(kind === 'workflow-intents' ? { intentType: 'world.events.generate' } : {}),
    });
    if (page.items.some(row => (row.data?.world_id || row.data?.payload?.world_id) !== worldId ||
      (kind === 'workflow-intents' && row.data?.intent_type !== 'world.events.generate')))
      throw Error('自动世界事件记录范围不匹配，请重新读取。');
    return page;
  };
  const clocks = useWorkspaceRead({ identity: ['world-event-clocks', worldId], keyOf: row => row.ref, read: cursor => read('workflow-intents', cursor) });
  const generations = useWorkspaceRead({ identity: ['world-event-generations', worldId], keyOf: row => row.ref, read: cursor => read('world-event-generations', cursor) });
  return <Surface className="r-padded r-form-stack" aria-label="自动世界事件">
    <div className="section-heading"><div><h2>自动世界事件</h2><p>每小时根据世界现状生成候选事件，经审核后执行；本小时也可能没有事件。</p></div>
      <Button variant="outline" disabled={clocks.busy || generations.busy} onClick={() => Promise.all([clocks.load(), generations.load()])}>刷新自动事件</Button>
    </div>
    <WorldReader source={clocks} label="世界事件任务" />
    {clocks.items.map(({ ref, data }) => <div key={ref} className="flex flex-wrap items-center gap-3 border-b py-4">
      <div className="min-w-0 flex-1"><h3>世界事件时钟</h3><p>{data.last_cycle_status && `上一轮：${taskStatusLabel(data.last_cycle_status)}`}{data.next_cycle_at && ` · 下一轮：${userInstant(data.next_cycle_at)}`}</p></div>
      <Badge variant="outline">{taskStatusPresentation(data).label}</Badge>
      <Button variant="outline" onClick={() => w.navigatePath('/diag?view=tasks&taskId=' + encodeURIComponent(data.intent_id))}>查看生成任务</Button>
    </div>)}
    {clocks.ready && !clocks.items.length && !clocks.error && <p className="r-muted">尚未读到自动生成任务，可刷新查看后台准备进度。</p>}
    {clocks.cursor && <Button variant="outline" disabled={clocks.busy} onClick={clocks.more}>加载更多生成任务</Button>}
    <WorldReader source={generations} label="自动事件记录" />
    {generations.items.map(({ ref, data }) => <div key={ref} className="flex flex-wrap items-center gap-3 border-b py-4">
      <div className="min-w-0 flex-1"><h3>{data.proposal?.name || data.title || '本小时世界事件'}</h3><p>{userInstant(data.slot_at)}{data.proposal?.reason ? ' · ' + data.proposal.reason : ''}</p>
        {data.execution && <p>执行结果：{({ queued: '等待执行', applied: '已应用', filtered: '条件过滤', failed: '执行失败' })[data.execution.status] || taskStatusLabel(data.execution.status)}</p>}
      </div>
      <Badge variant="outline">{taskStatusLabel(data.status)}</Badge>
      <Button variant="outline" onClick={() => w.navigatePath('/diag/record/world-event-generations/' + encodeURIComponent(ref))}>查看生成与审核记录</Button>
    </div>)}
    {generations.ready && !generations.items.length && !generations.error && <EmptyState><h3>尚无自动事件记录</h3><p>后台按小时运行后会显示生成、审核和执行结果，无需先创建事件模板。</p></EmptyState>}
    {generations.cursor && <Button variant="outline" disabled={generations.busy} onClick={generations.more}>加载更多自动事件</Button>}
  </Surface>;
}
