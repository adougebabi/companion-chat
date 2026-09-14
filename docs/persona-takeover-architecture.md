# Turn Takeover 与 Working Persona 架构

本文记录 `09-14-persona-takeover-working-prompt` 当前实现的运行边界。权威约束仍在 `.trellis/tasks/09-14-persona-takeover-working-prompt/design.md`；生产代码位于 `apps/core-go/internal/core/`。

## 核心链路

```text
用户消息
  → 冻结当前 turn 的状态、active profile 与上下文快照
  → A Main Cognition（Working Persona A）
  → 归一化并校验 A Candidate（未执行）
  → pure QUERY continuation？──是→ 既有无 Tools 续接路径，跳过 takeover
                         └─否→ 是否有可执行 typed takeover rule？
                                  └─否→ A
                                  └─是→ 一次 takeover Judge
                                           ├─ false / degraded → A
                                           └─ true → B Main Cognition（禁止再次仲裁）
  → 冻结最终 winner
  → winner-only Prepare / Execute / 结算 / 投递
```

冻结 payload 的阶段为 `a_frozen → arbitration_decided → b_frozen → winner_ready → executing → settled`。CAS 阶段门和版本门保证只有 `winner_ready` 或可恢复的 `executing` 才能进入执行；崩溃恢复不会重新调用 Judge，也不会把被拒的 A 候选当成事实。

## 权威与作用域

- `active_profile_id` 表示持久主导人格；`reply_owner_profile_id` 表示本轮实际发言人格。Turn Takeover 只改变后者，不写入前者。
- 持久切换仍由原有 Main 认知提出，但必须经过 E1–E5：候选授权、冻结标记、结算门、恢复门和覆盖时重置。Judge 与 takeover B 没有持久切换权限。
- A、B 都经过同一个 `normalizeTurnDecision` 和 Judge 前 candidate validator。被拒 A 的文本只存在于有界诊断 payload，不执行消息、能力、状态、关系、记忆或演化写入。
- 可见文本由 `resolveCanonicalVisibleReply` 生成期计算并冻结一次。根级 `visible_text` 优先；根字段为空时允许 `conversation.reply.text` fallback；两者冲突直接 `visible_text_source_conflict` fail closed。最终消息仍走既有 `conversation_messages` 管线。
- B 的读取、关系/目标/意图写入、记忆视角、引用索引和 Overlay 合成都按冻结 reply owner 重建；下一轮恢复持久 active 视角。

## Working Persona 与 Judge 投影

Full Persona 与已接受的 Evolution Overlay 确定性派生两种投影：

```text
Full Persona + accepted Overlay
  ├─ Working Persona：只进入当前 Main 的唯一 System Persona 段
  └─ Takeover Control View：只进入 Judge 的有限输入
```

System Persona 只包含 shared identity、当前发言人格和授权场景下的 persistent switch 信息；`takeover_rules` 不进入 Main，未激活 profile 只以 `{id,name}` roster 出现。Judge 只接收当前用户消息、必要引用、A 的冻结候选文本、短动作摘要、B 的控制投影和相关事实，不接收完整 Main Prompt、工具、全部 profiles 或完整记忆。

Working Persona 保留来源中的描述性 traits 原文；只有来源真的给出数字时才输出数值。非对象 `traits` / `expression` 会显式保留为描述性载体，绝不静默清空。`life_profile` 当前只允许 `preferences` 与 `character_constraints` 进入每轮投影，其余字段留在完整数据或任务作用域。

## 规则与迁移

Runtime 只消费这一套 typed takeover 语义：新的初始化输出放在 `personality_system.takeover_rules[]`；为兼容迁移中间态，`forced_activation` 下只有本身带齐同一组 typed 字段的对象才可映射到同一套规范 rule。每条可执行声明必须有 `id`、`kind=turn_takeover`、`version=turn-takeover.v1`、非空 `condition` 和与声明 profile ID 完全相等的 `target_profile_id`；可选 `source_profile_id` 也必须是声明 ID。

| 来源 | 当前处置 |
|---|---|
| `switching.rules[]` | 继续作为授权 Main 的 persistent switch；时间窗只分类并诊断，不执行 deterministic evaluator。 |
| `forced_activation` prose / 迁移中间态 | 原文保留为迁移证据。没有完整 typed migration 元数据时生成 `unclassified`/有界诊断；只有显式完整 typed 对象才映射到同一规范 rule，不从关键词、正则、profile name 或 condition mention 猜测 kind/target。 |
| `takeover_rules[]` | 接管唯一正式执行入口；规则 ID、版本、target 在初始化、归一化、冻结和恢复中保持一致。 |

仓库的 `dense_multi_card.txt` 保留「公开质疑时星火接管」的 prose 断言。`dense_multi_typed_migration.json` 是固定的授权迁移 Provider 输出，验证同一语义可形成一条 typed executable rule；它不代表真实 Provider 已产出该字段。真实初始化产出需显式启用 live Provider 测试。

## 调用预算与失败边界

- 普通无规则轮：A 一次；有规则且 Judge 保留 A：A + Judge；接管：A + Judge + B。Judge 单独按 `takeover_judge` 角色计时计费，零重试。
- 结果依赖型 pure QUERY continuation 保留原有 A + 一次无 Tools 合成，并跳过 takeover；QUERY + ACTION 混合候选归一化为 `final`，可以仲裁但不能假装查询结果已知。
- B 若再次要求 pure QUERY continuation，进入 `takeover_reply_budget_exhausted` 受控失败，不调用第三次主模型、不发送被拒 A。
- Overlay clone、load 或 compose 失败会记录稳定错误码，跳过 Judge 并走 `judge_degraded` 保留 A；只有明确“无 Overlay 记录”才使用 baseline。

## 证据与限制

`apps/core-go/internal/core/testdata/turn_path_cost_report.json` 是 Fake Provider wire capture，给出四条路径的估算输入 Token、脚本输出 Token、字符数、字节数和请求序列：plain `23,439`、Judge 保留 A `25,379`、接管 B `48,342`、pure QUERY `30,343` 估算输入 Token。`life_profile_before_after_report.json` 是 dense-shaped synthetic fixture 的局部 allowlist 对照。两者都不能代表真实模型 Token、缓存命中、首条可见延迟或同一真实卡的 full-request before/after。

本版明确未实现 deterministic 时间窗 Runtime evaluator；真实 Provider 的初始化 typed rule 产出、Judge 误报/漏报、抗注入、真实计费、缓存和延迟必须在配置 live 环境后单独验收。
