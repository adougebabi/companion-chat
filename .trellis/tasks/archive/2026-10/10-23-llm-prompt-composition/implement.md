# 执行与验收计划

状态：用户已批准并完成实现与变更范围验证；工作提交已完成：`2d82c94e73e823b8001f6cb9d1f3195cbbae9b66`。使用当前分支；主会话修改代码与最终验证，子代理仅探索/核验。

## 顺序

- [x] 建立用户附件基线和 28 Agent/子任务/旁路清单，记录 `research/current-composition.md`。
- [x] PRD 收敛：token 限制冻结、前端边界、记忆/摘要机制保留；完成 design 与 manifests。
- [x] 用户确认最新方案后激活任务；重新读取 git status，保护其他 session 的改动。
- [x] 读取 before-dev 清单及将修改代码的完整函数/模块，先补字段/引用与历史语义回归；不以 byte/token 阈值作拒绝条件。
- [x] 统一 JSON renderer 和协议模块；common composer 两条入口使用同一格式；保留 native 图片、tool transcript、实际 user 原文、稳定前缀。
- [x] 修复通用源相交去重，保留同来源不同陈述、必要历史和整组 source coverage；确认用户认可的记忆优先行为未被替换。
- [x] GoalEvaluation 独立输入 profile，消除 selected goal/runtime 重复；catalog outcome 语义投影；相同 evidence data 共享正文、所有证据身份仍独立；修复任意 `_id/_ref` 递归损坏 schema/用户数据。
- [x] 按组成矩阵迁移每个 task/subtask：Studio/NPC/Planner/媒体/视觉/日记/初始化/服装等只带白名单；需要 raw ID/version 的现有绑定逐项变成 Core 绑定或写明必要例外。
- [x] 覆盖 Kev state、Owner life request、probe、Writer/ADK/persona/media correction；不修改它们的传输 envelope、工具授权或重试/提交语义。
- [x] 新增 prompt-composition 规范与 AGENTS 入口，清理现行 spec 中与已取消 token caps 冲突的条文；发布更新后的全部 Agent 组成文档。
- [x] 跑定向 Go 回归与整体 Core 检查；必要时只读子代理独立核验覆盖/泄漏，主会话汇总证据。
- [x] 交付代码变化、测试结果、真实生成输入的前后字符/条目对比及最终 Agent 清单；明确 live Provider 是否执行。

## 必须通过的行为检查

1. 注册表全部 Agent 和 shared Agent 子任务在矩阵中，raw caller 不漏；不需要人格的分析任务不注入扮演规则。
2. 静态/动态/任务数据仅 JSON；同一输入事实/规则单点拥有；native schema 和图片完整；当前输入只出现一次，合法 JSON 用户正文不被解释为内部 payload。
3. 数据 poison fixture 检查纯内部 id/version/digest/lease 不进入模型，正文与业务中的同名字段保留；必需 refs 完成 round trip、错误/跨权限 refs 仍拒绝。
4. 样本重建包含多个相同 catalog/schema、重复 outcome、重复目标/时间；catalog evidence 不带无关完整工具契约，保留每次事件与真实可用性；原生 catalog schema 仍深相等。
5. 同 source 不同 claims、不同 Actor/有效时间/可信度不合并；完整覆盖才能删整个 history group；无摘要/半覆盖/当前输入重复/真实回补与 Moment/WakeUp 历史语义均覆盖。
6. 超长真实消息、领域证据和第三轮工具结果完整保留；ordinary/stream/native/Writer 请求均不含 max_tokens/max_completion_tokens。旧配置不成为限制，规范不新增预算目标。
7. stable 前缀不因时钟/结果变化抖动，诊断以最终物理请求为准；旧 Writer 草稿 contract/hash/续接兼容。

## 验证命令

```sh
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/ai/prompt -count=1
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/core -count=1 -run 'Test(ComposeProvider|PromptAssembler|AssembledProvider|ProviderYAML|FormatProvider|GoalEvaluation|FactOnly|PrivateChat|RecentHistory|WorkingMemory|GoalPlanner|Kev|Media|VisualIdentity|Studio|NPC|Diary)'
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/core -count=1 -run '^(TestOrdinaryStructuredAndStreamSendFullInputWithoutOutputCaps|TestProviderADKSendsFullThirdToolContinuationWithoutTokenCap|TestDirectEinoRequestOmitsApplicationCompletionLimit|TestLegacyTokenSettingsDoNotInvalidateReplyConfiguration|TestPromptAssignmentsIgnoreLegacyTokenBudgetMetadata)$'
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./internal/core ./internal/ai/... -count=1
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go vet ./internal/core ./internal/ai/...
git diff --check
```

按实际新增测试名调整定向 pattern；测试名的旧 budget 用语不代表现行行为。使用受控 HTTP/native 测试；数据库用例仅任务自有临时 PostgreSQL，不访问共享生产数据。数据库 SKIP 必须如实标识，未跑真实模型不得宣称业务输出或延迟验收。未变更前端无需运行其完整构建或触碰其记录。

## 回退点与最终审查

- renderer 回退仅恢复表示，不触及 token settings。
- Goal evidence 回退保留冻结 snapshots 与 accepted submissions，绝不重做已提交副作用。
- task 迁移逐个核对输入，避免全局 remove-key/regex 改写造成无法定位的损失。
- 用户附件保持在附件路径，仓库只保留统计/合成回归，不提交个人对话原文。


## 实际验收结果

- Core/AI 6 个包通过；1,739 项测试/子测试通过，658 项按环境条件跳过。
- 全量运行发现 4 项既有失败，已在当前 HEAD 的临时只读副本复现；最终回归仅排除
  这四项。具体名称和命令见 evidence/verification-summary.json，未修改其产品逻辑。
- go vet、gofmt、git diff --check 通过。真实 Provider 与 PostgreSQL 业务验收未运行。
- 用户 prompt 重建经过实际 assembler：57,020 → 24,447 字符，减少 57.1%；
  32 个独立证据事件保留，4 条初始消息、8 个共享正文。未调用真实模型。
- 最终审查发现并修复 Owner JSON 被提升到 system、库存来源 ID 泄漏，补全
  NPC opportunity 子任务及其必要 id/revision 例外；真实初始化 HTTP 回归通过。
- 当前分支不变；前端/其他任务记录未修改。用户已确认提交计划；工作提交完成，本记录随任务归档保存。
