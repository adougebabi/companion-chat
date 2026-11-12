# R0 恢复核对

## 1. 现场与保全状态

- 核对时间：2026-11-12，Asia/Shanghai。分支为 `codex/fluctlight-meromero-plugin-humanization`；开始 R0 时实际 HEAD 为 `a45c4a3d98be602ec4a430db47b9da0a9a39ae92`。`4f2cf83c` 是其祖先提交，不是本次受检 HEAD，也不因存在提交而视为验收通过。
- 原始实施基线为 `ffa808bc81cc4d800376432216f3ea679da8de30`，已核对它是上述 HEAD 的祖先；二者之间有 122 个提交，涉及 411 个文件、33,619 行新增和 5,589 行删除，包含代码、测试、文档及 AEL 记录。数量不代表完成度。旧报告主要受检的 `42b43abf` 和 T02 场景报告受检的 `604a28aa` 均不是当前 HEAD。
- R0 开始时没有暂存文件，也没有未跟踪文件；以下四个已有改动均为已跟踪文件的未暂存修改，R0 没有覆盖或提交它们：
  - `apps/core-go/internal/core/agent_model_routing_test.go`
  - `apps/core-go/internal/core/media_capture_plan.go`
  - `apps/core-go/internal/core/media_prompt_branches.go`
  - `apps/core-go/internal/core/task_prompt_sources_test.go`
- 已保全至权限受限的本机目录 `/private/tmp/fluctlight-r0-20261112-8PoUA9/`：`committed.bundle` 包含上述 HEAD 的可达提交历史，`git bundle verify` 通过；`working-tree.patch` 保存四处未暂存改动，`index.patch` 保存空的暂存状态，`status.txt` 和 `manifest.json` 保存清单与 SHA-256；未跟踪清单为空。补丁反向适用检查通过，R0 核对后工作树补丁与开始时一致。恢复前仍应核对目标工作区及补丁，不直接覆盖现有改动。该目录是本机临时备份，**不是持久或异地备份**；忽略文件及用户数据没有被打包，也没有被本轮操作。
- 当前**无已保存 `conversation.reply` 配置**时，`defaultConversationReplySettings` 选择 `fact_only`；正常私聊在主 Agent 的 `conversation.reply` 后仍进入独立 Writer。配置可显式选 `direct`，旧草稿、独立工具和 WakeUp 仍按各自旧合同处理。它不意味着两次物理调用一定使用不同模型。依据：`apps/core-go/internal/core/reply_vision_settings.go:43`、`private_reply_run.go:65`、`:75`、`:97`、`fact_only_reply_capability.go:48`、`:420`。这里是代码缺省值，不宣称已读取用户实际保存的配置。
- R0 没有修改业务代码、数据库结构、运行默认配置或前端功能；没有连接生产业务库。验证在仓库外的两个 Git 导出副本及本机回环专用 PostgreSQL 中进行，测试自行创建并清理临时数据库。为记录调用顺序，对两个临时副本的同一个测试文件加入完全相同的一行日志；为重跑 direct 红测，在另一个临时副本中加入仅测试用探针并显式设置 `direct`。这些验证文件和日志只在上述临时目录，不属于工作区业务 diff。

## 2. 关键缺口分类与证据

以下结论均针对本节注明的代码或受检版本；未执行的测试不算通过。`EVIDENCE_MISSING` 不自动意味着外部服务已阻塞。

