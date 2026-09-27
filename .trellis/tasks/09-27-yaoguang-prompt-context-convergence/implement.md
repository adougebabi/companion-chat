# 集成执行计划

本父任务不直接启动实施；逐一启动并完成下列子任务，最后回到父任务做联合验收。

1. Agent失败诊断：逻辑run/物理模型/Tool层级、Owner安全原因、旧行兼容。
2. 地点与认知上下文：异地主语、场景Tool授权、重复outcome/receipt/历史减重与预算。
3. 媒体拍摄视角与格式：capture贯穿、默认自摄、TOON投影与多模态text-part清理。
4. Wake-up计时：t0+10/30/每30分钟、重排、取消锁内栅栏、旧历史回放。
5. 阶段摘要与日记忆：静默摘要、当地日归并、typed Memory来源、退役/失效、70+回合连续性。
6. 最后跨层验证：Core+Worker+Browser OpenAPI/生成客户端+Web，受控异地对话/媒体/长历史/慢模型/重复队列场景；审核所有改动 specs，并核对没有覆盖同日其他任务的代码。

每个子任务按自己的 `implement.md` 和 `check.jsonl`验收，失败则停在该子任务修复，不继续扩大范围。父任务联合检查包括：新聊天取消 wake-up但不删除已完成的阶段摘要；日记忆失效后仍能靠Raw/有效阶段摘要回退；诊断能展示以上生命周期的逻辑失败与安全原因。数据库迁移/Temporal history与配置向后兼容必须由所属子任务证明。

联合验证命令：`go -C apps/core-go test ./...`、`pnpm generate`、`pnpm typecheck`、`pnpm test`、`pnpm build`；有数据和服务依赖的端到端测试在隔离环境执行，不使用共享生产库。
