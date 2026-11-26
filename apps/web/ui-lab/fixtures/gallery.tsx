// UI Lab gallery: every shared pattern in each applicable state, with fixed
// fixture data. Dev/test only. Playwright iterates GALLERY × states × viewports.
import { useState, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { ChoiceSelect, DrawerFrame } from '@/react-app/controls';
import { DetailDrawer } from '@/react-app/components';
import {
  AsyncTaskStatus, DataState, DetailFields, DetailSheet, EditSheet, Feedback, FormActions, FormField, FormSection,
  PageContainer, PageSection, RecordList, RecordListItem, ResponsiveTabs, type AsyncTaskState, type FeedbackKind,
} from '@/components/patterns';
import TurnDetailsDrawer, { type TurnRecord, type TurnRecordReader } from '@/react-app/shared/TurnTimeline';
import { events } from './events';
import { GALLERY_CATALOG, type GalleryId } from './gallery-catalog';

const LONG = '在旧街尽头那家只在周末营业、需要提前两天预约座位并且会根据天气调整菜单的小咖啡馆里，一边整理这周拍下的照片一边写下明天想去的地方';
const LONG_WORD = 'https://example.invalid/fluctlight/records/2026-11-16/very-long-identifier-without-any-spaces-0123456789abcdef';
const record = (action: string, ...args: unknown[]) => events.push({ action, args: JSON.parse(JSON.stringify(args)) });

const promises = (long: boolean) => [
  { id: 'p1', title: long ? LONG : '周六一起去看展', meta: long ? `${LONG_WORD} · 2026-11-21 18:00（北京时间）` : '2026-11-21 18:00（北京时间）', status: '已接受' },
  { id: 'p2', title: '每天睡前道晚安', meta: '2026-11-30 23:00（北京时间）', status: '等待判断' },
  { id: 'p3', title: '学会做一道新的汤', meta: '未设定期限', status: '已履行' },
];

function Records({ state }: { state: string }) {
  const long = state === 'long';
  const rows = state === 'empty' ? [] : promises(long);
  return (
    <DataState
      label="约定"
      loading={state === 'loading'}
      error={state === 'error' ? '服务暂时没有响应（fixture）。' : state === 'stale' ? '最新一页没有读到（fixture）。' : undefined}
      empty={rows.length === 0 && state !== 'loading' && state !== 'error'}
      hasData={rows.length > 0 && state !== 'loading' && state !== 'error'}
      emptyDescription="可通过“和她约定”提出内容及履行期限。"
      emptyAction={<Button type="button" onClick={() => record('empty-action')}>和她约定</Button>}
      onRetry={() => record('retry', 'records')}
    >
      <RecordList label="约定列表">
        {rows.map(row => (
          <RecordListItem key={row.id} title={row.title} meta={row.meta} status={row.status}
            actions={<><Button type="button" variant="outline" onClick={() => record('open', row.id)}>查看约定</Button><Button type="button" variant="ghost" onClick={() => record('arrange', row.id)}>安排时间</Button></>} />
        ))}
      </RecordList>
    </DataState>
  );
}

function Details({ state }: { state: string }) {
  const long = state === 'long';
  return (
    <DetailFields items={state === 'empty' ? [{ label: '约定内容' }, { label: '履行期限' }, { label: '来源' }] : [
      { label: '约定内容', value: long ? LONG : '周六一起去看展' },
      { label: long ? '最早开始时间（北京时间，可留空）' : '最早开始', value: '2026-11-21 10:00（北京时间）' },
      { label: '履行期限', value: '2026-11-21 18:00（北京时间）' },
      { label: '来源', value: long ? LONG_WORD : '对话中提出' },
      { label: '备注' },
    ]} />
  );
}

function SheetDemo({ kind, state }: { kind: 'detail' | 'edit'; state: string }) {
  const [open, setOpen] = useState(true);
  const long = state === 'long';
  const trigger = <Button type="button" variant="outline">{kind === 'detail' ? '查看约定详情' : '编辑约定'}</Button>;
  if (kind === 'detail') return (
    <DetailSheet open={open} onOpenChange={setOpen} trigger={trigger} title={long ? LONG : '约定与履行证据'}
      footer={state === 'normal' || long ? <Button type="button" variant="outline" onClick={() => setOpen(false)}>关闭</Button> : undefined}>
      {state === 'loading' ? <DataState label="约定详情" loading /> : state === 'error' ? <DataState label="约定详情" error="读取约定时服务没有响应（fixture）。" onRetry={() => record('retry', 'detail')} /> : <div className="ui-stack"><Details state={state} /><AsyncTaskStatus state="running" message="正在评估履行证据。" progress={40} /></div>}
    </DetailSheet>
  );
  return (
    <EditSheet open={open} onOpenChange={setOpen} trigger={trigger} title={long ? LONG : '编辑约定'}
      busy={state === 'busy'} submitDisabled={state === 'disabled'}
      error={state === 'error' ? '约定已经在其他设备上被修改。草稿已保留，请重新读取后再提交。' : undefined}
      onSubmit={data => { record('edit-submit', Object.fromEntries(data)); setOpen(false); }}>
      <FormSection title="约定内容" description={long ? LONG : '提议需要她判断；受理不等于接受或履行。'}>
        <FormField label="约定内容" required error={state === 'error' ? '请填写约定内容' : undefined}>
          {p => <Textarea {...p} name="title" rows={3} defaultValue={long ? LONG : state === 'error' ? '' : '周六一起去看展'} />}
        </FormField>
      </FormSection>
      <FormSection title="时间" columns={2}>
        <FormField label="最早开始" hint="北京时间，可留空">{p => <Input {...p} name="start" type="time" defaultValue="10:00" />}</FormField>
        <FormField label="最迟期限" required>{p => <Input {...p} name="end" type="time" defaultValue="18:00" />}</FormField>
        <FormField label="提出原因" wide hint={long ? LONG_WORD : undefined}>{p => <Input {...p} name="reason" defaultValue={long ? LONG : '想一起放松一下'} />}</FormField>
      </FormSection>
    </EditSheet>
  );
}

const TAB_LABELS = ['当天安排', '接下来几天', '意向', '约定', '生活倾向'];
function TabsDemo({ state }: { state: string }) {
  const labels = state === 'long' ? [...TAB_LABELS, '需要提前确认的重要约定', '历史记录与原始请求'] : TAB_LABELS;
  return (
    <ResponsiveTabs label="日程分区" defaultValue={state === 'long' ? 'tab-6' : 'tab-0'} onValueChange={v => record('tab', v)}
      items={labels.map((label, index) => ({ value: `tab-${index}`, label, disabled: state === 'disabled' && index === 2, content: <p className="py-3 text-sm">{label}：{state === 'long' ? LONG : '这里是该分区的内容。'}</p> }))} />
  );
}

function FormDemo({ state }: { state: string }) {
  const long = state === 'long', disabled = state === 'disabled';
  return (
    <form className="ui-stack" onSubmit={e => { e.preventDefault(); record('form-submit', Object.fromEntries(new FormData(e.currentTarget))); }}>
      <fieldset disabled={disabled} className="ui-stack m-0 min-w-0 border-0 p-0">
        <FormSection title={long ? LONG : '基本信息'} description={long ? LONG_WORD : '名称在列表和通知中显示。'} columns={2}>
          <FormField label={long ? LONG : '显示名称'} required error={state === 'error' ? '名称不能为空' : undefined}>{p => <Input {...p} name="name" defaultValue={state === 'error' ? '' : long ? LONG_WORD : '小雨'} />}</FormField>
          <FormField label="称呼" hint={long ? LONG : '她如何称呼你。'}>{p => <Input {...p} name="nickname" defaultValue="你" />}</FormField>
          <FormField label="简介" wide>{p => <Textarea {...p} name="bio" rows={3} defaultValue={long ? LONG : ''} />}</FormField>
        </FormSection>
      </fieldset>
      <FormActions status={state === 'error' ? '有 1 项需要修改' : disabled ? '当前账户只读，不能保存' : '修改尚未保存'}>
        <Button type="reset" variant="outline" disabled={disabled}>还原</Button>
        <Button type="submit" disabled={disabled}>保存修改</Button>
      </FormActions>
    </form>
  );
}

function TaskDemo({ state }: { state: string }) {
  const s = (state === 'long' ? 'failed' : state) as AsyncTaskState;
  return <AsyncTaskStatus state={s} message={state === 'long' ? LONG : s === 'failed' ? '图片服务没有在 120 秒内返回结果。' : '为 2026-11-17 规划日程。'}
    progress={s === 'running' ? 60 : undefined} updatedAt="更新于 2026-11-16 10:30（北京时间）"
    onRetry={() => record('retry', 'task')} onCancel={() => record('cancel', 'task')}
    details={s === 'failed' ? <pre className="whitespace-pre-wrap break-all text-xs">{state === 'long' ? LONG_WORD.repeat(3) : 'provider_timeout after 120s'}</pre> : undefined} />;
}

const FEEDBACK_TITLES: Record<string, string> = { information: '日程按北京时间显示', 'action-required': '有 2 项建议等待你确认', warning: '部分记录暂未读全', error: '保存失败' };
function FeedbackDemo({ state }: { state: string }) {
  const kind = (state === 'long' ? 'warning' : state) as FeedbackKind;
  return <Feedback kind={kind} title={state === 'long' ? LONG : FEEDBACK_TITLES[kind]}
    actions={kind === 'action-required' || kind === 'error' || state === 'long' ? <Button type="button" variant="outline" onClick={() => record('feedback-action', kind)}>{kind === 'error' ? '重新保存' : '去处理'}</Button> : undefined}
    details={state === 'long' ? LONG_WORD : undefined}>{state === 'long' ? LONG : '这是一条固定的示例反馈文本。'}</Feedback>;
}

function PageDemo({ state }: { state: string }) {
  const long = state === 'long';
  return (
    <PageContainer eyebrow="生活与约定" title={long ? LONG : '我们的约定'} description={long ? LONG_WORD : '提出、接受、实际执行和履约分别记录。'}
      actions={<><Button type="button" variant="outline" onClick={() => record('refresh')}>刷新</Button><Button type="button" onClick={() => record('propose')}>和她约定</Button></>}>
      <PageSection title="进行中" description="履行以约定窗口内的事实及评估为依据。" actions={<Button type="button" variant="ghost">查看全部</Button>}>
        <Records state={state} />
      </PageSection>
    </PageContainer>
  );
}

/* The acceptance proof: a brand-new settings form composed only of shared
   patterns. No page CSS, no layout decisions beyond choosing components. */
function NotificationSettings({ state }: { state: string }) {
  const [open, setOpen] = useState(true);
  const [digest, setDigest] = useState('daily');
  const busy = state === 'busy';
  return (
    <EditSheet open={open} onOpenChange={setOpen} trigger={<Button type="button" variant="outline">通知偏好设置</Button>}
      title="通知偏好设置" description="关闭某一类提醒后仍保留通知记录。" submitLabel="保存设置" busy={busy}
      error={state === 'error' ? '设置没有保存：免打扰结束时间需要晚于开始时间。' : undefined}
      onSubmit={data => { record('settings-submit', Object.fromEntries(data)); setOpen(false); }}>
      <FormSection title="提醒方式">
        <FormField label="她主动发来消息时提醒我" inline>{p => <Switch {...p} name="proactive" defaultChecked />}</FormField>
        <FormField label="新的动态发布时提醒我" inline>{p => <Switch {...p} name="moment" />}</FormField>
        <FormField label="约定即将到期时提醒我" inline hint="提前 2 小时提醒。">{p => <Switch {...p} name="promise" defaultChecked />}</FormField>
      </FormSection>
      <FormSection title="免打扰时段" description="时段内只记录通知，不弹出提醒。" columns={2}>
        <FormField label="开始" required>{p => <Input {...p} name="quietStart" type="time" defaultValue="23:00" />}</FormField>
        <FormField label="结束" required error={state === 'error' ? '结束时间需要晚于开始时间' : undefined}>{p => <Input {...p} name="quietEnd" type="time" defaultValue={state === 'error' ? '22:00' : '07:30'} />}</FormField>
      </FormSection>
      <FormSection title="摘要">
        <FormField label="摘要频率">{p => <><ChoiceSelect {...p} value={digest} onChange={(e: any) => setDigest(e.target.value)}><option value="off">不发送</option><option value="daily">每天一次</option><option value="weekly">每周一次</option></ChoiceSelect><input type="hidden" name="digest" value={digest} /></>}</FormField>
        <FormField label="备注" hint="只在本设备显示。">{p => <Textarea {...p} name="note" rows={3} />}</FormField>
      </FormSection>
    </EditSheet>
  );
}

function MigratedDetailDrawer({ state }: { state: string }) {
  const [open, setOpen] = useState(true);
  const long = state === 'long';
  if (!open) return <Button type="button" variant="outline" onClick={() => setOpen(true)}>重新打开</Button>;
  return (
    <DetailDrawer title={long ? LONG : '人物来源'} testId="migrated-drawer" onClose={() => setOpen(false)}
      footer={<Button type="button" variant="outline" onClick={() => setOpen(false)}>完成</Button>}>
      <div className="ui-stack"><Details state={state} /><Records state={long ? 'long' : 'normal'} /></div>
    </DetailDrawer>
  );
}
function MigratedDrawerFrame({ state }: { state: string }) {
  const [open, setOpen] = useState(true);
  const long = state === 'long';
  if (!open) return <Button type="button" variant="outline" onClick={() => setOpen(true)}>重新打开</Button>;
  return (
    <DrawerFrame onClose={() => setOpen(false)}>
      <aside data-testid="person-drawer" className="drawer-form ui-stack" aria-label="小雨">
        <div className="ui-actions"><Button type="button" variant="outline" onClick={() => setOpen(false)}>关闭</Button></div>
        <h2>{long ? LONG : '小雨'}</h2>
        <p>{long ? LONG_WORD : '朋友 · 人物 · 活跃'}</p>
        <p>{long ? LONG : '喜欢在周末逛花市。'}</p>
      </aside>
    </DrawerFrame>
  );
}

// Owner diagnostic records as /api/diagnostic-records/{kind} returns them (raw snake_case rows).
const TURN = 'turn-fixture-0001';
const at = (second: number) => new Date(Date.UTC(2026, 10, 16, 2, 29, second)).toISOString();
function modelRun(n: number, patch: Record<string, unknown>): TurnRecord {
  const created = at(n * 6), seconds = Number(patch.seconds ?? 2.4);
  const data: Record<string, unknown> = {
    id: `mr-${n}`, correlation_id: `turn:${TURN}`, logical_run_id: `run-${n < 3 ? 1 : 2}`, model_call_id: `mc-${n}`, sequence: n < 3 ? n : n - 2, round_count: 2,
    status: 'completed', stage: 'final_response', model_id: 'qwen3-32b', fluctlight_id: 'fl-linxia', queued_at: created, created_at: created, started_at: created,
    completed_at: new Date(Date.parse(created) + seconds * 1000).toISOString(), latency_ms: Math.round(seconds * 1000),
    prompt: { messages: [{ role: 'system', content: '你是林夏。' }, { role: 'user', content: '今天去看展了吗？' }] }, response: { content: '去了，人不多，下次一起去。' }, ...patch,
  };
  delete data.seconds;
  return { id: `mr-${n}`, ref: `mr-${n}`, kind: 'model-runs', occurred_at: created, data };
}
const agentRun = (id: string, agent: string, patch: Record<string, unknown> = {}): TurnRecord =>
  ({ id, ref: id, kind: 'agent-runs', occurred_at: at(0), data: { fluctlight_id: 'fl-linxia', agent_id: agent, run_id: id, correlation_id: `turn:${TURN}`, status: 'completed', source: 'agent_runs', started_at: at(0), ...patch } });
function turnRecords(state: string): { modelRuns: TurnRecord[]; agentRuns: TurnRecord[] } {
  if (state === 'empty') return { modelRuns: [], agentRuns: [] };
  if (state === 'long') return {
    modelRuns: [modelRun(1, { role: 'fact_only_reply', status: 'failed', stage: 'failed', seconds: 41.5, response: undefined, model_id: LONG_WORD, error_code: 'schema_mismatch', error: `${LONG}。${LONG_WORD}`, prompt: Array.from({ length: 40 }, (_, i) => `第 ${i + 1} 段：${LONG}`).join('\n') }),
      modelRun(2, { role: 'fact_only_reply', seconds: 12, response: Array.from({ length: 30 }, (_, i) => `输出 ${i + 1}：${LONG}${LONG_WORD}`).join('\n') })],
    agentRuns: [agentRun('ar-long', LONG_WORD, { status: 'failed', failure_code: 'agent_run_failed', failure_stage: LONG_WORD })],
  };
  // Returned newest first on purpose: the timeline orders by recorded time.
  return {
    modelRuns: [
      modelRun(4, { role: 'fact_only_reply', seconds: 3.1 }),
      modelRun(3, { role: 'fact_only_reply', status: 'failed', stage: 'failed', seconds: 30, response: undefined, error_code: 'provider_unavailable', error: 'provider HTTP 503: upstream overloaded' }),
      modelRun(2, { scenario: 'tools.select', role: 'kev_decision', seconds: 0.9, response: { tools: ['wardrobe.read'] } }),
      modelRun(1, { role: 'conversation_cognition', seconds: 2.1 }),
    ],
    agentRuns: [agentRun('ar-1', 'conversation_cognition', { status: 'failed', failure_code: 'agent_run_failed', failure_stage: 'reply' }), agentRun('ar-2', 'reflection')],
  };
}
function turnReader(state: string): TurnRecordReader {
  return async kind => {
    record('turn-read', kind);
    if (state === 'loading') return new Promise(() => {});
    if (state === 'read-error') throw new Error('诊断服务暂时没有响应（fixture）。');
    const rows = turnRecords(state);
    return { items: kind === 'model-runs' ? rows.modelRuns : rows.agentRuns, next_cursor: '' };
  };
}
function TurnDetailsDemo({ state }: { state: string }) {
  const [open, setOpen] = useState(true);
  if (!open) return <Button type="button" variant="outline" onClick={() => setOpen(true)}>重新打开</Button>;
  return <TurnDetailsDrawer turnId={state === 'long' ? `${TURN}-${LONG_WORD}` : TURN} read={turnReader(state)} onClose={() => setOpen(false)} />;
}

const RENDER: Record<GalleryId, (state: string) => ReactNode> = {
  'turn-details': s => <TurnDetailsDemo state={s} />,
  'migrated-detail-drawer': s => <MigratedDetailDrawer state={s} />,
  'migrated-drawer-frame': s => <MigratedDrawerFrame state={s} />,
  page: s => <PageDemo state={s} />,
  'detail-sheet': s => <SheetDemo kind="detail" state={s} />,
  'edit-sheet': s => <SheetDemo kind="edit" state={s} />,
  tabs: s => <TabsDemo state={s} />,
  form: s => <FormDemo state={s} />,
  'task-status': s => <TaskDemo state={s} />,
  feedback: s => <FeedbackDemo state={s} />,
  'record-list': s => <Records state={s} />,
  'detail-fields': s => <Details state={s} />,
  'settings-form': s => <NotificationSettings state={s} />,
};
const GALLERY = Object.fromEntries(Object.entries(GALLERY_CATALOG).map(([id, e]) => [id, { ...e, states: e.states as readonly string[], render: RENDER[id as GalleryId] }])) as Record<string, { title: string; overlay: boolean; states: readonly string[]; render: (state: string) => ReactNode }>;

export function Gallery({ component, state }: { component: string | null; state: string | null }) {
  const entry = component ? GALLERY[component] : null;
  if (!entry) return (
    <PageContainer eyebrow="UI LAB · 仅开发与测试" title="共享组件" description="每个组件都可切换适用状态；数据固定，不连接服务。">
      <RecordList label="组件列表">
        {Object.entries(GALLERY).map(([id, e]) => (
          <RecordListItem key={id} title={e.title} meta={e.states.join(' · ')} actions={<Button asChild variant="outline"><a href={`?fixture=gallery&component=${id}&state=${e.states[0]}`}>查看</a></Button>} />
        ))}
      </RecordList>
    </PageContainer>
  );
  const current = state && entry.states.includes(state) ? state : entry.states[0];
  return (
    <div className="ui-stack">
      <nav aria-label="组件状态" className="ui-actions" data-gallery-states>
        <Button asChild variant="ghost"><a href="?fixture=gallery">全部组件</a></Button>
        {entry.states.map(s => <Button key={s} asChild variant={s === current ? 'default' : 'outline'}><a href={`?fixture=gallery&component=${component}&state=${s}`} aria-current={s === current ? 'page' : undefined}>{s}</a></Button>)}
      </nav>
      <div data-gallery-target data-component={component} data-state={current} className="min-w-0">{entry.render(current)}</div>
    </div>
  );
}