| 分类 | 事实、范围与证据 |
| --- | --- |
| 已有局部实现及证据 | T02 的 `agentModelBindingCandidates` 按 variant → Agent → 用途选择；视觉及 embedding 走独立用途，配置保存复用 `model_roles`。见 `apps/core-go/internal/core/agent_model_routing.go:114`、`provider.go:137`、`operations.go:889`。在 **a45c4a3d 干净源码**、隔离 PostgreSQL 和受控 Provider 上，`TestAgentModelBindingCandidatesPreferVariantThenAgentThenDefault`、`TestAgentModelBindingSaveSelectAndFrozenRun`、`TestAgentModelConfigurationSavesReadsRunsAndClears`、`TestReplyRefusalAndFailedMediaClaimNeverInvokeFallbackModel` 均通过，日志 `t02-controlled-head.log`。模型名在这些受控请求中只是被断言的 ID，不是真实模型推理。 |
| `EVIDENCE_MISSING` | 真实 Qwen 的历史反思测试通过（`/private/tmp/fluctlight-t02-qwen-reflection-live.log`），但 `formal_agent_e2e_helpers_test.go:107` 的夹具将同一模型配置到多个用途；它没有证明“只覆盖一个任务，其他任务仍实际使用默认模型”。当前受控测试证明配置隔离，不能替代这一真实同场景证据。两份证据分别来自不同测试和历史受检版本。 |
| `EVIDENCE_MISSING` | 已有新媒体快照、旧媒体诊断、旧衣柜显式恢复和认知续接的代表性隔离验证；本轮在 a45c4a3d 复跑 5 项均通过，日志 `t02-snapshots-head.log`。续接测试在观察原模型请求后故意让 Provider 失败，不能证明最终媒体交付（`reply_continuation_model_snapshot_integration_test.go:14`、`:95`）。这些测试使用受控 Provider，不代表全部后台入口、真实 Temporal 或混合版本恢复已通过。 |
| `POLICY_REQUIRED`；旧绑定一致性缺口 | 没有可靠快照的旧任务**没有统一策略**。旧媒体、部分 Goal/视觉等路径保留既有重新选模行为并记录 `legacy_unfrozen`；`media_model_snapshot.go:41`–`:54` 每次可读取当前绑定，不能保证旧任务仍使用原模型。旧衣柜准备、Diary 和 Studio 缺失可证明快照时分别明确失败或要求显式新尝试（`wardrobe_model_snapshot.go:24`–`:40`、`diary_model_snapshot.go:47`–`:50`、`studio_model_snapshot.go:21`–`:37`）。是否维持旧媒体兼容选模，或改为保留现场并显式恢复，仍须结合原 T02 和既有合同确定；不能虚构旧模型，也不能在本轮更改策略。 |
| `IMPLEMENTATION_MISSING` / `POLICY_REQUIRED` | 对原 T02“每次请求记录有效配置与版本”的严格持久保证，当前实现不具备：`eino_model_runtime.go:86`–`:112` 在结果返回或流打开后以 goroutine 尽力写诊断，进程中断或存储失败可能没有记录。`provider_attempt_configuration_integration_test.go:30` 验证诊断阻塞时不阻挡模型；如何同时满足不可阻塞和要求持久记录，需明确适用的记录保证与失败语义。本 R0 不扩展 Trace。 |
| `IMPLEMENTATION_MISSING` | 当前显式 `direct` 对无媒体回执的正文缺少完成声明核验。`direct_main_reply.go:110`–`:131` 校验决策与引用，`:149` 转交发布；`builtin_capabilities.go:240`–`:263` 没有向发布命令提供附件，`tool_publication.go:80`–`:86` 将空附件归一化，`:213` 写入消息，`:220` 只检查实际存在的附件。R0 在 a45c4a3d 干净源码的临时测试中显式保存 `direct`，受控 Provider 只调用 reply：写入“我已经把照片发给你了。”的空附件消息 **1** 条、媒体 ToolCall **0**、媒体 intent **0**，调用无错误；阻断断言失败。日志 `direct-unverified-claim.log`。这是受控隔离复现，不证明真实模型一定会这样说。 |
| `EVIDENCE_MISSING` | direct 同一模型响应中同时请求 `media.image.generate` 和 `conversation.reply` 的拒绝保护已有实现（`adk_conversation_runtime.go:326`–`:337`）；它不覆盖上面的零媒体调用，也不足以证明分轮状态、正文与附件全部一致。T05 的 ready 资源恢复为测试夹具（`domain_plugins_integration_test.go:134`）；真实渲染及完整在途恢复仍未验收。 |
| `EXTERNAL_BLOCKED`（历史一次尝试）/ `EVIDENCE_MISSING`（当前） | 历史 direct 真实模型请求曾遇 GPU 内存不足，亦有参数无效失败，见 `/private/tmp/fluctlight-direct-descriptions-live.log` 和 `/private/tmp/fluctlight-current-explicit-direct-live.log`；不能由历史资源失败推断当前仍阻塞。R0 未尝试真实 Temporal、ComfyUI、真实视觉或真实 UI 整链，当前应标未验证，不虚报为通过或确定不可用。本机 `127.0.0.1:11234` 的模型列表可读，Qwen 与指定 MeroMero 的独立合成短请求各返回 HTTP 200、非空内容、`finish_reason=stop`；这只确认本次基本可调用，不能证明正式 Agent 路由、结构化输出或 T04。记录见 `qwen-local-probe.json`、`meromero-local-probe.json`。 |

