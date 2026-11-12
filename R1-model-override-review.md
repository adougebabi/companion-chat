# R1 单任务模型覆盖验证回执

## 1. 受检提交与实际测试改动

- R1 开始时分支：`codex/fluctlight-meromero-plugin-humanization`；HEAD：`7af49691e4e4fbe0038f9dc2235f2b040015f78a`。相对于 R0 受检提交 `a45c4a3d98be602ec4a430db47b9da0a9a39ae92`，只有新增 `recovery-review.md`，相关生产实现没有变化。
- 按用户要求，先尝试把父 T02 通过现有 AEL 流程恢复为未完成；该前置条件被 AEL 的终态限制阻塞。因此本轮没有创建或修改 R1 集成测试文件，没有运行模型路由 A/B/C 场景，也没有修改生产源码。
- R0 的 bundle、补丁、原清单及关键验证日志共 17 个文件已复制至仓库外私有目录 `/Users/vinson/Library/Application Support/Fluctlight/Recovery/R0-20261112-a45c4a3d-hlwhk6oa/`。原清单中的 6 项 SHA-256 全部匹配，复制后所有文件的 SHA-256 与源文件一致；复制核验见该目录的 `r1-copy-verification.json`。R0 原目录及原文件未删除。

## 2. 两轮 A/B/C 验证表

| 独立场景 | 阶段 | 配置保存与回读 | 实际结果 |
| --- | --- | --- | --- |
| 1 | A：无 variant 覆盖 | 未执行 | 未验证 |
| 1 | B：reflection 覆盖 Qwen | 未执行 | 未验证 |
| 1 | C：撤销覆盖 | 未执行 | 未验证 |
| 2 | A：无 variant 覆盖 | 未执行 | 未验证 |
| 2 | B：reflection 覆盖 Qwen | 未执行 | 未验证 |
| 2 | C：撤销覆盖 | 未执行 | 未验证 |

## 3. 每阶段任务模型与合同结果

上述六个阶段的目标 reflection 和固定对照任务均未发起请求；实际请求 model、Provider 返回 model、有效绑定来源和配置版本均未知，输出合同均未校验。本轮没有重新查询当前 Provider 的模型列表，因此 R0 历史记录中的模型 ID 不能当作本轮已确认的当前结果。

## 4. 命令、退出码、执行与证据

| 操作 | 结果 | 证据或说明 |
| --- | --- | --- |
| `git diff --name-status a45c4a3d98be602ec4a430db47b9da0a9a39ae92 HEAD` | 退出 0 | 仅 `A recovery-review.md`。 |
| R0 材料复制及 SHA-256 核对 | 退出 0 | 私有目录中的 `r1-copy-verification.json`；原 bundle、patch、manifest 和日志保留。 |
| `./ael doctor`、`./ael task list`、`./ael task show --task meromero-t02-task-model-routing` | 均退出 0 | AEL 0.1.18；T02 仍为 `completed / verified`。 |
| `./ael resume --task meromero-t02-task-model-routing --return-to execute` | 退出 1 | 返回 `TERMINAL_TASK: meromero-t02-task-model-routing`。 |
| 创建并登记 R1 工作项、提交阻塞状态 | 退出 0 | `.ael/work-items/meromero-r1-model-override-isolation.md`；状态为 open、paused、`wait_environment`。该关联未重开 T02。 |
| R1 真实 Provider 测试 | **未执行** | 测试文件尚未创建；没有可报告的测试退出码、模型结果或 SKIP 计数。“未执行”不等于通过或测试自动 SKIP。 |

## 5. 全部失败及修正记录

唯一实际失败是恢复 T02 的 AEL 命令返回 `TERMINAL_TASK`。只读核对 AEL 0.1.18 对应源码：`internal/core/service.go` 的 `Resume` 对非 `open` 任务直接拒绝，`Edit` 对 `completed` 任务同样返回终态错误；命令行和看板后端使用相同服务。已有关联反馈机制不会改变原任务的 completed 状态。

没有绕过流程修改 AEL 数据库，也没有改造 AEL；没有进行代码、测试夹具或模型输出的修正。父 T02 仍显示 `completed / verified`，其历史 pass、受检提交与原验收报告保留；R1 只作为关联工作项登记阻塞，不能冒充父状态已更正。

## 6. 允许范围外的代码是否发生修改

没有修改业务路由、默认聊天链、Prompt、采样、token 限额、回退策略、数据库结构、Temporal、媒体、发布门禁、UI 或 Trace。四个原有未暂存文件的内容在 R1 开始时与 R0 保存的补丁一致，未被本轮测试或提交覆盖；本轮未连接生产业务库。为登记 R1，AEL 在 T02 记录中追加了 R1 的反馈关联并生成 R1 自己的工作项和状态记录；这没有改变 T02 的业务定义或历史验收。

## 7. 最终状态：BLOCKED

阻塞点是用户明确要求的**执行前条件**：先将 T02 恢复为未完成，而当前 AEL 没有受支持的方式重开该终态任务。恢复条件是现有任务流程提供保留历史的合法重开方式并完成该状态更正，或用户明确调整这一前置要求；随后从 R1 的定义阶段恢复。不得通过直接修改 AEL 数据库或擅自改造 AEL 绕过。

## 8. 父 T02 仍未完成

无论历史 AEL 状态如何，T02 的真实单任务覆盖隔离、迁移主链无 Qwen 验收、旧任务无快照策略、物理请求配置记录保证以及真实 Temporal/视觉等要求仍不能整体宣称完成。本次没有执行 R1 验收，不能声称 R1、T02 或 MeroMero-first 全链通过。
