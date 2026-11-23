import {InlineFeedback as OperationStatus, Feedback} from './Presentation';
import {useEffect, useMemo, useSyncExternalStore, useRef} from 'react';
import {Button} from '@/components/ui/button';
import {Dialog, DialogContent, DialogTitle, DialogDescription} from '@/components/ui/dialog';
import {createDomainCommand} from '@/state/domain-command.mjs';
import {stateLabel} from '@/v3/domain-presentation.mjs';
import {useWorkspace, afterPaint} from '../page';
import RecordEditor from './RecordEditor';
import JsonDetails from './JsonDetails';
import CommandSyncAction from './CommandSyncAction';
import {ClaimValueSummary, ProfileFieldsSummary, ProfileTime} from './ProfileRecordSummary';

const operationTextFields = {
 title: '标题',
 name: '名称',
 description: '说明',
 claim: '认识内容',
 text: '消息正文',
 excerpt: '候选摘录',
 situation: '适用情境',
 source_text: '来源消息',
 trigger_text: '适用触发词',
 triggerText: '适用触发词',
 reason: '操作理由',
 rationale: '申请原因',
 review_note: '审核意见',
 note: '审核说明',
};

function OperationRecord({value, title}: {value: any; title: string}) {
 const status = typeof value.status === 'string' ? stateLabel(value.status, '') : '';
 const learning = value.learning_enabled ?? value.learningEnabled;
 const reference = value.use_enabled ?? value.useEnabled;
 return <div className="grid gap-2">
  {status && <p>状态：{status}</p>}
  {Object.entries(operationTextFields).map(([key, label]) => {
   const text = value[key];
   return typeof text === 'string' && text.trim()
    ? <p key={key} className="whitespace-pre-wrap break-words"><b>{label}：</b>{text}</p>
    : null;
  })}
  {typeof learning === 'boolean' && <p>表达学习：{learning ? '已开启' : '已关闭'}</p>}
  {typeof reference === 'boolean' && <p>表达参考：{reference ? '已开启' : '已关闭'}</p>}
  {typeof value.source_valid === 'boolean' && <p>{value.source_valid ? '来源有效。' : '来源已变化。'}</p>}
  {value.claim && <ClaimValueSummary value={value.value} />}
  {value.identity && <ProfileFieldsSummary value={{identity: value.identity}} identity={value.identity} />}
  <ProfileTime value={value.created_at} />
  <JsonDetails value={value} title={title} downloadOnly downloadEmpty />
 </div>;
}

export default function CommandDialog({action, onClose}: any) {
 const w = useWorkspace(), owner = w.state.session.actorId;
 const model = useMemo(() => createDomainCommand({
  ...action, owner, ownerCurrent: () => w.state.session?.actorId === owner,
 }), [owner, action.identity]);
 const state: any = useSyncExternalStore(model.subscribe, model.getSnapshot, model.getSnapshot);
 const reviewHeading = useRef<HTMLHeadingElement | null>(null);
 useEffect(() => {
  if (state.review) void afterPaint(() => {
   if (reviewHeading.current?.isConnected) {
    reviewHeading.current.scrollIntoView({block: 'nearest'});
    reviewHeading.current.focus({preventScroll: true});
   }
  });
 }, [state.review]);
 const syncAvailable = action.readLatest && (!action.canReviewLatest || action.canReviewLatest(state.draft));
 useEffect(() => { model.start(); return () => model.dispose(); }, [model]);
 useEffect(() => { if (state.finished && !action.keepOpen) onClose(); }, [state.finished]);

 const pending = state.pending?.value;
 const snapshot = action.snapshot;
 const latest = state.review?.object;
 return <Dialog open onOpenChange={open => !open && onClose()}>
  <DialogContent className="react-workspace max-h-[90dvh] overflow-y-auto">
   <DialogTitle>{action.title}</DialogTitle>
   <DialogDescription>{action.description || '核对当前输入后提交。原提交结果未知时，会保留原内容与身份。'}</DialogDescription>
   <div className="grid gap-4">
    {state.error && <Feedback kind={state.readbackPending || state.pending && !state.pending.rejection ? 'action-required' : state.needsRebase || state.pending?.rejection ? 'warning' : 'error'}>{state.error}</Feedback>}
    {state.notice && !state.error && <OperationStatus kind="information">{state.notice}</OperationStatus>}
    {state.draft && <form className="grid gap-4" onSubmit={event => { event.preventDefault(); void model.submit(); }}>
     {action.fields ? action.fields(state.draft, model.edit) : <RecordEditor
      value={Object.fromEntries((action.editable || ['reason']).map((key: string) => [key, state.draft[key]]))}
      fieldLabels={action.labels || {reason: '操作理由'}}
      onChange={(value: any) => model.edit(value)}
     />}
     <div className="flex flex-wrap gap-3">
      <Button type="submit" disabled={state.busy || !!state.pending || state.readbackPending || state.needsRebase || state.finished}>{action.submitLabel || '确认提交'}</Button>
      {state.pending && !state.pending.rejection && <Button type="button" variant="outline" disabled={state.busy} onClick={() => void model.submit(true)}>核对或重试原提交</Button>}
      {state.pending && !state.pending.rejection && action.reconcilePending && <Button type="button" variant="outline" disabled={state.busy} onClick={() => void model.checkPending()}>查询原提交状态</Button>}
      {syncAvailable && <>
       <CommandSyncAction state={state} onReview={model.reviewLatest} />
       <CommandSyncAction state={state} onReview={model.reviewLatest} placement="recovery" />
      </>}
      {state.readbackPending && <Button type="button" variant="outline" disabled={state.busy} onClick={() => void model.finishReadback()}>只读恢复页面结果</Button>}
     </div>
    </form>}
    {(pending != null || snapshot != null || latest != null) &&
     <p className="text-sm text-muted-foreground">这项操作的完整内容可在诊断记录中下载。</p>}
    {pending != null && <OperationRecord value={pending} title="原始提交记录（诊断）" />}
    {snapshot != null && <OperationRecord value={snapshot} title="操作打开时的原始记录（诊断）" />}
    {state.review && <section className="grid gap-3">
     <h3 ref={reviewHeading} tabIndex={-1} className="font-medium">核对最新对象</h3>
     {latest != null && <OperationRecord value={latest} title="原始核对记录（诊断）" />}
     <Button type="button" disabled={state.busy} onClick={() => model.confirmLatest()}>已核对，采用最新基线</Button>
    </section>}
   </div>
  </DialogContent>
 </Dialog>;
}
