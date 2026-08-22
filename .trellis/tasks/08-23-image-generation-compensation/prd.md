# 图片创建补偿作业与失败重试

## Goal

确保图片/视频生成不会因为活动投影、异步 poll 或质量重试中的进程崩溃而停在 `queued/processing`。所有补偿都通过现有 durable job、lease、retry 和媒体投影边界完成。

## Confirmed Code Facts

- 聊天媒体 `media-flow.plan/apply` 已具备 placeholder/job replay repair。
- 活动媒体 projection 先写 activity 再发布 effect，当前没有明确的 caller transaction 传入；enqueue 失败可能留下无 job 活动。
- `media-job-service.submit()` 在 source complete 后单独 enqueue poll；poll enqueue 失败可能留下 source complete + target processing + 无 poll。
- generic dispatcher 不理解媒体 target，只负责 lease/attempt/settlement；补偿应放在 media application/repository 边界。
- quality retry 当前 successor key 不足够确定，重复 tick 有重复 job 风险。

## Requirements

- 活动创建与 `activity_image` / `activity_video` effect/job 入队必须在同一 SQLite transaction，重放同一 decision 使用稳定 idempotency key。
- 为 source->poll 提供原子 follow-up 或 durable compensation：能够基于 frozen payload、external id 和 source job 修复缺失 poll job。
- 为 quality retry 和 poll successor 使用确定性 key，保证 crash/restart/retry 幂等。
- 补偿作业只修复缺失或不一致的 durable job/target，不重新生成 prompt、不重新调用 LLM 做场景推断。
- 临时 provider/network 错误继续使用通用 retry/backoff；达到最大尝试后才将 activity/message target 标记 failed 并保留有限错误信息。
- 增加活动 enqueue rollback、poll enqueue failure repair、quality retry dedupe、lease recovery 和 stale worker 测试。

## Acceptance Criteria

- [x] 活动媒体 enqueue 现在和 activity insert 共用 caller transaction，失败时不会留下半提交 activity；稳定 key 支持重放单一 job。
- [x] source job 已 complete 而 poll job 缺失时，`media_poll_compensation` job/tick 恢复唯一 poll job，图片可继续完成。
- [x] 重复补偿、worker lease、crash-after-enqueue 和 quality retry 使用确定性 key，不重复 provider submit/poll 或 successor。
- [x] provider 临时失败继续按既有 attempt/backoff 重试，terminal failure 更新用户可见 target 为 failed。
- [x] 现有聊天媒体 replay/repair、全部 media/worker/runtime 测试继续通过。

## Out of Scope

- 不在 persona analyzer 中直接创建图片或等待 provider。
- 不更换现有 ComfyUI/H3/provider 协议，不新增独立媒体队列系统。
