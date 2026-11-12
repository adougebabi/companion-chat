# M2 主对话与插件迁移：当前进度

起始提交 `838bb902b18280cbe5eeb119fb500db5da30ffa2`；沿用 M1 证据和四处原有未提交草稿。用户澄清：MeroMero 是在现有设置中选择的默认文本模型，不能将模型仓库或 ID 写死；默认私聊应读取现有配置并移除固定二段式。

| 子项 | 原任务 / 可验收行为 | 状态 | 证据与下一步 |
| --- | --- | --- | --- |
| M2-01 | T04/T06：跨轮媒体话题中省略图片名的已交付声明进入现有核验 | BLOCKED | 正式路径无依据红测已修复，受控 Provider 的匹配、无关交付和普通闲聊对照通过；两次真实 MeroMero 的无依据场景均通过，但匹配交付场景未通过：首次是后端对完整引用误拦，修正后第二次模型返回空 `subject_quote`，后端保守拒绝。按用户授权不继续调 Prompt 或模型字段，保留全部证据，不标 VERIFIED。 |
| M2-02 | T04：默认私聊使用现有配置的主模型及原生工具链，不固定进入独立 Writer | BLOCKED | 未设置 mode 时默认 direct；已保存或显式选择的 fact_only/legacy 继续生效，开关保留，不写死模型 ID。隔离库与受控 Provider 已验证默认选模、variant 覆盖、模式切换、原生查询、认知结算、幂等、旧草稿和相关回归；页面测试通过。本机真实 MeroMero 默认私聊发出 3 次请求后因 `direct_reply_arguments_unresolved` 未发布，真实模型合同未通过。保留日志 `meromero-m2-evidence/m2-02-live-default.log` 与请求证据 `meromero-m2-evidence/m2-02-live-default.json`，按用户要求不调整 Prompt、采样或合同。 |

M1 中同一调用 ID 重放的非阻断缺陷已登记 AEL，按用户要求暂缓，不在 M2 顺手修复。其余 M2 验收仍按原 T03～T06 和里程碑要求逐项处理，不以局部完成替代父任务验收。

M2-02 的受检代码已保全为 `bd148af710aaffd72e761647684e0021c722ad40`。
提交后使用干净导出、无额外源码覆盖及一次性 PostgreSQL 复跑 8 个顶层 Go
用例，退出 0、无 SKIP；Web 定向 4 项退出 0、无 SKIP。日志分别为
`meromero-m2-evidence/m2-02-postcommit.log` 和 `meromero-m2-evidence/m2-02-web-postcommit.log`。
真实模型失败前的请求证据含前两次工具参数，第三次响应仅保留工具名及调用 ID
是否存在，不能当作完整原始响应；首次测试脚本不兼容本机 Python、以及切换测试
读取版本类型错误的失败和修正也分别保留在对应日志中。
