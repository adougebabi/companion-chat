# M2 主对话与插件迁移：当前进度

起始提交 `838bb902b18280cbe5eeb119fb500db5da30ffa2`；沿用 M1 证据和四处原有未提交草稿。用户澄清：MeroMero 是在现有设置中选择的默认文本模型，不能将模型仓库或 ID 写死；默认私聊应读取现有配置并移除固定二段式。

| 子项 | 原任务 / 可验收行为 | 状态 | 证据与下一步 |
| --- | --- | --- | --- |
| M2-01 | T04/T06：跨轮媒体话题中省略图片名的已交付声明进入现有核验 | BLOCKED | 正式路径无依据红测已修复，受控 Provider 的匹配、无关交付和普通闲聊对照通过；两次真实 MeroMero 的无依据场景均通过，但匹配交付场景未通过：首次是后端对完整引用误拦，修正后第二次模型返回空 `subject_quote`，后端保守拒绝。按用户授权不继续调 Prompt 或模型字段，保留全部证据，不标 VERIFIED。 |
| M2-02 | T04：默认私聊使用现有配置的主模型及原生工具链，不固定进入独立 Writer | BLOCKED | 未设置 mode 时默认 direct；已保存或显式选择的 fact_only/legacy 继续生效，开关保留，不写死模型 ID。隔离库与受控 Provider 已验证默认选模、variant 覆盖、模式切换、原生查询、认知结算、幂等、旧草稿和相关回归；页面测试通过。本机真实 MeroMero 默认私聊发出 3 次请求后因 `direct_reply_arguments_unresolved` 未发布，真实模型合同未通过。保留日志 `meromero-m2-evidence/m2-02-live-default.log` 与请求证据 `meromero-m2-evidence/m2-02-live-default.json`，按用户要求不调整 Prompt、采样或合同。 |
| M2-03 | T05：Image 停用时默认私聊与纯文字动态隔离 | VERIFIED | 受检提交 `1d67046e`：一次性数据库中，正式设置关闭 Image 后，默认 direct 私聊经真实原生工具调用发布纯文字动态、消费结果并回复；独立 Moment 调用及重放通过，新的图片调用被拒绝且无媒体 intent。干净提交的 10 个相关顶层受控回归通过、无 SKIP；含四处原有草稿的工作区主场景也通过。日志 `meromero-m2-evidence/m2-03-postcommit.log`、`meromero-m2-evidence/m2-03-working-tree.log`。Provider 是测试替身，真实模型和渲染不在此项证据范围内。 |
| M2-04 | T05/T06：已保存渲染 Job ID 的未完成图片跨进程恢复 | VERIFIED | 受检提交 `707ac2ad`：正式 Image 请求受理后，第一个进程在 Job ID 落库后遇到受控 503；Image 停用后第二个独立进程恢复同一 Job，经受控质量检查和存储后只交付一次，重放不重复；新图片请求被拒绝。干净提交的 7 个定向顶层回归通过、无 SKIP，含四处原有草稿的工作区主场景也通过；日志 `meromero-m2-evidence/m2-04-postcommit.log`、`meromero-m2-evidence/m2-04-working-tree.log`。渲染服务已接单但 ID 尚未落库的崩溃窗口仍为父任务缺口；受控服务不冒充真实 Temporal、ComfyUI 或模型。 |
| M2-05 | T05/T06：渲染服务已接单、Job ID 未落库时避免重复提交 | VERIFIED | 受检提交 `6ba41249`：用户选择直接暂停、等待人工核对。发送前持久化标记；丢失响应后暂停，旧任务无可靠 Job ID 也保守处理；普通媒体经明确核对后可恢复已就绪资产或明确重新生成。干净提交的 19 个 Core、6 个受控工作流、4 个迁移、3 个浏览器边界及 11 个定向页面用例通过，均无 SKIP；含原有草稿的工作区主场景通过。旧红测和提交后证据均保留在 `meromero-m2-evidence/m2-05-*.log`。Studio 可核对后显式丢弃任务，Studio/视觉身份尚无核对后接回原任务的领域入口，不能据此称完整恢复；真实 Temporal/ComfyUI 未验证。 |
| M2-06 | T05：无人物图片的 SDXL 正向文本边界 | VERIFIED | 受检提交 `d7e5f8ce`：正向 `text_g`/`text_l`、`positive_prompt` 和连接的 `PrimitiveString` 接入现有工作流事实边界。干净提交与隔离 PostgreSQL 上 12 个顶层专项及 4 个当前拍摄子场景通过，含四处原有草稿的工作区主场景通过，均无 SKIP；红测和提交后日志见 `meromero-m2-evidence/m2-06-*.log`。模型和渲染为受控替身，不代表真实生图或 T05 完整验收。`enum-rejected` 同环境基线失败单列 AEL；图片无字规则在 `PrimitiveString` 的独立问题登记为 `image-no-text-primitive-string-boundary`。 |
| M2-07 | T04/T05：direct 主链先换衣再自拍的事实与交付 | VERIFIED | 受检提交 `04ac059f`：正式 `HandleTurn` 中，实际衣柜查询回执驱动换衣，受理自拍前核对持久穿着，再核对媒体快照、受控渲染请求及附件幂等交付。干净提交的 9 个顶层回归及含四处原有草稿的工作区主场景通过，均无 SKIP，见 `meromero-m2-evidence/m2-07-postcommit*.log`；首两次测试夹具错误及修正日志保留。模型、渲染、质量审核及存储均为受控替身，不代表真实模型或真实生图验收。 |

M1 中同一调用 ID 重放的非阻断缺陷已登记 AEL，按用户要求暂缓，不在 M2 顺手修复。其余 M2 验收仍按原 T03～T06 和里程碑要求逐项处理，不以局部完成替代父任务验收。

M2-02 的受检代码已保全为 `bd148af710aaffd72e761647684e0021c722ad40`。
提交后使用干净导出、无额外源码覆盖及一次性 PostgreSQL 复跑 8 个顶层 Go
用例，退出 0、无 SKIP；Web 定向 4 项退出 0、无 SKIP。日志分别为
`meromero-m2-evidence/m2-02-postcommit.log` 和 `meromero-m2-evidence/m2-02-web-postcommit.log`。
真实模型失败前的请求证据含前两次工具参数，第三次响应仅保留工具名及调用 ID
是否存在，不能当作完整原始响应；首次测试脚本不兼容本机 Python、以及切换测试
读取版本类型错误的失败和修正也分别保留在对应日志中。

M2-05 扩大回归中，诊断页面 5 项失败及头像人工审核快照 1 项失败，在干净
基线 `b3f5cd40` 的同环境均可复现；已分别登记 AEL
`diagnostic-task-detail-baseline-regression` 和
`avatar-manual-review-snapshot-baseline-failure`，未在本项修改。