本报告中不以绝对路径开头的 R0 日志和临时文件均位于 `/private/tmp/fluctlight-r0-20261112-8PoUA9/`。该目录不含本轮读取的真实用户对话；历史日志和备份仍应按本地私有材料处理。

## 3. T02、T04–T06 的实际依赖关系

- **T02 可以独立收敛的部分：** 单个内部任务的模型覆盖、配置保存/回读、其他任务不受影响、角色拒绝不触发隐式换模型、视觉用途隔离，以及各自新任务快照与旧任务策略的专项验证，都不要求先将默认聊天切到 `direct`。本轮实际通过的受控测试和两种本机模型基本可调用情况见第 2 节。
- **T02 依赖 T04 的部分：** 原验收“Qwen 未配置时，已迁移文本主链能够运行”要求已迁移的主链；当前缺省仍是 `fact_only` + Writer，故不能用 T02 的普通会话样本替代 T04 的单主私聊验收。
- **T04 依赖的具体 T06 条件：** 切换默认到 `direct` 前，至少须阻止第 2 节复现的“零媒体调用仍发布已发送”，并验证对实际 exposed 媒体状态、附件和正文声明的对应关系；还要保留认知结算、幂等、旧草稿和交互等 T04 自身条件。同批调用保护不等于正文事实核验。不能把任意一条红测修复或一次模型成功当作完整 T06 验收。
- **T05 的位置：** 检验无媒体调用时的错误完成声明，不需要真实 ComfyUI 先完成；但完整图片生成、质量检查和交付，以及原 T05/E18 真实能力验收，仍需实际渲染与视觉条件。不能以 ready 夹具冒充真实生成，也不能把整个 T05 无差别列成修复零调用声明的前置条件。

## 4. 回归与基线失败的区别

- **`REGRESSION`（测试结果层面，业务回归未确认）：** R0 使用同一个本机隔离 PostgreSQL 实例、同一 Go 版本、同一 `-run '^TestR6PrivateChatDiscoveryWearReceiptAndWriter$/^wear_unavailable_fallback_repair$'` 命令，分别在 `ffa808bc` 和 `a45c4a3d` 的 Git 导出中运行。起始版本流式/非流式均 PASS，当前版本均 FAIL，日志为 `wear-baseline.log` 和 `wear-head.log`；目标测试文件的 Git blob 相同。每次测试使用各自新建数据库并应用各版本自己的迁移，**不是同一个数据库/schema**。
- **`EVIDENCE_MISSING`（业务行为层面）：** 两个版本的共享夹具不同：`test_base_test.go:506` 的当前实现额外把 `cognitive_assessment` 复制为 `conversation_reply`，起始版本没有这一步。为核对实际输入，R0 对两个临时副本的目标测试增加相同模型/任务名日志，修改后的测试文件 SHA-256 均为 `df66e689ca1e41d765eedaa360559e8c172d0cc1fd28fa2758da17a5513d08f4`。起始版本的两个 PASS 实际直接请求 `r7-fallback`，并未请求 `r5-writer`、也就没有先触发它的 HTTP 400；当前版本则在发布阶段请求 `turn-model`，被夹具当作额外 cognition，随后报 400。证据 `wear-baseline-instrumented.log`、`wear-head-instrumented.log`。因此，旧版 PASS 不能证明原测试真的覆盖了主 Writer 失败后的回退，也不能据此认定当前有效 503 回退是本轮生产回归。
- 旧夹具的 `r5-writer` 分支设为 HTTP 400（`r6_chat_wear_test.go:101`）；当前策略只允许有明确配置且属于技术故障的回退，不应为旧断言恢复 400 跨模型回退（`reply_failure_policy.go:95`–`:117`、`:210`）。历史临时 503 实验仍失败，但没有请求模型或 503 响应的证据，不能说已验证了 503 回退。要判定业务回归，须在两个版本使用可核对的等效模型绑定及同等测试输入，实际记录主 Writer → 503 → 备用模型 → 纠错的流式和非流式链；该复核不是本报告的下一张工单。
- **`BASELINE_FAILURE`（同名同表现，根因未确认）：** 既有 Goal/活动日志 `/private/tmp/fluctlight-t10-goal.log`、`/private/tmp/fluctlight-t10-life.log` 与 `/private/tmp/fluctlight-t10-goal-baseline.log` 在起始及历史当前测试中记录了相同测试名的失败，部分返回 `evaluation_cadence`。共享配置夹具及迁移链存在版本差异，日志本身没有完整环境和物理模型顺序；只能说明这些失败表现已有基线记录，不能推断相同根因或证明全程输入完全相同。本轮未修改这些行为。

