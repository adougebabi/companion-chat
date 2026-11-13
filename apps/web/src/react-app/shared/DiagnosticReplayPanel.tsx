import {useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore} from 'react';
import {Button} from '@/components/ui/button';
import {useWorkspace} from '../page';
import {errorMessage} from '@/v3/domain-presentation.mjs';

const sourceLabels = {
  events: '事件与工具',
  'model-runs': '模型调用',
  'agent-runs': 'Agent 运行',
  turns: '对话回合',
  links: '工作流关联',
  'media-prompts': '媒体任务',
  'workflow-intents': '异步任务',
  'media-recoveries': '媒体恢复',
  kev: '决策记录',
};
type SourceKind = keyof typeof sourceLabels;
const sourceKinds = Object.keys(sourceLabels) as SourceKind[];
type SourcePage = {rows: any[]; cursor: string; snapshot?: string; error: string; loading: boolean};
const emptySource = (): SourcePage => ({rows: [], cursor: '', error: '', loading: true});
const recordKey = (record: any) => record.kind + ':' + record.ref;

/** Playback reads stored diagnostics only. No runtime or recovery service is used. */
export default function DiagnosticReplayPanel({correlationId}: {correlationId: string}) {
  const workspace = useWorkspace();
  const session = useSyncExternalStore<{session?: {actorId?: string}}>(workspace.subscribe, workspace.getSnapshot, workspace.getSnapshot).session;
  const identity = JSON.stringify([session?.actorId, correlationId]);
  const generation = useRef(0);
  const pending = useRef(new Set<string>());
  const [state, setState] = useState<{identity: string; sources: Partial<Record<SourceKind, SourcePage>>}>({identity: '', sources: {}});
  const [selectedKey, setSelectedKey] = useState('');

  const load = useCallback(async (kind: SourceKind, cursor: string, epoch: number) => {
    const requestKey = JSON.stringify([epoch, kind, cursor]);
    if (pending.current.has(requestKey)) return;
    pending.current.add(requestKey);
    const current = () => generation.current === epoch;
    const update = (change: (previous: SourcePage) => SourcePage) => {
      if (!current()) return;
      setState(previous => ({...previous, sources: {...previous.sources, [kind]: change(previous.sources[kind] || emptySource())}}));
    };
    update(previous => ({...previous, loading: true, error: ''}));
    try {
      const page = await workspace.services.diagnosticsClient.page(kind, {correlationId, limit: 30, ...(cursor ? {cursor} : {})});
      if (!page || !Array.isArray(page.items) || typeof page.next_cursor !== 'string' || typeof page.snapshot !== 'string' || !page.snapshot || page.items.some((row: any) => row.kind !== kind || typeof row.ref !== 'string' || !row.ref)) {
        throw new Error('诊断回放未返回有效的来源分页与快照，请重新读取。');
      }
      if (cursor && page.next_cursor === cursor) throw new Error('来源分页未向前推进，请关闭回放后重新读取。');
      update(previous => {
        if (cursor && previous.snapshot !== page.snapshot) {
          return {...previous, loading: false, error: '来源快照发生变化，请关闭回放后重新读取。'};
        }
        const rows = new Map<string, any>(previous.rows.map(row => [recordKey(row), row]));
        for (const row of page.items) rows.set(recordKey(row), row);
        return {rows: [...rows.values()], cursor: page.next_cursor, snapshot: page.snapshot, loading: false, error: ''};
      });
    } catch (cause) {
      update(previous => ({...previous, loading: false, error: errorMessage(cause)}));
    } finally {
      pending.current.delete(requestKey);
    }
  }, [workspace, correlationId]);

  useEffect(() => {
    const epoch = ++generation.current;
    setState({identity, sources: {}});
    setSelectedKey('');
    for (const kind of sourceKinds) void load(kind, '', epoch);
    return () => { generation.current++; };
  }, [identity, load]);

  // Clear old-owner and old-correlation data synchronously during render.
  const sources = state.identity === identity ? state.sources : {};
  const rows = useMemo(() => Object.values(sources).flatMap(source => source?.rows || []).sort((left, right) => {
    const leftTime = Date.parse(left.occurred_at), rightTime = Date.parse(right.occurred_at);
    if (Number.isFinite(leftTime) && Number.isFinite(rightTime) && leftTime !== rightTime) return leftTime - rightTime;
    if (Number.isFinite(leftTime) !== Number.isFinite(rightTime)) return Number.isFinite(leftTime) ? -1 : 1;
    return recordKey(left).localeCompare(recordKey(right));
  }), [sources]);
  useEffect(() => {
    if (!selectedKey && rows[0]) setSelectedKey(recordKey(rows[0]));
  }, [rows, selectedKey]);
  const selectedIndex = Math.max(0, rows.findIndex(row => recordKey(row) === selectedKey));
  const selected = rows[selectedIndex];
  const loading = sourceKinds.some(kind => !sources[kind] || sources[kind]?.loading);
  const unread = sourceKinds.filter(kind => sources[kind]?.cursor);
  const errors = sourceKinds.filter(kind => sources[kind]?.error);

  return <section className="grid gap-3" aria-label="只读记录回放" data-testid="diagnostic-replay">
    <h3>只读记录回放</h3>
    <p className="text-sm text-muted-foreground">按已存时间排序；时间相同或未记录时，先后顺序待核对。各来源按自己的创建快照分页，可变状态为读取时状态。缺失或已清理的输入、工具回执和发布结果仍需核对。</p>
    <p role="status">已读取 {rows.length} 条记录{loading ? '，正在读取来源…' : ''}{unread.length ? `，${unread.length} 个来源仍有后续页` : ''}{errors.length ? `，${errors.length} 个来源读取失败` : ''}。</p>
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" disabled={!selected || selectedIndex === 0} onClick={() => setSelectedKey(recordKey(rows[selectedIndex - 1]))}>上一条记录</Button>
      <Button variant="outline" disabled={!selected || selectedIndex >= rows.length - 1} onClick={() => setSelectedKey(recordKey(rows[selectedIndex + 1]))}>下一条记录</Button>
      {selected && <span>{selectedIndex + 1} / {rows.length}</span>}
    </div>
    {selected && <article data-testid="replay-record" className="grid gap-2">
      <h4>{sourceLabels[selected.kind as SourceKind] || selected.kind} · {selected.data?.event_type || selected.data?.status || '已记录'}</h4>
      <p>{selected.occurred_at || '时间未记录'}</p>
      <pre className="replica-model-call-wire">{JSON.stringify(selected, null, 2)}</pre>
    </article>}
    {!loading && !rows.length && !errors.length && <p>该关联范围没有可回放记录，原任务结果仍需核对。</p>}
    {sourceKinds.map(kind => {
      const source = sources[kind];
      return <div key={kind} className="grid gap-1">
        {source?.error && <p role="alert">{sourceLabels[kind]}读取失败，已读记录保留。{source.error}</p>}
        {(source?.error || source?.cursor) && <Button variant="outline" disabled={source.loading} onClick={() => void load(kind, source.cursor, generation.current)}>{source.error ? `重试${sourceLabels[kind]}` : `继续读取${sourceLabels[kind]}`}</Button>}
      </div>;
    })}
  </section>;
}
