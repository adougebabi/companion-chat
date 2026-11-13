# M2 里程碑回执

2026-11-13，M2 状态为 **BLOCKED**。受检生产源码 `1e8dcdbd`，后续提交保存
任务状态与测试证据。分支 `codex/fluctlight-meromero-plugin-humanization`。
原 M2 起始提交 `838bb902`。最新 T03 提交后 45 个顶层回归通过、无 SKIP。

| 原 M2 验收门槛 | 已实现及已有证据 | 剩余状态 |
| --- | --- | --- |
| Qwen 未配置时默认主聊天可运行 | 默认读取已配置主模型；缺省 direct，保留显式模式选择。`m2-02-postcommit.log`、`m2-default-private` 相关测试与本轮模式切换通过 | 真实模型历史场景三次参数未满足合同，未发布。当前本机 Provider 连接拒绝，真实验收 BLOCKED |
| 主模型直接调用授权 Tool 并消费真实结果 | 正式 HandleTurn / ADK / ExecuteTool 的隔离库回归验证发现、schema、调用、回执和续轮；T03 保留相关输入与原生结构 | 真实主模型完整场景未通过，保留原失败样本 |
| 普通聊天取消固定二段生成 | default direct 的配置选模、原子回复与认知结算、claims、输出偏好、影响和关系消费已有代码及数据库证据；`TestDefaultPrivateReplyUsesConfiguredModelWithoutWriter` 验证无固定 Writer | 结构与受控回归已通过，真实主链随第一项复验 |
| Image / Moment 有生产调用入口 | 插件声明与现有领域服务注册，正式独立工具和 Agent 入口、公开动态、附图目标准入及在途恢复已有回归 | 完整真实能力验收尚未收口 |
| 无人物图片不注入人物身份 | 冻结主体/视角、允许来源及 SDXL 正向边界已有实现和受控回归，见 M2-06 | 真实无人物出图场景待验 |
| 换衣自拍依据真实穿着与媒体结果 | M2-07 在正式私聊入口核对实际穿着、冻结快照及受控媒体结果和唯一交付 | 真实主模型、渲染和视觉质量场景待验 |
| 插件停用后其他业务正常 | M2-03/10 正反场景验证 Image/Moment 独立开关、已有回执及普通聊天/文字动态/会话图片 | 受控验收通过；Moment 旧动态新图片目标语义另列非阻断待核工单 |
| 发布、附件和异步恢复有对应集成证据 | M2-04/05/08/18 验证同任务恢复、提交不确定暂停、目标准入和唯一交付；M2-05-T 使用真实 Temporal 验证暂停。M2-18 修复中断后重复图片受理及旧领取写入 | 真实媒体成功链待验；已知历史/动态正文语义场景继续保持 M2-01/11 原状态 |

M2 目标中的 Prompt/Context 原 T03 已收口。全任务声明与生产组装核对，
修复 Moment 已选外貌、其余类型证据和默认背景的实际业务值损伤，按来源
区分 Core 标识；完整验收映射见 `meromero-m2-evidence/t03-acceptance-20261113.md`。
这项完成已写回原任务，T04、T05、T06 和 M2 的原真实能力条目继续保留。

当前真实阻塞为本机文本 Provider 未监听。最终只读检查 `127.0.0.1:11234`
和 `127.0.0.1:1234` 的 `/v1/models` 均返回连接拒绝，见
`meromero-m2-evidence/m2-final-provider-preflight-20261113.log`。MLX-Serve
界面显示已选指定 MeroMero；服务控制菜单返回失效元素，服务恢复未完成。
本轮没有发起新的真实聊天推理，原真实模型失败日志继续保留。

ComfyUI 单独工作流已真实出图，见既有 `m2-comfy-direct-smoke-20261113.md`。
完整摇光动态提示词、质量、存储及附件成功链的验收仍待执行。用户指定的
多输入 LoRA 兼容问题、Moment 停用目标语义、Goal 基线失败及既有实时 token
增量流问题均沿已登记工单保留，遵循本轮暂缓非阻断问题的指示。

恢复服务后，在 `apps/core-go` 使用一次性 PostgreSQL 的
`GO_CORE_TEST_DATABASE_URL`，设置 `FLUCTLIGHT_LIVE_PROVIDER_TEST=1`、
`FLUCTLIGHT_LIVE_PROVIDER_URL` 和 `FLUCTLIGHT_LIVE_PROVIDER_MODEL`，执行
`go test ./internal/core -count=1 -run '^TestLiveDirectConversationDoesNotRequireASecondWriter$' -v -timeout=15m`。
预期测试实际执行、无 SKIP，默认主模型发布一次正式回复并完成认知结算。
模型响应若仍不符合现有合同，保留新请求、实际模型和失败样本。

既有 `TestLiveHandleTurnRealToolCallsReachDurableMediaAndReply` 仅将真实
工具链推进到受控渲染提交错误，完整 Image/Moment 的真实媒体成功链尚缺可
直接复跑的同场景验收入口与结果。该验收须核对同任务真实资产与交付。

本轮可回滚到各代码检查点前的提交，保留已提交的业务事实与在途记录。
四处原有未提交草稿的校验值持续一致，均保持未暂存；图片提交检查通过。
