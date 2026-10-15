# F16：Goal Evaluation 提交可用性与程序兜底

## 用户指令与现场证据

用户报告Goal Evaluation卡死、各种错误后循环：goal.plan.submit/goal_submission_conflict；两次goal.evaluation.submit/goal_evaluation_unknown_requires_evidence；goal.evaluation.submit/goal_evaluation_wire_criterion_ref_invalid。随后明确：程序能兜底的由程序兜底，不应把这些可恢复情况变成报错/不可用。

本轮与F15一并完成；不修改普通认知/WakeUp Tool权限，不访问生产。F15已有工作区源码，F16必须保留30min周期、Planner空位准入及0063迁移。不要仅在Prompt要求模型自我纠正，也不要通过伪造Goal完成或移除授权/来源/CAS来消除报错。

## 主代理已核对的根因

1. goal_evaluation_tools.go的physical policy在missing roots时一直forced Tools；专用Agent没有连续无进展收束，底层生产迭代近乎无上限。非retryable业务拒绝依然作为正常Tool结果交给模型，阻断外层claim重试预算生效。
2. executeGoalEvaluationSubmissionTx的target journal key已有accepted记录而digest变化会goal_submission_conflict；该错误不是并发CAS冲突，程序知道原结果，可返回它。
3. all normalized judgments=unknown且impact=no_change被核心验证拒绝；合法保守表达应needs_evidence。native instruction漏掉旧instruction已有的对应规则，可直接在受信native适配层归一，保留核心严格验证。
4. hydrateJudgments把未知、跨Goal、跨对象、重复criterion都归为一个错误。冻结refs可发现且跨claim稳定；真正缺口是程序缺少确定性规范化/去重与不能解析时的安全处置。
5. accepted_submissions投影丢失journal key/kind且map遍历无序，plan与root返回形状一致，恢复模型无法确定哪个已提交。

源码定位见本任务research/goal-submission-audit.md。主代理已亲读将受影响的协议、Tool事务、wire hydration、finalizer及physical request policy。

## 程序处理契约（F16覆盖旧严格报错的模型表示层行为）

### 已接受目标的重复提交

同一私有request、同capability/goal/object target已有accepted journal时，返回原已提交output与replayed/已处理说明。即使模型又给不同payload，也不执行第二次写入、不覆盖原digest/历史。记录本次变更被忽略/复用的程序诊断，向模型明确已接受的是root/plan/object以及下一未解决root。底层native call ID或权限/claim/CAS冲突仍执行原授权，不与这种target重复混为一谈。

### all-unknown的impact

仅受信专用native adapter，将本次缺项补unknown后无任何known judgment的no_change归一为needs_evidence，再调用原ApplyGoalEvaluation验证。保留真实unknown、原证据与来源规则，不添加satisfied/伪造证据/完成进度。若缺等待状态，可给最小技术性“等待可用证据，按下一评估周期复核”处置，不能编造业务动作或新的成功标准。归一记录可排查元数据，普通核心validator可继续严格拒绝未经适配的不一致候选。

### criterion引用

只在本次冻结Goal/对象作用域内做确定性处理：canonical ref、明确唯一的等价标识可规范化，完全重复的同criterion judgment可去重/合并同义证据。不得按模糊文本、数组位置或另一个Goal的ref猜测归属。相互矛盾、未知或跨scope的引用不能用于证明完成。

目标身份已可信绑定但内容无法确定修复时，返回正常的handled/awaiting_evidence处置回执（assessment_accepted=false、mutation_committed=false及合法refs/原因），保留原Goal/进度/证据。不要写假unknown judgment覆盖已有真实进度；不要把未完成root伪装成accepted。真实来源/版本/授权错误仍保留原因并转既有fresh/defer路径，不无限重复原冻结输入。

### 无进展收束

在Goal Evaluation专用session中观察持久accepted journal与真实Tool trace；连续有限轮没有新增accepted target时（包括反复replay旧plan，而不只相同error字符串），程序停止强制重复Tool。可复用physical request policy，以显式无Tool/final-summary请求收束；若现有框架不接受显式空catalog，只做使这一task-owned合法终止可用的最小共享修复，其他Agent默认不变。

程序处理后的cycle可以正常结束并明确disposition=awaiting_evidence/assessment_complete=false、未解决goal refs及已有真实结果；这是操作性收束，不是宣称这些Goal评估accepted或完成。保留未解决sources为pending，不造native calls、成功memo、成功判断或完成事件。原request可作为handled cycle结清，后续新周期/真实事实沿F15调度继续；同一claim/workflow重放不得重新进入刚才的无进展循环。真实基础设施/Provider/取消错误保持其真实原因。

### 可辨认恢复输入

accepted_submissions须稳定排序，并显式包含capability_name/submission_kind、frozen goal_ref/object_ref、原output、replayed等；不能把plan accepted当root coverage，不暴露raw IDs。必要的已处理/待证据提示也可在恢复输入明确给出，但不复制大段失败payload。

## 修改边界

F16源码预计：core/goal_evaluation_tools.go、goal_evaluation_wire.go或独立native-normalization helper、goal_evaluation_submission_runtime.go、goal_evaluation_task.go、goal_evaluation_runtime.go的处理收束衔接；必要时eino_model_request_policy.go允许task明确禁用所有Tools以正常总结。对应纯行为、实际脚本HTTP/native loop与可选隔离PG回归。保留F15现有未提交改动，不改其他Agent迭代规则和Tool权限。

## 必须验证

- 已成功plan不同payload重交返回原回执，真实写一次；原digest/accepted journal不覆盖；root仍标未提交。
- 空/全unknown+no_change自动needs_evidence并保持未完成；不需要模型再猜impact；不把无证据satisfied变为完成。
- 合法alias/完全重复可确定处理；未知、跨Goal、跨对象、冲突duplicate被安全待处理，旧Goal/进度/源证据不受污染。
- accepted输入稳定区分plan/root/object，不因map顺序或partial retry混淆。
- 实际native HTTP Provider持续重放plan或持续给无效criterion，超过有限物理请求预算测试立即失败，不能依靠超时；程序正常收束、保留成功兄弟与未处理sources、重放无额外模型/副作用。正常多个不同目标持续进展可完整提交，不被提前截断。
- 明确无Tools的final-summary请求无tool_choice/parallel工具语义且保留原summary grammar，普通native可选Tools与原tool-free repair矩阵回归。
- 无PostgreSQL环境时，持久化用例仍如实SKIP；纯helper和脚本native loop必须实际执行，不以迁移head断言冒充业务RED/GREEN。

## 实施收敛

F16已实现并完成业务helper/native HTTP/SSE敏感性RED→GREEN。独立检查补齐Provider忽略无Tools的程序性保险，原始响应/usage先记录、违规ToolCall不执行，真实503/取消保真。最终Go/workspace通过，PG持久化SKIP，详见verification/goal-runtime-check-summary.json；未提交，不部署。
