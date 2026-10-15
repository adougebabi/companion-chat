# 用户真实环境验证

代理只运行本地用例、类型与构建。以下操作由你在自己的测试环境执行，不会由代理自动部署或调用真实模型。

## 配置与首次普通私聊

1. 按 `fact-only-deployment.md` 在独立数据/Temporal namespace 切换同一 release manifest 的 Core/Web digest。先保存 stable digest 与私有配置备份。
2. 设置页为通用模型、独立回复模型分别绑定真实 endpoint/model。选择 fact_only；先关闭视觉。`fact-only-config/fact-only-vision-off.json` 是设置 values 示例，采样是可调整默认值，不是已经验证的 Cydonia 最优参数。
3. 在真实私聊发送一条普通文字。聊天里应只出现一次角色消息。Owner 诊断应看到 Qwen 的原生 conversation.reply、独立 Writer 请求、真正 committed 的 Tool result、随后最终认知。最终认知正文不是第二条人物消息。
4. 途中更改模式/模型只影响后续 run。缺模型、截断、OOM 必须明确失败；只在你显式选择 fallback 后才使用认知模型作为 Writer，并标记 legacy_fallback。

## 场景矩阵

- 选择你已有的三种人格/有效 Goal，各聊至少十次：有用户配合、有明确拒绝、有中性回复。检查人格与原 Goal/完成条件保留；主认知模型不把自己的“对方冷淡”推断变成 Writer 限制。检查当前真实场景/衣物/库存，不由一句角色正文改变。
- 请求图片：第一次只保存草稿并触发真实 image Tool；排队期间不发送“照片已发”的成功正文。完成时一条 assistant 同时包含文字和实际图片。关闭视觉仍能生图并保存，inspection 是 skipped/unverified。
- 分别制造不可用媒体、异步慢任务、已生成后 Worker 重启；不重新生图，不重复 assistant。失败时 Writer 调整正文，不出现占位图片或假成功。表情包 unavailable，Unicode 🙂 保留。
- 等待媒体期间取消，或发送同会话的新消息；旧任务即使完成，也不得晚发。主动 WakeUp 续接还需保持 awake/权限/topic 并且没有新 inbound message。
- 为视觉单独绑定支持图片的模型，保存 enabled 后点击“测试视觉理解”。配置就绪不表示测试已通过。换成不支持图片的模型或超时服务，应明确 unavailable，不能回退给 generic 模型猜图。
- 创建/生成设定卡时视觉关闭：文字初始化和生成继续，candidate 可读且 awaiting_review；旧 canonical 保留。再次打开视觉不会自动集中补跑旧 skipped。直接人工评审必须标明 manual，不能生成“模型已查看”时间线。
- 验证日志/CI不含 API key、Authorization 或完整私聊；Owner诊断可看到允许的 Writer 上下文/候选与实际 Tool result。diagnostic_model_runs 按时间倒序。

## 可选本地数据库用例

使用可创建 disposable 数据库的**测试** PostgreSQL；不要设置成生产连接：

```bash
GO_CORE_TEST_DATABASE_URL='postgresql://TEST_USER:TEST_PASSWORD@127.0.0.1:5432/postgres?sslmode=disable' \
  go -C apps/core-go test -race ./internal/core \
  -run '^Test(ReplyMediaPublication|VisionOptionalReview|MediaQualityFinalProviderRequest)' -count=1 -v
```

测试里的网络响应为受控 stub，不代表真实模型体验。真实 Writer 的 formal task 入口已加入 `TestFormalAgentE2E/conversation_reply`；运行它需现有 live suite 的通用模型配置及单独的 `FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL` / `FLUCTLIGHT_LIVE_REPLY_PROVIDER_MODEL` / 可选 `FLUCTLIGHT_LIVE_REPLY_PROVIDER_API_KEY`。这只验证独立 task；上面的实际聊天 Tool/发布矩阵仍要执行。

## 回退前清理

先停止新 fact_only 运行并等当前发布事务结束；可排空已启动续接。若直接回退，停止新分支 Worker，在你的数据库事务里关闭未发布草稿与等待 source，避免旧 cognition worker 不认识 waiting_reply 后重新 claim。保留原 user、已发消息及 ready 资产：

```sql
BEGIN;
UPDATE public.cognition_inbox SET status='failed',error_code='fact_only_rollback',processed_at=now(),claimed_by=NULL,claimed_at=NULL
 WHERE status='waiting_reply';
UPDATE public.conversation_reply_drafts SET status='cancelled',last_error='fact_only_rollback',updated_at=now()
 WHERE status IN ('generating','prepared','needs_action','waiting_media','ready');
UPDATE public.conversation_reply_continuations SET status='cancelled',last_error='fact_only_rollback',completed_at=now()
 WHERE status IN ('pending','started');
COMMIT;
```

取消/排空 `reply-continuation-v1` 的在途 Temporal workflows，再恢复兼容设置/模型绑定并按部署文档显式切两个 stable refs。不要让旧迁移器自动对0062执行 downgrade；本轮仅新增兼容数据，实际旧镜像启动与运行由你确认。只重建 master 不会切换仍指向 fact-only-latest 的部署。