## 5. 第一张恢复工单：真实单任务模型覆盖隔离

**工单状态：仅提出，未创建或执行。** 原父任务为 `meromero-t02-task-model-routing`；对应原始 T02 的“某个内部任务改为 Qwen，只影响该任务”和“保存后新请求使用新配置、其他任务不被连带改变”。本工单不降低或替代 T02 其余原始验收。

**唯一目标：** 在同一一次性隔离数据库场景中，以真实本机 Provider 证明：将 reflection 的一个 variant 覆盖为 Qwen 后，reflection 的实际生产任务请求使用 Qwen，而另一个不相关的文本任务在覆盖前后仍实际使用 MeroMero。

**当前证据与已满足的依赖：** T02 的保存/回读和变体优先级已在当前提交的受控测试通过；本机回环模型服务在 R0 时列出并分别响应这两个模型的合成请求；用户此前授权的 PostgreSQL 容器运行中且只绑定 `127.0.0.1`。真实 Qwen 的历史 reflection 场景已有单独通过记录，但没有同场景的单任务覆盖对照。模型列表及短请求不能保证正式任务一定成功；届时若服务状态改变应如实阻塞。

**允许范围：** 仅增加 `apps/core-go/internal/core/t02_live_model_override_integration_test.go` 中此场景的可选真实 Provider 测试及必要的本测试局部夹具，并保存脱敏文字证据。复用现有真实 `ConfigureProviderRole` / `ReadSettings`、reflection 的 `RunReflectionProposalTask`、另一个无外部副作用的正式文本任务入口及透明转发型请求观察；不得用伪造响应替代模型。若发现实际路由实现缺陷，本工单停止并保留证据，再单独确定修复；不为通过而重写路由。

**禁止范围：** 不改业务路由、模型默认值、数据库结构、提示词或 token 上限；不连接生产业务库、不使用真实用户内容、不调用购买、换衣、消息发布或生图；不改 T03–T12、AEL 状态或用户已有四处草稿；不把模型 ID 写死进业务代码。

**完成条件：**

1. 对每次独立运行，使用新的隔离库和合成资料，通过正式配置入口保存默认文本用途为指定 MeroMero，并只把 `agent:reflection:reflection_proposal_v2` 覆盖为 `Huihui-Qwen3.8-27B-3.8bpw`；回读配置版本、有效来源和模型，确认没有对其他任务添加覆盖。
2. 使用真实服务调用 reflection 生产任务入口并消费合法的任务结果；透明观察到实际请求选择 Qwen。覆盖前后调用同一个独立文本任务，其实际请求均选择 MeroMero，任务结果按现有合同校验；不能仅比较配置或 HTTP 200。
3. 至少进行两次独立运行，每次使用新库、新任务和真实新请求；保存各次模型、任务、来源/配置版本、结果及失败样本。不得重复读取同一缓存结果，也不得将受控 Provider 的通过写成真实模型通过。
4. 证据不含密钥、真实用户对话或完整私有 Prompt；确认其他配置未被本次覆盖修改，测试库和测试资源按夹具正常清理。若不能达成任一条件，记录失败或阻塞，不能标通过。

