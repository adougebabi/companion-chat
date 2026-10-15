# 私聊表达媒介纠偏与人格保真 T0—T9

## 目标
按用户提供的实施任务书核对现有 fact-only 实现，复用已完成组件并补回归，使 Cydonia 以当前人物亲自发送私聊消息，保留任意人格、自主社交判断、有效 Goal 和真实工具边界。

## 来源与范围
- 原始要求：`/Users/vinson/Downloads/微调混合语料方案.md`；完整副本与 hash 保存在本任务，文档中的职责示意须对应现有实现。
- 用户已在最新完整方案后明确要求实施；当前状态为 in_progress，代码已落地，验证与交付按本任务分别记录。
- 当前功能分支 `codex/fact-only-reply-vision`，基线 HEAD `b24fdd1`。已有上一任务 7 个未提交文档改动全部保留。
- 覆盖 T0—T9，逐项对应 R01—R36；不是仅替换协议的任务。

## 硬约束
- 原始 Persona / Working Persona 存储含义、已发送历史、承诺和关系状态不得改写；输入视图只读且可追溯。
- Qwen 仅认知、事实、真实工具与原合同结算，不恢复正文、润色或社交策略指导权。
- 复用既有 Slot、配置快照、人格投影、完整历史片段、draft、媒体恢复、唯一发布和认知修复；不另建 Agent 或重复组件。
- 不更换模型、采样，不训练 LoRA，不增加生产期限、静默权限或固定话术；不重构领域 / ADK / Goal / 调度。
- 留在已有功能分支，不自动合并、部署、更新真实环境或发送真实用户消息。

## 要求
| ID | 可验收行为 |
| --- | --- |
| T0 | 定位实际请求 / 生效来源 / 旧协议 / 配置 / 缓存 / 恢复路径，冻结具明确材料边界的基线。当前来源见 `reply_context.go:12`、`reply_vision_settings.go:22`、`provider_prompt_composer.go:225` |
| T1 | 两处调用显式投影 text_chat 与有来源的空间关系；unknown / same / different 分开。修复 `fact_only_context_views.go:62` 与 `proactive_topics.go:27` 的来源错位 |
| T2 | 当前私聊作用域内唯一新协议替换旧规则；独立保留 B 组只改协议对照。当前 `reply_context.go:24` 含模糊“人物正文/轻微神态”合同 |
| T3 | 回复侧只读人格视图保留任意完整原文、条件习惯、表达差异与来源；生效 profile 隔离。当前使用 `persona_compilation.go:419` 的编译 portrait；不能冒称原始导入全文 |
| T4 | 必要活动 / 时间 / Goal 可见，外貌按需提供，底层及媒体完整数据保留。当前 `fact_only_context_views.go:194` 全量展开 body_fields |
| T5 | 真实原历史 hash / role / 顺序 / 关键语义不变，来源标识和文体边界两处一致。复用 `fact_only_history.go:48`、`prompt_context_assembler.go:558` |
| T6 | 复用验证入口，可靠诊断触发最多一次同 Cydonia 发送前无工具修复，保留首稿 / 最终稿。现有 `reply_failure_policy.go:89` 格式重生成不是该修复 |
| T7 | 原媒体 / 视觉 / 状态 / 唯一提交 / 结算链路无退化，无候选事实升格或重复发布。复用 `reply_media.go:96`、`fact_only_reply_capability.go:118`、`eino_model_runtime.go:989` |
| T8 | 实际版本 / 缓存依赖 / 在途 draft 语义稳定，诊断与回退可查，测试镜像路径核验。现有 `reply_continuation.go:41` 只冻结配置/模型，发布 `fact_only_reply_capability.go:123` 读当前常量会误标旧草稿 |
| T9 | R01—R36 逐项结果；A/B/C/D 固定输入对照，真实模型与结构 / 模拟结果分列。复用 `test_base_test.go:383` wire capture，不能把裁剪 live export 当完整请求 |

## 验收
- 每项都有实际文件 / 符号落点、已存在证据或明确缺口，不以说明替代实现。
- 实际发给模型的 messages / tool schemas 与配置摘要可在受控测试和已有授权诊断中核对，不能只看前端 Prompt。
- 温知意重建 fixture 和干净 / 小说式原历史、多样人格均有来源标签；真实原日志未找到前不声称完全复现。
- 首稿、修复次数、最终发布和认知回读绑定同一 draft；默认正常回复不新增模型调用。
- 自动化测试 / 真实模型验证 / 未执行项目分别列出，缺 Provider / DB / 远端构建不计为通过。

## 已核实的验证前提
- 当前 requirements 和 repo 材料未找到温知意完整 API wire，附录明确不是等价请求；基线使用重建 fixture，缺失原始 sampling / 原响应 / 完整 3705 tokens 输入必须列出。
- 现有受控基线执行成功，53 个 test/subtest PASS 事件、18 个 SKIP 事件，详见 `verification/baseline-summary.json`。它们不是新 R01—R36 的正向验收。
- 当前进程缺隔离数据库与 live Provider 环境配置；Docker daemon 不可用。真实模型、数据库持久化、远端构建和部署均未执行。
- CI 对当前功能分支没有 paths 过滤。现存发布清单对应旧 SHA，不能代表当前 HEAD 或本任务修改版。

## 规划收敛
一个复杂任务按四个顺序切片共同验收同一消息链路，复用共享 fixture / 权限 / 发布边界，不拆成独立领域改造。无未解决产品范围问题。完整规划已获实施批准并激活；真实环境、数据库、模型和远端构建结果不得由本地模拟替代。最终结果见本任务交付报告和逐项验收表。
