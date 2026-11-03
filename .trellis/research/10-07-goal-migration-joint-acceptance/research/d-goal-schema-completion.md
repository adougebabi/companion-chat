# D：无阶段目标的完成输出 schema 修复（2026-10-08）

## 现场事实
用户的新 response 已返回 impact=completed，但还声称空 object_ref 的阶段完成，并为同一目标提出后续聊天计划。
只读接口确认目标 active/progress0/evaluations=[]/stages=[]。首次查看请求 goal_review_event_94_... processing；稍后最新请求已切换到 goal_review_event_95_...，5次尝试后 failed，错误 goal_evaluation_wire_source_ref_invalid。
不能将后来请求的错误直接归给用户贴出的截断 response；该 response 内的非空阶段评估对象与空引用本身无效。
登录只用于读取，最后退出并验证 authenticated=false，删除本次 cookie。未触发 Owner重评、真实Provider或修改生产状态。

## 根因与修复
此前只要求模型省略不存在对象，但通用 schema 仍广告这些字段且 refs 可为任意字符串。
RunGoalEvaluationTask 现从冻结 typed binding 创建实际 Provider schema：Goal/标准/证据/阶段/承诺/依赖/目标Actor refs 为排序短引用枚举，无原始ID/版本。
无阶段/承诺时，移除对应 evaluation 属性；无阶段时移除 review.stage_ref；新建计划保留，但没有已有对象时不提供 plan.object_ref。无来源的证据数组 maxItems=0。
review StageRef DTO 序列化改 omitempty；解码/领域字段不变。正式 Task 在 hydration 前校验实际 schema；非空 ghost、null stage、空 commitment_evaluations、空 review.stage_ref 在不可用时都在 formal schema 边界拒绝。未知 ref 仍由绑定/来源/版本领域校验独立拒绝。没有丢弃虚构阶段或绕过证据。
完成 Goal 不应再为自己提出计划，已固定在协议；既有 applyGoalPlanTx 对非active父Goal为no-op，不创建子对象，也不重开目标。保留此终态保护语义，未为无副作用的多余计划新增完成失败条件。
固定系统协议/人格/任务定义仍置前；动态 schema 在 Provider payload 后于messages，与原始ID隔离。稳定物理前缀回归通过。

## 质量结果
使用本次唯一 disposable PG容器和逐测试隔离数据库，最终：
- go test -race -json ./internal/core -run '^(TestGoal|TestFormalConversationWithoutIntentionCompletesOriginalExpressionGoal)' -count=1
- 116 test PASS / 104 leaf PASS / 0 FAIL / 0 SKIP。
- go vet ./...、go build ./...、gofmt、git diff --check通过。

首轮早期schema拒绝导致3处旧错误断言失效，另一个Review测试夹具序列化空stage_ref被新schema拒绝；修正为更早的adk_final_contract_invalid断言和省略空stage_ref后复跑通过。保留首轮失败日志，不计为通过。
独立trellis-check指出两处边界：一是终态计划已有no-op，主线程保留既有领域合同并补规范解释；二是直接DTO hydration的零值可等同缺省，主线程通过实际Task→Provider→schema→binder数据库用例证明不可用字段在正式流中先被拒绝，不靠Provider厂商遵从。

## 本次边界与剩余真实验收
仅本地源码、测试和规范变更；无数据库迁移、API/UI变更、生产状态修补或真实模型调用。
父任务仍in_progress。需要部署后通过正式入口让用户本地模型重新评估，并观察 Goal completed / 成功Evaluation / Resolution。schema收紧仍不能保证语义判断正确，真实验收交由用户。
