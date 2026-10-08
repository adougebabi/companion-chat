# 2026-10-08 Goal语义边界、稳定前缀及重复评估闭环

用户指出goal_evaluation实际prompt包含大量ID/版本，要求评估TOON、同Agent保持稳定前缀，以及同证据不反复评估。源码证实 assembled path bypass通用formatter，原JSON是实际HTTP wire，不是仅诊断显示。

## 最终行为

- per-call immutable typed short refs映射Goal/criterion/Stage/Commitment/dependency/source/actor/object；无raw DB IDs/versions/FKs/audit，closed输出也不要求model echo版本。Core从同一冻结快照恢复ID/revision/criteria version，保留真实source、profile、CAS及关系门禁。严格拒绝unknown/kind/owner/duplicate/legacy raw-authority输出。
- 共享Goal的真实Actor消息保留实际发布profile；私有目标/对象的作用域不放宽。参与者转actor refs。Source refs使用eN，与durable source:N区分。Common Runtime Goal嵌套execution、只读评估的编译人格wrapper及legacy JSON-shaped人格事实也清理metadata；其他原生persona switching selector协议保留。
- GoalAuthority的EntityID/TargetActorID是json:"-"，持久化entry保留GoalID/TargetActorID；binding在私有副本恢复，两种输入生成同一语义packet且不改冻结快照。实际all/any optional_ids转optional_refs。
- 固定协议/人格system → required STABLE TASK CONTEXT user定义 → slow-first RUNTIME CONTEXT user事实 → 原chronological role history → current user一次。Stable definitions不重复放尾部，不升为system policy，不可prune；按真实wire计预算/diagnostics。Nested typed policy先canonicalize，防止JSON复制改变相同前缀。
- 当前采用compact semantic JSON；同数据JSON/TOON/YAML比较保留，非强制换格式。无全局cap增加，无原文/criteria截断。真实KV命中/TTFT/模型语义未测。
- 成功评估后在既有request.result保存per-Goal memo，无迁移/进程内状态依赖。own修订/判断/Plan IDs、instant/as_of秒级clock和mood decay不dirty。标准/相关slow约束、source validity/version、新source、due review和Owner force仍评估；deadline/commitment边界按coarse phase dirty。
- stamp读取按实例/Goal，effective profile进入constraint签名，防止共享请求的空profile与同一working profile造成重复。Owner force合并保留，JSONB array extraction显式括号。内部goal_revision不推进Review证据水位。
- Memo仅完整successful domain transaction写入；cache hit不重放旧model output，不伪造Evaluation/Attempt/Resolution。新proof未入选/六Goal及source remainder继续；mandatory proof占满但无新source可入选时bounded可见失败。缓存/真实模型结算都复查claim/source/Goal/Foundation/Life；合法初始Foundation revision0也按实际CAS处理。

## 编码及前缀测量

同一脱敏fixture：old durable baseline2569B/~2977估算tokens；semantic JSON1874B/~2108；TOON/YAML2277B/~2612。JSON比baseline少约27.1%bytes、29.2%估算tokens；本fixture TOON比JSON大约21.5%bytes。仅fixture测量，不能外推全部任务或当实际tokenizer结果。

组装fixture：13650wire bytes/~13571保守估算input tokens，其中stable context1308B/~1449估算tokens。测试证明clock/evidence变化时system+stable messages逐字节相同、slow facts位于volatile前，definitions变化才更新对应前缀；不声称已命中本地Provider cache。

## 验证证据

- d-goal-semantic-cache-final-go-race.jsonl：1891 PASS events / 1745 leaf PASS / 0 FAIL / 36 SKIP。真实Provider/媒体/联合设施缺配置的SKIP不计通过。
- d-goal-semantic-cache-targeted-race.jsonl：130 PASS events / 0 FAIL / 0 SKIP，实际一次性PG/Redis环境。
- 全Go vet/build与gofmt/diff check通过。公开HTTP/client/schema未修改，未重复无改动的Web门禁。
- 实际HTTPpayload捕获metadata omission，source/cross-profile/domainquery/recommendation closure；persisted replay无mutation；shared/private scope；未知/错类型/版本注入拒绝；same-proof重放不调用模型及阶段不重复；force合并/newproof/criteria变更/失败重试/Review水位；source remainder大输入续批和预算无progress保护；稳定context budget/copy/role顺序。

失败尝试摘录保留：instant签名永远miss、共享source profile错拒、revision0错拒、Runtime criterion IDs/versions及人格profile残留、force JSONB precedence、跨requestprofile误cache-miss、snapshot json:-身份及typed policy顺序差异、fixture强制available_at小clock skew、short ref与durable ref碰名造成检测误判、stable/current拆分旧断言。每个都修复后重跑，不算通过证据。

本轮没有调用用户真实Provider、改线上数据/队列、部署或强制Goal完成。自有设施只用于回归，实际缓存命中和真实模型验收待更新API/Worker后验证。整体及D仍in_progress，用户人格JSON保留。
