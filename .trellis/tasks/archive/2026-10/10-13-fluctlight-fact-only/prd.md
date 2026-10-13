# Fact-only 双模型私聊与可选视觉理解接入

## 目标

在真实 Fluctlight 私聊里保留 Qwen 的认知与原生工具执行能力，由独立 Cydonia 回复模型直接读取权威人格、Goal、关系、状态与真实历史，自主生成角色回复。功能必须可关闭、可独立构建并可切回稳定镜像；视觉理解是默认关闭的独立可选能力。

## 来源与授权

- 用户请求：开始处理 `/Users/vinson/Downloads/fluctlight-fact-only-codex-tasks.md`，允许创建任务。
- 文档作为需求来源，原文保存在 `source-requirements.md`；其中执行建议需结合仓库证据落实。
- 已获任务创建、接入盘点与隔离分支授权；实施需依 Trellis 规划审阅门禁进入。
- 功能分支为 `codex/fact-only-reply-vision`，从已 fetch 的 `origin/master`（`e36b3be8b2efb91e02fbed2771352f49557a60df`）建立。本地 master 的 `6e11490c381fbb61cad73dda686b35846eda2f06` 已保留。
- 不合并 master、不覆盖稳定 latest、不自动部署真实环境。

## 已确认的仓库事实

- 生产私聊的原生 Tool 已先提交真实消息再回填，最终认知不会再次发布：`apps/core-go/internal/core/agent_result_adapter.go:14,287,543`、`adk_conversation_runtime.go:263,307`。
- reply 仍接收主模型的 text，且回填隐藏正文：`apps/core-go/internal/core/tool_contract.go:196,221`。
- 当前所有非 embedding Provider 角色都映射 generic_llm，视觉角色尚不独立：`apps/core-go/internal/core/diagnostics.go:179`。
- 实际图片输入在媒体 quality、VisualIdentityAgent 的 review 阶段及旧可直调 VisionTask；用户附图目前只保存 refs。详见 `research/vision-media.md` 的全入口与阴性检索清单。
- Compose 只有 Core/Web 两个应用镜像，CORE_GO_IMAGE 同时用于 API/Worker/migrate/cutover；当前功能分支不匹配 CI push filter，master 的 workflow_dispatch 不发布镜像。详见 `research/release-baseline.md`。
- 代码迁移 head 为 0060_media_kev；真实部署数据库版本、稳定镜像 digest 和私有配置备份尚未确认。本机 gh 不存在，Docker daemon 不可用。

## 要求

1. FO-00/01：隔离功能分支，记录稳定基线，增加独立测试镜像渠道及真实回滚步骤。
2. FO-02：legacy/fact_only 默认兼容旧行为；回复模型、采样、预算、超时、并发、失败策略与视觉模型独立配置，run 内固定快照。
3. FO-03：从结构化权威来源装配有预算的 ReplyContext；保留人格、当前消息和有效 Goal；不传 Qwen 社交指导、内部认知合同或原始证据标识。
4. FO-04/05：保持 conversation.reply 与原生 ADK，回复工具真实生成并走既有提交/发送入口，结果回填 Qwen 后完成原认知合同；保持权限、幂等、取消及发送状态语义。
5. FO-06：媒体意图保存为草稿并回交现有工具，完成后恢复同一回复；不伪造图片、附件、送达或目标完成。
6. FO-07：视觉 disabled/unconfigured/ready/unavailable 独立解析；治理所有实际视觉入口，关闭时请求数为零且非视觉初始化、生图、视频继续。
7. FO-08/09：有限重试、独立上下文预算、可追踪 Provider 状态与隐私保护；沿用诊断中心查看真实 Writer 输入和实际发送内容。
8. FO-10/11：原文 T01–T20 验收矩阵全量保留，分别记录单测、真实 registry/ADK/发送集成、真实模型、远端 CI 与部署回滚是否执行；提供配置、报告、视觉清单、trace、发布清单与操作手册。

## 不在范围内

不重做 direct/guided/fact-only 实验平台，不训练 LoRA，不更换 Eino/ADK，不重新建设 Goal/Memory/Persona/物品/日程；首轮仅私聊和走同一私聊入口的主动行为，群聊/朋友圈保持原契约；不新增表情包工具，不迁移 registry，不修改用户模型常驻/内存策略，不加入生产固定十轮或每次必回规则。

## 验收

- 普通私聊链路可追踪到 Qwen 原生 reply ToolCall → Cydonia → 现有发送提交 → Tool result → 原认知合同，无二次发送。
- Writer 输入包含权威人格/Goal/状态与真实历史，Qwen 不能通过自由文本、字段别名或证据推断夹带社交策略。
- 媒体成功、失败、异步、发送部分成功与恢复均保持真实状态，不重复生成或发送。
- 视觉关闭或未配置时没有视觉 HTTP 请求，skip 与通过不同，且不阻塞非视觉初始化或关闭媒体生成。
- 旧配置默认 legacy；模式/模型错误明确，降级仅显式启用并可辨认；取消、截断、OOM 和认知失败不产生假成功。
- 功能分支测试镜像与稳定 latest 隔离，所有受影响服务同一发布清单；回滚真实切换镜像引用并恢复兼容配置。
- 验证证据按真实执行层级填写，未执行环境验收不可用 mock 或 YAML 校验替代。

## 子任务与验收归属

| 子任务 | 需求范围 | 独立验收 |
| --- | --- | --- |
| FO-A：10-13-fact-only-release | FO-00/01 | T18/19 构建隔离与回退基线 |
| FO-B：10-13-fact-only-config-context | FO-02/03、FO-08预算 | T01/04/05/06/07/17 配置、权威上下文及队列 |
| FO-C：10-13-fact-only-private-reply | FO-04/05 | T02/03/10/11/16/17 原生私聊闭环 |
| FO-D：10-13-fact-only-media-handoff | FO-06 | T07/08/09/10/19 媒体回交、组件重试与恢复 |
| FO-E：10-13-fact-only-optional-vision | FO-07 | T12/13/14/15/19 全入口 gate与初始化 |
| FO-F：10-13-fact-only-acceptance | FO-08/09/10/11 | T20 + T01–T20 联合验收与交付 |

父任务负责需求原文、跨子任务合同与最后联合审阅，不作为默认产品实现目标。技术顺序与文件边界在 design.md / implement.md；具体测试逐项记录在 acceptance-matrix.md。

## 限制与延期验证

回复 envelope 对自然表达的影响、双模型常驻实际内存、真实10–20次稳定聊天、独立视觉模型支持、远端双镜像发布与测试环境回滚必须实测；不将预算估计或mock替代体验及部署证明。当前附件没有用户已测采样参数或可确认的模型服务地址，实施采用独立可配置参数，不自动更换模型/量化或加载第三模型。环境信息是后续实测前置条件，不改变功能交付范围。

## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