**测试命令、环境和预期：** 先在仅包含受检提交及该工单测试的干净源码副本设置以下非秘密环境；下列 `TestLiveT02VariantOverrideIsolation` 和次模型变量是**拟新增合同，目前尚不存在，R0 未运行该工单**。数据库连接只能通过安全方式提供给隔离测试；现有本机脚本可从 Docker 容器安全注入临时凭据，不把连接串写入日志或报告。

```sh
export FLUCTLIGHT_TEST_SOURCE=/path/to/clean/apps/core-go
export FLUCTLIGHT_LIVE_PROVIDER_TEST=1
export FLUCTLIGHT_LIVE_PROVIDER_URL=http://127.0.0.1:11234/v1
export FLUCTLIGHT_LIVE_PROVIDER_MODEL=llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4
export FLUCTLIGHT_LIVE_PROVIDER_SECONDARY_MODEL=Huihui-Qwen3.8-27B-3.8bpw

# 先确认新增测试确实被发现；不得把“没有测试可运行”当作通过。
go -C "$FLUCTLIGHT_TEST_SOURCE" test ./internal/core -list '^TestLiveT02VariantOverrideIsolation$'

# 在同一隔离容器上执行两次；每次测试建立并清理自己的数据库。
python3 /private/tmp/run-fluctlight-isolated-pg-tests.py '^TestLiveT02VariantOverrideIsolation$' ./internal/core
python3 /private/tmp/run-fluctlight-isolated-pg-tests.py '^TestLiveT02VariantOverrideIsolation$' ./internal/core
```

预期两次均实际运行、无 SKIP、退出 0，且每次都观察到 Qwen 的 reflection 请求及 MeroMero 的对照任务请求、合法任务结果和配置来源；逐次保存脱敏日志。`/private/tmp/run-fluctlight-isolated-pg-tests.py` 是本机临时辅助脚本，不是仓库内持久依赖；若不存在，应以等价的、只指向一次性本机 PostgreSQL 的安全方式设置 `GO_CORE_TEST_DATABASE_URL`，然后在干净副本执行 `go test ./internal/core -count=1 -v -run '^TestLiveT02VariantOverrideIsolation$'` 两次。

**保留的失败样本：** 不删除本报告第 2 节的 direct 红测、历史真实 direct 参数/GPU 失败，以及第 4 节换衣测试的基线、当前和模型顺序日志；本工单产生的新失败也逐次保留。**完成后仍不能宣称 T02 完成：** 迁移主链无 Qwen、旧任务快照策略、每次物理请求持久配置记录、真实 Temporal/视觉以及其他未覆盖入口仍需分别解决或验收。

**停止条件：** 本机 Provider 或隔离库不可用、无法确认请求只到回环及一次性实例、需要真实用户数据或密钥、真实任务不满足现有输出合同、路由与预期不符、或需要修改业务/Prompt/默认配置时，立即停止并记录具体失败与恢复条件；不以 Mock、放宽断言、换模型或降低父任务标准冒充通过。

## 6. 建议更正的任务状态

- **T02：建议撤销当前 `completed / verified`，改回未完成并保留原历史。** AEL 实时查询结果与 `.ael/records/meromero-t02-task-model-routing.json` 当前均为 `completed / verified`；原评审和场景报告 `.ael/outcomes/meromero-t02-routing-review.md`、`.ael/outcomes/meromero-t02-routing-scenario.md` 自身已记载旧无快照、诊断尽力写入、真实 Temporal/视觉未验收；本报告第 2 节另指出真实单任务覆盖缺少同场景证据，且原要求的迁移主链依赖未完成的 T04。历史 `pass` 及其受检版本 `604a28aa` 应保留，不能据此将原始 T02 的全部条件称为已满足。
- **T01：** 仅依据现有入口清单和其验收记录保留已完成盘点，不在 R0 重新全仓审计，也不把盘点当成真实能力验收。
- **T03–T12：** AEL 当前为 open / pending / repair；按既有记录保留部分完成或未验证，本轮不展开其他任务、不更改状态。T04–T06 的具体证据边界见第 2、3 节。
- 本轮没有改写或删除 AEL 历史，也没有领取或实施第 5 节工单；建议的状态更正留待后续按项目流程处理。
