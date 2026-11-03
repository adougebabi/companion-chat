# 日程正式契约定位（只读探子结果，主代理关键源已亲读）

- lifeClient 是注册 provider 透明代理，真实实现 apps/web/src/v3/providers/life.ts。
- schedules(fid,{from,to}) / detail(fid)；requests(fid,cursor) / request(fid,id)；request会验证fid。
- scheduleItem(fid,detail,item,body,operation)：要求schedule.id/revision及context.context_revision；冻结operation/scheduleId/itemId/request；自动owner evidence和idempotency。
- scheduleItemPending/retryScheduleItem/releaseScheduleItem/rebaseScheduleItem：unknown transport重放原body/key；仅首次明确409 revision_conflict可review rebase；同schedule同item，保留内容，仅替换两个版本。未知后的冲突不能释放。
- 条目建议通道 sessionStorage key fluctlight:pending-schedule:[owner,fid,item]。
- submitSchedule(fid,{localDate,expectedRevision,expectedLifeContextRevision,reason,...promise}) / schedulePending / retrySchedule / scheduleRequest。生成通道按owner/fid/date/promiseId隔离。
- 原生成表单 v3/proto/components/schedule-request.js：已有schedule即使items空也保护；修改date后提交前重新读该日并核对timezone/version；关闭观察不取消任务。源小文件63行已亲读。
- 原身份协议 v3/providers/persistent-command.mjs（60行）已完整亲读；life.ts:25-65/:100-106已亲读。
- OwnerRequestEnvelope={request,audit_id?,schedule_revision?}；request.status pending/processing/accepted/rejected/deferred/awaiting_goal/cancelled/failed/completed；不能把accepted视为日程已改。result_refs/tasks_state/tasks保留。
- Core没有Owner直接采纳/拒绝日程建议API，Demo模拟按钮必须剔除。由真实请求结果显示已接受/拒绝/失败等。
- requestDecisionText在v3/proto/components/request-failure.js可作为纯显示函数复用，原始reason不得改写。
- ScheduleRequestView={request_id,fluctlight_id,kind,local_date,timezone,status,result,error_code,task_id,...};status queued/running/completed/failed/conflict；disposition preserved/already_scheduled/scheduled区别显示。
- React暂无OwnerRequestReceipt/ScheduleRequest通用组件；原DOM requestReceipt / observeScheduleRequest都有可见性、epoch/live、bounded polling和原任务链接。新的React实现应复用正式provider而不是嵌套旧页面。
- WorkflowRecovery.tsx可复用真实allowed_actions任务恢复；不能替代业务请求状态。
- 下一步：设计React SchedulePage（日期/时间线/建议列表/生活倾向、读/编辑/规划Dialogs），保留原CAS/身份恢复；以现有provider作为写入owner。不复制固定habits、模拟采纳/拒绝或生成示例。
- fixture需要补齐date查询from/to/context_revision、合法schedule/OwnerRequest/生成请求响应；正常/empty/loading/failure服务已在4190-4193。
