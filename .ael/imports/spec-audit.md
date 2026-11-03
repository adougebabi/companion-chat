# 规范审计与 AEL 切换回执

## 结论与口径

56份规范共17,254行、1,131,908字节（约1.08MiB）。最大的8份占41.03%。全量结构统计已经完成；本轮语义核对聚焦现行技术栈、提示词限制、旧记忆层和明确历史阶段，不等同于逐条验证全部业务条款。

按段落去空白、至少120字符、跨文件比较的口径，未发现完全重复的实质段落。该结果不能排除语义重述或责任重叠，不能用来宣称所有规范均无重复。更突出的问题是长文持续追加、局部旧术语、局部相反规则与权威入口混杂。

## 迁移后的层次

- 52份旧合同继承为现行规范基线：保留原文、来源hash与范围，不赋予新的业务验收状态。
- guides/index.md与mini-memory.md：本次AEL完全切换后退役为历史记忆层资料；不是追认它们在旧系统中从未有效。
- frontend/v3-mock-acceptance-contract.md：原文已明示旧阶段，归历史，不把其中Mock-only条款作为当前阻断。
- backend/fluctlight-autonomy-contract.md：局部相反规则尚未裁定，标记conflict；相关范围新任务会被阻断，其他明确范围仍可工作。
- 一个简短项目入口与模块索引负责按需展开；没有新增一份巨型总规范，也没有擅自删除领域合同。

## 已核查的旧表述和冲突

| 来源 | 发现 | 处理 |
|---|---|---|
| README.md:147、:225 | 仍称Vue/Vite/Pinia，实际包与入口为React/TSX | README原文保留；项目入口说明以现行React合同为准 |
| README.md:92 | 固定token预算与预算拒绝 | 不恢复此机制；服从AGENTS及Prompt Composition的prompt-unlimited要求 |
| backend/fluctlight-bff-contract.md:10 | Vue/Pinia及后面的历史Fastify示例 | 附审阅说明；Go公共边界与领域所有权规则保留 |
| backend/directory-structure.md:22 | apps/web仍标Vue | 附React现行宿主说明，不重写其他目录职责 |
| frontend/state-management.md:470 | 后部仍称Pinia store、App.vue | 按文首2026-10-26 React/Valtio owner声明；保留领域草稿/时序规则 |
| backend/kev-decision-contract.md:238 | compatible Vue editor旧称谓 | 按React；毫秒超时与token上限分别处理，r22媒体默认值优先于旧章 |
| backend/fluctlight-autonomy-contract.md:783 | 12000 provider-input bounds残句 | 不作为当前token门禁；128-source查询数量不因此删除 |
| 同上文首与WakeUp矩阵 | 缺会话时拒绝/不创建，与确保会话/可创建失败的相反描述 | 标记conflict，不自动以追加时间或LLM猜测裁定权限 |

## 优先收敛的大文档

下列文件不是已判定可删除的清单。后续应逐份分离现行规则、历史说明、例子与验证记录，并以明确引用取代重复权威定义；涉及行为差异先核实，不按字数机械压缩。

| 来源 | 行数 | 字节 |
|---|---:|---:|
| .trellis/spec/frontend/workspace-replica-contract.md | 1453 | 89770 |
| .trellis/spec/backend/fluctlight-provider-contract.md | 1194 | 77966 |
| .trellis/spec/backend/structured-turn-contract.md | 895 | 64873 |
| .trellis/spec/backend/fluctlight-workflow-contract.md | 1081 | 57680 |
| .trellis/spec/backend/fluctlight-diagnostics-contract.md | 852 | 52200 |
| .trellis/spec/backend/fluctlight-autonomy-contract.md | 843 | 47595 |
| .trellis/spec/backend/goal-life-consistency-contract.md | 449 | 37286 |
| .trellis/spec/backend/fact-only-reply-contract.md | 467 | 37067 |

## 本次切换

迁移ID：`migration-7007f9d3f504fc3f4ff8`。AEL 0.1.1 / e2601f7，项目ID：`project-0fc969276648a0c75bf9`，模式active。

1070来源完成对账。原始日志/JSONL/二进制不转成规范或通过证据；保存在旧只读树和仓库外备份。工程任务队列0，未把已折叠research复活为待办。

先在隔离项目完成apply、activate、真实命令失败/修复/通过、冲突阻断和rollback；再按同一review应用真实项目。正式切换未运行旧hooks、业务测试、模型请求、数据库迁移或发布。看板没有启动。

原快照1265文件中：AGENTS.md与.codex/hooks.json按批准目的更新，6个旧发现入口已备份退役，其他1257个原文件哈希保持一致。全部.trellis原资料保留。Claude权限、Codex通用配置、用户业务规则保留；AGENTS内指向现行规范的链接已按映射更新。

用户另行要求删除的.gocache、.gocache-gateway、.gomodcache、.venv均已删除，合计10,629,873,944字节逻辑文件大小。私有.env文件保留，未重建这些目录。

业务仓库未stage/commit/push；新增AEL目录只含可审阅文本/JSON/YAML，无DB/WAL/日志/图片。完整原始计划、校验、运行结果、备份与清理明细在仓库外证据目录。

## 后续使用与恢复

`ael doctor --project .`、`ael task list --project .`；按实际任务定义目标/范围/验收，再通过$ael-work领取推进。

`ael migrate verify --project . --migration migration-7007f9d3f504fc3f4ff8`核对切换；`ael doctor`显示本机运行目录。回滚仅在明确操作时执行`ael migrate rollback --project . --migration migration-7007f9d3f504fc3f4ff8`，会先检查用户新修改与活跃Attempt；不盲目覆盖。完整运行数据导出用storage backup，Git只保留定义/正文/摘要。
